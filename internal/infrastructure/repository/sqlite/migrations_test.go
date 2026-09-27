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

func TestMigration021(t *testing.T) {
	db := openMigrationTestDB(t)

	if err := migrate.Run(db, migrationsUpTo(t, 20), "."); err != nil {
		t.Fatalf("migrate to 020: %v", err)
	}

	for _, stmt := range []string{
		`INSERT INTO workspaces (id, name, remote_workspace_id, last_sync_seq) VALUES ('linked', 'Linked', 'r1', 42)`,
		`INSERT INTO workspaces (id, name, remote_workspace_id, last_sync_seq) VALUES ('linked-zero', 'Never pulled', 'r2', 0)`,
		`INSERT INTO workspaces (id, name, last_sync_seq) VALUES ('unlinked', 'Local', 7)`,
		`INSERT INTO workspaces (id, name, remote_workspace_id, last_sync_seq) VALUES ('blank-remote', 'Blank', '', 5)`,
		`INSERT INTO workspaces (id, name, remote_workspace_id, last_sync_seq, is_delete) VALUES ('deleted', 'Deleted', 'r3', 9, 1)`,
		`INSERT INTO sync_queue (workspace_id, entity_type, entity_id, action, operation_id, status, retry_count, created_at)
		 VALUES ('linked', 'request', 'q1', 'update', 'op', 'failed', 3, '2026-09-25T00:00:00Z')`,
	} {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatalf("seed %q: %v", stmt, err)
		}
	}

	if err := migrate.Run(db, migrations.FS, "."); err != nil {
		t.Fatalf("migrate to head: %v", err)
	}

	want := map[string]int{"linked": 1, "linked-zero": 0, "unlinked": 0, "blank-remote": 0, "deleted": 0}
	for id, wantFlag := range want {
		var flag, incomplete, failed int
		var backfillToken, pageToken string
		if err := db.QueryRow(`SELECT examples_backfill_pending, examples_backfill_token, sync_page_token,
			examples_backfill_incomplete, examples_backfill_failed_passes FROM workspaces WHERE id = ?`, id).
			Scan(&flag, &backfillToken, &pageToken, &incomplete, &failed); err != nil {
			t.Fatalf("read %s: %v", id, err)
		}
		if flag != wantFlag {
			t.Errorf("%s: examples_backfill_pending = %d, want %d", id, flag, wantFlag)
		}
		if incomplete != 0 || failed != 0 {
			t.Errorf("%s: a pass that never ran = incomplete %d, failed %d", id, incomplete, failed)
		}
		if backfillToken != "" || pageToken != "" {
			t.Errorf("%s: tokens = %q, %q, want empty", id, backfillToken, pageToken)
		}
	}

	if _, err := db.Exec(`INSERT INTO response_examples (id, request_id, workspace_id, name, headers, created_at, updated_at)
		VALUES ('bad', 'no-such-request', 'linked', 'Bad', 'not json', '2026-09-25T00:00:00Z', '2026-09-25T00:00:00Z')`); err == nil {
		t.Error("CHECK accepted headers that are not JSON")
	}

	if _, err := db.Exec(`INSERT INTO response_examples (id, request_id, workspace_id, name, created_at, updated_at)
		VALUES ('ok', 'no-such-request', 'linked', 'OK', '2026-09-25T00:00:00Z', '2026-09-25T00:00:00Z')`); err != nil {
		t.Fatalf("insert with defaults: %v", err)
	}
	var headers, protocol string
	var version, isDelete, isSynced int
	if err := db.QueryRow(`SELECT headers, protocol, version, is_delete, is_synced FROM response_examples WHERE id = 'ok'`).
		Scan(&headers, &protocol, &version, &isDelete, &isSynced); err != nil {
		t.Fatal(err)
	}
	if headers != "[]" || protocol != "http" || version != 1 || isDelete != 0 || isSynced != 0 {
		t.Errorf("defaults = (%q, %q, %d, %d, %d)", headers, protocol, version, isDelete, isSynced)
	}

	var idx string
	if err := db.QueryRow(`SELECT name FROM sqlite_master WHERE type='index' AND name='idx_response_examples_request'`).Scan(&idx); err != nil {
		t.Errorf("idx_response_examples_request missing: %v", err)
	}

	var retries, defers int
	if err := db.QueryRow(`SELECT retry_count, defer_count FROM sync_queue WHERE entity_id = 'q1'`).Scan(&retries, &defers); err != nil {
		t.Fatal(err)
	}
	if retries != 3 || defers != 0 {
		t.Errorf("queued row = retry_count %d, defer_count %d; want 3, 0", retries, defers)
	}

	seen := `INSERT INTO sync_snapshot_seen (workspace_id, walk, entity_type, entity_id) VALUES ('linked', 'snapshot', 'request', 'q1')`
	if _, err := db.Exec(seen); err != nil {
		t.Fatalf("record a seen entity: %v", err)
	}
	if _, err := db.Exec(seen); err == nil {
		t.Error("one walk recorded the same entity twice")
	}
}

func TestMigration022(t *testing.T) {
	db := openMigrationTestDB(t)

	if err := migrate.Run(db, migrationsUpTo(t, 21), "."); err != nil {
		t.Fatalf("migrate to 021: %v", err)
	}
	if err := migrate.Run(db, migrations.FS, "."); err != nil {
		t.Fatalf("migrate to head: %v", err)
	}

	if _, err := db.Exec(`INSERT INTO publications (collection_id, workspace_id, owner_key, publication_id, created_at, updated_at)
		VALUES ('c1', 'w1', 'owner', 'p1', '2026-09-25T00:00:00Z', '2026-09-25T00:00:00Z')`); err != nil {
		t.Fatalf("insert with defaults: %v", err)
	}
	var (
		slug, visibility, status, counters, refreshed string
		settings                                      sql.NullString
		revision, blocked, pending, canManage         int
	)
	if err := db.QueryRow(`SELECT slug, visibility, status, counters, refreshed_at, settings, revision, blocked, pending_unpublish, can_manage
		FROM publications WHERE collection_id = 'c1'`).
		Scan(&slug, &visibility, &status, &counters, &refreshed, &settings, &revision, &blocked, &pending, &canManage); err != nil {
		t.Fatal(err)
	}
	if slug != "" || visibility != "" || status != "" || counters != "{}" || refreshed != "" || settings.Valid ||
		revision != 0 || blocked != 0 || pending != 0 || canManage != 0 {
		t.Errorf("defaults = (%q, %q, %q, %q, %q, %v, %d, %d, %d, %d)",
			slug, visibility, status, counters, refreshed, settings, revision, blocked, pending, canManage)
	}

	for name, stmt := range map[string]string{
		"settings": `INSERT INTO publications (collection_id, workspace_id, owner_key, publication_id, settings, created_at, updated_at)
			VALUES ('c2', 'w1', 'owner', 'p2', 'not json', '2026-09-25T00:00:00Z', '2026-09-25T00:00:00Z')`,
		"counters": `INSERT INTO publications (collection_id, workspace_id, owner_key, publication_id, counters, created_at, updated_at)
			VALUES ('c3', 'w1', 'owner', 'p3', 'not json', '2026-09-25T00:00:00Z', '2026-09-25T00:00:00Z')`,
	} {
		if _, err := db.Exec(stmt); err == nil {
			t.Errorf("CHECK accepted %s that is not JSON", name)
		}
	}

	var idx string
	if err := db.QueryRow(`SELECT name FROM sqlite_master WHERE type='index' AND name='idx_publications_workspace'`).Scan(&idx); err != nil {
		t.Errorf("idx_publications_workspace missing: %v", err)
	}
}

func TestMigration023(t *testing.T) {
	db := openMigrationTestDB(t)

	if err := migrate.Run(db, migrationsUpTo(t, 22), "."); err != nil {
		t.Fatalf("migrate to 022: %v", err)
	}
	for _, stmt := range []string{
		`INSERT INTO workspaces (id, name, remote_workspace_id) VALUES ('cloud', 'Team', 'remote-1')`,
		`INSERT INTO workspaces (id, name, remote_workspace_id) VALUES ('blank', 'Blank', '')`,
		`INSERT INTO publications (collection_id, workspace_id, owner_key, publication_id, slug, pending_unpublish, created_at, updated_at)
			VALUES ('c1', 'w1', 'owner-a', 'p1', 'petstore', 1, '2026-09-25T00:00:00Z', '2026-09-25T00:00:00Z')`,
		`INSERT INTO publications (collection_id, workspace_id, owner_key, publication_id, created_at, updated_at)
			VALUES ('c-cloud', 'cloud', 'owner-a', 'p3', '2026-09-25T00:00:00Z', '2026-09-25T00:00:00Z')`,
		`INSERT INTO publications (collection_id, workspace_id, owner_key, publication_id, created_at, updated_at)
			VALUES ('c-blank', 'blank', 'owner-a', 'p4', '2026-09-25T00:00:00Z', '2026-09-25T00:00:00Z')`,
	} {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatalf("%s: %v", stmt, err)
		}
	}
	if err := migrate.Run(db, migrations.FS, "."); err != nil {
		t.Fatalf("migrate to head: %v", err)
	}

	var attempts int
	var refusal string
	if err := db.QueryRow(`SELECT unpublish_attempts, unpublish_error FROM publications WHERE collection_id = 'c1'`).
		Scan(&attempts, &refusal); err != nil || attempts != 0 || refusal != "" {
		t.Errorf("refusal defaults = (%d, %q, %v)", attempts, refusal, err)
	}

	for id, want := range map[string]int{"c1": 0, "c-cloud": 1, "c-blank": 0} {
		var cloud int
		if err := db.QueryRow(`SELECT cloud FROM publications WHERE collection_id = ?`, id).Scan(&cloud); err != nil {
			t.Fatal(err)
		}
		if cloud != want {
			t.Errorf("%s: cloud = %d, want %d from the workspace it was published from", id, cloud, want)
		}
	}

	var slug string
	var pending int
	if err := db.QueryRow(`SELECT slug, pending_unpublish FROM publications WHERE collection_id = 'c1' AND owner_key = 'owner-a'`).
		Scan(&slug, &pending); err != nil {
		t.Fatalf("row lost in the rebuild: %v", err)
	}
	if slug != "petstore" || pending != 1 {
		t.Errorf("row = (%q, %d), want (petstore, 1)", slug, pending)
	}

	insert := `INSERT INTO publications (collection_id, workspace_id, owner_key, publication_id, created_at, updated_at)
		VALUES ('c1', 'w1', ?, 'p2', '2026-09-25T00:00:00Z', '2026-09-25T00:00:00Z')`
	if _, err := db.Exec(insert, "owner-b"); err != nil {
		t.Errorf("another account's row for the same collection: %v", err)
	}
	if _, err := db.Exec(insert, "owner-a"); err == nil {
		t.Error("two rows for one collection and account")
	}

	var idx string
	if err := db.QueryRow(`SELECT name FROM sqlite_master WHERE type='index' AND name='idx_publications_workspace'`).Scan(&idx); err != nil {
		t.Errorf("idx_publications_workspace missing: %v", err)
	}
}

func TestMigration024(t *testing.T) {
	db := openMigrationTestDB(t)

	if err := migrate.Run(db, migrationsUpTo(t, 23), "."); err != nil {
		t.Fatalf("migrate to 023: %v", err)
	}
	for _, stmt := range []string{
		`INSERT INTO workspaces (id, name, remote_workspace_id) VALUES ('linked', 'Linked', 'remote-1')`,
		`INSERT INTO workspaces (id, name, remote_workspace_id) VALUES ('blank', 'Blank', '')`,
		`INSERT INTO workspaces (id, name) VALUES ('local', 'Local')`,
		`INSERT INTO collections (id, workspace_id, name) VALUES ('c-local', 'local', 'Never pushed')`,
		`INSERT INTO workspaces (id, name) VALUES ('had-folder', 'Unlinked')`,
		`INSERT INTO collections (id, workspace_id, name, is_synced) VALUES ('c-synced', 'had-folder', 'Pushed', 1)`,
		`INSERT INTO workspaces (id, name) VALUES ('had-env', 'Unlinked')`,
		`INSERT INTO environments (id, workspace_id, name, is_synced) VALUES ('e-synced', 'had-env', 'Pushed', 1)`,
		`INSERT INTO workspaces (id, name, last_sync_seq) VALUES ('pulled', 'Unlinked', 7)`,
	} {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatalf("%s: %v", stmt, err)
		}
	}
	if err := migrate.Run(db, migrations.FS, "."); err != nil {
		t.Fatalf("migrate to head: %v", err)
	}

	wantByID := map[string]int{"linked": 1, "blank": 0, "local": 0, "had-folder": 1, "had-env": 1, "pulled": 1}
	wantByID["00000000-0000-4000-a000-000000000001"] = 0
	for id, want := range wantByID {
		var wasLinked int
		if err := db.QueryRow(`SELECT was_linked FROM workspaces WHERE id = ?`, id).Scan(&wasLinked); err != nil {
			t.Fatal(err)
		}
		if wasLinked != want {
			t.Errorf("%s: was_linked = %d, want %d", id, wasLinked, want)
		}
	}
}

func TestMigrationsKeepThe111SeededEnvironmentOutOfTheFirstLink(t *testing.T) {
	db := openMigrationTestDB(t)
	ws := testWorkspaceID.String()

	if err := migrate.Run(db, migrationsUpTo(t, 20), "."); err != nil {
		t.Fatalf("migrate to 020: %v", err)
	}
	for _, stmt := range []string{
		`INSERT INTO variables (id, environment_id, key) VALUES ('v1', '` + seededEnvironmentID + `', 'host')`,
		`INSERT INTO variables (id, environment_id, key, is_delete) VALUES ('v2', '` + seededEnvironmentID + `', 'old', 1)`,
		`INSERT INTO collections (id, workspace_id, name) VALUES ('c1', '` + ws + `', 'Dogs API')`,
	} {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatalf("%s: %v", stmt, err)
		}
	}
	if err := migrate.Run(db, migrations.FS, "."); err != nil {
		t.Fatalf("migrate to head: %v", err)
	}

	var name string
	var active, synced, vars int
	if err := db.QueryRow(`SELECT name, is_active, is_synced, (SELECT COUNT(*) FROM variables WHERE environment_id = e.id)
		FROM environments e WHERE id = ?`, seededEnvironmentID).Scan(&name, &active, &synced, &vars); err != nil {
		t.Fatalf("seeded environment: %v", err)
	}
	if name != "Default" || active != 1 || synced != 0 || vars != 2 {
		t.Errorf("seeded environment = (%q, active %d, synced %d, %d variables), want (Default, 1, 0, 2)", name, active, synced, vars)
	}
	var wasLinked int
	if err := db.QueryRow(`SELECT was_linked FROM workspaces WHERE id = ?`, ws).Scan(&wasLinked); err != nil || wasLinked != 0 {
		t.Fatalf("was_linked = %d, %v; want 0 so the first link uploads", wasLinked, err)
	}

	if _, err := NewSyncQueueRepo(db).EnqueueWorkspace(context.Background(), ws); err != nil {
		t.Fatalf("EnqueueWorkspace: %v", err)
	}
	if got, want := queueRows(t, db, ws), []string{"collection:c1:create:pending"}; fmt.Sprint(got) != fmt.Sprint(want) {
		t.Errorf("queue = %v, want %v", got, want)
	}
}
