package wails

import (
	"context"
	"errors"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	authv1 "github.com/tetiva-app/proto/go/gophercourier/auth/v1"

	"github.com/tetiva-app/client/internal/adapters/wails/dto"
	"github.com/tetiva-app/client/internal/constants"
	"github.com/tetiva-app/client/internal/domain/usecase/auth"
	syncsvc "github.com/tetiva-app/client/internal/infrastructure/sync"
)

// fakeSignInManager replays a scripted sign-in inside Start, so the hooks the
// service installs run without a server, a browser or a real manager.
type fakeSignInManager struct {
	mu sync.Mutex

	startErr  error
	info      auth.SignInInfo
	statuses  map[string]auth.SignInStatus
	cancelErr error

	// onStart runs inside Start, before it returns: the place to snapshot what
	// the service did on the way in and to play the hooks.
	onStart func(flowID string, hooks auth.SignInHooks)

	startCalls int
	params     auth.SignInParams
	client     auth.SignInClient
	cancelled  []string
}

func (m *fakeSignInManager) Start(_ context.Context, flowID string, client auth.SignInClient,
	p auth.SignInParams, hooks auth.SignInHooks,
) (auth.SignInInfo, error) {
	m.mu.Lock()
	m.startCalls++
	m.params = p
	m.client = client
	onStart, startErr, info := m.onStart, m.startErr, m.info
	m.mu.Unlock()

	if onStart != nil {
		onStart(flowID, hooks)
	}
	if startErr != nil {
		return auth.SignInInfo{}, startErr
	}
	info.ID = flowID
	return info, nil
}

func (m *fakeSignInManager) Status(flowID string) (auth.SignInStatus, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	st, ok := m.statuses[flowID]
	return st, ok
}

func (m *fakeSignInManager) Cancel(flowID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.cancelled = append(m.cancelled, flowID)
	return m.cancelErr
}

func (m *fakeSignInManager) Shutdown(context.Context) error { return nil }

func (m *fakeSignInManager) setStatus(flowID string, st auth.SignInStatus) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.statuses == nil {
		m.statuses = map[string]auth.SignInStatus{}
	}
	m.statuses[flowID] = st
}

func (m *fakeSignInManager) startedParams() auth.SignInParams {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.params
}

func (m *fakeSignInManager) calls() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.startCalls
}

func serverInfoResponse(capability bool, signInURL string) *authv1.GetServerInfoResponse {
	var caps []string
	if capability {
		caps = []string{"desktop_signin"}
	}
	return authv1.GetServerInfoResponse_builder{
		ServerVersion:    "0.17.0",
		Capabilities:     caps,
		DesktopSigninUrl: signInURL,
		RegistrationOpen: true,
	}.Build()
}

func TestGetServerCapabilities_Advertised(t *testing.T) {
	f := newVerificationFixture(t)
	f.authStub.setServerInfo(serverInfoResponse(true, "https://app.tetiva.app/desktop-signin"), nil)

	res := f.svc.GetServerCapabilities(dto.ServerCapabilitiesRequest{ServerURL: testServerURL})

	require.Nil(t, res.Error)
	assert.True(t, res.Data.DesktopSignIn)
	assert.Equal(t, "app.tetiva.app", res.Data.SignInHost)
	assert.Equal(t, "0.17.0", res.Data.ServerVersion)
	assert.True(t, res.Data.RegistrationOpen)
}

func TestGetServerCapabilities_OldServerFallsBackToTheForm(t *testing.T) {
	f := newVerificationFixture(t)
	f.authStub.setServerInfo(nil, status.Error(codes.Unimplemented, "unknown method"))

	res := f.svc.GetServerCapabilities(dto.ServerCapabilitiesRequest{ServerURL: testServerURL})

	require.Nil(t, res.Error, "a server without discovery is not an error")
	assert.False(t, res.Data.DesktopSignIn)
}

func TestGetServerCapabilities_UnusableSignInURLIsNotAdvertised(t *testing.T) {
	f := newVerificationFixture(t)
	f.authStub.setServerInfo(serverInfoResponse(true, "http://evil.example/desktop-signin"), nil)

	res := f.svc.GetServerCapabilities(dto.ServerCapabilitiesRequest{ServerURL: testServerURL})

	require.Nil(t, res.Error)
	assert.False(t, res.Data.DesktopSignIn)
	assert.Empty(t, res.Data.SignInHost)
}

func TestGetServerCapabilities_DialFailureIsUnreachable(t *testing.T) {
	f := newVerificationFixture(t)
	f.svc.newClient = func(string) (*syncsvc.GRPCClient, error) { return nil, errors.New("no route") }

	res := f.svc.GetServerCapabilities(dto.ServerCapabilitiesRequest{ServerURL: testServerURL})

	require.NotNil(t, res.Error)
	assert.Equal(t, ErrCodeServerUnreachable, res.Error.Code)
}

func TestGetServerCapabilities_RateLimitedIsUnreachableNotMissingCapability(t *testing.T) {
	f := newVerificationFixture(t)
	f.authStub.setServerInfo(nil, status.Error(codes.ResourceExhausted, "slow down"))

	res := f.svc.GetServerCapabilities(dto.ServerCapabilitiesRequest{ServerURL: testServerURL})

	require.NotNil(t, res.Error)
	assert.Equal(t, ErrCodeServerUnreachable, res.Error.Code,
		"a refused discovery must offer Retry, not the in-app form and not rate_limited")
}

func TestStartBrowserSignIn_DescribesThisInstall(t *testing.T) {
	f := newVerificationFixture(t)
	flowID := uuid.NewString()

	res := f.svc.StartBrowserSignIn(dto.StartBrowserSignInRequest{
		FlowID: flowID, ServerURL: testServerURL, Intent: "register", Locale: "ru",
	})

	require.Nil(t, res.Error)
	assert.Equal(t, flowID, res.Data.FlowID)

	cfg, err := f.configRepo.Get(context.Background())
	require.NoError(t, err)

	p := f.signIn.startedParams()
	assert.Equal(t, cfg.ClientID, p.ClientID)
	assert.Equal(t, runtime.GOOS, p.Platform)
	assert.Equal(t, constants.AppVersion, p.AppVersion)
	assert.Equal(t, testServerURL, p.ServerURL)
	assert.Equal(t, "register", p.Intent)
	assert.Equal(t, "ru", p.Locale)
}

func TestStartBrowserSignIn_RejectsUnknownIntent(t *testing.T) {
	f := newVerificationFixture(t)

	res := f.svc.StartBrowserSignIn(dto.StartBrowserSignInRequest{
		FlowID: uuid.NewString(), ServerURL: testServerURL, Intent: "bogus",
	})

	require.NotNil(t, res.Error)
	assert.Equal(t, ErrCodeValidation, res.Error.Code)
	assert.Contains(t, res.Error.Fields, "intent")
	assert.Zero(t, f.signIn.calls(), "a bad intent must not stop the syncers")
}

func TestStartBrowserSignIn_RejectsAMalformedID(t *testing.T) {
	f := newVerificationFixture(t)
	f.startSeededSyncer(t)

	res := f.svc.StartBrowserSignIn(dto.StartBrowserSignInRequest{
		FlowID: "", ServerURL: testServerURL,
	})

	require.NotNil(t, res.Error)
	assert.Equal(t, ErrCodeValidation, res.Error.Code)
	assert.Contains(t, res.Error.Fields, "flowId")
	assert.True(t, f.engine.IsEnabledForWorkspace(seededWorkspaceID),
		"an id the restore cannot name must not reach StopAll")
}

func TestStartBrowserSignIn_UnsupportedServerNamesTheField(t *testing.T) {
	f := newVerificationFixture(t)
	f.signIn.startErr = auth.ErrSignInUnsupported

	res := f.svc.StartBrowserSignIn(dto.StartBrowserSignInRequest{
		FlowID: uuid.NewString(), ServerURL: testServerURL,
	})

	require.NotNil(t, res.Error)
	assert.Equal(t, ErrCodeValidation, res.Error.Code)
	assert.Contains(t, res.Error.Fields, "serverUrl")
}

func TestStartBrowserSignIn_SyncersAreStoppedBeforeTheFlowRuns(t *testing.T) {
	f := newVerificationFixture(t)
	f.registerUnverified(t)
	f.engine.StartWorkspace(seededWorkspaceID, "remote-1", 0)
	require.True(t, f.engine.IsEnabledForWorkspace(seededWorkspaceID))
	require.True(t, f.svc.GetStatus().Data.AwaitingVerification)

	var enabledDuringStart, awaitingDuringStart bool
	f.signIn.onStart = func(string, auth.SignInHooks) {
		enabledDuringStart = f.engine.IsEnabledForWorkspace(seededWorkspaceID)
		awaitingDuringStart = f.svc.awaitingVerification.Load()
	}

	require.Nil(t, f.svc.StartBrowserSignIn(dto.StartBrowserSignInRequest{
		FlowID: uuid.NewString(), ServerURL: testServerURL,
	}).Error)

	assert.False(t, enabledDuringStart, "adoption must not race a live syncer")
	assert.False(t, awaitingDuringStart)
}

// approvedTokens is the session an approved request hands back.
func approvedTokens(requiresVerification bool) auth.SignInTokens {
	return auth.SignInTokens{
		AccessToken:               "access-1",
		RefreshToken:              "refresh-1",
		ActiveOrgID:               "org-1",
		Email:                     "a@b.c",
		RequiresEmailVerification: requiresVerification,
	}
}

func TestStartBrowserSignIn_CommitAdoptsTheSessionAndTheEngineClient(t *testing.T) {
	f := newVerificationFixture(t)
	flowID := uuid.NewString()

	f.signIn.onStart = func(_ string, hooks auth.SignInHooks) {
		require.NoError(t, hooks.OnApproved(context.Background(), f.client,
			auth.SignInParams{ServerURL: testServerURL}, approvedTokens(false)))
		hooks.OnDone(auth.SignInOutcome{Email: "a@b.c"})
	}

	require.Nil(t, f.svc.StartBrowserSignIn(dto.StartBrowserSignInRequest{
		FlowID: flowID, ServerURL: testServerURL,
	}).Error)

	cfg, err := f.configRepo.Get(context.Background())
	require.NoError(t, err)
	assert.True(t, cfg.Enabled)
	assert.Equal(t, testServerURL, cfg.ServerURL)
	assert.Equal(t, "a@b.c", cfg.UserEmail)

	assert.Same(t, f.client, f.svc.client())
	assert.Same(t, f.client, f.engine.GRPCClient(),
		"the engine holds its own reference; a stale one would push to the previous server")

	// OnDone runs in its own goroutine and ends in enableSync.
	assert.Eventually(t, func() bool { return f.wsStub.calls() == 1 }, 2*time.Second, 20*time.Millisecond)
	assert.Eventually(t, func() bool { return f.engine.IsEnabledForWorkspace(seededWorkspaceID) },
		2*time.Second, 20*time.Millisecond)
}

func TestStartBrowserSignIn_UnconfirmedAccountWaitsInsteadOfSyncing(t *testing.T) {
	f := newVerificationFixture(t)

	f.signIn.onStart = func(_ string, hooks auth.SignInHooks) {
		require.NoError(t, hooks.OnApproved(context.Background(), f.client,
			auth.SignInParams{ServerURL: testServerURL}, approvedTokens(true)))
		hooks.OnDone(auth.SignInOutcome{Email: "a@b.c", RequiresEmailVerification: true})
	}

	require.Nil(t, f.svc.StartBrowserSignIn(dto.StartBrowserSignInRequest{
		FlowID: uuid.NewString(), ServerURL: testServerURL,
	}).Error)

	assert.Eventually(t, func() bool { return f.svc.GetStatus().Data.AwaitingVerification },
		2*time.Second, 20*time.Millisecond)
	assert.Zero(t, f.wsStub.calls(), "workspace discovery must wait for the confirmation")
}

func TestStartBrowserSignIn_CommitRejectsAForeignClient(t *testing.T) {
	f := newVerificationFixture(t)

	var commitErr error
	f.signIn.onStart = func(_ string, hooks auth.SignInHooks) {
		commitErr = hooks.OnApproved(context.Background(), stubSignInClient{},
			auth.SignInParams{ServerURL: testServerURL}, approvedTokens(false))
	}

	require.Nil(t, f.svc.StartBrowserSignIn(dto.StartBrowserSignInRequest{
		FlowID: uuid.NewString(), ServerURL: testServerURL,
	}).Error)

	require.Error(t, commitErr)
	assert.Nil(t, f.svc.client())
}

// stubSignInClient is a SignInClient that is not the transport the service
// expects, so the commit's type assertion has something to refuse.
type stubSignInClient struct{ auth.SignInClient }

func TestSetClient_KeepsThePreviousConnectionOpen(t *testing.T) {
	f := newVerificationFixture(t)

	// A real ClientConn: grpc.NewClient is lazy, so nothing is dialed. Close
	// answers ErrClientConnClosing on an already-closed conn, which is the probe.
	prev, err := syncsvc.NewGRPCClient("127.0.0.1:1")
	require.NoError(t, err)
	f.svc.setClient(prev)

	f.svc.setClient(f.client)

	require.NoError(t, prev.Close(),
		"setClient must not close the client it replaces — the engine and its streams still hold it")
}

func TestBrowserSignInStatus_UnknownFlowIsEmpty(t *testing.T) {
	f := newVerificationFixture(t)

	res := f.svc.BrowserSignInStatus(dto.FlowIDRequest{FlowID: uuid.NewString()})

	require.Nil(t, res.Error)
	assert.Empty(t, res.Data.State, "an empty state is how the frontend tells gone from pending")
	assert.Nil(t, res.Data.Auth)
}

func TestBrowserSignInStatus_PendingCarriesTheDeadlineAndTheHint(t *testing.T) {
	f := newVerificationFixture(t)
	flowID := uuid.NewString()
	deadline := time.Now().Add(20 * time.Minute).UTC().Truncate(time.Second)
	f.signIn.setStatus(flowID, auth.SignInStatus{
		State: auth.FlowPending,
		Info: auth.SignInInfo{
			ID: flowID, LoginURL: "https://app.tetiva.app/desktop-signin?request_id=r#claim=s",
			Host: "app.tetiva.app", ExpiresAt: deadline,
		},
		EmailVerificationPending: true,
	})

	res := f.svc.BrowserSignInStatus(dto.FlowIDRequest{FlowID: flowID})

	require.Nil(t, res.Error)
	assert.Equal(t, "pending", res.Data.State)
	assert.True(t, res.Data.EmailVerificationPending)
	assert.Equal(t, deadline.Format(time.RFC3339), res.Data.Info.ExpiresAt)
	assert.Equal(t, "app.tetiva.app", res.Data.Info.Host)
	assert.Nil(t, res.Data.Auth)
}

func TestBrowserSignInStatus_DoneCarriesTheAccount(t *testing.T) {
	f := newVerificationFixture(t)
	flowID := uuid.NewString()
	f.signIn.setStatus(flowID, auth.SignInStatus{
		State:   auth.FlowDone,
		Outcome: &auth.SignInOutcome{Email: "a@b.c", RequiresEmailVerification: true},
	})

	res := f.svc.BrowserSignInStatus(dto.FlowIDRequest{FlowID: flowID})

	require.Nil(t, res.Error)
	require.NotNil(t, res.Data.Auth)
	assert.Equal(t, "a@b.c", res.Data.Auth.Email)
	assert.True(t, res.Data.Auth.RequiresEmailVerification)
}

func TestBrowserSignInStatus_ErrorTravels(t *testing.T) {
	f := newVerificationFixture(t)
	flowID := uuid.NewString()
	f.signIn.setStatus(flowID, auth.SignInStatus{State: auth.FlowError, Err: auth.ErrSignInDenied})

	res := f.svc.BrowserSignInStatus(dto.FlowIDRequest{FlowID: flowID})

	require.Nil(t, res.Error)
	assert.Equal(t, "error", res.Data.State)
	assert.Equal(t, "Sign-in was denied in the browser", res.Data.Error,
		"a panel restored after a reload reads the same copy the event carried")
}

func TestBrowserSignInStatus_RejectsAMalformedID(t *testing.T) {
	f := newVerificationFixture(t)

	res := f.svc.BrowserSignInStatus(dto.FlowIDRequest{FlowID: "not-a-uuid"})

	require.NotNil(t, res.Error)
	assert.Equal(t, ErrCodeValidation, res.Error.Code)
	assert.Contains(t, res.Error.Fields, "flowId")
}

func TestCancelBrowserSignIn_UnknownIDIsFine(t *testing.T) {
	f := newVerificationFixture(t)
	flowID := uuid.NewString()

	require.Nil(t, f.svc.CancelBrowserSignIn(dto.FlowIDRequest{FlowID: flowID}).Error)
	assert.Equal(t, []string{flowID}, f.signIn.cancelled)
}

func TestCancelBrowserSignIn_RejectsAMalformedID(t *testing.T) {
	f := newVerificationFixture(t)

	res := f.svc.CancelBrowserSignIn(dto.FlowIDRequest{FlowID: "not-a-uuid"})

	require.NotNil(t, res.Error)
	assert.Equal(t, ErrCodeValidation, res.Error.Code)
	assert.Contains(t, res.Error.Fields, "flowId")
	assert.Empty(t, f.signIn.cancelled)
}

// startSeededSyncer gives the seeded workspace a remote mapping and a running
// syncer, the state a sign-in interrupts.
func (f *verificationFixture) startSeededSyncer(t *testing.T) {
	t.Helper()
	f.linkSeededWorkspace(t, "remote-1")
	f.enableSyncConfig(t)
	f.engine.StartWorkspace(seededWorkspaceID, "remote-1", 0)
	require.True(t, f.engine.IsEnabledForWorkspace(seededWorkspaceID))
}

// enableSyncConfig marks the config the way a connected account leaves it.
func (f *verificationFixture) enableSyncConfig(t *testing.T) {
	t.Helper()
	ctx := context.Background()

	cfg, err := f.configRepo.GetOrCreate(ctx)
	require.NoError(t, err)
	cfg.Enabled = true
	cfg.ServerURL = testServerURL
	require.NoError(t, f.configRepo.Update(ctx, cfg))
}

func TestStartBrowserSignIn_RefusedStartBringsTheSyncersBack(t *testing.T) {
	f := newVerificationFixture(t)
	f.startSeededSyncer(t)
	f.signIn.startErr = errors.New("boom")

	res := f.svc.StartBrowserSignIn(dto.StartBrowserSignInRequest{
		FlowID: uuid.NewString(), ServerURL: testServerURL,
	})

	require.NotNil(t, res.Error)
	assert.True(t, f.engine.IsEnabledForWorkspace(seededWorkspaceID),
		"a sign-in that never started must not leave local edits out of the outbox")
}

func TestStartBrowserSignIn_DialFailureIsUserCopyNotGoInternals(t *testing.T) {
	f := newVerificationFixture(t)
	f.startSeededSyncer(t)
	f.svc.newClient = func(string) (*syncsvc.GRPCClient, error) { return nil, errors.New("no route to host") }

	res := f.svc.StartBrowserSignIn(dto.StartBrowserSignInRequest{
		FlowID: uuid.NewString(), ServerURL: testServerURL,
	})

	require.NotNil(t, res.Error)
	assert.Equal(t, "Sign-in failed, try again", res.Error.Message)
	assert.True(t, f.engine.IsEnabledForWorkspace(seededWorkspaceID))
}

func TestStartBrowserSignIn_CancelledFlowBringsTheSyncersBack(t *testing.T) {
	f := newVerificationFixture(t)
	f.startSeededSyncer(t)

	f.signIn.onStart = func(_ string, hooks auth.SignInHooks) {
		hooks.OnTerminal(auth.FlowCancelled)
	}

	require.Nil(t, f.svc.StartBrowserSignIn(dto.StartBrowserSignInRequest{
		FlowID: uuid.NewString(), ServerURL: testServerURL,
	}).Error)

	assert.Eventually(t, func() bool { return f.engine.IsEnabledForWorkspace(seededWorkspaceID) },
		2*time.Second, 20*time.Millisecond)
}

func TestStartBrowserSignIn_CancelledFlowLeavesSyncOffWhenItWasOff(t *testing.T) {
	f := newVerificationFixture(t)
	f.linkSeededWorkspace(t, "remote-1")
	require.False(t, f.svc.GetStatus().Data.Enabled)

	f.signIn.onStart = func(_ string, hooks auth.SignInHooks) {
		hooks.OnTerminal(auth.FlowCancelled)
	}

	require.Nil(t, f.svc.StartBrowserSignIn(dto.StartBrowserSignInRequest{
		FlowID: uuid.NewString(), ServerURL: testServerURL,
	}).Error)

	assert.Never(t, func() bool { return f.engine.IsEnabledForWorkspace(seededWorkspaceID) },
		300*time.Millisecond, 20*time.Millisecond,
		"a linked workspace with no session behind it would only log a token failure per pull")
}

func TestStartBrowserSignIn_RetryKeepsTheVerificationWaitOfTheFirstAttempt(t *testing.T) {
	f := newVerificationFixture(t)
	f.registerUnverified(t)

	var first auth.SignInHooks
	f.signIn.onStart = func(_ string, hooks auth.SignInHooks) { first = hooks }
	require.Nil(t, f.svc.StartBrowserSignIn(dto.StartBrowserSignInRequest{
		FlowID: uuid.NewString(), ServerURL: testServerURL,
	}).Error)

	// Try again: the second attempt is armed while the first still holds, so the
	// wait it reads is the one the first attempt already cleared.
	var second auth.SignInHooks
	f.signIn.onStart = func(_ string, hooks auth.SignInHooks) { second = hooks }
	require.Nil(t, f.svc.StartBrowserSignIn(dto.StartBrowserSignInRequest{
		FlowID: uuid.NewString(), ServerURL: testServerURL,
	}).Error)

	first.OnTerminal(auth.FlowCancelled)
	second.OnTerminal(auth.FlowCancelled)

	assert.Eventually(t, func() bool { return f.svc.GetStatus().Data.AwaitingVerification },
		2*time.Second, 20*time.Millisecond)
}

func TestStartBrowserSignIn_CancelledFlowKeepsTheVerificationWait(t *testing.T) {
	f := newVerificationFixture(t)
	f.registerUnverified(t)
	require.True(t, f.svc.GetStatus().Data.AwaitingVerification)

	f.signIn.onStart = func(_ string, hooks auth.SignInHooks) {
		hooks.OnTerminal(auth.FlowCancelled)
	}

	require.Nil(t, f.svc.StartBrowserSignIn(dto.StartBrowserSignInRequest{
		FlowID: uuid.NewString(), ServerURL: testServerURL,
	}).Error)

	assert.Eventually(t, func() bool { return f.svc.GetStatus().Data.AwaitingVerification },
		2*time.Second, 20*time.Millisecond)
}

func TestStartBrowserSignIn_ReplacedFlowDoesNotRestoreUnderItsSuccessor(t *testing.T) {
	f := newVerificationFixture(t)
	f.startSeededSyncer(t)

	var first auth.SignInHooks
	f.signIn.onStart = func(_ string, hooks auth.SignInHooks) { first = hooks }
	require.Nil(t, f.svc.StartBrowserSignIn(dto.StartBrowserSignInRequest{
		FlowID: uuid.NewString(), ServerURL: testServerURL,
	}).Error)
	require.NotNil(t, first.OnTerminal)

	f.signIn.onStart = nil
	require.Nil(t, f.svc.StartBrowserSignIn(dto.StartBrowserSignInRequest{
		FlowID: uuid.NewString(), ServerURL: testServerURL,
	}).Error)

	// The replaced flow ends late; its restore belongs to an attempt that is over.
	first.OnTerminal(auth.FlowCancelled)

	assert.Never(t, func() bool { return f.engine.IsEnabledForWorkspace(seededWorkspaceID) },
		300*time.Millisecond, 20*time.Millisecond)
}

func TestStartBrowserSignIn_AFailedAttemptLeavesALiveOneAlone(t *testing.T) {
	f := newVerificationFixture(t)
	f.startSeededSyncer(t)

	var live auth.SignInHooks
	f.signIn.onStart = func(_ string, hooks auth.SignInHooks) { live = hooks }
	require.Nil(t, f.svc.StartBrowserSignIn(dto.StartBrowserSignInRequest{
		FlowID: uuid.NewString(), ServerURL: testServerURL,
	}).Error)
	require.NotNil(t, live.OnTerminal)

	f.signIn.onStart = nil
	f.signIn.startErr = errors.New("another sign-in started meanwhile")
	require.NotNil(t, f.svc.StartBrowserSignIn(dto.StartBrowserSignInRequest{
		FlowID: uuid.NewString(), ServerURL: testServerURL,
	}).Error)

	assert.False(t, f.engine.IsEnabledForWorkspace(seededWorkspaceID),
		"the attempt that failed does not own the syncers the live one is waiting on")

	live.OnTerminal(auth.FlowCancelled)
	assert.Eventually(t, func() bool { return f.engine.IsEnabledForWorkspace(seededWorkspaceID) },
		2*time.Second, 20*time.Millisecond, "the last attempt to end owes the restart")
}

func TestStartBrowserSignIn_AFailedAttemptAfterASuccessStillRestores(t *testing.T) {
	f := newVerificationFixture(t)
	f.startSeededSyncer(t)

	f.signIn.onStart = func(_ string, hooks auth.SignInHooks) {
		require.NoError(t, hooks.OnApproved(context.Background(), f.client,
			auth.SignInParams{ServerURL: testServerURL}, approvedTokens(false)))
		hooks.OnDone(auth.SignInOutcome{Email: "a@b.c"})
	}
	require.Nil(t, f.svc.StartBrowserSignIn(dto.StartBrowserSignInRequest{
		FlowID: uuid.NewString(), ServerURL: testServerURL,
	}).Error)
	require.Eventually(t, func() bool { return f.engine.IsEnabledForWorkspace(seededWorkspaceID) },
		2*time.Second, 20*time.Millisecond)

	f.signIn.onStart = nil
	f.signIn.startErr = errors.New("boom")
	require.NotNil(t, f.svc.StartBrowserSignIn(dto.StartBrowserSignInRequest{
		FlowID: uuid.NewString(), ServerURL: testServerURL,
	}).Error)

	assert.True(t, f.engine.IsEnabledForWorkspace(seededWorkspaceID),
		"a finished sign-in must not hold the restore of the next one")
}

func TestBrowserSignInStatus_InternalFailureIsNotShownToTheUser(t *testing.T) {
	f := newVerificationFixture(t)
	flowID := uuid.NewString()
	f.signIn.setStatus(flowID, auth.SignInStatus{
		State: auth.FlowError,
		Err:   errors.New("SyncService.signInHooks: update config: database is locked"),
	})

	res := f.svc.BrowserSignInStatus(dto.FlowIDRequest{FlowID: flowID})

	require.Nil(t, res.Error)
	assert.Equal(t, "Sign-in failed, try again", res.Data.Error)
}
