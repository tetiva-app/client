-- Add pre/post script columns to collections
ALTER TABLE collections ADD COLUMN pre_script TEXT NOT NULL DEFAULT '';
ALTER TABLE collections ADD COLUMN post_script TEXT NOT NULL DEFAULT '';
