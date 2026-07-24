package sync

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/tetiva-app/client/internal/domain/entities"
	"github.com/tetiva-app/client/internal/domain/usecase/environment"
	"github.com/tetiva-app/client/internal/infrastructure/repository/sqlite"
)

// SyncedVariableRepo decorates environment.VariableRepository to enqueue sync operations on writes.
type SyncedVariableRepo struct {
	inner     environment.VariableRepository
	syncQueue sqlite.SyncQueueRepository
	db        *sql.DB
	engine    *SyncEngine
}

// NewSyncedVariableRepo creates a new SyncedVariableRepo.
func NewSyncedVariableRepo(inner environment.VariableRepository, syncQueue sqlite.SyncQueueRepository, db *sql.DB, engine *SyncEngine) *SyncedVariableRepo {
	return &SyncedVariableRepo{
		inner:     inner,
		syncQueue: syncQueue,
		db:        db,
		engine:    engine,
	}
}

func (r *SyncedVariableRepo) isSyncEnabled(workspaceID string) bool {
	return r.engine != nil && r.engine.IsEnabledForWorkspace(workspaceID)
}

// Create inserts a variable and enqueues a sync "create" entry within the same TX.
func (r *SyncedVariableRepo) Create(ctx context.Context, v *entities.Variable) error {
	wsID, err := r.getWorkspaceIDByEnvironment(ctx, v.EnvironmentID)
	if err != nil || !r.isSyncEnabled(wsID) {
		return r.inner.Create(ctx, v)
	}
	txErr := sqlite.WithTx(ctx, r.db, func(txCtx context.Context) error {
		if err := r.inner.Create(txCtx, v); err != nil {
			return err
		}
		return r.syncQueue.Enqueue(txCtx, sqlite.SyncEntry{
			WorkspaceID: wsID,
			EntityType:  "variable",
			EntityID:    v.ID.String(),
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
func (r *SyncedVariableRepo) GetByID(ctx context.Context, id uuid.UUID) (*entities.Variable, error) {
	return r.inner.GetByID(ctx, id)
}

// List delegates to the inner repository.
func (r *SyncedVariableRepo) List(ctx context.Context, environmentID uuid.UUID) ([]*entities.Variable, error) {
	return r.inner.List(ctx, environmentID)
}

// Update persists a variable and enqueues a sync "update" or "delete" entry within the same TX.
func (r *SyncedVariableRepo) Update(ctx context.Context, v *entities.Variable) error {
	wsID, err := r.getWorkspaceIDByEnvironment(ctx, v.EnvironmentID)
	if err != nil || !r.isSyncEnabled(wsID) {
		return r.inner.Update(ctx, v)
	}
	action := "update"
	if v.IsDelete {
		action = "delete"
	}
	txErr := sqlite.WithTx(ctx, r.db, func(txCtx context.Context) error {
		if err := r.inner.Update(txCtx, v); err != nil {
			return err
		}
		return r.syncQueue.Enqueue(txCtx, sqlite.SyncEntry{
			WorkspaceID: wsID,
			EntityType:  "variable",
			EntityID:    v.ID.String(),
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

// Delete removes a variable and enqueues a sync "delete" entry within the same TX.
func (r *SyncedVariableRepo) Delete(ctx context.Context, id uuid.UUID) error {
	wsID, err := r.getWorkspaceIDByVariable(ctx, id)
	if err != nil || !r.isSyncEnabled(wsID) {
		return r.inner.Delete(ctx, id)
	}
	txErr := sqlite.WithTx(ctx, r.db, func(txCtx context.Context) error {
		if err := r.inner.Delete(txCtx, id); err != nil {
			return err
		}
		return r.syncQueue.Enqueue(txCtx, sqlite.SyncEntry{
			WorkspaceID: wsID,
			EntityType:  "variable",
			EntityID:    id.String(),
			Action:      "delete",
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

func (r *SyncedVariableRepo) getWorkspaceIDByEnvironment(ctx context.Context, environmentID uuid.UUID) (string, error) {
	var wsID string
	err := sqlite.DBTXFromContext(ctx, r.db).QueryRowContext(ctx,
		`SELECT workspace_id FROM environments WHERE id = ?`, environmentID.String()).Scan(&wsID)
	if err != nil {
		return "", fmt.Errorf("SyncedVariableRepo.getWorkspaceIDByEnvironment: %w", err)
	}
	return wsID, nil
}

// getWorkspaceIDByVariable looks up the workspace_id by joining variables → environments.
func (r *SyncedVariableRepo) getWorkspaceIDByVariable(ctx context.Context, variableID uuid.UUID) (string, error) {
	var wsID string
	err := sqlite.DBTXFromContext(ctx, r.db).QueryRowContext(ctx,
		`SELECT e.workspace_id FROM variables v JOIN environments e ON v.environment_id = e.id WHERE v.id = ?`,
		variableID.String()).Scan(&wsID)
	if err != nil {
		return "", fmt.Errorf("SyncedVariableRepo.getWorkspaceIDByVariable: %w", err)
	}
	return wsID, nil
}
