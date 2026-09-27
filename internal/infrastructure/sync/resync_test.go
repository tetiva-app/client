package sync

import (
	"context"
	gosync "sync"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	syncv1 "github.com/tetiva-app/proto/go/gophercourier/sync/v1"

	"github.com/tetiva-app/client/internal/domain/usecase/example"
	"github.com/tetiva-app/client/internal/infrastructure/repository/sqlite"
)

// storingServer keeps what it is pushed and serves it back as a one-page snapshot.
type storingServer struct {
	mu       gosync.Mutex
	entities map[string]*syncv1.SyncEntity
	calls    []string
}

func newStoringServer(seed ...*syncv1.SyncEntity) *storingServer {
	s := &storingServer{entities: map[string]*syncv1.SyncEntity{}}
	for _, e := range seed {
		s.entities[e.GetEntityId()] = e
	}
	return s
}

func (s *storingServer) push(req *syncv1.PushRequest) (*syncv1.PushResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.calls = append(s.calls, "push")
	for _, e := range req.GetEntities() {
		if e.GetIsDeleted() {
			delete(s.entities, e.GetEntityId())
			continue
		}
		s.entities[e.GetEntityId()] = e
	}
	return acceptAll(req), nil
}

func (s *storingServer) pull(r *syncv1.PullRequest) (*syncv1.PullResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.calls = append(s.calls, "pull")
	if r.GetLastSyncSeq() != 0 {
		return syncv1.PullResponse_builder{NextSyncSeq: 50}.Build(), nil
	}
	var changes []*syncv1.SyncChange
	for _, e := range s.entities {
		changes = append(changes, entityChange(e))
	}
	return syncv1.PullResponse_builder{Changes: changes, NextSyncSeq: 50}.Build(), nil
}

func (s *storingServer) made() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]string(nil), s.calls...)
}

func uuidOf(t *testing.T, e *syncv1.SyncEntity) uuid.UUID {
	t.Helper()
	id, err := uuid.Parse(e.GetEntityId())
	require.NoError(t, err)
	return id
}

// unsentExampleChanges leaves one confirmed example edited and another deleted, neither pushed yet.
func unsentExampleChanges(t *testing.T, env *exampleSyncEnv) (edited, deleted, serverEdited, serverDeleted *syncv1.SyncEntity) {
	t.Helper()
	ctx := context.Background()
	e := env.createExample(t, "Server name")
	d := env.createExample(t, "Will be deleted")
	_, err := env.db.Exec(`UPDATE response_examples SET is_synced = 1`)
	require.NoError(t, err)
	_, err = env.db.Exec(`DELETE FROM sync_queue`)
	require.NoError(t, err)
	serverEdited = exampleEntity(e.ID, env.req.ID, "Server name", int32(e.Version))
	serverDeleted = exampleEntity(d.ID, env.req.ID, "Will be deleted", int32(d.Version))

	_, err = env.usecase.Edit(ctx, example.Edit{Name: "Local unsent edit", StatusCode: 200, StatusText: "OK"},
		example.EditOpt{ExampleID: e.ID, UserID: "test_user", Version: e.Version})
	require.NoError(t, err)
	require.NoError(t, env.usecase.Delete(ctx, example.DeleteOpt{ExampleID: d.ID, UserID: "test_user", Version: d.Version}))
	return exampleEntity(e.ID, env.req.ID, "", 0), exampleEntity(d.ID, env.req.ID, "", 0), serverEdited, serverDeleted
}

func TestResync_PushesRequeuedExamplesBeforeTheSnapshot(t *testing.T) {
	env := newExampleSyncEnv(t)
	ctx := context.Background()
	edited, deleted, serverEdited, serverDeleted := unsentExampleChanges(t, env)
	server := newStoringServer(serverEdited, serverDeleted)
	env.engine.SetGRPCClient(examplesCapableClient(&fakeSyncClient{pull: server.pull, push: server.push}))
	env.ws.examplesCap = capSupported

	require.NoError(t, env.ws.resync(ctx))

	require.NotEmpty(t, server.made())
	assert.Equal(t, "push", server.made()[0], "the snapshot must not land on the edits just queued again")
	assert.Equal(t, "Local unsent edit", server.entities[edited.GetEntityId()].GetResponseExample().GetName())
	assert.NotContains(t, server.entities, deleted.GetEntityId())

	row, err := sqlite.NewResponseExampleRepo(env.db).GetByIDIncludingDeleted(ctx, uuidOf(t, edited))
	require.NoError(t, err)
	assert.Equal(t, "Local unsent edit", row.Name)
	assert.True(t, isDeletedRow(t, env.db, "response_example", deleted.GetEntityId()), "a local delete stays deleted")
}

func TestPullAll_SnapshotLeavesQueuedExamplesAlone(t *testing.T) {
	env := newExampleSyncEnv(t)
	ctx := context.Background()
	edited, deleted, serverEdited, serverDeleted := unsentExampleChanges(t, env)
	server := newStoringServer(serverEdited, serverDeleted)
	env.engine.SetGRPCClient(&GRPCClient{sync: &fakeSyncClient{pull: server.pull, push: server.push}})
	env.ws.examplesCap = capUnsupported

	require.NoError(t, env.ws.resync(ctx))

	row, err := sqlite.NewResponseExampleRepo(env.db).GetByIDIncludingDeleted(ctx, uuidOf(t, edited))
	require.NoError(t, err)
	assert.Equal(t, "Local unsent edit", row.Name, "an older server gets no examples, so the local edit waits in the queue")
	assert.True(t, isDeletedRow(t, env.db, "response_example", deleted.GetEntityId()))
	assert.Len(t, queueRowsOf(t, env.db, edited.GetEntityId()), 1)
	assert.Len(t, queueRowsOf(t, env.db, deleted.GetEntityId()), 1)
}

func TestResync_DataLostLeavesOutWhatWasQueuedAgain(t *testing.T) {
	env := newExampleSyncEnv(t)
	ctx := context.Background()
	events := captureEvents(env.engine)
	env.engine.SetGRPCClient(&GRPCClient{sync: &fakeSyncClient{
		pull: func(*syncv1.PullRequest) (*syncv1.PullResponse, error) {
			return syncv1.PullResponse_builder{}.Build(), nil
		},
		push: func(req *syncv1.PushRequest) (*syncv1.PushResponse, error) { return acceptAll(req), nil },
	}})
	env.ws.examplesCap = capUnsupported

	ex := env.createExample(t, "Unsent")
	_, err := env.usecase.Edit(ctx, example.Edit{Name: "Unsent twice", StatusCode: 200, StatusText: "OK"},
		example.EditOpt{ExampleID: ex.ID, UserID: "test_user", Version: ex.Version})
	require.NoError(t, err)
	require.Len(t, queueRowsOf(t, env.db, ex.ID.String()), 2)
	lostID, _ := enqueuePending(t, env.engine, env.ws.localWorkspaceID)

	require.NoError(t, env.ws.resync(ctx))

	data, n := findEvent(*events, "sync:data_lost")
	require.Equal(t, 1, n)
	assert.Equal(t, 1, data["count"], "only the row of %s is gone for good", lostID)
}

func TestResync_NothingLostRaisesNoEvent(t *testing.T) {
	env := newExampleSyncEnv(t)
	events := captureEvents(env.engine)
	env.engine.SetGRPCClient(&GRPCClient{sync: &fakeSyncClient{
		pull: func(*syncv1.PullRequest) (*syncv1.PullResponse, error) {
			return syncv1.PullResponse_builder{}.Build(), nil
		},
	}})
	env.ws.examplesCap = capUnsupported
	env.createExample(t, "Unsent")

	require.NoError(t, env.ws.resync(context.Background()))

	_, n := findEvent(*events, "sync:data_lost")
	assert.Zero(t, n)
}
