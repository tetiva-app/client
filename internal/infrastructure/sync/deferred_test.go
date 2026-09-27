package sync

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	syncv1 "github.com/tetiva-app/proto/go/gophercourier/sync/v1"
)

func deferCountOf(t *testing.T, db *sql.DB, entityID string) int {
	t.Helper()
	var n int
	require.NoError(t, db.QueryRow(`SELECT defer_count FROM sync_queue WHERE entity_id = ?`, entityID).Scan(&n))
	return n
}

func rejectEveryParent(env *exampleSyncEnv) {
	env.client.respond = func(_ int, req *syncv1.PushRequest) (*syncv1.PushResponse, error) {
		results := make([]*syncv1.PushResult, 0, len(req.GetEntities()))
		for _, e := range req.GetEntities() {
			results = append(results, rejectedResult(e.GetEntityId(), syncv1.PushRejectReason_PUSH_REJECT_REASON_PARENT_NOT_FOUND))
		}
		return syncv1.PushResponse_builder{Results: results}.Build(), nil
	}
}

func TestDrainOutbox_QuotaParksDoNotSpendTheParentBudget(t *testing.T) {
	env := newExampleSyncEnv(t)
	ctx := context.Background()
	ex := env.createExample(t, "Held by the quota")
	var id int64
	require.NoError(t, env.db.QueryRow(`SELECT id FROM sync_queue WHERE entity_id = ?`, ex.ID.String()).Scan(&id))
	for range 4 {
		require.NoError(t, env.engine.syncQueue.MarkFailed(ctx, id, time.Now().Add(-time.Second)))
	}
	_, err := env.engine.syncQueue.RequeueDue(ctx, env.ws.localWorkspaceID, time.Now())
	require.NoError(t, err)
	rejectEveryParent(env)

	require.NoError(t, env.ws.pushAll(ctx))

	rows := queueRowsOf(t, env.db, ex.ID.String())
	require.Len(t, rows, 1, "the first miss after four plan-limit waits defers, it does not drop")
	assert.Equal(t, "deferred", rows[0].status)
	assert.Equal(t, 1, deferCountOf(t, env.db, ex.ID.String()))
	assert.Equal(t, 1, env.client.pushesOf(env.req.ID.String()), "the first miss re-offers the request")
}

func TestDrainOutbox_HeldParentKeepsTheExampleWaiting(t *testing.T) {
	env := newExampleSyncEnv(t)
	ctx := context.Background()
	ex := env.createExample(t, "Waits for its request")
	parkRequestRow(t, env.ws, env.req.ID)
	rejectEveryParent(env)
	wsID := env.ws.localWorkspaceID

	for range maxParentRetries + 2 {
		require.NoError(t, env.ws.pushAll(ctx))
		_, err := env.db.Exec(`UPDATE sync_queue SET next_retry_at = ? WHERE entity_id = ?`,
			time.Now().Add(-time.Minute).Format(time.RFC3339), ex.ID.String())
		require.NoError(t, err)
		_, err = env.engine.syncQueue.RequeueDue(ctx, wsID, time.Now())
		require.NoError(t, err)
	}

	require.Len(t, queueRowsOf(t, env.db, ex.ID.String()), 1, "a request held back by the server is not a lost parent")
	assert.Zero(t, deferCountOf(t, env.db, ex.ID.String()))
	assert.Equal(t, maxParentRetries+2, env.client.pushesOf(ex.ID.String()))
}
