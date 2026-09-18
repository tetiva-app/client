package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"io/fs"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/tetiva-app/client/internal/domain"
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

func TestMigration020BackfillsRequestDescriptions(t *testing.T) {
	db := openMigrationTestDB(t)

	if err := migrate.Run(db, migrationsUpTo(t, 19), "."); err != nil {
		t.Fatalf("migrate to 019: %v", err)
	}

	seed := []string{
		`INSERT INTO workspaces (id, name, remote_workspace_id) VALUES ('ws1', 'Synced', 'remote-1')`,
		`INSERT INTO workspaces (id, name) VALUES ('ws2', 'Local only')`,
		`INSERT INTO collections (id, workspace_id, name) VALUES ('col1', 'ws1', 'Root')`,
		`INSERT INTO collections (id, workspace_id, name) VALUES ('col2', 'ws2', 'Root')`,
		`INSERT INTO requests (id, collection_id, name, description) VALUES ('req1', 'col1', 'Documented', '# Docs')`,
		`INSERT INTO requests (id, collection_id, name) VALUES ('req2', 'col1', 'Undocumented')`,
		`INSERT INTO requests (id, collection_id, name, description, is_delete) VALUES ('req3', 'col1', 'Deleted', 'gone', 1)`,
		`INSERT INTO requests (id, collection_id, name, description, is_draft) VALUES ('req4', 'col1', 'Draft', 'scratch', 1)`,
		`INSERT INTO requests (id, collection_id, name, description) VALUES ('req5', 'col2', 'Unlinked', 'local only')`,
		fmt.Sprintf(`INSERT INTO requests (id, collection_id, name, description) VALUES ('req6', 'col1', 'Oversized', '%s')`,
			strings.Repeat("x", domain.MaxDescriptionLen+1)),
	}
	for _, stmt := range seed {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatalf("seed %q: %v", stmt, err)
		}
	}

	if err := migrate.Run(db, migrations.FS, "."); err != nil {
		t.Fatalf("migrate to head: %v", err)
	}

	rows, err := db.Query(`SELECT entity_id, workspace_id, entity_type, action, status, operation_id, created_at FROM sync_queue`)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = rows.Close() }()

	var queued []string
	for rows.Next() {
		var entityID, wsID, entityType, action, status, opID, createdAt string
		if err := rows.Scan(&entityID, &wsID, &entityType, &action, &status, &opID, &createdAt); err != nil {
			t.Fatal(err)
		}
		queued = append(queued, entityID)
		if wsID != "ws1" || entityType != "request" || action != "update" || status != "pending" {
			t.Errorf("queued row: ws %q, type %q, action %q, status %q", wsID, entityType, action, status)
		}
		if !uuidLike.MatchString(opID) {
			t.Errorf("operation_id %q is not a v4 UUID", opID)
		}
		if createdAt == "" {
			t.Error("created_at is empty")
		}
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}

	if len(queued) != 1 || queued[0] != "req1" {
		t.Errorf("queued = %v, want [req1]: only a live, non-draft request under the description cap", queued)
	}

	pending, err := NewSyncQueueRepo(db).ListPending(context.Background(), "ws1", 10)
	if err != nil {
		t.Fatalf("ListPending: %v", err)
	}
	if len(pending) != 1 || pending[0].EntityID != "req1" || pending[0].CreatedAt.IsZero() {
		t.Errorf("ListPending = %+v, want one readable req1 entry", pending)
	}

	if err := migrate.Run(db, migrations.FS, "."); err != nil {
		t.Fatalf("second run: %v", err)
	}
	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM sync_queue`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Errorf("sync_queue rows after a second run: %d, want 1", count)
	}
}

var uuidLike = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)
