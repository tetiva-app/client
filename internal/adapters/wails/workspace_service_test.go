package wails

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tetiva-app/client/internal/adapters/wails/dto"
	"github.com/tetiva-app/client/internal/domain"
	"github.com/tetiva-app/client/internal/domain/entities"
	"github.com/tetiva-app/client/internal/domain/usecase/workspace"
)

// stubWorkspaceUsecase is an injectable workspace.Usecase double.
// Each test sets only the function fields it needs; unused methods panic.
type stubWorkspaceUsecase struct {
	listFn      func(context.Context) ([]*entities.Workspace, error)
	createFn    func(context.Context, workspace.Create, workspace.CreateOpt) (*entities.Workspace, error)
	editFn      func(context.Context, workspace.Edit, workspace.EditOpt) (*entities.Workspace, error)
	deleteFn    func(context.Context, workspace.DeleteOpt) error
	getByIDFn   func(context.Context, uuid.UUID) (*entities.Workspace, error)
	getActiveFn func(context.Context) (*entities.Workspace, error)
	setActiveFn func(context.Context, workspace.SetActiveOpt) error
}

func (s *stubWorkspaceUsecase) List(ctx context.Context) ([]*entities.Workspace, error) {
	if s.listFn == nil {
		panic("listFn not set")
	}
	return s.listFn(ctx)
}

func (s *stubWorkspaceUsecase) Create(ctx context.Context, in workspace.Create, opt workspace.CreateOpt) (*entities.Workspace, error) {
	if s.createFn == nil {
		panic("createFn not set")
	}
	return s.createFn(ctx, in, opt)
}

func (s *stubWorkspaceUsecase) Edit(ctx context.Context, in workspace.Edit, opt workspace.EditOpt) (*entities.Workspace, error) {
	if s.editFn == nil {
		panic("editFn not set")
	}
	return s.editFn(ctx, in, opt)
}

func (s *stubWorkspaceUsecase) Delete(ctx context.Context, opt workspace.DeleteOpt) error {
	if s.deleteFn == nil {
		panic("deleteFn not set")
	}
	return s.deleteFn(ctx, opt)
}

func (s *stubWorkspaceUsecase) GetByID(ctx context.Context, id uuid.UUID) (*entities.Workspace, error) {
	if s.getByIDFn == nil {
		panic("getByIDFn not set")
	}
	return s.getByIDFn(ctx, id)
}

func (s *stubWorkspaceUsecase) GetActive(ctx context.Context) (*entities.Workspace, error) {
	if s.getActiveFn == nil {
		panic("getActiveFn not set")
	}
	return s.getActiveFn(ctx)
}

func (s *stubWorkspaceUsecase) SetActive(ctx context.Context, opt workspace.SetActiveOpt) error {
	if s.setActiveFn == nil {
		panic("setActiveFn not set")
	}
	return s.setActiveFn(ctx, opt)
}

func fixtureWorkspace(name string, active bool, version int) *entities.Workspace {
	now := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	return &entities.Workspace{
		ID:        uuid.New(),
		Name:      name,
		IsActive:  active,
		Version:   version,
		CreatedBy: "local_user",
		CreatedAt: now,
		UpdatedBy: "local_user",
		UpdatedAt: now,
	}
}

func TestWorkspaceService_List_Happy(t *testing.T) {
	w1 := fixtureWorkspace("alpha", true, 1)
	w2 := fixtureWorkspace("beta", false, 3)
	uc := &stubWorkspaceUsecase{
		listFn: func(_ context.Context) ([]*entities.Workspace, error) {
			return []*entities.Workspace{w1, w2}, nil
		},
	}
	svc := NewWorkspaceService(uc)

	res := svc.List()

	require.Nil(t, res.Error)
	require.Len(t, res.Data, 2)
	assert.Equal(t, w1.ID.String(), res.Data[0].ID)
	assert.Equal(t, "alpha", res.Data[0].Name)
	assert.True(t, res.Data[0].IsActive)
	assert.Equal(t, "beta", res.Data[1].Name)
	assert.Equal(t, 3, res.Data[1].Version)
}

func TestWorkspaceService_List_Error(t *testing.T) {
	uc := &stubWorkspaceUsecase{
		listFn: func(_ context.Context) ([]*entities.Workspace, error) {
			return nil, errors.New("boom")
		},
	}
	svc := NewWorkspaceService(uc)

	res := svc.List()

	require.NotNil(t, res.Error)
	assert.Equal(t, ErrCodeInternal, res.Error.Code)
}

func TestWorkspaceService_Create_Happy(t *testing.T) {
	created := fixtureWorkspace("new-ws", false, 1)
	uc := &stubWorkspaceUsecase{
		createFn: func(_ context.Context, in workspace.Create, opt workspace.CreateOpt) (*entities.Workspace, error) {
			assert.Equal(t, "new-ws", in.Name)
			assert.Equal(t, defaultUserID, opt.UserID)
			return created, nil
		},
	}
	svc := NewWorkspaceService(uc)

	res := svc.Create(dto.CreateWorkspaceRequest{Name: "new-ws"})

	require.Nil(t, res.Error)
	assert.Equal(t, created.ID.String(), res.Data.ID)
	assert.Equal(t, "new-ws", res.Data.Name)
	assert.Equal(t, 1, res.Data.Version)
}

func TestWorkspaceService_Create_Error_Validation(t *testing.T) {
	uc := &stubWorkspaceUsecase{
		createFn: func(_ context.Context, _ workspace.Create, _ workspace.CreateOpt) (*entities.Workspace, error) {
			return nil, &domain.ValidationError{Fields: map[string]string{"name": "required"}}
		},
	}
	svc := NewWorkspaceService(uc)

	res := svc.Create(dto.CreateWorkspaceRequest{Name: ""})

	require.NotNil(t, res.Error)
	assert.Equal(t, ErrCodeValidation, res.Error.Code)
	assert.Equal(t, "required", res.Error.Fields["name"])
}

func TestWorkspaceService_Edit_Happy(t *testing.T) {
	id := uuid.New()
	updated := fixtureWorkspace("renamed", true, 2)
	updated.ID = id
	uc := &stubWorkspaceUsecase{
		editFn: func(_ context.Context, in workspace.Edit, opt workspace.EditOpt) (*entities.Workspace, error) {
			assert.Equal(t, "renamed", in.Name)
			assert.Equal(t, id, opt.WorkspaceID)
			assert.Equal(t, 1, opt.Version)
			return updated, nil
		},
	}
	svc := NewWorkspaceService(uc)

	res := svc.Edit(dto.EditWorkspaceRequest{ID: id.String(), Name: "renamed", Version: 1})

	require.Nil(t, res.Error)
	assert.Equal(t, id.String(), res.Data.ID)
	assert.Equal(t, "renamed", res.Data.Name)
	assert.Equal(t, 2, res.Data.Version)
}

func TestWorkspaceService_Edit_Error_BadUUID(t *testing.T) {
	// usecase must NOT be invoked when UUID parsing fails.
	uc := &stubWorkspaceUsecase{}
	svc := NewWorkspaceService(uc)

	res := svc.Edit(dto.EditWorkspaceRequest{ID: "not-a-uuid", Name: "x", Version: 1})

	require.NotNil(t, res.Error)
	assert.Equal(t, ErrCodeValidation, res.Error.Code)
	assert.Contains(t, res.Error.Fields, "id")
}

func TestWorkspaceService_Delete_Happy(t *testing.T) {
	id := uuid.New()
	uc := &stubWorkspaceUsecase{
		deleteFn: func(_ context.Context, opt workspace.DeleteOpt) error {
			assert.Equal(t, id, opt.WorkspaceID)
			assert.Equal(t, 4, opt.Version)
			return nil
		},
	}
	svc := NewWorkspaceService(uc)

	res := svc.Delete(dto.DeleteWorkspaceRequest{ID: id.String(), Version: 4})

	require.Nil(t, res.Error)
}

func TestWorkspaceService_Delete_Error_BadUUID(t *testing.T) {
	uc := &stubWorkspaceUsecase{}
	svc := NewWorkspaceService(uc)

	res := svc.Delete(dto.DeleteWorkspaceRequest{ID: "garbage", Version: 1})

	require.NotNil(t, res.Error)
	assert.Equal(t, ErrCodeValidation, res.Error.Code)
	assert.Contains(t, res.Error.Fields, "id")
}

func TestWorkspaceService_GetActive_Happy(t *testing.T) {
	active := fixtureWorkspace("primary", true, 7)
	uc := &stubWorkspaceUsecase{
		getActiveFn: func(_ context.Context) (*entities.Workspace, error) {
			return active, nil
		},
	}
	svc := NewWorkspaceService(uc)

	res := svc.GetActive()

	require.Nil(t, res.Error)
	assert.Equal(t, active.ID.String(), res.Data.ID)
	assert.True(t, res.Data.IsActive)
	assert.Equal(t, "primary", res.Data.Name)
}

func TestWorkspaceService_GetActive_Error_NotFound(t *testing.T) {
	uc := &stubWorkspaceUsecase{
		getActiveFn: func(_ context.Context) (*entities.Workspace, error) {
			return nil, &domain.NotFoundError{Entity: "workspace", ID: "active"}
		},
	}
	svc := NewWorkspaceService(uc)

	res := svc.GetActive()

	require.NotNil(t, res.Error)
	assert.Equal(t, ErrCodeNotFound, res.Error.Code)
}

func TestWorkspaceService_SetActive_Happy(t *testing.T) {
	id := uuid.New()
	uc := &stubWorkspaceUsecase{
		setActiveFn: func(_ context.Context, opt workspace.SetActiveOpt) error {
			assert.Equal(t, id, opt.WorkspaceID)
			return nil
		},
	}
	svc := NewWorkspaceService(uc)

	res := svc.SetActive(dto.SetActiveWorkspaceRequest{WorkspaceID: id.String()})

	require.Nil(t, res.Error)
}

func TestWorkspaceService_SetActive_Error_BadUUID(t *testing.T) {
	uc := &stubWorkspaceUsecase{}
	svc := NewWorkspaceService(uc)

	res := svc.SetActive(dto.SetActiveWorkspaceRequest{WorkspaceID: "nope"})

	require.NotNil(t, res.Error)
	assert.Equal(t, ErrCodeValidation, res.Error.Code)
	assert.Contains(t, res.Error.Fields, "workspaceId")
}
