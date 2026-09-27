-- No FK on request_id: an example may arrive before its request.
CREATE TABLE IF NOT EXISTS response_examples (
  id TEXT PRIMARY KEY, request_id TEXT NOT NULL, workspace_id TEXT NOT NULL,
  name TEXT NOT NULL, status_code INTEGER NOT NULL DEFAULT 0, status_text TEXT NOT NULL DEFAULT '',
  headers TEXT NOT NULL DEFAULT '[]' CHECK (json_valid(headers)), body TEXT NOT NULL DEFAULT '',
  content_type TEXT NOT NULL DEFAULT '', protocol TEXT NOT NULL DEFAULT 'http', sort_order INTEGER NOT NULL DEFAULT 0,
  version INTEGER NOT NULL DEFAULT 1, is_delete INTEGER NOT NULL DEFAULT 0, is_synced INTEGER NOT NULL DEFAULT 0,
  created_by TEXT NOT NULL DEFAULT 'local_user', created_at TEXT NOT NULL,
  updated_by TEXT NOT NULL DEFAULT 'local_user', updated_at TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_response_examples_request ON response_examples(request_id, is_delete);
ALTER TABLE workspaces ADD COLUMN examples_backfill_pending INTEGER NOT NULL DEFAULT 0;
ALTER TABLE workspaces ADD COLUMN examples_backfill_token TEXT NOT NULL DEFAULT '';
ALTER TABLE workspaces ADD COLUMN examples_backfill_incomplete INTEGER NOT NULL DEFAULT 0;
ALTER TABLE workspaces ADD COLUMN examples_backfill_failed_passes INTEGER NOT NULL DEFAULT 0;
ALTER TABLE workspaces ADD COLUMN sync_page_token TEXT NOT NULL DEFAULT '';
ALTER TABLE sync_queue ADD COLUMN defer_count INTEGER NOT NULL DEFAULT 0;
UPDATE workspaces SET examples_backfill_pending = 1
 WHERE remote_workspace_id IS NOT NULL AND remote_workspace_id != '' AND is_delete = 0 AND last_sync_seq > 0;
CREATE TABLE IF NOT EXISTS sync_snapshot_seen (
  workspace_id TEXT NOT NULL, walk TEXT NOT NULL, generation INTEGER NOT NULL DEFAULT 0,
  entity_type TEXT NOT NULL, entity_id TEXT NOT NULL,
  PRIMARY KEY (workspace_id, walk, entity_type, entity_id)
);
