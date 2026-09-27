package sync

import (
	"context"
	"database/sql"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	syncv1 "github.com/tetiva-app/proto/go/gophercourier/sync/v1"

	"github.com/tetiva-app/client/internal/domain/usecase/example"
)

func seenRows(t *testing.T, db *sql.DB) int {
	t.Helper()
	var n int
	require.NoError(t, db.QueryRow(`SELECT COUNT(*) FROM sync_snapshot_seen`).Scan(&n))
	return n
}

func unavailable(*syncv1.PullRequest) (*syncv1.PullResponse, error) {
	return nil, status.Error(codes.Unavailable, "connection reset")
}

func refused(*syncv1.PullRequest) (*syncv1.PullResponse, error) {
	return nil, status.Error(codes.InvalidArgument, "snapshot boundary expired")
}

func changesOf(entities ...*syncv1.SyncEntity) []*syncv1.SyncChange {
	changes := make([]*syncv1.SyncChange, 0, len(entities))
	for _, e := range entities {
		changes = append(changes, entityChange(e))
	}
	return changes
}

func TestPullAll_RestartedSnapshotDeletesWhatTheServerDroppedMeanwhile(t *testing.T) {
	env := newExampleSyncEnv(t)
	ctx := context.Background()
	script := &pullScript{t: t}
	env.engine.SetGRPCClient(examplesCapableClient(&fakeSyncClient{pull: script.pull}))
	setPullPosition(t, env.ws, env.db, 0, "")

	dropped := newTestCollection("Deleted between the walks", nil)
	kept := newTestCollection("Still there", nil)
	droppedExample := uuid.New()
	editedHere := uuid.New()
	script.steps = []pullStep{
		answer(syncv1.PullResponse_builder{
			Changes: changesOf(CollectionToProto(dropped, ""), CollectionToProto(kept, ""),
				exampleEntity(droppedExample, env.req.ID, "Dropped", 1), exampleEntity(editedHere, env.req.ID, "Edited here", 1)),
			HasMore: true, NextSnapshotPageToken: "t1",
		}.Build()),
		unavailable,
		refused,
		answer(syncv1.PullResponse_builder{Changes: changesOf(CollectionToProto(kept, "")), NextSyncSeq: 20}.Build()),
		answer(syncv1.PullResponse_builder{NextSyncSeq: 20}.Build()),
	}

	require.Error(t, env.ws.pullAll(ctx))
	require.Positive(t, seenRows(t, env.db))
	_, err := env.usecase.Edit(ctx, example.Edit{Name: "Local edit", StatusCode: 200, StatusText: "OK"},
		example.EditOpt{ExampleID: editedHere, UserID: "test_user", Version: 1})
	require.NoError(t, err)

	require.NoError(t, env.ws.pullAll(ctx))

	assert.True(t, collectionIsDeleted(t, env.db, dropped.ID), "a delete the new walk no longer shows")
	assert.True(t, isDeletedRow(t, env.db, "response_example", droppedExample.String()))
	assert.False(t, collectionIsDeleted(t, env.db, kept.ID))
	assert.False(t, isDeletedRow(t, env.db, "response_example", editedHere.String()), "a queued local edit is pushed, not dropped")
	assert.Equal(t, "Local edit", exampleContent(t, env.db, editedHere).name)
	assert.Empty(t, queueRowsOf(t, env.db, dropped.ID.String()), "an inbound delete goes to no outbox")
	assert.Empty(t, queueRowsOf(t, env.db, droppedExample.String()))
	assert.Zero(t, seenRows(t, env.db))
	seq, token := pullPosition(t, env.db)
	assert.EqualValues(t, 20, seq)
	assert.Empty(t, token)
}

func TestPullAll_CollectionRevivedAfterTheRestartedWalkDeletedItComesBack(t *testing.T) {
	env := newExampleSyncEnv(t)
	ctx := context.Background()
	script := &pullScript{t: t}
	env.engine.SetGRPCClient(examplesCapableClient(&fakeSyncClient{pull: script.pull}))
	setPullPosition(t, env.ws, env.db, 0, "")

	c := newTestCollection("Flapping", nil)
	revived := *c
	revived.Name, revived.Version = "Back again", 3
	script.steps = []pullStep{
		answer(syncv1.PullResponse_builder{Changes: changesOf(CollectionToProto(c, "")), HasMore: true, NextSnapshotPageToken: "t1"}.Build()),
		refused,
		answer(syncv1.PullResponse_builder{NextSyncSeq: 20}.Build()),
		answer(syncv1.PullResponse_builder{Changes: changesOf(CollectionToProto(&revived, "")), NextSyncSeq: 21}.Build()),
		answer(syncv1.PullResponse_builder{NextSyncSeq: 21}.Build()),
	}

	require.NoError(t, env.ws.pullAll(ctx))

	assert.False(t, collectionIsDeleted(t, env.db, c.ID), "a peer's later edit brought it back on the server")
	var name string
	require.NoError(t, env.db.QueryRow(`SELECT name FROM collections WHERE id = ?`, c.ID.String()).Scan(&name))
	assert.Equal(t, "Back again", name)
}

func TestPullAll_UnbrokenSnapshotDeletesNothing(t *testing.T) {
	ws, db, script := newPullSyncer(t)
	ctx := context.Background()
	local := hostCollection(t, db)
	_, err := db.Exec(`UPDATE collections SET is_synced = 1 WHERE id = ?`, local.ID.String())
	require.NoError(t, err)
	walked := newTestCollection("Walked", nil)
	script.steps = []pullStep{
		answer(syncv1.PullResponse_builder{Changes: changesOf(CollectionToProto(walked, "")), HasMore: true, NextSnapshotPageToken: "t1"}.Build()),
		answer(syncv1.PullResponse_builder{NextSyncSeq: 5}.Build()),
		answer(syncv1.PullResponse_builder{NextSyncSeq: 5}.Build()),
	}

	require.NoError(t, ws.pullAll(ctx))

	assert.False(t, collectionIsDeleted(t, db, local.ID), "only a restarted walk compares what it saw")
	assert.Zero(t, seenRows(t, db))
}

func TestBackfill_RestartedPassDeletesExamplesTheServerDroppedMeanwhile(t *testing.T) {
	env, script := newBackfillEnv(t)
	ctx := context.Background()
	dropped, editedHere, kept := uuid.New(), uuid.New(), uuid.New()
	script.steps = []pullStep{
		answer(examplePage("b1", exampleEntity(dropped, env.req.ID, "Dropped", 1),
			exampleEntity(editedHere, env.req.ID, "Edited here", 1), exampleEntity(kept, env.req.ID, "Kept", 1))),
		unavailable,
		refused,
		answer(examplePage("", exampleEntity(kept, env.req.ID, "Kept", 1))),
	}

	require.NoError(t, env.ws.backfillExamples(ctx))
	_, err := env.usecase.Edit(ctx, example.Edit{Name: "Local edit", StatusCode: 200, StatusText: "OK"},
		example.EditOpt{ExampleID: editedHere, UserID: "test_user", Version: 1})
	require.NoError(t, err)

	require.NoError(t, env.ws.backfillExamples(ctx))

	assert.True(t, isDeletedRow(t, env.db, "response_example", dropped.String()))
	assert.False(t, isDeletedRow(t, env.db, "response_example", editedHere.String()))
	assert.False(t, isDeletedRow(t, env.db, "response_example", kept.String()))
	assert.Empty(t, queueRowsOf(t, env.db, dropped.String()))
	assert.Zero(t, seenRows(t, env.db))
	pending, _ := backfillPosition(t, env.db)
	assert.False(t, pending)
}

func TestClearOutbox_ForgetsTheSeenEntities(t *testing.T) {
	env := newExampleSyncEnv(t)
	_, err := env.db.Exec(`INSERT INTO sync_snapshot_seen (workspace_id, walk, generation, entity_type, entity_id)
		VALUES (?, 'snapshot', 1, 'collection', 'c1'), (?, 'backfill', 0, 'response_example', 'e1')`,
		testWorkspaceID.String(), testWorkspaceID.String())
	require.NoError(t, err)

	_, err = env.engine.clearOutbox(context.Background(), testWorkspaceID.String())
	require.NoError(t, err)

	assert.Zero(t, seenRows(t, env.db))
}

func TestApplyEntity_InboundEntitiesOfEveryTypeAreSynced(t *testing.T) {
	sender := newExampleSyncEnv(t)
	receiver := newExampleSyncEnv(t)
	ctx := context.Background()
	fixture := newEveryType()
	fixture.seed(t, sender.db)

	for entityType, entry := range fixture.deleteEntries() {
		entity, err := sender.ws.readEntity(ctx, entityType, entry.EntityID)
		require.NoError(t, err)
		require.NoError(t, receiver.ws.applyInbound(ctx, entityProto(entity, "")), entityType)

		var synced int
		require.NoError(t, receiver.db.QueryRow(`SELECT is_synced FROM `+entityType+`s WHERE id = ?`, entry.EntityID).Scan(&synced))
		assert.Equal(t, 1, synced, entityType)
	}
}
