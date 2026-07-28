package wails

import (
	"context"
	"database/sql"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

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
