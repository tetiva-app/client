-- Descriptions predating the sync contract (015) were never pushed; re-offer them once.
INSERT INTO sync_queue (workspace_id, entity_type, entity_id, action, operation_id, status, retry_count, created_at)
SELECT
    c.workspace_id,
    'request',
    r.id,
    'update',
    lower(
        hex(randomblob(4)) || '-' ||
        hex(randomblob(2)) || '-4' ||
        substr(hex(randomblob(2)), 2) || '-' ||
        substr('89ab', (random() & 3) + 1, 1) || substr(hex(randomblob(2)), 2) || '-' ||
        hex(randomblob(6))
    ),
    'pending',
    0,
    strftime('%Y-%m-%dT%H:%M:%SZ', 'now')
FROM requests r
JOIN collections c ON c.id = r.collection_id
JOIN workspaces w ON w.id = c.workspace_id
WHERE r.description != ''
  AND length(CAST(r.description AS BLOB)) <= 16384 -- domain.MaxDescriptionLen
  AND r.is_delete = 0
  AND r.is_draft = 0
  AND c.is_delete = 0
  AND w.is_delete = 0
  AND w.remote_workspace_id IS NOT NULL
  AND w.remote_workspace_id != '';
