package mcp

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tetiva-app/client/internal/domain/entities"
	"github.com/tetiva-app/client/internal/domain/usecase/collection"
	"github.com/tetiva-app/client/internal/domain/usecase/environment"
	"github.com/tetiva-app/client/internal/domain/usecase/request"
	"github.com/tetiva-app/client/internal/domain/usecase/workspace"
	"github.com/tetiva-app/client/internal/infrastructure/repository/sqlite"
	syncsvc "github.com/tetiva-app/client/internal/infrastructure/sync"

	_ "modernc.org/sqlite"
)

func setupTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	require.NoError(t, err)
	_, err = db.Exec("PRAGMA foreign_keys=ON")
	require.NoError(t, err)

	migrations := []string{
		"001_initial.sql", "002_requests_json_checks.sql", "003_auth.sql",
		"004_collection_scripts.sql", "005_collection_auth_description.sql",
		"006_workspace_is_active.sql", "007_grpc_collection_metadata.sql",
		"008_graphql.sql", "009_sync.sql", "010_workspace_remote_index.sql",
		"011_sync_config_refresh_token.sql", "012_cookies.sql", "013_request_drafts.sql",
	}
	for _, name := range migrations {
		data, err := os.ReadFile("../../../migrations/" + name)
		require.NoError(t, err, "read migration %s", name)
		_, err = db.Exec(string(data))
		require.NoError(t, err, "apply migration %s", name)
	}

	t.Cleanup(func() { _ = db.Close() })
	return db
}

func setupTestServer(t *testing.T) *Server {
	t.Helper()

	db := setupTestDB(t)

	colRepo := sqlite.NewCollectionRepo(db)
	reqRepo := sqlite.NewRequestRepo(db)
	envRepo := sqlite.NewEnvironmentRepo(db)
	varRepo := sqlite.NewVariableRepo(db)
	wsRepo := sqlite.NewWorkspaceRepo(db)
	syncQueueRepo := sqlite.NewSyncQueueRepo(db)
	syncConfigRepo := sqlite.NewSyncConfigRepo(db)

	colUC := collection.NewUsecase(colRepo)
	envUC := environment.NewUsecase(envRepo, varRepo)
	wsUC := workspace.NewUsecase(wsRepo)

	reqUC := request.NewUsecase(
		reqRepo, &stubHistoryRepo{}, &stubHTTPRequester{},
		&stubGRPCRequester{}, &stubGraphQLRequester{},
		&stubEnvResolver{}, &stubScriptEngine{},
		&stubScriptResolver{}, &stubVarPersister{},
		&stubAuthResolver{}, &stubCookieReader{}, nil,
	)

	auth := syncsvc.NewSyncAuthManager(syncConfigRepo)
	engine := syncsvc.NewSyncEngine(auth, syncQueueRepo, syncConfigRepo, db,
		colRepo, reqRepo, envRepo, varRepo)

	return NewServer(engine, syncQueueRepo, colUC, reqUC, envUC, wsUC, ":0")
}

func TestServer_AllToolsRegistered(t *testing.T) {
	srv := setupTestServer(t)

	tools := srv.mcp.ListTools()

	toolNames := make(map[string]bool)
	for name := range tools {
		toolNames[name] = true
	}

	expected := []string{
		"sync_status", "sync_push", "sync_pull",
		"sync_queue_list", "sync_queue_clear", "sync_resync",
		"sync_pause", "sync_resume", "sync_disconnect_stream",
		"list_workspaces", "get_workspace", "create_workspace",
		"list_collections", "get_collection", "create_collection",
		"update_collection", "move_collection", "delete_collection",
		"list_requests", "get_request", "create_request",
		"update_request", "move_request", "send_request", "delete_request",
		"list_environments", "get_environment", "create_environment", "delete_environment",
		"list_variables", "create_variable", "delete_variable",
	}

	for _, name := range expected {
		assert.True(t, toolNames[name], "tool %q should be registered", name)
	}

	assert.Equal(t, len(expected), len(tools),
		"expected %d tools, got %d", len(expected), len(tools))
}

func TestServer_CreateAndListCollections(t *testing.T) {
	srv := setupTestServer(t)

	createResult := callTool(t, srv, "create_collection", map[string]any{
		"name":        "Test Collection",
		"description": "For MCP testing",
	})
	require.Contains(t, createResult, "collection created")

	var created map[string]any
	require.NoError(t, json.Unmarshal([]byte(createResult), &created))
	colID := created["id"].(string)
	require.NotEmpty(t, colID)

	listResult := callTool(t, srv, "list_collections", map[string]any{})
	var listed map[string]any
	require.NoError(t, json.Unmarshal([]byte(listResult), &listed))
	assert.Equal(t, float64(1), listed["count"])

	getResult := callTool(t, srv, "get_collection", map[string]any{"id": colID})
	var got map[string]any
	require.NoError(t, json.Unmarshal([]byte(getResult), &got))
	assert.Equal(t, "Test Collection", got["name"])
	assert.Equal(t, "For MCP testing", got["description"])

	deleteResult := callTool(t, srv, "delete_collection", map[string]any{"id": colID})
	require.Contains(t, deleteResult, `"deleted": true`)
}

func TestServer_CreateAndListRequests(t *testing.T) {
	srv := setupTestServer(t)

	createColResult := callTool(t, srv, "create_collection", map[string]any{
		"name": "API Collection",
	})
	var col map[string]any
	require.NoError(t, json.Unmarshal([]byte(createColResult), &col))
	colID := col["id"].(string)

	createResult := callTool(t, srv, "create_request", map[string]any{
		"collection_id": colID,
		"name":          "Get Users",
		"method":        "GET",
		"url":           "https://api.example.com/users",
	})
	var req map[string]any
	require.NoError(t, json.Unmarshal([]byte(createResult), &req))
	assert.Equal(t, "Get Users", req["name"])
	assert.Equal(t, "GET", req["method"])
	reqID := req["id"].(string)

	listResult := callTool(t, srv, "list_requests", map[string]any{
		"collection_id": colID,
	})
	var listed map[string]any
	require.NoError(t, json.Unmarshal([]byte(listResult), &listed))
	assert.Equal(t, float64(1), listed["count"])

	getResult := callTool(t, srv, "get_request", map[string]any{"id": reqID})
	var got map[string]any
	require.NoError(t, json.Unmarshal([]byte(getResult), &got))
	assert.Equal(t, "https://api.example.com/users", got["url"])

	deleteResult := callTool(t, srv, "delete_request", map[string]any{"id": reqID})
	require.Contains(t, deleteResult, `"deleted": true`)
}

func TestServer_CreateAndListEnvironments(t *testing.T) {
	srv := setupTestServer(t)

	createResult := callTool(t, srv, "create_environment", map[string]any{
		"name": "Development",
	})
	var env map[string]any
	require.NoError(t, json.Unmarshal([]byte(createResult), &env))
	assert.Equal(t, "Development", env["name"])
	envID := env["id"].(string)

	// list includes the default "Default" environment created by migration
	listResult := callTool(t, srv, "list_environments", map[string]any{})
	var listed map[string]any
	require.NoError(t, json.Unmarshal([]byte(listResult), &listed))
	assert.GreaterOrEqual(t, listed["count"], float64(1))

	createVarResult := callTool(t, srv, "create_variable", map[string]any{
		"environment_id": envID,
		"key":            "API_KEY",
		"value":          "test-key-123",
		"is_secret":      true,
	})
	var variable map[string]any
	require.NoError(t, json.Unmarshal([]byte(createVarResult), &variable))
	assert.Equal(t, "API_KEY", variable["key"])
	varID := variable["id"].(string)

	getResult := callTool(t, srv, "get_environment", map[string]any{"id": envID})
	var got map[string]any
	require.NoError(t, json.Unmarshal([]byte(getResult), &got))
	assert.Equal(t, "Development", got["name"])
	vars := got["variables"].([]any)
	assert.Len(t, vars, 1)

	listVarsResult := callTool(t, srv, "list_variables", map[string]any{
		"environment_id": envID,
	})
	var listedVars map[string]any
	require.NoError(t, json.Unmarshal([]byte(listVarsResult), &listedVars))
	assert.Equal(t, float64(1), listedVars["count"])

	deleteVarResult := callTool(t, srv, "delete_variable", map[string]any{"id": varID})
	require.Contains(t, deleteVarResult, `"deleted": true`)

	deleteResult := callTool(t, srv, "delete_environment", map[string]any{"id": envID})
	require.Contains(t, deleteResult, `"deleted": true`)
}

func TestServer_ListWorkspaces(t *testing.T) {
	srv := setupTestServer(t)

	result := callTool(t, srv, "list_workspaces", map[string]any{})
	var listed map[string]any
	require.NoError(t, json.Unmarshal([]byte(result), &listed))
	assert.GreaterOrEqual(t, listed["count"], float64(1))

	items := listed["workspaces"].([]any)
	require.NotEmpty(t, items)
	first := items[0].(map[string]any)
	assert.NotEmpty(t, first["id"])
	assert.NotEmpty(t, first["name"])
}

func TestServer_CreateWorkspace(t *testing.T) {
	srv := setupTestServer(t)

	result := callTool(t, srv, "create_workspace", map[string]any{
		"name": "MCP Test Workspace",
	})
	var created map[string]any
	require.NoError(t, json.Unmarshal([]byte(result), &created))
	assert.Equal(t, "MCP Test Workspace", created["name"])
	assert.NotEmpty(t, created["id"])
}

func TestServer_NestedCollectionsAndUpdate(t *testing.T) {
	srv := setupTestServer(t)

	parentRes := callTool(t, srv, "create_collection", map[string]any{
		"name": "Parent",
	})
	var parent map[string]any
	require.NoError(t, json.Unmarshal([]byte(parentRes), &parent))
	parentID := parent["id"].(string)

	childRes := callTool(t, srv, "create_collection", map[string]any{
		"name":       "Child",
		"parent_id":  parentID,
		"pre_script": "pm.environment.set('x', 1);",
	})
	var child map[string]any
	require.NoError(t, json.Unmarshal([]byte(childRes), &child))
	assert.Equal(t, parentID, child["parent_id"])
	assert.Equal(t, "pm.environment.set('x', 1);", child["pre_script"])
	childID := child["id"].(string)
	childVersion := child["version"].(float64)

	updateRes := callTool(t, srv, "update_collection", map[string]any{
		"id":          childID,
		"version":     childVersion,
		"name":        "Child Renamed",
		"description": "updated",
	})
	var updated map[string]any
	require.NoError(t, json.Unmarshal([]byte(updateRes), &updated))
	assert.Equal(t, "Child Renamed", updated["name"])
	assert.Equal(t, "updated", updated["description"])
	newVersion := updated["version"].(float64)
	assert.Equal(t, childVersion+1, newVersion)

	moveRes := callTool(t, srv, "move_collection", map[string]any{
		"id":      childID,
		"version": newVersion,
	})
	var moved map[string]any
	require.NoError(t, json.Unmarshal([]byte(moveRes), &moved))
	assert.Equal(t, "", moved["parent_id"])
}

func TestServer_CreateRequestWithAllFields(t *testing.T) {
	srv := setupTestServer(t)

	colRes := callTool(t, srv, "create_collection", map[string]any{"name": "API"})
	var col map[string]any
	require.NoError(t, json.Unmarshal([]byte(colRes), &col))
	colID := col["id"].(string)

	reqRes := callTool(t, srv, "create_request", map[string]any{
		"collection_id": colID,
		"name":          "Get Users",
		"method":        "POST",
		"url":           "https://api.example.com/users",
		"body":          `{"q":"hello"}`,
		"body_type":     "json",
		"headers": []map[string]any{
			{"key": "X-API-Key", "value": "abc", "enabled": true},
			{"key": "X-Debug", "value": "1", "enabled": false},
		},
		"auth_type":   "bearer",
		"auth_data":   `{"token":"xyz"}`,
		"pre_script":  "pm.request.headers.add({key:'X-Trace', value:'1'})",
		"post_script": "pm.test('ok', () => pm.response.to.have.status(200))",
	})
	var created map[string]any
	require.NoError(t, json.Unmarshal([]byte(reqRes), &created))
	assert.Equal(t, "POST", created["method"])
	assert.Equal(t, "json", created["body_type"])
	assert.Equal(t, "bearer", created["auth_type"])
	assert.Equal(t, `{"token":"xyz"}`, created["auth_data"])
	assert.Contains(t, created["pre_script"], "X-Trace")
	assert.Contains(t, created["post_script"], "pm.test")

	headers := created["headers"].([]any)
	assert.Len(t, headers, 2)

	reqID := created["id"].(string)
	reqVersion := created["version"].(float64)

	updateRes := callTool(t, srv, "update_request", map[string]any{
		"id":        reqID,
		"version":   reqVersion,
		"name":      "Get Users v2",
		"method":    "GET",
		"url":       "https://api.example.com/v2/users",
		"body_type": "none",
		"auth_type": "inherit",
	})
	var updated map[string]any
	require.NoError(t, json.Unmarshal([]byte(updateRes), &updated))
	assert.Equal(t, "Get Users v2", updated["name"])
	assert.Equal(t, "GET", updated["method"])
	newVersion := updated["version"].(float64)

	col2Res := callTool(t, srv, "create_collection", map[string]any{"name": "API-v2"})
	var col2 map[string]any
	require.NoError(t, json.Unmarshal([]byte(col2Res), &col2))
	col2ID := col2["id"].(string)

	moveRes := callTool(t, srv, "move_request", map[string]any{
		"id":                   reqID,
		"version":              newVersion,
		"target_collection_id": col2ID,
	})
	var moved map[string]any
	require.NoError(t, json.Unmarshal([]byte(moveRes), &moved))
	assert.Equal(t, col2ID, moved["collection_id"])
}

func TestServer_SendRequest_UsesExecute(t *testing.T) {
	srv := setupTestServer(t)

	colRes := callTool(t, srv, "create_collection", map[string]any{"name": "Exec"})
	var col map[string]any
	require.NoError(t, json.Unmarshal([]byte(colRes), &col))
	colID := col["id"].(string)

	reqRes := callTool(t, srv, "create_request", map[string]any{
		"collection_id": colID,
		"name":          "Ping",
		"method":        "GET",
		"url":           "https://example.com",
	})
	var req map[string]any
	require.NoError(t, json.Unmarshal([]byte(reqRes), &req))
	reqID := req["id"].(string)

	// Stub HTTP requester returns nil, nil — so response payload is nil but executed=true.
	sendRes := callTool(t, srv, "send_request", map[string]any{"id": reqID})
	var sent map[string]any
	require.NoError(t, json.Unmarshal([]byte(sendRes), &sent))
	assert.Equal(t, true, sent["executed"])
	assert.Equal(t, reqID, sent["request_id"])
}

func TestServer_SyncStatus_Disconnected(t *testing.T) {
	srv := setupTestServer(t)

	result := callTool(t, srv, "sync_status", map[string]any{})
	var status map[string]any
	require.NoError(t, json.Unmarshal([]byte(result), &status))
	assert.Equal(t, "disconnected", status["state"])
	assert.Equal(t, float64(0), status["pending"])
}

func TestServer_SyncQueueList_Empty(t *testing.T) {
	srv := setupTestServer(t)

	result := callTool(t, srv, "sync_queue_list", map[string]any{})
	var queue map[string]any
	require.NoError(t, json.Unmarshal([]byte(result), &queue))
	assert.Equal(t, float64(0), queue["count"])
}

func TestServer_SSE_Starts(t *testing.T) {
	srv := setupTestServer(t)
	ctx := context.Background()

	srv.addr = "127.0.0.1:9399"
	require.NoError(t, srv.Start(ctx))

	// Give SSE server a moment to bind.
	time.Sleep(200 * time.Millisecond)

	client := &http.Client{Timeout: time.Second}
	resp, err := client.Get("http://127.0.0.1:9399/sse")
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestServer_SyncPause_UnknownWorkspace_ReturnsError(t *testing.T) {
	srv := setupTestServer(t)

	result := callTool(t, srv, "sync_pause", map[string]any{
		"workspace_id": "00000000-dead-beef-0000-000000000001",
	})
	assert.Contains(t, result, "error:")
}

func TestServer_SyncResume_UnknownWorkspace_ReturnsError(t *testing.T) {
	srv := setupTestServer(t)

	result := callTool(t, srv, "sync_resume", map[string]any{
		"workspace_id": "00000000-dead-beef-0000-000000000002",
	})
	assert.Contains(t, result, "error:")
}

func TestServer_SyncDisconnectStream_UnknownWorkspace_ReturnsError(t *testing.T) {
	srv := setupTestServer(t)

	result := callTool(t, srv, "sync_disconnect_stream", map[string]any{
		"workspace_id": "00000000-dead-beef-0000-000000000003",
	})
	assert.Contains(t, result, "error:")
}

func TestServer_SyncPause_KnownWorkspace_ReturnsPaused(t *testing.T) {
	srv := setupTestServer(t)

	wsID := "00000000-0000-4001-b000-000000000099"
	_, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	srv.Engine().InjectRawSyncer(wsID, "remote-"+wsID, cancel)

	result := callTool(t, srv, "sync_pause", map[string]any{
		"workspace_id": wsID,
	})

	var resp map[string]any
	require.NoError(t, json.Unmarshal([]byte(result), &resp))
	assert.Equal(t, true, resp["paused"])
	assert.Equal(t, wsID, resp["workspace_id"])
}

func TestServer_SyncResume_AfterPause_ReturnsResumed(t *testing.T) {
	srv := setupTestServer(t)

	wsID := "00000000-0000-4001-b000-000000000098"
	_, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	srv.Engine().InjectRawSyncer(wsID, "remote-"+wsID, cancel)

	require.NoError(t, srv.Engine().Pause(wsID))

	result := callTool(t, srv, "sync_resume", map[string]any{
		"workspace_id": wsID,
	})

	var resp map[string]any
	require.NoError(t, json.Unmarshal([]byte(result), &resp))
	assert.Equal(t, true, resp["resumed"])
	assert.Equal(t, wsID, resp["workspace_id"])
}

func TestServer_SyncDisconnectStream_NoActiveStream_ReturnsError(t *testing.T) {
	srv := setupTestServer(t)

	wsID := "00000000-0000-4001-b000-000000000097"
	_, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	srv.Engine().InjectRawSyncer(wsID, "remote-"+wsID, cancel)

	result := callTool(t, srv, "sync_disconnect_stream", map[string]any{
		"workspace_id": wsID,
	})
	assert.Contains(t, result, "error:")
}

func callTool(t *testing.T, srv *Server, toolName string, args map[string]any) string {
	t.Helper()
	ctx := context.Background()

	rpcReq := map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "tools/call",
		"params": map[string]any{
			"name":      toolName,
			"arguments": args,
		},
	}
	rawMsg, err := json.Marshal(rpcReq)
	require.NoError(t, err)

	result := srv.mcp.HandleMessage(ctx, rawMsg)

	data, err := json.Marshal(result)
	require.NoError(t, err)

	var resp struct {
		Result struct {
			Content []struct {
				Text string `json:"text"`
			} `json:"content"`
		} `json:"result"`
	}
	require.NoError(t, json.Unmarshal(data, &resp))

	if len(resp.Result.Content) == 0 {
		t.Fatalf("no content in tool result for %q: %s", toolName, string(data))
	}

	return resp.Result.Content[0].Text
}

type stubHistoryRepo struct{}

func (r *stubHistoryRepo) Create(_ context.Context, _ *entities.History) error { return nil }
func (r *stubHistoryRepo) GetByID(_ context.Context, _ uuid.UUID) (*entities.History, error) {
	return nil, nil
}

type stubHTTPRequester struct{}

func (r *stubHTTPRequester) Execute(_ context.Context, _ request.HTTPExecuteRequest) (*entities.Response, error) {
	return nil, nil
}

type stubGRPCRequester struct{}

func (r *stubGRPCRequester) Execute(_ context.Context, _ request.GRPCExecuteRequest) (*entities.Response, error) {
	return nil, nil
}

func (r *stubGRPCRequester) ListServices(_ context.Context, _ request.GRPCConnectRequest) (*request.GRPCSchema, error) {
	return nil, nil
}

type stubGraphQLRequester struct{}

func (r *stubGraphQLRequester) Execute(_ context.Context, _ request.GraphQLExecuteRequest) (*entities.Response, error) {
	return nil, nil
}

func (r *stubGraphQLRequester) Introspect(_ context.Context, _ request.GraphQLIntrospectRequest) (*request.GraphQLSchema, error) {
	return nil, nil
}

func (r *stubGraphQLRequester) GenerateExampleQuery(_ *request.GraphQLSchema, _ string) (*request.GraphQLExampleResponse, error) {
	return nil, nil
}

type stubEnvResolver struct{}

func (r *stubEnvResolver) ResolveVariables(_ context.Context, _ uuid.UUID) (map[string]string, error) {
	return nil, nil
}

type stubScriptEngine struct{}

func (r *stubScriptEngine) RunPreScript(_ context.Context, _ string, _ request.ScriptContext) (*request.PreScriptResult, error) {
	return &request.PreScriptResult{}, nil
}

func (r *stubScriptEngine) RunPostScript(_ context.Context, _ string, _ request.ScriptContext) (*request.PostScriptResult, error) {
	return &request.PostScriptResult{}, nil
}

type stubScriptResolver struct{}

func (r *stubScriptResolver) ResolvePreScript(_ context.Context, _ *entities.Request) (string, error) {
	return "", nil
}

func (r *stubScriptResolver) ResolvePostScript(_ context.Context, _ *entities.Request) (string, error) {
	return "", nil
}

type stubVarPersister struct{}

func (r *stubVarPersister) PersistVariableChanges(_ context.Context, _ uuid.UUID, _ string, _ map[string]string) error {
	return nil
}

type stubAuthResolver struct{}

func (r *stubAuthResolver) ResolveAuth(_ context.Context, _ *entities.Request) (entities.AuthType, string, error) {
	return entities.AuthTypeNone, "{}", nil
}

type stubCookieReader struct{}

func (r *stubCookieReader) CookiesFor(_ context.Context, _ uuid.UUID, _ string) []*http.Cookie {
	return nil
}

var (
	_ request.HistoryRepository   = (*stubHistoryRepo)(nil)
	_ request.HTTPRequester       = (*stubHTTPRequester)(nil)
	_ request.GRPCRequester       = (*stubGRPCRequester)(nil)
	_ request.GraphQLRequester    = (*stubGraphQLRequester)(nil)
	_ request.EnvironmentResolver = (*stubEnvResolver)(nil)
	_ request.ScriptEngine        = (*stubScriptEngine)(nil)
	_ request.ScriptResolver      = (*stubScriptResolver)(nil)
	_ request.VariablePersister   = (*stubVarPersister)(nil)
	_ request.AuthResolver        = (*stubAuthResolver)(nil)
	_ request.CookieReader        = (*stubCookieReader)(nil)
)
