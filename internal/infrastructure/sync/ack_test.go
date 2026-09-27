package sync

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	syncv1 "github.com/tetiva-app/proto/go/gophercourier/sync/v1"

	"github.com/tetiva-app/client/internal/domain/usecase/example"
)

// clientWon answers every entity the way the server answers an edit that overwrote another client's copy.
func clientWon(req *syncv1.PushRequest) *syncv1.PushResponse {
	results := make([]*syncv1.PushResult, 0, len(req.GetEntities()))
	for _, e := range req.GetEntities() {
		results = append(results, syncv1.PushResult_builder{
			EntityId: e.GetEntityId(),
			Status:   syncv1.PushStatus_PUSH_STATUS_CONFLICT_RESOLVED,
		}.Build())
	}
	return syncv1.PushResponse_builder{Results: results}.Build()
}

func TestPushAll_AckOfAnOlderVersionLeavesTheLaterEditUnsynced(t *testing.T) {
	answers := map[string]func(*syncv1.PushRequest) *syncv1.PushResponse{
		"accepted":   acceptAll,
		"client won": clientWon,
	}
	for name, answer := range answers {
		t.Run(name, func(t *testing.T) {
			env := newExampleSyncEnv(t)
			ctx := context.Background()
			ex := env.createExample(t, "Sent")

			env.client.respond = func(call int, req *syncv1.PushRequest) (*syncv1.PushResponse, error) {
				if call > 1 {
					return nil, status.Error(codes.Unavailable, "connection lost")
				}
				_, err := env.usecase.Edit(ctx, example.Edit{Name: "Saved during the push", StatusCode: 200, StatusText: "OK"},
					example.EditOpt{ExampleID: ex.ID, UserID: "test_user", Version: ex.Version})
				require.NoError(t, err)
				return answer(req), nil
			}

			require.Error(t, env.ws.pushAll(ctx), "the second push, carrying the edit, fails")
			assert.Zero(t, exampleIsSynced(t, env.db, ex.ID), "the ACK was for the version before the edit")

			_, err := env.engine.clearOutbox(ctx, testWorkspaceID.String())
			require.NoError(t, err)
			assert.Equal(t, []queueRow{{testWorkspaceID.String(), "create", "pending", 0}}, queueRowsOf(t, env.db, ex.ID.String()),
				"a resync queues the unconfirmed edit again")
		})
	}
}

func TestPushAll_ClientWonConflictMarksEveryTypeSynced(t *testing.T) {
	env := newExampleSyncEnv(t)
	ctx := context.Background()
	fixture := newEveryType()
	fixture.seed(t, env.db)
	for _, entry := range fixture.deleteEntries() {
		entry.Action, entry.Status, entry.CreatedAt = "update", "pending", time.Now()
		require.NoError(t, env.engine.syncQueue.Enqueue(ctx, *entry))
	}
	env.client.respond = func(_ int, req *syncv1.PushRequest) (*syncv1.PushResponse, error) {
		return clientWon(req), nil
	}

	require.NoError(t, env.ws.pushAll(ctx))

	for entityType, entry := range fixture.deleteEntries() {
		var synced int
		require.NoError(t, env.db.QueryRow(`SELECT is_synced FROM `+entityType+`s WHERE id = ?`, entry.EntityID).Scan(&synced))
		assert.Equal(t, 1, synced, entityType)
		assert.Empty(t, queueRowsOf(t, env.db, entry.EntityID), entityType)
	}
}

func TestPushAll_AckedTombstoneMarksTheDeletedRowSynced(t *testing.T) {
	env := newExampleSyncEnv(t)
	ctx := context.Background()
	ex := env.createExample(t, "Deleted")
	require.NoError(t, env.usecase.Delete(ctx, example.DeleteOpt{ExampleID: ex.ID, UserID: "test_user", Version: ex.Version}))

	require.NoError(t, env.ws.pushAll(ctx))

	assert.Equal(t, 1, exampleIsSynced(t, env.db, ex.ID))
	assert.Empty(t, queueRowsOf(t, env.db, ex.ID.String()))
}
