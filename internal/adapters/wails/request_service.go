package wails

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/uuid"

	"github.com/tetiva-app/client/internal/adapters/wails/dto"
	"github.com/tetiva-app/client/internal/domain"
	"github.com/tetiva-app/client/internal/domain/entities"
	"github.com/tetiva-app/client/internal/domain/usecase/request"
)

// RequestService exposes Request operations to the Wails frontend.
type RequestService struct {
	uc request.Usecase
}

// NewRequestService creates a new RequestService instance.
func NewRequestService(uc request.Usecase) *RequestService {
	return &RequestService{uc: uc}
}

// Create creates a new request and returns the result.
func (s *RequestService) Create(req dto.CreateRequestRequest) Result[dto.RequestResponse] {
	ctx := context.Background()

	collectionID, err := uuid.Parse(req.CollectionID)
	if err != nil {
		return Err[dto.RequestResponse](&domain.ValidationError{
			Fields: map[string]string{"collectionId": "invalid UUID"},
		})
	}

	input := request.Create{
		CollectionID:      collectionID,
		Name:              req.Name,
		Protocol:          entities.Protocol(req.Protocol),
		Method:            entities.HTTPMethod(req.Method),
		URL:               req.URL,
		Headers:           dto.HeaderItemsToEntity(req.Headers),
		Body:              req.Body,
		BodyType:          entities.BodyType(req.BodyType),
		AuthType:          entities.AuthType(req.AuthType),
		AuthData:          req.AuthData,
		PreScript:         req.PreScript,
		PostScript:        req.PostScript,
		GRPCService:       req.GRPCService,
		GRPCMethod:        req.GRPCMethod,
		GRPCProtoPath:     req.GRPCProtoPath,
		GRPCMetadata:      req.GRPCMetadata,
		GraphQLQuery:      req.GraphQLQuery,
		GraphQLVariables:  req.GraphQLVariables,
		GraphQLSchemaPath: req.GraphQLSchemaPath,
		GraphQLOperation:  req.GraphQLOperation,
	}
	opt := request.CreateOpt{
		UserID: defaultUserID,
	}

	r, err := s.uc.Create(ctx, input, opt)
	if err != nil {
		return Err[dto.RequestResponse](err)
	}

	return OK(dto.RequestToResponse(r))
}

// GetByID retrieves a request by its ID.
func (s *RequestService) GetByID(id string) Result[dto.RequestResponse] {
	ctx := context.Background()

	parsed, err := uuid.Parse(id)
	if err != nil {
		return Err[dto.RequestResponse](&domain.ValidationError{
			Fields: map[string]string{"id": "invalid UUID"},
		})
	}

	r, err := s.uc.GetByID(ctx, parsed)
	if err != nil {
		return Err[dto.RequestResponse](err)
	}

	return OK(dto.RequestToResponse(r))
}

// List returns all requests for the given collection.
func (s *RequestService) List(collectionID string) Result[[]dto.RequestResponse] {
	ctx := context.Background()

	parsed, err := uuid.Parse(collectionID)
	if err != nil {
		return Err[[]dto.RequestResponse](&domain.ValidationError{
			Fields: map[string]string{"collectionId": "invalid UUID"},
		})
	}

	opt := request.ListOpt{
		CollectionID: parsed,
	}

	requests, err := s.uc.List(ctx, opt)
	if err != nil {
		return Err[[]dto.RequestResponse](err)
	}

	return OK(dto.RequestsToResponse(requests))
}

// Edit updates an existing request.
func (s *RequestService) Edit(req dto.EditRequestRequest) Result[dto.RequestResponse] {
	ctx := context.Background()

	requestID, err := uuid.Parse(req.ID)
	if err != nil {
		return Err[dto.RequestResponse](&domain.ValidationError{
			Fields: map[string]string{"id": "invalid UUID"},
		})
	}

	input := request.Edit{
		Name:              req.Name,
		Method:            entities.HTTPMethod(req.Method),
		URL:               req.URL,
		Headers:           dto.HeaderItemsToEntity(req.Headers),
		Body:              req.Body,
		BodyType:          entities.BodyType(req.BodyType),
		AuthType:          entities.AuthType(req.AuthType),
		AuthData:          req.AuthData,
		PreScript:         req.PreScript,
		PostScript:        req.PostScript,
		GRPCService:       req.GRPCService,
		GRPCMethod:        req.GRPCMethod,
		GRPCProtoPath:     req.GRPCProtoPath,
		GRPCMetadata:      req.GRPCMetadata,
		GraphQLQuery:      req.GraphQLQuery,
		GraphQLVariables:  req.GraphQLVariables,
		GraphQLSchemaPath: req.GraphQLSchemaPath,
		GraphQLOperation:  req.GraphQLOperation,
	}
	opt := request.EditOpt{
		RequestID: requestID,
		UserID:    defaultUserID,
		Version:   req.Version,
	}

	r, err := s.uc.Edit(ctx, input, opt)
	if err != nil {
		return Err[dto.RequestResponse](err)
	}

	return OK(dto.RequestToResponse(r))
}

// Delete soft-deletes a request by ID.
func (s *RequestService) Delete(req dto.DeleteRequestRequest) Result[Empty] {
	ctx := context.Background()

	requestID, err := uuid.Parse(req.ID)
	if err != nil {
		return Err[Empty](&domain.ValidationError{
			Fields: map[string]string{"id": "invalid UUID"},
		})
	}

	opt := request.DeleteOpt{
		RequestID: requestID,
		UserID:    defaultUserID,
		Version:   req.Version,
	}

	if err := s.uc.Delete(ctx, opt); err != nil {
		return Err[Empty](err)
	}

	return OK(Empty{})
}

// Move changes the collection of a request.
func (s *RequestService) Move(req dto.MoveRequestRequest) Result[dto.RequestResponse] {
	ctx := context.Background()

	requestID, err := uuid.Parse(req.ID)
	if err != nil {
		return Err[dto.RequestResponse](&domain.ValidationError{
			Fields: map[string]string{"id": "invalid UUID"},
		})
	}

	targetCollectionID, err := uuid.Parse(req.TargetCollectionID)
	if err != nil {
		return Err[dto.RequestResponse](&domain.ValidationError{
			Fields: map[string]string{"targetCollectionId": "invalid UUID"},
		})
	}

	opt := request.MoveOpt{
		RequestID:          requestID,
		TargetCollectionID: targetCollectionID,
		UserID:             defaultUserID,
		Version:            req.Version,
	}

	r, err := s.uc.Move(ctx, opt)
	if err != nil {
		return Err[dto.RequestResponse](err)
	}

	return OK(dto.RequestToResponse(r))
}

// Reorder updates the sort order of a request.
func (s *RequestService) Reorder(req dto.ReorderRequestRequest) Result[Empty] {
	ctx := context.Background()

	requestID, err := uuid.Parse(req.ID)
	if err != nil {
		return Err[Empty](&domain.ValidationError{
			Fields: map[string]string{"id": "invalid UUID"},
		})
	}

	if err := s.uc.Reorder(ctx, requestID, req.SortOrder); err != nil {
		return Err[Empty](err)
	}

	return OK(Empty{})
}

// SaveResponseToFile copies a binary response temp file to the user-selected destination.
// The save dialog is handled on the frontend side; this method only performs the copy.
func (s *RequestService) SaveResponseToFile(tempPath string, destPath string) Result[string] {
	const funcName = "RequestService.SaveResponseToFile"

	if destPath == "" {
		return Err[string](fmt.Errorf("%s: destination path is empty", funcName))
	}
	if !filepath.IsAbs(destPath) {
		return Err[string](fmt.Errorf("%s: destination path must be absolute", funcName))
	}

	// Resolve symlinks for both temp path and temp dir to prevent traversal
	absPath, err := filepath.Abs(tempPath)
	if err != nil {
		return Err[string](fmt.Errorf("%s: invalid path: %w", funcName, err))
	}
	resolvedPath, err := filepath.EvalSymlinks(absPath)
	if err != nil {
		return Err[string](fmt.Errorf("%s: cannot resolve path: %w", funcName, err))
	}
	resolvedTempDir, err := filepath.EvalSymlinks(os.TempDir())
	if err != nil {
		return Err[string](fmt.Errorf("%s: cannot resolve temp dir: %w", funcName, err))
	}

	// Add separator suffix to prevent /tmp-evil matching /tmp
	tempDirPrefix := resolvedTempDir
	if !strings.HasSuffix(tempDirPrefix, string(filepath.Separator)) {
		tempDirPrefix += string(filepath.Separator)
	}
	if !strings.HasPrefix(resolvedPath, tempDirPrefix) {
		return Err[string](fmt.Errorf("%s: path is not in temp directory", funcName))
	}

	// Open source (no Stat — rely on Open to avoid TOCTOU)
	src, err := os.Open(resolvedPath)
	if err != nil {
		return Err[string](fmt.Errorf("%s: failed to open temp file: %w", funcName, err))
	}
	defer func() { _ = src.Close() }()

	dst, err := os.Create(destPath)
	if err != nil {
		return Err[string](fmt.Errorf("%s: failed to create destination: %w", funcName, err))
	}

	if _, err := io.Copy(dst, src); err != nil {
		_ = dst.Close()
		return Err[string](fmt.Errorf("%s: failed to copy file: %w", funcName, err))
	}

	if err := dst.Close(); err != nil {
		return Err[string](fmt.Errorf("%s: failed to finalize destination file: %w", funcName, err))
	}

	_ = os.Remove(resolvedPath)

	return OK(destPath)
}

// Execute sends the request and returns the response.
func (s *RequestService) Execute(req dto.ExecuteRequestRequest) Result[dto.ExecuteResponseDTO] {
	ctx := context.Background()

	requestID, err := uuid.Parse(req.RequestID)
	if err != nil {
		return Err[dto.ExecuteResponseDTO](&domain.ValidationError{
			Fields: map[string]string{"requestId": "invalid UUID"},
		})
	}

	workspaceID, err := uuid.Parse(req.WorkspaceID)
	if err != nil {
		return Err[dto.ExecuteResponseDTO](&domain.ValidationError{
			Fields: map[string]string{"workspaceId": "invalid UUID"},
		})
	}

	opt := request.ExecuteOpt{
		UserID:      defaultUserID,
		WorkspaceID: workspaceID,
	}

	resp, err := s.uc.Execute(ctx, requestID, opt)
	if err != nil {
		return Err[dto.ExecuteResponseDTO](err)
	}

	return OK(dto.ResponseToExecuteDTO(resp))
}

// GenerateCurl returns a shell-ready curl command for an HTTP request.
// Pre-script runs in dry-run mode (no var persistence, no history).
func (s *RequestService) GenerateCurl(req dto.GenerateCurlRequest) Result[dto.GenerateCurlResponse] {
	ctx := context.Background()

	requestID, err := uuid.Parse(req.RequestID)
	if err != nil {
		return Err[dto.GenerateCurlResponse](&domain.ValidationError{
			Fields: map[string]string{"requestId": "invalid UUID"},
		})
	}
	workspaceID, err := uuid.Parse(req.WorkspaceID)
	if err != nil {
		return Err[dto.GenerateCurlResponse](&domain.ValidationError{
			Fields: map[string]string{"workspaceId": "invalid UUID"},
		})
	}

	cmd, scriptResult, buildErr := s.uc.BuildCurl(ctx, requestID, request.BuildCurlOpt{
		WorkspaceID: workspaceID,
	})
	if buildErr != nil {
		return Err[dto.GenerateCurlResponse](buildErr)
	}

	return OK(dto.GenerateCurlResponse{
		Command:      cmd,
		ScriptResult: dto.ScriptResultToDTO(scriptResult),
	})
}

// GRPCListServices connects to a gRPC server and returns available services.
func (s *RequestService) GRPCListServices(req dto.GRPCConnectRequest) Result[dto.GRPCSchemaResponse] {
	ctx := context.Background()
	schema, err := s.uc.GRPCListServices(ctx, request.GRPCConnectRequest{
		Host:      req.Host,
		UseTLS:    req.UseTLS,
		ProtoPath: req.ProtoPath,
	})
	if err != nil {
		return Err[dto.GRPCSchemaResponse](err)
	}
	return OK(dto.GRPCSchemaToResponse(schema))
}

// GRPCGenerateExample generates an example JSON body for a gRPC method.
func (s *RequestService) GRPCGenerateExample(req dto.GRPCGenerateExampleRequest) Result[string] {
	ctx := context.Background()
	example, err := s.uc.GRPCGenerateExample(ctx, request.GRPCConnectRequest{
		Host:      req.Host,
		UseTLS:    req.UseTLS,
		ProtoPath: req.ProtoPath,
	}, req.Service, req.Method)
	if err != nil {
		return Err[string](err)
	}
	return OK(example)
}

// GRPCGetProtoDefinition returns the proto definition for a gRPC method.
func (s *RequestService) GRPCGetProtoDefinition(req dto.GRPCGetProtoDefinitionRequest) Result[string] {
	ctx := context.Background()
	proto, err := s.uc.GRPCGetProtoDefinition(ctx, request.GRPCConnectRequest{
		Host:      req.Host,
		UseTLS:    req.UseTLS,
		ProtoPath: req.ProtoPath,
	}, req.Service, req.Method)
	if err != nil {
		return Err[string](err)
	}
	return OK(proto)
}

func singleValueHeaders(h map[string]string) map[string][]string {
	result := make(map[string][]string, len(h))
	for k, v := range h {
		result[k] = []string{v}
	}
	return result
}

// GraphQLIntrospect loads schema from endpoint or file.
func (s *RequestService) GraphQLIntrospect(req dto.GraphQLIntrospectRequest) Result[dto.GraphQLSchemaResponse] {
	ctx := context.Background()
	domainReq := request.GraphQLIntrospectRequest{
		Endpoint:   req.Endpoint,
		SchemaPath: req.SchemaPath,
		Headers:    singleValueHeaders(req.Headers),
	}
	schema, err := s.uc.GraphQLIntrospect(ctx, domainReq)
	if err != nil {
		return Err[dto.GraphQLSchemaResponse](err)
	}
	return OK(dto.GraphQLSchemaToResponse(schema))
}

// GraphQLGenerateExample generates an example query for an operation.
func (s *RequestService) GraphQLGenerateExample(req dto.GraphQLGenerateExampleRequest) Result[dto.GraphQLExampleResponseDTO] {
	ctx := context.Background()
	domainReq := request.GraphQLIntrospectRequest{
		Endpoint:   req.Endpoint,
		SchemaPath: req.SchemaPath,
		Headers:    singleValueHeaders(req.Headers),
	}
	example, err := s.uc.GraphQLGenerateExample(ctx, domainReq, req.OperationName)
	if err != nil {
		return Err[dto.GraphQLExampleResponseDTO](err)
	}
	return OK(dto.GraphQLExampleResponseDTO{
		Query:     example.Query,
		Variables: example.Variables,
	})
}

// DeleteDraft hard-deletes a draft request (used when closing an unsaved tab).
func (s *RequestService) DeleteDraft(req dto.DeleteDraftRequest) Result[Empty] {
	ctx := context.Background()

	id, err := uuid.Parse(req.ID)
	if err != nil {
		return Err[Empty](&domain.ValidationError{
			Fields: map[string]string{"id": "invalid UUID"},
		})
	}

	if err := s.uc.DeleteDraft(ctx, id); err != nil {
		return Err[Empty](err)
	}

	return OK(Empty{})
}

// PromoteDraft converts a draft request into a regular request with the given
// name and target collection.
func (s *RequestService) PromoteDraft(req dto.PromoteDraftRequest) Result[dto.RequestResponse] {
	ctx := context.Background()

	id, err := uuid.Parse(req.ID)
	if err != nil {
		return Err[dto.RequestResponse](&domain.ValidationError{
			Fields: map[string]string{"id": "invalid UUID"},
		})
	}

	targetColl, err := uuid.Parse(req.TargetCollectionID)
	if err != nil {
		return Err[dto.RequestResponse](&domain.ValidationError{
			Fields: map[string]string{"targetCollectionId": "invalid UUID"},
		})
	}

	promoted, err := s.uc.PromoteDraft(ctx, request.PromoteDraftOpt{
		DraftID:            id,
		Name:               req.Name,
		TargetCollectionID: targetColl,
		UserID:             defaultUserID,
		Version:            req.Version,
	})
	if err != nil {
		return Err[dto.RequestResponse](err)
	}

	return OK(dto.RequestToResponse(promoted))
}

// GraphQLGetTypeDefinition returns the SDL for a specific type.
func (s *RequestService) GraphQLGetTypeDefinition(req dto.GraphQLGetTypeDefinitionRequest) Result[string] {
	ctx := context.Background()
	domainReq := request.GraphQLIntrospectRequest{
		Endpoint:   req.Endpoint,
		SchemaPath: req.SchemaPath,
		Headers:    singleValueHeaders(req.Headers),
	}
	def, err := s.uc.GraphQLGetTypeDefinition(ctx, domainReq, req.TypeName)
	if err != nil {
		return Err[string](err)
	}
	return OK(def)
}
