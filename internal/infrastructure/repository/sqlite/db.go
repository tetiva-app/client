package sqlite

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"

	"github.com/tetiva-app/client/internal/constants"
)

// NewDB opens the SQLite database at ~/.tetiva/data.db with recommended PRAGMAs,
// renaming the legacy ~/.gophercourier dir in place on first start so data carries over.
func NewDB() (*sql.DB, error) {
	const funcName = "sqlite.NewDB"

	var dbDir string
	if envDir := dataDirFromEnv(); envDir != "" {
		dbDir = envDir
	} else {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("%s: get home dir: %w", funcName, err)
		}
		dbDir = filepath.Join(homeDir, constants.AppDir)
		dbDir = migrateLegacyDataDir(homeDir, dbDir)
	}
	if err := os.MkdirAll(dbDir, 0o750); err != nil {
		return nil, fmt.Errorf("%s: create db dir: %w", funcName, err)
	}

	dbPath := filepath.Join(dbDir, constants.DBFileName)
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("%s: open: %w", funcName, err)
	}

	pragmas := []string{
		"PRAGMA journal_mode=WAL",
		"PRAGMA foreign_keys=ON",
		"PRAGMA busy_timeout=5000",
	}
	for _, p := range pragmas {
		if _, err := db.Exec(p); err != nil {
			return nil, fmt.Errorf("%s: %s: %w", funcName, p, err)
		}
	}

	return db, nil
}

// dataDirFromEnv resolves the data-dir override. The Tetiva variable wins;
// the legacy GopherCourier one keeps existing test setups and scripts working.
func dataDirFromEnv() string {
	if v := os.Getenv("TETIVA_DATA_DIR"); v != "" {
		return v
	}
	return os.Getenv("GOPHERCOURIER_DATA_DIR")
}

// migrateLegacyDataDir renames ~/.gophercourier to ~/.tetiva once. If the new
// dir already exists or the rename fails, the legacy dir keeps being used.
func migrateLegacyDataDir(homeDir, newDir string) string {
	legacyDir := filepath.Join(homeDir, constants.LegacyAppDir)
	if _, err := os.Stat(newDir); err == nil {
		return newDir
	}
	if _, err := os.Stat(legacyDir); err != nil {
		return newDir
	}
	if err := os.Rename(legacyDir, newDir); err != nil {
		return legacyDir
	}
	return newDir
}
