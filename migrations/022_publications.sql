-- Local-only cache of server state plus the unpublish intent; rows are deleted physically, like 017_auth_tokens.sql.
CREATE TABLE IF NOT EXISTS publications (
  collection_id TEXT PRIMARY KEY, workspace_id TEXT NOT NULL, owner_key TEXT NOT NULL, publication_id TEXT NOT NULL,
  slug TEXT NOT NULL DEFAULT '', public_url TEXT NOT NULL DEFAULT '', visibility TEXT NOT NULL DEFAULT '', status TEXT NOT NULL DEFAULT '',
  content_hash TEXT NOT NULL DEFAULT '', revision INTEGER NOT NULL DEFAULT 0, blocked INTEGER NOT NULL DEFAULT 0, blocked_reason TEXT NOT NULL DEFAULT '',
  badge INTEGER NOT NULL DEFAULT 0, can_manage INTEGER NOT NULL DEFAULT 0,
  settings TEXT CHECK (settings IS NULL OR json_valid(settings)), counters TEXT NOT NULL DEFAULT '{}' CHECK (json_valid(counters)),
  pending_unpublish INTEGER NOT NULL DEFAULT 0, server_updated_at TEXT NOT NULL DEFAULT '', refreshed_at TEXT NOT NULL DEFAULT '',
  created_at TEXT NOT NULL, updated_at TEXT NOT NULL);
CREATE INDEX IF NOT EXISTS idx_publications_workspace ON publications(workspace_id);
