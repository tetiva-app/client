package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"
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
	// Moves 'failed' entries whose retry window elapsed back to 'pending'.
	RequeueDue(ctx context.Context, workspaceID string, now time.Time) (int64, error)
	// Resets 'sending' back to 'pending'; called on startup.
	ResetSending(ctx context.Context) error
	DeleteByWorkspace(ctx context.Context, workspaceID string) (int, error)
	// Only the latest entry per (entity_type, entity_id).
	CoalescedPending(ctx context.Context, workspaceID string, limit int) ([]*SyncEntry, error)
	// Counts entries the server has not taken, parked retries included.
	CountPendingOrFailed(ctx context.Context, workspaceID string) (int, error)
	// Entries the server refused, waiting for a retry.
	CountParked(ctx context.Context, workspaceID string) (int, error)
	// ok is false when nothing is parked.
	EarliestParkedRetryAt(ctx context.Context, workspaceID string) (t time.Time, ok bool, err error)
}

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

	_, err := r.db.ExecContext(ctx, query, int64SliceToAny(ids)...)
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

// 'failed' entries are parked quota retries, not losses.
func (r *SyncQueueRepo) CountPendingOrFailed(ctx context.Context, workspaceID string) (int, error) {
	const funcName = "SyncQueueRepo.CountPendingOrFailed"

	query := `SELECT COUNT(*) FROM sync_queue WHERE workspace_id = ? AND status IN ('pending', 'failed')`

	var count int
	if err := r.db.QueryRowContext(ctx, query, workspaceID).Scan(&count); err != nil {
		return 0, fmt.Errorf("%s: %w", funcName, err)
	}

	return count, nil
}

// On the UI side these are the changes a plan limit keeps out of the cloud.
func (r *SyncQueueRepo) CountParked(ctx context.Context, workspaceID string) (int, error) {
	const funcName = "SyncQueueRepo.CountParked"

	query := `SELECT COUNT(*) FROM sync_queue WHERE workspace_id = ? AND status = 'failed'`

	var count int
	if err := r.db.QueryRowContext(ctx, query, workspaceID).Scan(&count); err != nil {
		return 0, fmt.Errorf("%s: %w", funcName, err)
	}

	return count, nil
}

// Lets a syncer that just started wake on time instead of waiting a full window.
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
