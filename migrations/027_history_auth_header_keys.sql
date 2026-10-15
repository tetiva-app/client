-- Header names auth injected, so replay can strip them from the draft.
ALTER TABLE history ADD COLUMN auth_header_keys TEXT NOT NULL DEFAULT '[]';
