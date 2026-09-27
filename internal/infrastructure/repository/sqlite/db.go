package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	_ "modernc.org/sqlite"

	"github.com/tetiva-app/client/internal/constants"
)

// Pragmas ride in the DSN: a db.Exec("PRAGMA …") reaches one arbitrary pooled connection.
// No "file:" prefix — the URI parser fails on '%' and truncates at '#'; foreign_keys is in NewDB.
func DSN(path string) string {
	if path == ":memory:" {
		return path
	}
	if strings.HasPrefix(path, "file:") {
		u, err := url.Parse(path)
		if err != nil {
			return path
		}
		q := u.Query()
		q.Add("_pragma", "busy_timeout(5000)")
		q.Add("_pragma", "journal_mode(WAL)")
		u.RawQuery = q.Encode()

		return u.String()
	}

	return path + "?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)"
}

// DataDir is the resolved profile directory; the instance lock and the database share it.
type DataDir string

// ResolveDataDir runs once, in main.go, so the instance lock and the database cannot end up
// in different directories.
func ResolveDataDir() (string, error) {
	const funcName = "sqlite.ResolveDataDir"

	var dir string
	if envDir := dataDirFromEnv(); envDir != "" {
		dir = envDir
	} else {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("%s: get home dir: %w", funcName, err)
		}
		dir = migrateLegacyDataDir(homeDir, filepath.Join(homeDir, constants.AppDir))
	}
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return "", fmt.Errorf("%s: create dir: %w", funcName, err)
	}
	abs, err := filepath.Abs(dir)
	if err != nil {
		return "", fmt.Errorf("%s: %w", funcName, err)
	}
	resolved, err := filepath.EvalSymlinks(abs)
	if err != nil {
		return "", fmt.Errorf("%s: %w", funcName, err)
	}

	return resolved, nil
}

func NewDB(dir DataDir) (*sql.DB, error) {
	const funcName = "sqlite.NewDB"

	if dir == "" {
		return nil, fmt.Errorf("%s: empty data dir", funcName)
	}
	dbPath := filepath.Join(string(dir), constants.DBFileName)
	db, err := sql.Open("sqlite", DSN(dbPath))
	if err != nil {
		return nil, fmt.Errorf("%s: open: %w", funcName, err)
	}
	// sql.Open is lazy: without a ping an unopenable file would only fail later.
	if err := db.PingContext(context.Background()); err != nil {
		_ = db.Close()

		return nil, fmt.Errorf("%s: open: %w", funcName, err)
	}

	// Reaches one pooled connection only; inbound sync turns it off on its own (WithInboundTx).
	if _, err := db.Exec("PRAGMA foreign_keys=ON"); err != nil {
		_ = db.Close()

		return nil, fmt.Errorf("%s: PRAGMA foreign_keys=ON: %w", funcName, err)
	}

	return db, nil
}

// The Tetiva variable wins; the legacy GopherCourier one keeps old setups and scripts working.
func dataDirFromEnv() string {
	if v := os.Getenv("TETIVA_DATA_DIR"); v != "" {
		return v
	}
	return os.Getenv("GOPHERCOURIER_DATA_DIR")
}

var rename = os.Rename

// If the new dir already exists or the rename fails, the legacy dir keeps being used.
func migrateLegacyDataDir(homeDir, newDir string) string {
	legacyDir := filepath.Join(homeDir, constants.LegacyAppDir)
	if _, err := os.Stat(newDir); err == nil {
		return newDir
	}
	if _, err := os.Stat(legacyDir); err != nil {
		return newDir
	}
	if err := rename(legacyDir, newDir); err != nil {
		return legacyDir
	}
	return newDir
}
