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
	"github.com/tetiva-app/client/internal/domain/entities"
	"github.com/tetiva-app/client/internal/domain/usecase/environment"
)

// stubEnvironmentUsecase is the environment.Usecase double shared with the
// portability tests; methods without an injected Fn panic.
type stubEnvironmentUsecase struct {
	createFn               func(context.Context, environment.Create, environment.CreateOpt) (*entities.Environment, error)
	duplicateFn            func(context.Context, environment.DuplicateOpt) (*entities.Environment, error)
	getByIDFn              func(context.Context, uuid.UUID) (*entities.Environment, error)
	listFn                 func(context.Context, environment.ListOpt) ([]*entities.Environment, error)
	editFn                 func(context.Context, environment.Edit, environment.EditOpt) (*entities.Environment, error)
	deleteFn               func(context.Context, environment.DeleteOpt) error
	setActiveFn            func(context.Context, environment.SetActiveOpt) error
	addVariableFn          func(context.Context, environment.AddVariable, environment.AddVariableOpt) (*entities.Variable, error)
	editVariableFn         func(context.Context, environment.EditVariable, environment.EditVariableOpt) (*entities.Variable, error)
	deleteVariableFn       func(context.Context, environment.DeleteVariableOpt) error
	listVariablesFn        func(context.Context, uuid.UUID) ([]*entities.Variable, error)
	resolveVariablesFn     func(context.Context, uuid.UUID) (map[string]string, error)
	persistVariableChanges func(context.Context, uuid.UUID, string, map[string]string) error
}

func (s *stubEnvironmentUsecase) Create(ctx context.Context, in environment.Create, opt environment.CreateOpt) (*entities.Environment, error) {
	if s.createFn == nil {
		panic("createFn not set")
	}
	return s.createFn(ctx, in, opt)
}

func (s *stubEnvironmentUsecase) Duplicate(ctx context.Context, opt environment.DuplicateOpt) (*entities.Environment, error) {
	if s.duplicateFn == nil {
		panic("duplicateFn not set")
	}
	return s.duplicateFn(ctx, opt)
}

func (s *stubEnvironmentUsecase) GetByID(ctx context.Context, id uuid.UUID) (*entities.Environment, error) {
	if s.getByIDFn == nil {
		panic("getByIDFn not set")
	}
	return s.getByIDFn(ctx, id)
}

func (s *stubEnvironmentUsecase) List(ctx context.Context, opt environment.ListOpt) ([]*entities.Environment, error) {
	if s.listFn == nil {
		panic("listFn not set")
	}
	return s.listFn(ctx, opt)
}

func (s *stubEnvironmentUsecase) Edit(ctx context.Context, in environment.Edit, opt environment.EditOpt) (*entities.Environment, error) {
	if s.editFn == nil {
		panic("editFn not set")
	}
	return s.editFn(ctx, in, opt)
}

func (s *stubEnvironmentUsecase) Delete(ctx context.Context, opt environment.DeleteOpt) error {
	if s.deleteFn == nil {
		panic("deleteFn not set")
	}
	return s.deleteFn(ctx, opt)
}

func (s *stubEnvironmentUsecase) SetActive(ctx context.Context, opt environment.SetActiveOpt) error {
	if s.setActiveFn == nil {
		panic("setActiveFn not set")
	}
	return s.setActiveFn(ctx, opt)
}

func (s *stubEnvironmentUsecase) AddVariable(ctx context.Context, in environment.AddVariable, opt environment.AddVariableOpt) (*entities.Variable, error) {
	if s.addVariableFn == nil {
		panic("addVariableFn not set")
	}
	return s.addVariableFn(ctx, in, opt)
}

func (s *stubEnvironmentUsecase) EditVariable(ctx context.Context, in environment.EditVariable, opt environment.EditVariableOpt) (*entities.Variable, error) {
	if s.editVariableFn == nil {
		panic("editVariableFn not set")
	}
	return s.editVariableFn(ctx, in, opt)
}

func (s *stubEnvironmentUsecase) DeleteVariable(ctx context.Context, opt environment.DeleteVariableOpt) error {
	if s.deleteVariableFn == nil {
		panic("deleteVariableFn not set")
	}
	return s.deleteVariableFn(ctx, opt)
}

func (s *stubEnvironmentUsecase) ListVariables(ctx context.Context, environmentID uuid.UUID) ([]*entities.Variable, error) {
	if s.listVariablesFn == nil {
		panic("listVariablesFn not set")
	}
	return s.listVariablesFn(ctx, environmentID)
}

func (s *stubEnvironmentUsecase) ResolveVariables(ctx context.Context, workspaceID uuid.UUID) (map[string]string, error) {
	if s.resolveVariablesFn == nil {
		panic("resolveVariablesFn not set")
	}
	return s.resolveVariablesFn(ctx, workspaceID)
}

func (s *stubEnvironmentUsecase) PersistVariableChanges(ctx context.Context, workspaceID uuid.UUID, userID string, newVars map[string]string) error {
	if s.persistVariableChanges == nil {
		panic("persistVariableChanges not set")
	}
	return s.persistVariableChanges(ctx, workspaceID, userID, newVars)
}

func fixtureEnvironment(name string, active bool, version int) *entities.Environment {
	now := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	return &entities.Environment{
		ID:          uuid.New(),
		WorkspaceID: uuid.New(),
		Name:        name,
		IsActive:    active,
		Version:     version,
		CreatedBy:   "local_user",
		CreatedAt:   now,
		UpdatedBy:   "local_user",
		UpdatedAt:   now,
	}
}

func fixtureVariable(envID uuid.UUID, key, value string) *entities.Variable {
	now := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	return &entities.Variable{
		ID:            uuid.New(),
		EnvironmentID: envID,
		Key:           key,
		Value:         value,
		IsSecret:      false,
		Enabled:       true,
		SortOrder:     0,
		Version:       1,
		CreatedBy:     "local_user",
		CreatedAt:     now,
		UpdatedBy:     "local_user",
		UpdatedAt:     now,
	}
}

func TestEnvironmentService_List_Happy(t *testing.T) {
	workspaceID := uuid.New()
	e1 := fixtureEnvironment("dev", true, 1)
	e2 := fixtureEnvironment("prod", false, 2)
	uc := &stubEnvironmentUsecase{
		listFn: func(_ context.Context, opt environment.ListOpt) ([]*entities.Environment, error) {
			assert.Equal(t, workspaceID, opt.WorkspaceID)
			return []*entities.Environment{e1, e2}, nil
		},
	}
	svc := NewEnvironmentService(uc)

	res := svc.List(workspaceID.String())

	require.Nil(t, res.Error)
	require.Len(t, res.Data, 2)
	assert.Equal(t, "dev", res.Data[0].Name)
	assert.True(t, res.Data[0].IsActive)
	assert.Equal(t, "prod", res.Data[1].Name)
}

func TestEnvironmentService_List_Error_BadUUID(t *testing.T) {
	uc := &stubEnvironmentUsecase{}
	svc := NewEnvironmentService(uc)

	res := svc.List("not-a-uuid")

	require.NotNil(t, res.Error)
	assert.Equal(t, ErrCodeValidation, res.Error.Code)
	assert.Contains(t, res.Error.Fields, "workspaceId")
}

func TestEnvironmentService_Create_Happy(t *testing.T) {
	workspaceID := uuid.New()
	created := fixtureEnvironment("staging", false, 1)
	uc := &stubEnvironmentUsecase{
		createFn: func(_ context.Context, in environment.Create, opt environment.CreateOpt) (*entities.Environment, error) {
			assert.Equal(t, "staging", in.Name)
			assert.Equal(t, workspaceID, opt.WorkspaceID)
			assert.Equal(t, defaultUserID, opt.UserID)
			return created, nil
		},
	}
	svc := NewEnvironmentService(uc)

	res := svc.Create(dto.CreateEnvironmentRequest{Name: "staging", WorkspaceID: workspaceID.String()})

	require.Nil(t, res.Error)
	assert.Equal(t, created.ID.String(), res.Data.ID)
	assert.Equal(t, "staging", res.Data.Name)
}

func TestEnvironmentService_Create_Error_BadWorkspaceUUID(t *testing.T) {
	uc := &stubEnvironmentUsecase{}
	svc := NewEnvironmentService(uc)

	res := svc.Create(dto.CreateEnvironmentRequest{Name: "x", WorkspaceID: "garbage"})

	require.NotNil(t, res.Error)
	assert.Equal(t, ErrCodeValidation, res.Error.Code)
	assert.Contains(t, res.Error.Fields, "workspaceId")
}

func TestEnvironmentService_Duplicate_Happy(t *testing.T) {
	sourceID := uuid.New()
	workspaceID := uuid.New()
	dup := fixtureEnvironment("dev-copy", false, 1)
	uc := &stubEnvironmentUsecase{
		duplicateFn: func(_ context.Context, opt environment.DuplicateOpt) (*entities.Environment, error) {
			assert.Equal(t, sourceID, opt.SourceID)
			assert.Equal(t, workspaceID, opt.WorkspaceID)
			assert.Equal(t, "dev-copy", opt.NewName)
			assert.Equal(t, defaultUserID, opt.UserID)
			return dup, nil
		},
	}
	svc := NewEnvironmentService(uc)

	res := svc.Duplicate(dto.DuplicateEnvironmentRequest{
		SourceID:    sourceID.String(),
		NewName:     "dev-copy",
		WorkspaceID: workspaceID.String(),
	})

	require.Nil(t, res.Error)
	assert.Equal(t, dup.ID.String(), res.Data.ID)
	assert.Equal(t, "dev-copy", res.Data.Name)
}

func TestEnvironmentService_Duplicate_Error_BadSourceUUID(t *testing.T) {
	uc := &stubEnvironmentUsecase{}
	svc := NewEnvironmentService(uc)

	res := svc.Duplicate(dto.DuplicateEnvironmentRequest{
		SourceID:    "bad",
		NewName:     "x",
		WorkspaceID: uuid.New().String(),
	})

	require.NotNil(t, res.Error)
	assert.Equal(t, ErrCodeValidation, res.Error.Code)
	assert.Contains(t, res.Error.Fields, "sourceId")
}

func TestEnvironmentService_Edit_Happy(t *testing.T) {
	id := uuid.New()
	updated := fixtureEnvironment("renamed", false, 2)
	updated.ID = id
	uc := &stubEnvironmentUsecase{
		editFn: func(_ context.Context, in environment.Edit, opt environment.EditOpt) (*entities.Environment, error) {
			assert.Equal(t, "renamed", in.Name)
			assert.Equal(t, id, opt.EnvironmentID)
			assert.Equal(t, 1, opt.Version)
			assert.Equal(t, defaultUserID, opt.UserID)
			return updated, nil
		},
	}
	svc := NewEnvironmentService(uc)

	res := svc.Edit(dto.EditEnvironmentRequest{ID: id.String(), Name: "renamed", Version: 1})

	require.Nil(t, res.Error)
	assert.Equal(t, id.String(), res.Data.ID)
	assert.Equal(t, "renamed", res.Data.Name)
	assert.Equal(t, 2, res.Data.Version)
}

func TestEnvironmentService_Edit_Error_BadUUID(t *testing.T) {
	uc := &stubEnvironmentUsecase{}
	svc := NewEnvironmentService(uc)

	res := svc.Edit(dto.EditEnvironmentRequest{ID: "garbage", Name: "x", Version: 1})

	require.NotNil(t, res.Error)
	assert.Equal(t, ErrCodeValidation, res.Error.Code)
	assert.Contains(t, res.Error.Fields, "id")
}

func TestEnvironmentService_Delete_Happy(t *testing.T) {
	id := uuid.New()
	uc := &stubEnvironmentUsecase{
		deleteFn: func(_ context.Context, opt environment.DeleteOpt) error {
			assert.Equal(t, id, opt.EnvironmentID)
			assert.Equal(t, 5, opt.Version)
			assert.Equal(t, defaultUserID, opt.UserID)
			return nil
		},
	}
	svc := NewEnvironmentService(uc)

	res := svc.Delete(dto.DeleteEnvironmentRequest{ID: id.String(), Version: 5})

	require.Nil(t, res.Error)
}

func TestEnvironmentService_Delete_Error_BadUUID(t *testing.T) {
	uc := &stubEnvironmentUsecase{}
	svc := NewEnvironmentService(uc)

	res := svc.Delete(dto.DeleteEnvironmentRequest{ID: "garbage", Version: 1})

	require.NotNil(t, res.Error)
	assert.Equal(t, ErrCodeValidation, res.Error.Code)
	assert.Contains(t, res.Error.Fields, "id")
}

func TestEnvironmentService_SetActive_Happy(t *testing.T) {
	workspaceID := uuid.New()
	envID := uuid.New()
	uc := &stubEnvironmentUsecase{
		setActiveFn: func(_ context.Context, opt environment.SetActiveOpt) error {
			assert.Equal(t, workspaceID, opt.WorkspaceID)
			assert.Equal(t, envID, opt.EnvironmentID)
			return nil
		},
	}
	svc := NewEnvironmentService(uc)

	res := svc.SetActive(workspaceID.String(), dto.SetActiveEnvironmentRequest{EnvironmentID: envID.String()})

	require.Nil(t, res.Error)
}

func TestEnvironmentService_SetActive_Error_BadWorkspaceUUID(t *testing.T) {
	uc := &stubEnvironmentUsecase{}
	svc := NewEnvironmentService(uc)

	res := svc.SetActive("not-a-uuid", dto.SetActiveEnvironmentRequest{EnvironmentID: uuid.New().String()})

	require.NotNil(t, res.Error)
	assert.Equal(t, ErrCodeValidation, res.Error.Code)
	assert.Contains(t, res.Error.Fields, "workspaceId")
}

func TestEnvironmentService_SetActive_Error_BadEnvironmentUUID(t *testing.T) {
	uc := &stubEnvironmentUsecase{}
	svc := NewEnvironmentService(uc)

	res := svc.SetActive(uuid.New().String(), dto.SetActiveEnvironmentRequest{EnvironmentID: "bad"})

	require.NotNil(t, res.Error)
	assert.Equal(t, ErrCodeValidation, res.Error.Code)
	assert.Contains(t, res.Error.Fields, "environmentId")
}

func TestEnvironmentService_ListVariables_Happy(t *testing.T) {
	envID := uuid.New()
	v1 := fixtureVariable(envID, "API_URL", "https://api.example.com")
	v2 := fixtureVariable(envID, "TOKEN", "secret")
	uc := &stubEnvironmentUsecase{
		listVariablesFn: func(_ context.Context, id uuid.UUID) ([]*entities.Variable, error) {
			assert.Equal(t, envID, id)
			return []*entities.Variable{v1, v2}, nil
		},
	}
	svc := NewEnvironmentService(uc)

	res := svc.ListVariables(envID.String())

	require.Nil(t, res.Error)
	require.Len(t, res.Data, 2)
	assert.Equal(t, "API_URL", res.Data[0].Key)
	assert.Equal(t, "TOKEN", res.Data[1].Key)
}

func TestEnvironmentService_ListVariables_Error_BadUUID(t *testing.T) {
	uc := &stubEnvironmentUsecase{}
	svc := NewEnvironmentService(uc)

	res := svc.ListVariables("garbage")

	require.NotNil(t, res.Error)
	assert.Equal(t, ErrCodeValidation, res.Error.Code)
	assert.Contains(t, res.Error.Fields, "environmentId")
}

func TestEnvironmentService_AddVariable_Happy(t *testing.T) {
	envID := uuid.New()
	created := fixtureVariable(envID, "KEY", "VAL")
	uc := &stubEnvironmentUsecase{
		addVariableFn: func(_ context.Context, in environment.AddVariable, opt environment.AddVariableOpt) (*entities.Variable, error) {
			assert.Equal(t, envID, in.EnvironmentID)
			assert.Equal(t, "KEY", in.Key)
			assert.Equal(t, "VAL", in.Value)
			assert.True(t, in.IsSecret)
			assert.Equal(t, defaultUserID, opt.UserID)
			return created, nil
		},
	}
	svc := NewEnvironmentService(uc)

	res := svc.AddVariable(dto.AddVariableRequest{
		EnvironmentID: envID.String(),
		Key:           "KEY",
		Value:         "VAL",
		IsSecret:      true,
	})

	require.Nil(t, res.Error)
	assert.Equal(t, created.ID.String(), res.Data.ID)
	assert.Equal(t, "KEY", res.Data.Key)
}

func TestEnvironmentService_AddVariable_Error_BadEnvironmentUUID(t *testing.T) {
	uc := &stubEnvironmentUsecase{}
	svc := NewEnvironmentService(uc)

	res := svc.AddVariable(dto.AddVariableRequest{
		EnvironmentID: "bad",
		Key:           "K",
		Value:         "V",
	})

	require.NotNil(t, res.Error)
	assert.Equal(t, ErrCodeValidation, res.Error.Code)
	assert.Contains(t, res.Error.Fields, "environmentId")
}

func TestEnvironmentService_EditVariable_Happy(t *testing.T) {
	varID := uuid.New()
	envID := uuid.New()
	updated := fixtureVariable(envID, "K2", "V2")
	updated.ID = varID
	updated.Version = 3
	uc := &stubEnvironmentUsecase{
		editVariableFn: func(_ context.Context, in environment.EditVariable, opt environment.EditVariableOpt) (*entities.Variable, error) {
			assert.Equal(t, "K2", in.Key)
			assert.Equal(t, "V2", in.Value)
			assert.True(t, in.IsSecret)
			assert.False(t, in.Enabled)
			assert.Equal(t, varID, opt.VariableID)
			assert.Equal(t, 2, opt.Version)
			assert.Equal(t, defaultUserID, opt.UserID)
			return updated, nil
		},
	}
	svc := NewEnvironmentService(uc)

	res := svc.EditVariable(dto.EditVariableRequest{
		ID:       varID.String(),
		Key:      "K2",
		Value:    "V2",
		IsSecret: true,
		Enabled:  false,
		Version:  2,
	})

	require.Nil(t, res.Error)
	assert.Equal(t, varID.String(), res.Data.ID)
	assert.Equal(t, "K2", res.Data.Key)
	assert.Equal(t, 3, res.Data.Version)
}

func TestEnvironmentService_EditVariable_Error_BadUUID(t *testing.T) {
	uc := &stubEnvironmentUsecase{}
	svc := NewEnvironmentService(uc)

	res := svc.EditVariable(dto.EditVariableRequest{
		ID:      "bad",
		Key:     "K",
		Value:   "V",
		Version: 1,
	})

	require.NotNil(t, res.Error)
	assert.Equal(t, ErrCodeValidation, res.Error.Code)
	assert.Contains(t, res.Error.Fields, "id")
}

func TestEnvironmentService_DeleteVariable_Happy(t *testing.T) {
	varID := uuid.New()
	uc := &stubEnvironmentUsecase{
		deleteVariableFn: func(_ context.Context, opt environment.DeleteVariableOpt) error {
			assert.Equal(t, varID, opt.VariableID)
			return nil
		},
	}
	svc := NewEnvironmentService(uc)

	res := svc.DeleteVariable(dto.DeleteVariableRequest{ID: varID.String()})

	require.Nil(t, res.Error)
}

func TestEnvironmentService_DeleteVariable_Error_BadUUID(t *testing.T) {
	uc := &stubEnvironmentUsecase{}
	svc := NewEnvironmentService(uc)

	res := svc.DeleteVariable(dto.DeleteVariableRequest{ID: "bad"})

	require.NotNil(t, res.Error)
	assert.Equal(t, ErrCodeValidation, res.Error.Code)
	assert.Contains(t, res.Error.Fields, "id")
}

func TestEnvironmentService_ResolveVariables_Happy(t *testing.T) {
	workspaceID := uuid.New()
	resolved := map[string]string{"API_URL": "https://api.example.com", "TOKEN": "abc"}
	uc := &stubEnvironmentUsecase{
		resolveVariablesFn: func(_ context.Context, id uuid.UUID) (map[string]string, error) {
			assert.Equal(t, workspaceID, id)
			return resolved, nil
		},
	}
	svc := NewEnvironmentService(uc)

	res := svc.ResolveVariables(workspaceID.String())

	require.Nil(t, res.Error)
	assert.Equal(t, resolved, res.Data)
}

func TestEnvironmentService_ResolveVariables_Error_BadUUID(t *testing.T) {
	uc := &stubEnvironmentUsecase{
		resolveVariablesFn: func(_ context.Context, _ uuid.UUID) (map[string]string, error) {
			return nil, errors.New("should not be called")
		},
	}
	svc := NewEnvironmentService(uc)

	res := svc.ResolveVariables("bad")

	require.NotNil(t, res.Error)
	assert.Equal(t, ErrCodeValidation, res.Error.Code)
	assert.Contains(t, res.Error.Fields, "workspaceId")
}
