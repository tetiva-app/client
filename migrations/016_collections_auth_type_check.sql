-- migrate:fk-off
-- SQLite cannot alter a CHECK in place and requests.collection_id cascades on
-- delete, so the rebuild needs foreign keys off outside the transaction.
CREATE TABLE collections_new (
    id TEXT PRIMARY KEY,
    workspace_id TEXT NOT NULL REFERENCES workspaces(id),
    parent_id TEXT REFERENCES collections(id),
    name TEXT NOT NULL,
    sort_order INTEGER NOT NULL DEFAULT 0,
    version INTEGER NOT NULL DEFAULT 1,
    is_delete INTEGER NOT NULL DEFAULT 0,
    created_by TEXT NOT NULL DEFAULT 'local_user',
    created_at TEXT NOT NULL DEFAULT (datetime('now')),
    updated_by TEXT NOT NULL DEFAULT 'local_user',
    updated_at TEXT NOT NULL DEFAULT (datetime('now')),
    pre_script TEXT NOT NULL DEFAULT '',
    post_script TEXT NOT NULL DEFAULT '',
    description TEXT NOT NULL DEFAULT '',
    auth_type TEXT NOT NULL DEFAULT 'none' CHECK(auth_type IN ('none','basic','bearer','api_key','oauth2','jwt','digest','aws_sigv4')),
    auth_data TEXT NOT NULL DEFAULT '{}' CHECK(json_valid(auth_data)),
    grpc_metadata TEXT NOT NULL DEFAULT '[]',
    is_synced INTEGER NOT NULL DEFAULT 0
);

INSERT INTO collections_new (
    id, workspace_id, parent_id, name, sort_order, version, is_delete,
    created_by, created_at, updated_by, updated_at,
    pre_script, post_script, description, auth_type, auth_data, grpc_metadata, is_synced
)
SELECT
    id, workspace_id, parent_id, name, sort_order, version, is_delete,
    created_by, created_at, updated_by, updated_at,
    pre_script, post_script, description, auth_type, auth_data, grpc_metadata, is_synced
FROM collections;

DROP TABLE collections;
ALTER TABLE collections_new RENAME TO collections;

-- sqlite_master at 015 lists one index and no triggers for collections
CREATE INDEX idx_collections_workspace ON collections(workspace_id);
