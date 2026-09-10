package sync

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/tetiva-app/client/internal/domain/entities"
	"github.com/tetiva-app/client/internal/domain/usecase/collection"
	"github.com/tetiva-app/client/internal/infrastructure/repository/sqlite"
)

// SyncedCollectionRepo decorates collection.Repository to enqueue sync operations on writes.
type SyncedCollectionRepo struct {
	inner     collection.Repository
	syncQueue sqlite.SyncQueueRepository
	db        *sql.DB
	engine    *SyncEngine
}

func NewSyncedCollectionRepo(inner collection.Repository, syncQueue sqlite.SyncQueueRepository, db *sql.DB, engine *SyncEngine) *SyncedCollectionRepo {
	return &SyncedCollectionRepo{
		inner:     inner,
		syncQueue: syncQueue,
		db:        db,
		engine:    engine,
	}
}

func (r *SyncedCollectionRepo) isSyncEnabled(workspaceID string) bool {
	return r.engine != nil && r.engine.IsEnabledForWorkspace(workspaceID)
}

// Create inserts a collection and enqueues a sync "create" entry within the same TX.
func (r *SyncedCollectionRepo) Create(ctx context.Context, c *entities.Collection) error {
	wsID := c.WorkspaceID.String()
	if !r.isSyncEnabled(wsID) {
		return r.inner.Create(ctx, c)
	}
	err := sqlite.WithTx(ctx, r.db, func(txCtx context.Context) error {
		if err := r.inner.Create(txCtx, c); err != nil {
			return err
		}
		return r.syncQueue.Enqueue(txCtx, sqlite.SyncEntry{
			WorkspaceID: wsID,
			EntityType:  "collection",
			EntityID:    c.ID.String(),
			Action:      "create",
			OperationID: uuid.New().String(),
			Status:      "pending",
			CreatedAt:   time.Now(),
		})
	})
	if err == nil {
		r.engine.NotifyWrite(wsID)
	}
	return err
}

func (r *SyncedCollectionRepo) GetByID(ctx context.Context, id uuid.UUID) (*entities.Collection, error) {
	return r.inner.GetByID(ctx, id)
}

func (r *SyncedCollectionRepo) List(ctx context.Context, filter collection.Filter) ([]*entities.Collection, error) {
	return r.inner.List(ctx, filter)
}

// Update persists a collection and enqueues a sync "update" or "delete" entry within the same TX.
func (r *SyncedCollectionRepo) Update(ctx context.Context, c *entities.Collection) error {
	wsID := c.WorkspaceID.String()
	if !r.isSyncEnabled(wsID) {
		return r.inner.Update(ctx, c)
	}
	action := "update"
	if c.IsDelete {
		action = "delete"
	}
	err := sqlite.WithTx(ctx, r.db, func(txCtx context.Context) error {
		if err := r.inner.Update(txCtx, c); err != nil {
			return err
		}
		return r.syncQueue.Enqueue(txCtx, sqlite.SyncEntry{
			WorkspaceID: wsID,
			EntityType:  "collection",
			EntityID:    c.ID.String(),
			Action:      action,
			OperationID: uuid.New().String(),
			Status:      "pending",
			CreatedAt:   time.Now(),
		})
	})
	if err == nil {
		r.engine.NotifyWrite(wsID)
	}
	return err
}

// UpdateSortOrder is not sync-relevant, so nothing is enqueued.
func (r *SyncedCollectionRepo) UpdateSortOrder(ctx context.Context, id uuid.UUID, sortOrder int) error {
	return r.inner.UpdateSortOrder(ctx, id, sortOrder)
}

// SoftDeleteDescendants enqueues a "delete" entry per descendant in a sync-enabled workspace.
func (r *SyncedCollectionRepo) SoftDeleteDescendants(ctx context.Context, parentID uuid.UUID, updatedBy string, updatedAt time.Time) error {
	var notifyWorkspaces []string
	err := sqlite.WithTx(ctx, r.db, func(txCtx context.Context) error {
		descendants, err := r.selectDescendants(txCtx, parentID)
		if err != nil {
			return err
		}

		if err := r.inner.SoftDeleteDescendants(txCtx, parentID, updatedBy, updatedAt); err != nil {
			return err
		}

		for _, d := range descendants {
			if !r.isSyncEnabled(d.workspaceID) {
				continue
			}
			if err := r.syncQueue.Enqueue(txCtx, sqlite.SyncEntry{
				WorkspaceID: d.workspaceID,
				EntityType:  "collection",
				EntityID:    d.id,
				Action:      "delete",
				OperationID: uuid.New().String(),
				Status:      "pending",
				CreatedAt:   time.Now(),
			}); err != nil {
				return err
			}
			notifyWorkspaces = append(notifyWorkspaces, d.workspaceID)
		}
		return nil
	})
	if err == nil {
		seen := make(map[string]struct{}, len(notifyWorkspaces))
		for _, wsID := range notifyWorkspaces {
			if _, ok := seen[wsID]; !ok {
				seen[wsID] = struct{}{}
				r.engine.NotifyWrite(wsID)
			}
		}
	}
	return err
}

type descendantRow struct {
	id          string
	workspaceID string
}

func (r *SyncedCollectionRepo) selectDescendants(ctx context.Context, parentID uuid.UUID) ([]descendantRow, error) {
	const funcName = "SyncedCollectionRepo.selectDescendants"

	query := `WITH RECURSIVE descendants AS (
		SELECT id, workspace_id FROM collections WHERE parent_id = ? AND is_delete = 0
		UNION ALL
		SELECT c.id, c.workspace_id FROM collections c
		INNER JOIN descendants d ON c.parent_id = d.id
		WHERE c.is_delete = 0
	) SELECT id, workspace_id FROM descendants`

	rows, err := sqlite.DBTXFromContext(ctx, r.db).QueryContext(ctx, query, parentID.String())
	if err != nil {
		return nil, fmt.Errorf("%s: %w", funcName, err)
	}
	defer func() { _ = rows.Close() }()

	var result []descendantRow
	for rows.Next() {
		var d descendantRow
		if err := rows.Scan(&d.id, &d.workspaceID); err != nil {
			return nil, fmt.Errorf("%s: scan: %w", funcName, err)
		}
		result = append(result, d)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("%s: rows: %w", funcName, err)
	}

	return result, nil
}
