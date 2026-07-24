package environment_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/tetiva-app/client/internal/domain"
	"github.com/tetiva-app/client/internal/domain/entities"
	"github.com/tetiva-app/client/internal/domain/usecase/environment"
)

var testWorkspaceID = uuid.MustParse("00000000-0000-4000-a000-000000000001")

type mockEnvRepo struct {
	envs map[uuid.UUID]*entities.Environment
}

func newMockEnvRepo() *mockEnvRepo {
	return &mockEnvRepo{envs: make(map[uuid.UUID]*entities.Environment)}
}

func (m *mockEnvRepo) Create(_ context.Context, e *entities.Environment) error {
	m.envs[e.ID] = e
	return nil
}

func (m *mockEnvRepo) GetByID(_ context.Context, id uuid.UUID) (*entities.Environment, error) {
	e, ok := m.envs[id]
	if !ok {
		return nil, nil
	}
	cp := *e
	return &cp, nil
}

func (m *mockEnvRepo) List(_ context.Context, filter environment.Filter) ([]*entities.Environment, error) {
	var result []*entities.Environment
	for _, e := range m.envs {
		if e.WorkspaceID != filter.WorkspaceID || e.IsDelete {
			continue
		}
		cp := *e
		result = append(result, &cp)
	}
	return result, nil
}

func (m *mockEnvRepo) Update(_ context.Context, e *entities.Environment) error {
	m.envs[e.ID] = e
	return nil
}

func (m *mockEnvRepo) GetActive(_ context.Context, workspaceID uuid.UUID) (*entities.Environment, error) {
	for _, e := range m.envs {
		if e.WorkspaceID == workspaceID && e.IsActive && !e.IsDelete {
			cp := *e
			return &cp, nil
		}
	}
	return nil, nil
}

func (m *mockEnvRepo) SetActive(_ context.Context, workspaceID uuid.UUID, envID uuid.UUID) error {
	for _, e := range m.envs {
		if e.WorkspaceID == workspaceID {
			e.IsActive = e.ID == envID
		}
	}
	return nil
}

type mockVarRepo struct {
	vars map[uuid.UUID]*entities.Variable
}

func newMockVarRepo() *mockVarRepo {
	return &mockVarRepo{vars: make(map[uuid.UUID]*entities.Variable)}
}

func (m *mockVarRepo) Create(_ context.Context, v *entities.Variable) error {
	m.vars[v.ID] = v
	return nil
}

func (m *mockVarRepo) GetByID(_ context.Context, id uuid.UUID) (*entities.Variable, error) {
	v, ok := m.vars[id]
	if !ok {
		return nil, nil
	}
	cp := *v
	return &cp, nil
}

func (m *mockVarRepo) List(_ context.Context, envID uuid.UUID) ([]*entities.Variable, error) {
	var result []*entities.Variable
	for _, v := range m.vars {
		if v.EnvironmentID == envID && !v.IsDelete {
			cp := *v
			result = append(result, &cp)
		}
	}
	return result, nil
}

func (m *mockVarRepo) Update(_ context.Context, v *entities.Variable) error {
	m.vars[v.ID] = v
	return nil
}

func (m *mockVarRepo) Delete(_ context.Context, id uuid.UUID) error {
	delete(m.vars, id)
	return nil
}

func newTestUsecase() (environment.Usecase, *mockEnvRepo, *mockVarRepo) {
	envRepo := newMockEnvRepo()
	varRepo := newMockVarRepo()
	uc := environment.NewUsecase(envRepo, varRepo)
	return uc, envRepo, varRepo
}

func createTestEnv(t *testing.T, uc environment.Usecase, name string) *entities.Environment {
	t.Helper()
	env, err := uc.Create(context.Background(), environment.Create{Name: name}, environment.CreateOpt{
		UserID:      "local_user",
		WorkspaceID: testWorkspaceID,
	})
	if err != nil {
		t.Fatalf("create env: %v", err)
	}
	return env
}

func TestCreateEnvironment(t *testing.T) {
	uc, _, _ := newTestUsecase()

	env, err := uc.Create(context.Background(), environment.Create{Name: "Staging"}, environment.CreateOpt{
		UserID:      "local_user",
		WorkspaceID: testWorkspaceID,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if env.Name != "Staging" {
		t.Errorf("name = %q, want %q", env.Name, "Staging")
	}
	if env.Version != 1 {
		t.Errorf("version = %d, want 1", env.Version)
	}
	if env.IsActive {
		t.Error("new environment should not be active")
	}
}

func TestCreateEnvironment_EmptyName(t *testing.T) {
	uc, _, _ := newTestUsecase()

	_, err := uc.Create(context.Background(), environment.Create{Name: ""}, environment.CreateOpt{
		UserID:      "local_user",
		WorkspaceID: testWorkspaceID,
	})
	if err == nil {
		t.Fatal("expected validation error")
	}
	var valErr *domain.ValidationError
	if !errors.As(err, &valErr) {
		t.Fatalf("expected ValidationError, got %T", err)
	}
}

func TestEditEnvironment(t *testing.T) {
	uc, _, _ := newTestUsecase()

	env := createTestEnv(t, uc, "Staging")

	edited, err := uc.Edit(context.Background(), environment.Edit{Name: "Production"}, environment.EditOpt{
		EnvironmentID: env.ID,
		UserID:        "local_user",
		Version:       1,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if edited.Name != "Production" {
		t.Errorf("name = %q, want %q", edited.Name, "Production")
	}
	if edited.Version != 2 {
		t.Errorf("version = %d, want 2", edited.Version)
	}
}

func TestEditEnvironment_VersionConflict(t *testing.T) {
	uc, _, _ := newTestUsecase()

	env := createTestEnv(t, uc, "Staging")

	_, err := uc.Edit(context.Background(), environment.Edit{Name: "Prod"}, environment.EditOpt{
		EnvironmentID: env.ID,
		UserID:        "local_user",
		Version:       99,
	})
	if err == nil {
		t.Fatal("expected conflict error")
	}
	var conflictErr *domain.ConflictError
	if !errors.As(err, &conflictErr) {
		t.Fatalf("expected ConflictError, got %T: %v", err, err)
	}
}

func TestEditEnvironment_NotFound(t *testing.T) {
	uc, _, _ := newTestUsecase()

	_, err := uc.Edit(context.Background(), environment.Edit{Name: "X"}, environment.EditOpt{
		EnvironmentID: uuid.New(),
		UserID:        "local_user",
		Version:       1,
	})
	if err == nil {
		t.Fatal("expected not found error")
	}
	var notFoundErr *domain.NotFoundError
	if !errors.As(err, &notFoundErr) {
		t.Fatalf("expected NotFoundError, got %T: %v", err, err)
	}
}

func TestDeleteEnvironment(t *testing.T) {
	uc, envRepo, _ := newTestUsecase()

	env := createTestEnv(t, uc, "ToDelete")

	err := uc.Delete(context.Background(), environment.DeleteOpt{
		EnvironmentID: env.ID,
		UserID:        "local_user",
		Version:       1,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	deleted := envRepo.envs[env.ID]
	if !deleted.IsDelete {
		t.Error("expected is_delete = true")
	}
}

func TestListEnvironments(t *testing.T) {
	uc, _, _ := newTestUsecase()

	createTestEnv(t, uc, "Dev")
	createTestEnv(t, uc, "Staging")
	createTestEnv(t, uc, "Prod")

	envs, err := uc.List(context.Background(), environment.ListOpt{WorkspaceID: testWorkspaceID})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(envs) != 3 {
		t.Errorf("len = %d, want 3", len(envs))
	}
}

func TestListEnvironments_ExcludesDeleted(t *testing.T) {
	uc, _, _ := newTestUsecase()

	createTestEnv(t, uc, "Active")
	toDelete := createTestEnv(t, uc, "ToDelete")

	_ = uc.Delete(context.Background(), environment.DeleteOpt{
		EnvironmentID: toDelete.ID,
		UserID:        "local_user",
		Version:       1,
	})

	envs, err := uc.List(context.Background(), environment.ListOpt{WorkspaceID: testWorkspaceID})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(envs) != 1 {
		t.Errorf("len = %d, want 1", len(envs))
	}
}

func TestSetActive(t *testing.T) {
	uc, envRepo, _ := newTestUsecase()

	env1 := createTestEnv(t, uc, "Dev")
	env2 := createTestEnv(t, uc, "Staging")

	err := uc.SetActive(context.Background(), environment.SetActiveOpt{
		WorkspaceID:   testWorkspaceID,
		EnvironmentID: env2.ID,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if envRepo.envs[env1.ID].IsActive {
		t.Error("env1 should not be active")
	}
	if !envRepo.envs[env2.ID].IsActive {
		t.Error("env2 should be active")
	}
}

func TestAddVariable(t *testing.T) {
	uc, _, _ := newTestUsecase()

	env := createTestEnv(t, uc, "Dev")

	v, err := uc.AddVariable(context.Background(), environment.AddVariable{
		EnvironmentID: env.ID,
		Key:           "base_url",
		Value:         "https://api.dev.example.com",
		IsSecret:      false,
	}, environment.AddVariableOpt{UserID: "local_user"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if v.Key != "base_url" {
		t.Errorf("key = %q, want %q", v.Key, "base_url")
	}
	if v.Value != "https://api.dev.example.com" {
		t.Errorf("value = %q, want %q", v.Value, "https://api.dev.example.com")
	}
	if !v.Enabled {
		t.Error("new variable should be enabled")
	}
}

func TestAddVariable_EmptyKey(t *testing.T) {
	uc, _, _ := newTestUsecase()

	env := createTestEnv(t, uc, "Dev")

	_, err := uc.AddVariable(context.Background(), environment.AddVariable{
		EnvironmentID: env.ID,
		Key:           "",
		Value:         "val",
	}, environment.AddVariableOpt{UserID: "local_user"})
	if err == nil {
		t.Fatal("expected validation error")
	}
	var valErr *domain.ValidationError
	if !errors.As(err, &valErr) {
		t.Fatalf("expected ValidationError, got %T", err)
	}
}

func TestAddVariable_Secret(t *testing.T) {
	uc, _, _ := newTestUsecase()

	env := createTestEnv(t, uc, "Dev")

	v, err := uc.AddVariable(context.Background(), environment.AddVariable{
		EnvironmentID: env.ID,
		Key:           "token",
		Value:         "sk-secret-123",
		IsSecret:      true,
	}, environment.AddVariableOpt{UserID: "local_user"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !v.IsSecret {
		t.Error("expected is_secret = true")
	}
}

func TestEditVariable(t *testing.T) {
	uc, _, _ := newTestUsecase()

	env := createTestEnv(t, uc, "Dev")

	v, _ := uc.AddVariable(context.Background(), environment.AddVariable{
		EnvironmentID: env.ID,
		Key:           "token",
		Value:         "old",
	}, environment.AddVariableOpt{UserID: "local_user"})

	edited, err := uc.EditVariable(context.Background(), environment.EditVariable{
		Key:     "token",
		Value:   "new_value",
		Enabled: true,
	}, environment.EditVariableOpt{
		VariableID: v.ID,
		UserID:     "local_user",
		Version:    1,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if edited.Value != "new_value" {
		t.Errorf("value = %q, want %q", edited.Value, "new_value")
	}
	if edited.Version != 2 {
		t.Errorf("version = %d, want 2", edited.Version)
	}
}

func TestEditVariable_VersionConflict(t *testing.T) {
	uc, _, _ := newTestUsecase()

	env := createTestEnv(t, uc, "Dev")

	v, _ := uc.AddVariable(context.Background(), environment.AddVariable{
		EnvironmentID: env.ID,
		Key:           "key",
		Value:         "val",
	}, environment.AddVariableOpt{UserID: "local_user"})

	_, err := uc.EditVariable(context.Background(), environment.EditVariable{
		Key:     "key",
		Value:   "new",
		Enabled: true,
	}, environment.EditVariableOpt{
		VariableID: v.ID,
		UserID:     "local_user",
		Version:    99,
	})
	if err == nil {
		t.Fatal("expected conflict error")
	}
	var conflictErr *domain.ConflictError
	if !errors.As(err, &conflictErr) {
		t.Fatalf("expected ConflictError, got %T: %v", err, err)
	}
}

func TestDeleteVariable(t *testing.T) {
	uc, _, varRepo := newTestUsecase()

	env := createTestEnv(t, uc, "Dev")

	v, _ := uc.AddVariable(context.Background(), environment.AddVariable{
		EnvironmentID: env.ID,
		Key:           "to_delete",
		Value:         "val",
	}, environment.AddVariableOpt{UserID: "local_user"})

	err := uc.DeleteVariable(context.Background(), environment.DeleteVariableOpt{
		VariableID: v.ID,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := varRepo.vars[v.ID]; ok {
		t.Error("variable should be deleted")
	}
}

func TestResolveVariables(t *testing.T) {
	uc, _, _ := newTestUsecase()

	env := createTestEnv(t, uc, "Dev")

	_ = uc.SetActive(context.Background(), environment.SetActiveOpt{
		WorkspaceID:   testWorkspaceID,
		EnvironmentID: env.ID,
	})

	_, _ = uc.AddVariable(context.Background(), environment.AddVariable{
		EnvironmentID: env.ID,
		Key:           "base_url",
		Value:         "https://api.dev.example.com",
	}, environment.AddVariableOpt{UserID: "local_user"})

	_, _ = uc.AddVariable(context.Background(), environment.AddVariable{
		EnvironmentID: env.ID,
		Key:           "token",
		Value:         "sk-123",
		IsSecret:      true,
	}, environment.AddVariableOpt{UserID: "local_user"})

	vars, err := uc.ResolveVariables(context.Background(), testWorkspaceID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if vars["base_url"] != "https://api.dev.example.com" {
		t.Errorf("base_url = %q", vars["base_url"])
	}
	if vars["token"] != "sk-123" {
		t.Errorf("token = %q", vars["token"])
	}
}

func TestResolveVariables_NoActiveEnv(t *testing.T) {
	uc, _, _ := newTestUsecase()

	vars, err := uc.ResolveVariables(context.Background(), testWorkspaceID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(vars) != 0 {
		t.Errorf("expected empty map, got %d entries", len(vars))
	}
}

func TestResolveVariables_DisabledSkipped(t *testing.T) {
	uc, _, varRepo := newTestUsecase()

	env := createTestEnv(t, uc, "Dev")
	_ = uc.SetActive(context.Background(), environment.SetActiveOpt{
		WorkspaceID:   testWorkspaceID,
		EnvironmentID: env.ID,
	})

	v, _ := uc.AddVariable(context.Background(), environment.AddVariable{
		EnvironmentID: env.ID,
		Key:           "disabled_key",
		Value:         "val",
	}, environment.AddVariableOpt{UserID: "local_user"})

	varRepo.vars[v.ID].Enabled = false

	vars, err := uc.ResolveVariables(context.Background(), testWorkspaceID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := vars["disabled_key"]; ok {
		t.Error("disabled variable should not be resolved")
	}
}
