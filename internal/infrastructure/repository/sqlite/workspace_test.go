package sqlite

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/tetiva-app/client/internal/domain/entities"
)

func newTestWorkspace(name string) *entities.Workspace {
	now := time.Now().Truncate(time.Second)
	return &entities.Workspace{
		ID:        uuid.New(),
		Name:      name,
		IsActive:  false,
		Version:   1,
		IsDelete:  false,
		CreatedBy: "test_user",
		CreatedAt: now,
		UpdatedBy: "test_user",
		UpdatedAt: now,
	}
}

func TestWorkspaceRepo_CreateAndGetByID(t *testing.T) {
	db := setupTestDB(t)
	repo := NewWorkspaceRepo(db)
	ctx := context.Background()

	w := newTestWorkspace("Test Workspace")

	if err := repo.Create(ctx, w); err != nil {
		t.Fatalf("Create: %v", err)
	}

	got, err := repo.GetByID(ctx, w.ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got == nil {
		t.Fatal("GetByID returned nil")
	}
	if got.Name != w.Name {
		t.Errorf("Name = %q, want %q", got.Name, w.Name)
	}
	if got.Version != 1 {
		t.Errorf("Version = %d, want 1", got.Version)
	}
	if got.IsActive {
		t.Error("IsActive should be false")
	}
	if got.IsDelete {
		t.Error("IsDelete should be false")
	}
}

func TestWorkspaceRepo_List(t *testing.T) {
	db := setupTestDB(t)
	repo := NewWorkspaceRepo(db)
	ctx := context.Background()

	w1 := newTestWorkspace("WS One")
	w2 := newTestWorkspace("WS Two")

	if err := repo.Create(ctx, w1); err != nil {
		t.Fatalf("Create w1: %v", err)
	}
	if err := repo.Create(ctx, w2); err != nil {
		t.Fatalf("Create w2: %v", err)
	}

	// Default workspace from seed + 2 created = 3
	list, err := repo.List(ctx)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(list) != 3 {
		t.Fatalf("List len = %d, want 3", len(list))
	}

	w1.IsDelete = true
	w1.Version++
	w1.UpdatedAt = time.Now()
	if err := repo.Update(ctx, w1); err != nil {
		t.Fatalf("Update (soft delete): %v", err)
	}

	list, err = repo.List(ctx)
	if err != nil {
		t.Fatalf("List after delete: %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("List len after delete = %d, want 2", len(list))
	}
}

func TestWorkspaceRepo_Update(t *testing.T) {
	db := setupTestDB(t)
	repo := NewWorkspaceRepo(db)
	ctx := context.Background()

	w := newTestWorkspace("Original Name")
	if err := repo.Create(ctx, w); err != nil {
		t.Fatalf("Create: %v", err)
	}

	w.Name = "Updated Name"
	w.Version++
	w.UpdatedAt = time.Now().Truncate(time.Second)
	if err := repo.Update(ctx, w); err != nil {
		t.Fatalf("Update: %v", err)
	}

	got, err := repo.GetByID(ctx, w.ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got.Name != "Updated Name" {
		t.Errorf("Name = %q, want %q", got.Name, "Updated Name")
	}
	if got.Version != 2 {
		t.Errorf("Version = %d, want 2", got.Version)
	}
}

func TestWorkspaceRepo_SetActive(t *testing.T) {
	db := setupTestDB(t)
	repo := NewWorkspaceRepo(db)
	ctx := context.Background()

	w1 := newTestWorkspace("WS One")
	w2 := newTestWorkspace("WS Two")

	if err := repo.Create(ctx, w1); err != nil {
		t.Fatalf("Create w1: %v", err)
	}
	if err := repo.Create(ctx, w2); err != nil {
		t.Fatalf("Create w2: %v", err)
	}

	if err := repo.SetActive(ctx, w1.ID); err != nil {
		t.Fatalf("SetActive w1: %v", err)
	}

	got1, _ := repo.GetByID(ctx, w1.ID)
	if !got1.IsActive {
		t.Error("w1 should be active")
	}

	if err := repo.SetActive(ctx, w2.ID); err != nil {
		t.Fatalf("SetActive w2: %v", err)
	}

	got1, _ = repo.GetByID(ctx, w1.ID)
	got2, _ := repo.GetByID(ctx, w2.ID)

	if got1.IsActive {
		t.Error("w1 should be deactivated after setting w2 active")
	}
	if !got2.IsActive {
		t.Error("w2 should be active")
	}
}

func TestWorkspaceRepo_GetActive(t *testing.T) {
	db := setupTestDB(t)
	repo := NewWorkspaceRepo(db)
	ctx := context.Background()

	// Default workspace from seed should be active
	active, err := repo.GetActive(ctx)
	if err != nil {
		t.Fatalf("GetActive: %v", err)
	}
	if active == nil {
		t.Fatal("GetActive returned nil, expected default workspace")
	}
	if active.ID != testWorkspaceID {
		t.Errorf("Active ID = %s, want %s", active.ID, testWorkspaceID)
	}

	w := newTestWorkspace("New Active")
	if err := repo.Create(ctx, w); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if err := repo.SetActive(ctx, w.ID); err != nil {
		t.Fatalf("SetActive: %v", err)
	}

	active, err = repo.GetActive(ctx)
	if err != nil {
		t.Fatalf("GetActive after switch: %v", err)
	}
	if active.ID != w.ID {
		t.Errorf("Active ID = %s, want %s", active.ID, w.ID)
	}
}

func TestWorkspaceRepo_CountNonDeleted(t *testing.T) {
	db := setupTestDB(t)
	repo := NewWorkspaceRepo(db)
	ctx := context.Background()

	// Default workspace from seed = 1
	count, err := repo.CountNonDeleted(ctx)
	if err != nil {
		t.Fatalf("CountNonDeleted: %v", err)
	}
	if count != 1 {
		t.Errorf("Count = %d, want 1", count)
	}

	w := newTestWorkspace("Second")
	if err := repo.Create(ctx, w); err != nil {
		t.Fatalf("Create: %v", err)
	}

	count, err = repo.CountNonDeleted(ctx)
	if err != nil {
		t.Fatalf("CountNonDeleted: %v", err)
	}
	if count != 2 {
		t.Errorf("Count = %d, want 2", count)
	}

	w.IsDelete = true
	w.Version++
	w.UpdatedAt = time.Now()
	if err := repo.Update(ctx, w); err != nil {
		t.Fatalf("Update: %v", err)
	}

	count, err = repo.CountNonDeleted(ctx)
	if err != nil {
		t.Fatalf("CountNonDeleted: %v", err)
	}
	if count != 1 {
		t.Errorf("Count = %d, want 1", count)
	}
}
