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
)

// stubCollectionUsecase is an injectable collection.Usecase double.
// Each test sets only the function fields it needs; unset methods panic.
type stubCollectionUsecase struct {
	createFn  func(context.Context, collection.Create, collection.CreateOpt) (*entities.Collection, error)
	getByIDFn func(context.Context, uuid.UUID) (*entities.Collection, error)
	listFn    func(context.Context, collection.ListOpt) ([]*entities.Collection, error)
	editFn    func(context.Context, collection.Edit, collection.EditOpt) (*entities.Collection, error)
	deleteFn  func(context.Context, collection.DeleteOpt) error
	moveFn    func(context.Context, collection.MoveOpt) (*entities.Collection, error)
	reorderFn func(context.Context, uuid.UUID, int) error
}

func (s *stubCollectionUsecase) Create(ctx context.Context, in collection.Create, opt collection.CreateOpt) (*entities.Collection, error) {
	if s.createFn == nil {
		panic("createFn not set")
	}
	return s.createFn(ctx, in, opt)
}

func (s *stubCollectionUsecase) GetByID(ctx context.Context, id uuid.UUID) (*entities.Collection, error) {
	if s.getByIDFn == nil {
		panic("getByIDFn not set")
	}
	return s.getByIDFn(ctx, id)
}

func (s *stubCollectionUsecase) List(ctx context.Context, opt collection.ListOpt) ([]*entities.Collection, error) {
	if s.listFn == nil {
		panic("listFn not set")
	}
	return s.listFn(ctx, opt)
}

func (s *stubCollectionUsecase) Edit(ctx context.Context, in collection.Edit, opt collection.EditOpt) (*entities.Collection, error) {
	if s.editFn == nil {
		panic("editFn not set")
	}
	return s.editFn(ctx, in, opt)
}

func (s *stubCollectionUsecase) Delete(ctx context.Context, opt collection.DeleteOpt) error {
	if s.deleteFn == nil {
		panic("deleteFn not set")
	}
	return s.deleteFn(ctx, opt)
}

func (s *stubCollectionUsecase) Reorder(ctx context.Context, id uuid.UUID, sortOrder int) error {
	if s.reorderFn == nil {
		panic("reorderFn not set")
	}
	return s.reorderFn(ctx, id, sortOrder)
}

func (s *stubCollectionUsecase) Move(ctx context.Context, opt collection.MoveOpt) (*entities.Collection, error) {
	if s.moveFn == nil {
		panic("moveFn not set")
	}
	return s.moveFn(ctx, opt)
}

func fixtureCollection(name string, version int) *entities.Collection {
	now := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	return &entities.Collection{
		ID:           uuid.New(),
		WorkspaceID:  uuid.New(),
		Name:         name,
		AuthType:     entities.AuthTypeNone,
		AuthData:     "{}",
		GRPCMetadata: []entities.HeaderItem{},
		Version:      version,
		CreatedBy:    "local_user",
		CreatedAt:    now,
		UpdatedBy:    "local_user",
		UpdatedAt:    now,
	}
}

func TestCollectionService_Create_Happy(t *testing.T) {
	wsID := uuid.New()
	created := fixtureCollection("api-folder", 1)
	created.WorkspaceID = wsID
	uc := &stubCollectionUsecase{
		createFn: func(_ context.Context, in collection.Create, opt collection.CreateOpt) (*entities.Collection, error) {
			assert.Equal(t, "api-folder", in.Name)
			assert.Equal(t, defaultUserID, opt.UserID)
			assert.Equal(t, wsID, opt.WorkspaceID)
			return created, nil
		},
	}
	svc := NewCollectionService(uc)

	res := svc.Create(dto.CreateCollectionRequest{
		Name:        "api-folder",
		WorkspaceID: wsID.String(),
	})

	require.Nil(t, res.Error)
	assert.Equal(t, created.ID.String(), res.Data.ID)
	assert.Equal(t, "api-folder", res.Data.Name)
	assert.Equal(t, wsID.String(), res.Data.WorkspaceID)
}

func TestCollectionService_Create_Error_BadWorkspaceUUID(t *testing.T) {
	uc := &stubCollectionUsecase{}
	svc := NewCollectionService(uc)

	res := svc.Create(dto.CreateCollectionRequest{
		Name:        "x",
		WorkspaceID: "not-a-uuid",
	})

	require.NotNil(t, res.Error)
	assert.Equal(t, ErrCodeValidation, res.Error.Code)
	assert.Contains(t, res.Error.Fields, "workspaceId")
}

func TestCollectionService_GetByID_Happy(t *testing.T) {
	c := fixtureCollection("alpha", 2)
	uc := &stubCollectionUsecase{
		getByIDFn: func(_ context.Context, id uuid.UUID) (*entities.Collection, error) {
			assert.Equal(t, c.ID, id)
			return c, nil
		},
	}
	svc := NewCollectionService(uc)

	res := svc.GetByID(c.ID.String())

	require.Nil(t, res.Error)
	assert.Equal(t, c.ID.String(), res.Data.ID)
	assert.Equal(t, "alpha", res.Data.Name)
	assert.Equal(t, 2, res.Data.Version)
}

func TestCollectionService_GetByID_Error_BadUUID(t *testing.T) {
	uc := &stubCollectionUsecase{}
	svc := NewCollectionService(uc)

	res := svc.GetByID("garbage")

	require.NotNil(t, res.Error)
	assert.Equal(t, ErrCodeValidation, res.Error.Code)
	assert.Contains(t, res.Error.Fields, "id")
}

func TestCollectionService_List_Happy(t *testing.T) {
	wsID := uuid.New()
	c1 := fixtureCollection("first", 1)
	c2 := fixtureCollection("second", 1)
	uc := &stubCollectionUsecase{
		listFn: func(_ context.Context, opt collection.ListOpt) ([]*entities.Collection, error) {
			assert.Equal(t, wsID, opt.WorkspaceID)
			return []*entities.Collection{c1, c2}, nil
		},
	}
	svc := NewCollectionService(uc)

	res := svc.List(wsID.String())

	require.Nil(t, res.Error)
	require.Len(t, res.Data, 2)
	assert.Equal(t, "first", res.Data[0].Name)
	assert.Equal(t, "second", res.Data[1].Name)
}

func TestCollectionService_List_Error_BadUUID(t *testing.T) {
	uc := &stubCollectionUsecase{}
	svc := NewCollectionService(uc)

	res := svc.List("not-a-uuid")

	require.NotNil(t, res.Error)
	assert.Equal(t, ErrCodeValidation, res.Error.Code)
	assert.Contains(t, res.Error.Fields, "workspaceId")
}

func TestCollectionService_Edit_Happy(t *testing.T) {
	id := uuid.New()
	updated := fixtureCollection("renamed", 3)
	updated.ID = id
	uc := &stubCollectionUsecase{
		editFn: func(_ context.Context, in collection.Edit, opt collection.EditOpt) (*entities.Collection, error) {
			assert.Equal(t, "renamed", in.Name)
			assert.Equal(t, id, opt.CollectionID)
			assert.Equal(t, 2, opt.Version)
			return updated, nil
		},
	}
	svc := NewCollectionService(uc)

	res := svc.Edit(dto.EditCollectionRequest{
		ID:      id.String(),
		Name:    "renamed",
		Version: 2,
	})

	require.Nil(t, res.Error)
	assert.Equal(t, id.String(), res.Data.ID)
	assert.Equal(t, "renamed", res.Data.Name)
	assert.Equal(t, 3, res.Data.Version)
}

func TestCollectionService_Edit_Error_BadUUID(t *testing.T) {
	uc := &stubCollectionUsecase{}
	svc := NewCollectionService(uc)

	res := svc.Edit(dto.EditCollectionRequest{ID: "nope", Name: "x", Version: 1})

	require.NotNil(t, res.Error)
	assert.Equal(t, ErrCodeValidation, res.Error.Code)
	assert.Contains(t, res.Error.Fields, "id")
}

func TestCollectionService_Delete_Happy(t *testing.T) {
	id := uuid.New()
	uc := &stubCollectionUsecase{
		deleteFn: func(_ context.Context, opt collection.DeleteOpt) error {
			assert.Equal(t, id, opt.CollectionID)
			assert.Equal(t, 5, opt.Version)
			return nil
		},
	}
	svc := NewCollectionService(uc)

	res := svc.Delete(dto.DeleteCollectionRequest{ID: id.String(), Version: 5})

	require.Nil(t, res.Error)
}

func TestCollectionService_Delete_Error_BadUUID(t *testing.T) {
	uc := &stubCollectionUsecase{}
	svc := NewCollectionService(uc)

	res := svc.Delete(dto.DeleteCollectionRequest{ID: "garbage", Version: 1})

	require.NotNil(t, res.Error)
	assert.Equal(t, ErrCodeValidation, res.Error.Code)
	assert.Contains(t, res.Error.Fields, "id")
}

func TestCollectionService_Move_Happy(t *testing.T) {
	id := uuid.New()
	newParent := uuid.New()
	newParentStr := newParent.String()
	moved := fixtureCollection("moved", 4)
	moved.ID = id
	moved.ParentID = &newParent
	uc := &stubCollectionUsecase{
		moveFn: func(_ context.Context, opt collection.MoveOpt) (*entities.Collection, error) {
			assert.Equal(t, id, opt.CollectionID)
			require.NotNil(t, opt.TargetParentID)
			assert.Equal(t, newParent, *opt.TargetParentID)
			assert.Equal(t, 3, opt.Version)
			return moved, nil
		},
	}
	svc := NewCollectionService(uc)

	res := svc.Move(dto.MoveCollectionRequest{
		ID:             id.String(),
		TargetParentID: &newParentStr,
		Version:        3,
	})

	require.Nil(t, res.Error)
	require.NotNil(t, res.Data.ParentID)
	assert.Equal(t, newParent.String(), *res.Data.ParentID)
}

func TestCollectionService_Move_Error_BadUUID(t *testing.T) {
	uc := &stubCollectionUsecase{}
	svc := NewCollectionService(uc)

	res := svc.Move(dto.MoveCollectionRequest{ID: "not-a-uuid", Version: 1})

	require.NotNil(t, res.Error)
	assert.Equal(t, ErrCodeValidation, res.Error.Code)
	assert.Contains(t, res.Error.Fields, "id")
}

func TestCollectionService_Reorder_Happy(t *testing.T) {
	id := uuid.New()
	uc := &stubCollectionUsecase{
		reorderFn: func(_ context.Context, gotID uuid.UUID, sortOrder int) error {
			assert.Equal(t, id, gotID)
			assert.Equal(t, 7, sortOrder)
			return nil
		},
	}
	svc := NewCollectionService(uc)

	res := svc.Reorder(dto.ReorderCollectionRequest{ID: id.String(), SortOrder: 7})

	require.Nil(t, res.Error)
}

func TestCollectionService_Reorder_Error_BadUUID(t *testing.T) {
	uc := &stubCollectionUsecase{}
	svc := NewCollectionService(uc)

	res := svc.Reorder(dto.ReorderCollectionRequest{ID: "nope", SortOrder: 1})

	require.NotNil(t, res.Error)
	assert.Equal(t, ErrCodeValidation, res.Error.Code)
	assert.Contains(t, res.Error.Fields, "id")
}
