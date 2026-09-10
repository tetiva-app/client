-- Request-level documentation (Markdown), mirroring collections.description.
-- Outside the sync contract: RequestData has no such field, so the sync engine
-- keeps the local value instead of overwriting it with an empty one.
ALTER TABLE requests ADD COLUMN description TEXT NOT NULL DEFAULT '';
