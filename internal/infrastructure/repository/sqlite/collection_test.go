package sqlite

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/tetiva-app/client/internal/domain/entities"
	"github.com/tetiva-app/client/internal/domain/usecase/collection"
	"github.com/tetiva-app/client/migrations"
	"github.com/tetiva-app/client/pkg/migrate"
)

var testWorkspaceID = uuid.MustParse("00000000-0000-4000-a000-000000000001")

func setupTestDB(t *testing.T) *sql.DB {
	t.Helper()
	// A file DB with one connection: ":memory:" hands every pooled connection its
	// own empty database, and the fk-off migration runs on a dedicated one.
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	if _, err := db.Exec("PRAGMA foreign_keys=ON"); err != nil {
		t.Fatal(err)
	}
	if err := migrate.Run(db, migrations.FS, "."); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func newTestCollection(name string, parentID *uuid.UUID) *entities.Collection {
	now := time.Now().Truncate(time.Second)
	return &entities.Collection{
		ID:           uuid.New(),
		WorkspaceID:  testWorkspaceID,
		ParentID:     parentID,
		Name:         name,
		AuthType:     entities.AuthTypeNone,
		AuthData:     "{}",
		GRPCMetadata: []entities.HeaderItem{},
		SortOrder:    0,
		Version:      1,
		IsDelete:     false,
		CreatedBy:    "test_user",
		CreatedAt:    now,
		UpdatedBy:    "test_user",
		UpdatedAt:    now,
	}
}

func TestCollectionRepo_CreateAndGetByID(t *testing.T) {
	db := setupTestDB(t)
	repo := NewCollectionRepo(db)
	ctx := context.Background()

	c := newTestCollection("My Collection", nil)

	if err := repo.Create(ctx, c); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	got, err := repo.GetByID(ctx, c.ID)
	if err != nil {
		t.Fatalf("GetByID failed: %v", err)
	}
	if got == nil {
		t.Fatal("GetByID returned nil")
	}

	if got.ID != c.ID {
		t.Errorf("ID mismatch: got %s, want %s", got.ID, c.ID)
	}
	if got.WorkspaceID != c.WorkspaceID {
		t.Errorf("WorkspaceID mismatch: got %s, want %s", got.WorkspaceID, c.WorkspaceID)
	}
	if got.ParentID != nil {
		t.Errorf("ParentID should be nil, got %v", got.ParentID)
	}
	if got.Name != c.Name {
		t.Errorf("Name mismatch: got %q, want %q", got.Name, c.Name)
	}
	if got.SortOrder != c.SortOrder {
		t.Errorf("SortOrder mismatch: got %d, want %d", got.SortOrder, c.SortOrder)
	}
	if got.Version != c.Version {
		t.Errorf("Version mismatch: got %d, want %d", got.Version, c.Version)
	}
	if got.IsDelete != false {
		t.Errorf("IsDelete should be false")
	}
	if got.CreatedBy != c.CreatedBy {
		t.Errorf("CreatedBy mismatch: got %q, want %q", got.CreatedBy, c.CreatedBy)
	}

	notFound, err := repo.GetByID(ctx, uuid.New())
	if err != nil {
		t.Fatalf("GetByID for missing ID returned error: %v", err)
	}
	if notFound != nil {
		t.Fatal("GetByID for missing ID should return nil")
	}
}

func TestCollectionRepo_List(t *testing.T) {
	db := setupTestDB(t)
	repo := NewCollectionRepo(db)
	ctx := context.Background()

	c1 := newTestCollection("Gamma", nil)
	c1.SortOrder = 2
	c2 := newTestCollection("Alpha", nil)
	c2.SortOrder = 0
	c3 := newTestCollection("Beta", nil)
	c3.SortOrder = 1

	for _, c := range []*entities.Collection{c1, c2, c3} {
		if err := repo.Create(ctx, c); err != nil {
			t.Fatalf("Create failed: %v", err)
		}
	}

	filter := collection.Filter{
		WorkspaceID: testWorkspaceID,
	}
	list, err := repo.List(ctx, filter)
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}

	if len(list) != 3 {
		t.Fatalf("expected 3 collections, got %d", len(list))
	}

	if list[0].Name != "Alpha" {
		t.Errorf("first collection should be Alpha (sort_order=0), got %q", list[0].Name)
	}
	if list[1].Name != "Beta" {
		t.Errorf("second collection should be Beta (sort_order=1), got %q", list[1].Name)
	}
	if list[2].Name != "Gamma" {
		t.Errorf("third collection should be Gamma (sort_order=2), got %q", list[2].Name)
	}

	otherFilter := collection.Filter{
		WorkspaceID: uuid.New(),
	}
	empty, err := repo.List(ctx, otherFilter)
	if err != nil {
		t.Fatalf("List with other workspace failed: %v", err)
	}
	if len(empty) != 0 {
		t.Errorf("expected 0 collections for unknown workspace, got %d", len(empty))
	}
}

func TestCollectionRepo_Update(t *testing.T) {
	db := setupTestDB(t)
	repo := NewCollectionRepo(db)
	ctx := context.Background()

	c := newTestCollection("Original", nil)
	if err := repo.Create(ctx, c); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	c.Name = "Updated"
	c.Version = 2
	c.UpdatedBy = "editor"
	c.UpdatedAt = time.Now().Truncate(time.Second)

	if err := repo.Update(ctx, c); err != nil {
		t.Fatalf("Update failed: %v", err)
	}

	got, err := repo.GetByID(ctx, c.ID)
	if err != nil {
		t.Fatalf("GetByID after update failed: %v", err)
	}
	if got == nil {
		t.Fatal("GetByID returned nil after update")
	}

	if got.Name != "Updated" {
		t.Errorf("Name not updated: got %q, want %q", got.Name, "Updated")
	}
	if got.Version != 2 {
		t.Errorf("Version not updated: got %d, want %d", got.Version, 2)
	}
	if got.UpdatedBy != "editor" {
		t.Errorf("UpdatedBy not updated: got %q, want %q", got.UpdatedBy, "editor")
	}
}

func TestCollectionRepo_SoftDelete(t *testing.T) {
	db := setupTestDB(t)
	repo := NewCollectionRepo(db)
	ctx := context.Background()

	c := newTestCollection("ToDelete", nil)
	if err := repo.Create(ctx, c); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	c.IsDelete = true
	c.Version = 2
	if err := repo.Update(ctx, c); err != nil {
		t.Fatalf("Update for soft delete failed: %v", err)
	}

	got, err := repo.GetByID(ctx, c.ID)
	if err != nil {
		t.Fatalf("GetByID after soft delete returned error: %v", err)
	}
	if got != nil {
		t.Error("GetByID should return nil for soft-deleted collection")
	}

	filter := collection.Filter{WorkspaceID: testWorkspaceID}
	list, err := repo.List(ctx, filter)
	if err != nil {
		t.Fatalf("List after soft delete failed: %v", err)
	}
	for _, item := range list {
		if item.ID == c.ID {
			t.Error("List should not include soft-deleted collection")
		}
	}
}

func TestCollectionRepo_NestedCollection(t *testing.T) {
	db := setupTestDB(t)
	repo := NewCollectionRepo(db)
	ctx := context.Background()

	parent := newTestCollection("Parent", nil)
	if err := repo.Create(ctx, parent); err != nil {
		t.Fatalf("Create parent failed: %v", err)
	}

	child := newTestCollection("Child", &parent.ID)
	if err := repo.Create(ctx, child); err != nil {
		t.Fatalf("Create child failed: %v", err)
	}

	got, err := repo.GetByID(ctx, child.ID)
	if err != nil {
		t.Fatalf("GetByID child failed: %v", err)
	}
	if got == nil {
		t.Fatal("GetByID child returned nil")
	}
	if got.ParentID == nil {
		t.Fatal("Child ParentID should not be nil")
	}
	if *got.ParentID != parent.ID {
		t.Errorf("Child ParentID mismatch: got %s, want %s", *got.ParentID, parent.ID)
	}

	filter := collection.Filter{
		WorkspaceID: testWorkspaceID,
		ParentID:    &parent.ID,
	}
	list, err := repo.List(ctx, filter)
	if err != nil {
		t.Fatalf("List with ParentID filter failed: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("expected 1 child collection, got %d", len(list))
	}
	if list[0].ID != child.ID {
		t.Errorf("expected child ID %s, got %s", child.ID, list[0].ID)
	}

	allFilter := collection.Filter{WorkspaceID: testWorkspaceID}
	all, err := repo.List(ctx, allFilter)
	if err != nil {
		t.Fatalf("List all failed: %v", err)
	}
	if len(all) != 2 {
		t.Fatalf("expected 2 collections, got %d", len(all))
	}
}
