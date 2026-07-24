package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/tetiva-app/client/internal/domain/entities"
	"github.com/tetiva-app/client/internal/domain/usecase/environment"
)

// EnvironmentRepo implements environment.Repository using SQLite.
type EnvironmentRepo struct {
	db *sql.DB
}

// NewEnvironmentRepo creates a new EnvironmentRepo instance.
func NewEnvironmentRepo(db *sql.DB) environment.Repository {
	return &EnvironmentRepo{db: db}
}

// Create inserts a new environment into the database.
func (r *EnvironmentRepo) Create(ctx context.Context, e *entities.Environment) error {
	const funcName = "EnvironmentRepo.Create"

	query := `INSERT INTO environments (id, workspace_id, name, is_active, version, is_delete, created_by, created_at, updated_by, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

	_, err := DBTXFromContext(ctx, r.db).ExecContext(ctx, query,
		e.ID.String(),
		e.WorkspaceID.String(),
		e.Name,
		boolToInt(e.IsActive),
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

// GetByID retrieves an environment by ID, returning nil if not found or soft-deleted.
func (r *EnvironmentRepo) GetByID(ctx context.Context, id uuid.UUID) (*entities.Environment, error) {
	const funcName = "EnvironmentRepo.GetByID"

	query := `SELECT id, workspace_id, name, is_active, version, is_delete, created_by, created_at, updated_by, updated_at
		FROM environments WHERE id = ? AND is_delete = 0`

	row := DBTXFromContext(ctx, r.db).QueryRowContext(ctx, query, id.String())

	e, err := scanEnvironment(row)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("%s: %w", funcName, err)
	}

	return e, nil
}

// List returns environments matching the given filter.
func (r *EnvironmentRepo) List(ctx context.Context, filter environment.Filter) ([]*entities.Environment, error) {
	const funcName = "EnvironmentRepo.List"

	query := `SELECT id, workspace_id, name, is_active, version, is_delete, created_by, created_at, updated_by, updated_at
		FROM environments WHERE workspace_id = ? AND is_delete = 0 ORDER BY created_at ASC`

	rows, err := DBTXFromContext(ctx, r.db).QueryContext(ctx, query, filter.WorkspaceID.String())
	if err != nil {
		return nil, fmt.Errorf("%s: %w", funcName, err)
	}
	defer func() { _ = rows.Close() }()

	var result []*entities.Environment
	for rows.Next() {
		e, err := scanEnvironment(rows)
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

// Update persists all fields matched by id only — last-write-wins, no version guard:
// the sync engine applies server changes; the UI enforces optimistic locking one layer up.
func (r *EnvironmentRepo) Update(ctx context.Context, e *entities.Environment) error {
	const funcName = "EnvironmentRepo.Update"

	query := `UPDATE environments SET workspace_id = ?, name = ?, is_active = ?, version = ?, is_delete = ?, created_by = ?, created_at = ?, updated_by = ?, updated_at = ?
		WHERE id = ?`

	_, err := DBTXFromContext(ctx, r.db).ExecContext(ctx, query,
		e.WorkspaceID.String(),
		e.Name,
		boolToInt(e.IsActive),
		e.Version,
		boolToInt(e.IsDelete),
		e.CreatedBy,
		e.CreatedAt.Format(time.RFC3339),
		e.UpdatedBy,
		e.UpdatedAt.Format(time.RFC3339),
		e.ID.String(),
	)
	if err != nil {
		return fmt.Errorf("%s: %w", funcName, err)
	}

	return nil
}

// GetActive returns the active environment for a workspace, or nil if none.
func (r *EnvironmentRepo) GetActive(ctx context.Context, workspaceID uuid.UUID) (*entities.Environment, error) {
	const funcName = "EnvironmentRepo.GetActive"

	query := `SELECT id, workspace_id, name, is_active, version, is_delete, created_by, created_at, updated_by, updated_at
		FROM environments WHERE workspace_id = ? AND is_active = 1 AND is_delete = 0 LIMIT 1`

	row := DBTXFromContext(ctx, r.db).QueryRowContext(ctx, query, workspaceID.String())

	e, err := scanEnvironment(row)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("%s: %w", funcName, err)
	}

	return e, nil
}

// SetActive deactivates all environments in a workspace and activates the given one.
func (r *EnvironmentRepo) SetActive(ctx context.Context, workspaceID uuid.UUID, environmentID uuid.UUID) error {
	const funcName = "EnvironmentRepo.SetActive"

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("%s: begin tx: %w", funcName, err)
	}
	defer func() { _ = tx.Rollback() }()

	_, err = tx.ExecContext(ctx, `UPDATE environments SET is_active = 0 WHERE workspace_id = ?`, workspaceID.String())
	if err != nil {
		return fmt.Errorf("%s: deactivate: %w", funcName, err)
	}

	_, err = tx.ExecContext(ctx, `UPDATE environments SET is_active = 1 WHERE id = ? AND workspace_id = ?`,
		environmentID.String(), workspaceID.String())
	if err != nil {
		return fmt.Errorf("%s: activate: %w", funcName, err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("%s: commit: %w", funcName, err)
	}

	return nil
}

func scanEnvironment(s scannable) (*entities.Environment, error) {
	var (
		e           entities.Environment
		idStr       string
		workspaceID string
		isActive    int
		isDelete    int
		createdAt   string
		updatedAt   string
	)

	err := s.Scan(
		&idStr,
		&workspaceID,
		&e.Name,
		&isActive,
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

	e.ID = uuid.MustParse(idStr)
	e.WorkspaceID = uuid.MustParse(workspaceID)
	e.IsActive = isActive != 0
	e.IsDelete = isDelete != 0

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
