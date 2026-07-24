package migrate

import (
	"database/sql"
	"fmt"
	"io/fs"
	"sort"
	"strconv"
	"strings"
)

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

		content, err := fs.ReadFile(fsys, dir+"/"+name)
		if err != nil {
			return fmt.Errorf("%s: read %s: %w", funcName, name, err)
		}

		tx, err := db.Begin()
		if err != nil {
			return fmt.Errorf("%s: begin tx for %s: %w", funcName, name, err)
		}

		if _, err := tx.Exec(string(content)); err != nil {
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
