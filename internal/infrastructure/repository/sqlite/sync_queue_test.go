package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/tetiva-app/client/internal/domain"
)

func newTestSyncEntry(workspaceID, entityType, entityID, action string) SyncEntry {
	return SyncEntry{
		WorkspaceID: workspaceID,
		EntityType:  entityType,
		EntityID:    entityID,
		Action:      action,
		OperationID: uuid.New().String(),
		Status:      "pending",
		RetryCount:  0,
		CreatedAt:   time.Now().Truncate(time.Second),
	}
}

func TestSyncQueueRepo_EnqueueAndListPending(t *testing.T) {
	db := setupTestDB(t)
	repo := NewSyncQueueRepo(db)
	ctx := context.Background()

	wsID := uuid.New().String()
	e1 := newTestSyncEntry(wsID, "collection", uuid.New().String(), "upsert")
	e2 := newTestSyncEntry(wsID, "request", uuid.New().String(), "upsert")

	if err := repo.Enqueue(ctx, e1); err != nil {
		t.Fatalf("Enqueue e1 failed: %v", err)
	}
	if err := repo.Enqueue(ctx, e2); err != nil {
		t.Fatalf("Enqueue e2 failed: %v", err)
	}

	list, err := repo.ListPending(ctx, wsID, 10)
	if err != nil {
		t.Fatalf("ListPending failed: %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("expected 2 pending entries, got %d", len(list))
	}

	if list[0].EntityType != "collection" {
		t.Errorf("expected first entry to be 'collection', got %q", list[0].EntityType)
	}
	if list[1].EntityType != "request" {
		t.Errorf("expected second entry to be 'request', got %q", list[1].EntityType)
	}

	other, err := repo.ListPending(ctx, uuid.New().String(), 10)
	if err != nil {
		t.Fatalf("ListPending other workspace failed: %v", err)
	}
	if len(other) != 0 {
		t.Errorf("expected 0 entries for other workspace, got %d", len(other))
	}
}

func TestSyncQueueRepo_CoalescedPending(t *testing.T) {
	db := setupTestDB(t)
	repo := NewSyncQueueRepo(db)
	ctx := context.Background()

	wsID := uuid.New().String()
	entityID := uuid.New().String()

	// Three entries for the same entity — only the latest (highest id) should survive.
	e1 := newTestSyncEntry(wsID, "collection", entityID, "upsert")
	e2 := newTestSyncEntry(wsID, "collection", entityID, "upsert")
	e3 := newTestSyncEntry(wsID, "collection", entityID, "delete")

	for _, e := range []SyncEntry{e1, e2, e3} {
		if err := repo.Enqueue(ctx, e); err != nil {
			t.Fatalf("Enqueue failed: %v", err)
		}
	}

	other := newTestSyncEntry(wsID, "request", uuid.New().String(), "upsert")
	if err := repo.Enqueue(ctx, other); err != nil {
		t.Fatalf("Enqueue other failed: %v", err)
	}

	list, err := repo.CoalescedPending(ctx, wsID, 10, nil)
	if err != nil {
		t.Fatalf("CoalescedPending failed: %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("expected 2 coalesced entries (1 per entity), got %d", len(list))
	}

	var found *SyncEntry
	for _, e := range list {
		if e.EntityType == "collection" {
			found = e
			break
		}
	}
	if found == nil {
		t.Fatal("coalesced result missing collection entry")
	}
	if found.Action != "delete" {
		t.Errorf("expected latest action 'delete', got %q", found.Action)
	}
	if found.OperationID != e3.OperationID {
		t.Errorf("expected latest operation_id %q, got %q", e3.OperationID, found.OperationID)
	}
}

func TestSyncQueueRepo_MarkSendingAndResetSending(t *testing.T) {
	db := setupTestDB(t)
	repo := NewSyncQueueRepo(db)
	ctx := context.Background()

	wsID := uuid.New().String()
	e1 := newTestSyncEntry(wsID, "collection", uuid.New().String(), "upsert")
	e2 := newTestSyncEntry(wsID, "request", uuid.New().String(), "upsert")

	if err := repo.Enqueue(ctx, e1); err != nil {
		t.Fatalf("Enqueue failed: %v", err)
	}
	if err := repo.Enqueue(ctx, e2); err != nil {
		t.Fatalf("Enqueue failed: %v", err)
	}

	list, _ := repo.ListPending(ctx, wsID, 10)
	ids := make([]int64, len(list))
	for i, e := range list {
		ids[i] = e.ID
	}

	if err := repo.MarkSending(ctx, ids); err != nil {
		t.Fatalf("MarkSending failed: %v", err)
	}

	pending, _ := repo.ListPending(ctx, wsID, 10)
	if len(pending) != 0 {
		t.Errorf("expected 0 pending after MarkSending, got %d", len(pending))
	}

	if err := repo.ResetSending(ctx); err != nil {
		t.Fatalf("ResetSending failed: %v", err)
	}

	pending, _ = repo.ListPending(ctx, wsID, 10)
	if len(pending) != 2 {
		t.Errorf("expected 2 pending after ResetSending, got %d", len(pending))
	}
}

func TestSyncQueueRepo_Delete(t *testing.T) {
	db := setupTestDB(t)
	repo := NewSyncQueueRepo(db)
	ctx := context.Background()

	wsID := uuid.New().String()
	e1 := newTestSyncEntry(wsID, "collection", uuid.New().String(), "upsert")
	e2 := newTestSyncEntry(wsID, "request", uuid.New().String(), "upsert")

	if err := repo.Enqueue(ctx, e1); err != nil {
		t.Fatalf("Enqueue e1 failed: %v", err)
	}
	if err := repo.Enqueue(ctx, e2); err != nil {
		t.Fatalf("Enqueue e2 failed: %v", err)
	}

	list, _ := repo.ListPending(ctx, wsID, 10)
	if len(list) != 2 {
		t.Fatalf("expected 2 entries before delete, got %d", len(list))
	}

	if err := repo.Delete(ctx, []int64{list[0].ID}); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	remaining, _ := repo.ListPending(ctx, wsID, 10)
	if len(remaining) != 1 {
		t.Fatalf("expected 1 entry after delete, got %d", len(remaining))
	}
	if remaining[0].ID != list[1].ID {
		t.Errorf("remaining entry ID mismatch: got %d, want %d", remaining[0].ID, list[1].ID)
	}

	if err := repo.Delete(ctx, []int64{}); err != nil {
		t.Fatalf("Delete with empty list failed: %v", err)
	}
}

func TestSyncQueueRepo_MarkFailed(t *testing.T) {
	db := setupTestDB(t)
	repo := NewSyncQueueRepo(db)
	ctx := context.Background()

	wsID := uuid.New().String()
	e := newTestSyncEntry(wsID, "collection", uuid.New().String(), "upsert")

	if err := repo.Enqueue(ctx, e); err != nil {
		t.Fatalf("Enqueue failed: %v", err)
	}

	list, _ := repo.ListPending(ctx, wsID, 10)
	id := list[0].ID

	nextRetry := time.Now().Add(5 * time.Minute).Truncate(time.Second)
	if err := repo.MarkFailed(ctx, id, nextRetry); err != nil {
		t.Fatalf("MarkFailed failed: %v", err)
	}

	pending, _ := repo.ListPending(ctx, wsID, 10)
	if len(pending) != 0 {
		t.Errorf("expected 0 pending after MarkFailed, got %d", len(pending))
	}

	var retryCount int
	var nextRetryAtStr string
	err := db.QueryRowContext(ctx, `SELECT retry_count, next_retry_at FROM sync_queue WHERE id = ?`, id).
		Scan(&retryCount, &nextRetryAtStr)
	if err != nil {
		t.Fatalf("direct query failed: %v", err)
	}
	if retryCount != 1 {
		t.Errorf("expected retry_count=1, got %d", retryCount)
	}

	got, err := parseTime(nextRetryAtStr)
	if err != nil {
		t.Fatalf("parse next_retry_at: %v", err)
	}
	if !got.Equal(nextRetry) {
		t.Errorf("next_retry_at mismatch: got %v, want %v", got, nextRetry)
	}
}

func TestSyncQueueRepo_DeleteByWorkspace(t *testing.T) {
	db := setupTestDB(t)
	repo := NewSyncQueueRepo(db)
	ctx := context.Background()

	ws1 := uuid.New().String()
	ws2 := uuid.New().String()

	for i := 0; i < 3; i++ {
		if err := repo.Enqueue(ctx, newTestSyncEntry(ws1, "collection", uuid.New().String(), "upsert")); err != nil {
			t.Fatalf("Enqueue ws1 failed: %v", err)
		}
	}
	for i := 0; i < 2; i++ {
		if err := repo.Enqueue(ctx, newTestSyncEntry(ws2, "collection", uuid.New().String(), "upsert")); err != nil {
			t.Fatalf("Enqueue ws2 failed: %v", err)
		}
	}

	n, err := repo.DeleteByWorkspace(ctx, ws1)
	if err != nil {
		t.Fatalf("DeleteByWorkspace failed: %v", err)
	}
	if n != 3 {
		t.Errorf("expected 3 deleted, got %d", n)
	}

	list, _ := repo.ListPending(ctx, ws1, 10)
	if len(list) != 0 {
		t.Errorf("expected 0 entries for ws1 after delete, got %d", len(list))
	}

	list2, _ := repo.ListPending(ctx, ws2, 10)
	if len(list2) != 2 {
		t.Errorf("expected 2 entries for ws2 to remain, got %d", len(list2))
	}
}

func TestSyncQueueRepo_RequeueDue(t *testing.T) {
	db := setupTestDB(t)
	repo := NewSyncQueueRepo(db)
	ctx := context.Background()

	wsID := uuid.New().String()
	for _, entityType := range []string{"collection", "request"} {
		if err := repo.Enqueue(ctx, newTestSyncEntry(wsID, entityType, uuid.New().String(), "upsert")); err != nil {
			t.Fatalf("Enqueue %s failed: %v", entityType, err)
		}
	}

	list, err := repo.ListPending(ctx, wsID, 10)
	if err != nil {
		t.Fatalf("ListPending failed: %v", err)
	}
	now := time.Now()
	if err := repo.MarkFailed(ctx, list[0].ID, now.Add(-time.Minute)); err != nil {
		t.Fatalf("MarkFailed due entry failed: %v", err)
	}
	if err := repo.MarkFailed(ctx, list[1].ID, now.Add(5*time.Minute)); err != nil {
		t.Fatalf("MarkFailed pending entry failed: %v", err)
	}

	n, err := repo.RequeueDue(ctx, wsID, now)
	if err != nil {
		t.Fatalf("RequeueDue failed: %v", err)
	}
	if n != 1 {
		t.Errorf("expected 1 requeued entry, got %d", n)
	}

	pending, err := repo.ListPending(ctx, wsID, 10)
	if err != nil {
		t.Fatalf("ListPending after RequeueDue failed: %v", err)
	}
	if len(pending) != 1 || pending[0].ID != list[0].ID {
		t.Fatalf("expected only the due entry back in the queue, got %+v", pending)
	}

	n, err = repo.RequeueDue(ctx, uuid.New().String(), now.Add(time.Hour))
	if err != nil {
		t.Fatalf("RequeueDue other workspace failed: %v", err)
	}
	if n != 0 {
		t.Errorf("expected 0 requeued entries for another workspace, got %d", n)
	}
}

func TestSyncQueueRepo_CountParked(t *testing.T) {
	db := setupTestDB(t)
	repo := NewSyncQueueRepo(db)
	ctx := context.Background()

	wsID := uuid.New().String()
	for i := 0; i < 3; i++ {
		if err := repo.Enqueue(ctx, newTestSyncEntry(wsID, "collection", uuid.New().String(), "upsert")); err != nil {
			t.Fatalf("Enqueue failed: %v", err)
		}
	}
	if err := repo.Enqueue(ctx, newTestSyncEntry(uuid.New().String(), "collection", uuid.New().String(), "upsert")); err != nil {
		t.Fatalf("Enqueue other workspace failed: %v", err)
	}

	list, _ := repo.ListPending(ctx, wsID, 10)
	for _, e := range list[:2] {
		if err := repo.MarkFailed(ctx, e.ID, time.Now().Add(5*time.Minute)); err != nil {
			t.Fatalf("MarkFailed failed: %v", err)
		}
	}

	if err := repo.MarkParked(ctx, list[2].ID); err != nil {
		t.Fatalf("MarkParked failed: %v", err)
	}

	count, err := repo.CountParked(ctx, wsID)
	if err != nil {
		t.Fatalf("CountParked failed: %v", err)
	}
	if count != 2 {
		t.Errorf("CountParked() = %d, want 2", count)
	}

	tooLarge, err := repo.CountTooLarge(ctx, wsID)
	if err != nil {
		t.Fatalf("CountTooLarge failed: %v", err)
	}
	if tooLarge != 1 {
		t.Errorf("CountTooLarge() = %d, want 1", tooLarge)
	}

	if _, err := repo.RequeueDue(ctx, wsID, time.Now().Add(time.Hour)); err != nil {
		t.Fatalf("RequeueDue failed: %v", err)
	}
	count, err = repo.CountParked(ctx, wsID)
	if err != nil {
		t.Fatalf("CountParked after requeue failed: %v", err)
	}
	if count != 0 {
		t.Errorf("CountParked() after requeue = %d, want 0", count)
	}

	tooLarge, err = repo.CountTooLarge(ctx, wsID)
	if err != nil {
		t.Fatalf("CountTooLarge after requeue failed: %v", err)
	}
	if tooLarge != 1 {
		t.Errorf("CountTooLarge() after requeue = %d, want 1: no retry window shrinks an entity", tooLarge)
	}
}

func TestSyncQueueRepo_MarkParked_SurvivesRequeueDue(t *testing.T) {
	db := setupTestDB(t)
	repo := NewSyncQueueRepo(db)
	ctx := context.Background()

	wsID := uuid.New().String()
	if err := repo.Enqueue(ctx, newTestSyncEntry(wsID, "request", uuid.New().String(), "update")); err != nil {
		t.Fatalf("Enqueue failed: %v", err)
	}
	list, _ := repo.ListPending(ctx, wsID, 10)
	if err := repo.MarkParked(ctx, list[0].ID); err != nil {
		t.Fatalf("MarkParked failed: %v", err)
	}

	n, err := repo.RequeueDue(ctx, wsID, time.Now().Add(24*time.Hour))
	if err != nil {
		t.Fatalf("RequeueDue failed: %v", err)
	}
	if n != 0 {
		t.Errorf("RequeueDue() revived %d parked entries, want 0", n)
	}

	pending, _ := repo.ListPending(ctx, wsID, 10)
	if len(pending) != 0 {
		t.Errorf("ListPending() = %d, want 0", len(pending))
	}

	if _, ok, err := repo.EarliestParkedRetryAt(ctx, wsID); err != nil {
		t.Fatalf("EarliestParkedRetryAt failed: %v", err)
	} else if ok {
		t.Error("a parked entry must not arm a retry wake-up")
	}

	parked, err := repo.CountParked(ctx, wsID)
	if err != nil {
		t.Fatalf("CountParked failed: %v", err)
	}
	if parked != 0 {
		t.Errorf("CountParked() = %d, want 0: an upgrade does not shrink an oversized entity", parked)
	}

	tooLarge, err := repo.CountTooLarge(ctx, wsID)
	if err != nil {
		t.Fatalf("CountTooLarge failed: %v", err)
	}
	if tooLarge != 1 {
		t.Errorf("CountTooLarge() = %d, want 1", tooLarge)
	}

	unsynced, err := repo.CountPendingOrFailed(ctx, wsID, nil)
	if err != nil {
		t.Fatalf("CountPendingOrFailed failed: %v", err)
	}
	if unsynced != 1 {
		t.Errorf("CountPendingOrFailed() = %d, want 1", unsynced)
	}
}

func TestSyncQueueRepo_DeleteSupersededParked(t *testing.T) {
	db := setupTestDB(t)
	repo := NewSyncQueueRepo(db)
	ctx := context.Background()

	wsID := uuid.New().String()
	superseded := uuid.New().String()
	lonely := uuid.New().String()

	for _, id := range []string{superseded, lonely} {
		if err := repo.Enqueue(ctx, newTestSyncEntry(wsID, "request", id, "update")); err != nil {
			t.Fatalf("Enqueue failed: %v", err)
		}
	}
	if err := repo.Enqueue(ctx, newTestSyncEntry(wsID, "request", superseded, "update")); err != nil {
		t.Fatalf("Enqueue newer failed: %v", err)
	}
	list, _ := repo.ListPending(ctx, wsID, 10)
	for _, e := range list[:2] {
		if err := repo.MarkParked(ctx, e.ID); err != nil {
			t.Fatalf("MarkParked failed: %v", err)
		}
	}

	n, err := repo.DeleteSupersededParked(ctx, wsID)
	if err != nil {
		t.Fatalf("DeleteSupersededParked failed: %v", err)
	}
	if n != 1 {
		t.Errorf("DeleteSupersededParked() = %d, want 1", n)
	}

	var rows int
	if err := db.QueryRow(`SELECT COUNT(*) FROM sync_queue WHERE entity_id = ?`, superseded).Scan(&rows); err != nil {
		t.Fatalf("count superseded failed: %v", err)
	}
	if rows != 1 {
		t.Errorf("rows for the superseded entity = %d, want 1 (the newer pending one)", rows)
	}

	tooLarge, err := repo.CountTooLarge(ctx, wsID)
	if err != nil {
		t.Fatalf("CountTooLarge failed: %v", err)
	}
	if tooLarge != 1 {
		t.Errorf("CountTooLarge() = %d, want 1 — the entity with no newer write stays parked", tooLarge)
	}
}

func TestSyncQueueRepo_CoalescedPending_ParkedDoesNotShadowNewer(t *testing.T) {
	db := setupTestDB(t)
	repo := NewSyncQueueRepo(db)
	ctx := context.Background()

	wsID := uuid.New().String()
	entityID := uuid.New().String()
	if err := repo.Enqueue(ctx, newTestSyncEntry(wsID, "request", entityID, "update")); err != nil {
		t.Fatalf("Enqueue failed: %v", err)
	}
	list, _ := repo.ListPending(ctx, wsID, 10)
	if err := repo.MarkParked(ctx, list[0].ID); err != nil {
		t.Fatalf("MarkParked failed: %v", err)
	}
	if err := repo.Enqueue(ctx, newTestSyncEntry(wsID, "request", entityID, "update")); err != nil {
		t.Fatalf("Enqueue newer failed: %v", err)
	}

	coalesced, err := repo.CoalescedPending(ctx, wsID, 10, nil)
	if err != nil {
		t.Fatalf("CoalescedPending failed: %v", err)
	}
	if len(coalesced) != 1 {
		t.Fatalf("CoalescedPending() = %d entries, want 1", len(coalesced))
	}
	if coalesced[0].ID == list[0].ID {
		t.Error("CoalescedPending() returned the parked entry instead of the newer pending one")
	}
}

func TestSyncQueueRepo_CountPendingOrFailed(t *testing.T) {
	db := setupTestDB(t)
	repo := NewSyncQueueRepo(db)
	ctx := context.Background()

	wsID := uuid.New().String()
	for i := 0; i < 3; i++ {
		if err := repo.Enqueue(ctx, newTestSyncEntry(wsID, "collection", uuid.New().String(), "upsert")); err != nil {
			t.Fatalf("Enqueue failed: %v", err)
		}
	}
	if err := repo.Enqueue(ctx, newTestSyncEntry(uuid.New().String(), "collection", uuid.New().String(), "upsert")); err != nil {
		t.Fatalf("Enqueue other workspace failed: %v", err)
	}

	list, _ := repo.ListPending(ctx, wsID, 10)
	if err := repo.MarkFailed(ctx, list[0].ID, time.Now().Add(5*time.Minute)); err != nil {
		t.Fatalf("MarkFailed failed: %v", err)
	}
	if err := repo.MarkSending(ctx, []int64{list[1].ID}); err != nil {
		t.Fatalf("MarkSending failed: %v", err)
	}

	count, err := repo.CountPendingOrFailed(ctx, wsID, nil)
	if err != nil {
		t.Fatalf("CountPendingOrFailed failed: %v", err)
	}
	if count != 2 {
		t.Errorf("CountPendingOrFailed() = %d, want 2", count)
	}
}

func TestSyncQueueRepo_EarliestParkedRetryAt(t *testing.T) {
	db := setupTestDB(t)
	repo := NewSyncQueueRepo(db)
	ctx := context.Background()

	wsID := uuid.New().String()
	if _, _, err := repo.EarliestParkedRetryAt(ctx, wsID); err != nil {
		t.Fatalf("EarliestParkedRetryAt on empty queue failed: %v", err)
	}
	if _, ok, _ := repo.EarliestParkedRetryAt(ctx, wsID); ok {
		t.Error("EarliestParkedRetryAt() reported a deadline with nothing parked")
	}

	for i := 0; i < 2; i++ {
		if err := repo.Enqueue(ctx, newTestSyncEntry(wsID, "collection", uuid.New().String(), "upsert")); err != nil {
			t.Fatalf("Enqueue failed: %v", err)
		}
	}

	list, _ := repo.ListPending(ctx, wsID, 10)
	soonest := time.Now().Add(2 * time.Minute).Truncate(time.Second)
	if err := repo.MarkFailed(ctx, list[0].ID, time.Now().Add(9*time.Minute)); err != nil {
		t.Fatalf("MarkFailed failed: %v", err)
	}
	if err := repo.MarkFailed(ctx, list[1].ID, soonest); err != nil {
		t.Fatalf("MarkFailed failed: %v", err)
	}

	got, ok, err := repo.EarliestParkedRetryAt(ctx, wsID)
	if err != nil {
		t.Fatalf("EarliestParkedRetryAt failed: %v", err)
	}
	if !ok {
		t.Fatal("EarliestParkedRetryAt() reported nothing parked")
	}
	if !got.Equal(soonest) {
		t.Errorf("EarliestParkedRetryAt() = %s, want %s", got, soonest)
	}

	if _, ok, _ := repo.EarliestParkedRetryAt(ctx, uuid.New().String()); ok {
		t.Error("EarliestParkedRetryAt() leaked another workspace's deadline")
	}
}

func TestSyncQueueRepo_EnqueueDocumentedRequests(t *testing.T) {
	db := setupTestDB(t)
	repo := NewSyncQueueRepo(db)
	ctx := context.Background()

	seed := []string{
		`INSERT INTO workspaces (id, name) VALUES ('ws1', 'Synced')`,
		`INSERT INTO workspaces (id, name) VALUES ('ws2', 'Other')`,
		`INSERT INTO collections (id, workspace_id, name) VALUES ('col1', 'ws1', 'Root')`,
		`INSERT INTO collections (id, workspace_id, name) VALUES ('col2', 'ws2', 'Root')`,
		`INSERT INTO requests (id, collection_id, name, description) VALUES ('req1', 'col1', 'Documented', '# Docs')`,
		`INSERT INTO requests (id, collection_id, name) VALUES ('req2', 'col1', 'Undocumented')`,
		`INSERT INTO requests (id, collection_id, name, description, is_draft) VALUES ('req3', 'col1', 'Draft', 'scratch', 1)`,
		`INSERT INTO requests (id, collection_id, name, description, is_delete) VALUES ('req4', 'col1', 'Deleted', 'gone', 1)`,
		fmt.Sprintf(`INSERT INTO requests (id, collection_id, name, description) VALUES ('req5', 'col1', 'Oversized', '%s')`,
			strings.Repeat("x", domain.MaxDescriptionLen+1)),
		`INSERT INTO requests (id, collection_id, name, description) VALUES ('req6', 'col2', 'Other workspace', 'docs')`,
	}
	for _, stmt := range seed {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatalf("seed %q: %v", stmt, err)
		}
	}

	if err := repo.Enqueue(ctx, newTestSyncEntry("ws1", "collection", "req1", "update")); err != nil {
		t.Fatalf("Enqueue decoy collection row failed: %v", err)
	}
	if err := repo.Enqueue(ctx, newTestSyncEntry("ws2", "request", "req1", "update")); err != nil {
		t.Fatalf("Enqueue decoy row of another workspace failed: %v", err)
	}

	n, err := repo.EnqueueDocumentedRequests(ctx, "ws1")
	if err != nil {
		t.Fatalf("EnqueueDocumentedRequests failed: %v", err)
	}
	if n != 1 {
		t.Errorf("EnqueueDocumentedRequests() = %d, want 1", n)
	}

	pending, err := repo.ListPending(ctx, "ws1", 10)
	if err != nil {
		t.Fatalf("ListPending failed: %v", err)
	}
	var offered []*SyncEntry
	for _, e := range pending {
		if e.EntityType == "request" {
			offered = append(offered, e)
		}
	}
	if len(offered) != 1 || offered[0].EntityID != "req1" {
		t.Fatalf("pending = %+v, want one request entry for req1", pending)
	}
	if offered[0].Action != "update" {
		t.Errorf("entry action = %q, want update", offered[0].Action)
	}
	if !uuidLike.MatchString(offered[0].OperationID) {
		t.Errorf("operation_id %q is not a v4 UUID", offered[0].OperationID)
	}
	if offered[0].CreatedAt.IsZero() {
		t.Error("created_at is zero")
	}

	other, err := repo.ListPending(ctx, "ws2", 10)
	if err != nil {
		t.Fatalf("ListPending ws2 failed: %v", err)
	}
	if len(other) != 1 {
		t.Errorf("ws2 = %d entries, want only its own decoy: the call is scoped to one workspace", len(other))
	}

	n, err = repo.EnqueueDocumentedRequests(ctx, "ws1")
	if err != nil {
		t.Fatalf("second EnqueueDocumentedRequests failed: %v", err)
	}
	if n != 0 {
		t.Errorf("second EnqueueDocumentedRequests() = %d, want 0", n)
	}

	pending, err = repo.ListPending(ctx, "ws1", 10)
	if err != nil {
		t.Fatalf("ListPending after the second call failed: %v", err)
	}
	if len(pending) != 2 {
		t.Errorf("pending after two calls = %d, want 2: repeated resyncs must not grow the outbox", len(pending))
	}
}

func parkedStatuses(t *testing.T, db *sql.DB, workspaceID, entityID string) []string {
	t.Helper()
	rows, err := db.Query(
		`SELECT status FROM sync_queue WHERE workspace_id = ? AND entity_id = ? ORDER BY id`,
		workspaceID, entityID)
	if err != nil {
		t.Fatalf("query statuses: %v", err)
	}
	defer func() { _ = rows.Close() }()

	var out []string
	for rows.Next() {
		var s string
		if err := rows.Scan(&s); err != nil {
			t.Fatalf("scan status: %v", err)
		}
		out = append(out, s)
	}
	return out
}

func TestSyncQueueRepo_Enqueue_DropsParkedRowOfTheSameEntity(t *testing.T) {
	db := setupTestDB(t)
	repo := NewSyncQueueRepo(db)
	ctx := context.Background()

	wsID := uuid.New().String()
	otherWsID := uuid.New().String()
	entityID := uuid.New().String()

	park := func(workspaceID, entityType, entityID string) {
		t.Helper()
		if err := repo.Enqueue(ctx, newTestSyncEntry(workspaceID, entityType, entityID, "update")); err != nil {
			t.Fatalf("Enqueue failed: %v", err)
		}
		list, _ := repo.ListPending(ctx, workspaceID, 10)
		if err := repo.MarkParked(ctx, list[len(list)-1].ID); err != nil {
			t.Fatalf("MarkParked failed: %v", err)
		}
	}

	park(wsID, "request", entityID)
	park(wsID, "collection", entityID)
	park(otherWsID, "request", entityID)

	if err := repo.Enqueue(ctx, newTestSyncEntry(wsID, "request", entityID, "update")); err != nil {
		t.Fatalf("Enqueue newer failed: %v", err)
	}

	got := parkedStatuses(t, db, wsID, entityID)
	want := []string{"parked", "pending"}
	if len(got) != len(want) {
		t.Fatalf("rows for the re-written entity = %v, want %v: the newer write replaces the parked row", got, want)
	}
	for i := range got {
		if got[i] != want[i] {
			t.Fatalf("rows for the re-written entity = %v, want %v", got, want)
		}
	}

	if other := parkedStatuses(t, db, otherWsID, entityID); len(other) != 1 || other[0] != "parked" {
		t.Errorf("rows of another workspace = %v, want one parked row", other)
	}
}

func TestSyncQueueRepo_DeleteParkedForMissingEntities(t *testing.T) {
	db := setupTestDB(t)
	repo := NewSyncQueueRepo(db)
	ctx := context.Background()

	seed := []string{
		`INSERT INTO workspaces (id, name) VALUES ('ws1', 'Synced')`,
		`INSERT INTO collections (id, workspace_id, name) VALUES ('col1', 'ws1', 'Live')`,
		`INSERT INTO collections (id, workspace_id, name, is_delete) VALUES ('col2', 'ws1', 'Deleted', 1)`,
		`INSERT INTO requests (id, collection_id, name) VALUES ('req-live', 'col1', 'Live')`,
		`INSERT INTO requests (id, collection_id, name, is_delete) VALUES ('req-deleted', 'col1', 'Deleted', 1)`,
		`INSERT INTO requests (id, collection_id, name, is_delete) VALUES ('req-leaving', 'col1', 'Delete on its way out', 1)`,
		`INSERT INTO requests (id, collection_id, name) VALUES ('req-orphan', 'col2', 'Under a deleted collection')`,
		`INSERT INTO environments (id, workspace_id, name) VALUES ('env1', 'ws1', 'Live')`,
		`INSERT INTO variables (id, environment_id, key) VALUES ('var1', 'env1', 'token')`,
	}
	for _, stmt := range seed {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatalf("seed %q: %v", stmt, err)
		}
	}

	parked := map[string]string{
		"request":     "req-live",
		"collection":  "col1",
		"environment": "env1",
		"variable":    "var1",
	}
	gone := map[string]string{
		"request":    "req-deleted",
		"collection": "col2",
	}
	for entityType, entityID := range parked {
		if err := repo.Enqueue(ctx, newTestSyncEntry("ws1", entityType, entityID, "update")); err != nil {
			t.Fatalf("Enqueue failed: %v", err)
		}
	}
	for entityType, entityID := range gone {
		if err := repo.Enqueue(ctx, newTestSyncEntry("ws1", entityType, entityID, "update")); err != nil {
			t.Fatalf("Enqueue failed: %v", err)
		}
	}
	for _, entityID := range []string{"req-orphan", "req-absent"} {
		if err := repo.Enqueue(ctx, newTestSyncEntry("ws1", "request", entityID, "update")); err != nil {
			t.Fatalf("Enqueue failed: %v", err)
		}
	}
	list, _ := repo.ListPending(ctx, "ws1", 20)
	for _, e := range list {
		if err := repo.MarkParked(ctx, e.ID); err != nil {
			t.Fatalf("MarkParked failed: %v", err)
		}
	}

	if err := repo.Enqueue(ctx, newTestSyncEntry("ws1", "request", "req-leaving", "delete")); err != nil {
		t.Fatalf("Enqueue delete failed: %v", err)
	}

	n, err := repo.DeleteParkedForMissingEntities(ctx, "ws1")
	if err != nil {
		t.Fatalf("DeleteParkedForMissingEntities failed: %v", err)
	}
	if n != 4 {
		t.Errorf("DeleteParkedForMissingEntities() = %d, want 4", n)
	}

	for entityType, entityID := range parked {
		if got := parkedStatuses(t, db, "ws1", entityID); len(got) != 1 {
			t.Errorf("%s %q rows = %v, want the parked row kept", entityType, entityID, got)
		}
	}
	for _, entityID := range []string{"col2", "req-deleted", "req-orphan", "req-absent"} {
		if got := parkedStatuses(t, db, "ws1", entityID); len(got) != 0 {
			t.Errorf("%q rows = %v, want none: the entity is gone locally", entityID, got)
		}
	}
	if got := parkedStatuses(t, db, "ws1", "req-leaving"); len(got) != 1 || got[0] != "pending" {
		t.Errorf("req-leaving rows = %v, want the pending delete kept", got)
	}
}

func TestSyncQueueRepo_CoalescedPending_ParentsFirst(t *testing.T) {
	db := setupTestDB(t)
	repo := NewSyncQueueRepo(db)
	ctx := context.Background()

	wsID := uuid.New().String()
	requestID := uuid.New().String()
	order := []struct{ entityType, entityID string }{
		{"response_example", uuid.New().String()},
		{"variable", uuid.New().String()},
		{"request", requestID},
		{"response_example", uuid.New().String()},
		{"environment", uuid.New().String()},
		{"collection", uuid.New().String()},
		{"request", requestID},
	}
	for _, o := range order {
		if err := repo.Enqueue(ctx, newTestSyncEntry(wsID, o.entityType, o.entityID, "update")); err != nil {
			t.Fatalf("Enqueue failed: %v", err)
		}
	}

	list, err := repo.CoalescedPending(ctx, wsID, 10, nil)
	if err != nil {
		t.Fatalf("CoalescedPending failed: %v", err)
	}
	var got []string
	for _, e := range list {
		got = append(got, e.EntityType)
	}
	want := []string{"collection", "environment", "request", "variable", "response_example", "response_example"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("order = %v, want %v", got, want)
	}
	if list[4].EntityID != order[0].entityID {
		t.Errorf("examples out of id order: first = %s, want %s", list[4].EntityID, order[0].entityID)
	}

	first, err := repo.CoalescedPending(ctx, wsID, 1, nil)
	if err != nil {
		t.Fatalf("CoalescedPending with limit failed: %v", err)
	}
	if len(first) != 1 || first[0].EntityType != "collection" {
		t.Errorf("limited page = %+v, want the collection: the limit cuts after ordering", first)
	}
}

func deferEntry(t *testing.T, repo SyncQueueRepository, wsID, entityType, entityID string, retryAt time.Time) int64 {
	t.Helper()
	ctx := context.Background()
	if err := repo.Enqueue(ctx, newTestSyncEntry(wsID, entityType, entityID, "create")); err != nil {
		t.Fatalf("Enqueue failed: %v", err)
	}
	list, err := repo.ListPending(ctx, wsID, 100)
	if err != nil || len(list) == 0 {
		t.Fatalf("ListPending: %v (%d rows)", err, len(list))
	}
	id := list[len(list)-1].ID
	if err := repo.MarkDeferred(ctx, id, retryAt, true); err != nil {
		t.Fatalf("MarkDeferred failed: %v", err)
	}
	return id
}

func TestSyncQueueRepo_MarkDeferred_WaitsApartFromQuotaAndTooLarge(t *testing.T) {
	db := setupTestDB(t)
	repo := NewSyncQueueRepo(db)
	ctx := context.Background()

	wsID := uuid.New().String()
	retryAt := time.Now().Add(5 * time.Minute).Truncate(time.Second)
	id := deferEntry(t, repo, wsID, "response_example", uuid.New().String(), retryAt)

	var status string
	var retries, defers int
	if err := db.QueryRow(`SELECT status, retry_count, defer_count FROM sync_queue WHERE id = ?`, id).Scan(&status, &retries, &defers); err != nil {
		t.Fatalf("read row: %v", err)
	}
	if status != "deferred" || retries != 0 || defers != 1 {
		t.Errorf("row = %s/%d/%d, want deferred with one wait for the parent and the quota budget untouched", status, retries, defers)
	}

	if n, _ := repo.CountParked(ctx, wsID); n != 0 {
		t.Errorf("CountParked() = %d, want 0: a missing parent is not a plan limit", n)
	}
	if n, _ := repo.CountTooLarge(ctx, wsID); n != 0 {
		t.Errorf("CountTooLarge() = %d, want 0", n)
	}
	if n, _ := repo.CountPendingOrFailed(ctx, wsID, nil); n != 1 {
		t.Errorf("CountPendingOrFailed() = %d, want 1: the example is still unsynced", n)
	}
	if pending, _ := repo.CoalescedPending(ctx, wsID, 10, nil); len(pending) != 0 {
		t.Errorf("CoalescedPending() = %d rows, want 0 until the retry is due", len(pending))
	}

	due, ok, err := repo.EarliestParkedRetryAt(ctx, wsID)
	if err != nil || !ok || !due.Equal(retryAt) {
		t.Errorf("EarliestParkedRetryAt() = %s, %v, %v; want %s", due, ok, err, retryAt)
	}

	if n, _ := repo.RequeueDue(ctx, wsID, time.Now()); n != 0 {
		t.Errorf("RequeueDue() before the deadline = %d, want 0", n)
	}
	if n, _ := repo.RequeueDue(ctx, wsID, retryAt.Add(time.Second)); n != 1 {
		t.Errorf("RequeueDue() after the deadline = %d, want 1", n)
	}
	if pending, _ := repo.CoalescedPending(ctx, wsID, 10, nil); len(pending) != 1 || pending[0].DeferCount != 1 {
		t.Errorf("CoalescedPending() after requeue = %+v, want the row back with its wait count", pending)
	}
}

func TestSyncQueueRepo_MarkDeferred_WithoutCountingKeepsTheBudget(t *testing.T) {
	db := setupTestDB(t)
	repo := NewSyncQueueRepo(db)
	ctx := context.Background()
	wsID := uuid.New().String()
	id := deferEntry(t, repo, wsID, "response_example", uuid.New().String(), time.Now().Add(-time.Second))
	if _, err := repo.RequeueDue(ctx, wsID, time.Now()); err != nil {
		t.Fatalf("RequeueDue: %v", err)
	}

	if err := repo.MarkDeferred(ctx, id, time.Now().Add(time.Hour), false); err != nil {
		t.Fatalf("MarkDeferred: %v", err)
	}

	held, err := repo.ListHeld(ctx, wsID, "response_example")
	if err != nil || len(held) != 1 {
		t.Fatalf("ListHeld = %+v, %v", held, err)
	}
	if held[0].Status != "deferred" || held[0].DeferCount != 1 {
		t.Errorf("row = %s/%d, want deferred with the earlier count", held[0].Status, held[0].DeferCount)
	}
}

func TestSyncQueueRepo_ExampleParentHeld(t *testing.T) {
	db := setupTestDB(t)
	repo := NewSyncQueueRepo(db)
	ctx := context.Background()
	coll := newTestCollection("Host", nil)
	req := newTestRequest("Parent", coll.ID)
	ex := newTestExample(req.ID, "Child", 0)
	if err := NewCollectionRepo(db).Create(ctx, coll); err != nil {
		t.Fatal(err)
	}
	if err := NewRequestRepo(db).Create(ctx, req); err != nil {
		t.Fatal(err)
	}
	if err := NewResponseExampleRepo(db).Create(ctx, ex); err != nil {
		t.Fatal(err)
	}
	wsID := testWorkspaceID.String()

	held := func() bool {
		t.Helper()
		got, err := repo.ExampleParentHeld(ctx, wsID, ex.ID.String())
		if err != nil {
			t.Fatalf("ExampleParentHeld: %v", err)
		}
		return got
	}

	if held() {
		t.Error("held with no queue row for the request")
	}
	if err := repo.Enqueue(ctx, newTestSyncEntry(wsID, "request", req.ID.String(), "update")); err != nil {
		t.Fatal(err)
	}
	if held() {
		t.Error("a pending request is on its way, not held")
	}
	var id int64
	if err := db.QueryRow(`SELECT id FROM sync_queue WHERE entity_id = ?`, req.ID.String()).Scan(&id); err != nil {
		t.Fatal(err)
	}
	for _, status := range []string{"failed", "parked", "deferred"} {
		if _, err := db.Exec(`UPDATE sync_queue SET status = ? WHERE id = ?`, status, id); err != nil {
			t.Fatal(err)
		}
		if !held() {
			t.Errorf("a %s request row holds the parent", status)
		}
	}
	if got, err := repo.ExampleParentHeld(ctx, uuid.NewString(), ex.ID.String()); err != nil || got {
		t.Errorf("another workspace's queue: %v, %v", got, err)
	}
}

func TestSyncQueueRepo_DeferredRowSupersededByNewerWrite(t *testing.T) {
	db := setupTestDB(t)
	repo := NewSyncQueueRepo(db)
	ctx := context.Background()

	wsID := uuid.New().String()
	rewritten := uuid.New().String()
	superseded := uuid.New().String()
	later := time.Now().Add(time.Hour)
	deferEntry(t, repo, wsID, "response_example", rewritten, later)
	supersededID := deferEntry(t, repo, wsID, "response_example", superseded, later)

	if err := repo.Enqueue(ctx, newTestSyncEntry(wsID, "response_example", rewritten, "update")); err != nil {
		t.Fatalf("Enqueue newer failed: %v", err)
	}
	if got := parkedStatuses(t, db, wsID, rewritten); len(got) != 1 || got[0] != "pending" {
		t.Errorf("rows of the rewritten example = %v, want only the newer pending row", got)
	}

	// A raw insert skips Enqueue's own cleanup, leaving the sweep to find the pair.
	if _, err := db.Exec(`INSERT INTO sync_queue (workspace_id, entity_type, entity_id, action, operation_id, status, retry_count, created_at)
		VALUES (?, 'response_example', ?, 'update', ?, 'pending', 0, ?)`,
		wsID, superseded, uuid.NewString(), time.Now().Format(time.RFC3339)); err != nil {
		t.Fatalf("insert newer row: %v", err)
	}
	n, err := repo.DeleteSupersededParked(ctx, wsID)
	if err != nil {
		t.Fatalf("DeleteSupersededParked failed: %v", err)
	}
	if n != 1 {
		t.Errorf("DeleteSupersededParked() = %d, want 1", n)
	}
	var left int
	if err := db.QueryRow(`SELECT COUNT(*) FROM sync_queue WHERE id = ?`, supersededID).Scan(&left); err != nil {
		t.Fatalf("count: %v", err)
	}
	if left != 0 {
		t.Error("the deferred row behind a newer pending one survived the sweep")
	}
}

func TestSyncQueueRepo_EnqueueRequestIfAbsent(t *testing.T) {
	db := setupTestDB(t)
	repo := NewSyncQueueRepo(db)
	ctx := context.Background()

	seed := []string{
		`INSERT INTO workspaces (id, name) VALUES ('ws1', 'Synced')`,
		`INSERT INTO workspaces (id, name) VALUES ('ws2', 'Other')`,
		`INSERT INTO collections (id, workspace_id, name) VALUES ('col1', 'ws1', 'Live')`,
		`INSERT INTO collections (id, workspace_id, name, is_delete) VALUES ('col-gone', 'ws1', 'Deleted', 1)`,
		`INSERT INTO collections (id, workspace_id, name) VALUES ('col2', 'ws2', 'Other')`,
		`INSERT INTO requests (id, collection_id, name) VALUES ('req-live', 'col1', 'Live')`,
		`INSERT INTO requests (id, collection_id, name) VALUES ('req-queued', 'col1', 'Already queued')`,
		`INSERT INTO requests (id, collection_id, name, is_delete) VALUES ('req-deleted', 'col1', 'Deleted', 1)`,
		`INSERT INTO requests (id, collection_id, name) VALUES ('req-orphan', 'col-gone', 'Under a deleted collection')`,
		`INSERT INTO requests (id, collection_id, name) VALUES ('req-other', 'col2', 'Other workspace')`,
	}
	for _, stmt := range seed {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatalf("seed %q: %v", stmt, err)
		}
	}
	deferEntry(t, repo, "ws1", "request", "req-queued", time.Now().Add(time.Hour))

	cases := []struct {
		requestID string
		want      bool
	}{
		{"req-live", true},
		{"req-live", false},
		{"req-queued", false},
		{"req-deleted", false},
		{"req-orphan", false},
		{"req-other", false},
		{"req-absent", false},
	}
	for _, tc := range cases {
		got, err := repo.EnqueueRequestIfAbsent(ctx, "ws1", tc.requestID)
		if err != nil {
			t.Fatalf("EnqueueRequestIfAbsent(%s) failed: %v", tc.requestID, err)
		}
		if got != tc.want {
			t.Errorf("EnqueueRequestIfAbsent(%s) = %v, want %v", tc.requestID, got, tc.want)
		}
	}

	pending, err := repo.ListPending(ctx, "ws1", 10)
	if err != nil {
		t.Fatalf("ListPending failed: %v", err)
	}
	if len(pending) != 1 || pending[0].EntityID != "req-live" || pending[0].EntityType != "request" || pending[0].Action != "update" {
		t.Fatalf("pending = %+v, want one request update for req-live", pending)
	}
	if !uuidLike.MatchString(pending[0].OperationID) {
		t.Errorf("operation_id %q is not a v4 UUID", pending[0].OperationID)
	}
}

func TestSyncQueueRepo_ListHeld(t *testing.T) {
	db := setupTestDB(t)
	repo := NewSyncQueueRepo(db)
	ctx := context.Background()

	wsID := uuid.New().String()
	deferred := uuid.New().String()
	tooLarge := uuid.New().String()
	deferEntry(t, repo, wsID, "response_example", deferred, time.Now().Add(time.Hour))
	if err := repo.Enqueue(ctx, newTestSyncEntry(wsID, "response_example", tooLarge, "create")); err != nil {
		t.Fatalf("Enqueue failed: %v", err)
	}
	list, _ := repo.ListPending(ctx, wsID, 10)
	if err := repo.MarkParked(ctx, list[0].ID); err != nil {
		t.Fatalf("MarkParked failed: %v", err)
	}
	for _, noise := range []SyncEntry{
		newTestSyncEntry(wsID, "response_example", uuid.New().String(), "create"),
		newTestSyncEntry(uuid.New().String(), "response_example", uuid.New().String(), "create"),
	} {
		if err := repo.Enqueue(ctx, noise); err != nil {
			t.Fatalf("Enqueue failed: %v", err)
		}
	}
	quota := parkQuota(t, repo, wsID, "response_example")
	parkedRequest := uuid.New().String()
	if err := repo.Enqueue(ctx, newTestSyncEntry(wsID, "request", parkedRequest, "update")); err != nil {
		t.Fatalf("Enqueue failed: %v", err)
	}
	list, _ = repo.ListPending(ctx, wsID, 10)
	if err := repo.MarkParked(ctx, list[len(list)-1].ID); err != nil {
		t.Fatalf("MarkParked failed: %v", err)
	}

	held, err := repo.ListHeld(ctx, wsID, "response_example")
	if err != nil {
		t.Fatalf("ListHeld failed: %v", err)
	}
	got := map[string]string{}
	for _, e := range held {
		got[e.EntityID] = e.Status
	}
	want := map[string]string{deferred: "deferred", tooLarge: "parked"}
	if len(got) != len(want) || got[deferred] != want[deferred] || got[tooLarge] != want[tooLarge] {
		t.Errorf("ListHeld() = %v, want %v (quota row %s and the request are not held examples)", got, want, quota)
	}
}

func parkQuota(t *testing.T, repo SyncQueueRepository, wsID, entityType string) string {
	t.Helper()
	ctx := context.Background()
	entityID := uuid.New().String()
	if err := repo.Enqueue(ctx, newTestSyncEntry(wsID, entityType, entityID, "create")); err != nil {
		t.Fatalf("Enqueue failed: %v", err)
	}
	list, _ := repo.ListPending(ctx, wsID, 100)
	if err := repo.MarkFailed(ctx, list[len(list)-1].ID, time.Now().Add(time.Hour)); err != nil {
		t.Fatalf("MarkFailed failed: %v", err)
	}
	return entityID
}

func queueStatusesByID(t *testing.T, db *sql.DB) map[int64]string {
	t.Helper()
	rows, err := db.Query(`SELECT id, status FROM sync_queue`)
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	defer func() { _ = rows.Close() }()
	out := map[int64]string{}
	for rows.Next() {
		var id int64
		var s string
		if err := rows.Scan(&id, &s); err != nil {
			t.Fatalf("scan: %v", err)
		}
		out[id] = s
	}
	return out
}

func TestSyncQueueRepo_DropOlderPending(t *testing.T) {
	db := setupTestDB(t)
	repo := NewSyncQueueRepo(db)
	ctx := context.Background()

	wsID := uuid.New().String()
	otherWS := uuid.New().String()
	entityID := uuid.New().String()
	neighbour := uuid.New().String()
	enqueue := func(ws, entityType, id string) int64 {
		t.Helper()
		if err := repo.Enqueue(ctx, newTestSyncEntry(ws, entityType, id, "update")); err != nil {
			t.Fatalf("Enqueue failed: %v", err)
		}
		var last int64
		if err := db.QueryRow(`SELECT MAX(id) FROM sync_queue`).Scan(&last); err != nil {
			t.Fatalf("max id: %v", err)
		}
		return last
	}

	older1 := enqueue(wsID, "request", entityID)
	older2 := enqueue(wsID, "request", entityID)
	failedOlder := enqueue(wsID, "request", entityID)
	if err := repo.MarkFailed(ctx, failedOlder, time.Now().Add(time.Hour)); err != nil {
		t.Fatalf("MarkFailed failed: %v", err)
	}
	sameIDOtherType := enqueue(wsID, "collection", entityID)
	sameIDOtherWS := enqueue(otherWS, "request", entityID)
	neighbourRow := enqueue(wsID, "request", neighbour)
	chosen := enqueue(wsID, "request", entityID)

	entries, err := repo.CoalescedPending(ctx, wsID, 10, nil)
	if err != nil {
		t.Fatalf("CoalescedPending failed: %v", err)
	}
	var sent *SyncEntry
	for _, e := range entries {
		if e.EntityType == "request" && e.EntityID == entityID {
			sent = e
		}
	}
	if sent == nil || sent.ID != chosen {
		t.Fatalf("coalesced page = %+v, want row %d for the entity", entries, chosen)
	}
	newer := enqueue(wsID, "request", entityID)
	if err := repo.Delete(ctx, []int64{chosen}); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	if err := repo.DropOlderPending(ctx, []*SyncEntry{sent}); err != nil {
		t.Fatalf("DropOlderPending failed: %v", err)
	}

	got := queueStatusesByID(t, db)
	for _, id := range []int64{older1, older2} {
		if _, ok := got[id]; ok {
			t.Errorf("older pending row %d survived", id)
		}
	}
	for _, id := range []int64{failedOlder, sameIDOtherType, sameIDOtherWS, neighbourRow, newer} {
		if _, ok := got[id]; !ok {
			t.Errorf("row %d was dropped but is not an older pending row of the sent entity", id)
		}
	}
	if err := repo.DropOlderPending(ctx, nil); err != nil {
		t.Errorf("DropOlderPending(nil) = %v", err)
	}
}

func TestSyncQueueRepo_DeleteJoinsTheCallersTransaction(t *testing.T) {
	db := setupTestDB(t)
	repo := NewSyncQueueRepo(db)
	ctx := context.Background()

	wsID := uuid.New().String()
	if err := repo.Enqueue(ctx, newTestSyncEntry(wsID, "request", uuid.New().String(), "update")); err != nil {
		t.Fatalf("Enqueue failed: %v", err)
	}
	list, _ := repo.ListPending(ctx, wsID, 10)

	rollback := fmt.Errorf("rollback")
	done := make(chan error, 1)
	go func() {
		done <- WithTx(ctx, db, func(txCtx context.Context) error {
			if err := repo.Delete(txCtx, []int64{list[0].ID}); err != nil {
				return err
			}
			held, err := repo.ListHeld(txCtx, wsID, "request")
			if err != nil {
				return err
			}
			if err := repo.DropOlderPending(txCtx, held); err != nil {
				return err
			}
			return rollback
		})
	}()
	select {
	case err := <-done:
		if err != rollback {
			t.Fatalf("WithTx = %v, want the rollback marker", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("a queue method waited for the connection the transaction holds")
	}
	if n, _ := repo.CountPendingOrFailed(ctx, wsID, nil); n != 1 {
		t.Errorf("rows after rollback = %d, want 1", n)
	}
}

func TestSyncQueueRepo_CoalescedPending_ExcludesTypes(t *testing.T) {
	db := setupTestDB(t)
	repo := NewSyncQueueRepo(db)
	ctx := context.Background()

	wsID := uuid.New().String()
	for _, entityType := range []string{"response_example", "request", "response_example", "variable"} {
		if err := repo.Enqueue(ctx, newTestSyncEntry(wsID, entityType, uuid.New().String(), "update")); err != nil {
			t.Fatalf("Enqueue failed: %v", err)
		}
	}

	list, err := repo.CoalescedPending(ctx, wsID, 10, []string{"response_example"})
	if err != nil {
		t.Fatalf("CoalescedPending failed: %v", err)
	}
	var got []string
	for _, e := range list {
		got = append(got, e.EntityType)
	}
	if strings.Join(got, ",") != "request,variable" {
		t.Errorf("types = %v, want request,variable", got)
	}

	limited, err := repo.CoalescedPending(ctx, wsID, 1, []string{"request", "variable"})
	if err != nil {
		t.Fatalf("CoalescedPending with two exclusions failed: %v", err)
	}
	if len(limited) != 1 || limited[0].EntityType != "response_example" {
		t.Errorf("limited page = %+v, want one example", limited)
	}
}

func TestSyncQueueRepo_CountPendingOrFailed_ExcludesTypes(t *testing.T) {
	db := setupTestDB(t)
	repo := NewSyncQueueRepo(db)
	ctx := context.Background()

	wsID := uuid.New().String()
	for _, entityType := range []string{"response_example", "request", "response_example"} {
		if err := repo.Enqueue(ctx, newTestSyncEntry(wsID, entityType, uuid.New().String(), "update")); err != nil {
			t.Fatalf("Enqueue failed: %v", err)
		}
	}
	deferEntry(t, repo, wsID, "response_example", uuid.New().String(), time.Now().Add(time.Hour))

	if n, err := repo.CountPendingOrFailed(ctx, wsID, []string{"response_example"}); err != nil || n != 1 {
		t.Errorf("CountPendingOrFailed(excluding examples) = %d, %v; want 1", n, err)
	}
	if n, err := repo.CountPendingOrFailed(ctx, wsID, nil); err != nil || n != 4 {
		t.Errorf("CountPendingOrFailed(all) = %d, %v; want 4", n, err)
	}
}

func TestSyncQueueRepo_EnqueueUnsyncedExamples(t *testing.T) {
	db := setupTestDB(t)
	repo := NewSyncQueueRepo(db)
	ctx := context.Background()

	seed := []string{
		`INSERT INTO workspaces (id, name) VALUES ('ws1', 'Synced')`,
		`INSERT INTO workspaces (id, name) VALUES ('ws2', 'Other')`,
		`INSERT INTO response_examples (id, request_id, workspace_id, name, is_delete, is_synced, created_at, updated_at)
		 VALUES ('ex-new', 'req1', 'ws1', 'Created here', 0, 0, '2026-09-25T10:00:00Z', '2026-09-25T10:00:00Z')`,
		`INSERT INTO response_examples (id, request_id, workspace_id, name, is_delete, is_synced, created_at, updated_at)
		 VALUES ('ex-deleted', 'req1', 'ws1', 'Deleted here', 1, 0, '2026-09-25T10:00:00Z', '2026-09-25T10:00:00Z')`,
		`INSERT INTO response_examples (id, request_id, workspace_id, name, is_delete, is_synced, created_at, updated_at)
		 VALUES ('ex-pulled', 'req1', 'ws1', 'From the server', 0, 1, '2026-09-25T10:00:00Z', '2026-09-25T10:00:00Z')`,
		`INSERT INTO response_examples (id, request_id, workspace_id, name, is_delete, is_synced, created_at, updated_at)
		 VALUES ('ex-queued', 'req1', 'ws1', 'Already queued', 0, 0, '2026-09-25T10:00:00Z', '2026-09-25T10:00:00Z')`,
		`INSERT INTO response_examples (id, request_id, workspace_id, name, is_delete, is_synced, created_at, updated_at)
		 VALUES ('ex-other', 'req2', 'ws2', 'Other workspace', 0, 0, '2026-09-25T10:00:00Z', '2026-09-25T10:00:00Z')`,
	}
	for _, stmt := range seed {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatalf("seed %q: %v", stmt, err)
		}
	}
	if err := repo.Enqueue(ctx, newTestSyncEntry("ws1", "response_example", "ex-queued", "update")); err != nil {
		t.Fatalf("Enqueue failed: %v", err)
	}

	n, err := repo.EnqueueUnsyncedExamples(ctx, "ws1")
	if err != nil {
		t.Fatalf("EnqueueUnsyncedExamples failed: %v", err)
	}
	if n != 2 {
		t.Errorf("EnqueueUnsyncedExamples() = %d, want 2", n)
	}

	pending, err := repo.ListPending(ctx, "ws1", 10)
	if err != nil {
		t.Fatalf("ListPending failed: %v", err)
	}
	actions := map[string][]string{}
	for _, e := range pending {
		if e.EntityType != "response_example" {
			t.Errorf("entity_type = %q, want response_example", e.EntityType)
		}
		if !uuidLike.MatchString(e.OperationID) {
			t.Errorf("operation_id %q is not a v4 UUID", e.OperationID)
		}
		if e.CreatedAt.IsZero() {
			t.Errorf("%s: created_at is zero", e.EntityID)
		}
		actions[e.EntityID] = append(actions[e.EntityID], e.Action)
	}
	want := map[string][]string{
		"ex-new":     {"create"},
		"ex-deleted": {"delete"},
		"ex-queued":  {"update"},
	}
	if fmt.Sprint(actions) != fmt.Sprint(want) {
		t.Errorf("pending actions = %v, want %v", actions, want)
	}

	if other, _ := repo.ListPending(ctx, "ws2", 10); len(other) != 0 {
		t.Errorf("ws2 = %d entries, want none: the call is scoped to one workspace", len(other))
	}

	n, err = repo.EnqueueUnsyncedExamples(ctx, "ws1")
	if err != nil {
		t.Fatalf("second EnqueueUnsyncedExamples failed: %v", err)
	}
	if n != 0 {
		t.Errorf("second EnqueueUnsyncedExamples() = %d, want 0", n)
	}
}

func TestSyncQueueRepo_EnqueueUnsyncedExamplesJoinsTheCallersTransaction(t *testing.T) {
	db := setupTestDB(t)
	repo := NewSyncQueueRepo(db)
	ctx := context.Background()

	if _, err := db.Exec(`INSERT INTO response_examples (id, request_id, workspace_id, name, created_at, updated_at)
		VALUES ('ex1', 'req1', 'ws1', 'Local', '2026-09-25T10:00:00Z', '2026-09-25T10:00:00Z')`); err != nil {
		t.Fatalf("seed: %v", err)
	}

	rollback := fmt.Errorf("rollback")
	done := make(chan error, 1)
	go func() {
		done <- WithTx(ctx, db, func(txCtx context.Context) error {
			if _, err := repo.EnqueueUnsyncedExamples(txCtx, "ws1"); err != nil {
				return err
			}
			return rollback
		})
	}()
	select {
	case err := <-done:
		if err != rollback {
			t.Fatalf("WithTx = %v, want the rollback marker", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("EnqueueUnsyncedExamples waited for the connection the transaction holds")
	}
	if n, _ := repo.CountPendingOrFailed(ctx, "ws1", nil); n != 0 {
		t.Errorf("rows after rollback = %d, want 0", n)
	}
}

func seedSQL(t *testing.T, db *sql.DB, stmts ...string) {
	t.Helper()
	for _, stmt := range stmts {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatalf("seed %q: %v", stmt, err)
		}
	}
}

func queueRows(t *testing.T, db *sql.DB, workspaceID string) []string {
	t.Helper()
	rows, err := db.Query(`SELECT entity_type, entity_id, action, status FROM sync_queue WHERE workspace_id = ? ORDER BY id`, workspaceID)
	if err != nil {
		t.Fatalf("query queue: %v", err)
	}
	defer func() { _ = rows.Close() }()
	var out []string
	for rows.Next() {
		var entityType, entityID, action, status string
		if err := rows.Scan(&entityType, &entityID, &action, &status); err != nil {
			t.Fatalf("scan queue: %v", err)
		}
		out = append(out, entityType+":"+entityID+":"+action+":"+status)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("queue rows: %v", err)
	}
	return out
}

// The deepest folder gets the first rowid: only a depth sort puts parents first.
func seedDeepTree(t *testing.T, db *sql.DB) {
	t.Helper()
	seedSQL(t, db,
		`INSERT INTO workspaces (id, name) VALUES ('ws1', 'Local')`,
		`INSERT INTO workspaces (id, name) VALUES ('ws2', 'Other')`,
		`INSERT INTO collections (id, workspace_id, name) VALUES ('col-a', 'ws1', 'Deepest')`,
		`INSERT INTO collections (id, workspace_id, name) VALUES ('col-b', 'ws1', 'Root')`,
		`INSERT INTO collections (id, workspace_id, name, parent_id) VALUES ('col-c', 'ws1', 'Middle', 'col-b')`,
		`UPDATE collections SET parent_id = 'col-c' WHERE id = 'col-a'`,
		`INSERT INTO collections (id, workspace_id, name, is_delete) VALUES ('col-gone', 'ws1', 'Deleted', 1)`,
		`INSERT INTO collections (id, workspace_id, name) VALUES ('col-other', 'ws2', 'Other root')`,
		`INSERT INTO requests (id, collection_id, name) VALUES ('req-a', 'col-a', 'In the deepest folder')`,
		`INSERT INTO requests (id, collection_id, name, is_draft) VALUES ('req-draft', 'col-a', 'Draft', 1)`,
		`INSERT INTO requests (id, collection_id, name, is_delete) VALUES ('req-deleted', 'col-a', 'Deleted', 1)`,
		`INSERT INTO requests (id, collection_id, name) VALUES ('req-orphan', 'col-gone', 'In a deleted folder')`,
		`INSERT INTO requests (id, collection_id, name) VALUES ('req-other', 'col-other', 'Other workspace')`,
		`INSERT INTO response_examples (id, request_id, workspace_id, name, created_at, updated_at)
		 VALUES ('ex-a', 'req-a', 'ws1', 'Live', '2026-09-26T10:00:00Z', '2026-09-26T10:00:00Z')`,
		`INSERT INTO response_examples (id, request_id, workspace_id, name, is_delete, created_at, updated_at)
		 VALUES ('ex-deleted', 'req-a', 'ws1', 'Deleted', 1, '2026-09-26T10:00:00Z', '2026-09-26T10:00:00Z')`,
		`INSERT INTO response_examples (id, request_id, workspace_id, name, created_at, updated_at)
		 VALUES ('ex-draft', 'req-draft', 'ws1', 'Of a draft', '2026-09-26T10:00:00Z', '2026-09-26T10:00:00Z')`,
		`INSERT INTO response_examples (id, request_id, workspace_id, name, created_at, updated_at)
		 VALUES ('ex-orphan', 'req-deleted', 'ws1', 'Of a deleted request', '2026-09-26T10:00:00Z', '2026-09-26T10:00:00Z')`,
	)
}

func TestSyncQueueRepo_EnqueueWorkspace(t *testing.T) {
	db := setupTestDB(t)
	repo := NewSyncQueueRepo(db)
	ctx := context.Background()
	seedDeepTree(t, db)
	seedSQL(t, db,
		`INSERT INTO environments (id, workspace_id, name) VALUES ('env-1', 'ws1', 'Dev')`,
		`INSERT INTO environments (id, workspace_id, name, is_delete) VALUES ('env-gone', 'ws1', 'Deleted', 1)`,
		`INSERT INTO variables (id, environment_id, key) VALUES ('var-1', 'env-1', 'host')`,
		`INSERT INTO variables (id, environment_id, key, is_delete) VALUES ('var-deleted', 'env-1', 'old', 1)`,
		`INSERT INTO variables (id, environment_id, key) VALUES ('var-orphan', 'env-gone', 'token')`,
		`INSERT INTO collections (id, workspace_id, name, is_synced) VALUES ('col-synced', 'ws1', 'Synced to a previous remote', 1)`,
	)
	if err := repo.Enqueue(ctx, newTestSyncEntry("ws1", "request", "req-a", "update")); err != nil {
		t.Fatalf("Enqueue pending row failed: %v", err)
	}

	n, err := repo.EnqueueWorkspace(ctx, "ws1")
	if err != nil {
		t.Fatalf("EnqueueWorkspace failed: %v", err)
	}
	want := []string{
		"request:req-a:update:pending",
		"collection:col-b:create:pending",
		"collection:col-synced:create:pending",
		"collection:col-c:create:pending",
		"collection:col-a:create:pending",
		"environment:env-1:create:pending",
		"variable:var-1:create:pending",
		"response_example:ex-a:create:pending",
	}
	if got := queueRows(t, db, "ws1"); fmt.Sprint(got) != fmt.Sprint(want) {
		t.Errorf("queue = %v\nwant %v", got, want)
	}
	if n != len(want)-1 {
		t.Errorf("EnqueueWorkspace() = %d, want %d", n, len(want)-1)
	}
	if other := queueRows(t, db, "ws2"); len(other) != 0 {
		t.Errorf("ws2 queue = %v, want none", other)
	}

	if n, err := repo.EnqueueWorkspace(ctx, "ws1"); err != nil || n != 0 {
		t.Errorf("second EnqueueWorkspace() = %d, %v; want 0, nil", n, err)
	}
}

func TestSyncQueueRepo_EnqueueUnsyncedTree(t *testing.T) {
	db := setupTestDB(t)
	repo := NewSyncQueueRepo(db)
	ctx := context.Background()
	seedDeepTree(t, db)
	seedSQL(t, db,
		`UPDATE collections SET is_synced = 1 WHERE id = 'col-c'`,
		`INSERT INTO requests (id, collection_id, name, is_synced) VALUES ('req-synced', 'col-b', 'Confirmed', 1)`,
		`INSERT INTO requests (id, collection_id, name) VALUES ('req-parked', 'col-c', 'Held by the plan')`,
		`INSERT INTO collections (id, workspace_id, name) VALUES ('col-sibling', 'ws1', 'Another root')`,
		`INSERT INTO requests (id, collection_id, name) VALUES ('req-sibling', 'col-sibling', 'Outside the tree')`,
		`INSERT INTO environments (id, workspace_id, name) VALUES ('env-1', 'ws1', 'Dev')`,
	)
	if err := repo.Enqueue(ctx, newTestSyncEntry("ws1", "request", "req-parked", "create")); err != nil {
		t.Fatalf("Enqueue failed: %v", err)
	}
	pending, err := repo.ListPending(ctx, "ws1", 10)
	if err != nil || len(pending) != 1 {
		t.Fatalf("ListPending = %v, %v", pending, err)
	}
	if err := repo.MarkParked(ctx, pending[0].ID); err != nil {
		t.Fatalf("MarkParked failed: %v", err)
	}

	n, err := repo.EnqueueUnsyncedTree(ctx, "ws1", "col-b")
	if err != nil {
		t.Fatalf("EnqueueUnsyncedTree failed: %v", err)
	}
	want := []string{
		"request:req-parked:create:parked",
		"collection:col-b:create:pending",
		"collection:col-a:create:pending",
		"request:req-a:create:pending",
		"response_example:ex-a:create:pending",
	}
	if got := queueRows(t, db, "ws1"); fmt.Sprint(got) != fmt.Sprint(want) {
		t.Errorf("queue = %v\nwant %v", got, want)
	}
	if n != len(want)-1 {
		t.Errorf("EnqueueUnsyncedTree() = %d, want %d", n, len(want)-1)
	}

	if n, err := repo.EnqueueUnsyncedTree(ctx, "ws1", "col-b"); err != nil || n != 0 {
		t.Errorf("second EnqueueUnsyncedTree() = %d, %v; want 0, nil", n, err)
	}
	if n, err := repo.EnqueueUnsyncedTree(ctx, "ws2", "col-b"); err != nil || n != 0 {
		t.Errorf("EnqueueUnsyncedTree in another workspace = %d, %v; want 0, nil", n, err)
	}
}

func TestSyncQueueRepo_CoalescedPending_FoldersInDepthOrderAfterAParentEdit(t *testing.T) {
	db := setupTestDB(t)
	repo := NewSyncQueueRepo(db)
	ctx := context.Background()
	seedDeepTree(t, db)
	if _, err := repo.EnqueueWorkspace(ctx, "ws1"); err != nil {
		t.Fatalf("EnqueueWorkspace failed: %v", err)
	}
	if err := repo.Enqueue(ctx, newTestSyncEntry("ws1", "collection", "col-b", "update")); err != nil {
		t.Fatalf("Enqueue root rename failed: %v", err)
	}

	list, err := repo.CoalescedPending(ctx, "ws1", 1, nil)
	if err != nil {
		t.Fatalf("CoalescedPending failed: %v", err)
	}
	if len(list) != 1 || list[0].EntityID != "col-b" || list[0].Action != "update" {
		t.Fatalf("first batch = %v, want the latest row of the root col-b", list)
	}

	list, err = repo.CoalescedPending(ctx, "ws1", 10, nil)
	if err != nil {
		t.Fatalf("CoalescedPending failed: %v", err)
	}
	var got []string
	for _, e := range list {
		if e.EntityType == "collection" {
			got = append(got, e.EntityID)
		}
	}
	if want := []string{"col-b", "col-c", "col-a"}; fmt.Sprint(got) != fmt.Sprint(want) {
		t.Errorf("collections = %v, want %v", got, want)
	}
}

func TestSyncQueueRepo_EnqueueUnsyncedWorkspace(t *testing.T) {
	db := setupTestDB(t)
	repo := NewSyncQueueRepo(db)
	ctx := context.Background()
	seedDeepTree(t, db)
	seedSQL(t, db,
		`UPDATE collections SET is_synced = 1 WHERE id = 'col-c'`,
		`INSERT INTO environments (id, workspace_id, name) VALUES ('env-1', 'ws1', 'Dev')`,
		`INSERT INTO environments (id, workspace_id, name, is_synced) VALUES ('env-synced', 'ws1', 'Prod', 1)`,
		`INSERT INTO variables (id, environment_id, key) VALUES ('var-1', 'env-synced', 'host')`,
		`INSERT INTO variables (id, environment_id, key, is_synced) VALUES ('var-synced', 'env-synced', 'port', 1)`,
	)
	if err := repo.Enqueue(ctx, newTestSyncEntry("ws1", "request", "req-a", "update")); err != nil {
		t.Fatalf("Enqueue failed: %v", err)
	}

	n, err := repo.EnqueueUnsyncedWorkspace(ctx, "ws1")
	if err != nil {
		t.Fatalf("EnqueueUnsyncedWorkspace failed: %v", err)
	}
	want := []string{
		"request:req-a:update:pending",
		"collection:col-b:create:pending",
		"collection:col-a:create:pending",
		"environment:env-1:create:pending",
		"variable:var-1:create:pending",
		"response_example:ex-a:create:pending",
	}
	if got := queueRows(t, db, "ws1"); fmt.Sprint(got) != fmt.Sprint(want) {
		t.Errorf("queue = %v\nwant %v", got, want)
	}
	if n != len(want)-1 {
		t.Errorf("EnqueueUnsyncedWorkspace() = %d, want %d", n, len(want)-1)
	}
	if other := queueRows(t, db, "ws2"); len(other) != 0 {
		t.Errorf("ws2 queue = %v, want none", other)
	}
}

func TestSyncQueueRepo_UploadsLeaveTheSeededDefaultEnvironmentOut(t *testing.T) {
	ctx, ws := context.Background(), testWorkspaceID.String()
	withEnvironments := []string{
		"collection:col-root:create:pending", "environment:env-own:create:pending", "variable:var-own:create:pending",
	}
	for name, c := range map[string]struct {
		enqueue func(r SyncQueueRepository) (int, error)
		want    []string
	}{
		"first link": {
			enqueue: func(r SyncQueueRepository) (int, error) { return r.EnqueueWorkspace(ctx, ws) },
			want:    withEnvironments,
		},
		"writes mid-link": {
			enqueue: func(r SyncQueueRepository) (int, error) { return r.EnqueueUnsyncedWorkspace(ctx, ws) },
			want:    withEnvironments,
		},
		"publication not synced": {
			enqueue: func(r SyncQueueRepository) (int, error) { return r.EnqueueUnsyncedTree(ctx, ws, "col-root") },
			want:    []string{"collection:col-root:create:pending"},
		},
	} {
		t.Run(name, func(t *testing.T) {
			db := setupTestDB(t)
			seedSQL(t, db,
				`INSERT INTO variables (id, environment_id, key) VALUES ('var-seeded', '`+seededEnvironmentID+`', 'host')`,
				`INSERT INTO environments (id, workspace_id, name) VALUES ('env-own', '`+ws+`', 'Staging')`,
				`INSERT INTO variables (id, environment_id, key) VALUES ('var-own', 'env-own', 'host')`,
				`INSERT INTO collections (id, workspace_id, name) VALUES ('col-root', '`+ws+`', 'Dogs API')`,
			)

			if _, err := c.enqueue(NewSyncQueueRepo(db)); err != nil {
				t.Fatalf("enqueue: %v", err)
			}
			if got := queueRows(t, db, ws); fmt.Sprint(got) != fmt.Sprint(c.want) {
				t.Errorf("queue = %v\nwant %v", got, c.want)
			}
		})
	}
}

func TestSyncQueueRepo_ReissueSeededEnvironment(t *testing.T) {
	ctx, ws := context.Background(), testWorkspaceID.String()
	seed := func(t *testing.T, db *sql.DB) {
		seedSQL(t, db,
			`INSERT INTO variables (id, environment_id, key) VALUES ('var-live', '`+seededEnvironmentID+`', 'host')`,
			`INSERT INTO variables (id, environment_id, key, is_delete) VALUES ('var-gone', '`+seededEnvironmentID+`', 'old', 1)`,
			`INSERT INTO publications (collection_id, workspace_id, owner_key, publication_id, settings, created_at, updated_at)
			 VALUES ('col-pub', '`+ws+`', 'acc', 'pub-1', '{"environmentId":"`+seededEnvironmentID+`","includeScripts":false}', '', '')`,
		)
	}

	t.Run("a copy the server never saw moves to an id of its own", func(t *testing.T) {
		db := setupTestDB(t)
		seed(t, db)

		newID, err := NewSyncQueueRepo(db).ReissueSeededEnvironment(ctx, ws)
		if err != nil {
			t.Fatalf("ReissueSeededEnvironment: %v", err)
		}
		parsed, err := uuid.Parse(newID)
		if err != nil || parsed.Version() != 4 || newID == seededEnvironmentID {
			t.Fatalf("new id = %q (%v), want a fresh v4 UUID", newID, err)
		}

		var name, workspace string
		var active int
		if err := db.QueryRow(`SELECT name, workspace_id, is_active FROM environments WHERE id = ?`, newID).Scan(&name, &workspace, &active); err != nil {
			t.Fatalf("moved environment: %v", err)
		}
		if name != "Default" || workspace != ws || active != 1 {
			t.Errorf("moved environment = (%q, %q, active %d), want (Default, %q, 1)", name, workspace, active, ws)
		}
		var left int
		if err := db.QueryRow(`SELECT COUNT(*) FROM environments WHERE id = ?`, seededEnvironmentID).Scan(&left); err != nil || left != 0 {
			t.Errorf("rows under the seeded id = %d (%v), want 0", left, err)
		}
		var moved int
		if err := db.QueryRow(`SELECT COUNT(*) FROM variables WHERE environment_id = ? AND id IN ('var-live', 'var-gone')`, newID).Scan(&moved); err != nil || moved != 2 {
			t.Errorf("variables under the new id = %d (%v), want 2", moved, err)
		}
		var settingsEnv string
		if err := db.QueryRow(`SELECT json_extract(settings, '$.environmentId') FROM publications WHERE collection_id = 'col-pub'`).Scan(&settingsEnv); err != nil || settingsEnv != newID {
			t.Errorf("publication settings environment = %q (%v), want %q", settingsEnv, err, newID)
		}
		var fkViolations int
		if err := db.QueryRow(`SELECT COUNT(*) FROM pragma_foreign_key_check`).Scan(&fkViolations); err != nil || fkViolations != 0 {
			t.Errorf("foreign key violations = %d (%v), want 0", fkViolations, err)
		}
	})

	for name, setup := range map[string]string{
		"a copy pulled from the server": `UPDATE environments SET created_by = 'sync' WHERE id = '` + seededEnvironmentID + `'`,
		"a copy the server confirmed":   `UPDATE environments SET is_synced = 1 WHERE id = '` + seededEnvironmentID + `'`,
	} {
		t.Run(name+" keeps its id", func(t *testing.T) {
			db := setupTestDB(t)
			seed(t, db)
			seedSQL(t, db, setup)

			newID, err := NewSyncQueueRepo(db).ReissueSeededEnvironment(ctx, ws)
			if err != nil || newID != "" {
				t.Fatalf("ReissueSeededEnvironment = %q, %v; want nothing moved", newID, err)
			}
			var vars int
			if err := db.QueryRow(`SELECT COUNT(*) FROM variables WHERE environment_id = ?`, seededEnvironmentID).Scan(&vars); err != nil || vars != 2 {
				t.Errorf("variables under the seeded id = %d (%v), want 2", vars, err)
			}
		})
	}

	t.Run("another workspace leaves it alone", func(t *testing.T) {
		db := setupTestDB(t)
		seed(t, db)
		seedSQL(t, db, `INSERT INTO workspaces (id, name) VALUES ('ws-2', 'Other')`)

		newID, err := NewSyncQueueRepo(db).ReissueSeededEnvironment(ctx, "ws-2")
		if err != nil || newID != "" {
			t.Fatalf("ReissueSeededEnvironment = %q, %v; want nothing moved", newID, err)
		}
		var left int
		if err := db.QueryRow(`SELECT COUNT(*) FROM environments WHERE id = ?`, seededEnvironmentID).Scan(&left); err != nil || left != 1 {
			t.Errorf("rows under the seeded id = %d (%v), want 1", left, err)
		}
	})
}
