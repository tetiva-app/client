package migrate

import (
	"context"
	"database/sql"
	"fmt"
	"io/fs"
	"log/slog"
	"path"
	"sort"
	"strconv"
	"strings"
)

// fkOffHeader marks a migration that rebuilds a table other tables reference.
// PRAGMA foreign_keys is a no-op inside a transaction, so such a file needs its
// own connection with the pragma turned off before BEGIN.
const fkOffHeader = "-- migrate:fk-off"

// Run applies pending SQL migrations from the given filesystem.
// The dir parameter specifies the directory name inside the FS (e.g. "migrations").
func Run(db *sql.DB, fsys fs.FS, dir string) error {
	const funcName = "migrate.Run"

	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS schema_migrations (
		version INTEGER PRIMARY KEY,
		applied_at TEXT NOT NULL DEFAULT (datetime('now'))
	)`); err != nil {
		return fmt.Errorf("%s: create schema_migrations: %w", funcName, err)
	}

	var currentVersion int
	row := db.QueryRow("SELECT COALESCE(MAX(version), 0) FROM schema_migrations")
	if err := row.Scan(&currentVersion); err != nil {
		return fmt.Errorf("%s: get current version: %w", funcName, err)
	}

	entries, err := fs.ReadDir(fsys, dir)
	if err != nil {
		return fmt.Errorf("%s: read dir %q: %w", funcName, dir, err)
	}

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Name() < entries[j].Name()
	})

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		name := entry.Name()
		parts := strings.SplitN(name, "_", 2)
		version, err := strconv.Atoi(parts[0])
		if err != nil {
			continue
		}

		if version <= currentVersion {
			continue
		}

		content, err := fs.ReadFile(fsys, path.Join(dir, name))
		if err != nil {
			return fmt.Errorf("%s: read %s: %w", funcName, name, err)
		}

		script := string(content)
		if isFKOff(script) {
			if err := runFKOff(db, script, name, version); err != nil {
				return err
			}
			continue
		}

		tx, err := db.Begin()
		if err != nil {
			return fmt.Errorf("%s: begin tx for %s: %w", funcName, name, err)
		}

		if _, err := tx.Exec(script); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("%s: exec %s: %w", funcName, name, err)
		}

		if _, err := tx.Exec("INSERT INTO schema_migrations (version) VALUES (?)", version); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("%s: record version %d: %w", funcName, version, err)
		}

		if err := tx.Commit(); err != nil {
			return fmt.Errorf("%s: commit %s: %w", funcName, name, err)
		}
	}

	return nil
}

func isFKOff(script string) bool {
	first, _, _ := strings.Cut(script, "\n")
	return strings.TrimSpace(strings.TrimSuffix(first, "\r")) == fkOffHeader
}

// runFKOff applies a table-rebuild migration: foreign keys off outside the
// transaction, rebuild plus foreign_key_check plus the version row inside it.
func runFKOff(db *sql.DB, script, name string, version int) error {
	const funcName = "migrate.runFKOff"

	ctx := context.Background()
	conn, err := db.Conn(ctx)
	if err != nil {
		return fmt.Errorf("%s: conn for %s: %w", funcName, name, err)
	}
	defer func() { _ = conn.Close() }()

	var fkWas int
	if err := conn.QueryRowContext(ctx, "PRAGMA foreign_keys").Scan(&fkWas); err != nil {
		return fmt.Errorf("%s: read foreign_keys for %s: %w", funcName, name, err)
	}
	if _, err := conn.ExecContext(ctx, "PRAGMA foreign_keys=OFF"); err != nil {
		return fmt.Errorf("%s: disable foreign_keys for %s: %w", funcName, name, err)
	}
	// The connection goes back to the pool, so the original setting must go with it.
	defer func() {
		_, _ = conn.ExecContext(ctx, fmt.Sprintf("PRAGMA foreign_keys=%d", fkWas))
	}()

	// foreign_key_check is database-wide, and real databases carry old violations
	// (history rows whose request was hard-deleted); only new ones mean a broken rebuild.
	before, err := fkViolations(ctx, conn)
	if err != nil {
		return fmt.Errorf("%s: foreign key baseline for %s: %w", funcName, name, err)
	}
	if len(before) > 0 {
		slog.Warn("database has foreign key violations before migration", "migration", name, "kinds", len(before))
	}

	tx, err := conn.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("%s: begin tx for %s: %w", funcName, name, err)
	}

	if _, err := tx.ExecContext(ctx, script); err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("%s: exec %s: %w", funcName, name, err)
	}

	after, err := fkViolations(ctx, tx)
	if err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("%s: foreign key check for %s: %w", funcName, name, err)
	}
	if broke := newViolations(before, after); broke != "" {
		_ = tx.Rollback()
		return fmt.Errorf("%s: %s left foreign key violations: %s", funcName, name, broke)
	}

	if _, err := tx.ExecContext(ctx, "INSERT INTO schema_migrations (version) VALUES (?)", version); err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("%s: record version %d: %w", funcName, version, err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("%s: commit %s: %w", funcName, name, err)
	}

	return nil
}

type querier interface {
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
}

type fkViolation struct {
	child  string
	parent string
	fkID   int64
}

// fkViolations counts violations per constraint. Rowids are ignored: a rebuilt
// table hands its rows new ones, which would look like fresh damage.
func fkViolations(ctx context.Context, q querier) (map[fkViolation]int, error) {
	rows, err := q.QueryContext(ctx, "PRAGMA foreign_key_check")
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	out := map[fkViolation]int{}
	for rows.Next() {
		var v fkViolation
		var rowID sql.NullInt64
		if err := rows.Scan(&v.child, &rowID, &v.parent, &v.fkID); err != nil {
			return nil, err
		}
		out[v]++
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func newViolations(before, after map[fkViolation]int) string {
	var broke []string
	for v, count := range after {
		if grew := count - before[v]; grew > 0 {
			broke = append(broke, fmt.Sprintf("%s -> %s (%d rows)", v.child, v.parent, grew))
		}
	}
	sort.Strings(broke)
	return strings.Join(broke, ", ")
}
