package mcp

import (
	"context"
	"encoding/json"
	"fmt"

	mcplib "github.com/mark3labs/mcp-go/mcp"
)

func (s *Server) registerSyncTools() {
	s.mcp.AddTool(
		mcplib.NewTool("sync_status",
			mcplib.WithDescription("Get current sync engine state and pending count for a workspace"),
			mcplib.WithString("workspace_id",
				mcplib.Description("Local workspace UUID (defaults to default workspace)"),
			),
		),
		s.handleSyncStatus,
	)

	s.mcp.AddTool(
		mcplib.NewTool("sync_push",
			mcplib.WithDescription("Trigger an immediate push of pending changes to the sync server"),
			mcplib.WithString("workspace_id",
				mcplib.Description("Local workspace UUID (defaults to default workspace)"),
			),
		),
		s.handleSyncPush,
	)

	s.mcp.AddTool(
		mcplib.NewTool("sync_pull",
			mcplib.WithDescription("Restart sync cycle with incremental pull from current sequence (push pending + pull new + resubscribe)"),
			mcplib.WithString("workspace_id",
				mcplib.Description("Local workspace UUID (defaults to default workspace)"),
			),
		),
		s.handleSyncPull,
	)

	s.mcp.AddTool(
		mcplib.NewTool("sync_queue_list",
			mcplib.WithDescription("List pending entries in the sync outbox queue"),
			mcplib.WithString("workspace_id",
				mcplib.Description("Local workspace UUID (defaults to default workspace)"),
			),
			mcplib.WithNumber("limit",
				mcplib.Description("Max entries to return (default 50)"),
			),
		),
		s.handleSyncQueueList,
	)

	s.mcp.AddTool(
		mcplib.NewTool("sync_queue_clear",
			mcplib.WithDescription("Clear all pending entries from the sync outbox queue for a workspace"),
			mcplib.WithString("workspace_id",
				mcplib.Description("Local workspace UUID (defaults to default workspace)"),
			),
		),
		s.handleSyncQueueClear,
	)

	s.mcp.AddTool(
		mcplib.NewTool("sync_resync",
			mcplib.WithDescription("Stop sync, clear queue, reset sync sequence to 0, and restart full sync cycle"),
			mcplib.WithString("workspace_id",
				mcplib.Description("Local workspace UUID (defaults to default workspace)"),
			),
		),
		s.handleSyncResync,
	)

	s.mcp.AddTool(
		mcplib.NewTool("sync_pause",
			mcplib.WithDescription("Pause the sync engine for a workspace: stops push, pull, and realtime events. Queue keeps accumulating writes locally. Idempotent."),
			mcplib.WithString("workspace_id",
				mcplib.Required(),
				mcplib.Description("Local workspace UUID"),
			),
		),
		s.handleSyncPause,
	)

	s.mcp.AddTool(
		mcplib.NewTool("sync_resume",
			mcplib.WithDescription("Resume a paused workspace. Drains the outbox queue (push), pulls increments, and re-subscribes. Idempotent."),
			mcplib.WithString("workspace_id",
				mcplib.Required(),
				mcplib.Description("Local workspace UUID"),
			),
		),
		s.handleSyncResume,
	)

	s.mcp.AddTool(
		mcplib.NewTool("sync_disconnect_stream",
			mcplib.WithDescription("Drop the current subscribe stream once for a workspace. The engine reconnects via existing exponential backoff (5s–5min). Use to test backoff/reconnect paths and provoke ResyncRequired."),
			mcplib.WithString("workspace_id",
				mcplib.Required(),
				mcplib.Description("Local workspace UUID"),
			),
		),
		s.handleSyncDisconnectStream,
	)
}

func (s *Server) handleSyncStatus(ctx context.Context, req mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
	wsID := stringArg(req, "workspace_id", defaultWorkspaceID)

	state := s.engine.GetWorkspaceState(wsID)
	pending, err := s.engine.GetPendingCount(ctx, wsID)
	if err != nil {
		return errResult(err), nil
	}

	result := map[string]any{
		"workspace_id": wsID,
		"state":        string(state),
		"pending":      pending,
	}

	return jsonResult(result)
}

func (s *Server) handleSyncPush(ctx context.Context, req mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
	wsID := stringArg(req, "workspace_id", defaultWorkspaceID)

	pending, err := s.engine.ForcePush(ctx, wsID)
	if err != nil {
		return errResult(err), nil
	}

	result := map[string]any{
		"workspace_id":   wsID,
		"signal_sent":    true,
		"pending_before": pending,
	}

	return jsonResult(result)
}

func (s *Server) handleSyncPull(_ context.Context, req mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
	wsID := stringArg(req, "workspace_id", defaultWorkspaceID)

	if err := s.engine.ForcePull(wsID); err != nil {
		return errResult(err), nil
	}

	result := map[string]any{
		"workspace_id": wsID,
		"message":      "sync cycle restarted with incremental pull",
	}

	return jsonResult(result)
}

func (s *Server) handleSyncQueueList(ctx context.Context, req mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
	wsID := stringArg(req, "workspace_id", defaultWorkspaceID)
	limit := intArg(req, "limit", 50)

	entries, err := s.syncQueue.CoalescedPending(ctx, wsID, limit)
	if err != nil {
		return errResult(err), nil
	}

	items := make([]map[string]any, 0, len(entries))
	for _, e := range entries {
		items = append(items, map[string]any{
			"id":          e.ID,
			"entity_type": e.EntityType,
			"entity_id":   e.EntityID,
			"action":      e.Action,
			"status":      e.Status,
			"retry_count": e.RetryCount,
			"created_at":  e.CreatedAt.Format("2006-01-02T15:04:05Z"),
		})
	}

	result := map[string]any{
		"workspace_id": wsID,
		"count":        len(items),
		"entries":      items,
	}

	return jsonResult(result)
}

func (s *Server) handleSyncQueueClear(ctx context.Context, req mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
	wsID := stringArg(req, "workspace_id", defaultWorkspaceID)

	cleared, err := s.syncQueue.DeleteByWorkspace(ctx, wsID)
	if err != nil {
		return errResult(err), nil
	}

	result := map[string]any{
		"workspace_id": wsID,
		"cleared":      cleared,
	}

	return jsonResult(result)
}

func (s *Server) handleSyncResync(ctx context.Context, req mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
	wsID := stringArg(req, "workspace_id", defaultWorkspaceID)

	if err := s.engine.ForceResync(ctx, wsID); err != nil {
		return errResult(err), nil
	}

	result := map[string]any{
		"workspace_id": wsID,
		"resync":       true,
		"message":      "queue cleared, sync restarted with seq=0",
	}

	return jsonResult(result)
}

func (s *Server) handleSyncPause(_ context.Context, req mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
	wsID := stringArg(req, "workspace_id", defaultWorkspaceID)

	if err := s.engine.Pause(wsID); err != nil {
		return errResult(err), nil
	}

	result := map[string]any{
		"workspace_id": wsID,
		"paused":       true,
		"message":      "sync paused; local queue continues to accumulate writes",
	}

	return jsonResult(result)
}

func (s *Server) handleSyncResume(_ context.Context, req mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
	wsID := stringArg(req, "workspace_id", defaultWorkspaceID)

	if err := s.engine.Resume(wsID); err != nil {
		return errResult(err), nil
	}

	result := map[string]any{
		"workspace_id": wsID,
		"resumed":      true,
		"message":      "sync resumed; push → pull → subscribe cycle restarted",
	}

	return jsonResult(result)
}

func (s *Server) handleSyncDisconnectStream(_ context.Context, req mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
	wsID := stringArg(req, "workspace_id", defaultWorkspaceID)

	if err := s.engine.DisconnectStream(wsID); err != nil {
		return errResult(err), nil
	}

	result := map[string]any{
		"workspace_id": wsID,
		"disconnected": true,
		"message":      "subscribe stream dropped; engine will reconnect via exponential backoff",
	}

	return jsonResult(result)
}

func stringArg(req mcplib.CallToolRequest, name, fallback string) string {
	if v, ok := req.GetArguments()[name]; ok {
		if s, ok := v.(string); ok && s != "" {
			return s
		}
	}
	return fallback
}

func intArg(req mcplib.CallToolRequest, name string, fallback int) int {
	if v, ok := req.GetArguments()[name]; ok {
		switch n := v.(type) {
		case float64:
			return int(n)
		case int:
			return n
		}
	}
	return fallback
}

func jsonResult(v any) (*mcplib.CallToolResult, error) {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return errResult(fmt.Errorf("marshal result: %w", err)), nil
	}
	return textResult(string(data)), nil
}
