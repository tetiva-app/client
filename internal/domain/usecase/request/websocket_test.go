package request_test

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"github.com/tetiva-app/client/internal/domain/entities"
	"github.com/tetiva-app/client/internal/domain/usecase/request"
)

// newTestUsecaseWithEnv creates a usecase wired with the given environment vars,
// reusing the stub types defined across this package's _test files.
func newTestUsecaseWithEnv(t *testing.T, repo *mockRepo, vars map[string]string) request.Usecase {
	t.Helper()
	historyRepo := &mockHistoryRepo{}
	uc := request.NewUsecase(
		repo,
		historyRepo,
		&mockRequester{},
		nil,
		nil,
		&mockEnvResolver{vars: vars},
		&noopScriptEngine{},
		&noopScriptResolver{},
		&noopVarPersister{},
		request.NewAuthResolver(&mockCollectionReader{}),
		nil,
		nil,
	)
	return uc
}

func TestResolveWebSocketSubstitutesVarsAndAuth(t *testing.T) {
	reqID := uuid.New()
	wsID := uuid.New()
	repo := newMockRepo()
	repo.requests[reqID] = &entities.Request{
		ID:           reqID,
		CollectionID: testCollectionID,
		Protocol:     entities.ProtocolWebSocket,
		URL:          "wss://{{host}}/ws",
		Headers:      []entities.HeaderItem{{Key: "X-Token", Value: "{{tok}}", Enabled: true}},
		AuthType:     entities.AuthTypeNone,
		AuthData:     "{}",
	}

	uc := newTestUsecaseWithEnv(t, repo, map[string]string{"host": "example.com", "tok": "secret"})

	url, headers, err := uc.ResolveWebSocket(context.Background(), reqID, wsID, "user-1")
	if err != nil {
		t.Fatalf("ResolveWebSocket: %v", err)
	}
	if url != "wss://example.com/ws" {
		t.Fatalf("url = %q", url)
	}
	if got := headers["X-Token"]; len(got) != 1 || got[0] != "secret" {
		t.Fatalf("header X-Token = %v", got)
	}
}

func TestResolveWebSocketRejectsNonWSScheme(t *testing.T) {
	reqID := uuid.New()
	repo := newMockRepo()
	repo.requests[reqID] = &entities.Request{
		ID:           reqID,
		CollectionID: testCollectionID,
		Protocol:     entities.ProtocolWebSocket,
		URL:          "https://example.com/ws", // not ws:// or wss://
		AuthType:     entities.AuthTypeNone,
		AuthData:     "{}",
	}
	uc := newTestUsecaseWithEnv(t, repo, map[string]string{})

	if _, _, err := uc.ResolveWebSocket(context.Background(), reqID, uuid.New(), ""); err == nil {
		t.Fatal("expected validation error for non-ws scheme")
	}
}

func TestResolveWebSocketRejectsNonWebSocketProtocol(t *testing.T) {
	reqID := uuid.New()
	repo := newMockRepo()
	repo.requests[reqID] = &entities.Request{
		ID:           reqID,
		CollectionID: testCollectionID,
		Protocol:     entities.ProtocolHTTP, // wrong protocol for ResolveWebSocket
		URL:          "ws://example.com/ws",
		AuthType:     entities.AuthTypeNone,
		AuthData:     "{}",
	}
	uc := newTestUsecaseWithEnv(t, repo, map[string]string{})

	if _, _, err := uc.ResolveWebSocket(context.Background(), reqID, uuid.New(), ""); err == nil {
		t.Fatal("expected validation error for non-websocket protocol")
	}
}
