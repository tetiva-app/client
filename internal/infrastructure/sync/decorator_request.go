package sync

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/tetiva-app/client/internal/domain/entities"
	"github.com/tetiva-app/client/internal/domain/usecase/request"
	"github.com/tetiva-app/client/internal/infrastructure/repository/sqlite"
)

// SyncedRequestRepo decorates request.Repository to enqueue sync operations on writes.
type SyncedRequestRepo struct {
	inner     request.Repository
	syncQueue sqlite.SyncQueueRepository
	db        *sql.DB
	engine    *SyncEngine
}

// NewSyncedRequestRepo creates a new SyncedRequestRepo.
func NewSyncedRequestRepo(inner request.Repository, syncQueue sqlite.SyncQueueRepository, db *sql.DB, engine *SyncEngine) *SyncedRequestRepo {
	return &SyncedRequestRepo{
		inner:     inner,
		syncQueue: syncQueue,
		db:        db,
		engine:    engine,
	}
}

func (r *SyncedRequestRepo) isSyncEnabled(workspaceID string) bool {
	return r.engine != nil && r.engine.IsEnabledForWorkspace(workspaceID)
}

// Create inserts a request and enqueues a sync "create" entry within the same TX.
func (r *SyncedRequestRepo) Create(ctx context.Context, req *entities.Request) error {
	wsID, err := r.getWorkspaceID(ctx, req.CollectionID)
	if err != nil || !r.isSyncEnabled(wsID) {
		return r.inner.Create(ctx, req)
	}
	txErr := sqlite.WithTx(ctx, r.db, func(txCtx context.Context) error {
		if err := r.inner.Create(txCtx, req); err != nil {
			return err
		}
		return r.syncQueue.Enqueue(txCtx, sqlite.SyncEntry{
			WorkspaceID: wsID,
			EntityType:  "request",
			EntityID:    req.ID.String(),
			Action:      "create",
			OperationID: uuid.New().String(),
			Status:      "pending",
			CreatedAt:   time.Now(),
		})
	})
	if txErr == nil {
		r.engine.NotifyWrite(wsID)
	}
	return txErr
}

// GetByID delegates to the inner repository.
func (r *SyncedRequestRepo) GetByID(ctx context.Context, id uuid.UUID) (*entities.Request, error) {
	return r.inner.GetByID(ctx, id)
}

// List delegates to the inner repository.
func (r *SyncedRequestRepo) List(ctx context.Context, filter request.Filter) ([]*entities.Request, error) {
	return r.inner.List(ctx, filter)
}

// Update persists a request and enqueues a sync "update" or "delete" entry within the same TX.
func (r *SyncedRequestRepo) Update(ctx context.Context, req *entities.Request) error {
	wsID, err := r.getWorkspaceID(ctx, req.CollectionID)
	if err != nil || !r.isSyncEnabled(wsID) {
		return r.inner.Update(ctx, req)
	}
	action := "update"
	if req.IsDelete {
		action = "delete"
	}
	txErr := sqlite.WithTx(ctx, r.db, func(txCtx context.Context) error {
		if err := r.inner.Update(txCtx, req); err != nil {
			return err
		}
		return r.syncQueue.Enqueue(txCtx, sqlite.SyncEntry{
			WorkspaceID: wsID,
			EntityType:  "request",
			EntityID:    req.ID.String(),
			Action:      action,
			OperationID: uuid.New().String(),
			Status:      "pending",
			CreatedAt:   time.Now(),
		})
	})
	if txErr == nil {
		r.engine.NotifyWrite(wsID)
	}
	return txErr
}

// UpdateSortOrder delegates to the inner repository (not a sync-relevant operation).
func (r *SyncedRequestRepo) UpdateSortOrder(ctx context.Context, id uuid.UUID, sortOrder int) error {
	return r.inner.UpdateSortOrder(ctx, id, sortOrder)
}

// DeleteHard delegates to the inner repository. Drafts are local-only and bypass sync queue.
func (r *SyncedRequestRepo) DeleteHard(ctx context.Context, id uuid.UUID) error {
	return r.inner.DeleteHard(ctx, id)
}

// CleanupDrafts delegates to the inner repository. Drafts are local-only and bypass sync queue.
func (r *SyncedRequestRepo) CleanupDrafts(ctx context.Context) (int, error) {
	return r.inner.CleanupDrafts(ctx)
}

// getWorkspaceID looks up the workspace_id of the collection that owns this request.
func (r *SyncedRequestRepo) getWorkspaceID(ctx context.Context, collectionID uuid.UUID) (string, error) {
	var wsID string
	err := sqlite.DBTXFromContext(ctx, r.db).QueryRowContext(ctx,
		`SELECT workspace_id FROM collections WHERE id = ?`, collectionID.String()).Scan(&wsID)
	if err != nil {
		return "", fmt.Errorf("SyncedRequestRepo.getWorkspaceID: %w", err)
	}
	return wsID, nil
}
