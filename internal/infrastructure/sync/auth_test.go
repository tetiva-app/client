package sync

import (
	"context"
	"database/sql"
	"sync"
	"testing"
	"time"

	"github.com/zalando/go-keyring"
	_ "modernc.org/sqlite"

	"github.com/tetiva-app/client/internal/infrastructure/repository/sqlite"
)

func newTestSyncConfigRepo(db *sql.DB) sqlite.SyncConfigRepository {
	return sqlite.NewSyncConfigRepo(db)
}

func TestNewSyncAuthManager_InitialState(t *testing.T) {
	db := setupTestDB(t)
	configRepo := newTestSyncConfigRepo(db)

	mgr := NewSyncAuthManager(configRepo)

	if mgr == nil {
		t.Fatal("expected non-nil SyncAuthManager")
	}
	if mgr.IsLoggedIn() {
		t.Error("expected IsLoggedIn() == false on fresh manager")
	}
	if mgr.accessToken != "" {
		t.Error("expected empty access token on fresh manager")
	}
	if !mgr.expiresAt.IsZero() {
		t.Error("expected zero expiresAt on fresh manager")
	}
}

func TestSetClientID(t *testing.T) {
	db := setupTestDB(t)
	configRepo := newTestSyncConfigRepo(db)

	mgr := NewSyncAuthManager(configRepo)
	mgr.SetClientID("test-client-id")

	if mgr.clientID != "test-client-id" {
		t.Errorf("expected clientID == 'test-client-id', got %q", mgr.clientID)
	}
}

func TestIsLoggedIn_AfterTokenSet(t *testing.T) {
	db := setupTestDB(t)
	configRepo := newTestSyncConfigRepo(db)

	mgr := NewSyncAuthManager(configRepo)

	if mgr.IsLoggedIn() {
		t.Error("expected IsLoggedIn() == false before any token is set")
	}

	mgr.storeTokens("test-access-token", "", "")

	if !mgr.IsLoggedIn() {
		t.Error("expected IsLoggedIn() == true after token is stored")
	}
}

func TestIsLoggedIn_AfterLogout(t *testing.T) {
	db := setupTestDB(t)
	configRepo := newTestSyncConfigRepo(db)

	mgr := NewSyncAuthManager(configRepo)
	mgr.storeTokens("some-token", "", "")

	if !mgr.IsLoggedIn() {
		t.Fatal("precondition: IsLoggedIn() should be true")
	}

	// Must create config first so Logout can update it.
	_, err := configRepo.GetOrCreate(context.Background())
	if err != nil {
		t.Fatalf("GetOrCreate: %v", err)
	}

	if err := mgr.Logout(context.Background()); err != nil {
		t.Fatalf("Logout: %v", err)
	}

	if mgr.IsLoggedIn() {
		t.Error("expected IsLoggedIn() == false after Logout")
	}
}

func TestGetAccessToken_ValidToken(t *testing.T) {
	db := setupTestDB(t)
	configRepo := newTestSyncConfigRepo(db)

	mgr := NewSyncAuthManager(configRepo)

	mgr.mu.Lock()
	mgr.accessToken = "valid-access-token"
	mgr.expiresAt = time.Now().Add(10 * time.Minute) // well within leeway
	mgr.mu.Unlock()

	token, err := mgr.GetAccessToken(context.Background(), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if token != "valid-access-token" {
		t.Errorf("expected 'valid-access-token', got %q", token)
	}
}

func TestGetAccessToken_NoRefreshToken(t *testing.T) {
	db := setupTestDB(t)
	configRepo := newTestSyncConfigRepo(db)

	mgr := NewSyncAuthManager(configRepo)
	// No tokens set — refresh path will be triggered, but no refresh token exists.

	_, err := mgr.GetAccessToken(context.Background(), nil)
	if err == nil {
		t.Error("expected error when no refresh token is available")
	}
}

func TestStoreTokens_UpdatesExpiry(t *testing.T) {
	db := setupTestDB(t)
	configRepo := newTestSyncConfigRepo(db)

	mgr := NewSyncAuthManager(configRepo)
	before := time.Now()
	mgr.storeTokens("tok", "", "")
	after := time.Now()

	mgr.mu.RLock()
	expiry := mgr.expiresAt
	mgr.mu.RUnlock()

	minExpected := before.Add(accessTokenTTL)
	maxExpected := after.Add(accessTokenTTL)

	if expiry.Before(minExpected) || expiry.After(maxExpected) {
		t.Errorf("expiresAt %v is outside expected range [%v, %v]", expiry, minExpected, maxExpected)
	}
}

func TestGetAccessToken_Singleflight_NoConcurrentRefresh(t *testing.T) {
	db := setupTestDB(t)
	configRepo := newTestSyncConfigRepo(db)

	mgr := NewSyncAuthManager(configRepo)

	mgr.mu.Lock()
	mgr.accessToken = "concurrent-token"
	mgr.expiresAt = time.Now().Add(10 * time.Minute)
	mgr.mu.Unlock()

	const goroutines = 50
	errors := make([]error, goroutines)
	tokens := make([]string, goroutines)

	var wg sync.WaitGroup
	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		i := i
		go func() {
			defer wg.Done()
			tok, err := mgr.GetAccessToken(context.Background(), nil)
			tokens[i] = tok
			errors[i] = err
		}()
	}
	wg.Wait()

	for i, err := range errors {
		if err != nil {
			t.Errorf("goroutine %d got unexpected error: %v", i, err)
		}
		if tokens[i] != "concurrent-token" {
			t.Errorf("goroutine %d got unexpected token: %q", i, tokens[i])
		}
	}
}

// Regression: unscoped keyring entries let two installs under the same email
// overwrite each other's refresh token. keyring.MockInit avoids the real keychain.
func TestKeyringScoping_DifferentClientIDsHaveIsolatedEntries(t *testing.T) {
	keyring.MockInit()

	ctx := context.Background()

	dbA := setupTestDB(t)
	repoA := newTestSyncConfigRepo(dbA)
	if _, err := repoA.GetOrCreate(ctx); err != nil {
		t.Fatalf("repoA.GetOrCreate: %v", err)
	}
	mgrA := NewSyncAuthManager(repoA)
	mgrA.SetClientID("client-aaa-uuid")
	mgrA.storeTokens("access-a", "refresh-a", "org-1")

	dbB := setupTestDB(t)
	repoB := newTestSyncConfigRepo(dbB)
	if _, err := repoB.GetOrCreate(ctx); err != nil {
		t.Fatalf("repoB.GetOrCreate: %v", err)
	}
	mgrB := NewSyncAuthManager(repoB)
	mgrB.SetClientID("client-bbb-uuid")
	mgrB.storeTokens("access-b", "refresh-b", "org-1")

	// With the legacy unscoped key, mgrA would read "refresh-b" (last write wins).
	gotA := mgrA.loadRefreshToken()
	gotB := mgrB.loadRefreshToken()

	if gotA != "refresh-a" {
		t.Errorf("mgrA.loadRefreshToken() = %q, want %q (other client overwrote keyring entry)", gotA, "refresh-a")
	}
	if gotB != "refresh-b" {
		t.Errorf("mgrB.loadRefreshToken() = %q, want %q", gotB, "refresh-b")
	}
}

func TestLogout_ClearsConfigEnabled(t *testing.T) {
	db := setupTestDB(t)
	configRepo := newTestSyncConfigRepo(db)

	ctx := context.Background()
	mgr := NewSyncAuthManager(configRepo)
	mgr.storeTokens("some-token", "", "")

	cfg, err := configRepo.GetOrCreate(ctx)
	if err != nil {
		t.Fatalf("GetOrCreate: %v", err)
	}
	cfg.Enabled = true
	if err := configRepo.Update(ctx, cfg); err != nil {
		t.Fatalf("Update: %v", err)
	}

	if err := mgr.Logout(ctx); err != nil {
		t.Fatalf("Logout: %v", err)
	}

	got, err := configRepo.Get(ctx)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Enabled {
		t.Error("expected config.Enabled == false after Logout")
	}
}
