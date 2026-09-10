package websocket

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/tetiva-app/client/internal/domain/entities"
)

// fakeConn feeds queued inbound frames, then an error to end readPump.
type fakeConn struct {
	mu       sync.Mutex
	inbound  []Message
	idx      int
	written  []Message
	pings    int
	closed   bool
	readGate chan struct{}                   // unblocks each Read call
	readErr  error                           // returned once the queue is drained
	pingFn   func(ctx context.Context) error // overrides the default instant pong
}

func newFakeConn(inbound ...Message) *fakeConn {
	return &fakeConn{inbound: inbound, readGate: make(chan struct{}, 16)}
}

func (c *fakeConn) Read(ctx context.Context) (Message, error) {
	select {
	case <-ctx.Done():
		return Message{}, ctx.Err()
	case <-c.readGate:
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.idx >= len(c.inbound) {
		if c.readErr != nil {
			return Message{}, c.readErr
		}
		return Message{}, errors.New("EOF")
	}
	m := c.inbound[c.idx]
	c.idx++
	return m, nil
}

func (c *fakeConn) Write(_ context.Context, m Message) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.written = append(c.written, m)
	return nil
}

func (c *fakeConn) Ping(ctx context.Context) error {
	c.mu.Lock()
	c.pings++
	fn := c.pingFn
	c.mu.Unlock()
	if fn != nil {
		return fn(ctx)
	}
	return nil
}

func (c *fakeConn) Close(_ int, _ string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.closed = true
	return nil
}

func (c *fakeConn) pingCount() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.pings
}

func (c *fakeConn) isClosed() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.closed
}

func (c *fakeConn) writtenFrames() []Message {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]Message(nil), c.written...)
}

type fakeDialer struct {
	mu        sync.Mutex
	conn      Conn
	info      DialInfo
	err       error
	params    DialParams
	gate      chan struct{} // when set, Dial blocks until it is closed or ctx ends
	ignoreCtx bool          // gate wins over cancellation: models a dial that lands late
	dialed    chan struct{} // closed on the first Dial call
}

func (d *fakeDialer) Dial(ctx context.Context, p DialParams) (Conn, DialInfo, error) {
	d.mu.Lock()
	d.params = p
	gate, ignoreCtx := d.gate, d.ignoreCtx
	if d.dialed != nil {
		select {
		case <-d.dialed:
		default:
			close(d.dialed)
		}
	}
	d.mu.Unlock()
	if gate != nil {
		if ignoreCtx {
			<-gate
		} else {
			select {
			case <-gate:
			case <-ctx.Done():
				return nil, DialInfo{}, ctx.Err()
			}
		}
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.conn, d.info, d.err
}

func (d *fakeDialer) dialParams() DialParams {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.params
}

type fakeResolver struct {
	dial ResolvedDial
	err  error
	subs func(text string) string
}

func (r *fakeResolver) ResolveWebSocket(_ context.Context, _, _ uuid.UUID, _ string) (ResolvedDial, error) {
	return r.dial, r.err
}

func (r *fakeResolver) SubstituteMessage(_ context.Context, _ uuid.UUID, text string) (string, error) {
	if r.subs != nil {
		return r.subs(text), nil
	}
	return text, nil
}

type fakeSink struct {
	mu       sync.Mutex
	messages []InboundMessage
	states   []ConnState
	systems  []string
	onState  func(ConnState) // may run under the registry lock: a hook must not call back into the usecase
}

func (s *fakeSink) OnMessage(_ ConnectionID, m InboundMessage) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.messages = append(s.messages, m)
}

func (s *fakeSink) OnSystem(_ ConnectionID, text string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.systems = append(s.systems, text)
}

func (s *fakeSink) OnStateChange(_ ConnectionID, st ConnState, _ error) {
	s.mu.Lock()
	s.states = append(s.states, st)
	hook := s.onState
	s.mu.Unlock()
	if hook != nil {
		hook(st)
	}
}

func (s *fakeSink) snapshotMessages() []InboundMessage {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]InboundMessage(nil), s.messages...)
}

func (s *fakeSink) snapshotStates() []ConnState {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]ConnState(nil), s.states...)
}

func (s *fakeSink) snapshotSystems() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]string(nil), s.systems...)
}

func (s *fakeSink) countState(want ConnState) int {
	n := 0
	for _, st := range s.snapshotStates() {
		if st == want {
			n++
		}
	}
	return n
}

type fakeHistory struct {
	mu      sync.Mutex
	records []*entities.History
}

func (h *fakeHistory) Create(_ context.Context, rec *entities.History) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.records = append(h.records, rec)
	return nil
}

func (h *fakeHistory) all() []*entities.History {
	h.mu.Lock()
	defer h.mu.Unlock()
	return append([]*entities.History(nil), h.records...)
}

func newTestUsecase(d Dialer, r RequestResolver, s MessageSink, h HistoryRepository) *usecase {
	return &usecase{
		conns:    make(map[ConnectionID]*entry),
		dialer:   d,
		resolver: r,
		sink:     s,
		history:  h,
		now:      time.Now,
	}
}

// stepClock returns base, then base+step, so a handshake measured across two calls lasts exactly step.
func stepClock(base time.Time, step time.Duration) func() time.Time {
	var mu sync.Mutex
	first := true
	return func() time.Time {
		mu.Lock()
		defer mu.Unlock()
		if first {
			first = false
			return base
		}
		return base.Add(step)
	}
}

func (u *usecase) has(connID ConnectionID) bool {
	u.mu.Lock()
	defer u.mu.Unlock()
	_, ok := u.conns[connID]
	return ok
}

func (u *usecase) isLive(connID ConnectionID) bool {
	u.mu.Lock()
	defer u.mu.Unlock()
	e, ok := u.conns[connID]
	return ok && e.state == entryLive
}

func waitFor(t *testing.T, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("condition not met within deadline")
}

func TestConnectDialsAndPushesInbound(t *testing.T) {
	conn := newFakeConn(Message{Type: MessageText, Data: []byte("hello")})
	dialer := &fakeDialer{conn: conn, info: DialInfo{StatusCode: 101, Subprotocol: "chat"}}
	resolver := &fakeResolver{dial: ResolvedDial{URL: "ws://example/ws", Headers: map[string][]string{"X-A": {"1"}}}}
	sink := &fakeSink{}
	hist := &fakeHistory{}
	uc := newTestUsecase(dialer, resolver, sink, hist)

	connID := uuid.New()
	res, err := uc.Connect(context.Background(), ConnectOpt{ConnectionID: connID, RequestID: uuid.New(), WorkspaceID: uuid.New()})
	if err != nil {
		t.Fatalf("Connect: %v", err)
	}
	if !res.Connected || res.Status != 101 || res.Subprotocol != "chat" {
		t.Fatalf("result = %+v", res)
	}
	if dialer.dialParams().URL != "ws://example/ws" {
		t.Fatalf("dialer got URL %q", dialer.dialParams().URL)
	}

	conn.readGate <- struct{}{}

	waitFor(t, func() bool { return len(sink.snapshotMessages()) == 1 })
	got := sink.snapshotMessages()[0]
	if string(got.Data) != "hello" {
		t.Fatalf("inbound = %q, want hello", got.Data)
	}
	recs := hist.all()
	if len(recs) != 1 || recs[0].ResponseStatus != 101 {
		t.Fatalf("expected 1 history record with status 101, got %+v", recs)
	}
}

func TestConnectResolverErrorAborts(t *testing.T) {
	hist := &fakeHistory{}
	uc := newTestUsecase(&fakeDialer{}, &fakeResolver{err: errors.New("boom")}, &fakeSink{}, hist)
	connID := uuid.New()
	if _, err := uc.Connect(context.Background(), ConnectOpt{ConnectionID: connID, RequestID: uuid.New(), WorkspaceID: uuid.New()}); err == nil {
		t.Fatal("expected error when resolver fails")
	}
	if len(hist.all()) != 0 {
		t.Fatalf("expected no history on resolver failure, got %d", len(hist.all()))
	}
	if uc.has(connID) {
		t.Fatal("expected the reservation to be rolled back")
	}
}

func TestSendWritesToConn(t *testing.T) {
	conn := newFakeConn()
	uc := newTestUsecase(&fakeDialer{conn: conn}, &fakeResolver{dial: ResolvedDial{URL: "ws://x"}}, &fakeSink{}, &fakeHistory{})
	connID := uuid.New()
	if _, err := uc.Connect(context.Background(), ConnectOpt{ConnectionID: connID, RequestID: uuid.New(), WorkspaceID: uuid.New()}); err != nil {
		t.Fatalf("Connect: %v", err)
	}
	if err := uc.Send(context.Background(), connID, OutgoingMessage{Type: MessageText, Data: []byte("ping")}); err != nil {
		t.Fatalf("Send: %v", err)
	}
	written := conn.writtenFrames()
	if len(written) != 1 || string(written[0].Data) != "ping" {
		t.Fatalf("expected one written frame 'ping', got %+v", written)
	}
}

func TestSendSubstitutesTextButNotBinary(t *testing.T) {
	conn := newFakeConn()
	resolver := &fakeResolver{
		dial: ResolvedDial{URL: "ws://x"},
		subs: func(text string) string { return strings.ReplaceAll(text, "{{v}}", "resolved") },
	}
	uc := newTestUsecase(&fakeDialer{conn: conn}, resolver, &fakeSink{}, &fakeHistory{})
	connID := uuid.New()
	if _, err := uc.Connect(context.Background(), ConnectOpt{ConnectionID: connID, RequestID: uuid.New(), WorkspaceID: uuid.New()}); err != nil {
		t.Fatalf("Connect: %v", err)
	}
	if err := uc.Send(context.Background(), connID, OutgoingMessage{Type: MessageText, Data: []byte(`{"a":"{{v}}"}`)}); err != nil {
		t.Fatalf("Send text: %v", err)
	}
	if err := uc.Send(context.Background(), connID, OutgoingMessage{Type: MessageBinary, Data: []byte("{{v}}")}); err != nil {
		t.Fatalf("Send binary: %v", err)
	}
	written := conn.writtenFrames()
	if len(written) != 2 {
		t.Fatalf("expected 2 frames, got %d", len(written))
	}
	if string(written[0].Data) != `{"a":"resolved"}` {
		t.Fatalf("text frame = %q", written[0].Data)
	}
	if string(written[1].Data) != "{{v}}" {
		t.Fatalf("binary frame = %q, want it untouched", written[1].Data)
	}
}

func TestSendUnknownConnErrors(t *testing.T) {
	uc := newTestUsecase(&fakeDialer{}, &fakeResolver{}, &fakeSink{}, &fakeHistory{})
	if err := uc.Send(context.Background(), uuid.New(), OutgoingMessage{Data: []byte("x")}); err == nil {
		t.Fatal("expected error sending to unknown connection")
	}
}

func TestDisconnectClosesAndRemoves(t *testing.T) {
	conn := newFakeConn()
	uc := newTestUsecase(&fakeDialer{conn: conn}, &fakeResolver{dial: ResolvedDial{URL: "ws://x"}}, &fakeSink{}, &fakeHistory{})
	connID := uuid.New()
	if _, err := uc.Connect(context.Background(), ConnectOpt{ConnectionID: connID, RequestID: uuid.New(), WorkspaceID: uuid.New()}); err != nil {
		t.Fatalf("Connect: %v", err)
	}
	if err := uc.Disconnect(context.Background(), connID); err != nil {
		t.Fatalf("Disconnect: %v", err)
	}
	waitFor(t, func() bool { return conn.isClosed() })
	if err := uc.Send(context.Background(), connID, OutgoingMessage{Data: []byte("x")}); err == nil {
		t.Fatal("expected error sending after disconnect")
	}
}

func TestDisconnectAll(t *testing.T) {
	uc := newTestUsecase(&fakeDialer{conn: newFakeConn()}, &fakeResolver{dial: ResolvedDial{URL: "ws://x"}}, &fakeSink{}, &fakeHistory{})
	if _, err := uc.Connect(context.Background(), ConnectOpt{ConnectionID: uuid.New(), RequestID: uuid.New(), WorkspaceID: uuid.New()}); err != nil {
		t.Fatalf("Connect: %v", err)
	}
	if err := uc.DisconnectAll(context.Background()); err != nil {
		t.Fatalf("DisconnectAll: %v", err)
	}
	uc.mu.Lock()
	n := len(uc.conns)
	uc.mu.Unlock()
	if n != 0 {
		t.Fatalf("expected empty registry, got %d", n)
	}
}

// dialStep scripts one Dial outcome; a gate blocks the call, ignoring cancellation, so a dial can land late.
type dialStep struct {
	conn   Conn
	info   DialInfo
	err    error
	gate   chan struct{}
	dialed chan struct{}
}

// seqDialer hands out one outcome per Dial call, so two attempts on the same id run independently.
type seqDialer struct {
	mu    sync.Mutex
	steps []dialStep
	calls int
}

func (d *seqDialer) Dial(_ context.Context, _ DialParams) (Conn, DialInfo, error) {
	d.mu.Lock()
	i := d.calls
	if i >= len(d.steps) {
		i = len(d.steps) - 1
	}
	d.calls++
	step := d.steps[i]
	d.mu.Unlock()
	if step.dialed != nil {
		close(step.dialed)
	}
	if step.gate != nil {
		<-step.gate
	}
	return step.conn, step.info, step.err
}
