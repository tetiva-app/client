package sync

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"runtime/debug"
	"slices"
	"strings"
	gosync "sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	syncv1 "github.com/tetiva-app/proto/go/gophercourier/sync/v1"

	"github.com/tetiva-app/client/internal/domain/entities"
	"github.com/tetiva-app/client/internal/domain/usecase/collection"
	"github.com/tetiva-app/client/internal/domain/usecase/environment"
	"github.com/tetiva-app/client/internal/domain/usecase/request"
	"github.com/tetiva-app/client/internal/infrastructure/repository/sqlite"
)

// SyncState represents the current synchronization state for a workspace.
type SyncState string

const (
	StateIdle         SyncState = "idle"
	StatePushing      SyncState = "pushing"
	StatePulling      SyncState = "pulling"
	StateSubscribing  SyncState = "subscribing"
	StateConnected    SyncState = "connected"
	StateOffline      SyncState = "offline"
	StateResyncing    SyncState = "resyncing"
	StateDisconnected SyncState = "disconnected"
	// StateAuthExpired means the refresh token was rejected: retrying is
	// pointless until the user logs in again. The syncer goroutine stops.
	StateAuthExpired SyncState = "auth_expired"
	// StatePlanLimit means the server refuses pushes until the org's plan changes.
	StatePlanLimit SyncState = "plan_limit"
)

const (
	pushBatchSize        = 100
	pullBatchSize        = 100
	heartbeatTimeout     = 60 * time.Second
	fallbackPollInterval = 15 * time.Minute
	maxBackoff           = 5 * time.Minute
	initialBackoff       = 5 * time.Second
	// rejectEventInterval throttles the id-conflict event: one refused push can
	// carry a whole batch of rejected items.
	rejectEventInterval = 5 * time.Minute
	// quotaRetryDelay parks a quota-rejected outbox entry: the plan has to change
	// before the server accepts it, and nothing local will trigger another push.
	quotaRetryDelay = 5 * time.Minute
	// planLimitMarker is the text of the server's domain.ErrQuotaExceeded: sync shares
	// ResourceExhausted with transport limits, so the status code alone means nothing.
	planLimitMarker = "quota exceeded"
)

// requeueDelay is how long the engine waits before waking a syncer whose entries
// are parked. A var so tests need not wait out the real window.
var requeueDelay = quotaRetryDelay

// requeueFloor keeps a stale deadline from spinning the wake-up: next_retry_at is
// stored with second precision, so a past-due value only ever misses by a fraction.
const requeueFloor = 250 * time.Millisecond

// EventEmitter is a function that emits events to the frontend via Wails.
type EventEmitter func(name string, data any)

// SyncEngine manages sync lifecycle for all workspaces.
type SyncEngine struct {
	auth       *SyncAuthManager
	grpcClient *GRPCClient
	syncQueue  sqlite.SyncQueueRepository
	configRepo sqlite.SyncConfigRepository
	db         *sql.DB
	// Inner repos (bypass decorators to avoid re-enqueue loop).
	collections       collection.Repository
	requests          request.Repository
	environments      environment.Repository
	variables         environment.VariableRepository
	eventEmitter      EventEmitter
	workspaces        gosync.Map // workspaceID string → *workspaceSyncer
	enabledWorkspaces gosync.Map // workspaceID string → bool
	// planLimit marks an open plan-limit episode; the freeze is org-wide, so the
	// stamp lives here and not on every workspace syncer.
	planLimit atomic.Bool
}

// NewSyncEngine creates a new SyncEngine instance.
func NewSyncEngine(
	auth *SyncAuthManager,
	syncQueue sqlite.SyncQueueRepository,
	configRepo sqlite.SyncConfigRepository,
	db *sql.DB,
	collections collection.Repository,
	requests request.Repository,
	environments environment.Repository,
	variables environment.VariableRepository,
) *SyncEngine {
	return &SyncEngine{
		auth:         auth,
		syncQueue:    syncQueue,
		configRepo:   configRepo,
		db:           db,
		collections:  collections,
		requests:     requests,
		environments: environments,
		variables:    variables,
		eventEmitter: func(string, any) {}, // no-op until set
	}
}

// SetEventEmitter sets the Wails event emitter function.
func (e *SyncEngine) SetEventEmitter(fn EventEmitter) {
	e.eventEmitter = fn
}

// SetGRPCClient sets the gRPC client (allows late binding during DI setup).
func (e *SyncEngine) SetGRPCClient(client *GRPCClient) {
	e.grpcClient = client
}

// startPlanLimitEpisode reports whether the org has just entered a plan-limit
// episode, so N linked workspaces raise one toast instead of N.
func (e *SyncEngine) startPlanLimitEpisode() bool {
	return e.planLimit.CompareAndSwap(false, true)
}

// clearPlanLimit ends the episode: the next refusal notifies again.
func (e *SyncEngine) clearPlanLimit() {
	e.planLimit.Store(false)
}

// IsEnabledForWorkspace returns true if sync is enabled for the given workspace.
// Used by decorator repos to decide whether to enqueue operations.
func (e *SyncEngine) IsEnabledForWorkspace(workspaceID string) bool {
	v, ok := e.enabledWorkspaces.Load(workspaceID)
	if !ok {
		return false
	}
	return v.(bool)
}

// StartWorkspace starts the sync goroutine for a workspace.
func (e *SyncEngine) StartWorkspace(localWorkspaceID, remoteWorkspaceID string, lastSyncSeq int64) {
	e.enabledWorkspaces.Store(localWorkspaceID, true)

	ws := &workspaceSyncer{
		engine:            e,
		localWorkspaceID:  localWorkspaceID,
		remoteWorkspaceID: remoteWorkspaceID,
		state:             StateIdle,
		pushSignal:        make(chan struct{}, 1),
		lastSyncSeq:       lastSyncSeq,
	}

	ctx, cancel := context.WithCancel(context.Background())
	ws.cancel = cancel
	e.workspaces.Store(localWorkspaceID, ws)

	go ws.run(ctx)
}

// StopWorkspace cancels the sync goroutine for a workspace.
func (e *SyncEngine) StopWorkspace(localWorkspaceID string) {
	e.enabledWorkspaces.Delete(localWorkspaceID)
	if v, ok := e.workspaces.LoadAndDelete(localWorkspaceID); ok {
		ws := v.(*workspaceSyncer)
		ws.stop()
	}
}

// StopAll cancels all running workspace syncers.
func (e *SyncEngine) StopAll() {
	e.workspaces.Range(func(key, value any) bool {
		ws := value.(*workspaceSyncer)
		ws.stop()
		e.workspaces.Delete(key)
		e.enabledWorkspaces.Delete(key)
		return true
	})
}

// NotifyWrite signals the workspace syncer that a write has occurred.
// Called by decorator repos (or Wails service) after any entity write.
func (e *SyncEngine) NotifyWrite(workspaceID string) {
	if v, ok := e.workspaces.Load(workspaceID); ok {
		ws := v.(*workspaceSyncer)
		select {
		case ws.pushSignal <- struct{}{}:
		default: // already signaled — no-op
		}
	}
}

// GetWorkspaceState returns the current sync state for a workspace.
func (e *SyncEngine) GetWorkspaceState(localWorkspaceID string) SyncState {
	if v, ok := e.workspaces.Load(localWorkspaceID); ok {
		ws := v.(*workspaceSyncer)
		return ws.getState()
	}
	return StateDisconnected
}

// GetPendingCount returns how many entries still owe the server a push, parked
// quota retries included: to the user they are unsynced either way.
func (e *SyncEngine) GetPendingCount(ctx context.Context, workspaceID string) (int, error) {
	return e.syncQueue.CountPendingOrFailed(ctx, workspaceID)
}

// GetParkedCount returns how many entries the server refused over the plan quota
// and that wait in the outbox for a retry.
func (e *SyncEngine) GetParkedCount(ctx context.Context, workspaceID string) (int, error) {
	return e.syncQueue.CountParked(ctx, workspaceID)
}

// ForcePush triggers an immediate push for the given workspace.
// Returns the number of pending entries before the push signal was sent.
func (e *SyncEngine) ForcePush(ctx context.Context, workspaceID string) (int, error) {
	count, err := e.GetPendingCount(ctx, workspaceID)
	if err != nil {
		return 0, fmt.Errorf("get pending count: %w", err)
	}
	e.NotifyWrite(workspaceID)
	return count, nil
}

// ForcePull restarts the workspace syncer preserving current seq
// (performs push + incremental pull + subscribe).
func (e *SyncEngine) ForcePull(workspaceID string) error {
	v, ok := e.workspaces.Load(workspaceID)
	if !ok {
		return fmt.Errorf("workspace %s is not syncing", workspaceID)
	}
	ws := v.(*workspaceSyncer)
	remoteID := ws.remoteWorkspaceID
	lastSeq := ws.lastSyncSeq

	e.StopWorkspace(workspaceID)
	e.StartWorkspace(workspaceID, remoteID, lastSeq)
	return nil
}

// ForceResync stops the workspace syncer, resets last_sync_seq to 0,
// clears the outbox queue, and restarts the full sync cycle.
func (e *SyncEngine) ForceResync(ctx context.Context, workspaceID string) error {
	v, ok := e.workspaces.Load(workspaceID)
	if !ok {
		return fmt.Errorf("workspace %s is not syncing", workspaceID)
	}
	ws := v.(*workspaceSyncer)
	remoteID := ws.remoteWorkspaceID

	e.StopWorkspace(workspaceID)

	if _, err := e.syncQueue.DeleteByWorkspace(ctx, workspaceID); err != nil {
		return fmt.Errorf("clear queue: %w", err)
	}

	e.StartWorkspace(workspaceID, remoteID, 0)
	return nil
}

// InjectRawSyncer inserts a minimal workspaceSyncer into the engine without
// starting a goroutine. Intended for use in tests only.
func (e *SyncEngine) InjectRawSyncer(localWorkspaceID, remoteWorkspaceID string, cancel context.CancelFunc) {
	ws := &workspaceSyncer{
		engine:            e,
		localWorkspaceID:  localWorkspaceID,
		remoteWorkspaceID: remoteWorkspaceID,
		state:             StateConnected,
		pushSignal:        make(chan struct{}, 1),
		cancel:            cancel,
	}
	e.workspaces.Store(localWorkspaceID, ws)
	e.enabledWorkspaces.Store(localWorkspaceID, true)
}

// Pause stops the sync goroutine for a workspace; writes keep accumulating in the local queue.
// Idempotent; returns an error when the workspace is not tracked by the engine.
func (e *SyncEngine) Pause(workspaceID string) error {
	v, ok := e.workspaces.Load(workspaceID)
	if !ok {
		return fmt.Errorf("workspace %s is not syncing", workspaceID)
	}
	ws := v.(*workspaceSyncer)

	ws.mu.Lock()
	defer ws.mu.Unlock()

	if ws.paused {
		return nil
	}

	ws.paused = true
	ws.stopRequeueTimerLocked()
	ws.cancel()
	return nil
}

// Resume restarts a previously paused workspace through the full push → pull → subscribe cycle.
// Idempotent; returns an error when the workspace is not tracked by the engine.
func (e *SyncEngine) Resume(workspaceID string) error {
	v, ok := e.workspaces.Load(workspaceID)
	if !ok {
		return fmt.Errorf("workspace %s is not syncing", workspaceID)
	}
	ws := v.(*workspaceSyncer)

	ws.mu.Lock()
	if !ws.paused {
		ws.mu.Unlock()
		return nil
	}
	remoteID := ws.remoteWorkspaceID
	lastSeq := ws.lastSyncSeq
	ws.mu.Unlock()

	// Drop the stale paused syncer and restart fresh — mirrors ForcePull.
	e.workspaces.Delete(workspaceID)
	e.enabledWorkspaces.Store(workspaceID, true)
	e.StartWorkspace(workspaceID, remoteID, lastSeq)
	return nil
}

// DisconnectStream closes the current Subscribe stream once; the backoff reconnect loop takes over.
// Returns an error when the workspace is not tracked or has no active stream.
func (e *SyncEngine) DisconnectStream(workspaceID string) error {
	v, ok := e.workspaces.Load(workspaceID)
	if !ok {
		return fmt.Errorf("workspace %s is not syncing", workspaceID)
	}
	ws := v.(*workspaceSyncer)

	ws.mu.Lock()
	defer ws.mu.Unlock()

	if ws.streamCancel == nil {
		return fmt.Errorf("workspace %s has no active stream to disconnect", workspaceID)
	}
	ws.streamCancel()
	ws.streamCancel = nil
	return nil
}

type workspaceSyncer struct {
	engine            *SyncEngine
	localWorkspaceID  string
	remoteWorkspaceID string
	state             SyncState
	mu                gosync.RWMutex
	cancel            context.CancelFunc
	pushSignal        chan struct{} // buffered(1) — non-blocking send on write
	lastSyncSeq       int64
	// paused: the goroutine ctx is cancelled but the entry stays in the map
	// so Resume can restart it without losing remoteWorkspaceID / lastSyncSeq.
	paused bool
	// streamCancel cancels only the current Subscribe stream, leaving the reconnect loop to take over.
	streamCancel context.CancelFunc
	// Throttle stamp for the id-conflict rejection event, guarded by mu.
	lastRejectEvent time.Time
	// parked is the last published count of quota-rejected entries; quotaEpisode
	// is open while the user still owes the cloud those changes. Guarded by mu.
	parked       int
	quotaEpisode bool
	// requeueTimer wakes the syncer once when parked entries fall due: no local
	// write is coming to trigger the push that would requeue them.
	requeueTimer *time.Timer
}

// scheduleRequeue arms a single wake-up for entries parked just now.
func (ws *workspaceSyncer) scheduleRequeue() {
	ws.scheduleRequeueIn(requeueDelay)
}

// scheduleRequeueIn arms the wake-up d from now, clamped to [requeueFloor, requeueDelay];
// a pending wake-up wins, so a workspace never holds more than one timer.
func (ws *workspaceSyncer) scheduleRequeueIn(d time.Duration) {
	d = min(max(d, requeueFloor), requeueDelay)

	ws.mu.Lock()
	defer ws.mu.Unlock()

	if ws.requeueTimer != nil {
		return
	}
	ws.requeueTimer = time.AfterFunc(d, func() {
		ws.mu.Lock()
		ws.requeueTimer = nil
		ws.mu.Unlock()
		ws.engine.NotifyWrite(ws.localWorkspaceID)
	})
}

// rearmRequeue keeps a wake-up armed while anything stays parked. Timers do not
// survive a restart, and after one fires nothing else would requeue the rest.
func (ws *workspaceSyncer) rearmRequeue(ctx context.Context) {
	due, ok, err := ws.engine.syncQueue.EarliestParkedRetryAt(ctx, ws.localWorkspaceID)
	if err != nil {
		slog.Warn("sync: failed to read the earliest parked retry", "workspace", ws.localWorkspaceID, "err", err)
		return
	}
	if !ok {
		ws.stopRequeueTimer()
		return
	}
	ws.scheduleRequeueIn(time.Until(due))
}

func (ws *workspaceSyncer) stopRequeueTimer() {
	ws.mu.Lock()
	defer ws.mu.Unlock()
	ws.stopRequeueTimerLocked()
}

func (ws *workspaceSyncer) stopRequeueTimerLocked() {
	if ws.requeueTimer != nil {
		ws.requeueTimer.Stop()
		ws.requeueTimer = nil
	}
}

// stop ends the syncer goroutine and drops its pending wake-up.
func (ws *workspaceSyncer) stop() {
	ws.stopRequeueTimer()
	ws.cancel()
}

func (ws *workspaceSyncer) getState() SyncState {
	ws.mu.RLock()
	defer ws.mu.RUnlock()
	return ws.state
}

func (ws *workspaceSyncer) setState(s SyncState) {
	// Reaching Connected proves the server talks to us again, whether the recovery
	// went through a push or through subscribe with an empty outbox.
	if s == StateConnected {
		ws.engine.clearPlanLimit()
	}

	ws.mu.Lock()
	ws.state = s
	ws.mu.Unlock()
	ws.engine.eventEmitter("sync:status", map[string]any{
		"workspaceId": ws.localWorkspaceID,
		"state":       string(s),
	})
}

// run performs push → pull → subscribe for the workspace goroutine.
func (ws *workspaceSyncer) run(ctx context.Context) {
	defer func() {
		if r := recover(); r != nil {
			slog.Error("sync: goroutine panic recovered", "panic", r, "stack", string(debug.Stack()), "workspace", ws.localWorkspaceID)
			// goOffline retries with backoff and respects ctx, so a panic
			// becomes a reconnect attempt instead of a dead workspace.
			ws.goOffline(ctx)
		}
	}()

	ws.setState(StatePushing)
	if err := ws.pushAll(ctx); err != nil {
		slog.Error("sync push failed", "workspace", ws.localWorkspaceID, "err", err)
		if ws.handleAuthErr(err) {
			return
		}
		ws.retryLoop(ctx, ws.retryAfterSyncErr(err, initialBackoff))
		return
	}

	ws.setState(StatePulling)
	if err := ws.pullAll(ctx); err != nil {
		slog.Error("sync pull failed", "workspace", ws.localWorkspaceID, "err", err)
		if ws.handleAuthErr(err) {
			return
		}
		ws.goOffline(ctx)
		return
	}

	ws.setState(StateSubscribing)
	ws.subscribeLoop(ctx)
}

// pushAll requeues the parked entries that fell due, drains the outbox queue, and
// re-arms the wake-up for whatever is still parked.
func (ws *workspaceSyncer) pushAll(ctx context.Context) error {
	if _, err := ws.engine.syncQueue.RequeueDue(ctx, ws.localWorkspaceID, time.Now()); err != nil {
		slog.Warn("sync: failed to requeue deferred entries", "workspace", ws.localWorkspaceID, "err", err)
	}

	if err := ws.drainOutbox(ctx); err != nil {
		return err
	}

	ws.rearmRequeue(ctx)
	return nil
}

// drainOutbox pushes all pending entries to the server, batch by batch.
func (ws *workspaceSyncer) drainOutbox(ctx context.Context) error {
	for {
		entries, err := ws.engine.syncQueue.CoalescedPending(ctx, ws.localWorkspaceID, pushBatchSize)
		if err != nil {
			return fmt.Errorf("coalesce pending: %w", err)
		}
		if len(entries) == 0 {
			return nil
		}

		var protoEntities []*syncv1.SyncEntity
		var entryIDs []int64

		for _, entry := range entries {
			entity, err := ws.readEntity(ctx, entry.EntityType, entry.EntityID)
			if err != nil {
				slog.Warn("sync: skip entity read error", "type", entry.EntityType, "id", entry.EntityID, "err", err)
				if err := ws.engine.syncQueue.Delete(ctx, []int64{entry.ID}); err != nil {
					slog.Warn("sync: failed to delete queue entry after read error", "entry", entry.ID, "workspace", ws.localWorkspaceID, "err", err)
				}
				continue
			}
			if entity == nil {
				if entry.Action == "delete" {
					// Soft-deleted: build a minimal delete proto directly.
					protoEntity := buildDeleteProto(entry)
					if protoEntity != nil {
						protoEntities = append(protoEntities, protoEntity)
						entryIDs = append(entryIDs, entry.ID)
					} else {
						if err := ws.engine.syncQueue.Delete(ctx, []int64{entry.ID}); err != nil {
							slog.Warn("sync: failed to delete unresolvable delete queue entry", "entry", entry.ID, "workspace", ws.localWorkspaceID, "err", err)
						}
					}
				} else {
					if err := ws.engine.syncQueue.Delete(ctx, []int64{entry.ID}); err != nil {
						slog.Warn("sync: failed to delete stale queue entry for nil entity", "entry", entry.ID, "workspace", ws.localWorkspaceID, "err", err)
					}
				}
				continue
			}

			var protoEntity *syncv1.SyncEntity
			switch entry.EntityType {
			case "collection":
				if c, ok := entity.(*entities.Collection); ok {
					protoEntity = CollectionToProto(c, entry.OperationID)
				}
			case "request":
				if r, ok := entity.(*entities.Request); ok {
					protoEntity = RequestToProto(r, entry.OperationID)
				}
			case "environment":
				if e, ok := entity.(*entities.Environment); ok {
					protoEntity = EnvironmentToProto(e, entry.OperationID)
				}
			case "variable":
				if v, ok := entity.(*entities.Variable); ok {
					protoEntity = VariableToProto(v, entry.OperationID)
				}
			}

			if protoEntity != nil {
				protoEntities = append(protoEntities, protoEntity)
				entryIDs = append(entryIDs, entry.ID)
			}
		}

		if len(protoEntities) == 0 {
			return nil
		}

		// Parents must be pushed before children.
		sortByEntityType(protoEntities, entryIDs)

		token, err := ws.engine.auth.GetAccessToken(ctx, ws.engine.grpcClient)
		if err != nil {
			return fmt.Errorf("get access token: %w", err)
		}

		cfg, err := ws.engine.configRepo.Get(ctx)
		if err != nil || cfg == nil {
			return fmt.Errorf("get config: %w", err)
		}

		authCtx := ContextWithAuth(ctx, token)
		pushReq := syncv1.PushRequest_builder{
			WorkspaceId: ws.remoteWorkspaceID,
			ClientId:    cfg.ClientID,
			Entities:    protoEntities,
		}.Build()

		resp, err := ws.engine.grpcClient.Sync().Push(authCtx, pushReq)
		if err != nil {
			return fmt.Errorf("push rpc: %w", err)
		}
		ws.engine.clearPlanLimit()

		var quotaRejected []int64
		for _, result := range resp.GetResults() {
			switch result.GetStatus() {
			case syncv1.PushStatus_PUSH_STATUS_ACCEPTED, syncv1.PushStatus_PUSH_STATUS_DUPLICATE:
				ws.markSynced(ctx, result.GetEntityId(), entries)
			case syncv1.PushStatus_PUSH_STATUS_CONFLICT_RESOLVED:
				if result.GetWinner() != nil {
					if err := ws.applyEntity(ctx, result.GetWinner()); err != nil {
						slog.Warn("sync: apply conflict winner failed", "entityId", result.GetEntityId(), "err", err)
					}
					ws.engine.eventEmitter("sync:entity_updated", map[string]any{
						"workspaceId": ws.localWorkspaceID,
						"entityId":    result.GetEntityId(),
					})
				}
			case syncv1.PushStatus_PUSH_STATUS_REJECTED:
				ws.handleRejected(result, entries)
				if result.GetRejectReason() == syncv1.PushRejectReason_PUSH_REJECT_REASON_QUOTA_EXCEEDED {
					if id, ok := entryIDOf(result.GetEntityId(), entries); ok {
						quotaRejected = append(quotaRejected, id)
					}
				}
			}
		}

		entryIDs = ws.deferQuotaRejected(ctx, entryIDs, quotaRejected)

		if err := ws.engine.syncQueue.Delete(ctx, entryIDs); err != nil {
			return fmt.Errorf("delete queue entries: %w", err)
		}

		ws.refreshParked(ctx)
	}
}

// readEntity returns (nil, nil) when the entity does not exist (soft-deleted or never created).
// Explicit nil returns avoid the (*T)(nil)-wrapped-in-any != nil pitfall.
func (ws *workspaceSyncer) readEntity(ctx context.Context, entityType, entityID string) (any, error) {
	id, err := uuid.Parse(entityID)
	if err != nil {
		return nil, fmt.Errorf("readEntity: invalid entity id %q: %w", entityID, err)
	}
	switch entityType {
	case "collection":
		c, err := ws.engine.collections.GetByID(ctx, id)
		if c == nil || err != nil {
			return nil, err
		}
		return c, nil
	case "request":
		r, err := ws.engine.requests.GetByID(ctx, id)
		if r == nil || err != nil {
			return nil, err
		}
		return r, nil
	case "environment":
		e, err := ws.engine.environments.GetByID(ctx, id)
		if e == nil || err != nil {
			return nil, err
		}
		return e, nil
	case "variable":
		v, err := ws.engine.variables.GetByID(ctx, id)
		if v == nil || err != nil {
			return nil, err
		}
		return v, nil
	default:
		return nil, fmt.Errorf("unknown entity type: %s", entityType)
	}
}

// buildDeleteProto builds the minimal SyncEntity server-side deletion needs: type, ID, is_deleted.
func buildDeleteProto(entry *sqlite.SyncEntry) *syncv1.SyncEntity {
	var entityType syncv1.EntityType
	switch entry.EntityType {
	case "collection":
		entityType = syncv1.EntityType_ENTITY_TYPE_COLLECTION
	case "request":
		entityType = syncv1.EntityType_ENTITY_TYPE_REQUEST
	case "environment":
		entityType = syncv1.EntityType_ENTITY_TYPE_ENVIRONMENT
	case "variable":
		entityType = syncv1.EntityType_ENTITY_TYPE_VARIABLE
	default:
		return nil
	}
	return syncv1.SyncEntity_builder{
		EntityType:  entityType,
		EntityId:    entry.EntityID,
		IsDeleted:   true,
		OperationId: entry.OperationID,
	}.Build()
}

func (ws *workspaceSyncer) markSynced(ctx context.Context, entityID string, entries []*sqlite.SyncEntry) {
	entityType := entityTypeOf(entityID, entries)
	if entityType == "" {
		return
	}

	table := entityType + "s" // collections, requests, environments, variables
	query := fmt.Sprintf("UPDATE %s SET is_synced = 1 WHERE id = ?", table)
	if _, err := sqlite.DBTXFromContext(ctx, ws.engine.db).ExecContext(ctx, query, entityID); err != nil {
		slog.Warn("sync: failed to mark entity as synced", "type", entityType, "id", entityID, "err", err)
	}
}

func entityTypeOf(entityID string, entries []*sqlite.SyncEntry) string {
	for _, e := range entries {
		if e.EntityID == entityID {
			return e.EntityType
		}
	}
	return ""
}

func entryIDOf(entityID string, entries []*sqlite.SyncEntry) (int64, bool) {
	for _, e := range entries {
		if e.EntityID == entityID {
			return e.ID, true
		}
	}
	return 0, false
}

// handleRejected reports a per-item rejection to the UI. Except for quota, the queue
// entry is dropped with the batch: the entity stays local until a later write re-enqueues it.
func (ws *workspaceSyncer) handleRejected(result *syncv1.PushResult, entries []*sqlite.SyncEntry) {
	entityID := result.GetEntityId()
	entityType := entityTypeOf(entityID, entries)

	switch result.GetRejectReason() {
	case syncv1.PushRejectReason_PUSH_REJECT_REASON_QUOTA_EXCEEDED:
		slog.Warn("sync: push rejected by plan quota", "entityId", entityID, "error", result.GetErrorMessage())
		ws.emitQuotaOnce(map[string]any{
			"workspaceId": ws.localWorkspaceID,
			"kind":        "cloud_collections",
			"entityType":  entityType,
			"entityId":    entityID,
		})
	case syncv1.PushRejectReason_PUSH_REJECT_REASON_ID_CONFLICT:
		slog.Warn("sync: push rejected — entity id taken by another workspace", "entityId", entityID, "error", result.GetErrorMessage())
		ws.emitThrottled("sync:rejected", &ws.lastRejectEvent, map[string]any{
			"workspaceId": ws.localWorkspaceID,
			"entityType":  entityType,
			"entityId":    entityID,
			"reason":      "id_conflict",
		})
	default:
		slog.Warn("sync: push rejected", "entityId", entityID, "error", result.GetErrorMessage())
	}
}

// deferQuotaRejected parks quota-rejected entries in the outbox instead of dropping
// them: 'failed' hides them from the pending queries until their retry window elapses.
// Returns the entry IDs the caller may still delete.
func (ws *workspaceSyncer) deferQuotaRejected(ctx context.Context, entryIDs, rejected []int64) []int64 {
	if len(rejected) == 0 {
		return entryIDs
	}

	retryAt := time.Now().Add(quotaRetryDelay)
	kept := make(map[int64]bool, len(rejected))
	for _, id := range rejected {
		if err := ws.engine.syncQueue.MarkFailed(ctx, id, retryAt); err != nil {
			slog.Warn("sync: failed to defer quota-rejected queue entry", "entry", id, "workspace", ws.localWorkspaceID, "err", err)
			continue
		}
		kept[id] = true
	}

	if len(kept) > 0 {
		ws.scheduleRequeue()
	}

	return slices.DeleteFunc(entryIDs, func(id int64) bool { return kept[id] })
}

// emitQuotaOnce notifies on the first rejection of an episode; the rest stay silent
// until the parked entries reach the cloud and the episode ends.
func (ws *workspaceSyncer) emitQuotaOnce(data map[string]any) {
	ws.mu.Lock()
	open := ws.quotaEpisode
	ws.quotaEpisode = true
	ws.mu.Unlock()

	if open {
		return
	}
	ws.engine.eventEmitter("sync:quota_exceeded", data)
}

// refreshParked republishes the parked count when it moves: the indicator shows it,
// and a drop to zero closes the episode so the next rejection notifies again.
func (ws *workspaceSyncer) refreshParked(ctx context.Context) {
	count, err := ws.engine.syncQueue.CountParked(ctx, ws.localWorkspaceID)
	if err != nil {
		slog.Warn("sync: failed to count parked entries", "workspace", ws.localWorkspaceID, "err", err)
		return
	}

	ws.mu.Lock()
	changed := ws.parked != count
	ws.parked = count
	ws.quotaEpisode = count > 0
	ws.mu.Unlock()

	if changed {
		ws.engine.eventEmitter("sync:parked_changed", map[string]any{
			"workspaceId": ws.localWorkspaceID,
			"parked":      count,
		})
	}
}

// emitThrottled emits at most one event per rejectEventInterval per workspace.
func (ws *workspaceSyncer) emitThrottled(name string, last *time.Time, data map[string]any) {
	now := time.Now()

	ws.mu.Lock()
	if now.Sub(*last) < rejectEventInterval {
		ws.mu.Unlock()
		return
	}
	*last = now
	ws.mu.Unlock()

	ws.engine.eventEmitter(name, data)
}

// pullAll fetches all remote changes since lastSyncSeq and applies them.
func (ws *workspaceSyncer) pullAll(ctx context.Context) error {
	for {
		token, err := ws.engine.auth.GetAccessToken(ctx, ws.engine.grpcClient)
		if err != nil {
			return fmt.Errorf("get access token: %w", err)
		}

		authCtx := ContextWithAuth(ctx, token)
		pullReq := syncv1.PullRequest_builder{
			WorkspaceId: ws.remoteWorkspaceID,
			LastSyncSeq: ws.lastSyncSeq,
			Limit:       pullBatchSize,
		}.Build()

		resp, err := ws.engine.grpcClient.Sync().Pull(authCtx, pullReq)
		if err != nil {
			return fmt.Errorf("pull rpc: %w", err)
		}

		if resp.GetResyncRequired() {
			return ws.resync(ctx)
		}

		if len(resp.GetChanges()) > 0 {
			err = sqlite.WithTx(ctx, ws.engine.db, func(txCtx context.Context) error {
				for _, change := range resp.GetChanges() {
					if err := ws.applyChange(txCtx, change); err != nil {
						slog.Warn("sync: skip change apply", "entityId", change.GetEntityId(), "err", err)
					}
				}
				_, err := sqlite.DBTXFromContext(txCtx, ws.engine.db).ExecContext(txCtx,
					`UPDATE workspaces SET last_sync_seq = ? WHERE id = ?`,
					resp.GetNextSyncSeq(), ws.localWorkspaceID)
				return err
			})
			if err != nil {
				return fmt.Errorf("apply changes: %w", err)
			}

			ws.lastSyncSeq = resp.GetNextSyncSeq()

			ws.engine.eventEmitter("sync:changed", map[string]any{
				"workspaceId": ws.localWorkspaceID,
			})
		}

		if !resp.GetHasMore() {
			return nil
		}
	}
}

func (ws *workspaceSyncer) applyChange(ctx context.Context, change *syncv1.SyncChange) error {
	entity := change.GetEntity()
	if entity == nil {
		return nil
	}
	return ws.applyEntity(ctx, entity)
}

func (ws *workspaceSyncer) applyEntity(ctx context.Context, entity *syncv1.SyncEntity) error {
	wsID, err := uuid.Parse(ws.localWorkspaceID)
	if err != nil {
		return fmt.Errorf("applyEntity: invalid workspace id %q: %w", ws.localWorkspaceID, err)
	}

	switch entity.GetEntityType() {
	case syncv1.EntityType_ENTITY_TYPE_COLLECTION:
		c, err := CollectionFromProto(entity, wsID)
		if err != nil {
			return err
		}
		return ws.upsertCollection(ctx, c)
	case syncv1.EntityType_ENTITY_TYPE_REQUEST:
		r, err := RequestFromProto(entity)
		if err != nil {
			return err
		}
		return ws.upsertRequest(ctx, r)
	case syncv1.EntityType_ENTITY_TYPE_ENVIRONMENT:
		e, err := EnvironmentFromProto(entity, wsID)
		if err != nil {
			return err
		}
		return ws.upsertEnvironment(ctx, e)
	case syncv1.EntityType_ENTITY_TYPE_VARIABLE:
		v, err := VariableFromProto(entity)
		if err != nil {
			return err
		}
		return ws.upsertVariable(ctx, v)
	default:
		return fmt.Errorf("unknown entity type: %v", entity.GetEntityType())
	}
}

func (ws *workspaceSyncer) upsertCollection(ctx context.Context, c *entities.Collection) error {
	existing, err := ws.engine.collections.GetByID(ctx, c.ID)
	if err != nil {
		return err
	}
	if existing != nil {
		return ws.engine.collections.Update(ctx, c)
	}
	return ws.engine.collections.Create(ctx, c)
}

func (ws *workspaceSyncer) upsertRequest(ctx context.Context, r *entities.Request) error {
	// Raw SQL check to detect soft-deleted records that GetByID won't return.
	exists, err := ws.entityExists(ctx, "requests", r.ID.String())
	if err != nil {
		return err
	}
	if exists {
		return ws.engine.requests.Update(ctx, r)
	}
	return ws.engine.requests.Create(ctx, r)
}

func (ws *workspaceSyncer) upsertEnvironment(ctx context.Context, e *entities.Environment) error {
	existing, err := ws.engine.environments.GetByID(ctx, e.ID)
	if err != nil {
		return err
	}
	if existing != nil {
		return ws.engine.environments.Update(ctx, e)
	}
	return ws.engine.environments.Create(ctx, e)
}

func (ws *workspaceSyncer) upsertVariable(ctx context.Context, v *entities.Variable) error {
	existing, err := ws.engine.variables.GetByID(ctx, v.ID)
	if err != nil {
		return err
	}
	if existing != nil {
		return ws.engine.variables.Update(ctx, v)
	}
	return ws.engine.variables.Create(ctx, v)
}

// entityExists checks for a row in the given table regardless of soft-delete flag.
func (ws *workspaceSyncer) entityExists(ctx context.Context, table, id string) (bool, error) {
	var count int
	err := sqlite.DBTXFromContext(ctx, ws.engine.db).QueryRowContext(ctx,
		fmt.Sprintf("SELECT COUNT(1) FROM %s WHERE id = ?", table), id).Scan(&count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// isUnauthenticatedErr walks the wrap chain because engine errors wrap RPC
// status errors via fmt.Errorf("...: %w", err).
func isUnauthenticatedErr(err error) bool {
	for e := err; e != nil; e = errors.Unwrap(e) {
		if s, ok := status.FromError(e); ok && s.Code() == codes.Unauthenticated {
			return true
		}
	}
	return false
}

// handleAuthErr returns true on terminal ErrAuthExpired (caller stops the syncer). A plain
// Unauthenticated drops the cached token (stale expiry after macOS sleep) so the next attempt refreshes.
func (ws *workspaceSyncer) handleAuthErr(err error) (stop bool) {
	if errors.Is(err, ErrAuthExpired) {
		slog.Warn("sync auth expired — stopping syncer until re-login",
			"workspace", ws.localWorkspaceID)
		ws.setState(StateAuthExpired)
		return true
	}
	if isUnauthenticatedErr(err) {
		ws.engine.auth.InvalidateAccessToken()
	}
	return false
}

// grpcStatusOf walks the wrap chain like isUnauthenticatedErr.
func grpcStatusOf(err error) (codes.Code, string, bool) {
	for e := err; e != nil; e = errors.Unwrap(e) {
		if s, ok := status.FromError(e); ok {
			return s.Code(), s.Message(), true
		}
	}
	return codes.OK, "", false
}

// retryAfterSyncErr moves the syncer into the state it waits in and returns the delay
// before the next attempt, for both push and subscribe failures. A plan limit is not a
// connectivity problem: the server keeps refusing until the plan changes, so it waits
// at the slowest interval.
func (ws *workspaceSyncer) retryAfterSyncErr(err error, next time.Duration) time.Duration {
	if code, msg, ok := grpcStatusOf(err); ok && code == codes.ResourceExhausted {
		if strings.Contains(msg, planLimitMarker) {
			ws.enterPlanLimit()
			return maxBackoff
		}
		slog.Warn("sync: resource exhausted without a plan-limit marker", "workspace", ws.localWorkspaceID, "err", msg)
	}
	ws.setState(StateOffline)
	return next
}

// enterPlanLimit parks the syncer and notifies once per episode, not once per retry.
func (ws *workspaceSyncer) enterPlanLimit() {
	ws.setState(StatePlanLimit)
	if !ws.engine.startPlanLimitEpisode() {
		return
	}
	ws.engine.eventEmitter("sync:quota_exceeded", map[string]any{
		"kind":        "members",
		"workspaceId": ws.localWorkspaceID,
	})
}

func (ws *workspaceSyncer) subscribeLoop(ctx context.Context) {
	ws.setState(StateConnected)

	for {
		select {
		case <-ctx.Done():
			ws.setState(StateDisconnected)
			return
		default:
		}

		err := ws.subscribe(ctx)
		if err != nil {
			if ctx.Err() != nil {
				ws.setState(StateDisconnected)
				return
			}
			slog.Error("sync subscribe error", "workspace", ws.localWorkspaceID, "err", err)
			if ws.handleAuthErr(err) {
				return
			}
		}

		backoff := ws.retryAfterSyncErr(err, initialBackoff)
		for {
			select {
			case <-ctx.Done():
				ws.setState(StateDisconnected)
				return
			case <-time.After(backoff):
			}

			ws.setState(StatePushing)
			if err := ws.pushAll(ctx); err != nil {
				slog.Error("sync reconnect push failed", "err", err)
				if ws.handleAuthErr(err) {
					return
				}
				backoff = ws.retryAfterSyncErr(err, min(backoff*2, maxBackoff))
				continue
			}
			ws.setState(StatePulling)
			if err := ws.pullAll(ctx); err != nil {
				slog.Error("sync reconnect pull failed", "err", err)
				if ws.handleAuthErr(err) {
					return
				}
				backoff = min(backoff*2, maxBackoff)
				ws.setState(StateOffline)
				continue
			}
			ws.setState(StateSubscribing)
			break // re-enter subscribe
		}
	}
}

// subscribe opens a gRPC subscribe stream and handles incoming events.
// Returns when the stream ends (disconnect, error, ctx cancel).
func (ws *workspaceSyncer) subscribe(ctx context.Context) error {
	token, err := ws.engine.auth.GetAccessToken(ctx, ws.engine.grpcClient)
	if err != nil {
		return fmt.Errorf("get access token: %w", err)
	}

	cfg, err := ws.engine.configRepo.Get(ctx)
	if err != nil || cfg == nil {
		return fmt.Errorf("get config: %w", err)
	}

	// Per-stream child context so DisconnectStream can cancel just this stream.
	streamCtx, streamCancel := context.WithCancel(ctx)
	ws.mu.Lock()
	ws.streamCancel = streamCancel
	ws.mu.Unlock()
	defer func() {
		ws.mu.Lock()
		if ws.streamCancel != nil {
			ws.streamCancel()
			ws.streamCancel = nil
		}
		ws.mu.Unlock()
	}()

	authCtx := ContextWithAuth(streamCtx, token)
	subReq := syncv1.SubscribeRequest_builder{
		WorkspaceId: ws.remoteWorkspaceID,
		ClientId:    cfg.ClientID,
	}.Build()

	stream, err := ws.engine.grpcClient.Sync().Subscribe(authCtx, subReq)
	if err != nil {
		return fmt.Errorf("subscribe rpc: %w", err)
	}

	ws.setState(StateConnected)

	// Receive loop in a goroutine (Recv is blocking).
	type recvResult struct {
		resp *syncv1.SubscribeResponse
		err  error
	}
	recvCh := make(chan recvResult, 1)
	go func() {
		defer func() {
			if r := recover(); r != nil {
				slog.Error("sync: recv goroutine panic recovered", "panic", r, "stack", string(debug.Stack()), "workspace", ws.localWorkspaceID)
				recvCh <- recvResult{nil, fmt.Errorf("recv goroutine panic: %v", r)}
			}
		}()
		for {
			resp, err := stream.Recv()
			recvCh <- recvResult{resp, err}
			if err != nil {
				return
			}
		}
	}()

	heartbeatTimer := time.NewTimer(heartbeatTimeout)
	defer heartbeatTimer.Stop()

	for {
		select {
		case <-streamCtx.Done():
			// Propagate the outer ctx error, not streamCtx's: subscribeLoop
			// must reconnect after DisconnectStream but stop on real shutdown.
			return ctx.Err()

		case <-heartbeatTimer.C:
			return fmt.Errorf("heartbeat timeout")

		case result := <-recvCh:
			if result.err != nil {
				return fmt.Errorf("stream recv: %w", result.err)
			}

			resp := result.resp

			if resp.HasHeartbeat() {
				heartbeatTimer.Reset(heartbeatTimeout)
				continue
			}

			if resp.HasResync() {
				return ws.resync(ctx)
			}

			if resp.HasChange() {
				change := resp.GetChange()
				if err := ws.applyChange(ctx, change); err != nil {
					slog.Warn("sync: apply subscribe change failed", "err", err)
				}
				ws.engine.eventEmitter("sync:entity_updated", map[string]any{
					"workspaceId": ws.localWorkspaceID,
					"entityType":  change.GetEntityType().String(),
					"entityId":    change.GetEntityId(),
				})
				heartbeatTimer.Reset(heartbeatTimeout)
			}

		case <-ws.pushSignal:
			ws.setState(StatePushing)
			if err := ws.pushAll(ctx); err != nil {
				return fmt.Errorf("push during subscribe: %w", err)
			}
			ws.setState(StateConnected)
		}
	}
}

// resync performs a full resync: clears the queue and re-pulls everything.
func (ws *workspaceSyncer) resync(ctx context.Context) error {
	ws.setState(StateResyncing)

	count, err := ws.engine.syncQueue.DeleteByWorkspace(ctx, ws.localWorkspaceID)
	if err != nil {
		return fmt.Errorf("delete queue: %w", err)
	}
	if count > 0 {
		ws.engine.eventEmitter("sync:data_lost", map[string]any{
			"workspaceId": ws.localWorkspaceID,
			"count":       count,
		})
	}

	ws.lastSyncSeq = 0
	_, err = sqlite.DBTXFromContext(ctx, ws.engine.db).ExecContext(ctx,
		`UPDATE workspaces SET last_sync_seq = 0 WHERE id = ?`, ws.localWorkspaceID)
	if err != nil {
		return fmt.Errorf("reset last_sync_seq: %w", err)
	}

	if err := ws.pullAll(ctx); err != nil {
		return fmt.Errorf("resync pull: %w", err)
	}

	return nil
}

// goOffline starts the offline-retry loop and re-enters the subscribe lifecycle on reconnect.
func (ws *workspaceSyncer) goOffline(ctx context.Context) {
	ws.setState(StateOffline)
	ws.retryLoop(ctx, initialBackoff)
}

// retryLoop waits out backoff, retries push → pull and re-enters subscribe on success.
// The caller sets the state it waits in.
func (ws *workspaceSyncer) retryLoop(ctx context.Context, backoff time.Duration) {
	for {
		select {
		case <-ctx.Done():
			ws.setState(StateDisconnected)
			return
		case <-time.After(backoff):
		}

		ws.setState(StatePushing)
		if err := ws.pushAll(ctx); err != nil {
			if ws.handleAuthErr(err) {
				return
			}
			backoff = ws.retryAfterSyncErr(err, min(backoff*2, maxBackoff))
			continue
		}
		ws.setState(StatePulling)
		if err := ws.pullAll(ctx); err != nil {
			if ws.handleAuthErr(err) {
				return
			}
			backoff = min(backoff*2, maxBackoff)
			ws.setState(StateOffline)
			continue
		}
		ws.setState(StateSubscribing)
		ws.subscribeLoop(ctx)
		return
	}
}

// entityTypePriority orders pushes parent-first: collections, environments, requests, variables.
func entityTypePriority(t syncv1.EntityType) int {
	switch t {
	case syncv1.EntityType_ENTITY_TYPE_COLLECTION:
		return 0
	case syncv1.EntityType_ENTITY_TYPE_ENVIRONMENT:
		return 1
	case syncv1.EntityType_ENTITY_TYPE_REQUEST:
		return 2
	case syncv1.EntityType_ENTITY_TYPE_VARIABLE:
		return 3
	default:
		return 99
	}
}

func sortByEntityType(entities []*syncv1.SyncEntity, ids []int64) {
	type pair struct {
		entity *syncv1.SyncEntity
		id     int64
	}
	pairs := make([]pair, len(entities))
	for i := range entities {
		pairs[i] = pair{entity: entities[i], id: ids[i]}
	}
	slices.SortStableFunc(pairs, func(a, b pair) int {
		return entityTypePriority(a.entity.GetEntityType()) - entityTypePriority(b.entity.GetEntityType())
	})
	for i, p := range pairs {
		entities[i] = p.entity
		ids[i] = p.id
	}
}
