package sync

import (
	"context"
	"database/sql"
	"errors"
	gosync "sync"
	"sync/atomic"
	"testing"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/zalando/go-keyring"
	_ "modernc.org/sqlite"

	authv1 "github.com/tetiva-app/proto/go/gophercourier/auth/v1"

	"github.com/tetiva-app/client/internal/domain/usecase/auth"
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

	if err := mgr.Logout(context.Background(), nil); err != nil {
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

	var wg gosync.WaitGroup
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

	if err := mgr.Logout(ctx, nil); err != nil {
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

// Regression: Register read cfg before storeTokens, so its Update wrote the
// pre-login refresh token back over the rotated one.
func TestRegister_KeepsRotatedRefreshTokenInDB(t *testing.T) {
	keyring.MockInit()
	db := setupTestDB(t)
	repo := newTestSyncConfigRepo(db)
	ctx := context.Background()

	stub := &stubAuthClient{registerResp: authv1.RegisterResponse_builder{
		AccessToken:  "access-1",
		RefreshToken: "refresh-1",
		ActiveOrgId:  "org-1",
	}.Build()}

	mgr := NewSyncAuthManager(repo)
	if _, err := mgr.Register(ctx, NewGRPCClientWithStubs(stub, nil), "a@b.c", "pw", "A", "ru"); err != nil {
		t.Fatalf("Register: %v", err)
	}

	cfg, err := repo.Get(ctx)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if cfg.RefreshToken != "refresh-1" {
		t.Errorf("cfg.RefreshToken = %q, want %q", cfg.RefreshToken, "refresh-1")
	}
}

func TestRegister_ReportsRequiresEmailVerification(t *testing.T) {
	keyring.MockInit()
	db := setupTestDB(t)
	repo := newTestSyncConfigRepo(db)
	ctx := context.Background()

	stub := &stubAuthClient{registerResp: authv1.RegisterResponse_builder{
		AccessToken:               "access-1",
		RefreshToken:              "refresh-1",
		ActiveOrgId:               "org-1",
		RequiresEmailVerification: true,
	}.Build()}

	mgr := NewSyncAuthManager(repo)
	res, err := mgr.Register(ctx, NewGRPCClientWithStubs(stub, nil), "a@b.c", "pw", "A", "ru")
	if err != nil {
		t.Fatalf("Register: %v", err)
	}
	if !res.RequiresEmailVerification {
		t.Error("expected RequiresEmailVerification == true")
	}
	if res.Email != "a@b.c" {
		t.Errorf("res.Email = %q, want %q", res.Email, "a@b.c")
	}
}

func TestLogin_ReportsRequiresEmailVerification(t *testing.T) {
	keyring.MockInit()
	db := setupTestDB(t)
	repo := newTestSyncConfigRepo(db)
	ctx := context.Background()

	stub := &stubAuthClient{loginResp: authv1.LoginResponse_builder{
		AccessToken:               "access-1",
		RefreshToken:              "refresh-1",
		ActiveOrgId:               "org-1",
		RequiresEmailVerification: true,
	}.Build()}

	mgr := NewSyncAuthManager(repo)
	res, err := mgr.Login(ctx, NewGRPCClientWithStubs(stub, nil), "a@b.c", "pw")
	if err != nil {
		t.Fatalf("Login: %v", err)
	}
	if !res.RequiresEmailVerification {
		t.Error("expected RequiresEmailVerification == true")
	}

	cfg, err := repo.Get(ctx)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if cfg.RefreshToken != "refresh-1" {
		t.Errorf("cfg.RefreshToken = %q, want %q", cfg.RefreshToken, "refresh-1")
	}
}

func TestGetMe_ReturnsEmailVerifiedAndSendsBearer(t *testing.T) {
	db := setupTestDB(t)
	repo := newTestSyncConfigRepo(db)

	stub := &stubAuthClient{getMeResp: authv1.GetMeResponse_builder{
		User: authv1.User_builder{Email: "a@b.c", EmailVerified: true}.Build(),
	}.Build()}

	mgr := NewSyncAuthManager(repo)
	mgr.storeTokens("access-1", "", "org-1")

	email, verified, err := mgr.GetMe(context.Background(), NewGRPCClientWithStubs(stub, nil))
	if err != nil {
		t.Fatalf("GetMe: %v", err)
	}
	if email != "a@b.c" || !verified {
		t.Errorf("GetMe() = (%q, %v), want (%q, true)", email, verified, "a@b.c")
	}
	if len(stub.authHeaders) != 1 || stub.authHeaders[0] != "Bearer access-1" {
		t.Errorf("authHeaders = %v, want [\"Bearer access-1\"]", stub.authHeaders)
	}
}

func TestGetMe_UnverifiedAccount(t *testing.T) {
	db := setupTestDB(t)
	repo := newTestSyncConfigRepo(db)

	stub := &stubAuthClient{getMeResp: authv1.GetMeResponse_builder{
		User: authv1.User_builder{Email: "a@b.c", EmailVerified: false}.Build(),
	}.Build()}

	mgr := NewSyncAuthManager(repo)
	mgr.storeTokens("access-1", "", "org-1")

	_, verified, err := mgr.GetMe(context.Background(), NewGRPCClientWithStubs(stub, nil))
	if err != nil {
		t.Fatalf("GetMe: %v", err)
	}
	if verified {
		t.Error("expected verified == false")
	}
}

func TestResendVerification_RateLimitedStatusSurvivesWrapping(t *testing.T) {
	db := setupTestDB(t)
	repo := newTestSyncConfigRepo(db)

	stub := &stubAuthClient{resendErr: status.Error(codes.ResourceExhausted, "too many requests")}

	mgr := NewSyncAuthManager(repo)
	mgr.storeTokens("access-1", "", "org-1")

	err := mgr.ResendVerification(context.Background(), NewGRPCClientWithStubs(stub, nil))
	if err == nil {
		t.Fatal("expected an error")
	}
	if got := status.Code(err); got != codes.ResourceExhausted {
		t.Errorf("status.Code(err) = %v, want %v", got, codes.ResourceExhausted)
	}
}

func TestResendVerification_SendsBearer(t *testing.T) {
	db := setupTestDB(t)
	repo := newTestSyncConfigRepo(db)

	stub := &stubAuthClient{}

	mgr := NewSyncAuthManager(repo)
	mgr.storeTokens("access-1", "", "org-1")

	if err := mgr.ResendVerification(context.Background(), NewGRPCClientWithStubs(stub, nil)); err != nil {
		t.Fatalf("ResendVerification: %v", err)
	}
	if stub.resendCalls != 1 {
		t.Errorf("resendCalls = %d, want 1", stub.resendCalls)
	}
	if len(stub.authHeaders) != 1 || stub.authHeaders[0] != "Bearer access-1" {
		t.Errorf("authHeaders = %v, want [\"Bearer access-1\"]", stub.authHeaders)
	}
}

func TestLogout_RevokesServerSessionAndClearsLocalStateOnError(t *testing.T) {
	keyring.MockInit()
	db := setupTestDB(t)
	repo := newTestSyncConfigRepo(db)
	ctx := context.Background()

	stub := &stubAuthClient{logoutErr: status.Error(codes.Unavailable, "server down")}
	mgr := NewSyncAuthManager(repo)
	mgr.storeTokens("access-1", "refresh-1", "org-1")

	cfg, err := repo.GetOrCreate(ctx)
	if err != nil {
		t.Fatalf("GetOrCreate: %v", err)
	}
	cfg.Enabled = true
	if err := repo.Update(ctx, cfg); err != nil {
		t.Fatalf("Update: %v", err)
	}

	if err := mgr.Logout(ctx, NewGRPCClientWithStubs(stub, nil)); err != nil {
		t.Fatalf("Logout: %v", err)
	}

	if stub.logoutCalls != 1 {
		t.Errorf("logoutCalls = %d, want 1", stub.logoutCalls)
	}
	if len(stub.authHeaders) != 1 || stub.authHeaders[0] != "Bearer access-1" {
		t.Errorf("authHeaders = %v, want [\"Bearer access-1\"]", stub.authHeaders)
	}
	if mgr.IsLoggedIn() {
		t.Error("expected IsLoggedIn() == false after Logout")
	}
	got, err := repo.Get(ctx)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Enabled {
		t.Error("expected config.Enabled == false after Logout")
	}
	if got.RefreshToken != "" {
		t.Errorf("cfg.RefreshToken = %q, want empty", got.RefreshToken)
	}
}

func TestMe_MapsSessionsToDevices(t *testing.T) {
	db := setupTestDB(t)
	repo := newTestSyncConfigRepo(db)
	lastUsed := time.Date(2026, 8, 15, 10, 0, 0, 0, time.UTC)

	stub := &stubAuthClient{meResp: authv1.MeResponse_builder{
		Sessions: []*authv1.SessionView{
			authv1.SessionView_builder{
				Id: "sess_1", ClientId: "client-1", UserAgent: "Tetiva/0.17.0 (darwin; mac) grpc-go/1.79.3",
				Ip: "1.2.3.4", LastUsedAt: timestamppb.New(lastUsed), IsCurrent: true,
			}.Build(),
			authv1.SessionView_builder{Id: "sess_2", ClientId: "client-2"}.Build(),
		},
	}.Build()}

	mgr := NewSyncAuthManager(repo)
	mgr.storeTokens("access-1", "", "org-1")

	sessions, err := mgr.Me(context.Background(), NewGRPCClientWithStubs(stub, nil))
	if err != nil {
		t.Fatalf("Me: %v", err)
	}
	if len(sessions) != 2 {
		t.Fatalf("len(sessions) = %d, want 2", len(sessions))
	}
	want := SessionInfo{
		ID: "sess_1", ClientID: "client-1", UserAgent: "Tetiva/0.17.0 (darwin; mac) grpc-go/1.79.3",
		IP: "1.2.3.4", LastUsedAt: lastUsed, IsCurrent: true,
	}
	if sessions[0] != want {
		t.Errorf("sessions[0] = %+v, want %+v", sessions[0], want)
	}
	if !sessions[1].LastUsedAt.IsZero() {
		t.Errorf("sessions[1].LastUsedAt = %v, want zero", sessions[1].LastUsedAt)
	}
	if len(stub.authHeaders) != 1 || stub.authHeaders[0] != "Bearer access-1" {
		t.Errorf("authHeaders = %v, want [\"Bearer access-1\"]", stub.authHeaders)
	}
}

func TestRevokeSession_SendsSessionID(t *testing.T) {
	db := setupTestDB(t)
	repo := newTestSyncConfigRepo(db)

	stub := &stubAuthClient{}
	mgr := NewSyncAuthManager(repo)
	mgr.storeTokens("access-1", "", "org-1")

	if err := mgr.RevokeSession(context.Background(), NewGRPCClientWithStubs(stub, nil), "sess_2"); err != nil {
		t.Fatalf("RevokeSession: %v", err)
	}
	if len(stub.revokedIDs) != 1 || stub.revokedIDs[0] != "sess_2" {
		t.Errorf("revokedIDs = %v, want [sess_2]", stub.revokedIDs)
	}
}

func TestLogoutAll_ReturnsRevokedCount(t *testing.T) {
	db := setupTestDB(t)
	repo := newTestSyncConfigRepo(db)

	stub := &stubAuthClient{logoutAllN: 3}
	mgr := NewSyncAuthManager(repo)
	mgr.storeTokens("access-1", "", "org-1")

	n, err := mgr.LogoutAll(context.Background(), NewGRPCClientWithStubs(stub, nil))
	if err != nil {
		t.Fatalf("LogoutAll: %v", err)
	}
	if n != 3 {
		t.Errorf("LogoutAll() = %d, want 3", n)
	}
	if stub.logoutAllRun != 1 {
		t.Errorf("logoutAllRun = %d, want 1", stub.logoutAllRun)
	}
	if mgr.IsLoggedIn() != true {
		t.Error("LogoutAll must keep this device signed in")
	}
}

// Regression: a keychain write that outlives Logout resurrected the refresh token
// the user had just signed out of.
func TestStoreRefreshToken_LateWriteLosesToLogout(t *testing.T) {
	keyring.MockInit()
	db := setupTestDB(t)
	repo := newTestSyncConfigRepo(db)
	ctx := context.Background()
	if _, err := repo.GetOrCreate(ctx); err != nil {
		t.Fatalf("GetOrCreate: %v", err)
	}

	mgr := NewSyncAuthManager(repo)
	mgr.SetClientID("client-1")

	started := make(chan struct{})
	release := make(chan struct{})
	mgr.keyringSet = func(service, account, token string) error {
		close(started)
		<-release
		return keyring.Set(service, account, token)
	}

	done := make(chan struct{})
	go func() {
		defer close(done)
		mgr.storeRefreshToken("late-token", true)
	}()
	<-started

	if err := mgr.Logout(ctx, nil); err != nil {
		t.Fatalf("Logout: %v", err)
	}
	close(release)
	<-done

	if got := mgr.loadRefreshTokenDB(); got != "" {
		t.Errorf("sync_config.refresh_token = %q, want empty", got)
	}
	if got, err := keyring.Get(keyringService, keyringKeyFor("client-1")); err == nil {
		t.Errorf("keychain still holds %q after Logout", got)
	}
}

// Regression: the epoch was bumped before the server revoke, so the refresh that revoke
// triggers stored its rotated token under the new epoch and survived the delete after it.
func TestLogout_BumpsEpochAfterServerRevoke(t *testing.T) {
	keyring.MockInit()
	db := setupTestDB(t)
	repo := newTestSyncConfigRepo(db)
	ctx := context.Background()

	cfg, err := repo.GetOrCreate(ctx)
	if err != nil {
		t.Fatalf("GetOrCreate: %v", err)
	}
	cfg.Enabled = true
	cfg.RefreshToken = "refresh-1"
	if err := repo.Update(ctx, cfg); err != nil {
		t.Fatalf("Update: %v", err)
	}

	stub := &stubAuthClient{refreshResp: authv1.RefreshResponse_builder{
		AccessToken:  "access-2",
		RefreshToken: "refresh-2",
		ActiveOrgId:  "org-1",
	}.Build()}

	mgr := NewSyncAuthManager(repo)
	mgr.SetClientID("client-1")

	var epochAtStore atomic.Uint64
	epochAtStore.Store(^uint64(0))
	mgr.keyringSet = func(service, account, token string) error {
		epochAtStore.Store(mgr.sessionEpoch.Load())
		return keyring.Set(service, account, token)
	}

	if err := mgr.Logout(ctx, NewGRPCClientWithStubs(stub, nil)); err != nil {
		t.Fatalf("Logout: %v", err)
	}

	if stub.logoutCalls != 1 {
		t.Fatalf("logoutCalls = %d, want 1", stub.logoutCalls)
	}
	if got := epochAtStore.Load(); got != 0 {
		t.Errorf("epoch seen by the revoke's own store = %d, want 0", got)
	}
	if got := mgr.sessionEpoch.Load(); got != 1 {
		t.Errorf("sessionEpoch after Logout = %d, want 1", got)
	}
	if got := mgr.loadRefreshTokenDB(); got != "" {
		t.Errorf("sync_config.refresh_token = %q, want empty", got)
	}
	if got, err := keyring.Get(keyringService, keyringKeyFor("client-1")); err == nil {
		t.Errorf("keychain still holds %q after Logout", got)
	}
}

// countingConfigRepo watches the writes AdoptSignIn makes: the commit budget
// allows exactly one, and a failing or slow one must not reach the keychain.
type countingConfigRepo struct {
	sqlite.SyncConfigRepository
	mu        gosync.Mutex
	updates   int
	updateErr error
	delay     time.Duration
}

func (r *countingConfigRepo) Update(ctx context.Context, cfg *sqlite.SyncConfig) error {
	r.mu.Lock()
	r.updates++
	delay, updateErr := r.delay, r.updateErr
	r.mu.Unlock()

	if delay > 0 {
		select {
		case <-time.After(delay):
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	if updateErr != nil {
		return updateErr
	}
	return r.SyncConfigRepository.Update(ctx, cfg)
}

func (r *countingConfigRepo) updateCount() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.updates
}

func adoptTokens() auth.SignInTokens {
	return auth.SignInTokens{
		AccessToken:  "access-new",
		RefreshToken: "refresh-new",
		ActiveOrgID:  "org-new",
		Email:        "new@example.com",
	}
}

func TestAdoptSignIn_WritesConfigOnce(t *testing.T) {
	keyring.MockInit()
	db := setupTestDB(t)
	repo := &countingConfigRepo{SyncConfigRepository: newTestSyncConfigRepo(db)}
	ctx := context.Background()

	cfg, err := repo.GetOrCreate(ctx)
	if err != nil {
		t.Fatalf("GetOrCreate: %v", err)
	}
	cfg.ReauthRequired = true
	if err := repo.Update(ctx, cfg); err != nil {
		t.Fatalf("Update: %v", err)
	}
	before := repo.updateCount()

	mgr := NewSyncAuthManager(repo)
	res, err := mgr.AdoptSignIn(ctx, "sync.example.com:443", adoptTokens())
	if err != nil {
		t.Fatalf("AdoptSignIn: %v", err)
	}

	if res.Email != "new@example.com" {
		t.Errorf("res.Email = %q, want %q", res.Email, "new@example.com")
	}
	if got := repo.updateCount() - before; got != 1 {
		t.Errorf("config writes during AdoptSignIn = %d, want 1", got)
	}

	stored, err := repo.Get(ctx)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if stored.ServerURL != "sync.example.com:443" {
		t.Errorf("ServerURL = %q", stored.ServerURL)
	}
	if stored.UserEmail != "new@example.com" {
		t.Errorf("UserEmail = %q", stored.UserEmail)
	}
	if !stored.Enabled {
		t.Error("Enabled should be true after AdoptSignIn")
	}
	if stored.RefreshToken != "refresh-new" {
		t.Errorf("RefreshToken = %q, want %q", stored.RefreshToken, "refresh-new")
	}
	if stored.AuthGeneration != CurrentAuthGeneration {
		t.Errorf("AuthGeneration = %d, want %d", stored.AuthGeneration, CurrentAuthGeneration)
	}
	if stored.ReauthRequired {
		t.Error("AdoptSignIn must clear reauth_required")
	}

	if got, err := keyring.Get(keyringService, keyringKeyFor(cfg.ClientID)); err != nil || got != "refresh-new" {
		t.Errorf("keychain holds %q (err %v), want %q", got, err, "refresh-new")
	}
	if !mgr.IsLoggedIn() {
		t.Error("expected IsLoggedIn() == true after AdoptSignIn")
	}
	if mgr.GetActiveOrgID() != "org-new" {
		t.Errorf("GetActiveOrgID() = %q, want %q", mgr.GetActiveOrgID(), "org-new")
	}
}

func TestAdoptSignIn_ConfigWriteFails_LeavesNothingBehind(t *testing.T) {
	keyring.MockInit()
	db := setupTestDB(t)
	repo := &countingConfigRepo{SyncConfigRepository: newTestSyncConfigRepo(db)}
	ctx := context.Background()

	if _, err := repo.GetOrCreate(ctx); err != nil {
		t.Fatalf("GetOrCreate: %v", err)
	}
	repo.updateErr = errors.New("database is locked")

	mgr := NewSyncAuthManager(repo)
	if _, err := mgr.AdoptSignIn(ctx, "sync.example.com:443", adoptTokens()); err == nil {
		t.Fatal("expected AdoptSignIn to fail when the config write fails")
	}

	if mgr.IsLoggedIn() {
		t.Error("memory must be untouched when the config write fails")
	}
	if got := mgr.loadRefreshToken(); got != "" {
		t.Errorf("refresh token = %q, want empty", got)
	}
}

func TestAdoptSignIn_SlowConfigWriteStaysInsideBudget(t *testing.T) {
	keyring.MockInit()
	db := setupTestDB(t)
	repo := &countingConfigRepo{SyncConfigRepository: newTestSyncConfigRepo(db), delay: 10 * time.Second}
	ctx := context.Background()

	if _, err := repo.GetOrCreate(ctx); err != nil {
		t.Fatalf("GetOrCreate: %v", err)
	}

	mgr := NewSyncAuthManager(repo)
	start := time.Now()
	_, err := mgr.AdoptSignIn(ctx, "sync.example.com:443", adoptTokens())
	elapsed := time.Since(start)

	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("AdoptSignIn error = %v, want context.DeadlineExceeded", err)
	}
	if elapsed > 3*time.Second {
		t.Errorf("AdoptSignIn took %s, must stay inside the commit budget", elapsed)
	}
	if mgr.IsLoggedIn() {
		t.Error("memory must be untouched when the config write times out")
	}
}

// A refresh of the previous session that answers after another was adopted must
// throw its tokens away: memory, keychain and sync_config belong to the new one.
func TestAdoptSignIn_DropsRefreshOfReplacedSession(t *testing.T) {
	keyring.MockInit()
	db := setupTestDB(t)
	repo := newTestSyncConfigRepo(db)
	ctx := context.Background()

	cfg, err := repo.GetOrCreate(ctx)
	if err != nil {
		t.Fatalf("GetOrCreate: %v", err)
	}
	cfg.Enabled = true
	cfg.RefreshToken = "refresh-old"
	if err := repo.Update(ctx, cfg); err != nil {
		t.Fatalf("Update: %v", err)
	}

	stub := &stubAuthClient{
		refreshEntered: make(chan struct{}, 1),
		refreshGate:    make(chan struct{}),
		refreshResp: authv1.RefreshResponse_builder{
			AccessToken:  "access-old-2",
			RefreshToken: "refresh-old-2",
			ActiveOrgId:  "org-old",
		}.Build(),
	}

	mgr := NewSyncAuthManager(repo)
	mgr.SetClientID(cfg.ClientID)

	refreshErr := make(chan error, 1)
	go func() {
		_, err := mgr.GetAccessToken(ctx, NewGRPCClientWithStubs(stub, nil))
		refreshErr <- err
	}()
	<-stub.refreshEntered

	if _, err := mgr.AdoptSignIn(ctx, "sync.example.com:443", adoptTokens()); err != nil {
		t.Fatalf("AdoptSignIn: %v", err)
	}

	close(stub.refreshGate)
	if err := <-refreshErr; !errors.Is(err, ErrSessionReplaced) {
		t.Fatalf("GetAccessToken error = %v, want ErrSessionReplaced", err)
	}

	mgr.mu.RLock()
	access, org := mgr.accessToken, mgr.activeOrgID
	mgr.mu.RUnlock()
	if access != "access-new" {
		t.Errorf("in-memory access token = %q, want %q", access, "access-new")
	}
	if org != "org-new" {
		t.Errorf("active org = %q, want %q", org, "org-new")
	}
	if got, err := keyring.Get(keyringService, keyringKeyFor(cfg.ClientID)); err != nil || got != "refresh-new" {
		t.Errorf("keychain holds %q (err %v), want %q", got, err, "refresh-new")
	}
	if got := mgr.loadRefreshTokenDB(); got != "refresh-new" {
		t.Errorf("sync_config.refresh_token = %q, want %q", got, "refresh-new")
	}
}

// The narrowest overlap: the refresh is already past its epoch check and waiting
// for the lock when the adoption lands, so the guard has to sit inside the write.
func TestGetAccessToken_RefreshDropsTheAnswerAdoptedOverAtTheLastMoment(t *testing.T) {
	keyring.MockInit()
	db := setupTestDB(t)
	repo := newTestSyncConfigRepo(db)
	ctx := context.Background()

	cfg, err := repo.GetOrCreate(ctx)
	if err != nil {
		t.Fatalf("GetOrCreate: %v", err)
	}
	cfg.Enabled = true
	cfg.RefreshToken = "refresh-old"
	if err := repo.Update(ctx, cfg); err != nil {
		t.Fatalf("Update: %v", err)
	}

	stub := &stubAuthClient{
		refreshEntered: make(chan struct{}, 1),
		refreshGate:    make(chan struct{}),
		refreshResp: authv1.RefreshResponse_builder{
			AccessToken:  "access-old-2",
			RefreshToken: "refresh-old-2",
			ActiveOrgId:  "org-old",
		}.Build(),
	}

	mgr := NewSyncAuthManager(repo)
	mgr.SetClientID(cfg.ClientID)

	refreshErr := make(chan error, 1)
	go func() {
		_, err := mgr.GetAccessToken(ctx, NewGRPCClientWithStubs(stub, nil))
		refreshErr <- err
	}()
	<-stub.refreshEntered

	// Held so the refresh gets past its epoch check and stops at the write; the
	// adoption is then replayed field by field, exactly as AdoptSignIn does it.
	mgr.mu.Lock()
	close(stub.refreshGate)
	time.Sleep(50 * time.Millisecond)
	mgr.sessionEpoch.Add(1)
	mgr.accessToken = "access-new"
	mgr.activeOrgID = "org-new"
	mgr.mu.Unlock()

	if err := <-refreshErr; !errors.Is(err, ErrSessionReplaced) {
		t.Fatalf("GetAccessToken error = %v, want ErrSessionReplaced", err)
	}

	mgr.mu.RLock()
	access, org := mgr.accessToken, mgr.activeOrgID
	mgr.mu.RUnlock()
	if access != "access-new" {
		t.Errorf("in-memory access token = %q, want %q", access, "access-new")
	}
	if org != "org-new" {
		t.Errorf("active org = %q, want %q", org, "org-new")
	}
	if got, err := keyring.Get(keyringService, keyringKeyFor(cfg.ClientID)); err == nil && got == "refresh-old-2" {
		t.Error("the replaced session's refresh token reached the keychain")
	}
}

// The narrower overlap: the previous session's store is still inside the keychain
// write when the adoption lands, so its self-undo runs afterwards.
func TestStoreRefreshToken_LateWriteKeepsAdoptedSession(t *testing.T) {
	keyring.MockInit()
	db := setupTestDB(t)
	repo := newTestSyncConfigRepo(db)
	ctx := context.Background()

	cfg, err := repo.GetOrCreate(ctx)
	if err != nil {
		t.Fatalf("GetOrCreate: %v", err)
	}

	mgr := NewSyncAuthManager(repo)
	mgr.SetClientID(cfg.ClientID)

	started := make(chan struct{})
	release := make(chan struct{})
	mgr.keyringSet = func(service, account, token string) error {
		if token == "refresh-old" {
			close(started)
			<-release
		}
		return keyring.Set(service, account, token)
	}

	done := make(chan struct{})
	go func() {
		defer close(done)
		mgr.storeRefreshToken("refresh-old", true)
	}()
	<-started

	if _, err := mgr.AdoptSignIn(ctx, "sync.example.com:443", adoptTokens()); err != nil {
		t.Fatalf("AdoptSignIn: %v", err)
	}
	close(release)
	<-done

	if got := mgr.loadRefreshTokenDB(); got != "refresh-new" {
		t.Errorf("sync_config.refresh_token = %q, want %q", got, "refresh-new")
	}
	// Asserted on the keychain itself: reading through loadRefreshToken would pass
	// on the sync_config fallback alone.
	if got, err := keyring.Get(keyringService, keyringKeyFor(cfg.ClientID)); err != nil || got != "refresh-new" {
		t.Errorf("keychain holds %q (err %v), want %q", got, err, "refresh-new")
	}
	if got := mgr.loadRefreshToken(); got != "refresh-new" {
		t.Errorf("refresh token = %q, want %q", got, "refresh-new")
	}
}

func TestLogin_StampsAuthGenerationAndClearsReauth(t *testing.T) {
	keyring.MockInit()
	db := setupTestDB(t)
	repo := newTestSyncConfigRepo(db)
	ctx := context.Background()

	cfg, err := repo.GetOrCreate(ctx)
	if err != nil {
		t.Fatalf("GetOrCreate: %v", err)
	}
	cfg.ReauthRequired = true
	if err := repo.Update(ctx, cfg); err != nil {
		t.Fatalf("Update: %v", err)
	}

	stub := &stubAuthClient{loginResp: authv1.LoginResponse_builder{
		AccessToken:  "access-1",
		RefreshToken: "refresh-1",
		ActiveOrgId:  "org-1",
	}.Build()}

	mgr := NewSyncAuthManager(repo)
	if _, err := mgr.Login(ctx, NewGRPCClientWithStubs(stub, nil), "a@b.c", "pw"); err != nil {
		t.Fatalf("Login: %v", err)
	}

	stored, err := repo.Get(ctx)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if stored.AuthGeneration != CurrentAuthGeneration {
		t.Errorf("AuthGeneration = %d, want %d", stored.AuthGeneration, CurrentAuthGeneration)
	}
	if stored.ReauthRequired {
		t.Error("a successful sign-in must clear reauth_required")
	}

	// The refresh that follows rewrites only the token; the flags must survive it.
	mgr.storeRefreshTokenDB("refresh-2")

	after, err := repo.Get(ctx)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if after.RefreshToken != "refresh-2" {
		t.Errorf("RefreshToken = %q, want %q", after.RefreshToken, "refresh-2")
	}
	if after.AuthGeneration != CurrentAuthGeneration || after.ReauthRequired {
		t.Errorf("flags after a refresh: generation=%d reauth=%v", after.AuthGeneration, after.ReauthRequired)
	}
}
