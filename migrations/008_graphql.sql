ALTER TABLE requests ADD COLUMN graphql_query TEXT DEFAULT '';
ALTER TABLE requests ADD COLUMN graphql_variables TEXT DEFAULT '';
ALTER TABLE requests ADD COLUMN graphql_schema_path TEXT DEFAULT '';
ALTER TABLE requests ADD COLUMN graphql_operation TEXT DEFAULT '';
