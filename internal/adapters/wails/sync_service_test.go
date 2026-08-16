package wails

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	_ "modernc.org/sqlite"

	authv1 "github.com/tetiva-app/proto/go/gophercourier/auth/v1"

	"github.com/tetiva-app/client/internal/adapters/wails/dto"
	"github.com/tetiva-app/client/internal/infrastructure/repository/sqlite"
	syncsvc "github.com/tetiva-app/client/internal/infrastructure/sync"
)

func setupSyncTestDB(t *testing.T) *sql.DB {
	t.Helper()
	// A file DB, not ":memory:": every pooled connection would otherwise open its
	// own empty database, and the sync engine queries from its own goroutines.
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	for _, pragma := range []string{"PRAGMA journal_mode=WAL", "PRAGMA foreign_keys=ON", "PRAGMA busy_timeout=5000"} {
		if _, err := db.Exec(pragma); err != nil {
			t.Fatal(err)
		}
	}
	migrations := []string{
		"001_initial.sql",
		"002_requests_json_checks.sql",
		"003_auth.sql",
		"004_collection_scripts.sql",
		"005_collection_auth_description.sql",
		"006_workspace_is_active.sql",
		"007_grpc_collection_metadata.sql",
		"008_graphql.sql",
		"009_sync.sql",
		"010_workspace_remote_index.sql",
		"011_sync_config_refresh_token.sql",
		"012_cookies.sql",
		"013_request_drafts.sql",
	}
	for _, name := range migrations {
		migration, err := os.ReadFile("../../../migrations/" + name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := db.Exec(string(migration)); err != nil {
			t.Fatalf("migration %s: %v", name, err)
		}
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func TestSyncService_CreateRemoteWorkspace_Error_NotConnected(t *testing.T) {
	svc := &SyncService{}

	res := svc.CreateRemoteWorkspace(dto.CreateWorkspaceRequest{Name: "ws"})

	require.NotNil(t, res.Error)
	assert.Equal(t, ErrCodeInternal, res.Error.Code)
	assert.True(t,
		strings.Contains(res.Error.Message, "not connected to sync server"),
		"expected 'not connected to sync server' in message, got %q", res.Error.Message,
	)
}

func TestSyncService_ListRemoteWorkspaces_Error_NotConnected(t *testing.T) {
	svc := &SyncService{}

	res := svc.ListRemoteWorkspaces()

	require.NotNil(t, res.Error)
	assert.Equal(t, ErrCodeInternal, res.Error.Code)
	assert.True(t,
		strings.Contains(res.Error.Message, "not connected to sync server"),
		"expected 'not connected to sync server' in message, got %q", res.Error.Message,
	)
}

func TestResumeOnStartup_ResetsStuckSendingRows(t *testing.T) {
	db := setupSyncTestDB(t)
	ctx := context.Background()

	queueRepo := sqlite.NewSyncQueueRepo(db)
	configRepo := sqlite.NewSyncConfigRepo(db)

	wsID := uuid.New().String()
	entityID := uuid.New().String()
	require.NoError(t, queueRepo.Enqueue(ctx, sqlite.SyncEntry{
		WorkspaceID: wsID,
		EntityType:  "collection",
		EntityID:    entityID,
		Action:      "upsert",
		OperationID: uuid.New().String(),
		Status:      "pending",
		CreatedAt:   time.Now().Truncate(time.Second),
	}))

	pending, err := queueRepo.ListPending(ctx, wsID, 10)
	require.NoError(t, err)
	require.Len(t, pending, 1)
	stuckID := pending[0].ID
	require.NoError(t, queueRepo.MarkSending(ctx, []int64{stuckID}))

	pending, _ = queueRepo.ListPending(ctx, wsID, 10)
	require.Empty(t, pending, "row should be in 'sending' before startup")

	// Sync is not configured, so ResumeOnStartup early-returns after
	// ResetSending; the remaining nil deps are never touched.
	svc := &SyncService{
		configRepo: configRepo,
		queueRepo:  queueRepo,
		db:         db,
	}

	err = svc.ResumeOnStartup(ctx)
	require.NoError(t, err)

	pending, err = queueRepo.ListPending(ctx, wsID, 10)
	require.NoError(t, err)
	require.Len(t, pending, 1, "expected stuck row to be reset to 'pending'")
	require.Equal(t, stuckID, pending[0].ID)
	require.Equal(t, "pending", pending[0].Status)
}

func TestSyncService_GetStatus_ReportsParkedEntries(t *testing.T) {
	f := newVerificationFixture(t)
	ctx := context.Background()

	for i := 0; i < 2; i++ {
		require.NoError(t, f.svc.queueRepo.Enqueue(ctx, sqlite.SyncEntry{
			WorkspaceID: seededWorkspaceID,
			EntityType:  "collection",
			EntityID:    uuid.New().String(),
			Action:      "upsert",
			OperationID: uuid.New().String(),
			Status:      "pending",
			CreatedAt:   time.Now().Truncate(time.Second),
		}))
	}

	pending, err := f.svc.queueRepo.ListPending(ctx, seededWorkspaceID, 10)
	require.NoError(t, err)
	require.Len(t, pending, 2)
	require.NoError(t, f.svc.queueRepo.MarkFailed(ctx, pending[0].ID, time.Now().Add(5*time.Minute)))

	st := f.svc.GetStatus()
	require.Nil(t, st.Error)
	assert.Equal(t, 2, st.Data.Pending)
	assert.Equal(t, 1, st.Data.Parked)
}

func TestActiveWorkspaceNeedingLink(t *testing.T) {
	db := setupSyncTestDB(t)
	ctx := context.Background()

	// Migrations seed an active, unlinked Default Workspace.
	id, name, ok := activeWorkspaceNeedingLink(ctx, db)
	require.True(t, ok)
	assert.Equal(t, "00000000-0000-4000-a000-000000000001", id)
	assert.Equal(t, "Default Workspace", name)

	_, err := db.Exec(`UPDATE workspaces SET remote_workspace_id = 'ws_r1' WHERE id = ?`, id)
	require.NoError(t, err)
	_, _, ok = activeWorkspaceNeedingLink(ctx, db)
	assert.False(t, ok)

	wsID := uuid.New().String()
	_, err = db.Exec(`INSERT INTO workspaces (id, name, is_active) VALUES (?, 'Work', 1)`, wsID)
	require.NoError(t, err)
	_, err = db.Exec(`UPDATE workspaces SET is_active = 0 WHERE id != ?`, wsID)
	require.NoError(t, err)
	id2, name2, ok := activeWorkspaceNeedingLink(ctx, db)
	require.True(t, ok)
	assert.Equal(t, wsID, id2)
	assert.Equal(t, "Work", name2)
}

func TestMigrateLegacyServerURL(t *testing.T) {
	db := setupSyncTestDB(t)
	ctx := context.Background()
	repo := sqlite.NewSyncConfigRepo(db)
	svc := &SyncService{configRepo: repo, db: db}

	cfg, err := repo.GetOrCreate(ctx)
	require.NoError(t, err)
	cfg.ServerURL = legacyDefaultServer
	require.NoError(t, repo.Update(ctx, cfg))

	svc.migrateLegacyServerURL(ctx, cfg)
	assert.Equal(t, currentDefaultServer, cfg.ServerURL)
	stored, err := repo.Get(ctx)
	require.NoError(t, err)
	assert.Equal(t, currentDefaultServer, stored.ServerURL)

	// Self-hosted URLs are never touched.
	cfg.ServerURL = "sync.corp.local:50051"
	require.NoError(t, repo.Update(ctx, cfg))
	svc.migrateLegacyServerURL(ctx, cfg)
	assert.Equal(t, "sync.corp.local:50051", cfg.ServerURL)
}

// linkSeededWorkspace maps the active workspace to a remote, as a previous run
// of the app would have left it.
func (f *verificationFixture) linkSeededWorkspace(t *testing.T, remoteID string) {
	t.Helper()
	_, err := f.db.Exec(`UPDATE workspaces SET remote_workspace_id = ? WHERE id = ?`,
		remoteID, seededWorkspaceID)
	require.NoError(t, err)
}

// restart drops the in-memory access token, leaving only what survives a
// process restart: the config row and the refresh token.
func (f *verificationFixture) restart() {
	f.auth.InvalidateAccessToken()
}

func TestResumeOnStartup_UnreachableServerStartsEngineOffline(t *testing.T) {
	f := newVerificationFixture(t)
	f.seedSession(t)
	f.linkSeededWorkspace(t, "remote-1")
	f.restart()
	f.authStub.setRefreshErr(status.Error(codes.Internal, "server selection error: connection refused"))

	require.NoError(t, f.svc.ResumeOnStartup(context.Background()))

	require.NotNil(t, f.svc.client(), "a failed refresh must not throw the client away")
	assert.Zero(t, f.wsStub.calls(), "workspace discovery needs a token the server never gave")
	assert.True(t, f.engine.IsEnabledForWorkspace(seededWorkspaceID))

	st := f.svc.GetStatus()
	require.Nil(t, st.Error)
	assert.True(t, st.Data.Enabled)
	require.Eventually(t, func() bool {
		return f.svc.GetStatus().Data.State == string(syncsvc.StateOffline)
	}, 3*time.Second, 10*time.Millisecond)
}

func TestResumeOnStartup_AuthExpiredLeavesEngineDark(t *testing.T) {
	f := newVerificationFixture(t)
	f.seedSession(t)
	f.linkSeededWorkspace(t, "remote-1")
	f.restart()
	f.authStub.setRefreshErr(status.Error(codes.Unauthenticated, "refresh token rejected"))

	err := f.svc.ResumeOnStartup(context.Background())

	require.ErrorIs(t, err, syncsvc.ErrAuthExpired)
	assert.Nil(t, f.svc.client())
	assert.Zero(t, f.wsStub.calls())
	assert.False(t, f.engine.IsEnabledForWorkspace(seededWorkspaceID))
}

func TestSyncService_ListSessions_DialsAfterAFailedStartup(t *testing.T) {
	f := newVerificationFixture(t)
	f.seedSession(t)
	f.authStub.meResp = authv1.MeResponse_builder{
		Sessions: []*authv1.SessionView{authv1.SessionView_builder{Id: "sess_1"}.Build()},
	}.Build()
	require.Nil(t, f.svc.client(), "startup left the service without a client")

	res := f.svc.ListSessions()

	require.Nil(t, res.Error)
	require.Len(t, res.Data, 1)
	assert.NotNil(t, f.svc.client())
}

func TestSyncService_ListSessions_DisabledSyncStaysNotConnected(t *testing.T) {
	f := newVerificationFixture(t)
	f.seedSession(t)
	ctx := context.Background()
	cfg, err := f.configRepo.Get(ctx)
	require.NoError(t, err)
	cfg.Enabled = false
	require.NoError(t, f.configRepo.Update(ctx, cfg))

	res := f.svc.ListSessions()

	require.NotNil(t, res.Error)
	assert.Equal(t, ErrCodeNotConnected, res.Error.Code)
	assert.Nil(t, f.svc.client())
}
