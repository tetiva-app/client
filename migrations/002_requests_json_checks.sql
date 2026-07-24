-- Recreate requests table with JSON validation constraints
PRAGMA foreign_keys=OFF;

CREATE TABLE requests_new (
    id TEXT PRIMARY KEY,
    collection_id TEXT NOT NULL REFERENCES collections(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    protocol TEXT NOT NULL DEFAULT 'http',
    method TEXT NOT NULL DEFAULT 'GET',
    url TEXT NOT NULL DEFAULT '',
    headers TEXT NOT NULL DEFAULT '{}' CHECK(json_valid(headers)),
    body TEXT NOT NULL DEFAULT '',
    body_type TEXT NOT NULL DEFAULT 'none',
    grpc_service TEXT NOT NULL DEFAULT '',
    grpc_method TEXT NOT NULL DEFAULT '',
    grpc_proto_path TEXT NOT NULL DEFAULT '',
    grpc_metadata TEXT NOT NULL DEFAULT '{}' CHECK(json_valid(grpc_metadata)),
    pre_script TEXT NOT NULL DEFAULT '',
    post_script TEXT NOT NULL DEFAULT '',
    sort_order INTEGER NOT NULL DEFAULT 0,
    version INTEGER NOT NULL DEFAULT 1,
    is_delete INTEGER NOT NULL DEFAULT 0,
    created_by TEXT NOT NULL DEFAULT 'local_user',
    created_at TEXT NOT NULL DEFAULT (datetime('now')),
    updated_by TEXT NOT NULL DEFAULT 'local_user',
    updated_at TEXT NOT NULL DEFAULT (datetime('now'))
);

INSERT INTO requests_new SELECT * FROM requests;
DROP TABLE requests;
ALTER TABLE requests_new RENAME TO requests;

CREATE INDEX IF NOT EXISTS idx_requests_collection ON requests(collection_id);

PRAGMA foreign_keys=ON;
