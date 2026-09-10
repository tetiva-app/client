package wails

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/zalando/go-keyring"

	"github.com/tetiva-app/client/internal/adapters/wails/dto"
	"github.com/tetiva-app/client/internal/domain/usecase/auth"
	"github.com/tetiva-app/client/internal/infrastructure/repository/sqlite"
	syncsvc "github.com/tetiva-app/client/internal/infrastructure/sync"
)

// keyringService and the account prefix are unexported in the sync package, so
// the assertions on the keychain repeat them here.
const (
	testKeyringService = "tetiva"
	testKeyringPrefix  = "refresh_token:"
)

// failingConfigRepo starts failing Update from the failFrom-th call on, which is
// how a run that dies halfway through the §2.7 cleanup is reproduced.
type failingConfigRepo struct {
	sqlite.SyncConfigRepository

	mu       sync.Mutex
	failFrom int
	calls    int
}

func (r *failingConfigRepo) Update(ctx context.Context, cfg *sqlite.SyncConfig) error {
	r.mu.Lock()
	r.calls++
	fail := r.failFrom > 0 && r.calls >= r.failFrom
	r.mu.Unlock()
	if fail {
		return errors.New("sync_config is locked")
	}
	return r.SyncConfigRepository.Update(ctx, cfg)
}

func (r *failingConfigRepo) heal() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.failFrom = 0
}

// legacyInstall rewinds a signed-in config to what a build before this release
// left behind: a live session with no auth generation stamped on it.
func (f *verificationFixture) legacyInstall(t *testing.T) *sqlite.SyncConfig {
	t.Helper()
	ctx := context.Background()

	cfg, err := f.configRepo.Get(ctx)
	require.NoError(t, err)
	cfg.AuthGeneration = 0
	cfg.ReauthRequired = false
	require.NoError(t, f.configRepo.Update(ctx, cfg))
	return cfg
}

func readConfig(t *testing.T, repo sqlite.SyncConfigRepository) *sqlite.SyncConfig {
	t.Helper()
	cfg, err := repo.Get(context.Background())
	require.NoError(t, err)
	require.NotNil(t, cfg)
	return cfg
}

func TestResumeOnStartup_LegacySessionIsSignedOutOnce(t *testing.T) {
	f := newVerificationFixture(t)
	f.seedSession(t)
	cfg := f.legacyInstall(t)
	account := testKeyringPrefix + cfg.ClientID
	require.NotEmpty(t, cfg.RefreshToken, "the legacy install must start with a session to drop")

	f.restartApp(t)
	require.NoError(t, f.svc.ResumeOnStartup(context.Background()))

	after := readConfig(t, f.configRepo)
	assert.Empty(t, after.RefreshToken, "the SQLite mirror of the refresh token must be gone")
	assert.False(t, after.Enabled)
	assert.Equal(t, syncsvc.CurrentAuthGeneration, after.AuthGeneration)
	assert.True(t, after.ReauthRequired, "the UI still owes the user a sign-in prompt")
	assert.Equal(t, testServerURL, after.ServerURL, "the server stays so the modal can offer it back")
	assert.Equal(t, "a@b.c", after.UserEmail)

	_, err := keyring.Get(testKeyringService, account)
	assert.Error(t, err, "the keychain entry must be gone too")

	assert.Zero(t, f.wsStub.calls(), "sync must not start on the run that signs the install out")
	assert.False(t, f.engine.IsEnabledForWorkspace(seededWorkspaceID))
}

func TestResumeOnStartup_CrashAfterTheFlagsIsFinishedOnTheNextStart(t *testing.T) {
	f := newVerificationFixture(t)
	f.seedSession(t)
	f.legacyInstall(t)

	// The flags land on the first Update; everything the cleanup writes after
	// that fails, the way a locked database would fail it.
	repo := &failingConfigRepo{SyncConfigRepository: f.configRepo, failFrom: 2}
	f.configRepo = repo
	f.restartApp(t)
	require.NoError(t, f.svc.ResumeOnStartup(context.Background()))

	mid := readConfig(t, repo.SyncConfigRepository)
	assert.True(t, mid.ReauthRequired, "the flag is written first so the cleanup can repeat")
	assert.True(t, mid.Enabled, "the cleanup never got to disable the config")

	repo.heal()
	f.restartApp(t)
	require.NoError(t, f.svc.ResumeOnStartup(context.Background()))

	after := readConfig(t, repo.SyncConfigRepository)
	assert.False(t, after.Enabled)
	assert.Empty(t, after.RefreshToken)
	assert.True(t, after.ReauthRequired)
	assert.Zero(t, f.wsStub.calls())
}

func TestResumeOnStartup_ReauthRequiredSurvivesRepeatedStarts(t *testing.T) {
	f := newVerificationFixture(t)
	f.seedSession(t)
	f.legacyInstall(t)

	f.restartApp(t)
	require.NoError(t, f.svc.ResumeOnStartup(context.Background()))
	f.restartApp(t)
	require.NoError(t, f.svc.ResumeOnStartup(context.Background()))

	after := readConfig(t, f.configRepo)
	assert.True(t, after.ReauthRequired, "only a sign-in clears the flag")
	assert.False(t, after.Enabled)
	assert.Zero(t, f.wsStub.calls(), "sync must not start on either run")
	assert.False(t, f.engine.IsEnabledForWorkspace(seededWorkspaceID))
}

// Revoking the session on the server is best effort; the local credentials go
// either way, or an install that updates offline would stay signed in forever.
func TestResumeOnStartup_UnreachableServerStillSignsTheInstallOut(t *testing.T) {
	f := newVerificationFixture(t)
	f.seedSession(t)
	f.legacyInstall(t)

	f.restartApp(t)
	f.svc.newClient = func(string) (*syncsvc.GRPCClient, error) { return nil, errors.New("dial tcp: no route to host") }
	require.NoError(t, f.svc.ResumeOnStartup(context.Background()))

	after := readConfig(t, f.configRepo)
	assert.False(t, after.Enabled)
	assert.Empty(t, after.RefreshToken)
	assert.True(t, after.ReauthRequired)
}

func TestResumeOnStartup_CurrentGenerationIsLeftAlone(t *testing.T) {
	f := newVerificationFixture(t)
	f.seedSession(t)
	f.authStub.setGetMe(getMeResponse(true), nil)

	before := readConfig(t, f.configRepo)
	require.Equal(t, syncsvc.CurrentAuthGeneration, before.AuthGeneration)
	require.False(t, before.ReauthRequired)

	require.NoError(t, f.svc.ResumeOnStartup(context.Background()))

	after := readConfig(t, f.configRepo)
	assert.True(t, after.Enabled)
	assert.NotEmpty(t, after.RefreshToken)
	assert.False(t, after.ReauthRequired)
	assert.Equal(t, 1, f.wsStub.calls(), "a current session still starts sync")
	assert.True(t, f.engine.IsEnabledForWorkspace(seededWorkspaceID))
}

func TestResumeOnStartup_FreshInstallIsNotAskedToSignInAgain(t *testing.T) {
	f := newVerificationFixture(t)
	cfg, err := f.configRepo.GetOrCreate(context.Background())
	require.NoError(t, err)
	require.False(t, cfg.Enabled)
	require.Zero(t, cfg.AuthGeneration)

	require.NoError(t, f.svc.ResumeOnStartup(context.Background()))

	after := readConfig(t, f.configRepo)
	assert.Zero(t, after.AuthGeneration, "nobody was signed in, so there is nothing to stamp")
	assert.False(t, after.ReauthRequired)
	assert.False(t, f.svc.GetStatus().Data.ReauthRequired)
}

func TestGetStatus_ReportsReauthRequired(t *testing.T) {
	f := newVerificationFixture(t)
	f.seedSession(t)
	f.legacyInstall(t)

	f.restartApp(t)
	require.NoError(t, f.svc.ResumeOnStartup(context.Background()))

	st := f.svc.GetStatus()
	require.Nil(t, st.Error)
	assert.True(t, st.Data.ReauthRequired)
	assert.False(t, st.Data.Enabled)
}

func TestBrowserSignIn_ClearsReauthRequired(t *testing.T) {
	f := newVerificationFixture(t)
	f.seedSession(t)
	f.legacyInstall(t)

	f.restartApp(t)
	require.NoError(t, f.svc.ResumeOnStartup(context.Background()))
	require.True(t, readConfig(t, f.configRepo).ReauthRequired)

	f.signIn.onStart = func(_ string, hooks auth.SignInHooks) {
		require.NoError(t, hooks.OnApproved(context.Background(), f.client,
			auth.SignInParams{ServerURL: testServerURL}, approvedTokens(false)))
		hooks.OnDone(auth.SignInOutcome{Email: "a@b.c"})
	}
	require.Nil(t, f.svc.StartBrowserSignIn(dto.StartBrowserSignInRequest{
		FlowID: uuid.NewString(), ServerURL: testServerURL,
	}).Error)

	after := readConfig(t, f.configRepo)
	assert.False(t, after.ReauthRequired)
	assert.Equal(t, syncsvc.CurrentAuthGeneration, after.AuthGeneration)
	assert.True(t, after.Enabled)
	assert.False(t, f.svc.GetStatus().Data.ReauthRequired)
}

// The refresh cycle reads the config and writes it back whole; a partial write
// would zero the generation and put the install back into the re-login loop.
func TestResumeOnStartup_TokenRefreshDoesNotBringTheFlagBack(t *testing.T) {
	f := newVerificationFixture(t)
	f.seedSession(t)
	f.legacyInstall(t)

	f.restartApp(t)
	require.NoError(t, f.svc.ResumeOnStartup(context.Background()))

	f.signIn.onStart = func(_ string, hooks auth.SignInHooks) {
		require.NoError(t, hooks.OnApproved(context.Background(), f.client,
			auth.SignInParams{ServerURL: testServerURL}, approvedTokens(false)))
		hooks.OnDone(auth.SignInOutcome{Email: "a@b.c"})
	}
	require.Nil(t, f.svc.StartBrowserSignIn(dto.StartBrowserSignInRequest{
		FlowID: uuid.NewString(), ServerURL: testServerURL,
	}).Error)
	assert.Eventually(t, func() bool { return f.wsStub.calls() >= 1 }, 2*time.Second, 20*time.Millisecond)

	f.restartApp(t)
	f.authStub.setGetMe(getMeResponse(true), nil)
	require.NoError(t, f.svc.ResumeOnStartup(context.Background()))

	after := readConfig(t, f.configRepo)
	assert.Equal(t, syncsvc.CurrentAuthGeneration, after.AuthGeneration)
	assert.False(t, after.ReauthRequired, "a refreshed token must not re-trigger the sign-out")
	assert.True(t, after.Enabled)
}
