package auth

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/tetiva-app/client/internal/domain"
	"github.com/tetiva-app/client/internal/domain/entities"
)

// fakeClock advances only when a flow sleeps, so a five-minute deadline is a few
// microseconds of test time.
type fakeClock struct {
	mu    sync.Mutex
	t     time.Time
	slept []time.Duration
}

func newFakeClock() *fakeClock { return &fakeClock{t: testNow} }

func (c *fakeClock) now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()

	return c.t
}

func (c *fakeClock) advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.t = c.t.Add(d)
}

// sleep records what the flow asked for, advances the fake clock by it and waits
// a real millisecond, so a poll loop neither spins nor takes its real interval.
func (c *fakeClock) sleep(ctx context.Context, d time.Duration) error {
	c.mu.Lock()
	c.slept = append(c.slept, d)
	c.t = c.t.Add(d)
	c.mu.Unlock()

	timer := time.NewTimer(time.Millisecond)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func (c *fakeClock) sleeps() []time.Duration {
	c.mu.Lock()
	defer c.mu.Unlock()

	return append([]time.Duration(nil), c.slept...)
}

// seqReader is a deterministic stand-in for crypto/rand.Reader.
type seqReader struct {
	mu sync.Mutex
	n  byte
}

func (r *seqReader) Read(p []byte) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	for i := range p {
		r.n++
		p[i] = r.n
	}

	return len(p), nil
}

type sinkEvent struct {
	id    string
	state FlowState
	err   error
}

type recordSink struct {
	mu     sync.Mutex
	events []sinkEvent
}

func newRecordSink() *recordSink { return &recordSink{} }

func (s *recordSink) OnFlowState(flowID string, st FlowState, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.events = append(s.events, sinkEvent{id: flowID, state: st, err: err})
}

func (s *recordSink) lastOf(flowID string) (sinkEvent, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for i := len(s.events) - 1; i >= 0; i-- {
		if s.events[i].id == flowID {
			return s.events[i], true
		}
	}

	return sinkEvent{}, false
}

func (s *recordSink) statesOf(flowID string) []FlowState {
	s.mu.Lock()
	defer s.mu.Unlock()

	var out []FlowState
	for _, e := range s.events {
		if e.id == flowID {
			out = append(out, e.state)
		}
	}

	return out
}

// hookRepo stands between a flow and the store: it can block a Put with or
// without honouring the context, fail it, or run a competing writer inside it.
type hookRepo struct {
	*fakeRepo

	mu             sync.Mutex
	beforePut      func()
	putErr         error
	gate           chan struct{}
	cooperative    bool
	entered        chan struct{}
	reserveGate    chan struct{}
	reserveEntered chan struct{}
}

func newHookRepo() *hookRepo {
	return &hookRepo{
		fakeRepo:       newFakeRepo(),
		entered:        make(chan struct{}, 8),
		reserveEntered: make(chan struct{}, 8),
	}
}

// blockReserve holds the next Reserve, which is a start still setting itself up:
// no listener is bound and no flow is installed yet.
func (r *hookRepo) blockReserve() chan struct{} {
	gate := make(chan struct{})

	r.mu.Lock()
	r.reserveGate = gate
	r.mu.Unlock()

	return gate
}

func (r *hookRepo) Reserve(ctx context.Context, owner entities.AuthOwner) (int64, error) {
	r.mu.Lock()
	gate := r.reserveGate
	r.reserveGate = nil
	r.mu.Unlock()

	if gate != nil {
		select {
		case r.reserveEntered <- struct{}{}:
		default:
		}
		select {
		case <-gate:
		case <-ctx.Done():
			return 0, ctx.Err()
		}
	}

	return r.fakeRepo.Reserve(ctx, owner)
}

func (r *hookRepo) blockPut(cooperative bool) chan struct{} {
	gate := make(chan struct{})

	r.mu.Lock()
	r.gate, r.cooperative = gate, cooperative
	r.mu.Unlock()

	return gate
}

func (r *hookRepo) failPut(err error) {
	r.mu.Lock()
	r.putErr = err
	r.mu.Unlock()
}

func (r *hookRepo) onBeforePut(fn func()) {
	r.mu.Lock()
	r.beforePut = fn
	r.mu.Unlock()
}

func (r *hookRepo) Put(ctx context.Context, owner entities.AuthOwner, generation int64, hash string, t *Token) error {
	r.mu.Lock()
	before, gate, cooperative, putErr := r.beforePut, r.gate, r.cooperative, r.putErr
	r.beforePut = nil
	r.mu.Unlock()

	select {
	case r.entered <- struct{}{}:
	default:
	}
	if before != nil {
		before()
	}
	if gate != nil {
		if cooperative {
			select {
			case <-gate:
			case <-ctx.Done():
				return ctx.Err()
			}
		} else {
			<-gate
		}
	}
	if putErr != nil {
		return putErr
	}

	return r.fakeRepo.Put(ctx, owner, generation, hash, t)
}

// stuckTransport ignores the request context the way a wedged native stack
// would: the flow becomes a survivor instead of joining.
type stuckTransport struct {
	entered chan struct{}
	release chan struct{}
}

func newStuckTransport() *stuckTransport {
	return &stuckTransport{entered: make(chan struct{}, 4), release: make(chan struct{})}
}

func (t *stuckTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	select {
	case t.entered <- struct{}{}:
	default:
	}
	<-t.release

	return &http.Response{
		Status:     "200 OK",
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": {"application/json"}},
		Body:       io.NopCloser(strings.NewReader(`{"access_token":"late"}`)),
		Request:    req,
	}, nil
}

type idpReply struct {
	status int
	body   string
}

// flowIDP answers the device-authorization and token endpoints from a script and
// records every form it received.
type flowIDP struct {
	server *httptest.Server

	mu            sync.Mutex
	device        idpReply
	replies       []idpReply
	tokens        []url.Values
	devices       []url.Values
	deviceGate    chan struct{}
	deviceEntered chan struct{}
}

func newFlowIDP(t *testing.T) *flowIDP {
	t.Helper()

	s := &flowIDP{
		device:        idpReply{status: http.StatusOK, body: `{}`},
		deviceEntered: make(chan struct{}, 4),
	}
	s.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			t.Errorf("parse form: %v", err)
		}

		s.mu.Lock()
		reply := idpReply{status: http.StatusOK, body: `{"access_token":"at-flow","expires_in":3600}`}
		if r.URL.Path == "/device" {
			s.devices = append(s.devices, r.PostForm)
			reply = s.device
			gate := s.deviceGate
			s.deviceGate = nil
			if gate != nil {
				s.mu.Unlock()
				select {
				case s.deviceEntered <- struct{}{}:
				default:
				}
				select {
				case <-gate:
				case <-r.Context().Done():
					return
				}
				s.mu.Lock()
			}
		} else {
			s.tokens = append(s.tokens, r.PostForm)
			if len(s.replies) > 0 {
				reply = s.replies[0]
				if len(s.replies) > 1 {
					s.replies = s.replies[1:]
				}
			}
		}
		s.mu.Unlock()

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(reply.status)
		_, _ = io.WriteString(w, reply.body)
	}))
	t.Cleanup(s.server.Close)

	return s
}

func (s *flowIDP) tokenURL() string      { return s.server.URL + "/token" }
func (s *flowIDP) deviceAuthURL() string { return s.server.URL + "/device" }
func (s *flowIDP) authorizeURL() string  { return s.server.URL + "/authorize" }

// blockDevice holds the device-authorization round trip, which is the slow part
// of StartDevice: nothing is installed until it answers.
func (s *flowIDP) blockDevice() chan struct{} {
	gate := make(chan struct{})

	s.mu.Lock()
	s.deviceGate = gate
	s.mu.Unlock()

	return gate
}

func (s *flowIDP) setDevice(status int, body string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.device = idpReply{status: status, body: body}
}

func (s *flowIDP) script(replies ...idpReply) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.replies = replies
}

func (s *flowIDP) tokenForms() []url.Values {
	s.mu.Lock()
	defer s.mu.Unlock()

	return append([]url.Values(nil), s.tokens...)
}

func (s *flowIDP) deviceForms() []url.Values {
	s.mu.Lock()
	defer s.mu.Unlock()

	return append([]url.Values(nil), s.devices...)
}

func oauthErrorReply(code string) idpReply {
	return idpReply{status: http.StatusBadRequest, body: `{"error":"` + code + `"}`}
}

type flowFixture struct {
	t     *testing.T
	repo  *hookRepo
	sink  *recordSink
	clock *fakeClock
	idp   *flowIDP
	mgr   *manager
	opts  FlowOptions
}

func newFlowFixture(t *testing.T, tweak func(*FlowOptions)) *flowFixture {
	t.Helper()

	clock := newFakeClock()
	idp := newFlowIDP(t)
	repo := newHookRepo()
	sink := newRecordSink()
	opts := FlowOptions{
		Now:             clock.now,
		Sleep:           clock.sleep,
		Rand:            &seqReader{},
		CallbackTimeout: 3 * time.Second,
		JoinCap:         750 * time.Millisecond,
		DrainWindow:     100 * time.Millisecond,
	}
	if tweak != nil {
		tweak(&opts)
	}

	mgr, ok := NewFlowManager(repo, NewTokenHTTPClient(), sink, opts).(*manager)
	if !ok {
		t.Fatal("NewFlowManager returned an unexpected type")
	}
	t.Cleanup(func() { _ = mgr.Shutdown(context.Background()) })

	return &flowFixture{t: t, repo: repo, sink: sink, clock: clock, idp: idp, mgr: mgr, opts: mgr.opts}
}

func (f *flowFixture) codeConfig() OAuth2Config {
	return OAuth2Config{
		Grant: GrantAuthorizationCode, TokenURL: f.idp.tokenURL(), AuthURL: f.idp.authorizeURL(),
		ClientID: "app", ClientAuth: ClientAuthNone,
	}
}

func (f *flowFixture) deviceConfig() OAuth2Config {
	return OAuth2Config{
		Grant: GrantDeviceCode, TokenURL: f.idp.tokenURL(), DeviceAuthURL: f.idp.deviceAuthURL(),
		ClientID: "app", ClientAuth: ClientAuthNone,
	}
}

func (f *flowFixture) startCode(owner entities.AuthOwner, port string) (string, FlowInfo) {
	f.t.Helper()

	id := uuid.NewString()
	info, err := f.mgr.StartAuthCode(context.Background(), id, owner, f.codeConfig(), port)
	if err != nil {
		var invalid *domain.ValidationError
		if errors.As(err, &invalid) {
			f.t.Fatalf("StartAuthCode: %v", invalid.Fields)
		}
		f.t.Fatalf("StartAuthCode: %v", err)
	}

	return id, info
}

func (f *flowFixture) startCodeWith(t *testing.T, cfg OAuth2Config) (string, FlowInfo) {
	t.Helper()

	id := uuid.NewString()
	info, err := f.mgr.StartAuthCode(context.Background(), id, testOwner(), cfg, "0")
	if err != nil {
		t.Fatalf("StartAuthCode: %v", err)
	}

	return id, info
}

func (f *flowFixture) startDevice(owner entities.AuthOwner) (string, FlowInfo) {
	f.t.Helper()

	id := uuid.NewString()
	info, err := f.mgr.StartDevice(context.Background(), id, owner, f.deviceConfig())
	if err != nil {
		f.t.Fatalf("StartDevice: %v", err)
	}

	return id, info
}

func (f *flowFixture) waitState(flowID string, want FlowState) FlowStatus {
	f.t.Helper()

	var last FlowStatus
	waitFor(f.t, string(want)+" for flow "+flowID, func() bool {
		st, ok := f.mgr.Status(flowID)
		last = st

		return ok && st.State == want
	})

	return last
}

func (f *flowFixture) activeFlowID(owner entities.AuthOwner) string {
	f.mgr.mu.Lock()
	defer f.mgr.mu.Unlock()

	return f.mgr.active[ownerKey(owner)]
}

func (f *flowFixture) portHolder(port int) string {
	f.mgr.mu.Lock()
	defer f.mgr.mu.Unlock()

	return f.mgr.ports[port]
}

func waitFor(t *testing.T, what string, cond func() bool) {
	t.Helper()

	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(2 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %s", what)
}

func authorizeQuery(t *testing.T, authorizeURL string) url.Values {
	t.Helper()

	u, err := url.Parse(authorizeURL)
	if err != nil {
		t.Fatalf("parse authorize URL: %v", err)
	}

	return u.Query()
}

func deliverCallback(t *testing.T, info FlowInfo, extra url.Values) *http.Response {
	t.Helper()

	q := authorizeQuery(t, info.AuthorizeURL)
	extra.Set("state", q.Get("state"))
	resp := callbackGet(t, q.Get("redirect_uri")+"?"+extra.Encode())

	// The browser must get the whole page: the flow may close the listener the
	// moment it has the result, and a truncated response is a connection error.
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("reading the callback page: %v", err)
	}
	if resp.StatusCode == http.StatusOK && !strings.Contains(string(body), "You can close this tab") {
		t.Errorf("callback page = %q", body)
	}

	return resp
}

func fieldError(t *testing.T, err error, key string) string {
	t.Helper()

	var invalid *domain.ValidationError
	if !errors.As(err, &invalid) {
		t.Fatalf("error %v is not a validation error", err)
	}
	message, ok := invalid.Fields[key]
	if !ok {
		t.Fatalf("validation error has no %q field: %v", key, invalid.Fields)
	}

	return message
}

func freePort(t *testing.T) string {
	t.Helper()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	_ = ln.Close()

	return strconv.Itoa(port)
}

func assertPortFree(t *testing.T, port string) {
	t.Helper()

	waitFor(t, "port "+port+" to be free", func() bool {
		ln, err := net.Listen("tcp", net.JoinHostPort("127.0.0.1", port))
		if err != nil {
			return false
		}
		_ = ln.Close()

		return true
	})
}

func TestStartAuthCodeRejectsBadPreconditions(t *testing.T) {
	tests := []struct {
		name  string
		id    string
		cfg   func(f *flowFixture) OAuth2Config
		port  string
		field string
	}{
		{
			name: "not a uuid", id: "flow-1",
			cfg: func(f *flowFixture) OAuth2Config { return f.codeConfig() }, field: "flowId",
		},
		{
			name: "grant mismatch", id: uuid.NewString(),
			cfg: func(f *flowFixture) OAuth2Config { return f.deviceConfig() }, field: "grant",
		},
		{
			name: "missing authUrl", id: uuid.NewString(),
			cfg: func(f *flowFixture) OAuth2Config {
				cfg := f.codeConfig()
				cfg.AuthURL = ""

				return cfg
			},
			field: "authUrl",
		},
		{
			name: "unsupported clientAuth", id: uuid.NewString(),
			cfg: func(f *flowFixture) OAuth2Config {
				cfg := f.codeConfig()
				cfg.ClientAuth = "mutual-tls"

				return cfg
			},
			field: "clientAuth",
		},
		{
			name: "bad redirect port", id: uuid.NewString(),
			cfg:  func(f *flowFixture) OAuth2Config { return f.codeConfig() },
			port: "not-a-port", field: "redirectPort",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newFlowFixture(t, nil)
			port := tt.port
			if port == "" {
				port = freePort(t)
			}

			_, err := f.mgr.StartAuthCode(context.Background(), tt.id, testOwner(), tt.cfg(f), port)
			if err == nil {
				t.Fatal("StartAuthCode accepted an invalid start")
			}
			fieldError(t, err, tt.field)

			if reserves, puts, _ := f.repo.counts(); reserves != 0 || puts != 0 {
				t.Errorf("the store was touched before validation: %d reserves, %d puts", reserves, puts)
			}
			if tt.port == "" {
				assertPortFree(t, port)
			}
		})
	}
}

func TestStartRefusesAFlowIDThatIsAlreadyUsed(t *testing.T) {
	f := newFlowFixture(t, nil)
	owner := testOwner()

	id, _ := f.startCode(owner, "0")

	_, err := f.mgr.StartAuthCode(context.Background(), id, owner, f.codeConfig(), "0")
	if got := fieldError(t, err, "flowId"); got != "already used" {
		t.Errorf("active id: %q", got)
	}

	if err = f.mgr.Cancel(id); err != nil {
		t.Fatalf("Cancel: %v", err)
	}
	_, err = f.mgr.StartAuthCode(context.Background(), id, owner, f.codeConfig(), "0")
	if got := fieldError(t, err, "flowId"); got != "already used" {
		t.Errorf("retained id: %q", got)
	}
}

func TestAuthCodeFlowStoresTheToken(t *testing.T) {
	f := newFlowFixture(t, nil)
	owner := testOwner()
	cfg := f.codeConfig()
	cfg.Scope = "read write"
	cfg.Audience = "api://x"

	id := uuid.NewString()
	info, err := f.mgr.StartAuthCode(context.Background(), id, owner, cfg, "0")
	if err != nil {
		t.Fatalf("StartAuthCode: %v", err)
	}

	q := authorizeQuery(t, info.AuthorizeURL)
	want := map[string]string{
		"response_type": "code", "client_id": "app", "state": q.Get("state"),
		"code_challenge_method": "S256", "scope": "read write", "audience": "api://x",
	}
	for key, value := range want {
		if q.Get(key) != value {
			t.Errorf("authorize %s = %q, want %q", key, q.Get(key), value)
		}
	}
	redirectURI := q.Get("redirect_uri")
	if !strings.HasPrefix(redirectURI, "http://127.0.0.1:") || !strings.HasSuffix(redirectURI, "/callback") {
		t.Fatalf("redirect_uri = %q", redirectURI)
	}
	if !info.ExpiresAt.Equal(testNow.Add(f.opts.CallbackTimeout)) {
		t.Errorf("ExpiresAt = %v", info.ExpiresAt)
	}

	if resp := deliverCallback(t, info, url.Values{"code": {"the-code"}}); resp.StatusCode != http.StatusOK {
		t.Fatalf("callback status = %d", resp.StatusCode)
	}

	status := f.waitState(id, FlowDone)
	if status.Info.AuthorizeURL != info.AuthorizeURL {
		t.Error("the terminal status lost its info")
	}

	forms := f.idp.tokenForms()
	if len(forms) != 1 {
		t.Fatalf("token endpoint hits: %d", len(forms))
	}
	assertForm(t, forms[0], url.Values{
		"grant_type":    {GrantAuthorizationCode},
		"code":          {"the-code"},
		"redirect_uri":  {redirectURI},
		"code_verifier": {forms[0].Get("code_verifier")},
		"client_id":     {"app"},
	})

	sum := sha256.Sum256([]byte(forms[0].Get("code_verifier")))
	if got := base64.RawURLEncoding.EncodeToString(sum[:]); got != q.Get("code_challenge") {
		t.Errorf("code_challenge does not match the verifier: %q vs %q", got, q.Get("code_challenge"))
	}

	row := f.repo.row(owner)
	if row == nil || row.AccessToken != "at-flow" || row.ConfigHash != ConfigHash(cfg) {
		t.Fatalf("stored row: %+v", row)
	}
	if states := f.sink.statesOf(id); len(states) != 1 || states[0] != FlowDone {
		t.Errorf("sink states: %v", states)
	}

	port := strings.TrimSuffix(strings.TrimPrefix(redirectURI, "http://127.0.0.1:"), "/callback")
	assertPortFree(t, port)
}

func TestAuthorizeURLKeepsTheConfiguredQuery(t *testing.T) {
	f := newFlowFixture(t, nil)
	cfg := f.codeConfig()
	cfg.AuthURL = f.idp.authorizeURL() + "?prompt=consent&response_type=token"

	_, info := f.startCodeWith(t, cfg)

	q := authorizeQuery(t, info.AuthorizeURL)
	if q.Get("prompt") != "consent" {
		t.Errorf("the configured query was dropped: %v", q)
	}
	if q.Get("response_type") != "code" {
		t.Errorf("response_type = %q, want the flow's value to win", q.Get("response_type"))
	}
}

func TestAuthCodeWrongStateLeavesTheFlowPending(t *testing.T) {
	f := newFlowFixture(t, nil)
	owner := testOwner()
	id, info := f.startCode(owner, "0")

	q := authorizeQuery(t, info.AuthorizeURL)
	bad := url.Values{"code": {"the-code"}, "state": {"forged"}}
	if resp := callbackGet(t, q.Get("redirect_uri")+"?"+bad.Encode()); resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("forged state: status = %d", resp.StatusCode)
	}
	if status, _ := f.mgr.Status(id); status.State != FlowPending {
		t.Fatalf("state after a forged callback: %s", status.State)
	}

	if resp := deliverCallback(t, info, url.Values{"code": {"the-code"}}); resp.StatusCode != http.StatusOK {
		t.Fatalf("callback status = %d", resp.StatusCode)
	}
	f.waitState(id, FlowDone)
}

func TestAuthCodeErrorRedirectIsTerminal(t *testing.T) {
	f := newFlowFixture(t, nil)
	owner := testOwner()
	id, info := f.startCode(owner, "0")

	deliverCallback(t, info, url.Values{"error": {"access_denied"}, "error_description": {"user said no"}})

	status := f.waitState(id, FlowError)
	if !strings.Contains(status.Err.Error(), "access_denied") {
		t.Errorf("error = %v", status.Err)
	}
	event, ok := f.sink.lastOf(id)
	if !ok || event.state != FlowError || event.err == nil {
		t.Errorf("sink event = %+v", event)
	}
	if _, puts, _ := f.repo.counts(); puts != 0 {
		t.Errorf("a denied flow wrote %d times", puts)
	}
}

func TestAuthCodeTimesOutWhenTheBrowserNeverReturns(t *testing.T) {
	f := newFlowFixture(t, func(o *FlowOptions) { o.CallbackTimeout = 60 * time.Millisecond })
	id, _ := f.startCode(testOwner(), "0")

	status := f.waitState(id, FlowError)
	if !strings.Contains(status.Err.Error(), "did not come back") {
		t.Errorf("error = %v", status.Err)
	}
}

func TestZeroPortGivesEachFlowItsOwnPort(t *testing.T) {
	f := newFlowFixture(t, nil)

	_, first := f.startCode(testOwner(), "0")
	_, second := f.startCode(testOwner(), "0")

	one := authorizeQuery(t, first.AuthorizeURL).Get("redirect_uri")
	two := authorizeQuery(t, second.AuthorizeURL).Get("redirect_uri")
	if one == two {
		t.Errorf("both flows bound %s", one)
	}
}

func TestBindConflictWithAForeignProcess(t *testing.T) {
	f := newFlowFixture(t, nil)
	port := freePort(t)
	ln, err := net.Listen("tcp", net.JoinHostPort("127.0.0.1", port))
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer func() { _ = ln.Close() }()

	_, err = f.mgr.StartAuthCode(context.Background(), uuid.NewString(), testOwner(), f.codeConfig(), port)
	message := fieldError(t, err, "redirectPort")
	if !strings.Contains(message, "another application is using it") {
		t.Errorf("message = %q", message)
	}
}

func TestBindConflictWithAnotherOwnersLiveFlow(t *testing.T) {
	f := newFlowFixture(t, nil)
	port := freePort(t)
	first, _ := f.startCode(testOwner(), port)

	_, err := f.mgr.StartAuthCode(context.Background(), uuid.NewString(), testOwner(), f.codeConfig(), port)
	message := fieldError(t, err, "redirectPort")
	if !strings.Contains(message, "another request is waiting for its browser callback") {
		t.Errorf("message = %q", message)
	}
	if status, ok := f.mgr.Status(first); !ok || status.State != FlowPending {
		t.Errorf("the first owner's flow was disturbed: %+v", status)
	}
}

func TestSameOwnerRebindsItsOwnPort(t *testing.T) {
	f := newFlowFixture(t, nil)
	owner := testOwner()
	port := freePort(t)
	first, _ := f.startCode(owner, port)

	second, info := f.startCode(owner, port)
	if got := authorizeQuery(t, info.AuthorizeURL).Get("redirect_uri"); !strings.Contains(got, ":"+port+"/") {
		t.Fatalf("the replacement did not take the port back: %q", got)
	}
	if status, _ := f.mgr.Status(first); status.State != FlowCancelled {
		t.Errorf("the replaced flow is %s", status.State)
	}
	if got := f.activeFlowID(owner); got != second {
		t.Errorf("active flow = %q, want the replacement", got)
	}
}

func TestSameOwnerRebindsItsOwnPortWhileThePreviousFlowDrains(t *testing.T) {
	// The predecessor is committing, so the takeover does not close its listener:
	// the bind has to wait out the drain instead of retrying straight away.
	f := newFlowFixture(t, func(o *FlowOptions) { o.DrainWindow = 300 * time.Millisecond })
	owner := testOwner()
	port := freePort(t)
	gate := f.repo.blockPut(true)
	first, info := f.startCode(owner, port)

	deliverCallback(t, info, url.Values{"code": {"the-code"}})
	<-f.repo.entered

	started := make(chan error, 1)
	go func() {
		_, err := f.mgr.StartAuthCode(context.Background(), uuid.NewString(), owner, f.codeConfig(), port)
		started <- err
	}()
	time.Sleep(20 * time.Millisecond)
	close(gate)

	if err := <-started; err != nil {
		t.Fatalf("the replacement did not get its own port back: %v", err)
	}
	if status, _ := f.mgr.Status(first); status.State != FlowDone {
		t.Errorf("the committing flow reports %s", status.State)
	}
}

func TestCancelBeforeInstallLeavesNoListenerBehind(t *testing.T) {
	f := newFlowFixture(t, nil)
	port := freePort(t)
	// Released after the cancel: a manager that lost it would then go on to bind
	// the port and install the flow, which is exactly what this asserts against.
	gate := f.repo.blockReserve()
	release := sync.OnceFunc(func() { close(gate) })
	defer release()

	id := uuid.NewString()
	started := make(chan error, 1)
	go func() {
		_, err := f.mgr.StartAuthCode(context.Background(), id, testOwner(), f.codeConfig(), port)
		started <- err
	}()

	<-f.repo.reserveEntered
	if err := f.mgr.Cancel(id); err != nil {
		t.Fatalf("Cancel: %v", err)
	}
	release()

	if err := <-started; err == nil {
		t.Fatal("StartAuthCode installed a flow that was cancelled while it was starting")
	}
	if active := f.activeFlowID(testOwner()); active != "" {
		t.Errorf("owner still has the active flow %q", active)
	}
	if holder := f.portHolder(portNumber(t, port)); holder != "" {
		t.Errorf("port %s is still held by %q", port, holder)
	}
	if _, puts, _ := f.repo.counts(); puts != 0 {
		t.Errorf("puts = %d, want none", puts)
	}
	assertPortFree(t, port)
}

func TestStatusReportsStartingUntilTheFlowIsInstalled(t *testing.T) {
	f := newFlowFixture(t, nil)
	gate := f.repo.blockReserve()
	release := sync.OnceFunc(func() { close(gate) })
	defer release()

	id := uuid.NewString()
	started := make(chan error, 1)
	go func() {
		_, err := f.mgr.StartAuthCode(context.Background(), id, testOwner(), f.codeConfig(), "0")
		started <- err
	}()

	<-f.repo.reserveEntered
	status, ok := f.mgr.Status(id)
	if !ok {
		t.Fatal("Status does not know the reserved flow")
	}
	if status.State != FlowStarting {
		t.Errorf("state = %s, want %s", status.State, FlowStarting)
	}
	if status.Info.AuthorizeURL != "" {
		t.Errorf("a flow still starting reported the authorize URL %q", status.Info.AuthorizeURL)
	}

	release()
	if err := <-started; err != nil {
		t.Fatalf("StartAuthCode: %v", err)
	}

	if status, _ = f.mgr.Status(id); status.State != FlowPending {
		t.Errorf("state after install = %s, want %s", status.State, FlowPending)
	}
	if status.Info.AuthorizeURL == "" {
		t.Error("the installed flow reports no authorize URL")
	}
}

func TestCancelBeforeInstallStopsTheDeviceStart(t *testing.T) {
	f := newFlowFixture(t, nil)
	f.idp.setDevice(http.StatusOK, `{"device_code":"dc","user_code":"UC","verification_uri":"https://idp/device","interval":1,"expires_in":300}`)
	gate := f.idp.blockDevice()
	release := sync.OnceFunc(func() { close(gate) })
	defer release()

	id := uuid.NewString()
	started := make(chan error, 1)
	go func() {
		_, err := f.mgr.StartDevice(context.Background(), id, testOwner(), f.deviceConfig())
		started <- err
	}()

	<-f.idp.deviceEntered
	if err := f.mgr.Cancel(id); err != nil {
		t.Fatalf("Cancel: %v", err)
	}
	release()

	if err := <-started; err == nil {
		t.Fatal("StartDevice installed a flow that was cancelled while it was starting")
	}
	if active := f.activeFlowID(testOwner()); active != "" {
		t.Errorf("owner still has the active flow %q", active)
	}
	if _, puts, _ := f.repo.counts(); puts != 0 {
		t.Errorf("puts = %d, want none", puts)
	}
	st, ok := f.mgr.Status(id)
	if !ok || st.State != FlowCancelled {
		t.Errorf("Status = %v/%v, want a retained cancelled flow", st.State, ok)
	}
	// The poller would keep asking the IdP for a flow nobody is watching.
	f.clock.advance(2 * time.Second)
	if forms := f.idp.tokenForms(); len(forms) != 0 {
		t.Errorf("the cancelled flow polled the token endpoint %d times", len(forms))
	}
}

func portNumber(t *testing.T, port string) int {
	t.Helper()

	n, err := strconv.Atoi(port)
	if err != nil {
		t.Fatalf("port %q: %v", port, err)
	}

	return n
}

func TestCancelDuringCommitJoinsWithoutFreezingTheManager(t *testing.T) {
	// A JoinCap far above any scheduling noise: a manager that froze would block
	// the calls below for all of it, and a healthy one answers immediately.
	f := newFlowFixture(t, func(o *FlowOptions) { o.JoinCap = 3 * time.Second })
	owner := testOwner()
	gate := f.repo.blockPut(true)
	id, info := f.startCode(owner, "0")

	deliverCallback(t, info, url.Values{"code": {"the-code"}})
	<-f.repo.entered

	cancelled := make(chan error, 1)
	go func() { cancelled <- f.mgr.Cancel(id) }()
	// Without this the assertions below could run before Cancel ever took mu, and
	// would pass against a manager that waits inside the lock.
	time.Sleep(20 * time.Millisecond)

	// The join waits outside the lock, so every other caller stays responsive.
	start := time.Now()
	if err := f.mgr.Cancel(uuid.NewString()); err != nil {
		t.Errorf("Cancel(unknown): %v", err)
	}
	if _, ok := f.mgr.Status(id); !ok {
		t.Error("Status lost the flow")
	}
	other := testOwner()
	if _, err := f.mgr.StartAuthCode(context.Background(), uuid.NewString(), other, f.codeConfig(), "0"); err != nil {
		t.Errorf("a start for another owner was blocked: %v", err)
	}
	if elapsed := time.Since(start); elapsed > f.opts.JoinCap/2 {
		t.Errorf("the manager was blocked for %s while a cancel waited", elapsed)
	}

	close(gate)
	select {
	case err := <-cancelled:
		if err != nil {
			t.Errorf("Cancel: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Cancel never returned")
	}

	if status, _ := f.mgr.Status(id); status.State != FlowDone {
		t.Errorf("a committed flow reports %s", status.State)
	}
}

func TestSurvivingTransportNeverWritesAToken(t *testing.T) {
	transport := newStuckTransport()
	f := newFlowFixture(t, nil)
	f.mgr.client = &http.Client{Transport: transport}
	owner := testOwner()

	id, info := f.startCode(owner, "0")
	deliverCallback(t, info, url.Values{"code": {"the-code"}})
	<-transport.entered

	if err := f.mgr.Cancel(id); err == nil {
		t.Fatal("Cancel reported a join it did not get")
	}
	if status, _ := f.mgr.Status(id); status.State != FlowCancelled {
		t.Errorf("state = %s", status.State)
	}

	close(transport.release)
	waitFor(t, "the survivor to exit", func() bool {
		f.mgr.mu.Lock()
		defer f.mgr.mu.Unlock()

		select {
		case <-f.mgr.flows[id].done:
			return true
		default:
			return false
		}
	})
	if _, puts, _ := f.repo.counts(); puts != 0 {
		t.Errorf("the survivor wrote %d times", puts)
	}
}

func TestReplacementIsRefusedWhileThePreviousFlowCommits(t *testing.T) {
	f := newFlowFixture(t, nil)
	owner := testOwner()
	gate := f.repo.blockPut(false)
	first, info := f.startCode(owner, "0")

	deliverCallback(t, info, url.Values{"code": {"the-code"}})
	<-f.repo.entered

	newID := uuid.NewString()
	_, err := f.mgr.StartAuthCode(context.Background(), newID, owner, f.codeConfig(), "0")
	message := fieldError(t, err, "auth")
	if !strings.Contains(message, "still finishing") {
		t.Errorf("message = %q", message)
	}
	if _, ok := f.mgr.Status(newID); ok {
		t.Error("the refused start installed a flow anyway")
	}
	if got := f.activeFlowID(owner); got != first {
		t.Errorf("active flow = %q, want the committing one", got)
	}

	close(gate)
	f.waitState(first, FlowDone)
	if _, puts, _ := f.repo.counts(); puts != 1 {
		t.Errorf("puts = %d, want the committing flow's single write", puts)
	}
}

func TestCancelledMidExchangeIsJoinedWithoutWriting(t *testing.T) {
	f := newFlowFixture(t, nil)
	blocked, stop := make(chan struct{}), make(chan struct{})
	slow := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// The body must be drained before the server watches for a disconnect.
		_ = r.ParseForm()
		close(blocked)
		select {
		case <-r.Context().Done():
		case <-stop:
		}
	}))
	t.Cleanup(slow.Close)
	t.Cleanup(func() { close(stop) })

	cfg := f.codeConfig()
	cfg.TokenURL = slow.URL + "/token"
	owner := testOwner()
	id := uuid.NewString()
	info, err := f.mgr.StartAuthCode(context.Background(), id, owner, cfg, "0")
	if err != nil {
		t.Fatalf("StartAuthCode: %v", err)
	}

	deliverCallback(t, info, url.Values{"code": {"the-code"}})
	<-blocked

	if err = f.mgr.Cancel(id); err != nil {
		t.Fatalf("Cancel: %v", err)
	}
	if status, _ := f.mgr.Status(id); status.State != FlowCancelled {
		t.Errorf("state = %s", status.State)
	}
	if _, puts, _ := f.repo.counts(); puts != 0 {
		t.Errorf("puts = %d, want none", puts)
	}
}

func TestCancelDoesNotDeleteAReplacementFlow(t *testing.T) {
	f := newFlowFixture(t, func(o *FlowOptions) { o.JoinCap = 3 * time.Second })
	owner := testOwner()
	gate := f.repo.blockPut(true)
	first, info := f.startCode(owner, "0")

	deliverCallback(t, info, url.Values{"code": {"the-code"}})
	<-f.repo.entered

	cancelled := make(chan error, 1)
	go func() { cancelled <- f.mgr.Cancel(first) }()
	time.Sleep(20 * time.Millisecond)

	newID := uuid.NewString()
	started := make(chan error, 1)
	go func() {
		_, startErr := f.mgr.StartAuthCode(context.Background(), newID, owner, f.codeConfig(), "0")
		started <- startErr
	}()
	time.Sleep(20 * time.Millisecond)
	close(gate)

	if err := <-cancelled; err != nil {
		t.Fatalf("Cancel: %v", err)
	}
	if err := <-started; err != nil {
		t.Fatalf("the replacement failed: %v", err)
	}

	if got := f.activeFlowID(owner); got != newID {
		t.Errorf("active flow = %q, want the replacement %q", got, newID)
	}
	status, ok := f.mgr.Status(newID)
	if !ok || status.State != FlowPending {
		t.Fatalf("the replacement is %+v", status)
	}
	port := authorizeQuery(t, status.Info.AuthorizeURL).Get("redirect_uri")
	number := strings.TrimSuffix(strings.TrimPrefix(port, "http://127.0.0.1:"), "/callback")
	parsed, err := strconv.Atoi(number)
	if err != nil {
		t.Fatalf("port %q: %v", number, err)
	}
	if got := f.portHolder(parsed); got != newID {
		t.Errorf("port %d is registered to %q, want the replacement", parsed, got)
	}
}

func TestTwoReplacementsInstallExactlyOne(t *testing.T) {
	f := newFlowFixture(t, func(o *FlowOptions) { o.JoinCap = 3 * time.Second })
	owner := testOwner()
	gate := f.repo.blockPut(true)
	_, info := f.startCode(owner, "0")

	deliverCallback(t, info, url.Values{"code": {"the-code"}})
	<-f.repo.entered

	type outcome struct {
		id  string
		err error
	}
	results := make(chan outcome, 2)
	for range 2 {
		go func() {
			id := uuid.NewString()
			_, err := f.mgr.StartAuthCode(context.Background(), id, owner, f.codeConfig(), "0")
			results <- outcome{id: id, err: err}
		}()
	}
	time.Sleep(20 * time.Millisecond)
	close(gate)

	installed, refused := 0, 0
	for range 2 {
		got := <-results
		switch {
		case got.err == nil:
			installed++
			if f.activeFlowID(owner) != got.id {
				t.Errorf("flow %s reported success but is not the active one", got.id)
			}
		case strings.Contains(fieldError(t, got.err, "auth"), "another token request started meanwhile"):
			refused++
			if _, ok := f.mgr.Status(got.id); ok {
				t.Errorf("the refused start installed %s", got.id)
			}
		default:
			t.Errorf("unexpected error: %v", got.err)
		}
	}
	if installed != 1 || refused != 1 {
		t.Errorf("installed = %d, refused = %d", installed, refused)
	}
}

func TestJoinCapOutlivesTheCommitBudget(t *testing.T) {
	defaults := FlowOptions{}.withDefaults()
	// A spent attempt, the backoff, a second attempt and the adoption re-read.
	if budget := 3*commitAttempt + commitBackoff; defaults.JoinCap <= budget {
		t.Fatalf("JoinCap %s must outlast the commit budget %s", defaults.JoinCap, budget)
	}
}

func TestASlowStoreStillJoinsWithinJoinCap(t *testing.T) {
	f := newFlowFixture(t, func(o *FlowOptions) { o.JoinCap = defaultJoinCap })
	owner := testOwner()
	// Blocks until its own attempt context expires, so commitToken retries once.
	f.repo.blockPut(true)
	id, info := f.startCode(owner, "0")

	deliverCallback(t, info, url.Values{"code": {"the-code"}})
	<-f.repo.entered

	start := time.Now()
	if err := f.mgr.Cancel(id); err != nil {
		t.Fatalf("a cooperative flow was reported as a survivor after %s: %v", time.Since(start), err)
	}
	if elapsed := time.Since(start); elapsed >= defaultJoinCap {
		t.Errorf("the join took %s", elapsed)
	}
}

func TestFlowAdoptsAConcurrentWinnersToken(t *testing.T) {
	f := newFlowFixture(t, nil)
	owner := testOwner()
	cfg := f.codeConfig()
	hash := ConfigHash(cfg)

	f.repo.onBeforePut(func() {
		generation, err := f.repo.fakeRepo.Reserve(context.Background(), owner)
		if err != nil {
			t.Errorf("Reserve: %v", err)
		}
		winner := &Token{AccessToken: "winner", ExpiresAt: testNow.Add(time.Hour)}
		if err = f.repo.fakeRepo.Put(context.Background(), owner, generation, hash, winner); err != nil {
			t.Errorf("competing Put: %v", err)
		}
	})

	id := uuid.NewString()
	info, err := f.mgr.StartAuthCode(context.Background(), id, owner, cfg, "0")
	if err != nil {
		t.Fatalf("StartAuthCode: %v", err)
	}
	deliverCallback(t, info, url.Values{"code": {"the-code"}})

	f.waitState(id, FlowDone)
	if row := f.repo.row(owner); row == nil || row.AccessToken != "winner" {
		t.Errorf("stored row: %+v", row)
	}
}

func TestFlowReportsAStoreThatMovedUnderIt(t *testing.T) {
	tests := []struct {
		name  string
		setup func(f *flowFixture, owner entities.AuthOwner)
		want  string
	}{
		{
			name: "cleared during the flow",
			setup: func(f *flowFixture, owner entities.AuthOwner) {
				f.repo.onBeforePut(func() {
					if err := f.repo.fakeRepo.Clear(context.Background(), owner); err != nil {
						f.t.Errorf("Clear: %v", err)
					}
				})
			},
			want: "the token was cleared while the flow was running",
		},
		{
			// The SQLite store reports a gone owner from Reserve, not from Put, so
			// this covers the mapping rather than a deletion mid-flow.
			name:  "the store reports the owner is gone",
			setup: func(f *flowFixture, _ entities.AuthOwner) { f.repo.failPut(ErrOwnerGone) },
			want:  "the request or collection was deleted",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newFlowFixture(t, nil)
			owner := testOwner()
			tt.setup(f, owner)

			id, info := f.startCode(owner, "0")
			deliverCallback(t, info, url.Values{"code": {"the-code"}})

			status := f.waitState(id, FlowError)
			if status.Err.Error() != tt.want {
				t.Errorf("error = %q, want %q", status.Err, tt.want)
			}
		})
	}
}

func TestStatusForgetsATerminalFlowAfterItsRetention(t *testing.T) {
	f := newFlowFixture(t, nil)
	id, info := f.startCode(testOwner(), "0")

	deliverCallback(t, info, url.Values{"code": {"the-code"}})
	status := f.waitState(id, FlowDone)
	if status.Info.ID != id {
		t.Errorf("terminal status lost its info: %+v", status.Info)
	}

	f.clock.advance(f.opts.StatusRetention + time.Second)
	if _, ok := f.mgr.Status(id); ok {
		t.Error("a terminal flow outlived its retention")
	}
}

func TestShutdownJoinsCooperativeFlows(t *testing.T) {
	f := newFlowFixture(t, nil)
	code, _ := f.startCode(testOwner(), "0")

	f.idp.setDevice(http.StatusOK, `{"device_code":"dc","user_code":"UC","verification_uri":"https://idp.example/device","interval":5,"expires_in":300}`)
	f.idp.script(oauthErrorReply(OAuthErrAuthorizationPending))
	device, _ := f.startDevice(testOwner())

	if err := f.mgr.Shutdown(context.Background()); err != nil {
		t.Fatalf("Shutdown: %v", err)
	}
	for _, id := range []string{code, device} {
		if status, _ := f.mgr.Status(id); status.State != FlowCancelled {
			t.Errorf("flow %s is %s", id, status.State)
		}
	}

	_, err := f.mgr.StartAuthCode(context.Background(), uuid.NewString(), testOwner(), f.codeConfig(), "0")
	if !errors.Is(err, ErrManagerClosed) {
		t.Errorf("start after shutdown: %v", err)
	}
	_, err = f.mgr.StartDevice(context.Background(), uuid.NewString(), testOwner(), f.deviceConfig())
	if !errors.Is(err, ErrManagerClosed) {
		t.Errorf("device start after shutdown: %v", err)
	}
}

func TestShutdownReportsSurvivors(t *testing.T) {
	transport := newStuckTransport()
	f := newFlowFixture(t, nil)
	f.mgr.client = &http.Client{Transport: transport}

	id, info := f.startCode(testOwner(), "0")
	deliverCallback(t, info, url.Values{"code": {"the-code"}})
	<-transport.entered

	first := f.mgr.Shutdown(context.Background())
	if first == nil || !strings.Contains(first.Error(), "1 flow(s) did not stop") {
		t.Fatalf("Shutdown: %v", first)
	}
	if second := f.mgr.Shutdown(context.Background()); second == nil || second.Error() != first.Error() {
		t.Errorf("second Shutdown: %v", second)
	}

	close(transport.release)
	waitFor(t, "the survivor to exit", func() bool {
		f.mgr.mu.Lock()
		defer f.mgr.mu.Unlock()

		select {
		case <-f.mgr.flows[id].done:
			return true
		default:
			return false
		}
	})
	if err := f.mgr.Shutdown(context.Background()); err != nil {
		t.Errorf("third Shutdown: %v", err)
	}
	if _, puts, _ := f.repo.counts(); puts != 0 {
		t.Errorf("the survivor wrote %d times", puts)
	}
}

func TestFlowLosesTheRaceToAProviderRefresh(t *testing.T) {
	f := newFlowFixture(t, nil)
	f.idp.script(
		idpReply{status: http.StatusOK, body: `{"access_token":"at-exchange","expires_in":3600}`},
		idpReply{status: http.StatusOK, body: `{"access_token":"at-refresh","expires_in":3600}`},
	)
	owner := testOwner()
	cfg := f.codeConfig()
	f.repo.seed(owner, ConfigHash(cfg), Token{
		AccessToken: "old", RefreshToken: "rt-old", ExpiresAt: testNow.Add(-time.Minute),
	})

	provider := NewProvider(f.repo, NewTokenHTTPClient(), f.clock.now)
	f.repo.onBeforePut(func() {
		if _, err := provider.Fetch(context.Background(), owner, cfg); err != nil {
			t.Errorf("competing Fetch: %v", err)
		}
	})

	id := uuid.NewString()
	info, err := f.mgr.StartAuthCode(context.Background(), id, owner, cfg, "0")
	if err != nil {
		t.Fatalf("StartAuthCode: %v", err)
	}
	deliverCallback(t, info, url.Values{"code": {"the-code"}})

	f.waitState(id, FlowDone)
	row := f.repo.row(owner)
	if row == nil || row.AccessToken != "at-refresh" {
		t.Fatalf("stored row: %+v, want the winner's token", row)
	}
}
