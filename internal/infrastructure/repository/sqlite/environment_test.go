package sqlite

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/tetiva-app/client/internal/domain/entities"
	"github.com/tetiva-app/client/internal/domain/usecase/environment"
)

func newTestEnvironment(name string) *entities.Environment {
	now := time.Now().Truncate(time.Second)
	return &entities.Environment{
		ID:          uuid.New(),
		WorkspaceID: testWorkspaceID,
		Name:        name,
		IsActive:    false,
		Version:     1,
		IsDelete:    false,
		CreatedBy:   "test_user",
		CreatedAt:   now,
		UpdatedBy:   "test_user",
		UpdatedAt:   now,
	}
}

func newTestVariable(envID uuid.UUID, key, value string) *entities.Variable {
	now := time.Now().Truncate(time.Second)
	return &entities.Variable{
		ID:            uuid.New(),
		EnvironmentID: envID,
		Key:           key,
		Value:         value,
		IsSecret:      false,
		Enabled:       true,
		SortOrder:     0,
		Version:       1,
		IsDelete:      false,
		CreatedBy:     "test_user",
		CreatedAt:     now,
		UpdatedBy:     "test_user",
		UpdatedAt:     now,
	}
}

func TestEnvironmentRepo_CreateAndGetByID(t *testing.T) {
	db := setupTestDB(t)
	repo := NewEnvironmentRepo(db)
	ctx := context.Background()

	env := newTestEnvironment("Staging")
	if err := repo.Create(ctx, env); err != nil {
		t.Fatalf("create: %v", err)
	}

	got, err := repo.GetByID(ctx, env.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got == nil {
		t.Fatal("expected environment, got nil")
	}
	if got.Name != "Staging" {
		t.Errorf("name = %q, want %q", got.Name, "Staging")
	}
}

func TestEnvironmentRepo_List(t *testing.T) {
	db := setupTestDB(t)
	repo := NewEnvironmentRepo(db)
	ctx := context.Background()

	// Default env already exists from migration
	env1 := newTestEnvironment("Dev")
	env2 := newTestEnvironment("Prod")
	_ = repo.Create(ctx, env1)
	_ = repo.Create(ctx, env2)

	envs, err := repo.List(ctx, environment.Filter{WorkspaceID: testWorkspaceID})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	// Default + Dev + Prod = 3
	if len(envs) != 3 {
		t.Errorf("len = %d, want 3", len(envs))
	}
}

func TestEnvironmentRepo_List_ExcludesDeleted(t *testing.T) {
	db := setupTestDB(t)
	repo := NewEnvironmentRepo(db)
	ctx := context.Background()

	env := newTestEnvironment("ToDelete")
	_ = repo.Create(ctx, env)
	env.IsDelete = true
	_ = repo.Update(ctx, env)

	envs, err := repo.List(ctx, environment.Filter{WorkspaceID: testWorkspaceID})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	for _, e := range envs {
		if e.ID == env.ID {
			t.Error("deleted environment should not appear in list")
		}
	}
}

func TestEnvironmentRepo_SetActive(t *testing.T) {
	db := setupTestDB(t)
	repo := NewEnvironmentRepo(db)
	ctx := context.Background()

	env1 := newTestEnvironment("Dev")
	env2 := newTestEnvironment("Staging")
	_ = repo.Create(ctx, env1)
	_ = repo.Create(ctx, env2)

	if err := repo.SetActive(ctx, testWorkspaceID, env2.ID); err != nil {
		t.Fatalf("set active: %v", err)
	}

	active, err := repo.GetActive(ctx, testWorkspaceID)
	if err != nil {
		t.Fatalf("get active: %v", err)
	}
	if active == nil {
		t.Fatal("expected active environment")
	}
	if active.ID != env2.ID {
		t.Errorf("active = %s, want %s", active.ID, env2.ID)
	}

	got1, _ := repo.GetByID(ctx, env1.ID)
	if got1.IsActive {
		t.Error("env1 should not be active")
	}
}

func TestEnvironmentRepo_GetActive_None(t *testing.T) {
	db := setupTestDB(t)
	repo := NewEnvironmentRepo(db)
	ctx := context.Background()

	_ = repo.SetActive(ctx, testWorkspaceID, uuid.New()) // activate non-existent ID → all deactivated

	active, err := repo.GetActive(ctx, testWorkspaceID)
	if err != nil {
		t.Fatalf("get active: %v", err)
	}
	// SetActive deactivates all rows first; the non-existent ID matches none.
	if active != nil {
		t.Errorf("expected nil active, got %v", active.Name)
	}
}

func TestVariableRepo_CreateAndList(t *testing.T) {
	db := setupTestDB(t)
	envRepo := NewEnvironmentRepo(db)
	varRepo := NewVariableRepo(db)
	ctx := context.Background()

	env := newTestEnvironment("Dev")
	_ = envRepo.Create(ctx, env)

	v1 := newTestVariable(env.ID, "base_url", "https://api.dev.example.com")
	v2 := newTestVariable(env.ID, "token", "sk-123")
	v2.IsSecret = true
	_ = varRepo.Create(ctx, v1)
	_ = varRepo.Create(ctx, v2)

	vars, err := varRepo.List(ctx, env.ID)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(vars) != 2 {
		t.Errorf("len = %d, want 2", len(vars))
	}
}

func TestVariableRepo_GetByID(t *testing.T) {
	db := setupTestDB(t)
	envRepo := NewEnvironmentRepo(db)
	varRepo := NewVariableRepo(db)
	ctx := context.Background()

	env := newTestEnvironment("Dev")
	_ = envRepo.Create(ctx, env)

	v := newTestVariable(env.ID, "key", "value")
	_ = varRepo.Create(ctx, v)

	got, err := varRepo.GetByID(ctx, v.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got == nil {
		t.Fatal("expected variable")
	}
	if got.Key != "key" {
		t.Errorf("key = %q, want %q", got.Key, "key")
	}
}

func TestVariableRepo_Update(t *testing.T) {
	db := setupTestDB(t)
	envRepo := NewEnvironmentRepo(db)
	varRepo := NewVariableRepo(db)
	ctx := context.Background()

	env := newTestEnvironment("Dev")
	_ = envRepo.Create(ctx, env)

	v := newTestVariable(env.ID, "key", "old_value")
	_ = varRepo.Create(ctx, v)

	v.Value = "new_value"
	v.Version = 2
	if err := varRepo.Update(ctx, v); err != nil {
		t.Fatalf("update: %v", err)
	}

	got, _ := varRepo.GetByID(ctx, v.ID)
	if got.Value != "new_value" {
		t.Errorf("value = %q, want %q", got.Value, "new_value")
	}
}

func TestVariableRepo_Delete(t *testing.T) {
	db := setupTestDB(t)
	envRepo := NewEnvironmentRepo(db)
	varRepo := NewVariableRepo(db)
	ctx := context.Background()

	env := newTestEnvironment("Dev")
	_ = envRepo.Create(ctx, env)

	v := newTestVariable(env.ID, "to_delete", "val")
	_ = varRepo.Create(ctx, v)

	if err := varRepo.Delete(ctx, v.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}

	got, err := varRepo.GetByID(ctx, v.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got != nil {
		t.Error("expected nil after delete")
	}
}

func TestVariableRepo_CascadeDelete(t *testing.T) {
	db := setupTestDB(t)
	envRepo := NewEnvironmentRepo(db)
	varRepo := NewVariableRepo(db)
	ctx := context.Background()

	env := newTestEnvironment("Dev")
	_ = envRepo.Create(ctx, env)

	v := newTestVariable(env.ID, "cascade_test", "val")
	_ = varRepo.Create(ctx, v)

	// Delete environment via SQL (CASCADE should remove variables)
	_, err := db.ExecContext(ctx, `DELETE FROM environments WHERE id = ?`, env.ID.String())
	if err != nil {
		t.Fatalf("delete env: %v", err)
	}

	got, err := varRepo.GetByID(ctx, v.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got != nil {
		t.Error("variable should be cascade-deleted with environment")
	}
}
