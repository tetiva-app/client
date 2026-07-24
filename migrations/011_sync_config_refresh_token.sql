-- Add refresh_token column to sync_config for fallback when OS keychain is unavailable.
ALTER TABLE sync_config ADD COLUMN refresh_token TEXT DEFAULT '';
