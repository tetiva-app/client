package sync

import (
	"context"
	"database/sql"
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

var errSecondExample = errors.New("disk full on the second example")

type failingExampleRepo struct {
	*sqlite.ResponseExampleRepo
	creates, updates int
}

func (r *failingExampleRepo) Create(ctx context.Context, e *entities.ResponseExample) error {
	r.creates++
	if r.creates == 2 {
		return errSecondExample
	}
	return r.ResponseExampleRepo.Create(ctx, e)
}

func (r *failingExampleRepo) Update(ctx context.Context, e *entities.ResponseExample) error {
	r.updates++
	if r.updates == 2 {
		return errSecondExample
	}
	return r.ResponseExampleRepo.Update(ctx, e)
}

type cascadeCollections struct{ collection.Repository }

func (c cascadeCollections) ListByWorkspace(ctx context.Context, workspaceID uuid.UUID) ([]*entities.Collection, error) {
	return c.List(ctx, collection.Filter{WorkspaceID: workspaceID})
}

type cascadeEnv struct {
	db         *sql.DB
	signal     chan struct{}
	reqs       request.Repository
	examples   *sqlite.ResponseExampleRepo
	req        *entities.Request
	exampleIDs []uuid.UUID
	uc         request.Usecase
}

func newCascadeEnv(t *testing.T, wrap func(*sqlite.ResponseExampleRepo) example.Repository) *cascadeEnv {
	t.Helper()
	db := setupTestDB(t)
	ctx := context.Background()
	engine := &SyncEngine{}
	engine.InjectRawSyncer(testWorkspaceID.String(), "remote", func() {})
	v, _ := engine.workspaces.Load(testWorkspaceID.String())

	queue := sqlite.NewSyncQueueRepo(db)
	innerCols := sqlite.NewCollectionRepo(db)
	innerReqs := sqlite.NewRequestRepo(db)
	innerExamples := sqlite.NewResponseExampleRepo(db)
	cols := NewSyncedCollectionRepo(innerCols, queue, db, engine)
	reqs := NewSyncedRequestRepo(innerReqs, queue, db, engine)
	examples := NewSyncedResponseExampleRepo(wrap(innerExamples), queue, db, engine)

	coll := newTestCollection("Host", nil)
	require.NoError(t, innerCols.Create(ctx, coll))
	req := newLocalRequest("Get user", coll.ID)
	require.NoError(t, innerReqs.Create(ctx, req))
	env := &cascadeEnv{db: db, signal: v.(*workspaceSyncer).pushSignal, reqs: innerReqs, examples: innerExamples, req: req}
	for _, name := range []string{"200", "404", "500"} {
		now := time.Now().Truncate(time.Second)
		ex := &entities.ResponseExample{
			ID: uuid.New(), RequestID: req.ID, WorkspaceID: testWorkspaceID, Name: name,
			Headers: []entities.HeaderItem{}, Protocol: entities.ProtocolHTTP, Version: 1, CreatedAt: now, UpdatedAt: now,
		}
		require.NoError(t, innerExamples.Create(ctx, ex))
		env.exampleIDs = append(env.exampleIDs, ex.ID)
	}

	env.uc = request.NewUsecase(reqs, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil,
		cascadeCollections{cols}, nil, nil, example.NewUsecase(examples, reqs, cols), sqlite.NewTxRunner(db))
	return env
}

func failingExamples(r *sqlite.ResponseExampleRepo) example.Repository {
	return &failingExampleRepo{ResponseExampleRepo: r}
}

func queuedOps(t *testing.T, db *sql.DB) []string {
	t.Helper()
	rows, err := db.Query(`SELECT entity_type, action, entity_id FROM sync_queue`)
	require.NoError(t, err)
	defer func() { _ = rows.Close() }()
	var ops []string
	for rows.Next() {
		var entityType, action, id string
		require.NoError(t, rows.Scan(&entityType, &action, &id))
		ops = append(ops, entityType+" "+action+" "+id)
	}
	require.NoError(t, rows.Err())
	return ops
}

func TestRequestDelete_FailedExampleCascadeLeavesNothingQueued(t *testing.T) {
	env := newCascadeEnv(t, failingExamples)
	ctx := context.Background()

	err := env.uc.Delete(ctx, request.DeleteOpt{RequestID: env.req.ID, UserID: "test_user", Version: 1})
	require.ErrorIs(t, err, errSecondExample)

	got, err := env.reqs.GetByID(ctx, env.req.ID)
	require.NoError(t, err)
	require.NotNil(t, got, "the request delete must roll back with its cascade")
	assert.Equal(t, 1, got.Version)
	for _, id := range env.exampleIDs {
		ex, err := env.examples.GetByID(ctx, id)
		require.NoError(t, err)
		assert.NotNil(t, ex, "example %s must stay live", id)
	}
	assert.Empty(t, queuedOps(t, env.db), "no tombstone of a rolled-back delete may reach the outbox")
	assert.Empty(t, env.signal, "a rolled-back delete must not wake the syncer")
}

func TestRequestDuplicate_QueuesTheCopyAndEachExampleOnce(t *testing.T) {
	env := newCascadeEnv(t, func(r *sqlite.ResponseExampleRepo) example.Repository { return r })
	ctx := context.Background()

	dup, err := env.uc.Duplicate(ctx, request.DuplicateOpt{RequestID: env.req.ID, Name: "Get user (copy)", UserID: "test_user"})
	require.NoError(t, err)

	copies, err := env.examples.ListByRequest(ctx, dup.ID)
	require.NoError(t, err)
	require.Len(t, copies, len(env.exampleIDs))
	want := []string{"request create " + dup.ID.String()}
	for _, c := range copies {
		assert.NotContains(t, env.exampleIDs, c.ID)
		assert.Equal(t, testWorkspaceID, c.WorkspaceID)
		want = append(want, "response_example create "+c.ID.String())
	}
	assert.ElementsMatch(t, want, queuedOps(t, env.db))
	assert.Len(t, env.signal, 1)
}

func TestRequestDuplicate_FailedExampleCopyLeavesNothingBehind(t *testing.T) {
	env := newCascadeEnv(t, failingExamples)
	ctx := context.Background()

	_, err := env.uc.Duplicate(ctx, request.DuplicateOpt{RequestID: env.req.ID, Name: "Get user (copy)", UserID: "test_user"})
	require.ErrorIs(t, err, errSecondExample)

	var requests, examples int
	require.NoError(t, env.db.QueryRow(`SELECT COUNT(*) FROM requests`).Scan(&requests))
	require.NoError(t, env.db.QueryRow(`SELECT COUNT(*) FROM response_examples`).Scan(&examples))
	assert.Equal(t, 1, requests, "the copy must roll back")
	assert.Equal(t, len(env.exampleIDs), examples, "the copied examples must roll back")
	assert.Empty(t, queuedOps(t, env.db), "nothing of a rolled-back copy may reach the outbox")
	assert.Empty(t, env.signal, "a rolled-back copy must not wake the syncer")
}
