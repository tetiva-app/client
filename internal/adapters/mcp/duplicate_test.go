package mcp

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDuplicateRequest_CopiesUnderTheOriginalsName(t *testing.T) {
	srv := setupTestServer(t)
	col := callToolJSON(t, srv, "create_collection", map[string]any{"name": "Orders"})
	original := callToolJSON(t, srv, "create_request", map[string]any{
		"collection_id": col["id"],
		"name":          "Create order",
		"method":        "POST",
		"url":           "https://api.example.com/api/order",
		"body":          `{"qty":1}`,
		"body_type":     "json",
		"headers":       []map[string]any{{"key": "X-Trace", "value": "1", "enabled": true}},
	})
	rec := recordEvents(srv)

	dup := callToolJSON(t, srv, "duplicate_request", map[string]any{"id": original["id"]})

	assert.NotEqual(t, original["id"], dup["id"])
	assert.Equal(t, "Create order (copy)", dup["name"])
	assert.Equal(t, col["id"], dup["collection_id"])
	assert.Equal(t, float64(1), dup["version"])
	for _, field := range []string{"method", "url", "body", "body_type", "headers"} {
		assert.Equal(t, original[field], dup[field], field)
	}
	assert.Equal(t, "request duplicated", dup["message"])
	assert.Equal(t, []emitted{
		{name: "collection:updated", data: map[string]any{"collectionIds": []string{col["id"].(string)}}},
	}, rec.take())
}

func TestDuplicateRequest_SetsNameMethodAndURLInOneCall(t *testing.T) {
	srv := setupTestServer(t)
	col := callToolJSON(t, srv, "create_collection", map[string]any{"name": "Orders"})
	original := callToolJSON(t, srv, "create_request", map[string]any{
		"collection_id": col["id"],
		"name":          "Create order",
		"method":        "POST",
		"url":           "{{base}}/api/order",
		"body":          `{"qty":1}`,
		"body_type":     "json",
	})

	dup := callToolJSON(t, srv, "duplicate_request", map[string]any{
		"id":     original["id"],
		"name":   "Update order",
		"method": "patch",
		"url":    "{{base}}/api/order/1",
	})

	assert.Equal(t, "Update order", dup["name"])
	assert.Equal(t, "PATCH", dup["method"])
	assert.Equal(t, "{{base}}/api/order/1", dup["url"])
	assert.Equal(t, `{"qty":1}`, dup["body"])
	stored := callToolJSON(t, srv, "get_request", map[string]any{"id": original["id"]})
	assert.Equal(t, "POST", stored["method"])
	assert.Equal(t, "{{base}}/api/order", stored["url"])
}

func TestDuplicateRequest_EchoedMaskKeepsStoredSecret(t *testing.T) {
	srv := setupTestServer(t)
	col := callToolJSON(t, srv, "create_collection", map[string]any{"name": "Kept"})
	created := callToolJSON(t, srv, "create_request", map[string]any{
		"collection_id": col["id"],
		"name":          "Order",
		"url":           "https://api.example.com/api/order?token=tok_1",
	})
	read := callToolJSON(t, srv, "get_request", map[string]any{"id": created["id"]})
	require.Contains(t, read["url"], redactedValue)

	dup := callToolJSON(t, srv, "duplicate_request", map[string]any{
		"id":  created["id"],
		"url": "https://api.example.com/api/order/1?token=" + redactedValue,
	})

	assert.Equal(t, []any{"url.query.token"}, dup["kept_secrets"])
	stored, err := srv.reqUC.GetByID(context.Background(), uuid.MustParse(dup["id"].(string)))
	require.NoError(t, err)
	assert.Equal(t, "https://api.example.com/api/order/1?token=tok_1", stored.URL)
}

func TestDuplicateRequest_RejectsUnknownMethod(t *testing.T) {
	srv := setupTestServer(t)
	colID, reqID, _ := createEventFixtures(t, srv)

	res := callTool(t, srv, "duplicate_request", map[string]any{"id": reqID, "method": "FETCH"})

	assert.Contains(t, res, "method")
	list := callToolJSON(t, srv, "list_requests", map[string]any{"collection_id": colID})
	assert.Equal(t, float64(1), list["count"])
}
