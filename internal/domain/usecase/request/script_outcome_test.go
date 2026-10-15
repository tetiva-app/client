package request_test

import (
	"context"
	"reflect"
	"testing"

	"github.com/tetiva-app/client/internal/domain/entities"
	"github.com/tetiva-app/client/internal/domain/usecase/request"
)

func TestExecute_PmVariablesReachTheRequestButNotTheEnvironment(t *testing.T) {
	for _, protocol := range []entities.Protocol{entities.ProtocolHTTP, entities.ProtocolGraphQL} {
		t.Run(string(protocol), func(t *testing.T) {
			repo := newMockRepo()
			r := snippetRequest(protocol, entities.MethodPOST, "https://api.example.com/items/{{reqId}}")
			r.Headers = []entities.HeaderItem{{Key: "X-Req-Id", Value: "{{reqId}}", Enabled: true}}
			repo.requests[r.ID] = r
			httpReq := &mockRequester{response: &entities.Response{StatusCode: 200}}
			gql := &mockGraphQLRequester{response: &entities.Response{StatusCode: 200}}
			persister := &capturingPersister{}
			engine := &captureScriptEngine{preLocals: map[string]string{"reqId": "42"}}
			uc := request.NewUsecase(repo, &mockHistoryRepo{}, httpReq, &captureGRPCRequester{}, gql,
				&mockEnvResolver{vars: map[string]string{"host": "api.example.com"}},
				engine, &scriptResolverWithPre{pre: "pm.variables.set('reqId', '42')"},
				persister, request.NewAuthResolver(fixtureCollections()), nil, nil, nil, nil, nil, nil)

			if _, err := uc.Execute(context.Background(), r.ID, request.ExecuteOpt{WorkspaceID: testWorkspaceID}); err != nil {
				t.Fatalf("execute: %v", err)
			}

			url, headers := httpReq.lastRequest.URL, httpReq.lastRequest.Headers
			if protocol == entities.ProtocolGraphQL {
				url, headers = gql.last.Endpoint, gql.last.Headers
			}
			if url != "https://api.example.com/items/42" {
				t.Errorf("url = %q", url)
			}
			if got := headers["X-Req-Id"]; !reflect.DeepEqual(got, []string{"42"}) {
				t.Errorf("X-Req-Id = %v", got)
			}
			if want := map[string]string{"host": "api.example.com"}; !reflect.DeepEqual(persister.vars, want) {
				t.Errorf("persisted %v, want %v", persister.vars, want)
			}
		})
	}
}

func TestExecute_FailedPostScriptKeepsTheTestsAndConsoleBeforeIt(t *testing.T) {
	repo := newMockRepo()
	r := snippetRequest(entities.ProtocolHTTP, entities.MethodGET, "https://api.example.com/x")
	repo.requests[r.ID] = r
	engine := &captureScriptEngine{
		postResult: &request.PostScriptResult{
			TestResults:   []request.TestResult{{Name: "status is 200", Passed: true}},
			ConsoleOutput: []string{"before"},
		},
		postErr: &request.ScriptRunError{Message: "TypeError: Cannot read property 'forEach' of undefined", Line: 3, Column: 22},
	}
	uc := request.NewUsecase(repo, &mockHistoryRepo{}, &mockRequester{response: &entities.Response{StatusCode: 200}}, nil, nil,
		&mockEnvResolver{}, engine, &scriptResolverWithPre{post: "pm.test(...)"},
		&noopVarPersister{}, request.NewAuthResolver(fixtureCollections()), nil, nil, nil, nil, nil, nil)

	resp, err := uc.Execute(context.Background(), r.ID, request.ExecuteOpt{WorkspaceID: testWorkspaceID})
	if err != nil {
		t.Fatalf("execute: %v", err)
	}

	want := &entities.ScriptResult{
		PostConsole: []string{"before"},
		Tests:       []entities.ScriptTestResult{{Name: "status is 200", Passed: true}},
		Errors: []entities.ScriptError{{
			Phase:   "post-script",
			Message: "TypeError: Cannot read property 'forEach' of undefined (line 3, column 22)",
		}},
	}
	if !reflect.DeepEqual(resp.ScriptResult, want) {
		t.Fatalf("script result = %#v, want %#v", resp.ScriptResult, want)
	}
}

func TestExecute_GRPCIsNotBlockedByHTTPOnlyAuth(t *testing.T) {
	for _, authType := range []entities.AuthType{entities.AuthTypeDigest, entities.AuthTypeAWSSigV4} {
		t.Run(string(authType), func(t *testing.T) {
			repo := newMockRepo()
			r := snippetRequest(entities.ProtocolGRPC, entities.MethodGET, "127.0.0.1:18991")
			r.AuthType, r.AuthData = authType, `{"username":"u","password":"p"}`
			repo.requests[r.ID] = r
			grpc := &captureGRPCRequester{}
			uc := request.NewUsecase(repo, &mockHistoryRepo{}, &mockRequester{}, grpc, nil, &mockEnvResolver{},
				&noopScriptEngine{}, &noopScriptResolver{}, &noopVarPersister{}, request.NewAuthResolver(fixtureCollections()),
				nil, nil, nil, nil, nil, nil)

			if _, err := uc.Execute(context.Background(), r.ID, request.ExecuteOpt{WorkspaceID: testWorkspaceID}); err != nil {
				t.Fatalf("execute: %v", err)
			}
			if grpc.last.Host != "127.0.0.1:18991" {
				t.Fatalf("grpc host = %q, want the call to go out", grpc.last.Host)
			}
		})
	}
}

func TestExecute_ScriptContentTypeInAnyCaseIsNotDoubledByTheJSONDefault(t *testing.T) {
	repo := newMockRepo()
	r := snippetRequest(entities.ProtocolHTTP, entities.MethodPOST, "https://api.example.com/x")
	r.BodyType, r.Body = entities.BodyTypeJSON, `{"a":1}`
	repo.requests[r.ID] = r
	httpReq := &mockRequester{response: &entities.Response{StatusCode: 200}}
	engine := &captureScriptEngine{preHeaders: map[string][]string{"content-type": {"application/custom"}}}
	uc := request.NewUsecase(repo, &mockHistoryRepo{}, httpReq, nil, nil, &mockEnvResolver{},
		engine, &scriptResolverWithPre{pre: "upsert"}, &noopVarPersister{},
		request.NewAuthResolver(fixtureCollections()), nil, nil, nil, nil, nil, nil)

	if _, err := uc.Execute(context.Background(), r.ID, request.ExecuteOpt{WorkspaceID: testWorkspaceID}); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if want := map[string][]string{"content-type": {"application/custom"}}; !reflect.DeepEqual(httpReq.lastRequest.Headers, want) {
		t.Fatalf("headers = %v, want %v", httpReq.lastRequest.Headers, want)
	}
}

func TestExecute_PostScriptSeesThePreScriptsPmVariables(t *testing.T) {
	for _, protocol := range []entities.Protocol{entities.ProtocolHTTP, entities.ProtocolGraphQL} {
		t.Run(string(protocol), func(t *testing.T) {
			repo := newMockRepo()
			r := snippetRequest(protocol, entities.MethodPOST, "https://api.example.com/x")
			r.GraphQLVariables = `{"id":"{{id}}"}`
			repo.requests[r.ID] = r
			engine := &captureScriptEngine{preLocals: map[string]string{"id": "new"}}
			uc := request.NewUsecase(repo, &mockHistoryRepo{}, &mockRequester{response: &entities.Response{StatusCode: 200}},
				nil, &mockGraphQLRequester{response: &entities.Response{StatusCode: 200}},
				&mockEnvResolver{vars: map[string]string{"id": "old"}}, engine,
				&scriptResolverWithPre{pre: "set", post: "get"}, &noopVarPersister{},
				request.NewAuthResolver(fixtureCollections()), nil, nil, nil, nil, nil, nil)

			if _, err := uc.Execute(context.Background(), r.ID, request.ExecuteOpt{WorkspaceID: testWorkspaceID}); err != nil {
				t.Fatalf("execute: %v", err)
			}
			if want := map[string]string{"id": "new"}; !reflect.DeepEqual(engine.postCtx.LocalVariables, want) {
				t.Errorf("post-script locals = %v, want %v", engine.postCtx.LocalVariables, want)
			}
			if protocol == entities.ProtocolGraphQL && engine.postCtx.GraphQLVariables != `{"id":"new"}` {
				t.Errorf("post-script GraphQL variables = %q", engine.postCtx.GraphQLVariables)
			}
		})
	}
}
