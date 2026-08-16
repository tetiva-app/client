package wails

import (
	"context"
	"database/sql"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tetiva-app/client/internal/adapters/wails/dto"
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
