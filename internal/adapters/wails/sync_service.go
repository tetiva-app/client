package wails

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"

	workspacev1 "github.com/tetiva-app/proto/go/gophercourier/workspace/v1"
	"github.com/tetiva-app/client/internal/adapters/wails/dto"
	"github.com/tetiva-app/client/internal/domain/usecase/workspace"
	"github.com/tetiva-app/client/internal/infrastructure/repository/sqlite"
	syncsvc "github.com/tetiva-app/client/internal/infrastructure/sync"
)

// SyncService exposes sync operations to the Wails frontend.
type SyncService struct {
	engine     *syncsvc.SyncEngine
	auth       *syncsvc.SyncAuthManager
	configRepo sqlite.SyncConfigRepository
	queueRepo  sqlite.SyncQueueRepository
	db         *sql.DB
	grpcClient *syncsvc.GRPCClient
	wsUC       workspace.Usecase
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
	}
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

	client, err := syncsvc.NewGRPCClient(cfg.ServerURL)
	if err != nil {
		return fmt.Errorf("reconnect grpc: %w", err)
	}

	// Set clientID so GetAccessToken can refresh via keyring token.
	s.auth.SetClientID(cfg.ClientID)

	s.grpcClient = client
	s.engine.SetGRPCClient(client)

	if _, err := s.auth.GetAccessToken(ctx, client); err != nil {
		_ = client.Close()
		s.grpcClient = nil
		return fmt.Errorf("restore auth: %w", err)
	}

	if err := s.syncRemoteWorkspaces(ctx); err != nil {
		slog.Warn("sync: failed to sync remote workspaces on startup", "err", err)
	}
	s.linkActiveWorkspace(ctx)

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
func (s *SyncService) Connect(req dto.ConnectRequest) Result[Empty] {
	ctx := context.Background()

	slog.Info("sync: Connect called", "serverURL", req.ServerURL, "email", req.Email)

	client, err := syncsvc.NewGRPCClient(req.ServerURL)
	if err != nil {
		return Err[Empty](fmt.Errorf("connect: %w", err))
	}

	if err := s.auth.Login(ctx, client, req.Email, req.Password); err != nil {
		_ = client.Close()
		return Err[Empty](fmt.Errorf("login: %w", err))
	}

	cfg, err := s.configRepo.GetOrCreate(ctx)
	if err != nil {
		_ = client.Close()
		return Err[Empty](err)
	}
	cfg.ServerURL = req.ServerURL
	cfg.Enabled = true
	if err := s.configRepo.Update(ctx, cfg); err != nil {
		_ = client.Close()
		return Err[Empty](err)
	}

	s.grpcClient = client
	s.engine.SetGRPCClient(client)

	if err := s.syncRemoteWorkspaces(ctx); err != nil {
		slog.Warn("sync: failed to sync remote workspaces", "err", err)
	}
	s.linkActiveWorkspace(ctx)

	return OK(Empty{})
}

// Register creates a new account on the sync server and enables sync.
func (s *SyncService) Register(req dto.RegisterRequest) Result[Empty] {
	ctx := context.Background()

	client, err := syncsvc.NewGRPCClient(req.ServerURL)
	if err != nil {
		return Err[Empty](fmt.Errorf("connect: %w", err))
	}

	if err := s.auth.Register(ctx, client, req.Email, req.Password, req.Name, req.Locale); err != nil {
		_ = client.Close()
		return Err[Empty](fmt.Errorf("register: %w", err))
	}

	cfg, err := s.configRepo.GetOrCreate(ctx)
	if err != nil {
		_ = client.Close()
		return Err[Empty](err)
	}
	cfg.ServerURL = req.ServerURL
	cfg.Enabled = true
	if err := s.configRepo.Update(ctx, cfg); err != nil {
		_ = client.Close()
		return Err[Empty](err)
	}

	s.grpcClient = client
	s.engine.SetGRPCClient(client)

	if err := s.syncRemoteWorkspaces(ctx); err != nil {
		slog.Warn("sync: failed to sync remote workspaces", "err", err)
	}
	s.linkActiveWorkspace(ctx)

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
	if s.grpcClient == nil {
		return "", fmt.Errorf("not connected")
	}
	orgID := s.auth.GetActiveOrgID()
	if orgID == "" {
		return "", fmt.Errorf("no active org")
	}
	token, err := s.auth.GetAccessToken(ctx, s.grpcClient)
	if err != nil {
		return "", fmt.Errorf("get access token: %w", err)
	}
	authCtx := syncsvc.ContextWithAuth(ctx, token)
	resp, err := s.grpcClient.Workspace().Create(authCtx, workspacev1.CreateRequest_builder{
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
	if s.grpcClient == nil {
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

	if err := s.auth.Logout(ctx); err != nil {
		return Err[Empty](fmt.Errorf("logout: %w", err))
	}

	return OK(Empty{})
}

// GetStatus returns the current sync status including state and pending count.
func (s *SyncService) GetStatus() Result[dto.SyncStatusResponse] {
	ctx := context.Background()

	cfg, err := s.configRepo.Get(ctx)
	if err != nil {
		return Err[dto.SyncStatusResponse](err)
	}

	resp := dto.SyncStatusResponse{}
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
	} else {
		resp.State = string(syncsvc.StateDisconnected)
	}

	return OK(resp)
}

// LinkWorkspace maps a local workspace to a remote one and starts sync.
func (s *SyncService) LinkWorkspace(req dto.LinkWorkspaceRequest) Result[Empty] {
	ctx := context.Background()

	_, err := s.db.ExecContext(ctx,
		`UPDATE workspaces SET remote_workspace_id = ? WHERE id = ?`,
		req.RemoteWorkspaceID, req.LocalWorkspaceID)
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
	if s.grpcClient == nil {
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
	if s.grpcClient == nil {
		return fmt.Errorf("%s: not connected", funcName)
	}

	slog.Info("sync: syncRemoteWorkspaces start")
	remotes, err := s.fetchRemoteWorkspaces(ctx)
	if err != nil {
		return fmt.Errorf("%s: %w", funcName, err)
	}
	slog.Info("sync: fetched remote workspaces", "count", len(remotes))

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

// fetchRemoteWorkspaces paginates ListByOrg under the active org captured by
// the auth manager from the Login/Refresh response.
func (s *SyncService) fetchRemoteWorkspaces(ctx context.Context) ([]*workspacev1.Workspace, error) {
	activeOrg := s.auth.GetActiveOrgID()
	if activeOrg == "" {
		return nil, fmt.Errorf("no active org — auth must complete before workspace discovery")
	}

	slog.Info("sync: fetchRemoteWorkspaces: getting access token", "active_org", activeOrg)
	token, err := s.auth.GetAccessToken(ctx, s.grpcClient)
	if err != nil {
		return nil, fmt.Errorf("get access token: %w", err)
	}
	authCtx := syncsvc.ContextWithAuth(ctx, token)
	slog.Info("sync: fetchRemoteWorkspaces: calling ListByOrg", "active_org", activeOrg)

	var all []*workspacev1.Workspace
	cursor := ""
	for {
		req := workspacev1.ListByOrgRequest_builder{OrgId: activeOrg, Cursor: cursor}.Build()
		resp, err := s.grpcClient.Workspace().ListByOrg(authCtx, req)
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
