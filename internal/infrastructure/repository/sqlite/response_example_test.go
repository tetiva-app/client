package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/tetiva-app/client/internal/domain/entities"
)

func newTestExample(requestID uuid.UUID, name string, sortOrder int) *entities.ResponseExample {
	now := time.Now().Truncate(time.Second)
	return &entities.ResponseExample{
		ID:          uuid.New(),
		RequestID:   requestID,
		WorkspaceID: testWorkspaceID,
		Name:        name,
		StatusCode:  200,
		StatusText:  "OK",
		Headers:     []entities.HeaderItem{{Key: "Content-Type", Value: "application/json", Enabled: true}},
		Body:        `{"ok":true}`,
		ContentType: "application/json",
		Protocol:    entities.ProtocolHTTP,
		SortOrder:   sortOrder,
		Version:     1,
		CreatedBy:   "test_user",
		CreatedAt:   now,
		UpdatedBy:   "test_user",
		UpdatedAt:   now,
	}
}

// Bounded: a query that skips the caller's tx would hang on the single pooled connection.
func testCtx(t *testing.T) context.Context {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	t.Cleanup(cancel)
	return ctx
}

func exampleSynced(t *testing.T, db *sql.DB, id uuid.UUID) int {
	t.Helper()
	var synced int
	if err := db.QueryRow(`SELECT is_synced FROM response_examples WHERE id = ?`, id.String()).Scan(&synced); err != nil {
		t.Fatalf("read is_synced: %v", err)
	}
	return synced
}

func TestResponseExampleRepo_CreateAndGetByID(t *testing.T) {
	db := setupTestDB(t)
	repo := NewResponseExampleRepo(db)
	ctx := testCtx(t)

	ex := newTestExample(uuid.New(), "404 Not Found", 3)
	ex.WorkspaceID = uuid.New()
	ex.StatusCode = 404
	ex.StatusText = "Not Found"
	ex.Headers = []entities.HeaderItem{
		{Key: "Set-Cookie", Value: "a=1", Enabled: true},
		{Key: "Set-Cookie", Value: "b=2", Enabled: true},
		{Key: "X-Off", Value: "", Enabled: false},
	}
	ex.Body = "не найдено"
	ex.ContentType = "text/plain; charset=utf-8"
	ex.Protocol = entities.ProtocolGraphQL
	ex.Version = 4
	ex.CreatedBy = "alice"
	ex.UpdatedBy = "bob"
	ex.UpdatedAt = ex.CreatedAt.Add(time.Minute)

	if err := repo.Create(ctx, ex); err != nil {
		t.Fatalf("create: %v", err)
	}

	got, err := repo.GetByID(ctx, ex.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got == nil {
		t.Fatal("expected example, got nil")
	}
	if !got.CreatedAt.Equal(ex.CreatedAt) || !got.UpdatedAt.Equal(ex.UpdatedAt) {
		t.Errorf("timestamps: got %v/%v, want %v/%v", got.CreatedAt, got.UpdatedAt, ex.CreatedAt, ex.UpdatedAt)
	}
	got.CreatedAt, got.UpdatedAt = ex.CreatedAt, ex.UpdatedAt
	if !reflect.DeepEqual(got, ex) {
		t.Errorf("round trip:\n got  %+v\n want %+v", got, ex)
	}
}

func TestResponseExampleRepo_CreateWithoutHeaders(t *testing.T) {
	db := setupTestDB(t)
	repo := NewResponseExampleRepo(db)
	ctx := testCtx(t)

	ex := newTestExample(uuid.New(), "Empty", 0)
	ex.Headers = nil
	if err := repo.Create(ctx, ex); err != nil {
		t.Fatalf("create: %v", err)
	}

	var raw string
	if err := db.QueryRow(`SELECT headers FROM response_examples WHERE id = ?`, ex.ID.String()).Scan(&raw); err != nil {
		t.Fatal(err)
	}
	if raw != "[]" {
		t.Errorf("stored headers = %q, want []", raw)
	}
	got, err := repo.GetByID(ctx, ex.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Headers == nil || len(got.Headers) != 0 {
		t.Errorf("headers = %#v, want empty non-nil slice", got.Headers)
	}
}

func TestResponseExampleRepo_GetByIDMissingOrDeleted(t *testing.T) {
	db := setupTestDB(t)
	repo := NewResponseExampleRepo(db)
	ctx := testCtx(t)

	got, err := repo.GetByID(ctx, uuid.New())
	if err != nil || got != nil {
		t.Fatalf("missing: got %v, %v; want nil, nil", got, err)
	}

	ex := newTestExample(uuid.New(), "Gone", 0)
	ex.IsDelete = true
	if err := repo.Create(ctx, ex); err != nil {
		t.Fatal(err)
	}
	got, err = repo.GetByID(ctx, ex.ID)
	if err != nil || got != nil {
		t.Fatalf("deleted: got %v, %v; want nil, nil", got, err)
	}
}

func TestResponseExampleRepo_ListByRequest(t *testing.T) {
	db := setupTestDB(t)
	repo := NewResponseExampleRepo(db)
	ctx := testCtx(t)

	requestID := uuid.New()
	base := time.Now().Truncate(time.Second)

	third := newTestExample(requestID, "third", 2)
	firstOlder := newTestExample(requestID, "first-older", 1)
	firstOlder.CreatedAt = base.Add(-time.Hour)
	firstNewer := newTestExample(requestID, "first-newer", 1)
	firstNewer.CreatedAt = base
	deleted := newTestExample(requestID, "deleted", 0)
	deleted.IsDelete = true
	other := newTestExample(uuid.New(), "other request", 0)

	for _, ex := range []*entities.ResponseExample{third, firstNewer, deleted, firstOlder, other} {
		if err := repo.Create(ctx, ex); err != nil {
			t.Fatalf("create %s: %v", ex.Name, err)
		}
	}

	list, err := repo.ListByRequest(ctx, requestID)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	var names []string
	for _, ex := range list {
		names = append(names, ex.Name)
	}
	want := []string{"first-older", "first-newer", "third"}
	if !reflect.DeepEqual(names, want) {
		t.Errorf("names = %v, want %v", names, want)
	}

	empty, err := repo.ListByRequest(ctx, uuid.New())
	if err != nil || len(empty) != 0 {
		t.Errorf("unknown request: got %v, %v", empty, err)
	}
}

func TestResponseExampleRepo_LocalWritesClearSynced(t *testing.T) {
	db := setupTestDB(t)
	repo := NewResponseExampleRepo(db)
	ctx := testCtx(t)

	ex := newTestExample(uuid.New(), "Synced", 0)
	if err := repo.Create(ctx, ex); err != nil {
		t.Fatal(err)
	}
	if got := exampleSynced(t, db, ex.ID); got != 0 {
		t.Fatalf("after Create is_synced = %d, want 0", got)
	}

	if _, err := db.Exec(`UPDATE response_examples SET is_synced = 1 WHERE id = ?`, ex.ID.String()); err != nil {
		t.Fatal(err)
	}
	ex.Name = "Edited"
	ex.Version++
	if err := repo.Update(ctx, ex); err != nil {
		t.Fatalf("update: %v", err)
	}
	if got := exampleSynced(t, db, ex.ID); got != 0 {
		t.Errorf("after Update is_synced = %d, want 0", got)
	}

	if _, err := db.Exec(`UPDATE response_examples SET is_synced = 1 WHERE id = ?`, ex.ID.String()); err != nil {
		t.Fatal(err)
	}
	ex.IsDelete = true
	ex.Version++
	if err := repo.Update(ctx, ex); err != nil {
		t.Fatalf("soft delete: %v", err)
	}
	if got := exampleSynced(t, db, ex.ID); got != 0 {
		t.Errorf("after soft delete is_synced = %d, want 0", got)
	}
}

func TestResponseExampleRepo_UpdateSoftDeletedRow(t *testing.T) {
	db := setupTestDB(t)
	repo := NewResponseExampleRepo(db)
	ctx := testCtx(t)

	ex := newTestExample(uuid.New(), "Original", 0)
	if err := repo.Create(ctx, ex); err != nil {
		t.Fatal(err)
	}

	ex.IsDelete = true
	ex.Version = 2
	if err := repo.Update(ctx, ex); err != nil {
		t.Fatalf("soft delete: %v", err)
	}
	if got, _ := repo.GetByID(ctx, ex.ID); got != nil {
		t.Fatal("soft-deleted example still returned by GetByID")
	}

	ex.Name = "Tombstone touched"
	ex.Version = 3
	ex.UpdatedAt = ex.UpdatedAt.Add(time.Minute)
	if err := repo.Update(ctx, ex); err != nil {
		t.Fatalf("update of a soft-deleted row: %v", err)
	}
	var name string
	var version, isDelete int
	if err := db.QueryRow(`SELECT name, version, is_delete FROM response_examples WHERE id = ?`, ex.ID.String()).
		Scan(&name, &version, &isDelete); err != nil {
		t.Fatal(err)
	}
	if name != "Tombstone touched" || version != 3 || isDelete != 1 {
		t.Errorf("row = (%q, %d, %d), want (Tombstone touched, 3, 1)", name, version, isDelete)
	}

	ex.IsDelete = false
	ex.Version = 4
	if err := repo.Update(ctx, ex); err != nil {
		t.Fatalf("restore: %v", err)
	}
	got, err := repo.GetByID(ctx, ex.ID)
	if err != nil || got == nil || got.Version != 4 {
		t.Fatalf("restored: got %+v, %v", got, err)
	}
}

func TestResponseExampleRepo_JoinsCallerTx(t *testing.T) {
	db := setupTestDB(t)
	repo := NewResponseExampleRepo(db)
	ctx := testCtx(t)

	kept := newTestExample(uuid.New(), "Kept", 0)
	if err := repo.Create(ctx, kept); err != nil {
		t.Fatal(err)
	}

	errBoom := errors.New("boom")
	created := newTestExample(kept.RequestID, "Rolled back", 1)
	err := WithTx(ctx, db, func(txCtx context.Context) error {
		if err := repo.Create(txCtx, created); err != nil {
			return err
		}
		kept.IsDelete = true
		if err := repo.Update(txCtx, kept); err != nil {
			return err
		}
		inside, err := repo.ListByRequest(txCtx, kept.RequestID)
		if err != nil {
			return err
		}
		if len(inside) != 1 || inside[0].ID != created.ID {
			t.Errorf("inside tx: got %d examples, want only the new one", len(inside))
		}
		if got, err := repo.GetByID(txCtx, created.ID); err != nil || got == nil {
			t.Errorf("inside tx GetByID: %v, %v", got, err)
		}
		return errBoom
	})
	if !errors.Is(err, errBoom) {
		t.Fatalf("WithTx err = %v, want boom", err)
	}

	if got, err := repo.GetByID(ctx, created.ID); err != nil || got != nil {
		t.Errorf("rolled-back create visible: %v, %v", got, err)
	}
	if got, err := repo.GetByID(ctx, kept.ID); err != nil || got == nil {
		t.Errorf("rolled-back soft delete applied: %v, %v", got, err)
	}
}
