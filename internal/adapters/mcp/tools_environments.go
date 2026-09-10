package mcp

import (
	"context"

	mcplib "github.com/mark3labs/mcp-go/mcp"

	"github.com/tetiva-app/client/internal/domain/usecase/environment"
)

func (s *Server) registerEnvironmentTools() {
	s.mcp.AddTool(
		mcplib.NewTool("list_environments",
			mcplib.WithDescription("List all environments in a workspace"),
			mcplib.WithString("workspace_id",
				mcplib.Description("Workspace UUID (defaults to default workspace)"),
			),
		),
		s.handleListEnvironments,
	)

	s.mcp.AddTool(
		mcplib.NewTool("get_environment",
			mcplib.WithDescription("Get an environment by ID"),
			mcplib.WithString("id", mcplib.Required(),
				mcplib.Description("Environment UUID"),
			),
		),
		s.handleGetEnvironment,
	)

	s.mcp.AddTool(
		mcplib.NewTool("create_environment",
			mcplib.WithDescription("Create a new environment"),
			mcplib.WithString("name", mcplib.Required(),
				mcplib.Description("Environment name"),
			),
			mcplib.WithString("workspace_id",
				mcplib.Description("Workspace UUID (defaults to default workspace)"),
			),
		),
		s.handleCreateEnvironment,
	)

	s.mcp.AddTool(
		mcplib.NewTool("delete_environment",
			mcplib.WithDescription("Soft-delete an environment by ID"),
			mcplib.WithString("id", mcplib.Required(),
				mcplib.Description("Environment UUID"),
			),
		),
		s.handleDeleteEnvironment,
	)
}

func (s *Server) handleListEnvironments(ctx context.Context, req mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
	wsID := uuidArg(req, "workspace_id", defaultWorkspaceID)

	envs, err := s.envUC.List(ctx, environment.ListOpt{WorkspaceID: wsID})
	if err != nil {
		return errResult(err), nil
	}

	items := make([]map[string]any, 0, len(envs))
	for _, e := range envs {
		items = append(items, map[string]any{
			"id":        e.ID.String(),
			"name":      e.Name,
			"is_active": e.IsActive,
			"version":   e.Version,
			"is_delete": e.IsDelete,
		})
	}

	return jsonResult(map[string]any{
		"count":        len(items),
		"environments": items,
	})
}

func (s *Server) handleGetEnvironment(ctx context.Context, req mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
	id, err := uuidArgRequired(req, "id")
	if err != nil {
		return errResult(err), nil
	}

	e, err := s.envUC.GetByID(ctx, id)
	if err != nil {
		return errResult(err), nil
	}

	vars, err := s.envUC.ListVariables(ctx, id)
	if err != nil {
		return errResult(err), nil
	}
	varItems := make([]map[string]any, 0, len(vars))
	for _, v := range vars {
		varItems = append(varItems, map[string]any{
			"id":        v.ID.String(),
			"key":       v.Key,
			"value":     maskVariableValue(v.Value, v.IsSecret),
			"is_secret": v.IsSecret,
			"enabled":   v.Enabled,
		})
	}

	return jsonResult(map[string]any{
		"id":         e.ID.String(),
		"name":       e.Name,
		"is_active":  e.IsActive,
		"version":    e.Version,
		"is_delete":  e.IsDelete,
		"created_at": e.CreatedAt.Format("2006-01-02T15:04:05Z"),
		"updated_at": e.UpdatedAt.Format("2006-01-02T15:04:05Z"),
		"variables":  varItems,
	})
}

func (s *Server) handleCreateEnvironment(ctx context.Context, req mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
	wsID := uuidArg(req, "workspace_id", defaultWorkspaceID)
	name := stringArg(req, "name", "")

	e, err := s.envUC.Create(ctx, environment.Create{
		Name: name,
	}, environment.CreateOpt{
		UserID:      mcpUserID,
		WorkspaceID: wsID,
	})
	if err != nil {
		return errResult(err), nil
	}

	return jsonResult(map[string]any{
		"id":      e.ID.String(),
		"name":    e.Name,
		"version": e.Version,
		"message": "environment created",
	})
}

func (s *Server) handleDeleteEnvironment(ctx context.Context, req mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
	id, err := uuidArgRequired(req, "id")
	if err != nil {
		return errResult(err), nil
	}

	e, err := s.envUC.GetByID(ctx, id)
	if err != nil {
		return errResult(err), nil
	}

	if err := s.envUC.Delete(ctx, environment.DeleteOpt{
		EnvironmentID: id,
		UserID:        mcpUserID,
		Version:       e.Version,
	}); err != nil {
		return errResult(err), nil
	}

	return jsonResult(map[string]any{
		"id":      id.String(),
		"deleted": true,
	})
}
