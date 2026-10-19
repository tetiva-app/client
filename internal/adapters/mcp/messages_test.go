package mcp

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tetiva-app/client/internal/domain"
)

func TestMessages_UnauthorizedNamesSettingsSection(t *testing.T) {
	handler := NewTokenAuth("s3cret", true).Middleware(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/sse", nil))

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
	assert.Contains(t, rec.Body.String(), "Settings -> AI agents (MCP)")
}

func TestMessages_SendWebSocketRequest(t *testing.T) {
	srv := setupTestServer(t)
	col := callToolJSON(t, srv, "create_collection", map[string]any{"name": "Realtime"})
	created := callToolJSON(t, srv, "create_request", map[string]any{
		"collection_id": col["id"],
		"name":          "Ticker",
		"protocol":      "websocket",
		"url":           "wss://api.example.com/ws",
	})

	res := callTool(t, srv, "send_request", map[string]any{"id": created["id"]})

	assert.Contains(t, res, "send_request does not run WebSocket requests")
}

func TestMessages_BadUUID(t *testing.T) {
	srv := setupTestServer(t)

	res := callTool(t, srv, "get_request", map[string]any{"id": "not-a-uuid"})
	assert.Contains(t, res, "id must be a UUID; get it from the list_* tools")

	res = callTool(t, srv, "create_collection", map[string]any{"name": "Child", "parent_id": "x"})
	assert.Contains(t, res, "parent_id must be a UUID")
	assert.NotContains(t, res, "parent_id: parent_id")
}

func TestMessages_ConflictShowsCurrentVersion(t *testing.T) {
	t.Run("update_request", func(t *testing.T) {
		srv := setupTestServer(t)
		_, reqID, version := createEventFixtures(t, srv)
		callToolJSON(t, srv, "update_request", map[string]any{"id": reqID, "version": version, "name": "Bumped"})

		res := callTool(t, srv, "update_request", map[string]any{"id": reqID, "version": 1, "name": "Stale"})

		assert.Contains(t, res, "request version conflict: current version is 2; re-read it and retry")
	})

	t.Run("update_collection", func(t *testing.T) {
		srv := setupTestServer(t)
		col := callToolJSON(t, srv, "create_collection", map[string]any{"name": "Original"})
		callToolJSON(t, srv, "update_collection", map[string]any{"id": col["id"], "version": col["version"], "name": "Bumped"})

		res := callTool(t, srv, "update_collection", map[string]any{"id": col["id"], "version": 1, "name": "Stale"})

		assert.Contains(t, res, "collection version conflict: current version is 2")
	})

	t.Run("move_request", func(t *testing.T) {
		srv := setupTestServer(t)
		_, reqID, version := createEventFixtures(t, srv)
		target := callToolJSON(t, srv, "create_collection", map[string]any{"name": "Target"})
		callToolJSON(t, srv, "move_request", map[string]any{"id": reqID, "version": version, "target_collection_id": target["id"]})

		res := callTool(t, srv, "move_request", map[string]any{"id": reqID, "version": 1, "target_collection_id": target["id"]})

		assert.Contains(t, res, "request version conflict: current version is 2")
	})

	t.Run("move_collection", func(t *testing.T) {
		srv := setupTestServer(t)
		parent := callToolJSON(t, srv, "create_collection", map[string]any{"name": "Parent"})
		child := callToolJSON(t, srv, "create_collection", map[string]any{"name": "Child"})
		callToolJSON(t, srv, "move_collection", map[string]any{"id": child["id"], "version": child["version"], "parent_id": parent["id"]})

		res := callTool(t, srv, "move_collection", map[string]any{"id": child["id"], "version": 1})

		assert.Contains(t, res, "collection version conflict: current version is 2")
	})
}

func TestConflictWithVersion_RereadFails(t *testing.T) {
	conflict := &domain.ConflictError{Entity: "request", ID: "r1"}

	got := conflictWithVersion(conflict, func() (int, error) { return 0, errors.New("gone") })

	assert.Same(t, conflict, got)
}

func TestMessages_Descriptions(t *testing.T) {
	srv := setupTestServer(t)
	tools := srv.mcp.ListTools()

	propDescription := func(tool, prop string) string {
		t.Helper()
		entry, ok := tools[tool]
		require.True(t, ok, tool)
		schema, ok := entry.Tool.InputSchema.Properties[prop].(map[string]any)
		require.True(t, ok, "%s.%s", tool, prop)
		desc, _ := schema["description"].(string)
		return desc
	}

	assert.Contains(t, propDescription("create_request", "url"), "{{name}}")
	for _, tool := range []string{"create_request", "update_request", "create_collection", "update_collection"} {
		desc := propDescription(tool, "auth_data")
		assert.Contains(t, desc, `"token"`, tool)
		assert.Contains(t, desc, `"addTo"`, tool)
	}
	assert.Contains(t, tools["delete_variable"].Tool.Description, "permanent")
	assert.Contains(t, propDescription("create_collection", "workspace_id"), "Default Workspace")
	assert.Contains(t, propDescription("sync_status", "workspace_id"), "Default Workspace")
}
