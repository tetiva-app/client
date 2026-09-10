package auth

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/tetiva-app/client/internal/domain"
	"github.com/tetiva-app/client/internal/domain/entities"
)

// FlowState is the lifecycle of one browser flow as the Auth tab shows it.
type FlowState string

const (
	// FlowStarting is a reserved id whose start has not finished assembling the
	// flow: Status answers with an empty Info until install fills it in.
	FlowStarting  FlowState = "starting"
	FlowPending   FlowState = "pending"
	FlowDone      FlowState = "done"
	FlowError     FlowState = "error"
	FlowCancelled FlowState = "cancelled"
)

// ErrManagerClosed is returned by Start* after Shutdown.
var ErrManagerClosed = errors.New("auth: flow manager is shut down")

// ErrFlowCancelled is returned by Start* when Cancel or Shutdown reached the
// flow while the start was still setting it up.
var ErrFlowCancelled = errors.New("auth: the token request was cancelled")

// Production defaults; a zero FlowOptions takes all of them.
const (
	defaultCallbackTimeout = 5 * time.Minute
	defaultPollInterval    = 5 * time.Second
	defaultMinInterval     = 1 * time.Second
	defaultMaxInterval     = 60 * time.Second
	defaultDeviceTimeout   = 5 * time.Minute
	defaultStatusRetention = 10 * time.Minute
	defaultJoinCap         = 5 * time.Second
	defaultDrainWindow     = 200 * time.Millisecond
)

// FlowInfo is safe to hand back after a page reload: the code_verifier and the device_code never
// leave the manager, and AuthorizeURL's state and S256 challenge are useless without the verifier.
type FlowInfo struct {
	ID                      string
	AuthorizeURL            string
	UserCode                string
	VerificationURI         string
	VerificationURIComplete string
	Interval                time.Duration
	ExpiresAt               time.Time
}

// FlowStatus carries Info as well as the state, so a UI that lost its memory to
// a page reload can restore the pending panel from Status alone.
type FlowStatus struct {
	State FlowState
	Info  FlowInfo
	Err   error
}

// FlowSink receives every state change, outside the manager's lock.
type FlowSink interface {
	OnFlowState(flowID string, st FlowState, err error)
}

// FlowOptions injects the clock, the sleeper and the deadlines so tests assert five-minute
// behaviour in milliseconds; a zero value takes the production defaults listed on each field.
//
// Sleep and the *http.Client handed to NewFlowManager MUST return promptly when their context is
// cancelled: that is what makes Cancel and Shutdown joinable, and a double that ignores it is
// reported as a survivor instead of hanging the app.
//
// INVARIANT: JoinCap > 3*commitAttempt + commitBackoff — a spent attempt, the backoff, a second
// attempt and the adoption re-read. The commit budget bounds itself (commit.go) and JoinCap is
// derived from it; changing either alone makes a cooperative flow report as a survivor.
type FlowOptions struct {
	Now             func() time.Time                                 // time.Now
	Sleep           func(ctx context.Context, d time.Duration) error // context-aware time.After
	Rand            io.Reader                                        // crypto/rand.Reader
	CallbackTimeout time.Duration                                    // 5 min
	DefaultInterval time.Duration                                    // 5 s
	MaxInterval     time.Duration                                    // 60 s — caps slow_down growth
	MinInterval     time.Duration                                    // 1 s — floors a server-chosen interval
	DeviceTimeout   time.Duration                                    // 5 min when expires_in is absent
	StatusRetention time.Duration                                    // 10 min
	JoinCap         time.Duration                                    // 5 s — bounds Cancel, replace and Shutdown
	DrainWindow     time.Duration                                    // 200 ms — 410 window after a terminal callback
	// Listen opens the loopback listener; a test injects a failing one to cover
	// the bind-failure branch without exhausting file descriptors.
	Listen func(network, address string) (net.Listener, error) // net.Listen
}

func (o FlowOptions) withDefaults() FlowOptions {
	if o.Now == nil {
		o.Now = time.Now
	}
	if o.Sleep == nil {
		o.Sleep = sleepContext
	}
	if o.Rand == nil {
		o.Rand = rand.Reader
	}
	if o.CallbackTimeout <= 0 {
		o.CallbackTimeout = defaultCallbackTimeout
	}
	if o.DefaultInterval <= 0 {
		o.DefaultInterval = defaultPollInterval
	}
	if o.MaxInterval <= 0 {
		o.MaxInterval = defaultMaxInterval
	}
	if o.MinInterval <= 0 {
		o.MinInterval = defaultMinInterval
	}
	if o.DeviceTimeout <= 0 {
		o.DeviceTimeout = defaultDeviceTimeout
	}
	if o.StatusRetention <= 0 {
		o.StatusRetention = defaultStatusRetention
	}
	if o.JoinCap <= 0 {
		o.JoinCap = defaultJoinCap
	}
	if o.DrainWindow <= 0 {
		o.DrainWindow = defaultDrainWindow
	}
	if o.Listen == nil {
		o.Listen = net.Listen
	}

	return o
}

func sleepContext(ctx context.Context, d time.Duration) error {
	timer := time.NewTimer(d)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

// FlowManager owns the in-flight browser flows, one per owner.
type FlowManager interface {
	// StartAuthCode takes a caller-chosen flowID so the caller can subscribe before starting. One flow
	// per owner: a second start cancels and joins the first, and FAILS if a committing flow misses JoinCap.
	StartAuthCode(ctx context.Context, flowID string, owner entities.AuthOwner, cfg OAuth2Config, redirectPort string) (FlowInfo, error)
	// StartDevice requests the device authorization before it returns, so the
	// caller already has the user code to show.
	StartDevice(ctx context.Context, flowID string, owner entities.AuthOwner, cfg OAuth2Config) (FlowInfo, error)
	// Status is the only way to observe a flow's outcome; terminal statuses are
	// retained for StatusRetention and carry the Info the flow started with.
	Status(flowID string) (FlowStatus, bool)
	// Cancel is idempotent — nil for an unknown id, a retained terminal one, and a flow that was
	// committing and then joined; it errors only when a flow does not join within JoinCap.
	Cancel(flowID string) error
	// Shutdown returns nil only when every flow joined; otherwise it reports the
	// survivors, and a second Shutdown returns the same error while they remain.
	Shutdown(ctx context.Context) error
}

type flowPhase int

const (
	phaseRunning flowPhase = iota
	phaseCommitting
	phaseTerminal
)

type flow struct {
	id       string
	owner    entities.AuthOwner
	ownerKey string
	ctx      context.Context
	cancel   context.CancelFunc
	// done is closed by the worker itself on exit, so "joined" means the goroutine can no longer
	// touch anything. A start that never reaches its worker closes it from abandon.
	done chan struct{}

	// Written once by install under mu, read-only afterwards.
	hash string
	info FlowInfo
	gen  int64
	port int
	lb   *loopback

	// guarded by manager.mu
	phase    flowPhase
	state    FlowState
	err      error
	retireAt time.Time
}

type manager struct {
	repo   TokenRepository
	client *http.Client
	sink   FlowSink
	opts   FlowOptions

	mu     sync.Mutex
	closed bool
	flows  map[string]*flow // active and retained terminal flows, by flow id
	active map[string]string
	ports  map[int]string
}

var _ FlowManager = (*manager)(nil)

// NewFlowManager wires the token store and the token endpoint client to the
// browser flows. A nil client falls back to the production default.
func NewFlowManager(repo TokenRepository, client *http.Client, sink FlowSink, opts FlowOptions) FlowManager {
	if client == nil {
		client = NewTokenHTTPClient()
	}

	return &manager{
		repo:   repo,
		client: client,
		sink:   sink,
		opts:   opts.withDefaults(),
		flows:  map[string]*flow{},
		active: map[string]string{},
		ports:  map[int]string{},
	}
}

func (m *manager) StartAuthCode(ctx context.Context, flowID string, owner entities.AuthOwner,
	cfg OAuth2Config, redirectPort string,
) (FlowInfo, error) {
	const funcName = "auth.FlowManager.StartAuthCode"

	if err := m.precheck(flowID, cfg, GrantAuthorizationCode); err != nil {
		return FlowInfo{}, err
	}
	port, err := ValidateRedirectPort(redirectPort)
	if err != nil {
		return FlowInfo{}, err
	}
	authorize, err := ValidateEndpointURL(cfg.AuthURL)
	if err != nil {
		return FlowInfo{}, fmt.Errorf("oauth2: authUrl: %w", err)
	}
	verifier, challenge, err := newPKCE(m.opts.Rand)
	if err != nil {
		return FlowInfo{}, err
	}
	state, err := newState(m.opts.Rand)
	if err != nil {
		return FlowInfo{}, err
	}

	f, err := m.reserve(flowID, owner)
	if err != nil {
		return FlowInfo{}, err
	}
	setupCtx, stopSetup := m.setupContext(ctx, f)
	defer stopSetup()

	prev, err := m.takeOver(owner)
	if err != nil {
		return FlowInfo{}, m.abandon(f, err)
	}

	generation, err := m.repo.Reserve(setupCtx, owner)
	if err != nil {
		if errors.Is(err, ErrOwnerGone) {
			return FlowInfo{}, m.abandon(f, err)
		}

		return FlowInfo{}, m.abandon(f, fmt.Errorf("%s: %w", funcName, err))
	}

	lb, err := m.bind(port, state, prev)
	if err != nil {
		return FlowInfo{}, m.abandon(f, err)
	}

	redirectURI := lb.redirectURI()
	ready := flowSetup{
		hash: ConfigHash(cfg),
		gen:  generation,
		port: lb.port,
		lb:   lb,
		info: FlowInfo{
			ID:           flowID,
			AuthorizeURL: authorizeURL(authorize, cfg, redirectURI, state, challenge),
			ExpiresAt:    m.opts.Now().Add(m.opts.CallbackTimeout),
		},
	}

	if err = m.install(f, prev.id, ready); err != nil {
		lb.close()

		return FlowInfo{}, m.abandon(f, err)
	}

	go m.run(f.ctx, f, func(workCtx context.Context) (*Token, error) {
		res, waitErr := m.waitCallback(workCtx, lb)
		if waitErr != nil {
			return nil, waitErr
		}
		if res.Err != "" {
			return nil, &OAuthError{Code: res.Err, Description: res.ErrDescription}
		}

		return requestToken(workCtx, m.client, cfg, url.Values{
			"grant_type":    {GrantAuthorizationCode},
			"code":          {res.Code},
			"redirect_uri":  {redirectURI},
			"code_verifier": {verifier},
		}, m.opts.Now())
	})

	return ready.info, nil
}

func (m *manager) StartDevice(ctx context.Context, flowID string, owner entities.AuthOwner,
	cfg OAuth2Config,
) (FlowInfo, error) {
	const funcName = "auth.FlowManager.StartDevice"

	if err := m.precheck(flowID, cfg, GrantDeviceCode); err != nil {
		return FlowInfo{}, err
	}

	f, err := m.reserve(flowID, owner)
	if err != nil {
		return FlowInfo{}, err
	}
	setupCtx, stopSetup := m.setupContext(ctx, f)
	defer stopSetup()

	prev, err := m.takeOver(owner)
	if err != nil {
		return FlowInfo{}, m.abandon(f, err)
	}

	generation, err := m.repo.Reserve(setupCtx, owner)
	if err != nil {
		if errors.Is(err, ErrOwnerGone) {
			return FlowInfo{}, m.abandon(f, err)
		}

		return FlowInfo{}, m.abandon(f, fmt.Errorf("%s: %w", funcName, err))
	}

	da, err := requestDeviceAuthorization(setupCtx, m.client, cfg, m.opts.Now(), m.opts)
	if err != nil {
		return FlowInfo{}, m.abandon(f, err)
	}

	ready := flowSetup{
		hash: ConfigHash(cfg),
		gen:  generation,
		info: FlowInfo{
			ID:                      flowID,
			UserCode:                da.UserCode,
			VerificationURI:         da.VerificationURI,
			VerificationURIComplete: da.VerificationURIComplete,
			Interval:                da.Interval,
			ExpiresAt:               da.ExpiresAt,
		},
	}

	if err = m.install(f, prev.id, ready); err != nil {
		return FlowInfo{}, m.abandon(f, err)
	}

	go m.run(f.ctx, f, func(workCtx context.Context) (*Token, error) {
		return m.pollDevice(workCtx, cfg, da)
	})

	return ready.info, nil
}

func (m *manager) Status(flowID string) (FlowStatus, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.purge()
	f, ok := m.flows[flowID]
	if !ok {
		return FlowStatus{}, false
	}

	return FlowStatus{State: f.state, Info: f.info, Err: f.err}, true
}

func (m *manager) Cancel(flowID string) error {
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
		m.notify(f.id, FlowCancelled, nil)
	}
	if !m.join(done) {
		return fmt.Errorf("auth: the token request did not stop within %s", m.opts.JoinCap)
	}

	m.mu.Lock()
	m.release(f)
	m.mu.Unlock()

	return nil
}

func (m *manager) Shutdown(ctx context.Context) error {
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
	pending := make([]*flow, 0, len(m.flows))
	cancelled := make([]string, 0, len(m.flows))
	for _, f := range m.flows {
		if m.stopRunning(f) {
			cancelled = append(cancelled, f.id)
		}
		pending = append(pending, f)
	}
	m.mu.Unlock()

	for _, id := range cancelled {
		m.notify(id, FlowCancelled, nil)
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
		return fmt.Errorf("auth: %d flow(s) did not stop: %w", survivors, ctx.Err())
	}

	return nil
}

// precheck runs every refusal that must happen before a reservation, a socket or
// a network call, in the order the spec lists them.
func (m *manager) precheck(flowID string, cfg OAuth2Config, grant string) error {
	if err := uuid.Validate(flowID); err != nil {
		return &domain.ValidationError{Fields: map[string]string{"flowId": "invalid UUID"}}
	}

	m.mu.Lock()
	closed := m.closed
	m.purge()
	_, used := m.flows[flowID]
	m.mu.Unlock()

	if closed {
		return ErrManagerClosed
	}
	if used {
		return &domain.ValidationError{Fields: map[string]string{"flowId": "already used"}}
	}

	return ValidateFlowConfig(cfg, grant)
}

// takeOverSnapshot is what a replacement saw before it waited: the owner's previous flow, the port
// it held, and whether the takeover closed its listener — a terminal flow keeps it for the drain.
type takeOverSnapshot struct {
	id             string
	port           int
	listenerClosed bool
}

// takeOver cancels and joins the owner's current flow. The wait happens with the lock released —
// endCommit needs the same mutex, so waiting under it would freeze the whole manager for JoinCap.
func (m *manager) takeOver(owner entities.AuthOwner) (takeOverSnapshot, error) {
	key := ownerKey(owner)

	m.mu.Lock()
	id, ok := m.active[key]
	if !ok {
		m.mu.Unlock()

		return takeOverSnapshot{}, nil
	}
	f, ok := m.flows[id]
	if !ok {
		delete(m.active, key)
		m.mu.Unlock()

		return takeOverSnapshot{}, nil
	}
	notify := m.stopRunning(f)
	snapshot := takeOverSnapshot{id: f.id, port: f.port, listenerClosed: notify}
	done := f.done
	m.mu.Unlock()

	if notify {
		m.notify(f.id, FlowCancelled, nil)
	}
	if !m.join(done) {
		return takeOverSnapshot{}, &domain.ValidationError{Fields: map[string]string{
			"auth": "the previous token request is still finishing — try again",
		}}
	}

	m.mu.Lock()
	m.release(f)
	m.mu.Unlock()

	return snapshot, nil
}

// reserve publishes the flow id before the slow part of a start (takeOver's join, the bind, the
// device round trip), so a mid-start Cancel reaches the flow; until install runs it reports FlowStarting.
func (m *manager) reserve(flowID string, owner entities.AuthOwner) (*flow, error) {
	ctx, cancel := context.WithCancel(context.Background())
	f := &flow{
		id:       flowID,
		owner:    owner,
		ownerKey: ownerKey(owner),
		ctx:      ctx,
		cancel:   cancel,
		done:     make(chan struct{}),
		state:    FlowStarting,
		info:     FlowInfo{ID: flowID},
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if m.closed {
		cancel()

		return nil, ErrManagerClosed
	}
	if _, used := m.flows[flowID]; used {
		cancel()

		return nil, &domain.ValidationError{Fields: map[string]string{"flowId": "already used"}}
	}
	m.flows[flowID] = f

	return f, nil
}

// setupContext bounds the slow part of a start by the caller's context and by
// the flow's own, so a Cancel or a Shutdown stops the work in progress.
func (m *manager) setupContext(ctx context.Context, f *flow) (context.Context, func()) {
	setupCtx, cancel := context.WithCancel(ctx)
	stop := context.AfterFunc(f.ctx, cancel)

	return setupCtx, func() {
		stop()
		cancel()
	}
}

// abandon closes a reservation whose start failed: a flow Cancel or Shutdown already marked terminal
// keeps its retained status, one that never ran is dropped, and done is closed here.
func (m *manager) abandon(f *flow, cause error) error {
	m.mu.Lock()
	if f.phase != phaseTerminal {
		delete(m.flows, f.id)
	}
	m.release(f)
	m.mu.Unlock()

	f.cancel()
	close(f.done)

	return cause
}

// flowSetup is everything a start assembles outside the lock. install applies it
// under mu, so a Cancel racing the last step of a start never sees half a flow.
type flowSetup struct {
	hash string
	gen  int64
	port int
	lb   *loopback
	info FlowInfo
}

// install publishes the flow only if it was not cancelled during setup and nobody else claimed the
// owner while the replacement waited for its predecessor to join.
func (m *manager) install(f *flow, prevID string, ready flowSetup) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.closed {
		return ErrManagerClosed
	}
	if f.phase != phaseRunning {
		return ErrFlowCancelled
	}
	if current, ok := m.active[f.ownerKey]; ok && current != prevID {
		return &domain.ValidationError{Fields: map[string]string{
			"auth": "another token request started meanwhile — try again",
		}}
	}

	f.hash, f.gen, f.port, f.lb = ready.hash, ready.gen, ready.port, ready.lb
	f.info = ready.info
	f.state = FlowPending
	m.active[f.ownerKey] = f.id
	if f.port != 0 {
		m.ports[f.port] = f.id
	}

	return nil
}

// stopRunning cancels a running flow and lets go of its resources; the caller holds mu and fires the
// sink after. A committing flow is left alone: interrupting its bounded Put leaves only the generation guard.
func (m *manager) stopRunning(f *flow) bool {
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

func (m *manager) markTerminal(f *flow, st FlowState, err error) bool {
	if f.phase == phaseTerminal {
		return false
	}
	f.phase = phaseTerminal
	f.state = st
	f.err = err
	f.retireAt = m.opts.Now().Add(m.opts.StatusRetention)

	return true
}

// release drops the owner slot and the port entry, but only while they still
// point at this flow: another caller may already have installed its successor.
func (m *manager) release(f *flow) {
	if m.active[f.ownerKey] == f.id {
		delete(m.active, f.ownerKey)
	}
	if f.port != 0 && m.ports[f.port] == f.id {
		delete(m.ports, f.port)
	}
}

func (m *manager) purge() {
	now := m.opts.Now()
	for id, f := range m.flows {
		if f.phase == phaseTerminal && now.After(f.retireAt) {
			delete(m.flows, id)
			m.release(f)
		}
	}
}

func (m *manager) join(done <-chan struct{}) bool {
	timer := time.NewTimer(m.opts.JoinCap)
	defer timer.Stop()

	select {
	case <-done:
		return true
	case <-timer.C:
		return false
	}
}

func (m *manager) notify(flowID string, st FlowState, err error) {
	if m.sink == nil {
		return
	}
	m.sink.OnFlowState(flowID, st, err)
}

// beginCommit is the one gate between a flow and the token store: a flow that
// was cancelled, replaced or shut down never gets past it.
func (m *manager) beginCommit(flowID string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	f, ok := m.flows[flowID]
	if !ok || f.phase != phaseRunning {
		return false
	}
	f.phase = phaseCommitting

	return true
}

func (m *manager) endCommit(flowID string, st FlowState, err error) {
	m.mu.Lock()
	f, ok := m.flows[flowID]
	notify := ok && m.markTerminal(f, st, err)
	m.mu.Unlock()

	if notify {
		m.notify(flowID, st, err)
	}
}

// run is the worker: everything after it returns is another caller's business,
// which is why done is closed here and nowhere else.
func (m *manager) run(ctx context.Context, f *flow, work func(context.Context) (*Token, error)) {
	defer close(f.done)
	defer func() {
		m.mu.Lock()
		m.release(f)
		m.mu.Unlock()
	}()
	defer f.cancel()

	if f.lb != nil {
		defer f.lb.closeAfterDrain()
	}

	tok, err := work(ctx)
	if err != nil {
		m.endCommit(f.id, FlowError, err)

		return
	}
	if !m.beginCommit(f.id) {
		return
	}

	_, err = commitToken(ctx, m.repo, f.owner, f.gen, f.hash, tok, m.opts.Now())
	switch {
	case err == nil:
		m.endCommit(f.id, FlowDone, nil)
	case errors.Is(err, ErrStaleToken):
		m.endCommit(f.id, FlowError, errors.New("the token was cleared while the flow was running"))
	case errors.Is(err, ErrOwnerGone):
		// Defensive: the SQLite store reports a deleted owner from Reserve, and a
		// deletion during the flow reaches Put as ErrStaleToken instead.
		m.endCommit(f.id, FlowError, errors.New("the request or collection was deleted"))
	default:
		m.endCommit(f.id, FlowError, err)
	}
}

// bind opens the loopback listener. A busy fixed port is retried once: the owner's own previous flow
// just released it, or a finished flow is still inside its drain window.
func (m *manager) bind(port, state string, prev takeOverSnapshot) (*loopback, error) {
	lb, err := startLoopback(port, state, m.opts)
	if err == nil {
		return lb, nil
	}

	requested, convErr := strconv.Atoi(port)
	switch {
	case convErr != nil || requested == 0:
		return nil, m.bindConflictError(port)
	case requested == prev.port && prev.listenerClosed:
	case !m.portHeldByLiveFlow(requested):
		// Real time, not the injected clock: this waits for a socket, not a deadline.
		time.Sleep(m.opts.DrainWindow)
	default:
		return nil, m.bindConflictError(port)
	}
	if lb, retryErr := startLoopback(port, state, m.opts); retryErr == nil {
		return lb, nil
	}

	return nil, m.bindConflictError(port)
}

func (m *manager) portHeldByLiveFlow(port int) bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	f, ok := m.flows[m.ports[port]]

	return ok && f.phase != phaseTerminal
}

func (m *manager) bindConflictError(port string) error {
	held := false
	if requested, err := strconv.Atoi(port); err == nil {
		held = m.portHeldByLiveFlow(requested)
	}

	message := fmt.Sprintf("port %s is busy — another application is using it; "+
		"change Redirect Port or set it to 0 to pick a free one", port)
	if held {
		message = fmt.Sprintf("port %s is busy: another request is waiting for its browser callback — "+
			"change Redirect Port or set it to 0 to pick a free one", port)
	}

	return &domain.ValidationError{Fields: map[string]string{"redirectPort": message}}
}

// authorizeURL merges the flow's parameters into whatever query the user
// configured on authUrl: the flow's values win on a collision, the rest survive.
func authorizeURL(endpoint *url.URL, cfg OAuth2Config, redirectURI, state, challenge string) string {
	q := endpoint.Query()
	q.Set("response_type", "code")
	q.Set("client_id", cfg.ClientID)
	q.Set("redirect_uri", redirectURI)
	q.Set("state", state)
	q.Set("code_challenge", challenge)
	q.Set("code_challenge_method", "S256")
	if cfg.Scope != "" {
		q.Set("scope", cfg.Scope)
	}
	if cfg.Audience != "" {
		q.Set("audience", cfg.Audience)
	}
	endpoint.RawQuery = q.Encode()

	return endpoint.String()
}

func (m *manager) waitCallback(ctx context.Context, lb *loopback) (callbackResult, error) {
	timer := time.NewTimer(m.opts.CallbackTimeout)
	defer timer.Stop()

	select {
	case res := <-lb.done:
		return res, nil
	case <-timer.C:
		return callbackResult{}, fmt.Errorf("oauth2: the browser did not come back within %s", m.opts.CallbackTimeout)
	case <-ctx.Done():
		return callbackResult{}, ctx.Err()
	}
}

func (m *manager) pollDevice(ctx context.Context, cfg OAuth2Config, da deviceAuth) (*Token, error) {
	interval := da.Interval
	form := devicePollForm(cfg, da.DeviceCode)

	for {
		// An IdP may pair a long interval with a short expires_in, and the flow
		// must end when it said it would rather than one interval later.
		if err := m.opts.Sleep(ctx, min(interval, max(da.ExpiresAt.Sub(m.opts.Now()), 0))); err != nil {
			return nil, err
		}
		if !m.opts.Now().Before(da.ExpiresAt) {
			return nil, errors.New("oauth2: the device code expired before it was approved")
		}

		tok, err := requestToken(ctx, m.client, cfg, form, m.opts.Now())
		if err == nil {
			return tok, nil
		}

		var oauthErr *OAuthError
		if !errors.As(err, &oauthErr) {
			return nil, err
		}
		switch oauthErr.Code {
		case OAuthErrAuthorizationPending:
		case OAuthErrSlowDown:
			// The cap may never speed the poll up: an IdP is allowed to ask for an
			// interval above MaxInterval, and slow_down must not undo it.
			interval = max(interval, min(interval+slowDownStep, m.opts.MaxInterval))
		default:
			return nil, err
		}
	}
}
