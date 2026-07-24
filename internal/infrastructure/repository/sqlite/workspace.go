package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/tetiva-app/client/internal/domain/entities"
	"github.com/tetiva-app/client/internal/domain/usecase/workspace"
)

// WorkspaceRepo implements workspace.Repository using SQLite.
type WorkspaceRepo struct {
	db *sql.DB
}

// NewWorkspaceRepo creates a new WorkspaceRepo instance.
func NewWorkspaceRepo(db *sql.DB) workspace.Repository {
	return &WorkspaceRepo{db: db}
}

// Create inserts a new workspace into the database.
func (r *WorkspaceRepo) Create(ctx context.Context, w *entities.Workspace) error {
	const funcName = "WorkspaceRepo.Create"

	query := `INSERT INTO workspaces (id, name, remote_workspace_id, is_active, version, is_delete, created_by, created_at, updated_by, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

	_, err := DBTXFromContext(ctx, r.db).ExecContext(ctx, query,
		w.ID.String(),
		w.Name,
		w.RemoteWorkspaceID,
		boolToInt(w.IsActive),
		w.Version,
		boolToInt(w.IsDelete),
		w.CreatedBy,
		w.CreatedAt.Format(time.RFC3339),
		w.UpdatedBy,
		w.UpdatedAt.Format(time.RFC3339),
	)
	if err != nil {
		return fmt.Errorf("%s: %w", funcName, err)
	}

	return nil
}

// GetByID retrieves a workspace by ID, returning nil if not found or soft-deleted.
func (r *WorkspaceRepo) GetByID(ctx context.Context, id uuid.UUID) (*entities.Workspace, error) {
	const funcName = "WorkspaceRepo.GetByID"

	query := `SELECT id, name, remote_workspace_id, is_active, version, is_delete, created_by, created_at, updated_by, updated_at
		FROM workspaces WHERE id = ? AND is_delete = 0`

	row := DBTXFromContext(ctx, r.db).QueryRowContext(ctx, query, id.String())

	w, err := scanWorkspace(row)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("%s: %w", funcName, err)
	}

	return w, nil
}

// List returns all non-deleted workspaces ordered by created_at ASC.
func (r *WorkspaceRepo) List(ctx context.Context) ([]*entities.Workspace, error) {
	const funcName = "WorkspaceRepo.List"

	query := `SELECT id, name, remote_workspace_id, is_active, version, is_delete, created_by, created_at, updated_by, updated_at
		FROM workspaces WHERE is_delete = 0 ORDER BY created_at ASC`

	rows, err := DBTXFromContext(ctx, r.db).QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", funcName, err)
	}
	defer func() { _ = rows.Close() }()

	var result []*entities.Workspace
	for rows.Next() {
		w, err := scanWorkspace(rows)
		if err != nil {
			return nil, fmt.Errorf("%s: scan: %w", funcName, err)
		}
		result = append(result, w)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("%s: rows: %w", funcName, err)
	}

	return result, nil
}

// Update persists all fields matched by id only — last-write-wins, no version guard:
// the sync engine applies server changes; the UI enforces optimistic locking one layer up.
func (r *WorkspaceRepo) Update(ctx context.Context, w *entities.Workspace) error {
	const funcName = "WorkspaceRepo.Update"

	query := `UPDATE workspaces SET name = ?, is_active = ?, version = ?, is_delete = ?, updated_by = ?, updated_at = ?
		WHERE id = ?`

	_, err := DBTXFromContext(ctx, r.db).ExecContext(ctx, query,
		w.Name,
		boolToInt(w.IsActive),
		w.Version,
		boolToInt(w.IsDelete),
		w.UpdatedBy,
		w.UpdatedAt.Format(time.RFC3339),
		w.ID.String(),
	)
	if err != nil {
		return fmt.Errorf("%s: %w", funcName, err)
	}

	return nil
}

// GetActive returns the currently active workspace, or nil if none is active.
func (r *WorkspaceRepo) GetActive(ctx context.Context) (*entities.Workspace, error) {
	const funcName = "WorkspaceRepo.GetActive"

	query := `SELECT id, name, remote_workspace_id, is_active, version, is_delete, created_by, created_at, updated_by, updated_at
		FROM workspaces WHERE is_active = 1 AND is_delete = 0`

	row := DBTXFromContext(ctx, r.db).QueryRowContext(ctx, query)

	w, err := scanWorkspace(row)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("%s: %w", funcName, err)
	}

	return w, nil
}

// SetActive deactivates all workspaces and activates the target one within a transaction.
func (r *WorkspaceRepo) SetActive(ctx context.Context, id uuid.UUID) error {
	const funcName = "WorkspaceRepo.SetActive"

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("%s: begin tx: %w", funcName, err)
	}
	defer func() { _ = tx.Rollback() }()

	_, err = tx.ExecContext(ctx, `UPDATE workspaces SET is_active = 0 WHERE is_delete = 0`)
	if err != nil {
		return fmt.Errorf("%s: deactivate all: %w", funcName, err)
	}

	res, err := tx.ExecContext(ctx, `UPDATE workspaces SET is_active = 1 WHERE id = ? AND is_delete = 0`, id.String())
	if err != nil {
		return fmt.Errorf("%s: activate target: %w", funcName, err)
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("%s: rows affected: %w", funcName, err)
	}
	if affected == 0 {
		return fmt.Errorf("%s: workspace not found or deleted", funcName)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("%s: commit: %w", funcName, err)
	}

	return nil
}

// CountNonDeleted returns the number of non-deleted workspaces.
func (r *WorkspaceRepo) CountNonDeleted(ctx context.Context) (int, error) {
	const funcName = "WorkspaceRepo.CountNonDeleted"

	var count int
	err := DBTXFromContext(ctx, r.db).QueryRowContext(ctx, `SELECT COUNT(*) FROM workspaces WHERE is_delete = 0`).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("%s: %w", funcName, err)
	}

	return count, nil
}

// GetByRemoteID retrieves a workspace by its remote workspace ID.
func (r *WorkspaceRepo) GetByRemoteID(ctx context.Context, remoteID string) (*entities.Workspace, error) {
	const funcName = "WorkspaceRepo.GetByRemoteID"

	query := `SELECT id, name, remote_workspace_id, is_active, version, is_delete, created_by, created_at, updated_by, updated_at
		FROM workspaces WHERE remote_workspace_id = ? AND is_delete = 0`

	row := DBTXFromContext(ctx, r.db).QueryRowContext(ctx, query, remoteID)

	w, err := scanWorkspace(row)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("%s: %w", funcName, err)
	}

	return w, nil
}

func scanWorkspace(s scannable) (*entities.Workspace, error) {
	var (
		w                 entities.Workspace
		idStr             string
		remoteWorkspaceID *string
		isActive          int
		isDelete          int
		createdAt         string
		updatedAt         string
	)

	err := s.Scan(
		&idStr,
		&w.Name,
		&remoteWorkspaceID,
		&isActive,
		&w.Version,
		&isDelete,
		&w.CreatedBy,
		&createdAt,
		&w.UpdatedBy,
		&updatedAt,
	)
	if err != nil {
		return nil, err
	}

	w.ID = uuid.MustParse(idStr)
	w.RemoteWorkspaceID = remoteWorkspaceID
	w.IsActive = isActive != 0
	w.IsDelete = isDelete != 0

	w.CreatedAt, err = parseTime(createdAt)
	if err != nil {
		return nil, fmt.Errorf("parse created_at: %w", err)
	}

	w.UpdatedAt, err = parseTime(updatedAt)
	if err != nil {
		return nil, fmt.Errorf("parse updated_at: %w", err)
	}

	return &w, nil
}
