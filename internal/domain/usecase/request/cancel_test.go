package request_test

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/uuid"

	"github.com/tetiva-app/client/internal/domain/entities"
	"github.com/tetiva-app/client/internal/domain/usecase/request"
)

type cancellingHTTPRequester struct{ cancel context.CancelFunc }

func (r cancellingHTTPRequester) Execute(ctx context.Context, _ request.HTTPExecuteRequest) (*entities.Response, error) {
	r.cancel()
	return nil, fmt.Errorf("request failed: %w", ctx.Err())
}

type cancellingGRPCRequester struct {
	captureGRPCRequester
	cancel context.CancelFunc
}

func (r *cancellingGRPCRequester) Execute(context.Context, request.GRPCExecuteRequest) (*entities.Response, error) {
	r.cancel()
	return &entities.Response{StatusCode: 1, StatusText: "Canceled", Headers: map[string][]string{}}, nil
}

type cancellingGraphQLRequester struct {
	mockGraphQLRequester
	cancel context.CancelFunc
}

func (r *cancellingGraphQLRequester) Execute(ctx context.Context, _ request.GraphQLExecuteRequest) (*entities.Response, error) {
	r.cancel()
	return nil, fmt.Errorf("request failed: %w", ctx.Err())
}

type binaryResponseRequester struct {
	path   string
	cancel context.CancelFunc
}

func (r *binaryResponseRequester) Execute(context.Context, request.HTTPExecuteRequest) (*entities.Response, error) {
	if r.cancel != nil {
		r.cancel()
	}
	return &entities.Response{StatusCode: 200, Headers: map[string][]string{}, IsBinary: true, BinaryPath: r.path}, nil
}

type countingScriptEngine struct {
	postCalls int
	onPost    func()
}

func (e *countingScriptEngine) RunPreScript(_ context.Context, _ string, sctx request.ScriptContext) (*request.PreScriptResult, error) {
	vars := maps.Clone(sctx.Variables)
	vars["token"] = "from-pre-script"
	return &request.PreScriptResult{Headers: sctx.RequestHeaders, Variables: vars}, nil
}

func (e *countingScriptEngine) RunPostScript(_ context.Context, _ string, sctx request.ScriptContext) (*request.PostScriptResult, error) {
	e.postCalls++
	if e.onPost != nil {
		e.onPost()
	}
	return &request.PostScriptResult{Variables: sctx.Variables}, nil
}

type cancellingPersister struct {
	recordingPersister
	cancel context.CancelFunc
}

func (p *cancellingPersister) PersistVariableChanges(ctx context.Context, workspaceID uuid.UUID, userID string, vars map[string]string) error {
	p.cancel()
	return p.recordingPersister.PersistVariableChanges(ctx, workspaceID, userID, vars)
}

type ctxHistoryRepo struct{ mockHistoryRepo }

func (r *ctxHistoryRepo) Create(ctx context.Context, h *entities.History) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return r.mockHistoryRepo.Create(ctx, h)
}

type cancelDeps struct {
	repo      *mockRepo
	history   request.HistoryRepository
	http      request.HTTPRequester
	grpc      request.GRPCRequester
	graphql   request.GraphQLRequester
	engine    request.ScriptEngine
	persister request.VariablePersister
}

func (d cancelDeps) usecase() request.Usecase {
	return request.NewUsecase(d.repo, d.history, d.http, d.grpc, d.graphql, &mockEnvResolver{}, d.engine,
		&scriptResolverWithPre{pre: "pre", post: "post"}, d.persister, request.NewAuthResolver(fixtureCollections()),
		nil, nil, nil, nil, nil, nil)
}

func createSlowRequest(t *testing.T, uc request.Usecase, protocol entities.Protocol) uuid.UUID {
	t.Helper()
	created, err := uc.Create(context.Background(), request.Create{
		CollectionID: testCollectionID,
		Name:         "Slow",
		Protocol:     protocol,
		Method:       entities.MethodGET,
		URL:          "localhost:1",
		GRPCService:  "pkg.Svc",
		GRPCMethod:   "Do",
		GraphQLQuery: "{ me { id } }",
		BodyType:     entities.BodyTypeNone,
		AuthType:     entities.AuthTypeNone,
	}, request.CreateOpt{UserID: "user-1"})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	return created.ID
}

func TestExecute_CancelledRunIsNotRecorded(t *testing.T) {
	tests := []struct {
		name     string
		protocol entities.Protocol
		wire     func(*cancelDeps, *countingScriptEngine, context.CancelFunc)
		wantPost int
	}{
		{"http", entities.ProtocolHTTP, func(d *cancelDeps, _ *countingScriptEngine, cancel context.CancelFunc) {
			d.http = cancellingHTTPRequester{cancel}
		}, 0},
		{"grpc", entities.ProtocolGRPC, func(d *cancelDeps, _ *countingScriptEngine, cancel context.CancelFunc) {
			d.grpc = &cancellingGRPCRequester{cancel: cancel}
		}, 0},
		{"graphql", entities.ProtocolGraphQL, func(d *cancelDeps, _ *countingScriptEngine, cancel context.CancelFunc) {
			d.graphql = &cancellingGraphQLRequester{cancel: cancel}
		}, 0},
		{"during the post-script", entities.ProtocolHTTP, func(_ *cancelDeps, e *countingScriptEngine, cancel context.CancelFunc) {
			e.onPost = cancel
		}, 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			history, engine, persister := &mockHistoryRepo{}, &countingScriptEngine{}, &recordingPersister{}
			deps := cancelDeps{
				repo: newMockRepo(), history: history, engine: engine, persister: persister,
				http:    &mockRequester{response: &entities.Response{StatusCode: 200, Headers: map[string][]string{}}},
				grpc:    &captureGRPCRequester{},
				graphql: &mockGraphQLRequester{response: &entities.Response{StatusCode: 200}},
			}
			tt.wire(&deps, engine, cancel)
			uc := deps.usecase()
			id := createSlowRequest(t, uc, tt.protocol)

			resp, err := uc.Execute(ctx, id, request.ExecuteOpt{UserID: "user-1", WorkspaceID: testWorkspaceID})

			if !errors.Is(err, context.Canceled) {
				t.Fatalf("err = %v, want context.Canceled", err)
			}
			if resp != nil {
				t.Errorf("resp = %+v, want nil", resp)
			}
			if len(history.entries) != 0 {
				t.Errorf("history entries = %d, want 0", len(history.entries))
			}
			if persister.calls != 0 {
				t.Errorf("variables persisted %d times, want 0", persister.calls)
			}
			if engine.postCalls != tt.wantPost {
				t.Errorf("post-script ran %d times, want %d", engine.postCalls, tt.wantPost)
			}
		})
	}
}

func TestExecute_CancelAfterThePostScriptStillRecordsTheRun(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	history, persister := &ctxHistoryRepo{}, &cancellingPersister{cancel: cancel}
	uc := cancelDeps{
		repo: newMockRepo(), history: history, engine: &countingScriptEngine{}, persister: persister,
		http: &mockRequester{response: &entities.Response{StatusCode: 200, Headers: map[string][]string{}}},
	}.usecase()
	id := createSlowRequest(t, uc, entities.ProtocolHTTP)

	resp, err := uc.Execute(ctx, id, request.ExecuteOpt{UserID: "user-1", WorkspaceID: testWorkspaceID})

	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if resp == nil || resp.StatusCode != 200 {
		t.Errorf("resp = %+v, want the 200", resp)
	}
	if persister.calls != 1 || len(history.entries) != 1 {
		t.Errorf("persisted %d times, %d history entries; want 1 and 1", persister.calls, len(history.entries))
	}
}

func TestExecute_CancelledRunRemovesItsBinaryResponse(t *testing.T) {
	for _, during := range []string{"request", "post-script"} {
		t.Run(during, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			path := filepath.Join(t.TempDir(), "gopher-response-1")
			if err := os.WriteFile(path, []byte{0xff}, 0o600); err != nil {
				t.Fatal(err)
			}
			requester, engine := &binaryResponseRequester{path: path}, &countingScriptEngine{}
			if during == "request" {
				requester.cancel = cancel
			} else {
				engine.onPost = cancel
			}
			uc := cancelDeps{
				repo: newMockRepo(), history: &mockHistoryRepo{}, http: requester, engine: engine, persister: &recordingPersister{},
			}.usecase()
			id := createSlowRequest(t, uc, entities.ProtocolHTTP)

			if _, err := uc.Execute(ctx, id, request.ExecuteOpt{UserID: "user-1", WorkspaceID: testWorkspaceID}); !errors.Is(err, context.Canceled) {
				t.Fatalf("err = %v, want context.Canceled", err)
			}
			if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
				t.Errorf("binary response file still there: %v", err)
			}
		})
	}
}
