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

	// An oversized entity is a size problem, not a plan problem.
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

	unsynced, err := repo.CountPendingOrFailed(ctx, wsID)
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
	// Parked after the newer write landed, the state a push racing Enqueue leaves behind.
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

	coalesced, err := repo.CoalescedPending(ctx, wsID, 10)
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

	// Rows that share the id but not the entity: neither carries the request's docs.
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

	// A pending row of a deleted entity is a delete on its way out, not a leftover.
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
