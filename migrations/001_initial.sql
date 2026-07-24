-- Workspaces
CREATE TABLE IF NOT EXISTS workspaces (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    version INTEGER NOT NULL DEFAULT 1,
    is_delete INTEGER NOT NULL DEFAULT 0,
    created_by TEXT NOT NULL DEFAULT 'local_user',
    created_at TEXT NOT NULL DEFAULT (datetime('now')),
    updated_by TEXT NOT NULL DEFAULT 'local_user',
    updated_at TEXT NOT NULL DEFAULT (datetime('now'))
);

-- Collections (nested via parent_id)
CREATE TABLE IF NOT EXISTS collections (
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
    updated_at TEXT NOT NULL DEFAULT (datetime('now'))
);
CREATE INDEX IF NOT EXISTS idx_collections_workspace ON collections(workspace_id);

-- Requests (HTTP + gRPC)
CREATE TABLE IF NOT EXISTS requests (
    id TEXT PRIMARY KEY,
    collection_id TEXT NOT NULL REFERENCES collections(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    protocol TEXT NOT NULL DEFAULT 'http',
    -- HTTP fields
    method TEXT NOT NULL DEFAULT 'GET',
    url TEXT NOT NULL DEFAULT '',
    headers TEXT NOT NULL DEFAULT '{}',
    body TEXT NOT NULL DEFAULT '',
    body_type TEXT NOT NULL DEFAULT 'none',
    -- gRPC fields
    grpc_service TEXT NOT NULL DEFAULT '',
    grpc_method TEXT NOT NULL DEFAULT '',
    grpc_proto_path TEXT NOT NULL DEFAULT '',
    grpc_metadata TEXT NOT NULL DEFAULT '{}',
    -- Scripts
    pre_script TEXT NOT NULL DEFAULT '',
    post_script TEXT NOT NULL DEFAULT '',
    -- Common
    sort_order INTEGER NOT NULL DEFAULT 0,
    version INTEGER NOT NULL DEFAULT 1,
    is_delete INTEGER NOT NULL DEFAULT 0,
    created_by TEXT NOT NULL DEFAULT 'local_user',
    created_at TEXT NOT NULL DEFAULT (datetime('now')),
    updated_by TEXT NOT NULL DEFAULT 'local_user',
    updated_at TEXT NOT NULL DEFAULT (datetime('now'))
);
CREATE INDEX IF NOT EXISTS idx_requests_collection ON requests(collection_id);

-- Environments
CREATE TABLE IF NOT EXISTS environments (
    id TEXT PRIMARY KEY,
    workspace_id TEXT NOT NULL REFERENCES workspaces(id),
    name TEXT NOT NULL,
    is_active INTEGER NOT NULL DEFAULT 0,
    version INTEGER NOT NULL DEFAULT 1,
    is_delete INTEGER NOT NULL DEFAULT 0,
    created_by TEXT NOT NULL DEFAULT 'local_user',
    created_at TEXT NOT NULL DEFAULT (datetime('now')),
    updated_by TEXT NOT NULL DEFAULT 'local_user',
    updated_at TEXT NOT NULL DEFAULT (datetime('now'))
);
CREATE INDEX IF NOT EXISTS idx_environments_workspace ON environments(workspace_id);

-- Variables (separate table, not JSON blob)
CREATE TABLE IF NOT EXISTS variables (
    id TEXT PRIMARY KEY,
    environment_id TEXT NOT NULL REFERENCES environments(id) ON DELETE CASCADE,
    key TEXT NOT NULL,
    value TEXT NOT NULL DEFAULT '',
    is_secret INTEGER NOT NULL DEFAULT 0,
    enabled INTEGER NOT NULL DEFAULT 1,
    sort_order INTEGER NOT NULL DEFAULT 0,
    version INTEGER NOT NULL DEFAULT 1,
    is_delete INTEGER NOT NULL DEFAULT 0,
    created_by TEXT NOT NULL DEFAULT 'local_user',
    created_at TEXT NOT NULL DEFAULT (datetime('now')),
    updated_by TEXT NOT NULL DEFAULT 'local_user',
    updated_at TEXT NOT NULL DEFAULT (datetime('now'))
);
CREATE INDEX IF NOT EXISTS idx_variables_env ON variables(environment_id);

-- History (execution log — immutable)
CREATE TABLE IF NOT EXISTS history (
    id TEXT PRIMARY KEY,
    request_id TEXT REFERENCES requests(id) ON DELETE SET NULL,
    workspace_id TEXT NOT NULL REFERENCES workspaces(id),
    protocol TEXT NOT NULL,
    method TEXT NOT NULL,
    url TEXT NOT NULL,
    request_headers TEXT NOT NULL DEFAULT '{}',
    request_body TEXT NOT NULL DEFAULT '',
    response_status INTEGER,
    response_headers TEXT NOT NULL DEFAULT '{}',
    response_body TEXT NOT NULL DEFAULT '',
    response_size INTEGER NOT NULL DEFAULT 0,
    duration_ms INTEGER NOT NULL DEFAULT 0,
    error_message TEXT NOT NULL DEFAULT '',
    created_at TEXT NOT NULL DEFAULT (datetime('now'))
);
CREATE INDEX IF NOT EXISTS idx_history_workspace ON history(workspace_id);
CREATE INDEX IF NOT EXISTS idx_history_created ON history(created_at DESC);

-- Default data
INSERT OR IGNORE INTO workspaces (id, name) VALUES ('00000000-0000-4000-a000-000000000001', 'Default Workspace');
INSERT OR IGNORE INTO environments (id, workspace_id, name, is_active)
VALUES ('00000000-0000-4000-a000-000000000002', '00000000-0000-4000-a000-000000000001', 'Default', 1);
