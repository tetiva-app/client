package sqlite

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
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

	list, err := repo.CoalescedPending(ctx, wsID, 10)
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

	count, err := repo.CountParked(ctx, wsID)
	if err != nil {
		t.Fatalf("CountParked failed: %v", err)
	}
	if count != 2 {
		t.Errorf("CountParked() = %d, want 2", count)
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

	count, err := repo.CountPendingOrFailed(ctx, wsID)
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
