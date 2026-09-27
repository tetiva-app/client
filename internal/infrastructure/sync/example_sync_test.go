package sync

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	gosync "sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	syncv1 "github.com/tetiva-app/proto/go/gophercourier/sync/v1"

	"github.com/tetiva-app/client/internal/domain"
	"github.com/tetiva-app/client/internal/domain/entities"
	"github.com/tetiva-app/client/internal/domain/usecase/example"
	"github.com/tetiva-app/client/internal/infrastructure/repository/sqlite"
)

// recordingPushClient records every push and answers through respond, ACCEPTED for all by default.
type recordingPushClient struct {
	syncv1.SyncServiceClient
	mu      gosync.Mutex
	batches [][]*syncv1.SyncEntity
	respond func(call int, req *syncv1.PushRequest) (*syncv1.PushResponse, error)
}

func (c *recordingPushClient) Push(_ context.Context, req *syncv1.PushRequest, _ ...grpc.CallOption) (*syncv1.PushResponse, error) {
	c.mu.Lock()
	c.batches = append(c.batches, req.GetEntities())
	call := len(c.batches)
	c.mu.Unlock()
	if c.respond != nil {
		return c.respond(call, req)
	}
	return acceptAll(req), nil
}

func (c *recordingPushClient) pushed() [][]*syncv1.SyncEntity {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([][]*syncv1.SyncEntity(nil), c.batches...)
}

func (c *recordingPushClient) pushesOf(entityID string) int {
	n := 0
	for _, b := range c.pushed() {
		for _, e := range b {
			if e.GetEntityId() == entityID {
				n++
			}
		}
	}
	return n
}

func acceptAll(req *syncv1.PushRequest) *syncv1.PushResponse {
	results := make([]*syncv1.PushResult, 0, len(req.GetEntities()))
	for _, e := range req.GetEntities() {
		results = append(results, syncv1.PushResult_builder{
			EntityId: e.GetEntityId(),
			Status:   syncv1.PushStatus_PUSH_STATUS_ACCEPTED,
		}.Build())
	}
	return syncv1.PushResponse_builder{Results: results}.Build()
}

// exampleSyncEnv is a syncing workspace over real SQLite: inner repos for the engine,
// sync decorators for the usecases, one live collection and request.
type exampleSyncEnv struct {
	db          *sql.DB
	engine      *SyncEngine
	ws          *workspaceSyncer
	client      *recordingPushClient
	collections *SyncedCollectionRepo
	requests    *SyncedRequestRepo
	examples    *SyncedResponseExampleRepo
	usecase     example.Usecase
	coll        *entities.Collection
	req         *entities.Request
}

func newExampleSyncEnv(t *testing.T) *exampleSyncEnv {
	t.Helper()
	db := setupTestDB(t)
	ctx := context.Background()
	configRepo := sqlite.NewSyncConfigRepo(db)
	queue := sqlite.NewSyncQueueRepo(db)
	innerCols := sqlite.NewCollectionRepo(db)
	innerReqs := sqlite.NewRequestRepo(db)
	innerExamples := sqlite.NewResponseExampleRepo(db)

	engine := NewSyncEngine(NewSyncAuthManager(configRepo), queue, configRepo, db,
		innerCols, innerReqs, sqlite.NewEnvironmentRepo(db), sqlite.NewVariableRepo(db), innerExamples, nil)
	client := &recordingPushClient{}
	engine.SetGRPCClient(examplesCapableClient(client))
	engine.auth.storeTokens("access-1", "", "org-1")
	_, err := configRepo.GetOrCreate(ctx)
	require.NoError(t, err)

	ws, cancel := newSyncer(engine, testWorkspaceID.String(), StateConnected)
	t.Cleanup(func() {
		ws.stopRequeueTimer()
		cancel()
	})
	ws.refreshCapability(ctx, false)
	require.Equal(t, capSupported, ws.examplesCap)

	env := &exampleSyncEnv{
		db:          db,
		engine:      engine,
		ws:          ws,
		client:      client,
		collections: NewSyncedCollectionRepo(innerCols, queue, db, engine),
		requests:    NewSyncedRequestRepo(innerReqs, queue, db, engine),
		examples:    NewSyncedResponseExampleRepo(innerExamples, queue, db, engine),
		coll:        newTestCollection("Host", nil),
	}
	env.usecase = example.NewUsecase(env.examples, env.requests, env.collections)
	require.NoError(t, innerCols.Create(ctx, env.coll))
	env.req = newLocalRequest("Get user", env.coll.ID)
	require.NoError(t, innerReqs.Create(ctx, env.req))
	return env
}

func (env *exampleSyncEnv) createExample(t *testing.T, name string, headers ...entities.HeaderItem) *entities.ResponseExample {
	t.Helper()
	ex, err := env.usecase.Create(context.Background(), example.Create{
		RequestID: env.req.ID, Name: name, StatusCode: 200, StatusText: "OK",
		Headers: headers, Body: `{"ok":true}`, ContentType: "application/json", Protocol: entities.ProtocolHTTP,
	}, example.CreateOpt{UserID: "test_user"})
	require.NoError(t, err)
	return ex
}

func (env *exampleSyncEnv) addWorkspace(t *testing.T) uuid.UUID {
	t.Helper()
	id := uuid.New()
	_, err := env.db.Exec(`INSERT INTO workspaces (id, name) VALUES (?, 'Other')`, id.String())
	require.NoError(t, err)
	return id
}

func exampleIsSynced(t *testing.T, db *sql.DB, id uuid.UUID) int {
	t.Helper()
	var synced int
	require.NoError(t, db.QueryRow(`SELECT is_synced FROM response_examples WHERE id = ?`, id.String()).Scan(&synced))
	return synced
}

type queueRow struct {
	workspaceID, action, status string
	retryCount                  int
}

func queueRowsOf(t *testing.T, db *sql.DB, entityID string) []queueRow {
	t.Helper()
	rows, err := db.Query(`SELECT workspace_id, action, status, retry_count FROM sync_queue WHERE entity_id = ? ORDER BY id`, entityID)
	require.NoError(t, err)
	defer func() { _ = rows.Close() }()
	var out []queueRow
	for rows.Next() {
		var r queueRow
		require.NoError(t, rows.Scan(&r.workspaceID, &r.action, &r.status, &r.retryCount))
		out = append(out, r)
	}
	require.NoError(t, rows.Err())
	return out
}

func exampleEntity(id, requestID uuid.UUID, name string, version int32) *syncv1.SyncEntity {
	return syncv1.SyncEntity_builder{
		EntityType: syncv1.EntityType_ENTITY_TYPE_RESPONSE_EXAMPLE,
		EntityId:   id.String(),
		Version:    version,
		ResponseExample: syncv1.ResponseExampleData_builder{
			RequestId:  requestID.String(),
			Name:       name,
			StatusCode: 201,
			Headers:    []*syncv1.HeaderItem{syncv1.HeaderItem_builder{Key: "X-Id", Value: "7", Enabled: true}.Build()},
			Body:       "{}",
			Protocol:   "http",
			SortOrder:  4,
		}.Build(),
	}.Build()
}

func TestApplyEntity_ExampleArrivesBeforeItsRequest(t *testing.T) {
	env := newExampleSyncEnv(t)
	ctx := context.Background()
	exampleID, requestID := uuid.New(), uuid.New()

	require.NoError(t, env.ws.applyEntity(ctx, exampleEntity(exampleID, requestID, "Created", 1)))

	var name, workspaceID string
	var sortOrder, isDelete int
	require.NoError(t, env.db.QueryRow(
		`SELECT name, workspace_id, sort_order, is_delete FROM response_examples WHERE id = ? AND request_id = ?`,
		exampleID.String(), requestID.String()).Scan(&name, &workspaceID, &sortOrder, &isDelete))
	assert.Equal(t, "Created", name)
	assert.Equal(t, testWorkspaceID.String(), workspaceID)
	assert.Equal(t, 4, sortOrder)
	assert.Zero(t, isDelete)
	assert.Equal(t, 1, exampleIsSynced(t, env.db, exampleID), "what the server sent is not an unsynced local write")
}

func TestApplyEntity_InboundExampleReplacesLocalEditAndMarksSynced(t *testing.T) {
	env := newExampleSyncEnv(t)
	ctx := context.Background()
	local := env.createExample(t, "Local")
	require.Zero(t, exampleIsSynced(t, env.db, local.ID))

	require.NoError(t, env.ws.applyEntity(ctx, exampleEntity(local.ID, env.req.ID, "Remote", 5)))

	got, err := sqlite.NewResponseExampleRepo(env.db).GetByID(ctx, local.ID)
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, "Remote", got.Name)
	assert.Equal(t, 5, got.Version)
	assert.Equal(t, 1, exampleIsSynced(t, env.db, local.ID))
}

func TestApplyEntity_InboundExampleHeadersAreStoredMasked(t *testing.T) {
	env := newExampleSyncEnv(t)
	ctx := context.Background()
	e := exampleEntity(uuid.New(), env.req.ID, "From a buggy peer", 1)
	e.GetResponseExample().SetHeaders([]*syncv1.HeaderItem{
		syncv1.HeaderItem_builder{Key: "Authorization", Value: "Bearer live", Enabled: true}.Build(),
	})

	require.NoError(t, env.ws.applyEntity(ctx, e))

	got, err := sqlite.NewResponseExampleRepo(env.db).GetByID(ctx, uuid.MustParse(e.GetEntityId()))
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, "Bearer <redacted>", got.Headers[0].Value)
}

type exampleContentRow struct {
	requestID, name, statusText, headers, body, contentType, protocol string
	statusCode, sortOrder                                             int
}

func exampleContent(t *testing.T, db *sql.DB, id uuid.UUID) exampleContentRow {
	t.Helper()
	var r exampleContentRow
	require.NoError(t, db.QueryRow(
		`SELECT request_id, name, status_code, status_text, headers, body, content_type, protocol, sort_order
		FROM response_examples WHERE id = ?`, id.String()).
		Scan(&r.requestID, &r.name, &r.statusCode, &r.statusText, &r.headers, &r.body, &r.contentType, &r.protocol, &r.sortOrder))
	return r
}

func TestApplyEntity_ExampleTombstoneOnlyMarksTheRow(t *testing.T) {
	stamp := time.Date(2026, 9, 20, 12, 0, 0, 0, time.UTC)
	shapes := map[string]func(id, requestID uuid.UUID) *syncv1.ResponseExampleData{
		"bare": func(uuid.UUID, uuid.UUID) *syncv1.ResponseExampleData { return nil },
		// Server 0.19 keeps only request_id in tombstone data but still builds the payload.
		"server request_id only": func(_, requestID uuid.UUID) *syncv1.ResponseExampleData {
			return syncv1.ResponseExampleData_builder{RequestId: requestID.String()}.Build()
		},
		"full payload": func(id, requestID uuid.UUID) *syncv1.ResponseExampleData {
			return exampleEntity(id, requestID, "Remote name", 3).GetResponseExample()
		},
	}
	for name, payload := range shapes {
		t.Run(name, func(t *testing.T) {
			env := newExampleSyncEnv(t)
			ctx := context.Background()
			local := env.createExample(t, "Keep my body", entities.HeaderItem{Key: "X-Trace", Value: "1", Enabled: true})
			before := exampleContent(t, env.db, local.ID)

			tombstone := func(id uuid.UUID) *syncv1.SyncEntity {
				return syncv1.SyncEntity_builder{
					EntityType:      syncv1.EntityType_ENTITY_TYPE_RESPONSE_EXAMPLE,
					EntityId:        id.String(),
					IsDeleted:       true,
					Version:         9,
					UpdatedAt:       timestamppb.New(stamp),
					ResponseExample: payload(id, env.req.ID),
				}.Build()
			}
			require.NoError(t, env.ws.applyEntity(ctx, tombstone(local.ID)))

			assert.Equal(t, before, exampleContent(t, env.db, local.ID), "a tombstone must not overwrite the content")
			var isDelete, version int
			var updatedAt string
			require.NoError(t, env.db.QueryRow(
				`SELECT is_delete, version, updated_at FROM response_examples WHERE id = ?`, local.ID.String()).
				Scan(&isDelete, &version, &updatedAt))
			assert.Equal(t, 1, isDelete)
			assert.Equal(t, 9, version)
			assert.Equal(t, stamp.Format(time.RFC3339), updatedAt)
			assert.Equal(t, 1, exampleIsSynced(t, env.db, local.ID))

			absent := uuid.New()
			require.NoError(t, env.ws.applyEntity(ctx, tombstone(absent)))
			var n int
			require.NoError(t, env.db.QueryRow(`SELECT COUNT(*) FROM response_examples WHERE id = ?`, absent.String()).Scan(&n))
			assert.Zero(t, n, "a tombstone of an unknown example has nothing to mark")
		})
	}
}

func TestPushAll_ExampleSyncedFlagFollowsAckAndConflictWinner(t *testing.T) {
	env := newExampleSyncEnv(t)
	ctx := context.Background()
	acked := env.createExample(t, "Acked")
	lost := env.createExample(t, "Lost the race")
	require.Zero(t, exampleIsSynced(t, env.db, acked.ID), "a local write through the decorator is unsynced")

	env.client.respond = func(_ int, req *syncv1.PushRequest) (*syncv1.PushResponse, error) {
		results := make([]*syncv1.PushResult, 0, len(req.GetEntities()))
		for _, e := range req.GetEntities() {
			if e.GetEntityId() == lost.ID.String() {
				results = append(results, syncv1.PushResult_builder{
					EntityId: e.GetEntityId(),
					Status:   syncv1.PushStatus_PUSH_STATUS_CONFLICT_RESOLVED,
					Winner:   exampleEntity(lost.ID, env.req.ID, "Winner", 4),
				}.Build())
				continue
			}
			results = append(results, syncv1.PushResult_builder{EntityId: e.GetEntityId(), Status: syncv1.PushStatus_PUSH_STATUS_ACCEPTED}.Build())
		}
		return syncv1.PushResponse_builder{Results: results}.Build(), nil
	}

	require.NoError(t, env.ws.pushAll(ctx))

	assert.Equal(t, 1, exampleIsSynced(t, env.db, acked.ID))
	assert.Equal(t, 1, exampleIsSynced(t, env.db, lost.ID))
	winner, err := sqlite.NewResponseExampleRepo(env.db).GetByID(ctx, lost.ID)
	require.NoError(t, err)
	assert.Equal(t, "Winner", winner.Name)
	assert.Empty(t, queueRowsOf(t, env.db, acked.ID.String()))
}

func TestSyncedResponseExampleRepo_EnqueuesWhileSyncing(t *testing.T) {
	env := newExampleSyncEnv(t)
	ctx := context.Background()

	ex := env.createExample(t, "Queued")
	ex.Name = "Renamed"
	ex.Version++
	require.NoError(t, env.examples.Update(ctx, ex))
	ex.IsDelete = true
	ex.Version++
	require.NoError(t, env.examples.Update(ctx, ex))

	rows := queueRowsOf(t, env.db, ex.ID.String())
	require.Len(t, rows, 3)
	for i, action := range []string{"create", "update", "delete"} {
		assert.Equal(t, action, rows[i].action)
		assert.Equal(t, testWorkspaceID.String(), rows[i].workspaceID)
	}

	offline := NewSyncedResponseExampleRepo(sqlite.NewResponseExampleRepo(env.db), sqlite.NewSyncQueueRepo(env.db), env.db, nil)
	quiet := *ex
	quiet.ID = uuid.New()
	quiet.IsDelete = false
	require.NoError(t, offline.Create(ctx, &quiet))
	assert.Empty(t, queueRowsOf(t, env.db, quiet.ID.String()), "a workspace that does not sync queues nothing")
}

func TestSyncedResponseExampleRepo_VersionedWritesQueueOnlyWhatLanded(t *testing.T) {
	env := newExampleSyncEnv(t)
	ctx := context.Background()
	ex := env.createExample(t, "Base")

	stale := *ex
	stale.Name, stale.Version = "Stale", ex.Version+1
	var conflict *domain.ConflictError
	require.ErrorAs(t, env.examples.UpdateAtVersion(ctx, &stale, ex.Version-1), &conflict)
	require.Len(t, queueRowsOf(t, env.db, ex.ID.String()), 1, "a refused write queues nothing")

	_, err := env.usecase.Edit(ctx, example.Edit{Name: "Edited", StatusCode: 200},
		example.EditOpt{ExampleID: ex.ID, UserID: "u", Version: ex.Version})
	require.NoError(t, err)
	require.NoError(t, env.usecase.Delete(ctx, example.DeleteOpt{ExampleID: ex.ID, UserID: "u", Version: ex.Version + 1}))

	rows := queueRowsOf(t, env.db, ex.ID.String())
	require.Len(t, rows, 3)
	assert.Equal(t, "update", rows[1].action)
	assert.Equal(t, "delete", rows[2].action)
}

func TestSyncedResponseExampleRepo_MoveRoutesEachHalfToItsWorkspace(t *testing.T) {
	env := newExampleSyncEnv(t)
	ctx := context.Background()
	otherWS := env.addWorkspace(t)
	other, cancel := newSyncer(env.engine, otherWS.String(), StateConnected)
	t.Cleanup(func() {
		other.stopRequeueTimer()
		cancel()
	})
	original := env.createExample(t, "Moving")

	require.NoError(t, env.usecase.MoveToWorkspace(ctx, env.req.ID, otherWS, "test_user"))

	rows := queueRowsOf(t, env.db, original.ID.String())
	require.Len(t, rows, 2)
	assert.Equal(t, queueRow{workspaceID: testWorkspaceID.String(), action: "delete", status: "pending"}, rows[1],
		"the original's tombstone belongs to the workspace it left")

	var copyID string
	require.NoError(t, env.db.QueryRow(`SELECT id FROM response_examples WHERE workspace_id = ?`, otherWS.String()).Scan(&copyID))
	copyRows := queueRowsOf(t, env.db, copyID)
	require.Len(t, copyRows, 1)
	assert.Equal(t, queueRow{workspaceID: otherWS.String(), action: "create", status: "pending"}, copyRows[0])
}

func TestSyncedResponseExampleRepo_NotifiesOnlyAfterCommit(t *testing.T) {
	env := newExampleSyncEnv(t)
	ctx := context.Background()
	ex := env.createExample(t, "Signalled")
	<-env.ws.pushSignal

	ex.IsDelete = true
	ex.Version++
	err := sqlite.WithTx(ctx, env.db, func(txCtx context.Context) error {
		if err := env.examples.Update(txCtx, ex); err != nil {
			return err
		}
		assert.Empty(t, env.ws.pushSignal, "push signalled before the outer transaction committed")
		return nil
	})
	require.NoError(t, err)
	assert.Len(t, env.ws.pushSignal, 1)
	<-env.ws.pushSignal

	boom := errors.New("rollback")
	ex.Version++
	err = sqlite.WithTx(ctx, env.db, func(txCtx context.Context) error {
		if err := env.examples.Update(txCtx, ex); err != nil {
			return err
		}
		return boom
	})
	require.ErrorIs(t, err, boom)
	assert.Empty(t, env.ws.pushSignal, "a rolled-back write must not wake the syncer")
	assert.Len(t, queueRowsOf(t, env.db, ex.ID.String()), 2)
}

func TestDecorators_NotifyOnlyAfterCommit(t *testing.T) {
	env := newExampleSyncEnv(t)
	ctx := context.Background()
	envRepo := NewSyncedEnvironmentRepo(sqlite.NewEnvironmentRepo(env.db), env.engine.syncQueue, env.db, env.engine)
	varRepo := NewSyncedVariableRepo(sqlite.NewVariableRepo(env.db), env.engine.syncQueue, env.db, env.engine)
	now := time.Now().Truncate(time.Second)
	environmentID := uuid.New()

	writes := map[string]func(context.Context) error{
		"collection": func(txCtx context.Context) error {
			return env.collections.Create(txCtx, newTestCollection("New", nil))
		},
		"collection descendants": func(txCtx context.Context) error {
			child := newTestCollection("Child", &env.coll.ID)
			if err := sqlite.NewCollectionRepo(env.db).Create(txCtx, child); err != nil {
				return err
			}
			return env.collections.SoftDeleteDescendants(txCtx, env.coll.ID, "test_user", now)
		},
		"environment": func(txCtx context.Context) error {
			return envRepo.Create(txCtx, &entities.Environment{
				ID: environmentID, WorkspaceID: testWorkspaceID, Name: "Dev", Version: 1, CreatedAt: now, UpdatedAt: now,
			})
		},
		"variable": func(txCtx context.Context) error {
			return varRepo.Create(txCtx, &entities.Variable{
				ID: uuid.New(), EnvironmentID: environmentID, Key: "k", Version: 1, CreatedAt: now, UpdatedAt: now,
			})
		},
	}
	for _, name := range []string{"collection", "collection descendants", "environment", "variable"} {
		err := sqlite.WithTx(ctx, env.db, func(txCtx context.Context) error {
			if err := writes[name](txCtx); err != nil {
				return err
			}
			assert.Empty(t, env.ws.pushSignal, "%s: push signalled before the outer commit", name)
			return nil
		})
		require.NoError(t, err, name)
		assert.Len(t, env.ws.pushSignal, 1, "%s: no signal after the commit", name)
		<-env.ws.pushSignal
	}
}

func TestDrainOutbox_RequestLeavesBeforeItsExamples(t *testing.T) {
	env := newExampleSyncEnv(t)
	ctx := context.Background()
	req := newLocalRequest("Imported", env.coll.ID)
	require.NoError(t, env.requests.Create(ctx, req))
	env.req = req

	body := strings.Repeat("x", 250*1024)
	for i := range 20 {
		_, err := env.usecase.Create(ctx, example.Create{
			RequestID: req.ID, Name: "Example " + string(rune('A'+i)), StatusCode: 200, Body: body, Protocol: entities.ProtocolHTTP,
		}, example.CreateOpt{UserID: "test_user"})
		require.NoError(t, err)
	}
	req.Name = "Imported and edited"
	req.Version++
	require.NoError(t, env.requests.Update(ctx, req))

	require.NoError(t, env.ws.pushAll(ctx))

	batches := env.client.pushed()
	require.GreaterOrEqual(t, len(batches), 2, "twenty 250 KiB examples do not fit one 3 MiB batch")
	assert.Equal(t, req.ID.String(), batches[0][0].GetEntityId(), "the parent must lead the first batch")
	examples := 0
	for _, b := range batches {
		for _, e := range b {
			if e.GetEntityType() == syncv1.EntityType_ENTITY_TYPE_RESPONSE_EXAMPLE {
				examples++
			}
		}
	}
	assert.Equal(t, 20, examples, "every example goes out exactly once")
	assert.Equal(t, 1, env.client.pushesOf(req.ID.String()))

	owed, err := env.engine.syncQueue.CountPendingOrFailed(ctx, env.ws.localWorkspaceID, nil)
	require.NoError(t, err)
	assert.Zero(t, owed)
}

func TestDrainOutbox_RepeatedEditsPushOnce(t *testing.T) {
	env := newExampleSyncEnv(t)
	ctx := context.Background()
	for range 5 {
		env.req.Version++
		require.NoError(t, env.requests.Update(ctx, env.req))
	}

	require.NoError(t, env.ws.pushAll(ctx))

	assert.Equal(t, 1, env.client.pushesOf(env.req.ID.String()), "the newest row stands for the request")
	assert.Empty(t, queueRowsOf(t, env.db, env.req.ID.String()))
}

func TestDrainOutbox_QuotaParkingKeepsOneRowAndTheConcurrentWrite(t *testing.T) {
	env := newExampleSyncEnv(t)
	ctx := context.Background()
	for range 3 {
		env.req.Version++
		require.NoError(t, env.requests.Update(ctx, env.req))
	}

	env.client.respond = func(call int, req *syncv1.PushRequest) (*syncv1.PushResponse, error) {
		if call > 1 {
			return nil, status.Error(codes.Unavailable, "offline")
		}
		env.req.Version++
		require.NoError(t, env.requests.Update(ctx, env.req))
		return syncv1.PushResponse_builder{Results: []*syncv1.PushResult{
			rejectedResult(env.req.ID.String(), syncv1.PushRejectReason_PUSH_REJECT_REASON_QUOTA_EXCEEDED),
		}}.Build(), nil
	}

	require.Error(t, env.ws.pushAll(ctx))

	rows := queueRowsOf(t, env.db, env.req.ID.String())
	require.Len(t, rows, 2)
	assert.Equal(t, "failed", rows[0].status, "one parked row stands for the three that went out")
	assert.Equal(t, "pending", rows[1].status, "the write made during the push is untouched")
}

func TestDrainOutbox_ParentNotFoundDefersExample(t *testing.T) {
	env := newExampleSyncEnv(t)
	ctx := context.Background()
	ex := env.createExample(t, "Orphan for now")
	env.client.respond = func(_ int, req *syncv1.PushRequest) (*syncv1.PushResponse, error) {
		results := make([]*syncv1.PushResult, 0, len(req.GetEntities()))
		for _, e := range req.GetEntities() {
			if e.GetEntityType() == syncv1.EntityType_ENTITY_TYPE_RESPONSE_EXAMPLE {
				results = append(results, rejectedResult(e.GetEntityId(), syncv1.PushRejectReason_PUSH_REJECT_REASON_PARENT_NOT_FOUND))
				continue
			}
			results = append(results, syncv1.PushResult_builder{EntityId: e.GetEntityId(), Status: syncv1.PushStatus_PUSH_STATUS_ACCEPTED}.Build())
		}
		return syncv1.PushResponse_builder{Results: results}.Build(), nil
	}
	wsID := env.ws.localWorkspaceID
	requeue := func() {
		t.Helper()
		n, err := env.engine.syncQueue.RequeueDue(ctx, wsID, time.Now().Add(quotaRetryDelay+time.Minute))
		require.NoError(t, err)
		require.Equal(t, int64(1), n)
	}

	require.NoError(t, env.ws.pushAll(ctx))

	rows := queueRowsOf(t, env.db, ex.ID.String())
	require.Len(t, rows, 1)
	assert.Equal(t, "deferred", rows[0].status)
	assert.Equal(t, 1, env.client.pushesOf(env.req.ID.String()), "the first miss re-offers the parent request")
	for name, count := range map[string]func(context.Context, string) (int, error){
		"parked": env.engine.GetParkedCount, "too large": env.engine.GetTooLargeCount,
	} {
		n, err := count(ctx, wsID)
		require.NoError(t, err)
		assert.Zero(t, n, "%s: a missing parent is neither a plan limit nor an oversized entity", name)
	}
	owed, err := env.engine.GetPendingCount(ctx, wsID)
	require.NoError(t, err)
	assert.Equal(t, 1, owed)

	for attempt := 2; attempt <= 4; attempt++ {
		requeue()
		require.NoError(t, env.ws.pushAll(ctx))
		rows = queueRowsOf(t, env.db, ex.ID.String())
		require.Len(t, rows, 1)
		assert.Equal(t, queueRow{workspaceID: wsID, action: "create", status: "deferred"}, rows[0])
		assert.Equal(t, attempt, deferCountOf(t, env.db, ex.ID.String()))
	}
	assert.Equal(t, 1, env.client.pushesOf(env.req.ID.String()), "only the first miss re-offers the request")

	requeue()
	require.NoError(t, env.ws.pushAll(ctx))
	assert.Empty(t, queueRowsOf(t, env.db, ex.ID.String()), "the fifth miss gives up on the example")
	assert.Equal(t, 5, env.client.pushesOf(ex.ID.String()))
}

func TestDrainOutbox_ParentNotFoundLeavesAQueuedRequestAlone(t *testing.T) {
	env := newExampleSyncEnv(t)
	ctx := context.Background()
	env.createExample(t, "Orphan for now")
	parkRequestRow(t, env.ws, env.req.ID)
	env.client.respond = func(_ int, req *syncv1.PushRequest) (*syncv1.PushResponse, error) {
		results := make([]*syncv1.PushResult, 0, len(req.GetEntities()))
		for _, e := range req.GetEntities() {
			results = append(results, rejectedResult(e.GetEntityId(), syncv1.PushRejectReason_PUSH_REJECT_REASON_PARENT_NOT_FOUND))
		}
		return syncv1.PushResponse_builder{Results: results}.Build(), nil
	}

	require.NoError(t, env.ws.pushAll(ctx))

	rows := queueRowsOf(t, env.db, env.req.ID.String())
	require.Len(t, rows, 1, "a request already waiting in the queue is not queued twice")
	assert.Equal(t, "parked", rows[0].status)
}

func TestBuildDeleteProto_AllTypes(t *testing.T) {
	types := map[string]syncv1.EntityType{
		"collection":       syncv1.EntityType_ENTITY_TYPE_COLLECTION,
		"request":          syncv1.EntityType_ENTITY_TYPE_REQUEST,
		"environment":      syncv1.EntityType_ENTITY_TYPE_ENVIRONMENT,
		"variable":         syncv1.EntityType_ENTITY_TYPE_VARIABLE,
		"response_example": syncv1.EntityType_ENTITY_TYPE_RESPONSE_EXAMPLE,
	}
	for name, want := range types {
		entry := &sqlite.SyncEntry{EntityType: name, EntityID: uuid.NewString(), OperationID: "op-" + name}

		dated := buildDeleteProto(entry, true)
		require.NotNil(t, dated, name)
		assert.Equal(t, want, dated.GetEntityType(), name)
		assert.Equal(t, entry.EntityID, dated.GetEntityId(), name)
		assert.Equal(t, entry.OperationID, dated.GetOperationId(), name)
		assert.True(t, dated.GetIsDeleted(), name)
		assert.False(t, dated.HasUpdatedAt(), "%s: the server dates the tombstone itself", name)

		stamped := buildDeleteProto(entry, false)
		require.True(t, stamped.HasUpdatedAt(), "%s: an older server needs the push time to let the delete win", name)
		assert.WithinDuration(t, time.Now(), stamped.GetUpdatedAt().AsTime(), 2*time.Second, name)
	}
	assert.Nil(t, buildDeleteProto(&sqlite.SyncEntry{EntityType: "folder"}, true))
}

func TestSubscribe_AppliesRequestTombstone(t *testing.T) {
	env := newExampleSyncEnv(t)
	ctx := context.Background()
	env.engine.SetGRPCClient(examplesCapableClient(&fakeSyncClient{
		stream: func() (grpc.ServerStreamingClient[syncv1.SubscribeResponse], error) {
			return &fakeSubscribeStream{responses: []*syncv1.SubscribeResponse{
				syncv1.SubscribeResponse_builder{Change: syncv1.SyncChange_builder{
					EntityType: syncv1.EntityType_ENTITY_TYPE_REQUEST,
					EntityId:   env.req.ID.String(),
					Action:     syncv1.Action_ACTION_DELETE,
					Entity:     deletedRequestEntity(env.req.ID, env.coll.ID),
				}.Build()}.Build(),
			}}, nil
		},
	}))
	env.ws.remoteWorkspaceID = "remote-workspace"

	require.Error(t, env.ws.subscribe(ctx), "the canned stream ends with EOF")

	got, err := sqlite.NewRequestRepo(env.db).GetByID(ctx, env.req.ID)
	require.NoError(t, err)
	assert.Nil(t, got, "a delete event from the stream removes the request locally")
}

func holdExample(t *testing.T, env *exampleSyncEnv, ex *entities.ResponseExample, deferred bool) {
	t.Helper()
	ctx := context.Background()
	require.NoError(t, env.engine.syncQueue.Enqueue(ctx, sqlite.SyncEntry{
		WorkspaceID: env.ws.localWorkspaceID, EntityType: "response_example", EntityID: ex.ID.String(),
		Action: "create", OperationID: uuid.NewString(), Status: "pending", CreatedAt: time.Now(),
	}))
	pending, err := env.engine.syncQueue.ListPending(ctx, env.ws.localWorkspaceID, 100)
	require.NoError(t, err)
	id := pending[len(pending)-1].ID
	if deferred {
		require.NoError(t, env.engine.syncQueue.MarkDeferred(ctx, id, time.Now().Add(time.Hour), true))
		return
	}
	require.NoError(t, env.engine.syncQueue.MarkParked(ctx, id))
}

// seedHeldExamples leaves three held examples: one under a live chain, one whose grandparent
// collection is deleted, one whose workspace_id disagrees with its live chain.
func seedHeldExamples(t *testing.T, env *exampleSyncEnv) (live, deadChain, foreign *entities.ResponseExample, grandparent *entities.Collection) {
	t.Helper()
	ctx := context.Background()
	cols := sqlite.NewCollectionRepo(env.db)
	grandparent = newTestCollection("Grandparent", nil)
	parent := newTestCollection("Parent", &grandparent.ID)
	require.NoError(t, cols.Create(ctx, grandparent))
	require.NoError(t, cols.Create(ctx, parent))
	nested := newLocalRequest("Nested", parent.ID)
	require.NoError(t, sqlite.NewRequestRepo(env.db).Create(ctx, nested))

	live = env.createExample(t, "Live")
	env.req = nested
	deadChain = env.createExample(t, "Dead chain")
	foreign = &entities.ResponseExample{
		ID: uuid.New(), RequestID: live.RequestID, WorkspaceID: env.addWorkspace(t), Name: "Foreign",
		Headers: []entities.HeaderItem{}, Protocol: entities.ProtocolHTTP, Version: 1,
		CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}
	require.NoError(t, sqlite.NewResponseExampleRepo(env.db).Create(ctx, foreign))
	_, err := env.db.Exec(`DELETE FROM sync_queue`)
	require.NoError(t, err)

	holdExample(t, env, live, true)
	holdExample(t, env, deadChain, false)
	holdExample(t, env, foreign, true)
	return live, deadChain, foreign, grandparent
}

func TestPushAll_DropsHeldExamplesThatAreNotLive(t *testing.T) {
	env := newExampleSyncEnv(t)
	ctx := context.Background()
	live, deadChain, foreign, grandparent := seedHeldExamples(t, env)
	grandparent.IsDelete = true
	grandparent.Version++
	require.NoError(t, sqlite.NewCollectionRepo(env.db).Update(ctx, grandparent))

	require.NoError(t, env.ws.pushAll(ctx))

	assert.Len(t, queueRowsOf(t, env.db, live.ID.String()), 1, "a live example keeps waiting")
	assert.Empty(t, queueRowsOf(t, env.db, deadChain.ID.String()), "its grandparent collection is gone")
	assert.Empty(t, queueRowsOf(t, env.db, foreign.ID.String()), "it belongs to no tree this workspace shows")
}

func TestPullAll_InboundCollectionDeleteDropsHeldExamplesInsideTheTransaction(t *testing.T) {
	env := newExampleSyncEnv(t)
	ctx := context.Background()
	live, deadChain, _, grandparent := seedHeldExamples(t, env)
	pulled := false
	env.engine.SetGRPCClient(examplesCapableClient(&fakeSyncClient{
		pull: func(*syncv1.PullRequest) (*syncv1.PullResponse, error) {
			if pulled {
				return syncv1.PullResponse_builder{}.Build(), nil
			}
			pulled = true
			deleted := CollectionToProto(grandparent, "")
			deleted.SetIsDeleted(true)
			return syncv1.PullResponse_builder{
				Changes: []*syncv1.SyncChange{syncv1.SyncChange_builder{
					EntityType: syncv1.EntityType_ENTITY_TYPE_COLLECTION, EntityId: grandparent.ID.String(), Entity: deleted,
				}.Build()},
				NextSyncSeq: 3,
			}.Build(), nil
		},
	}))

	done := make(chan error, 1)
	go func() { done <- env.ws.pullAll(ctx) }()
	select {
	case err := <-done:
		require.NoError(t, err)
	case <-time.After(5 * time.Second):
		t.Fatal("the held-example sweep waited for the connection the pull transaction holds")
	}

	assert.Len(t, queueRowsOf(t, env.db, live.ID.String()), 1)
	assert.Empty(t, queueRowsOf(t, env.db, deadChain.ID.String()))
}

func TestPushAll_ExampleHeadersLeaveMasked(t *testing.T) {
	env := newExampleSyncEnv(t)
	ctx := context.Background()
	ex := env.createExample(t, "Canary",
		entities.HeaderItem{Key: "Authorization", Value: "Bearer abc", Enabled: true},
		entities.HeaderItem{Key: "{{hdr}}", Value: "literal", Enabled: true},
		entities.HeaderItem{Key: "X-Token", Value: "Bearer {{token}}", Enabled: true},
	)

	require.NoError(t, env.ws.pushAll(ctx))

	var sent *syncv1.SyncEntity
	for _, b := range env.client.pushed() {
		for _, e := range b {
			if e.GetEntityId() == ex.ID.String() {
				sent = e
			}
		}
	}
	require.NotNil(t, sent, "the example never reached Push")
	values := map[string]string{}
	for _, h := range sent.GetResponseExample().GetHeaders() {
		values[h.GetKey()] = h.GetValue()
	}
	assert.Equal(t, map[string]string{
		"Authorization": "Bearer <redacted>",
		"{{hdr}}":       "<redacted>",
		"X-Token":       "Bearer {{token}}",
	}, values)
}
