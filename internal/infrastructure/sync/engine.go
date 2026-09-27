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
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	syncv1 "github.com/tetiva-app/proto/go/gophercourier/sync/v1"

	"github.com/tetiva-app/client/internal/domain/entities"
	"github.com/tetiva-app/client/internal/domain/usecase/auth"
	"github.com/tetiva-app/client/internal/domain/usecase/collection"
	"github.com/tetiva-app/client/internal/domain/usecase/environment"
	"github.com/tetiva-app/client/internal/domain/usecase/example"
	"github.com/tetiva-app/client/internal/domain/usecase/request"
	"github.com/tetiva-app/client/internal/infrastructure/repository/sqlite"
)

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
	// StateUpdateRequired means a peer sent an entity this build cannot represent.
	// The syncer stops, keeps its outbox, and resumes after the app is updated.
	StateUpdateRequired SyncState = "update_required"
)

const (
	pushBatchSize = 100
	// pushBatchBytes stays under the 4 MiB gRPC default: overflow mimics a plan limit.
	pushBatchBytes   = 3 << 20
	pullBatchSize    = 100
	heartbeatTimeout = 60 * time.Second
	maxBackoff       = 5 * time.Minute
	initialBackoff   = 5 * time.Second
	// rejectEventInterval throttles the id-conflict event: one refused push can
	// carry a whole batch of rejected items.
	rejectEventInterval = 5 * time.Minute
	// quotaRetryDelay parks a quota-rejected outbox entry: the plan has to change
	// before the server accepts it, and nothing local will trigger another push.
	quotaRetryDelay = 5 * time.Minute
	// planLimitMarker is the text of the server's domain.ErrQuotaExceeded: sync shares
	// ResourceExhausted with transport limits, so the status code alone means nothing.
	planLimitMarker = "quota exceeded"
	// maxParentRetries bounds how often an example waits for a request the server never received.
	maxParentRetries = 5
	// maxSnapshotRestarts bounds how often one pull starts a snapshot over after its page token was refused.
	maxSnapshotRestarts = 3
	// maxBackfillFailedPasses drops the backfill flag after this many failed passes in a row: otherwise
	// every recheck would download all examples again.
	maxBackfillFailedPasses = 3
)

// knownTypes tells the server this build stores every entity type: an empty list reads as a client
// that predates response examples.
var knownTypes = []syncv1.EntityType{
	syncv1.EntityType_ENTITY_TYPE_COLLECTION,
	syncv1.EntityType_ENTITY_TYPE_ENVIRONMENT,
	syncv1.EntityType_ENTITY_TYPE_REQUEST,
	syncv1.EntityType_ENTITY_TYPE_VARIABLE,
	syncv1.EntityType_ENTITY_TYPE_RESPONSE_EXAMPLE,
}

var backfillTypes = []syncv1.EntityType{syncv1.EntityType_ENTITY_TYPE_RESPONSE_EXAMPLE}

// requeueDelay is how long the engine waits before waking a syncer whose entries
// are parked. A var so tests need not wait out the real window.
var requeueDelay = quotaRetryDelay

// requeueFloor keeps a stale deadline from spinning the wake-up: next_retry_at is
// stored with second precision, so a past-due value only ever misses by a fraction.
const requeueFloor = 250 * time.Millisecond

// stopAllTimeout bounds the barrier: a syncer wedged in a Push must not hold
// sign-in or shutdown hostage.
const stopAllTimeout = 5 * time.Second

// capability is what the connected server said about response examples.
type capability int

const (
	capUnknown capability = iota
	capSupported
	capUnsupported
)

// serverInfoTimeout bounds the capability check at the start of a cycle; a var so tests can shorten it.
var serverInfoTimeout = 5 * time.Second

// capabilityRecheckInterval restarts a cycle whose capability stayed unknown; a var so tests can shorten it.
var capabilityRecheckInterval = 15 * time.Minute

// errRecheck ends a subscribe so the reconnect path reads the capability again.
var errRecheck = errors.New("sync: recheck cycle")

// EventEmitter is a function that emits events to the frontend via Wails.
type EventEmitter func(name string, data any)

// TokenCleaner invalidates locally stored OAuth 2.0 tokens: inbound sync writes go straight
// to the repositories and bypass the usecase hooks, so the engine keeps the store in step.
type TokenCleaner interface {
	Clear(ctx context.Context, owner entities.AuthOwner) error
	DeleteOrphans(ctx context.Context) (int, error)
}

// noopTokenCleaner stands in for engines built without a token store.
type noopTokenCleaner struct{}

func (noopTokenCleaner) Clear(context.Context, entities.AuthOwner) error { return nil }

func (noopTokenCleaner) DeleteOrphans(context.Context) (int, error) { return 0, nil }

type SyncEngine struct {
	auth *SyncAuthManager
	// grpcClient is swapped while syncers run: a browser sign-in adopts another
	// session and hands the engine the connection it was made on.
	grpcClient atomic.Pointer[GRPCClient]
	syncQueue  sqlite.SyncQueueRepository
	configRepo sqlite.SyncConfigRepository
	db         *sql.DB
	// Inner repos (bypass decorators to avoid re-enqueue loop).
	collections       collection.Repository
	requests          request.Repository
	environments      environment.Repository
	variables         environment.VariableRepository
	examples          example.Repository
	tokens            TokenCleaner
	eventEmitter      EventEmitter
	workspaces        gosync.Map // workspaceID string → *workspaceSyncer
	enabledWorkspaces gosync.Map // workspaceID string → bool
	// planLimit marks an open plan-limit episode; the freeze is org-wide, so the
	// stamp lives here and not on every workspace syncer.
	planLimit atomic.Bool
	// syncers counts live run goroutines, replaced ones included: StopWorkspace, ForcePull,
	// ForceResync and Resume drop the map entry and start a successor.
	syncers syncerCount
	// startBarrier serializes starts against stopAll: a start landing after the
	// cancel sweep would outlive the barrier the client swap relies on.
	startBarrier gosync.RWMutex
	// examplesClient is the client whose server confirmed response examples; capMu
	// also covers the grpcClient swap so a late answer cannot land on the new client.
	capMu          gosync.Mutex
	examplesClient *GRPCClient

	connectedMu    gosync.Mutex
	connectedHooks []func()
}

// syncerCount tracks the live run goroutines. A WaitGroup cannot serve here: the
// barrier gives up on wedged syncers, and a later start would reuse a parked Wait.
type syncerCount struct {
	mu gosync.Mutex
	n  int
	// idle closes when the count falls back to zero; the next add opens a fresh one.
	idle chan struct{}
}

func (c *syncerCount) add() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.n == 0 {
		c.idle = make(chan struct{})
	}
	c.n++
}

func (c *syncerCount) done() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.n--
	if c.n == 0 && c.idle != nil {
		close(c.idle)
	}
}

// wait reports whether every counted goroutine exited before the timeout. A
// timed-out wait leaves nothing behind, so the count stays usable afterwards.
func (c *syncerCount) wait(timeout time.Duration) bool {
	c.mu.Lock()
	if c.n == 0 {
		c.mu.Unlock()
		return true
	}
	idle := c.idle
	c.mu.Unlock()

	timer := time.NewTimer(timeout)
	defer timer.Stop()
	select {
	case <-idle:
		return true
	case <-timer.C:
		return false
	}
}

func NewSyncEngine(
	auth *SyncAuthManager,
	syncQueue sqlite.SyncQueueRepository,
	configRepo sqlite.SyncConfigRepository,
	db *sql.DB,
	collections collection.Repository,
	requests request.Repository,
	environments environment.Repository,
	variables environment.VariableRepository,
	examples example.Repository,
	tokens TokenCleaner,
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
		examples:     examples,
		tokens:       tokens,
		eventEmitter: func(string, any) {}, // no-op until set
	}
}

func (e *SyncEngine) tokenCleaner() TokenCleaner {
	if e.tokens == nil {
		return noopTokenCleaner{}
	}
	return e.tokens
}

// OnConnected registers fn to run in its own goroutine whenever a workspace syncer comes back to
// StateConnected, which is when work waiting for the server can be retried.
func (e *SyncEngine) OnConnected(fn func()) {
	e.connectedMu.Lock()
	defer e.connectedMu.Unlock()
	e.connectedHooks = append(e.connectedHooks, fn)
}

func (e *SyncEngine) notifyConnected() {
	e.connectedMu.Lock()
	hooks := slices.Clone(e.connectedHooks)
	e.connectedMu.Unlock()
	for _, fn := range hooks {
		go fn()
	}
}

func (e *SyncEngine) SetEventEmitter(fn EventEmitter) {
	e.eventEmitter = fn
}

// SetGRPCClient allows late binding during DI setup.
func (e *SyncEngine) SetGRPCClient(client *GRPCClient) {
	e.capMu.Lock()
	defer e.capMu.Unlock()
	e.grpcClient.Store(client)
	e.examplesClient = nil
}

// GRPCClient is exported because the sign-in commit must be verifiable from the adapters package.
func (e *SyncEngine) GRPCClient() *GRPCClient {
	return e.grpcClient.Load()
}

// examplesCapability asks the server whether it takes response examples. Only support
// is cached: an unknown or negative answer is asked again on the next cycle.
func (e *SyncEngine) examplesCapability(ctx context.Context) capability {
	client := e.grpcClient.Load()
	if client == nil || client.auth == nil {
		return capUnsupported
	}
	if e.examplesConfirmed(client) {
		return capSupported
	}

	callCtx, cancel := context.WithTimeout(ctx, serverInfoTimeout)
	defer cancel()
	info, err := client.GetServerInfo(callCtx)

	e.capMu.Lock()
	defer e.capMu.Unlock()
	if e.grpcClient.Load() != client {
		return capUnknown
	}
	switch {
	case errors.Is(err, auth.ErrServerInfoUnsupported):
		return capUnsupported
	case err != nil:
		if ctx.Err() == nil {
			slog.Warn("sync: server capability check failed", "err", err)
		}
		return capUnknown
	case !info.ResponseExamples:
		return capUnsupported
	}
	e.examplesClient = client
	return capSupported
}

func (e *SyncEngine) examplesConfirmed(client *GRPCClient) bool {
	e.capMu.Lock()
	defer e.capMu.Unlock()
	return client != nil && e.examplesClient == client
}

func (e *SyncEngine) forgetExamplesCapability() {
	e.capMu.Lock()
	defer e.capMu.Unlock()
	e.examplesClient = nil
}

// excludedTypes lists the entity types a server without the examples capability must not be sent.
func excludedTypes(c capability) []string {
	if c == capSupported {
		return nil
	}
	return []string{"response_example"}
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

// IsEnabledForWorkspace is used by decorator repos to decide whether to enqueue operations.
func (e *SyncEngine) IsEnabledForWorkspace(workspaceID string) bool {
	v, ok := e.enabledWorkspaces.Load(workspaceID)
	if !ok {
		return false
	}
	return v.(bool)
}

// StartWorkspace waits out a running StopAll instead of slipping a syncer past its barrier.
func (e *SyncEngine) StartWorkspace(localWorkspaceID, remoteWorkspaceID string, lastSyncSeq int64) {
	e.startBarrier.RLock()
	defer e.startBarrier.RUnlock()

	e.enabledWorkspaces.Store(localWorkspaceID, true)

	ws := &workspaceSyncer{
		engine:            e,
		localWorkspaceID:  localWorkspaceID,
		remoteWorkspaceID: remoteWorkspaceID,
		state:             StateIdle,
		pushSignal:        make(chan struct{}, 1),
		lastSyncSeq:       lastSyncSeq,
		done:              make(chan struct{}),
	}

	ctx, cancel := context.WithCancel(context.Background())
	ws.cancel = cancel
	e.workspaces.Store(localWorkspaceID, ws)

	e.syncers.add()
	go ws.run(ctx)
}

func (e *SyncEngine) StopWorkspace(localWorkspaceID string) {
	e.stopWorkspace(localWorkspaceID)
}

// stopWorkspace returns the syncer it cancelled, nil when none was tracked.
func (e *SyncEngine) stopWorkspace(localWorkspaceID string) *workspaceSyncer {
	e.enabledWorkspaces.Delete(localWorkspaceID)
	v, ok := e.workspaces.LoadAndDelete(localWorkspaceID)
	if !ok {
		return nil
	}
	ws := v.(*workspaceSyncer)
	ws.stop()
	return ws
}

// StopWorkspaceAndWait also waits for the syncer's goroutine, so a pull it still has in flight cannot
// land after the caller's next write. False when the goroutine outlived stopAllTimeout.
func (e *SyncEngine) StopWorkspaceAndWait(localWorkspaceID string) bool {
	return e.stopWorkspaceAndWait(localWorkspaceID, stopAllTimeout)
}

func (e *SyncEngine) stopWorkspaceAndWait(localWorkspaceID string, timeout time.Duration) bool {
	ws := e.stopWorkspace(localWorkspaceID)
	if ws == nil || ws.done == nil {
		return true
	}

	timer := time.NewTimer(timeout)
	defer timer.Stop()
	select {
	case <-ws.done:
		return true
	case <-timer.C:
		slog.Warn("sync: workspace syncer did not stop in time", "workspace", localWorkspaceID, "after", timeout)
		return false
	}
}

// StopAll cancels every workspace syncer, replaced ones included, and reports
// whether they all joined — a false barrier must not pass for a held one.
func (e *SyncEngine) StopAll() bool {
	return e.stopAll(stopAllTimeout)
}

// stopAll takes the barrier's ceiling so a test need not wait out the real one.
func (e *SyncEngine) stopAll(timeout time.Duration) bool {
	e.startBarrier.Lock()
	defer e.startBarrier.Unlock()

	e.workspaces.Range(func(key, value any) bool {
		ws := value.(*workspaceSyncer)
		ws.stop()
		e.workspaces.Delete(key)
		e.enabledWorkspaces.Delete(key)
		return true
	})

	if !e.syncers.wait(timeout) {
		// The count has no names: the log says a syncer is wedged, not which.
		slog.Warn("sync: StopAll timed out waiting for syncers", "after", timeout)

		return false
	}

	return true
}

// NotifyWrite is called by decorator repos (or the Wails service) after any entity write.
func (e *SyncEngine) NotifyWrite(workspaceID string) {
	if v, ok := e.workspaces.Load(workspaceID); ok {
		ws := v.(*workspaceSyncer)
		select {
		case ws.pushSignal <- struct{}{}:
		default: // already signaled — no-op
		}
	}
}

func (e *SyncEngine) GetWorkspaceState(localWorkspaceID string) SyncState {
	if v, ok := e.workspaces.Load(localWorkspaceID); ok {
		ws := v.(*workspaceSyncer)
		return ws.getState()
	}
	return StateDisconnected
}

// GetPendingCount counts entries that still owe the server a push, parked retries included;
// examples only once the current server confirmed it takes them, without asking it here.
func (e *SyncEngine) GetPendingCount(ctx context.Context, workspaceID string) (int, error) {
	confirmed := capUnknown
	if e.examplesConfirmed(e.grpcClient.Load()) {
		confirmed = capSupported
	}
	return e.syncQueue.CountPendingOrFailed(ctx, workspaceID, excludedTypes(confirmed))
}

func (e *SyncEngine) GetParkedCount(ctx context.Context, workspaceID string) (int, error) {
	return e.syncQueue.CountParked(ctx, workspaceID)
}

func (e *SyncEngine) GetTooLargeCount(ctx context.Context, workspaceID string) (int, error) {
	return e.syncQueue.CountTooLarge(ctx, workspaceID)
}

// QueueUnsyncedTree queues the parts of a collection tree the server never confirmed and wakes the push.
func (e *SyncEngine) QueueUnsyncedTree(ctx context.Context, workspaceID, collectionID string) (int, error) {
	n, err := e.syncQueue.EnqueueUnsyncedTree(ctx, workspaceID, collectionID)
	if err != nil {
		return 0, fmt.Errorf("SyncEngine.QueueUnsyncedTree: %w", err)
	}
	if n > 0 {
		e.NotifyWrite(workspaceID)
	}
	return n, nil
}

// ForcePush returns the number of pending entries seen before the push signal was sent.
func (e *SyncEngine) ForcePush(ctx context.Context, workspaceID string) (int, error) {
	count, err := e.GetPendingCount(ctx, workspaceID)
	if err != nil {
		return 0, fmt.Errorf("get pending count: %w", err)
	}
	e.NotifyWrite(workspaceID)
	return count, nil
}

// ForcePull restarts the workspace syncer preserving current seq: push + incremental pull + subscribe.
func (e *SyncEngine) ForcePull(workspaceID string) error {
	v, ok := e.workspaces.Load(workspaceID)
	if !ok {
		return fmt.Errorf("workspace %s is not syncing", workspaceID)
	}
	ws := v.(*workspaceSyncer)
	remoteID := ws.remoteWorkspaceID
	lastSeq := ws.cursor()

	e.StopWorkspace(workspaceID)
	e.StartWorkspace(workspaceID, remoteID, lastSeq)
	return nil
}

// ForceResync resets last_sync_seq to 0, clears the outbox queue and restarts the full cycle.
func (e *SyncEngine) ForceResync(ctx context.Context, workspaceID string) error {
	v, ok := e.workspaces.Load(workspaceID)
	if !ok {
		return fmt.Errorf("workspace %s is not syncing", workspaceID)
	}
	ws := v.(*workspaceSyncer)
	remoteID := ws.remoteWorkspaceID
	lastSeq := ws.cursor()

	// A pull still in flight would write its cursor over the reset.
	if !e.StopWorkspaceAndWait(workspaceID) {
		e.StartWorkspace(workspaceID, remoteID, lastSeq)
		return errors.New("sync is still stopping, try again in a moment")
	}

	if _, err := e.clearOutbox(ctx, workspaceID); err != nil {
		e.StartWorkspace(workspaceID, remoteID, lastSeq)
		return fmt.Errorf("clear queue: %w", err)
	}

	e.StartWorkspace(workspaceID, remoteID, 0)
	return nil
}

// clearOutbox sends the workspace back to cursor 0 and returns how many queued rows are gone for good.
// The clear, the re-offers and the reset share one transaction: a clear alone drops unsent docs and examples.
func (e *SyncEngine) clearOutbox(ctx context.Context, workspaceID string) (int, error) {
	var lost, reoffered, examples int

	err := sqlite.WithTx(ctx, e.db, func(txCtx context.Context) error {
		before, err := e.queuedEntities(txCtx, workspaceID)
		if err != nil {
			return err
		}
		if _, err = e.syncQueue.DeleteByWorkspace(txCtx, workspaceID); err != nil {
			return fmt.Errorf("delete queue: %w", err)
		}
		if reoffered, err = e.syncQueue.EnqueueDocumentedRequests(txCtx, workspaceID); err != nil {
			return fmt.Errorf("re-queue documented requests: %w", err)
		}
		if examples, err = e.syncQueue.EnqueueUnsyncedExamples(txCtx, workspaceID); err != nil {
			return fmt.Errorf("re-queue unsynced examples: %w", err)
		}
		after, err := e.queuedEntities(txCtx, workspaceID)
		if err != nil {
			return err
		}
		for entity, rows := range before {
			if after[entity] == 0 {
				lost += rows
			}
		}
		if _, err := sqlite.DBTXFromContext(txCtx, e.db).ExecContext(txCtx,
			`DELETE FROM sync_snapshot_seen WHERE workspace_id = ?`, workspaceID); err != nil {
			return fmt.Errorf("forget the walks: %w", err)
		}
		// The snapshot from cursor 0 brings every example, so no backfill is owed.
		if _, err := sqlite.DBTXFromContext(txCtx, e.db).ExecContext(txCtx,
			`UPDATE workspaces
			 SET last_sync_seq = 0, sync_page_token = '', examples_backfill_pending = 0, examples_backfill_token = '',
			     examples_backfill_incomplete = 0, examples_backfill_failed_passes = 0
			 WHERE id = ?`, workspaceID); err != nil {
			return fmt.Errorf("reset pull position: %w", err)
		}
		return nil
	})
	if err != nil {
		return 0, err
	}

	if reoffered > 0 {
		slog.Info("sync: re-queued documented requests after a resync", "workspace", workspaceID, "count", reoffered)
	}
	if examples > 0 {
		slog.Info("sync: re-queued unsynced examples after a resync", "workspace", workspaceID, "count", examples)
	}
	return lost, nil
}

// queuedEntities counts the queue rows of each entity, keyed by type and id.
func (e *SyncEngine) queuedEntities(ctx context.Context, workspaceID string) (map[string]int, error) {
	rows, err := sqlite.DBTXFromContext(ctx, e.db).QueryContext(ctx,
		`SELECT entity_type || ':' || entity_id, COUNT(*) FROM sync_queue WHERE workspace_id = ? GROUP BY entity_type, entity_id`,
		workspaceID)
	if err != nil {
		return nil, fmt.Errorf("count queued entities: %w", err)
	}
	defer func() { _ = rows.Close() }()

	counts := make(map[string]int)
	for rows.Next() {
		var key string
		var n int
		if err := rows.Scan(&key, &n); err != nil {
			return nil, fmt.Errorf("count queued entities: %w", err)
		}
		counts[key] = n
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("count queued entities: %w", err)
	}
	return counts, nil
}

// InjectRawSyncer inserts a workspaceSyncer without starting a goroutine. Tests only.
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
	parked          int
	tooLarge        int
	quotaEpisode    bool
	// requeueTimer wakes the syncer once when parked entries fall due: no local
	// write is coming to trigger the push that would requeue them.
	requeueTimer *time.Timer
	// examplesCap is set once per cycle and read by drain, which runs inside subscribe's
	// select and must not wait on an RPC. Only the run goroutine touches it.
	examplesCap capability
	// backfillPending is examples_backfill_pending as the latest backfill attempt left it; like
	// examplesCap, it belongs to the run goroutine.
	backfillPending bool
	// done closes when run returns; nil for a syncer that never had a goroutine.
	done chan struct{}
}

// refreshCapability runs at the start of every cycle, before its first push. Only a recheck
// of an unbroken stream may reuse the cache: a restart can put an older server behind the same address.
func (ws *workspaceSyncer) refreshCapability(ctx context.Context, reuseCached bool) {
	if !reuseCached {
		ws.engine.forgetExamplesCapability()
	}
	ws.examplesCap = ws.engine.examplesCapability(ctx)
}

// needsRecheck arms no timer for an older server: migration 021 flags every workspace linked
// before it, and the cycle would restart every interval for nothing.
func (ws *workspaceSyncer) needsRecheck() bool {
	return ws.examplesCap == capUnknown || (ws.examplesCap == capSupported && ws.backfillPending)
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

func (ws *workspaceSyncer) cursor() int64 {
	ws.mu.RLock()
	defer ws.mu.RUnlock()
	return ws.lastSyncSeq
}

func (ws *workspaceSyncer) setCursor(seq int64) {
	ws.mu.Lock()
	defer ws.mu.Unlock()
	ws.lastSyncSeq = seq
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
	prev := ws.state
	ws.state = s
	ws.mu.Unlock()
	ws.engine.eventEmitter("sync:status", map[string]any{
		"workspaceId": ws.localWorkspaceID,
		"state":       string(s),
	})
	if s == StateConnected && prev != StateConnected {
		ws.engine.notifyConnected()
	}
}

// run performs push → pull → subscribe for the workspace goroutine.
func (ws *workspaceSyncer) run(ctx context.Context) {
	// Registered first so it runs last: StopAll's barrier must also clear after a
	// panic the recover below turns into a reconnect.
	defer ws.engine.syncers.done()
	defer close(ws.done)
	defer func() {
		if r := recover(); r != nil {
			slog.Error("sync: goroutine panic recovered", "panic", r, "stack", string(debug.Stack()), "workspace", ws.localWorkspaceID)
			// goOffline retries with backoff and respects ctx, so a panic
			// becomes a reconnect attempt instead of a dead workspace.
			ws.goOffline(ctx)
		}
	}()

	ws.setState(StatePushing)
	ws.refreshCapability(ctx, false)
	if err := ws.pushAll(ctx); err != nil {
		slog.Error("sync push failed", "workspace", ws.localWorkspaceID, "err", err)
		if ws.handleAuthErr(err) {
			return
		}
		if ws.parkForUpdate(err) {
			return
		}
		ws.retryLoop(ctx, ws.retryAfterSyncErr(err, initialBackoff))
		return
	}

	ws.setState(StatePulling)
	if err := ws.pullCycle(ctx); err != nil {
		slog.Error("sync pull failed", "workspace", ws.localWorkspaceID, "err", err)
		if ws.handleAuthErr(err) {
			return
		}
		if ws.parkForUpdate(err) {
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

	if _, err := ws.engine.syncQueue.DeleteSupersededParked(ctx, ws.localWorkspaceID); err != nil {
		slog.Warn("sync: failed to drop superseded parked entries", "workspace", ws.localWorkspaceID, "err", err)
	}

	ws.dropParkedForGoneEntities(ctx)

	if err := ws.drainOutbox(ctx); err != nil {
		return err
	}

	ws.rearmRequeue(ctx)
	return nil
}

// drainOutbox pushes all pending entries to the server, batch by batch.
func (ws *workspaceSyncer) drainOutbox(ctx context.Context) error {
	for {
		entries, err := ws.engine.syncQueue.CoalescedPending(ctx, ws.localWorkspaceID, pushBatchSize, excludedTypes(ws.examplesCap))
		if err != nil {
			return fmt.Errorf("coalesce pending: %w", err)
		}
		if len(entries) == 0 {
			return nil
		}

		var protoEntities []*syncv1.SyncEntity
		var entryIDs []int64
		// Every row accounted for in this pass stands for its entity; its older pending rows go after it.
		var handled []*sqlite.SyncEntry
		exampleParents := make(map[string]string)
		sizes := make(map[int64]int)
		// sent holds the row version each entity went out with: an ACK confirms that version only.
		sent := make(map[string]int)
		batchBytes := 0

		for _, entry := range entries {
			entity, err := ws.readEntity(ctx, entry.EntityType, entry.EntityID)
			if err != nil {
				slog.Warn("sync: skip entity read error", "type", entry.EntityType, "id", entry.EntityID, "err", err)
				if err := ws.engine.syncQueue.Delete(ctx, []int64{entry.ID}); err != nil {
					slog.Warn("sync: failed to delete queue entry after read error", "entry", entry.ID, "workspace", ws.localWorkspaceID, "err", err)
				}
				handled = append(handled, entry)
				continue
			}

			var protoEntity *syncv1.SyncEntity
			version, versionKnown := 0, false
			switch {
			case entity != nil:
				protoEntity = entityProto(entity, entry.OperationID)
				if ex, ok := entity.(*entities.ResponseExample); ok {
					exampleParents[entry.EntityID] = ex.RequestID.String()
				}
				if protoEntity != nil {
					version, versionKnown = int(protoEntity.GetVersion()), true
				}
			case entry.Action == "delete":
				if protoEntity, version, versionKnown = ws.tombstone(ctx, entry); protoEntity == nil {
					if err := ws.engine.syncQueue.Delete(ctx, []int64{entry.ID}); err != nil {
						slog.Warn("sync: failed to delete unresolvable delete queue entry", "entry", entry.ID, "workspace", ws.localWorkspaceID, "err", err)
					}
					handled = append(handled, entry)
					continue
				}
			default:
				if err := ws.engine.syncQueue.Delete(ctx, []int64{entry.ID}); err != nil {
					slog.Warn("sync: failed to delete stale queue entry for nil entity", "entry", entry.ID, "workspace", ws.localWorkspaceID, "err", err)
				}
				handled = append(handled, entry)
				continue
			}

			if protoEntity != nil {
				size := proto.Size(protoEntity)
				// An entity over the budget still goes, alone: only the server can refuse it.
				if len(protoEntities) > 0 && batchBytes+size > pushBatchBytes {
					break
				}
				batchBytes += size
				protoEntities = append(protoEntities, protoEntity)
				entryIDs = append(entryIDs, entry.ID)
				sizes[entry.ID] = size
				if versionKnown {
					sent[entry.EntityID] = version
				}
				handled = append(handled, entry)
			}
		}

		if len(protoEntities) == 0 {
			ws.dropOlderPending(ctx, handled)
			return nil
		}

		// Parents must be pushed before children.
		sortByEntityType(protoEntities, entryIDs)

		token, err := ws.engine.auth.GetAccessToken(ctx, ws.engine.GRPCClient())
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

		resp, err := ws.engine.GRPCClient().Sync().Push(authCtx, pushReq)
		if err != nil {
			if len(entryIDs) == 1 && isOversizedPushErr(err) &&
				ws.parkOversized(ctx, entryByID(entries, entryIDs[0]), batchBytes) {
				ws.dropOlderPending(ctx, handled)
				continue
			}
			return fmt.Errorf("push rpc: %w", err)
		}
		ws.engine.clearPlanLimit()

		var quotaRejected, updateRejected, oversized []int64
		var parentMissing []*sqlite.SyncEntry
		needsUpdate := false
		for _, result := range resp.GetResults() {
			switch result.GetStatus() {
			case syncv1.PushStatus_PUSH_STATUS_ACCEPTED, syncv1.PushStatus_PUSH_STATUS_DUPLICATE:
				ws.markSynced(ctx, result.GetEntityId(), entries, sent)
			case syncv1.PushStatus_PUSH_STATUS_CONFLICT_RESOLVED:
				// No winner means the server stored the pushed copy.
				if result.GetWinner() == nil {
					ws.markSynced(ctx, result.GetEntityId(), entries, sent)
				} else {
					err := ws.applyInbound(ctx, result.GetWinner())
					switch {
					case err == nil:
						ws.sweepTokens(ctx)
					case errors.Is(err, ErrUpdateRequired):
						// Nothing changed locally, so no event; the outbox entry stays
						// parked instead of deleted or the local edit would be lost.
						needsUpdate = true
						if id, ok := entryIDOf(result.GetEntityId(), entries); ok {
							updateRejected = append(updateRejected, id)
						}
						continue
					default:
						slog.Warn("sync: apply conflict winner failed", "entityId", result.GetEntityId(), "err", err)
					}
					ws.engine.eventEmitter("sync:entity_updated", map[string]any{
						"workspaceId": ws.localWorkspaceID,
						"entityId":    result.GetEntityId(),
					})
				}
			case syncv1.PushStatus_PUSH_STATUS_REJECTED:
				ws.handleRejected(result, entries)
				switch result.GetRejectReason() {
				case syncv1.PushRejectReason_PUSH_REJECT_REASON_QUOTA_EXCEEDED:
					if id, ok := entryIDOf(result.GetEntityId(), entries); ok {
						quotaRejected = append(quotaRejected, id)
					}
				case syncv1.PushRejectReason_PUSH_REJECT_REASON_PARENT_NOT_FOUND:
					if id, ok := entryIDOf(result.GetEntityId(), entries); ok {
						if entry := entryByID(entries, id); entry.EntityType == "response_example" {
							parentMissing = append(parentMissing, entry)
						}
					}
				case syncv1.PushRejectReason_PUSH_REJECT_REASON_TOO_LARGE:
					if id, ok := entryIDOf(result.GetEntityId(), entries); ok {
						oversized = append(oversized, id)
					}
				}
			}
		}

		entryIDs = ws.parkEntries(ctx, entryIDs, quotaRejected)
		entryIDs = ws.parkEntries(ctx, entryIDs, updateRejected)
		entryIDs = ws.deferOrphanExamples(ctx, entryIDs, parentMissing, exampleParents)
		for _, id := range oversized {
			if ws.parkOversized(ctx, entryByID(entries, id), sizes[id]) {
				entryIDs = slices.DeleteFunc(entryIDs, func(x int64) bool { return x == id })
			}
		}

		if err := ws.engine.syncQueue.Delete(ctx, entryIDs); err != nil {
			return fmt.Errorf("delete queue entries: %w", err)
		}
		ws.dropOlderPending(ctx, handled)

		ws.refreshParked(ctx)

		// Reported only after the batch is accounted for: the rest of the results
		// are ordinary work and must not be lost with the batch.
		if needsUpdate {
			return fmt.Errorf("apply conflict winner: %w", ErrUpdateRequired)
		}
	}
}

// dropOlderPending keeps a coalesced entity from going out again under each of its older rows.
func (ws *workspaceSyncer) dropOlderPending(ctx context.Context, handled []*sqlite.SyncEntry) {
	if err := ws.engine.syncQueue.DropOlderPending(ctx, handled); err != nil {
		slog.Warn("sync: failed to drop superseded pending entries", "workspace", ws.localWorkspaceID, "err", err)
	}
}

// deferOrphanExamples holds examples the server refused for a missing request; the first miss re-offers
// the request, and a miss while the request itself is held in the queue spends no attempt.
func (ws *workspaceSyncer) deferOrphanExamples(ctx context.Context, entryIDs []int64, orphans []*sqlite.SyncEntry, parents map[string]string) []int64 {
	if len(orphans) == 0 {
		return entryIDs
	}

	retryAt := time.Now().Add(quotaRetryDelay)
	kept := make(map[int64]bool, len(orphans))
	for _, entry := range orphans {
		held, err := ws.engine.syncQueue.ExampleParentHeld(ctx, entry.WorkspaceID, entry.EntityID)
		if err != nil {
			slog.Warn("sync: failed to check the parent of a deferred example", "workspace", ws.localWorkspaceID, "exampleId", entry.EntityID, "err", err)
			held = true
		}
		if !held && entry.DeferCount >= maxParentRetries-1 {
			slog.Warn("sync: dropping an example whose request never reached the server",
				"workspace", ws.localWorkspaceID, "exampleId", entry.EntityID, "attempts", entry.DeferCount+1)
			continue
		}
		if requestID, ok := parents[entry.EntityID]; ok && entry.DeferCount == 0 {
			if _, err := ws.engine.syncQueue.EnqueueRequestIfAbsent(ctx, entry.WorkspaceID, requestID); err != nil {
				slog.Warn("sync: failed to re-offer the parent request", "workspace", ws.localWorkspaceID, "requestId", requestID, "err", err)
			}
		}
		if err := ws.engine.syncQueue.MarkDeferred(ctx, entry.ID, retryAt, !held); err != nil {
			slog.Warn("sync: failed to defer queue entry", "entry", entry.ID, "workspace", ws.localWorkspaceID, "err", err)
			continue
		}
		kept[entry.ID] = true
	}

	if len(kept) > 0 {
		ws.scheduleRequeue()
	}

	return slices.DeleteFunc(entryIDs, func(id int64) bool { return kept[id] })
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
	case "response_example":
		ex, err := ws.engine.examples.GetByID(ctx, id)
		if ex == nil || err != nil {
			return nil, err
		}
		return ex, nil
	default:
		return nil, fmt.Errorf("unknown entity type: %s", entityType)
	}
}

func entityProto(entity any, operationID string) *syncv1.SyncEntity {
	switch e := entity.(type) {
	case *entities.Collection:
		return CollectionToProto(e, operationID)
	case *entities.Request:
		return RequestToProto(e, operationID)
	case *entities.Environment:
		return EnvironmentToProto(e, operationID)
	case *entities.Variable:
		return VariableToProto(e, operationID)
	case *entities.ResponseExample:
		return ResponseExampleToProto(e, operationID)
	}
	return nil
}

// tombstone builds a delete push and the row version it confirms. An older server stores a tombstone
// as sent, so it gets the deleted row stamped with the push time: a bare one reaches peers unreadable.
func (ws *workspaceSyncer) tombstone(ctx context.Context, entry *sqlite.SyncEntry) (*syncv1.SyncEntity, int, bool) {
	serverDated := ws.examplesCap == capSupported
	if !serverDated {
		deleted, err := ws.readDeletedEntity(ctx, entry.EntityType, entry.EntityID)
		if err != nil {
			slog.Warn("sync: failed to read a deleted row, sending a bare tombstone", "type", entry.EntityType, "id", entry.EntityID, "err", err)
		}
		if p := entityProto(deleted, entry.OperationID); p != nil {
			p.SetIsDeleted(true)
			p.SetUpdatedAt(timestamppb.Now())
			return p, int(p.GetVersion()), true
		}
	}

	p := buildDeleteProto(entry, serverDated)
	if p == nil {
		return nil, 0, false
	}
	version, ok := ws.rowVersion(ctx, entry.EntityType, entry.EntityID)
	return p, version, ok
}

// deletedRowReader is offered by the SQLite repositories the engine runs on; without it a delete goes out bare.
type deletedRowReader[T any] interface {
	GetByIDIncludingDeleted(ctx context.Context, id uuid.UUID) (T, error)
}

// readDeletedEntity returns (nil, nil) when the row is gone for good, like a hard-deleted draft.
func (ws *workspaceSyncer) readDeletedEntity(ctx context.Context, entityType, entityID string) (any, error) {
	id, err := uuid.Parse(entityID)
	if err != nil {
		return nil, fmt.Errorf("readDeletedEntity: invalid entity id %q: %w", entityID, err)
	}
	e := ws.engine
	switch entityType {
	case "collection":
		return readDeleted[*entities.Collection](ctx, e.collections, id)
	case "request":
		return readDeleted[*entities.Request](ctx, e.requests, id)
	case "environment":
		return readDeleted[*entities.Environment](ctx, e.environments, id)
	case "variable":
		return readDeleted[*entities.Variable](ctx, e.variables, id)
	case "response_example":
		return readDeleted[*entities.ResponseExample](ctx, e.examples, id)
	}
	return nil, nil
}

func readDeleted[T comparable](ctx context.Context, repo any, id uuid.UUID) (any, error) {
	reader, ok := repo.(deletedRowReader[T])
	if !ok {
		return nil, nil
	}
	row, err := reader.GetByIDIncludingDeleted(ctx, id)
	var none T
	if err != nil || row == none {
		return nil, err
	}
	return row, nil
}

// buildDeleteProto builds the minimal SyncEntity server-side deletion needs. A serverDated
// tombstone carries no time and the server dates it; an older server needs the push time to win LWW.
func buildDeleteProto(entry *sqlite.SyncEntry, serverDated bool) *syncv1.SyncEntity {
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
	case "response_example":
		entityType = syncv1.EntityType_ENTITY_TYPE_RESPONSE_EXAMPLE
	default:
		return nil
	}
	b := syncv1.SyncEntity_builder{
		EntityType:  entityType,
		EntityId:    entry.EntityID,
		IsDeleted:   true,
		OperationId: entry.OperationID,
	}
	if !serverDated {
		b.UpdatedAt = timestamppb.Now()
	}
	return b.Build()
}

// markSynced leaves a row edited during the push unsynced: its newer version is still owed to the server.
func (ws *workspaceSyncer) markSynced(ctx context.Context, entityID string, entries []*sqlite.SyncEntry, sent map[string]int) {
	table, ok := syncedTable(entityTypeOf(entityID, entries))
	version, known := sent[entityID]
	if !ok || !known {
		return
	}

	query := fmt.Sprintf("UPDATE %s SET is_synced = 1 WHERE id = ? AND version = ?", table)
	if _, err := sqlite.DBTXFromContext(ctx, ws.engine.db).ExecContext(ctx, query, entityID, version); err != nil {
		slog.Warn("sync: failed to mark entity as synced", "table", table, "id", entityID, "err", err)
	}
}

// rowVersion reads the version of a row whatever its is_delete; false when the row is gone.
func (ws *workspaceSyncer) rowVersion(ctx context.Context, entityType, entityID string) (int, bool) {
	table, ok := syncedTable(entityType)
	if !ok {
		return 0, false
	}
	var version int
	err := sqlite.DBTXFromContext(ctx, ws.engine.db).QueryRowContext(ctx,
		fmt.Sprintf("SELECT version FROM %s WHERE id = ?", table), entityID).Scan(&version)
	if err != nil {
		if !errors.Is(err, sql.ErrNoRows) {
			slog.Warn("sync: failed to read a row version", "table", table, "id", entityID, "err", err)
		}
		return 0, false
	}
	return version, true
}

func syncedTable(entityType string) (string, bool) {
	switch entityType {
	case "collection", "request", "environment", "variable", "response_example":
		return entityType + "s", true
	}
	return "", false
}

func entityTypeOf(entityID string, entries []*sqlite.SyncEntry) string {
	for _, e := range entries {
		if e.EntityID == entityID {
			return e.EntityType
		}
	}
	return ""
}

func entryByID(entries []*sqlite.SyncEntry, id int64) *sqlite.SyncEntry {
	for _, e := range entries {
		if e.ID == id {
			return e
		}
	}
	return nil
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

// parkEntries keeps entries instead of dropping them: 'failed' hides them from the pending
// queries until their retry window elapses. Returns the IDs the caller may still delete.
func (ws *workspaceSyncer) parkEntries(ctx context.Context, entryIDs, rejected []int64) []int64 {
	if len(rejected) == 0 {
		return entryIDs
	}

	retryAt := time.Now().Add(quotaRetryDelay)
	kept := make(map[int64]bool, len(rejected))
	for _, id := range rejected {
		if err := ws.engine.syncQueue.MarkFailed(ctx, id, retryAt); err != nil {
			slog.Warn("sync: failed to park queue entry", "entry", id, "workspace", ws.localWorkspaceID, "err", err)
			continue
		}
		kept[id] = true
	}

	if len(kept) > 0 {
		ws.scheduleRequeue()
	}

	return slices.DeleteFunc(entryIDs, func(id int64) bool { return kept[id] })
}

func isOversizedPushErr(err error) bool {
	code, msg, ok := grpcStatusOf(err)
	return ok && code == codes.ResourceExhausted && !strings.Contains(msg, planLimitMarker)
}

func (ws *workspaceSyncer) parkOversized(ctx context.Context, entry *sqlite.SyncEntry, size int) bool {
	if entry == nil {
		return false
	}
	if err := ws.engine.syncQueue.MarkParked(ctx, entry.ID); err != nil {
		slog.Warn("sync: failed to park an oversized queue entry", "entry", entry.ID, "workspace", ws.localWorkspaceID, "err", err)
		return false
	}

	slog.Warn("sync: entity too large for the server, parked",
		"workspace", ws.localWorkspaceID, "type", entry.EntityType, "entityId", entry.EntityID, "bytes", size)
	ws.emitThrottled("sync:rejected", &ws.lastRejectEvent, map[string]any{
		"workspaceId": ws.localWorkspaceID,
		"entityType":  entry.EntityType,
		"entityId":    entry.EntityID,
		"reason":      "too_large",
	})
	ws.refreshParked(ctx)
	return true
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

func (ws *workspaceSyncer) refreshParked(ctx context.Context) {
	count, err := ws.engine.syncQueue.CountParked(ctx, ws.localWorkspaceID)
	if err != nil {
		slog.Warn("sync: failed to count parked entries", "workspace", ws.localWorkspaceID, "err", err)
		return
	}
	oversized, err := ws.engine.syncQueue.CountTooLarge(ctx, ws.localWorkspaceID)
	if err != nil {
		slog.Warn("sync: failed to count oversized entries", "workspace", ws.localWorkspaceID, "err", err)
		return
	}

	ws.mu.Lock()
	changed := ws.parked != count || ws.tooLarge != oversized
	ws.parked = count
	ws.tooLarge = oversized
	ws.quotaEpisode = count > 0
	ws.mu.Unlock()

	if changed {
		ws.engine.eventEmitter("sync:parked_changed", map[string]any{
			"workspaceId": ws.localWorkspaceID,
			"parked":      count,
			"tooLarge":    oversized,
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

func (ws *workspaceSyncer) pullCycle(ctx context.Context) error {
	if err := ws.backfillExamples(ctx); err != nil {
		return err
	}
	return ws.pullAll(ctx)
}

// pullAll walks a snapshot page by page when there is no cursor or a walk to resume, then pulls
// incrementally from the snapshot's boundary until the server has nothing more.
func (ws *workspaceSyncer) pullAll(ctx context.Context) error {
	pageToken, err := ws.loadPageToken(ctx)
	if err != nil {
		return err
	}

	snapshotted := false
	restarts := 0
	for {
		snapshot := pageToken != "" || ws.cursor() == 0
		snapshotted = snapshotted || snapshot

		token, err := ws.engine.auth.GetAccessToken(ctx, ws.engine.GRPCClient())
		if err != nil {
			return fmt.Errorf("get access token: %w", err)
		}

		authCtx := ContextWithAuth(ctx, token)
		pullReq := syncv1.PullRequest_builder{
			WorkspaceId:       ws.remoteWorkspaceID,
			LastSyncSeq:       ws.cursor(),
			Limit:             pullBatchSize,
			KnownTypes:        knownTypes,
			PagedSnapshot:     snapshot,
			SnapshotPageToken: pageToken,
		}.Build()

		resp, err := ws.engine.GRPCClient().Sync().Pull(authCtx, pullReq)
		if err != nil {
			if code, _, ok := grpcStatusOf(err); ok && code == codes.InvalidArgument && pageToken != "" && restarts < maxSnapshotRestarts {
				restarts++
				slog.Warn("sync: snapshot page token refused, starting the snapshot over",
					"workspace", ws.localWorkspaceID, "restart", restarts, "err", err)
				if err := ws.restartSnapshot(ctx); err != nil {
					return err
				}
				pageToken = ""
				continue
			}
			return fmt.Errorf("pull rpc: %w", err)
		}

		if resp.GetResyncRequired() {
			// Resyncing here would pull the same snapshot again and could be told to resync again.
			if snapshotted {
				slog.Warn("sync: resync required right after a snapshot, ending the pull", "workspace", ws.localWorkspaceID)
				return nil
			}
			return ws.resync(ctx)
		}

		if err := ws.applyPullPage(ctx, resp, snapshot); err != nil {
			return err
		}

		if next := resp.GetNextSnapshotPageToken(); next != "" {
			pageToken = next
			continue
		}
		if snapshot {
			pageToken = ""
			// An empty workspace leaves the cursor at 0, where the next request would be a snapshot again.
			if resp.GetNextSyncSeq() == 0 {
				return nil
			}
			continue
		}
		if !resp.GetHasMore() {
			return nil
		}
	}
}

func (ws *workspaceSyncer) loadPageToken(ctx context.Context) (string, error) {
	var token string
	err := sqlite.DBTXFromContext(ctx, ws.engine.db).QueryRowContext(ctx,
		`SELECT sync_page_token FROM workspaces WHERE id = ?`, ws.localWorkspaceID).Scan(&token)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("read snapshot page token: %w", err)
	}
	return token, nil
}

func (ws *workspaceSyncer) restartSnapshot(ctx context.Context) error {
	err := sqlite.WithTx(ctx, ws.engine.db, func(txCtx context.Context) error {
		if _, err := sqlite.DBTXFromContext(txCtx, ws.engine.db).ExecContext(txCtx,
			`UPDATE workspaces SET sync_page_token = '', last_sync_seq = 0 WHERE id = ?`, ws.localWorkspaceID); err != nil {
			return err
		}
		return ws.ageWalk(txCtx, walkSnapshot)
	})
	if err != nil {
		return fmt.Errorf("restart snapshot: %w", err)
	}
	ws.setCursor(0)
	return nil
}

// applyPullPage stores a page together with the position it reached: a snapshot page keeps its
// token and leaves the cursor alone, any other page moves the cursor forward.
func (ws *workspaceSyncer) applyPullPage(ctx context.Context, resp *syncv1.PullResponse, snapshot bool) error {
	pageToken := resp.GetNextSnapshotPageToken()
	nextSeq := resp.GetNextSyncSeq()
	advance := pageToken == "" && nextSeq > ws.cursor()
	hasEntities := slices.ContainsFunc(resp.GetChanges(), func(c *syncv1.SyncChange) bool { return c.GetEntity() != nil })
	if !hasEntities && !advance && !snapshot {
		return nil
	}

	reconciled := 0
	err := sqlite.WithInboundTx(ctx, ws.engine.db, func(txCtx context.Context) error {
		if hasEntities {
			if err := ws.applyChanges(txCtx, resp.GetChanges(), snapshot); err != nil {
				return err
			}
		}
		db := sqlite.DBTXFromContext(txCtx, ws.engine.db)
		if snapshot {
			var err error
			if pageToken != "" {
				err = ws.recordSeen(txCtx, walkSnapshot, resp.GetChanges(), snapshotEntityTypes)
			} else {
				reconciled, err = ws.finishWalk(txCtx, walkSnapshot, resp.GetChanges(), snapshotEntityTypes)
			}
			if err != nil {
				return err
			}
			if _, err := db.ExecContext(txCtx,
				`UPDATE workspaces SET sync_page_token = ? WHERE id = ?`, pageToken, ws.localWorkspaceID); err != nil {
				return fmt.Errorf("save snapshot page token: %w", err)
			}
		}
		if advance {
			if _, err := db.ExecContext(txCtx,
				`UPDATE workspaces SET last_sync_seq = ? WHERE id = ?`, nextSeq, ws.localWorkspaceID); err != nil {
				return fmt.Errorf("save cursor: %w", err)
			}
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("apply changes: %w", err)
	}

	if advance {
		ws.setCursor(nextSeq)
	}
	if hasEntities || reconciled > 0 {
		ws.engine.eventEmitter("sync:changed", map[string]any{
			"workspaceId": ws.localWorkspaceID,
		})
	}
	return nil
}

// backfillExamples fetches the examples that clients before 1.2.0 skipped while their cursor moved
// past them. Only an expired session, an update-required entity or cancellation end the cycle.
func (ws *workspaceSyncer) backfillExamples(ctx context.Context) error {
	var err error
	if ws.examplesCap == capSupported {
		err = ws.runBackfill(ctx)
	}
	state, readErr := ws.loadBackfillState(ctx)
	if readErr == nil {
		ws.backfillPending = state.pending
	} else if err == nil {
		err = readErr
	}
	if err == nil {
		return nil
	}
	if ctx.Err() != nil || errors.Is(err, ErrAuthExpired) || errors.Is(err, ErrUpdateRequired) {
		return err
	}
	if isUnauthenticatedErr(err) {
		ws.engine.auth.InvalidateAccessToken()
	}
	slog.Warn("sync: examples backfill failed, going on with the pull", "workspace", ws.localWorkspaceID, "err", err)
	if !isConnectivityErr(err) {
		if err := ws.failBackfillPass(ctx); err != nil {
			slog.Warn("sync: failed to count a failed backfill pass", "workspace", ws.localWorkspaceID, "err", err)
		}
		if state, err := ws.loadBackfillState(ctx); err == nil {
			ws.backfillPending = state.pending
		}
	}
	return nil
}

// isConnectivityErr is a failure that says nothing about the backfill itself: no answer came back.
func isConnectivityErr(err error) bool {
	code, _, ok := grpcStatusOf(err)
	if !ok {
		return false
	}
	switch code {
	case codes.Unavailable, codes.DeadlineExceeded, codes.Canceled, codes.Unauthenticated:
		return true
	}
	return false
}

type backfillState struct {
	pending      bool
	token        string
	cursor       int64
	incomplete   bool
	failedPasses int
}

// loadBackfillState reads the database on every attempt: a resync may have cleared the flag since.
func (ws *workspaceSyncer) loadBackfillState(ctx context.Context) (backfillState, error) {
	var s backfillState
	err := sqlite.DBTXFromContext(ctx, ws.engine.db).QueryRowContext(ctx,
		`SELECT examples_backfill_pending, examples_backfill_token, last_sync_seq, examples_backfill_incomplete, examples_backfill_failed_passes
		 FROM workspaces WHERE id = ?`,
		ws.localWorkspaceID).Scan(&s.pending, &s.token, &s.cursor, &s.incomplete, &s.failedPasses)
	if errors.Is(err, sql.ErrNoRows) {
		return backfillState{}, nil
	}
	if err != nil {
		return backfillState{}, fmt.Errorf("read examples backfill state: %w", err)
	}
	return s, nil
}

func (ws *workspaceSyncer) runBackfill(ctx context.Context) error {
	state, err := ws.loadBackfillState(ctx)
	if err != nil {
		return err
	}
	if !state.pending {
		return nil
	}
	if state.cursor == 0 {
		// The snapshot from cursor 0 brings every example itself.
		return ws.clearBackfill(ctx)
	}

	pageToken := state.token
	restarts := 0
	for {
		token, err := ws.engine.auth.GetAccessToken(ctx, ws.engine.GRPCClient())
		if err != nil {
			return fmt.Errorf("get access token: %w", err)
		}
		pullReq := syncv1.PullRequest_builder{
			WorkspaceId:       ws.remoteWorkspaceID,
			Limit:             pullBatchSize,
			KnownTypes:        knownTypes,
			PagedSnapshot:     true,
			SnapshotPageToken: pageToken,
			EntityTypes:       backfillTypes,
		}.Build()

		resp, err := ws.engine.GRPCClient().Sync().Pull(ContextWithAuth(ctx, token), pullReq)
		if err != nil {
			if code, _, ok := grpcStatusOf(err); ok && code == codes.InvalidArgument && pageToken != "" {
				if err := ws.restartBackfillPass(ctx); err != nil {
					return err
				}
				pageToken = ""
				if restarts == maxSnapshotRestarts {
					return fmt.Errorf("backfill token refused again, the next cycle starts over: %w", err)
				}
				restarts++
				slog.Warn("sync: examples backfill token refused, starting over",
					"workspace", ws.localWorkspaceID, "restart", restarts, "err", err)
				continue
			}
			return fmt.Errorf("backfill pull rpc: %w", err)
		}

		next := resp.GetNextSnapshotPageToken()
		if err := ws.applyBackfillPage(ctx, resp.GetChanges(), next); err != nil {
			return err
		}
		if next == "" {
			return nil
		}
		pageToken = next
	}
}

// applyBackfillPage stores a page with the walk's position and whether the pass left an example
// unstored; the cursor, next_sync_seq and resync_required belong to the regular pull.
func (ws *workspaceSyncer) applyBackfillPage(ctx context.Context, changes []*syncv1.SyncChange, next string) error {
	var applied, reconciled int

	err := sqlite.WithInboundTx(ctx, ws.engine.db, func(txCtx context.Context) error {
		var unapplied int
		var err error
		if applied, unapplied, err = ws.applyBackfillExamples(txCtx, changes); err != nil {
			return err
		}
		if next != "" {
			if err := ws.recordSeen(txCtx, walkBackfill, changes, backfillEntityTypes); err != nil {
				return err
			}
			return ws.saveBackfillPage(txCtx, next, unapplied > 0)
		}
		if reconciled, err = ws.finishWalk(txCtx, walkBackfill, changes, backfillEntityTypes); err != nil {
			return err
		}
		state, err := ws.loadBackfillState(txCtx)
		if err != nil {
			return err
		}
		if !state.incomplete && unapplied == 0 {
			return ws.clearBackfill(txCtx)
		}
		if err := ws.restartBackfillPass(txCtx); err != nil {
			return err
		}
		return ws.failBackfillPass(txCtx)
	})
	if err != nil {
		return fmt.Errorf("apply backfill page: %w", err)
	}

	if applied > 0 || reconciled > 0 {
		ws.engine.eventEmitter("sync:changed", map[string]any{
			"workspaceId": ws.localWorkspaceID,
		})
	}
	return nil
}

// applyBackfillExamples skips examples with a queued local change: that change is pushed next and
// the server settles it. A failed example is skipped like in the regular pull, but counted.
func (ws *workspaceSyncer) applyBackfillExamples(ctx context.Context, changes []*syncv1.SyncChange) (applied, unapplied int, err error) {
	for _, change := range changes {
		entity := change.GetEntity()
		if entity == nil || entity.GetEntityType() != syncv1.EntityType_ENTITY_TYPE_RESPONSE_EXAMPLE {
			continue
		}
		queued, err := ws.exampleQueued(ctx, entity.GetEntityId())
		if err != nil {
			return 0, 0, err
		}
		if queued {
			continue
		}
		if err := ws.applyEntityData(ctx, entity); err != nil {
			if errors.Is(err, ErrUpdateRequired) {
				return 0, 0, err
			}
			slog.Warn("sync: skip backfilled example", "workspace", ws.localWorkspaceID, "exampleId", entity.GetEntityId(), "err", err)
			unapplied++
			continue
		}
		applied++
	}
	return applied, unapplied, nil
}

func (ws *workspaceSyncer) saveBackfillPage(ctx context.Context, token string, unstored bool) error {
	if _, err := sqlite.DBTXFromContext(ctx, ws.engine.db).ExecContext(ctx,
		`UPDATE workspaces SET examples_backfill_token = ?, examples_backfill_incomplete = examples_backfill_incomplete OR ?
		 WHERE id = ?`, token, unstored, ws.localWorkspaceID); err != nil {
		return fmt.Errorf("save examples backfill page: %w", err)
	}
	return nil
}

// restartBackfillPass sends the next attempt back to the first page as a pass of its own.
func (ws *workspaceSyncer) restartBackfillPass(ctx context.Context) error {
	err := sqlite.WithTx(ctx, ws.engine.db, func(txCtx context.Context) error {
		if _, err := sqlite.DBTXFromContext(txCtx, ws.engine.db).ExecContext(txCtx,
			`UPDATE workspaces SET examples_backfill_token = '', examples_backfill_incomplete = 0 WHERE id = ?`,
			ws.localWorkspaceID); err != nil {
			return err
		}
		return ws.ageWalk(txCtx, walkBackfill)
	})
	if err != nil {
		return fmt.Errorf("restart examples backfill: %w", err)
	}
	return nil
}

// failBackfillPass counts a pass that failed and drops the flag once maxBackfillFailedPasses fail in a row.
func (ws *workspaceSyncer) failBackfillPass(ctx context.Context) error {
	return sqlite.WithTx(ctx, ws.engine.db, func(txCtx context.Context) error {
		state, err := ws.loadBackfillState(txCtx)
		if err != nil || !state.pending {
			return err
		}
		passes := state.failedPasses + 1
		if passes >= maxBackfillFailedPasses {
			slog.Warn("sync: examples backfill gave up after failed passes",
				"workspace", ws.localWorkspaceID, "passes", passes)
			return ws.clearBackfill(txCtx)
		}
		if _, err := sqlite.DBTXFromContext(txCtx, ws.engine.db).ExecContext(txCtx,
			`UPDATE workspaces SET examples_backfill_failed_passes = ? WHERE id = ?`, passes, ws.localWorkspaceID); err != nil {
			return fmt.Errorf("count a failed examples backfill pass: %w", err)
		}
		return nil
	})
}

func (ws *workspaceSyncer) exampleQueued(ctx context.Context, exampleID string) (bool, error) {
	var queued bool
	err := sqlite.DBTXFromContext(ctx, ws.engine.db).QueryRowContext(ctx,
		`SELECT EXISTS (SELECT 1 FROM sync_queue WHERE workspace_id = ? AND entity_type = 'response_example' AND entity_id = ?)`,
		ws.localWorkspaceID, exampleID).Scan(&queued)
	if err != nil {
		return false, fmt.Errorf("check queued example: %w", err)
	}
	return queued, nil
}

func (ws *workspaceSyncer) clearBackfill(ctx context.Context) error {
	err := sqlite.WithTx(ctx, ws.engine.db, func(txCtx context.Context) error {
		if _, err := sqlite.DBTXFromContext(txCtx, ws.engine.db).ExecContext(txCtx,
			`UPDATE workspaces SET examples_backfill_pending = 0, examples_backfill_token = '',
			 examples_backfill_incomplete = 0, examples_backfill_failed_passes = 0 WHERE id = ?`,
			ws.localWorkspaceID); err != nil {
			return err
		}
		return ws.forgetWalk(txCtx, walkBackfill)
	})
	if err != nil {
		return fmt.Errorf("finish examples backfill: %w", err)
	}
	return nil
}

// applyChanges leaves an example with a queued local change out of a snapshot page, as the backfill
// does: a server without the capability never got it, and the page would overwrite it.
func (ws *workspaceSyncer) applyChanges(ctx context.Context, changes []*syncv1.SyncChange, snapshot bool) error {
	var deleted bool
	for _, change := range changes {
		entity := change.GetEntity()
		if entity == nil {
			continue
		}
		if snapshot && entity.GetEntityType() == syncv1.EntityType_ENTITY_TYPE_RESPONSE_EXAMPLE {
			queued, err := ws.exampleQueued(ctx, entity.GetEntityId())
			if err != nil {
				return err
			}
			if queued {
				continue
			}
		}
		if err := ws.applyEntityData(ctx, entity); err != nil {
			// A compatibility failure rolls the batch back: skipping would
			// advance the cursor past an entity this build cannot store.
			if errors.Is(err, ErrUpdateRequired) {
				return err
			}
			slog.Warn("sync: skip change apply", "entityId", change.GetEntityId(), "err", err)
			continue
		}
		deleted = deleted || entity.GetIsDeleted()
	}
	if deleted {
		ws.dropParkedForGoneEntities(ctx)
	}
	ws.sweepTokens(ctx)
	return nil
}

func (ws *workspaceSyncer) applyChange(ctx context.Context, change *syncv1.SyncChange) error {
	entity := change.GetEntity()
	if entity == nil {
		return nil
	}
	return ws.applyInbound(ctx, entity)
}

// applyInbound stores one entity that did not come with a page: a stream event or a conflict winner.
func (ws *workspaceSyncer) applyInbound(ctx context.Context, entity *syncv1.SyncEntity) error {
	return sqlite.WithInboundTx(ctx, ws.engine.db, func(txCtx context.Context) error {
		return ws.applyEntity(txCtx, entity)
	})
}

func (ws *workspaceSyncer) applyEntity(ctx context.Context, entity *syncv1.SyncEntity) error {
	if err := ws.applyEntityData(ctx, entity); err != nil {
		return err
	}
	if entity.GetIsDeleted() {
		ws.dropParkedForGoneEntities(ctx)
	}
	return nil
}

func (ws *workspaceSyncer) dropParkedForGoneEntities(ctx context.Context) {
	if _, err := ws.engine.syncQueue.DeleteParkedForMissingEntities(ctx, ws.localWorkspaceID); err != nil {
		slog.Warn("sync: failed to drop parked entries of deleted entities", "workspace", ws.localWorkspaceID, "err", err)
	}
	ws.dropHeldExamplesNotLive(ctx)
}

// dropHeldExamplesNotLive applies the usecase's liveness rule, which SQL alone cannot: the
// chain of ancestor collections has no fixed depth.
func (ws *workspaceSyncer) dropHeldExamplesNotLive(ctx context.Context) {
	held, err := ws.engine.syncQueue.ListHeld(ctx, ws.localWorkspaceID, "response_example")
	if err != nil {
		slog.Warn("sync: failed to list held examples", "workspace", ws.localWorkspaceID, "err", err)
		return
	}

	var gone []int64
	for _, entry := range held {
		live, err := ws.exampleIsLive(ctx, entry.EntityID)
		if err != nil {
			slog.Warn("sync: failed to check a held example", "workspace", ws.localWorkspaceID, "exampleId", entry.EntityID, "err", err)
			continue
		}
		if !live {
			gone = append(gone, entry.ID)
		}
	}

	if err := ws.engine.syncQueue.Delete(ctx, gone); err != nil {
		slog.Warn("sync: failed to drop held examples", "workspace", ws.localWorkspaceID, "err", err)
	}
}

func (ws *workspaceSyncer) exampleIsLive(ctx context.Context, exampleID string) (bool, error) {
	id, err := uuid.Parse(exampleID)
	if err != nil {
		return false, nil
	}
	ex, err := ws.engine.examples.GetByID(ctx, id)
	if err != nil || ex == nil {
		return false, err
	}
	workspaceID, ok, err := example.ChainWorkspace(ctx, ws.engine.requests, ws.engine.collections, ex.RequestID)
	if err != nil {
		return false, err
	}
	return ok && workspaceID == ex.WorkspaceID, nil
}

func (ws *workspaceSyncer) applyEntityData(ctx context.Context, entity *syncv1.SyncEntity) error {
	wsID, err := uuid.Parse(ws.localWorkspaceID)
	if err != nil {
		return fmt.Errorf("applyEntityData: invalid workspace id %q: %w", ws.localWorkspaceID, err)
	}

	switch entity.GetEntityType() {
	case syncv1.EntityType_ENTITY_TYPE_COLLECTION:
		c, err := CollectionFromProto(entity, wsID)
		if err != nil {
			return err
		}
		prev, err := ws.readLocalAuth(ctx, "collections", c.ID.String())
		if err != nil {
			return err
		}
		if err := ws.upsertCollection(ctx, c); err != nil {
			return err
		}
		ws.clearTokenOnAuthChange(ctx,
			entities.AuthOwner{WorkspaceID: wsID, Kind: entities.AuthOwnerKindCollection, ID: c.ID},
			prev, string(c.AuthType), c.AuthData)
		return ws.markInboundSynced(ctx, "collections", c.ID)
	case syncv1.EntityType_ENTITY_TYPE_REQUEST:
		r, err := RequestFromProto(entity)
		if err != nil {
			return err
		}
		prev, err := ws.readLocalAuth(ctx, "requests", r.ID.String())
		if err != nil {
			return err
		}
		if err := ws.upsertRequest(ctx, r, entity.GetRequest().HasDescription()); err != nil {
			return err
		}
		ws.clearTokenOnAuthChange(ctx,
			entities.AuthOwner{WorkspaceID: wsID, Kind: entities.AuthOwnerKindRequest, ID: r.ID},
			prev, string(r.AuthType), r.AuthData)
		return ws.markInboundSynced(ctx, "requests", r.ID)
	case syncv1.EntityType_ENTITY_TYPE_ENVIRONMENT:
		e, err := EnvironmentFromProto(entity, wsID)
		if err != nil {
			return err
		}
		if err := ws.upsertEnvironment(ctx, e); err != nil {
			return err
		}
		return ws.markInboundSynced(ctx, "environments", e.ID)
	case syncv1.EntityType_ENTITY_TYPE_VARIABLE:
		v, err := VariableFromProto(entity)
		if err != nil {
			return err
		}
		if err := ws.upsertVariable(ctx, v); err != nil {
			return err
		}
		return ws.markInboundSynced(ctx, "variables", v.ID)
	case syncv1.EntityType_ENTITY_TYPE_RESPONSE_EXAMPLE:
		return ws.applyExample(ctx, entity, wsID)
	default:
		return fmt.Errorf("unknown entity type: %v", entity.GetEntityType())
	}
}

// markInboundSynced records that the row holds what the server has; a restarted walk deletes only such rows.
func (ws *workspaceSyncer) markInboundSynced(ctx context.Context, table string, id uuid.UUID) error {
	if _, err := sqlite.DBTXFromContext(ctx, ws.engine.db).ExecContext(ctx,
		fmt.Sprintf("UPDATE %s SET is_synced = 1 WHERE id = ?", table), id.String()); err != nil {
		return fmt.Errorf("mark inbound %s synced: %w", table, err)
	}
	return nil
}

func (ws *workspaceSyncer) upsertCollection(ctx context.Context, c *entities.Collection) error {
	exists, err := ws.entityExists(ctx, "collections", c.ID.String())
	if err != nil {
		return err
	}
	if exists {
		return ws.engine.collections.Update(ctx, c)
	}
	return ws.engine.collections.Create(ctx, c)
}

func (ws *workspaceSyncer) upsertRequest(ctx context.Context, r *entities.Request, hasDescription bool) error {
	exists, err := ws.entityExists(ctx, "requests", r.ID.String())
	if err != nil {
		return err
	}
	if !exists {
		return ws.engine.requests.Create(ctx, r)
	}

	if !hasDescription {
		local, err := ws.engine.requests.GetDescriptionByID(ctx, r.ID)
		if err != nil {
			return err
		}
		r.Description = local
	}

	return ws.engine.requests.Update(ctx, r)
}

func (ws *workspaceSyncer) upsertEnvironment(ctx context.Context, e *entities.Environment) error {
	exists, err := ws.entityExists(ctx, "environments", e.ID.String())
	if err != nil {
		return err
	}
	if exists {
		return ws.engine.environments.Update(ctx, e)
	}
	return ws.engine.environments.Create(ctx, e)
}

func (ws *workspaceSyncer) upsertVariable(ctx context.Context, v *entities.Variable) error {
	exists, err := ws.entityExists(ctx, "variables", v.ID.String())
	if err != nil {
		return err
	}
	if exists {
		return ws.engine.variables.Update(ctx, v)
	}
	return ws.engine.variables.Create(ctx, v)
}

// applyExample upserts without checking the request: an example may arrive before it. A tombstone
// only marks the row: the server still sends a payload for it, but one holding just request_id.
func (ws *workspaceSyncer) applyExample(ctx context.Context, entity *syncv1.SyncEntity, workspaceID uuid.UUID) error {
	const funcName = "workspaceSyncer.applyExample"

	db := sqlite.DBTXFromContext(ctx, ws.engine.db)
	if entity.GetIsDeleted() {
		updatedAt := time.Now()
		if entity.HasUpdatedAt() {
			updatedAt = entity.GetUpdatedAt().AsTime()
		}
		if _, err := db.ExecContext(ctx,
			`UPDATE response_examples SET is_delete = 1, version = ?, updated_at = ?, is_synced = 1 WHERE id = ?`,
			entity.GetVersion(), updatedAt.UTC().Format(time.RFC3339), entity.GetEntityId()); err != nil {
			return fmt.Errorf("%s: mark deleted: %w", funcName, err)
		}
		return nil
	}

	ex, err := ResponseExampleFromProto(entity, workspaceID)
	if err != nil {
		return err
	}
	exists, err := ws.entityExists(ctx, "response_examples", ex.ID.String())
	if err != nil {
		return fmt.Errorf("%s: %w", funcName, err)
	}
	if exists {
		err = ws.engine.examples.Update(ctx, ex)
	} else {
		err = ws.engine.examples.Create(ctx, ex)
	}
	if err != nil {
		return err
	}
	if _, err := db.ExecContext(ctx, `UPDATE response_examples SET is_synced = 1 WHERE id = ?`, ex.ID.String()); err != nil {
		return fmt.Errorf("%s: mark synced: %w", funcName, err)
	}
	return nil
}

// entityExists sees soft-deleted rows too: the server's copy overwrites them, a Create would hit the primary key.
func (ws *workspaceSyncer) entityExists(ctx context.Context, table, id string) (bool, error) {
	var count int
	err := sqlite.DBTXFromContext(ctx, ws.engine.db).QueryRowContext(ctx,
		fmt.Sprintf("SELECT COUNT(1) FROM %s WHERE id = ?", table), id).Scan(&count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// localAuthState is the auth of the row an inbound entity replaces.
type localAuthState struct {
	authType string
	authData string
	found    bool
}

// readLocalAuth reads the auth columns of the row an inbound entity is about to
// overwrite, soft-deleted rows included.
func (ws *workspaceSyncer) readLocalAuth(ctx context.Context, table, id string) (localAuthState, error) {
	var prev localAuthState
	err := sqlite.DBTXFromContext(ctx, ws.engine.db).QueryRowContext(ctx,
		fmt.Sprintf("SELECT auth_type, auth_data FROM %s WHERE id = ?", table), id).
		Scan(&prev.authType, &prev.authData)
	if errors.Is(err, sql.ErrNoRows) {
		return localAuthState{}, nil
	}
	if err != nil {
		return localAuthState{}, err
	}
	prev.found = true
	return prev, nil
}

// clearTokenOnAuthChange drops the owner's token whenever an inbound edit touched auth_data,
// cosmetic edits included: a needless re-fetch costs less than serving a foreign token.
func (ws *workspaceSyncer) clearTokenOnAuthChange(ctx context.Context, owner entities.AuthOwner, prev localAuthState, newType, newData string) {
	if !prev.found {
		return
	}
	oauth2 := string(entities.AuthTypeOAuth2)
	if prev.authType != oauth2 && newType != oauth2 {
		return
	}
	if prev.authType == newType && prev.authData == newData {
		return
	}
	if err := ws.engine.tokenCleaner().Clear(ctx, owner); err != nil {
		slog.Warn("sync: clear token after inbound auth change", "entityId", owner.ID, "err", err)
	}
}

// sweepTokens drops token rows whose owner an inbound batch removed, moved or
// rewrote. Housekeeping: a failure is logged, never propagated into the batch.
func (ws *workspaceSyncer) sweepTokens(ctx context.Context) {
	if _, err := ws.engine.tokenCleaner().DeleteOrphans(ctx); err != nil {
		slog.Warn("sync: token sweep failed", "workspaceId", ws.localWorkspaceID, "err", err)
	}
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

// parkForUpdate stops the syncer on a compatibility failure: the peer sent an
// entity this build cannot represent, so retrying repeats the same failure.
func (ws *workspaceSyncer) parkForUpdate(err error) (stop bool) {
	if !errors.Is(err, ErrUpdateRequired) {
		return false
	}
	slog.Warn("sync stopped — the workspace holds entities this version cannot read",
		"workspace", ws.localWorkspaceID, "err", err)
	ws.setState(StateUpdateRequired)
	return true
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

// A plan limit is not a connectivity problem: the server keeps refusing until the
// plan changes, so it waits at the slowest interval.
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
		// A recheck is not a failure: no log, no offline state, no backoff before the new cycle.
		recheck := errors.Is(err, errRecheck)
		backoff := time.Duration(0)
		if !recheck {
			if err != nil {
				if ctx.Err() != nil {
					ws.setState(StateDisconnected)
					return
				}
				slog.Error("sync subscribe error", "workspace", ws.localWorkspaceID, "err", err)
				if ws.handleAuthErr(err) {
					return
				}
				if ws.parkForUpdate(err) {
					return
				}
			}
			backoff = ws.retryAfterSyncErr(err, initialBackoff)
		}

		for {
			select {
			case <-ctx.Done():
				ws.setState(StateDisconnected)
				return
			case <-time.After(backoff):
			}

			ws.setState(StatePushing)
			ws.refreshCapability(ctx, recheck)
			recheck = false
			if err := ws.pushAll(ctx); err != nil {
				slog.Error("sync reconnect push failed", "err", err)
				if ws.handleAuthErr(err) {
					return
				}
				if ws.parkForUpdate(err) {
					return
				}
				backoff = ws.retryAfterSyncErr(err, nextBackoff(backoff))
				continue
			}
			ws.setState(StatePulling)
			if err := ws.pullCycle(ctx); err != nil {
				slog.Error("sync reconnect pull failed", "err", err)
				if ws.handleAuthErr(err) {
					return
				}
				if ws.parkForUpdate(err) {
					return
				}
				backoff = nextBackoff(backoff)
				ws.setState(StateOffline)
				continue
			}
			ws.setState(StateSubscribing)
			break // re-enter subscribe
		}
	}
}

// nextBackoff doubles the wait; a recheck that failed starts from the initial one.
func nextBackoff(d time.Duration) time.Duration {
	return min(max(d*2, initialBackoff), maxBackoff)
}

// subscribe returns when the stream ends (disconnect, error, ctx cancel).
func (ws *workspaceSyncer) subscribe(ctx context.Context) error {
	token, err := ws.engine.auth.GetAccessToken(ctx, ws.engine.GRPCClient())
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
		KnownTypes:  knownTypes,
	}.Build()

	stream, err := ws.engine.GRPCClient().Sync().Subscribe(authCtx, subReq)
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

	var recheck <-chan time.Time
	if ws.needsRecheck() {
		recheckTimer := time.NewTimer(capabilityRecheckInterval)
		defer recheckTimer.Stop()
		recheck = recheckTimer.C
	}

	for {
		select {
		case <-recheck:
			return errRecheck

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
					if errors.Is(err, ErrUpdateRequired) {
						return fmt.Errorf("apply subscribe change: %w", err)
					}
					slog.Warn("sync: apply subscribe change failed", "err", err)
				} else {
					ws.sweepTokens(ctx)
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

	lost, err := ws.engine.clearOutbox(ctx, ws.localWorkspaceID)
	if err != nil {
		return fmt.Errorf("clear outbox: %w", err)
	}
	ws.setCursor(0)
	if lost > 0 {
		ws.engine.eventEmitter("sync:data_lost", map[string]any{
			"workspaceId": ws.localWorkspaceID,
			"count":       lost,
		})
	}

	// Before the snapshot: its pages would otherwise land on the edits just queued again.
	if err := ws.pushAll(ctx); err != nil {
		return fmt.Errorf("resync push: %w", err)
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
		ws.refreshCapability(ctx, false)
		if err := ws.pushAll(ctx); err != nil {
			if ws.handleAuthErr(err) {
				return
			}
			if ws.parkForUpdate(err) {
				return
			}
			backoff = ws.retryAfterSyncErr(err, min(backoff*2, maxBackoff))
			continue
		}
		ws.setState(StatePulling)
		if err := ws.pullCycle(ctx); err != nil {
			if ws.handleAuthErr(err) {
				return
			}
			if ws.parkForUpdate(err) {
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

// entityTypePriority orders pushes parent-first: collections, environments, requests, variables, examples.
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
	case syncv1.EntityType_ENTITY_TYPE_RESPONSE_EXAMPLE:
		return 4
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
