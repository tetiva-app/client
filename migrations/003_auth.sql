-- Add authentication fields to requests table
ALTER TABLE requests ADD COLUMN auth_type TEXT DEFAULT 'none';
ALTER TABLE requests ADD COLUMN auth_data TEXT DEFAULT '{}' CHECK(json_valid(auth_data));
