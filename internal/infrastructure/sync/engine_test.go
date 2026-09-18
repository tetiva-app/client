package sync

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"strings"
	gosync "sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"

	syncv1 "github.com/tetiva-app/proto/go/gophercourier/sync/v1"

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

func (r *stubRequestRepo) GetDescriptionByID(_ context.Context, id uuid.UUID) (string, error) {
	req, ok := r.data[id]
	if !ok {
		return "", nil
	}
	return req.Description, nil
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
		newStubVariableRepo(), nil,
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
		newStubVariableRepo(), nil,
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
		newStubVariableRepo(), nil,
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
	assert.Nil(t, engine.GRPCClient())

	// GRPCClient requires a live server — just verify the field is set.
	fakeClient := &GRPCClient{}
	engine.SetGRPCClient(fakeClient)
	assert.Equal(t, fakeClient, engine.GRPCClient())
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

// newSyncer inserts a pre-built workspaceSyncer without starting a goroutine.
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
		newStubEnvironmentRepo(), newStubVariableRepo(), nil)

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

type capturedEvent struct {
	name string
	data map[string]any
}

func captureEvents(engine *SyncEngine) *[]capturedEvent {
	events := &[]capturedEvent{}
	engine.SetEventEmitter(func(name string, data any) {
		payload, _ := data.(map[string]any)
		*events = append(*events, capturedEvent{name: name, data: payload})
	})
	return events
}

func findEvent(events []capturedEvent, name string) (map[string]any, int) {
	var data map[string]any
	count := 0
	for _, e := range events {
		if e.name == name {
			if count == 0 {
				data = e.data
			}
			count++
		}
	}
	return data, count
}

func lastEventData(events []capturedEvent, name string) map[string]any {
	var data map[string]any
	for _, e := range events {
		if e.name == name {
			data = e.data
		}
	}
	return data
}

func rejectedResult(entityID string, reason syncv1.PushRejectReason) *syncv1.PushResult {
	return syncv1.PushResult_builder{
		EntityId:     entityID,
		Status:       syncv1.PushStatus_PUSH_STATUS_REJECTED,
		RejectReason: reason,
		ErrorMessage: "rejected",
	}.Build()
}

func TestWorkspaceSyncer_HandleRejected_QuotaExceeded_EmitsEvent(t *testing.T) {
	engine := newTestEngine(t)
	events := captureEvents(engine)
	ws, cancel := newSyncer(engine, uuid.New().String(), StateConnected)
	defer cancel()

	entityID := uuid.New().String()
	entries := []*sqlite.SyncEntry{{EntityID: entityID, EntityType: "collection"}}

	ws.handleRejected(rejectedResult(entityID, syncv1.PushRejectReason_PUSH_REJECT_REASON_QUOTA_EXCEEDED), entries)

	data, count := findEvent(*events, "sync:quota_exceeded")
	require.Equal(t, 1, count)
	assert.Equal(t, ws.localWorkspaceID, data["workspaceId"])
	assert.Equal(t, "cloud_collections", data["kind"])
	assert.Equal(t, "collection", data["entityType"])
	assert.Equal(t, entityID, data["entityId"])
}

func TestWorkspaceSyncer_HandleRejected_IDConflict_EmitsRejectedEvent(t *testing.T) {
	engine := newTestEngine(t)
	events := captureEvents(engine)
	ws, cancel := newSyncer(engine, uuid.New().String(), StateConnected)
	defer cancel()

	entityID := uuid.New().String()
	entries := []*sqlite.SyncEntry{{EntityID: entityID, EntityType: "request"}}

	ws.handleRejected(rejectedResult(entityID, syncv1.PushRejectReason_PUSH_REJECT_REASON_ID_CONFLICT), entries)

	data, count := findEvent(*events, "sync:rejected")
	require.Equal(t, 1, count)
	assert.Equal(t, "id_conflict", data["reason"])
	assert.Equal(t, "request", data["entityType"])
	assert.Equal(t, entityID, data["entityId"])
}

func TestWorkspaceSyncer_HandleRejected_SilentWhileParked(t *testing.T) {
	engine := newTestEngine(t)
	events := captureEvents(engine)
	ws, cancel := newSyncer(engine, uuid.New().String(), StateConnected)
	defer cancel()

	entries := []*sqlite.SyncEntry{{EntityID: "a", EntityType: "collection"}, {EntityID: "b", EntityType: "collection"}}
	ws.handleRejected(rejectedResult("a", syncv1.PushRejectReason_PUSH_REJECT_REASON_QUOTA_EXCEEDED), entries)
	ws.handleRejected(rejectedResult("b", syncv1.PushRejectReason_PUSH_REJECT_REASON_QUOTA_EXCEEDED), entries)

	_, count := findEvent(*events, "sync:quota_exceeded")
	assert.Equal(t, 1, count, "a rejection inside an open episode must not emit")
}

func TestWorkspaceSyncer_HandleRejected_ParentNotFound_NoEvent(t *testing.T) {
	engine := newTestEngine(t)
	events := captureEvents(engine)
	ws, cancel := newSyncer(engine, uuid.New().String(), StateConnected)
	defer cancel()

	entries := []*sqlite.SyncEntry{{EntityID: "a", EntityType: "request"}}
	ws.handleRejected(rejectedResult("a", syncv1.PushRejectReason_PUSH_REJECT_REASON_PARENT_NOT_FOUND), entries)

	assert.Empty(t, *events)
}

// quotaErr mimics the server: mapError prefixes the handler name to domain.ErrQuotaExceeded.
func quotaErr() error {
	return status.Error(codes.ResourceExhausted, "SyncHandler.Push: quota exceeded: org exceeds member limit; sync paused")
}

func TestWorkspaceSyncer_RetryAfterSyncErr_QuotaMarker_PlanLimit(t *testing.T) {
	engine := newTestEngine(t)
	events := captureEvents(engine)
	ws, cancel := newSyncer(engine, uuid.New().String(), StateConnected)
	defer cancel()

	backoff := ws.retryAfterSyncErr(fmt.Errorf("push rpc: %w", quotaErr()), initialBackoff)

	assert.Equal(t, maxBackoff, backoff)
	assert.Equal(t, StatePlanLimit, ws.getState())
	data, count := findEvent(*events, "sync:quota_exceeded")
	require.Equal(t, 1, count)
	assert.Equal(t, "members", data["kind"])
	assert.Equal(t, ws.localWorkspaceID, data["workspaceId"])
}

func TestWorkspaceSyncer_RetryAfterSyncErr_MessageTooLarge_Offline(t *testing.T) {
	engine := newTestEngine(t)
	events := captureEvents(engine)
	ws, cancel := newSyncer(engine, uuid.New().String(), StateConnected)
	defer cancel()

	tooLarge := status.Error(codes.ResourceExhausted, "grpc: received message larger than max (5242880 vs. 4194304)")
	backoff := ws.retryAfterSyncErr(fmt.Errorf("push rpc: %w", tooLarge), 20*time.Second)

	assert.Equal(t, 20*time.Second, backoff)
	assert.Equal(t, StateOffline, ws.getState())
	_, count := findEvent(*events, "sync:quota_exceeded")
	assert.Zero(t, count)
}

func TestWorkspaceSyncer_RetryAfterSyncErr_OtherError_Offline(t *testing.T) {
	engine := newTestEngine(t)
	events := captureEvents(engine)
	ws, cancel := newSyncer(engine, uuid.New().String(), StateConnected)
	defer cancel()

	backoff := ws.retryAfterSyncErr(errors.New("connection refused"), 20*time.Second)

	assert.Equal(t, 20*time.Second, backoff)
	assert.Equal(t, StateOffline, ws.getState())
	_, count := findEvent(*events, "sync:quota_exceeded")
	assert.Zero(t, count)
}

func TestWorkspaceSyncer_PlanLimit_EmitsOncePerEpisode(t *testing.T) {
	engine := newTestEngine(t)
	events := captureEvents(engine)
	ws, cancel := newSyncer(engine, uuid.New().String(), StateConnected)
	defer cancel()

	ws.retryAfterSyncErr(quotaErr(), initialBackoff)
	ws.retryAfterSyncErr(quotaErr(), maxBackoff)

	_, count := findEvent(*events, "sync:quota_exceeded")
	assert.Equal(t, 1, count)

	engine.clearPlanLimit()
	ws.retryAfterSyncErr(quotaErr(), initialBackoff)

	_, count = findEvent(*events, "sync:quota_exceeded")
	assert.Equal(t, 2, count, "a new episode must notify again")
}

func TestWorkspaceSyncer_PlanLimit_OneEventForAllWorkspaces(t *testing.T) {
	engine := newTestEngine(t)
	events := captureEvents(engine)
	wsA, cancelA := newSyncer(engine, uuid.New().String(), StateConnected)
	defer cancelA()
	wsB, cancelB := newSyncer(engine, uuid.New().String(), StateConnected)
	defer cancelB()

	wsA.retryAfterSyncErr(quotaErr(), initialBackoff)
	wsB.retryAfterSyncErr(quotaErr(), initialBackoff)

	_, count := findEvent(*events, "sync:quota_exceeded")
	assert.Equal(t, 1, count)
	assert.Equal(t, StatePlanLimit, wsB.getState())
}

func TestWorkspaceSyncer_QuotaEvents_MembersDoesNotMuteCollections(t *testing.T) {
	engine := newTestEngine(t)
	events := captureEvents(engine)
	ws, cancel := newSyncer(engine, uuid.New().String(), StateConnected)
	defer cancel()

	entries := []*sqlite.SyncEntry{{EntityID: "a", EntityType: "collection"}}
	ws.handleRejected(rejectedResult("a", syncv1.PushRejectReason_PUSH_REJECT_REASON_QUOTA_EXCEEDED), entries)
	ws.retryAfterSyncErr(quotaErr(), initialBackoff)

	kinds := []string{}
	for _, e := range *events {
		if e.name == "sync:quota_exceeded" {
			kinds = append(kinds, e.data["kind"].(string))
		}
	}
	assert.Equal(t, []string{"cloud_collections", "members"}, kinds,
		"a members freeze must not swallow the collection-limit event")
}

type stubSubscribeStream struct {
	grpc.ClientStream
	ctx context.Context
}

func (s *stubSubscribeStream) Recv() (*syncv1.SubscribeResponse, error) {
	<-s.ctx.Done()
	return nil, s.ctx.Err()
}

type stubSyncClient struct {
	syncv1.SyncServiceClient
	mu        gosync.Mutex
	pushErr   error
	pushed    [][]*syncv1.SyncEntity
	pushResp  func(call int, req *syncv1.PushRequest) *syncv1.PushResponse
	pushErrFn func(req *syncv1.PushRequest) error
}

func (s *stubSyncClient) Push(_ context.Context, req *syncv1.PushRequest, _ ...grpc.CallOption) (*syncv1.PushResponse, error) {
	s.mu.Lock()
	s.pushed = append(s.pushed, req.GetEntities())
	calls := len(s.pushed)
	s.mu.Unlock()

	if s.pushErr != nil {
		return nil, s.pushErr
	}
	if s.pushErrFn != nil {
		if err := s.pushErrFn(req); err != nil {
			return nil, err
		}
	}
	if s.pushResp != nil {
		return s.pushResp(calls, req), nil
	}
	return syncv1.PushResponse_builder{}.Build(), nil
}

func (s *stubSyncClient) Pull(_ context.Context, _ *syncv1.PullRequest, _ ...grpc.CallOption) (*syncv1.PullResponse, error) {
	return syncv1.PullResponse_builder{}.Build(), nil
}

// pushBatches snapshots the recorded pushes for tests that read them while a syncer goroutine runs.
func (s *stubSyncClient) pushBatches() [][]*syncv1.SyncEntity {
	s.mu.Lock()
	defer s.mu.Unlock()
	return slices.Clone(s.pushed)
}

func (s *stubSyncClient) Subscribe(ctx context.Context, _ *syncv1.SubscribeRequest, _ ...grpc.CallOption) (grpc.ServerStreamingClient[syncv1.SubscribeResponse], error) {
	return &stubSubscribeStream{ctx: ctx}, nil
}

func TestWorkspaceSyncer_SubscribeLoop_PushQuotaExceeded_EntersPlanLimit(t *testing.T) {
	engine := newTestEngine(t)
	events := captureEvents(engine)
	ctx := context.Background()

	_, err := engine.configRepo.GetOrCreate(ctx)
	require.NoError(t, err)
	engine.auth.storeTokens("access-1", "", "org-1")
	engine.SetGRPCClient(&GRPCClient{sync: &stubSyncClient{
		pushErr: quotaErr(),
	}})

	wsID := uuid.New().String()
	ws, cancelSyncer := newSyncer(engine, wsID, StateConnected)
	defer cancelSyncer()

	require.NoError(t, engine.syncQueue.Enqueue(ctx, sqlite.SyncEntry{
		WorkspaceID: wsID,
		EntityType:  "collection",
		EntityID:    uuid.NewString(),
		Action:      "delete",
		OperationID: uuid.NewString(),
		Status:      "pending",
		CreatedAt:   time.Now().Truncate(time.Second),
	}))

	loopCtx, cancelLoop := context.WithCancel(ctx)
	done := make(chan struct{})
	go func() {
		defer close(done)
		ws.subscribeLoop(loopCtx)
	}()
	ws.pushSignal <- struct{}{}

	require.Eventually(t, func() bool { return ws.getState() == StatePlanLimit }, 5*time.Second, 10*time.Millisecond)
	cancelLoop()
	<-done

	data, count := findEvent(*events, "sync:quota_exceeded")
	require.Equal(t, 1, count)
	assert.Equal(t, "members", data["kind"])
	assert.Equal(t, wsID, data["workspaceId"])
}

func TestWorkspaceSyncer_PushAll_QuotaRejected_KeptForRetry(t *testing.T) {
	engine := newTestEngine(t)
	ctx := context.Background()

	_, err := engine.configRepo.GetOrCreate(ctx)
	require.NoError(t, err)
	engine.auth.storeTokens("access-1", "", "org-1")

	stub := &stubSyncClient{pushResp: func(call int, req *syncv1.PushRequest) *syncv1.PushResponse {
		pushStatus := syncv1.PushStatus_PUSH_STATUS_ACCEPTED
		reason := syncv1.PushRejectReason_PUSH_REJECT_REASON_UNSPECIFIED
		if call == 1 {
			pushStatus = syncv1.PushStatus_PUSH_STATUS_REJECTED
			reason = syncv1.PushRejectReason_PUSH_REJECT_REASON_QUOTA_EXCEEDED
		}
		return syncv1.PushResponse_builder{Results: []*syncv1.PushResult{
			syncv1.PushResult_builder{
				EntityId:     req.GetEntities()[0].GetEntityId(),
				Status:       pushStatus,
				RejectReason: reason,
			}.Build(),
		}}.Build()
	}}
	engine.SetGRPCClient(&GRPCClient{sync: stub})

	wsID := testWorkspaceID.String()
	ws, cancel := newSyncer(engine, wsID, StateConnected)
	defer cancel()

	require.NoError(t, engine.syncQueue.Enqueue(ctx, sqlite.SyncEntry{
		WorkspaceID: wsID,
		EntityType:  "collection",
		EntityID:    uuid.NewString(),
		Action:      "delete",
		OperationID: uuid.NewString(),
		Status:      "pending",
		CreatedAt:   time.Now().Truncate(time.Second),
	}))

	require.NoError(t, ws.pushAll(ctx))
	require.Len(t, stub.pushed, 1)

	var queueStatus, retryAt string
	require.NoError(t, engine.db.QueryRowContext(ctx,
		`SELECT status, next_retry_at FROM sync_queue WHERE workspace_id = ?`, wsID).Scan(&queueStatus, &retryAt))
	assert.Equal(t, "failed", queueStatus)
	parsed, err := time.Parse(time.RFC3339, retryAt)
	require.NoError(t, err)
	assert.WithinDuration(t, time.Now().Add(quotaRetryDelay), parsed, time.Minute)

	pending, err := engine.syncQueue.ListPending(ctx, wsID, 10)
	require.NoError(t, err)
	assert.Empty(t, pending)

	requeued, err := engine.syncQueue.RequeueDue(ctx, wsID, time.Now().Add(quotaRetryDelay+time.Minute))
	require.NoError(t, err)
	assert.Equal(t, int64(1), requeued)

	require.NoError(t, ws.pushAll(ctx))
	assert.Len(t, stub.pushed, 2)

	remaining, err := engine.syncQueue.ListPending(ctx, wsID, 10)
	require.NoError(t, err)
	assert.Empty(t, remaining)

	var rows int
	require.NoError(t, engine.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM sync_queue`).Scan(&rows))
	assert.Zero(t, rows, "an accepted entry must leave the outbox")
}

func TestWorkspaceSyncer_PushAll_QuotaEpisode_OneEventUntilParkedClears(t *testing.T) {
	engine := newTestEngine(t)
	events := captureEvents(engine)
	ctx := context.Background()

	_, err := engine.configRepo.GetOrCreate(ctx)
	require.NoError(t, err)
	engine.auth.storeTokens("access-1", "", "org-1")

	accept := false
	engine.SetGRPCClient(&GRPCClient{sync: &stubSyncClient{pushResp: func(_ int, req *syncv1.PushRequest) *syncv1.PushResponse {
		pushStatus := syncv1.PushStatus_PUSH_STATUS_REJECTED
		reason := syncv1.PushRejectReason_PUSH_REJECT_REASON_QUOTA_EXCEEDED
		if accept {
			pushStatus = syncv1.PushStatus_PUSH_STATUS_ACCEPTED
			reason = syncv1.PushRejectReason_PUSH_REJECT_REASON_UNSPECIFIED
		}
		results := make([]*syncv1.PushResult, 0, len(req.GetEntities()))
		for _, e := range req.GetEntities() {
			results = append(results, syncv1.PushResult_builder{
				EntityId:     e.GetEntityId(),
				Status:       pushStatus,
				RejectReason: reason,
			}.Build())
		}
		return syncv1.PushResponse_builder{Results: results}.Build()
	}}})

	wsID := testWorkspaceID.String()
	ws, cancel := newSyncer(engine, wsID, StateConnected)
	defer cancel()

	enqueue := func() {
		require.NoError(t, engine.syncQueue.Enqueue(ctx, sqlite.SyncEntry{
			WorkspaceID: wsID,
			EntityType:  "collection",
			EntityID:    uuid.NewString(),
			Action:      "delete",
			OperationID: uuid.NewString(),
			Status:      "pending",
			CreatedAt:   time.Now().Truncate(time.Second),
		}))
	}

	enqueue()
	require.NoError(t, ws.pushAll(ctx))

	_, quota := findEvent(*events, "sync:quota_exceeded")
	require.Equal(t, 1, quota)
	first, changes := findEvent(*events, "sync:parked_changed")
	require.Equal(t, 1, changes)
	assert.Equal(t, wsID, first["workspaceId"])
	assert.Equal(t, 1, first["parked"])

	enqueue()
	require.NoError(t, ws.pushAll(ctx))

	_, quota = findEvent(*events, "sync:quota_exceeded")
	assert.Equal(t, 1, quota, "a rejection while entries are parked must stay silent")
	assert.Equal(t, 2, lastEventData(*events, "sync:parked_changed")["parked"])

	accept = true
	_, err = engine.syncQueue.RequeueDue(ctx, wsID, time.Now().Add(quotaRetryDelay+time.Minute))
	require.NoError(t, err)
	require.NoError(t, ws.pushAll(ctx))

	assert.Equal(t, 0, lastEventData(*events, "sync:parked_changed")["parked"])

	accept = false
	enqueue()
	require.NoError(t, ws.pushAll(ctx))

	_, quota = findEvent(*events, "sync:quota_exceeded")
	assert.Equal(t, 2, quota, "the episode ended with the parked entries, so the next rejection notifies again")
}

func TestWorkspaceSyncer_PlanLimit_ClearedByReconnectWithoutPush(t *testing.T) {
	engine := newTestEngine(t)
	events := captureEvents(engine)
	ctx := context.Background()

	_, err := engine.configRepo.GetOrCreate(ctx)
	require.NoError(t, err)
	engine.auth.storeTokens("access-1", "", "org-1")
	stub := &stubSyncClient{}
	engine.SetGRPCClient(&GRPCClient{sync: stub})

	ws, cancelSyncer := newSyncer(engine, uuid.New().String(), StateOffline)
	defer cancelSyncer()

	ws.retryAfterSyncErr(quotaErr(), initialBackoff)
	require.Equal(t, StatePlanLimit, ws.getState())

	loopCtx, cancelLoop := context.WithCancel(ctx)
	done := make(chan struct{})
	go func() {
		defer close(done)
		ws.subscribeLoop(loopCtx)
	}()
	require.Eventually(t, func() bool { return ws.getState() == StateConnected }, 5*time.Second, 10*time.Millisecond)
	cancelLoop()
	<-done

	require.Empty(t, stub.pushed, "an empty outbox recovers without a push")

	ws.retryAfterSyncErr(quotaErr(), initialBackoff)

	_, count := findEvent(*events, "sync:quota_exceeded")
	assert.Equal(t, 2, count, "the episode after a recovery must notify again")
}

func TestWorkspaceSyncer_QuotaParked_RequeueTimerSignalsPush(t *testing.T) {
	restore := requeueDelay
	requeueDelay = 10 * time.Millisecond
	t.Cleanup(func() { requeueDelay = restore })

	engine := newTestEngine(t)
	ctx := context.Background()
	wsID := testWorkspaceID.String()
	ws, cancel := newSyncer(engine, wsID, StateConnected)
	defer cancel()

	require.NoError(t, engine.syncQueue.Enqueue(ctx, sqlite.SyncEntry{
		WorkspaceID: wsID,
		EntityType:  "collection",
		EntityID:    uuid.NewString(),
		Action:      "delete",
		OperationID: uuid.NewString(),
		Status:      "pending",
		CreatedAt:   time.Now().Truncate(time.Second),
	}))

	entries, err := engine.syncQueue.CoalescedPending(ctx, wsID, 10)
	require.NoError(t, err)
	require.Len(t, entries, 1)

	ws.parkEntries(ctx, []int64{entries[0].ID}, []int64{entries[0].ID})

	select {
	case <-ws.pushSignal:
	case <-time.After(2 * time.Second):
		t.Fatal("parked entries never woke the syncer")
	}
}

func TestWorkspaceSyncer_RequeueTimer_SingleOutstandingAndStopped(t *testing.T) {
	engine := newTestEngine(t)
	wsID := uuid.New().String()
	ws, cancel := newSyncer(engine, wsID, StateConnected)
	defer cancel()

	ws.scheduleRequeue()
	first := ws.requeueTimer
	ws.scheduleRequeue()
	assert.Same(t, first, ws.requeueTimer, "a pending wake-up must not be replaced")

	engine.StopWorkspace(wsID)
	assert.Nil(t, ws.requeueTimer)
}

func TestSyncEngine_GetPendingCount_IncludesParkedEntries(t *testing.T) {
	engine := newTestEngine(t)
	ctx := context.Background()
	wsID := testWorkspaceID.String()

	for i := 0; i < 2; i++ {
		require.NoError(t, engine.syncQueue.Enqueue(ctx, sqlite.SyncEntry{
			WorkspaceID: wsID,
			EntityType:  "collection",
			EntityID:    uuid.NewString(),
			Action:      "create",
			OperationID: uuid.NewString(),
			Status:      "pending",
			CreatedAt:   time.Now(),
		}))
	}

	entries, err := engine.syncQueue.ListPending(ctx, wsID, 10)
	require.NoError(t, err)
	require.NoError(t, engine.syncQueue.MarkFailed(ctx, entries[0].ID, time.Now().Add(quotaRetryDelay)))

	count, err := engine.GetPendingCount(ctx, wsID)
	require.NoError(t, err)
	assert.Equal(t, 2, count)
}

func enqueuePending(t *testing.T, engine *SyncEngine, wsID string) (entityID string, queueID int64) {
	t.Helper()
	ctx := context.Background()
	entityID = uuid.NewString()
	require.NoError(t, engine.syncQueue.Enqueue(ctx, sqlite.SyncEntry{
		WorkspaceID: wsID,
		EntityType:  "collection",
		EntityID:    entityID,
		Action:      "delete",
		OperationID: uuid.NewString(),
		Status:      "pending",
		CreatedAt:   time.Now().Truncate(time.Second),
	}))

	pending, err := engine.syncQueue.ListPending(ctx, wsID, 100)
	require.NoError(t, err)
	require.NotEmpty(t, pending)
	return entityID, pending[len(pending)-1].ID
}

func parkEntry(t *testing.T, engine *SyncEngine, wsID string, dueIn time.Duration) string {
	t.Helper()
	entityID, queueID := enqueuePending(t, engine, wsID)
	require.NoError(t, engine.syncQueue.MarkFailed(context.Background(), queueID, time.Now().Add(dueIn)))
	return entityID
}

func waitPushSignal(t *testing.T, ws *workspaceSyncer) {
	t.Helper()
	select {
	case <-ws.pushSignal:
	case <-time.After(5 * time.Second):
		t.Fatal("the requeue timer never woke the syncer")
	}
}

func TestWorkspaceSyncer_Start_RetriesParkedEntries(t *testing.T) {
	restore := requeueDelay
	requeueDelay = 5 * time.Second
	t.Cleanup(func() { requeueDelay = restore })

	engine := newTestEngine(t)
	ctx := context.Background()
	_, err := engine.configRepo.GetOrCreate(ctx)
	require.NoError(t, err)
	engine.auth.storeTokens("access-1", "", "org-1")

	stub := &stubSyncClient{}
	engine.SetGRPCClient(&GRPCClient{sync: stub})

	wsID := testWorkspaceID.String()
	due := parkEntry(t, engine, wsID, -time.Minute)
	// next_retry_at has second precision, so the deadline has to clear the current second.
	later := parkEntry(t, engine, wsID, 2*time.Second)

	engine.StartWorkspace(wsID, "remote-"+wsID, 0)
	t.Cleanup(func() { engine.StopWorkspace(wsID) })

	require.Eventually(t, func() bool { return len(stub.pushBatches()) == 2 }, 10*time.Second, 20*time.Millisecond)

	batches := stub.pushBatches()
	require.Len(t, batches[0], 1)
	assert.Equal(t, due, batches[0][0].GetEntityId(), "a syncer that starts must push what is already due")
	require.Len(t, batches[1], 1)
	assert.Equal(t, later, batches[1][0].GetEntityId(), "the rest must follow when their window elapses")

	owed, err := engine.syncQueue.CountPendingOrFailed(ctx, wsID)
	require.NoError(t, err)
	assert.Zero(t, owed)
}

func TestWorkspaceSyncer_RequeueTimer_RearmsWhileEntriesStayParked(t *testing.T) {
	restore := requeueDelay
	requeueDelay = 500 * time.Millisecond
	t.Cleanup(func() { requeueDelay = restore })

	engine := newTestEngine(t)
	ctx := context.Background()
	_, err := engine.configRepo.GetOrCreate(ctx)
	require.NoError(t, err)
	engine.auth.storeTokens("access-1", "", "org-1")

	engine.SetGRPCClient(&GRPCClient{sync: &stubSyncClient{pushResp: func(_ int, req *syncv1.PushRequest) *syncv1.PushResponse {
		results := make([]*syncv1.PushResult, 0, len(req.GetEntities()))
		for _, e := range req.GetEntities() {
			results = append(results, rejectedResult(e.GetEntityId(), syncv1.PushRejectReason_PUSH_REJECT_REASON_QUOTA_EXCEEDED))
		}
		return syncv1.PushResponse_builder{Results: results}.Build()
	}}})

	wsID := testWorkspaceID.String()
	ws, cancel := newSyncer(engine, wsID, StateConnected)
	defer cancel()

	enqueuePending(t, engine, wsID)
	require.NoError(t, ws.pushAll(ctx))

	enqueuePending(t, engine, wsID)
	require.NoError(t, ws.pushAll(ctx))

	select {
	case <-ws.pushSignal:
		t.Fatal("the wake-up fired before its window elapsed")
	default:
	}

	waitPushSignal(t, ws)
	require.NoError(t, ws.pushAll(ctx))
	waitPushSignal(t, ws)

	parked, err := engine.syncQueue.CountParked(ctx, wsID)
	require.NoError(t, err)
	assert.Equal(t, 2, parked)
}

// seedRequestRow inserts the bare row upsertRequest's existence check looks for;
// the stub repo holds the entity itself.
func seedRequestRow(t *testing.T, db *sql.DB, requestID uuid.UUID) {
	t.Helper()
	collID := uuid.NewString()
	_, err := db.Exec(
		"INSERT INTO collections (id, workspace_id, name) VALUES (?, '00000000-0000-4000-a000-000000000001', 'Synced')",
		collID)
	require.NoError(t, err)
	_, err = db.Exec("INSERT INTO requests (id, collection_id, name) VALUES (?, ?, 'Ping')",
		requestID.String(), collID)
	require.NoError(t, err)
}

func TestUpsertRequest_OldPeerKeepsLocalDescription(t *testing.T) {
	engine := newTestEngine(t)
	ws, cancel := newSyncer(engine, uuid.New().String(), StateConnected)
	defer cancel()

	repo := engine.requests.(*stubRequestRepo)
	id := uuid.New()
	repo.data[id] = &entities.Request{ID: id, Name: "Ping", Description: "local docs"}
	seedRequestRow(t, engine.db, id)

	incoming := &entities.Request{ID: id, CollectionID: uuid.New(), Name: "Ping v2"}
	require.NoError(t, ws.upsertRequest(context.Background(), incoming, false))

	assert.Equal(t, "local docs", repo.data[id].Description,
		"a peer that never sent the field must not wipe the local docs")
	assert.Equal(t, "Ping v2", repo.data[id].Name)
}

func TestUpsertRequest_IncomingDescriptionWins(t *testing.T) {
	engine := newTestEngine(t)
	ws, cancel := newSyncer(engine, uuid.New().String(), StateConnected)
	defer cancel()

	repo := engine.requests.(*stubRequestRepo)
	id := uuid.New()
	repo.data[id] = &entities.Request{ID: id, Name: "Ping", Description: "local docs"}
	seedRequestRow(t, engine.db, id)

	incoming := &entities.Request{ID: id, CollectionID: uuid.New(), Name: "Ping", Description: "remote docs"}
	require.NoError(t, ws.upsertRequest(context.Background(), incoming, true))

	assert.Equal(t, "remote docs", repo.data[id].Description)
}

func TestUpsertRequest_CreatesWhenRowIsAbsent(t *testing.T) {
	engine := newTestEngine(t)
	ws, cancel := newSyncer(engine, uuid.New().String(), StateConnected)
	defer cancel()

	repo := engine.requests.(*stubRequestRepo)
	incoming := &entities.Request{ID: uuid.New(), CollectionID: uuid.New(), Name: "New"}
	require.NoError(t, ws.upsertRequest(context.Background(), incoming, true))

	assert.Equal(t, "New", repo.data[incoming.ID].Name)
}

// gatedSyncClient holds a run goroutine inside the transport: Pull blocks until its context
// is cancelled and then until the test opens the gate, so a syncer can outlive a missing barrier.
type gatedSyncClient struct {
	syncv1.SyncServiceClient
	entered   chan struct{}
	gate      chan struct{}
	ignoreCtx bool
}

func newGatedSyncClient() *gatedSyncClient {
	return &gatedSyncClient{entered: make(chan struct{}, 8), gate: make(chan struct{})}
}

func (s *gatedSyncClient) Push(_ context.Context, _ *syncv1.PushRequest, _ ...grpc.CallOption) (*syncv1.PushResponse, error) {
	return syncv1.PushResponse_builder{}.Build(), nil
}

func (s *gatedSyncClient) Pull(ctx context.Context, _ *syncv1.PullRequest, _ ...grpc.CallOption) (*syncv1.PullResponse, error) {
	s.entered <- struct{}{}
	if !s.ignoreCtx {
		<-ctx.Done()
	}
	<-s.gate
	return nil, errors.New("transport closed")
}

// gatedEngine wires an engine to a gated transport and reports the workspace of
// every syncer goroutine that reached its final state.
func gatedEngine(t *testing.T) (*SyncEngine, *gatedSyncClient, chan string) {
	t.Helper()
	engine := newTestEngine(t)
	_, err := engine.configRepo.GetOrCreate(context.Background())
	require.NoError(t, err)
	engine.auth.storeTokens("access-1", "", "org-1")

	stub := newGatedSyncClient()
	engine.SetGRPCClient(&GRPCClient{sync: stub})

	exits := make(chan string, 8)
	engine.SetEventEmitter(func(name string, data any) {
		m, ok := data.(map[string]any)
		if !ok || name != "sync:status" || m["state"] != string(StateDisconnected) {
			return
		}
		exits <- m["workspaceId"].(string)
	})
	return engine, stub, exits
}

func TestSyncEngine_StopAll_JoinsSyncerGoroutine(t *testing.T) {
	engine, stub, exits := gatedEngine(t)

	wsID := uuid.New().String()
	engine.StartWorkspace(wsID, "remote-"+wsID, 0)
	<-stub.entered

	go func() {
		time.Sleep(100 * time.Millisecond)
		close(stub.gate)
	}()

	start := time.Now()
	assert.True(t, engine.StopAll(), "a joined syncer is a held barrier")
	assert.GreaterOrEqual(t, time.Since(start), 100*time.Millisecond, "StopAll must wait for the syncer, not just cancel it")

	select {
	case got := <-exits:
		assert.Equal(t, wsID, got)
	default:
		t.Fatal("StopAll returned while the syncer goroutine was still running")
	}
}

// A replaced syncer is gone from the workspaces map but its goroutine lives on,
// which is why the barrier counts goroutines instead of map entries.
func TestSyncEngine_StopAll_JoinsReplacedSyncer(t *testing.T) {
	cases := []struct {
		name    string
		restart func(t *testing.T, e *SyncEngine, wsID string)
	}{
		{"stop then start", func(_ *testing.T, e *SyncEngine, wsID string) {
			e.StopWorkspace(wsID)
			e.StartWorkspace(wsID, "remote-"+wsID, 0)
		}},
		{"force pull", func(t *testing.T, e *SyncEngine, wsID string) {
			require.NoError(t, e.ForcePull(wsID))
		}},
		{"pause then resume", func(t *testing.T, e *SyncEngine, wsID string) {
			require.NoError(t, e.Pause(wsID))
			require.NoError(t, e.Resume(wsID))
		}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			engine, stub, exits := gatedEngine(t)

			wsID := uuid.New().String()
			engine.StartWorkspace(wsID, "remote-"+wsID, 0)
			<-stub.entered

			tc.restart(t, engine, wsID)
			<-stub.entered

			go func() {
				time.Sleep(100 * time.Millisecond)
				close(stub.gate)
			}()
			engine.StopAll()

			assert.Len(t, exits, 2, "both the replaced syncer and its successor must be joined")
		})
	}
}

func TestSyncEngine_StopAll_TimesOutOnWedgedSyncer(t *testing.T) {
	engine, stub, exits := gatedEngine(t)
	stub.ignoreCtx = true

	logs := &lockedBuffer{}
	previous := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(logs, &slog.HandlerOptions{Level: slog.LevelDebug})))
	t.Cleanup(func() { slog.SetDefault(previous) })

	wsID := uuid.New().String()
	engine.StartWorkspace(wsID, "remote-"+wsID, 0)
	<-stub.entered

	start := time.Now()
	held := engine.stopAll(80 * time.Millisecond)
	elapsed := time.Since(start)

	assert.False(t, held, "a wedged syncer must be reported, not only logged")

	assert.GreaterOrEqual(t, elapsed, 80*time.Millisecond)
	assert.Less(t, elapsed, 2*time.Second, "a wedged syncer must not hold StopAll past its ceiling")
	assert.Contains(t, logs.String(), "StopAll timed out waiting for syncers")

	close(stub.gate)
	select {
	case <-exits:
	case <-time.After(2 * time.Second):
		t.Fatal("the wedged syncer never finished after the gate opened")
	}
}

// A timed-out barrier must leave nothing parked on the count: a WaitGroup here
// would keep a waiter on a counter the next start reuses, which panics the process.
func TestSyncerCount_ReusableAfterTimedOutWait(t *testing.T) {
	var c syncerCount

	c.add()
	assert.False(t, c.wait(10*time.Millisecond), "a live goroutine must hold the barrier")
	assert.False(t, c.wait(10*time.Millisecond))

	c.done()
	assert.True(t, c.wait(time.Second), "the count must still join after a barrier gave up")

	var hammer gosync.WaitGroup
	for i := 0; i < 8; i++ {
		hammer.Add(1)
		go func() {
			defer hammer.Done()
			c.wait(time.Second)
		}()
	}
	for i := 0; i < 200; i++ {
		c.add()
		hammer.Add(1)
		go func() {
			defer hammer.Done()
			c.done()
		}()
	}
	hammer.Wait()

	assert.True(t, c.wait(time.Second))
}

// A barrier that gave up must leave the counter usable: the wedged syncer still
// owes it an exit, and the starts that follow are ordinary work.
func TestSyncEngine_StopAll_StartsAgainAfterTimeout(t *testing.T) {
	engine, stub, exits := gatedEngine(t)
	stub.ignoreCtx = true

	logs := &lockedBuffer{}
	previous := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(logs, &slog.HandlerOptions{Level: slog.LevelDebug})))
	t.Cleanup(func() { slog.SetDefault(previous) })

	wedged := uuid.New().String()
	engine.StartWorkspace(wedged, "remote-"+wedged, 0)
	<-stub.entered

	engine.stopAll(80 * time.Millisecond)
	require.Contains(t, logs.String(), "StopAll timed out waiting for syncers")

	next := uuid.New().String()
	engine.StartWorkspace(next, "remote-"+next, 0)
	<-stub.entered

	close(stub.gate)
	engine.stopAll(2 * time.Second)

	assert.Len(t, exits, 2, "the second barrier must join the freed syncer and its successor")
}

// A start that raced the barrier used to leave a syncer nobody cancelled — and,
// in the tighter interleaving, panicked the WaitGroup with an Add during Wait.
func TestSyncEngine_StopAll_RacesStartWorkspace(t *testing.T) {
	engine, stub, exits := gatedEngine(t)
	close(stub.gate)

	drained := make(chan struct{})
	t.Cleanup(func() { close(drained) })
	go func() {
		for {
			select {
			case <-stub.entered:
			case <-exits:
			case <-drained:
				return
			}
		}
	}()

	logs := &lockedBuffer{}
	previous := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(logs, &slog.HandlerOptions{Level: slog.LevelDebug})))
	t.Cleanup(func() { slog.SetDefault(previous) })

	var starts gosync.WaitGroup
	for i := 0; i < 10; i++ {
		wsID := uuid.New().String()
		starts.Add(1)
		go func() {
			defer starts.Done()
			engine.StartWorkspace(wsID, "remote-"+wsID, 0)
		}()
	}

	engine.stopAll(2 * time.Second)
	starts.Wait()
	// The second barrier collects whatever started after the first one returned.
	engine.stopAll(2 * time.Second)

	assert.NotContains(t, logs.String(), "StopAll timed out waiting for syncers")
}

func TestSyncEngine_StopAll_WithoutGoroutines(t *testing.T) {
	engine := newTestEngine(t)

	start := time.Now()
	engine.StopAll()

	wsID := uuid.New().String()
	_, cancel := context.WithCancel(context.Background())
	defer cancel()
	engine.InjectRawSyncer(wsID, "remote-"+wsID, cancel)

	engine.StopAll()
	engine.StopAll()
	assert.Less(t, time.Since(start), 2*time.Second, "an engine with no run goroutines must not wait")
}

func TestSyncEngine_GRPCClient_SwapDuringReads(t *testing.T) {
	engine := newTestEngine(t)

	stop := make(chan struct{})
	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			select {
			case <-stop:
				return
			default:
				_ = engine.GRPCClient()
			}
		}
	}()

	var last *GRPCClient
	for i := 0; i < 200; i++ {
		last = &GRPCClient{}
		engine.SetGRPCClient(last)
	}
	close(stop)
	<-done

	assert.Same(t, last, engine.GRPCClient())
}

// lockedBuffer collects log output written by both the test and a syncer goroutine.
type lockedBuffer struct {
	mu  gosync.Mutex
	buf bytes.Buffer
}

func (b *lockedBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *lockedBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

func requestDescriptionEntity(id uuid.UUID, description *string) *syncv1.SyncEntity {
	return syncv1.SyncEntity_builder{
		EntityType: syncv1.EntityType_ENTITY_TYPE_REQUEST,
		EntityId:   id.String(),
		Request: syncv1.RequestData_builder{
			CollectionId: uuid.NewString(),
			Name:         "Ping v2",
			Method:       "GET",
			Url:          "https://example.com",
			Description:  description,
		}.Build(),
	}.Build()
}

func TestApplyEntity_RequestDescriptionPresenceThreadsThrough(t *testing.T) {
	engine := newTestEngine(t)
	ws, cancel := newSyncer(engine, testWorkspaceID.String(), StateConnected)
	defer cancel()

	repo := engine.requests.(*stubRequestRepo)
	ctx := context.Background()

	absent := uuid.New()
	repo.data[absent] = &entities.Request{ID: absent, Name: "Ping", Description: "local docs"}
	seedRequestRow(t, engine.db, absent)
	require.NoError(t, ws.applyEntity(ctx, requestDescriptionEntity(absent, nil)))
	assert.Equal(t, "local docs", repo.data[absent].Description,
		"an entity without the field must not wipe the local docs")
	assert.Equal(t, "Ping v2", repo.data[absent].Name, "the rest of the entity still applies")

	cleared := uuid.New()
	repo.data[cleared] = &entities.Request{ID: cleared, Name: "Ping", Description: "local docs"}
	seedRequestRow(t, engine.db, cleared)
	require.NoError(t, ws.applyEntity(ctx, requestDescriptionEntity(cleared, proto.String(""))))
	assert.Empty(t, repo.data[cleared].Description, "an explicit empty description clears the local docs")
}

func bigRequest(t *testing.T, engine *SyncEngine, ws string, bodyBytes int) uuid.UUID {
	t.Helper()
	id := uuid.New()
	engine.requests.(*stubRequestRepo).data[id] = &entities.Request{
		ID:     id,
		Name:   "Big",
		Method: entities.MethodPOST,
		URL:    "https://example.com",
		Body:   strings.Repeat("x", bodyBytes),
	}
	require.NoError(t, engine.syncQueue.Enqueue(context.Background(), sqlite.SyncEntry{
		WorkspaceID: ws,
		EntityType:  "request",
		EntityID:    id.String(),
		Action:      "update",
		OperationID: uuid.NewString(),
		Status:      "pending",
		CreatedAt:   time.Now().Truncate(time.Second),
	}))
	return id
}

func batchBytes(entities []*syncv1.SyncEntity) int {
	total := 0
	for _, e := range entities {
		total += proto.Size(e)
	}
	return total
}

func TestWorkspaceSyncer_DrainOutbox_SplitsBatchesByByteBudget(t *testing.T) {
	engine := newTestEngine(t)
	ctx := context.Background()
	_, err := engine.configRepo.GetOrCreate(ctx)
	require.NoError(t, err)
	engine.auth.storeTokens("access-1", "", "org-1")

	stub := &stubSyncClient{}
	engine.SetGRPCClient(&GRPCClient{sync: stub})

	wsID := testWorkspaceID.String()
	ws, cancel := newSyncer(engine, wsID, StateConnected)
	defer cancel()

	for range 3 {
		bigRequest(t, engine, wsID, 1500*1024)
	}

	require.NoError(t, ws.pushAll(ctx))

	batches := stub.pushBatches()
	require.GreaterOrEqual(t, len(batches), 2, "1.5 MiB entities must not share one batch past the budget")

	pushed := 0
	for i, b := range batches {
		pushed += len(b)
		assert.LessOrEqual(t, batchBytes(b), pushBatchBytes, "batch %d is over the byte budget", i)
	}
	assert.Equal(t, 3, pushed, "every entity still leaves the outbox")

	remaining, err := engine.syncQueue.ListPending(ctx, wsID, 10)
	require.NoError(t, err)
	assert.Empty(t, remaining)
}

func TestWorkspaceSyncer_DrainOutbox_OversizedEntityParkedAndRestPushed(t *testing.T) {
	engine := newTestEngine(t)
	events := captureEvents(engine)
	ctx := context.Background()
	_, err := engine.configRepo.GetOrCreate(ctx)
	require.NoError(t, err)
	engine.auth.storeTokens("access-1", "", "org-1")

	stub := &stubSyncClient{pushErrFn: func(req *syncv1.PushRequest) error {
		if batchBytes(req.GetEntities()) > 4<<20 {
			return status.Error(codes.ResourceExhausted, "grpc: received message larger than max (5242880 vs. 4194304)")
		}
		return nil
	}}
	engine.SetGRPCClient(&GRPCClient{sync: stub})

	wsID := testWorkspaceID.String()
	ws, cancel := newSyncer(engine, wsID, StateConnected)
	defer cancel()

	oversized := bigRequest(t, engine, wsID, 5<<20)
	small := bigRequest(t, engine, wsID, 64)

	require.NoError(t, ws.pushAll(ctx), "one oversized entity must not stall the outbox")

	var sentSmall bool
	for _, b := range stub.pushBatches() {
		for _, e := range b {
			if e.GetEntityId() == small.String() {
				sentSmall = true
			}
		}
	}
	assert.True(t, sentSmall, "the entity behind the oversized one must still go out")

	var queueStatus string
	require.NoError(t, engine.db.QueryRowContext(ctx,
		`SELECT status FROM sync_queue WHERE entity_id = ?`, oversized.String()).Scan(&queueStatus))
	assert.Equal(t, "parked", queueStatus, "the oversized entry is parked, not retried in place")

	pending, err := engine.syncQueue.ListPending(ctx, wsID, 10)
	require.NoError(t, err)
	assert.Empty(t, pending)

	data, count := findEvent(*events, "sync:rejected")
	require.Equal(t, 1, count)
	assert.Equal(t, "too_large", data["reason"])
	assert.Equal(t, oversized.String(), data["entityId"])
	assert.Equal(t, "request", data["entityType"])
	assert.Equal(t, StateConnected, ws.getState(), "a parked entity is not an offline event")
}

func oversizedSyncer(t *testing.T) (*SyncEngine, *workspaceSyncer, *stubSyncClient, func()) {
	t.Helper()
	engine := newTestEngine(t)
	_, err := engine.configRepo.GetOrCreate(context.Background())
	require.NoError(t, err)
	engine.auth.storeTokens("access-1", "", "org-1")

	stub := &stubSyncClient{pushErrFn: func(req *syncv1.PushRequest) error {
		if batchBytes(req.GetEntities()) > 4<<20 {
			return status.Error(codes.ResourceExhausted, "grpc: received message larger than max (5242880 vs. 4194304)")
		}
		return nil
	}}
	engine.SetGRPCClient(&GRPCClient{sync: stub})

	ws, cancel := newSyncer(engine, testWorkspaceID.String(), StateConnected)
	return engine, ws, stub, cancel
}

func TestWorkspaceSyncer_ParkedOversized_EventCarriesTooLargeCount(t *testing.T) {
	engine, ws, _, cancel := oversizedSyncer(t)
	defer cancel()
	events := captureEvents(engine)

	bigRequest(t, engine, ws.localWorkspaceID, 5<<20)
	require.NoError(t, ws.pushAll(context.Background()))

	data, count := findEvent(*events, "sync:parked_changed")
	require.Equal(t, 1, count, "the badge must not wait for the five-second status poll")
	assert.Equal(t, 1, data["tooLarge"])
	assert.Equal(t, 0, data["parked"], "an oversized entity is not a plan limit")
}

func TestWorkspaceSyncer_ParkedOversized_SupersededByLaterWrite(t *testing.T) {
	engine, ws, stub, cancel := oversizedSyncer(t)
	defer cancel()
	ctx := context.Background()
	wsID := ws.localWorkspaceID

	oversized := bigRequest(t, engine, wsID, 5<<20)
	require.NoError(t, ws.pushAll(ctx))

	engine.requests.(*stubRequestRepo).data[oversized].Body = "trimmed"
	require.NoError(t, engine.syncQueue.Enqueue(ctx, sqlite.SyncEntry{
		WorkspaceID: wsID,
		EntityType:  "request",
		EntityID:    oversized.String(),
		Action:      "update",
		OperationID: uuid.NewString(),
		Status:      "pending",
		CreatedAt:   time.Now().Truncate(time.Second),
	}))

	require.NoError(t, ws.pushAll(ctx))

	batches := stub.pushBatches()
	accepted := batches[len(batches)-1]
	require.Len(t, accepted, 1)
	assert.Equal(t, oversized.String(), accepted[0].GetEntityId(), "a trimmed entity must go out on the next push")
	assert.LessOrEqual(t, batchBytes(accepted), 4<<20, "and this time the server takes it")

	var rows int
	require.NoError(t, engine.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM sync_queue WHERE entity_id = ?`, oversized.String()).Scan(&rows))
	assert.Zero(t, rows, "the parked row is superseded by the newer write, not kept forever")

	parked, err := engine.GetParkedCount(ctx, wsID)
	require.NoError(t, err)
	assert.Zero(t, parked)
}

func TestWorkspaceSyncer_Resync_ReoffersDocumentedRequests(t *testing.T) {
	engine := newTestEngine(t)
	ctx := context.Background()
	_, err := engine.configRepo.GetOrCreate(ctx)
	require.NoError(t, err)
	engine.auth.storeTokens("access-1", "", "org-1")
	stub := &stubSyncClient{}
	engine.SetGRPCClient(&GRPCClient{sync: stub})

	wsID := testWorkspaceID.String()
	ws, cancel := newSyncer(engine, wsID, StateConnected)
	defer cancel()

	documented := seedDocumentedRequest(t, engine)

	require.NoError(t, ws.resync(ctx))

	var pushed []string
	for _, b := range stub.pushBatches() {
		for _, e := range b {
			pushed = append(pushed, e.GetEntityId())
		}
	}
	assert.Contains(t, pushed, documented.String(),
		"a resync must push the docs it re-offered, not wait for an unrelated local write")

	pending, err := engine.syncQueue.ListPending(ctx, wsID, 10)
	require.NoError(t, err)
	assert.Empty(t, pending, "an accepted re-offer leaves the outbox")
}

func seedDocumentedRequest(t *testing.T, engine *SyncEngine) uuid.UUID {
	t.Helper()
	id := uuid.New()
	seedRequestRow(t, engine.db, id)
	_, err := engine.db.Exec(`UPDATE requests SET description = '# Docs' WHERE id = ?`, id.String())
	require.NoError(t, err)
	engine.requests.(*stubRequestRepo).data[id] = &entities.Request{
		ID:          id,
		Name:        "Ping",
		Method:      entities.MethodGET,
		URL:         "https://example.com",
		Description: "# Docs",
	}
	return id
}

type failingQueueRepo struct {
	sqlite.SyncQueueRepository
	deleteErr  error
	enqueueErr error
}

func (q *failingQueueRepo) DeleteByWorkspace(ctx context.Context, workspaceID string) (int, error) {
	if q.deleteErr != nil {
		return 0, q.deleteErr
	}
	return q.SyncQueueRepository.DeleteByWorkspace(ctx, workspaceID)
}

func (q *failingQueueRepo) EnqueueDocumentedRequests(ctx context.Context, workspaceID string) (int, error) {
	if q.enqueueErr != nil {
		return 0, q.enqueueErr
	}
	return q.SyncQueueRepository.EnqueueDocumentedRequests(ctx, workspaceID)
}

func TestSyncEngine_ForceResync_PropagatesQueueErrors(t *testing.T) {
	tests := []struct {
		name string
		fail func(*failingQueueRepo)
	}{
		{"delete fails", func(q *failingQueueRepo) { q.deleteErr = errors.New("boom") }},
		{"re-offer fails", func(q *failingQueueRepo) { q.enqueueErr = errors.New("boom") }},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			engine := newTestEngine(t)
			ctx := context.Background()
			wsID := testWorkspaceID.String()

			require.NoError(t, engine.syncQueue.Enqueue(ctx, sqlite.SyncEntry{
				WorkspaceID: wsID,
				EntityType:  "collection",
				EntityID:    uuid.NewString(),
				Action:      "delete",
				OperationID: uuid.NewString(),
				Status:      "pending",
				CreatedAt:   time.Now().Truncate(time.Second),
			}))

			failing := &failingQueueRepo{SyncQueueRepository: engine.syncQueue}
			tc.fail(failing)
			engine.syncQueue = failing

			_, cancel := newSyncer(engine, wsID, StateConnected)
			defer cancel()
			defer engine.StopAll()

			err := engine.ForceResync(ctx, wsID)
			require.Error(t, err, "a half-done resync must be retried, not reported as fine")

			var rows int
			require.NoError(t, engine.db.QueryRowContext(ctx,
				`SELECT COUNT(*) FROM sync_queue WHERE workspace_id = ?`, wsID).Scan(&rows))
			assert.Equal(t, 1, rows, "a failed resync leaves the outbox as it was")

			assert.NotEqual(t, StateDisconnected, engine.GetWorkspaceState(wsID),
				"the workspace must keep syncing, or the retry reports it as not syncing")

			failing.deleteErr = nil
			failing.enqueueErr = nil
			assert.NoError(t, engine.ForceResync(ctx, wsID), "the retry must go through once the repo recovers")
		})
	}
}

func TestWorkspaceSyncer_Resync_PropagatesReofferError(t *testing.T) {
	engine := newTestEngine(t)
	ctx := context.Background()
	_, err := engine.configRepo.GetOrCreate(ctx)
	require.NoError(t, err)
	engine.auth.storeTokens("access-1", "", "org-1")
	engine.SetGRPCClient(&GRPCClient{sync: &stubSyncClient{}})
	wsID := testWorkspaceID.String()

	engine.syncQueue = &failingQueueRepo{
		SyncQueueRepository: engine.syncQueue,
		enqueueErr:          errors.New("boom"),
	}

	ws, cancel := newSyncer(engine, wsID, StateConnected)
	defer cancel()

	require.Error(t, ws.resync(ctx))
}

func parkRequestRow(t *testing.T, ws *workspaceSyncer, requestID uuid.UUID) {
	t.Helper()
	ctx := context.Background()
	require.NoError(t, ws.engine.syncQueue.Enqueue(ctx, sqlite.SyncEntry{
		WorkspaceID: ws.localWorkspaceID,
		EntityType:  "request",
		EntityID:    requestID.String(),
		Action:      "update",
		OperationID: uuid.NewString(),
		Status:      "pending",
		CreatedAt:   time.Now().Truncate(time.Second),
	}))
	pending, err := ws.engine.syncQueue.ListPending(ctx, ws.localWorkspaceID, 10)
	require.NoError(t, err)
	require.NotEmpty(t, pending)
	require.NoError(t, ws.engine.syncQueue.MarkParked(ctx, pending[len(pending)-1].ID))
}

func queueRowsFor(t *testing.T, db *sql.DB, entityID string) int {
	t.Helper()
	var n int
	require.NoError(t, db.QueryRow(`SELECT COUNT(*) FROM sync_queue WHERE entity_id = ?`, entityID).Scan(&n))
	return n
}

func TestApplyEntity_InboundDelete_DropsParkedRowOfTheRequest(t *testing.T) {
	ws, db, _ := newTokenSyncer(t)
	ctx := context.Background()

	coll := hostCollection(t, db)
	req := newLocalRequest("Oversized", coll.ID)
	require.NoError(t, sqlite.NewRequestRepo(db).Create(ctx, req))
	parkRequestRow(t, ws, req.ID)

	require.NoError(t, ws.applyEntity(ctx, deletedRequestEntity(req.ID, coll.ID)))

	assert.Zero(t, queueRowsFor(t, db, req.ID.String()),
		"a request another device deleted must not stay unsynced forever")
}

func deletedRequestEntity(requestID, collectionID uuid.UUID) *syncv1.SyncEntity {
	return syncv1.SyncEntity_builder{
		EntityType: syncv1.EntityType_ENTITY_TYPE_REQUEST,
		EntityId:   requestID.String(),
		IsDeleted:  true,
		Request: syncv1.RequestData_builder{
			CollectionId: collectionID.String(),
			Name:         "Oversized",
			Method:       "GET",
			Url:          "https://example.com",
			AuthType:     "none",
			AuthData:     "{}",
		}.Build(),
	}.Build()
}

func TestApplyEntity_InboundDelete_SweepsInsideTheCallersTransaction(t *testing.T) {
	ws, db, _ := newTokenSyncer(t)
	ctx := context.Background()

	coll := hostCollection(t, db)
	req := newLocalRequest("Oversized", coll.ID)
	require.NoError(t, sqlite.NewRequestRepo(db).Create(ctx, req))
	parkRequestRow(t, ws, req.ID)

	done := make(chan error, 1)
	go func() {
		done <- sqlite.WithTx(ctx, db, func(txCtx context.Context) error {
			return ws.applyEntity(txCtx, deletedRequestEntity(req.ID, coll.ID))
		})
	}()

	select {
	case err := <-done:
		require.NoError(t, err)
	case <-time.After(5 * time.Second):
		t.Fatal("the sweep waited for a connection the pull transaction holds")
	}

	assert.Zero(t, queueRowsFor(t, db, req.ID.String()))
}

type countingSweepRepo struct {
	sqlite.SyncQueueRepository
	sweeps int
}

func (q *countingSweepRepo) DeleteParkedForMissingEntities(ctx context.Context, workspaceID string) (int, error) {
	q.sweeps++
	return q.SyncQueueRepository.DeleteParkedForMissingEntities(ctx, workspaceID)
}

func TestPullAll_SweepsParkedRowsOncePerBatch(t *testing.T) {
	client := &fakeSyncClient{}
	ws, db, _ := newInboundSyncer(t, client)
	ctx := context.Background()
	coll := hostCollection(t, db)

	var ids []uuid.UUID
	var changes []*syncv1.SyncChange
	for range 3 {
		req := newLocalRequest("Oversized", coll.ID)
		require.NoError(t, sqlite.NewRequestRepo(db).Create(ctx, req))
		parkRequestRow(t, ws, req.ID)
		ids = append(ids, req.ID)
		changes = append(changes, syncv1.SyncChange_builder{
			EntityType: syncv1.EntityType_ENTITY_TYPE_REQUEST,
			EntityId:   req.ID.String(),
			Entity:     deletedRequestEntity(req.ID, coll.ID),
		}.Build())
	}

	counting := &countingSweepRepo{SyncQueueRepository: ws.engine.syncQueue}
	ws.engine.syncQueue = counting
	client.pull = func(*syncv1.PullRequest) (*syncv1.PullResponse, error) {
		return syncv1.PullResponse_builder{Changes: changes, NextSyncSeq: 9}.Build(), nil
	}

	require.NoError(t, ws.pullAll(ctx))

	assert.Equal(t, 1, counting.sweeps, "one sweep clears the parked rows of the whole batch")
	for _, id := range ids {
		assert.Zero(t, queueRowsFor(t, db, id.String()))
	}
}

func TestPushAll_DropsParkedRowsOfRequestsUnderADeletedCollection(t *testing.T) {
	ws, db, _ := newTokenSyncer(t)
	ctx := context.Background()

	coll := hostCollection(t, db)
	req := newLocalRequest("Oversized", coll.ID)
	require.NoError(t, sqlite.NewRequestRepo(db).Create(ctx, req))
	parkRequestRow(t, ws, req.ID)

	coll.IsDelete = true
	coll.Version++
	require.NoError(t, sqlite.NewCollectionRepo(db).Update(ctx, coll))

	require.NoError(t, ws.pushAll(ctx))

	assert.Zero(t, queueRowsFor(t, db, req.ID.String()),
		"a request the user deleted with its collection is not waiting for the cloud")
}

type cursorPullClient struct {
	syncv1.SyncServiceClient
	seq     atomic.Int64
	entered chan struct{}
}

func (c *cursorPullClient) Push(_ context.Context, _ *syncv1.PushRequest, _ ...grpc.CallOption) (*syncv1.PushResponse, error) {
	return syncv1.PushResponse_builder{}.Build(), nil
}

func (c *cursorPullClient) Pull(ctx context.Context, _ *syncv1.PullRequest, _ ...grpc.CallOption) (*syncv1.PullResponse, error) {
	select {
	case c.entered <- struct{}{}:
	default:
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return syncv1.PullResponse_builder{
		Changes:     []*syncv1.SyncChange{syncv1.SyncChange_builder{EntityId: uuid.NewString()}.Build()},
		NextSyncSeq: c.seq.Add(1),
		HasMore:     true,
	}.Build(), nil
}

func (c *cursorPullClient) Subscribe(ctx context.Context, _ *syncv1.SubscribeRequest, _ ...grpc.CallOption) (grpc.ServerStreamingClient[syncv1.SubscribeResponse], error) {
	return &stubSubscribeStream{ctx: ctx}, nil
}

func TestSyncEngine_ForceResync_ReadsCursorUnderLock(t *testing.T) {
	engine := newTestEngine(t)
	_, err := engine.configRepo.GetOrCreate(context.Background())
	require.NoError(t, err)
	engine.auth.storeTokens("access-1", "", "org-1")

	stub := &cursorPullClient{entered: make(chan struct{}, 1)}
	engine.SetGRPCClient(&GRPCClient{sync: stub})
	defer engine.StopAll()

	wsID := testWorkspaceID.String()
	engine.StartWorkspace(wsID, "remote-"+wsID, 0)
	<-stub.entered
	// Unsynchronised on purpose: a channel here would order the writes and hide the race.
	time.Sleep(100 * time.Millisecond)

	require.NoError(t, engine.ForceResync(context.Background(), wsID))
}
