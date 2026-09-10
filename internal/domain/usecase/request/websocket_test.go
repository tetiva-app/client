package request_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/tetiva-app/client/internal/domain/entities"
	"github.com/tetiva-app/client/internal/domain/usecase/request"
)

// wsDeps overrides the stubs a websocket resolution test cares about.
type wsDeps struct {
	vars     map[string]string
	engine   request.ScriptEngine
	preHook  string
	persist  request.VariablePersister
	resolver request.AuthResolver
}

type capturingPersister struct {
	calls       int
	workspaceID uuid.UUID
	userID      string
	vars        map[string]string
	err         error
}

func (p *capturingPersister) PersistVariableChanges(_ context.Context, workspaceID uuid.UUID, userID string, vars map[string]string) error {
	p.calls++
	p.workspaceID = workspaceID
	p.userID = userID
	p.vars = vars
	return p.err
}

// failingAuthResolver makes the auth stage fail without touching collections.
type failingAuthResolver struct{}

func (failingAuthResolver) ResolveAuth(_ context.Context, _ *entities.Request) (request.ResolvedAuth, error) {
	return request.ResolvedAuth{}, errors.New("auth backend down")
}

func newWSUsecase(t *testing.T, repo *mockRepo, d wsDeps) request.Usecase {
	t.Helper()
	var engine request.ScriptEngine = &noopScriptEngine{}
	if d.engine != nil {
		engine = d.engine
	}
	var scripts request.ScriptResolver = &noopScriptResolver{}
	if d.preHook != "" {
		scripts = &scriptResolverWithPre{pre: d.preHook}
	}
	var persist request.VariablePersister = &noopVarPersister{}
	if d.persist != nil {
		persist = d.persist
	}
	var auth request.AuthResolver = request.NewAuthResolver(fixtureCollections())
	if d.resolver != nil {
		auth = d.resolver
	}
	return request.NewUsecase(repo, &mockHistoryRepo{}, &mockRequester{}, nil, nil,
		&mockEnvResolver{vars: d.vars}, engine, scripts, persist, auth, nil, nil, nil, nil)
}

// newTestUsecaseWithEnv keeps the plain wiring used by the older websocket tests.
func newTestUsecaseWithEnv(t *testing.T, repo *mockRepo, vars map[string]string) request.Usecase {
	t.Helper()
	return newWSUsecase(t, repo, wsDeps{vars: vars})
}

func wsRequest(id uuid.UUID, url string) *entities.Request {
	return &entities.Request{
		ID:           id,
		CollectionID: testCollectionID,
		Protocol:     entities.ProtocolWebSocket,
		URL:          url,
		AuthType:     entities.AuthTypeNone,
		AuthData:     "{}",
	}
}

func TestResolveWebSocketSubstitutesVarsAndAuth(t *testing.T) {
	reqID := uuid.New()
	repo := newMockRepo()
	r := wsRequest(reqID, "wss://{{host}}/ws")
	r.Headers = []entities.HeaderItem{{Key: "X-Token", Value: "{{tok}}", Enabled: true}}
	repo.requests[reqID] = r

	uc := newTestUsecaseWithEnv(t, repo, map[string]string{"host": "example.com", "tok": "secret"})

	dial, err := uc.ResolveWebSocket(context.Background(), reqID, testWorkspaceID, "user-1")
	if err != nil {
		t.Fatalf("ResolveWebSocket: %v", err)
	}
	if dial.URL != "wss://example.com/ws" {
		t.Fatalf("url = %q", dial.URL)
	}
	if got := dial.Headers["X-Token"]; len(got) != 1 || got[0] != "secret" {
		t.Fatalf("header X-Token = %v", got)
	}
	if dial.Failed != "" {
		t.Fatalf("unexpected Failed: %q", dial.Failed)
	}
	if dial.Script != nil {
		t.Fatalf("no script configured, got %+v", dial.Script)
	}
}

func TestResolveWebSocketRejectsNonWSScheme(t *testing.T) {
	reqID := uuid.New()
	repo := newMockRepo()
	repo.requests[reqID] = wsRequest(reqID, "https://example.com/ws")
	uc := newTestUsecaseWithEnv(t, repo, nil)

	dial, err := uc.ResolveWebSocket(context.Background(), reqID, testWorkspaceID, "")
	if err != nil {
		t.Fatalf("scheme guard must not be a Go error: %v", err)
	}
	if dial.Failed == "" {
		t.Fatal("expected Failed for a non-ws scheme")
	}
}

func TestResolveWebSocketRejectsNonWebSocketProtocol(t *testing.T) {
	reqID := uuid.New()
	repo := newMockRepo()
	r := wsRequest(reqID, "ws://example.com/ws")
	r.Protocol = entities.ProtocolHTTP
	repo.requests[reqID] = r
	uc := newTestUsecaseWithEnv(t, repo, nil)

	if _, err := uc.ResolveWebSocket(context.Background(), reqID, testWorkspaceID, ""); err == nil {
		t.Fatal("expected validation error for non-websocket protocol")
	}
}

func TestResolveWebSocketNotFound(t *testing.T) {
	uc := newTestUsecaseWithEnv(t, newMockRepo(), nil)
	if _, err := uc.ResolveWebSocket(context.Background(), uuid.New(), uuid.New(), ""); err == nil {
		t.Fatal("expected error for a missing request")
	}
}

func TestResolveWebSocketRunsPreScript(t *testing.T) {
	reqID := uuid.New()
	repo := newMockRepo()
	r := wsRequest(reqID, "wss://example.com/ws")
	r.Headers = []entities.HeaderItem{{Key: "X-Keep", Value: "1", Enabled: true}}
	repo.requests[reqID] = r

	engine := &captureScriptEngine{
		preHeaders: map[string][]string{"X-Token": {"{{tok}}"}},
		preConsole: []string{"connecting"},
	}
	uc := newWSUsecase(t, repo, wsDeps{vars: map[string]string{"tok": "secret"}, engine: engine, preHook: "// pre"})

	dial, err := uc.ResolveWebSocket(context.Background(), reqID, testWorkspaceID, "user-1")
	if err != nil {
		t.Fatalf("ResolveWebSocket: %v", err)
	}
	if dial.Script == nil || len(dial.Script.PreConsole) != 1 || dial.Script.PreConsole[0] != "connecting" {
		t.Fatalf("script outcome = %+v", dial.Script)
	}
	if got := engine.seenHeaders["X-Keep"]; len(got) != 1 || got[0] != "1" {
		t.Fatalf("script saw headers %v", engine.seenHeaders)
	}
	if got := dial.Headers["X-Token"]; len(got) != 1 || got[0] != "secret" {
		t.Fatalf("X-Token = %v, want [secret]", got)
	}
}

func TestResolveWebSocketRecomputesURLWhenScriptSetsVariable(t *testing.T) {
	reqID := uuid.New()
	repo := newMockRepo()
	repo.requests[reqID] = wsRequest(reqID, "wss://{{host}}/ws")

	engine := &captureScriptEngine{preVars: map[string]string{"host": "new.example.com"}}
	uc := newWSUsecase(t, repo, wsDeps{vars: map[string]string{"host": "old.example.com"}, engine: engine, preHook: "// pre"})

	dial, err := uc.ResolveWebSocket(context.Background(), reqID, testWorkspaceID, "user-1")
	if err != nil {
		t.Fatalf("ResolveWebSocket: %v", err)
	}
	if dial.URL != "wss://new.example.com/ws" {
		t.Fatalf("url = %q", dial.URL)
	}
}

func TestResolveWebSocketPersistsChangedVariables(t *testing.T) {
	reqID := uuid.New()
	wsID := testWorkspaceID
	repo := newMockRepo()
	repo.requests[reqID] = wsRequest(reqID, "wss://example.com/ws")

	persister := &capturingPersister{}
	engine := &captureScriptEngine{preVars: map[string]string{"tok": "fresh"}}
	uc := newWSUsecase(t, repo, wsDeps{engine: engine, preHook: "// pre", persist: persister})

	if _, err := uc.ResolveWebSocket(context.Background(), reqID, wsID, "user-1"); err != nil {
		t.Fatalf("ResolveWebSocket: %v", err)
	}
	if persister.calls != 1 {
		t.Fatalf("persist calls = %d", persister.calls)
	}
	if persister.userID != "user-1" || persister.workspaceID != wsID {
		t.Fatalf("persisted for %q / %s", persister.userID, persister.workspaceID)
	}
	if persister.vars["tok"] != "fresh" {
		t.Fatalf("persisted vars = %v", persister.vars)
	}
}

func TestResolveWebSocketPersistFailureIsReported(t *testing.T) {
	reqID := uuid.New()
	repo := newMockRepo()
	repo.requests[reqID] = wsRequest(reqID, "wss://example.com/ws")

	persister := &capturingPersister{err: errors.New("disk full")}
	engine := &captureScriptEngine{preVars: map[string]string{"tok": "fresh"}}
	uc := newWSUsecase(t, repo, wsDeps{engine: engine, preHook: "// pre", persist: persister})

	dial, err := uc.ResolveWebSocket(context.Background(), reqID, testWorkspaceID, "user-1")
	if err != nil {
		t.Fatalf("persist failure must not be a Go error: %v", err)
	}
	if dial.Failed != "" {
		t.Fatalf("Failed = %q, want empty", dial.Failed)
	}
	if dial.Script == nil || len(dial.Script.Errors) != 1 || dial.Script.Errors[0].Phase != "variable-persist" {
		t.Fatalf("script errors = %+v", dial.Script)
	}
}

func TestResolveWebSocketSchemeGuardKeepsScriptOutcome(t *testing.T) {
	reqID := uuid.New()
	repo := newMockRepo()
	repo.requests[reqID] = wsRequest(reqID, "http://example.com/ws")

	engine := &captureScriptEngine{preConsole: []string{"about to fail"}}
	uc := newWSUsecase(t, repo, wsDeps{engine: engine, preHook: "// pre"})

	dial, err := uc.ResolveWebSocket(context.Background(), reqID, testWorkspaceID, "user-1")
	if err != nil {
		t.Fatalf("scheme guard must not be a Go error: %v", err)
	}
	if dial.Failed == "" {
		t.Fatal("expected Failed for http:// scheme")
	}
	if dial.Script == nil || len(dial.Script.PreConsole) != 1 {
		t.Fatalf("script outcome dropped: %+v", dial.Script)
	}
}

// Auth is resolved before the script stage, so a resolver failure has no dial to
// report on and comes back as an error.
func TestResolveWebSocketAuthErrorIsReturned(t *testing.T) {
	reqID := uuid.New()
	repo := newMockRepo()
	repo.requests[reqID] = wsRequest(reqID, "wss://example.com/ws")

	uc := newWSUsecase(t, repo, wsDeps{resolver: failingAuthResolver{}})

	if _, err := uc.ResolveWebSocket(context.Background(), reqID, testWorkspaceID, "user-1"); err == nil {
		t.Fatal("expected an error when auth resolution fails")
	}
}

func TestResolveWebSocketReadsSettingsFromBody(t *testing.T) {
	reqID := uuid.New()
	repo := newMockRepo()
	r := wsRequest(reqID, "wss://example.com/ws")
	r.Body = `{"version":1,"pingIntervalSec":30,"subprotocols":["graphql-ws","json"]}`
	repo.requests[reqID] = r

	uc := newTestUsecaseWithEnv(t, repo, nil)

	dial, err := uc.ResolveWebSocket(context.Background(), reqID, testWorkspaceID, "")
	if err != nil {
		t.Fatalf("ResolveWebSocket: %v", err)
	}
	if dial.PingInterval != 30*time.Second {
		t.Fatalf("ping interval = %v", dial.PingInterval)
	}
	if len(dial.Subprotocols) != 2 || dial.Subprotocols[0] != "graphql-ws" {
		t.Fatalf("subprotocols = %v", dial.Subprotocols)
	}
}

func TestSubstituteMessageResolvesKnownVariables(t *testing.T) {
	uc := newTestUsecaseWithEnv(t, newMockRepo(), map[string]string{"v": "42"})

	got, err := uc.SubstituteMessage(context.Background(), uuid.New(), `{"a":"{{v}}","b":"{{missing}}"}`)
	if err != nil {
		t.Fatalf("SubstituteMessage: %v", err)
	}
	if got != `{"a":"42","b":"{{missing}}"}` {
		t.Fatalf("substituted = %q", got)
	}
}
