package mcp

import (
	"context"
	"fmt"

	mcplib "github.com/mark3labs/mcp-go/mcp"

	"github.com/tetiva-app/client/internal/domain/entities"
	"github.com/tetiva-app/client/internal/domain/usecase/request"
)

func (s *Server) registerRequestTools() {
	s.mcp.AddTool(
		mcplib.NewTool("list_requests",
			mcplib.WithDescription("List all requests in a collection"),
			mcplib.WithString("collection_id", mcplib.Required(),
				mcplib.Description("Collection UUID"),
			),
		),
		s.handleListRequests,
	)

	s.mcp.AddTool(
		mcplib.NewTool("get_request",
			mcplib.WithDescription("Get a request by ID with all fields (headers, body, auth, scripts, grpc, graphql)"),
			mcplib.WithString("id", mcplib.Required(),
				mcplib.Description("Request UUID"),
			),
		),
		s.handleGetRequest,
	)

	s.mcp.AddTool(
		mcplib.NewTool("create_request",
			mcplib.WithDescription("Create a request in a collection. Supports HTTP, gRPC, GraphQL and optional headers/auth/scripts."),
			mcplib.WithString("collection_id", mcplib.Required(),
				mcplib.Description("Collection UUID to add the request to"),
			),
			mcplib.WithString("name", mcplib.Required(),
				mcplib.Description("Request name"),
			),
			mcplib.WithString("protocol",
				mcplib.Description("Protocol: http, grpc, graphql (default: http)"),
			),
			mcplib.WithString("method",
				mcplib.Description("HTTP method: GET, POST, PUT, PATCH, DELETE, HEAD, OPTIONS (default: GET, ignored for non-HTTP)"),
			),
			mcplib.WithString("url",
				mcplib.Description("Request URL (HTTP/GraphQL) or host (gRPC)"),
			),
			mcplib.WithString("body",
				mcplib.Description("Request body (HTTP/gRPC message)"),
			),
			mcplib.WithString("body_type",
				mcplib.Description("Body type: none, json, xml, form, binary, raw (default: none)"),
			),
			mcplib.WithArray("headers",
				mcplib.Description("Array of {key, value, enabled} header items"),
			),
			mcplib.WithString("auth_type",
				mcplib.Description("Auth type: none, basic, bearer, api_key, inherit (default: inherit)"),
			),
			mcplib.WithString("auth_data",
				mcplib.Description("Auth payload as JSON string (shape depends on auth_type)"),
			),
			mcplib.WithString("pre_script",
				mcplib.Description("JavaScript pre-request script"),
			),
			mcplib.WithString("post_script",
				mcplib.Description("JavaScript post-response script"),
			),
			mcplib.WithString("grpc_service",
				mcplib.Description("Fully-qualified gRPC service name"),
			),
			mcplib.WithString("grpc_method",
				mcplib.Description("gRPC method name"),
			),
			mcplib.WithString("grpc_proto_path",
				mcplib.Description("Filesystem path to .proto file or directory"),
			),
			mcplib.WithObject("grpc_metadata",
				mcplib.Description("gRPC metadata as {key: [values]} or [{key,value}]"),
			),
			mcplib.WithString("graphql_query",
				mcplib.Description("GraphQL query or mutation"),
			),
			mcplib.WithString("graphql_variables",
				mcplib.Description("GraphQL variables JSON"),
			),
			mcplib.WithString("graphql_schema_path",
				mcplib.Description("Filesystem path to GraphQL SDL schema file"),
			),
			mcplib.WithString("graphql_operation",
				mcplib.Description("GraphQL operation name"),
			),
		),
		s.handleCreateRequest,
	)

	s.mcp.AddTool(
		mcplib.NewTool("update_request",
			mcplib.WithDescription("Update an existing request (name, url, method, body, headers, auth, scripts, grpc, graphql). Uses optimistic locking."),
			mcplib.WithString("id", mcplib.Required(),
				mcplib.Description("Request UUID"),
			),
			mcplib.WithNumber("version", mcplib.Required(),
				mcplib.Description("Current version for optimistic locking"),
			),
			mcplib.WithString("name", mcplib.Required(),
				mcplib.Description("Request name"),
			),
			mcplib.WithString("method",
				mcplib.Description("HTTP method"),
			),
			mcplib.WithString("url",
				mcplib.Description("Request URL / gRPC host"),
			),
			mcplib.WithString("body",
				mcplib.Description("Request body"),
			),
			mcplib.WithString("body_type",
				mcplib.Description("Body type"),
			),
			mcplib.WithArray("headers",
				mcplib.Description("Array of {key, value, enabled}"),
			),
			mcplib.WithString("auth_type",
				mcplib.Description("Auth type: none, basic, bearer, api_key, inherit"),
			),
			mcplib.WithString("auth_data",
				mcplib.Description("Auth payload as JSON string"),
			),
			mcplib.WithString("pre_script",
				mcplib.Description("JavaScript pre-request script"),
			),
			mcplib.WithString("post_script",
				mcplib.Description("JavaScript post-response script"),
			),
			mcplib.WithString("grpc_service",
				mcplib.Description("Fully-qualified gRPC service name"),
			),
			mcplib.WithString("grpc_method",
				mcplib.Description("gRPC method name"),
			),
			mcplib.WithString("grpc_proto_path",
				mcplib.Description("Path to .proto file or directory"),
			),
			mcplib.WithObject("grpc_metadata",
				mcplib.Description("gRPC metadata"),
			),
			mcplib.WithString("graphql_query",
				mcplib.Description("GraphQL query"),
			),
			mcplib.WithString("graphql_variables",
				mcplib.Description("GraphQL variables JSON"),
			),
			mcplib.WithString("graphql_schema_path",
				mcplib.Description("Path to GraphQL schema file"),
			),
			mcplib.WithString("graphql_operation",
				mcplib.Description("GraphQL operation name"),
			),
		),
		s.handleUpdateRequest,
	)

	s.mcp.AddTool(
		mcplib.NewTool("move_request",
			mcplib.WithDescription("Move a request to a different collection (same workspace)"),
			mcplib.WithString("id", mcplib.Required(),
				mcplib.Description("Request UUID"),
			),
			mcplib.WithNumber("version", mcplib.Required(),
				mcplib.Description("Current version for optimistic locking"),
			),
			mcplib.WithString("target_collection_id", mcplib.Required(),
				mcplib.Description("Destination collection UUID"),
			),
		),
		s.handleMoveRequest,
	)

	s.mcp.AddTool(
		mcplib.NewTool("send_request",
			mcplib.WithDescription("Execute a stored request (HTTP/gRPC/GraphQL): resolves env vars, runs scripts, returns status/headers/body and appends history"),
			mcplib.WithString("id", mcplib.Required(),
				mcplib.Description("Request UUID"),
			),
			mcplib.WithString("workspace_id",
				mcplib.Description("Workspace UUID for env-variable resolution (defaults to default workspace)"),
			),
		),
		s.handleSendRequest,
	)

	s.mcp.AddTool(
		mcplib.NewTool("delete_request",
			mcplib.WithDescription("Soft-delete a request by ID"),
			mcplib.WithString("id", mcplib.Required(),
				mcplib.Description("Request UUID"),
			),
		),
		s.handleDeleteRequest,
	)
}

func (s *Server) handleListRequests(ctx context.Context, req mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
	colID, err := uuidArgRequired(req, "collection_id")
	if err != nil {
		return errResult(err), nil
	}

	reqs, err := s.reqUC.List(ctx, request.ListOpt{CollectionID: colID})
	if err != nil {
		return errResult(err), nil
	}

	items := make([]map[string]any, 0, len(reqs))
	for _, r := range reqs {
		items = append(items, map[string]any{
			"id":       r.ID.String(),
			"name":     r.Name,
			"protocol": string(r.Protocol),
			"method":   string(r.Method),
			"url":      r.URL,
			"version":  r.Version,
		})
	}

	return jsonResult(map[string]any{
		"count":    len(items),
		"requests": items,
	})
}

func (s *Server) handleGetRequest(ctx context.Context, req mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
	id, err := uuidArgRequired(req, "id")
	if err != nil {
		return errResult(err), nil
	}

	r, err := s.reqUC.GetByID(ctx, id)
	if err != nil {
		return errResult(err), nil
	}

	return jsonResult(serializeRequest(r))
}

func (s *Server) handleCreateRequest(ctx context.Context, req mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
	colID, err := uuidArgRequired(req, "collection_id")
	if err != nil {
		return errResult(err), nil
	}

	protocol := entities.Protocol(stringArg(req, "protocol", string(entities.ProtocolHTTP)))
	if !protocol.IsValid() {
		protocol = entities.ProtocolHTTP
	}

	method := entities.HTTPMethod(stringArg(req, "method", "GET"))
	if !method.IsValid() {
		method = entities.MethodGET
	}

	bodyType := entities.BodyType(stringArg(req, "body_type", "none"))
	if !bodyType.IsValid() {
		bodyType = entities.BodyTypeNone
	}

	authType := entities.AuthType(stringArg(req, "auth_type", string(entities.AuthTypeInherit)))
	if !authType.IsValid() {
		authType = entities.AuthTypeInherit
	}

	r, err := s.reqUC.Create(ctx, request.Create{
		CollectionID:      colID,
		Name:              stringArg(req, "name", ""),
		Protocol:          protocol,
		Method:            method,
		URL:               stringArg(req, "url", ""),
		Headers:           headersFromArg(req, "headers"),
		Body:              stringArg(req, "body", ""),
		BodyType:          bodyType,
		AuthType:          authType,
		AuthData:          stringArg(req, "auth_data", ""),
		PreScript:         stringArg(req, "pre_script", ""),
		PostScript:        stringArg(req, "post_script", ""),
		GRPCService:       stringArg(req, "grpc_service", ""),
		GRPCMethod:        stringArg(req, "grpc_method", ""),
		GRPCProtoPath:     stringArg(req, "grpc_proto_path", ""),
		GRPCMetadata:      grpcMetadataFromArg(req, "grpc_metadata"),
		GraphQLQuery:      stringArg(req, "graphql_query", ""),
		GraphQLVariables:  stringArg(req, "graphql_variables", ""),
		GraphQLSchemaPath: stringArg(req, "graphql_schema_path", ""),
		GraphQLOperation:  stringArg(req, "graphql_operation", ""),
	}, request.CreateOpt{
		UserID: mcpUserID,
	})
	if err != nil {
		return errResult(err), nil
	}

	result := serializeRequest(r)
	result["message"] = "request created"
	return jsonResult(result)
}

func (s *Server) handleUpdateRequest(ctx context.Context, req mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
	id, err := uuidArgRequired(req, "id")
	if err != nil {
		return errResult(err), nil
	}

	version := intArg(req, "version", 0)
	if version < 1 {
		return errResult(fmt.Errorf("version is required (>= 1)")), nil
	}

	method := entities.HTTPMethod(stringArg(req, "method", "GET"))
	if !method.IsValid() {
		method = entities.MethodGET
	}

	bodyType := entities.BodyType(stringArg(req, "body_type", "none"))
	if !bodyType.IsValid() {
		bodyType = entities.BodyTypeNone
	}

	authType := entities.AuthType(stringArg(req, "auth_type", string(entities.AuthTypeInherit)))
	if !authType.IsValid() {
		authType = entities.AuthTypeInherit
	}

	r, err := s.reqUC.Edit(ctx, request.Edit{
		Name:              stringArg(req, "name", ""),
		Method:            method,
		URL:               stringArg(req, "url", ""),
		Headers:           headersFromArg(req, "headers"),
		Body:              stringArg(req, "body", ""),
		BodyType:          bodyType,
		AuthType:          authType,
		AuthData:          stringArg(req, "auth_data", ""),
		PreScript:         stringArg(req, "pre_script", ""),
		PostScript:        stringArg(req, "post_script", ""),
		GRPCService:       stringArg(req, "grpc_service", ""),
		GRPCMethod:        stringArg(req, "grpc_method", ""),
		GRPCProtoPath:     stringArg(req, "grpc_proto_path", ""),
		GRPCMetadata:      grpcMetadataFromArg(req, "grpc_metadata"),
		GraphQLQuery:      stringArg(req, "graphql_query", ""),
		GraphQLVariables:  stringArg(req, "graphql_variables", ""),
		GraphQLSchemaPath: stringArg(req, "graphql_schema_path", ""),
		GraphQLOperation:  stringArg(req, "graphql_operation", ""),
	}, request.EditOpt{
		RequestID: id,
		UserID:    mcpUserID,
		Version:   version,
	})
	if err != nil {
		return errResult(err), nil
	}

	result := serializeRequest(r)
	result["message"] = "request updated"
	return jsonResult(result)
}

func (s *Server) handleMoveRequest(ctx context.Context, req mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
	id, err := uuidArgRequired(req, "id")
	if err != nil {
		return errResult(err), nil
	}

	version := intArg(req, "version", 0)
	if version < 1 {
		return errResult(fmt.Errorf("version is required (>= 1)")), nil
	}

	targetID, err := uuidArgRequired(req, "target_collection_id")
	if err != nil {
		return errResult(err), nil
	}

	r, err := s.reqUC.Move(ctx, request.MoveOpt{
		RequestID:          id,
		TargetCollectionID: targetID,
		UserID:             mcpUserID,
		Version:            version,
	})
	if err != nil {
		return errResult(err), nil
	}

	result := serializeRequest(r)
	result["message"] = "request moved"
	return jsonResult(result)
}

func (s *Server) handleSendRequest(ctx context.Context, req mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
	id, err := uuidArgRequired(req, "id")
	if err != nil {
		return errResult(err), nil
	}

	wsID := uuidArg(req, "workspace_id", defaultWorkspaceID)

	resp, err := s.reqUC.Execute(ctx, id, request.ExecuteOpt{
		UserID:      mcpUserID,
		WorkspaceID: wsID,
	})
	if err != nil {
		return errResult(err), nil
	}

	if resp == nil {
		return jsonResult(map[string]any{
			"request_id": id.String(),
			"executed":   true,
			"response":   nil,
		})
	}

	out := map[string]any{
		"request_id":         id.String(),
		"executed":           true,
		"status_code":        resp.StatusCode,
		"status_text":        resp.StatusText,
		"headers":            resp.Headers,
		"body":               resp.Body,
		"size_bytes":         resp.Size,
		"duration_ms":        resp.Duration.Milliseconds(),
		"protocol":           string(resp.Protocol),
		"is_binary":          resp.IsBinary,
		"binary_path":        resp.BinaryPath,
		"suggested_filename": resp.SuggestedFilename,
	}
	if resp.ScriptResult != nil {
		out["script_result"] = resp.ScriptResult
	}
	return jsonResult(out)
}

func (s *Server) handleDeleteRequest(ctx context.Context, req mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
	id, err := uuidArgRequired(req, "id")
	if err != nil {
		return errResult(err), nil
	}

	// Fetch current version for optimistic locking.
	r, err := s.reqUC.GetByID(ctx, id)
	if err != nil {
		return errResult(err), nil
	}

	if err := s.reqUC.Delete(ctx, request.DeleteOpt{
		RequestID: id,
		UserID:    mcpUserID,
		Version:   r.Version,
	}); err != nil {
		return errResult(err), nil
	}

	return jsonResult(map[string]any{
		"id":      id.String(),
		"deleted": true,
	})
}
