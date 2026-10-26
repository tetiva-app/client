package mcp

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func pathVars(kv ...string) []any {
	out := []any{}
	for i := 0; i < len(kv); i += 2 {
		out = append(out, map[string]any{"key": kv[i], "value": kv[i+1]})
	}
	return out
}

func createUserRequest(t *testing.T, srv *Server, extra map[string]any) map[string]any {
	t.Helper()
	col := callToolJSON(t, srv, "create_collection", map[string]any{"name": "Users"})
	args := map[string]any{
		"collection_id": col["id"],
		"name":          "Get post",
		"url":           "{{baseUrl}}/users/:id/posts/:postId",
	}
	for k, v := range extra {
		args[k] = v
	}
	return callToolJSON(t, srv, "create_request", args)
}

func TestCreateRequest_PathVariablesAsObjectOrArray(t *testing.T) {
	srv := setupTestServer(t)

	byObject := createUserRequest(t, srv, map[string]any{
		"path_variables": map[string]any{"postId": "7", "id": "{{userId}}"},
	})
	assert.Equal(t, pathVars("id", "{{userId}}", "postId", "7"), byObject["path_variables"])

	byArray := createUserRequest(t, srv, map[string]any{"path_variables": pathVars("id", "42")})
	assert.Equal(t, pathVars("id", "42", "postId", ""), byArray["path_variables"])

	got := callToolJSON(t, srv, "get_request", map[string]any{"id": byArray["id"]})
	assert.Equal(t, byArray["path_variables"], got["path_variables"])
}

func TestCreateRequest_RejectsPathVariableMissingFromTheURL(t *testing.T) {
	srv := setupTestServer(t)
	col := callToolJSON(t, srv, "create_collection", map[string]any{"name": "Users"})

	err := callToolErr(srv, "create_request", map[string]any{
		"collection_id":  col["id"],
		"name":           "Get user",
		"url":            "/users/{{userId}}",
		"path_variables": map[string]any{"userId": "42"},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), ":userId")

	err = callToolErr(srv, "create_request", map[string]any{
		"collection_id":  col["id"],
		"name":           "Search",
		"protocol":       "graphql",
		"url":            "/graphql/:tenant",
		"path_variables": map[string]any{"tenant": "acme"},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "http and websocket")
}

func TestUpdateRequest_MergesPathVariables(t *testing.T) {
	srv := setupTestServer(t)
	created := createUserRequest(t, srv, map[string]any{"path_variables": pathVars("id", "42", "postId", "7")})

	updated := callToolJSON(t, srv, "update_request", map[string]any{
		"id":             created["id"],
		"version":        created["version"],
		"path_variables": map[string]any{"postId": "8"},
	})
	assert.Equal(t, pathVars("id", "42", "postId", "8"), updated["path_variables"])

	renamed := callToolJSON(t, srv, "update_request", map[string]any{
		"id":      updated["id"],
		"version": updated["version"],
		"url":     "{{baseUrl}}/users/:id",
	})
	assert.Equal(t, pathVars("id", "42"), renamed["path_variables"])

	err := callToolErr(srv, "update_request", map[string]any{
		"id":             renamed["id"],
		"version":        renamed["version"],
		"path_variables": map[string]any{"postId": "9"},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "postId")
}

func TestDuplicateRequest_TakesPathVariablesForTheCopy(t *testing.T) {
	srv := setupTestServer(t)
	original := createUserRequest(t, srv, map[string]any{"path_variables": pathVars("id", "42", "postId", "7")})

	dup := callToolJSON(t, srv, "duplicate_request", map[string]any{
		"id":             original["id"],
		"path_variables": map[string]any{"postId": "8"},
	})
	assert.Equal(t, pathVars("id", "42", "postId", "8"), dup["path_variables"])

	stored := callToolJSON(t, srv, "get_request", map[string]any{"id": original["id"]})
	assert.Equal(t, pathVars("id", "42", "postId", "7"), stored["path_variables"])
}

func TestPathVariables_AnExplicitEmptyListClears(t *testing.T) {
	srv := setupTestServer(t)
	created := createUserRequest(t, srv, map[string]any{"path_variables": pathVars("id", "42", "postId", "7")})

	for _, empty := range []any{[]any{}, map[string]any{}} {
		dup := callToolJSON(t, srv, "duplicate_request", map[string]any{"id": created["id"], "path_variables": empty})
		assert.Equal(t, pathVars("id", "", "postId", ""), dup["path_variables"])
	}

	kept := callToolJSON(t, srv, "update_request", map[string]any{"id": created["id"], "version": created["version"], "name": "Renamed"})
	assert.Equal(t, pathVars("id", "42", "postId", "7"), kept["path_variables"])

	cleared := callToolJSON(t, srv, "update_request", map[string]any{
		"id": created["id"], "version": kept["version"], "path_variables": []any{},
	})
	assert.Equal(t, pathVars("id", "", "postId", ""), cleared["path_variables"])
}

func TestPathVariables_AnEntryNeedsAStringValue(t *testing.T) {
	srv := setupTestServer(t)
	created := createUserRequest(t, srv, map[string]any{"path_variables": pathVars("id", "42")})

	for name, arg := range map[string]any{
		"missing value": []any{map[string]any{"key": "id"}},
		"null value":    []any{map[string]any{"key": "id", "value": nil}},
		"number value":  []any{map[string]any{"key": "id", "value": 7}},
		"null in map":   map[string]any{"id": nil},
		"number in map": map[string]any{"id": 7},
	} {
		err := callToolErr(srv, "update_request", map[string]any{"id": created["id"], "version": created["version"], "path_variables": arg})
		require.Error(t, err, name)
		assert.Contains(t, err.Error(), `"id"`, name)
	}

	stored := callToolJSON(t, srv, "get_request", map[string]any{"id": created["id"]})
	assert.Equal(t, pathVars("id", "42", "postId", ""), stored["path_variables"])

	cleared := callToolJSON(t, srv, "update_request", map[string]any{
		"id": created["id"], "version": created["version"], "path_variables": map[string]any{"id": ""},
	})
	assert.Equal(t, pathVars("id", "", "postId", ""), cleared["path_variables"])
}
