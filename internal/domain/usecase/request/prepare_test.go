package request_test

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"github.com/tetiva-app/client/internal/domain/entities"
	"github.com/tetiva-app/client/internal/domain/usecase/request"
)

// captureScriptEngine records calls and returns scripted output.
type captureScriptEngine struct {
	preCalls   int
	preHeaders map[string][]string
	preVars    map[string]string
	preConsole []string
	preErr     error
}

func (c *captureScriptEngine) RunPreScript(_ context.Context, _ string, sctx request.ScriptContext) (*request.PreScriptResult, error) {
	c.preCalls++
	if c.preErr != nil {
		return nil, c.preErr
	}
	headers := c.preHeaders
	if headers == nil {
		headers = sctx.RequestHeaders
	}
	vars := c.preVars
	if vars == nil {
		vars = sctx.Variables
	}
	return &request.PreScriptResult{Headers: headers, Variables: vars, ConsoleOutput: c.preConsole}, nil
}

func (c *captureScriptEngine) RunPostScript(_ context.Context, _ string, _ request.ScriptContext) (*request.PostScriptResult, error) {
	return &request.PostScriptResult{}, nil
}

// scriptResolverWithPre returns a fixed pre-script.
type scriptResolverWithPre struct{ pre string }

func (s *scriptResolverWithPre) ResolvePreScript(_ context.Context, _ *entities.Request) (string, error) {
	return s.pre, nil
}

func (s *scriptResolverWithPre) ResolvePostScript(_ context.Context, _ *entities.Request) (string, error) {
	return "", nil
}

// prepareHTTP is unexported; the parity tests below exercise it via Execute.

func TestExecute_HTTPParity_PlainGET(t *testing.T) {
	uc, repo, _, requester := newTestUsecase()
	ctx := context.Background()

	id := uuid.New()
	repo.requests[id] = &entities.Request{
		ID:       id,
		Protocol: entities.ProtocolHTTP,
		Method:   entities.MethodGET,
		URL:      "https://api.example.com/x",
		BodyType: entities.BodyTypeNone,
		AuthType: entities.AuthTypeNone,
	}
	requester.response = &entities.Response{StatusCode: 200}

	if _, err := uc.Execute(ctx, id, request.ExecuteOpt{WorkspaceID: uuid.New()}); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if requester.lastRequest.URL != "https://api.example.com/x" {
		t.Errorf("URL: got %q", requester.lastRequest.URL)
	}
	if requester.lastRequest.Method != entities.MethodGET {
		t.Errorf("method: got %q", requester.lastRequest.Method)
	}
}

func TestExecute_HTTPParity_JSONPostAddsContentType(t *testing.T) {
	uc, repo, _, requester := newTestUsecase()
	ctx := context.Background()

	id := uuid.New()
	repo.requests[id] = &entities.Request{
		ID:       id,
		Protocol: entities.ProtocolHTTP,
		Method:   entities.MethodPOST,
		URL:      "https://api.example.com/users",
		Body:     `{"name":"Alice"}`,
		BodyType: entities.BodyTypeJSON,
		AuthType: entities.AuthTypeNone,
	}
	requester.response = &entities.Response{StatusCode: 201}

	if _, err := uc.Execute(ctx, id, request.ExecuteOpt{WorkspaceID: uuid.New()}); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if got := requester.lastRequest.Headers["Content-Type"]; len(got) != 1 || got[0] != "application/json" {
		t.Errorf("Content-Type: got %v, want [application/json]", got)
	}
	if requester.lastRequest.Body != `{"name":"Alice"}` {
		t.Errorf("body mismatch: %q", requester.lastRequest.Body)
	}
}

func TestExecute_HTTPParity_BearerAuthHeader(t *testing.T) {
	uc, repo, _, requester := newTestUsecase()
	ctx := context.Background()

	id := uuid.New()
	repo.requests[id] = &entities.Request{
		ID:       id,
		Protocol: entities.ProtocolHTTP,
		Method:   entities.MethodGET,
		URL:      "https://api.example.com/me",
		BodyType: entities.BodyTypeNone,
		AuthType: entities.AuthTypeBearer,
		AuthData: `{"token":"xyz"}`,
	}
	requester.response = &entities.Response{StatusCode: 200}

	if _, err := uc.Execute(ctx, id, request.ExecuteOpt{WorkspaceID: uuid.New()}); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if got := requester.lastRequest.Headers["Authorization"]; len(got) != 1 || got[0] != "Bearer xyz" {
		t.Errorf("Authorization: got %v, want [Bearer xyz]", got)
	}
}

func TestExecute_HTTPParity_PreScriptInjectsHeader(t *testing.T) {
	repo := newMockRepo()
	historyRepo := &mockHistoryRepo{}
	requester := &mockRequester{response: &entities.Response{StatusCode: 200}}
	cap := &captureScriptEngine{
		preHeaders: map[string][]string{"X-Trace": {"abc"}},
		preVars:    map[string]string{},
	}
	uc := request.NewUsecase(repo, historyRepo, requester, nil, nil, &mockEnvResolver{}, cap, &scriptResolverWithPre{pre: "// inject"}, &noopVarPersister{}, request.NewAuthResolver(&mockCollectionReader{}), nil, nil)

	id := uuid.New()
	repo.requests[id] = &entities.Request{
		ID:       id,
		Protocol: entities.ProtocolHTTP,
		Method:   entities.MethodGET,
		URL:      "https://api.example.com/x",
		BodyType: entities.BodyTypeNone,
		AuthType: entities.AuthTypeNone,
	}

	ctx := context.Background()
	if _, err := uc.Execute(ctx, id, request.ExecuteOpt{WorkspaceID: uuid.New()}); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if cap.preCalls != 1 {
		t.Errorf("expected 1 pre-script call, got %d", cap.preCalls)
	}
	if got := requester.lastRequest.Headers["X-Trace"]; len(got) != 1 || got[0] != "abc" {
		t.Errorf("X-Trace: got %v", got)
	}
}
