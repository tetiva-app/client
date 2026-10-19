package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type emitted struct {
	name string
	data map[string]any
}

type recordingEmitter struct {
	mu     sync.Mutex
	events []emitted
}

func (r *recordingEmitter) emit(name string, data any) {
	m, _ := data.(map[string]any)
	r.mu.Lock()
	r.events = append(r.events, emitted{name: name, data: m})
	r.mu.Unlock()
}

func (r *recordingEmitter) take() []emitted {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := r.events
	r.events = nil
	return out
}

func recordEvents(srv *Server) *recordingEmitter {
	rec := &recordingEmitter{}
	srv.SetEventEmitter(rec.emit)
	return rec
}

func callToolErr(srv *Server, name string, args map[string]any) error {
	raw, err := json.Marshal(map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "tools/call",
		"params":  map[string]any{"name": name, "arguments": args},
	})
	if err != nil {
		return err
	}
	data, err := json.Marshal(srv.mcp.HandleMessage(context.Background(), raw))
	if err != nil {
		return err
	}
	var resp struct {
		Error  json.RawMessage `json:"error"`
		Result struct {
			IsError bool `json:"isError"`
		} `json:"result"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return err
	}
	if len(resp.Error) > 0 || resp.Result.IsError {
		return fmt.Errorf("%s failed: %s", name, data)
	}
	return nil
}

func createEventFixtures(t *testing.T, srv *Server) (colID, reqID string, version float64) {
	t.Helper()
	col := callToolJSON(t, srv, "create_collection", map[string]any{"name": "Events"})
	req := callToolJSON(t, srv, "create_request", map[string]any{
		"collection_id": col["id"],
		"name":          "Ping",
		"url":           "https://api.example.com/ping",
	})
	return col["id"].(string), req["id"].(string), req["version"].(float64)
}

func TestEvents_CreateRequestEmitsCollectionUpdated(t *testing.T) {
	srv := setupTestServer(t)
	col := callToolJSON(t, srv, "create_collection", map[string]any{"name": "Events"})
	colID := col["id"].(string)
	rec := recordEvents(srv)

	callToolJSON(t, srv, "create_request", map[string]any{
		"collection_id": colID,
		"name":          "Ping",
	})

	assert.Equal(t, []emitted{
		{name: "collection:updated", data: map[string]any{"collectionIds": []string{colID}}},
	}, rec.take())
}

func TestEvents_UpdateRequestEmitsRequestUpdated(t *testing.T) {
	srv := setupTestServer(t)
	colID, reqID, version := createEventFixtures(t, srv)
	rec := recordEvents(srv)

	callToolJSON(t, srv, "update_request", map[string]any{
		"id":      reqID,
		"version": version,
		"name":    "Pong",
	})

	assert.Equal(t, []emitted{
		{name: "collection:updated", data: map[string]any{"collectionIds": []string{colID}}},
		{name: "request:updated:" + reqID},
	}, rec.take())
}

func TestEvents_MoveRequestListsTargetThenSource(t *testing.T) {
	srv := setupTestServer(t)
	sourceID, reqID, version := createEventFixtures(t, srv)
	target := callToolJSON(t, srv, "create_collection", map[string]any{"name": "Target"})
	targetID := target["id"].(string)
	rec := recordEvents(srv)

	moved := callToolJSON(t, srv, "move_request", map[string]any{
		"id":                   reqID,
		"version":              version,
		"target_collection_id": targetID,
	})

	assert.Equal(t, version+1, moved["version"])
	assert.Equal(t, []emitted{
		{name: "collection:updated", data: map[string]any{"collectionIds": []string{targetID, sourceID}}},
		{name: "request:updated:" + reqID},
	}, rec.take())
}

func TestEvents_DeleteRequestEmitsRequestDeleted(t *testing.T) {
	srv := setupTestServer(t)
	colID, reqID, _ := createEventFixtures(t, srv)
	rec := recordEvents(srv)

	callToolJSON(t, srv, "delete_request", map[string]any{"id": reqID})

	assert.Equal(t, []emitted{
		{name: "collection:updated", data: map[string]any{"collectionIds": []string{colID}}},
		{name: "request:deleted:" + reqID},
	}, rec.take())
}

func TestEvents_CollectionToolsEmitWorkspace(t *testing.T) {
	srv := setupTestServer(t)
	rec := recordEvents(srv)
	want := []emitted{
		{name: "collection:updated", data: map[string]any{"workspaceId": defaultWorkspaceID}},
	}

	created := callToolJSON(t, srv, "create_collection", map[string]any{"name": "Events"})
	assert.Equal(t, want, rec.take(), "create_collection")

	updated := callToolJSON(t, srv, "update_collection", map[string]any{
		"id":      created["id"],
		"version": created["version"],
		"name":    "Renamed",
	})
	assert.Equal(t, want, rec.take(), "update_collection")

	callToolJSON(t, srv, "move_collection", map[string]any{
		"id":      created["id"],
		"version": updated["version"],
	})
	assert.Equal(t, want, rec.take(), "move_collection")

	callToolJSON(t, srv, "delete_collection", map[string]any{"id": created["id"]})
	assert.Equal(t, want, rec.take(), "delete_collection")
}

func TestEvents_EnvironmentToolsEmitEnvChanged(t *testing.T) {
	srv := setupTestServer(t)
	rec := recordEvents(srv)
	withWorkspace := []emitted{
		{name: "env:changed", data: map[string]any{"workspaceId": defaultWorkspaceID}},
	}
	empty := []emitted{{name: "env:changed", data: map[string]any{}}}

	env := callToolJSON(t, srv, "create_environment", map[string]any{"name": "Staging"})
	assert.Equal(t, withWorkspace, rec.take(), "create_environment")

	variable := callToolJSON(t, srv, "create_variable", map[string]any{
		"environment_id": env["id"],
		"key":            "HOST",
		"value":          "staging.example.com",
	})
	assert.Equal(t, empty, rec.take(), "create_variable")

	callToolJSON(t, srv, "delete_variable", map[string]any{"id": variable["id"]})
	assert.Equal(t, empty, rec.take(), "delete_variable")

	callToolJSON(t, srv, "delete_environment", map[string]any{"id": env["id"]})
	assert.Equal(t, withWorkspace, rec.take(), "delete_environment")
}

func TestEvents_FailedWriteEmitsNothing(t *testing.T) {
	srv := setupTestServer(t)
	_, reqID, version := createEventFixtures(t, srv)
	rec := recordEvents(srv)

	err := callToolErr(srv, "update_request", map[string]any{
		"id":      reqID,
		"version": version + 1,
		"name":    "Pong",
	})

	require.Error(t, err)
	assert.Empty(t, rec.take())
}

func TestEvents_ConcurrentSetAndEmit(t *testing.T) {
	srv := setupTestServer(t)
	rec := &recordingEmitter{}

	const calls = 50
	errs := make(chan error, calls)
	var wg sync.WaitGroup
	for i := range calls {
		wg.Go(func() {
			errs <- callToolErr(srv, "create_collection", map[string]any{"name": fmt.Sprintf("C%d", i)})
		})
	}
	wg.Go(func() {
		for range calls {
			srv.SetEventEmitter(rec.emit)
		}
	})
	wg.Wait()
	close(errs)

	for err := range errs {
		assert.NoError(t, err)
	}
}
