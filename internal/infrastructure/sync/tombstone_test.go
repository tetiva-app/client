package sync

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	syncv1 "github.com/tetiva-app/proto/go/gophercourier/sync/v1"

	"github.com/tetiva-app/client/internal/domain/entities"
	"github.com/tetiva-app/client/internal/infrastructure/repository/sqlite"
)

// everyType is one entity of each synced type, the same ids for every database it is seeded into.
type everyType struct {
	coll     entities.Collection
	req      entities.Request
	env      entities.Environment
	variable entities.Variable
	ex       entities.ResponseExample
}

func newEveryType() everyType {
	now := time.Now().Truncate(time.Second)
	coll := newTestCollection("Doomed folder", nil)
	req := newLocalRequest("Doomed request", coll.ID)
	env := entities.Environment{
		ID: uuid.New(), WorkspaceID: testWorkspaceID, Name: "Doomed env", Version: 1,
		CreatedBy: "test_user", CreatedAt: now, UpdatedBy: "test_user", UpdatedAt: now,
	}
	return everyType{
		coll: *coll,
		req:  *req,
		env:  env,
		variable: entities.Variable{
			ID: uuid.New(), EnvironmentID: env.ID, Key: "token", Value: "abc", Enabled: true, Version: 1,
			CreatedBy: "test_user", CreatedAt: now, UpdatedBy: "test_user", UpdatedAt: now,
		},
		ex: entities.ResponseExample{
			ID: uuid.New(), RequestID: req.ID, WorkspaceID: testWorkspaceID, Name: "Doomed example", StatusCode: 200,
			StatusText: "OK", Headers: []entities.HeaderItem{}, Body: "{}", Protocol: entities.ProtocolHTTP, Version: 1,
			CreatedBy: "test_user", CreatedAt: now, UpdatedBy: "test_user", UpdatedAt: now,
		},
	}
}

func (e everyType) seed(t *testing.T, db *sql.DB) {
	t.Helper()
	ctx := context.Background()
	coll, req, env, variable, ex := e.coll, e.req, e.env, e.variable, e.ex
	require.NoError(t, sqlite.NewCollectionRepo(db).Create(ctx, &coll))
	require.NoError(t, sqlite.NewRequestRepo(db).Create(ctx, &req))
	require.NoError(t, sqlite.NewEnvironmentRepo(db).Create(ctx, &env))
	require.NoError(t, sqlite.NewVariableRepo(db).Create(ctx, &variable))
	require.NoError(t, sqlite.NewResponseExampleRepo(db).Create(ctx, &ex))
}

// softDelete deletes every entity locally the way the usecases do: is_delete and a new version.
func (e everyType) softDelete(t *testing.T, db *sql.DB) {
	t.Helper()
	ctx := context.Background()
	coll, req, env, variable, ex := e.coll, e.req, e.env, e.variable, e.ex
	coll.IsDelete, req.IsDelete, env.IsDelete, variable.IsDelete, ex.IsDelete = true, true, true, true, true
	coll.Version, req.Version, env.Version, variable.Version, ex.Version = 2, 2, 2, 2, 2
	require.NoError(t, sqlite.NewCollectionRepo(db).Update(ctx, &coll))
	require.NoError(t, sqlite.NewRequestRepo(db).Update(ctx, &req))
	require.NoError(t, sqlite.NewEnvironmentRepo(db).Update(ctx, &env))
	require.NoError(t, sqlite.NewVariableRepo(db).Update(ctx, &variable))
	require.NoError(t, sqlite.NewResponseExampleRepo(db).Update(ctx, &ex))
}

func (e everyType) deleteEntries() map[string]*sqlite.SyncEntry {
	entry := func(entityType string, id uuid.UUID) *sqlite.SyncEntry {
		return &sqlite.SyncEntry{WorkspaceID: testWorkspaceID.String(), EntityType: entityType, EntityID: id.String(),
			Action: "delete", OperationID: "op-" + entityType}
	}
	return map[string]*sqlite.SyncEntry{
		"collection":       entry("collection", e.coll.ID),
		"request":          entry("request", e.req.ID),
		"environment":      entry("environment", e.env.ID),
		"variable":         entry("variable", e.variable.ID),
		"response_example": entry("response_example", e.ex.ID),
	}
}

func isDeletedRow(t *testing.T, db *sql.DB, entityType, id string) bool {
	t.Helper()
	var deleted bool
	require.NoError(t, db.QueryRow(`SELECT is_delete FROM `+entityType+`s WHERE id = ?`, id).Scan(&deleted))
	return deleted
}

func hasPayload(e *syncv1.SyncEntity) bool {
	return e.HasCollection() || e.HasRequest() || e.HasEnvironment() || e.HasVariable() || e.HasResponseExample()
}

func TestTombstone_OlderServerGetsTheDeletedRowAndThePushTime(t *testing.T) {
	env := newExampleSyncEnv(t)
	ctx := context.Background()
	fixture := newEveryType()
	fixture.seed(t, env.db)
	fixture.softDelete(t, env.db)
	env.ws.examplesCap = capUnsupported

	for entityType, entry := range fixture.deleteEntries() {
		tombstone, version, known := env.ws.tombstone(ctx, entry)
		require.NotNil(t, tombstone, entityType)
		assert.True(t, tombstone.GetIsDeleted(), entityType)
		assert.Equal(t, entry.OperationID, tombstone.GetOperationId(), entityType)
		assert.True(t, hasPayload(tombstone), "%s: an older server stores the tombstone whole", entityType)
		require.True(t, tombstone.HasUpdatedAt(), entityType)
		assert.WithinDuration(t, time.Now(), tombstone.GetUpdatedAt().AsTime(), 2*time.Second, entityType)
		assert.True(t, known, entityType)
		assert.Equal(t, 2, version, entityType)
	}
	tombstone, _, _ := env.ws.tombstone(ctx, fixture.deleteEntries()["request"])
	assert.Equal(t, fixture.coll.ID.String(), tombstone.GetRequest().GetCollectionId())
	tombstone, _, _ = env.ws.tombstone(ctx, fixture.deleteEntries()["variable"])
	assert.Equal(t, fixture.env.ID.String(), tombstone.GetVariable().GetEnvironmentId())
}

func TestTombstone_ServerWithTheCapabilityGetsABareUndatedTombstone(t *testing.T) {
	env := newExampleSyncEnv(t)
	ctx := context.Background()
	fixture := newEveryType()
	fixture.seed(t, env.db)
	fixture.softDelete(t, env.db)

	for entityType, entry := range fixture.deleteEntries() {
		tombstone, version, known := env.ws.tombstone(ctx, entry)
		require.NotNil(t, tombstone, entityType)
		assert.True(t, tombstone.GetIsDeleted(), entityType)
		assert.False(t, hasPayload(tombstone), entityType)
		assert.False(t, tombstone.HasUpdatedAt(), "%s: the server dates it", entityType)
		assert.True(t, known, entityType)
		assert.Equal(t, 2, version, entityType)
	}
}

func TestTombstone_RowGoneForGoodFallsBackToABareDatedTombstone(t *testing.T) {
	env := newExampleSyncEnv(t)
	env.ws.examplesCap = capUnsupported
	entry := &sqlite.SyncEntry{EntityType: "variable", EntityID: uuid.NewString(), Action: "delete", OperationID: "op"}

	tombstone, _, known := env.ws.tombstone(context.Background(), entry)

	require.NotNil(t, tombstone)
	assert.False(t, hasPayload(tombstone))
	assert.True(t, tombstone.HasUpdatedAt())
	assert.False(t, known)
}

func TestTombstone_OlderServersTombstoneAppliesOnAnotherClient(t *testing.T) {
	sender := newExampleSyncEnv(t)
	receiver := newExampleSyncEnv(t)
	ctx := context.Background()
	fixture := newEveryType()
	fixture.seed(t, sender.db)
	fixture.softDelete(t, sender.db)
	fixture.seed(t, receiver.db)
	sender.ws.examplesCap = capUnsupported

	for entityType, entry := range fixture.deleteEntries() {
		tombstone, _, _ := sender.ws.tombstone(ctx, entry)
		require.NotNil(t, tombstone, entityType)

		require.NoError(t, receiver.ws.applyEntity(ctx, tombstone), entityType)

		assert.True(t, isDeletedRow(t, receiver.db, entityType, entry.EntityID), entityType)
	}
}

func TestTombstone_OlderServersTombstoneAppliesToARowDeletedHereToo(t *testing.T) {
	sender := newExampleSyncEnv(t)
	receiver := newExampleSyncEnv(t)
	ctx := context.Background()
	fixture := newEveryType()
	fixture.seed(t, sender.db)
	fixture.softDelete(t, sender.db)
	fixture.seed(t, receiver.db)
	fixture.softDelete(t, receiver.db)
	sender.ws.examplesCap = capUnsupported

	for entityType, entry := range fixture.deleteEntries() {
		tombstone, _, _ := sender.ws.tombstone(ctx, entry)
		require.NotNil(t, tombstone, entityType)

		require.NoError(t, receiver.ws.applyInbound(ctx, tombstone), entityType)

		assert.True(t, isDeletedRow(t, receiver.db, entityType, entry.EntityID), entityType)
	}
}

func TestApplyEntity_ServersLiveCopyRevivesARowDeletedHere(t *testing.T) {
	sender := newExampleSyncEnv(t)
	receiver := newExampleSyncEnv(t)
	ctx := context.Background()
	fixture := newEveryType()
	fixture.seed(t, sender.db)
	fixture.seed(t, receiver.db)
	fixture.softDelete(t, receiver.db)

	for entityType, entry := range fixture.deleteEntries() {
		entity, err := sender.ws.readEntity(ctx, entityType, entry.EntityID)
		require.NoError(t, err)
		live := entityProto(entity, "")
		live.SetVersion(3)

		require.NoError(t, receiver.ws.applyInbound(ctx, live), entityType)

		assert.False(t, isDeletedRow(t, receiver.db, entityType, entry.EntityID), entityType)
	}
}

func TestDrainOutbox_OlderServerTombstonesCountAgainstTheBatchBudget(t *testing.T) {
	env := newExampleSyncEnv(t)
	ctx := context.Background()
	env.ws.examplesCap = capUnsupported
	big := newLocalRequest("Big", env.coll.ID)
	big.Body = string(make([]byte, pushBatchBytes))
	require.NoError(t, env.requests.Create(ctx, big))
	big.IsDelete, big.Version = true, 2
	require.NoError(t, env.requests.Update(ctx, big))
	env.req.Name = "Edited"
	require.NoError(t, env.requests.Update(ctx, env.req))
	env.req.Name = "Edited again"
	require.NoError(t, env.requests.Update(ctx, env.req))

	require.NoError(t, env.ws.pushAll(ctx))

	batches := env.client.pushed()
	require.Len(t, batches, 2, "the tombstone carries the request body and fills a batch on its own")
	assert.Equal(t, big.ID.String(), batches[0][0].GetEntityId())
	assert.True(t, batches[0][0].GetIsDeleted())
}

var (
	_ deletedRowReader[*entities.Collection]      = (*sqlite.CollectionRepo)(nil)
	_ deletedRowReader[*entities.Request]         = (*sqlite.RequestRepo)(nil)
	_ deletedRowReader[*entities.Environment]     = (*sqlite.EnvironmentRepo)(nil)
	_ deletedRowReader[*entities.Variable]        = (*sqlite.VariableRepo)(nil)
	_ deletedRowReader[*entities.ResponseExample] = (*sqlite.ResponseExampleRepo)(nil)
)

func TestSyncedVariableRepo_DeleteWhileSyncingKeepsTheRowForItsTombstone(t *testing.T) {
	env := newExampleSyncEnv(t)
	ctx := context.Background()
	fixture := newEveryType()
	fixture.seed(t, env.db)
	inner := sqlite.NewVariableRepo(env.db)
	variables := NewSyncedVariableRepo(inner, env.engine.syncQueue, env.db, env.engine)

	require.NoError(t, variables.Delete(ctx, fixture.variable.ID))

	got, err := variables.GetByID(ctx, fixture.variable.ID)
	require.NoError(t, err)
	assert.Nil(t, got)
	listed, err := variables.List(ctx, fixture.env.ID)
	require.NoError(t, err)
	assert.Empty(t, listed)
	assert.True(t, isDeletedRow(t, env.db, "variable", fixture.variable.ID.String()))
	assert.Equal(t, 1, countQueueEntriesWithAction(t, env.db, fixture.variable.ID.String(), "delete"))

	env.ws.examplesCap = capUnsupported
	tombstone, version, _ := env.ws.tombstone(ctx, fixture.deleteEntries()["variable"])
	assert.Equal(t, fixture.env.ID.String(), tombstone.GetVariable().GetEnvironmentId())
	assert.Equal(t, 2, version)
}

func TestSyncedVariableRepo_PeersWinningEditRevivesAVariableDeletedWhileSyncing(t *testing.T) {
	env := newExampleSyncEnv(t)
	ctx := context.Background()
	fixture := newEveryType()
	fixture.seed(t, env.db)
	variables := NewSyncedVariableRepo(sqlite.NewVariableRepo(env.db), env.engine.syncQueue, env.db, env.engine)
	require.NoError(t, variables.Delete(ctx, fixture.variable.ID))

	edited := fixture.variable
	edited.Value, edited.Version, edited.UpdatedAt = "edited on B", 3, time.Now().Add(time.Minute)
	require.NoError(t, env.ws.applyInbound(ctx, VariableToProto(&edited, "")))

	got, err := variables.GetByID(ctx, fixture.variable.ID)
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, "edited on B", got.Value)
}

func TestSyncedVariableRepo_DeleteWithoutSyncStillRemovesTheRow(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()
	fixture := newEveryType()
	fixture.seed(t, db)
	variables := NewSyncedVariableRepo(sqlite.NewVariableRepo(db), sqlite.NewSyncQueueRepo(db), db, testEngineWithSync(uuid.NewString()))

	require.NoError(t, variables.Delete(ctx, fixture.variable.ID))

	var n int
	require.NoError(t, db.QueryRow(`SELECT COUNT(*) FROM variables WHERE id = ?`, fixture.variable.ID.String()).Scan(&n))
	assert.Zero(t, n)
}
