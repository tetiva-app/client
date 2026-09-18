package wails

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	workspacev1 "github.com/tetiva-app/proto/go/gophercourier/workspace/v1"

	"github.com/tetiva-app/client/internal/adapters/wails/dto"
	"github.com/tetiva-app/client/internal/constants"
	"github.com/tetiva-app/client/internal/domain"
	"github.com/tetiva-app/client/internal/domain/usecase/auth"
	"github.com/tetiva-app/client/internal/domain/usecase/workspace"
	"github.com/tetiva-app/client/internal/infrastructure/repository/sqlite"
	syncsvc "github.com/tetiva-app/client/internal/infrastructure/sync"
)

// sessionRPCTimeout bounds the device-management calls: the UI waits on them.
const sessionRPCTimeout = 10 * time.Second

const (
	// serverInfoTimeout bounds discovery: the modal blocks on it.
	serverInfoTimeout = 5 * time.Second
	// browserSignInFinishTimeout covers the work that happens after a flow already
	// reached a terminal state, so its failures cost discovery, not the session.
	browserSignInFinishTimeout = 30 * time.Second
)

// The intents the cabinet understands; "" lets the server pick sign-in.
const (
	signInIntentSignIn   = "signin"
	signInIntentRegister = "register"
)

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

	signIn     auth.SignInManager
	signInSink *SignInEventSink

	// signInHolds are the sign-in attempts that stopped the syncers and owe them a
	// restart; the syncers come back only when the last hold is released.
	signInMu    sync.Mutex
	signInHolds map[string]struct{}
	signInPrior signInHold

	focusMu sync.Mutex
	// nil in tests; main.go closes over the "main" window.
	focusMain func()
}

func NewSyncService(
	engine *syncsvc.SyncEngine,
	auth *syncsvc.SyncAuthManager,
	configRepo sqlite.SyncConfigRepository,
	queueRepo sqlite.SyncQueueRepository,
	db *sql.DB,
	wsUC workspace.Usecase,
	signIn auth.SignInManager,
	signInSink *SignInEventSink,
) *SyncService {
	return &SyncService{
		engine:     engine,
		auth:       auth,
		configRepo: configRepo,
		queueRepo:  queueRepo,
		db:         db,
		wsUC:       wsUC,
		newClient:  syncsvc.NewGRPCClient,
		signIn:     signIn,
		signInSink: signInSink,
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

// Dials from the saved config when a failed startup left no client, or Retry in the
// UI could never recover. A disabled or serverless config stays ErrNotConnected.
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

// Wires the Wails event system to the sync engine and to the browser sign-in events.
func (s *SyncService) SetEventEmitter(fn syncsvc.EventEmitter) {
	s.engine.SetEventEmitter(fn)
	if s.signInSink != nil {
		s.signInSink.SetEmit(fn)
	}
}

// SetFocusMain lets the loopback callback raise the window. main.go passes a
// closure over the "main" window: this package must not import application.
func (s *SyncService) SetFocusMain(fn func()) {
	s.focusMu.Lock()
	s.focusMain = fn
	s.focusMu.Unlock()
}

func (s *SyncService) raiseWindow() {
	s.focusMu.Lock()
	focus := s.focusMain
	s.focusMu.Unlock()

	if focus != nil {
		focus()
	}
}

// Resumes sync for all linked workspaces, if credentials are saved.
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
	if err != nil || cfg == nil {
		return nil
	}

	if handled, err := s.enforceReauth(ctx, cfg); handled {
		return err
	}

	if !cfg.Enabled || cfg.ServerURL == "" {
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

// enforceReauth signs every pre-browser-signin install out once (spec §2.7). The
// flags are written first, so a crash before the cleanup only repeats it.
func (s *SyncService) enforceReauth(ctx context.Context, cfg *sqlite.SyncConfig) (bool, error) {
	// A fresh install was never signed in, so there is nothing to drop.
	if cfg.AuthGeneration < syncsvc.CurrentAuthGeneration && cfg.Enabled {
		cfg.AuthGeneration = syncsvc.CurrentAuthGeneration
		cfg.ReauthRequired = true
		if err := s.configRepo.Update(ctx, cfg); err != nil {
			return true, fmt.Errorf("mark reauth required: %w", err)
		}
	}

	if !cfg.ReauthRequired {
		return false, nil
	}

	s.migrateLegacyServerURL(ctx, cfg)
	// The keychain entry is scoped by client_id, so the cleanup needs it even
	// when the server is out of reach.
	s.auth.SetClientID(cfg.ClientID)

	var client *syncsvc.GRPCClient
	if cfg.ServerURL != "" {
		dialed, err := s.dial(cfg.ServerURL)
		if err != nil {
			// Revoking the session on the server is best effort; the local
			// credentials go either way.
			slog.WarnContext(ctx, "sync: reauth cleanup could not dial the server", "err", err)
		} else {
			client = dialed
			defer func() { _ = dialed.Close() }()
		}
	}

	if err := s.auth.Logout(ctx, client); err != nil {
		slog.WarnContext(ctx, "sync: reauth cleanup failed; repeating on the next start", "err", err)
	}

	return true, nil
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

// The offline counterpart of enableSync: no RPC, so an unreachable server at
// startup costs discovery, not sync.
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

// errNoActiveOrg means the session carries no organization even after a refresh.
var errNoActiveOrg = errors.New("no active organization")

func (s *SyncService) createRemoteWorkspace(ctx context.Context, name string) (string, error) {
	// ensureClient, not client(): a failed startup leaves the service without a
	// client, and the user asking for a cloud workspace is a fine moment to dial.
	client, err := s.ensureClient(ctx)
	if err != nil {
		return "", err
	}
	// The token first: the organization only arrives with a Login or a Refresh
	// response, so a session restored from disk has none until this call runs.
	token, err := s.auth.GetAccessToken(ctx, client)
	if err != nil {
		return "", fmt.Errorf("get access token: %w", err)
	}
	orgID := s.auth.GetActiveOrgID()
	if orgID == "" {
		return "", errNoActiveOrg
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
	if err := s.linkLocalWorkspace(ctx, localID, remoteID); err != nil {
		slog.Warn("sync: auto-link mapping update failed", "err", err)
		return
	}
	slog.Info("sync: active workspace auto-linked", "local_id", localID, "remote_id", remoteID)
}

// Local first, server second: a refused cloud copy costs the sync mapping, never
// the name the user typed. The reason travels back as a warning.
func (s *SyncService) CreateRemoteWorkspace(req dto.CreateWorkspaceRequest) Result[dto.CreateRemoteWorkspaceResult] {
	ctx := context.Background()

	w, err := s.wsUC.Create(ctx,
		workspace.Create{Name: req.Name},
		workspace.CreateOpt{UserID: defaultUserID},
	)
	if err != nil {
		return Err[dto.CreateRemoteWorkspaceResult](fmt.Errorf("createRemoteWorkspace: %w", err))
	}

	out := dto.CreateRemoteWorkspaceResult{Workspace: dto.WorkspaceToResponse(w)}

	// Only the network half gets the deadline: the local workspace above must
	// survive a hung server, and the mapping write below is local too.
	rpcCtx, cancel := context.WithTimeout(ctx, sessionRPCTimeout)
	defer cancel()

	remoteID, err := s.createRemoteWorkspace(rpcCtx, req.Name)
	if err != nil {
		slog.Warn("sync: cloud workspace creation failed; kept local",
			"local_id", w.ID, "err", err)
		out.SyncWarning = localOnlyWorkspaceWarning(err)
		return OK(out)
	}

	if err := s.linkLocalWorkspace(ctx, w.ID.String(), remoteID); err != nil {
		// The cloud copy exists; only the local mapping is missing, so say that
		// instead of blaming the server.
		slog.Warn("sync: linking the new workspace failed; kept local",
			"local_id", w.ID, "remote_id", remoteID, "err", err)
		out.SyncWarning = "Workspace created, but this device could not save the link to its cloud copy. It will be linked again the next time sync connects."
		return OK(out)
	}

	out.Workspace.RemoteWorkspaceID = &remoteID
	return OK(out)
}

// localOnlyWorkspaceWarning names the cloud step that failed in one sentence the
// user can act on; the underlying error goes to the log, not to the dialog.
func localOnlyWorkspaceWarning(err error) string {
	const prefix = "Workspace created on this device only: "
	// A configured but unreachable server never reaches ErrNotConnected, and
	// "did not accept it" would read as a refusal by the account.
	const unreachable = "the sync server is unreachable. It will be linked to the cloud on the next connection."
	switch {
	case errors.Is(err, ErrNotConnected):
		return prefix + "not connected to the sync server."
	case errors.Is(err, errNoActiveOrg):
		return prefix + "this account has no active organization yet. Reconnect and link the workspace afterwards."
	case errors.Is(err, context.DeadlineExceeded):
		// The token refresh can blow the deadline before any RPC returns a status.
		return prefix + unreachable
	}
	switch status.Code(err) {
	case codes.Unauthenticated:
		return prefix + "the session has expired. Sign in again to sync it."
	case codes.PermissionDenied:
		return prefix + "this account may not create workspaces in the organization."
	case codes.ResourceExhausted:
		return prefix + "the plan's workspace limit is reached."
	case codes.Unavailable, codes.DeadlineExceeded:
		return prefix + unreachable
	}
	return prefix + "the sync server did not accept it. It can be linked to the cloud later."
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
		resp.ReauthRequired = cfg.ReauthRequired
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

		tooLarge, err := s.engine.GetTooLargeCount(ctx, activeWorkspaceID)
		if err == nil {
			resp.TooLarge = tooLarge
		}
	} else {
		resp.State = string(syncsvc.StateDisconnected)
	}

	return OK(resp)
}

// linkLocalWorkspace maps a local workspace to a remote one and starts its syncer.
func (s *SyncService) linkLocalWorkspace(ctx context.Context, localID, remoteID string) error {
	// last_sync_seq is a position in the previous remote's change log, so a
	// re-link to a different remote must start from scratch.
	_, err := s.db.ExecContext(ctx,
		`UPDATE workspaces
		 SET remote_workspace_id = ?,
		     last_sync_seq = CASE WHEN remote_workspace_id IS ? THEN last_sync_seq ELSE 0 END
		 WHERE id = ?`,
		remoteID, remoteID, localID)
	if err != nil {
		return err
	}

	var lastSyncSeq int64
	_ = s.db.QueryRowContext(ctx,
		`SELECT last_sync_seq FROM workspaces WHERE id = ?`, localID).Scan(&lastSyncSeq)

	s.engine.StartWorkspace(localID, remoteID, lastSyncSeq)
	return nil
}

func (s *SyncService) LinkWorkspace(req dto.LinkWorkspaceRequest) Result[Empty] {
	if err := s.linkLocalWorkspace(context.Background(), req.LocalWorkspaceID, req.RemoteWorkspaceID); err != nil {
		return Err[Empty](err)
	}
	return OK(Empty{})
}

// UnlinkWorkspace stops sync for a workspace and clears the remote mapping.
func (s *SyncService) UnlinkWorkspace(req dto.UnlinkWorkspaceRequest) Result[Empty] {
	ctx := context.Background()

	s.engine.StopWorkspace(req.LocalWorkspaceID)

	// Mapping and outbox drop together: a surviving outbox reaches the next account.
	err := sqlite.WithTx(ctx, s.db, func(txCtx context.Context) error {
		if _, err := sqlite.DBTXFromContext(txCtx, s.db).ExecContext(txCtx,
			`UPDATE workspaces SET remote_workspace_id = NULL WHERE id = ?`,
			req.LocalWorkspaceID); err != nil {
			return fmt.Errorf("clear remote mapping: %w", err)
		}
		if _, err := s.queueRepo.DeleteByWorkspace(txCtx, req.LocalWorkspaceID); err != nil {
			return fmt.Errorf("drop outbox: %w", err)
		}
		return nil
	})
	if err != nil {
		s.restartLinkedSyncer(ctx, req.LocalWorkspaceID)
		return Err[Empty](err)
	}

	return OK(Empty{})
}

func (s *SyncService) restartLinkedSyncer(ctx context.Context, localWorkspaceID string) {
	var (
		remoteID    sql.NullString
		lastSyncSeq int64
	)
	err := s.db.QueryRowContext(ctx,
		`SELECT remote_workspace_id, last_sync_seq FROM workspaces WHERE id = ?`,
		localWorkspaceID).Scan(&remoteID, &lastSyncSeq)
	if err != nil {
		slog.Warn("sync: failed to read the workspace mapping after a failed unlink",
			"workspace", localWorkspaceID, "err", err)
		return
	}
	if !remoteID.Valid || remoteID.String == "" {
		return
	}
	s.engine.StartWorkspace(localWorkspaceID, remoteID.String, lastSyncSeq)
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
		if _, err := s.queueRepo.DeleteByWorkspace(ctx, localID); err != nil {
			slog.Warn("sync: clearing the outbox of a foreign workspace failed", "local_id", localID, "err", err)
			continue
		}
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

// GetServerCapabilities asks one server what it offers before anyone signs in. No
// discovery endpoint is not an error — the modal falls back to the in-app form.
func (s *SyncService) GetServerCapabilities(req dto.ServerCapabilitiesRequest) Result[dto.ServerCapabilities] {
	const funcName = "SyncService.GetServerCapabilities"
	ctx, cancel := context.WithTimeout(context.Background(), serverInfoTimeout)
	defer cancel()

	client, err := s.dial(req.ServerURL)
	if err != nil {
		slog.Warn("sync: discovery dial failed", "server", req.ServerURL, "err", err)
		return Err[dto.ServerCapabilities](fmt.Errorf("%s: %w", funcName, ErrServerUnreachable))
	}
	defer func() { _ = client.Close() }()

	info, err := client.GetServerInfo(ctx)
	switch {
	case errors.Is(err, auth.ErrServerInfoUnsupported):
		return OK(dto.ServerCapabilities{})
	case err != nil:
		// A rate limit lands here too: a refused discovery is still no answer to
		// branch on, and the status is deliberately not kept in the chain.
		slog.Warn("sync: discovery failed", "server", req.ServerURL, "err", err)
		return Err[dto.ServerCapabilities](fmt.Errorf("%s: %w", funcName, ErrServerUnreachable))
	}

	out := dto.ServerCapabilities{
		ServerVersion:    info.Version,
		RegistrationOpen: info.RegistrationOpen,
	}
	if info.DesktopSignIn {
		u, err := auth.ValidateEndpointURL(info.DesktopSignInURL)
		if err != nil {
			slog.Warn("sync: server advertises browser sign-in with an unusable url",
				"server", req.ServerURL, "err", err)
		} else {
			out.DesktopSignIn = true
			out.SignInHost = u.Hostname()
		}
	}
	return OK(out)
}

// StartBrowserSignIn files one sign-in request and hands back the link to open.
func (s *SyncService) StartBrowserSignIn(req dto.StartBrowserSignInRequest) Result[dto.BrowserSignInInfo] {
	const funcName = "SyncService.StartBrowserSignIn"
	ctx := context.Background()

	// Checked before the syncers stop: an id the restore cannot name would leave
	// them down, and the manager would refuse it anyway.
	if _, err := uuid.Parse(req.FlowID); err != nil {
		return Err[dto.BrowserSignInInfo](&domain.ValidationError{
			Fields: map[string]string{"flowId": "invalid UUID"},
		})
	}

	switch req.Intent {
	case "", signInIntentSignIn, signInIntentRegister:
	default:
		return Err[dto.BrowserSignInInfo](&domain.ValidationError{
			Fields: map[string]string{"intent": `must be "signin" or "register"`},
		})
	}

	// Read before the syncers stop, so the hold knows the state a failed attempt
	// has to put back.
	cfg, err := s.configRepo.GetOrCreate(ctx)
	if err != nil {
		return Err[dto.BrowserSignInInfo](signInError(fmt.Errorf("%s: read sync config: %w", funcName, err)))
	}

	// The syncers hold the engine's client, so they are stopped and joined before
	// any adoption: a live one would carry the new bearer to the previous server.
	s.armSignInRestore(req.FlowID, signInHold{
		syncEnabled:          cfg.Enabled,
		awaitingVerification: s.awaitingVerification.Load(),
	})
	if !s.engine.StopAll() {
		s.restoreSyncAfterSignIn(req.FlowID)
		return Err[dto.BrowserSignInInfo](errors.New("sync is still stopping, try again in a moment"))
	}
	s.awaitingVerification.Store(false)

	client, err := s.dial(req.ServerURL)
	if err != nil {
		s.restoreSyncAfterSignIn(req.FlowID)
		return Err[dto.BrowserSignInInfo](signInError(fmt.Errorf("%s: dial: %w", funcName, err)))
	}

	deviceName, _ := os.Hostname()
	params := auth.SignInParams{
		ServerURL:  req.ServerURL,
		ClientID:   cfg.ClientID,
		DeviceName: deviceName,
		Platform:   runtime.GOOS,
		AppVersion: constants.AppVersion,
		Locale:     req.Locale,
		Intent:     req.Intent,
	}

	info, err := s.signIn.Start(ctx, req.FlowID, client, params, s.signInHooks(req.FlowID))
	if err != nil {
		// The manager owns the client from the first line of Start, refusals
		// included, so it is never closed here.
		s.restoreSyncAfterSignIn(req.FlowID)
		return Err[dto.BrowserSignInInfo](signInError(err))
	}

	return OK(dto.BrowserSignInInfo{
		FlowID:    info.ID,
		LoginURL:  info.LoginURL,
		Host:      info.Host,
		ExpiresAt: formatSessionTime(info.ExpiresAt),
	})
}

// signInHooks closes over the flow id so a flow that ends after its successor
// started cannot restore the syncers under it.
func (s *SyncService) signInHooks(flowID string) auth.SignInHooks {
	const funcName = "SyncService.signInHooks"

	return auth.SignInHooks{
		OnApproved: func(ctx context.Context, c auth.SignInClient, p auth.SignInParams, tokens auth.SignInTokens) error {
			// The manager hands back the very client it was given, so this is a
			// contract check, not a conversion.
			grpcClient, ok := c.(*syncsvc.GRPCClient)
			if !ok {
				return fmt.Errorf("%s: unexpected sign-in client %T", funcName, c)
			}
			if _, err := s.auth.AdoptSignIn(ctx, p.ServerURL, tokens); err != nil {
				return err
			}
			s.setClient(grpcClient)
			// The engine holds its own reference: without this the next push would
			// carry the new bearer to the previous server.
			s.engine.SetGRPCClient(grpcClient)

			return nil
		},
		OnDone: func(outcome auth.SignInOutcome) {
			// finishBrowserSignIn starts the syncers itself, on the adopted account,
			// but the hold still has to go or the next failed sign-in never restores.
			s.releaseSignInHold(flowID)
			go s.finishBrowserSignIn(outcome)
		},
		OnCallback: s.raiseWindow,
		OnTerminal: func(auth.FlowState) { go s.restoreSyncAfterSignIn(flowID) },
	}
}

// BrowserSignInStatus is the whole observable state of one attempt, so a panel
// restored after a webview reload renders from it alone.
func (s *SyncService) BrowserSignInStatus(req dto.FlowIDRequest) Result[dto.BrowserSignInStatus] {
	if _, err := uuid.Parse(req.FlowID); err != nil {
		return Err[dto.BrowserSignInStatus](&domain.ValidationError{
			Fields: map[string]string{"flowId": "invalid UUID"},
		})
	}

	st, ok := s.signIn.Status(req.FlowID)
	if !ok {
		return OK(dto.BrowserSignInStatus{})
	}

	out := dto.BrowserSignInStatus{
		State: string(st.State),
		Info: dto.BrowserSignInInfo{
			FlowID:    req.FlowID,
			LoginURL:  st.Info.LoginURL,
			Host:      st.Info.Host,
			ExpiresAt: formatSessionTime(st.Info.ExpiresAt),
		},
		EmailVerificationPending: st.EmailVerificationPending,
	}
	if st.Err != nil {
		out.Error = signInMessage(st.Err)
	}
	if st.State == auth.FlowDone && st.Outcome != nil {
		out.Auth = &dto.AuthStateResult{
			Email:                     st.Outcome.Email,
			RequiresEmailVerification: st.Outcome.RequiresEmailVerification,
		}
	}
	return OK(out)
}

// CancelBrowserSignIn stops one attempt; an id the manager never had is not an error.
func (s *SyncService) CancelBrowserSignIn(req dto.FlowIDRequest) Result[Empty] {
	const funcName = "SyncService.CancelBrowserSignIn"

	if _, err := uuid.Parse(req.FlowID); err != nil {
		return Err[Empty](&domain.ValidationError{
			Fields: map[string]string{"flowId": "invalid UUID"},
		})
	}

	if err := s.signIn.Cancel(req.FlowID); err != nil {
		return Err[Empty](fmt.Errorf("%s: %w", funcName, err))
	}
	return OK(Empty{})
}

// finishBrowserSignIn runs after the flow already reached done, so its failures
// cost discovery, not the session.
func (s *SyncService) finishBrowserSignIn(outcome auth.SignInOutcome) {
	ctx, cancel := context.WithTimeout(context.Background(), browserSignInFinishTimeout)
	defer cancel()

	if outcome.RequiresEmailVerification {
		s.awaitingVerification.Store(true)
		return
	}

	client := s.client()
	if client == nil {
		slog.Warn("sync: browser sign-in finished without a client")
		return
	}
	if err := s.enableSync(ctx, client); err != nil {
		slog.Warn("sync: failed to sync remote workspaces after browser sign-in", "err", err)
	}
}

// signInHold is the state the sign-in interrupted, read before the syncers stop.
type signInHold struct {
	syncEnabled          bool
	awaitingVerification bool
}

// armSignInRestore records that this attempt is holding the syncers down.
func (s *SyncService) armSignInRestore(flowID string, prior signInHold) {
	s.signInMu.Lock()
	if s.signInHolds == nil {
		s.signInHolds = map[string]struct{}{}
	}
	// What goes back is the state from before the syncers first went down: an
	// attempt armed beside a live one only sees what that one already cleared.
	if len(s.signInHolds) == 0 {
		s.signInPrior = prior
	}
	s.signInHolds[flowID] = struct{}{}
	s.signInMu.Unlock()
}

// releaseSignInHold drops one attempt's hold and reports whether it was the last
// one: an attempt that fails while another is still running owes nothing.
func (s *SyncService) releaseSignInHold(flowID string) (signInHold, bool) {
	s.signInMu.Lock()
	defer s.signInMu.Unlock()

	if _, held := s.signInHolds[flowID]; !held {
		return signInHold{}, false
	}
	delete(s.signInHolds, flowID)

	return s.signInPrior, len(s.signInHolds) == 0
}

// restoreSyncAfterSignIn puts back what StartBrowserSignIn took down, so a
// sign-in that never happened does not leave the still valid session half-dead.
func (s *SyncService) restoreSyncAfterSignIn(flowID string) {
	hold, last := s.releaseSignInHold(flowID)
	if !last {
		return
	}

	// The wait goes back unless a sign-in that landed meanwhile already raised it.
	s.awaitingVerification.CompareAndSwap(false, hold.awaitingVerification)

	// Sync that was off has no session behind it: starting the syncers would only
	// log a token failure per linked workspace.
	if !hold.syncEnabled {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), browserSignInFinishTimeout)
	defer cancel()

	// startLinkedWorkspaces, not enableSync: nothing about the account changed,
	// and enableSync would go to the network and clear the verification wait.
	s.startLinkedWorkspaces(ctx)
}

// signInError turns a sign-in that never started into something the modal can print;
// validation errors are already user-facing copy and keep their fields.
func signInError(err error) error {
	var valErr *domain.ValidationError
	switch {
	case errors.Is(err, auth.ErrSignInUnsupported):
		return &domain.ValidationError{
			Fields: map[string]string{"serverUrl": "this server does not offer browser sign-in"},
		}
	case errors.As(err, &valErr):
		return err
	}
	slog.Warn("sync: browser sign-in did not start", "err", err)

	return errors.New(signInMessage(err))
}

// signInMessage is the copy a terminal sign-in shows the user; anything unmapped
// could carry wrapped internals to the panel.
func signInMessage(err error) string {
	switch {
	case errors.Is(err, auth.ErrSignInDenied):
		return "Sign-in was denied in the browser"
	case errors.Is(err, auth.ErrSignInExpired):
		return "Sign-in link expired, try again"
	case errors.Is(err, auth.ErrSignInUnsupported):
		return "This server does not offer browser sign-in"
	}
	return "Sign-in failed, try again"
}
