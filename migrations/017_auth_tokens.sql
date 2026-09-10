-- Local-only OAuth 2.0 tokens: never synced, never exported, one row per owner.
-- Deliberate exception to the soft-delete rule; a cleared token must leave the file.
CREATE TABLE IF NOT EXISTS auth_tokens (
    workspace_id TEXT NOT NULL,
    owner_kind TEXT NOT NULL,
    owner_id TEXT NOT NULL,
    config_hash TEXT NOT NULL,
    access_token TEXT NOT NULL,
    refresh_token TEXT NOT NULL DEFAULT '',
    token_type TEXT NOT NULL DEFAULT '',
    scope TEXT NOT NULL DEFAULT '',
    id_token TEXT NOT NULL DEFAULT '',
    expires_at TEXT NOT NULL DEFAULT '',
    obtained_at TEXT NOT NULL,
    updated_at TEXT NOT NULL,
    generation INTEGER NOT NULL DEFAULT 0,
    PRIMARY KEY (workspace_id, owner_kind, owner_id)
);
