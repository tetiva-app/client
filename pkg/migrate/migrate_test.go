package migrate_test

import (
	"database/sql"
	"embed"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"

	_ "modernc.org/sqlite"

	"github.com/tetiva-app/client/pkg/migrate"
)

//go:embed testdata/migrations/*.sql
var testFS embed.FS

func TestRun(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()

	if err := migrate.Run(db, testFS, "testdata/migrations"); err != nil {
		t.Fatalf("first run: %v", err)
	}

	var name string
	if err := db.QueryRow("SELECT name FROM test_table WHERE id = '1'").Scan(&name); err != nil {
		t.Fatalf("query test_table: %v", err)
	}
	if name != "hello" {
		t.Errorf("expected name 'hello', got %q", name)
	}

	var version int
	if err := db.QueryRow("SELECT MAX(version) FROM schema_migrations").Scan(&version); err != nil {
		t.Fatalf("query version: %v", err)
	}
	if version != 1 {
		t.Errorf("expected version 1, got %d", version)
	}

	if err := migrate.Run(db, testFS, "testdata/migrations"); err != nil {
		t.Fatalf("second run (idempotent): %v", err)
	}
}

func openFileDB(t *testing.T) *sql.DB {
	t.Helper()
	// A file DB with a single connection: ":memory:" gives every pooled connection
	// its own empty database, and the fk-off path takes a dedicated one.
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	if _, err := db.Exec("PRAGMA foreign_keys=ON"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

const parentChildSchema = `CREATE TABLE parent (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL CHECK(name IN ('a','b'))
);
CREATE TABLE child (
    id TEXT PRIMARY KEY,
    parent_id TEXT NOT NULL REFERENCES parent(id) ON DELETE CASCADE
);
CREATE INDEX idx_child_parent ON child(parent_id);`

const parentRebuild = `-- migrate:fk-off
CREATE TABLE parent_new (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL CHECK(name IN ('a','b','c'))
);
INSERT INTO parent_new (id, name) SELECT id, name FROM parent;
DROP TABLE parent;
ALTER TABLE parent_new RENAME TO parent;`

func seedParentChild(t *testing.T, db *sql.DB) {
	t.Helper()
	if _, err := db.Exec("INSERT INTO parent (id, name) VALUES ('p1', 'a'), ('p2', 'b')"); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec("INSERT INTO child (id, parent_id) VALUES ('c1', 'p1'), ('c2', 'p1'), ('c3', 'p2')"); err != nil {
		t.Fatal(err)
	}
}

func countRows(t *testing.T, db *sql.DB, table string) int {
	t.Helper()
	var n int
	if err := db.QueryRow("SELECT COUNT(*) FROM " + table).Scan(&n); err != nil {
		t.Fatalf("count %s: %v", table, err)
	}
	return n
}

func TestRunFKOffKeepsCascadingChildren(t *testing.T) {
	db := openFileDB(t)

	base := fstest.MapFS{"m/001_init.sql": &fstest.MapFile{Data: []byte(parentChildSchema)}}
	if err := migrate.Run(db, base, "m"); err != nil {
		t.Fatalf("base migration: %v", err)
	}
	seedParentChild(t, db)

	full := fstest.MapFS{
		"m/001_init.sql":    &fstest.MapFile{Data: []byte(parentChildSchema)},
		"m/002_rebuild.sql": &fstest.MapFile{Data: []byte(parentRebuild)},
	}
	if err := migrate.Run(db, full, "m"); err != nil {
		t.Fatalf("fk-off migration: %v", err)
	}

	if got := countRows(t, db, "child"); got != 3 {
		t.Errorf("child rows: got %d, want 3", got)
	}
	if got := countRows(t, db, "parent"); got != 2 {
		t.Errorf("parent rows: got %d, want 2", got)
	}
	if _, err := db.Exec("INSERT INTO parent (id, name) VALUES ('p3', 'c')"); err != nil {
		t.Errorf("widened check should accept 'c': %v", err)
	}

	var version int
	if err := db.QueryRow("SELECT MAX(version) FROM schema_migrations").Scan(&version); err != nil {
		t.Fatal(err)
	}
	if version != 2 {
		t.Errorf("version: got %d, want 2", version)
	}
}

func TestRunFKOffRestoresForeignKeys(t *testing.T) {
	db := openFileDB(t)

	full := fstest.MapFS{
		"m/001_init.sql":    &fstest.MapFile{Data: []byte(parentChildSchema)},
		"m/002_rebuild.sql": &fstest.MapFile{Data: []byte(parentRebuild)},
	}
	if err := migrate.Run(db, full, "m"); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	seedParentChild(t, db)

	var fk int
	if err := db.QueryRow("PRAGMA foreign_keys").Scan(&fk); err != nil {
		t.Fatal(err)
	}
	if fk != 1 {
		t.Errorf("foreign_keys after fk-off migration: got %d, want 1", fk)
	}
	if _, err := db.Exec("INSERT INTO child (id, parent_id) VALUES ('c9', 'nope')"); err == nil {
		t.Error("foreign key enforcement is off: an orphan child was accepted")
	}
}

func TestRunFKOffFailureLeavesSchemaIntact(t *testing.T) {
	tests := []struct {
		name   string
		script string
		errIs  string
	}{
		{
			name: "statement fails",
			script: `-- migrate:fk-off
CREATE TABLE parent_new (id TEXT PRIMARY KEY, name TEXT NOT NULL CHECK(name IN ('a','b','c')));
INSERT INTO parent_new (id, name) SELECT id, 'zzz' FROM parent;
DROP TABLE parent;
ALTER TABLE parent_new RENAME TO parent;`,
			errIs: "exec",
		},
		{
			name: "foreign_key_check finds orphans",
			script: `-- migrate:fk-off
CREATE TABLE parent_new (id TEXT PRIMARY KEY, name TEXT NOT NULL CHECK(name IN ('a','b','c')));
INSERT INTO parent_new (id, name) SELECT id, name FROM parent WHERE id <> 'p1';
DROP TABLE parent;
ALTER TABLE parent_new RENAME TO parent;`,
			errIs: "foreign key",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db := openFileDB(t)
			base := fstest.MapFS{"m/001_init.sql": &fstest.MapFile{Data: []byte(parentChildSchema)}}
			if err := migrate.Run(db, base, "m"); err != nil {
				t.Fatalf("base migration: %v", err)
			}
			seedParentChild(t, db)

			full := fstest.MapFS{
				"m/001_init.sql":    &fstest.MapFile{Data: []byte(parentChildSchema)},
				"m/002_rebuild.sql": &fstest.MapFile{Data: []byte(tt.script)},
			}
			err := migrate.Run(db, full, "m")
			if err == nil {
				t.Fatal("expected an error")
			}
			if !strings.Contains(err.Error(), tt.errIs) {
				t.Errorf("error %q does not mention %q", err, tt.errIs)
			}

			var version int
			if err := db.QueryRow("SELECT MAX(version) FROM schema_migrations").Scan(&version); err != nil {
				t.Fatal(err)
			}
			if version != 1 {
				t.Errorf("version: got %d, want 1", version)
			}
			if got := countRows(t, db, "parent"); got != 2 {
				t.Errorf("parent rows: got %d, want 2", got)
			}
			if got := countRows(t, db, "child"); got != 3 {
				t.Errorf("child rows: got %d, want 3", got)
			}
			if _, err := db.Exec("INSERT INTO parent (id, name) VALUES ('p3', 'c')"); err == nil {
				t.Error("old table was replaced despite the failure")
			}
			var fk int
			if err := db.QueryRow("PRAGMA foreign_keys").Scan(&fk); err != nil {
				t.Fatal(err)
			}
			if fk != 1 {
				t.Errorf("foreign_keys after a failed fk-off migration: got %d, want 1", fk)
			}
		})
	}
}

func TestRunNormalFileFailureRollsBack(t *testing.T) {
	db := openFileDB(t)

	broken := fstest.MapFS{"m/001_init.sql": &fstest.MapFile{Data: []byte(
		"CREATE TABLE t (id TEXT PRIMARY KEY);\nINSERT INTO nosuchtable (id) VALUES ('1');")}}
	if err := migrate.Run(db, broken, "m"); err == nil {
		t.Fatal("expected an error")
	}

	var version int
	if err := db.QueryRow("SELECT COALESCE(MAX(version), 0) FROM schema_migrations").Scan(&version); err != nil {
		t.Fatal(err)
	}
	if version != 0 {
		t.Errorf("version: got %d, want 0", version)
	}
	var name string
	err := db.QueryRow("SELECT name FROM sqlite_master WHERE type='table' AND name='t'").Scan(&name)
	if !errors.Is(err, sql.ErrNoRows) {
		t.Errorf("table t survived a failed migration: %v", err)
	}
}

func TestRunFKOffToleratesPreexistingViolations(t *testing.T) {
	db := openFileDB(t)

	schema := parentChildSchema + "\nCREATE TABLE note (id TEXT PRIMARY KEY, child_id TEXT REFERENCES child(id));"
	base := fstest.MapFS{"m/001_init.sql": &fstest.MapFile{Data: []byte(schema)}}
	if err := migrate.Run(db, base, "m"); err != nil {
		t.Fatalf("base migration: %v", err)
	}
	seedParentChild(t, db)

	// Damage unrelated to the rebuilt table, as a real database carries after a
	// hard delete: a note pointing at a child that is gone.
	if _, err := db.Exec("PRAGMA foreign_keys=OFF"); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec("INSERT INTO note (id, child_id) VALUES ('n1', 'gone')"); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec("PRAGMA foreign_keys=ON"); err != nil {
		t.Fatal(err)
	}

	full := fstest.MapFS{
		"m/001_init.sql":    &fstest.MapFile{Data: []byte(schema)},
		"m/002_rebuild.sql": &fstest.MapFile{Data: []byte(parentRebuild)},
	}
	if err := migrate.Run(db, full, "m"); err != nil {
		t.Fatalf("fk-off migration refused to run over old damage: %v", err)
	}

	if got := countRows(t, db, "child"); got != 3 {
		t.Errorf("child rows: got %d, want 3", got)
	}
	var version int
	if err := db.QueryRow("SELECT MAX(version) FROM schema_migrations").Scan(&version); err != nil {
		t.Fatal(err)
	}
	if version != 2 {
		t.Errorf("version: got %d, want 2", version)
	}
}
