package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/tetiva-app/client/internal/domain"
)

type SyncEntry struct {
	ID          int64
	WorkspaceID string
	EntityType  string
	EntityID    string
	Action      string
	OperationID string
	Status      string
	RetryCount  int
	DeferCount  int
	NextRetryAt *time.Time
	CreatedAt   time.Time
}

type SyncQueueRepository interface {
	// Uses the TX from context when present.
	Enqueue(ctx context.Context, entry SyncEntry) error
	// Ordered by id ASC.
	ListPending(ctx context.Context, workspaceID string, limit int) ([]*SyncEntry, error)
	MarkSending(ctx context.Context, ids []int64) error
	Delete(ctx context.Context, ids []int64) error
	// Increments retry_count and schedules the next retry.
	MarkFailed(ctx context.Context, id int64, nextRetryAt time.Time) error
	MarkParked(ctx context.Context, id int64) error
	// Holds an entry whose parent the server has not seen yet; counted adds one to defer_count.
	MarkDeferred(ctx context.Context, id int64, nextRetryAt time.Time, counted bool) error
	// Reports whether the example's request has a row the push holds back: 'failed', 'parked' or 'deferred'.
	ExampleParentHeld(ctx context.Context, workspaceID, exampleID string) (bool, error)
	// Moves 'failed' and 'deferred' entries whose retry window elapsed back to 'pending'.
	RequeueDue(ctx context.Context, workspaceID string, now time.Time) (int64, error)
	DeleteSupersededParked(ctx context.Context, workspaceID string) (int, error)
	DeleteParkedForMissingEntities(ctx context.Context, workspaceID string) (int, error)
	// Resets 'sending' back to 'pending'; called on startup.
	ResetSending(ctx context.Context) error
	DeleteByWorkspace(ctx context.Context, workspaceID string) (int, error)
	// Only the latest entry per (entity_type, entity_id), parents' types first; excludeTypes are left out.
	CoalescedPending(ctx context.Context, workspaceID string, limit int, excludeTypes []string) ([]*SyncEntry, error)
	// Counts entries the server has not taken, parked retries included; excludeTypes are left out.
	CountPendingOrFailed(ctx context.Context, workspaceID string, excludeTypes []string) (int, error)
	CountParked(ctx context.Context, workspaceID string) (int, error)
	CountTooLarge(ctx context.Context, workspaceID string) (int, error)
	// ok is false when nothing is parked.
	EarliestParkedRetryAt(ctx context.Context, workspaceID string) (t time.Time, ok bool, err error)
	EnqueueDocumentedRequests(ctx context.Context, workspaceID string) (int, error)
	// Queues every example of the workspace the server has not confirmed, deleted ones as deletes.
	EnqueueUnsyncedExamples(ctx context.Context, workspaceID string) (int, error)
	// Reports whether an 'update' row was added: the request and its collection must be live
	// in workspaceID and the queue must hold no row of any status for it.
	EnqueueRequestIfAbsent(ctx context.Context, workspaceID, requestID string) (bool, error)
	// Queues a 'create' for every live entity of the workspace without a pending row, parent folders first.
	EnqueueWorkspace(ctx context.Context, workspaceID string) (int, error)
	// Queues the live rows of a collection's tree the server never confirmed and the queue holds no row for.
	EnqueueUnsyncedTree(ctx context.Context, workspaceID, collectionID string) (int, error)
	// EnqueueUnsyncedTree for every root of the workspace, environments included.
	EnqueueUnsyncedWorkspace(ctx context.Context, workspaceID string) (int, error)
	// Entries held back from the push: 'parked' and 'deferred'.
	ListHeld(ctx context.Context, workspaceID, entityType string) ([]*SyncEntry, error)
	// Deletes the pending rows of each entry's entity older than the entry itself.
	DropOlderPending(ctx context.Context, entries []*SyncEntry) error
	// Moves the workspace's copy of the seeded Default environment, if the server has never had it, to a
	// fresh id along with its variables and the publication settings naming it; "" when nothing moved.
	ReissueSeededEnvironment(ctx context.Context, workspaceID string) (string, error)
}

// sqlUUIDv4 generates a v4 UUID inside a statement, for rows inserted by INSERT … SELECT.
const sqlUUIDv4 = `lower(
		hex(randomblob(4)) || '-' ||
		hex(randomblob(2)) || '-4' ||
		substr(hex(randomblob(2)), 2) || '-' ||
		substr('89ab', (random() & 3) + 1, 1) || substr(hex(randomblob(2)), 2) || '-' ||
		hex(randomblob(6))
	)`

// 001 seeds the Default environment under this id on every install, and server ids are global,
// so the uploads a link queues leave it and its variables out; a first link reissues it first.
const seededEnvironmentID = "00000000-0000-4000-a000-000000000002"

type SyncQueueRepo struct {
	db *sql.DB
}

func NewSyncQueueRepo(db *sql.DB) SyncQueueRepository {
	return &SyncQueueRepo{db: db}
}

// Uses DBTXFromContext so it joins the caller's transaction when present.
func (r *SyncQueueRepo) Enqueue(ctx context.Context, entry SyncEntry) error {
	const funcName = "SyncQueueRepo.Enqueue"

	var nextRetryAt any
	if entry.NextRetryAt != nil {
		nextRetryAt = entry.NextRetryAt.Format(time.RFC3339)
	}

	query := `INSERT INTO sync_queue (workspace_id, entity_type, entity_id, action, operation_id, status, retry_count, next_retry_at, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`

	db := DBTXFromContext(ctx, r.db)
	_, err := db.ExecContext(ctx, query,
		entry.WorkspaceID,
		entry.EntityType,
		entry.EntityID,
		entry.Action,
		entry.OperationID,
		entry.Status,
		entry.RetryCount,
		nextRetryAt,
		entry.CreatedAt.Format(time.RFC3339),
	)
	if err != nil {
		return fmt.Errorf("%s: %w", funcName, err)
	}

	// Dropping held rows here, not at push time, closes a race with a running push.
	_, err = db.ExecContext(ctx,
		`DELETE FROM sync_queue
		 WHERE workspace_id = ? AND entity_type = ? AND entity_id = ? AND status IN ('parked', 'deferred')`,
		entry.WorkspaceID, entry.EntityType, entry.EntityID)
	if err != nil {
		return fmt.Errorf("%s: drop parked: %w", funcName, err)
	}

	return nil
}

func (r *SyncQueueRepo) ListPending(ctx context.Context, workspaceID string, limit int) ([]*SyncEntry, error) {
	const funcName = "SyncQueueRepo.ListPending"

	query := `SELECT id, workspace_id, entity_type, entity_id, action, operation_id, status, retry_count, defer_count, next_retry_at, created_at
		FROM sync_queue
		WHERE status = 'pending' AND workspace_id = ?
		ORDER BY id ASC
		LIMIT ?`

	rows, err := r.db.QueryContext(ctx, query, workspaceID, limit)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", funcName, err)
	}
	defer func() { _ = rows.Close() }()

	return scanSyncEntries(funcName, rows)
}

func (r *SyncQueueRepo) MarkSending(ctx context.Context, ids []int64) error {
	const funcName = "SyncQueueRepo.MarkSending"

	if len(ids) == 0 {
		return nil
	}

	query := fmt.Sprintf(
		`UPDATE sync_queue SET status = 'sending' WHERE id IN (%s)`,
		placeholders(len(ids)),
	)

	_, err := r.db.ExecContext(ctx, query, int64SliceToAny(ids)...)
	if err != nil {
		return fmt.Errorf("%s: %w", funcName, err)
	}

	return nil
}

func (r *SyncQueueRepo) Delete(ctx context.Context, ids []int64) error {
	const funcName = "SyncQueueRepo.Delete"

	if len(ids) == 0 {
		return nil
	}

	query := fmt.Sprintf(
		`DELETE FROM sync_queue WHERE id IN (%s)`,
		placeholders(len(ids)),
	)

	_, err := DBTXFromContext(ctx, r.db).ExecContext(ctx, query, int64SliceToAny(ids)...)
	if err != nil {
		return fmt.Errorf("%s: %w", funcName, err)
	}

	return nil
}

func (r *SyncQueueRepo) MarkFailed(ctx context.Context, id int64, nextRetryAt time.Time) error {
	const funcName = "SyncQueueRepo.MarkFailed"

	query := `UPDATE sync_queue SET status = 'failed', retry_count = retry_count + 1, next_retry_at = ? WHERE id = ?`

	_, err := r.db.ExecContext(ctx, query, nextRetryAt.Format(time.RFC3339), id)
	if err != nil {
		return fmt.Errorf("%s: %w", funcName, err)
	}

	return nil
}

func (r *SyncQueueRepo) MarkParked(ctx context.Context, id int64) error {
	const funcName = "SyncQueueRepo.MarkParked"

	query := `UPDATE sync_queue SET status = 'parked', retry_count = retry_count + 1, next_retry_at = NULL WHERE id = ?`

	_, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("%s: %w", funcName, err)
	}

	return nil
}

func (r *SyncQueueRepo) MarkDeferred(ctx context.Context, id int64, nextRetryAt time.Time, counted bool) error {
	const funcName = "SyncQueueRepo.MarkDeferred"

	query := `UPDATE sync_queue SET status = 'deferred', defer_count = defer_count + ?, next_retry_at = ? WHERE id = ?`

	_, err := DBTXFromContext(ctx, r.db).ExecContext(ctx, query, boolToInt(counted), nextRetryAt.Format(time.RFC3339), id)
	if err != nil {
		return fmt.Errorf("%s: %w", funcName, err)
	}

	return nil
}

func (r *SyncQueueRepo) ExampleParentHeld(ctx context.Context, workspaceID, exampleID string) (bool, error) {
	const funcName = "SyncQueueRepo.ExampleParentHeld"

	query := `SELECT EXISTS (
		SELECT 1 FROM response_examples e
		JOIN sync_queue q ON q.entity_type = 'request' AND q.entity_id = e.request_id
		WHERE e.id = ? AND q.workspace_id = ? AND q.status IN ('failed', 'parked', 'deferred')
	)`

	var held bool
	if err := DBTXFromContext(ctx, r.db).QueryRowContext(ctx, query, exampleID, workspaceID).Scan(&held); err != nil {
		return false, fmt.Errorf("%s: %w", funcName, err)
	}

	return held, nil
}

func (r *SyncQueueRepo) DeleteSupersededParked(ctx context.Context, workspaceID string) (int, error) {
	const funcName = "SyncQueueRepo.DeleteSupersededParked"

	query := `DELETE FROM sync_queue
		WHERE workspace_id = ? AND status IN ('parked', 'deferred')
		  AND EXISTS (
			SELECT 1 FROM sync_queue newer
			WHERE newer.workspace_id = sync_queue.workspace_id
			  AND newer.entity_type = sync_queue.entity_type
			  AND newer.entity_id = sync_queue.entity_id
			  AND newer.status = 'pending'
			  AND newer.id > sync_queue.id
		  )`

	result, err := r.db.ExecContext(ctx, query, workspaceID)
	if err != nil {
		return 0, fmt.Errorf("%s: %w", funcName, err)
	}

	n, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("%s: rows affected: %w", funcName, err)
	}

	return int(n), nil
}

func (r *SyncQueueRepo) DeleteParkedForMissingEntities(ctx context.Context, workspaceID string) (int, error) {
	const funcName = "SyncQueueRepo.DeleteParkedForMissingEntities"

	query := `DELETE FROM sync_queue
		WHERE workspace_id = ? AND status = 'parked'
		  AND entity_type IN ('collection', 'request', 'environment', 'variable')
		  AND NOT EXISTS (
			SELECT 1 FROM collections c
			WHERE sync_queue.entity_type = 'collection' AND c.id = sync_queue.entity_id AND c.is_delete = 0
			UNION ALL
			SELECT 1 FROM requests r
			JOIN collections rc ON rc.id = r.collection_id
			WHERE sync_queue.entity_type = 'request' AND r.id = sync_queue.entity_id
			  AND r.is_delete = 0 AND rc.is_delete = 0
			UNION ALL
			SELECT 1 FROM environments e
			WHERE sync_queue.entity_type = 'environment' AND e.id = sync_queue.entity_id AND e.is_delete = 0
			UNION ALL
			SELECT 1 FROM variables v
			JOIN environments ve ON ve.id = v.environment_id
			WHERE sync_queue.entity_type = 'variable' AND v.id = sync_queue.entity_id
			  AND v.is_delete = 0 AND ve.is_delete = 0
		  )`

	result, err := DBTXFromContext(ctx, r.db).ExecContext(ctx, query, workspaceID)
	if err != nil {
		return 0, fmt.Errorf("%s: %w", funcName, err)
	}

	n, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("%s: rows affected: %w", funcName, err)
	}

	return int(n), nil
}

func (r *SyncQueueRepo) RequeueDue(ctx context.Context, workspaceID string, now time.Time) (int64, error) {
	const funcName = "SyncQueueRepo.RequeueDue"

	query := `UPDATE sync_queue SET status = 'pending'
		WHERE workspace_id = ? AND status IN ('failed', 'deferred') AND next_retry_at <= ?`

	result, err := r.db.ExecContext(ctx, query, workspaceID, now.Format(time.RFC3339))
	if err != nil {
		return 0, fmt.Errorf("%s: %w", funcName, err)
	}

	n, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("%s: rows affected: %w", funcName, err)
	}

	return n, nil
}

// Called on startup to recover from interrupted sessions.
func (r *SyncQueueRepo) ResetSending(ctx context.Context) error {
	const funcName = "SyncQueueRepo.ResetSending"

	query := `UPDATE sync_queue SET status = 'pending' WHERE status = 'sending'`

	_, err := r.db.ExecContext(ctx, query)
	if err != nil {
		return fmt.Errorf("%s: %w", funcName, err)
	}

	return nil
}

func (r *SyncQueueRepo) DeleteByWorkspace(ctx context.Context, workspaceID string) (int, error) {
	const funcName = "SyncQueueRepo.DeleteByWorkspace"

	query := `DELETE FROM sync_queue WHERE workspace_id = ?`

	result, err := DBTXFromContext(ctx, r.db).ExecContext(ctx, query, workspaceID)
	if err != nil {
		return 0, fmt.Errorf("%s: %w", funcName, err)
	}

	n, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("%s: rows affected: %w", funcName, err)
	}

	return int(n), nil
}

func (r *SyncQueueRepo) EnqueueDocumentedRequests(ctx context.Context, workspaceID string) (int, error) {
	const funcName = "SyncQueueRepo.EnqueueDocumentedRequests"

	query := `INSERT INTO sync_queue (workspace_id, entity_type, entity_id, action, operation_id, status, retry_count, created_at)
		SELECT
			c.workspace_id,
			'request',
			r.id,
			'update',
			` + sqlUUIDv4 + `,
			'pending',
			0,
			strftime('%Y-%m-%dT%H:%M:%SZ', 'now')
		FROM requests r
		JOIN collections c ON c.id = r.collection_id
		WHERE c.workspace_id = ?
		  AND r.description != ''
		  AND length(CAST(r.description AS BLOB)) <= ?
		  AND r.is_delete = 0
		  AND r.is_draft = 0
		  AND c.is_delete = 0
		  AND NOT EXISTS (
			SELECT 1 FROM sync_queue q
			WHERE q.workspace_id = c.workspace_id
			  AND q.entity_type = 'request'
			  AND q.entity_id = r.id
			  AND q.status = 'pending'
		  )`

	result, err := DBTXFromContext(ctx, r.db).ExecContext(ctx, query, workspaceID, domain.MaxDescriptionLen)
	if err != nil {
		return 0, fmt.Errorf("%s: %w", funcName, err)
	}

	n, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("%s: rows affected: %w", funcName, err)
	}

	return int(n), nil
}

func (r *SyncQueueRepo) EnqueueUnsyncedExamples(ctx context.Context, workspaceID string) (int, error) {
	const funcName = "SyncQueueRepo.EnqueueUnsyncedExamples"

	query := `INSERT INTO sync_queue (workspace_id, entity_type, entity_id, action, operation_id, status, retry_count, created_at)
		SELECT
			e.workspace_id,
			'response_example',
			e.id,
			CASE e.is_delete WHEN 1 THEN 'delete' ELSE 'create' END,
			` + sqlUUIDv4 + `,
			'pending',
			0,
			strftime('%Y-%m-%dT%H:%M:%SZ', 'now')
		FROM response_examples e
		WHERE e.workspace_id = ?
		  AND e.is_synced = 0
		  AND NOT EXISTS (
			SELECT 1 FROM sync_queue q
			WHERE q.workspace_id = e.workspace_id
			  AND q.entity_type = 'response_example'
			  AND q.entity_id = e.id
			  AND q.status = 'pending'
		  )`

	result, err := DBTXFromContext(ctx, r.db).ExecContext(ctx, query, workspaceID)
	if err != nil {
		return 0, fmt.Errorf("%s: %w", funcName, err)
	}

	n, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("%s: rows affected: %w", funcName, err)
	}

	return int(n), nil
}

func (r *SyncQueueRepo) EnqueueRequestIfAbsent(ctx context.Context, workspaceID, requestID string) (bool, error) {
	const funcName = "SyncQueueRepo.EnqueueRequestIfAbsent"

	query := `INSERT INTO sync_queue (workspace_id, entity_type, entity_id, action, operation_id, status, retry_count, created_at)
		SELECT c.workspace_id, 'request', r.id, 'update', ` + sqlUUIDv4 + `, 'pending', 0, strftime('%Y-%m-%dT%H:%M:%SZ', 'now')
		FROM requests r
		JOIN collections c ON c.id = r.collection_id
		WHERE r.id = ?
		  AND c.workspace_id = ?
		  AND r.is_delete = 0
		  AND r.is_draft = 0
		  AND c.is_delete = 0
		  AND NOT EXISTS (
			SELECT 1 FROM sync_queue q
			WHERE q.workspace_id = c.workspace_id
			  AND q.entity_type = 'request'
			  AND q.entity_id = r.id
		  )`

	result, err := DBTXFromContext(ctx, r.db).ExecContext(ctx, query, requestID, workspaceID)
	if err != nil {
		return false, fmt.Errorf("%s: %w", funcName, err)
	}

	n, err := result.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("%s: rows affected: %w", funcName, err)
	}

	return n > 0, nil
}

func (r *SyncQueueRepo) EnqueueWorkspace(ctx context.Context, workspaceID string) (int, error) {
	n, err := r.enqueueLive(ctx, `parent_id IS NULL`, []any{workspaceID}, true, false)
	if err != nil {
		return 0, fmt.Errorf("SyncQueueRepo.EnqueueWorkspace: %w", err)
	}
	return n, nil
}

func (r *SyncQueueRepo) EnqueueUnsyncedTree(ctx context.Context, workspaceID, collectionID string) (int, error) {
	n, err := r.enqueueLive(ctx, `id = ?2`, []any{workspaceID, collectionID}, false, true)
	if err != nil {
		return 0, fmt.Errorf("SyncQueueRepo.EnqueueUnsyncedTree: %w", err)
	}
	return n, nil
}

func (r *SyncQueueRepo) EnqueueUnsyncedWorkspace(ctx context.Context, workspaceID string) (int, error) {
	n, err := r.enqueueLive(ctx, `parent_id IS NULL`, []any{workspaceID}, true, true)
	if err != nil {
		return 0, fmt.Errorf("SyncQueueRepo.EnqueueUnsyncedWorkspace: %w", err)
	}
	return n, nil
}

// enqueueLive queues the live entities under the root collections picked by roots; ?1 is the workspace.
// Folders go in depth order: a push batch that carried a child before its parent would lose the child.
func (r *SyncQueueRepo) enqueueLive(ctx context.Context, roots string, args []any, withEnvironments, unsyncedOnly bool) (int, error) {
	tree := `WITH RECURSIVE tree(id, depth) AS (
			SELECT id, 0 FROM collections WHERE workspace_id = ?1 AND is_delete = 0 AND ` + roots + `
			UNION ALL
			SELECT c.id, t.depth + 1 FROM collections c JOIN tree t ON c.parent_id = t.id WHERE c.is_delete = 0
		) `
	liveRequests := `(SELECT r.id FROM requests r
		WHERE r.collection_id IN (SELECT id FROM tree) AND r.is_delete = 0 AND r.is_draft = 0)`

	type source struct{ entityType, from, orderBy string }
	sources := []source{{"collection", `tree JOIN collections x ON x.id = tree.id WHERE 1 = 1`, ` ORDER BY tree.depth, x.sort_order, x.id`}}
	if withEnvironments {
		sources = append(sources, source{entityType: "environment", from: `environments x
			WHERE x.workspace_id = ?1 AND x.is_delete = 0 AND x.id <> '` + seededEnvironmentID + `'`})
	}
	sources = append(sources, source{entityType: "request", from: `requests x WHERE x.id IN ` + liveRequests})
	if withEnvironments {
		sources = append(sources, source{entityType: "variable", from: `variables x JOIN environments e ON e.id = x.environment_id
			WHERE e.workspace_id = ?1 AND e.is_delete = 0 AND x.is_delete = 0 AND e.id <> '` + seededEnvironmentID + `'`})
	}
	sources = append(sources, source{entityType: "response_example", from: `response_examples x
		WHERE x.workspace_id = ?1 AND x.is_delete = 0 AND x.request_id IN ` + liveRequests})

	total := 0
	err := WithTx(ctx, r.db, func(txCtx context.Context) error {
		for _, s := range sources {
			held := `q.status = 'pending'`
			filter := ``
			if unsyncedOnly {
				held, filter = `1 = 1`, ` AND x.is_synced = 0`
			}
			query := tree + `INSERT INTO sync_queue (workspace_id, entity_type, entity_id, action, operation_id, status, retry_count, created_at)
				SELECT ?1, '` + s.entityType + `', x.id, 'create', ` + sqlUUIDv4 + `, 'pending', 0, strftime('%Y-%m-%dT%H:%M:%SZ', 'now')
				FROM ` + s.from + filter + `
				  AND NOT EXISTS (SELECT 1 FROM sync_queue q
					WHERE q.workspace_id = ?1 AND q.entity_type = '` + s.entityType + `' AND q.entity_id = x.id AND ` + held + `)` + s.orderBy
			result, err := DBTXFromContext(txCtx, r.db).ExecContext(txCtx, query, args...)
			if err != nil {
				return fmt.Errorf("%s: %w", s.entityType, err)
			}
			n, err := result.RowsAffected()
			if err != nil {
				return fmt.Errorf("%s: rows affected: %w", s.entityType, err)
			}
			total += int(n)
		}
		return nil
	})
	if err != nil {
		return 0, err
	}
	return total, nil
}

func (r *SyncQueueRepo) ReissueSeededEnvironment(ctx context.Context, workspaceID string) (string, error) {
	const funcName = "SyncQueueRepo.ReissueSeededEnvironment"

	newID := uuid.NewString()
	moved := false
	err := WithTx(ctx, r.db, func(txCtx context.Context) error {
		db := DBTXFromContext(txCtx, r.db)
		// The variables still name the old id until the statements below run; the check waits for the commit.
		if _, err := db.ExecContext(txCtx, `PRAGMA defer_foreign_keys = ON`); err != nil {
			return fmt.Errorf("defer foreign keys: %w", err)
		}
		// created_by, not is_synced alone: a pull before 1.2.0 never marked what it wrote synced.
		result, err := db.ExecContext(txCtx,
			`UPDATE environments SET id = ?, updated_at = strftime('%Y-%m-%dT%H:%M:%SZ', 'now')
			 WHERE id = ? AND workspace_id = ? AND created_by <> 'sync' AND is_synced = 0`,
			newID, seededEnvironmentID, workspaceID)
		if err != nil {
			return fmt.Errorf("environment: %w", err)
		}
		n, err := result.RowsAffected()
		if err != nil {
			return fmt.Errorf("environment: rows affected: %w", err)
		}
		if n == 0 {
			return nil
		}
		moved = true
		if _, err := db.ExecContext(txCtx,
			`UPDATE variables SET environment_id = ? WHERE environment_id = ?`, newID, seededEnvironmentID); err != nil {
			return fmt.Errorf("variables: %w", err)
		}
		if _, err := db.ExecContext(txCtx,
			`UPDATE publications SET settings = json_set(settings, '$.environmentId', ?)
			 WHERE workspace_id = ? AND json_extract(settings, '$.environmentId') = ?`,
			newID, workspaceID, seededEnvironmentID); err != nil {
			return fmt.Errorf("publication settings: %w", err)
		}
		return nil
	})
	if err != nil {
		return "", fmt.Errorf("%s: %w", funcName, err)
	}
	if !moved {
		return "", nil
	}
	return newID, nil
}

func (r *SyncQueueRepo) ListHeld(ctx context.Context, workspaceID, entityType string) ([]*SyncEntry, error) {
	const funcName = "SyncQueueRepo.ListHeld"

	query := `SELECT id, workspace_id, entity_type, entity_id, action, operation_id, status, retry_count, defer_count, next_retry_at, created_at
		FROM sync_queue
		WHERE workspace_id = ? AND entity_type = ? AND status IN ('parked', 'deferred')
		ORDER BY id ASC`

	rows, err := DBTXFromContext(ctx, r.db).QueryContext(ctx, query, workspaceID, entityType)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", funcName, err)
	}
	defer func() { _ = rows.Close() }()

	return scanSyncEntries(funcName, rows)
}

// The entry need not exist any more: its ID only bounds which rows count as older.
func (r *SyncQueueRepo) DropOlderPending(ctx context.Context, entries []*SyncEntry) error {
	const funcName = "SyncQueueRepo.DropOlderPending"

	db := DBTXFromContext(ctx, r.db)
	for _, e := range entries {
		_, err := db.ExecContext(ctx,
			`DELETE FROM sync_queue
			 WHERE workspace_id = ? AND entity_type = ? AND entity_id = ? AND status = 'pending' AND id < ?`,
			e.WorkspaceID, e.EntityType, e.EntityID, e.ID)
		if err != nil {
			return fmt.Errorf("%s: %w", funcName, err)
		}
	}

	return nil
}

func (r *SyncQueueRepo) CoalescedPending(ctx context.Context, workspaceID string, limit int, excludeTypes []string) ([]*SyncEntry, error) {
	const funcName = "SyncQueueRepo.CoalescedPending"

	excluded, excludedArgs := notInTypes(excludeTypes)
	// Folders go by their depth in the local tree, not by row id: an edit queued for a parent after
	// its children would otherwise send the children first, and the server refuses them for good.
	query := `WITH RECURSIVE up(id, ancestor, depth) AS (
			SELECT id, parent_id, 0 FROM collections WHERE id IN (
				SELECT entity_id FROM sync_queue WHERE status = 'pending' AND workspace_id = ? AND entity_type = 'collection')
			UNION ALL
			SELECT up.id, c.parent_id, up.depth + 1 FROM up JOIN collections c ON c.id = up.ancestor WHERE up.depth < 256
		)
		SELECT sq.id, sq.workspace_id, sq.entity_type, sq.entity_id, sq.action, sq.operation_id, sq.status, sq.retry_count, sq.defer_count, sq.next_retry_at, sq.created_at
		FROM sync_queue sq
		INNER JOIN (
			SELECT MAX(id) AS max_id
			FROM sync_queue
			WHERE status = 'pending' AND workspace_id = ?` + excluded + `
			GROUP BY entity_type, entity_id
		) latest ON sq.id = latest.max_id
		LEFT JOIN (SELECT id, MAX(depth) AS depth FROM up GROUP BY id) folder
			ON sq.entity_type = 'collection' AND folder.id = sq.entity_id
		ORDER BY CASE sq.entity_type
			WHEN 'collection' THEN 0
			WHEN 'environment' THEN 1
			WHEN 'request' THEN 2
			WHEN 'variable' THEN 3
			WHEN 'response_example' THEN 4
			ELSE 5
		END, COALESCE(folder.depth, 0), sq.id ASC
		LIMIT ?`

	args := append(append([]any{workspaceID, workspaceID}, excludedArgs...), limit)
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", funcName, err)
	}
	defer func() { _ = rows.Close() }()

	return scanSyncEntries(funcName, rows)
}

func (r *SyncQueueRepo) CountPendingOrFailed(ctx context.Context, workspaceID string, excludeTypes []string) (int, error) {
	const funcName = "SyncQueueRepo.CountPendingOrFailed"

	excluded, excludedArgs := notInTypes(excludeTypes)
	query := `SELECT COUNT(*) FROM sync_queue
		WHERE workspace_id = ? AND status IN ('pending', 'failed', 'parked', 'deferred')` + excluded

	var count int
	if err := r.db.QueryRowContext(ctx, query, append([]any{workspaceID}, excludedArgs...)...).Scan(&count); err != nil {
		return 0, fmt.Errorf("%s: %w", funcName, err)
	}

	return count, nil
}

// Plan-held entries live under 'failed' with a retry; 'parked' is CountTooLarge.
func (r *SyncQueueRepo) CountParked(ctx context.Context, workspaceID string) (int, error) {
	const funcName = "SyncQueueRepo.CountParked"

	query := `SELECT COUNT(*) FROM sync_queue
		WHERE workspace_id = ? AND status = 'failed' AND next_retry_at IS NOT NULL`

	var count int
	if err := r.db.QueryRowContext(ctx, query, workspaceID).Scan(&count); err != nil {
		return 0, fmt.Errorf("%s: %w", funcName, err)
	}

	return count, nil
}

func (r *SyncQueueRepo) CountTooLarge(ctx context.Context, workspaceID string) (int, error) {
	const funcName = "SyncQueueRepo.CountTooLarge"

	query := `SELECT COUNT(*) FROM sync_queue WHERE workspace_id = ? AND status = 'parked'`

	var count int
	if err := r.db.QueryRowContext(ctx, query, workspaceID).Scan(&count); err != nil {
		return 0, fmt.Errorf("%s: %w", funcName, err)
	}

	return count, nil
}

// Lets a syncer that just started wake on time instead of waiting a full window.
func (r *SyncQueueRepo) EarliestParkedRetryAt(ctx context.Context, workspaceID string) (time.Time, bool, error) {
	const funcName = "SyncQueueRepo.EarliestParkedRetryAt"

	query := `SELECT MIN(next_retry_at) FROM sync_queue WHERE workspace_id = ? AND status IN ('failed', 'deferred')`

	var raw sql.NullString
	if err := r.db.QueryRowContext(ctx, query, workspaceID).Scan(&raw); err != nil {
		return time.Time{}, false, fmt.Errorf("%s: %w", funcName, err)
	}
	if !raw.Valid {
		return time.Time{}, false, nil
	}

	parsed, err := time.Parse(time.RFC3339, raw.String)
	if err != nil {
		return time.Time{}, false, fmt.Errorf("%s: parse next_retry_at: %w", funcName, err)
	}
	return parsed, true, nil
}

func scanSyncEntries(funcName string, rows *sql.Rows) ([]*SyncEntry, error) {
	var result []*SyncEntry
	for rows.Next() {
		e, err := scanSyncEntry(rows)
		if err != nil {
			return nil, fmt.Errorf("%s: scan: %w", funcName, err)
		}
		result = append(result, e)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("%s: rows: %w", funcName, err)
	}
	return result, nil
}

func scanSyncEntry(s scannable) (*SyncEntry, error) {
	var (
		e            SyncEntry
		nextRetryAt  sql.NullString
		createdAtStr string
	)

	err := s.Scan(
		&e.ID,
		&e.WorkspaceID,
		&e.EntityType,
		&e.EntityID,
		&e.Action,
		&e.OperationID,
		&e.Status,
		&e.RetryCount,
		&e.DeferCount,
		&nextRetryAt,
		&createdAtStr,
	)
	if err != nil {
		return nil, err
	}

	e.CreatedAt, err = parseTime(createdAtStr)
	if err != nil {
		return nil, fmt.Errorf("parse created_at: %w", err)
	}

	if nextRetryAt.Valid && nextRetryAt.String != "" {
		t, err := parseTime(nextRetryAt.String)
		if err != nil {
			return nil, fmt.Errorf("parse next_retry_at: %w", err)
		}
		e.NextRetryAt = &t
	}

	return &e, nil
}

func notInTypes(types []string) (string, []any) {
	if len(types) == 0 {
		return "", nil
	}
	args := make([]any, len(types))
	for i, t := range types {
		args[i] = t
	}
	return " AND entity_type NOT IN (" + placeholders(len(types)) + ")", args
}

func placeholders(n int) string {
	if n <= 0 {
		return ""
	}
	return strings.TrimRight(strings.Repeat("?,", n), ",")
}

func int64SliceToAny(ids []int64) []any {
	args := make([]any, len(ids))
	for i, id := range ids {
		args[i] = id
	}
	return args
}
