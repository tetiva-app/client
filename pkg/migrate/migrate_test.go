package migrate_test

import (
	"database/sql"
	"embed"
	"testing"

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
