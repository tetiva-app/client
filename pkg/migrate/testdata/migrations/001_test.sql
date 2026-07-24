CREATE TABLE IF NOT EXISTS test_table (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL
);

INSERT OR IGNORE INTO test_table (id, name) VALUES ('1', 'hello');
