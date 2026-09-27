package sync

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tetiva-app/client/internal/domain/entities"
	"github.com/tetiva-app/client/internal/domain/usecase/collection"
	"github.com/tetiva-app/client/internal/domain/usecase/example"
	"github.com/tetiva-app/client/internal/domain/usecase/request"
	"github.com/tetiva-app/client/internal/infrastructure/repository/sqlite"
)

var errSecondUpdate = errors.New("disk full on the second example")

// failingExampleRepo lets the first Update through so the rollback has a written row to undo.
type failingExampleRepo struct {
	*sqlite.ResponseExampleRepo
	updates int
}

func (r *failingExampleRepo) Update(ctx context.Context, e *entities.ResponseExample) error {
	r.updates++
	if r.updates == 2 {
		return errSecondUpdate
	}
	return r.ResponseExampleRepo.Update(ctx, e)
}

type cascadeCollections struct{ collection.Repository }

func (c cascadeCollections) ListByWorkspace(ctx context.Context, workspaceID uuid.UUID) ([]*entities.Collection, error) {
	return c.List(ctx, collection.Filter{WorkspaceID: workspaceID})
}

func TestRequestDelete_FailedExampleCascadeLeavesNothingQueued(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()
	engine := &SyncEngine{}
	engine.InjectRawSyncer(testWorkspaceID.String(), "remote", func() {})
	v, _ := engine.workspaces.Load(testWorkspaceID.String())
	signal := v.(*workspaceSyncer).pushSignal

	queue := sqlite.NewSyncQueueRepo(db)
	innerCols := sqlite.NewCollectionRepo(db)
	innerReqs := sqlite.NewRequestRepo(db)
	innerExamples := sqlite.NewResponseExampleRepo(db)
	cols := NewSyncedCollectionRepo(innerCols, queue, db, engine)
	reqs := NewSyncedRequestRepo(innerReqs, queue, db, engine)
	examples := NewSyncedResponseExampleRepo(&failingExampleRepo{ResponseExampleRepo: innerExamples}, queue, db, engine)

	coll := newTestCollection("Host", nil)
	require.NoError(t, innerCols.Create(ctx, coll))
	req := newLocalRequest("Get user", coll.ID)
	require.NoError(t, innerReqs.Create(ctx, req))
	var exampleIDs []uuid.UUID
	for _, name := range []string{"200", "404", "500"} {
		now := time.Now().Truncate(time.Second)
		ex := &entities.ResponseExample{
			ID: uuid.New(), RequestID: req.ID, WorkspaceID: testWorkspaceID, Name: name,
			Headers: []entities.HeaderItem{}, Protocol: entities.ProtocolHTTP, Version: 1, CreatedAt: now, UpdatedAt: now,
		}
		require.NoError(t, innerExamples.Create(ctx, ex))
		exampleIDs = append(exampleIDs, ex.ID)
	}

	uc := request.NewUsecase(reqs, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil,
		cascadeCollections{cols}, nil, nil, example.NewUsecase(examples, reqs, cols), sqlite.NewTxRunner(db))

	err := uc.Delete(ctx, request.DeleteOpt{RequestID: req.ID, UserID: "test_user", Version: 1})
	require.ErrorIs(t, err, errSecondUpdate)

	got, err := innerReqs.GetByID(ctx, req.ID)
	require.NoError(t, err)
	require.NotNil(t, got, "the request delete must roll back with its cascade")
	assert.Equal(t, 1, got.Version)
	for _, id := range exampleIDs {
		ex, err := innerExamples.GetByID(ctx, id)
		require.NoError(t, err)
		assert.NotNil(t, ex, "example %s must stay live", id)
	}
	var queued int
	require.NoError(t, db.QueryRow(`SELECT COUNT(*) FROM sync_queue`).Scan(&queued))
	assert.Zero(t, queued, "no tombstone of a rolled-back delete may reach the outbox")
	assert.Empty(t, signal, "a rolled-back delete must not wake the syncer")
}
