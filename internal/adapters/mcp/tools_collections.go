package mcp

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	mcplib "github.com/mark3labs/mcp-go/mcp"

	"github.com/tetiva-app/client/internal/domain/entities"
	"github.com/tetiva-app/client/internal/domain/usecase/collection"
)

func (s *Server) registerCollectionTools() {
	s.mcp.AddTool(
		mcplib.NewTool("list_collections",
			mcplib.WithDescription("List all collections in a workspace"),
			mcplib.WithString("workspace_id",
				mcplib.Description("Workspace UUID (defaults to default workspace)"),
			),
		),
		s.handleListCollections,
	)

	s.mcp.AddTool(
		mcplib.NewTool("get_collection",
			mcplib.WithDescription("Get a collection by ID (full object with scripts and auth)"),
			mcplib.WithString("id", mcplib.Required(),
				mcplib.Description("Collection UUID"),
			),
		),
		s.handleGetCollection,
	)

	s.mcp.AddTool(
		mcplib.NewTool("create_collection",
			mcplib.WithDescription("Create a new collection. Supports nesting via parent_id and optional scripts/auth."),
			mcplib.WithString("name", mcplib.Required(),
				mcplib.Description("Collection name"),
			),
			mcplib.WithString("description",
				mcplib.Description("Collection description"),
			),
			mcplib.WithString("workspace_id",
				mcplib.Description("Workspace UUID (defaults to default workspace)"),
			),
			mcplib.WithString("parent_id",
				mcplib.Description("Parent collection UUID (omit for top-level)"),
			),
			mcplib.WithString("pre_script",
				mcplib.Description("JavaScript pre-request script inherited by descendants"),
			),
			mcplib.WithString("post_script",
				mcplib.Description("JavaScript post-response script inherited by descendants"),
			),
			mcplib.WithString("auth_type",
				mcplib.Description("Auth type: none, basic, bearer, api_key (default: none). 'inherit' is not valid for collections."),
			),
			mcplib.WithString("auth_data",
				mcplib.Description("Auth data as JSON string (shape depends on auth_type)"),
			),
		),
		s.handleCreateCollection,
	)

	s.mcp.AddTool(
		mcplib.NewTool("update_collection",
			mcplib.WithDescription("Update an existing collection (name, description, scripts, auth). Uses optimistic locking — pass the current version."),
			mcplib.WithString("id", mcplib.Required(),
				mcplib.Description("Collection UUID"),
			),
			mcplib.WithNumber("version", mcplib.Required(),
				mcplib.Description("Current version for optimistic locking"),
			),
			mcplib.WithString("name", mcplib.Required(),
				mcplib.Description("Collection name"),
			),
			mcplib.WithString("description",
				mcplib.Description("Collection description"),
			),
			mcplib.WithString("pre_script",
				mcplib.Description("JavaScript pre-request script"),
			),
			mcplib.WithString("post_script",
				mcplib.Description("JavaScript post-response script"),
			),
			mcplib.WithString("auth_type",
				mcplib.Description("Auth type: none, basic, bearer, api_key"),
			),
			mcplib.WithString("auth_data",
				mcplib.Description("Auth data as JSON string"),
			),
		),
		s.handleUpdateCollection,
	)

	s.mcp.AddTool(
		mcplib.NewTool("move_collection",
			mcplib.WithDescription("Move a collection to a new parent (or to top level if parent_id is empty). Validates against cycles."),
			mcplib.WithString("id", mcplib.Required(),
				mcplib.Description("Collection UUID"),
			),
			mcplib.WithNumber("version", mcplib.Required(),
				mcplib.Description("Current version for optimistic locking"),
			),
			mcplib.WithString("parent_id",
				mcplib.Description("New parent collection UUID. Omit or leave empty to move to top level."),
			),
		),
		s.handleMoveCollection,
	)

	s.mcp.AddTool(
		mcplib.NewTool("delete_collection",
			mcplib.WithDescription("Soft-delete a collection by ID (cascades to descendants)"),
			mcplib.WithString("id", mcplib.Required(),
				mcplib.Description("Collection UUID"),
			),
		),
		s.handleDeleteCollection,
	)
}

func (s *Server) handleListCollections(ctx context.Context, req mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
	wsID := uuidArg(req, "workspace_id", defaultWorkspaceID)

	cols, err := s.colUC.List(ctx, collection.ListOpt{WorkspaceID: wsID})
	if err != nil {
		return errResult(err), nil
	}

	items := make([]map[string]any, 0, len(cols))
	for _, c := range cols {
		items = append(items, map[string]any{
			"id":          c.ID.String(),
			"name":        c.Name,
			"description": c.Description,
			"parent_id":   uuidPtrStr(c.ParentID),
			"version":     c.Version,
			"is_delete":   c.IsDelete,
			"created_at":  c.CreatedAt.Format("2006-01-02T15:04:05Z"),
		})
	}

	return jsonResult(map[string]any{
		"count":       len(items),
		"collections": items,
	})
}

func (s *Server) handleGetCollection(ctx context.Context, req mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
	id, err := uuidArgRequired(req, "id")
	if err != nil {
		return errResult(err), nil
	}

	c, err := s.colUC.GetByID(ctx, id)
	if err != nil {
		return errResult(err), nil
	}

	return jsonResult(serializeCollection(c))
}

func (s *Server) handleCreateCollection(ctx context.Context, req mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
	wsID := uuidArg(req, "workspace_id", defaultWorkspaceID)

	parentID, err := uuidPtrArg(req, "parent_id")
	if err != nil {
		return errResult(fmt.Errorf("parent_id: %w", err)), nil
	}

	authType := entities.AuthType(stringArg(req, "auth_type", ""))
	if authType == "" {
		authType = entities.AuthTypeNone
	}

	c, err := s.colUC.Create(ctx, collection.Create{
		Name:        stringArg(req, "name", ""),
		ParentID:    parentID,
		Description: stringArg(req, "description", ""),
		PreScript:   stringArg(req, "pre_script", ""),
		PostScript:  stringArg(req, "post_script", ""),
		AuthType:    authType,
		AuthData:    stringArg(req, "auth_data", ""),
	}, collection.CreateOpt{
		UserID:      mcpUserID,
		WorkspaceID: wsID,
	})
	if err != nil {
		return errResult(err), nil
	}

	result := serializeCollection(c)
	result["message"] = "collection created"
	return jsonResult(result)
}

func (s *Server) handleUpdateCollection(ctx context.Context, req mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
	id, err := uuidArgRequired(req, "id")
	if err != nil {
		return errResult(err), nil
	}

	version := intArg(req, "version", 0)
	if version < 1 {
		return errResult(fmt.Errorf("version is required (>= 1)")), nil
	}

	authType := entities.AuthType(stringArg(req, "auth_type", ""))
	if authType == "" {
		authType = entities.AuthTypeNone
	}

	c, err := s.colUC.Edit(ctx, collection.Edit{
		Name:        stringArg(req, "name", ""),
		Description: stringArg(req, "description", ""),
		PreScript:   stringArg(req, "pre_script", ""),
		PostScript:  stringArg(req, "post_script", ""),
		AuthType:    authType,
		AuthData:    stringArg(req, "auth_data", ""),
	}, collection.EditOpt{
		CollectionID: id,
		UserID:       mcpUserID,
		Version:      version,
	})
	if err != nil {
		return errResult(err), nil
	}

	result := serializeCollection(c)
	result["message"] = "collection updated"
	return jsonResult(result)
}

func (s *Server) handleMoveCollection(ctx context.Context, req mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
	id, err := uuidArgRequired(req, "id")
	if err != nil {
		return errResult(err), nil
	}

	version := intArg(req, "version", 0)
	if version < 1 {
		return errResult(fmt.Errorf("version is required (>= 1)")), nil
	}

	parentID, err := uuidPtrArg(req, "parent_id")
	if err != nil {
		return errResult(fmt.Errorf("parent_id: %w", err)), nil
	}

	c, err := s.colUC.Move(ctx, collection.MoveOpt{
		CollectionID:   id,
		TargetParentID: parentID,
		UserID:         mcpUserID,
		Version:        version,
	})
	if err != nil {
		return errResult(err), nil
	}

	result := serializeCollection(c)
	result["message"] = "collection moved"
	return jsonResult(result)
}

func (s *Server) handleDeleteCollection(ctx context.Context, req mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
	id, err := uuidArgRequired(req, "id")
	if err != nil {
		return errResult(err), nil
	}

	// Fetch current version for optimistic locking.
	c, err := s.colUC.GetByID(ctx, id)
	if err != nil {
		return errResult(err), nil
	}

	if err := s.colUC.Delete(ctx, collection.DeleteOpt{
		CollectionID: id,
		UserID:       mcpUserID,
		Version:      c.Version,
	}); err != nil {
		return errResult(err), nil
	}

	return jsonResult(map[string]any{
		"id":      id.String(),
		"deleted": true,
	})
}

func uuidArg(req mcplib.CallToolRequest, name, fallback string) uuid.UUID {
	s := stringArg(req, name, fallback)
	id, err := uuid.Parse(s)
	if err != nil {
		id, _ = uuid.Parse(fallback)
	}
	return id
}

func uuidArgRequired(req mcplib.CallToolRequest, name string) (uuid.UUID, error) {
	s := stringArg(req, name, "")
	if s == "" {
		return uuid.Nil, fmt.Errorf("%s is required", name)
	}
	return uuid.Parse(s)
}

func uuidPtrStr(id *uuid.UUID) string {
	if id == nil {
		return ""
	}
	return id.String()
}
