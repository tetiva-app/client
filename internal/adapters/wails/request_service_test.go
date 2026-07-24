package wails

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tetiva-app/client/internal/adapters/wails/dto"
	"github.com/tetiva-app/client/internal/domain"
	"github.com/tetiva-app/client/internal/domain/entities"
	"github.com/tetiva-app/client/internal/domain/usecase/request"
)

// stubRequestUsecase is a configurable request.Usecase double; unset function
// fields panic to surface accidental misuse.
type stubRequestUsecase struct {
	createFn                   func(context.Context, request.Create, request.CreateOpt) (*entities.Request, error)
	getByIDFn                  func(context.Context, uuid.UUID) (*entities.Request, error)
	listFn                     func(context.Context, request.ListOpt) ([]*entities.Request, error)
	editFn                     func(context.Context, request.Edit, request.EditOpt) (*entities.Request, error)
	deleteFn                   func(context.Context, request.DeleteOpt) error
	reorderFn                  func(context.Context, uuid.UUID, int) error
	executeFn                  func(context.Context, uuid.UUID, request.ExecuteOpt) (*entities.Response, error)
	buildCurlFn                func(context.Context, uuid.UUID, request.BuildCurlOpt) (string, *entities.ScriptResult, error)
	moveFn                     func(context.Context, request.MoveOpt) (*entities.Request, error)
	grpcListServicesFn         func(context.Context, request.GRPCConnectRequest) (*request.GRPCSchema, error)
	grpcGenerateExampleFn      func(context.Context, request.GRPCConnectRequest, string, string) (string, error)
	grpcGetProtoDefinitionFn   func(context.Context, request.GRPCConnectRequest, string, string) (string, error)
	graphqlIntrospectFn        func(context.Context, request.GraphQLIntrospectRequest) (*request.GraphQLSchema, error)
	graphqlGenerateExampleFn   func(context.Context, request.GraphQLIntrospectRequest, string) (*request.GraphQLExampleResponse, error)
	graphqlGetTypeDefinitionFn func(context.Context, request.GraphQLIntrospectRequest, string) (string, error)
	createDraftFromHistoryFn   func(context.Context, request.CreateDraftFromHistoryOpt) (*entities.Request, error)
	deleteDraftFn              func(context.Context, uuid.UUID) error
	promoteDraftFn             func(context.Context, request.PromoteDraftOpt) (*entities.Request, error)
}

func (s *stubRequestUsecase) Create(ctx context.Context, in request.Create, opt request.CreateOpt) (*entities.Request, error) {
	if s.createFn == nil {
		panic("createFn not set")
	}
	return s.createFn(ctx, in, opt)
}

func (s *stubRequestUsecase) GetByID(ctx context.Context, id uuid.UUID) (*entities.Request, error) {
	if s.getByIDFn == nil {
		panic("getByIDFn not set")
	}
	return s.getByIDFn(ctx, id)
}

func (s *stubRequestUsecase) List(ctx context.Context, opt request.ListOpt) ([]*entities.Request, error) {
	if s.listFn == nil {
		panic("listFn not set")
	}
	return s.listFn(ctx, opt)
}

func (s *stubRequestUsecase) Edit(ctx context.Context, in request.Edit, opt request.EditOpt) (*entities.Request, error) {
	if s.editFn == nil {
		panic("editFn not set")
	}
	return s.editFn(ctx, in, opt)
}

func (s *stubRequestUsecase) Delete(ctx context.Context, opt request.DeleteOpt) error {
	if s.deleteFn == nil {
		panic("deleteFn not set")
	}
	return s.deleteFn(ctx, opt)
}

func (s *stubRequestUsecase) Reorder(ctx context.Context, id uuid.UUID, sortOrder int) error {
	if s.reorderFn == nil {
		panic("reorderFn not set")
	}
	return s.reorderFn(ctx, id, sortOrder)
}

func (s *stubRequestUsecase) Execute(ctx context.Context, id uuid.UUID, opt request.ExecuteOpt) (*entities.Response, error) {
	if s.executeFn == nil {
		panic("executeFn not set")
	}
	return s.executeFn(ctx, id, opt)
}

func (s *stubRequestUsecase) BuildCurl(ctx context.Context, id uuid.UUID, opt request.BuildCurlOpt) (string, *entities.ScriptResult, error) {
	if s.buildCurlFn == nil {
		panic("buildCurlFn not set")
	}
	return s.buildCurlFn(ctx, id, opt)
}

func (s *stubRequestUsecase) Move(ctx context.Context, opt request.MoveOpt) (*entities.Request, error) {
	if s.moveFn == nil {
		panic("moveFn not set")
	}
	return s.moveFn(ctx, opt)
}

func (s *stubRequestUsecase) GRPCListServices(ctx context.Context, req request.GRPCConnectRequest) (*request.GRPCSchema, error) {
	if s.grpcListServicesFn == nil {
		panic("grpcListServicesFn not set")
	}
	return s.grpcListServicesFn(ctx, req)
}

func (s *stubRequestUsecase) GRPCGenerateExample(ctx context.Context, req request.GRPCConnectRequest, service, method string) (string, error) {
	if s.grpcGenerateExampleFn == nil {
		panic("grpcGenerateExampleFn not set")
	}
	return s.grpcGenerateExampleFn(ctx, req, service, method)
}

func (s *stubRequestUsecase) GRPCGetProtoDefinition(ctx context.Context, req request.GRPCConnectRequest, service, method string) (string, error) {
	if s.grpcGetProtoDefinitionFn == nil {
		panic("grpcGetProtoDefinitionFn not set")
	}
	return s.grpcGetProtoDefinitionFn(ctx, req, service, method)
}

func (s *stubRequestUsecase) GraphQLIntrospect(ctx context.Context, req request.GraphQLIntrospectRequest) (*request.GraphQLSchema, error) {
	if s.graphqlIntrospectFn == nil {
		panic("graphqlIntrospectFn not set")
	}
	return s.graphqlIntrospectFn(ctx, req)
}

func (s *stubRequestUsecase) GraphQLGenerateExample(ctx context.Context, req request.GraphQLIntrospectRequest, operationName string) (*request.GraphQLExampleResponse, error) {
	if s.graphqlGenerateExampleFn == nil {
		panic("graphqlGenerateExampleFn not set")
	}
	return s.graphqlGenerateExampleFn(ctx, req, operationName)
}

func (s *stubRequestUsecase) GraphQLGetTypeDefinition(ctx context.Context, req request.GraphQLIntrospectRequest, typeName string) (string, error) {
	if s.graphqlGetTypeDefinitionFn == nil {
		panic("graphqlGetTypeDefinitionFn not set")
	}
	return s.graphqlGetTypeDefinitionFn(ctx, req, typeName)
}

func (s *stubRequestUsecase) CreateDraftFromHistory(ctx context.Context, opt request.CreateDraftFromHistoryOpt) (*entities.Request, error) {
	if s.createDraftFromHistoryFn == nil {
		panic("createDraftFromHistoryFn not set")
	}
	return s.createDraftFromHistoryFn(ctx, opt)
}

func (s *stubRequestUsecase) DeleteDraft(ctx context.Context, id uuid.UUID) error {
	if s.deleteDraftFn == nil {
		panic("deleteDraftFn not set")
	}
	return s.deleteDraftFn(ctx, id)
}

func (s *stubRequestUsecase) PromoteDraft(ctx context.Context, opt request.PromoteDraftOpt) (*entities.Request, error) {
	if s.promoteDraftFn == nil {
		panic("promoteDraftFn not set")
	}
	return s.promoteDraftFn(ctx, opt)
}

func (s *stubRequestUsecase) CleanupDrafts(_ context.Context) (int, error) {
	panic("not implemented")
}

func (s *stubRequestUsecase) ResolveWebSocket(_ context.Context, _, _ uuid.UUID, _ string) (string, map[string][]string, error) {
	return "", nil, nil
}

func fixtureRequest(name string, version int) *entities.Request {
	now := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	return &entities.Request{
		ID:           uuid.New(),
		CollectionID: uuid.New(),
		Name:         name,
		Protocol:     entities.ProtocolHTTP,
		Method:       entities.MethodGET,
		URL:          "https://example.com",
		Version:      version,
		CreatedBy:    "local_user",
		CreatedAt:    now,
		UpdatedBy:    "local_user",
		UpdatedAt:    now,
	}
}

func TestRequestService_SaveResponseToFile_Happy(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "gopher-response-test-*")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	content := []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A}
	if _, err := tmpFile.Write(content); err != nil {
		t.Fatalf("failed to write temp file: %v", err)
	}
	_ = tmpFile.Close()

	destFile, err := os.CreateTemp("", "gopher-save-test-*")
	if err != nil {
		t.Fatalf("failed to create dest file: %v", err)
	}
	_ = destFile.Close()
	destPath := destFile.Name()
	t.Cleanup(func() { _ = os.Remove(destPath) })

	svc := &RequestService{}
	result := svc.SaveResponseToFile(tmpFile.Name(), destPath)

	if result.Error != nil {
		t.Fatalf("unexpected error: %v", result.Error)
	}
	if result.Data != destPath {
		t.Errorf("expected destPath %q, got %q", destPath, result.Data)
	}

	saved, err := os.ReadFile(destPath)
	if err != nil {
		t.Fatalf("failed to read saved file: %v", err)
	}
	if len(saved) != len(content) {
		t.Errorf("expected %d bytes, got %d", len(content), len(saved))
	}

	if _, err := os.Stat(tmpFile.Name()); !os.IsNotExist(err) {
		t.Error("expected temp file to be deleted after successful save")
	}
}

func TestRequestService_SaveResponseToFile_Error_TempFileNotFound(t *testing.T) {
	svc := &RequestService{}
	resolvedTempDir, err := filepath.EvalSymlinks(os.TempDir())
	if err != nil {
		t.Fatalf("failed to resolve temp dir: %v", err)
	}
	fakePath := filepath.Join(resolvedTempDir, "nonexistent-gopher-file")

	result := svc.SaveResponseToFile(fakePath, "/tmp/dest")

	if result.Error == nil {
		t.Error("expected error for nonexistent temp file")
	}
}

func TestRequestService_SaveResponseToFile_Error_PathOutsideTempDir(t *testing.T) {
	svc := &RequestService{}
	result := svc.SaveResponseToFile("/etc/passwd", "/tmp/dest")

	if result.Error == nil {
		t.Error("expected error for path outside temp directory")
	}
}

func TestRequestService_SaveResponseToFile_Error_EmptyDestPath(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "gopher-test-*")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	_ = tmpFile.Close()
	t.Cleanup(func() { _ = os.Remove(tmpFile.Name()) })

	svc := &RequestService{}
	result := svc.SaveResponseToFile(tmpFile.Name(), "")

	if result.Error == nil {
		t.Error("expected error for empty destination path")
	}
}

func TestRequestService_SaveResponseToFile_Error_RelativeDestPath(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "gopher-test-*")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	_ = tmpFile.Close()
	t.Cleanup(func() { _ = os.Remove(tmpFile.Name()) })

	svc := &RequestService{}
	result := svc.SaveResponseToFile(tmpFile.Name(), "relative/path.bin")

	if result.Error == nil {
		t.Error("expected error for relative destination path")
	}
}

func TestRequestService_GenerateCurl_Happy(t *testing.T) {
	id := uuid.New()
	wsID := uuid.New()
	stub := &stubRequestUsecase{
		buildCurlFn: func(_ context.Context, gotID uuid.UUID, opt request.BuildCurlOpt) (string, *entities.ScriptResult, error) {
			assert.Equal(t, id, gotID)
			assert.Equal(t, wsID, opt.WorkspaceID)
			return "curl 'https://example.com'", nil, nil
		},
	}
	svc := NewRequestService(stub)

	res := svc.GenerateCurl(dto.GenerateCurlRequest{
		RequestID:   id.String(),
		WorkspaceID: wsID.String(),
	})

	require.Nil(t, res.Error)
	assert.Equal(t, "curl 'https://example.com'", res.Data.Command)
	assert.Nil(t, res.Data.ScriptResult, "scriptResult should be nil when usecase returns nil")
}

func TestRequestService_GenerateCurl_Error_InvalidRequestUUID(t *testing.T) {
	svc := NewRequestService(&stubRequestUsecase{})
	res := svc.GenerateCurl(dto.GenerateCurlRequest{
		RequestID:   "not-a-uuid",
		WorkspaceID: uuid.New().String(),
	})
	require.NotNil(t, res.Error)
	assert.Equal(t, ErrCodeValidation, res.Error.Code)
}

func TestRequestService_GenerateCurl_Error_InvalidWorkspaceUUID(t *testing.T) {
	svc := NewRequestService(&stubRequestUsecase{})
	res := svc.GenerateCurl(dto.GenerateCurlRequest{
		RequestID:   uuid.New().String(),
		WorkspaceID: "not-a-uuid",
	})
	require.NotNil(t, res.Error)
	assert.Equal(t, ErrCodeValidation, res.Error.Code)
}

func TestRequestService_GenerateCurl_Error_PropagatesUsecaseError(t *testing.T) {
	stub := &stubRequestUsecase{
		buildCurlFn: func(context.Context, uuid.UUID, request.BuildCurlOpt) (string, *entities.ScriptResult, error) {
			return "", nil, &domain.ValidationError{Fields: map[string]string{"protocol": "Copy as cURL is only supported for HTTP requests"}}
		},
	}
	svc := NewRequestService(stub)

	res := svc.GenerateCurl(dto.GenerateCurlRequest{
		RequestID:   uuid.New().String(),
		WorkspaceID: uuid.New().String(),
	})
	require.NotNil(t, res.Error)
	assert.Equal(t, ErrCodeValidation, res.Error.Code)
	assert.NotEmpty(t, res.Error.Fields["protocol"])
}

func TestRequestService_GenerateCurl_Happy_PassesScriptResultThrough(t *testing.T) {
	stub := &stubRequestUsecase{
		buildCurlFn: func(context.Context, uuid.UUID, request.BuildCurlOpt) (string, *entities.ScriptResult, error) {
			return "curl 'x'", &entities.ScriptResult{
				PreConsole: []string{"hello"},
				Errors:     []entities.ScriptError{{Phase: "pre-script", Message: "boom"}},
			}, nil
		},
	}
	svc := NewRequestService(stub)
	res := svc.GenerateCurl(dto.GenerateCurlRequest{
		RequestID:   uuid.New().String(),
		WorkspaceID: uuid.New().String(),
	})
	require.Nil(t, res.Error)
	require.NotNil(t, res.Data.ScriptResult)
	require.Len(t, res.Data.ScriptResult.PreConsole, 1)
	assert.Equal(t, "hello", res.Data.ScriptResult.PreConsole[0])
	require.Len(t, res.Data.ScriptResult.Errors, 1)
	assert.Equal(t, "pre-script", res.Data.ScriptResult.Errors[0].Phase)
	// Nil-promotion invariant: ScriptResultToDTO must turn nil source slices
	// into non-nil empty slices so the JSON payload contains [] not null.
	assert.NotNil(t, res.Data.ScriptResult.Tests, "Tests should be non-nil empty slice")
	assert.NotNil(t, res.Data.ScriptResult.PostConsole, "PostConsole should be non-nil empty slice")
}

func TestRequestService_Create_Happy(t *testing.T) {
	collID := uuid.New()
	created := fixtureRequest("new-req", 1)
	created.CollectionID = collID
	stub := &stubRequestUsecase{
		createFn: func(_ context.Context, in request.Create, opt request.CreateOpt) (*entities.Request, error) {
			assert.Equal(t, collID, in.CollectionID)
			assert.Equal(t, "new-req", in.Name)
			assert.Equal(t, entities.ProtocolHTTP, in.Protocol)
			assert.Equal(t, entities.MethodGET, in.Method)
			assert.Equal(t, defaultUserID, opt.UserID)
			return created, nil
		},
	}
	svc := NewRequestService(stub)

	res := svc.Create(dto.CreateRequestRequest{
		CollectionID: collID.String(),
		Name:         "new-req",
		Protocol:     "http",
		Method:       "GET",
		URL:          "https://example.com",
	})

	require.Nil(t, res.Error)
	assert.Equal(t, created.ID.String(), res.Data.ID)
	assert.Equal(t, "new-req", res.Data.Name)
}

func TestRequestService_Create_Error_BadCollectionUUID(t *testing.T) {
	svc := NewRequestService(&stubRequestUsecase{})

	res := svc.Create(dto.CreateRequestRequest{
		CollectionID: "not-a-uuid",
		Name:         "x",
	})

	require.NotNil(t, res.Error)
	assert.Equal(t, ErrCodeValidation, res.Error.Code)
	assert.Contains(t, res.Error.Fields, "collectionId")
}

func TestRequestService_GetByID_Happy(t *testing.T) {
	id := uuid.New()
	r := fixtureRequest("get-me", 2)
	r.ID = id
	stub := &stubRequestUsecase{
		getByIDFn: func(_ context.Context, gotID uuid.UUID) (*entities.Request, error) {
			assert.Equal(t, id, gotID)
			return r, nil
		},
	}
	svc := NewRequestService(stub)

	res := svc.GetByID(id.String())

	require.Nil(t, res.Error)
	assert.Equal(t, id.String(), res.Data.ID)
	assert.Equal(t, "get-me", res.Data.Name)
}

func TestRequestService_GetByID_Error_BadUUID(t *testing.T) {
	svc := NewRequestService(&stubRequestUsecase{})

	res := svc.GetByID("not-a-uuid")

	require.NotNil(t, res.Error)
	assert.Equal(t, ErrCodeValidation, res.Error.Code)
	assert.Contains(t, res.Error.Fields, "id")
}

func TestRequestService_List_Happy(t *testing.T) {
	collID := uuid.New()
	r1 := fixtureRequest("a", 1)
	r2 := fixtureRequest("b", 1)
	stub := &stubRequestUsecase{
		listFn: func(_ context.Context, opt request.ListOpt) ([]*entities.Request, error) {
			assert.Equal(t, collID, opt.CollectionID)
			return []*entities.Request{r1, r2}, nil
		},
	}
	svc := NewRequestService(stub)

	res := svc.List(collID.String())

	require.Nil(t, res.Error)
	require.Len(t, res.Data, 2)
	assert.Equal(t, "a", res.Data[0].Name)
	assert.Equal(t, "b", res.Data[1].Name)
}

func TestRequestService_List_Error_BadUUID(t *testing.T) {
	svc := NewRequestService(&stubRequestUsecase{})

	res := svc.List("not-a-uuid")

	require.NotNil(t, res.Error)
	assert.Equal(t, ErrCodeValidation, res.Error.Code)
	assert.Contains(t, res.Error.Fields, "collectionId")
}

func TestRequestService_Edit_Happy(t *testing.T) {
	id := uuid.New()
	updated := fixtureRequest("renamed", 2)
	updated.ID = id
	stub := &stubRequestUsecase{
		editFn: func(_ context.Context, in request.Edit, opt request.EditOpt) (*entities.Request, error) {
			assert.Equal(t, "renamed", in.Name)
			assert.Equal(t, id, opt.RequestID)
			assert.Equal(t, 1, opt.Version)
			assert.Equal(t, defaultUserID, opt.UserID)
			return updated, nil
		},
	}
	svc := NewRequestService(stub)

	res := svc.Edit(dto.EditRequestRequest{
		ID:      id.String(),
		Name:    "renamed",
		Method:  "POST",
		URL:     "https://example.com",
		Version: 1,
	})

	require.Nil(t, res.Error)
	assert.Equal(t, id.String(), res.Data.ID)
	assert.Equal(t, "renamed", res.Data.Name)
	assert.Equal(t, 2, res.Data.Version)
}

func TestRequestService_Edit_Error_BadUUID(t *testing.T) {
	svc := NewRequestService(&stubRequestUsecase{})

	res := svc.Edit(dto.EditRequestRequest{ID: "not-a-uuid"})

	require.NotNil(t, res.Error)
	assert.Equal(t, ErrCodeValidation, res.Error.Code)
	assert.Contains(t, res.Error.Fields, "id")
}

func TestRequestService_Delete_Happy(t *testing.T) {
	id := uuid.New()
	stub := &stubRequestUsecase{
		deleteFn: func(_ context.Context, opt request.DeleteOpt) error {
			assert.Equal(t, id, opt.RequestID)
			assert.Equal(t, 7, opt.Version)
			return nil
		},
	}
	svc := NewRequestService(stub)

	res := svc.Delete(dto.DeleteRequestRequest{ID: id.String(), Version: 7})

	require.Nil(t, res.Error)
}

func TestRequestService_Delete_Error_BadUUID(t *testing.T) {
	svc := NewRequestService(&stubRequestUsecase{})

	res := svc.Delete(dto.DeleteRequestRequest{ID: "garbage", Version: 1})

	require.NotNil(t, res.Error)
	assert.Equal(t, ErrCodeValidation, res.Error.Code)
	assert.Contains(t, res.Error.Fields, "id")
}

func TestRequestService_Move_Happy(t *testing.T) {
	id := uuid.New()
	targetCollID := uuid.New()
	moved := fixtureRequest("moved", 3)
	moved.ID = id
	moved.CollectionID = targetCollID
	stub := &stubRequestUsecase{
		moveFn: func(_ context.Context, opt request.MoveOpt) (*entities.Request, error) {
			assert.Equal(t, id, opt.RequestID)
			assert.Equal(t, targetCollID, opt.TargetCollectionID)
			assert.Equal(t, 2, opt.Version)
			return moved, nil
		},
	}
	svc := NewRequestService(stub)

	res := svc.Move(dto.MoveRequestRequest{
		ID:                 id.String(),
		TargetCollectionID: targetCollID.String(),
		Version:            2,
	})

	require.Nil(t, res.Error)
	assert.Equal(t, id.String(), res.Data.ID)
	assert.Equal(t, targetCollID.String(), res.Data.CollectionID)
}

func TestRequestService_Move_Error_BadUUID(t *testing.T) {
	svc := NewRequestService(&stubRequestUsecase{})

	res := svc.Move(dto.MoveRequestRequest{
		ID:                 "not-a-uuid",
		TargetCollectionID: uuid.New().String(),
	})

	require.NotNil(t, res.Error)
	assert.Equal(t, ErrCodeValidation, res.Error.Code)
	assert.Contains(t, res.Error.Fields, "id")
}

func TestRequestService_Reorder_Happy(t *testing.T) {
	id := uuid.New()
	stub := &stubRequestUsecase{
		reorderFn: func(_ context.Context, gotID uuid.UUID, sortOrder int) error {
			assert.Equal(t, id, gotID)
			assert.Equal(t, 5, sortOrder)
			return nil
		},
	}
	svc := NewRequestService(stub)

	res := svc.Reorder(dto.ReorderRequestRequest{ID: id.String(), SortOrder: 5})

	require.Nil(t, res.Error)
}

func TestRequestService_Reorder_Error_BadUUID(t *testing.T) {
	svc := NewRequestService(&stubRequestUsecase{})

	res := svc.Reorder(dto.ReorderRequestRequest{ID: "garbage", SortOrder: 1})

	require.NotNil(t, res.Error)
	assert.Equal(t, ErrCodeValidation, res.Error.Code)
	assert.Contains(t, res.Error.Fields, "id")
}

func TestRequestService_Execute_Happy(t *testing.T) {
	id := uuid.New()
	wsID := uuid.New()
	resp := &entities.Response{
		StatusCode: 200,
		StatusText: "OK",
		URL:        "https://api.example.com",
		Protocol:   entities.ProtocolHTTP,
		Body:       "ok",
		Size:       2,
		Duration:   150 * time.Millisecond,
	}
	stub := &stubRequestUsecase{
		executeFn: func(_ context.Context, gotID uuid.UUID, opt request.ExecuteOpt) (*entities.Response, error) {
			assert.Equal(t, id, gotID)
			assert.Equal(t, wsID, opt.WorkspaceID)
			assert.Equal(t, defaultUserID, opt.UserID)
			return resp, nil
		},
	}
	svc := NewRequestService(stub)

	res := svc.Execute(dto.ExecuteRequestRequest{
		RequestID:   id.String(),
		WorkspaceID: wsID.String(),
	})

	require.Nil(t, res.Error)
	assert.Equal(t, 200, res.Data.StatusCode)
	assert.Equal(t, "OK", res.Data.StatusText)
	assert.Equal(t, int64(150), res.Data.DurationMs)
}

func TestRequestService_Execute_Error_BadUUID(t *testing.T) {
	svc := NewRequestService(&stubRequestUsecase{})

	res := svc.Execute(dto.ExecuteRequestRequest{
		RequestID:   "not-a-uuid",
		WorkspaceID: uuid.New().String(),
	})

	require.NotNil(t, res.Error)
	assert.Equal(t, ErrCodeValidation, res.Error.Code)
	assert.Contains(t, res.Error.Fields, "requestId")
}

func TestRequestService_GRPCListServices_Happy(t *testing.T) {
	stub := &stubRequestUsecase{
		grpcListServicesFn: func(_ context.Context, req request.GRPCConnectRequest) (*request.GRPCSchema, error) {
			assert.Equal(t, "localhost:9000", req.Host)
			assert.True(t, req.UseTLS)
			return &request.GRPCSchema{
				Source: "reflection",
				Services: []request.GRPCService{
					{FullName: "pkg.Svc", Methods: []request.GRPCMethodInfo{{Name: "Do"}}},
				},
			}, nil
		},
	}
	svc := NewRequestService(stub)

	res := svc.GRPCListServices(dto.GRPCConnectRequest{
		Host:   "localhost:9000",
		UseTLS: true,
	})

	require.Nil(t, res.Error)
	require.Len(t, res.Data.Services, 1)
	assert.Equal(t, "pkg.Svc", res.Data.Services[0].FullName)
}

func TestRequestService_GRPCListServices_Error_UsecaseFailure(t *testing.T) {
	stub := &stubRequestUsecase{
		grpcListServicesFn: func(context.Context, request.GRPCConnectRequest) (*request.GRPCSchema, error) {
			return nil, errors.New("dial failed")
		},
	}
	svc := NewRequestService(stub)

	res := svc.GRPCListServices(dto.GRPCConnectRequest{Host: "x"})

	require.NotNil(t, res.Error)
	assert.Equal(t, ErrCodeInternal, res.Error.Code)
}

func TestRequestService_GRPCGenerateExample_Happy(t *testing.T) {
	stub := &stubRequestUsecase{
		grpcGenerateExampleFn: func(_ context.Context, req request.GRPCConnectRequest, service, method string) (string, error) {
			assert.Equal(t, "host:1", req.Host)
			assert.Equal(t, "pkg.Svc", service)
			assert.Equal(t, "Do", method)
			return `{"foo":"bar"}`, nil
		},
	}
	svc := NewRequestService(stub)

	res := svc.GRPCGenerateExample(dto.GRPCGenerateExampleRequest{
		Host:    "host:1",
		Service: "pkg.Svc",
		Method:  "Do",
	})

	require.Nil(t, res.Error)
	assert.Equal(t, `{"foo":"bar"}`, res.Data)
}

func TestRequestService_GRPCGenerateExample_Error_UsecaseFailure(t *testing.T) {
	stub := &stubRequestUsecase{
		grpcGenerateExampleFn: func(context.Context, request.GRPCConnectRequest, string, string) (string, error) {
			return "", errors.New("boom")
		},
	}
	svc := NewRequestService(stub)

	res := svc.GRPCGenerateExample(dto.GRPCGenerateExampleRequest{Host: "x"})

	require.NotNil(t, res.Error)
	assert.Equal(t, ErrCodeInternal, res.Error.Code)
}

func TestRequestService_GRPCGetProtoDefinition_Happy(t *testing.T) {
	stub := &stubRequestUsecase{
		grpcGetProtoDefinitionFn: func(_ context.Context, req request.GRPCConnectRequest, service, method string) (string, error) {
			assert.Equal(t, "host:1", req.Host)
			assert.Equal(t, "pkg.Svc", service)
			assert.Equal(t, "Do", method)
			return "rpc Do (Req) returns (Resp);", nil
		},
	}
	svc := NewRequestService(stub)

	res := svc.GRPCGetProtoDefinition(dto.GRPCGetProtoDefinitionRequest{
		Host:    "host:1",
		Service: "pkg.Svc",
		Method:  "Do",
	})

	require.Nil(t, res.Error)
	assert.Equal(t, "rpc Do (Req) returns (Resp);", res.Data)
}

func TestRequestService_GRPCGetProtoDefinition_Error_UsecaseFailure(t *testing.T) {
	stub := &stubRequestUsecase{
		grpcGetProtoDefinitionFn: func(context.Context, request.GRPCConnectRequest, string, string) (string, error) {
			return "", errors.New("boom")
		},
	}
	svc := NewRequestService(stub)

	res := svc.GRPCGetProtoDefinition(dto.GRPCGetProtoDefinitionRequest{Host: "x"})

	require.NotNil(t, res.Error)
	assert.Equal(t, ErrCodeInternal, res.Error.Code)
}

func TestRequestService_GraphQLIntrospect_Happy(t *testing.T) {
	stub := &stubRequestUsecase{
		graphqlIntrospectFn: func(_ context.Context, req request.GraphQLIntrospectRequest) (*request.GraphQLSchema, error) {
			assert.Equal(t, "https://api/graphql", req.Endpoint)
			assert.Equal(t, []string{"Bearer t"}, req.Headers["Authorization"])
			return &request.GraphQLSchema{
				Source:  "introspection",
				Queries: []request.GraphQLOperation{{Name: "users"}},
			}, nil
		},
	}
	svc := NewRequestService(stub)

	res := svc.GraphQLIntrospect(dto.GraphQLIntrospectRequest{
		Endpoint: "https://api/graphql",
		Headers:  map[string]string{"Authorization": "Bearer t"},
	})

	require.Nil(t, res.Error)
	require.Len(t, res.Data.Queries, 1)
	assert.Equal(t, "users", res.Data.Queries[0].Name)
}

func TestRequestService_GraphQLIntrospect_Error_UsecaseFailure(t *testing.T) {
	stub := &stubRequestUsecase{
		graphqlIntrospectFn: func(context.Context, request.GraphQLIntrospectRequest) (*request.GraphQLSchema, error) {
			return nil, errors.New("introspection failed")
		},
	}
	svc := NewRequestService(stub)

	res := svc.GraphQLIntrospect(dto.GraphQLIntrospectRequest{Endpoint: "x"})

	require.NotNil(t, res.Error)
	assert.Equal(t, ErrCodeInternal, res.Error.Code)
}

func TestRequestService_GraphQLGenerateExample_Happy(t *testing.T) {
	stub := &stubRequestUsecase{
		graphqlGenerateExampleFn: func(_ context.Context, req request.GraphQLIntrospectRequest, op string) (*request.GraphQLExampleResponse, error) {
			assert.Equal(t, "https://api/graphql", req.Endpoint)
			assert.Equal(t, "users", op)
			return &request.GraphQLExampleResponse{
				Query:     "query { users { id } }",
				Variables: `{"id":"1"}`,
			}, nil
		},
	}
	svc := NewRequestService(stub)

	res := svc.GraphQLGenerateExample(dto.GraphQLGenerateExampleRequest{
		Endpoint:      "https://api/graphql",
		OperationName: "users",
	})

	require.Nil(t, res.Error)
	assert.Equal(t, "query { users { id } }", res.Data.Query)
	assert.Equal(t, `{"id":"1"}`, res.Data.Variables)
}

func TestRequestService_GraphQLGenerateExample_Error_UsecaseFailure(t *testing.T) {
	stub := &stubRequestUsecase{
		graphqlGenerateExampleFn: func(context.Context, request.GraphQLIntrospectRequest, string) (*request.GraphQLExampleResponse, error) {
			return nil, errors.New("op missing")
		},
	}
	svc := NewRequestService(stub)

	res := svc.GraphQLGenerateExample(dto.GraphQLGenerateExampleRequest{Endpoint: "x"})

	require.NotNil(t, res.Error)
	assert.Equal(t, ErrCodeInternal, res.Error.Code)
}

func TestRequestService_GraphQLGetTypeDefinition_Happy(t *testing.T) {
	stub := &stubRequestUsecase{
		graphqlGetTypeDefinitionFn: func(_ context.Context, req request.GraphQLIntrospectRequest, typeName string) (string, error) {
			assert.Equal(t, "https://api/graphql", req.Endpoint)
			assert.Equal(t, "User", typeName)
			return "type User { id: ID! }", nil
		},
	}
	svc := NewRequestService(stub)

	res := svc.GraphQLGetTypeDefinition(dto.GraphQLGetTypeDefinitionRequest{
		Endpoint: "https://api/graphql",
		TypeName: "User",
	})

	require.Nil(t, res.Error)
	assert.Equal(t, "type User { id: ID! }", res.Data)
}

func TestRequestService_GraphQLGetTypeDefinition_Error_UsecaseFailure(t *testing.T) {
	stub := &stubRequestUsecase{
		graphqlGetTypeDefinitionFn: func(context.Context, request.GraphQLIntrospectRequest, string) (string, error) {
			return "", errors.New("type missing")
		},
	}
	svc := NewRequestService(stub)

	res := svc.GraphQLGetTypeDefinition(dto.GraphQLGetTypeDefinitionRequest{Endpoint: "x"})

	require.NotNil(t, res.Error)
	assert.Equal(t, ErrCodeInternal, res.Error.Code)
}

func TestRequestService_DeleteDraft_Happy(t *testing.T) {
	deleted := uuid.Nil
	stub := &stubRequestUsecase{
		deleteDraftFn: func(_ context.Context, id uuid.UUID) error {
			deleted = id
			return nil
		},
	}
	svc := NewRequestService(stub)
	id := uuid.New()

	res := svc.DeleteDraft(dto.DeleteDraftRequest{ID: id.String()})

	require.Nil(t, res.Error)
	assert.Equal(t, id, deleted)
}

func TestRequestService_DeleteDraft_InvalidID(t *testing.T) {
	svc := NewRequestService(&stubRequestUsecase{})

	res := svc.DeleteDraft(dto.DeleteDraftRequest{ID: "x"})

	require.NotNil(t, res.Error)
	assert.Equal(t, ErrCodeValidation, res.Error.Code)
	assert.Contains(t, res.Error.Fields, "id")
}

func TestRequestService_PromoteDraft_Happy(t *testing.T) {
	targetColl := uuid.New()
	stub := &stubRequestUsecase{
		promoteDraftFn: func(_ context.Context, opt request.PromoteDraftOpt) (*entities.Request, error) {
			assert.Equal(t, "renamed", opt.Name)
			assert.Equal(t, targetColl, opt.TargetCollectionID)
			assert.Equal(t, defaultUserID, opt.UserID)
			assert.Equal(t, 1, opt.Version)
			return &entities.Request{
				ID:           opt.DraftID,
				Name:         opt.Name,
				CollectionID: opt.TargetCollectionID,
				Version:      opt.Version + 1,
			}, nil
		},
	}
	svc := NewRequestService(stub)

	res := svc.PromoteDraft(dto.PromoteDraftRequest{
		ID:                 uuid.NewString(),
		Name:               "renamed",
		TargetCollectionID: targetColl.String(),
		Version:            1,
	})

	require.Nil(t, res.Error)
	assert.Equal(t, "renamed", res.Data.Name)
	assert.Equal(t, targetColl.String(), res.Data.CollectionID)
	assert.Equal(t, 2, res.Data.Version)
}

func TestRequestService_PromoteDraft_InvalidID(t *testing.T) {
	svc := NewRequestService(&stubRequestUsecase{})

	res := svc.PromoteDraft(dto.PromoteDraftRequest{
		ID:                 "x",
		TargetCollectionID: uuid.NewString(),
	})

	require.NotNil(t, res.Error)
	assert.Equal(t, ErrCodeValidation, res.Error.Code)
	assert.Contains(t, res.Error.Fields, "id")
}

func TestRequestService_PromoteDraft_InvalidCollection(t *testing.T) {
	svc := NewRequestService(&stubRequestUsecase{})

	res := svc.PromoteDraft(dto.PromoteDraftRequest{
		ID:                 uuid.NewString(),
		TargetCollectionID: "x",
	})

	require.NotNil(t, res.Error)
	assert.Equal(t, ErrCodeValidation, res.Error.Code)
	assert.Contains(t, res.Error.Fields, "targetCollectionId")
}
