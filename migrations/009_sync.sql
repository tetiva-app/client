-- Sync configuration (singleton)
CREATE TABLE IF NOT EXISTS sync_config (
    id          INTEGER PRIMARY KEY CHECK (id = 1),
    client_id   TEXT    NOT NULL,
    server_url  TEXT,
    user_email  TEXT,
    enabled     INTEGER NOT NULL DEFAULT 0
);

-- Sync outbox queue
CREATE TABLE IF NOT EXISTS sync_queue (
    id            INTEGER PRIMARY KEY AUTOINCREMENT,
    workspace_id  TEXT    NOT NULL,
    entity_type   TEXT    NOT NULL,
    entity_id     TEXT    NOT NULL,
    action        TEXT    NOT NULL,
    operation_id  TEXT    NOT NULL UNIQUE,
    status        TEXT    NOT NULL DEFAULT 'pending',
    retry_count   INTEGER NOT NULL DEFAULT 0,
    next_retry_at TEXT,
    created_at    TEXT    NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_sync_queue_fetch
    ON sync_queue(status, workspace_id, entity_type, entity_id);

-- Extend workspaces with sync mapping
ALTER TABLE workspaces ADD COLUMN remote_workspace_id TEXT;
ALTER TABLE workspaces ADD COLUMN last_sync_seq INTEGER NOT NULL DEFAULT 0;

-- Track sync status per entity
ALTER TABLE collections  ADD COLUMN is_synced INTEGER NOT NULL DEFAULT 0;
ALTER TABLE requests     ADD COLUMN is_synced INTEGER NOT NULL DEFAULT 0;
ALTER TABLE environments ADD COLUMN is_synced INTEGER NOT NULL DEFAULT 0;
ALTER TABLE variables    ADD COLUMN is_synced INTEGER NOT NULL DEFAULT 0;
