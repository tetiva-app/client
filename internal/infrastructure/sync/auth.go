package sync

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/zalando/go-keyring"
	"golang.org/x/sync/singleflight"

	authv1 "github.com/tetiva-app/proto/go/gophercourier/auth/v1"
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
)

// ErrAuthExpired means the refresh token was rejected: only an interactive
// re-login can restore sync. Sync engines stop retrying on it.
var ErrAuthExpired = errors.New("sync auth expired: refresh token rejected")

// keyringKeyFor falls back to the legacy unscoped key when clientID is
// empty (early boot or tests that never SetClientID).
func keyringKeyFor(clientID string) string {
	if clientID == "" {
		return keyringLegacyKey
	}
	return keyringKeyPrefix + clientID
}

// SyncAuthManager manages authentication with the sync server.
// Access token is kept in memory; refresh token in OS keychain (fallback: sync_config table).
type SyncAuthManager struct {
	mu          sync.RWMutex
	accessToken string
	expiresAt   time.Time
	clientID    string
	activeOrgID string
	configRepo  sqlite.SyncConfigRepository
	sf          singleflight.Group
}

// NewSyncAuthManager creates a new SyncAuthManager.
func NewSyncAuthManager(configRepo sqlite.SyncConfigRepository) *SyncAuthManager {
	return &SyncAuthManager{
		configRepo: configRepo,
	}
}

// SetClientID sets the client ID (called during initialization from config).
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

// GetActiveOrgID returns the active org ID captured at last Login/Register/Refresh.
// Empty string means no successful auth has happened in this session.
func (a *SyncAuthManager) GetActiveOrgID() string {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.activeOrgID
}

// Login authenticates with the sync server and stores tokens.
func (a *SyncAuthManager) Login(ctx context.Context, client *GRPCClient, email, password string) error {
	const funcName = "SyncAuthManager.Login"

	cfg, err := a.configRepo.GetOrCreate(ctx)
	if err != nil {
		return fmt.Errorf("%s: get config: %w", funcName, err)
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
		return fmt.Errorf("%s: login rpc: %w", funcName, err)
	}

	slog.Info("sync: login rpc ok", "email", email, "active_org_id", resp.GetActiveOrgId())

	a.storeTokens(resp.GetAccessToken(), resp.GetRefreshToken(), resp.GetActiveOrgId())

	cfg.UserEmail = email
	cfg.Enabled = true
	if err := a.configRepo.Update(ctx, cfg); err != nil {
		return fmt.Errorf("%s: update config: %w", funcName, err)
	}

	return nil
}

// Register creates a new account on the sync server and stores tokens.
func (a *SyncAuthManager) Register(ctx context.Context, client *GRPCClient, email, password, name, locale string) error {
	const funcName = "SyncAuthManager.Register"

	cfg, err := a.configRepo.GetOrCreate(ctx)
	if err != nil {
		return fmt.Errorf("%s: get config: %w", funcName, err)
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
		return fmt.Errorf("%s: register rpc: %w", funcName, err)
	}

	a.storeTokens(resp.GetAccessToken(), resp.GetRefreshToken(), resp.GetActiveOrgId())

	cfg.UserEmail = email
	cfg.Enabled = true
	if err := a.configRepo.Update(ctx, cfg); err != nil {
		return fmt.Errorf("%s: update config: %w", funcName, err)
	}

	return nil
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

		a.storeTokens(resp.GetAccessToken(), resp.GetRefreshToken(), resp.GetActiveOrgId())

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

// InvalidateAccessToken forces the next GetAccessToken through the refresh path.
// Called on Unauthenticated: the in-memory expiry can be stale after macOS sleep.
func (a *SyncAuthManager) InvalidateAccessToken() {
	a.mu.Lock()
	a.accessToken = ""
	a.expiresAt = time.Time{}
	a.mu.Unlock()
}

// Logout clears authentication state and disables sync.
func (a *SyncAuthManager) Logout(ctx context.Context) error {
	const funcName = "SyncAuthManager.Logout"

	a.mu.Lock()
	a.accessToken = ""
	a.expiresAt = time.Time{}
	a.activeOrgID = ""
	a.mu.Unlock()

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

// storeTokens saves the access token in memory and the refresh token in the keychain.
// activeOrgID is captured because the server's workspace-discovery contract is org-rooted.
func (a *SyncAuthManager) storeTokens(accessToken, refreshToken, activeOrgID string) {
	a.mu.Lock()
	a.accessToken = accessToken
	// Round(0) strips the monotonic reading: macOS stops the monotonic clock
	// during sleep, so the expiry must be wall-clock only.
	a.expiresAt = time.Now().Add(accessTokenTTL).Round(0)
	if activeOrgID != "" {
		a.activeOrgID = activeOrgID
	}
	a.mu.Unlock()

	if refreshToken != "" {
		a.storeRefreshToken(refreshToken)
	}
}

// keyringTimeout bounds keychain access: on macOS the first access can block
// forever on a hidden security-agent prompt, hanging the Login UI thread.
const keyringTimeout = 2 * time.Second

// storeRefreshToken persists the refresh token in the OS keychain, falling back
// to sync_config. Entries are scoped by client_id to avoid collisions between installs.
func (a *SyncAuthManager) storeRefreshToken(token string) {
	a.mu.RLock()
	account := keyringKeyFor(a.clientID)
	a.mu.RUnlock()

	done := make(chan error, 1)
	go func() { done <- keyring.Set(keyringService, account, token) }()
	select {
	case <-done:
	case <-time.After(keyringTimeout):
	}
	// Always mirror into SQLite: if a later load hits the keychain timeout,
	// the fallback must hold the current token, not a long-rotated one.
	a.storeRefreshTokenDB(token)
}

// loadRefreshToken reads the refresh token from the OS keychain, falling back
// to sync_config if the keychain is unavailable or slow.
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

// deleteRefreshToken removes the refresh token from both keychain and SQLite,
// including the legacy unscoped entry; either may be absent.
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
