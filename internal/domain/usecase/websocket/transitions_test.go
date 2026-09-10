package websocket

import (
	"context"
	"errors"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestConcurrentDisconnectNeverReportsConnectedAfterClose(t *testing.T) {
	for i := 0; i < 300; i++ {
		conn := newFakeConn()
		sink := &fakeSink{}
		uc := newTestUsecase(
			&fakeDialer{conn: conn, info: DialInfo{StatusCode: 101}},
			&fakeResolver{dial: ResolvedDial{URL: "ws://x"}},
			sink, &fakeHistory{},
		)
		connID := uuid.New()
		var wg sync.WaitGroup
		wg.Add(1)
		go func() {
			defer wg.Done()
			// Disconnect the instant the entry goes live: the promotion has just
			// released the registry lock, which is where the transition can tear.
			for !uc.isLive(connID) {
				runtime.Gosched()
			}
			_ = uc.Disconnect(context.Background(), connID)
		}()

		res, err := uc.Connect(context.Background(), ConnectOpt{ConnectionID: connID, RequestID: uuid.New(), WorkspaceID: uuid.New()})
		if err != nil {
			t.Fatalf("Connect: %v", err)
		}
		wg.Wait()
		waitFor(t, func() bool { return !uc.has(connID) })

		states := sink.snapshotStates()
		if res.Connected {
			if len(states) == 0 || states[0] != StateConnected {
				t.Fatalf("iteration %d: reported connected but states = %v", i, states)
			}
			continue
		}
		for _, st := range states {
			if st == StateConnected {
				t.Fatalf("iteration %d: emitted connected for a cancelled attempt: %v", i, states)
			}
		}
	}
}

func TestPeerCloseRacingPingLoopEmitsOneTerminalState(t *testing.T) {
	releasePing := make(chan struct{})
	conn := newFakeConn()
	conn.readErr = ErrClosed
	conn.pingFn = func(_ context.Context) error {
		<-releasePing
		return errors.New("broken pipe")
	}

	sink := &fakeSink{}
	var once sync.Once
	sink.onState = func(st ConnState) {
		if st != StateClosed && st != StateError {
			return
		}
		// Let the in-flight ping fail exactly while the terminal state is being reported.
		once.Do(func() {
			close(releasePing)
			time.Sleep(50 * time.Millisecond)
		})
	}

	uc, connID := connectWithPing(t, conn, sink, 5*time.Millisecond)
	conn.readGate <- struct{}{}
	waitFor(t, func() bool { return !uc.has(connID) })
	time.Sleep(50 * time.Millisecond)

	if rows := sink.snapshotSystems(); len(rows) != 0 {
		t.Fatalf("expected no keepalive rows on a peer close, got %v", rows)
	}
	if n := sink.countState(StateClosed) + sink.countState(StateError); n != 1 {
		t.Fatalf("terminal states = %d, want exactly 1 (%v)", n, sink.snapshotStates())
	}
}

func TestDisconnectRacingPeerCloseEmitsNoClosingAfterClosed(t *testing.T) {
	conn := newFakeConn()
	conn.readErr = ErrClosed
	sink := &fakeSink{}
	uc := newTestUsecase(
		&fakeDialer{conn: conn, info: DialInfo{StatusCode: 101}},
		&fakeResolver{dial: ResolvedDial{URL: "ws://x"}},
		sink, &fakeHistory{},
	)
	connID := uuid.New()

	var once sync.Once
	sink.onState = func(st ConnState) {
		if st != StateClosed {
			return
		}
		once.Do(func() { _ = uc.Disconnect(context.Background(), connID) })
	}

	if _, err := uc.Connect(context.Background(), ConnectOpt{ConnectionID: connID, RequestID: uuid.New(), WorkspaceID: uuid.New()}); err != nil {
		t.Fatalf("Connect: %v", err)
	}
	conn.readGate <- struct{}{}
	waitFor(t, func() bool { return !uc.has(connID) })
	time.Sleep(50 * time.Millisecond)

	// The disconnect that lost the race says nothing: no closing after closed.
	states := sink.snapshotStates()
	want := []ConnState{StateConnected, StateClosed}
	if len(states) != len(want) || states[0] != want[0] || states[1] != want[1] {
		t.Fatalf("states = %v, want %v", states, want)
	}
}
