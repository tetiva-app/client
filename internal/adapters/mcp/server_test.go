package mcp

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	mcpserver "github.com/mark3labs/mcp-go/server"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tetiva-app/client/internal/domain/entities"
	authuc "github.com/tetiva-app/client/internal/domain/usecase/auth"
	"github.com/tetiva-app/client/internal/domain/usecase/collection"
	"github.com/tetiva-app/client/internal/domain/usecase/environment"
	"github.com/tetiva-app/client/internal/domain/usecase/request"
	"github.com/tetiva-app/client/internal/domain/usecase/workspace"
	"github.com/tetiva-app/client/internal/infrastructure/repository/sqlite"
	syncsvc "github.com/tetiva-app/client/internal/infrastructure/sync"
	"github.com/tetiva-app/client/migrations"
	"github.com/tetiva-app/client/pkg/migrate"

	_ "modernc.org/sqlite"
)

func setupTestDB(t *testing.T) *sql.DB {
	t.Helper()
	// A file DB with one connection: ":memory:" hands every pooled connection its
	// own empty database, and the fk-off migration runs on a dedicated one.
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "test.db"))
	require.NoError(t, err)
	db.SetMaxOpenConns(1)
	_, err = db.Exec("PRAGMA foreign_keys=ON")
	require.NoError(t, err)
	require.NoError(t, migrate.Run(db, migrations.FS, "."))

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
	tokenRepo := sqlite.NewAuthTokenRepo(db)

	colUC := collection.NewUsecase(colRepo, tokenRepo)
	envUC := environment.NewUsecase(envRepo, varRepo)
	wsUC := workspace.NewUsecase(wsRepo)

	reqUC := request.NewUsecase(
		reqRepo, &stubHistoryRepo{}, &stubHTTPRequester{},
		&stubGRPCRequester{}, &stubGraphQLRequester{},
		&stubEnvResolver{}, &stubScriptEngine{},
		&stubScriptResolver{}, &stubVarPersister{},
		request.NewAuthResolver(collectionReaderFor{repo: colRepo}),
		&stubCookieReader{}, nil, tokenRepo, authuc.NewProvider(tokenRepo, nil, nil),
	)

	syncAuth := syncsvc.NewSyncAuthManager(syncConfigRepo)
	engine := syncsvc.NewSyncEngine(syncAuth, syncQueueRepo, syncConfigRepo, db,
		colRepo, reqRepo, envRepo, varRepo, tokenRepo)

	return NewServer(engine, syncQueueRepo, colUC, reqUC, envUC, wsUC, ":0", NewTokenAuth("", false))
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
	assert.Equal(t, redactedValue, created["auth_data"])
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

func TestServer_UpdateRequest_PatchesOnlySentArguments(t *testing.T) {
	srv := setupTestServer(t)

	colRes := callTool(t, srv, "create_collection", map[string]any{"name": "API"})
	var col map[string]any
	require.NoError(t, json.Unmarshal([]byte(colRes), &col))

	reqRes := callTool(t, srv, "create_request", map[string]any{
		"collection_id": col["id"].(string),
		"name":          "Get Users",
		"description":   "# Users\n\nLists everyone.",
		"method":        "POST",
		"url":           "https://api.example.com/users",
		"body":          `{"q":"hello"}`,
		"body_type":     "json",
		"headers": []map[string]any{
			{"key": "X-Trace", "value": "on", "enabled": true},
		},
		"auth_type":   "bearer",
		"auth_data":   `{"token":"xyz"}`,
		"pre_script":  "console.log(1)",
		"post_script": "console.log(2)",
	})
	var created map[string]any
	require.NoError(t, json.Unmarshal([]byte(reqRes), &created))
	assert.Equal(t, "# Users\n\nLists everyone.", created["description"])
	assert.Equal(t, redactedValue, created["auth_data"])

	updateRes := callTool(t, srv, "update_request", map[string]any{
		"id":      created["id"].(string),
		"version": created["version"].(float64),
		"name":    "Get Users v2",
	})
	var updated map[string]any
	require.NoError(t, json.Unmarshal([]byte(updateRes), &updated))

	assert.Equal(t, "Get Users v2", updated["name"])
	assert.Equal(t, "# Users\n\nLists everyone.", updated["description"])
	assert.Equal(t, "https://api.example.com/users", updated["url"])
	assert.Equal(t, "POST", updated["method"])
	assert.Equal(t, "json", updated["body_type"])
	assert.Equal(t, `{"q":"hello"}`, updated["body"])
	assert.Equal(t, "bearer", updated["auth_type"])
	assert.Equal(t, "console.log(1)", updated["pre_script"])
	assert.Len(t, updated["headers"].([]any), 1)

	// Tool output masks auth_data, so an omitted argument must leave the real one alone.
	stored, err := srv.reqUC.GetByID(context.Background(), uuid.MustParse(created["id"].(string)))
	require.NoError(t, err)
	assert.Equal(t, `{"token":"xyz"}`, stored.AuthData)
}

func TestServer_UpdateRequest_ClearsFieldOnExplicitEmptyString(t *testing.T) {
	srv := setupTestServer(t)

	colRes := callTool(t, srv, "create_collection", map[string]any{"name": "API"})
	var col map[string]any
	require.NoError(t, json.Unmarshal([]byte(colRes), &col))

	reqRes := callTool(t, srv, "create_request", map[string]any{
		"collection_id": col["id"].(string),
		"name":          "Documented",
		"description":   "to be removed",
	})
	var created map[string]any
	require.NoError(t, json.Unmarshal([]byte(reqRes), &created))

	updateRes := callTool(t, srv, "update_request", map[string]any{
		"id":          created["id"].(string),
		"version":     created["version"].(float64),
		"name":        "Documented",
		"description": "",
	})
	var updated map[string]any
	require.NoError(t, json.Unmarshal([]byte(updateRes), &updated))
	assert.Equal(t, "", updated["description"])
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
	base := startHTTPServer(t, srv)

	client := &http.Client{Timeout: time.Second}
	resp, err := client.Get(base + "/sse")
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

func TestServer_ListVariables_RedactsSecretValue(t *testing.T) {
	srv := setupTestServer(t)

	envRes := callTool(t, srv, "create_environment", map[string]any{"name": "Secrets"})
	var env map[string]any
	require.NoError(t, json.Unmarshal([]byte(envRes), &env))
	envID := env["id"].(string)

	callTool(t, srv, "create_variable", map[string]any{
		"environment_id": envID, "key": "TOKEN", "value": "super-secret", "is_secret": true,
	})
	callTool(t, srv, "create_variable", map[string]any{
		"environment_id": envID, "key": "BASE_URL", "value": "https://api.example.com",
	})

	listRes := callTool(t, srv, "list_variables", map[string]any{"environment_id": envID})
	assert.NotContains(t, listRes, "super-secret")

	var listed map[string]any
	require.NoError(t, json.Unmarshal([]byte(listRes), &listed))
	byKey := map[string]map[string]any{}
	for _, raw := range listed["variables"].([]any) {
		v := raw.(map[string]any)
		byKey[v["key"].(string)] = v
	}
	assert.Equal(t, redactedValue, byKey["TOKEN"]["value"])
	assert.Equal(t, "https://api.example.com", byKey["BASE_URL"]["value"])

	envGetRes := callTool(t, srv, "get_environment", map[string]any{"id": envID})
	assert.NotContains(t, envGetRes, "super-secret")
}

func TestServer_CreateVariable_SecretValueNotEchoed(t *testing.T) {
	srv := setupTestServer(t)

	envRes := callTool(t, srv, "create_environment", map[string]any{"name": "Secrets"})
	var env map[string]any
	require.NoError(t, json.Unmarshal([]byte(envRes), &env))

	createRes := callTool(t, srv, "create_variable", map[string]any{
		"environment_id": env["id"].(string),
		"key":            "TOKEN",
		"value":          "super-secret",
		"is_secret":      true,
	})
	assert.NotContains(t, createRes, "super-secret")

	var created map[string]any
	require.NoError(t, json.Unmarshal([]byte(createRes), &created))
	assert.Equal(t, redactedValue, created["value"])
}

func TestServer_GetRequest_RedactsAuthData(t *testing.T) {
	srv := setupTestServer(t)

	colRes := callTool(t, srv, "create_collection", map[string]any{"name": "API"})
	var col map[string]any
	require.NoError(t, json.Unmarshal([]byte(colRes), &col))

	reqRes := callTool(t, srv, "create_request", map[string]any{
		"collection_id": col["id"].(string),
		"name":          "Authed",
		"method":        "GET",
		"url":           "https://api.example.com",
		"auth_type":     "bearer",
		"auth_data":     `{"token":"bearer-token-value"}`,
	})
	var created map[string]any
	require.NoError(t, json.Unmarshal([]byte(reqRes), &created))

	getRes := callTool(t, srv, "get_request", map[string]any{"id": created["id"].(string)})
	assert.NotContains(t, getRes, "bearer-token-value")

	var got map[string]any
	require.NoError(t, json.Unmarshal([]byte(getRes), &got))
	assert.Equal(t, "bearer", got["auth_type"], "auth type stays visible")
	assert.Equal(t, redactedValue, got["auth_data"])
}

func TestServer_GetCollection_RedactsAuthData(t *testing.T) {
	srv := setupTestServer(t)

	colRes := callTool(t, srv, "create_collection", map[string]any{
		"name":      "Authed",
		"auth_type": "basic",
		"auth_data": `{"username":"admin","password":"hunter2"}`,
	})
	var col map[string]any
	require.NoError(t, json.Unmarshal([]byte(colRes), &col))

	getRes := callTool(t, srv, "get_collection", map[string]any{"id": col["id"].(string)})
	assert.NotContains(t, getRes, "hunter2")
	assert.NotContains(t, getRes, "admin")

	var got map[string]any
	require.NoError(t, json.Unmarshal([]byte(getRes), &got))
	assert.Equal(t, "basic", got["auth_type"])
	assert.Equal(t, redactedValue, got["auth_data"])
}

func TestServer_SerializeAuthData_EmptyWhenUnset(t *testing.T) {
	srv := setupTestServer(t)

	colRes := callTool(t, srv, "create_collection", map[string]any{"name": "No auth"})
	var col map[string]any
	require.NoError(t, json.Unmarshal([]byte(colRes), &col))
	assert.Equal(t, "", col["auth_data"])
}

func TestIsLoopbackAddr(t *testing.T) {
	loopback := []string{"127.0.0.1:9300", "localhost:9300", "[::1]:9300", "127.0.0.2:9300"}
	for _, addr := range loopback {
		assert.True(t, isLoopbackAddr(addr), "%q should be loopback", addr)
	}
	exposed := []string{":9300", "0.0.0.0:9300", "192.168.1.5:9300", "[::]:9300", ""}
	for _, addr := range exposed {
		assert.False(t, isLoopbackAddr(addr), "%q should not be loopback", addr)
	}
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

// collectionReaderFor adapts the collection repo to request.CollectionReader so
// the tools resolve auth against the collections they create.
type collectionReaderFor struct{ repo collection.Repository }

func (c collectionReaderFor) GetByID(ctx context.Context, id uuid.UUID) (*entities.Collection, error) {
	return c.repo.GetByID(ctx, id)
}

func (c collectionReaderFor) ListByWorkspace(ctx context.Context, workspaceID uuid.UUID) ([]*entities.Collection, error) {
	return c.repo.List(ctx, collection.Filter{WorkspaceID: workspaceID})
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
	_ request.CookieReader        = (*stubCookieReader)(nil)
)

// The update tools promise that omitted fields keep their value, so a schema
// that marks name required contradicts them and blocks a rename-free patch.
func TestServer_UpdateTools_NameIsOptional(t *testing.T) {
	srv := setupTestServer(t)
	tools := srv.mcp.ListTools()

	for _, name := range []string{"update_request", "update_collection"} {
		tool, ok := tools[name]
		require.True(t, ok, "tool %q not registered", name)
		assert.NotContains(t, tool.Tool.InputSchema.Required, "name",
			"%q must not force a name on a partial update", name)
		assert.Contains(t, tool.Tool.InputSchema.Required, "id", "%q must still require id", name)
		assert.Contains(t, tool.Tool.InputSchema.Required, "version",
			"%q must still require version for optimistic locking", name)
	}
}

func TestServer_UpdateRequest_PatchWithoutName(t *testing.T) {
	srv := setupTestServer(t)
	col := callToolJSON(t, srv, "create_collection", map[string]any{"name": "Patch"})
	created := callToolJSON(t, srv, "create_request", map[string]any{
		"collection_id": col["id"],
		"name":          "Original",
		"method":        "GET",
		"url":           "https://api.example.com/u",
	})

	updated := callToolJSON(t, srv, "update_request", map[string]any{
		"id":          created["id"],
		"version":     created["version"],
		"description": "docs only",
	})
	assert.Equal(t, "Original", updated["name"])
	assert.Equal(t, "docs only", updated["description"])
	assert.Equal(t, "https://api.example.com/u", updated["url"])
}

func TestServer_UpdateCollection_PatchWithoutName(t *testing.T) {
	srv := setupTestServer(t)
	created := callToolJSON(t, srv, "create_collection", map[string]any{
		"name": "Original", "description": "old",
	})

	updated := callToolJSON(t, srv, "update_collection", map[string]any{
		"id":          created["id"],
		"version":     created["version"],
		"description": "new",
	})
	assert.Equal(t, "Original", updated["name"])
	assert.Equal(t, "new", updated["description"])
}

func TestServer_CreateRequest_WebSocketSettingsBody(t *testing.T) {
	srv := setupTestServer(t)

	colRes := callTool(t, srv, "create_collection", map[string]any{"name": "Realtime"})
	var col map[string]any
	require.NoError(t, json.Unmarshal([]byte(colRes), &col))

	settings := `{"version":1,"pingIntervalSec":30,"subprotocols":["graphql-ws"],` +
		`"messages":[{"id":"m1","name":"Login","format":"json","data":"{\"op\":\"login\"}"}]}`

	createRes := callTool(t, srv, "create_request", map[string]any{
		"collection_id": col["id"].(string),
		"name":          "Ticker",
		"protocol":      "websocket",
		"url":           "wss://api.example.com/ws",
		"body":          settings,
	})
	var created map[string]any
	require.NoError(t, json.Unmarshal([]byte(createRes), &created))
	assert.Equal(t, "websocket", created["protocol"])
	assert.Equal(t, "raw", created["body_type"])
	assert.Equal(t, settings, created["body"])

	badRes := callTool(t, srv, "create_request", map[string]any{
		"collection_id": col["id"].(string),
		"name":          "Broken",
		"protocol":      "websocket",
		"url":           "wss://api.example.com/ws",
		"body":          "not json at all",
	})
	assert.Contains(t, badRes, "body")
	assert.Contains(t, badRes, "websocket settings document")
}

func TestServer_UpdateRequest_WebSocketBodyIsValidated(t *testing.T) {
	srv := setupTestServer(t)

	colRes := callTool(t, srv, "create_collection", map[string]any{"name": "Realtime"})
	var col map[string]any
	require.NoError(t, json.Unmarshal([]byte(colRes), &col))

	createRes := callTool(t, srv, "create_request", map[string]any{
		"collection_id": col["id"].(string),
		"name":          "Ticker",
		"protocol":      "websocket",
		"url":           "wss://api.example.com/ws",
	})
	var created map[string]any
	require.NoError(t, json.Unmarshal([]byte(createRes), &created))
	reqID := created["id"].(string)
	version := created["version"].(float64)

	badRes := callTool(t, srv, "update_request", map[string]any{
		"id":      reqID,
		"version": version,
		"body":    `{"version":2,"messages":[{"id":"m1","format":"xml"}]}`,
	})
	assert.Contains(t, badRes, "body")
	assert.Contains(t, badRes, "version must be 1")

	okRes := callTool(t, srv, "update_request", map[string]any{
		"id":      reqID,
		"version": version,
		"body":    `{"version":1,"pingIntervalSec":15,"subprotocols":[],"messages":[]}`,
	})
	var updated map[string]any
	require.NoError(t, json.Unmarshal([]byte(okRes), &updated))
	assert.Equal(t, "raw", updated["body_type"])
	assert.Contains(t, updated["body"], "pingIntervalSec")
}

func TestServer_RequestToolDescriptions_DocumentWebSocket(t *testing.T) {
	srv := setupTestServer(t)

	tools := srv.mcp.ListTools()

	create, ok := tools["create_request"]
	require.True(t, ok)
	protocol, ok := create.Tool.InputSchema.Properties["protocol"].(map[string]any)
	require.True(t, ok)
	assert.Contains(t, protocol["description"], "websocket")

	body, ok := create.Tool.InputSchema.Properties["body"].(map[string]any)
	require.True(t, ok)
	assert.Contains(t, body["description"], "pingIntervalSec")

	update, ok := tools["update_request"]
	require.True(t, ok)
	updateBody, ok := update.Tool.InputSchema.Properties["body"].(map[string]any)
	require.True(t, ok)
	assert.Contains(t, updateBody["description"], "pingIntervalSec")
}

// authTypesInDescription reads back the list a tool advertises: "Auth type: a, b, c (…)".
func authTypesInDescription(t *testing.T, tools map[string]*mcpserver.ServerTool, tool string) []string {
	t.Helper()
	entry, ok := tools[tool]
	require.True(t, ok, "tool %q is not registered", tool)
	prop, ok := entry.Tool.InputSchema.Properties["auth_type"].(map[string]any)
	require.True(t, ok)
	desc, ok := prop["description"].(string)
	require.True(t, ok)

	list, ok := strings.CutPrefix(desc, "Auth type: ")
	require.True(t, ok, "description %q does not start with the list", desc)
	if idx := strings.Index(list, " ("); idx >= 0 {
		list = list[:idx]
	}
	return strings.Split(list, ", ")
}

func TestServer_AuthTypeDescriptions_MirrorTheRegistry(t *testing.T) {
	srv := setupTestServer(t)
	tools := srv.mcp.ListTools()

	requestTypes := make([]string, 0, len(entities.ValidAuthTypes()))
	for _, at := range entities.ValidAuthTypes() {
		requestTypes = append(requestTypes, string(at))
	}
	collectionTypes := make([]string, 0, len(entities.ValidCollectionAuthTypes()))
	for _, at := range entities.ValidCollectionAuthTypes() {
		collectionTypes = append(collectionTypes, string(at))
	}

	assert.Equal(t, requestTypes, authTypesInDescription(t, tools, "create_request"))
	assert.Equal(t, requestTypes, authTypesInDescription(t, tools, "update_request"))
	assert.Equal(t, collectionTypes, authTypesInDescription(t, tools, "create_collection"))
	assert.Equal(t, collectionTypes, authTypesInDescription(t, tools, "update_collection"))
	assert.NotContains(t, collectionTypes, string(entities.AuthTypeInherit))
}
