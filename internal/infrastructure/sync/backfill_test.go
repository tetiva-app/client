package sync

import (
	"context"
	"database/sql"
	"log/slog"
	"slices"
	gosync "sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	syncv1 "github.com/tetiva-app/proto/go/gophercourier/sync/v1"
)

func newBackfillEnv(t *testing.T) (*exampleSyncEnv, *pullScript) {
	t.Helper()
	env := newExampleSyncEnv(t)
	script := &pullScript{t: t}
	env.engine.SetGRPCClient(examplesCapableClient(&fakeSyncClient{pull: script.pull}))
	setPullPosition(t, env.ws, env.db, 10, "")
	setBackfill(t, env.db, true, "")
	return env, script
}

func setBackfill(t *testing.T, db *sql.DB, pending bool, token string) {
	t.Helper()
	_, err := db.Exec(`UPDATE workspaces SET examples_backfill_pending = ?, examples_backfill_token = ? WHERE id = ?`,
		pending, token, testWorkspaceID.String())
	require.NoError(t, err)
}

func backfillPosition(t *testing.T, db *sql.DB) (pending bool, token string) {
	t.Helper()
	require.NoError(t, db.QueryRow(`SELECT examples_backfill_pending, examples_backfill_token FROM workspaces WHERE id = ?`,
		testWorkspaceID.String()).Scan(&pending, &token))
	return pending, token
}

func exampleExists(db *sql.DB, id uuid.UUID) bool {
	var n int
	if err := db.QueryRow(`SELECT COUNT(*) FROM response_examples WHERE id = ?`, id.String()).Scan(&n); err != nil {
		return false
	}
	return n == 1
}

func isBackfillPull(r *syncv1.PullRequest) bool {
	return len(r.GetEntityTypes()) > 0
}

func brokenExample(id uuid.UUID) *syncv1.SyncEntity {
	e := exampleEntity(id, uuid.New(), "Broken", 1)
	e.GetResponseExample().SetRequestId("not-a-uuid")
	return e
}

func examplePage(token string, examples ...*syncv1.SyncEntity) *syncv1.PullResponse {
	changes := make([]*syncv1.SyncChange, 0, len(examples))
	for _, e := range examples {
		changes = append(changes, entityChange(e))
	}
	return syncv1.PullResponse_builder{Changes: changes, HasMore: token != "", NextSnapshotPageToken: token}.Build()
}

func TestBackfill_AppliesOnlyExamplesAndLeavesTheCursorAlone(t *testing.T) {
	env, script := newBackfillEnv(t)
	counting := countClears(env.ws)
	ctx := context.Background()
	exampleID := uuid.New()
	stranger := newTestCollection("Not an example", nil)
	script.steps = []pullStep{func(r *syncv1.PullRequest) (*syncv1.PullResponse, error) {
		assert.Zero(t, r.GetLastSyncSeq())
		assert.True(t, r.GetPagedSnapshot())
		assert.Empty(t, r.GetSnapshotPageToken())
		assert.Equal(t, []syncv1.EntityType{syncv1.EntityType_ENTITY_TYPE_RESPONSE_EXAMPLE}, r.GetEntityTypes())
		assert.ElementsMatch(t, allEntityTypes, r.GetKnownTypes())
		return syncv1.PullResponse_builder{
			Changes: []*syncv1.SyncChange{
				entityChange(CollectionToProto(stranger, "")),
				entityChange(exampleEntity(exampleID, env.req.ID, "Backfilled", 2)),
				syncv1.SyncChange_builder{SyncSeq: 99}.Build(),
			},
			NextSyncSeq:    99,
			ResyncRequired: true,
		}.Build(), nil
	}}

	require.NoError(t, env.ws.backfillExamples(ctx))

	assert.True(t, exampleExists(env.db, exampleID))
	assert.Equal(t, 1, exampleIsSynced(t, env.db, exampleID))
	got, err := env.engine.collections.GetByID(ctx, stranger.ID)
	require.NoError(t, err)
	assert.Nil(t, got, "the backfill stores examples only")
	seq, _ := pullPosition(t, env.db)
	assert.EqualValues(t, 10, seq)
	assert.EqualValues(t, 10, env.ws.cursor())
	assert.Zero(t, counting.clears, "resync_required belongs to the regular pull")
	pending, token := backfillPosition(t, env.db)
	assert.False(t, pending)
	assert.Empty(t, token)
	assert.False(t, env.ws.backfillPending)
}

func TestBackfill_SkipsExamplesWithAQueuedLocalChange(t *testing.T) {
	env, script := newBackfillEnv(t)
	local := env.createExample(t, "Edited here")
	fresh := uuid.New()
	script.steps = []pullStep{answer(examplePage("",
		exampleEntity(local.ID, env.req.ID, "Server copy", 7),
		exampleEntity(fresh, env.req.ID, "Only on the server", 1),
	))}

	require.NoError(t, env.ws.backfillExamples(context.Background()))

	assert.Equal(t, "Edited here", exampleContent(t, env.db, local.ID).name)
	assert.Zero(t, exampleIsSynced(t, env.db, local.ID))
	assert.True(t, exampleExists(env.db, fresh))
	pending, _ := backfillPosition(t, env.db)
	assert.False(t, pending, "a skipped example is settled by its own push, not owed by the backfill")
}

func TestBackfill_NewSyncerResumesFromTheSavedToken(t *testing.T) {
	env, script := newBackfillEnv(t)
	ctx := context.Background()
	first, second := uuid.New(), uuid.New()
	script.steps = []pullStep{
		answer(examplePage("b1", exampleEntity(first, env.req.ID, "Page one", 1))),
		func(*syncv1.PullRequest) (*syncv1.PullResponse, error) {
			return nil, status.Error(codes.Unavailable, "connection reset")
		},
		func(r *syncv1.PullRequest) (*syncv1.PullResponse, error) {
			assert.Equal(t, "b1", r.GetSnapshotPageToken())
			assert.Zero(t, r.GetLastSyncSeq())
			return examplePage("", exampleEntity(second, env.req.ID, "Page two", 1)), nil
		},
	}

	require.NoError(t, env.ws.backfillExamples(ctx), "a failed backfill does not fail the cycle")
	pending, token := backfillPosition(t, env.db)
	require.True(t, pending)
	require.Equal(t, "b1", token)
	assert.True(t, env.ws.backfillPending)

	restarted := &workspaceSyncer{
		engine:            env.engine,
		localWorkspaceID:  env.ws.localWorkspaceID,
		remoteWorkspaceID: env.ws.remoteWorkspaceID,
		pushSignal:        make(chan struct{}, 1),
		lastSyncSeq:       10,
		examplesCap:       capSupported,
	}
	require.NoError(t, restarted.backfillExamples(ctx))

	assert.True(t, exampleExists(env.db, first))
	assert.True(t, exampleExists(env.db, second))
	pending, token = backfillPosition(t, env.db)
	assert.False(t, pending)
	assert.Empty(t, token)
	assert.Len(t, script.requests(), 3)
}

func TestBackfill_DoesNotRunWithoutTheCapability(t *testing.T) {
	for _, c := range []capability{capUnknown, capUnsupported} {
		env, _ := newBackfillEnv(t)
		env.ws.examplesCap = c

		require.NoError(t, env.ws.backfillExamples(context.Background()))

		pending, _ := backfillPosition(t, env.db)
		assert.True(t, pending, "the flag waits for a server that takes examples")
		assert.True(t, env.ws.backfillPending)
		assert.Equal(t, c == capUnknown, env.ws.needsRecheck(), "an older server arms no timer for the backfill")
	}
}

func TestBackfill_AtCursorZeroDropsTheFlag(t *testing.T) {
	env, _ := newBackfillEnv(t)
	setPullPosition(t, env.ws, env.db, 0, "")
	setBackfill(t, env.db, true, "stale")

	require.NoError(t, env.ws.backfillExamples(context.Background()))

	pending, token := backfillPosition(t, env.db)
	assert.False(t, pending, "the snapshot from cursor 0 brings the examples itself")
	assert.Empty(t, token)
	assert.False(t, env.ws.needsRecheck())
}

func TestBackfill_FlagClearedByAResyncStopsTheNextAttempt(t *testing.T) {
	env, script := newBackfillEnv(t)
	ctx := context.Background()
	script.steps = []pullStep{answer(examplePage("", brokenExample(uuid.New())))}
	require.NoError(t, env.ws.backfillExamples(ctx))
	require.True(t, env.ws.backfillPending)

	_, err := env.engine.clearOutbox(ctx, env.ws.localWorkspaceID)
	require.NoError(t, err)
	setPullPosition(t, env.ws, env.db, 10, "")

	require.NoError(t, env.ws.backfillExamples(ctx))

	assert.Len(t, script.requests(), 1, "the flag is read from the database before every attempt")
	assert.False(t, env.ws.backfillPending)
	assert.False(t, env.ws.needsRecheck())
}

func TestBackfill_UnappliedExampleKeepsTheFlagForTheNextPass(t *testing.T) {
	env, script := newBackfillEnv(t)
	good, later := uuid.New(), uuid.New()
	script.steps = []pullStep{
		answer(examplePage("b1", brokenExample(uuid.New()), exampleEntity(good, env.req.ID, "Fine", 1))),
		answer(examplePage("", exampleEntity(later, env.req.ID, "Also fine", 1))),
	}

	require.NoError(t, env.ws.backfillExamples(context.Background()))

	assert.True(t, exampleExists(env.db, good))
	assert.True(t, exampleExists(env.db, later))
	pending, token := backfillPosition(t, env.db)
	assert.True(t, pending)
	assert.Empty(t, token, "the next pass starts from the first page")
	assert.True(t, env.ws.needsRecheck())
}

func TestBackfill_GivesUpAfterThreePassesWithUnappliedExamples(t *testing.T) {
	env, script := newBackfillEnv(t)
	ctx := context.Background()
	logs := &lockedBuffer{}
	previous := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(logs, &slog.HandlerOptions{Level: slog.LevelDebug})))
	t.Cleanup(func() { slog.SetDefault(previous) })

	broken := uuid.New()
	fromTheTop := func(r *syncv1.PullRequest) (*syncv1.PullResponse, error) {
		assert.Empty(t, r.GetSnapshotPageToken())
		return examplePage("", brokenExample(broken)), nil
	}
	script.steps = []pullStep{fromTheTop, fromTheTop, fromTheTop}

	for pass := 1; pass <= 2; pass++ {
		require.NoError(t, env.ws.backfillExamples(ctx))
		pending, _ := backfillPosition(t, env.db)
		require.True(t, pending, "pass %d", pass)
	}
	assert.NotContains(t, logs.String(), "gave up")

	require.NoError(t, env.ws.backfillExamples(ctx))

	pending, token := backfillPosition(t, env.db)
	assert.False(t, pending)
	assert.Empty(t, token)
	assert.False(t, env.ws.backfillPending)
	assert.Contains(t, logs.String(), "level=WARN")
	assert.Contains(t, logs.String(), "gave up")
	assert.Contains(t, logs.String(), broken.String())
}

func TestPullCycle_FailedBackfillStillPulls(t *testing.T) {
	env, script := newBackfillEnv(t)
	setBackfill(t, env.db, true, "b1")
	script.steps = []pullStep{
		func(r *syncv1.PullRequest) (*syncv1.PullResponse, error) {
			require.True(t, isBackfillPull(r))
			return nil, status.Error(codes.Unavailable, "backfill timed out")
		},
		func(r *syncv1.PullRequest) (*syncv1.PullResponse, error) {
			assert.False(t, isBackfillPull(r))
			assert.EqualValues(t, 10, r.GetLastSyncSeq())
			return syncv1.PullResponse_builder{NextSyncSeq: 10}.Build(), nil
		},
	}

	require.NoError(t, env.ws.pullCycle(context.Background()))

	assert.Len(t, script.requests(), 2)
	pending, token := backfillPosition(t, env.db)
	assert.True(t, pending)
	assert.Equal(t, "b1", token, "the walk resumes where it stopped")
}

func TestPullCycle_RefusedBackfillTokensDoNotFailTheCycle(t *testing.T) {
	env, script := newBackfillEnv(t)
	setBackfill(t, env.db, true, "stale")
	backfillPulls, regularPulls := 0, 0
	serve := func(r *syncv1.PullRequest) (*syncv1.PullResponse, error) {
		if !isBackfillPull(r) {
			regularPulls++
			return syncv1.PullResponse_builder{NextSyncSeq: 10}.Build(), nil
		}
		backfillPulls++
		if r.GetSnapshotPageToken() != "" {
			return nil, status.Error(codes.InvalidArgument, "snapshot boundary expired")
		}
		return examplePage("next"), nil
	}
	for range 8 {
		script.steps = append(script.steps, serve)
	}

	require.NoError(t, env.ws.pullCycle(context.Background()))

	assert.Equal(t, 7, backfillPulls, "three restarts, then the fourth refusal leaves the rest to the next cycle")
	assert.Equal(t, 1, regularPulls)
	pending, token := backfillPosition(t, env.db)
	assert.True(t, pending)
	assert.Empty(t, token)
}

func TestBackfill_AnnouncesPagesThatStoredExamples(t *testing.T) {
	env, script := newBackfillEnv(t)
	events := captureEvents(env.engine)
	script.steps = []pullStep{
		answer(examplePage("b1", exampleEntity(uuid.New(), env.req.ID, "One", 1))),
		answer(examplePage("b2")),
		answer(examplePage("", exampleEntity(uuid.New(), env.req.ID, "Two", 1))),
	}

	require.NoError(t, env.ws.backfillExamples(context.Background()))

	data, count := findEvent(*events, "sync:changed")
	assert.Equal(t, 2, count, "one per page that stored an example")
	assert.Equal(t, testWorkspaceID.String(), data["workspaceId"])
}

type backfillCycleClient struct {
	*cycleClient
	backfill func(n int, req *syncv1.PullRequest) *syncv1.PullResponse
	mu       gosync.Mutex
	calls    []string
}

func (c *backfillCycleClient) record(call string) int {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.calls = append(c.calls, call)
	n := 0
	for _, made := range c.calls {
		if made == call {
			n++
		}
	}
	return n
}

func (c *backfillCycleClient) made() []string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return slices.Clone(c.calls)
}

func (c *backfillCycleClient) count(call string) int {
	n := 0
	for _, made := range c.made() {
		if made == call {
			n++
		}
	}
	return n
}

func (c *backfillCycleClient) Push(ctx context.Context, req *syncv1.PushRequest, opts ...grpc.CallOption) (*syncv1.PushResponse, error) {
	c.record("push")
	return c.cycleClient.Push(ctx, req, opts...)
}

func (c *backfillCycleClient) Pull(_ context.Context, req *syncv1.PullRequest, _ ...grpc.CallOption) (*syncv1.PullResponse, error) {
	if !isBackfillPull(req) {
		c.record("pull")
		return syncv1.PullResponse_builder{NextSyncSeq: req.GetLastSyncSeq()}.Build(), nil
	}
	return c.backfill(c.record("backfill"), req), nil
}

func (env *exampleSyncEnv) startBackfillCycling(t *testing.T, capabilities []string,
	backfill func(n int, req *syncv1.PullRequest) *syncv1.PullResponse,
) (*backfillCycleClient, *capabilityAuthClient, *stateLog) {
	t.Helper()
	setPullPosition(t, env.ws, env.db, 10, "")
	setBackfill(t, env.db, true, "")
	client := &backfillCycleClient{cycleClient: newCycleClient(), backfill: backfill}
	info := &capabilityAuthClient{capabilities: capabilities}
	env.engine.SetGRPCClient(&GRPCClient{sync: client, auth: info})
	return client, info, env.startSyncerAt(t, 10)
}

func TestSyncer_BackfillRunsBetweenPushAndPull(t *testing.T) {
	env := newExampleSyncEnv(t)
	env.req.Version++
	require.NoError(t, env.requests.Update(context.Background(), env.req))
	backfilled := uuid.New()

	client, _, _ := env.startBackfillCycling(t, []string{"response_examples"}, func(int, *syncv1.PullRequest) *syncv1.PullResponse {
		return examplePage("", exampleEntity(backfilled, env.req.ID, "Backfilled", 1))
	})

	require.Eventually(t, func() bool { return client.subscribes.Load() == 1 }, 5*time.Second, 10*time.Millisecond)
	assert.Equal(t, []string{"push", "backfill", "pull"}, client.made())
	assert.True(t, exampleExists(env.db, backfilled))
}

func TestSyncer_UnfinishedBackfillIsRetriedOnTheRecheckTimer(t *testing.T) {
	restore := capabilityRecheckInterval
	capabilityRecheckInterval = 50 * time.Millisecond
	t.Cleanup(func() { capabilityRecheckInterval = restore })

	env := newExampleSyncEnv(t)
	fixed := uuid.New()
	client, info, states := env.startBackfillCycling(t, []string{"response_examples"}, func(n int, _ *syncv1.PullRequest) *syncv1.PullResponse {
		if n == 1 {
			return examplePage("", brokenExample(fixed))
		}
		return examplePage("", exampleEntity(fixed, env.req.ID, "Fixed on the server", 2))
	})

	require.Eventually(t, func() bool { return exampleExists(env.db, fixed) }, 5*time.Second, 10*time.Millisecond)
	require.Eventually(t, func() bool { return client.subscribes.Load() == 2 }, 5*time.Second, 10*time.Millisecond)
	assert.Never(t, func() bool { return client.subscribes.Load() > 2 }, 200*time.Millisecond, 10*time.Millisecond,
		"a finished backfill stops the rechecks")
	assert.Equal(t, 2, client.count("backfill"))
	assert.Equal(t, int32(1), info.calls.Load(), "an unbroken stream reuses the confirmed capability")
	assert.NotContains(t, states.all(), string(StateOffline))
	pending, _ := backfillPosition(t, env.db)
	assert.False(t, pending)
}

func TestSyncer_OlderServerArmsNoBackfillRecheck(t *testing.T) {
	restore := capabilityRecheckInterval
	capabilityRecheckInterval = 20 * time.Millisecond
	t.Cleanup(func() { capabilityRecheckInterval = restore })

	env := newExampleSyncEnv(t)
	client, _, _ := env.startBackfillCycling(t, []string{"desktop_signin"}, func(int, *syncv1.PullRequest) *syncv1.PullResponse {
		t.Error("an older server has no examples to backfill")
		return examplePage("")
	})

	require.Eventually(t, func() bool { return client.subscribes.Load() == 1 }, 5*time.Second, 10*time.Millisecond)
	assert.Never(t, func() bool { return client.subscribes.Load() > 1 }, 200*time.Millisecond, 10*time.Millisecond)
	pending, _ := backfillPosition(t, env.db)
	assert.True(t, pending)
}
