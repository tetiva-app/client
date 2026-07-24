package wails

import (
	"context"
	"database/sql"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	_ "modernc.org/sqlite"

	"github.com/tetiva-app/client/internal/adapters/wails/dto"
	"github.com/tetiva-app/client/internal/infrastructure/repository/sqlite"
)

func setupSyncTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec("PRAGMA foreign_keys=ON"); err != nil {
		t.Fatal(err)
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

// TODO: happy paths need SyncService deps behind interfaces; only the nil-client early exits are testable here.

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
