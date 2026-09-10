-- Forced re-login on the release that moves sign-in to the browser: the flags are
-- written before the cleanup, so a crash between them only repeats the cleanup.
ALTER TABLE sync_config ADD COLUMN auth_generation INTEGER NOT NULL DEFAULT 0;
ALTER TABLE sync_config ADD COLUMN reauth_required INTEGER NOT NULL DEFAULT 0;
