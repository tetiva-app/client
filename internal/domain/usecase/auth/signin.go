package auth

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/tetiva-app/client/internal/domain"
)

// SignInParams is everything the sync server needs to describe this install on
// the consent page.
type SignInParams struct {
	ServerURL  string // host:port the app dials; passed back to the completion hook
	ClientID   string // sync_config.client_id, read by the caller before Start
	DeviceName string // os.Hostname(), "" lets the server default it
	Platform   string // runtime.GOOS
	AppVersion string
	Locale     string
	Intent     string // "signin" | "register"
}

// ServerInfo is the discovery answer: what this server offers before anyone
// signs in.
type ServerInfo struct {
	Version          string
	DesktopSignIn    bool
	DesktopSignInURL string
	RegistrationOpen bool
}

// SignInStart is the accepted request the browser is about to be sent to.
type SignInStart struct {
	RequestID    string
	LoginURL     string // as returned by the server; the manager appends "#claim=<secret>"
	ExpiresAt    time.Time
	PollInterval time.Duration // 0 -> defaultSignInPollInterval
}

// SignInStatusCode is the request's state as the server reports it.
type SignInStatusCode string

const (
	SignInPending  SignInStatusCode = "pending"
	SignInApproved SignInStatusCode = "approved"
	SignInDenied   SignInStatusCode = "denied"
	SignInExpired  SignInStatusCode = "expired"
)

// SignInTokens is the session the approved request produced.
type SignInTokens struct {
	AccessToken, RefreshToken, ActiveOrgID, Email string
	RequiresEmailVerification                     bool
}

// SignInPoll is one answer of the poll loop.
type SignInPoll struct {
	Status                   SignInStatusCode
	EmailVerificationPending bool          // with "pending": keep polling, show the hint
	ExpiresAt                time.Time     // server deadline; moves the flow deadline when extended
	Tokens                   *SignInTokens // only with "approved"
}

// SignInClient is one dialed connection to the sync server. Ownership passes to
// the manager at the first line of Start; the caller never closes it again.
type SignInClient interface {
	GetServerInfo(ctx context.Context) (ServerInfo, error) // ErrServerInfoUnsupported on Unimplemented
	StartDesktopSignIn(ctx context.Context, p SignInParams, redirectURI, codeChallenge, claimChallenge string) (SignInStart, error)
	PollDesktopSignIn(ctx context.Context, requestID, clientID, codeVerifier string) (SignInPoll, error)
	CancelDesktopSignIn(ctx context.Context, requestID, clientID, codeVerifier string) error
	Logout(ctx context.Context, accessToken string) error // best-effort revoke of a session the flow will not keep
	Close() error
}

// SignInInfo is what the waiting panel renders; nothing in it is secret except
// the claim in LoginURL's fragment, which is why LoginURL is never persisted.
type SignInInfo struct {
	ID        string
	LoginURL  string
	Host      string // hostname of LoginURL, for the "Opens {host}" line
	ExpiresAt time.Time
}

// SignInOutcome is what a finished sign-in tells the caller.
type SignInOutcome struct {
	Email                     string
	RequiresEmailVerification bool
}

// SignInStatus is the whole observable state of a flow, so a panel restored
// after a webview reload renders from Status alone.
type SignInStatus struct {
	State                    FlowState // starting | pending | done | error | cancelled
	Info                     SignInInfo
	EmailVerificationPending bool
	Outcome                  *SignInOutcome // set with done
	Err                      error
}

// SignInHooks are the caller's four attachment points around the flow.
type SignInHooks struct {
	// OnApproved is the commit phase: bounded local work on a fresh 3 s context.
	// A non-nil error ends the flow in error and the session is revoked.
	OnApproved func(ctx context.Context, client SignInClient, p SignInParams, tokens SignInTokens) error
	// OnDone runs after the flow reached done, outside the commit phase and
	// outside the manager's lock: start sync, enter the verification wait.
	OnDone func(outcome SignInOutcome)
	// OnCallback runs when the loopback receives the redirect (raise the window).
	OnCallback func()
	// OnTerminal runs on every terminal state except done, outside the manager's lock: the caller
	// stopped its syncers before Start, and this is the only signal that brings them back.
	OnTerminal func(state FlowState)
}

// SignInSink receives every status change, outside the manager's lock; the full status travels
// because pending -> pending with a new flag or deadline is a change the UI must see.
type SignInSink interface {
	OnSignInState(flowID string, st SignInStatus)
}

// SignInManager runs at most one browser sign-in at a time; a second Start
// cancels and joins the first, exactly like FlowManager does per owner.
type SignInManager interface {
	// Start takes ownership of client immediately: every error path below closes
	// it, so the caller must not.
	Start(ctx context.Context, flowID string, client SignInClient, p SignInParams, hooks SignInHooks) (SignInInfo, error)
	// Status is the only way to observe an outcome; terminal statuses are kept
	// for StatusRetention, setup failures included.
	Status(flowID string) (SignInStatus, bool)
	// Cancel is idempotent: nil for an unknown id, an already-terminal one, and a flow that was
	// committing and then joined. It errors only when a flow does not join within JoinCap.
	Cancel(flowID string) error
	// Shutdown returns nil only when every flow joined; otherwise it reports the
	// survivors, and a second Shutdown returns the same error while they remain.
	Shutdown(ctx context.Context) error
}

var (
	ErrSignInDenied          = errors.New("auth: sign-in was denied in the browser")
	ErrSignInExpired         = errors.New("auth: the sign-in link expired")
	ErrSignInUnsupported     = errors.New("auth: this server does not offer browser sign-in")
	ErrServerInfoUnsupported = errors.New("auth: this server has no discovery endpoint")
	// ErrSignInMalformed is an answer the protocol does not allow: APPROVED with
	// no tokens, or a status code this build does not know.
	ErrSignInMalformed = errors.New("auth: the server sent a malformed sign-in answer")
	// Classification lives in the transport client so the manager stays transport-agnostic:
	// transient retries at the same interval, throttled grows it; anything else ends the flow.
	ErrSignInTransient = errors.New("auth: sign-in poll failed, retrying")
	ErrSignInThrottled = errors.New("auth: sign-in poll is rate limited")
)

const (
	signInInfoTimeout         = 5 * time.Second
	signInStartTimeout        = 10 * time.Second
	signInPollTimeout         = 10 * time.Second
	signInCommitTimeout       = 3 * time.Second
	signInAbortTimeout        = 2 * time.Second
	defaultSignInPollInterval = 3 * time.Second
	claimSecretBytes          = 32
)

type signInFlow struct {
	id     string
	ctx    context.Context
	cancel context.CancelFunc
	// done is closed by the worker itself on exit, so "joined" means the goroutine can no longer
	// touch anything. A start that never reaches its worker closes it from failSetup.
	done chan struct{}

	client SignInClient
	params SignInParams
	hooks  SignInHooks

	// Setup resources, published under mu as each is created rather than at install:
	// failSetup closes a listener and cancels a request a later setup step failed after.
	lb        *signalLoopback
	requestID string
	verifier  string

	// Written once by install under mu; the worker reads them after that and only
	// updatePending writes info.ExpiresAt again, under mu.
	info     SignInInfo
	interval time.Duration

	// guarded by signInManager.mu
	phase         flowPhase
	state         FlowState
	err           error
	verifyPending bool
	outcome       *SignInOutcome
	retireAt      time.Time
}

// status must be called with the manager's mu held.
func (f *signInFlow) status() SignInStatus {
	return SignInStatus{
		State:                    f.state,
		Info:                     f.info,
		EmailVerificationPending: f.verifyPending,
		Outcome:                  f.outcome,
		Err:                      f.err,
	}
}

type signInManager struct {
	sink SignInSink
	opts FlowOptions

	mu     sync.Mutex
	closed bool
	flows  map[string]*signInFlow // active and retained terminal flows, by id
	active string                 // id of the one live flow, "" when none
}

var _ SignInManager = (*signInManager)(nil)

// NewSignInManager stores opts.withDefaults(), exactly as NewFlowManager does.
// JoinCap has to outlast the commit: a 1.5 s config write plus a 2 s keychain write.
func NewSignInManager(sink SignInSink, opts FlowOptions) SignInManager {
	return &signInManager{
		sink:  sink,
		opts:  opts.withDefaults(),
		flows: map[string]*signInFlow{},
	}
}

func (m *signInManager) Start(ctx context.Context, flowID string, client SignInClient,
	p SignInParams, hooks SignInHooks,
) (SignInInfo, error) {
	const funcName = "auth.SignInManager.Start"

	if err := uuid.Validate(flowID); err != nil {
		return SignInInfo{}, m.refuse(client, &domain.ValidationError{Fields: map[string]string{"flowId": "invalid UUID"}})
	}

	m.mu.Lock()
	closed := m.closed
	m.purge()
	_, used := m.flows[flowID]
	m.mu.Unlock()

	switch {
	case closed:
		return SignInInfo{}, m.refuse(client, ErrManagerClosed)
	case used:
		return SignInInfo{}, m.refuse(client, &domain.ValidationError{Fields: map[string]string{"flowId": "already used"}})
	}

	f, err := m.reserve(flowID, client, p, hooks)
	if err != nil {
		return SignInInfo{}, err
	}
	setupCtx, stopSetup := m.setupContext(ctx, f)
	defer stopSetup()

	prev, err := m.takeOver()
	if err != nil {
		return SignInInfo{}, m.failSetup(f, err)
	}

	if err = m.discover(setupCtx, client); err != nil {
		return SignInInfo{}, m.failSetup(f, err)
	}

	verifier, challenge, err := newPKCE(m.opts.Rand)
	if err != nil {
		return SignInInfo{}, m.failSetup(f, err)
	}
	claim, err := randomURLSafe(m.opts.Rand, claimSecretBytes)
	if err != nil {
		return SignInInfo{}, m.failSetup(f, fmt.Errorf("%s: %w", funcName, err))
	}
	sum := sha256.Sum256([]byte(claim))
	claimChallenge := base64.RawURLEncoding.EncodeToString(sum[:])

	// A flow without a listener still works: the poll alone finishes it, only a
	// little later than the redirect would have.
	redirectURI := ""
	if lb, lbErr := startSignalLoopback("0", m.opts); lbErr != nil {
		slog.Warn("auth: browser sign-in runs without a loopback callback", "error", lbErr)
	} else {
		m.setLoopback(f, lb)
		redirectURI = lb.redirectURI()
	}

	startCtx, cancelStart := context.WithTimeout(setupCtx, signInStartTimeout)
	start, err := client.StartDesktopSignIn(startCtx, p, redirectURI, challenge, claimChallenge)
	cancelStart()
	if err != nil {
		return SignInInfo{}, m.failSetup(f, err)
	}
	m.setRequest(f, start.RequestID, verifier)

	login, err := ValidateEndpointURL(start.LoginURL)
	if err != nil {
		return SignInInfo{}, m.failSetup(f, fmt.Errorf("%s: loginUrl: %w", funcName, err))
	}

	ready := signInSetup{
		interval: start.PollInterval,
		info: SignInInfo{
			ID:        flowID,
			LoginURL:  start.LoginURL + "#claim=" + claim,
			Host:      login.Hostname(),
			ExpiresAt: start.ExpiresAt,
		},
	}
	if ready.interval <= 0 {
		ready.interval = defaultSignInPollInterval
	}
	// The server picks the cadence, but a hostile or broken one must not be able
	// to spin the loop or park it past the flow's own deadline.
	ready.interval = min(max(ready.interval, m.opts.MinInterval), m.opts.MaxInterval)
	if ready.info.ExpiresAt.IsZero() {
		ready.info.ExpiresAt = m.opts.Now().Add(m.opts.CallbackTimeout)
	}

	if err = m.install(f, prev, ready); err != nil {
		return SignInInfo{}, m.failSetup(f, err)
	}

	go m.run(f)

	return ready.info, nil
}

func (m *signInManager) Status(flowID string) (SignInStatus, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.purge()
	f, ok := m.flows[flowID]
	if !ok {
		return SignInStatus{}, false
	}

	return f.status(), true
}

func (m *signInManager) Cancel(flowID string) error {
	m.mu.Lock()
	m.purge()
	f, ok := m.flows[flowID]
	if !ok {
		m.mu.Unlock()

		return nil
	}
	notify := m.stopRunning(f)
	done := f.done
	m.mu.Unlock()

	if notify {
		m.terminated(f, FlowCancelled)
	}
	if !m.join(done) {
		return fmt.Errorf("auth: the sign-in did not stop within %s", m.opts.JoinCap)
	}

	m.mu.Lock()
	m.release(f)
	m.mu.Unlock()

	return nil
}

func (m *signInManager) Shutdown(ctx context.Context) error {
	if ctx == nil {
		ctx = context.Background()
	}
	// fx stops with a context that has no deadline, so the manager supplies one.
	if deadline, ok := ctx.Deadline(); !ok || time.Until(deadline) > m.opts.JoinCap {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, m.opts.JoinCap)
		defer cancel()
	}

	m.mu.Lock()
	m.closed = true
	pending := make([]*signInFlow, 0, len(m.flows))
	cancelled := make([]*signInFlow, 0, len(m.flows))
	for _, f := range m.flows {
		if m.stopRunning(f) {
			cancelled = append(cancelled, f)
		}
		pending = append(pending, f)
	}
	m.mu.Unlock()

	for _, f := range cancelled {
		m.terminated(f, FlowCancelled)
	}

	survivors := 0
	for _, f := range pending {
		select {
		case <-f.done:
			continue
		default:
		}
		select {
		case <-f.done:
		case <-ctx.Done():
			survivors++
		}
	}
	if survivors > 0 {
		return fmt.Errorf("auth: %d sign-in(s) did not stop: %w", survivors, ctx.Err())
	}

	return nil
}

// discover refuses a server that cannot run the flow before any request is
// created, so nothing has to be cleaned up afterwards.
func (m *signInManager) discover(ctx context.Context, client SignInClient) error {
	const funcName = "auth.SignInManager.discover"

	infoCtx, cancel := context.WithTimeout(ctx, signInInfoTimeout)
	info, err := client.GetServerInfo(infoCtx)
	cancel()

	switch {
	case errors.Is(err, ErrServerInfoUnsupported):
		return ErrSignInUnsupported
	case err != nil:
		return fmt.Errorf("%s: %w", funcName, err)
	case !info.DesktopSignIn:
		return ErrSignInUnsupported
	}
	if _, err = ValidateEndpointURL(info.DesktopSignInURL); err != nil {
		slog.Warn("auth: the server advertises an unusable desktop sign-in URL", "host", urlHost(info.DesktopSignInURL))

		return ErrSignInUnsupported
	}

	return nil
}

// refuse closes a client the manager took over but never installed: ownership starts at the first
// line of Start, so a precheck failure must not leak the dialed connection.
func (m *signInManager) refuse(client SignInClient, cause error) error {
	if client != nil {
		_ = client.Close()
	}

	return cause
}

// reserve publishes the flow id before any network call, so a Cancel arriving mid-start reaches the
// flow instead of being dropped as unknown; until install fills in the rest it reports FlowStarting.
func (m *signInManager) reserve(flowID string, client SignInClient, p SignInParams,
	hooks SignInHooks,
) (*signInFlow, error) {
	ctx, cancel := context.WithCancel(context.Background())
	f := &signInFlow{
		id:     flowID,
		ctx:    ctx,
		cancel: cancel,
		done:   make(chan struct{}),
		client: client,
		params: p,
		hooks:  hooks,
		info:   SignInInfo{ID: flowID},
		state:  FlowStarting,
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if m.closed {
		cancel()

		return nil, m.refuse(client, ErrManagerClosed)
	}
	if _, used := m.flows[flowID]; used {
		cancel()

		return nil, m.refuse(client, &domain.ValidationError{Fields: map[string]string{"flowId": "already used"}})
	}
	m.flows[flowID] = f

	return f, nil
}

// setupContext bounds the slow part of a start by the caller's context and by
// the flow's own, so a Cancel or a Shutdown stops the work in progress.
func (m *signInManager) setupContext(ctx context.Context, f *signInFlow) (context.Context, func()) {
	setupCtx, cancel := context.WithCancel(ctx)
	stop := context.AfterFunc(f.ctx, cancel)

	return setupCtx, func() {
		stop()
		cancel()
	}
}

// setLoopback and setRequest publish a setup resource under mu, so a concurrent
// Cancel and a later failSetup both see it.
func (m *signInManager) setLoopback(f *signInFlow, lb *signalLoopback) {
	m.mu.Lock()
	defer m.mu.Unlock()

	f.lb = lb
}

func (m *signInManager) setRequest(f *signInFlow, requestID, verifier string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	f.requestID, f.verifier = requestID, verifier
}

// takeOver cancels and joins the live flow. The wait happens with the lock released — endCommit
// needs the same mutex, so waiting under it would freeze the whole manager for JoinCap.
func (m *signInManager) takeOver() (string, error) {
	m.mu.Lock()
	id := m.active
	if id == "" {
		m.mu.Unlock()

		return "", nil
	}
	prev, ok := m.flows[id]
	if !ok {
		m.active = ""
		m.mu.Unlock()

		return "", nil
	}
	notify := m.stopRunning(prev)
	done := prev.done
	m.mu.Unlock()

	if notify {
		m.terminated(prev, FlowCancelled)
	}
	if !m.join(done) {
		return "", &domain.ValidationError{Fields: map[string]string{
			"auth": "the previous sign-in is still finishing — try again",
		}}
	}

	m.mu.Lock()
	m.release(prev)
	m.mu.Unlock()

	return id, nil
}

// signInSetup is everything a start assembles outside the lock. install applies
// it under mu, so a Cancel racing the last step never sees half a flow.
type signInSetup struct {
	info     SignInInfo
	interval time.Duration
}

func (m *signInManager) install(f *signInFlow, prevID string, ready signInSetup) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.closed {
		return ErrManagerClosed
	}
	if f.phase != phaseRunning {
		return ErrFlowCancelled
	}
	if m.active != "" && m.active != prevID {
		return &domain.ValidationError{Fields: map[string]string{
			"auth": "another sign-in started meanwhile — try again",
		}}
	}

	f.info, f.interval = ready.info, ready.interval
	f.state = FlowPending
	m.active = f.id

	return nil
}

// failSetup ends a Start that never reached its worker. A Cancel or a Shutdown
// that already marked the flow terminal wins; otherwise it becomes error(cause).
func (m *signInManager) failSetup(f *signInFlow, cause error) error {
	m.mu.Lock()
	notify := m.markTerminal(f, FlowError, cause)
	state := f.state
	lb, requestID := f.lb, f.requestID
	m.release(f)
	m.mu.Unlock()

	f.cancel()
	if requestID != "" {
		m.cancelRemote(f)
	}
	if lb != nil {
		lb.close()
	}
	_ = f.client.Close()
	close(f.done)

	if notify {
		m.terminated(f, state)
	}

	return cause
}

// stopRunning marks a running flow cancelled and lets go of its listener; the caller holds mu and
// fires the hooks after releasing it. A committing flow is left alone: JoinCap outlasts its commit.
func (m *signInManager) stopRunning(f *signInFlow) bool {
	if f.phase != phaseRunning {
		return false
	}
	m.markTerminal(f, FlowCancelled, nil)
	f.cancel()
	if f.lb != nil {
		f.lb.close()
	}

	return true
}

func (m *signInManager) markTerminal(f *signInFlow, st FlowState, err error) bool {
	if f.phase == phaseTerminal {
		return false
	}
	f.phase = phaseTerminal
	f.state = st
	f.err = err
	f.retireAt = m.opts.Now().Add(m.opts.StatusRetention)

	return true
}

// release drops the active slot, but only while it still points at this flow:
// another caller may already have installed its successor.
func (m *signInManager) release(f *signInFlow) {
	if m.active == f.id {
		m.active = ""
	}
}

func (m *signInManager) purge() {
	now := m.opts.Now()
	for id, f := range m.flows {
		if f.phase == phaseTerminal && now.After(f.retireAt) {
			delete(m.flows, id)
			m.release(f)
		}
	}
}

func (m *signInManager) join(done <-chan struct{}) bool {
	timer := time.NewTimer(m.opts.JoinCap)
	defer timer.Stop()

	select {
	case <-done:
		return true
	case <-timer.C:
		return false
	}
}

// notify builds the full status under mu and fires the sink after unlock.
func (m *signInManager) notify(f *signInFlow) {
	if m.sink == nil {
		return
	}

	m.mu.Lock()
	st := f.status()
	m.mu.Unlock()

	m.sink.OnSignInState(f.id, st)
}

// terminated is the single exit announcement: the sink always, and OnTerminal on
// every terminal state but done, where OnDone takes its place.
func (m *signInManager) terminated(f *signInFlow, st FlowState) {
	m.notify(f)
	if st != FlowDone && f.hooks.OnTerminal != nil {
		f.hooks.OnTerminal(st)
	}
}

// beginCommit is the one gate between a flow and the caller's storage: a flow
// that was cancelled, replaced or shut down never gets past it.
func (m *signInManager) beginCommit(flowID string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	f, ok := m.flows[flowID]
	if !ok || f.phase != phaseRunning {
		return false
	}
	f.phase = phaseCommitting

	return true
}

func (m *signInManager) endCommit(flowID string, st FlowState, err error) {
	m.mu.Lock()
	f, ok := m.flows[flowID]
	notify := ok && m.markTerminal(f, st, err)
	m.mu.Unlock()

	if notify {
		m.terminated(f, st)
	}
}

func (m *signInManager) finishDone(flowID string, outcome SignInOutcome) {
	m.mu.Lock()
	f, ok := m.flows[flowID]
	notify := ok && m.markTerminal(f, FlowDone, nil)
	if notify {
		f.outcome = &outcome
	}
	m.mu.Unlock()

	if notify {
		m.terminated(f, FlowDone)
	}
}

// updatePending records the flag and the possibly extended deadline, and
// notifies the sink only when one of them actually changed.
func (m *signInManager) updatePending(flowID string, pending bool, deadline time.Time) {
	m.mu.Lock()
	f, ok := m.flows[flowID]
	if !ok || f.phase != phaseRunning {
		m.mu.Unlock()

		return
	}
	changed := f.verifyPending != pending || !f.info.ExpiresAt.Equal(deadline)
	f.verifyPending = pending
	f.info.ExpiresAt = deadline
	m.mu.Unlock()

	if changed {
		m.notify(f)
	}
}

// run is the worker: everything after it returns is another caller's business,
// which is why done is closed here and nowhere else.
func (m *signInManager) run(f *signInFlow) {
	// handedOff silences the client close on the one outcome that keeps it: done.
	handedOff := false

	defer close(f.done)
	defer f.cancel()
	defer func() {
		if !handedOff {
			_ = f.client.Close()
		}
	}()
	if f.lb != nil {
		defer f.lb.closeAfterDrain()
	}
	defer func() {
		m.mu.Lock()
		m.release(f)
		m.mu.Unlock()
	}()

	res, ok := m.poll(f)
	if !ok {
		return
	}

	tokens := *res.Tokens
	if f.hooks.OnApproved == nil {
		m.endCommit(f.id, FlowError, errors.New("auth: the sign-in has no commit hook"))

		return
	}
	if !m.beginCommit(f.id) {
		m.revoke(f, tokens.AccessToken)

		return
	}

	commitCtx, cancelCommit := context.WithTimeout(context.Background(), signInCommitTimeout)
	err := f.hooks.OnApproved(commitCtx, f.client, f.params, tokens)
	cancelCommit()
	if err != nil {
		m.revoke(f, tokens.AccessToken)
		m.endCommit(f.id, FlowError, err)

		return
	}

	outcome := SignInOutcome{Email: tokens.Email, RequiresEmailVerification: tokens.RequiresEmailVerification}
	handedOff = true
	m.finishDone(f.id, outcome)
	if f.hooks.OnDone != nil {
		f.hooks.OnDone(outcome)
	}
}

// poll runs the loop until the server approves or the flow ends; ok is false
// when it already marked the flow terminal.
func (m *signInManager) poll(f *signInFlow) (SignInPoll, bool) {
	interval := f.interval
	deadline := f.info.ExpiresAt
	if latest := m.opts.Now().Add(m.opts.CallbackTimeout); deadline.After(latest) {
		deadline = latest
	}
	// The redirect fires once; after that the channel stays closed and selecting
	// on it again would spin the loop instead of waiting out the interval.
	signal := f.lb.signal()

	for {
		if !m.opts.Now().Before(deadline) {
			m.endCommit(f.id, FlowError, ErrSignInExpired)

			return SignInPoll{}, false
		}
		woke, err := m.waitTick(f.ctx, signal, interval, f.hooks)
		if err != nil {
			m.cancelRemote(f)
			m.endCommit(f.id, FlowCancelled, nil)

			return SignInPoll{}, false
		}
		if woke {
			signal = nil
		}

		pollCtx, cancel := context.WithTimeout(f.ctx, signInPollTimeout)
		res, err := f.client.PollDesktopSignIn(pollCtx, f.requestID, f.params.ClientID, f.verifier)
		cancel()

		switch {
		case errors.Is(err, ErrSignInThrottled):
			interval = min(interval*2, m.opts.MaxInterval)

			continue
		case errors.Is(err, ErrSignInTransient):
			continue
		case err != nil && f.ctx.Err() != nil:
			m.cancelRemote(f)
			m.endCommit(f.id, FlowCancelled, nil)

			return SignInPoll{}, false
		case err != nil:
			m.endCommit(f.id, FlowError, err)

			return SignInPoll{}, false
		}

		// The server extends an approved request while the user confirms their
		// email, so the deadline comes from the answer, not from Start alone.
		if res.ExpiresAt.After(deadline) {
			deadline = res.ExpiresAt
		}

		switch res.Status {
		case SignInPending:
			m.updatePending(f.id, res.EmailVerificationPending, deadline)

			continue
		case SignInDenied:
			m.endCommit(f.id, FlowError, ErrSignInDenied)

			return SignInPoll{}, false
		case SignInExpired:
			m.endCommit(f.id, FlowError, ErrSignInExpired)

			return SignInPoll{}, false
		case SignInApproved:
			// A self-hosted server can answer APPROVED with no tokens.
			if res.Tokens == nil {
				m.endCommit(f.id, FlowError, ErrSignInMalformed)

				return SignInPoll{}, false
			}

			return res, true
		default:
			m.endCommit(f.id, FlowError, ErrSignInMalformed)

			return SignInPoll{}, false
		}
	}
}

// waitTick waits out one poll interval, cut short by the loopback redirect, and
// reports whether the redirect woke it. opts.Sleep cannot be selected against a channel.
func (m *signInManager) waitTick(ctx context.Context, signal <-chan struct{}, d time.Duration,
	hooks SignInHooks,
) (bool, error) {
	timer := time.NewTimer(d)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return false, ctx.Err()
	case <-signal:
		if hooks.OnCallback != nil {
			hooks.OnCallback()
		}

		return true, nil
	case <-timer.C:
		return false, nil
	}
}

// revoke drops a session the flow will not keep: the commit lost to a Cancel, or
// failed. Best effort — a stale session expires on its own.
func (m *signInManager) revoke(f *signInFlow, accessToken string) {
	if accessToken == "" {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), signInAbortTimeout)
	defer cancel()

	if err := f.client.Logout(ctx, accessToken); err != nil {
		slog.Warn("auth: could not revoke the sign-in session the flow dropped", "error", err)
	}
}

// cancelRemote lets the server drop a pending request instead of waiting out its
// TTL. Best effort, on a fresh context because the flow's own is cancelled.
func (m *signInManager) cancelRemote(f *signInFlow) {
	m.mu.Lock()
	requestID, verifier := f.requestID, f.verifier
	m.mu.Unlock()

	if requestID == "" {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), signInAbortTimeout)
	defer cancel()

	if err := f.client.CancelDesktopSignIn(ctx, requestID, f.params.ClientID, verifier); err != nil {
		slog.Warn("auth: could not cancel the sign-in request on the server", "error", err)
	}
}

// urlHost is a best-effort hostname for a log line: the URL itself never goes to
// the log, and one that will not parse has no host worth logging.
func urlHost(raw string) string {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return ""
	}

	return u.Hostname()
}
