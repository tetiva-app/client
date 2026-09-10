package sqlite

import (
	"database/sql"
	"io/fs"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/tetiva-app/client/migrations"
	"github.com/tetiva-app/client/pkg/migrate"
)

// Lets a test build a database at an older schema with the real runner.
func migrationsUpTo(t *testing.T, max int) fs.FS {
	t.Helper()
	entries, err := fs.ReadDir(migrations.FS, ".")
	if err != nil {
		t.Fatal(err)
	}
	out := fstest.MapFS{}
	for _, e := range entries {
		name := e.Name()
		version, err := strconv.Atoi(strings.SplitN(name, "_", 2)[0])
		if err != nil || version > max {
			continue
		}
		data, err := fs.ReadFile(migrations.FS, name)
		if err != nil {
			t.Fatal(err)
		}
		out[name] = &fstest.MapFile{Data: data}
	}
	return out
}

func openMigrationTestDB(t *testing.T) *sql.DB {
	t.Helper()
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

func TestMigration016RebuildsCollectionsWithoutDataLoss(t *testing.T) {
	db := openMigrationTestDB(t)

	if err := migrate.Run(db, migrationsUpTo(t, 15), "."); err != nil {
		t.Fatalf("migrate to 015: %v", err)
	}

	seed := []string{
		`INSERT INTO workspaces (id, name) VALUES ('ws1', 'Test')`,
		`INSERT INTO collections (id, workspace_id, parent_id, name, auth_type, auth_data, description)
		 VALUES ('col1', 'ws1', NULL, 'Root', 'basic', '{"username":"u"}', 'root collection')`,
		`INSERT INTO collections (id, workspace_id, parent_id, name, auth_type)
		 VALUES ('col2', 'ws1', 'col1', 'Nested', 'bearer')`,
		`INSERT INTO requests (id, collection_id, name, url) VALUES ('req1', 'col1', 'One', 'https://a')`,
		`INSERT INTO requests (id, collection_id, name, url) VALUES ('req2', 'col2', 'Two', 'https://b')`,
		`INSERT INTO requests (id, collection_id, name, url) VALUES ('req3', 'col2', 'Three', 'https://c')`,
		`INSERT INTO history (id, request_id, workspace_id, protocol, method, url)
		 VALUES ('h1', 'req1', 'ws1', 'http', 'GET', 'https://a')`,
		`INSERT INTO history (id, request_id, workspace_id, protocol, method, url)
		 VALUES ('h2', NULL, 'ws1', 'http', 'POST', 'https://b')`,
	}
	for _, stmt := range seed {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatalf("seed %q: %v", stmt, err)
		}
	}

	if err := migrate.Run(db, migrations.FS, "."); err != nil {
		t.Fatalf("migrate to head: %v", err)
	}

	counts := map[string]int{"collections": 2, "requests": 3, "history": 2}
	for table, want := range counts {
		var got int
		if err := db.QueryRow("SELECT COUNT(*) FROM " + table).Scan(&got); err != nil {
			t.Fatalf("count %s: %v", table, err)
		}
		if got != want {
			t.Errorf("%s rows: got %d, want %d", table, got, want)
		}
	}

	var parent, authType, authData, description string
	if err := db.QueryRow(`SELECT parent_id, auth_type, auth_data, description FROM collections WHERE id = 'col2'`).
		Scan(&parent, &authType, &authData, &description); err != nil {
		t.Fatalf("read col2: %v", err)
	}
	if parent != "col1" || authType != "bearer" {
		t.Errorf("col2 after rebuild: parent %q, auth_type %q", parent, authType)
	}

	rows, err := db.Query("PRAGMA foreign_key_check")
	if err != nil {
		t.Fatal(err)
	}
	if rows.Next() {
		_ = rows.Close()
		t.Error("foreign_key_check reported violations")
	} else {
		_ = rows.Close()
	}

	var idx string
	if err := db.QueryRow(`SELECT name FROM sqlite_master WHERE type='index' AND name='idx_collections_workspace'`).Scan(&idx); err != nil {
		t.Errorf("idx_collections_workspace missing: %v", err)
	}

	if _, err := db.Exec(`INSERT INTO collections (id, workspace_id, name, auth_type) VALUES ('col3', 'ws1', 'OAuth', 'oauth2')`); err != nil {
		t.Errorf("widened CHECK rejects oauth2: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO collections (id, workspace_id, name, auth_type) VALUES ('col4', 'ws1', 'Bad', 'hawk')`); err == nil {
		t.Error("CHECK accepted an unknown auth type")
	}
	if _, err := db.Exec(`INSERT INTO collections (id, workspace_id, name, auth_data) VALUES ('col5', 'ws1', 'Bad JSON', 'not json')`); err == nil {
		t.Error("json_valid(auth_data) CHECK was lost")
	}
	if _, err := db.Exec(`INSERT INTO requests (id, collection_id, name, url) VALUES ('req9', 'nope', 'Orphan', 'https://x')`); err == nil {
		t.Error("requests no longer reference the rebuilt collections table")
	}

	// 017/018 land on top of the rebuild.
	if _, err := db.Exec(`INSERT INTO auth_tokens (workspace_id, owner_kind, owner_id, config_hash, access_token, obtained_at, updated_at)
		VALUES ('ws1', 'request', 'req1', 'hash', 'token', '2026-09-05T00:00:00Z', '2026-09-05T00:00:00Z')`); err != nil {
		t.Errorf("auth_tokens insert: %v", err)
	}
	var keys string
	if err := db.QueryRow(`SELECT auth_query_keys FROM history WHERE id = 'h1'`).Scan(&keys); err != nil {
		t.Fatalf("read auth_query_keys: %v", err)
	}
	if keys != "[]" {
		t.Errorf("auth_query_keys default: got %q, want %q", keys, "[]")
	}
}

func TestMigration016CascadeWouldHaveDeletedRequests(t *testing.T) {
	db := openMigrationTestDB(t)
	if err := migrate.Run(db, migrations.FS, "."); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	for _, stmt := range []string{
		`INSERT INTO workspaces (id, name) VALUES ('ws1', 'Test')`,
		`INSERT INTO collections (id, workspace_id, name) VALUES ('col1', 'ws1', 'Root')`,
		`INSERT INTO requests (id, collection_id, name, url) VALUES ('req1', 'col1', 'One', 'https://a')`,
	} {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatal(err)
		}
	}

	// The cascade the fk-off mode has to sidestep is still armed after the rebuild.
	if _, err := db.Exec(`DELETE FROM collections WHERE id = 'col1'`); err != nil {
		t.Fatal(err)
	}
	var n int
	if err := db.QueryRow(`SELECT COUNT(*) FROM requests`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Errorf("ON DELETE CASCADE lost in the rebuild: %d requests survived", n)
	}
}
