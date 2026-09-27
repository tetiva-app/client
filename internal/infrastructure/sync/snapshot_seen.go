package sync

import (
	"context"
	"fmt"
	"log/slog"
	"slices"
	"time"

	syncv1 "github.com/tetiva-app/proto/go/gophercourier/sync/v1"

	"github.com/tetiva-app/client/internal/infrastructure/repository/sqlite"
)

// A walk is a paged snapshot read page by page under one token; the backfill is a walk of its own.
const (
	walkSnapshot = "snapshot"
	walkBackfill = "backfill"
)

var (
	snapshotEntityTypes = []string{"collection", "request", "environment", "variable", "response_example"}
	backfillEntityTypes = []string{"response_example"}
)

func entityTypeName(t syncv1.EntityType) string {
	switch t {
	case syncv1.EntityType_ENTITY_TYPE_COLLECTION:
		return "collection"
	case syncv1.EntityType_ENTITY_TYPE_REQUEST:
		return "request"
	case syncv1.EntityType_ENTITY_TYPE_ENVIRONMENT:
		return "environment"
	case syncv1.EntityType_ENTITY_TYPE_VARIABLE:
		return "variable"
	case syncv1.EntityType_ENTITY_TYPE_RESPONSE_EXAMPLE:
		return "response_example"
	}
	return ""
}

// recordSeen notes a page's entities of the given types as seen by the walk in progress.
func (ws *workspaceSyncer) recordSeen(ctx context.Context, walk string, changes []*syncv1.SyncChange, types []string) error {
	db := sqlite.DBTXFromContext(ctx, ws.engine.db)
	for _, change := range changes {
		entity := change.GetEntity()
		if entity == nil {
			continue
		}
		entityType := entityTypeName(entity.GetEntityType())
		if !slices.Contains(types, entityType) {
			continue
		}
		if _, err := db.ExecContext(ctx,
			`INSERT INTO sync_snapshot_seen (workspace_id, walk, generation, entity_type, entity_id) VALUES (?, ?, 0, ?, ?)
			 ON CONFLICT (workspace_id, walk, entity_type, entity_id) DO UPDATE SET generation = 0`,
			ws.localWorkspaceID, walk, entityType, entity.GetEntityId()); err != nil {
			return fmt.Errorf("record seen %s: %w", walk, err)
		}
	}
	return nil
}

// ageWalk moves everything the walk has seen into an earlier generation: the walk starts over.
func (ws *workspaceSyncer) ageWalk(ctx context.Context, walk string) error {
	if _, err := sqlite.DBTXFromContext(ctx, ws.engine.db).ExecContext(ctx,
		`UPDATE sync_snapshot_seen SET generation = generation + 1 WHERE workspace_id = ? AND walk = ?`,
		ws.localWorkspaceID, walk); err != nil {
		return fmt.Errorf("restart %s walk: %w", walk, err)
	}
	return nil
}

func (ws *workspaceSyncer) forgetWalk(ctx context.Context, walk string) error {
	if _, err := sqlite.DBTXFromContext(ctx, ws.engine.db).ExecContext(ctx,
		`DELETE FROM sync_snapshot_seen WHERE workspace_id = ? AND walk = ?`, ws.localWorkspaceID, walk); err != nil {
		return fmt.Errorf("forget %s walk: %w", walk, err)
	}
	return nil
}

// finishWalk deletes, as inbound tombstones, what only an earlier generation of a restarted walk saw: the
// server dropped it while the walk was down and the pull resumes past that delete. Queued changes win.
func (ws *workspaceSyncer) finishWalk(ctx context.Context, walk string, last []*syncv1.SyncChange, types []string) (int, error) {
	var restarted bool
	if err := sqlite.DBTXFromContext(ctx, ws.engine.db).QueryRowContext(ctx,
		`SELECT EXISTS (SELECT 1 FROM sync_snapshot_seen WHERE workspace_id = ? AND walk = ? AND generation > 0)`,
		ws.localWorkspaceID, walk).Scan(&restarted); err != nil {
		return 0, fmt.Errorf("finish %s walk: %w", walk, err)
	}

	deleted := 0
	if restarted {
		if err := ws.recordSeen(ctx, walk, last, types); err != nil {
			return 0, err
		}
		for _, entityType := range types {
			n, err := ws.deleteUnseen(ctx, walk, entityType)
			if err != nil {
				return 0, err
			}
			deleted += n
		}
	}
	if err := ws.forgetWalk(ctx, walk); err != nil {
		return 0, err
	}

	if deleted > 0 {
		slog.Info("sync: deleted entities the server dropped while a walk was restarting",
			"workspace", ws.localWorkspaceID, "walk", walk, "count", deleted)
		ws.dropParkedForGoneEntities(ctx)
		ws.sweepTokens(ctx)
	}
	return deleted, nil
}

func (ws *workspaceSyncer) deleteUnseen(ctx context.Context, walk, entityType string) (int, error) {
	table, ok := syncedTable(entityType)
	if !ok {
		return 0, nil
	}
	result, err := sqlite.DBTXFromContext(ctx, ws.engine.db).ExecContext(ctx, fmt.Sprintf(
		`UPDATE %[1]s SET is_delete = 1, is_synced = 1, updated_at = ?
		 WHERE is_delete = 0 AND is_synced = 1
		   AND id IN (SELECT entity_id FROM sync_snapshot_seen
		              WHERE workspace_id = ? AND walk = ? AND entity_type = ? AND generation > 0)
		   AND NOT EXISTS (SELECT 1 FROM sync_queue q
		                   WHERE q.workspace_id = ? AND q.entity_type = ? AND q.entity_id = %[1]s.id)`, table),
		time.Now().UTC().Format(time.RFC3339),
		ws.localWorkspaceID, walk, entityType, ws.localWorkspaceID, entityType)
	if err != nil {
		return 0, fmt.Errorf("delete %s the server dropped: %w", table, err)
	}
	n, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("delete %s the server dropped: %w", table, err)
	}
	return int(n), nil
}
