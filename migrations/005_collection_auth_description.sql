ALTER TABLE collections ADD COLUMN description TEXT NOT NULL DEFAULT '';
ALTER TABLE collections ADD COLUMN auth_type TEXT NOT NULL DEFAULT 'none' CHECK(auth_type IN ('none','basic','bearer','api_key'));
ALTER TABLE collections ADD COLUMN auth_data TEXT NOT NULL DEFAULT '{}' CHECK(json_valid(auth_data));
