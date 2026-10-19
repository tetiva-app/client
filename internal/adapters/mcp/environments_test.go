package mcp

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tetiva-app/client/internal/domain/entities"
	"github.com/tetiva-app/client/internal/domain/usecase/environment"
)

func activateEnvironment(t *testing.T, srv *Server, id any) {
	t.Helper()
	require.NoError(t, srv.envUC.SetActive(context.Background(), environment.SetActiveOpt{
		WorkspaceID:   uuid.MustParse(defaultWorkspaceID),
		EnvironmentID: uuid.MustParse(id.(string)),
	}))
}

func activeEnvironmentIDs(t *testing.T, srv *Server) []any {
	t.Helper()
	envs, err := srv.envUC.List(context.Background(), environment.ListOpt{WorkspaceID: uuid.MustParse(defaultWorkspaceID)})
	require.NoError(t, err)
	var ids []any
	for _, e := range envs {
		if e.IsActive {
			ids = append(ids, e.ID.String())
		}
	}
	return ids
}

func createRequestToSend(t *testing.T, srv *Server) any {
	t.Helper()
	col := callToolJSON(t, srv, "create_collection", map[string]any{"name": "Exec"})
	req := callToolJSON(t, srv, "create_request", map[string]any{
		"collection_id": col["id"],
		"name":          "Ping",
		"url":           "https://api.example.com/ping",
	})
	return req["id"]
}

func TestSetActiveEnvironment_Switches(t *testing.T) {
	srv := setupTestServer(t)
	a := callToolJSON(t, srv, "create_environment", map[string]any{"name": "A"})
	b := callToolJSON(t, srv, "create_environment", map[string]any{"name": "B"})
	activateEnvironment(t, srv, a["id"])
	rec := recordEvents(srv)

	out := callToolJSON(t, srv, "set_active_environment", map[string]any{"environment_id": b["id"]})

	assert.Equal(t, map[string]any{"id": b["id"], "name": "B"}, out["environment"])
	assert.Equal(t, defaultWorkspaceID, out["workspace_id"])
	assert.Equal(t, []any{b["id"]}, activeEnvironmentIDs(t, srv))
	assert.Equal(t, []emitted{
		{name: "env:changed", data: map[string]any{"workspaceId": defaultWorkspaceID}},
	}, rec.take())
}

func TestSetActiveEnvironment_UnknownKeepsActive(t *testing.T) {
	srv := setupTestServer(t)
	a := callToolJSON(t, srv, "create_environment", map[string]any{"name": "A"})
	activateEnvironment(t, srv, a["id"])
	rec := recordEvents(srv)

	err := callToolErr(srv, "set_active_environment", map[string]any{"environment_id": uuid.NewString()})

	assert.ErrorContains(t, err, "environment not found")
	assert.Equal(t, []any{a["id"]}, activeEnvironmentIDs(t, srv))
	assert.Empty(t, rec.take())
}

func TestSetActiveEnvironment_DeletedKeepsActive(t *testing.T) {
	srv := setupTestServer(t)
	a := callToolJSON(t, srv, "create_environment", map[string]any{"name": "A"})
	b := callToolJSON(t, srv, "create_environment", map[string]any{"name": "B"})
	activateEnvironment(t, srv, a["id"])
	callToolJSON(t, srv, "delete_environment", map[string]any{"id": b["id"]})

	err := callToolErr(srv, "set_active_environment", map[string]any{"environment_id": b["id"]})

	assert.ErrorContains(t, err, "environment not found")
	assert.Equal(t, []any{a["id"]}, activeEnvironmentIDs(t, srv))
}

func TestSendRequest_ReportsEnvironment(t *testing.T) {
	srv := setupExecServer(t,
		&stubHTTPRequester{response: &entities.Response{
			StatusCode: 200, StatusText: "200 OK", Protocol: entities.ProtocolHTTP,
		}},
		&stubEnvResolver{}, &stubScriptEngine{}, &stubScriptResolver{},
	)
	a := callToolJSON(t, srv, "create_environment", map[string]any{"name": "A"})
	activateEnvironment(t, srv, a["id"])
	reqID := createRequestToSend(t, srv)

	sent := callToolJSON(t, srv, "send_request", map[string]any{"id": reqID})
	assert.Equal(t, float64(200), sent["status_code"])
	assert.Equal(t, map[string]any{"id": a["id"], "name": "A"}, sent["environment"])

	callToolJSON(t, srv, "delete_environment", map[string]any{"id": a["id"]})

	sent = callToolJSON(t, srv, "send_request", map[string]any{"id": reqID})
	assert.Equal(t, float64(200), sent["status_code"])
	require.Contains(t, sent, "environment")
	assert.Nil(t, sent["environment"])
}

func TestSendRequest_NilResponseReportsEnvironment(t *testing.T) {
	srv := setupTestServer(t)
	a := callToolJSON(t, srv, "create_environment", map[string]any{"name": "A"})
	activateEnvironment(t, srv, a["id"])
	reqID := createRequestToSend(t, srv)

	sent := callToolJSON(t, srv, "send_request", map[string]any{"id": reqID})

	require.Contains(t, sent, "response")
	assert.Nil(t, sent["response"])
	assert.Equal(t, map[string]any{"id": a["id"], "name": "A"}, sent["environment"])
}
