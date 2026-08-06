package mcp

import (
	"context"

	mcplib "github.com/mark3labs/mcp-go/mcp"

	"github.com/tetiva-app/client/internal/domain/usecase/environment"
)

func (s *Server) registerVariableTools() {
	s.mcp.AddTool(
		mcplib.NewTool("list_variables",
			mcplib.WithDescription("List all variables in an environment"),
			mcplib.WithString("environment_id", mcplib.Required(),
				mcplib.Description("Environment UUID"),
			),
		),
		s.handleListVariables,
	)

	s.mcp.AddTool(
		mcplib.NewTool("create_variable",
			mcplib.WithDescription("Create a new variable in an environment"),
			mcplib.WithString("environment_id", mcplib.Required(),
				mcplib.Description("Environment UUID"),
			),
			mcplib.WithString("key", mcplib.Required(),
				mcplib.Description("Variable key"),
			),
			mcplib.WithString("value",
				mcplib.Description("Variable value"),
			),
			mcplib.WithBoolean("is_secret",
				mcplib.Description("Whether the variable is secret (default: false)"),
			),
		),
		s.handleCreateVariable,
	)

	s.mcp.AddTool(
		mcplib.NewTool("delete_variable",
			mcplib.WithDescription("Delete a variable by ID"),
			mcplib.WithString("id", mcplib.Required(),
				mcplib.Description("Variable UUID"),
			),
		),
		s.handleDeleteVariable,
	)
}

func (s *Server) handleListVariables(ctx context.Context, req mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
	envID, err := uuidArgRequired(req, "environment_id")
	if err != nil {
		return errResult(err), nil
	}

	vars, err := s.envUC.ListVariables(ctx, envID)
	if err != nil {
		return errResult(err), nil
	}

	items := make([]map[string]any, 0, len(vars))
	for _, v := range vars {
		items = append(items, map[string]any{
			"id":        v.ID.String(),
			"key":       v.Key,
			"value":     maskVariableValue(v.Value, v.IsSecret),
			"is_secret": v.IsSecret,
			"enabled":   v.Enabled,
			"version":   v.Version,
		})
	}

	return jsonResult(map[string]any{
		"count":     len(items),
		"variables": items,
	})
}

func (s *Server) handleCreateVariable(ctx context.Context, req mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
	envID, err := uuidArgRequired(req, "environment_id")
	if err != nil {
		return errResult(err), nil
	}

	isSecret := false
	if v, ok := req.GetArguments()["is_secret"]; ok {
		if b, ok := v.(bool); ok {
			isSecret = b
		}
	}

	v, err := s.envUC.AddVariable(ctx, environment.AddVariable{
		EnvironmentID: envID,
		Key:           stringArg(req, "key", ""),
		Value:         stringArg(req, "value", ""),
		IsSecret:      isSecret,
	}, environment.AddVariableOpt{
		UserID: mcpUserID,
	})
	if err != nil {
		return errResult(err), nil
	}

	return jsonResult(map[string]any{
		"id":        v.ID.String(),
		"key":       v.Key,
		"value":     maskVariableValue(v.Value, v.IsSecret),
		"is_secret": v.IsSecret,
		"version":   v.Version,
		"message":   "variable created",
	})
}

func (s *Server) handleDeleteVariable(ctx context.Context, req mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
	id, err := uuidArgRequired(req, "id")
	if err != nil {
		return errResult(err), nil
	}

	if err := s.envUC.DeleteVariable(ctx, environment.DeleteVariableOpt{
		VariableID: id,
	}); err != nil {
		return errResult(err), nil
	}

	return jsonResult(map[string]any{
		"id":      id.String(),
		"deleted": true,
	})
}
