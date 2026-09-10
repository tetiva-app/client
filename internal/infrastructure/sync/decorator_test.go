package sync

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
	_ "modernc.org/sqlite"

	"github.com/tetiva-app/client/internal/domain/entities"
	"github.com/tetiva-app/client/internal/infrastructure/repository/sqlite"
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

func countQueueEntries(t *testing.T, db *sql.DB, entityType, entityID string) int {
	t.Helper()
	var n int
	err := db.QueryRowContext(context.Background(),
		`SELECT COUNT(*) FROM sync_queue WHERE entity_type = ? AND entity_id = ?`,
		entityType, entityID).Scan(&n)
	if err != nil {
		t.Fatalf("countQueueEntries: %v", err)
	}
	return n
}

func countQueueEntriesWithAction(t *testing.T, db *sql.DB, entityID, action string) int {
	t.Helper()
	var n int
	err := db.QueryRowContext(context.Background(),
		`SELECT COUNT(*) FROM sync_queue WHERE entity_id = ? AND action = ?`,
		entityID, action).Scan(&n)
	if err != nil {
		t.Fatalf("countQueueEntriesWithAction: %v", err)
	}
	return n
}

func testEngineWithSync(workspaceID string) *SyncEngine {
	e := &SyncEngine{}
	e.enabledWorkspaces.Store(workspaceID, true)
	return e
}

func TestSyncedCollectionRepo_Create_SyncEnabled(t *testing.T) {
	db := setupTestDB(t)
	inner := sqlite.NewCollectionRepo(db)
	queueRepo := sqlite.NewSyncQueueRepo(db)
	engine := testEngineWithSync(testWorkspaceID.String())
	decorator := NewSyncedCollectionRepo(inner, queueRepo, db, engine)

	ctx := context.Background()
	c := newTestCollection("SyncedCreate", nil)

	if err := decorator.Create(ctx, c); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	got, err := inner.GetByID(ctx, c.ID)
	if err != nil || got == nil {
		t.Fatalf("collection not found after Create: %v", err)
	}

	n := countQueueEntriesWithAction(t, db, c.ID.String(), "create")
	if n != 1 {
		t.Errorf("expected 1 queue entry with action=create, got %d", n)
	}
}

func TestSyncedCollectionRepo_Create_SyncDisabled(t *testing.T) {
	db := setupTestDB(t)
	inner := sqlite.NewCollectionRepo(db)
	queueRepo := sqlite.NewSyncQueueRepo(db)
	decorator := NewSyncedCollectionRepo(inner, queueRepo, db, nil)

	ctx := context.Background()
	c := newTestCollection("NoSyncCreate", nil)

	if err := decorator.Create(ctx, c); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	n := countQueueEntries(t, db, "collection", c.ID.String())
	if n != 0 {
		t.Errorf("expected 0 queue entries when sync disabled, got %d", n)
	}
}

func TestSyncedCollectionRepo_Update_ActionUpdate(t *testing.T) {
	db := setupTestDB(t)
	inner := sqlite.NewCollectionRepo(db)
	queueRepo := sqlite.NewSyncQueueRepo(db)
	engine := testEngineWithSync(testWorkspaceID.String())
	decorator := NewSyncedCollectionRepo(inner, queueRepo, db, engine)

	ctx := context.Background()
	c := newTestCollection("UpdateTest", nil)
	if err := inner.Create(ctx, c); err != nil {
		t.Fatalf("inner Create: %v", err)
	}

	c.Name = "Updated"
	c.Version = 2
	if err := decorator.Update(ctx, c); err != nil {
		t.Fatalf("Update failed: %v", err)
	}

	n := countQueueEntriesWithAction(t, db, c.ID.String(), "update")
	if n != 1 {
		t.Errorf("expected 1 queue entry with action=update, got %d", n)
	}
}

func TestSyncedCollectionRepo_Update_ActionDelete(t *testing.T) {
	db := setupTestDB(t)
	inner := sqlite.NewCollectionRepo(db)
	queueRepo := sqlite.NewSyncQueueRepo(db)
	engine := testEngineWithSync(testWorkspaceID.String())
	decorator := NewSyncedCollectionRepo(inner, queueRepo, db, engine)

	ctx := context.Background()
	c := newTestCollection("SoftDeleteTest", nil)
	if err := inner.Create(ctx, c); err != nil {
		t.Fatalf("inner Create: %v", err)
	}

	c.IsDelete = true
	c.Version = 2
	if err := decorator.Update(ctx, c); err != nil {
		t.Fatalf("Update (soft delete) failed: %v", err)
	}

	n := countQueueEntriesWithAction(t, db, c.ID.String(), "delete")
	if n != 1 {
		t.Errorf("expected 1 queue entry with action=delete for soft-deleted collection, got %d", n)
	}
}

func TestSyncedCollectionRepo_SoftDeleteDescendants(t *testing.T) {
	db := setupTestDB(t)
	inner := sqlite.NewCollectionRepo(db)
	queueRepo := sqlite.NewSyncQueueRepo(db)
	engine := testEngineWithSync(testWorkspaceID.String())
	decorator := NewSyncedCollectionRepo(inner, queueRepo, db, engine)

	ctx := context.Background()

	// Tree: parent → child1, child2; child1 → grandchild
	parent := newTestCollection("Parent", nil)
	child1 := newTestCollection("Child1", &parent.ID)
	child2 := newTestCollection("Child2", &parent.ID)
	grandchild := newTestCollection("Grandchild", &child1.ID)

	for _, c := range []*entities.Collection{parent, child1, child2, grandchild} {
		if err := inner.Create(ctx, c); err != nil {
			t.Fatalf("inner Create %s: %v", c.Name, err)
		}
	}

	now := time.Now().Truncate(time.Second)
	if err := decorator.SoftDeleteDescendants(ctx, parent.ID, "test_user", now); err != nil {
		t.Fatalf("SoftDeleteDescendants failed: %v", err)
	}

	descendantIDs := []string{child1.ID.String(), child2.ID.String(), grandchild.ID.String()}
	for _, id := range descendantIDs {
		n := countQueueEntriesWithAction(t, db, id, "delete")
		if n != 1 {
			t.Errorf("expected 1 delete queue entry for descendant %s, got %d", id, n)
		}
	}

	// SoftDeleteDescendants only covers children, not the parent itself.
	n := countQueueEntries(t, db, "collection", parent.ID.String())
	if n != 0 {
		t.Errorf("parent should not have queue entries, got %d", n)
	}
}

func TestSyncedCollectionRepo_SoftDeleteDescendants_SyncDisabled(t *testing.T) {
	db := setupTestDB(t)
	inner := sqlite.NewCollectionRepo(db)
	queueRepo := sqlite.NewSyncQueueRepo(db)
	decorator := NewSyncedCollectionRepo(inner, queueRepo, db, nil)

	ctx := context.Background()
	parent := newTestCollection("Parent", nil)
	child := newTestCollection("Child", &parent.ID)
	for _, c := range []*entities.Collection{parent, child} {
		if err := inner.Create(ctx, c); err != nil {
			t.Fatalf("inner Create: %v", err)
		}
	}

	now := time.Now().Truncate(time.Second)
	if err := decorator.SoftDeleteDescendants(ctx, parent.ID, "test_user", now); err != nil {
		t.Fatalf("SoftDeleteDescendants failed: %v", err)
	}

	n := countQueueEntries(t, db, "collection", child.ID.String())
	if n != 0 {
		t.Errorf("expected 0 queue entries when sync disabled, got %d", n)
	}
}
