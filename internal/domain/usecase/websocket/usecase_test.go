package websocket

import (
	"context"
	"errors"
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
	closed   bool
	readGate chan struct{} // unblocks each Read call
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
		return Message{}, errors.New("EOF")
	}
	m := c.inbound[c.idx]
	c.idx++
	return m, nil
}

func (c *fakeConn) Write(ctx context.Context, m Message) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.written = append(c.written, m)
	return nil
}

func (c *fakeConn) Close(code int, reason string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.closed = true
	return nil
}

type fakeDialer struct {
	conn   Conn
	err    error
	params DialParams
}

func (d *fakeDialer) Dial(ctx context.Context, p DialParams) (Conn, error) {
	d.params = p
	return d.conn, d.err
}

type fakeResolver struct {
	url     string
	headers map[string][]string
	err     error
}

func (r *fakeResolver) ResolveWebSocket(_ context.Context, _, _ uuid.UUID, _ string) (string, map[string][]string, error) {
	return r.url, r.headers, r.err
}

type fakeSink struct {
	mu       sync.Mutex
	messages []InboundMessage
	states   []ConnState
}

func (s *fakeSink) OnMessage(_ ConnectionID, m InboundMessage) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.messages = append(s.messages, m)
}

func (s *fakeSink) OnStateChange(_ ConnectionID, st ConnState, _ error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.states = append(s.states, st)
}

func (s *fakeSink) snapshotMessages() []InboundMessage {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]InboundMessage(nil), s.messages...)
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

func newTestUsecase(d Dialer, r RequestResolver, s MessageSink, h HistoryRepository) *usecase {
	return &usecase{
		conns:    make(map[ConnectionID]*activeConn),
		dialer:   d,
		resolver: r,
		sink:     s,
		history:  h,
		newID:    func() ConnectionID { return uuid.New() },
	}
}

func TestConnectDialsAndPushesInbound(t *testing.T) {
	conn := newFakeConn(Message{Type: MessageText, Data: []byte("hello")})
	dialer := &fakeDialer{conn: conn}
	resolver := &fakeResolver{url: "ws://example/ws", headers: map[string][]string{"X-A": {"1"}}}
	sink := &fakeSink{}
	hist := &fakeHistory{}
	uc := newTestUsecase(dialer, resolver, sink, hist)

	connID, err := uc.Connect(context.Background(), ConnectOpt{RequestID: uuid.New(), WorkspaceID: uuid.New()})
	if err != nil {
		t.Fatalf("Connect: %v", err)
	}
	if connID == uuid.Nil {
		t.Fatal("expected non-nil connID")
	}
	if dialer.params.URL != "ws://example/ws" {
		t.Fatalf("dialer got URL %q", dialer.params.URL)
	}

	conn.readGate <- struct{}{}

	waitFor(t, func() bool { return len(sink.snapshotMessages()) == 1 })
	got := sink.snapshotMessages()[0]
	if string(got.Data) != "hello" {
		t.Fatalf("inbound = %q, want hello", got.Data)
	}
	if len(hist.records) != 1 {
		t.Fatalf("expected 1 history record, got %d", len(hist.records))
	}
}

func TestConnectResolverErrorAborts(t *testing.T) {
	hist := &fakeHistory{}
	uc := newTestUsecase(&fakeDialer{}, &fakeResolver{err: errors.New("boom")}, &fakeSink{}, hist)
	if _, err := uc.Connect(context.Background(), ConnectOpt{RequestID: uuid.New(), WorkspaceID: uuid.New()}); err == nil {
		t.Fatal("expected error when resolver fails")
	}
	if len(hist.records) != 0 {
		t.Fatalf("expected no history on resolver failure, got %d", len(hist.records))
	}
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

func TestSendWritesToConn(t *testing.T) {
	conn := newFakeConn()
	uc := newTestUsecase(&fakeDialer{conn: conn}, &fakeResolver{url: "ws://x"}, &fakeSink{}, &fakeHistory{})
	connID, err := uc.Connect(context.Background(), ConnectOpt{RequestID: uuid.New(), WorkspaceID: uuid.New()})
	if err != nil {
		t.Fatalf("Connect: %v", err)
	}
	if err := uc.Send(context.Background(), connID, OutgoingMessage{Type: MessageText, Data: []byte("ping")}); err != nil {
		t.Fatalf("Send: %v", err)
	}
	conn.mu.Lock()
	defer conn.mu.Unlock()
	if len(conn.written) != 1 || string(conn.written[0].Data) != "ping" {
		t.Fatalf("expected one written frame 'ping', got %+v", conn.written)
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
	uc := newTestUsecase(&fakeDialer{conn: conn}, &fakeResolver{url: "ws://x"}, &fakeSink{}, &fakeHistory{})
	connID, _ := uc.Connect(context.Background(), ConnectOpt{RequestID: uuid.New(), WorkspaceID: uuid.New()})
	if err := uc.Disconnect(context.Background(), connID); err != nil {
		t.Fatalf("Disconnect: %v", err)
	}
	waitFor(t, func() bool {
		conn.mu.Lock()
		defer conn.mu.Unlock()
		return conn.closed
	})
	if err := uc.Send(context.Background(), connID, OutgoingMessage{Data: []byte("x")}); err == nil {
		t.Fatal("expected error sending after disconnect")
	}
}

func TestDisconnectAll(t *testing.T) {
	uc := newTestUsecase(&fakeDialer{conn: newFakeConn()}, &fakeResolver{url: "ws://x"}, &fakeSink{}, &fakeHistory{})
	_, _ = uc.Connect(context.Background(), ConnectOpt{RequestID: uuid.New(), WorkspaceID: uuid.New()})
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
