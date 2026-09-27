package sync

import (
	"context"
	"database/sql"
	"fmt"
	"slices"
	gosync "sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	authv1 "github.com/tetiva-app/proto/go/gophercourier/auth/v1"
	syncv1 "github.com/tetiva-app/proto/go/gophercourier/sync/v1"

	"github.com/tetiva-app/client/internal/domain/entities"
	"github.com/tetiva-app/client/internal/domain/usecase/example"
	"github.com/tetiva-app/client/internal/infrastructure/repository/sqlite"
)

func TestExamplesCapability_SupportIsCachedPerClient(t *testing.T) {
	engine := newTestEngine(t)
	ctx := context.Background()
	info := &capabilityAuthClient{capabilities: []string{"response_examples"}}
	client := &GRPCClient{auth: info}
	engine.SetGRPCClient(client)

	assert.Equal(t, capSupported, engine.examplesCapability(ctx))
	assert.Equal(t, capSupported, engine.examplesCapability(ctx))
	assert.Equal(t, int32(1), info.calls.Load(), "a confirmed capability is not asked again")

	engine.SetGRPCClient(client)
	assert.Equal(t, capSupported, engine.examplesCapability(ctx))
	assert.Equal(t, int32(2), info.calls.Load(), "setting the client forgets the answer")
}

func TestExamplesCapability_OnlySupportIsCached(t *testing.T) {
	cases := []struct {
		name   string
		answer func(context.Context, int32) (*authv1.GetServerInfoResponse, error)
		want   capability
	}{
		{"rpc error", func(context.Context, int32) (*authv1.GetServerInfoResponse, error) {
			return nil, status.Error(codes.Unavailable, "connection refused")
		}, capUnknown},
		{"no capability", func(context.Context, int32) (*authv1.GetServerInfoResponse, error) {
			return serverInfoWith("desktop_signin"), nil
		}, capUnsupported},
		{"server without the rpc", func(context.Context, int32) (*authv1.GetServerInfoResponse, error) {
			return nil, status.Error(codes.Unimplemented, "unknown method")
		}, capUnsupported},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			engine := newTestEngine(t)
			info := &capabilityAuthClient{answer: tc.answer}
			engine.SetGRPCClient(&GRPCClient{auth: info})

			assert.Equal(t, tc.want, engine.examplesCapability(context.Background()))
			assert.Equal(t, tc.want, engine.examplesCapability(context.Background()))
			assert.Equal(t, int32(2), info.calls.Load())
		})
	}
}

func TestExamplesCapability_WithoutAnAuthClientIsUnsupported(t *testing.T) {
	engine := newTestEngine(t)
	ctx := context.Background()

	assert.Equal(t, capUnsupported, engine.examplesCapability(ctx))

	engine.SetGRPCClient(&GRPCClient{sync: &stubSyncClient{}})
	assert.Equal(t, capUnsupported, engine.examplesCapability(ctx))
}

func TestExamplesCapability_TimeoutIsUnknown(t *testing.T) {
	restore := serverInfoTimeout
	serverInfoTimeout = 20 * time.Millisecond
	t.Cleanup(func() { serverInfoTimeout = restore })

	engine := newTestEngine(t)
	engine.SetGRPCClient(&GRPCClient{auth: &capabilityAuthClient{
		answer: func(ctx context.Context, _ int32) (*authv1.GetServerInfoResponse, error) {
			<-ctx.Done()
			return nil, status.FromContextError(ctx.Err()).Err()
		},
	}})

	start := time.Now()
	assert.Equal(t, capUnknown, engine.examplesCapability(context.Background()))
	assert.Less(t, time.Since(start), 2*time.Second)
}

func TestExamplesCapability_LateAnswerForAReplacedClientIsDiscarded(t *testing.T) {
	engine := newTestEngine(t)
	ctx := context.Background()
	wsID := testWorkspaceID.String()
	enqueueExampleRow(t, engine, wsID)

	entered, release := make(chan struct{}), make(chan struct{})
	engine.SetGRPCClient(&GRPCClient{auth: &capabilityAuthClient{
		answer: func(context.Context, int32) (*authv1.GetServerInfoResponse, error) {
			close(entered)
			<-release
			return serverInfoWith("response_examples"), nil
		},
	}})
	late := make(chan capability, 1)
	go func() { late <- engine.examplesCapability(ctx) }()
	<-entered

	fresh := &capabilityAuthClient{capabilities: []string{"desktop_signin"}}
	engine.SetGRPCClient(&GRPCClient{auth: fresh})
	close(release)

	assert.Equal(t, capUnknown, <-late, "the answer describes a server the engine no longer talks to")
	owed, err := engine.GetPendingCount(ctx, wsID)
	require.NoError(t, err)
	assert.Zero(t, owed, "the new client must not inherit the old server's capability")
	assert.Equal(t, capUnsupported, engine.examplesCapability(ctx))
	assert.Equal(t, int32(1), fresh.calls.Load())
}

func TestGetPendingCount_ExamplesCountOnceTheServerTakesThem(t *testing.T) {
	engine := newTestEngine(t)
	ctx := context.Background()
	wsID := testWorkspaceID.String()
	enqueueExampleRow(t, engine, wsID)
	enqueuePending(t, engine, wsID)
	owed := func() int {
		t.Helper()
		n, err := engine.GetPendingCount(ctx, wsID)
		require.NoError(t, err)
		return n
	}

	engine.SetGRPCClient(&GRPCClient{auth: &capabilityAuthClient{capabilities: []string{"response_examples"}}})
	assert.Equal(t, 1, owed(), "until a cycle confirms the capability the example is not owed")

	require.Equal(t, capSupported, engine.examplesCapability(ctx))
	assert.Equal(t, 2, owed())

	engine.SetGRPCClient(&GRPCClient{auth: &capabilityAuthClient{}})
	assert.Equal(t, 1, owed())
}

func TestPushAll_WithoutCapabilityExamplesWaitUncounted(t *testing.T) {
	env := newExampleSyncEnv(t)
	ctx := context.Background()
	env.engine.SetGRPCClient(&GRPCClient{sync: env.client, auth: &capabilityAuthClient{capabilities: []string{"desktop_signin"}}})
	env.ws.refreshCapability(ctx, false)
	require.Equal(t, capUnsupported, env.ws.examplesCap)

	for i := range 100 {
		env.createExample(t, fmt.Sprintf("Example %d", i))
	}
	env.req.Version++
	require.NoError(t, env.requests.Update(ctx, env.req))

	require.NoError(t, env.ws.pushAll(ctx))

	assert.Equal(t, 1, env.client.pushesOf(env.req.ID.String()), "the request goes out without its examples")
	for _, batch := range env.client.pushed() {
		for _, e := range batch {
			assert.NotEqual(t, syncv1.EntityType_ENTITY_TYPE_RESPONSE_EXAMPLE, e.GetEntityType())
		}
	}
	var waiting int
	require.NoError(t, env.db.QueryRow(
		`SELECT COUNT(*) FROM sync_queue WHERE entity_type = 'response_example' AND status = 'pending'`).Scan(&waiting))
	assert.Equal(t, 100, waiting)
	owed, err := env.engine.GetPendingCount(ctx, env.ws.localWorkspaceID)
	require.NoError(t, err)
	assert.Zero(t, owed)
}

func TestDrainOutbox_TombstoneDatingFollowsCapability(t *testing.T) {
	cases := []struct {
		name         string
		capabilities []string
		serverDated  bool
	}{
		{"server dates tombstones", []string{"response_examples"}, true},
		{"older server", nil, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			env := newExampleSyncEnv(t)
			ctx := context.Background()
			env.engine.SetGRPCClient(&GRPCClient{sync: env.client, auth: &capabilityAuthClient{capabilities: tc.capabilities}})
			env.ws.refreshCapability(ctx, false)
			ex := env.createExample(t, "Gone")
			require.NoError(t, env.usecase.Delete(ctx, example.DeleteOpt{ExampleID: ex.ID, UserID: "test_user", Version: ex.Version}))
			collectionID, _ := enqueuePending(t, env.engine, env.ws.localWorkspaceID)

			require.NoError(t, env.ws.pushAll(ctx))

			sent := map[string]*syncv1.SyncEntity{}
			for _, batch := range env.client.pushed() {
				for _, e := range batch {
					sent[e.GetEntityId()] = e
				}
			}
			tombstone := sent[collectionID]
			require.NotNil(t, tombstone)
			require.True(t, tombstone.GetIsDeleted())
			if !tc.serverDated {
				require.True(t, tombstone.HasUpdatedAt(), "an older server needs the push time to let the delete win")
				assert.WithinDuration(t, time.Now(), tombstone.GetUpdatedAt().AsTime(), 2*time.Second)
				assert.NotContains(t, sent, ex.ID.String(), "an older server never sees examples")
				return
			}
			assert.False(t, tombstone.HasUpdatedAt())
			require.Contains(t, sent, ex.ID.String())
			assert.True(t, sent[ex.ID.String()].GetIsDeleted())
			assert.False(t, sent[ex.ID.String()].HasUpdatedAt())
		})
	}
}

func TestDrainOutbox_TooLargeRejectParksTheEntry(t *testing.T) {
	env := newExampleSyncEnv(t)
	ctx := context.Background()
	events := captureEvents(env.engine)
	ex := env.createExample(t, "Huge")
	env.req.Version++
	require.NoError(t, env.requests.Update(ctx, env.req))
	env.client.respond = func(_ int, req *syncv1.PushRequest) (*syncv1.PushResponse, error) {
		results := make([]*syncv1.PushResult, 0, len(req.GetEntities()))
		for _, e := range req.GetEntities() {
			if e.GetEntityId() == ex.ID.String() {
				results = append(results, rejectedResult(e.GetEntityId(), syncv1.PushRejectReason_PUSH_REJECT_REASON_TOO_LARGE))
				continue
			}
			results = append(results, syncv1.PushResult_builder{EntityId: e.GetEntityId(), Status: syncv1.PushStatus_PUSH_STATUS_ACCEPTED}.Build())
		}
		return syncv1.PushResponse_builder{Results: results}.Build(), nil
	}

	require.NoError(t, env.ws.pushAll(ctx))

	rows := queueRowsOf(t, env.db, ex.ID.String())
	require.Len(t, rows, 1)
	assert.Equal(t, "parked", rows[0].status)
	assert.Empty(t, queueRowsOf(t, env.db, env.req.ID.String()))
	assert.Equal(t, 1, env.client.pushesOf(ex.ID.String()), "a parked entry is not pushed again in the same drain")
	tooLarge, err := env.engine.GetTooLargeCount(ctx, env.ws.localWorkspaceID)
	require.NoError(t, err)
	assert.Equal(t, 1, tooLarge)
	data := lastEventData(*events, "sync:rejected")
	require.NotNil(t, data)
	assert.Equal(t, "too_large", data["reason"])
}

func TestSyncer_PushSignalsReuseTheCycleCapability(t *testing.T) {
	restore := capabilityRecheckInterval
	capabilityRecheckInterval = 20 * time.Millisecond
	t.Cleanup(func() { capabilityRecheckInterval = restore })

	env := newExampleSyncEnv(t)
	ctx := context.Background()
	info := &capabilityAuthClient{capabilities: []string{"desktop_signin"}}
	client, _ := env.startCycling(t, info)
	require.Eventually(t, func() bool { return client.subscribes.Load() == 1 }, 5*time.Second, 10*time.Millisecond)

	for i := 1; i <= 3; i++ {
		env.req.Version++
		require.NoError(t, env.requests.Update(ctx, env.req))
		require.Eventually(t, func() bool { return client.pushesOf(env.req.ID.String()) == i }, 5*time.Second, 10*time.Millisecond)
	}

	assert.Equal(t, int32(1), info.calls.Load(), "a push inside the stream must not ask the server again")
	assert.Never(t, func() bool { return client.subscribes.Load() > 1 }, 200*time.Millisecond, 10*time.Millisecond,
		"an unsupported server arms no recheck: a server change drops the stream anyway")
}

func TestSyncer_CapabilityIsReadAgainAfterTheStreamDrops(t *testing.T) {
	env := newExampleSyncEnv(t)
	ctx := context.Background()
	ex := env.createExample(t, "Held back")
	info := &capabilityAuthClient{answer: func(_ context.Context, call int32) (*authv1.GetServerInfoResponse, error) {
		if call == 1 {
			return serverInfoWith("desktop_signin"), nil
		}
		return serverInfoWith("desktop_signin", "response_examples"), nil
	}}
	client, _ := env.startCycling(t, info)
	require.Eventually(t, func() bool { return client.subscribes.Load() == 1 }, 5*time.Second, 10*time.Millisecond)

	assert.Zero(t, client.pushesOf(ex.ID.String()))
	owed, err := env.engine.GetPendingCount(ctx, env.ws.localWorkspaceID)
	require.NoError(t, err)
	assert.Zero(t, owed)

	client.drop <- struct{}{}

	require.Eventually(t, func() bool { return client.pushesOf(ex.ID.String()) == 1 },
		initialBackoff+10*time.Second, 20*time.Millisecond, "the upgraded server gets the example on reconnect")
	assert.Equal(t, int32(2), info.calls.Load())
}

func TestSyncer_UnknownCapabilityRechecksOnATimer(t *testing.T) {
	restore := capabilityRecheckInterval
	capabilityRecheckInterval = 50 * time.Millisecond
	t.Cleanup(func() { capabilityRecheckInterval = restore })

	env := newExampleSyncEnv(t)
	ex := env.createExample(t, "Waiting")
	info := &capabilityAuthClient{answer: func(_ context.Context, call int32) (*authv1.GetServerInfoResponse, error) {
		if call == 1 {
			return nil, status.Error(codes.Unavailable, "server info unreachable")
		}
		return serverInfoWith("response_examples"), nil
	}}
	client, states := env.startCycling(t, info)

	require.Eventually(t, func() bool { return client.pushesOf(ex.ID.String()) == 1 }, 5*time.Second, 10*time.Millisecond)
	require.Eventually(t, func() bool { return client.subscribes.Load() == 2 }, 5*time.Second, 10*time.Millisecond,
		"the recheck reopens the stream itself")
	assert.Never(t, func() bool { return client.subscribes.Load() > 2 }, 200*time.Millisecond, 10*time.Millisecond,
		"a confirmed capability stops the rechecks")
	assert.Equal(t, int32(2), info.calls.Load())
	assert.NotContains(t, states.all(), string(StateOffline), "a recheck is not a connectivity failure")
}

func TestSyncer_ReconnectAsksARolledBackServerAgain(t *testing.T) {
	env := newExampleSyncEnv(t)
	shipped := env.createExample(t, "Shipped")
	server := &rollbackServer{}
	info := server.info()
	client := newCycleClient()
	client.respond = server.push
	env.engine.SetGRPCClient(&GRPCClient{sync: client, auth: info})
	states := env.startSyncer(t)
	require.Eventually(t, func() bool { return client.subscribes.Load() == 1 }, 5*time.Second, 10*time.Millisecond)
	require.Equal(t, 1, client.pushesOf(shipped.ID.String()))

	server.rolledBack.Store(true)
	client.drop <- struct{}{}
	require.Eventually(t, func() bool { return slices.Contains(states.all(), string(StateOffline)) }, 5*time.Second, 10*time.Millisecond)
	held, tombstoneID := env.queueAfterRollback(t)

	env.requireServedAsOlder(t, client, held, tombstoneID)
	require.Eventually(t, func() bool { return client.subscribes.Load() == 2 }, 5*time.Second, 10*time.Millisecond)
	assert.Equal(t, int32(2), info.calls.Load())
}

func TestSyncer_RetryAfterAFailedPullAsksAgain(t *testing.T) {
	env := newExampleSyncEnv(t)
	server := &rollbackServer{}
	info := server.info()
	client := newCycleClient()
	client.respond = server.push
	client.failPulls.Store(1)
	env.engine.SetGRPCClient(&GRPCClient{sync: client, auth: info})
	states := env.startSyncer(t)
	require.Eventually(t, func() bool { return slices.Contains(states.all(), string(StateOffline)) }, 5*time.Second, 10*time.Millisecond)
	require.Equal(t, int32(1), info.calls.Load())

	server.rolledBack.Store(true)
	held, tombstoneID := env.queueAfterRollback(t)

	env.requireServedAsOlder(t, client, held, tombstoneID)
	require.Eventually(t, func() bool { return client.subscribes.Load() == 1 }, 5*time.Second, 10*time.Millisecond)
	assert.Equal(t, int32(2), info.calls.Load())
}

func TestSyncer_StartAsksAgainDespiteAConfirmedAnswer(t *testing.T) {
	env := newExampleSyncEnv(t)
	server := &rollbackServer{}
	info := server.info()
	client := newCycleClient()
	client.respond = server.push
	env.engine.SetGRPCClient(&GRPCClient{sync: client, auth: info})
	require.Equal(t, capSupported, env.engine.examplesCapability(context.Background()))
	server.rolledBack.Store(true)
	held, tombstoneID := env.queueAfterRollback(t)

	env.startSyncer(t)

	env.requireServedAsOlder(t, client, held, tombstoneID)
	assert.Equal(t, int32(2), info.calls.Load())
}

type cycleClient struct {
	recordingPushClient
	subscribes atomic.Int32
	drop       chan struct{}
	failPulls  atomic.Int32
}

func newCycleClient() *cycleClient {
	return &cycleClient{drop: make(chan struct{}, 1)}
}

func (c *cycleClient) Pull(context.Context, *syncv1.PullRequest, ...grpc.CallOption) (*syncv1.PullResponse, error) {
	if c.failPulls.Add(-1) >= 0 {
		return nil, status.Error(codes.Unavailable, "server went down")
	}
	return syncv1.PullResponse_builder{}.Build(), nil
}

func (c *cycleClient) Subscribe(ctx context.Context, _ *syncv1.SubscribeRequest, _ ...grpc.CallOption) (grpc.ServerStreamingClient[syncv1.SubscribeResponse], error) {
	c.subscribes.Add(1)
	return &droppableStream{ctx: ctx, drop: c.drop}, nil
}

type droppableStream struct {
	grpc.ClientStream
	ctx  context.Context
	drop <-chan struct{}
}

func (s *droppableStream) Recv() (*syncv1.SubscribeResponse, error) {
	select {
	case <-s.ctx.Done():
		return nil, s.ctx.Err()
	case <-s.drop:
		return nil, status.Error(codes.Unavailable, "server restarted")
	}
}

type stateLog struct {
	mu     gosync.Mutex
	states []string
}

func (l *stateLog) all() []string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return slices.Clone(l.states)
}

func (env *exampleSyncEnv) startCycling(t *testing.T, info *capabilityAuthClient) (*cycleClient, *stateLog) {
	t.Helper()
	client := newCycleClient()
	env.engine.SetGRPCClient(&GRPCClient{sync: client, auth: info})
	return client, env.startSyncer(t)
}

func (env *exampleSyncEnv) startSyncer(t *testing.T) *stateLog {
	t.Helper()
	return env.startSyncerAt(t, 0)
}

func (env *exampleSyncEnv) startSyncerAt(t *testing.T, lastSyncSeq int64) *stateLog {
	t.Helper()
	states := &stateLog{}
	env.engine.SetEventEmitter(func(name string, data any) {
		if name != "sync:status" {
			return
		}
		payload, _ := data.(map[string]any)
		state, _ := payload["state"].(string)
		states.mu.Lock()
		states.states = append(states.states, state)
		states.mu.Unlock()
	})
	env.engine.StartWorkspace(testWorkspaceID.String(), "remote-workspace", lastSyncSeq)
	t.Cleanup(func() { require.True(t, env.engine.StopAll()) })
	return states
}

type rollbackServer struct {
	rolledBack atomic.Bool
}

func (s *rollbackServer) info() *capabilityAuthClient {
	return &capabilityAuthClient{answer: func(context.Context, int32) (*authv1.GetServerInfoResponse, error) {
		if s.rolledBack.Load() {
			return serverInfoWith("desktop_signin"), nil
		}
		return serverInfoWith("desktop_signin", "response_examples"), nil
	}}
}

func (s *rollbackServer) push(_ int, req *syncv1.PushRequest) (*syncv1.PushResponse, error) {
	if s.rolledBack.Load() {
		for _, e := range req.GetEntities() {
			if e.GetEntityType() == syncv1.EntityType_ENTITY_TYPE_RESPONSE_EXAMPLE {
				return nil, status.Error(codes.Unknown, "push: unsupported entity type")
			}
		}
	}
	return acceptAll(req), nil
}

func (env *exampleSyncEnv) queueAfterRollback(t *testing.T) (held *entities.ResponseExample, tombstoneID string) {
	t.Helper()
	held = env.createExample(t, "Held back")
	env.req.Version++
	require.NoError(t, env.requests.Update(context.Background(), env.req))
	tombstoneID, _ = enqueuePending(t, env.engine, env.ws.localWorkspaceID)
	return held, tombstoneID
}

func (env *exampleSyncEnv) requireServedAsOlder(t *testing.T, client *cycleClient, held *entities.ResponseExample, tombstoneID string) {
	t.Helper()
	require.Eventually(t, func() bool { return queuedRows(env.db, env.req.ID.String()) == 0 },
		initialBackoff+10*time.Second, 20*time.Millisecond, "the request must not wait on its example")

	assert.Zero(t, client.pushesOf(held.ID.String()))
	var tombstone *syncv1.SyncEntity
	for _, batch := range client.pushed() {
		for _, e := range batch {
			if e.GetEntityId() == tombstoneID {
				tombstone = e
			}
		}
	}
	require.NotNil(t, tombstone)
	assert.True(t, tombstone.HasUpdatedAt(), "an older server needs the push time to let the delete win")
	owed, err := env.engine.GetPendingCount(context.Background(), env.ws.localWorkspaceID)
	require.NoError(t, err)
	assert.Zero(t, owed)
}

func queuedRows(db *sql.DB, entityID string) int {
	var n int
	if err := db.QueryRow(`SELECT COUNT(*) FROM sync_queue WHERE entity_id = ?`, entityID).Scan(&n); err != nil {
		return -1
	}
	return n
}

func enqueueExampleRow(t *testing.T, engine *SyncEngine, wsID string) {
	t.Helper()
	require.NoError(t, engine.syncQueue.Enqueue(context.Background(), sqlite.SyncEntry{
		WorkspaceID: wsID,
		EntityType:  "response_example",
		EntityID:    uuid.NewString(),
		Action:      "create",
		OperationID: uuid.NewString(),
		Status:      "pending",
		CreatedAt:   time.Now().Truncate(time.Second),
	}))
}
