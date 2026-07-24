package sync

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tetiva-app/client/internal/domain/entities"
	"github.com/tetiva-app/client/internal/domain/usecase/collection"
	"github.com/tetiva-app/client/internal/domain/usecase/environment"
	"github.com/tetiva-app/client/internal/domain/usecase/request"
	"github.com/tetiva-app/client/internal/infrastructure/repository/sqlite"
)

type stubCollectionRepo struct {
	data map[uuid.UUID]*entities.Collection
}

func newStubCollectionRepo() *stubCollectionRepo {
	return &stubCollectionRepo{data: make(map[uuid.UUID]*entities.Collection)}
}

func (r *stubCollectionRepo) Create(_ context.Context, c *entities.Collection) error {
	r.data[c.ID] = c
	return nil
}

func (r *stubCollectionRepo) GetByID(_ context.Context, id uuid.UUID) (*entities.Collection, error) {
	c, ok := r.data[id]
	if !ok {
		return nil, nil
	}
	return c, nil
}

func (r *stubCollectionRepo) List(_ context.Context, _ collection.Filter) ([]*entities.Collection, error) {
	return nil, nil
}

func (r *stubCollectionRepo) Update(_ context.Context, c *entities.Collection) error {
	r.data[c.ID] = c
	return nil
}

func (r *stubCollectionRepo) UpdateSortOrder(_ context.Context, _ uuid.UUID, _ int) error {
	return nil
}

func (r *stubCollectionRepo) SoftDeleteDescendants(_ context.Context, _ uuid.UUID, _ string, _ time.Time) error {
	return nil
}

type stubRequestRepo struct {
	data map[uuid.UUID]*entities.Request
}

func newStubRequestRepo() *stubRequestRepo {
	return &stubRequestRepo{data: make(map[uuid.UUID]*entities.Request)}
}

func (r *stubRequestRepo) Create(_ context.Context, req *entities.Request) error {
	r.data[req.ID] = req
	return nil
}

func (r *stubRequestRepo) GetByID(_ context.Context, id uuid.UUID) (*entities.Request, error) {
	req, ok := r.data[id]
	if !ok {
		return nil, nil
	}
	return req, nil
}

func (r *stubRequestRepo) List(_ context.Context, _ request.Filter) ([]*entities.Request, error) {
	return nil, nil
}

func (r *stubRequestRepo) Update(_ context.Context, req *entities.Request) error {
	r.data[req.ID] = req
	return nil
}

func (r *stubRequestRepo) UpdateSortOrder(_ context.Context, _ uuid.UUID, _ int) error {
	return nil
}

func (r *stubRequestRepo) DeleteHard(_ context.Context, id uuid.UUID) error {
	delete(r.data, id)
	return nil
}

func (r *stubRequestRepo) CleanupDrafts(_ context.Context) (int, error) {
	n := 0
	for id, req := range r.data {
		if req.IsDraft {
			delete(r.data, id)
			n++
		}
	}
	return n, nil
}

type stubEnvironmentRepo struct {
	data map[uuid.UUID]*entities.Environment
}

func newStubEnvironmentRepo() *stubEnvironmentRepo {
	return &stubEnvironmentRepo{data: make(map[uuid.UUID]*entities.Environment)}
}

func (r *stubEnvironmentRepo) Create(_ context.Context, e *entities.Environment) error {
	r.data[e.ID] = e
	return nil
}

func (r *stubEnvironmentRepo) GetByID(_ context.Context, id uuid.UUID) (*entities.Environment, error) {
	e, ok := r.data[id]
	if !ok {
		return nil, nil
	}
	return e, nil
}

func (r *stubEnvironmentRepo) List(_ context.Context, _ environment.Filter) ([]*entities.Environment, error) {
	return nil, nil
}

func (r *stubEnvironmentRepo) Update(_ context.Context, e *entities.Environment) error {
	r.data[e.ID] = e
	return nil
}

func (r *stubEnvironmentRepo) GetActive(_ context.Context, _ uuid.UUID) (*entities.Environment, error) {
	return nil, nil
}

func (r *stubEnvironmentRepo) SetActive(_ context.Context, _ uuid.UUID, _ uuid.UUID) error {
	return nil
}

type stubVariableRepo struct {
	data map[uuid.UUID]*entities.Variable
}

func newStubVariableRepo() *stubVariableRepo {
	return &stubVariableRepo{data: make(map[uuid.UUID]*entities.Variable)}
}

func (r *stubVariableRepo) Create(_ context.Context, v *entities.Variable) error {
	r.data[v.ID] = v
	return nil
}

func (r *stubVariableRepo) GetByID(_ context.Context, id uuid.UUID) (*entities.Variable, error) {
	v, ok := r.data[id]
	if !ok {
		return nil, nil
	}
	return v, nil
}

func (r *stubVariableRepo) List(_ context.Context, _ uuid.UUID) ([]*entities.Variable, error) {
	return nil, nil
}

func (r *stubVariableRepo) Update(_ context.Context, v *entities.Variable) error {
	r.data[v.ID] = v
	return nil
}

func (r *stubVariableRepo) Delete(_ context.Context, _ uuid.UUID) error {
	return nil
}

func newTestEngine(t *testing.T) *SyncEngine {
	t.Helper()
	db := setupTestDB(t)
	queueRepo := sqlite.NewSyncQueueRepo(db)
	configRepo := sqlite.NewSyncConfigRepo(db)
	auth := NewSyncAuthManager(configRepo)

	return NewSyncEngine(
		auth,
		queueRepo,
		configRepo,
		db,
		newStubCollectionRepo(),
		newStubRequestRepo(),
		newStubEnvironmentRepo(),
		newStubVariableRepo(),
	)
}

func TestSyncEngine_NewEngine_DefaultState(t *testing.T) {
	engine := newTestEngine(t)

	state := engine.GetWorkspaceState("non-existent-workspace")
	assert.Equal(t, StateDisconnected, state)
}

func TestSyncEngine_IsEnabledForWorkspace_DefaultFalse(t *testing.T) {
	engine := newTestEngine(t)

	wsID := uuid.New().String()
	assert.False(t, engine.IsEnabledForWorkspace(wsID))
}

func TestSyncEngine_SetEventEmitter(t *testing.T) {
	engine := newTestEngine(t)

	called := false
	engine.SetEventEmitter(func(name string, data any) {
		called = true
	})

	ws := &workspaceSyncer{
		engine:           engine,
		localWorkspaceID: uuid.New().String(),
		state:            StateIdle,
		pushSignal:       make(chan struct{}, 1),
	}
	ws.setState(StatePushing)
	assert.True(t, called, "event emitter should have been called on setState")
}

func TestSyncEngine_StartWorkspace_EnablesWorkspace(t *testing.T) {
	// The run() goroutine fails fast (no gRPC client); only the enabled flag,
	// which StartWorkspace sets synchronously before spawning, is asserted.
	db := setupTestDB(t)
	queueRepo := sqlite.NewSyncQueueRepo(db)
	configRepo := sqlite.NewSyncConfigRepo(db)
	auth := NewSyncAuthManager(configRepo)

	engine := NewSyncEngine(
		auth,
		queueRepo,
		configRepo,
		db,
		newStubCollectionRepo(),
		newStubRequestRepo(),
		newStubEnvironmentRepo(),
		newStubVariableRepo(),
	)

	wsID := uuid.New().String()
	assert.False(t, engine.IsEnabledForWorkspace(wsID), "should be disabled before start")

	engine.StartWorkspace(wsID, "remote-"+wsID, 0)
	assert.True(t, engine.IsEnabledForWorkspace(wsID), "should be enabled after start")

	engine.StopWorkspace(wsID)
}

func TestSyncEngine_StopWorkspace_DisablesWorkspace(t *testing.T) {
	engine := newTestEngine(t)

	wsID := uuid.New().String()

	ctx, cancel := context.WithCancel(context.Background())
	ws := &workspaceSyncer{
		engine:           engine,
		localWorkspaceID: wsID,
		state:            StateConnected,
		pushSignal:       make(chan struct{}, 1),
		cancel:           cancel,
	}
	engine.workspaces.Store(wsID, ws)
	engine.enabledWorkspaces.Store(wsID, true)

	require.True(t, engine.IsEnabledForWorkspace(wsID))
	require.Equal(t, StateConnected, engine.GetWorkspaceState(wsID))

	engine.StopWorkspace(wsID)

	assert.False(t, engine.IsEnabledForWorkspace(wsID))
	// Removed from the map → reports Disconnected.
	assert.Equal(t, StateDisconnected, engine.GetWorkspaceState(wsID))
	assert.ErrorIs(t, ctx.Err(), context.Canceled)
}

func TestSyncEngine_StopAll(t *testing.T) {
	engine := newTestEngine(t)

	wsIDs := []string{uuid.New().String(), uuid.New().String(), uuid.New().String()}
	var cancels []context.CancelFunc

	for _, wsID := range wsIDs {
		_, cancel := context.WithCancel(context.Background())
		cancels = append(cancels, cancel)
		ws := &workspaceSyncer{
			engine:           engine,
			localWorkspaceID: wsID,
			state:            StateConnected,
			pushSignal:       make(chan struct{}, 1),
			cancel:           cancel,
		}
		engine.workspaces.Store(wsID, ws)
		engine.enabledWorkspaces.Store(wsID, true)
	}

	engine.StopAll()

	for _, wsID := range wsIDs {
		assert.False(t, engine.IsEnabledForWorkspace(wsID), "workspace should be disabled after StopAll")
		assert.Equal(t, StateDisconnected, engine.GetWorkspaceState(wsID), "workspace should report Disconnected after StopAll")
	}

	for _, cancel := range cancels {
		cancel()
	}
}

func TestSyncEngine_NotifyWrite_SignalDelivered(t *testing.T) {
	engine := newTestEngine(t)

	wsID := uuid.New().String()
	_, cancel := context.WithCancel(context.Background())
	defer cancel()

	ws := &workspaceSyncer{
		engine:           engine,
		localWorkspaceID: wsID,
		state:            StateConnected,
		pushSignal:       make(chan struct{}, 1),
		cancel:           cancel,
	}
	engine.workspaces.Store(wsID, ws)

	engine.NotifyWrite(wsID)

	select {
	case <-ws.pushSignal:
	default:
		t.Fatal("expected push signal to be delivered, channel was empty")
	}
}

func TestSyncEngine_NotifyWrite_NonBlockingOnFullChannel(t *testing.T) {
	engine := newTestEngine(t)

	wsID := uuid.New().String()
	_, cancel := context.WithCancel(context.Background())
	defer cancel()

	ws := &workspaceSyncer{
		engine:           engine,
		localWorkspaceID: wsID,
		state:            StateConnected,
		pushSignal:       make(chan struct{}, 1),
		cancel:           cancel,
	}
	engine.workspaces.Store(wsID, ws)

	ws.pushSignal <- struct{}{}

	done := make(chan struct{})
	go func() {
		engine.NotifyWrite(wsID)
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(100 * time.Millisecond):
		t.Fatal("NotifyWrite blocked on full channel")
	}
}

func TestSyncEngine_NotifyWrite_UnknownWorkspace_NoOp(t *testing.T) {
	engine := newTestEngine(t)

	engine.NotifyWrite("workspace-that-does-not-exist")
}

func TestSyncEngine_GetWorkspaceState_KnownWorkspace(t *testing.T) {
	engine := newTestEngine(t)

	wsID := uuid.New().String()
	_, cancel := context.WithCancel(context.Background())
	defer cancel()

	ws := &workspaceSyncer{
		engine:           engine,
		localWorkspaceID: wsID,
		state:            StateOffline,
		pushSignal:       make(chan struct{}, 1),
		cancel:           cancel,
	}
	engine.workspaces.Store(wsID, ws)

	assert.Equal(t, StateOffline, engine.GetWorkspaceState(wsID))

	ws.setState(StateConnected)
	assert.Equal(t, StateConnected, engine.GetWorkspaceState(wsID))
}

func TestSyncEngine_GetPendingCount(t *testing.T) {
	db := setupTestDB(t)
	queueRepo := sqlite.NewSyncQueueRepo(db)
	configRepo := sqlite.NewSyncConfigRepo(db)
	auth := NewSyncAuthManager(configRepo)

	engine := NewSyncEngine(
		auth,
		queueRepo,
		configRepo,
		db,
		newStubCollectionRepo(),
		newStubRequestRepo(),
		newStubEnvironmentRepo(),
		newStubVariableRepo(),
	)

	ctx := context.Background()
	wsID := testWorkspaceID.String()

	count, err := engine.GetPendingCount(ctx, wsID)
	require.NoError(t, err)
	assert.Equal(t, 0, count)

	for i := 0; i < 3; i++ {
		err := queueRepo.Enqueue(ctx, sqlite.SyncEntry{
			WorkspaceID: wsID,
			EntityType:  "collection",
			EntityID:    uuid.New().String(),
			Action:      "create",
			OperationID: uuid.New().String(),
			Status:      "pending",
			CreatedAt:   time.Now(),
		})
		require.NoError(t, err)
	}

	count, err = engine.GetPendingCount(ctx, wsID)
	require.NoError(t, err)
	assert.Equal(t, 3, count)
}

func TestWorkspaceSyncer_GetSetState_ThreadSafe(t *testing.T) {
	engine := newTestEngine(t)

	ws := &workspaceSyncer{
		engine:           engine,
		localWorkspaceID: uuid.New().String(),
		state:            StateIdle,
		pushSignal:       make(chan struct{}, 1),
	}
	_, ws.cancel = context.WithCancel(context.Background())

	done := make(chan struct{})
	go func() {
		for i := 0; i < 50; i++ {
			ws.setState(StatePushing)
			ws.setState(StateConnected)
		}
		close(done)
	}()

	for i := 0; i < 50; i++ {
		_ = ws.getState()
	}
	<-done
}

func TestSyncEngine_WorkspaceSyncer_StateTransitions(t *testing.T) {
	engine := newTestEngine(t)

	wsID := uuid.New().String()
	_, cancel := context.WithCancel(context.Background())

	ws := &workspaceSyncer{
		engine:           engine,
		localWorkspaceID: wsID,
		state:            StateIdle,
		pushSignal:       make(chan struct{}, 1),
		cancel:           cancel,
	}
	engine.workspaces.Store(wsID, ws)
	engine.enabledWorkspaces.Store(wsID, true)

	assert.Equal(t, StateIdle, engine.GetWorkspaceState(wsID))

	transitions := []SyncState{
		StatePushing,
		StatePulling,
		StateSubscribing,
		StateConnected,
		StateOffline,
		StateResyncing,
		StateDisconnected,
	}

	for _, s := range transitions {
		ws.setState(s)
		assert.Equal(t, s, engine.GetWorkspaceState(wsID))
	}

	cancel()
}

func TestSyncEngine_SetGRPCClient(t *testing.T) {
	engine := newTestEngine(t)
	assert.Nil(t, engine.grpcClient)

	// GRPCClient requires a live server — just verify the field is set.
	fakeClient := &GRPCClient{}
	engine.SetGRPCClient(fakeClient)
	assert.Equal(t, fakeClient, engine.grpcClient)
}

func TestSyncEngine_MultipleWorkspaces(t *testing.T) {
	engine := newTestEngine(t)

	wsA := uuid.New().String()
	wsB := uuid.New().String()

	_, cancelA := context.WithCancel(context.Background())
	_, cancelB := context.WithCancel(context.Background())
	defer cancelA()
	defer cancelB()

	wsaSyncer := &workspaceSyncer{
		engine:           engine,
		localWorkspaceID: wsA,
		state:            StateConnected,
		pushSignal:       make(chan struct{}, 1),
		cancel:           cancelA,
	}
	wsbSyncer := &workspaceSyncer{
		engine:           engine,
		localWorkspaceID: wsB,
		state:            StateOffline,
		pushSignal:       make(chan struct{}, 1),
		cancel:           cancelB,
	}

	engine.workspaces.Store(wsA, wsaSyncer)
	engine.workspaces.Store(wsB, wsbSyncer)
	engine.enabledWorkspaces.Store(wsA, true)
	engine.enabledWorkspaces.Store(wsB, true)

	assert.Equal(t, StateConnected, engine.GetWorkspaceState(wsA))
	assert.Equal(t, StateOffline, engine.GetWorkspaceState(wsB))
	assert.True(t, engine.IsEnabledForWorkspace(wsA))
	assert.True(t, engine.IsEnabledForWorkspace(wsB))

	engine.StopWorkspace(wsA)
	assert.False(t, engine.IsEnabledForWorkspace(wsA))
	assert.True(t, engine.IsEnabledForWorkspace(wsB), "stopping wsA must not affect wsB")
}

var (
	_ collection.Repository          = (*stubCollectionRepo)(nil)
	_ request.Repository             = (*stubRequestRepo)(nil)
	_ environment.Repository         = (*stubEnvironmentRepo)(nil)
	_ environment.VariableRepository = (*stubVariableRepo)(nil)
)

func TestSyncEngine_DB_NotNil(t *testing.T) {
	engine := newTestEngine(t)
	assert.NotNil(t, engine.db)
}

var _ *sql.DB = (*sql.DB)(nil)

// newSyncer is a helper that inserts a pre-built workspaceSyncer into the engine
// without starting a goroutine, giving tests full control over state.
func newSyncer(engine *SyncEngine, wsID string, state SyncState) (*workspaceSyncer, context.CancelFunc) {
	_, cancel := context.WithCancel(context.Background())
	ws := &workspaceSyncer{
		engine:            engine,
		localWorkspaceID:  wsID,
		remoteWorkspaceID: "remote-" + wsID,
		state:             state,
		pushSignal:        make(chan struct{}, 1),
		cancel:            cancel,
	}
	engine.workspaces.Store(wsID, ws)
	engine.enabledWorkspaces.Store(wsID, true)
	return ws, cancel
}

func TestSyncEngine_Pause_StopsGoroutine(t *testing.T) {
	engine := newTestEngine(t)
	wsID := uuid.New().String()

	ws, outerCancel := newSyncer(engine, wsID, StateConnected)
	defer outerCancel()

	require.NoError(t, engine.Pause(wsID))

	ws.mu.RLock()
	paused := ws.paused
	ws.mu.RUnlock()
	assert.True(t, paused, "paused flag must be set after Pause()")
}

func TestSyncEngine_Pause_Idempotent(t *testing.T) {
	engine := newTestEngine(t)
	wsID := uuid.New().String()

	_, outerCancel := newSyncer(engine, wsID, StateConnected)
	defer outerCancel()

	require.NoError(t, engine.Pause(wsID))
	require.NoError(t, engine.Pause(wsID), "Pause must be idempotent")
}

func TestSyncEngine_Pause_UnknownWorkspace(t *testing.T) {
	engine := newTestEngine(t)
	err := engine.Pause("non-existent-workspace")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not syncing")
}

func TestSyncEngine_Pause_QueueRetainsWrites(t *testing.T) {
	db := setupTestDB(t)
	queueRepo := sqlite.NewSyncQueueRepo(db)
	configRepo := sqlite.NewSyncConfigRepo(db)
	auth := NewSyncAuthManager(configRepo)

	engine := NewSyncEngine(auth, queueRepo, configRepo, db,
		newStubCollectionRepo(), newStubRequestRepo(),
		newStubEnvironmentRepo(), newStubVariableRepo())

	wsID := testWorkspaceID.String()
	_, outerCancel := newSyncer(engine, wsID, StateConnected)
	defer outerCancel()

	ctx := context.Background()
	for i := 0; i < 2; i++ {
		require.NoError(t, queueRepo.Enqueue(ctx, sqlite.SyncEntry{
			WorkspaceID: wsID,
			EntityType:  "collection",
			EntityID:    uuid.New().String(),
			Action:      "create",
			OperationID: uuid.New().String(),
			Status:      "pending",
			CreatedAt:   time.Now(),
		}))
	}

	require.NoError(t, engine.Pause(wsID))

	count, err := engine.GetPendingCount(ctx, wsID)
	require.NoError(t, err)
	assert.Equal(t, 2, count, "queue must retain writes after Pause")
}

func TestSyncEngine_Resume_RestartsWorkspace(t *testing.T) {
	engine := newTestEngine(t)
	wsID := uuid.New().String()

	_, outerCancel := newSyncer(engine, wsID, StateConnected)
	defer outerCancel()

	require.NoError(t, engine.Pause(wsID))
	require.NoError(t, engine.Resume(wsID))

	assert.True(t, engine.IsEnabledForWorkspace(wsID))

	v, ok := engine.workspaces.Load(wsID)
	require.True(t, ok, "syncer must exist in the map after Resume")

	newWS := v.(*workspaceSyncer)
	newWS.mu.RLock()
	isPaused := newWS.paused
	newWS.mu.RUnlock()
	assert.False(t, isPaused, "resumed syncer must not be marked as paused")
}

func TestSyncEngine_Resume_Idempotent(t *testing.T) {
	engine := newTestEngine(t)
	wsID := uuid.New().String()

	_, outerCancel := newSyncer(engine, wsID, StateConnected)
	defer outerCancel()

	require.NoError(t, engine.Resume(wsID), "Resume on running workspace must be idempotent")
}

func TestSyncEngine_Resume_UnknownWorkspace(t *testing.T) {
	engine := newTestEngine(t)
	err := engine.Resume("non-existent-workspace")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not syncing")
}

func TestSyncEngine_Resume_StateTransitionObservable(t *testing.T) {
	// Channel gives race-free observation of state events across goroutines.
	engine := newTestEngine(t)

	stateCh := make(chan string, 10)
	engine.SetEventEmitter(func(name string, data any) {
		if name == "sync:status" {
			if m, ok := data.(map[string]any); ok {
				select {
				case stateCh <- m["state"].(string):
				default:
				}
			}
		}
	})

	wsID := uuid.New().String()
	_, outerCancel := newSyncer(engine, wsID, StateConnected)
	defer outerCancel()

	require.NoError(t, engine.Pause(wsID))
	require.NoError(t, engine.Resume(wsID))

	// The restarted goroutine will emit at least one state event before failing on nil grpcClient.
	select {
	case state := <-stateCh:
		assert.NotEmpty(t, state, "emitted state must be non-empty")
	case <-time.After(500 * time.Millisecond):
		t.Fatal("timed out waiting for state transition event after Resume")
	}
}

func TestSyncEngine_DisconnectStream_NoActiveStream(t *testing.T) {
	engine := newTestEngine(t)
	wsID := uuid.New().String()

	_, outerCancel := newSyncer(engine, wsID, StateConnected)
	defer outerCancel()

	err := engine.DisconnectStream(wsID)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no active stream")
}

func TestSyncEngine_DisconnectStream_CancelsStream(t *testing.T) {
	engine := newTestEngine(t)
	wsID := uuid.New().String()

	ws, outerCancel := newSyncer(engine, wsID, StateConnected)
	defer outerCancel()

	// Inject a fake streamCancel so DisconnectStream has something to cancel.
	streamCtx, streamCancel := context.WithCancel(context.Background())
	ws.mu.Lock()
	ws.streamCancel = streamCancel
	ws.mu.Unlock()

	require.NoError(t, engine.DisconnectStream(wsID))

	assert.ErrorIs(t, streamCtx.Err(), context.Canceled)

	ws.mu.RLock()
	remaining := ws.streamCancel
	ws.mu.RUnlock()
	assert.Nil(t, remaining, "streamCancel must be cleared after DisconnectStream")
}

func TestSyncEngine_DisconnectStream_UnknownWorkspace(t *testing.T) {
	engine := newTestEngine(t)
	err := engine.DisconnectStream("non-existent-workspace")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not syncing")
}

func TestSyncEngine_PauseResume_ConcurrentSafe(t *testing.T) {
	engine := newTestEngine(t)
	wsID := uuid.New().String()

	_, outerCancel := newSyncer(engine, wsID, StateConnected)
	defer outerCancel()

	done := make(chan struct{})
	go func() {
		for i := 0; i < 20; i++ {
			_ = engine.Pause(wsID)
			_ = engine.Resume(wsID)
		}
		close(done)
	}()

	for i := 0; i < 20; i++ {
		_ = engine.Pause(wsID)
		_ = engine.Resume(wsID)
	}
	<-done
}
