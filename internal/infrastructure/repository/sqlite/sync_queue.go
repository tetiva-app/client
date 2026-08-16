package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"
)

// SyncEntry represents a pending sync operation.
type SyncEntry struct {
	ID          int64
	WorkspaceID string
	EntityType  string
	EntityID    string
	Action      string
	OperationID string
	Status      string
	RetryCount  int
	NextRetryAt *time.Time
	CreatedAt   time.Time
}

// SyncQueueRepository manages the sync outbox queue.
type SyncQueueRepository interface {
	// Enqueue inserts a new entry into the sync queue using the TX from context if present.
	Enqueue(ctx context.Context, entry SyncEntry) error
	// ListPending returns pending entries for a workspace ordered by id ASC.
	ListPending(ctx context.Context, workspaceID string, limit int) ([]*SyncEntry, error)
	// MarkSending sets the status of the given entries to 'sending'.
	MarkSending(ctx context.Context, ids []int64) error
	// Delete removes entries by their IDs.
	Delete(ctx context.Context, ids []int64) error
	// MarkFailed increments retry_count, sets status='failed', and schedules next retry.
	MarkFailed(ctx context.Context, id int64, nextRetryAt time.Time) error
	// RequeueDue returns 'failed' entries whose retry window has elapsed back to 'pending'.
	RequeueDue(ctx context.Context, workspaceID string, now time.Time) (int64, error)
	// ResetSending resets all 'sending' entries back to 'pending' (called on startup).
	ResetSending(ctx context.Context) error
	// DeleteByWorkspace removes all entries for a workspace and returns the count deleted.
	DeleteByWorkspace(ctx context.Context, workspaceID string) (int, error)
	// CoalescedPending returns deduplicated pending entries: only the latest entry per (entity_type, entity_id).
	CoalescedPending(ctx context.Context, workspaceID string, limit int) ([]*SyncEntry, error)
	// CountPendingOrFailed counts entries the server has not taken yet, including
	// the ones parked for a later retry.
	CountPendingOrFailed(ctx context.Context, workspaceID string) (int, error)
	// CountParked counts entries the server refused and that wait for a retry.
	CountParked(ctx context.Context, workspaceID string) (int, error)
	// EarliestParkedRetryAt returns when the first parked entry falls due; ok is false when none is parked.
	EarliestParkedRetryAt(ctx context.Context, workspaceID string) (t time.Time, ok bool, err error)
}

// SyncQueueRepo implements SyncQueueRepository using SQLite.
type SyncQueueRepo struct {
	db *sql.DB
}

// NewSyncQueueRepo creates a new SyncQueueRepo instance.
func NewSyncQueueRepo(db *sql.DB) SyncQueueRepository {
	return &SyncQueueRepo{db: db}
}

// Enqueue inserts a new entry into the sync queue.
// Uses DBTXFromContext so it participates in the caller's transaction when present.
func (r *SyncQueueRepo) Enqueue(ctx context.Context, entry SyncEntry) error {
	const funcName = "SyncQueueRepo.Enqueue"

	var nextRetryAt any
	if entry.NextRetryAt != nil {
		nextRetryAt = entry.NextRetryAt.Format(time.RFC3339)
	}

	query := `INSERT INTO sync_queue (workspace_id, entity_type, entity_id, action, operation_id, status, retry_count, next_retry_at, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`

	_, err := DBTXFromContext(ctx, r.db).ExecContext(ctx, query,
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

	return nil
}

// ListPending returns pending entries for a workspace ordered by id ASC.
func (r *SyncQueueRepo) ListPending(ctx context.Context, workspaceID string, limit int) ([]*SyncEntry, error) {
	const funcName = "SyncQueueRepo.ListPending"

	query := `SELECT id, workspace_id, entity_type, entity_id, action, operation_id, status, retry_count, next_retry_at, created_at
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

// MarkSending sets the status of the given entries to 'sending'.
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

// Delete removes entries by their IDs.
func (r *SyncQueueRepo) Delete(ctx context.Context, ids []int64) error {
	const funcName = "SyncQueueRepo.Delete"

	if len(ids) == 0 {
		return nil
	}

	query := fmt.Sprintf(
		`DELETE FROM sync_queue WHERE id IN (%s)`,
		placeholders(len(ids)),
	)

	_, err := r.db.ExecContext(ctx, query, int64SliceToAny(ids)...)
	if err != nil {
		return fmt.Errorf("%s: %w", funcName, err)
	}

	return nil
}

// MarkFailed increments retry_count, sets status='failed', and schedules the next retry.
func (r *SyncQueueRepo) MarkFailed(ctx context.Context, id int64, nextRetryAt time.Time) error {
	const funcName = "SyncQueueRepo.MarkFailed"

	query := `UPDATE sync_queue SET status = 'failed', retry_count = retry_count + 1, next_retry_at = ? WHERE id = ?`

	_, err := r.db.ExecContext(ctx, query, nextRetryAt.Format(time.RFC3339), id)
	if err != nil {
		return fmt.Errorf("%s: %w", funcName, err)
	}

	return nil
}

// RequeueDue returns failed entries whose retry window has elapsed back to 'pending'
// and reports how many were requeued.
func (r *SyncQueueRepo) RequeueDue(ctx context.Context, workspaceID string, now time.Time) (int64, error) {
	const funcName = "SyncQueueRepo.RequeueDue"

	query := `UPDATE sync_queue SET status = 'pending'
		WHERE workspace_id = ? AND status = 'failed' AND next_retry_at <= ?`

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

// ResetSending resets all 'sending' entries back to 'pending'.
// Should be called on startup to recover from interrupted sessions.
func (r *SyncQueueRepo) ResetSending(ctx context.Context) error {
	const funcName = "SyncQueueRepo.ResetSending"

	query := `UPDATE sync_queue SET status = 'pending' WHERE status = 'sending'`

	_, err := r.db.ExecContext(ctx, query)
	if err != nil {
		return fmt.Errorf("%s: %w", funcName, err)
	}

	return nil
}

// DeleteByWorkspace removes all entries for a workspace and returns the count deleted.
func (r *SyncQueueRepo) DeleteByWorkspace(ctx context.Context, workspaceID string) (int, error) {
	const funcName = "SyncQueueRepo.DeleteByWorkspace"

	query := `DELETE FROM sync_queue WHERE workspace_id = ?`

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

// CoalescedPending returns deduplicated pending entries, keeping only the latest entry
// per (entity_type, entity_id) combination.
func (r *SyncQueueRepo) CoalescedPending(ctx context.Context, workspaceID string, limit int) ([]*SyncEntry, error) {
	const funcName = "SyncQueueRepo.CoalescedPending"

	query := `SELECT sq.id, sq.workspace_id, sq.entity_type, sq.entity_id, sq.action, sq.operation_id, sq.status, sq.retry_count, sq.next_retry_at, sq.created_at
		FROM sync_queue sq
		INNER JOIN (
			SELECT MAX(id) AS max_id
			FROM sync_queue
			WHERE status = 'pending' AND workspace_id = ?
			GROUP BY entity_type, entity_id
		) latest ON sq.id = latest.max_id
		ORDER BY sq.id ASC
		LIMIT ?`

	rows, err := r.db.QueryContext(ctx, query, workspaceID, limit)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", funcName, err)
	}
	defer func() { _ = rows.Close() }()

	return scanSyncEntries(funcName, rows)
}

// CountPendingOrFailed counts entries still owed to the server: 'failed' ones are
// parked quota retries, not losses.
func (r *SyncQueueRepo) CountPendingOrFailed(ctx context.Context, workspaceID string) (int, error) {
	const funcName = "SyncQueueRepo.CountPendingOrFailed"

	query := `SELECT COUNT(*) FROM sync_queue WHERE workspace_id = ? AND status IN ('pending', 'failed')`

	var count int
	if err := r.db.QueryRowContext(ctx, query, workspaceID).Scan(&count); err != nil {
		return 0, fmt.Errorf("%s: %w", funcName, err)
	}

	return count, nil
}

// CountParked counts the entries the server pushed back: on the UI side they are
// the changes a plan limit keeps out of the cloud.
func (r *SyncQueueRepo) CountParked(ctx context.Context, workspaceID string) (int, error) {
	const funcName = "SyncQueueRepo.CountParked"

	query := `SELECT COUNT(*) FROM sync_queue WHERE workspace_id = ? AND status = 'failed'`

	var count int
	if err := r.db.QueryRowContext(ctx, query, workspaceID).Scan(&count); err != nil {
		return 0, fmt.Errorf("%s: %w", funcName, err)
	}

	return count, nil
}

// EarliestParkedRetryAt returns the closest retry deadline among the parked entries,
// so a syncer that just started knows when to wake instead of waiting a full window.
func (r *SyncQueueRepo) EarliestParkedRetryAt(ctx context.Context, workspaceID string) (time.Time, bool, error) {
	const funcName = "SyncQueueRepo.EarliestParkedRetryAt"

	query := `SELECT MIN(next_retry_at) FROM sync_queue WHERE workspace_id = ? AND status = 'failed'`

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
