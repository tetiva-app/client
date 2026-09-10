package main

import (
	"context"
	"database/sql"
	"path/filepath"
	"sync"
	"testing"
)

// A file-backed MCP database runs on an uncapped pool, so busy_timeout has to
// come from the DSN rather than from a pragma landing on one connection.
func TestOpenDB_FileBackedPoolCarriesBusyTimeout(t *testing.T) {
	t.Setenv("MCP_DB", filepath.Join(t.TempDir(), "mcp.db"))

	db := openDB()
	t.Cleanup(func() { _ = db.Close() })

	const conns = 4
	ctx := context.Background()
	held := make([]*sql.Conn, 0, conns)
	var wg sync.WaitGroup
	ready := make(chan struct{})

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
			if err := conn.QueryRowContext(ctx, "PRAGMA busy_timeout").Scan(&busy); err != nil {
				t.Errorf("busy_timeout on conn %d: %v", i, err)
				return
			}
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
}
