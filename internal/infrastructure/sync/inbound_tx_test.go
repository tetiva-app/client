package sync

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"

	syncv1 "github.com/tetiva-app/proto/go/gophercourier/sync/v1"
)

func rowCount(t *testing.T, ws *workspaceSyncer, table string, id uuid.UUID) int {
	t.Helper()
	var n int
	require.NoError(t, ws.engine.db.QueryRow(`SELECT COUNT(*) FROM `+table+` WHERE id = ?`, id.String()).Scan(&n))
	return n
}

func TestPullAll_ChildFolderOnAnEarlierPageIsKept(t *testing.T) {
	ws, db, script := newPullSyncer(t)
	parent := newTestCollection("Parent", nil)
	child := newTestCollection("Child", &parent.ID)
	req := newLocalRequest("In child", child.ID)

	script.steps = []pullStep{
		answer(syncv1.PullResponse_builder{
			Changes: []*syncv1.SyncChange{entityChange(CollectionToProto(child, "")), entityChange(RequestToProto(req, ""))},
			HasMore: true, NextSnapshotPageToken: "t1",
		}.Build()),
		answer(syncv1.PullResponse_builder{
			Changes: []*syncv1.SyncChange{entityChange(CollectionToProto(parent, ""))}, NextSyncSeq: 10,
		}.Build()),
		answer(syncv1.PullResponse_builder{NextSyncSeq: 10}.Build()),
	}

	require.NoError(t, ws.pullAll(context.Background()))

	assert.Equal(t, 1, rowCount(t, ws, "collections", parent.ID))
	assert.Equal(t, 1, rowCount(t, ws, "collections", child.ID), "a folder that came a page before its parent")
	assert.Equal(t, 1, rowCount(t, ws, "requests", req.ID), "a request of that folder")
	seq, _ := pullPosition(t, db)
	assert.EqualValues(t, 10, seq)

	var fk int
	require.NoError(t, db.QueryRow(`PRAGMA foreign_keys`).Scan(&fk))
	assert.Equal(t, 1, fk, "the pooled connection gets its foreign keys back")
}

func TestSubscribe_StoresARequestThatArrivesBeforeItsFolder(t *testing.T) {
	client := &fakeSyncClient{}
	ws, _, _ := newInboundSyncer(t, client)
	folder := newTestCollection("Late folder", nil)
	req := newLocalRequest("Early request", folder.ID)

	client.stream = func() (grpc.ServerStreamingClient[syncv1.SubscribeResponse], error) {
		return &fakeSubscribeStream{responses: []*syncv1.SubscribeResponse{
			syncv1.SubscribeResponse_builder{Change: entityChange(RequestToProto(req, ""))}.Build(),
			syncv1.SubscribeResponse_builder{Change: entityChange(CollectionToProto(folder, ""))}.Build(),
		}}, nil
	}

	require.Error(t, ws.subscribe(context.Background()), "the canned stream ends with EOF")

	assert.Equal(t, 1, rowCount(t, ws, "requests", req.ID))
	assert.Equal(t, 1, rowCount(t, ws, "collections", folder.ID))
}

func TestPushAll_ConflictWinnerUnderAFolderNotYetPulledIsStored(t *testing.T) {
	env := newExampleSyncEnv(t)
	ctx := context.Background()
	folder := newTestCollection("Not pulled yet", nil)
	winner := *env.req
	winner.CollectionID = folder.ID
	winner.Version = 7
	require.NoError(t, env.requests.Update(ctx, env.req))

	env.client.respond = func(_ int, req *syncv1.PushRequest) (*syncv1.PushResponse, error) {
		results := make([]*syncv1.PushResult, 0, len(req.GetEntities()))
		for _, e := range req.GetEntities() {
			results = append(results, syncv1.PushResult_builder{
				EntityId: e.GetEntityId(),
				Status:   syncv1.PushStatus_PUSH_STATUS_CONFLICT_RESOLVED,
				Winner:   RequestToProto(&winner, ""),
			}.Build())
		}
		return syncv1.PushResponse_builder{Results: results}.Build(), nil
	}
	require.NoError(t, env.ws.pushAll(ctx))

	var collectionID string
	var version int
	require.NoError(t, env.db.QueryRow(`SELECT collection_id, version FROM requests WHERE id = ?`, env.req.ID.String()).
		Scan(&collectionID, &version))
	assert.Equal(t, folder.ID.String(), collectionID, "the winner moved the request into a folder the next pull brings")
	assert.Equal(t, 7, version)
}
