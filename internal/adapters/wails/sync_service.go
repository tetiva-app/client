package wails

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"

	workspacev1 "github.com/tetiva-app/proto/go/gophercourier/workspace/v1"

	"github.com/tetiva-app/client/internal/adapters/wails/dto"
	"github.com/tetiva-app/client/internal/domain/usecase/workspace"
	"github.com/tetiva-app/client/internal/infrastructure/repository/sqlite"
	syncsvc "github.com/tetiva-app/client/internal/infrastructure/sync"
)

// sessionRPCTimeout bounds the device-management calls: the UI waits on them.
const sessionRPCTimeout = 10 * time.Second

// SyncService exposes sync operations to the Wails frontend.
type SyncService struct {
	engine     *syncsvc.SyncEngine
	auth       *syncsvc.SyncAuthManager
	configRepo sqlite.SyncConfigRepository
	queueRepo  sqlite.SyncQueueRepository
	db         *sql.DB
	wsUC       workspace.Usecase

	// clientMu guards grpcClient: the status poll, the sync modal and the startup
	// hook all reach for it, and a missing one is dialed on demand.
	clientMu   sync.Mutex
	grpcClient *syncsvc.GRPCClient

	// newClient is swapped in tests for a client built on stubs.
	newClient func(serverURL string) (*syncsvc.GRPCClient, error)

	awaitingMu sync.Mutex
	// Atomic so the 5-second status poll never waits behind enableSync's RPCs.
	awaitingVerification atomic.Bool
}

// NewSyncService creates a new SyncService.
func NewSyncService(
	engine *syncsvc.SyncEngine,
	auth *syncsvc.SyncAuthManager,
	configRepo sqlite.SyncConfigRepository,
	queueRepo sqlite.SyncQueueRepository,
	db *sql.DB,
	wsUC workspace.Usecase,
) *SyncService {
	return &SyncService{
		engine:     engine,
		auth:       auth,
		configRepo: configRepo,
		queueRepo:  queueRepo,
		db:         db,
		wsUC:       wsUC,
		newClient:  syncsvc.NewGRPCClient,
	}
}

func (s *SyncService) dial(serverURL string) (*syncsvc.GRPCClient, error) {
	return s.newClient(serverURL)
}

func (s *SyncService) client() *syncsvc.GRPCClient {
	s.clientMu.Lock()
	defer s.clientMu.Unlock()
	return s.grpcClient
}

func (s *SyncService) setClient(client *syncsvc.GRPCClient) {
	s.clientMu.Lock()
	defer s.clientMu.Unlock()
	s.grpcClient = client
}

// ensureClient returns the sync client, dialing one from the saved config when a
// failed startup left the service without it — otherwise Retry in the UI could
// never recover. A disabled or serverless config stays ErrNotConnected.
func (s *SyncService) ensureClient(ctx context.Context) (*syncsvc.GRPCClient, error) {
	s.clientMu.Lock()
	defer s.clientMu.Unlock()

	if s.grpcClient != nil {
		return s.grpcClient, nil
	}
	if s.configRepo == nil {
		return nil, ErrNotConnected
	}

	cfg, err := s.configRepo.Get(ctx)
	if err != nil {
		return nil, fmt.Errorf("read sync config: %w", err)
	}
	if cfg == nil || !cfg.Enabled || cfg.ServerURL == "" {
		return nil, ErrNotConnected
	}

	client, err := s.dial(cfg.ServerURL)
	if err != nil {
		return nil, fmt.Errorf("reconnect grpc: %w", err)
	}
	s.auth.SetClientID(cfg.ClientID)
	s.grpcClient = client
	return client, nil
}

// SetEventEmitter wires the Wails event system to the sync engine.
func (s *SyncService) SetEventEmitter(fn syncsvc.EventEmitter) {
	s.engine.SetEventEmitter(fn)
}

// ResumeOnStartup reconnects to the sync server and resumes sync
// for all linked workspaces if credentials are saved.
func (s *SyncService) ResumeOnStartup(ctx context.Context) error {
	// A crash mid-push leaves rows stuck in 'sending'; the push worker only
	// reads 'pending', so reset them here or they never retry. Non-fatal.
	if s.queueRepo != nil {
		if err := s.queueRepo.ResetSending(ctx); err != nil {
			slog.WarnContext(ctx, "sync: ResetSending on startup failed; continuing",
				"err", err)
		}
	}

	cfg, err := s.configRepo.Get(ctx)
	if err != nil || cfg == nil || !cfg.Enabled || cfg.ServerURL == "" {
		return nil // sync not configured
	}

	s.migrateLegacyServerURL(ctx, cfg)

	client, err := s.dial(cfg.ServerURL)
	if err != nil {
		return fmt.Errorf("reconnect grpc: %w", err)
	}

	// Set clientID so GetAccessToken can refresh via keyring token.
	s.auth.SetClientID(cfg.ClientID)

	// Keep the client before auth: the waiting screen, the device list and the
	// engine all go through it, including on a startup that never reaches the server.
	s.setClient(client)

	if _, err := s.auth.GetAccessToken(ctx, client); err != nil {
		if errors.Is(err, syncsvc.ErrAuthExpired) {
			s.setClient(nil)
			_ = client.Close()
			return fmt.Errorf("restore auth: %w", err)
		}
		// A server that is down at launch is transient: start the syncers offline and
		// let their backoff loop refresh the token once the server answers again.
		slog.WarnContext(ctx, "sync: auth restore failed on startup; starting offline", "err", err)
		s.startLinkedWorkspaces(ctx)
		return nil
	}

	_, verified, err := s.auth.GetMe(ctx, client)
	switch {
	case err != nil:
		// Fail-open: the gate exists to walk a fresh signup to confirmation,
		// not to break offline startup for accounts that are already confirmed.
		slog.Warn("sync: verification check failed on startup; starting sync anyway", "err", err)
	case !verified:
		s.awaitingVerification.Store(true)
		return nil
	}

	if err := s.enableSync(ctx, client); err != nil {
		slog.Warn("sync: failed to sync remote workspaces on startup", "err", err)
	}

	return nil
}

// Cloud endpoint migration: configs written before the tetiva.app move point
// at the old host. Both hosts serve the same backend, so this is safe.
const (
	legacyDefaultServer  = "gophercourier-sync.yudinsv-hub.ru:443"
	currentDefaultServer = "api.tetiva.app:443"
)

func (s *SyncService) migrateLegacyServerURL(ctx context.Context, cfg *sqlite.SyncConfig) {
	if cfg.ServerURL != legacyDefaultServer {
		return
	}
	cfg.ServerURL = currentDefaultServer
	if err := s.configRepo.Update(ctx, cfg); err != nil {
		cfg.ServerURL = legacyDefaultServer // keep a consistent in-memory view
		slog.Warn("sync: legacy server url migration failed", "err", err)
	}
}

// Connect authenticates with the sync server and enables sync.
func (s *SyncService) Connect(req dto.ConnectRequest) Result[dto.AuthStateResult] {
	ctx := context.Background()

	slog.Info("sync: Connect called", "serverURL", req.ServerURL, "email", req.Email)

	client, err := s.dial(req.ServerURL)
	if err != nil {
		return Err[dto.AuthStateResult](fmt.Errorf("connect: %w", err))
	}

	auth, err := s.auth.Login(ctx, client, req.Email, req.Password)
	if err != nil {
		_ = client.Close()
		return Err[dto.AuthStateResult](fmt.Errorf("login: %w", err))
	}

	state, err := s.completeAuth(ctx, client, req.ServerURL, auth)
	if err != nil {
		_ = client.Close()
		return Err[dto.AuthStateResult](err)
	}
	return OK(state)
}

// Register creates a new account on the sync server and enables sync.
func (s *SyncService) Register(req dto.RegisterRequest) Result[dto.AuthStateResult] {
	ctx := context.Background()

	client, err := s.dial(req.ServerURL)
	if err != nil {
		return Err[dto.AuthStateResult](fmt.Errorf("connect: %w", err))
	}

	auth, err := s.auth.Register(ctx, client, req.Email, req.Password, req.Name, req.Locale)
	if err != nil {
		_ = client.Close()
		return Err[dto.AuthStateResult](fmt.Errorf("register: %w", err))
	}

	state, err := s.completeAuth(ctx, client, req.ServerURL, auth)
	if err != nil {
		_ = client.Close()
		return Err[dto.AuthStateResult](err)
	}
	return OK(state)
}

// completeAuth records the connected account — Enabled means connected, not syncing.
func (s *SyncService) completeAuth(
	ctx context.Context,
	client *syncsvc.GRPCClient,
	serverURL string,
	auth *syncsvc.AuthResult,
) (dto.AuthStateResult, error) {
	cfg, err := s.configRepo.GetOrCreate(ctx)
	if err != nil {
		return dto.AuthStateResult{}, err
	}
	cfg.ServerURL = serverURL
	cfg.Enabled = true
	if err := s.configRepo.Update(ctx, cfg); err != nil {
		return dto.AuthStateResult{}, err
	}

	state := dto.AuthStateResult{
		Email:                     auth.Email,
		RequiresEmailVerification: auth.RequiresEmailVerification,
	}

	s.setClient(client)

	if auth.RequiresEmailVerification {
		s.awaitingVerification.Store(true)
		return state, nil
	}

	if err := s.enableSync(ctx, client); err != nil {
		slog.Warn("sync: failed to sync remote workspaces", "err", err)
	}
	return state, nil
}

// enableSync wires the authenticated client into the engine and links workspaces.
// awaitingMu serialises the verification poll and the manual "I confirmed" button.
func (s *SyncService) enableSync(ctx context.Context, client *syncsvc.GRPCClient) error {
	s.awaitingMu.Lock()
	defer s.awaitingMu.Unlock()

	s.awaitingVerification.Store(false)
	s.engine.SetGRPCClient(client)

	err := s.syncRemoteWorkspaces(ctx)
	s.linkActiveWorkspace(ctx)
	return err
}

// startLinkedWorkspaces starts the syncers for workspaces that already carry a
// remote mapping. The offline counterpart of enableSync: no RPC, so an
// unreachable server at startup costs discovery, not sync.
func (s *SyncService) startLinkedWorkspaces(ctx context.Context) {
	s.engine.SetGRPCClient(s.client())

	rows, err := s.db.QueryContext(ctx,
		`SELECT id, remote_workspace_id, last_sync_seq FROM workspaces
		 WHERE is_delete = 0 AND remote_workspace_id IS NOT NULL AND remote_workspace_id != ''`)
	if err != nil {
		slog.Warn("sync: reading linked workspaces failed", "err", err)
		return
	}

	type link struct {
		localID  string
		remoteID string
		lastSeq  int64
	}
	var links []link
	for rows.Next() {
		var l link
		if err := rows.Scan(&l.localID, &l.remoteID, &l.lastSeq); err != nil {
			slog.Warn("sync: reading linked workspace failed", "err", err)
			break
		}
		links = append(links, l)
	}
	rowsErr := rows.Err()
	_ = rows.Close()
	if rowsErr != nil {
		slog.Warn("sync: reading linked workspaces failed", "err", rowsErr)
		return
	}

	for _, l := range links {
		s.engine.StartWorkspace(l.localID, l.remoteID, l.lastSeq)
	}
	slog.Info("sync: started offline", "workspaces", len(links))
}

// GetMe reports the connected account. Crossing into verified is the moment
// sync may finally start, so the engine is wired here.
func (s *SyncService) GetMe() Result[dto.MeResult] {
	ctx := context.Background()

	client, err := s.ensureClient(ctx)
	if err != nil {
		return Err[dto.MeResult](fmt.Errorf("getMe: %w", err))
	}

	email, verified, err := s.auth.GetMe(ctx, client)
	if err != nil {
		return Err[dto.MeResult](fmt.Errorf("getMe: %w", err))
	}

	// Only the caller that wins the swap enables sync — the poll and the
	// manual button can observe the transition at the same time.
	if verified && s.awaitingVerification.CompareAndSwap(true, false) {
		if err := s.enableSync(ctx, client); err != nil {
			slog.Warn("sync: failed to sync remote workspaces after verification", "err", err)
		}
	}

	return OK(dto.MeResult{Email: email, EmailVerified: verified})
}

// ResendVerification asks the server to send the confirmation email again.
func (s *SyncService) ResendVerification() Result[Empty] {
	ctx := context.Background()

	client, err := s.ensureClient(ctx)
	if err != nil {
		return Err[Empty](fmt.Errorf("resendVerification: %w", err))
	}

	if err := s.auth.ResendVerification(ctx, client); err != nil {
		return Err[Empty](fmt.Errorf("resendVerification: %w", err))
	}

	return OK(Empty{})
}

func activeWorkspaceNeedingLink(ctx context.Context, db *sql.DB) (id, name string, ok bool) {
	var wsID, wsName string
	var remote sql.NullString
	err := db.QueryRowContext(ctx,
		`SELECT id, name, remote_workspace_id FROM workspaces
		 WHERE is_active = 1 AND is_delete = 0 LIMIT 1`).Scan(&wsID, &wsName, &remote)
	if err != nil {
		return "", "", false
	}
	if remote.Valid && remote.String != "" {
		return "", "", false
	}
	return wsID, wsName, true
}

func (s *SyncService) createRemoteWorkspace(ctx context.Context, name string) (string, error) {
	client := s.client()
	if client == nil {
		return "", fmt.Errorf("not connected")
	}
	orgID := s.auth.GetActiveOrgID()
	if orgID == "" {
		return "", fmt.Errorf("no active org")
	}
	token, err := s.auth.GetAccessToken(ctx, client)
	if err != nil {
		return "", fmt.Errorf("get access token: %w", err)
	}
	authCtx := syncsvc.ContextWithAuth(ctx, token)
	resp, err := client.Workspace().Create(authCtx, workspacev1.CreateRequest_builder{
		OrgId: orgID,
		Name:  name,
	}.Build())
	if err != nil {
		return "", fmt.Errorf("create workspace rpc: %w", err)
	}
	return resp.GetWorkspace().GetId(), nil
}

// linkActiveWorkspace pushes the active local workspace to the server when it
// has no remote mapping, so the workspace on screen actually syncs after connect.
func (s *SyncService) linkActiveWorkspace(ctx context.Context) {
	localID, name, ok := activeWorkspaceNeedingLink(ctx, s.db)
	if !ok {
		return
	}
	remoteID, err := s.createRemoteWorkspace(ctx, name)
	if err != nil {
		slog.Warn("sync: auto-link active workspace failed", "err", err)
		return
	}
	if _, err := s.db.ExecContext(ctx,
		`UPDATE workspaces SET remote_workspace_id = ? WHERE id = ?`, remoteID, localID); err != nil {
		slog.Warn("sync: auto-link mapping update failed", "err", err)
		return
	}
	s.engine.StartWorkspace(localID, remoteID, 0)
	slog.Info("sync: active workspace auto-linked", "local_id", localID, "remote_id", remoteID)
}

// CreateRemoteWorkspace creates a workspace on the sync server and
// links it locally for sync.
//
// TODO: server CreateRequest requires org_id; re-enable once the client carries an active org.
func (s *SyncService) CreateRemoteWorkspace(req dto.CreateWorkspaceRequest) Result[dto.WorkspaceResponse] {
	_ = context.Background()
	if s.client() == nil {
		return Err[dto.WorkspaceResponse](fmt.Errorf("not connected to sync server"))
	}
	_ = req
	return Err[dto.WorkspaceResponse](fmt.Errorf("CreateRemoteWorkspace: creating workspaces from the app is not supported yet"))
}

// Disconnect stops all sync goroutines but preserves tokens for reconnection.
func (s *SyncService) Disconnect() Result[Empty] {
	s.engine.StopAll()
	return OK(Empty{})
}

// Logout stops all sync goroutines and clears authentication tokens.
func (s *SyncService) Logout() Result[Empty] {
	ctx := context.Background()

	s.engine.StopAll()
	s.awaitingVerification.Store(false)

	if err := s.auth.Logout(ctx, s.client()); err != nil {
		return Err[Empty](fmt.Errorf("logout: %w", err))
	}

	return OK(Empty{})
}

// ListSessions returns the devices signed in to the connected account.
func (s *SyncService) ListSessions() Result[[]dto.SessionInfo] {
	ctx, cancel := context.WithTimeout(context.Background(), sessionRPCTimeout)
	defer cancel()

	client, err := s.ensureClient(ctx)
	if err != nil {
		return Err[[]dto.SessionInfo](fmt.Errorf("listSessions: %w", err))
	}

	sessions, err := s.auth.Me(ctx, client)
	if err != nil {
		return Err[[]dto.SessionInfo](fmt.Errorf("listSessions: %w", err))
	}

	out := make([]dto.SessionInfo, 0, len(sessions))
	for _, session := range sessions {
		out = append(out, dto.SessionInfo{
			ID:         session.ID,
			ClientID:   session.ClientID,
			UserAgent:  session.UserAgent,
			IP:         session.IP,
			LastUsedAt: formatSessionTime(session.LastUsedAt),
			IsCurrent:  session.IsCurrent,
		})
	}
	return OK(out)
}

func formatSessionTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format(time.RFC3339)
}

// RevokeSession signs one other device out of the account.
func (s *SyncService) RevokeSession(req dto.RevokeSessionRequest) Result[Empty] {
	ctx, cancel := context.WithTimeout(context.Background(), sessionRPCTimeout)
	defer cancel()

	client, err := s.ensureClient(ctx)
	if err != nil {
		return Err[Empty](fmt.Errorf("revokeSession: %w", err))
	}

	if err := s.auth.RevokeSession(ctx, client, req.SessionID); err != nil {
		return Err[Empty](fmt.Errorf("revokeSession: %w", err))
	}
	return OK(Empty{})
}

// LogoutAll signs every other device out, keeping this one connected.
func (s *SyncService) LogoutAll() Result[dto.LogoutAllResult] {
	ctx, cancel := context.WithTimeout(context.Background(), sessionRPCTimeout)
	defer cancel()

	client, err := s.ensureClient(ctx)
	if err != nil {
		return Err[dto.LogoutAllResult](fmt.Errorf("logoutAll: %w", err))
	}

	revoked, err := s.auth.LogoutAll(ctx, client)
	if err != nil {
		return Err[dto.LogoutAllResult](fmt.Errorf("logoutAll: %w", err))
	}
	return OK(dto.LogoutAllResult{RevokedCount: revoked})
}

// GetStatus returns the current sync status: state, pending count and how many of
// those the plan quota keeps parked.
func (s *SyncService) GetStatus() Result[dto.SyncStatusResponse] {
	ctx := context.Background()

	cfg, err := s.configRepo.Get(ctx)
	if err != nil {
		return Err[dto.SyncStatusResponse](err)
	}

	resp := dto.SyncStatusResponse{AwaitingVerification: s.awaitingVerification.Load()}
	if cfg != nil {
		resp.Enabled = cfg.Enabled
		resp.ServerURL = cfg.ServerURL
		resp.UserEmail = cfg.UserEmail
	}

	// Use the active workspace as the representative state.
	var activeWorkspaceID string
	row := s.db.QueryRowContext(ctx, `SELECT id FROM workspaces WHERE is_active = 1 AND is_delete = 0 LIMIT 1`)
	_ = row.Scan(&activeWorkspaceID)

	if activeWorkspaceID != "" {
		resp.State = string(s.engine.GetWorkspaceState(activeWorkspaceID))

		count, err := s.engine.GetPendingCount(ctx, activeWorkspaceID)
		if err == nil {
			resp.Pending = count
		}

		parked, err := s.engine.GetParkedCount(ctx, activeWorkspaceID)
		if err == nil {
			resp.Parked = parked
		}
	} else {
		resp.State = string(syncsvc.StateDisconnected)
	}

	return OK(resp)
}

// LinkWorkspace maps a local workspace to a remote one and starts sync.
func (s *SyncService) LinkWorkspace(req dto.LinkWorkspaceRequest) Result[Empty] {
	ctx := context.Background()

	// last_sync_seq is a position in the previous remote's change log, so a
	// re-link to a different remote must start from scratch.
	_, err := s.db.ExecContext(ctx,
		`UPDATE workspaces
		 SET remote_workspace_id = ?,
		     last_sync_seq = CASE WHEN remote_workspace_id IS ? THEN last_sync_seq ELSE 0 END
		 WHERE id = ?`,
		req.RemoteWorkspaceID, req.RemoteWorkspaceID, req.LocalWorkspaceID)
	if err != nil {
		return Err[Empty](err)
	}

	var lastSyncSeq int64
	_ = s.db.QueryRowContext(ctx,
		`SELECT last_sync_seq FROM workspaces WHERE id = ?`,
		req.LocalWorkspaceID).Scan(&lastSyncSeq)

	s.engine.StartWorkspace(req.LocalWorkspaceID, req.RemoteWorkspaceID, lastSyncSeq)

	return OK(Empty{})
}

// UnlinkWorkspace stops sync for a workspace and clears the remote mapping.
func (s *SyncService) UnlinkWorkspace(req dto.UnlinkWorkspaceRequest) Result[Empty] {
	ctx := context.Background()

	s.engine.StopWorkspace(req.LocalWorkspaceID)

	_, err := s.db.ExecContext(ctx,
		`UPDATE workspaces SET remote_workspace_id = NULL WHERE id = ?`,
		req.LocalWorkspaceID)
	if err != nil {
		return Err[Empty](err)
	}

	return OK(Empty{})
}

// ListRemoteWorkspaces fetches the workspaces visible to the user under their
// active org. Used by the connect-modal to show available remotes for linking.
func (s *SyncService) ListRemoteWorkspaces() Result[[]dto.RemoteWorkspace] {
	ctx := context.Background()
	if s.client() == nil {
		return Err[[]dto.RemoteWorkspace](fmt.Errorf("not connected to sync server"))
	}

	remotes, err := s.fetchRemoteWorkspaces(ctx)
	if err != nil {
		return Err[[]dto.RemoteWorkspace](err)
	}

	out := make([]dto.RemoteWorkspace, 0, len(remotes))
	for _, w := range remotes {
		out = append(out, dto.RemoteWorkspace{ID: w.GetId(), Name: w.GetName()})
	}
	return OK(out)
}

// syncRemoteWorkspaces mirrors the org's remote workspaces locally and starts
// sync for each. Idempotent — safe to call on every Connect/Resume.
func (s *SyncService) syncRemoteWorkspaces(ctx context.Context) error {
	const funcName = "SyncService.syncRemoteWorkspaces"
	if s.client() == nil {
		return fmt.Errorf("%s: not connected", funcName)
	}

	slog.Info("sync: syncRemoteWorkspaces start")
	remotes, err := s.fetchRemoteWorkspaces(ctx)
	if err != nil {
		return fmt.Errorf("%s: %w", funcName, err)
	}
	slog.Info("sync: fetched remote workspaces", "count", len(remotes))
	s.dropForeignWorkspaceMappings(ctx, remotes)

	started := 0
	for _, w := range remotes {
		if w.GetIsDeleted() {
			continue
		}
		localID, lastSeq, err := s.upsertLocalForRemote(ctx, w)
		if err != nil {
			slog.Warn("sync: upsert workspace failed", "remote_id", w.GetId(), "err", err)
			continue
		}
		slog.Info("sync: starting workspace", "local_id", localID, "remote_id", w.GetId(), "last_seq", lastSeq, "name", w.GetName())
		s.engine.StartWorkspace(localID, w.GetId(), lastSeq)
		started++
	}
	slog.Info("sync: syncRemoteWorkspaces done", "started", started)
	return nil
}

// dropForeignWorkspaceMappings clears remote IDs the current org does not own;
// remotes must come from a successful listing or everything gets unlinked.
func (s *SyncService) dropForeignWorkspaceMappings(ctx context.Context, remotes []*workspacev1.Workspace) {
	owned := make(map[string]struct{}, len(remotes))
	for _, w := range remotes {
		owned[w.GetId()] = struct{}{}
	}

	rows, err := s.db.QueryContext(ctx,
		`SELECT id, remote_workspace_id FROM workspaces
		 WHERE is_delete = 0 AND remote_workspace_id IS NOT NULL AND remote_workspace_id != ''`)
	if err != nil {
		slog.Warn("sync: reading workspace mappings failed", "err", err)
		return
	}

	var stale []string
	for rows.Next() {
		var localID, remoteID string
		if err := rows.Scan(&localID, &remoteID); err != nil {
			slog.Warn("sync: reading workspace mapping failed", "err", err)
			break
		}
		if _, ok := owned[remoteID]; !ok {
			stale = append(stale, localID)
		}
	}
	rowsErr := rows.Err()
	_ = rows.Close()
	if rowsErr != nil {
		slog.Warn("sync: reading workspace mappings failed", "err", rowsErr)
		return
	}

	for _, localID := range stale {
		s.engine.StopWorkspace(localID)
		if _, err := s.db.ExecContext(ctx,
			`UPDATE workspaces SET remote_workspace_id = NULL WHERE id = ?`, localID); err != nil {
			slog.Warn("sync: unlinking foreign workspace failed", "local_id", localID, "err", err)
			continue
		}
		slog.Info("sync: unlinked workspace owned by another account", "local_id", localID)
	}
}

// fetchRemoteWorkspaces paginates ListByOrg under the active org captured by
// the auth manager from the Login/Refresh response.
func (s *SyncService) fetchRemoteWorkspaces(ctx context.Context) ([]*workspacev1.Workspace, error) {
	client := s.client()
	if client == nil {
		return nil, ErrNotConnected
	}

	activeOrg := s.auth.GetActiveOrgID()
	if activeOrg == "" {
		return nil, fmt.Errorf("no active org — auth must complete before workspace discovery")
	}

	slog.Info("sync: fetchRemoteWorkspaces: getting access token", "active_org", activeOrg)
	token, err := s.auth.GetAccessToken(ctx, client)
	if err != nil {
		return nil, fmt.Errorf("get access token: %w", err)
	}
	authCtx := syncsvc.ContextWithAuth(ctx, token)
	slog.Info("sync: fetchRemoteWorkspaces: calling ListByOrg", "active_org", activeOrg)

	var all []*workspacev1.Workspace
	cursor := ""
	for {
		req := workspacev1.ListByOrgRequest_builder{OrgId: activeOrg, Cursor: cursor}.Build()
		resp, err := client.Workspace().ListByOrg(authCtx, req)
		if err != nil {
			return nil, fmt.Errorf("ListByOrg: %w", err)
		}
		all = append(all, resp.GetWorkspaces()...)
		cursor = resp.GetNextCursor()
		if cursor == "" {
			break
		}
	}
	slog.Info("sync: fetchRemoteWorkspaces: ListByOrg done", "count", len(all))
	return all, nil
}

// upsertLocalForRemote returns the local workspace ID and its last_sync_seq,
// inserting a local mirror row for remotes not linked yet.
func (s *SyncService) upsertLocalForRemote(ctx context.Context, w *workspacev1.Workspace) (string, int64, error) {
	var localID string
	var lastSeq int64
	err := s.db.QueryRowContext(ctx,
		`SELECT id, last_sync_seq FROM workspaces WHERE remote_workspace_id = ? AND is_delete = 0`,
		w.GetId()).Scan(&localID, &lastSeq)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		localID = uuid.NewString()
		now := time.Now().UTC().Format("2006-01-02 15:04:05")
		_, execErr := s.db.ExecContext(ctx, `
			INSERT INTO workspaces
				(id, name, version, is_delete, created_by, created_at, updated_by, updated_at,
				 remote_workspace_id, last_sync_seq, is_active)
			VALUES (?, ?, 1, 0, 'sync', ?, 'sync', ?, ?, 0, 0)
		`, localID, w.GetName(), now, now, w.GetId())
		if execErr != nil {
			return "", 0, fmt.Errorf("insert workspace: %w", execErr)
		}
		return localID, 0, nil
	case err != nil:
		return "", 0, fmt.Errorf("query workspace: %w", err)
	}
	return localID, lastSeq, nil
}
