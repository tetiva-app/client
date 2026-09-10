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

// Implements both request.HistoryRepository and history.Repository.
type HistoryRepo struct {
	db *sql.DB
}

// The concrete return type lets it satisfy multiple usecase interfaces via fx bridges.
func NewHistoryRepo(db *sql.DB) *HistoryRepo {
	return &HistoryRepo{db: db}
}

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

	authQueryKeys := h.AuthQueryKeys
	if authQueryKeys == nil {
		authQueryKeys = []string{}
	}
	queryKeys, err := json.Marshal(authQueryKeys)
	if err != nil {
		return fmt.Errorf("%s: marshal auth_query_keys: %w", funcName, err)
	}

	query := `INSERT INTO history (id, request_id, workspace_id, protocol, method, url,
		request_headers, request_body, response_status, response_headers, response_body,
		response_size, duration_ms, error_message, created_at, auth_query_keys)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

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
		string(queryKeys),
	)
	if err != nil {
		return fmt.Errorf("%s: %w", funcName, err)
	}

	return nil
}

const historyColumns = `id, request_id, workspace_id, protocol, method, url,
	request_headers, request_body, response_status, response_headers, response_body,
	response_size, duration_ms, error_message, created_at, auth_query_keys`

// Returns (nil, nil) when the record does not exist.
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

// Ordered DESC by created_at.
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

// The filter's limit/offset are ignored.
func (r *HistoryRepo) Count(ctx context.Context, f history.Filter) (int, error) {
	const funcName = "HistoryRepo.Count"
	where, args := buildHistoryWhere(f)
	var n int
	if err := r.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM history "+where, args...).Scan(&n); err != nil {
		return 0, fmt.Errorf("%s: %w", funcName, err)
	}
	return n, nil
}

// Missing rows are not an error.
func (r *HistoryRepo) Delete(ctx context.Context, id uuid.UUID) error {
	const funcName = "HistoryRepo.Delete"
	if _, err := r.db.ExecContext(ctx, "DELETE FROM history WHERE id = ?", id.String()); err != nil {
		return fmt.Errorf("%s: %w", funcName, err)
	}
	return nil
}

func (r *HistoryRepo) DeleteAll(ctx context.Context, workspaceID uuid.UUID) error {
	const funcName = "HistoryRepo.DeleteAll"
	if _, err := r.db.ExecContext(ctx, "DELETE FROM history WHERE workspace_id = ?", workspaceID.String()); err != nil {
		return fmt.Errorf("%s: %w", funcName, err)
	}
	return nil
}

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
				// 101 is the success status of a WebSocket handshake, it has no range of its own.
				kindParts = append(kindParts, "((response_status BETWEEN 200 AND 299 OR response_status = 101) AND error_message = '')")
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

type rowScanner interface {
	Scan(dest ...any) error
}

// UUID/JSON parsing is deferred to populateHistory. request_id reads into sql.NullString:
// the schema stores NULL, not zero-UUID, for orphan rows.
func scanHistoryFields(rs rowScanner, h *entities.History) (idStr, requestIDStr, workspaceIDStr, protocolStr, reqHeaders, respHeaders, createdAt, authQueryKeys string, err error) {
	var reqIDNS sql.NullString
	err = rs.Scan(
		&idStr, &reqIDNS, &workspaceIDStr, &protocolStr, &h.Method, &h.URL,
		&reqHeaders, &h.RequestBody, &h.ResponseStatus, &respHeaders, &h.ResponseBody,
		&h.ResponseSize, &h.DurationMs, &h.ErrorMessage, &createdAt, &authQueryKeys,
	)
	if reqIDNS.Valid {
		requestIDStr = reqIDNS.String
	}
	return
}

// id and workspace_id are parsed strictly; request_id is lenient because orphan
// history rows may carry an empty or zero UUID — a valid state.
func populateHistory(h *entities.History, idStr, requestIDStr, workspaceIDStr, protocolStr, reqHeaders, respHeaders, createdAt, authQueryKeys string) error {
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
	if err := json.Unmarshal([]byte(authQueryKeys), &h.AuthQueryKeys); err != nil {
		return fmt.Errorf("unmarshal auth_query_keys: %w", err)
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
	idStr, requestIDStr, workspaceIDStr, protocolStr, reqHeaders, respHeaders, createdAt, authQueryKeys, err := scanHistoryFields(rows, &h)
	if err != nil {
		return nil, err
	}
	if err := populateHistory(&h, idStr, requestIDStr, workspaceIDStr, protocolStr, reqHeaders, respHeaders, createdAt, authQueryKeys); err != nil {
		return nil, err
	}
	return &h, nil
}

// Returns sql.ErrNoRows unwrapped so callers can use errors.Is.
func scanHistoryRowSingle(row *sql.Row) (*entities.History, error) {
	var h entities.History
	idStr, requestIDStr, workspaceIDStr, protocolStr, reqHeaders, respHeaders, createdAt, authQueryKeys, err := scanHistoryFields(row, &h)
	if err != nil {
		return nil, err
	}
	if err := populateHistory(&h, idStr, requestIDStr, workspaceIDStr, protocolStr, reqHeaders, respHeaders, createdAt, authQueryKeys); err != nil {
		return nil, err
	}
	return &h, nil
}
