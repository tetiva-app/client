package wails

import (
	"context"
	"database/sql"
	"fmt"
	"slices"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"

	syncv1 "github.com/tetiva-app/proto/go/gophercourier/sync/v1"

	"github.com/tetiva-app/client/internal/adapters/wails/dto"
	"github.com/tetiva-app/client/internal/domain/entities"
	"github.com/tetiva-app/client/internal/infrastructure/repository/sqlite"
	syncsvc "github.com/tetiva-app/client/internal/infrastructure/sync"
)

func remoteIDOf(t *testing.T, db *sql.DB, workspaceID string) sql.NullString {
	t.Helper()
	var remote sql.NullString
	require.NoError(t, db.QueryRow(
		`SELECT remote_workspace_id FROM workspaces WHERE id = ?`, workspaceID).Scan(&remote))
	return remote
}

func setRemoteID(t *testing.T, db *sql.DB, workspaceID, remote string) {
	t.Helper()
	_, err := db.Exec(
		`UPDATE workspaces SET remote_workspace_id = ? WHERE id = ?`, remote, workspaceID)
	require.NoError(t, err)
}

func TestSyncRemoteWorkspaces_DropsMappingsFromAnotherAccount(t *testing.T) {
	f := newVerificationFixture(t)
	ctx := context.Background()

	setRemoteID(t, f.db, seededWorkspaceID, "ws_from_previous_account")

	f.seedSession(t)
	f.svc.grpcClient = f.client

	require.NoError(t, f.svc.syncRemoteWorkspaces(ctx))

	remote := remoteIDOf(t, f.db, seededWorkspaceID)
	assert.False(t, remote.Valid && remote.String != "",
		"a mapping the current org does not own must be dropped, got %q", remote.String)
}

func TestSyncRemoteWorkspaces_KeepsMappingOwnedByCurrentOrg(t *testing.T) {
	f := newVerificationFixture(t)
	ctx := context.Background()

	setRemoteID(t, f.db, seededWorkspaceID, f.wsStub.remoteID)

	f.seedSession(t)
	f.svc.grpcClient = f.client

	require.NoError(t, f.svc.syncRemoteWorkspaces(ctx))

	remote := remoteIDOf(t, f.db, seededWorkspaceID)
	assert.Equal(t, f.wsStub.remoteID, remote.String)
}

// Dropping the mapping is not enough — the workspace must reach the engine.
func TestEnableSync_StartsActiveWorkspaceAfterStaleMappingDropped(t *testing.T) {
	f := newVerificationFixture(t)
	ctx := context.Background()

	setRemoteID(t, f.db, seededWorkspaceID, "ws_from_previous_account")

	f.seedSession(t)
	f.svc.grpcClient = f.client

	require.NoError(t, f.svc.enableSync(ctx, f.client))

	assert.NotEqual(t, syncsvc.StateDisconnected, f.engine.GetWorkspaceState(seededWorkspaceID))
}

func lastSyncSeqOf(t *testing.T, db *sql.DB, workspaceID string) int64 {
	t.Helper()
	var seq int64
	require.NoError(t, db.QueryRow(
		`SELECT last_sync_seq FROM workspaces WHERE id = ?`, workspaceID).Scan(&seq))
	return seq
}

func TestLinkWorkspace_ResetsCursorWhenRemoteChanges(t *testing.T) {
	f := newVerificationFixture(t)
	setRemoteID(t, f.db, seededWorkspaceID, "remote-old")
	_, err := f.db.Exec(`UPDATE workspaces SET last_sync_seq = 42 WHERE id = ?`, seededWorkspaceID)
	require.NoError(t, err)

	res := f.svc.LinkWorkspace(dto.LinkWorkspaceRequest{
		LocalWorkspaceID:  seededWorkspaceID,
		RemoteWorkspaceID: "remote-new",
	})

	require.Nil(t, res.Error)
	assert.Equal(t, "remote-new", remoteIDOf(t, f.db, seededWorkspaceID).String)
	assert.Zero(t, lastSyncSeqOf(t, f.db, seededWorkspaceID))
}

func TestLinkWorkspace_KeepsCursorForSameRemote(t *testing.T) {
	f := newVerificationFixture(t)
	setRemoteID(t, f.db, seededWorkspaceID, "remote-1")
	_, err := f.db.Exec(`UPDATE workspaces SET last_sync_seq = 42 WHERE id = ?`, seededWorkspaceID)
	require.NoError(t, err)

	res := f.svc.LinkWorkspace(dto.LinkWorkspaceRequest{
		LocalWorkspaceID:  seededWorkspaceID,
		RemoteWorkspaceID: "remote-1",
	})

	require.Nil(t, res.Error)
	assert.EqualValues(t, 42, lastSyncSeqOf(t, f.db, seededWorkspaceID))
}

func enqueueOutbox(t *testing.T, f *verificationFixture, workspaceID string, n int) {
	t.Helper()
	for range n {
		require.NoError(t, f.svc.queueRepo.Enqueue(context.Background(), sqlite.SyncEntry{
			WorkspaceID: workspaceID,
			EntityType:  "request",
			EntityID:    uuid.NewString(),
			Action:      "update",
			OperationID: uuid.NewString(),
			Status:      "pending",
			CreatedAt:   time.Now().Truncate(time.Second),
		}))
	}
}

func outboxRows(t *testing.T, db *sql.DB, workspaceID string) int {
	t.Helper()
	var n int
	require.NoError(t, db.QueryRow(
		`SELECT COUNT(*) FROM sync_queue WHERE workspace_id = ?`, workspaceID).Scan(&n))
	return n
}

func TestUnlinkWorkspace_DropsOutbox(t *testing.T) {
	f := newVerificationFixture(t)
	setRemoteID(t, f.db, seededWorkspaceID, "remote-1")
	enqueueOutbox(t, f, seededWorkspaceID, 2)

	res := f.svc.UnlinkWorkspace(dto.UnlinkWorkspaceRequest{LocalWorkspaceID: seededWorkspaceID})

	require.Nil(t, res.Error)
	assert.Zero(t, outboxRows(t, f.db, seededWorkspaceID),
		"an unlinked workspace must not push its old outbox to the next account")
	assert.False(t, remoteIDOf(t, f.db, seededWorkspaceID).Valid)
}

func TestUnlinkWorkspace_KeepsOutboxWhenMappingSurvives(t *testing.T) {
	f := newVerificationFixture(t)
	setRemoteID(t, f.db, seededWorkspaceID, "remote-1")
	enqueueOutbox(t, f, seededWorkspaceID, 2)
	f.engine.StartWorkspace(seededWorkspaceID, "remote-1", 0)

	_, err := f.db.Exec(`CREATE TRIGGER block_unlink BEFORE UPDATE OF remote_workspace_id ON workspaces
		BEGIN SELECT RAISE(ABORT, 'mapping update refused'); END`)
	require.NoError(t, err)

	res := f.svc.UnlinkWorkspace(dto.UnlinkWorkspaceRequest{LocalWorkspaceID: seededWorkspaceID})

	require.NotNil(t, res.Error)
	assert.Equal(t, "remote-1", remoteIDOf(t, f.db, seededWorkspaceID).String)
	assert.Equal(t, 2, outboxRows(t, f.db, seededWorkspaceID),
		"a workspace that stayed linked must keep the edits it still owes the cloud")

	assert.NotEqual(t, syncsvc.StateDisconnected, f.engine.GetWorkspaceState(seededWorkspaceID),
		"a workspace that stayed linked must keep its syncer")

	coll := &entities.Collection{
		ID:           uuid.New(),
		WorkspaceID:  uuid.MustParse(seededWorkspaceID),
		Name:         "After the failed unlink",
		AuthType:     entities.AuthTypeNone,
		AuthData:     "{}",
		GRPCMetadata: []entities.HeaderItem{},
		Version:      1,
		CreatedBy:    "test_user",
		CreatedAt:    time.Now().Truncate(time.Second),
		UpdatedBy:    "test_user",
		UpdatedAt:    time.Now().Truncate(time.Second),
	}
	synced := syncsvc.NewSyncedCollectionRepo(sqlite.NewCollectionRepo(f.db), f.svc.queueRepo, f.db, f.engine)
	require.NoError(t, synced.Create(context.Background(), coll))

	assert.Equal(t, 3, outboxRows(t, f.db, seededWorkspaceID),
		"edits made after the failed unlink must still reach the outbox")
}

func TestSyncRemoteWorkspaces_DropsOutboxOfForeignWorkspace(t *testing.T) {
	f := newVerificationFixture(t)
	ctx := context.Background()

	setRemoteID(t, f.db, seededWorkspaceID, "ws_from_previous_account")
	enqueueOutbox(t, f, seededWorkspaceID, 2)

	f.seedSession(t)
	f.svc.grpcClient = f.client

	require.NoError(t, f.svc.syncRemoteWorkspaces(ctx))

	assert.Zero(t, outboxRows(t, f.db, seededWorkspaceID),
		"the outbox of a workspace owned by another account must go with the mapping")
}

func TestEnableSync_WorkspaceOfAnotherAccountUploadsNothingWhenRelinked(t *testing.T) {
	f := newVerificationFixture(t)
	ctx := context.Background()
	root, _, _ := seedLocalTree(t, f.db, seededWorkspaceID)
	require.Nil(t, f.svc.LinkWorkspace(dto.LinkWorkspaceRequest{LocalWorkspaceID: seededWorkspaceID, RemoteWorkspaceID: "ws_from_previous_account"}).Error)
	_, err := f.db.Exec(`UPDATE collections SET is_synced = 1 WHERE id = ?`, root)
	require.NoError(t, err)

	f.seedSession(t)
	f.svc.grpcClient = f.client

	require.NoError(t, f.svc.enableSync(ctx, f.client))

	require.Equal(t, "remote-linked", remoteIDOf(t, f.db, seededWorkspaceID).String)
	assert.Zero(t, outboxRows(t, f.db, seededWorkspaceID), "what the previous account's workspace holds must not reach the next account")
}

func TestLinkWorkspace_MirrorOfARemoteQueuesNothingForAnotherRemote(t *testing.T) {
	f := newVerificationFixture(t)
	f.seedSession(t)
	f.svc.grpcClient = f.client
	require.NoError(t, f.svc.syncRemoteWorkspaces(context.Background()))
	var mirror string
	require.NoError(t, f.db.QueryRow(`SELECT id FROM workspaces WHERE remote_workspace_id = 'remote-1'`).Scan(&mirror))
	seedLocalTree(t, f.db, mirror)

	res := f.svc.LinkWorkspace(dto.LinkWorkspaceRequest{LocalWorkspaceID: mirror, RemoteWorkspaceID: "remote-other"})

	require.Nil(t, res.Error)
	assert.Zero(t, outboxRows(t, f.db, mirror), "what a mirror pulled belongs to its remote")
}

type pullState struct {
	backfill           int
	backfillToken      string
	backfillIncomplete int
	backfillFailed     int
	pageToken          string
	seen               int
}

func setPullState(t *testing.T, db *sql.DB, workspaceID string, s pullState) {
	t.Helper()
	_, err := db.Exec(`UPDATE workspaces
		SET examples_backfill_pending = ?, examples_backfill_token = ?, examples_backfill_incomplete = ?,
		    examples_backfill_failed_passes = ?, sync_page_token = ? WHERE id = ?`,
		s.backfill, s.backfillToken, s.backfillIncomplete, s.backfillFailed, s.pageToken, workspaceID)
	require.NoError(t, err)
	for i := range s.seen {
		_, err := db.Exec(`INSERT INTO sync_snapshot_seen (workspace_id, walk, generation, entity_type, entity_id)
			VALUES (?, 'snapshot', 1, 'collection', ?)`, workspaceID, fmt.Sprintf("seen-%d", i))
		require.NoError(t, err)
	}
}

func pullStateOf(t *testing.T, db *sql.DB, workspaceID string) pullState {
	t.Helper()
	var s pullState
	require.NoError(t, db.QueryRow(`SELECT examples_backfill_pending, examples_backfill_token, examples_backfill_incomplete,
		examples_backfill_failed_passes, sync_page_token FROM workspaces WHERE id = ?`, workspaceID).
		Scan(&s.backfill, &s.backfillToken, &s.backfillIncomplete, &s.backfillFailed, &s.pageToken))
	require.NoError(t, db.QueryRow(`SELECT COUNT(*) FROM sync_snapshot_seen WHERE workspace_id = ?`, workspaceID).Scan(&s.seen))
	return s
}

var midWalk = pullState{backfill: 1, backfillToken: "fill", backfillIncomplete: 1, backfillFailed: 2, pageToken: "walk", seen: 2}

func TestLinkWorkspace_ResetsPullStateWhenRemoteChanges(t *testing.T) {
	f := newVerificationFixture(t)
	setRemoteID(t, f.db, seededWorkspaceID, "remote-old")
	setPullState(t, f.db, seededWorkspaceID, midWalk)

	res := f.svc.LinkWorkspace(dto.LinkWorkspaceRequest{
		LocalWorkspaceID:  seededWorkspaceID,
		RemoteWorkspaceID: "remote-new",
	})

	require.Nil(t, res.Error)
	assert.Equal(t, pullState{}, pullStateOf(t, f.db, seededWorkspaceID))
}

func TestLinkWorkspace_KeepsPullStateForSameRemote(t *testing.T) {
	f := newVerificationFixture(t)
	setRemoteID(t, f.db, seededWorkspaceID, "remote-1")
	setPullState(t, f.db, seededWorkspaceID, midWalk)

	res := f.svc.LinkWorkspace(dto.LinkWorkspaceRequest{
		LocalWorkspaceID:  seededWorkspaceID,
		RemoteWorkspaceID: "remote-1",
	})

	require.Nil(t, res.Error)
	assert.Equal(t, midWalk, pullStateOf(t, f.db, seededWorkspaceID))
}

func TestUnlinkWorkspace_ResetsPullState(t *testing.T) {
	f := newVerificationFixture(t)
	setRemoteID(t, f.db, seededWorkspaceID, "remote-1")
	setPullState(t, f.db, seededWorkspaceID, midWalk)

	res := f.svc.UnlinkWorkspace(dto.UnlinkWorkspaceRequest{LocalWorkspaceID: seededWorkspaceID})

	require.Nil(t, res.Error)
	assert.Equal(t, pullState{}, pullStateOf(t, f.db, seededWorkspaceID))
}

func TestSyncRemoteWorkspaces_ResetsPullStateOfForeignWorkspace(t *testing.T) {
	f := newVerificationFixture(t)
	setRemoteID(t, f.db, seededWorkspaceID, "ws_from_previous_account")
	setPullState(t, f.db, seededWorkspaceID, midWalk)

	f.seedSession(t)
	f.svc.grpcClient = f.client

	require.NoError(t, f.svc.syncRemoteWorkspaces(context.Background()))

	assert.Equal(t, pullState{}, pullStateOf(t, f.db, seededWorkspaceID))
}

type latePullClient struct {
	syncv1.SyncServiceClient
	calls    atomic.Int32
	entered  chan struct{}
	answered chan struct{}
}

func (c *latePullClient) Pull(ctx context.Context, _ *syncv1.PullRequest, _ ...grpc.CallOption) (*syncv1.PullResponse, error) {
	if c.calls.Add(1) > 1 {
		return syncv1.PullResponse_builder{}.Build(), nil
	}
	c.entered <- struct{}{}
	<-ctx.Done()
	defer close(c.answered)
	return syncv1.PullResponse_builder{
		Changes:     []*syncv1.SyncChange{syncv1.SyncChange_builder{SyncSeq: 99}.Build()},
		NextSyncSeq: 99,
	}.Build(), nil
}

func (c *latePullClient) Push(context.Context, *syncv1.PushRequest, ...grpc.CallOption) (*syncv1.PushResponse, error) {
	return syncv1.PushResponse_builder{}.Build(), nil
}

func (c *latePullClient) Subscribe(ctx context.Context, _ *syncv1.SubscribeRequest, _ ...grpc.CallOption) (grpc.ServerStreamingClient[syncv1.SubscribeResponse], error) {
	return &idleStream{ctx: ctx}, nil
}

type idleStream struct {
	grpc.ClientStream
	ctx context.Context
}

func (s *idleStream) Recv() (*syncv1.SubscribeResponse, error) {
	<-s.ctx.Done()
	return nil, s.ctx.Err()
}

func TestLinkWorkspace_WaitsOutThePreviousSyncersPull(t *testing.T) {
	f := newVerificationFixture(t)
	f.seedSession(t)
	setRemoteID(t, f.db, seededWorkspaceID, "remote-old")
	_, err := f.db.Exec(`UPDATE workspaces SET last_sync_seq = 42 WHERE id = ?`, seededWorkspaceID)
	require.NoError(t, err)

	stub := &latePullClient{entered: make(chan struct{}, 1), answered: make(chan struct{})}
	f.engine.SetGRPCClient(syncsvc.NewGRPCClientWithSyncStub(f.authStub, f.wsStub, stub))
	f.engine.StartWorkspace(seededWorkspaceID, "remote-old", 42)
	select {
	case <-stub.entered:
	case <-time.After(5 * time.Second):
		t.Fatal("the previous syncer never pulled")
	}

	res := f.svc.LinkWorkspace(dto.LinkWorkspaceRequest{
		LocalWorkspaceID:  seededWorkspaceID,
		RemoteWorkspaceID: "remote-new",
	})

	require.Nil(t, res.Error)
	select {
	case <-stub.answered:
	default:
		t.Fatal("the link reset the workspace while the previous syncer still had a pull in flight")
	}
	assert.Zero(t, lastSyncSeqOf(t, f.db, seededWorkspaceID), "the previous remote's cursor must not survive the relink")
	assert.Equal(t, "remote-new", remoteIDOf(t, f.db, seededWorkspaceID).String)
}

func queuedCreates(t *testing.T, db *sql.DB, workspaceID string) []string {
	t.Helper()
	rows, err := db.Query(`SELECT entity_type || ':' || entity_id FROM sync_queue
		WHERE workspace_id = ? AND action = 'create' AND status = 'pending' ORDER BY id`, workspaceID)
	require.NoError(t, err)
	defer func() { _ = rows.Close() }()
	var out []string
	for rows.Next() {
		var key string
		require.NoError(t, rows.Scan(&key))
		out = append(out, key)
	}
	require.NoError(t, rows.Err())
	return out
}

func seedLocalTree(t *testing.T, db *sql.DB, workspaceID string) (root, folder, req string) {
	t.Helper()
	root, folder, req = uuid.NewString(), uuid.NewString(), uuid.NewString()
	_, err := db.Exec(`INSERT INTO collections (id, workspace_id, name) VALUES (?, ?, 'Dogs API')`, root, workspaceID)
	require.NoError(t, err)
	_, err = db.Exec(`INSERT INTO collections (id, workspace_id, name, parent_id) VALUES (?, ?, 'Breeds', ?)`, folder, workspaceID, root)
	require.NoError(t, err)
	_, err = db.Exec(`INSERT INTO requests (id, collection_id, name) VALUES (?, ?, 'List breeds')`, req, folder)
	require.NoError(t, err)
	return root, folder, req
}

func TestEnableSync_AutoLinkQueuesTheEntitiesTheWorkspaceAlreadyHolds(t *testing.T) {
	f := newVerificationFixture(t)
	ctx := context.Background()
	root, folder, req := seedLocalTree(t, f.db, seededWorkspaceID)
	env, envVar, seededVar := uuid.NewString(), uuid.NewString(), uuid.NewString()
	_, err := f.db.Exec(`INSERT INTO environments (id, workspace_id, name) VALUES (?, ?, 'Staging')`, env, seededWorkspaceID)
	require.NoError(t, err)
	_, err = f.db.Exec(`INSERT INTO variables (id, environment_id, key) VALUES (?, ?, 'host'), (?, ?, 'host')`,
		envVar, env, seededVar, seededEnvironmentID)
	require.NoError(t, err)

	f.seedSession(t)
	f.svc.grpcClient = f.client

	require.NoError(t, f.svc.enableSync(ctx, f.client))

	require.Equal(t, "remote-linked", remoteIDOf(t, f.db, seededWorkspaceID).String)
	queued := queuedCreates(t, f.db, seededWorkspaceID)
	reissued := defaultEnvironmentID(t, f.db, seededWorkspaceID)
	assert.Subset(t, queued, []string{
		"collection:" + root, "collection:" + folder, "request:" + req, "environment:" + env, "variable:" + envVar,
		"environment:" + reissued, "variable:" + seededVar,
	})
	assert.Less(t, slices.Index(queued, "collection:"+root), slices.Index(queued, "collection:"+folder), "a folder goes after its parent")
	assert.NotContains(t, queued, "environment:"+seededEnvironmentID)
}

func defaultEnvironmentID(t *testing.T, db *sql.DB, workspaceID string) string {
	t.Helper()
	var id string
	require.NoError(t, db.QueryRow(`SELECT id FROM environments WHERE workspace_id = ? AND name = 'Default'`, workspaceID).Scan(&id))
	return id
}

type emittedEvents struct {
	mu    sync.Mutex
	names []string
}

func (e *emittedEvents) record(name string, _ any) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.names = append(e.names, name)
}

func (e *emittedEvents) seen() []string {
	e.mu.Lock()
	defer e.mu.Unlock()
	return slices.Clone(e.names)
}

func TestLinkWorkspace_FirstLinkReissuesTheSeededEnvironmentAndUploadsIt(t *testing.T) {
	f := newVerificationFixture(t)
	events := &emittedEvents{}
	f.svc.SetEventEmitter(events.record)
	seededVar := uuid.NewString()
	_, err := f.db.Exec(`INSERT INTO variables (id, environment_id, key) VALUES (?, ?, 'host')`, seededVar, seededEnvironmentID)
	require.NoError(t, err)

	res := f.svc.LinkWorkspace(dto.LinkWorkspaceRequest{LocalWorkspaceID: seededWorkspaceID, RemoteWorkspaceID: "remote-new"})

	require.Nil(t, res.Error)
	reissued := defaultEnvironmentID(t, f.db, seededWorkspaceID)
	assert.NotEqual(t, seededEnvironmentID, reissued)
	var varEnv string
	require.NoError(t, f.db.QueryRow(`SELECT environment_id FROM variables WHERE id = ?`, seededVar).Scan(&varEnv))
	assert.Equal(t, reissued, varEnv, "the variables follow their environment to the new id")
	queued := queuedCreates(t, f.db, seededWorkspaceID)
	envAt := slices.Index(queued, "environment:"+reissued)
	require.GreaterOrEqual(t, envAt, 0, "the reissued environment is uploaded")
	assert.Less(t, envAt, slices.Index(queued, "variable:"+seededVar), "the environment goes before its variables")
	assert.Contains(t, events.seen(), "env:changed", "the open windows still hold the old id")
}

func TestLinkWorkspace_SeededEnvironmentKeepsItsIdOutsideTheFirstLink(t *testing.T) {
	for name, setup := range map[string]string{
		"workspace linked before":     `UPDATE workspaces SET was_linked = 1 WHERE id = '` + seededWorkspaceID + `'`,
		"copy pulled from the server": `UPDATE environments SET created_by = 'sync' WHERE id = '` + seededEnvironmentID + `'`,
	} {
		t.Run(name, func(t *testing.T) {
			f := newVerificationFixture(t)
			events := &emittedEvents{}
			f.svc.SetEventEmitter(events.record)
			_, err := f.db.Exec(setup)
			require.NoError(t, err)

			res := f.svc.LinkWorkspace(dto.LinkWorkspaceRequest{LocalWorkspaceID: seededWorkspaceID, RemoteWorkspaceID: "remote-new"})

			require.Nil(t, res.Error)
			assert.Equal(t, seededEnvironmentID, defaultEnvironmentID(t, f.db, seededWorkspaceID))
			assert.NotContains(t, queuedCreates(t, f.db, seededWorkspaceID), "environment:"+seededEnvironmentID)
			assert.NotContains(t, events.seen(), "env:changed")
		})
	}
}

func TestLinkWorkspace_SameRemoteKeepsTheSeededEnvironment(t *testing.T) {
	f := newVerificationFixture(t)
	setRemoteID(t, f.db, seededWorkspaceID, "remote-1")

	res := f.svc.LinkWorkspace(dto.LinkWorkspaceRequest{LocalWorkspaceID: seededWorkspaceID, RemoteWorkspaceID: "remote-1"})

	require.Nil(t, res.Error)
	assert.Equal(t, seededEnvironmentID, defaultEnvironmentID(t, f.db, seededWorkspaceID))
}

func TestLinkWorkspace_QueuesExistingEntitiesForANewRemote(t *testing.T) {
	f := newVerificationFixture(t)
	root, _, req := seedLocalTree(t, f.db, seededWorkspaceID)

	res := f.svc.LinkWorkspace(dto.LinkWorkspaceRequest{LocalWorkspaceID: seededWorkspaceID, RemoteWorkspaceID: "remote-new"})

	require.Nil(t, res.Error)
	assert.Subset(t, queuedCreates(t, f.db, seededWorkspaceID), []string{"collection:" + root, "request:" + req})
}

func TestLinkWorkspace_SameRemoteQueuesNothing(t *testing.T) {
	f := newVerificationFixture(t)
	setRemoteID(t, f.db, seededWorkspaceID, "remote-1")
	seedLocalTree(t, f.db, seededWorkspaceID)

	res := f.svc.LinkWorkspace(dto.LinkWorkspaceRequest{LocalWorkspaceID: seededWorkspaceID, RemoteWorkspaceID: "remote-1"})

	require.Nil(t, res.Error)
	assert.Zero(t, outboxRows(t, f.db, seededWorkspaceID), "a link the workspace already had uploads nothing again")
}

type linkGapQueue struct {
	sqlite.SyncQueueRepository
	write func()
}

func (q *linkGapQueue) EnqueueWorkspace(ctx context.Context, workspaceID string) (int, error) {
	n, err := q.SyncQueueRepository.EnqueueWorkspace(ctx, workspaceID)
	sqlite.AfterCommit(ctx, q.write)
	return n, err
}

func TestLinkWorkspace_QueuesAWriteThatSkippedTheOutboxDuringTheLink(t *testing.T) {
	f := newVerificationFixture(t)
	late := uuid.NewString()
	f.svc.queueRepo = &linkGapQueue{SyncQueueRepository: f.svc.queueRepo, write: func() {
		_, err := f.db.Exec(`INSERT INTO collections (id, workspace_id, name) VALUES (?, ?, 'Added mid-link')`, late, seededWorkspaceID)
		require.NoError(t, err)
	}}

	res := f.svc.LinkWorkspace(dto.LinkWorkspaceRequest{LocalWorkspaceID: seededWorkspaceID, RemoteWorkspaceID: "remote-new"})

	require.Nil(t, res.Error)
	assert.Contains(t, queuedCreates(t, f.db, seededWorkspaceID), "collection:"+late)
}
