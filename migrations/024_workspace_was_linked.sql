-- Only a never-linked workspace uploads its rows on link: others hold an earlier account's.
ALTER TABLE workspaces ADD COLUMN was_linked INTEGER NOT NULL DEFAULT 0;
UPDATE workspaces SET was_linked = 1
WHERE (remote_workspace_id IS NOT NULL AND remote_workspace_id != '')
   OR last_sync_seq > 0
   OR EXISTS (SELECT 1 FROM collections c WHERE c.workspace_id = workspaces.id AND c.is_synced = 1)
   OR EXISTS (SELECT 1 FROM environments e WHERE e.workspace_id = workspaces.id AND e.is_synced = 1);
