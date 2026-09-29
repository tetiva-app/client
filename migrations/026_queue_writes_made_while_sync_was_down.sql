-- created_by: pre-1.2.0 pulls never marked rows synced; 001 seeds one env id everywhere.
WITH RECURSIVE tree(id, workspace_id, depth) AS (
    SELECT c.id, c.workspace_id, 0 FROM collections c
    JOIN workspaces w ON w.id = c.workspace_id
    WHERE c.parent_id IS NULL AND c.is_delete = 0 AND w.is_delete = 0
      AND w.remote_workspace_id IS NOT NULL AND w.remote_workspace_id != ''
    UNION ALL
    SELECT c.id, t.workspace_id, t.depth + 1 FROM collections c JOIN tree t ON c.parent_id = t.id WHERE c.is_delete = 0
)
INSERT INTO sync_queue (workspace_id, entity_type, entity_id, action, operation_id, status, retry_count, created_at)
SELECT t.workspace_id, 'collection', c.id, 'create',
    lower(
        hex(randomblob(4)) || '-' ||
        hex(randomblob(2)) || '-4' ||
        substr(hex(randomblob(2)), 2) || '-' ||
        substr('89ab', (random() & 3) + 1, 1) || substr(hex(randomblob(2)), 2) || '-' ||
        hex(randomblob(6))
    ),
    'pending', 0, strftime('%Y-%m-%dT%H:%M:%SZ', 'now')
FROM tree t JOIN collections c ON c.id = t.id
WHERE c.is_synced = 0 AND c.created_by != 'sync'
  AND NOT EXISTS (SELECT 1 FROM sync_queue q WHERE q.entity_type = 'collection' AND q.entity_id = c.id)
ORDER BY t.depth, c.sort_order, c.id;

INSERT INTO sync_queue (workspace_id, entity_type, entity_id, action, operation_id, status, retry_count, created_at)
SELECT e.workspace_id, 'environment', e.id, 'create',
    lower(
        hex(randomblob(4)) || '-' ||
        hex(randomblob(2)) || '-4' ||
        substr(hex(randomblob(2)), 2) || '-' ||
        substr('89ab', (random() & 3) + 1, 1) || substr(hex(randomblob(2)), 2) || '-' ||
        hex(randomblob(6))
    ),
    'pending', 0, strftime('%Y-%m-%dT%H:%M:%SZ', 'now')
FROM environments e JOIN workspaces w ON w.id = e.workspace_id
WHERE w.is_delete = 0 AND w.remote_workspace_id IS NOT NULL AND w.remote_workspace_id != ''
  AND e.is_delete = 0 AND e.is_synced = 0 AND e.created_by != 'sync'
  AND e.id != '00000000-0000-4000-a000-000000000002'
  AND NOT EXISTS (SELECT 1 FROM sync_queue q WHERE q.entity_type = 'environment' AND q.entity_id = e.id)
ORDER BY e.created_at, e.id;

WITH RECURSIVE tree(id, workspace_id) AS (
    SELECT c.id, c.workspace_id FROM collections c
    JOIN workspaces w ON w.id = c.workspace_id
    WHERE c.parent_id IS NULL AND c.is_delete = 0 AND w.is_delete = 0
      AND w.remote_workspace_id IS NOT NULL AND w.remote_workspace_id != ''
    UNION ALL
    SELECT c.id, t.workspace_id FROM collections c JOIN tree t ON c.parent_id = t.id WHERE c.is_delete = 0
)
INSERT INTO sync_queue (workspace_id, entity_type, entity_id, action, operation_id, status, retry_count, created_at)
SELECT t.workspace_id, 'request', r.id, 'create',
    lower(
        hex(randomblob(4)) || '-' ||
        hex(randomblob(2)) || '-4' ||
        substr(hex(randomblob(2)), 2) || '-' ||
        substr('89ab', (random() & 3) + 1, 1) || substr(hex(randomblob(2)), 2) || '-' ||
        hex(randomblob(6))
    ),
    'pending', 0, strftime('%Y-%m-%dT%H:%M:%SZ', 'now')
FROM requests r JOIN tree t ON t.id = r.collection_id
WHERE r.is_delete = 0 AND r.is_draft = 0 AND r.is_synced = 0 AND r.created_by != 'sync'
  AND NOT EXISTS (SELECT 1 FROM sync_queue q WHERE q.entity_type = 'request' AND q.entity_id = r.id)
ORDER BY r.sort_order, r.id;

INSERT INTO sync_queue (workspace_id, entity_type, entity_id, action, operation_id, status, retry_count, created_at)
SELECT e.workspace_id, 'variable', v.id, 'create',
    lower(
        hex(randomblob(4)) || '-' ||
        hex(randomblob(2)) || '-4' ||
        substr(hex(randomblob(2)), 2) || '-' ||
        substr('89ab', (random() & 3) + 1, 1) || substr(hex(randomblob(2)), 2) || '-' ||
        hex(randomblob(6))
    ),
    'pending', 0, strftime('%Y-%m-%dT%H:%M:%SZ', 'now')
FROM variables v
JOIN environments e ON e.id = v.environment_id
JOIN workspaces w ON w.id = e.workspace_id
WHERE w.is_delete = 0 AND w.remote_workspace_id IS NOT NULL AND w.remote_workspace_id != ''
  AND e.is_delete = 0 AND e.id != '00000000-0000-4000-a000-000000000002'
  AND v.is_delete = 0 AND v.is_synced = 0 AND v.created_by != 'sync'
  AND NOT EXISTS (SELECT 1 FROM sync_queue q WHERE q.entity_type = 'variable' AND q.entity_id = v.id)
ORDER BY v.environment_id, v.sort_order, v.created_at, v.id;

WITH RECURSIVE tree(id, workspace_id) AS (
    SELECT c.id, c.workspace_id FROM collections c
    JOIN workspaces w ON w.id = c.workspace_id
    WHERE c.parent_id IS NULL AND c.is_delete = 0 AND w.is_delete = 0
      AND w.remote_workspace_id IS NOT NULL AND w.remote_workspace_id != ''
    UNION ALL
    SELECT c.id, t.workspace_id FROM collections c JOIN tree t ON c.parent_id = t.id WHERE c.is_delete = 0
)
INSERT INTO sync_queue (workspace_id, entity_type, entity_id, action, operation_id, status, retry_count, created_at)
SELECT t.workspace_id, 'response_example', x.id, 'create',
    lower(
        hex(randomblob(4)) || '-' ||
        hex(randomblob(2)) || '-4' ||
        substr(hex(randomblob(2)), 2) || '-' ||
        substr('89ab', (random() & 3) + 1, 1) || substr(hex(randomblob(2)), 2) || '-' ||
        hex(randomblob(6))
    ),
    'pending', 0, strftime('%Y-%m-%dT%H:%M:%SZ', 'now')
FROM response_examples x
JOIN requests r ON r.id = x.request_id
JOIN tree t ON t.id = r.collection_id
WHERE x.is_delete = 0 AND x.is_synced = 0 AND x.created_by != 'sync'
  AND r.is_delete = 0 AND r.is_draft = 0
  AND NOT EXISTS (SELECT 1 FROM sync_queue q WHERE q.entity_type = 'response_example' AND q.entity_id = x.id)
ORDER BY x.sort_order, x.id;
