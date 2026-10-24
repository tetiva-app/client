package sync

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/zalando/go-keyring"
	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	authv1 "github.com/tetiva-app/proto/go/gophercourier/auth/v1"

	"github.com/tetiva-app/client/internal/infrastructure/repository/sqlite"
)

func statusWithReason(t *testing.T, code codes.Code, reason, domain string) error {
	t.Helper()
	st, err := status.New(code, "["+reason+"]").WithDetails(&errdetails.ErrorInfo{Reason: reason, Domain: domain})
	if err != nil {
		t.Fatalf("WithDetails: %v", err)
	}
	return st.Err()
}

func rejectedRefresh(t *testing.T) error {
	t.Helper()
	return statusWithReason(t, codes.Unauthenticated, "REFRESH_TOKEN_REJECTED", "tetiva.app")
}

func seedRefreshToken(t *testing.T, repo sqlite.SyncConfigRepository, token string) *sqlite.SyncConfig {
	t.Helper()
	ctx := context.Background()
	cfg, err := repo.GetOrCreate(ctx)
	if err != nil {
		t.Fatalf("GetOrCreate: %v", err)
	}
	cfg.Enabled = true
	cfg.RefreshToken = token
	cfg.AuthGeneration = CurrentAuthGeneration
	if err := repo.Update(ctx, cfg); err != nil {
		t.Fatalf("Update: %v", err)
	}
	return cfg
}

func reauthRequired(t *testing.T, repo sqlite.SyncConfigRepository) bool {
	t.Helper()
	cfg, err := repo.Get(context.Background())
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	return cfg.ReauthRequired
}

func TestGetAccessToken_RejectedRefreshAsksForSignIn(t *testing.T) {
	keyring.MockInit()
	repo := newTestSyncConfigRepo(setupTestDB(t))
	cfg := seedRefreshToken(t, repo, "refresh-1")
	if err := keyring.Set(keyringService, keyringKeyFor(cfg.ClientID), "refresh-1"); err != nil {
		t.Fatalf("keyring.Set: %v", err)
	}

	stub := &stubAuthClient{refreshErr: rejectedRefresh(t)}
	mgr := NewSyncAuthManager(repo)
	mgr.SetClientID(cfg.ClientID)

	_, err := mgr.GetAccessToken(context.Background(), NewGRPCClientWithStubs(stub, nil))

	if !errors.Is(err, ErrAuthExpired) {
		t.Fatalf("GetAccessToken error = %v, want ErrAuthExpired", err)
	}
	if got := status.Code(err); got != codes.Unauthenticated {
		t.Errorf("status.Code(err) = %v, want the server's Unauthenticated", got)
	}
	if !reauthRequired(t, repo) {
		t.Error("a rejected refresh token must set reauth_required")
	}
	if len(stub.refreshTokens) != 1 {
		t.Errorf("refresh calls = %v, want one: keychain and mirror agree", stub.refreshTokens)
	}
}

func divergedSession(t *testing.T) (sqlite.SyncConfigRepository, *SyncAuthManager) {
	t.Helper()
	keyring.MockInit()
	repo := newTestSyncConfigRepo(setupTestDB(t))
	cfg := seedRefreshToken(t, repo, "refresh-mirror")
	if err := keyring.Set(keyringService, keyringKeyFor(cfg.ClientID), "refresh-stale"); err != nil {
		t.Fatalf("keyring.Set: %v", err)
	}
	mgr := NewSyncAuthManager(repo)
	mgr.SetClientID(cfg.ClientID)
	return repo, mgr
}

func TestGetAccessToken_RejectedKeychainTokenFallsBackToTheMirror(t *testing.T) {
	repo, mgr := divergedSession(t)
	stub := &stubAuthClient{
		refreshErrFor: map[string]error{"refresh-stale": rejectedRefresh(t)},
		refreshResp: authv1.RefreshResponse_builder{
			AccessToken: "access-2", RefreshToken: "refresh-2", ActiveOrgId: "org-1",
		}.Build(),
	}

	token, err := mgr.GetAccessToken(context.Background(), NewGRPCClientWithStubs(stub, nil))

	if err != nil {
		t.Fatalf("GetAccessToken: %v", err)
	}
	if token != "access-2" {
		t.Errorf("token = %q, want %q", token, "access-2")
	}
	if got := strings.Join(stub.refreshTokens, ","); got != "refresh-stale,refresh-mirror" {
		t.Errorf("refresh calls = %s, want the keychain token, then the mirror", got)
	}
	if reauthRequired(t, repo) {
		t.Error("a session the mirror still holds must not ask for sign-in")
	}
	if got := mgr.loadRefreshToken(); got != "refresh-2" {
		t.Errorf("loadRefreshToken() = %q, want the rotated %q", got, "refresh-2")
	}
}

func TestGetAccessToken_RejectedKeychainAndMirrorAskForSignIn(t *testing.T) {
	repo, mgr := divergedSession(t)
	stub := &stubAuthClient{refreshErr: rejectedRefresh(t)}

	_, err := mgr.GetAccessToken(context.Background(), NewGRPCClientWithStubs(stub, nil))

	if !errors.Is(err, ErrAuthExpired) {
		t.Fatalf("GetAccessToken error = %v, want ErrAuthExpired", err)
	}
	if got := strings.Join(stub.refreshTokens, ","); got != "refresh-stale,refresh-mirror" {
		t.Errorf("refresh calls = %s, want the keychain token, then the mirror", got)
	}
	if !reauthRequired(t, repo) {
		t.Error("both tokens rejected must set reauth_required")
	}
}

func TestGetAccessToken_MirrorRetryOutageIsTransient(t *testing.T) {
	repo, mgr := divergedSession(t)
	stub := &stubAuthClient{refreshErrFor: map[string]error{
		"refresh-stale":  rejectedRefresh(t),
		"refresh-mirror": status.Error(codes.Unavailable, "connection refused"),
	}}

	_, err := mgr.GetAccessToken(context.Background(), NewGRPCClientWithStubs(stub, nil))

	if err == nil || errors.Is(err, ErrAuthExpired) {
		t.Fatalf("GetAccessToken error = %v, want a transient failure", err)
	}
	if reauthRequired(t, repo) {
		t.Error("reauth_required must stay clear")
	}
}

func TestGetAccessToken_OtherRefreshFailuresKeepTheSession(t *testing.T) {
	cases := map[string]error{
		"unauthenticated without a reason": status.Error(codes.Unauthenticated, "token expired"),
		"another reason":                   statusWithReason(t, codes.Unauthenticated, "SESSION_REVOKED", "tetiva.app"),
		"another domain":                   statusWithReason(t, codes.Unauthenticated, "REFRESH_TOKEN_REJECTED", "example.com"),
		"rejected with another code":       statusWithReason(t, codes.PermissionDenied, "REFRESH_TOKEN_REJECTED", "tetiva.app"),
	}
	for name, refreshErr := range cases {
		t.Run(name, func(t *testing.T) {
			keyring.MockInit()
			repo := newTestSyncConfigRepo(setupTestDB(t))
			cfg := seedRefreshToken(t, repo, "refresh-1")

			stub := &stubAuthClient{refreshErr: refreshErr}
			mgr := NewSyncAuthManager(repo)
			mgr.SetClientID(cfg.ClientID)

			_, err := mgr.GetAccessToken(context.Background(), NewGRPCClientWithStubs(stub, nil))

			if err == nil {
				t.Fatal("expected the refresh to fail")
			}
			if errors.Is(err, ErrAuthExpired) {
				t.Errorf("GetAccessToken error = %v, must not be ErrAuthExpired", err)
			}
			if reauthRequired(t, repo) {
				t.Error("reauth_required must stay clear")
			}
		})
	}
}

func TestGetAccessToken_RejectionAnsweredAfterAdoptionLeavesTheFlagClear(t *testing.T) {
	keyring.MockInit()
	repo := newTestSyncConfigRepo(setupTestDB(t))
	ctx := context.Background()
	cfg := seedRefreshToken(t, repo, "refresh-old")

	stub := &stubAuthClient{
		refreshEntered: make(chan struct{}, 1),
		refreshGate:    make(chan struct{}),
		refreshErr:     rejectedRefresh(t),
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

	if err := <-refreshErr; errors.Is(err, ErrAuthExpired) {
		t.Errorf("GetAccessToken error = %v; the adopted session is alive", err)
	}
	if reauthRequired(t, repo) {
		t.Error("the rejection of the replaced session must not flag the adopted one")
	}
	if got := mgr.loadRefreshTokenDB(); got != "refresh-new" {
		t.Errorf("sync_config.refresh_token = %q, want %q", got, "refresh-new")
	}
}

func TestStoreRefreshToken_FailedKeychainWriteFallsBackToTheMirror(t *testing.T) {
	keyring.MockInit()
	repo := newTestSyncConfigRepo(setupTestDB(t))
	cfg := seedRefreshToken(t, repo, "refresh-old")
	account := keyringKeyFor(cfg.ClientID)
	if err := keyring.Set(keyringService, account, "refresh-old"); err != nil {
		t.Fatalf("keyring.Set: %v", err)
	}

	mgr := NewSyncAuthManager(repo)
	mgr.SetClientID(cfg.ClientID)
	mgr.keyringSet = func(string, string, string) error { return errors.New("keychain is locked") }

	mgr.storeRefreshToken("refresh-new", true)

	if got, err := keyring.Get(keyringService, account); err == nil {
		t.Errorf("keychain still holds %q after its write failed", got)
	}
	if got := mgr.loadRefreshToken(); got != "refresh-new" {
		t.Errorf("loadRefreshToken() = %q, want the mirrored %q", got, "refresh-new")
	}
}

func TestStoreRefreshToken_FailedLateKeychainWriteKeepsTheAdoptedEntry(t *testing.T) {
	keyring.MockInit()
	repo := newTestSyncConfigRepo(setupTestDB(t))
	ctx := context.Background()
	cfg := seedRefreshToken(t, repo, "refresh-old")

	mgr := NewSyncAuthManager(repo)
	mgr.SetClientID(cfg.ClientID)

	started := make(chan struct{})
	release := make(chan struct{})
	mgr.keyringSet = func(service, account, token string) error {
		if token == "refresh-late" {
			close(started)
			<-release
			return errors.New("keychain is locked")
		}
		return keyring.Set(service, account, token)
	}

	done := make(chan struct{})
	go func() {
		defer close(done)
		mgr.storeRefreshToken("refresh-late", true)
	}()
	<-started

	if _, err := mgr.AdoptSignIn(ctx, "sync.example.com:443", adoptTokens()); err != nil {
		t.Fatalf("AdoptSignIn: %v", err)
	}
	close(release)
	<-done

	if got, err := keyring.Get(keyringService, keyringKeyFor(cfg.ClientID)); err != nil || got != "refresh-new" {
		t.Errorf("keychain holds %q (err %v), want %q", got, err, "refresh-new")
	}
}

func TestHandleAuthErr_UnauthenticatedFromTheStreamIsNotTheEndOfTheSession(t *testing.T) {
	engine := newTestEngine(t)
	seedRefreshToken(t, engine.configRepo, "refresh-1")
	engine.auth.storeTokens("access-1", "", "org-1")

	ws := &workspaceSyncer{
		engine:           engine,
		localWorkspaceID: "ws-1",
		state:            StateConnected,
		pushSignal:       make(chan struct{}, 1),
	}

	for _, err := range []error{
		fmt.Errorf("push batch: %w", status.Error(codes.Unauthenticated, "token expired")),
		fmt.Errorf("pull: %w", rejectedRefresh(t)),
	} {
		if ws.handleAuthErr(err) {
			t.Errorf("handleAuthErr(%v) stopped the syncer", err)
		}
	}

	if ws.getState() == StateAuthExpired {
		t.Error("a 401 outside Refresh must not read as an ended session")
	}
	if engine.auth.IsLoggedIn() {
		t.Error("the cached access token must be dropped so the next attempt refreshes")
	}
	if reauthRequired(t, engine.configRepo) {
		t.Error("reauth_required must stay clear")
	}
}
