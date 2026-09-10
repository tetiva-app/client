package sync

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/google/uuid"
	"google.golang.org/grpc"

	syncv1 "github.com/tetiva-app/proto/go/gophercourier/sync/v1"

	"github.com/tetiva-app/client/internal/infrastructure/repository/sqlite"
)

// recordEvents captures the engine's outbound events for the assertions below.
func recordEvents(ws *workspaceSyncer) *[]string {
	names := &[]string{}
	ws.engine.SetEventEmitter(func(name string, _ any) {
		*names = append(*names, name)
	})
	return names
}

func TestFromProto_UnknownAuthType_IsUpdateRequired(t *testing.T) {
	collID := uuid.New()

	_, err := CollectionFromProto(collectionEntity(collID, "hawk", "{}"), testWorkspaceID)
	if !errors.Is(err, ErrUpdateRequired) {
		t.Fatalf("collection error = %v, want ErrUpdateRequired", err)
	}

	_, err = RequestFromProto(requestEntity(uuid.New(), collID, "ntlm", "{}"))
	if !errors.Is(err, ErrUpdateRequired) {
		t.Fatalf("request error = %v, want ErrUpdateRequired", err)
	}

	if _, err := CollectionFromProto(collectionEntity(collID, "oauth2", "{}"), testWorkspaceID); err != nil {
		t.Fatalf("known type rejected: %v", err)
	}
	// An unset field is a peer that never wrote it, not a newer scheme.
	if _, err := RequestFromProto(requestEntity(uuid.New(), collID, "", "{}")); err != nil {
		t.Fatalf("empty auth type rejected: %v", err)
	}
}

func TestPullAll_UnknownAuthType_RollsBackBatchAndKeepsCursor(t *testing.T) {
	client := &fakeSyncClient{}
	ws, db, cleaner := newInboundSyncer(t, client)
	coll := hostCollection(t, db)
	goodID := uuid.New()

	pulls := 0
	client.pull = func(*syncv1.PullRequest) (*syncv1.PullResponse, error) {
		pulls++
		return syncv1.PullResponse_builder{
			Changes: []*syncv1.SyncChange{
				syncv1.SyncChange_builder{
					EntityType: syncv1.EntityType_ENTITY_TYPE_REQUEST,
					EntityId:   goodID.String(),
					Entity:     requestEntity(goodID, coll.ID, "none", "{}"),
				}.Build(),
				syncv1.SyncChange_builder{
					EntityType: syncv1.EntityType_ENTITY_TYPE_REQUEST,
					EntityId:   uuid.New().String(),
					Entity:     requestEntity(uuid.New(), coll.ID, "hawk", "{}"),
				}.Build(),
			},
			NextSyncSeq: 42,
		}.Build(), nil
	}

	err := ws.pullAll(context.Background())
	if !errors.Is(err, ErrUpdateRequired) {
		t.Fatalf("pullAll error = %v, want ErrUpdateRequired", err)
	}
	if pulls != 1 {
		t.Fatalf("pull calls = %d, want 1", pulls)
	}

	var seq int64
	if err := db.QueryRow(`SELECT last_sync_seq FROM workspaces WHERE id = ?`, testWorkspaceID.String()).Scan(&seq); err != nil {
		t.Fatalf("read last_sync_seq: %v", err)
	}
	if seq != 0 {
		t.Fatalf("last_sync_seq = %d, want the pre-batch 0", seq)
	}
	if ws.lastSyncSeq != 0 {
		t.Fatalf("in-memory cursor = %d, want 0", ws.lastSyncSeq)
	}

	r, err := sqlite.NewRequestRepo(db).GetByID(context.Background(), goodID)
	if err != nil {
		t.Fatalf("read the rolled-back request: %v", err)
	}
	if r != nil {
		t.Fatal("the batch was not rolled back: an applied change survived")
	}
	if cleaner.sweeps != 0 {
		t.Fatalf("sweeps = %d, want 0 for a rolled-back batch", cleaner.sweeps)
	}
}

func TestSubscribe_UnknownAuthType_ParksWithoutApplying(t *testing.T) {
	client := &fakeSyncClient{}
	ws, db, cleaner := newInboundSyncer(t, client)
	coll := hostCollection(t, db)
	reqID := uuid.New()
	events := recordEvents(ws)

	client.stream = func() (grpc.ServerStreamingClient[syncv1.SubscribeResponse], error) {
		return &fakeSubscribeStream{responses: []*syncv1.SubscribeResponse{
			syncv1.SubscribeResponse_builder{
				Change: syncv1.SyncChange_builder{
					EntityType: syncv1.EntityType_ENTITY_TYPE_REQUEST,
					EntityId:   reqID.String(),
					Entity:     requestEntity(reqID, coll.ID, "hawk", "{}"),
				}.Build(),
			}.Build(),
		}}, nil
	}

	err := ws.subscribe(context.Background())
	if !errors.Is(err, ErrUpdateRequired) {
		t.Fatalf("subscribe error = %v, want ErrUpdateRequired", err)
	}

	for _, name := range *events {
		if name == "sync:entity_updated" {
			t.Fatal("an unapplied change announced an update")
		}
	}
	if r, err := sqlite.NewRequestRepo(db).GetByID(context.Background(), reqID); err != nil || r != nil {
		t.Fatalf("request applied despite the unknown auth type: %v, %v", r, err)
	}
	if cleaner.sweeps != 0 {
		t.Fatalf("sweeps = %d, want 0", cleaner.sweeps)
	}
}

func TestPushAll_ConflictWinnerUnknownAuthType_KeepsOutboxEntry(t *testing.T) {
	client := &fakeSyncClient{}
	ws, db, _ := newInboundSyncer(t, client)
	ctx := context.Background()

	local := newTestCollection("Local", nil)
	if err := sqlite.NewCollectionRepo(db).Create(ctx, local); err != nil {
		t.Fatalf("create collection: %v", err)
	}
	if err := ws.engine.syncQueue.Enqueue(ctx, sqlite.SyncEntry{
		WorkspaceID: testWorkspaceID.String(),
		EntityType:  "collection",
		EntityID:    local.ID.String(),
		Action:      "update",
		OperationID: uuid.New().String(),
		Status:      "pending",
	}); err != nil {
		t.Fatalf("enqueue: %v", err)
	}
	events := recordEvents(ws)

	client.push = func(*syncv1.PushRequest) (*syncv1.PushResponse, error) {
		return syncv1.PushResponse_builder{Results: []*syncv1.PushResult{
			syncv1.PushResult_builder{
				EntityId: local.ID.String(),
				Status:   syncv1.PushStatus_PUSH_STATUS_CONFLICT_RESOLVED,
				Winner:   collectionEntity(local.ID, "hawk", "{}"),
			}.Build(),
		}}.Build(), nil
	}

	err := ws.pushAll(ctx)
	if !errors.Is(err, ErrUpdateRequired) {
		t.Fatalf("pushAll error = %v, want ErrUpdateRequired", err)
	}

	pending, err := ws.engine.syncQueue.CountPendingOrFailed(ctx, testWorkspaceID.String())
	if err != nil {
		t.Fatalf("count pending: %v", err)
	}
	if pending != 1 {
		t.Fatalf("outbox entries = %d, want the local edit kept", pending)
	}
	parked, err := ws.engine.syncQueue.CountParked(ctx, testWorkspaceID.String())
	if err != nil {
		t.Fatalf("count parked: %v", err)
	}
	if parked != 1 {
		t.Fatalf("parked entries = %d, want the entry held for a retry", parked)
	}
	for _, name := range *events {
		if name == "sync:entity_updated" {
			t.Fatal("an unapplied conflict winner announced an update")
		}
	}
}

func TestParkForUpdate_StopsTheSyncer(t *testing.T) {
	ws, _, _ := newTokenSyncer(t)

	if ws.parkForUpdate(errors.New("boom")) {
		t.Fatal("an ordinary error must not park the syncer")
	}
	if !ws.parkForUpdate(fmt.Errorf("apply changes: %w", ErrUpdateRequired)) {
		t.Fatal("a compatibility error must park the syncer")
	}
	if got := ws.getState(); got != StateUpdateRequired {
		t.Fatalf("state = %q, want %q", got, StateUpdateRequired)
	}
}
