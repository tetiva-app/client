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

func realPath(t *testing.T, path string) string {
	t.Helper()

	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		t.Fatalf("EvalSymlinks(%q): %v", path, err)
	}
	return resolved
}

func isolateHome(t *testing.T) string {
	t.Helper()

	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("TETIVA_DATA_DIR", "")
	t.Setenv("GOPHERCOURIER_DATA_DIR", "")
	return home
}

func TestResolveDataDir(t *testing.T) {
	t.Run("clean install creates the default dir", func(t *testing.T) {
		home := isolateHome(t)

		got, err := ResolveDataDir()
		if err != nil {
			t.Fatalf("ResolveDataDir: %v", err)
		}

		want := filepath.Join(realPath(t, home), constants.AppDir)
		if got != want {
			t.Fatalf("got %q, want %q", got, want)
		}
		if info, err := os.Stat(got); err != nil || !info.IsDir() {
			t.Fatalf("data dir not created: %v", err)
		}
	})

	t.Run("legacy dir is renamed", func(t *testing.T) {
		home := isolateHome(t)
		legacy := filepath.Join(home, constants.LegacyAppDir)
		if err := os.MkdirAll(legacy, 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(legacy, constants.DBFileName), []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}

		got, err := ResolveDataDir()
		if err != nil {
			t.Fatalf("ResolveDataDir: %v", err)
		}

		if want := filepath.Join(realPath(t, home), constants.AppDir); got != want {
			t.Fatalf("got %q, want %q", got, want)
		}
		if _, err := os.Stat(filepath.Join(got, constants.DBFileName)); err != nil {
			t.Fatalf("data.db not carried over: %v", err)
		}
		if _, err := os.Stat(legacy); !os.IsNotExist(err) {
			t.Fatalf("legacy dir still exists: %v", err)
		}
	})

	t.Run("failed rename keeps the legacy dir", func(t *testing.T) {
		home := isolateHome(t)
		legacy := filepath.Join(home, constants.LegacyAppDir)
		if err := os.MkdirAll(legacy, 0o750); err != nil {
			t.Fatal(err)
		}
		orig := rename
		rename = func(string, string) error { return os.ErrPermission }
		t.Cleanup(func() { rename = orig })

		got, err := ResolveDataDir()
		if err != nil {
			t.Fatalf("ResolveDataDir: %v", err)
		}

		if want := realPath(t, legacy); got != want {
			t.Fatalf("got %q, want %q", got, want)
		}
	})

	t.Run("relative env dir becomes absolute", func(t *testing.T) {
		isolateHome(t)
		cwd := t.TempDir()
		t.Chdir(cwd)
		t.Setenv("TETIVA_DATA_DIR", filepath.Join("rel", "profile"))

		got, err := ResolveDataDir()
		if err != nil {
			t.Fatalf("ResolveDataDir: %v", err)
		}

		if want := filepath.Join(realPath(t, cwd), "rel", "profile"); got != want {
			t.Fatalf("got %q, want %q", got, want)
		}
	})

	t.Run("symlinked and real dir resolve to the same path", func(t *testing.T) {
		isolateHome(t)
		base := t.TempDir()
		target := filepath.Join(base, "target")
		link := filepath.Join(base, "link")
		if err := os.MkdirAll(target, 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(target, link); err != nil {
			t.Skipf("symlinks unavailable: %v", err)
		}

		t.Setenv("TETIVA_DATA_DIR", link)
		viaLink, err := ResolveDataDir()
		if err != nil {
			t.Fatalf("ResolveDataDir via link: %v", err)
		}
		t.Setenv("TETIVA_DATA_DIR", target)
		direct, err := ResolveDataDir()
		if err != nil {
			t.Fatalf("ResolveDataDir direct: %v", err)
		}

		if viaLink != direct {
			t.Fatalf("via link %q, direct %q", viaLink, direct)
		}
	})
}

func TestNewDB_OpensDataFileInGivenDir(t *testing.T) {
	dir := t.TempDir()

	db, err := NewDB(DataDir(dir))
	if err != nil {
		t.Fatalf("NewDB: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	var seq int
	var name, file string
	if err := db.QueryRow("PRAGMA database_list").Scan(&seq, &name, &file); err != nil {
		t.Fatalf("database_list: %v", err)
	}
	if !samePath(t, file, filepath.Join(dir, constants.DBFileName)) {
		t.Fatalf("attached %q, want %q", file, filepath.Join(dir, constants.DBFileName))
	}
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
