package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/tetiva-app/client/internal/domain/entities"
	"github.com/tetiva-app/client/internal/domain/usecase/history"
)

// HistoryRepo implements both request.HistoryRepository (Create/GetByID)
// and history.Repository (List/Count/Delete/DeleteAll) using SQLite.
type HistoryRepo struct {
	db *sql.DB
}

// NewHistoryRepo creates a new HistoryRepo. The concrete return type lets it
// satisfy multiple usecase interfaces via fx bridges.
func NewHistoryRepo(db *sql.DB) *HistoryRepo {
	return &HistoryRepo{db: db}
}

// Create inserts a new history record into the database.
func (r *HistoryRepo) Create(ctx context.Context, h *entities.History) error {
	const funcName = "HistoryRepo.Create"

	reqHeaders, err := json.Marshal(h.RequestHeaders)
	if err != nil {
		return fmt.Errorf("%s: marshal request_headers: %w", funcName, err)
	}

	respHeaders, err := json.Marshal(h.ResponseHeaders)
	if err != nil {
		return fmt.Errorf("%s: marshal response_headers: %w", funcName, err)
	}

	query := `INSERT INTO history (id, request_id, workspace_id, protocol, method, url,
		request_headers, request_body, response_status, response_headers, response_body,
		response_size, duration_ms, error_message, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

	// request_id is nullable in schema (ON DELETE SET NULL). Persist a zero
	// UUID as NULL so the FK constraint accepts orphan history rows.
	var requestID any
	if h.RequestID != uuid.Nil {
		requestID = h.RequestID.String()
	}

	_, err = r.db.ExecContext(ctx, query,
		h.ID.String(),
		requestID,
		h.WorkspaceID.String(),
		string(h.Protocol),
		h.Method,
		h.URL,
		string(reqHeaders),
		h.RequestBody,
		h.ResponseStatus,
		string(respHeaders),
		h.ResponseBody,
		h.ResponseSize,
		h.DurationMs,
		h.ErrorMessage,
		h.CreatedAt.Format(time.RFC3339),
	)
	if err != nil {
		return fmt.Errorf("%s: %w", funcName, err)
	}

	return nil
}

// historyColumns is the canonical column list feeding scanHistoryRow / scanHistoryRowSingle.
const historyColumns = `id, request_id, workspace_id, protocol, method, url,
	request_headers, request_body, response_status, response_headers, response_body,
	response_size, duration_ms, error_message, created_at`

// GetByID retrieves a single history record by its ID. Returns (nil, nil)
// when the record does not exist.
func (r *HistoryRepo) GetByID(ctx context.Context, id uuid.UUID) (*entities.History, error) {
	const funcName = "HistoryRepo.GetByID"

	query := `SELECT ` + historyColumns + ` FROM history WHERE id = ?`

	row := r.db.QueryRowContext(ctx, query, id.String())
	h, err := scanHistoryRowSingle(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("%s: %w", funcName, err)
	}
	return h, nil
}

// List returns history rows matching the filter, ordered DESC by created_at.
func (r *HistoryRepo) List(ctx context.Context, f history.Filter) ([]*entities.History, error) {
	const funcName = "HistoryRepo.List"
	where, args := buildHistoryWhere(f)

	limit := f.Limit
	if limit <= 0 {
		limit = history.DefaultPageSize
	}

	query := `SELECT ` + historyColumns + ` FROM history ` + where + ` ORDER BY created_at DESC LIMIT ? OFFSET ?`
	args = append(args, limit, f.Offset)

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", funcName, err)
	}
	defer func() { _ = rows.Close() }()

	var out []*entities.History
	for rows.Next() {
		h, err := scanHistoryRow(rows)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", funcName, err)
		}
		out = append(out, h)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("%s: %w", funcName, err)
	}
	return out, nil
}

// Count returns the total number of history rows matching the filter (no
// limit/offset applied).
func (r *HistoryRepo) Count(ctx context.Context, f history.Filter) (int, error) {
	const funcName = "HistoryRepo.Count"
	where, args := buildHistoryWhere(f)
	var n int
	if err := r.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM history "+where, args...).Scan(&n); err != nil {
		return 0, fmt.Errorf("%s: %w", funcName, err)
	}
	return n, nil
}

// Delete removes a single history row by ID. Missing rows are not an error.
func (r *HistoryRepo) Delete(ctx context.Context, id uuid.UUID) error {
	const funcName = "HistoryRepo.Delete"
	if _, err := r.db.ExecContext(ctx, "DELETE FROM history WHERE id = ?", id.String()); err != nil {
		return fmt.Errorf("%s: %w", funcName, err)
	}
	return nil
}

// DeleteAll removes every history row belonging to the given workspace.
func (r *HistoryRepo) DeleteAll(ctx context.Context, workspaceID uuid.UUID) error {
	const funcName = "HistoryRepo.DeleteAll"
	if _, err := r.db.ExecContext(ctx, "DELETE FROM history WHERE workspace_id = ?", workspaceID.String()); err != nil {
		return fmt.Errorf("%s: %w", funcName, err)
	}
	return nil
}

// buildHistoryWhere returns a parameterized WHERE clause (with leading "WHERE ") and args.
// workspace_id is always the first clause, so queries are workspace-bounded by construction.
func buildHistoryWhere(f history.Filter) (string, []any) {
	var clauses []string
	var args []any

	clauses = append(clauses, "workspace_id = ?")
	args = append(args, f.WorkspaceID.String())

	if f.RequestID != nil && *f.RequestID != uuid.Nil {
		clauses = append(clauses, "request_id = ?")
		args = append(args, f.RequestID.String())
	}
	if len(f.Protocols) > 0 {
		placeholders := strings.TrimRight(strings.Repeat("?,", len(f.Protocols)), ",")
		clauses = append(clauses, "protocol IN ("+placeholders+")")
		for _, p := range f.Protocols {
			args = append(args, string(p))
		}
	}
	if len(f.StatusKinds) > 0 {
		var kindParts []string
		for _, k := range f.StatusKinds {
			switch k {
			case history.StatusKind2xx:
				kindParts = append(kindParts, "(response_status BETWEEN 200 AND 299 AND error_message = '')")
			case history.StatusKind3xx:
				kindParts = append(kindParts, "(response_status BETWEEN 300 AND 399 AND error_message = '')")
			case history.StatusKind4xx:
				kindParts = append(kindParts, "(response_status BETWEEN 400 AND 499 AND error_message = '')")
			case history.StatusKind5xx:
				kindParts = append(kindParts, "(response_status BETWEEN 500 AND 599 AND error_message = '')")
			case history.StatusKindError:
				kindParts = append(kindParts, "error_message != ''")
			}
		}
		if len(kindParts) > 0 {
			clauses = append(clauses, "("+strings.Join(kindParts, " OR ")+")")
		}
	}
	if f.URLContains != "" {
		clauses = append(clauses, "url LIKE ? COLLATE NOCASE")
		args = append(args, "%"+f.URLContains+"%")
	}
	return "WHERE " + strings.Join(clauses, " AND "), args
}

// rowScanner is the minimal interface satisfied by both *sql.Row and *sql.Rows.
type rowScanner interface {
	Scan(dest ...any) error
}

// scanHistoryFields defers UUID/JSON parsing to populateHistory. request_id is
// read into sql.NullString: the schema stores NULL, not zero-UUID, for orphan rows.
func scanHistoryFields(rs rowScanner, h *entities.History) (idStr, requestIDStr, workspaceIDStr, protocolStr, reqHeaders, respHeaders, createdAt string, err error) {
	var reqIDNS sql.NullString
	err = rs.Scan(
		&idStr, &reqIDNS, &workspaceIDStr, &protocolStr, &h.Method, &h.URL,
		&reqHeaders, &h.RequestBody, &h.ResponseStatus, &respHeaders, &h.ResponseBody,
		&h.ResponseSize, &h.DurationMs, &h.ErrorMessage, &createdAt,
	)
	if reqIDNS.Valid {
		requestIDStr = reqIDNS.String
	}
	return
}

// populateHistory parses id and workspace_id strictly; request_id is lenient
// because orphan history rows may carry an empty or zero UUID — a valid state.
func populateHistory(h *entities.History, idStr, requestIDStr, workspaceIDStr, protocolStr, reqHeaders, respHeaders, createdAt string) error {
	parsedID, err := uuid.Parse(idStr)
	if err != nil {
		return fmt.Errorf("parse id: %w", err)
	}
	h.ID = parsedID

	if rid, perr := uuid.Parse(requestIDStr); perr == nil {
		h.RequestID = rid
	}

	parsedWS, err := uuid.Parse(workspaceIDStr)
	if err != nil {
		return fmt.Errorf("parse workspace_id: %w", err)
	}
	h.WorkspaceID = parsedWS

	h.Protocol = entities.Protocol(protocolStr)

	if err := json.Unmarshal([]byte(reqHeaders), &h.RequestHeaders); err != nil {
		return fmt.Errorf("unmarshal request_headers: %w", err)
	}
	if err := json.Unmarshal([]byte(respHeaders), &h.ResponseHeaders); err != nil {
		return fmt.Errorf("unmarshal response_headers: %w", err)
	}

	parsedCreated, err := time.Parse(time.RFC3339, createdAt)
	if err != nil {
		return fmt.Errorf("parse created_at: %w", err)
	}
	h.CreatedAt = parsedCreated
	return nil
}

func scanHistoryRow(rows *sql.Rows) (*entities.History, error) {
	var h entities.History
	idStr, requestIDStr, workspaceIDStr, protocolStr, reqHeaders, respHeaders, createdAt, err := scanHistoryFields(rows, &h)
	if err != nil {
		return nil, err
	}
	if err := populateHistory(&h, idStr, requestIDStr, workspaceIDStr, protocolStr, reqHeaders, respHeaders, createdAt); err != nil {
		return nil, err
	}
	return &h, nil
}

// scanHistoryRowSingle scans a single-row *sql.Row result. Returns sql.ErrNoRows
// unwrapped so callers can use errors.Is.
func scanHistoryRowSingle(row *sql.Row) (*entities.History, error) {
	var h entities.History
	idStr, requestIDStr, workspaceIDStr, protocolStr, reqHeaders, respHeaders, createdAt, err := scanHistoryFields(row, &h)
	if err != nil {
		return nil, err
	}
	if err := populateHistory(&h, idStr, requestIDStr, workspaceIDStr, protocolStr, reqHeaders, respHeaders, createdAt); err != nil {
		return nil, err
	}
	return &h, nil
}
