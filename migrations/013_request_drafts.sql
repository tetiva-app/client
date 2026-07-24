-- Replay-as-draft mechanism for request history.
-- Drafts are temporary requests created from a history record. They are
-- excluded from regular List operations, never synced/exported, and are
-- hard-deleted when the user closes their tab (or on app start cleanup).
ALTER TABLE requests ADD COLUMN is_draft INTEGER NOT NULL DEFAULT 0;
CREATE INDEX IF NOT EXISTS idx_requests_drafts ON requests(is_draft) WHERE is_draft = 1;
