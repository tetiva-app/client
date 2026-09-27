package sync

import (
	"context"
	"database/sql"
	"log/slog"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	syncv1 "github.com/tetiva-app/proto/go/gophercourier/sync/v1"
)

func backfillFailedPasses(t *testing.T, db *sql.DB) int {
	t.Helper()
	var n int
	require.NoError(t, db.QueryRow(`SELECT examples_backfill_failed_passes FROM workspaces WHERE id = ?`,
		testWorkspaceID.String()).Scan(&n))
	return n
}

// restartedSyncer is what a new process builds from the database: nothing carried over in memory.
func restartedSyncer(env *exampleSyncEnv) *workspaceSyncer {
	return &workspaceSyncer{
		engine:            env.engine,
		localWorkspaceID:  env.ws.localWorkspaceID,
		remoteWorkspaceID: env.ws.remoteWorkspaceID,
		pushSignal:        make(chan struct{}, 1),
		lastSyncSeq:       env.ws.cursor(),
		examplesCap:       capSupported,
	}
}

func TestBackfill_StorageFailureOnAnEarlierPageSurvivesARestart(t *testing.T) {
	env, script := newBackfillEnv(t)
	ctx := context.Background()
	unstored, first, second := uuid.New(), uuid.New(), uuid.New()
	_, err := env.db.Exec(`CREATE TRIGGER fail_one_example BEFORE INSERT ON response_examples
		WHEN NEW.id = '` + unstored.String() + `' BEGIN SELECT RAISE(ABORT, 'disk I/O error'); END`)
	require.NoError(t, err)
	script.steps = []pullStep{
		answer(examplePage("b1", exampleEntity(unstored, env.req.ID, "Not stored", 1), exampleEntity(first, env.req.ID, "Stored", 1))),
		func(*syncv1.PullRequest) (*syncv1.PullResponse, error) {
			return nil, status.Error(codes.Unavailable, "the app was closed")
		},
		answer(examplePage("", exampleEntity(second, env.req.ID, "Last page", 1))),
	}

	require.NoError(t, env.ws.backfillExamples(ctx))
	_, err = env.db.Exec(`DROP TRIGGER fail_one_example`)
	require.NoError(t, err)
	require.False(t, exampleExists(env.db, unstored))

	require.NoError(t, restartedSyncer(env).backfillExamples(ctx))

	assert.True(t, exampleExists(env.db, second))
	pending, token := backfillPosition(t, env.db)
	assert.True(t, pending, "the pass left an example unstored, so the next one runs from the top")
	assert.Empty(t, token)
	assert.Equal(t, 1, backfillFailedPasses(t, env.db), "a closed app is not a failed pass, the unstored example is")
}

func TestBackfill_GivesUpAfterThreePassesTheServerFailed(t *testing.T) {
	env, script := newBackfillEnv(t)
	ctx := context.Background()
	logs := &lockedBuffer{}
	previous := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(logs, &slog.HandlerOptions{Level: slog.LevelDebug})))
	t.Cleanup(func() { slog.SetDefault(previous) })
	internal := func(*syncv1.PullRequest) (*syncv1.PullResponse, error) {
		return nil, status.Error(codes.Internal, "cannot decrypt an example")
	}
	script.steps = []pullStep{internal, internal, internal}

	require.NoError(t, env.ws.backfillExamples(ctx))
	require.NoError(t, env.ws.backfillExamples(ctx))
	pending, _ := backfillPosition(t, env.db)
	require.True(t, pending)
	assert.Equal(t, 2, backfillFailedPasses(t, env.db))
	assert.NotContains(t, logs.String(), "gave up")

	restarted := restartedSyncer(env)
	require.NoError(t, restarted.backfillExamples(ctx))

	pending, token := backfillPosition(t, env.db)
	assert.False(t, pending, "the count lives in the database, so a restart does not reset it")
	assert.Empty(t, token)
	assert.Zero(t, backfillFailedPasses(t, env.db))
	assert.False(t, restarted.backfillPending)
	assert.Contains(t, logs.String(), "gave up")
}

func TestBackfill_LostConnectionsAreNotFailedPasses(t *testing.T) {
	env, script := newBackfillEnv(t)
	ctx := context.Background()
	for _, code := range []codes.Code{codes.Unavailable, codes.DeadlineExceeded, codes.Unavailable, codes.Unauthenticated} {
		script.steps = append(script.steps, func(*syncv1.PullRequest) (*syncv1.PullResponse, error) {
			return nil, status.Error(code, "no answer")
		})
	}

	for range 4 {
		require.NoError(t, env.ws.backfillExamples(ctx))
	}

	pending, _ := backfillPosition(t, env.db)
	assert.True(t, pending)
	assert.Zero(t, backfillFailedPasses(t, env.db))
}

func TestBackfill_ACleanPassResetsTheFailedCount(t *testing.T) {
	env, script := newBackfillEnv(t)
	ctx := context.Background()
	script.steps = []pullStep{
		func(*syncv1.PullRequest) (*syncv1.PullResponse, error) {
			return nil, status.Error(codes.Internal, "boom")
		},
		answer(examplePage("", exampleEntity(uuid.New(), env.req.ID, "Fine", 1))),
	}

	require.NoError(t, env.ws.backfillExamples(ctx))
	require.Equal(t, 1, backfillFailedPasses(t, env.db))
	require.NoError(t, env.ws.backfillExamples(ctx))

	pending, _ := backfillPosition(t, env.db)
	assert.False(t, pending)
	assert.Zero(t, backfillFailedPasses(t, env.db))
}
