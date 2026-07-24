-- Ensure no duplicate remote workspace links.
CREATE UNIQUE INDEX IF NOT EXISTS idx_workspaces_remote_id
    ON workspaces(remote_workspace_id) WHERE remote_workspace_id IS NOT NULL;
