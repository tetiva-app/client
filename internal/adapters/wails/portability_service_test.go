package wails

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tetiva-app/client/internal/adapters/wails/dto"
	"github.com/tetiva-app/client/internal/domain/entities"
	"github.com/tetiva-app/client/internal/domain/usecase/collection"
	"github.com/tetiva-app/client/internal/domain/usecase/environment"
	"github.com/tetiva-app/client/internal/domain/usecase/request"
)

// Stubs live in request_service_test.go and environment_service_test.go;
// unset *Fn fields panic if the service under test wanders into them.

// minimalCollectionJSON is the smallest valid Postman Collection v2.1 payload:
// Info.Schema must contain "v2.1"; an empty Item slice is fine.
const minimalCollectionJSON = `{"info":{"name":"x","schema":"https://schema.getpostman.com/json/collection/v2.1.0/collection.json"},"item":[]}`

func TestPortabilityService_ImportCollection_Happy(t *testing.T) {
	wsID := uuid.New()
	collUC := &stubCollectionUsecase{
		createFn: func(_ context.Context, in collection.Create, opt collection.CreateOpt) (*entities.Collection, error) {
			assert.Equal(t, "x", in.Name)
			assert.Equal(t, defaultUserID, opt.UserID)
			assert.Equal(t, wsID, opt.WorkspaceID)
			return &entities.Collection{ID: uuid.New(), Name: in.Name, WorkspaceID: opt.WorkspaceID}, nil
		},
	}
	reqUC := &stubRequestUsecase{}
	envUC := &stubEnvironmentUsecase{}
	svc := NewPortabilityService(collUC, reqUC, envUC)

	res := svc.ImportCollection(dto.ImportCollectionRequest{
		Content:     minimalCollectionJSON,
		WorkspaceID: wsID.String(),
	})

	require.Nil(t, res.Error)
	assert.Equal(t, 1, res.Data.FoldersCreated)
	assert.Equal(t, 0, res.Data.RequestsCreated)
}

func TestPortabilityService_ImportCollection_Error_BadUUID(t *testing.T) {
	svc := NewPortabilityService(&stubCollectionUsecase{}, &stubRequestUsecase{}, &stubEnvironmentUsecase{})

	res := svc.ImportCollection(dto.ImportCollectionRequest{
		Content:     minimalCollectionJSON,
		WorkspaceID: "not-uuid",
	})

	require.NotNil(t, res.Error)
	assert.Equal(t, ErrCodeValidation, res.Error.Code)
	assert.Contains(t, res.Error.Fields, "workspaceId")
}

func TestPortabilityService_ImportCollection_Error_BadParentUUID(t *testing.T) {
	wsID := uuid.New()
	bad := "not-a-uuid"
	svc := NewPortabilityService(&stubCollectionUsecase{}, &stubRequestUsecase{}, &stubEnvironmentUsecase{})

	res := svc.ImportCollection(dto.ImportCollectionRequest{
		Content:     minimalCollectionJSON,
		WorkspaceID: wsID.String(),
		ParentID:    &bad,
	})

	require.NotNil(t, res.Error)
	assert.Equal(t, ErrCodeValidation, res.Error.Code)
	assert.Contains(t, res.Error.Fields, "parentId")
}

// minimalEnvironmentJSON: ImportEnvironment requires only a non-empty Name.
const minimalEnvironmentJSON = `{"id":"e-1","name":"prod","values":[]}`

func TestPortabilityService_ImportEnvironment_Happy(t *testing.T) {
	wsID := uuid.New()
	envUC := &stubEnvironmentUsecase{
		createFn: func(_ context.Context, in environment.Create, opt environment.CreateOpt) (*entities.Environment, error) {
			assert.Equal(t, "prod", in.Name)
			assert.Equal(t, defaultUserID, opt.UserID)
			assert.Equal(t, wsID, opt.WorkspaceID)
			return &entities.Environment{ID: uuid.New(), Name: in.Name, WorkspaceID: opt.WorkspaceID}, nil
		},
	}
	svc := NewPortabilityService(&stubCollectionUsecase{}, &stubRequestUsecase{}, envUC)

	res := svc.ImportEnvironment(dto.ImportEnvironmentRequest{
		Content:     minimalEnvironmentJSON,
		WorkspaceID: wsID.String(),
	})

	require.Nil(t, res.Error)
	assert.Equal(t, "prod", res.Data.EnvironmentName)
	assert.Equal(t, 0, res.Data.VariablesCreated)
}

func TestPortabilityService_ImportEnvironment_Error_BadUUID(t *testing.T) {
	svc := NewPortabilityService(&stubCollectionUsecase{}, &stubRequestUsecase{}, &stubEnvironmentUsecase{})

	res := svc.ImportEnvironment(dto.ImportEnvironmentRequest{
		Content:     minimalEnvironmentJSON,
		WorkspaceID: "not-uuid",
	})

	require.NotNil(t, res.Error)
	assert.Equal(t, ErrCodeValidation, res.Error.Code)
	assert.Contains(t, res.Error.Fields, "workspaceId")
}

// TODO: ExportCollection happy path needs a real *application.App — the Wails SaveFile dialog can't be stubbed; covered manually.

func TestPortabilityService_ExportCollection_Error_BadUUID(t *testing.T) {
	svc := NewPortabilityService(&stubCollectionUsecase{}, &stubRequestUsecase{}, &stubEnvironmentUsecase{})

	res := svc.ExportCollection(dto.ExportCollectionRequest{
		ID:          uuid.New().String(),
		WorkspaceID: "not-a-uuid",
	})

	require.NotNil(t, res.Error)
	assert.Equal(t, ErrCodeValidation, res.Error.Code)
	assert.Contains(t, res.Error.Fields, "workspaceId")
}

func TestPortabilityService_ExportCollection_Error_AppNotInitialized(t *testing.T) {
	wsID := uuid.New()
	rootID := uuid.New()
	now := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	root := &entities.Collection{
		ID:           rootID,
		WorkspaceID:  wsID,
		Name:         "root",
		AuthType:     entities.AuthTypeNone,
		AuthData:     "{}",
		GRPCMetadata: []entities.HeaderItem{},
		Version:      1,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	collUC := &stubCollectionUsecase{
		listFn: func(_ context.Context, opt collection.ListOpt) ([]*entities.Collection, error) {
			assert.Equal(t, wsID, opt.WorkspaceID)
			return []*entities.Collection{root}, nil
		},
		getByIDFn: func(_ context.Context, id uuid.UUID) (*entities.Collection, error) {
			assert.Equal(t, rootID, id)
			return root, nil
		},
	}
	reqUC := &stubRequestUsecase{
		listFn: func(_ context.Context, _ request.ListOpt) ([]*entities.Request, error) {
			return nil, nil
		},
	}
	svc := NewPortabilityService(collUC, reqUC, &stubEnvironmentUsecase{})

	res := svc.ExportCollection(dto.ExportCollectionRequest{
		ID:          rootID.String(),
		WorkspaceID: wsID.String(),
	})

	require.NotNil(t, res.Error)
	assert.Equal(t, ErrCodeInternal, res.Error.Code)
	assert.Contains(t, res.Error.Message, "app not initialized")
}

// TODO: ExportEnvironment happy path needs a real *application.App — the Wails SaveFile dialog can't be stubbed; covered manually.

func TestPortabilityService_ExportEnvironment_Error_BadUUID(t *testing.T) {
	svc := NewPortabilityService(&stubCollectionUsecase{}, &stubRequestUsecase{}, &stubEnvironmentUsecase{})

	res := svc.ExportEnvironment(dto.ExportEnvironmentRequest{ID: "not-a-uuid"})

	require.NotNil(t, res.Error)
	assert.Equal(t, ErrCodeValidation, res.Error.Code)
	assert.Contains(t, res.Error.Fields, "id")
}

func TestPortabilityService_ExportEnvironment_Error_AppNotInitialized(t *testing.T) {
	envID := uuid.New()
	now := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	env := &entities.Environment{
		ID:          envID,
		WorkspaceID: uuid.New(),
		Name:        "prod",
		Version:     1,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	envUC := &stubEnvironmentUsecase{
		getByIDFn: func(_ context.Context, id uuid.UUID) (*entities.Environment, error) {
			assert.Equal(t, envID, id)
			return env, nil
		},
		listVariablesFn: func(_ context.Context, id uuid.UUID) ([]*entities.Variable, error) {
			assert.Equal(t, envID, id)
			return nil, nil
		},
	}
	svc := NewPortabilityService(&stubCollectionUsecase{}, &stubRequestUsecase{}, envUC)

	res := svc.ExportEnvironment(dto.ExportEnvironmentRequest{ID: envID.String()})

	require.NotNil(t, res.Error)
	assert.Equal(t, ErrCodeInternal, res.Error.Code)
	assert.Contains(t, res.Error.Message, "app not initialized")
}
