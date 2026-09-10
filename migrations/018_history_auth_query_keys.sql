-- Query parameter names auth injected, so replay can strip them from the draft.
ALTER TABLE history ADD COLUMN auth_query_keys TEXT NOT NULL DEFAULT '[]';
