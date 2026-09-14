package wails

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tetiva-app/client/internal/adapters/wails/dto"
	"github.com/tetiva-app/client/internal/domain/entities"
	"github.com/tetiva-app/client/internal/infrastructure/repository/sqlite"
	syncsvc "github.com/tetiva-app/client/internal/infrastructure/sync"
)

func remoteIDOf(t *testing.T, db *sql.DB, workspaceID string) sql.NullString {
	t.Helper()
	var remote sql.NullString
	require.NoError(t, db.QueryRow(
		`SELECT remote_workspace_id FROM workspaces WHERE id = ?`, workspaceID).Scan(&remote))
	return remote
}

func setRemoteID(t *testing.T, db *sql.DB, workspaceID, remote string) {
	t.Helper()
	_, err := db.Exec(
		`UPDATE workspaces SET remote_workspace_id = ? WHERE id = ?`, remote, workspaceID)
	require.NoError(t, err)
}

func TestSyncRemoteWorkspaces_DropsMappingsFromAnotherAccount(t *testing.T) {
	f := newVerificationFixture(t)
	ctx := context.Background()

	setRemoteID(t, f.db, seededWorkspaceID, "ws_from_previous_account")

	f.seedSession(t)
	f.svc.grpcClient = f.client

	require.NoError(t, f.svc.syncRemoteWorkspaces(ctx))

	remote := remoteIDOf(t, f.db, seededWorkspaceID)
	assert.False(t, remote.Valid && remote.String != "",
		"a mapping the current org does not own must be dropped, got %q", remote.String)
}

func TestSyncRemoteWorkspaces_KeepsMappingOwnedByCurrentOrg(t *testing.T) {
	f := newVerificationFixture(t)
	ctx := context.Background()

	setRemoteID(t, f.db, seededWorkspaceID, f.wsStub.remoteID)

	f.seedSession(t)
	f.svc.grpcClient = f.client

	require.NoError(t, f.svc.syncRemoteWorkspaces(ctx))

	remote := remoteIDOf(t, f.db, seededWorkspaceID)
	assert.Equal(t, f.wsStub.remoteID, remote.String)
}

// Dropping the mapping is not enough — the workspace must reach the engine.
func TestEnableSync_StartsActiveWorkspaceAfterStaleMappingDropped(t *testing.T) {
	f := newVerificationFixture(t)
	ctx := context.Background()

	setRemoteID(t, f.db, seededWorkspaceID, "ws_from_previous_account")

	f.seedSession(t)
	f.svc.grpcClient = f.client

	require.NoError(t, f.svc.enableSync(ctx, f.client))

	assert.NotEqual(t, syncsvc.StateDisconnected, f.engine.GetWorkspaceState(seededWorkspaceID))
}

func lastSyncSeqOf(t *testing.T, db *sql.DB, workspaceID string) int64 {
	t.Helper()
	var seq int64
	require.NoError(t, db.QueryRow(
		`SELECT last_sync_seq FROM workspaces WHERE id = ?`, workspaceID).Scan(&seq))
	return seq
}

func TestLinkWorkspace_ResetsCursorWhenRemoteChanges(t *testing.T) {
	f := newVerificationFixture(t)
	setRemoteID(t, f.db, seededWorkspaceID, "remote-old")
	_, err := f.db.Exec(`UPDATE workspaces SET last_sync_seq = 42 WHERE id = ?`, seededWorkspaceID)
	require.NoError(t, err)

	res := f.svc.LinkWorkspace(dto.LinkWorkspaceRequest{
		LocalWorkspaceID:  seededWorkspaceID,
		RemoteWorkspaceID: "remote-new",
	})

	require.Nil(t, res.Error)
	assert.Equal(t, "remote-new", remoteIDOf(t, f.db, seededWorkspaceID).String)
	assert.Zero(t, lastSyncSeqOf(t, f.db, seededWorkspaceID))
}

func TestLinkWorkspace_KeepsCursorForSameRemote(t *testing.T) {
	f := newVerificationFixture(t)
	setRemoteID(t, f.db, seededWorkspaceID, "remote-1")
	_, err := f.db.Exec(`UPDATE workspaces SET last_sync_seq = 42 WHERE id = ?`, seededWorkspaceID)
	require.NoError(t, err)

	res := f.svc.LinkWorkspace(dto.LinkWorkspaceRequest{
		LocalWorkspaceID:  seededWorkspaceID,
		RemoteWorkspaceID: "remote-1",
	})

	require.Nil(t, res.Error)
	assert.EqualValues(t, 42, lastSyncSeqOf(t, f.db, seededWorkspaceID))
}

func enqueueOutbox(t *testing.T, f *verificationFixture, workspaceID string, n int) {
	t.Helper()
	for range n {
		require.NoError(t, f.svc.queueRepo.Enqueue(context.Background(), sqlite.SyncEntry{
			WorkspaceID: workspaceID,
			EntityType:  "request",
			EntityID:    uuid.NewString(),
			Action:      "update",
			OperationID: uuid.NewString(),
			Status:      "pending",
			CreatedAt:   time.Now().Truncate(time.Second),
		}))
	}
}

func outboxRows(t *testing.T, db *sql.DB, workspaceID string) int {
	t.Helper()
	var n int
	require.NoError(t, db.QueryRow(
		`SELECT COUNT(*) FROM sync_queue WHERE workspace_id = ?`, workspaceID).Scan(&n))
	return n
}

func TestUnlinkWorkspace_DropsOutbox(t *testing.T) {
	f := newVerificationFixture(t)
	setRemoteID(t, f.db, seededWorkspaceID, "remote-1")
	enqueueOutbox(t, f, seededWorkspaceID, 2)

	res := f.svc.UnlinkWorkspace(dto.UnlinkWorkspaceRequest{LocalWorkspaceID: seededWorkspaceID})

	require.Nil(t, res.Error)
	assert.Zero(t, outboxRows(t, f.db, seededWorkspaceID),
		"an unlinked workspace must not push its old outbox to the next account")
	assert.False(t, remoteIDOf(t, f.db, seededWorkspaceID).Valid)
}

func TestUnlinkWorkspace_KeepsOutboxWhenMappingSurvives(t *testing.T) {
	f := newVerificationFixture(t)
	setRemoteID(t, f.db, seededWorkspaceID, "remote-1")
	enqueueOutbox(t, f, seededWorkspaceID, 2)
	f.engine.StartWorkspace(seededWorkspaceID, "remote-1", 0)

	_, err := f.db.Exec(`CREATE TRIGGER block_unlink BEFORE UPDATE OF remote_workspace_id ON workspaces
		BEGIN SELECT RAISE(ABORT, 'mapping update refused'); END`)
	require.NoError(t, err)

	res := f.svc.UnlinkWorkspace(dto.UnlinkWorkspaceRequest{LocalWorkspaceID: seededWorkspaceID})

	require.NotNil(t, res.Error)
	assert.Equal(t, "remote-1", remoteIDOf(t, f.db, seededWorkspaceID).String)
	assert.Equal(t, 2, outboxRows(t, f.db, seededWorkspaceID),
		"a workspace that stayed linked must keep the edits it still owes the cloud")

	assert.NotEqual(t, syncsvc.StateDisconnected, f.engine.GetWorkspaceState(seededWorkspaceID),
		"a workspace that stayed linked must keep its syncer")

	coll := &entities.Collection{
		ID:           uuid.New(),
		WorkspaceID:  uuid.MustParse(seededWorkspaceID),
		Name:         "After the failed unlink",
		AuthType:     entities.AuthTypeNone,
		AuthData:     "{}",
		GRPCMetadata: []entities.HeaderItem{},
		Version:      1,
		CreatedBy:    "test_user",
		CreatedAt:    time.Now().Truncate(time.Second),
		UpdatedBy:    "test_user",
		UpdatedAt:    time.Now().Truncate(time.Second),
	}
	synced := syncsvc.NewSyncedCollectionRepo(sqlite.NewCollectionRepo(f.db), f.svc.queueRepo, f.db, f.engine)
	require.NoError(t, synced.Create(context.Background(), coll))

	assert.Equal(t, 3, outboxRows(t, f.db, seededWorkspaceID),
		"edits made after the failed unlink must still reach the outbox")
}

func TestSyncRemoteWorkspaces_DropsOutboxOfForeignWorkspace(t *testing.T) {
	f := newVerificationFixture(t)
	ctx := context.Background()

	setRemoteID(t, f.db, seededWorkspaceID, "ws_from_previous_account")
	enqueueOutbox(t, f, seededWorkspaceID, 2)

	f.seedSession(t)
	f.svc.grpcClient = f.client

	require.NoError(t, f.svc.syncRemoteWorkspaces(ctx))

	assert.Zero(t, outboxRows(t, f.db, seededWorkspaceID),
		"the outbox of a workspace owned by another account must go with the mapping")
}
