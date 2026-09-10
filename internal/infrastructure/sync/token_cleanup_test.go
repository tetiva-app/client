package sync

import (
	"context"
	"database/sql"
	"io"
	"testing"
	"time"

	"github.com/google/uuid"
	"google.golang.org/grpc"

	syncv1 "github.com/tetiva-app/proto/go/gophercourier/sync/v1"

	"github.com/tetiva-app/client/internal/domain/entities"
	"github.com/tetiva-app/client/internal/infrastructure/repository/sqlite"
)

// recordingCleaner records what the engine asked the token store to do.
type recordingCleaner struct {
	cleared []entities.AuthOwner
	sweeps  int
	// onSweep observes the database state the sweep sees, transaction included.
	onSweep func(ctx context.Context)
}

func (c *recordingCleaner) Clear(_ context.Context, owner entities.AuthOwner) error {
	c.cleared = append(c.cleared, owner)
	return nil
}

func (c *recordingCleaner) DeleteOrphans(ctx context.Context) (int, error) {
	c.sweeps++
	if c.onSweep != nil {
		c.onSweep(ctx)
	}
	return 0, nil
}

// newTokenSyncer builds a syncer over real repositories: the auth comparison
// reads the row the inbound entity is about to replace.
func newTokenSyncer(t *testing.T) (*workspaceSyncer, *sql.DB, *recordingCleaner) {
	t.Helper()
	db := setupTestDB(t)
	configRepo := sqlite.NewSyncConfigRepo(db)
	cleaner := &recordingCleaner{}
	engine := NewSyncEngine(
		NewSyncAuthManager(configRepo),
		sqlite.NewSyncQueueRepo(db),
		configRepo,
		db,
		sqlite.NewCollectionRepo(db),
		sqlite.NewRequestRepo(db),
		sqlite.NewEnvironmentRepo(db),
		sqlite.NewVariableRepo(db),
		cleaner,
	)
	ws := &workspaceSyncer{
		engine:           engine,
		localWorkspaceID: testWorkspaceID.String(),
		state:            StateIdle,
		pushSignal:       make(chan struct{}, 1),
	}
	return ws, db, cleaner
}

// newLocalRequest is the minimum a request row needs to exist under a collection.
func newLocalRequest(name string, collectionID uuid.UUID) *entities.Request {
	now := time.Now().Truncate(time.Second)
	return &entities.Request{
		ID:           uuid.New(),
		CollectionID: collectionID,
		Name:         name,
		Protocol:     entities.ProtocolHTTP,
		Method:       entities.MethodGET,
		URL:          "https://example.com",
		BodyType:     entities.BodyTypeNone,
		AuthType:     entities.AuthTypeNone,
		AuthData:     "{}",
		GRPCMetadata: map[string][]string{},
		Version:      1,
		CreatedBy:    "test_user",
		CreatedAt:    now,
		UpdatedBy:    "test_user",
		UpdatedAt:    now,
	}
}

func collectionEntity(id uuid.UUID, authType, authData string) *syncv1.SyncEntity {
	return syncv1.SyncEntity_builder{
		EntityType: syncv1.EntityType_ENTITY_TYPE_COLLECTION,
		EntityId:   id.String(),
		Collection: syncv1.CollectionData_builder{
			Name:     "Inbound",
			AuthType: authType,
			AuthData: authData,
		}.Build(),
	}.Build()
}

func requestEntity(id, collectionID uuid.UUID, authType, authData string) *syncv1.SyncEntity {
	return syncv1.SyncEntity_builder{
		EntityType: syncv1.EntityType_ENTITY_TYPE_REQUEST,
		EntityId:   id.String(),
		Request: syncv1.RequestData_builder{
			CollectionId: collectionID.String(),
			Name:         "Inbound",
			Method:       "GET",
			Url:          "https://example.com",
			AuthType:     authType,
			AuthData:     authData,
		}.Build(),
	}.Build()
}

func TestApplyEntity_CollectionAuthConfigChange_ClearsToken(t *testing.T) {
	ws, db, cleaner := newTokenSyncer(t)
	ctx := context.Background()

	local := newTestCollection("Local", nil)
	local.AuthType = entities.AuthTypeOAuth2
	local.AuthData = `{"grant":"client_credentials","scope":"read"}`
	if err := sqlite.NewCollectionRepo(db).Create(ctx, local); err != nil {
		t.Fatalf("create: %v", err)
	}

	entity := collectionEntity(local.ID, "oauth2", `{"grant":"client_credentials","scope":"write"}`)
	if err := ws.applyEntity(ctx, entity); err != nil {
		t.Fatalf("applyEntity: %v", err)
	}

	want := entities.AuthOwner{
		WorkspaceID: testWorkspaceID,
		Kind:        entities.AuthOwnerKindCollection,
		ID:          local.ID,
	}
	if len(cleaner.cleared) != 1 || cleaner.cleared[0] != want {
		t.Fatalf("cleared = %v, want [%v]", cleaner.cleared, want)
	}
}

func TestApplyEntity_UnchangedAuth_KeepsToken(t *testing.T) {
	ws, db, cleaner := newTokenSyncer(t)
	ctx := context.Background()

	local := newTestCollection("Local", nil)
	local.AuthType = entities.AuthTypeOAuth2
	local.AuthData = `{"grant":"client_credentials"}`
	if err := sqlite.NewCollectionRepo(db).Create(ctx, local); err != nil {
		t.Fatalf("create: %v", err)
	}

	entity := collectionEntity(local.ID, "oauth2", `{"grant":"client_credentials"}`)
	if err := ws.applyEntity(ctx, entity); err != nil {
		t.Fatalf("applyEntity: %v", err)
	}

	if len(cleaner.cleared) != 0 {
		t.Fatalf("cleared %v on an unchanged config", cleaner.cleared)
	}
}

func TestApplyEntity_NewEntity_ClearsNothing(t *testing.T) {
	ws, _, cleaner := newTokenSyncer(t)
	ctx := context.Background()

	entity := collectionEntity(uuid.New(), "oauth2", `{"grant":"client_credentials"}`)
	if err := ws.applyEntity(ctx, entity); err != nil {
		t.Fatalf("applyEntity: %v", err)
	}

	if len(cleaner.cleared) != 0 {
		t.Fatalf("cleared %v for an entity that never existed locally", cleaner.cleared)
	}
}

func TestApplyEntity_RequestLeavesOAuth2_ClearsToken(t *testing.T) {
	ws, db, cleaner := newTokenSyncer(t)
	ctx := context.Background()

	coll := newTestCollection("Host", nil)
	if err := sqlite.NewCollectionRepo(db).Create(ctx, coll); err != nil {
		t.Fatalf("create collection: %v", err)
	}
	local := newLocalRequest("Local", coll.ID)
	local.AuthType = entities.AuthTypeOAuth2
	local.AuthData = `{"grant":"client_credentials"}`
	if err := sqlite.NewRequestRepo(db).Create(ctx, local); err != nil {
		t.Fatalf("create request: %v", err)
	}

	entity := requestEntity(local.ID, coll.ID, "none", "{}")
	if err := ws.applyEntity(ctx, entity); err != nil {
		t.Fatalf("applyEntity: %v", err)
	}

	want := entities.AuthOwner{
		WorkspaceID: testWorkspaceID,
		Kind:        entities.AuthOwnerKindRequest,
		ID:          local.ID,
	}
	if len(cleaner.cleared) != 1 || cleaner.cleared[0] != want {
		t.Fatalf("cleared = %v, want [%v]", cleaner.cleared, want)
	}
}

func TestApplyEntity_NonAuthEntity_ClearsNothing(t *testing.T) {
	ws, db, cleaner := newTokenSyncer(t)
	ctx := context.Background()

	coll := newTestCollection("Host", nil)
	if err := sqlite.NewCollectionRepo(db).Create(ctx, coll); err != nil {
		t.Fatalf("create collection: %v", err)
	}
	local := newLocalRequest("Local", coll.ID)
	local.AuthType = entities.AuthTypeBasic
	local.AuthData = `{"username":"u"}`
	if err := sqlite.NewRequestRepo(db).Create(ctx, local); err != nil {
		t.Fatalf("create request: %v", err)
	}

	entity := requestEntity(local.ID, coll.ID, "basic", `{"username":"other"}`)
	if err := ws.applyEntity(ctx, entity); err != nil {
		t.Fatalf("applyEntity: %v", err)
	}

	if len(cleaner.cleared) != 0 {
		t.Fatalf("cleared %v for a scheme that owns no token", cleaner.cleared)
	}
}

func TestSweepTokens_CallsCleaner(t *testing.T) {
	ws, _, cleaner := newTokenSyncer(t)

	ws.sweepTokens(context.Background())

	if cleaner.sweeps != 1 {
		t.Fatalf("sweeps = %d, want 1", cleaner.sweeps)
	}
}

// fakeSyncClient answers only the RPCs the inbound-sweep tests drive; the rest
// of the interface stays nil and must not be called.
type fakeSyncClient struct {
	syncv1.SyncServiceClient
	pull   func(*syncv1.PullRequest) (*syncv1.PullResponse, error)
	push   func(*syncv1.PushRequest) (*syncv1.PushResponse, error)
	stream func() (grpc.ServerStreamingClient[syncv1.SubscribeResponse], error)
}

func (c *fakeSyncClient) Pull(_ context.Context, req *syncv1.PullRequest, _ ...grpc.CallOption) (*syncv1.PullResponse, error) {
	return c.pull(req)
}

func (c *fakeSyncClient) Push(_ context.Context, req *syncv1.PushRequest, _ ...grpc.CallOption) (*syncv1.PushResponse, error) {
	return c.push(req)
}

func (c *fakeSyncClient) Subscribe(_ context.Context, _ *syncv1.SubscribeRequest, _ ...grpc.CallOption) (grpc.ServerStreamingClient[syncv1.SubscribeResponse], error) {
	return c.stream()
}

// fakeSubscribeStream replays canned responses and then ends the stream.
type fakeSubscribeStream struct {
	grpc.ServerStreamingClient[syncv1.SubscribeResponse]
	responses []*syncv1.SubscribeResponse
}

func (s *fakeSubscribeStream) Recv() (*syncv1.SubscribeResponse, error) {
	if len(s.responses) == 0 {
		return nil, io.EOF
	}
	resp := s.responses[0]
	s.responses = s.responses[1:]
	return resp, nil
}

// newInboundSyncer wires the token syncer to a stubbed sync client and an access
// token that never needs refreshing.
func newInboundSyncer(t *testing.T, client syncv1.SyncServiceClient) (*workspaceSyncer, *sql.DB, *recordingCleaner) {
	t.Helper()
	ws, db, cleaner := newTokenSyncer(t)
	ws.remoteWorkspaceID = "remote-workspace"
	ws.engine.SetGRPCClient(&GRPCClient{sync: client})
	ws.engine.auth.mu.Lock()
	ws.engine.auth.accessToken = "access-token"
	ws.engine.auth.expiresAt = time.Now().Add(time.Hour)
	ws.engine.auth.mu.Unlock()
	if _, err := ws.engine.configRepo.GetOrCreate(context.Background()); err != nil {
		t.Fatalf("sync config: %v", err)
	}
	return ws, db, cleaner
}

func hostCollection(t *testing.T, db *sql.DB) *entities.Collection {
	t.Helper()
	coll := newTestCollection("Host", nil)
	if err := sqlite.NewCollectionRepo(db).Create(context.Background(), coll); err != nil {
		t.Fatalf("create collection: %v", err)
	}
	return coll
}

func TestPullAll_SweepsOncePerBatchInsideTheTransaction(t *testing.T) {
	client := &fakeSyncClient{}
	ws, db, cleaner := newInboundSyncer(t, client)
	coll := hostCollection(t, db)

	pulls := 0
	client.pull = func(*syncv1.PullRequest) (*syncv1.PullResponse, error) {
		pulls++
		return syncv1.PullResponse_builder{
			Changes: []*syncv1.SyncChange{
				syncv1.SyncChange_builder{
					EntityType: syncv1.EntityType_ENTITY_TYPE_REQUEST,
					EntityId:   uuid.New().String(),
					Entity:     requestEntity(uuid.New(), coll.ID, "none", "{}"),
				}.Build(),
				syncv1.SyncChange_builder{
					EntityType: syncv1.EntityType_ENTITY_TYPE_REQUEST,
					EntityId:   uuid.New().String(),
					Entity:     requestEntity(uuid.New(), coll.ID, "none", "{}"),
				}.Build(),
			},
			NextSyncSeq: 42,
		}.Build(), nil
	}

	seqAtSweep := int64(-1)
	cleaner.onSweep = func(ctx context.Context) {
		if err := sqlite.DBTXFromContext(ctx, db).QueryRowContext(ctx,
			`SELECT last_sync_seq FROM workspaces WHERE id = ?`, testWorkspaceID.String()).Scan(&seqAtSweep); err != nil {
			t.Errorf("read last_sync_seq during the sweep: %v", err)
		}
	}

	if err := ws.pullAll(context.Background()); err != nil {
		t.Fatalf("pullAll: %v", err)
	}

	if pulls != 1 {
		t.Fatalf("pull calls = %d, want 1", pulls)
	}
	if cleaner.sweeps != 1 {
		t.Fatalf("sweeps = %d, want 1 per batch", cleaner.sweeps)
	}
	if seqAtSweep != 0 {
		t.Fatalf("last_sync_seq at sweep time = %d, want the pre-batch 0", seqAtSweep)
	}

	var seq int64
	if err := db.QueryRow(`SELECT last_sync_seq FROM workspaces WHERE id = ?`, testWorkspaceID.String()).Scan(&seq); err != nil {
		t.Fatalf("read last_sync_seq: %v", err)
	}
	if seq != 42 {
		t.Fatalf("last_sync_seq = %d, want 42", seq)
	}
}

func TestPullAll_EmptyBatch_NoSweep(t *testing.T) {
	client := &fakeSyncClient{}
	ws, _, cleaner := newInboundSyncer(t, client)

	client.pull = func(*syncv1.PullRequest) (*syncv1.PullResponse, error) {
		return syncv1.PullResponse_builder{NextSyncSeq: 7}.Build(), nil
	}

	if err := ws.pullAll(context.Background()); err != nil {
		t.Fatalf("pullAll: %v", err)
	}
	if cleaner.sweeps != 0 {
		t.Fatalf("sweeps = %d, want 0", cleaner.sweeps)
	}
}

func TestSubscribe_SweepsAfterAppliedChange(t *testing.T) {
	client := &fakeSyncClient{}
	ws, db, cleaner := newInboundSyncer(t, client)
	coll := hostCollection(t, db)
	reqID := uuid.New()

	client.stream = func() (grpc.ServerStreamingClient[syncv1.SubscribeResponse], error) {
		return &fakeSubscribeStream{responses: []*syncv1.SubscribeResponse{
			syncv1.SubscribeResponse_builder{
				Change: syncv1.SyncChange_builder{
					EntityType: syncv1.EntityType_ENTITY_TYPE_REQUEST,
					EntityId:   reqID.String(),
					Entity:     requestEntity(reqID, coll.ID, "none", "{}"),
				}.Build(),
			}.Build(),
		}}, nil
	}

	if err := ws.subscribe(context.Background()); err == nil {
		t.Fatal("subscribe returned nil after the stream ended")
	}

	if cleaner.sweeps != 1 {
		t.Fatalf("sweeps = %d, want 1", cleaner.sweeps)
	}
	if r, err := sqlite.NewRequestRepo(db).GetByID(context.Background(), reqID); err != nil || r == nil {
		t.Fatalf("inbound request not applied: %v, %v", r, err)
	}
}

func TestPushAll_SweepsAfterConflictWinner(t *testing.T) {
	client := &fakeSyncClient{}
	ws, db, cleaner := newInboundSyncer(t, client)
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

	client.push = func(*syncv1.PushRequest) (*syncv1.PushResponse, error) {
		return syncv1.PushResponse_builder{Results: []*syncv1.PushResult{
			syncv1.PushResult_builder{
				EntityId: local.ID.String(),
				Status:   syncv1.PushStatus_PUSH_STATUS_CONFLICT_RESOLVED,
				Winner:   collectionEntity(local.ID, "none", "{}"),
			}.Build(),
		}}.Build(), nil
	}

	if err := ws.pushAll(ctx); err != nil {
		t.Fatalf("pushAll: %v", err)
	}

	if cleaner.sweeps != 1 {
		t.Fatalf("sweeps = %d, want 1", cleaner.sweeps)
	}
}
