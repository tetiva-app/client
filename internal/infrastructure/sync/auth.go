package sync

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/zalando/go-keyring"
	"golang.org/x/sync/singleflight"

	authv1 "github.com/tetiva-app/proto/go/gophercourier/auth/v1"

	"github.com/tetiva-app/client/internal/domain/usecase/auth"
	"github.com/tetiva-app/client/internal/infrastructure/repository/sqlite"
)

const (
	keyringService = "tetiva"
	// keyringLegacyService is the pre-rebrand service name; loadRefreshToken
	// falls back to it and migrates the entry so an existing login survives.
	keyringLegacyService = "gophercourier"
	// keyringLegacyKey is the unscoped account name from pre-v0.9.6 builds,
	// kept for best-effort cleanup on Logout — see keyringKeyFor.
	keyringLegacyKey = "refresh_token"
	// keyringKeyPrefix is combined with the per-install client_id: unscoped
	// entries collide when two installs share one Mac and email, breaking refresh.
	keyringKeyPrefix = "refresh_token:"
	// refreshLeeway is the time before token expiry when a proactive refresh is triggered.
	refreshLeeway = 2 * time.Minute
	// accessTokenTTL is the assumed lifetime of an access token (matches server default).
	accessTokenTTL = 15 * time.Minute
	// serverLogoutTimeout bounds the best-effort session revoke on Logout, so a
	// local sign-out does not wait on an unreachable server.
	serverLogoutTimeout = 3 * time.Second
	// CurrentAuthGeneration is bumped when a release must sign every install out
	// once; sync_config.auth_generation below it triggers the cleanup on startup.
	CurrentAuthGeneration = 1
	// syncConfigWriteTimeout bounds the one config write of the commit phase: a
	// busy SQLite must not eat the whole budget the sign-in gives OnApproved.
	syncConfigWriteTimeout = 1500 * time.Millisecond
)

// ErrAuthExpired means the refresh token was rejected: only an interactive
// re-login can restore sync. Sync engines stop retrying on it.
var ErrAuthExpired = errors.New("sync auth expired: refresh token rejected")

// ErrSessionReplaced means the answer arrived after another session was adopted;
// the caller retries and gets the new session's token.
var ErrSessionReplaced = errors.New("sync auth: session was replaced while refreshing")

// keyringKeyFor falls back to the legacy unscoped key when clientID is
// empty (early boot or tests that never SetClientID).
func keyringKeyFor(clientID string) string {
	if clientID == "" {
		return keyringLegacyKey
	}
	return keyringKeyPrefix + clientID
}

// AuthResult carries the account facts the UI needs right after Login/Register.
type AuthResult struct {
	Email                     string
	RequiresEmailVerification bool
}

// SessionInfo is one device signed in to the account: the server keeps a single
// active session per client_id.
type SessionInfo struct {
	ID         string
	ClientID   string
	UserAgent  string
	IP         string
	LastUsedAt time.Time
	IsCurrent  bool
}

// SyncAuthManager keeps the access token in memory and the refresh token in the OS keychain (fallback: sync_config).
type SyncAuthManager struct {
	mu          sync.RWMutex
	accessToken string
	expiresAt   time.Time
	clientID    string
	activeOrgID string
	configRepo  sqlite.SyncConfigRepository
	sf          singleflight.Group
	// sessionEpoch grows on every session boundary (a Logout, or a sign-in adopting another
	// account), so a store still in flight from the previous session undoes itself.
	sessionEpoch atomic.Uint64
	// keyringSet is swapped in tests: the real keychain write can block for seconds.
	keyringSet func(service, account, token string) error
}

func NewSyncAuthManager(configRepo sqlite.SyncConfigRepository) *SyncAuthManager {
	return &SyncAuthManager{
		configRepo: configRepo,
		keyringSet: keyring.Set,
	}
}

// SetClientID is called during initialization, from config.
func (a *SyncAuthManager) SetClientID(clientID string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.clientID = clientID
}

// IsLoggedIn returns true if an access token is present in memory (even if expired — refresh will be attempted).
func (a *SyncAuthManager) IsLoggedIn() bool {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.accessToken != ""
}

// GetActiveOrgID returns the org ID of the last Login/Register/Refresh; empty means none this session.
func (a *SyncAuthManager) GetActiveOrgID() string {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.activeOrgID
}

func (a *SyncAuthManager) Login(ctx context.Context, client *GRPCClient, email, password string) (*AuthResult, error) {
	const funcName = "SyncAuthManager.Login"

	cfg, err := a.configRepo.GetOrCreate(ctx)
	if err != nil {
		return nil, fmt.Errorf("%s: get config: %w", funcName, err)
	}

	clientID := cfg.ClientID
	a.mu.Lock()
	a.clientID = clientID
	a.mu.Unlock()

	req := authv1.LoginRequest_builder{
		Email:    email,
		Password: password,
		ClientId: clientID,
	}.Build()

	resp, err := client.Auth().Login(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("%s: login rpc: %w", funcName, err)
	}

	slog.Info("sync: login rpc ok", "email", email, "active_org_id", resp.GetActiveOrgId())

	a.storeTokens(resp.GetAccessToken(), resp.GetRefreshToken(), resp.GetActiveOrgId())

	if err := a.persistSession(ctx, cfg, email, resp.GetRefreshToken()); err != nil {
		return nil, fmt.Errorf("%s: %w", funcName, err)
	}

	return &AuthResult{
		Email:                     email,
		RequiresEmailVerification: resp.GetRequiresEmailVerification(),
	}, nil
}

func (a *SyncAuthManager) Register(ctx context.Context, client *GRPCClient, email, password, name, locale string) (*AuthResult, error) {
	const funcName = "SyncAuthManager.Register"

	cfg, err := a.configRepo.GetOrCreate(ctx)
	if err != nil {
		return nil, fmt.Errorf("%s: get config: %w", funcName, err)
	}

	clientID := cfg.ClientID
	a.mu.Lock()
	a.clientID = clientID
	a.mu.Unlock()

	req := authv1.RegisterRequest_builder{
		Email:    email,
		Password: password,
		Name:     name,
		ClientId: clientID,
		Locale:   locale,
	}.Build()

	resp, err := client.Auth().Register(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("%s: register rpc: %w", funcName, err)
	}

	a.storeTokens(resp.GetAccessToken(), resp.GetRefreshToken(), resp.GetActiveOrgId())

	if err := a.persistSession(ctx, cfg, email, resp.GetRefreshToken()); err != nil {
		return nil, fmt.Errorf("%s: %w", funcName, err)
	}

	return &AuthResult{
		Email:                     email,
		RequiresEmailVerification: resp.GetRequiresEmailVerification(),
	}, nil
}

// refreshToken is carried into cfg because cfg was read before storeTokens mirrored the
// rotated token into sync_config — writing cfg back as-is would restore the old token.
func (a *SyncAuthManager) persistSession(ctx context.Context, cfg *sqlite.SyncConfig, email, refreshToken string) error {
	cfg.UserEmail = email
	cfg.Enabled = true
	cfg.AuthGeneration = CurrentAuthGeneration
	cfg.ReauthRequired = false
	if refreshToken != "" {
		cfg.RefreshToken = refreshToken
	}
	if err := a.configRepo.Update(ctx, cfg); err != nil {
		return fmt.Errorf("update config: %w", err)
	}
	return nil
}

// AdoptSignIn is the tail of Login for a session obtained in the browser: the whole config
// goes out in one Update, so a partial failure cannot leave a fresh token by a stale server_url.
func (a *SyncAuthManager) AdoptSignIn(ctx context.Context, serverURL string, tokens auth.SignInTokens) (*AuthResult, error) {
	const funcName = "SyncAuthManager.AdoptSignIn"

	cfg, err := a.configRepo.GetOrCreate(ctx)
	if err != nil {
		return nil, fmt.Errorf("%s: get config: %w", funcName, err)
	}
	// The keychain entry is scoped by client_id, so it has to be known before
	// the token is stored.
	a.SetClientID(cfg.ClientID)

	cfg.ServerURL = serverURL
	cfg.UserEmail = tokens.Email
	cfg.Enabled = true
	cfg.RefreshToken = tokens.RefreshToken
	cfg.AuthGeneration = CurrentAuthGeneration
	cfg.ReauthRequired = false

	writeCtx, cancel := context.WithTimeout(ctx, syncConfigWriteTimeout)
	defer cancel()
	if err := a.configRepo.Update(writeCtx, cfg); err != nil {
		return nil, fmt.Errorf("%s: update config: %w", funcName, err)
	}

	// From here on a refresh of the previous session must discard its answer.
	a.sessionEpoch.Add(1)
	a.storeTokensNoMirror(tokens.AccessToken, tokens.RefreshToken, tokens.ActiveOrgID)

	return &AuthResult{
		Email:                     tokens.Email,
		RequiresEmailVerification: tokens.RequiresEmailVerification,
	}, nil
}

// GetMe reports the account behind the stored credentials and whether its email is confirmed.
func (a *SyncAuthManager) GetMe(ctx context.Context, client *GRPCClient) (email string, verified bool, err error) {
	const funcName = "SyncAuthManager.GetMe"

	token, err := a.GetAccessToken(ctx, client)
	if err != nil {
		return "", false, fmt.Errorf("%s: %w", funcName, err)
	}

	resp, err := client.Auth().GetMe(ContextWithAuth(ctx, token), authv1.GetMeRequest_builder{}.Build())
	if err != nil {
		return "", false, fmt.Errorf("%s: get me rpc: %w", funcName, err)
	}

	user := resp.GetUser()
	return user.GetEmail(), user.GetEmailVerified(), nil
}

// The server rate-limits ResendVerification; the codes.ResourceExhausted status survives the wrap.
func (a *SyncAuthManager) ResendVerification(ctx context.Context, client *GRPCClient) error {
	const funcName = "SyncAuthManager.ResendVerification"

	token, err := a.GetAccessToken(ctx, client)
	if err != nil {
		return fmt.Errorf("%s: %w", funcName, err)
	}

	_, err = client.Auth().ResendVerification(ContextWithAuth(ctx, token), authv1.ResendVerificationRequest_builder{}.Build())
	if err != nil {
		return fmt.Errorf("%s: resend verification rpc: %w", funcName, err)
	}

	return nil
}

// Me lists the devices currently signed in to the account.
func (a *SyncAuthManager) Me(ctx context.Context, client *GRPCClient) ([]SessionInfo, error) {
	const funcName = "SyncAuthManager.Me"

	token, err := a.GetAccessToken(ctx, client)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", funcName, err)
	}

	resp, err := client.Auth().Me(ContextWithAuth(ctx, token), authv1.MeRequest_builder{}.Build())
	if err != nil {
		return nil, fmt.Errorf("%s: me rpc: %w", funcName, err)
	}

	views := resp.GetSessions()
	sessions := make([]SessionInfo, 0, len(views))
	for _, v := range views {
		info := SessionInfo{
			ID:        v.GetId(),
			ClientID:  v.GetClientId(),
			UserAgent: v.GetUserAgent(),
			IP:        v.GetIp(),
			IsCurrent: v.GetIsCurrent(),
		}
		if v.HasLastUsedAt() {
			info.LastUsedAt = v.GetLastUsedAt().AsTime()
		}
		sessions = append(sessions, info)
	}
	return sessions, nil
}

// RevokeSession signs one other device out; the server answers NotFound for someone else's session.
func (a *SyncAuthManager) RevokeSession(ctx context.Context, client *GRPCClient, sessionID string) error {
	const funcName = "SyncAuthManager.RevokeSession"

	token, err := a.GetAccessToken(ctx, client)
	if err != nil {
		return fmt.Errorf("%s: %w", funcName, err)
	}

	req := authv1.RevokeSessionRequest_builder{SessionId: sessionID}.Build()
	if _, err := client.Auth().RevokeSession(ContextWithAuth(ctx, token), req); err != nil {
		return fmt.Errorf("%s: revoke session rpc: %w", funcName, err)
	}
	return nil
}

// LogoutAll signs every other device out and reports how many sessions were revoked.
func (a *SyncAuthManager) LogoutAll(ctx context.Context, client *GRPCClient) (int, error) {
	const funcName = "SyncAuthManager.LogoutAll"

	token, err := a.GetAccessToken(ctx, client)
	if err != nil {
		return 0, fmt.Errorf("%s: %w", funcName, err)
	}

	resp, err := client.Auth().LogoutAll(ContextWithAuth(ctx, token), authv1.LogoutAllRequest_builder{}.Build())
	if err != nil {
		return 0, fmt.Errorf("%s: logout all rpc: %w", funcName, err)
	}
	return int(resp.GetRevokedCount()), nil
}

// GetAccessToken returns a valid access token, refreshing it if expired or within leeway.
func (a *SyncAuthManager) GetAccessToken(ctx context.Context, client *GRPCClient) (string, error) {
	const funcName = "SyncAuthManager.GetAccessToken"

	// Fast path: token is valid and not within leeway.
	a.mu.RLock()
	if a.accessToken != "" && time.Now().Add(refreshLeeway).Before(a.expiresAt) {
		token := a.accessToken
		a.mu.RUnlock()
		return token, nil
	}
	a.mu.RUnlock()

	// Slow path: refresh needed — deduplicate concurrent calls.
	type result struct {
		token string
		err   error
	}

	v, err, _ := a.sf.Do("refresh", func() (interface{}, error) {
		// Re-check under write lock in case another goroutine already refreshed.
		a.mu.Lock()
		if a.accessToken != "" && time.Now().Add(refreshLeeway).Before(a.expiresAt) {
			token := a.accessToken
			a.mu.Unlock()
			return result{token: token}, nil
		}
		clientID := a.clientID
		a.mu.Unlock()
		epoch := a.sessionEpoch.Load()

		refreshToken := a.loadRefreshToken()
		if refreshToken == "" {
			return result{err: fmt.Errorf("%s: no refresh token available", funcName)}, nil
		}

		req := authv1.RefreshRequest_builder{
			RefreshToken: refreshToken,
			ClientId:     clientID,
		}.Build()

		resp, err := client.Auth().Refresh(ctx, req)
		if err != nil {
			if status.Code(err) == codes.Unauthenticated {
				return result{err: fmt.Errorf("%s: %w", funcName, ErrAuthExpired)}, nil
			}
			return result{err: fmt.Errorf("%s: refresh rpc: %w", funcName, err)}, nil
		}

		if a.sessionEpoch.Load() != epoch {
			// Another session was adopted while this refresh was in flight: its
			// tokens belong to an account this install no longer holds.
			return result{err: fmt.Errorf("%s: %w", funcName, ErrSessionReplaced)}, nil
		}

		if !a.storeTokensForEpoch(epoch, resp.GetAccessToken(), resp.GetRefreshToken(), resp.GetActiveOrgId()) {
			// The adoption landed between the check above and the write: its
			// tokens are the live ones and must not be overwritten.
			return result{err: fmt.Errorf("%s: %w", funcName, ErrSessionReplaced)}, nil
		}

		a.mu.RLock()
		token := a.accessToken
		a.mu.RUnlock()

		return result{token: token}, nil
	})

	if err != nil {
		return "", err
	}

	r := v.(result)
	return r.token, r.err
}

// Called on Unauthenticated: the in-memory expiry can be stale after macOS sleep.
func (a *SyncAuthManager) InvalidateAccessToken() {
	a.mu.Lock()
	a.accessToken = ""
	a.expiresAt = time.Time{}
	a.mu.Unlock()
}

func (a *SyncAuthManager) Logout(ctx context.Context, client *GRPCClient) error {
	const funcName = "SyncAuthManager.Logout"

	a.revokeServerSession(ctx, client)

	a.mu.Lock()
	a.accessToken = ""
	a.expiresAt = time.Time{}
	a.activeOrgID = ""
	a.mu.Unlock()

	// Bumped after the revoke and right before the delete: a store the revoke's own
	// refresh started captured the old epoch and undoes itself if it lands late.
	a.sessionEpoch.Add(1)
	a.deleteRefreshToken()

	cfg, err := a.configRepo.Get(ctx)
	if err != nil {
		return fmt.Errorf("%s: get config: %w", funcName, err)
	}
	if cfg == nil {
		return nil
	}

	cfg.Enabled = false
	if err := a.configRepo.Update(ctx, cfg); err != nil {
		return fmt.Errorf("%s: update config: %w", funcName, err)
	}

	return nil
}

// revokeServerSession frees the device slot this install occupies. Best effort:
// a failed call only leaves the session to expire on its own.
func (a *SyncAuthManager) revokeServerSession(ctx context.Context, client *GRPCClient) {
	if client == nil {
		return
	}

	ctx, cancel := context.WithTimeout(ctx, serverLogoutTimeout)
	defer cancel()

	token, err := a.GetAccessToken(ctx, client)
	if err != nil {
		slog.Warn("sync: skipping server logout, no access token", "err", err)
		return
	}
	if _, err := client.Auth().Logout(ContextWithAuth(ctx, token), authv1.LogoutRequest_builder{}.Build()); err != nil {
		slog.Warn("sync: server logout failed; session expires on its own", "err", err)
	}
}

// activeOrgID is captured because the server's workspace-discovery contract is org-rooted.
func (a *SyncAuthManager) storeTokens(accessToken, refreshToken, activeOrgID string) {
	a.storeTokensInto(accessToken, refreshToken, activeOrgID, true)
}

// storeTokensNoMirror skips the SQLite mirror: AdoptSignIn already wrote the token with its
// single config Update, and a second write would not fit the commit budget.
func (a *SyncAuthManager) storeTokensNoMirror(accessToken, refreshToken, activeOrgID string) {
	a.storeTokensInto(accessToken, refreshToken, activeOrgID, false)
}

// storeTokensForEpoch publishes a refreshed pair only while its session is still
// current; epoch and write share a lock, so an AdoptSignIn in between wins.
func (a *SyncAuthManager) storeTokensForEpoch(epoch uint64, accessToken, refreshToken, activeOrgID string) bool {
	a.mu.Lock()
	if a.sessionEpoch.Load() != epoch {
		a.mu.Unlock()

		return false
	}
	a.setTokensLocked(accessToken, activeOrgID)
	a.mu.Unlock()

	if refreshToken != "" {
		a.storeRefreshToken(refreshToken, true)
	}

	return true
}

func (a *SyncAuthManager) storeTokensInto(accessToken, refreshToken, activeOrgID string, mirror bool) {
	a.mu.Lock()
	a.setTokensLocked(accessToken, activeOrgID)
	a.mu.Unlock()

	if refreshToken != "" {
		a.storeRefreshToken(refreshToken, mirror)
	}
}

// setTokensLocked writes the in-memory half; the caller holds a.mu.
func (a *SyncAuthManager) setTokensLocked(accessToken, activeOrgID string) {
	a.accessToken = accessToken
	// Round(0) strips the monotonic reading: macOS stops the monotonic clock
	// during sleep, so the expiry must be wall-clock only.
	a.expiresAt = time.Now().Add(accessTokenTTL).Round(0)
	if activeOrgID != "" {
		a.activeOrgID = activeOrgID
	}
}

// keyringTimeout bounds keychain access: on macOS the first access can block
// forever on a hidden security-agent prompt, hanging the Login UI thread.
const keyringTimeout = 2 * time.Second

// storeRefreshToken writes the token to the OS keychain, with sync_config as the fallback.
func (a *SyncAuthManager) storeRefreshToken(token string, mirror bool) {
	epoch := a.sessionEpoch.Load()

	a.mu.RLock()
	account := keyringKeyFor(a.clientID)
	a.mu.RUnlock()

	done := make(chan struct{})
	go func() {
		defer close(done)
		if err := a.keyringSet(keyringService, account, token); err != nil {
			return
		}
		// A write that outlived its session must undo itself.
		if a.sessionEpoch.Load() != epoch {
			a.undoLateWrite(account, token)
		}
	}()
	select {
	case <-done:
	case <-time.After(keyringTimeout):
	}

	if a.sessionEpoch.Load() != epoch {
		// Only the mirror is skipped here: the undo belongs to the write goroutine,
		// whose keychain calls are as unbounded as the one we gave up waiting for.
		return
	}
	// The mirror keeps the SQLite fallback current for a later load that hits the keychain
	// timeout; AdoptSignIn skips it because its config write carried the same token.
	if mirror {
		a.storeRefreshTokenDB(token)
	}
}

// undoLateWrite takes back a write that outlived its session, and only that write:
// the session that replaced ours already mirrored its own token into sync_config.
func (a *SyncAuthManager) undoLateWrite(account, token string) {
	if current, err := keyring.Get(keyringService, account); err != nil || current != token {
		return
	}
	// The session that replaced ours mirrors its own token into sync_config;
	// anything equal to ours is our own stale mirror and must not come back.
	if live := a.loadRefreshTokenDB(); live != "" && live != token {
		if err := a.keyringSet(keyringService, account, live); err == nil {
			return
		}
	}
	_ = keyring.Delete(keyringService, account)
}

// loadRefreshToken falls back to sync_config when the keychain is unavailable or slow.
func (a *SyncAuthManager) loadRefreshToken() string {
	a.mu.RLock()
	account := keyringKeyFor(a.clientID)
	a.mu.RUnlock()

	type kr struct {
		token string
		err   error
	}
	done := make(chan kr, 1)
	go func() {
		t, e := keyring.Get(keyringService, account)
		if e != nil || t == "" {
			// Rebrand migration: a login made before the Tetiva rename
			// lives under the legacy service. Move it over once.
			if lt, le := keyring.Get(keyringLegacyService, account); le == nil && lt != "" {
				if keyring.Set(keyringService, account, lt) == nil {
					_ = keyring.Delete(keyringLegacyService, account)
				}
				done <- kr{token: lt, err: nil}
				return
			}
		}
		done <- kr{token: t, err: e}
	}()
	select {
	case r := <-done:
		if r.err == nil && r.token != "" {
			return r.token
		}
	case <-time.After(keyringTimeout):
		// fall through to SQLite
	}
	return a.loadRefreshTokenDB()
}

// deleteRefreshToken clears keychain and SQLite, the legacy unscoped entry included.
func (a *SyncAuthManager) deleteRefreshToken() {
	a.mu.RLock()
	account := keyringKeyFor(a.clientID)
	a.mu.RUnlock()

	_ = keyring.Delete(keyringService, account)
	_ = keyring.Delete(keyringLegacyService, account)
	if account != keyringLegacyKey {
		_ = keyring.Delete(keyringService, keyringLegacyKey)
		_ = keyring.Delete(keyringLegacyService, keyringLegacyKey)
	}
	a.storeRefreshTokenDB("")
}

func (a *SyncAuthManager) storeRefreshTokenDB(token string) {
	ctx := context.Background()
	cfg, err := a.configRepo.Get(ctx)
	if err != nil || cfg == nil {
		return
	}
	cfg.RefreshToken = token
	if err := a.configRepo.Update(ctx, cfg); err != nil {
		slog.Warn("sync: failed to persist refresh token to DB; user may be logged out on next restart", "err", err)
	}
}

func (a *SyncAuthManager) loadRefreshTokenDB() string {
	ctx := context.Background()
	cfg, err := a.configRepo.Get(ctx)
	if err != nil || cfg == nil {
		return ""
	}
	return cfg.RefreshToken
}
