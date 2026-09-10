package sqlite

import (
	"context"
	"database/sql"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

// openAt opens dsn and reports the file SQLite actually attached, so a truncated
// or re-parsed path shows up as a different database rather than as a pass.
func openAt(t *testing.T, dsn string) (*sql.DB, string, int) {
	t.Helper()

	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		t.Fatalf("open %q: %v", dsn, err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := db.PingContext(context.Background()); err != nil {
		t.Fatalf("ping %q: %v", dsn, err)
	}

	var seq int
	var name, file string
	if err := db.QueryRow("PRAGMA database_list").Scan(&seq, &name, &file); err != nil {
		t.Fatalf("database_list: %v", err)
	}
	var busy int
	if err := db.QueryRow("PRAGMA busy_timeout").Scan(&busy); err != nil {
		t.Fatalf("busy_timeout: %v", err)
	}

	return db, file, busy
}

func samePath(t *testing.T, got, want string) bool {
	t.Helper()

	gotReal, err := filepath.EvalSymlinks(got)
	if err != nil {
		gotReal = got
	}
	wantReal, err := filepath.EvalSymlinks(want)
	if err != nil {
		wantReal = want
	}

	return gotReal == wantReal
}

func TestDSN(t *testing.T) {
	if got := DSN(":memory:"); got != ":memory:" {
		t.Errorf(`DSN(":memory:") = %q, want it unchanged`, got)
	}

	dirs := []string{"plain", "with space", "pct%20dir", "hash#dir", "кириллица"}
	for _, name := range dirs {
		t.Run(name, func(t *testing.T) {
			dir := filepath.Join(t.TempDir(), name)
			if err := os.MkdirAll(dir, 0o750); err != nil {
				t.Fatal(err)
			}
			want := filepath.Join(dir, "data.db")

			_, file, busy := openAt(t, DSN(want))
			if !samePath(t, file, want) {
				t.Errorf("attached %q, want %q", file, want)
			}
			if busy != 5000 {
				t.Errorf("busy_timeout = %d, want 5000", busy)
			}
		})
	}

	t.Run("file uri keeps its own parameters", func(t *testing.T) {
		want := filepath.Join(t.TempDir(), "data.db")
		got := DSN("file:" + want + "?mode=rwc&cache=shared")

		u, err := url.Parse(got)
		if err != nil {
			t.Fatalf("parse %q: %v", got, err)
		}
		q := u.Query()
		if q.Get("mode") != "rwc" || q.Get("cache") != "shared" {
			t.Errorf("DSN dropped the caller's parameters: %q", got)
		}
		pragmas := strings.Join(q["_pragma"], ",")
		if !strings.Contains(pragmas, "busy_timeout(5000)") || !strings.Contains(pragmas, "journal_mode(WAL)") {
			t.Errorf("pragmas missing from %q", got)
		}

		_, file, busy := openAt(t, got)
		if !samePath(t, file, want) {
			t.Errorf("attached %q, want %q", file, want)
		}
		if busy != 5000 {
			t.Errorf("busy_timeout = %d, want 5000", busy)
		}
	})

	// Known limitation: modernc strips from the first '?', so a literal '?' in a dir name (legal
	// on macOS/Linux) opens the wrong file unbusied; percent-encoding needs "file:", broken on '%'/'#'.
	t.Run("question mark in path is the documented limitation", func(t *testing.T) {
		dir := filepath.Join(t.TempDir(), "q?dir")
		if err := os.MkdirAll(dir, 0o750); err != nil {
			t.Fatal(err)
		}
		want := filepath.Join(dir, "data.db")

		_, file, busy := openAt(t, DSN(want))
		if samePath(t, file, want) {
			t.Fatalf("the '?' limitation is gone: attached %q — update DSN's doc and drop this test", file)
		}
		if busy != 0 {
			t.Errorf("busy_timeout = %d, want 0 for the truncated DSN", busy)
		}
	})
}

func TestNewDB_BusyTimeoutOnEveryPooledConnection(t *testing.T) {
	t.Setenv("TETIVA_DATA_DIR", t.TempDir())

	db, err := NewDB()
	if err != nil {
		t.Fatalf("NewDB: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	const conns = 4
	ctx := context.Background()
	var wg sync.WaitGroup
	var mu sync.Mutex
	held := make([]*sql.Conn, 0, conns)
	results := make([]string, conns)
	ready := make(chan struct{})

	// Every connection is held open at once, so the pool must hand out four
	// distinct ones rather than reusing the one the pragmas would have landed on.
	for i := range conns {
		conn, err := db.Conn(ctx)
		if err != nil {
			t.Fatalf("conn %d: %v", i, err)
		}
		held = append(held, conn)

		wg.Add(1)
		go func() {
			defer wg.Done()
			<-ready

			var busy int
			var journal string
			if err := conn.QueryRowContext(ctx, "PRAGMA busy_timeout").Scan(&busy); err != nil {
				t.Errorf("busy_timeout on conn %d: %v", i, err)
				return
			}
			if err := conn.QueryRowContext(ctx, "PRAGMA journal_mode").Scan(&journal); err != nil {
				t.Errorf("journal_mode on conn %d: %v", i, err)
				return
			}
			mu.Lock()
			results[i] = journal
			mu.Unlock()
			if busy != 5000 {
				t.Errorf("conn %d: busy_timeout = %d, want 5000", i, busy)
			}
		}()
	}
	close(ready)
	wg.Wait()
	for _, conn := range held {
		_ = conn.Close()
	}

	for i, journal := range results {
		if journal != "wal" {
			t.Errorf("conn %d: journal_mode = %q, want wal", i, journal)
		}
	}
}
