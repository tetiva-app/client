package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/tetiva-app/client/internal/domain/entities"
	"github.com/tetiva-app/client/internal/domain/usecase/collection"
)

type scannable interface {
	Scan(dest ...any) error
}

type CollectionRepo struct {
	db *sql.DB
}

func NewCollectionRepo(db *sql.DB) collection.Repository {
	return &CollectionRepo{db: db}
}

func (r *CollectionRepo) Create(ctx context.Context, c *entities.Collection) error {
	const funcName = "CollectionRepo.Create"

	grpcMetadata, err := json.Marshal(c.GRPCMetadata)
	if err != nil {
		return fmt.Errorf("%s: marshal grpc_metadata: %w", funcName, err)
	}

	query := `INSERT INTO collections (id, workspace_id, parent_id, name, pre_script, post_script, description, auth_type, auth_data, grpc_metadata, sort_order, version, is_delete, created_by, created_at, updated_by, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

	_, err = DBTXFromContext(ctx, r.db).ExecContext(ctx, query,
		c.ID.String(),
		c.WorkspaceID.String(),
		uuidPtrToString(c.ParentID),
		c.Name,
		c.PreScript,
		c.PostScript,
		c.Description,
		string(c.AuthType),
		c.AuthData,
		string(grpcMetadata),
		c.SortOrder,
		c.Version,
		boolToInt(c.IsDelete),
		c.CreatedBy,
		c.CreatedAt.Format(time.RFC3339),
		c.UpdatedBy,
		c.UpdatedAt.Format(time.RFC3339),
	)
	if err != nil {
		return fmt.Errorf("%s: %w", funcName, err)
	}

	return nil
}

// Returns nil when the row is missing or soft-deleted.
func (r *CollectionRepo) GetByID(ctx context.Context, id uuid.UUID) (*entities.Collection, error) {
	const funcName = "CollectionRepo.GetByID"

	query := `SELECT id, workspace_id, parent_id, name, pre_script, post_script, description, auth_type, auth_data, grpc_metadata, sort_order, version, is_delete, created_by, created_at, updated_by, updated_at
		FROM collections WHERE id = ? AND is_delete = 0`

	row := DBTXFromContext(ctx, r.db).QueryRowContext(ctx, query, id.String())

	c, err := scanCollection(row)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("%s: %w", funcName, err)
	}

	return c, nil
}

// Ordered by sort_order ASC, created_at ASC.
func (r *CollectionRepo) List(ctx context.Context, filter collection.Filter) ([]*entities.Collection, error) {
	const funcName = "CollectionRepo.List"

	query := `SELECT id, workspace_id, parent_id, name, pre_script, post_script, description, auth_type, auth_data, grpc_metadata, sort_order, version, is_delete, created_by, created_at, updated_by, updated_at
		FROM collections WHERE workspace_id = ? AND is_delete = 0`
	args := []any{filter.WorkspaceID.String()}

	if filter.ParentID != nil {
		query += " AND parent_id = ?"
		args = append(args, filter.ParentID.String())
	}

	query += " ORDER BY sort_order ASC, created_at ASC"

	rows, err := DBTXFromContext(ctx, r.db).QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", funcName, err)
	}
	defer func() { _ = rows.Close() }()

	var result []*entities.Collection
	for rows.Next() {
		c, err := scanCollection(rows)
		if err != nil {
			return nil, fmt.Errorf("%s: scan: %w", funcName, err)
		}
		result = append(result, c)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("%s: rows: %w", funcName, err)
	}

	return result, nil
}

// No version guard — sync applies server changes; the UI does optimistic locking one layer up.
func (r *CollectionRepo) Update(ctx context.Context, c *entities.Collection) error {
	const funcName = "CollectionRepo.Update"

	grpcMetadata, err := json.Marshal(c.GRPCMetadata)
	if err != nil {
		return fmt.Errorf("%s: marshal grpc_metadata: %w", funcName, err)
	}

	query := `UPDATE collections SET workspace_id = ?, parent_id = ?, name = ?, pre_script = ?, post_script = ?, description = ?, auth_type = ?, auth_data = ?, grpc_metadata = ?, sort_order = ?, version = ?, is_delete = ?, created_by = ?, created_at = ?, updated_by = ?, updated_at = ?
		WHERE id = ?`

	_, err = DBTXFromContext(ctx, r.db).ExecContext(ctx, query,
		c.WorkspaceID.String(),
		uuidPtrToString(c.ParentID),
		c.Name,
		c.PreScript,
		c.PostScript,
		c.Description,
		string(c.AuthType),
		c.AuthData,
		string(grpcMetadata),
		c.SortOrder,
		c.Version,
		boolToInt(c.IsDelete),
		c.CreatedBy,
		c.CreatedAt.Format(time.RFC3339),
		c.UpdatedBy,
		c.UpdatedAt.Format(time.RFC3339),
		c.ID.String(),
	)
	if err != nil {
		return fmt.Errorf("%s: %w", funcName, err)
	}

	return nil
}

func (r *CollectionRepo) UpdateSortOrder(ctx context.Context, id uuid.UUID, sortOrder int) error {
	const funcName = "CollectionRepo.UpdateSortOrder"

	query := `UPDATE collections SET sort_order = ? WHERE id = ?`

	_, err := DBTXFromContext(ctx, r.db).ExecContext(ctx, query, sortOrder, id.String())
	if err != nil {
		return fmt.Errorf("%s: %w", funcName, err)
	}

	return nil
}

func (r *CollectionRepo) SoftDeleteDescendants(ctx context.Context, parentID uuid.UUID, updatedBy string, updatedAt time.Time) error {
	const funcName = "CollectionRepo.SoftDeleteDescendants"

	query := `
		WITH RECURSIVE descendants AS (
			SELECT id FROM collections WHERE parent_id = ? AND is_delete = 0
			UNION ALL
			SELECT c.id FROM collections c INNER JOIN descendants d ON c.parent_id = d.id WHERE c.is_delete = 0
		)
		UPDATE collections
		SET is_delete = 1, version = version + 1, updated_by = ?, updated_at = ?
		WHERE id IN (SELECT id FROM descendants)`

	_, err := DBTXFromContext(ctx, r.db).ExecContext(ctx, query, parentID.String(), updatedBy, updatedAt.Format(time.RFC3339))
	if err != nil {
		return fmt.Errorf("%s: %w", funcName, err)
	}

	return nil
}

func scanCollection(s scannable) (*entities.Collection, error) {
	var (
		c            entities.Collection
		idStr        string
		workspaceID  string
		parentID     sql.NullString
		authType     string
		grpcMetadata string
		isDelete     int
		createdAt    string
		updatedAt    string
	)

	err := s.Scan(
		&idStr,
		&workspaceID,
		&parentID,
		&c.Name,
		&c.PreScript,
		&c.PostScript,
		&c.Description,
		&authType,
		&c.AuthData,
		&grpcMetadata,
		&c.SortOrder,
		&c.Version,
		&isDelete,
		&c.CreatedBy,
		&createdAt,
		&c.UpdatedBy,
		&updatedAt,
	)
	if err != nil {
		return nil, err
	}

	c.ID = uuid.MustParse(idStr)
	c.WorkspaceID = uuid.MustParse(workspaceID)
	c.AuthType = entities.AuthType(authType)

	if grpcMetadata != "" && grpcMetadata != "[]" {
		if err := json.Unmarshal([]byte(grpcMetadata), &c.GRPCMetadata); err != nil {
			return nil, fmt.Errorf("parse grpc_metadata: %w", err)
		}
	}

	if parentID.Valid {
		pid := uuid.MustParse(parentID.String)
		c.ParentID = &pid
	}

	c.IsDelete = isDelete != 0

	c.CreatedAt, err = parseTime(createdAt)
	if err != nil {
		return nil, fmt.Errorf("parse created_at: %w", err)
	}

	c.UpdatedAt, err = parseTime(updatedAt)
	if err != nil {
		return nil, fmt.Errorf("parse updated_at: %w", err)
	}

	return &c, nil
}

func parseTime(s string) (time.Time, error) {
	t, err := time.Parse(time.RFC3339, s)
	if err == nil {
		return t, nil
	}
	// SQLite datetime('now') produces "2006-01-02 15:04:05"
	return time.Parse("2006-01-02 15:04:05", s)
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

// A nil pointer becomes SQL NULL.
func uuidPtrToString(id *uuid.UUID) any {
	if id == nil {
		return nil
	}
	return id.String()
}
