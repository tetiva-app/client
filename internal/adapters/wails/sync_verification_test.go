package wails

import (
	"context"
	"database/sql"
	"errors"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/zalando/go-keyring"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	authv1 "github.com/tetiva-app/proto/go/gophercourier/auth/v1"
	"github.com/tetiva-app/client/internal/adapters/wails/dto"
	"github.com/tetiva-app/client/internal/infrastructure/repository/sqlite"
	syncsvc "github.com/tetiva-app/client/internal/infrastructure/sync"
)

// seededWorkspaceID is the active workspace the migrations create.
const seededWorkspaceID = "00000000-0000-4000-a000-000000000001"

const testServerURL = "sync.test:443"

type verificationFixture struct {
	svc        *SyncService
	engine     *syncsvc.SyncEngine
	auth       *syncsvc.SyncAuthManager
	client     *syncsvc.GRPCClient
	authStub   *stubAuthClient
	wsStub     *stubWorkspaceClient
	configRepo sqlite.SyncConfigRepository
	db         *sql.DB
}

func newVerificationFixture(t *testing.T) *verificationFixture {
	t.Helper()
	keyring.MockInit()

	db := setupSyncTestDB(t)
	configRepo := sqlite.NewSyncConfigRepo(db)
	queueRepo := sqlite.NewSyncQueueRepo(db)
	auth := syncsvc.NewSyncAuthManager(configRepo)

	engine := syncsvc.NewSyncEngine(
		auth, queueRepo, configRepo, db,
		sqlite.NewCollectionRepo(db),
		sqlite.NewRequestRepo(db),
		sqlite.NewEnvironmentRepo(db),
		sqlite.NewVariableRepo(db),
	)
	t.Cleanup(engine.StopAll)

	authStub := &stubAuthClient{}
	wsStub := &stubWorkspaceClient{remoteID: "remote-1"}
	client := syncsvc.NewGRPCClientWithStubs(authStub, wsStub)

	return &verificationFixture{
		svc: &SyncService{
			engine:     engine,
			auth:       auth,
			configRepo: configRepo,
			queueRepo:  queueRepo,
			db:         db,
			newClient:  func(string) (*syncsvc.GRPCClient, error) { return client, nil },
		},
		engine:     engine,
		auth:       auth,
		client:     client,
		authStub:   authStub,
		wsStub:     wsStub,
		configRepo: configRepo,
		db:         db,
	}
}

func registerResponse(requiresVerification bool) *authv1.RegisterResponse {
	return authv1.RegisterResponse_builder{
		AccessToken:               "access-1",
		RefreshToken:              "refresh-1",
		ActiveOrgId:               "org-1",
		RequiresEmailVerification: requiresVerification,
	}.Build()
}

func getMeResponse(verified bool) *authv1.GetMeResponse {
	return authv1.GetMeResponse_builder{
		User: authv1.User_builder{Email: "a@b.c", EmailVerified: verified}.Build(),
	}.Build()
}

func TestSyncService_Register_UnverifiedEmailLeavesEngineDark(t *testing.T) {
	f := newVerificationFixture(t)
	f.authStub.registerResp = registerResponse(true)

	res := f.svc.Register(dto.RegisterRequest{ServerURL: testServerURL, Email: "a@b.c", Password: "pw"})

	require.Nil(t, res.Error)
	assert.True(t, res.Data.RequiresEmailVerification)
	assert.Equal(t, "a@b.c", res.Data.Email)
	assert.Zero(t, f.wsStub.calls(), "workspace discovery must not run before the address is confirmed")
	assert.False(t, f.engine.IsEnabledForWorkspace(seededWorkspaceID))

	st := f.svc.GetStatus()
	require.Nil(t, st.Error)
	assert.True(t, st.Data.AwaitingVerification)
	assert.True(t, st.Data.Enabled, "an unconfirmed account is still a connected account")
}

func TestSyncService_Register_VerifiedEmailStartsSync(t *testing.T) {
	f := newVerificationFixture(t)
	f.authStub.registerResp = registerResponse(false)

	res := f.svc.Register(dto.RegisterRequest{ServerURL: testServerURL, Email: "a@b.c", Password: "pw"})

	require.Nil(t, res.Error)
	assert.False(t, res.Data.RequiresEmailVerification)
	assert.Equal(t, 1, f.wsStub.calls())
	assert.True(t, f.engine.IsEnabledForWorkspace(seededWorkspaceID))
	assert.False(t, f.svc.GetStatus().Data.AwaitingVerification)
}

func TestSyncService_GetMe_ConcurrentVerificationEnablesSyncOnce(t *testing.T) {
	f := newVerificationFixture(t)
	f.registerUnverified(t)
	require.Zero(t, f.wsStub.calls())

	f.authStub.setGetMe(getMeResponse(true), nil)

	const callers = 8
	results := make([]Result[dto.MeResult], callers)
	start := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(callers)
	for i := 0; i < callers; i++ {
		go func(i int) {
			defer wg.Done()
			<-start
			results[i] = f.svc.GetMe()
		}(i)
	}
	close(start)
	wg.Wait()

	for i, res := range results {
		require.Nil(t, res.Error, "caller %d", i)
		assert.True(t, res.Data.EmailVerified, "caller %d", i)
	}
	assert.Equal(t, 1, f.wsStub.calls(), "sync must be enabled exactly once")
	assert.True(t, f.engine.IsEnabledForWorkspace(seededWorkspaceID))
	assert.False(t, f.svc.GetStatus().Data.AwaitingVerification)
}

func TestSyncService_GetMe_StillUnverifiedKeepsEngineDark(t *testing.T) {
	f := newVerificationFixture(t)
	f.registerUnverified(t)

	f.authStub.setGetMe(getMeResponse(false), nil)

	res := f.svc.GetMe()

	require.Nil(t, res.Error)
	assert.False(t, res.Data.EmailVerified)
	assert.Zero(t, f.wsStub.calls())
	assert.True(t, f.svc.GetStatus().Data.AwaitingVerification)
}

func TestSyncService_ResendVerification_RateLimitedCode(t *testing.T) {
	f := newVerificationFixture(t)
	f.registerUnverified(t)

	f.authStub.resendErr = status.Error(codes.ResourceExhausted, "too many requests")

	res := f.svc.ResendVerification()

	require.NotNil(t, res.Error)
	assert.Equal(t, ErrCodeRateLimited, res.Error.Code)
}

func TestSyncService_Logout_ClearsAwaitingVerification(t *testing.T) {
	f := newVerificationFixture(t)
	f.registerUnverified(t)
	require.True(t, f.svc.GetStatus().Data.AwaitingVerification)

	require.Nil(t, f.svc.Logout().Error)

	assert.False(t, f.svc.GetStatus().Data.AwaitingVerification)
}

// seedSession authenticates so the auth manager holds a live access token and
// the config looks like a previous run of the app.
func (f *verificationFixture) seedSession(t *testing.T) {
	t.Helper()
	ctx := context.Background()

	f.authStub.loginResp = authv1.LoginResponse_builder{
		AccessToken:  "access-1",
		RefreshToken: "refresh-1",
		ActiveOrgId:  "org-1",
	}.Build()
	_, err := f.auth.Login(ctx, f.client, "a@b.c", "pw")
	require.NoError(t, err)

	cfg, err := f.configRepo.Get(ctx)
	require.NoError(t, err)
	cfg.ServerURL = testServerURL
	require.NoError(t, f.configRepo.Update(ctx, cfg))
}

func (f *verificationFixture) registerUnverified(t *testing.T) {
	t.Helper()
	f.authStub.registerResp = registerResponse(true)
	require.Nil(t, f.svc.Register(
		dto.RegisterRequest{ServerURL: testServerURL, Email: "a@b.c", Password: "pw"}).Error)
}

func TestResumeOnStartup_GetMeFailureStartsSyncAnyway(t *testing.T) {
	f := newVerificationFixture(t)
	f.seedSession(t)
	f.authStub.setGetMe(nil, errors.New("server unreachable"))

	require.NoError(t, f.svc.ResumeOnStartup(context.Background()))

	assert.Equal(t, 1, f.wsStub.calls(), "an unreachable server must not gate a confirmed user")
	assert.True(t, f.engine.IsEnabledForWorkspace(seededWorkspaceID))
	assert.False(t, f.svc.GetStatus().Data.AwaitingVerification)
}

func TestResumeOnStartup_UnverifiedAccountLeavesEngineDark(t *testing.T) {
	f := newVerificationFixture(t)
	f.seedSession(t)
	f.authStub.setGetMe(getMeResponse(false), nil)

	require.NoError(t, f.svc.ResumeOnStartup(context.Background()))

	assert.Zero(t, f.wsStub.calls())
	assert.False(t, f.engine.IsEnabledForWorkspace(seededWorkspaceID))
	assert.True(t, f.svc.GetStatus().Data.AwaitingVerification)
}

func TestResumeOnStartup_VerifiedAccountStartsSync(t *testing.T) {
	f := newVerificationFixture(t)
	f.seedSession(t)
	f.authStub.setGetMe(getMeResponse(true), nil)

	require.NoError(t, f.svc.ResumeOnStartup(context.Background()))

	assert.Equal(t, 1, f.wsStub.calls())
	assert.True(t, f.engine.IsEnabledForWorkspace(seededWorkspaceID))
	assert.False(t, f.svc.GetStatus().Data.AwaitingVerification)
}
