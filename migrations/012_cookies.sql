CREATE TABLE cookies (
    id           TEXT PRIMARY KEY,
    workspace_id TEXT NOT NULL,
    domain       TEXT NOT NULL,              -- normalized lowercase host
    host_only    INTEGER NOT NULL DEFAULT 0, -- 1 = match exact host only (RFC 6265 §5.3.6)
    path         TEXT NOT NULL DEFAULT '/',
    name         TEXT NOT NULL,
    value        TEXT NOT NULL,
    expires_at   INTEGER,                    -- unix seconds; NULL = session cookie
    http_only    INTEGER NOT NULL DEFAULT 0,
    secure       INTEGER NOT NULL DEFAULT 0,
    same_site    TEXT NOT NULL DEFAULT '',
    created_at   INTEGER NOT NULL,
    updated_at   INTEGER NOT NULL,
    UNIQUE(workspace_id, domain, path, name)
);

CREATE INDEX idx_cookies_workspace_domain ON cookies(workspace_id, domain);
