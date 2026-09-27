-- One row per collection and account: a pending unpublish waits for its account while another one is signed in.
-- cloud is the server's word (Publication.workspace_id != ''); until the next refresh the local mapping stands in.
-- unpublish_attempts/unpublish_error count the server's refusals of a pending unpublish; network failures do not count.
CREATE TABLE publications_by_owner (
  collection_id TEXT NOT NULL, workspace_id TEXT NOT NULL, owner_key TEXT NOT NULL, publication_id TEXT NOT NULL,
  slug TEXT NOT NULL DEFAULT '', public_url TEXT NOT NULL DEFAULT '', visibility TEXT NOT NULL DEFAULT '', status TEXT NOT NULL DEFAULT '',
  content_hash TEXT NOT NULL DEFAULT '', revision INTEGER NOT NULL DEFAULT 0, blocked INTEGER NOT NULL DEFAULT 0, blocked_reason TEXT NOT NULL DEFAULT '',
  badge INTEGER NOT NULL DEFAULT 0, can_manage INTEGER NOT NULL DEFAULT 0,
  settings TEXT CHECK (settings IS NULL OR json_valid(settings)), counters TEXT NOT NULL DEFAULT '{}' CHECK (json_valid(counters)),
  pending_unpublish INTEGER NOT NULL DEFAULT 0, server_updated_at TEXT NOT NULL DEFAULT '', refreshed_at TEXT NOT NULL DEFAULT '',
  created_at TEXT NOT NULL, updated_at TEXT NOT NULL, cloud INTEGER NOT NULL DEFAULT 0,
  unpublish_attempts INTEGER NOT NULL DEFAULT 0, unpublish_error TEXT NOT NULL DEFAULT '',
  PRIMARY KEY (collection_id, owner_key));

INSERT INTO publications_by_owner (collection_id, workspace_id, owner_key, publication_id, slug, public_url, visibility, status,
  content_hash, revision, blocked, blocked_reason, badge, can_manage, settings, counters, pending_unpublish,
  server_updated_at, refreshed_at, created_at, updated_at, cloud)
SELECT collection_id, workspace_id, owner_key, publication_id, slug, public_url, visibility, status,
  content_hash, revision, blocked, blocked_reason, badge, can_manage, settings, counters, pending_unpublish,
  server_updated_at, refreshed_at, created_at, updated_at,
  workspace_id IN (SELECT id FROM workspaces WHERE remote_workspace_id IS NOT NULL AND remote_workspace_id <> '')
FROM publications;

DROP TABLE publications;
ALTER TABLE publications_by_owner RENAME TO publications;
CREATE INDEX IF NOT EXISTS idx_publications_workspace ON publications(workspace_id);
