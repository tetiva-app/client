package sqlite

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/tetiva-app/client/internal/constants"
)

func TestMigrateLegacyDataDir(t *testing.T) {
	t.Run("renames legacy dir when new one is absent", func(t *testing.T) {
		home := t.TempDir()
		legacy := filepath.Join(home, constants.LegacyAppDir)
		newDir := filepath.Join(home, constants.AppDir)
		if err := os.MkdirAll(legacy, 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(legacy, "data.db"), []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}

		got := migrateLegacyDataDir(home, newDir)

		if got != newDir {
			t.Fatalf("got %q, want %q", got, newDir)
		}
		if _, err := os.Stat(filepath.Join(newDir, "data.db")); err != nil {
			t.Fatalf("data.db not migrated: %v", err)
		}
		if _, err := os.Stat(legacy); !os.IsNotExist(err) {
			t.Fatalf("legacy dir still exists: %v", err)
		}
	})

	t.Run("keeps existing new dir untouched", func(t *testing.T) {
		home := t.TempDir()
		legacy := filepath.Join(home, constants.LegacyAppDir)
		newDir := filepath.Join(home, constants.AppDir)
		for _, d := range []string{legacy, newDir} {
			if err := os.MkdirAll(d, 0o750); err != nil {
				t.Fatal(err)
			}
		}

		got := migrateLegacyDataDir(home, newDir)

		if got != newDir {
			t.Fatalf("got %q, want %q", got, newDir)
		}
		if _, err := os.Stat(legacy); err != nil {
			t.Fatalf("legacy dir must stay untouched when new dir exists: %v", err)
		}
	})

	t.Run("returns new dir when nothing to migrate", func(t *testing.T) {
		home := t.TempDir()
		newDir := filepath.Join(home, constants.AppDir)

		if got := migrateLegacyDataDir(home, newDir); got != newDir {
			t.Fatalf("got %q, want %q", got, newDir)
		}
	})
}

func TestDataDirFromEnv(t *testing.T) {
	t.Setenv("TETIVA_DATA_DIR", "/new")
	t.Setenv("GOPHERCOURIER_DATA_DIR", "/legacy")
	if got := dataDirFromEnv(); got != "/new" {
		t.Fatalf("tetiva var must win, got %q", got)
	}

	t.Setenv("TETIVA_DATA_DIR", "")
	if got := dataDirFromEnv(); got != "/legacy" {
		t.Fatalf("legacy fallback broken, got %q", got)
	}
}
