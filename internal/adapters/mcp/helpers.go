package mcp

import (
	"github.com/google/uuid"
	mcplib "github.com/mark3labs/mcp-go/mcp"

	"github.com/tetiva-app/client/internal/domain/entities"
)

// uuidPtrArg parses a UUID argument that may be empty. Returns nil if absent/empty.
func uuidPtrArg(req mcplib.CallToolRequest, name string) (*uuid.UUID, error) {
	raw := stringArg(req, name, "")
	if raw == "" {
		return nil, nil
	}
	id, err := uuid.Parse(raw)
	if err != nil {
		return nil, err
	}
	return &id, nil
}

// headersFromArg extracts a headers array from arguments.
// Expected format: [{"key": "X-Foo", "value": "bar", "enabled": true}, ...]
func headersFromArg(req mcplib.CallToolRequest, name string) []entities.HeaderItem {
	v, ok := req.GetArguments()[name]
	if !ok {
		return nil
	}
	arr, ok := v.([]any)
	if !ok {
		return nil
	}
	out := make([]entities.HeaderItem, 0, len(arr))
	for _, item := range arr {
		obj, ok := item.(map[string]any)
		if !ok {
			continue
		}
		h := entities.HeaderItem{Enabled: true}
		if k, ok := obj["key"].(string); ok {
			h.Key = k
		}
		if val, ok := obj["value"].(string); ok {
			h.Value = val
		}
		if en, ok := obj["enabled"].(bool); ok {
			h.Enabled = en
		}
		if h.Key == "" {
			continue
		}
		out = append(out, h)
	}
	return out
}

// grpcMetadataFromArg extracts gRPC metadata (map[string][]string) from arguments.
// Accepts either [{"key","value"}, ...] or {"key": ["v1","v2"]}.
func grpcMetadataFromArg(req mcplib.CallToolRequest, name string) map[string][]string {
	v, ok := req.GetArguments()[name]
	if !ok {
		return nil
	}
	out := make(map[string][]string)
	switch val := v.(type) {
	case map[string]any:
		for k, raw := range val {
			if arr, ok := raw.([]any); ok {
				for _, s := range arr {
					if sv, ok := s.(string); ok {
						out[k] = append(out[k], sv)
					}
				}
			}
		}
	case []any:
		for _, item := range val {
			obj, ok := item.(map[string]any)
			if !ok {
				continue
			}
			k, _ := obj["key"].(string)
			sv, _ := obj["value"].(string)
			if k == "" {
				continue
			}
			out[k] = append(out[k], sv)
		}
	}
	return out
}

func serializeCollection(c *entities.Collection) map[string]any {
	return map[string]any{
		"id":           c.ID.String(),
		"workspace_id": c.WorkspaceID.String(),
		"parent_id":    uuidPtrStr(c.ParentID),
		"name":         c.Name,
		"description":  c.Description,
		"auth_type":    string(c.AuthType),
		"auth_data":    c.AuthData,
		"pre_script":   c.PreScript,
		"post_script":  c.PostScript,
		"sort_order":   c.SortOrder,
		"version":      c.Version,
		"is_delete":    c.IsDelete,
		"created_at":   c.CreatedAt.Format("2006-01-02T15:04:05Z"),
		"updated_at":   c.UpdatedAt.Format("2006-01-02T15:04:05Z"),
	}
}

func serializeRequest(r *entities.Request) map[string]any {
	return map[string]any{
		"id":                  r.ID.String(),
		"collection_id":       r.CollectionID.String(),
		"name":                r.Name,
		"protocol":            string(r.Protocol),
		"method":              string(r.Method),
		"url":                 r.URL,
		"headers":             r.Headers,
		"body":                r.Body,
		"body_type":           string(r.BodyType),
		"auth_type":           string(r.AuthType),
		"auth_data":           r.AuthData,
		"pre_script":          r.PreScript,
		"post_script":         r.PostScript,
		"grpc_service":        r.GRPCService,
		"grpc_method":         r.GRPCMethod,
		"grpc_proto_path":     r.GRPCProtoPath,
		"grpc_metadata":       r.GRPCMetadata,
		"graphql_query":       r.GraphQLQuery,
		"graphql_variables":   r.GraphQLVariables,
		"graphql_schema_path": r.GraphQLSchemaPath,
		"graphql_operation":   r.GraphQLOperation,
		"sort_order":          r.SortOrder,
		"version":             r.Version,
		"is_delete":           r.IsDelete,
		"created_at":          r.CreatedAt.Format("2006-01-02T15:04:05Z"),
		"updated_at":          r.UpdatedAt.Format("2006-01-02T15:04:05Z"),
	}
}
