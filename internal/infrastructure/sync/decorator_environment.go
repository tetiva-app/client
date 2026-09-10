package sync

import (
	"context"
	"database/sql"
	"time"

	"github.com/google/uuid"

	"github.com/tetiva-app/client/internal/domain/entities"
	"github.com/tetiva-app/client/internal/domain/usecase/environment"
	"github.com/tetiva-app/client/internal/infrastructure/repository/sqlite"
)

// SyncedEnvironmentRepo decorates environment.Repository to enqueue sync operations on writes.
type SyncedEnvironmentRepo struct {
	inner     environment.Repository
	syncQueue sqlite.SyncQueueRepository
	db        *sql.DB
	engine    *SyncEngine
}

func NewSyncedEnvironmentRepo(inner environment.Repository, syncQueue sqlite.SyncQueueRepository, db *sql.DB, engine *SyncEngine) *SyncedEnvironmentRepo {
	return &SyncedEnvironmentRepo{
		inner:     inner,
		syncQueue: syncQueue,
		db:        db,
		engine:    engine,
	}
}

func (r *SyncedEnvironmentRepo) isSyncEnabled(workspaceID string) bool {
	return r.engine != nil && r.engine.IsEnabledForWorkspace(workspaceID)
}

// Create inserts an environment and enqueues a sync "create" entry within the same TX.
func (r *SyncedEnvironmentRepo) Create(ctx context.Context, e *entities.Environment) error {
	wsID := e.WorkspaceID.String()
	if !r.isSyncEnabled(wsID) {
		return r.inner.Create(ctx, e)
	}
	err := sqlite.WithTx(ctx, r.db, func(txCtx context.Context) error {
		if err := r.inner.Create(txCtx, e); err != nil {
			return err
		}
		return r.syncQueue.Enqueue(txCtx, sqlite.SyncEntry{
			WorkspaceID: wsID,
			EntityType:  "environment",
			EntityID:    e.ID.String(),
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

func (r *SyncedEnvironmentRepo) GetByID(ctx context.Context, id uuid.UUID) (*entities.Environment, error) {
	return r.inner.GetByID(ctx, id)
}

func (r *SyncedEnvironmentRepo) List(ctx context.Context, filter environment.Filter) ([]*entities.Environment, error) {
	return r.inner.List(ctx, filter)
}

// Update persists an environment and enqueues a sync "update" or "delete" entry within the same TX.
func (r *SyncedEnvironmentRepo) Update(ctx context.Context, e *entities.Environment) error {
	wsID := e.WorkspaceID.String()
	if !r.isSyncEnabled(wsID) {
		return r.inner.Update(ctx, e)
	}
	action := "update"
	if e.IsDelete {
		action = "delete"
	}
	err := sqlite.WithTx(ctx, r.db, func(txCtx context.Context) error {
		if err := r.inner.Update(txCtx, e); err != nil {
			return err
		}
		return r.syncQueue.Enqueue(txCtx, sqlite.SyncEntry{
			WorkspaceID: wsID,
			EntityType:  "environment",
			EntityID:    e.ID.String(),
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

func (r *SyncedEnvironmentRepo) GetActive(ctx context.Context, workspaceID uuid.UUID) (*entities.Environment, error) {
	return r.inner.GetActive(ctx, workspaceID)
}

// SetActive is not sync-relevant, so nothing is enqueued.
func (r *SyncedEnvironmentRepo) SetActive(ctx context.Context, workspaceID uuid.UUID, environmentID uuid.UUID) error {
	return r.inner.SetActive(ctx, workspaceID, environmentID)
}
