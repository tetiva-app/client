package mcp

import (
	"context"

	mcplib "github.com/mark3labs/mcp-go/mcp"

	"github.com/tetiva-app/client/internal/domain/usecase/workspace"
)

func (s *Server) registerWorkspaceTools() {
	s.mcp.AddTool(
		mcplib.NewTool("list_workspaces",
			mcplib.WithDescription("List all workspaces with their active/remote/sync state"),
		),
		s.handleListWorkspaces,
	)

	s.mcp.AddTool(
		mcplib.NewTool("get_workspace",
			mcplib.WithDescription("Get a workspace by ID with full details"),
			mcplib.WithString("id", mcplib.Required(),
				mcplib.Description("Workspace UUID"),
			),
		),
		s.handleGetWorkspace,
	)

	s.mcp.AddTool(
		mcplib.NewTool("create_workspace",
			mcplib.WithDescription("Create a new local workspace"),
			mcplib.WithString("name", mcplib.Required(),
				mcplib.Description("Workspace name"),
			),
		),
		s.handleCreateWorkspace,
	)
}

func (s *Server) handleListWorkspaces(ctx context.Context, _ mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
	items, err := s.wsUC.List(ctx)
	if err != nil {
		return errResult(err), nil
	}

	out := make([]map[string]any, 0, len(items))
	for _, w := range items {
		remoteID := ""
		if w.RemoteWorkspaceID != nil {
			remoteID = *w.RemoteWorkspaceID
		}
		out = append(out, map[string]any{
			"id":                  w.ID.String(),
			"name":                w.Name,
			"is_active":           w.IsActive,
			"is_delete":           w.IsDelete,
			"remote_workspace_id": remoteID,
			"version":             w.Version,
			"created_at":          w.CreatedAt.Format("2006-01-02T15:04:05Z"),
			"updated_at":          w.UpdatedAt.Format("2006-01-02T15:04:05Z"),
		})
	}

	return jsonResult(map[string]any{
		"count":      len(out),
		"workspaces": out,
	})
}

func (s *Server) handleGetWorkspace(ctx context.Context, req mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
	id, err := uuidArgRequired(req, "id")
	if err != nil {
		return errResult(err), nil
	}

	w, err := s.wsUC.GetByID(ctx, id)
	if err != nil {
		return errResult(err), nil
	}

	remoteID := ""
	if w.RemoteWorkspaceID != nil {
		remoteID = *w.RemoteWorkspaceID
	}

	return jsonResult(map[string]any{
		"id":                  w.ID.String(),
		"name":                w.Name,
		"is_active":           w.IsActive,
		"is_delete":           w.IsDelete,
		"remote_workspace_id": remoteID,
		"version":             w.Version,
		"created_at":          w.CreatedAt.Format("2006-01-02T15:04:05Z"),
		"updated_at":          w.UpdatedAt.Format("2006-01-02T15:04:05Z"),
	})
}

func (s *Server) handleCreateWorkspace(ctx context.Context, req mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
	name := stringArg(req, "name", "")

	w, err := s.wsUC.Create(ctx,
		workspace.Create{Name: name},
		workspace.CreateOpt{UserID: mcpUserID},
	)
	if err != nil {
		return errResult(err), nil
	}

	return jsonResult(map[string]any{
		"id":         w.ID.String(),
		"name":       w.Name,
		"is_active":  w.IsActive,
		"version":    w.Version,
		"created_at": w.CreatedAt.Format("2006-01-02T15:04:05Z"),
		"message":    "workspace created",
	})
}
