package sync

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"runtime/debug"
	"slices"
	gosync "sync"
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
)

const (
	pushBatchSize        = 100
	pullBatchSize        = 100
	heartbeatTimeout     = 60 * time.Second
	fallbackPollInterval = 15 * time.Minute
	maxBackoff           = 5 * time.Minute
	initialBackoff       = 5 * time.Second
)

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
		ws.cancel()
	}
}

// StopAll cancels all running workspace syncers.
func (e *SyncEngine) StopAll() {
	e.workspaces.Range(func(key, value any) bool {
		ws := value.(*workspaceSyncer)
		ws.cancel()
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

// GetPendingCount returns the number of pending sync queue entries for a workspace.
func (e *SyncEngine) GetPendingCount(ctx context.Context, workspaceID string) (int, error) {
	entries, err := e.syncQueue.ListPending(ctx, workspaceID, 1000)
	if err != nil {
		return 0, err
	}
	return len(entries), nil
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
}

func (ws *workspaceSyncer) getState() SyncState {
	ws.mu.RLock()
	defer ws.mu.RUnlock()
	return ws.state
}

func (ws *workspaceSyncer) setState(s SyncState) {
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
		ws.goOffline(ctx)
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

// pushAll drains the outbox queue by pushing all pending entries to the server.
func (ws *workspaceSyncer) pushAll(ctx context.Context) error {
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
				slog.Warn("sync: push rejected", "entityId", result.GetEntityId(), "error", result.GetErrorMessage())
			}
		}

		if err := ws.engine.syncQueue.Delete(ctx, entryIDs); err != nil {
			return fmt.Errorf("delete queue entries: %w", err)
		}
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
	var entityType string
	for _, e := range entries {
		if e.EntityID == entityID {
			entityType = e.EntityType
			break
		}
	}
	if entityType == "" {
		return
	}

	table := entityType + "s" // collections, requests, environments, variables
	query := fmt.Sprintf("UPDATE %s SET is_synced = 1 WHERE id = ?", table)
	if _, err := sqlite.DBTXFromContext(ctx, ws.engine.db).ExecContext(ctx, query, entityID); err != nil {
		slog.Warn("sync: failed to mark entity as synced", "type", entityType, "id", entityID, "err", err)
	}
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

		ws.setState(StateOffline)
		backoff := initialBackoff
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
				backoff = min(backoff*2, maxBackoff)
				ws.setState(StateOffline)
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

	backoff := initialBackoff
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
			backoff = min(backoff*2, maxBackoff)
			ws.setState(StateOffline)
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
