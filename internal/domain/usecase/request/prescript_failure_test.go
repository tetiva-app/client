package request_test

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/google/uuid"

	"github.com/tetiva-app/client/internal/domain"
	"github.com/tetiva-app/client/internal/domain/entities"
	"github.com/tetiva-app/client/internal/domain/usecase/request"
)

var errBtoa = errors.New("script error: ReferenceError: btoa is not defined at <eval>:10:21(14)")

func assertPreScriptFailed(t *testing.T, err error) {
	t.Helper()
	var reasonErr *domain.ReasonError
	if !errors.As(err, &reasonErr) || reasonErr.Reason != request.ReasonPreScriptFailed || !errors.Is(err, errBtoa) {
		t.Fatalf("err = %v, want %s wrapping the script error", err, request.ReasonPreScriptFailed)
	}
}

func TestExecute_FailedPreScriptIsNotSent(t *testing.T) {
	for _, protocol := range []entities.Protocol{entities.ProtocolHTTP, entities.ProtocolGraphQL, entities.ProtocolGRPC} {
		t.Run(string(protocol), func(t *testing.T) {
			repo := newMockRepo()
			r := snippetRequest(protocol, entities.MethodGET, "https://api.example.com/x")
			repo.requests[r.ID] = r
			history := &mockHistoryRepo{}
			httpReq := &mockRequester{response: &entities.Response{StatusCode: 200}}
			gql := &mockGraphQLRequester{response: &entities.Response{StatusCode: 200}}
			grpc := &captureGRPCRequester{}
			uc := request.NewUsecase(repo, history, httpReq, grpc, gql, &mockEnvResolver{},
				&captureScriptEngine{preErr: errBtoa}, &scriptResolverWithPre{pre: "btoa('x')"},
				&noopVarPersister{}, request.NewAuthResolver(fixtureCollections()), nil, nil, nil, nil, nil, nil)

			_, err := uc.Execute(context.Background(), r.ID, request.ExecuteOpt{WorkspaceID: testWorkspaceID})

			assertPreScriptFailed(t, err)
			if httpReq.lastRequest.URL != "" || gql.last.Endpoint != "" || grpc.last.Host != "" {
				t.Fatal("request was sent despite the failed pre-script")
			}
			if len(history.entries) != 0 {
				t.Fatalf("history got %d entries for an unsent request", len(history.entries))
			}
		})
	}
}

func TestResolveWebSocket_FailedPreScriptDoesNotDial(t *testing.T) {
	repo := newMockRepo()
	r := wsRequest(uuid.New(), "wss://example.com/ws")
	repo.requests[r.ID] = r
	uc := newWSUsecase(t, repo, wsDeps{engine: &captureScriptEngine{preErr: errBtoa}, preHook: "btoa('x')"})

	dial, err := uc.ResolveWebSocket(context.Background(), r.ID, testWorkspaceID, "user-1")

	assertPreScriptFailed(t, err)
	if dial.URL != "" {
		t.Fatalf("dial = %+v, want none", dial)
	}
}

func TestBuildCurl_FailedPreScriptIsAnError(t *testing.T) {
	repo := newMockRepo()
	r := snippetRequest(entities.ProtocolHTTP, entities.MethodGET, "https://api.example.com/x")
	repo.requests[r.ID] = r
	uc := ucWithPreScript(repo, nil, &captureScriptEngine{preErr: errBtoa}, &mockRequester{}, nil)

	res, err := uc.BuildCurl(context.Background(), r.ID, request.BuildCurlOpt{WorkspaceID: testWorkspaceID})

	assertPreScriptFailed(t, err)
	if res.Command != "" {
		t.Fatalf("command = %q, want none", res.Command)
	}
}

func TestExecute_FailedPreScriptSaysWhatFailedWhereAndWhatItLogged(t *testing.T) {
	for _, protocol := range []entities.Protocol{entities.ProtocolHTTP, entities.ProtocolGraphQL, entities.ProtocolGRPC} {
		t.Run(string(protocol), func(t *testing.T) {
			repo := newMockRepo()
			r := snippetRequest(protocol, entities.MethodGET, "https://api.example.com/x")
			repo.requests[r.ID] = r
			engine := &captureScriptEngine{
				preErr:     &request.ScriptRunError{Message: "ReferenceError: computeSignature is not defined", Line: 3, Column: 13},
				preConsole: []string{"step 1", `step 2 {"id":7}`},
			}
			uc := request.NewUsecase(repo, &mockHistoryRepo{}, &mockRequester{}, &captureGRPCRequester{}, &mockGraphQLRequester{}, &mockEnvResolver{},
				engine, &scriptResolverWithPre{pre: "computeSignature()", collection: "F1 Folder"},
				&noopVarPersister{}, request.NewAuthResolver(fixtureCollections()), nil, nil, nil, nil, nil, nil)

			_, err := uc.Execute(context.Background(), r.ID, request.ExecuteOpt{WorkspaceID: testWorkspaceID})

			var reasonErr *domain.ReasonError
			if !errors.As(err, &reasonErr) || reasonErr.Reason != request.ReasonPreScriptFailed {
				t.Fatalf("err = %v, want %s", err, request.ReasonPreScriptFailed)
			}
			want := map[string]string{
				"message":    "ReferenceError: computeSignature is not defined",
				"line":       "3",
				"column":     "13",
				"collection": "F1 Folder",
				"console":    `["step 1","step 2 {\"id\":7}"]`,
			}
			if !reflect.DeepEqual(reasonErr.Fields, want) {
				t.Fatalf("fields = %#v, want %#v", reasonErr.Fields, want)
			}
		})
	}
}

func TestResolveWebSocket_FailedPreScriptMarksATimeout(t *testing.T) {
	repo := newMockRepo()
	r := wsRequest(uuid.New(), "wss://example.com/ws")
	repo.requests[r.ID] = r
	engine := &captureScriptEngine{preErr: &request.ScriptRunError{Message: "Script timed out after 5s", Timeout: true}}
	uc := newWSUsecase(t, repo, wsDeps{engine: engine, preHook: "while(true){}"})

	_, err := uc.ResolveWebSocket(context.Background(), r.ID, testWorkspaceID, "user-1")

	var reasonErr *domain.ReasonError
	if !errors.As(err, &reasonErr) {
		t.Fatalf("err = %v, want a ReasonError", err)
	}
	if want := map[string]string{"message": "Script timed out after 5s", "timeout": "true"}; !reflect.DeepEqual(reasonErr.Fields, want) {
		t.Fatalf("fields = %#v, want %#v", reasonErr.Fields, want)
	}
}
