-- NULL: values this device never knew (older rows and peers); sync leaves them out.
ALTER TABLE requests ADD COLUMN path_variables TEXT CHECK(path_variables IS NULL OR json_valid(path_variables));
