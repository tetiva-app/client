package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/tetiva-app/client/internal/domain"
	"github.com/tetiva-app/client/internal/domain/entities"
)

const responseExampleColumns = `id, request_id, workspace_id, name, status_code, status_text, headers, body, content_type,
	protocol, sort_order, version, is_delete, created_by, created_at, updated_by, updated_at`

// Local writes reset is_synced; only the sync engine sets it back to 1.
type ResponseExampleRepo struct {
	db *sql.DB
}

func NewResponseExampleRepo(db *sql.DB) *ResponseExampleRepo {
	return &ResponseExampleRepo{db: db}
}

func (r *ResponseExampleRepo) Create(ctx context.Context, e *entities.ResponseExample) error {
	const funcName = "ResponseExampleRepo.Create"

	headersJSON, err := json.Marshal(headersToJSON(e.Headers))
	if err != nil {
		return fmt.Errorf("%s: marshal headers: %w", funcName, err)
	}

	query := `INSERT INTO response_examples (` + responseExampleColumns + `, is_synced)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 0)`

	_, err = DBTXFromContext(ctx, r.db).ExecContext(ctx, query,
		e.ID.String(),
		e.RequestID.String(),
		e.WorkspaceID.String(),
		e.Name,
		e.StatusCode,
		e.StatusText,
		string(headersJSON),
		e.Body,
		e.ContentType,
		string(e.Protocol),
		e.SortOrder,
		e.Version,
		boolToInt(e.IsDelete),
		e.CreatedBy,
		e.CreatedAt.Format(time.RFC3339),
		e.UpdatedBy,
		e.UpdatedAt.Format(time.RFC3339),
	)
	if err != nil {
		return fmt.Errorf("%s: %w", funcName, err)
	}

	return nil
}

// Returns nil when the row is missing or soft-deleted.
func (r *ResponseExampleRepo) GetByID(ctx context.Context, id uuid.UUID) (*entities.ResponseExample, error) {
	return r.get(ctx, id, true)
}

// GetByIDIncludingDeleted returns soft-deleted rows too: a tombstone for an older server carries the last state.
func (r *ResponseExampleRepo) GetByIDIncludingDeleted(ctx context.Context, id uuid.UUID) (*entities.ResponseExample, error) {
	return r.get(ctx, id, false)
}

func (r *ResponseExampleRepo) get(ctx context.Context, id uuid.UUID, liveOnly bool) (*entities.ResponseExample, error) {
	const funcName = "ResponseExampleRepo.GetByID"

	query := `SELECT ` + responseExampleColumns + ` FROM response_examples WHERE id = ?`
	if liveOnly {
		query += ` AND is_delete = 0`
	}

	e, err := scanResponseExample(DBTXFromContext(ctx, r.db).QueryRowContext(ctx, query, id.String()))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("%s: %w", funcName, err)
	}

	return e, nil
}

// Does not check that the request or its collections are alive.
func (r *ResponseExampleRepo) ListByRequest(ctx context.Context, requestID uuid.UUID) ([]*entities.ResponseExample, error) {
	const funcName = "ResponseExampleRepo.ListByRequest"

	query := `SELECT ` + responseExampleColumns + ` FROM response_examples
		WHERE request_id = ? AND is_delete = 0
		ORDER BY sort_order ASC, created_at ASC`

	rows, err := DBTXFromContext(ctx, r.db).QueryContext(ctx, query, requestID.String())
	if err != nil {
		return nil, fmt.Errorf("%s: %w", funcName, err)
	}
	defer func() { _ = rows.Close() }()

	var result []*entities.ResponseExample
	for rows.Next() {
		e, err := scanResponseExample(rows)
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

// Update matches soft-deleted rows too and checks no version: the sync engine writes the server's copy
// through it, and the cascade runs inside the request's own write transaction.
func (r *ResponseExampleRepo) Update(ctx context.Context, e *entities.ResponseExample) error {
	const funcName = "ResponseExampleRepo.Update"

	if _, err := r.write(ctx, e, "WHERE id = ?", e.ID.String()); err != nil {
		return fmt.Errorf("%s: %w", funcName, err)
	}
	return nil
}

func (r *ResponseExampleRepo) UpdateAtVersion(ctx context.Context, e *entities.ResponseExample, baseVersion int) error {
	const funcName = "ResponseExampleRepo.UpdateAtVersion"

	n, err := r.write(ctx, e, "WHERE id = ? AND version = ? AND is_delete = 0", e.ID.String(), baseVersion)
	if err != nil {
		return fmt.Errorf("%s: %w", funcName, err)
	}
	if n == 0 {
		return &domain.ConflictError{Entity: "example", ID: e.ID.String()}
	}
	return nil
}

func (r *ResponseExampleRepo) write(ctx context.Context, e *entities.ResponseExample, where string, whereArgs ...any) (int64, error) {
	headersJSON, err := json.Marshal(headersToJSON(e.Headers))
	if err != nil {
		return 0, fmt.Errorf("marshal headers: %w", err)
	}

	query := `UPDATE response_examples SET request_id = ?, workspace_id = ?, name = ?, status_code = ?, status_text = ?,
		headers = ?, body = ?, content_type = ?, protocol = ?, sort_order = ?, version = ?, is_delete = ?, is_synced = 0,
		created_by = ?, created_at = ?, updated_by = ?, updated_at = ?
		` + where

	args := []any{
		e.RequestID.String(),
		e.WorkspaceID.String(),
		e.Name,
		e.StatusCode,
		e.StatusText,
		string(headersJSON),
		e.Body,
		e.ContentType,
		string(e.Protocol),
		e.SortOrder,
		e.Version,
		boolToInt(e.IsDelete),
		e.CreatedBy,
		e.CreatedAt.Format(time.RFC3339),
		e.UpdatedBy,
		e.UpdatedAt.Format(time.RFC3339),
	}
	res, err := DBTXFromContext(ctx, r.db).ExecContext(ctx, query, append(args, whereArgs...)...)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

func scanResponseExample(s scannable) (*entities.ResponseExample, error) {
	var (
		e           entities.ResponseExample
		idStr       string
		requestID   string
		workspaceID string
		headersJSON string
		protocol    string
		isDelete    int
		createdAt   string
		updatedAt   string
	)

	err := s.Scan(
		&idStr,
		&requestID,
		&workspaceID,
		&e.Name,
		&e.StatusCode,
		&e.StatusText,
		&headersJSON,
		&e.Body,
		&e.ContentType,
		&protocol,
		&e.SortOrder,
		&e.Version,
		&isDelete,
		&e.CreatedBy,
		&createdAt,
		&e.UpdatedBy,
		&updatedAt,
	)
	if err != nil {
		return nil, err
	}

	if e.ID, err = uuid.Parse(idStr); err != nil {
		return nil, fmt.Errorf("parse id: %w", err)
	}
	if e.RequestID, err = uuid.Parse(requestID); err != nil {
		return nil, fmt.Errorf("parse request_id: %w", err)
	}
	if e.WorkspaceID, err = uuid.Parse(workspaceID); err != nil {
		return nil, fmt.Errorf("parse workspace_id: %w", err)
	}
	e.Protocol = entities.Protocol(protocol)
	e.IsDelete = isDelete != 0

	e.Headers, err = unmarshalHeaders(headersJSON)
	if err != nil {
		return nil, fmt.Errorf("unmarshal headers: %w", err)
	}

	e.CreatedAt, err = parseTime(createdAt)
	if err != nil {
		return nil, fmt.Errorf("parse created_at: %w", err)
	}

	e.UpdatedAt, err = parseTime(updatedAt)
	if err != nil {
		return nil, fmt.Errorf("parse updated_at: %w", err)
	}

	return &e, nil
}
