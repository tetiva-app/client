package websocket

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
)

func connectWithPing(t *testing.T, conn *fakeConn, sink *fakeSink, interval time.Duration) (*usecase, ConnectionID) {
	t.Helper()
	uc := newTestUsecase(
		&fakeDialer{conn: conn, info: DialInfo{StatusCode: 101}},
		&fakeResolver{dial: ResolvedDial{URL: "ws://x", PingInterval: interval}},
		sink, &fakeHistory{},
	)
	connID := uuid.New()
	if _, err := uc.Connect(context.Background(), ConnectOpt{ConnectionID: connID, RequestID: uuid.New(), WorkspaceID: uuid.New()}); err != nil {
		t.Fatalf("Connect: %v", err)
	}
	return uc, connID
}

func TestPingLoopPingsAtInterval(t *testing.T) {
	conn := newFakeConn()
	uc, connID := connectWithPing(t, conn, &fakeSink{}, 20*time.Millisecond)
	defer uc.closeEntry(connID, nil)

	deadline := time.Now().Add(300 * time.Millisecond)
	for time.Now().Before(deadline) && conn.pingCount() < 3 {
		time.Sleep(5 * time.Millisecond)
	}
	if got := conn.pingCount(); got < 3 {
		t.Fatalf("pings = %d, want at least 3", got)
	}
}

func TestPingDoesNotHoldTheWriteLock(t *testing.T) {
	release := make(chan struct{})
	started := make(chan struct{}, 1)
	conn := newFakeConn()
	conn.pingFn = func(_ context.Context) error {
		select {
		case started <- struct{}{}:
		default:
		}
		<-release
		return nil
	}
	uc, connID := connectWithPing(t, conn, &fakeSink{}, 10*time.Millisecond)
	defer func() {
		close(release)
		uc.closeEntry(connID, nil)
	}()

	<-started
	sent := make(chan error, 1)
	go func() {
		sent <- uc.Send(context.Background(), connID, OutgoingMessage{Type: MessageText, Data: []byte("x")})
	}()
	select {
	case err := <-sent:
		if err != nil {
			t.Fatalf("Send during a blocked Ping: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("Send blocked while a Ping was in flight")
	}
}

func TestPingTimeoutClosesConnection(t *testing.T) {
	old := pingTimeout
	pingTimeout = 30 * time.Millisecond
	defer func() { pingTimeout = old }()

	conn := newFakeConn()
	conn.pingFn = func(ctx context.Context) error {
		<-ctx.Done()
		return ctx.Err()
	}
	sink := &fakeSink{}
	uc, connID := connectWithPing(t, conn, sink, 20*time.Millisecond)

	waitFor(t, func() bool { return len(sink.snapshotSystems()) == 1 })
	if got := sink.snapshotSystems()[0]; got != "keepalive ping timed out" {
		t.Fatalf("system row = %q", got)
	}
	waitFor(t, func() bool { return sink.countState(StateClosed) == 1 })
	if n := sink.countState(StateError); n != 0 {
		t.Fatalf("StateError emitted %d times, want 0", n)
	}
	if uc.has(connID) {
		t.Fatal("expected the entry to be gone after a keepalive failure")
	}
	if !conn.isClosed() {
		t.Fatal("expected the socket to be closed")
	}
}

func TestPingErrorWhileLiveReportsOnce(t *testing.T) {
	conn := newFakeConn()
	conn.pingFn = func(_ context.Context) error { return errors.New("broken pipe") }
	sink := &fakeSink{}
	uc, connID := connectWithPing(t, conn, sink, 20*time.Millisecond)

	waitFor(t, func() bool { return sink.countState(StateClosed) == 1 })
	time.Sleep(80 * time.Millisecond) // let further ticks happen, if any
	if n := len(sink.snapshotSystems()); n != 1 {
		t.Fatalf("system rows = %d, want exactly 1", n)
	}
	if n := sink.countState(StateClosed); n != 1 {
		t.Fatalf("StateClosed emitted %d times, want 1", n)
	}
	if n := sink.countState(StateError); n != 0 {
		t.Fatalf("StateError emitted %d times, want 0", n)
	}
	if uc.has(connID) {
		t.Fatal("expected the entry to be gone")
	}
}

func TestPingCancelledByDisconnectStaysSilent(t *testing.T) {
	started := make(chan struct{}, 1)
	conn := newFakeConn()
	conn.pingFn = func(ctx context.Context) error {
		select {
		case started <- struct{}{}:
		default:
		}
		<-ctx.Done()
		return ctx.Err()
	}
	sink := &fakeSink{}
	uc, connID := connectWithPing(t, conn, sink, 10*time.Millisecond)

	<-started
	if err := uc.Disconnect(context.Background(), connID); err != nil {
		t.Fatalf("Disconnect: %v", err)
	}
	time.Sleep(80 * time.Millisecond)
	if rows := sink.snapshotSystems(); len(rows) != 0 {
		t.Fatalf("expected no system rows on a cancelled ping, got %v", rows)
	}
}

func TestPeerCloseStopsPingLoop(t *testing.T) {
	conn := newFakeConn()
	conn.readErr = ErrClosed
	sink := &fakeSink{}
	uc, connID := connectWithPing(t, conn, sink, 20*time.Millisecond)

	conn.readGate <- struct{}{} // drains the queue, so Read returns ErrClosed
	waitFor(t, func() bool { return !uc.has(connID) })

	pingsAtClose := conn.pingCount()
	time.Sleep(80 * time.Millisecond)
	if got := conn.pingCount(); got != pingsAtClose {
		t.Fatalf("pings continued after close: %d → %d", pingsAtClose, got)
	}
	if n := sink.countState(StateClosed); n != 1 {
		t.Fatalf("StateClosed emitted %d times, want 1", n)
	}
	if rows := sink.snapshotSystems(); len(rows) != 0 {
		t.Fatalf("expected no system rows on a peer close, got %v", rows)
	}
}
