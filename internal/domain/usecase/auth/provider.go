package auth

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"golang.org/x/sync/singleflight"

	"github.com/tetiva-app/client/internal/domain/entities"
)

// Token states reported to the Auth tab.
const (
	TokenStateNone               = "none"
	TokenStateValid              = "valid"
	TokenStateExpiredRefreshable = "expired-refreshable"
	TokenStateExpired            = "expired"
)

// refreshSkew keeps a token that is about to expire from being sent.
const refreshSkew = 30 * time.Second

// ErrTokenRequired means the grant cannot acquire a token on its own: only the
// Auth tab's browser flow can obtain one.
var ErrTokenRequired = errors.New("auth: interactive grant needs a token")

// TokenStatus is what the Auth tab shows for an owner's stored token.
type TokenStatus struct {
	State     string
	ExpiresAt time.Time
}

// Provider turns an OAuth 2.0 configuration into an access token, caching and
// refreshing through the local token store.
type Provider interface {
	// AccessToken returns a usable token, refreshing or acquiring one if needed.
	AccessToken(ctx context.Context, owner entities.AuthOwner, cfg OAuth2Config) (string, error)
	// Peek returns the cached token only, never touching the network.
	Peek(ctx context.Context, owner entities.AuthOwner, cfg OAuth2Config) (string, bool, error)
	// Status describes the stored token for the given configuration.
	Status(ctx context.Context, owner entities.AuthOwner, cfg OAuth2Config) (TokenStatus, error)
	// Fetch acquires a token even when a valid one is cached ("Get token").
	Fetch(ctx context.Context, owner entities.AuthOwner, cfg OAuth2Config) (*Token, error)
	// Clear tombstones the owner's token.
	Clear(ctx context.Context, owner entities.AuthOwner) error
}

type provider struct {
	repo   TokenRepository
	client *http.Client
	now    func() time.Time
	group  singleflight.Group
}

var _ Provider = (*provider)(nil)

// NewProvider wires the token store to the token endpoint client. A nil client
// or clock falls back to the production defaults.
func NewProvider(repo TokenRepository, client *http.Client, now func() time.Time) Provider {
	if client == nil {
		client = NewTokenHTTPClient()
	}
	if now == nil {
		now = time.Now
	}

	return &provider{repo: repo, client: client, now: now}
}

func (p *provider) AccessToken(ctx context.Context, owner entities.AuthOwner, cfg OAuth2Config) (string, error) {
	hash := ConfigHash(cfg)

	// Keyed by configuration as well as owner, so an unsaved editor buffer never
	// receives the token acquired for the saved one.
	result, err := p.share(ctx, "use|"+ownerKey(owner)+"|"+hash, func(runCtx context.Context) (any, error) {
		return p.accessToken(runCtx, owner, cfg, hash)
	})
	if err != nil {
		return "", err
	}

	return result.(string), nil
}

func (p *provider) accessToken(ctx context.Context, owner entities.AuthOwner, cfg OAuth2Config, hash string) (string, error) {
	const funcName = "auth.Provider.AccessToken"

	stored, err := p.repo.Get(ctx, owner)
	if err != nil {
		return "", fmt.Errorf("%s: %w", funcName, err)
	}

	if stored != nil && stored.ConfigHash == hash {
		if p.usable(stored.Token) {
			return stored.AccessToken, nil
		}
		if stored.RefreshToken != "" {
			tok, refreshErr := p.refresh(ctx, owner, cfg, hash, stored.RefreshToken)
			if refreshErr == nil {
				return tok.AccessToken, nil
			}
			if errors.Is(refreshErr, ErrStaleToken) || errors.Is(refreshErr, ErrOwnerGone) {
				return "", refreshErr
			}
			// A revoked refresh token would otherwise wedge the owner forever;
			// client_credentials and password can just acquire a new one.
			if _, formErr := acquisitionForm(cfg); formErr != nil {
				return "", fmt.Errorf("oauth2: the stored refresh token was rejected — press Get token in the Auth tab to run the browser flow again, or Clear to drop the stored token: %w", refreshErr)
			}
		}
	}

	form, err := acquisitionForm(cfg)
	if err != nil {
		return "", err
	}
	tok, err := p.acquire(ctx, owner, cfg, form, "")
	if err != nil {
		return "", err
	}

	return tok.AccessToken, nil
}

func (p *provider) Peek(ctx context.Context, owner entities.AuthOwner, cfg OAuth2Config) (string, bool, error) {
	const funcName = "auth.Provider.Peek"

	stored, err := p.repo.Get(ctx, owner)
	if err != nil {
		return "", false, fmt.Errorf("%s: %w", funcName, err)
	}
	// A hash mismatch is a cache miss, not a reason to drop the row: the caller
	// may be probing an unsaved editor buffer.
	if stored == nil || stored.ConfigHash != ConfigHash(cfg) || !p.usable(stored.Token) {
		return "", false, nil
	}

	return stored.AccessToken, true, nil
}

func (p *provider) Status(ctx context.Context, owner entities.AuthOwner, cfg OAuth2Config) (TokenStatus, error) {
	const funcName = "auth.Provider.Status"

	stored, err := p.repo.Get(ctx, owner)
	if err != nil {
		return TokenStatus{}, fmt.Errorf("%s: %w", funcName, err)
	}
	if stored == nil || stored.ConfigHash != ConfigHash(cfg) {
		return TokenStatus{State: TokenStateNone}, nil
	}
	if p.usable(stored.Token) {
		return TokenStatus{State: TokenStateValid, ExpiresAt: stored.ExpiresAt}, nil
	}
	if stored.RefreshToken != "" {
		return TokenStatus{State: TokenStateExpiredRefreshable, ExpiresAt: stored.ExpiresAt}, nil
	}

	return TokenStatus{State: TokenStateExpired, ExpiresAt: stored.ExpiresAt}, nil
}

func (p *provider) Fetch(ctx context.Context, owner entities.AuthOwner, cfg OAuth2Config) (*Token, error) {
	hash := ConfigHash(cfg)

	result, err := p.share(ctx, "fetch|"+ownerKey(owner)+"|"+hash, func(runCtx context.Context) (any, error) {
		return p.fetch(runCtx, owner, cfg, hash)
	})
	if err != nil {
		return nil, err
	}

	return result.(*Token), nil
}

func (p *provider) fetch(ctx context.Context, owner entities.AuthOwner, cfg OAuth2Config, hash string) (*Token, error) {
	const funcName = "auth.Provider.Fetch"

	form, err := acquisitionForm(cfg)
	if errors.Is(err, ErrTokenRequired) {
		// An interactive grant has no flow in stage A; a stored refresh token is
		// the only way this button can renew one.
		stored, storeErr := p.repo.Get(ctx, owner)
		if storeErr != nil {
			return nil, fmt.Errorf("%s: %w", funcName, storeErr)
		}
		if stored == nil || stored.ConfigHash != hash || stored.RefreshToken == "" {
			return nil, err
		}

		return p.refresh(ctx, owner, cfg, hash, stored.RefreshToken)
	}
	if err != nil {
		return nil, err
	}

	return p.acquire(ctx, owner, cfg, form, "")
}

func (p *provider) Clear(ctx context.Context, owner entities.AuthOwner) error {
	const funcName = "auth.Provider.Clear"

	if err := p.repo.Clear(ctx, owner); err != nil {
		return fmt.Errorf("%s: %w", funcName, err)
	}

	return nil
}

// acquire runs one token endpoint call and stores the result. keepRefresh is the
// refresh token to retain when the response omits one.
func (p *provider) acquire(ctx context.Context, owner entities.AuthOwner, cfg OAuth2Config, form url.Values, keepRefresh string) (*Token, error) {
	const funcName = "auth.Provider.acquire"

	if err := cfg.validate(); err != nil {
		return nil, err
	}

	// Reserved before any network I/O: a clear, delete, sweep or edit in between bumps the generation,
	// so the Put below is refused instead of resurrecting a token nobody wants any more.
	generation, err := p.repo.Reserve(ctx, owner)
	if err != nil {
		if errors.Is(err, ErrOwnerGone) {
			return nil, err
		}

		return nil, fmt.Errorf("%s: %w", funcName, err)
	}

	tok, err := requestToken(ctx, p.client, cfg, form, p.now())
	if err != nil {
		return nil, err
	}
	if tok.RefreshToken == "" {
		tok.RefreshToken = keepRefresh
	}

	committed, err := commitToken(ctx, p.repo, owner, generation, ConfigHash(cfg), tok, p.now())
	if err != nil {
		if errors.Is(err, ErrStaleToken) {
			return nil, err
		}

		return nil, fmt.Errorf("%s: %w", funcName, err)
	}

	return committed, nil
}

// refresh exchanges the stored refresh token, deduplicated across "Get token" and sends: an IdP
// that rotates refresh tokens invalidates the pair when the same one is presented twice.
func (p *provider) refresh(ctx context.Context, owner entities.AuthOwner, cfg OAuth2Config, hash, refreshToken string) (*Token, error) {
	result, err := p.share(ctx, "refresh|"+ownerKey(owner)+"|"+hash, func(runCtx context.Context) (any, error) {
		return p.acquire(runCtx, owner, cfg, refreshForm(cfg, refreshToken), refreshToken)
	})
	if err != nil {
		return nil, err
	}

	return result.(*Token), nil
}

// share runs fn once per key for all concurrent callers. The flight runs detached from the caller's
// context — one caller walking away must not abort it — while each caller returns when its own ends.
func (p *provider) share(ctx context.Context, key string, fn func(context.Context) (any, error)) (any, error) {
	ch := p.group.DoChan(key, func() (any, error) {
		runCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), tokenEndpointTimeout)
		defer cancel()

		return fn(runCtx)
	})

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case result := <-ch:
		return result.Val, result.Err
	}
}

func (p *provider) usable(t Token) bool {
	return tokenUsable(t, p.now())
}

// acquisitionForm builds the grant parameters, rejecting configurations stage A
// cannot acquire on its own.
func acquisitionForm(cfg OAuth2Config) (url.Values, error) {
	if err := cfg.validate(); err != nil {
		return nil, err
	}

	switch cfg.Grant {
	case GrantClientCredentials:
		return withScopeAudience(cfg, url.Values{"grant_type": {GrantClientCredentials}}), nil
	case GrantPassword:
		return withScopeAudience(cfg, url.Values{
			"grant_type": {GrantPassword},
			"username":   {cfg.Username},
			"password":   {cfg.Password},
		}), nil
	default:
		return nil, ErrTokenRequired
	}
}

func refreshForm(cfg OAuth2Config, refreshToken string) url.Values {
	return withScopeAudience(cfg, url.Values{
		"grant_type":    {"refresh_token"},
		"refresh_token": {refreshToken},
	})
}

// withScopeAudience adds the configured scope and audience here rather than in postForm: the browser
// flows send their own forms, and the code exchange and the device poll carry neither.
func withScopeAudience(cfg OAuth2Config, form url.Values) url.Values {
	if cfg.Scope != "" {
		form.Set("scope", cfg.Scope)
	}
	if cfg.Audience != "" {
		form.Set("audience", cfg.Audience)
	}

	return form
}

func ownerKey(o entities.AuthOwner) string {
	return o.WorkspaceID.String() + "|" + o.Kind + "|" + o.ID.String()
}
