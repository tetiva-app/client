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

// VariableRepo implements environment.VariableRepository using SQLite.
type VariableRepo struct {
	db *sql.DB
}

// NewVariableRepo creates a new VariableRepo instance.
func NewVariableRepo(db *sql.DB) environment.VariableRepository {
	return &VariableRepo{db: db}
}

// Create inserts a new variable into the database.
func (r *VariableRepo) Create(ctx context.Context, v *entities.Variable) error {
	const funcName = "VariableRepo.Create"

	query := `INSERT INTO variables (id, environment_id, key, value, is_secret, enabled, sort_order, version, is_delete, created_by, created_at, updated_by, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

	_, err := DBTXFromContext(ctx, r.db).ExecContext(ctx, query,
		v.ID.String(),
		v.EnvironmentID.String(),
		v.Key,
		v.Value,
		boolToInt(v.IsSecret),
		boolToInt(v.Enabled),
		v.SortOrder,
		v.Version,
		boolToInt(v.IsDelete),
		v.CreatedBy,
		v.CreatedAt.Format(time.RFC3339),
		v.UpdatedBy,
		v.UpdatedAt.Format(time.RFC3339),
	)
	if err != nil {
		return fmt.Errorf("%s: %w", funcName, err)
	}

	return nil
}

// GetByID retrieves a variable by ID.
func (r *VariableRepo) GetByID(ctx context.Context, id uuid.UUID) (*entities.Variable, error) {
	const funcName = "VariableRepo.GetByID"

	query := `SELECT id, environment_id, key, value, is_secret, enabled, sort_order, version, is_delete, created_by, created_at, updated_by, updated_at
		FROM variables WHERE id = ? AND is_delete = 0`

	row := DBTXFromContext(ctx, r.db).QueryRowContext(ctx, query, id.String())

	v, err := scanVariable(row)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("%s: %w", funcName, err)
	}

	return v, nil
}

// List returns all non-deleted variables for an environment, ordered by sort_order.
func (r *VariableRepo) List(ctx context.Context, environmentID uuid.UUID) ([]*entities.Variable, error) {
	const funcName = "VariableRepo.List"

	query := `SELECT id, environment_id, key, value, is_secret, enabled, sort_order, version, is_delete, created_by, created_at, updated_by, updated_at
		FROM variables WHERE environment_id = ? AND is_delete = 0 ORDER BY sort_order ASC, created_at ASC`

	rows, err := DBTXFromContext(ctx, r.db).QueryContext(ctx, query, environmentID.String())
	if err != nil {
		return nil, fmt.Errorf("%s: %w", funcName, err)
	}
	defer func() { _ = rows.Close() }()

	var result []*entities.Variable
	for rows.Next() {
		v, err := scanVariable(rows)
		if err != nil {
			return nil, fmt.Errorf("%s: scan: %w", funcName, err)
		}
		result = append(result, v)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("%s: rows: %w", funcName, err)
	}

	return result, nil
}

// Update persists all fields of an existing variable.
func (r *VariableRepo) Update(ctx context.Context, v *entities.Variable) error {
	const funcName = "VariableRepo.Update"

	query := `UPDATE variables SET environment_id = ?, key = ?, value = ?, is_secret = ?, enabled = ?, sort_order = ?, version = ?, is_delete = ?, created_by = ?, created_at = ?, updated_by = ?, updated_at = ?
		WHERE id = ?`

	_, err := DBTXFromContext(ctx, r.db).ExecContext(ctx, query,
		v.EnvironmentID.String(),
		v.Key,
		v.Value,
		boolToInt(v.IsSecret),
		boolToInt(v.Enabled),
		v.SortOrder,
		v.Version,
		boolToInt(v.IsDelete),
		v.CreatedBy,
		v.CreatedAt.Format(time.RFC3339),
		v.UpdatedBy,
		v.UpdatedAt.Format(time.RFC3339),
		v.ID.String(),
	)
	if err != nil {
		return fmt.Errorf("%s: %w", funcName, err)
	}

	return nil
}

// Delete hard-deletes a variable by ID.
func (r *VariableRepo) Delete(ctx context.Context, id uuid.UUID) error {
	const funcName = "VariableRepo.Delete"

	_, err := DBTXFromContext(ctx, r.db).ExecContext(ctx, `DELETE FROM variables WHERE id = ?`, id.String())
	if err != nil {
		return fmt.Errorf("%s: %w", funcName, err)
	}

	return nil
}

func scanVariable(s scannable) (*entities.Variable, error) {
	var (
		v             entities.Variable
		idStr         string
		environmentID string
		isSecret      int
		enabled       int
		isDelete      int
		createdAt     string
		updatedAt     string
	)

	err := s.Scan(
		&idStr,
		&environmentID,
		&v.Key,
		&v.Value,
		&isSecret,
		&enabled,
		&v.SortOrder,
		&v.Version,
		&isDelete,
		&v.CreatedBy,
		&createdAt,
		&v.UpdatedBy,
		&updatedAt,
	)
	if err != nil {
		return nil, err
	}

	v.ID = uuid.MustParse(idStr)
	v.EnvironmentID = uuid.MustParse(environmentID)
	v.IsSecret = isSecret != 0
	v.Enabled = enabled != 0
	v.IsDelete = isDelete != 0

	v.CreatedAt, err = parseTime(createdAt)
	if err != nil {
		return nil, fmt.Errorf("parse created_at: %w", err)
	}

	v.UpdatedAt, err = parseTime(updatedAt)
	if err != nil {
		return nil, fmt.Errorf("parse updated_at: %w", err)
	}

	return &v, nil
}
