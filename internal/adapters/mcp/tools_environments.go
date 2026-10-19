package mcp

import (
	"context"

	"github.com/google/uuid"
	mcplib "github.com/mark3labs/mcp-go/mcp"

	"github.com/tetiva-app/client/internal/domain/entities"
	"github.com/tetiva-app/client/internal/domain/usecase/environment"
)

func (s *Server) registerEnvironmentTools() {
	s.mcp.AddTool(
		mcplib.NewTool("list_environments",
			mcplib.WithDescription("List all environments in a workspace"),
			mcplib.WithString("workspace_id",
				mcplib.Description("Workspace UUID ("+workspaceDefaultDoc+")"),
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
				mcplib.Description("Workspace UUID ("+workspaceDefaultDoc+")"),
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

	s.mcp.AddTool(
		mcplib.NewTool("set_active_environment",
			mcplib.WithDescription("Make an environment the active one for its workspace. send_request resolves variables from the active environment, and the app switches to it too."),
			mcplib.WithString("environment_id", mcplib.Required(),
				mcplib.Description("Environment UUID"),
			),
		),
		s.handleSetActiveEnvironment,
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
		"created_at": formatTimestamp(e.CreatedAt),
		"updated_at": formatTimestamp(e.UpdatedAt),
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
	s.emitEnvChanged(e.WorkspaceID)

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
	s.emitEnvChanged(e.WorkspaceID)

	return jsonResult(map[string]any{
		"id":      id.String(),
		"deleted": true,
	})
}

func (s *Server) handleSetActiveEnvironment(ctx context.Context, req mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
	id, err := uuidArgRequired(req, "environment_id")
	if err != nil {
		return errResult(err), nil
	}

	// SetActive with an id outside the workspace deactivates every environment in it.
	e, err := s.envUC.GetByID(ctx, id)
	if err != nil {
		return errResult(err), nil
	}

	if err := s.envUC.SetActive(ctx, environment.SetActiveOpt{
		WorkspaceID:   e.WorkspaceID,
		EnvironmentID: e.ID,
	}); err != nil {
		return errResult(err), nil
	}
	s.emitEnvChanged(e.WorkspaceID)

	return jsonResult(map[string]any{
		"environment":  environmentRef(e),
		"workspace_id": e.WorkspaceID.String(),
	})
}

func (s *Server) activeEnvironment(ctx context.Context, workspaceID uuid.UUID) (map[string]any, error) {
	envs, err := s.envUC.List(ctx, environment.ListOpt{WorkspaceID: workspaceID})
	if err != nil {
		return nil, err
	}
	for _, e := range envs {
		if e.IsActive {
			return environmentRef(e), nil
		}
	}
	return nil, nil
}

func environmentRef(e *entities.Environment) map[string]any {
	return map[string]any{
		"id":   e.ID.String(),
		"name": e.Name,
	}
}
