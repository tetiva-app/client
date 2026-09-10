package websocket

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/tetiva-app/client/internal/domain"
	"github.com/tetiva-app/client/internal/domain/entities"
)

func TestConnectRejectsEmptyConnectionID(t *testing.T) {
	uc := newTestUsecase(&fakeDialer{conn: newFakeConn()}, &fakeResolver{dial: ResolvedDial{URL: "ws://x"}}, &fakeSink{}, &fakeHistory{})
	_, err := uc.Connect(context.Background(), ConnectOpt{RequestID: uuid.New(), WorkspaceID: uuid.New()})
	var valErr *domain.ValidationError
	if !errors.As(err, &valErr) {
		t.Fatalf("expected validation error, got %v", err)
	}
}

func TestConnectDuplicateConnectionIDRejectedAndIDReusable(t *testing.T) {
	uc := newTestUsecase(&fakeDialer{conn: newFakeConn()}, &fakeResolver{dial: ResolvedDial{URL: "ws://x"}}, &fakeSink{}, &fakeHistory{})
	connID := uuid.New()
	opt := ConnectOpt{ConnectionID: connID, RequestID: uuid.New(), WorkspaceID: uuid.New()}
	if _, err := uc.Connect(context.Background(), opt); err != nil {
		t.Fatalf("first Connect: %v", err)
	}
	var valErr *domain.ValidationError
	if _, err := uc.Connect(context.Background(), opt); !errors.As(err, &valErr) {
		t.Fatalf("expected validation error on duplicate id, got %v", err)
	}
	uc.closeEntry(connID, nil)
	if _, err := uc.Connect(context.Background(), opt); err != nil {
		t.Fatalf("id should be reusable after the entry is finalized: %v", err)
	}
}

func TestDisconnectDuringSlowDialCancelsAttempt(t *testing.T) {
	dialer := &fakeDialer{conn: newFakeConn(), gate: make(chan struct{}), dialed: make(chan struct{})}
	uc := newTestUsecase(dialer, &fakeResolver{dial: ResolvedDial{URL: "ws://x"}}, &fakeSink{}, &fakeHistory{})
	connID := uuid.New()

	done := make(chan ConnectResult, 1)
	go func() {
		res, _ := uc.Connect(context.Background(), ConnectOpt{ConnectionID: connID, RequestID: uuid.New(), WorkspaceID: uuid.New()})
		done <- res
	}()

	<-dialer.dialed
	if err := uc.Disconnect(context.Background(), connID); err != nil {
		t.Fatalf("Disconnect: %v", err)
	}

	res := <-done
	if res.Connected {
		t.Fatalf("expected a cancelled attempt, got %+v", res)
	}
	if uc.has(connID) {
		t.Fatal("expected the entry to be gone")
	}
}

func TestLateDialResultIsClosedAndNeverRegistered(t *testing.T) {
	conn := newFakeConn()
	dialer := &fakeDialer{conn: conn, gate: make(chan struct{}), dialed: make(chan struct{}), ignoreCtx: true}
	uc := newTestUsecase(dialer, &fakeResolver{dial: ResolvedDial{URL: "ws://x"}}, &fakeSink{}, &fakeHistory{})
	connID := uuid.New()

	done := make(chan ConnectResult, 1)
	go func() {
		res, _ := uc.Connect(context.Background(), ConnectOpt{ConnectionID: connID, RequestID: uuid.New(), WorkspaceID: uuid.New()})
		done <- res
	}()

	<-dialer.dialed
	_ = uc.Disconnect(context.Background(), connID)
	close(dialer.gate) // the handshake lands after the cancellation

	res := <-done
	if res.Connected {
		t.Fatalf("expected the late dial not to be reported as connected: %+v", res)
	}
	if !conn.isClosed() {
		t.Fatal("expected the late connection to be closed")
	}
	if uc.has(connID) {
		t.Fatal("expected no registry entry for a cancelled attempt")
	}
}

func TestConnectResolverFailedAfterScriptKeepsScript(t *testing.T) {
	script := &entities.ScriptResult{PreConsole: []string{"hello from script"}}
	resolver := &fakeResolver{dial: ResolvedDial{URL: "http://x", Failed: "url must start with ws:// or wss://", Script: script}}
	hist := &fakeHistory{}
	uc := newTestUsecase(&fakeDialer{}, resolver, &fakeSink{}, hist)
	connID := uuid.New()

	res, err := uc.Connect(context.Background(), ConnectOpt{ConnectionID: connID, RequestID: uuid.New(), WorkspaceID: uuid.New()})
	if err != nil {
		t.Fatalf("expected no Go error, got %v", err)
	}
	if res.Connected || res.Error == "" || res.Script != script {
		t.Fatalf("result = %+v", res)
	}
	if uc.has(connID) {
		t.Fatal("expected the entry to be gone")
	}
	if len(hist.all()) != 0 {
		t.Fatalf("expected no history when the dial never happened, got %d", len(hist.all()))
	}
}

func TestConnectRejectedHandshakeRecordsHistory(t *testing.T) {
	dialer := &fakeDialer{
		err:  errors.New("expected handshake response status 101 but got 401"),
		info: DialInfo{StatusCode: 401, ResponseHeaders: map[string][]string{"Www-Authenticate": {"Basic"}}},
	}
	resolver := &fakeResolver{dial: ResolvedDial{URL: "ws://x/ws", Headers: map[string][]string{"X-A": {"1"}}}}
	hist := &fakeHistory{}
	uc := newTestUsecase(dialer, resolver, &fakeSink{}, hist)
	uc.now = stepClock(time.Now(), 42*time.Millisecond)
	connID := uuid.New()

	res, err := uc.Connect(context.Background(), ConnectOpt{ConnectionID: connID, RequestID: uuid.New(), WorkspaceID: uuid.New()})
	if err != nil {
		t.Fatalf("expected no Go error, got %v", err)
	}
	if res.Connected || res.Status != 401 || res.Error == "" {
		t.Fatalf("result = %+v", res)
	}
	recs := hist.all()
	if len(recs) != 1 {
		t.Fatalf("expected 1 history record, got %d", len(recs))
	}
	rec := recs[0]
	if rec.ResponseStatus != 401 {
		t.Fatalf("history status = %d", rec.ResponseStatus)
	}
	if rec.RequestHeaders["X-A"][0] != "1" {
		t.Fatalf("history request headers = %+v", rec.RequestHeaders)
	}
	if rec.ResponseHeaders["Www-Authenticate"][0] != "Basic" {
		t.Fatalf("history response headers = %+v", rec.ResponseHeaders)
	}
	if rec.ErrorMessage == "" {
		t.Fatal("expected an error message on the history row")
	}
	if rec.DurationMs != 42 {
		t.Fatalf("DurationMs = %d, want 42", rec.DurationMs)
	}
	if uc.has(connID) {
		t.Fatal("expected the entry to be gone after a failed handshake")
	}
}

func TestSendOnPendingConnectionErrors(t *testing.T) {
	dialer := &fakeDialer{conn: newFakeConn(), gate: make(chan struct{}), dialed: make(chan struct{})}
	uc := newTestUsecase(dialer, &fakeResolver{dial: ResolvedDial{URL: "ws://x"}}, &fakeSink{}, &fakeHistory{})
	connID := uuid.New()

	done := make(chan struct{})
	go func() {
		defer close(done)
		_, _ = uc.Connect(context.Background(), ConnectOpt{ConnectionID: connID, RequestID: uuid.New(), WorkspaceID: uuid.New()})
	}()

	<-dialer.dialed
	if err := uc.Send(context.Background(), connID, OutgoingMessage{Type: MessageText, Data: []byte("x")}); err == nil {
		t.Fatal("expected an error sending on a pending connection")
	}
	close(dialer.gate)
	<-done
}

func TestDisconnectAllFinalizesPendingAttempt(t *testing.T) {
	dialer := &fakeDialer{conn: newFakeConn(), gate: make(chan struct{}), dialed: make(chan struct{})}
	uc := newTestUsecase(dialer, &fakeResolver{dial: ResolvedDial{URL: "ws://x"}}, &fakeSink{}, &fakeHistory{})
	connID := uuid.New()

	done := make(chan struct{})
	go func() {
		defer close(done)
		_, _ = uc.Connect(context.Background(), ConnectOpt{ConnectionID: connID, RequestID: uuid.New(), WorkspaceID: uuid.New()})
	}()

	<-dialer.dialed
	if err := uc.DisconnectAll(context.Background()); err != nil {
		t.Fatalf("DisconnectAll: %v", err)
	}
	<-done // the cancelled dial unblocks the attempt
	if uc.has(connID) {
		t.Fatal("expected an empty registry")
	}
}

func TestDisconnectEmitsClosingThenClosed(t *testing.T) {
	conn := newFakeConn()
	sink := &fakeSink{}
	uc := newTestUsecase(&fakeDialer{conn: conn}, &fakeResolver{dial: ResolvedDial{URL: "ws://x"}}, sink, &fakeHistory{})
	connID := uuid.New()
	if _, err := uc.Connect(context.Background(), ConnectOpt{ConnectionID: connID, RequestID: uuid.New(), WorkspaceID: uuid.New()}); err != nil {
		t.Fatalf("Connect: %v", err)
	}
	if err := uc.Disconnect(context.Background(), connID); err != nil {
		t.Fatalf("Disconnect: %v", err)
	}
	waitFor(t, func() bool { return len(sink.snapshotStates()) == 3 })
	states := sink.snapshotStates()
	want := []ConnState{StateConnected, StateClosing, StateClosed}
	for i, st := range want {
		if states[i] != st {
			t.Fatalf("states = %v, want %v", states, want)
		}
	}
}

func TestLateDialRollbackSparesTheConnectionThatReusedTheID(t *testing.T) {
	late, live := newFakeConn(), newFakeConn()
	dialer := &seqDialer{steps: []dialStep{
		{conn: late, info: DialInfo{StatusCode: 101}, gate: make(chan struct{}), dialed: make(chan struct{})},
		{conn: live, info: DialInfo{StatusCode: 101}, dialed: make(chan struct{})},
	}}
	uc := newTestUsecase(dialer, &fakeResolver{dial: ResolvedDial{URL: "ws://x"}}, &fakeSink{}, &fakeHistory{})
	connID := uuid.New()
	opt := ConnectOpt{ConnectionID: connID, RequestID: uuid.New(), WorkspaceID: uuid.New()}

	first := make(chan ConnectResult, 1)
	go func() {
		res, _ := uc.Connect(context.Background(), opt)
		first <- res
	}()
	<-dialer.steps[0].dialed
	_ = uc.Disconnect(context.Background(), connID)

	res, err := uc.Connect(context.Background(), opt)
	if err != nil || !res.Connected {
		t.Fatalf("second Connect on the reused id: res=%+v err=%v", res, err)
	}

	close(dialer.steps[0].gate) // the first handshake lands after it lost the slot
	if r := <-first; r.Connected {
		t.Fatalf("expected the late attempt to report failure, got %+v", r)
	}
	if !late.isClosed() {
		t.Fatal("expected the late socket to be closed")
	}
	if !uc.has(connID) {
		t.Fatal("the late attempt's rollback unregistered the live connection")
	}
	if live.isClosed() {
		t.Fatal("the late attempt's rollback closed the live socket")
	}
	if err := uc.Send(context.Background(), connID, OutgoingMessage{Data: []byte("x")}); err != nil {
		t.Fatalf("the live connection should still accept sends: %v", err)
	}
}

func TestConnectRecordsAuthQueryKeysInHistory(t *testing.T) {
	resolver := &fakeResolver{dial: ResolvedDial{URL: "ws://x/ws?tok=abc", AuthQueryKeys: []string{"tok"}}}
	hist := &fakeHistory{}
	uc := newTestUsecase(&fakeDialer{conn: newFakeConn()}, resolver, &fakeSink{}, hist)

	if _, err := uc.Connect(context.Background(), ConnectOpt{ConnectionID: uuid.New(), RequestID: uuid.New(), WorkspaceID: uuid.New()}); err != nil {
		t.Fatalf("Connect: %v", err)
	}

	recs := hist.all()
	if len(recs) != 1 {
		t.Fatalf("history records = %d, want 1", len(recs))
	}
	if got := recs[0].AuthQueryKeys; len(got) != 1 || got[0] != "tok" {
		t.Errorf("AuthQueryKeys = %v, want [tok]", got)
	}
}
