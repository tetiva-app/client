package appupdate_test

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tetiva-app/client/internal/domain/entities"
	"github.com/tetiva-app/client/internal/infrastructure/appupdate"
)

func newStore(t *testing.T) *appupdate.FileStore {
	t.Helper()
	s, err := appupdate.NewFileStore(t.TempDir())
	require.NoError(t, err)
	return s
}

func stage(t *testing.T, s *appupdate.FileStore, r entities.Release) (string, []byte) {
	t.Helper()
	file := s.StagePath(r, r.Artifacts[0])
	require.NoError(t, os.MkdirAll(filepath.Dir(file), 0o700))
	require.NoError(t, os.WriteFile(file, []byte("zip"), 0o600))
	manifest := []byte(`{"version":"` + r.Version + `"}`)
	require.NoError(t, s.SaveStaged(r, manifest, file))
	return file, manifest
}

func releaseOf(version string) entities.Release {
	return entities.Release{
		Version: version,
		Artifacts: []entities.Artifact{
			{URL: "https://s3.twcstorage.ru/ccquota/releases/Tetiva-" + version + "-macos-universal.zip?x=1"},
		},
	}
}

func TestNewFileStore_CreatesPrivateDir(t *testing.T) {
	dataDir := t.TempDir()

	s, err := appupdate.NewFileStore(dataDir)

	require.NoError(t, err)
	assert.Equal(t, filepath.Join(dataDir, "updates"), s.Dir)
	info, err := os.Stat(s.Dir)
	require.NoError(t, err)
	assert.True(t, info.IsDir())
	if runtime.GOOS != "windows" {
		assert.Equal(t, os.FileMode(0o700), info.Mode().Perm())
	}
}

func TestFileStore_StagePath(t *testing.T) {
	s := newStore(t)

	assert.Equal(t, filepath.Join(s.Dir, "1.2.2", "Tetiva-1.2.2-macos-universal.zip"), s.StagePath(releaseOf("1.2.2"), releaseOf("1.2.2").Artifacts[0]))
}

func TestFileStore_StagedRoundTrip(t *testing.T) {
	s := newStore(t)
	_, _, ok := s.Staged()
	require.False(t, ok)

	file, manifest := stage(t, s, releaseOf("1.2.2"))
	gotManifest, gotFile, ok := s.Staged()

	require.True(t, ok)
	assert.Equal(t, manifest, gotManifest)
	assert.Equal(t, file, gotFile)
}

func TestFileStore_StagedNewestVersion(t *testing.T) {
	s := newStore(t)
	stage(t, s, releaseOf("1.2.10"))
	stage(t, s, releaseOf("1.2.9"))
	require.NoError(t, os.MkdirAll(filepath.Join(s.Dir, "1.3.0"), 0o700))

	_, file, ok := s.Staged()

	require.True(t, ok)
	assert.Equal(t, s.StagePath(releaseOf("1.2.10"), releaseOf("1.2.10").Artifacts[0]), file)
}

func TestFileStore_StagedNeedsArtifact(t *testing.T) {
	s := newStore(t)
	file, _ := stage(t, s, releaseOf("1.2.2"))
	require.NoError(t, os.Rename(file, file+".part"))

	_, _, ok := s.Staged()

	assert.False(t, ok)
}

func TestFileStore_DropStaged(t *testing.T) {
	s := newStore(t)
	file, _ := stage(t, s, releaseOf("1.2.2"))
	require.NoError(t, s.MarkAttempt("1.2.2"))

	require.NoError(t, s.DropStaged())

	assert.NoDirExists(t, filepath.Dir(file))
	_, _, ok := s.Staged()
	assert.False(t, ok)
	v, ok := s.TakeAttempt()
	assert.True(t, ok)
	assert.Equal(t, "1.2.2", v)
}

func TestFileStore_Attempt(t *testing.T) {
	s := newStore(t)
	_, ok := s.TakeAttempt()
	require.False(t, ok)

	require.NoError(t, s.MarkAttempt("1.2.2"))
	v, ok := s.TakeAttempt()

	assert.True(t, ok)
	assert.Equal(t, "1.2.2", v)
	_, ok = s.TakeAttempt()
	assert.False(t, ok)
}

func TestFileStore_RestoreRoundTrip(t *testing.T) {
	s := newStore(t)
	want := entities.RestoreTabs{
		WorkspaceID: "00000000-0000-4000-a000-000000000001",
		Tabs:        []entities.RestoreTab{{Type: "request", ID: "r1"}, {Type: "collection", ID: "c1"}},
		ActiveTabID: "r1",
		SavedAt:     time.Now().UTC(),
	}

	require.NoError(t, s.SaveRestore(want))
	got, ok := s.TakeRestore()

	require.True(t, ok)
	assert.True(t, want.SavedAt.Equal(got.SavedAt))
	got.SavedAt = want.SavedAt
	assert.Equal(t, want, got)
	assert.NoFileExists(t, filepath.Join(s.Dir, "restore.json"))
	_, ok = s.TakeRestore()
	assert.False(t, ok)
}

func TestFileStore_RestoreStale(t *testing.T) {
	s := newStore(t)
	require.NoError(t, s.SaveRestore(entities.RestoreTabs{
		WorkspaceID: "00000000-0000-4000-a000-000000000001",
		SavedAt:     time.Now().Add(-11 * time.Minute),
	}))

	_, ok := s.TakeRestore()

	assert.False(t, ok)
	assert.NoFileExists(t, filepath.Join(s.Dir, "restore.json"))
}

func TestFileStore_Cleanup(t *testing.T) {
	s := newStore(t)
	file, _ := stage(t, s, releaseOf("1.2.2"))
	part := s.StagePath(releaseOf("1.2.3"), releaseOf("1.2.3").Artifacts[0]) + ".part"
	require.NoError(t, os.MkdirAll(filepath.Dir(part), 0o700))
	require.NoError(t, os.WriteFile(part, []byte("half"), 0o600))

	s.Cleanup()

	assert.NoFileExists(t, part)
	assert.FileExists(t, file)
	_, _, ok := s.Staged()
	assert.True(t, ok)
}
