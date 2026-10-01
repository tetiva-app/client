package sync

import (
	"context"
	"database/sql"
	"time"

	"github.com/google/uuid"

	"github.com/tetiva-app/client/internal/domain/entities"
	"github.com/tetiva-app/client/internal/domain/usecase/example"
	"github.com/tetiva-app/client/internal/infrastructure/repository/sqlite"
)

// SyncedResponseExampleRepo queues under the example's own workspace, not its request's.
type SyncedResponseExampleRepo struct {
	inner     example.Repository
	syncQueue sqlite.SyncQueueRepository
	db        *sql.DB
	engine    *SyncEngine
}

func NewSyncedResponseExampleRepo(inner example.Repository, syncQueue sqlite.SyncQueueRepository, db *sql.DB, engine *SyncEngine) *SyncedResponseExampleRepo {
	return &SyncedResponseExampleRepo{
		inner:     inner,
		syncQueue: syncQueue,
		db:        db,
		engine:    engine,
	}
}

func (r *SyncedResponseExampleRepo) queuesWrites(ctx context.Context, workspaceID string) (bool, error) {
	return queuesWrites(ctx, r.db, r.engine, workspaceID)
}

func (r *SyncedResponseExampleRepo) Create(ctx context.Context, e *entities.ResponseExample) error {
	wsID := e.WorkspaceID.String()
	queued, err := r.queuesWrites(ctx, wsID)
	if err != nil {
		return err
	}
	if !queued {
		return r.inner.Create(ctx, e)
	}
	return sqlite.WithTx(ctx, r.db, func(txCtx context.Context) error {
		if err := r.inner.Create(txCtx, e); err != nil {
			return err
		}
		return r.enqueue(txCtx, wsID, e.ID, "create")
	})
}

func (r *SyncedResponseExampleRepo) GetByID(ctx context.Context, id uuid.UUID) (*entities.ResponseExample, error) {
	return r.inner.GetByID(ctx, id)
}

func (r *SyncedResponseExampleRepo) ListByRequest(ctx context.Context, requestID uuid.UUID) ([]*entities.ResponseExample, error) {
	return r.inner.ListByRequest(ctx, requestID)
}

func (r *SyncedResponseExampleRepo) Update(ctx context.Context, e *entities.ResponseExample) error {
	wsID := e.WorkspaceID.String()
	queued, err := r.queuesWrites(ctx, wsID)
	if err != nil {
		return err
	}
	if !queued {
		return r.inner.Update(ctx, e)
	}
	return sqlite.WithTx(ctx, r.db, func(txCtx context.Context) error {
		if err := r.inner.Update(txCtx, e); err != nil {
			return err
		}
		return r.enqueue(txCtx, wsID, e.ID, changeAction(e))
	})
}

func (r *SyncedResponseExampleRepo) UpdateAtVersion(ctx context.Context, e *entities.ResponseExample, baseVersion int) error {
	wsID := e.WorkspaceID.String()
	queued, err := r.queuesWrites(ctx, wsID)
	if err != nil {
		return err
	}
	if !queued {
		return r.inner.UpdateAtVersion(ctx, e, baseVersion)
	}
	return sqlite.WithTx(ctx, r.db, func(txCtx context.Context) error {
		if err := r.inner.UpdateAtVersion(txCtx, e, baseVersion); err != nil {
			return err
		}
		return r.enqueue(txCtx, wsID, e.ID, changeAction(e))
	})
}

func changeAction(e *entities.ResponseExample) string {
	if e.IsDelete {
		return "delete"
	}
	return "update"
}

func (r *SyncedResponseExampleRepo) enqueue(txCtx context.Context, wsID string, id uuid.UUID, action string) error {
	if err := r.syncQueue.Enqueue(txCtx, sqlite.SyncEntry{
		WorkspaceID: wsID,
		EntityType:  "response_example",
		EntityID:    id.String(),
		Action:      action,
		OperationID: uuid.New().String(),
		Status:      "pending",
		CreatedAt:   time.Now(),
	}); err != nil {
		return err
	}
	sqlite.AfterCommit(txCtx, func() { r.engine.NotifyWrite(wsID) })
	return nil
}
