//go:build darwin

package appupdate

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/sys/unix"
)

func TestDarwinRenameSwap(t *testing.T) {
	dir := t.TempDir()
	current, next := filepath.Join(dir, "Tetiva.app"), filepath.Join(dir, ".tetiva-update-1", "Tetiva.app")
	require.NoError(t, os.MkdirAll(current, 0o755))
	require.NoError(t, os.MkdirAll(next, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(current, "version"), []byte("1.2.1"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(next, "version"), []byte("1.2.2"), 0o644))

	require.NoError(t, unix.RenamexNp(next, current, unix.RENAME_SWAP))

	got, err := os.ReadFile(filepath.Join(current, "version"))
	require.NoError(t, err)
	assert.Equal(t, "1.2.2", string(got))
	got, err = os.ReadFile(filepath.Join(next, "version"))
	require.NoError(t, err)
	assert.Equal(t, "1.2.1", string(got))
}

func TestDarwinRelaunchScript(t *testing.T) {
	dir := t.TempDir()
	bin := filepath.Join(dir, "bin")
	calls := filepath.Join(dir, "open-calls")
	old := filepath.Join(dir, ".tetiva-update-1")
	require.NoError(t, os.MkdirAll(bin, 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(old, "Tetiva.app"), 0o755))
	stub := "#!/bin/sh\necho \"$*\" >> '" + calls + "'\n"
	require.NoError(t, os.WriteFile(filepath.Join(bin, "open"), []byte(stub), 0o755))

	child := exec.Command("sleep", "1")
	require.NoError(t, child.Start())
	exited := make(chan time.Time, 1)
	go func() {
		_ = child.Wait()
		exited <- time.Now()
	}()

	script := exec.Command("/bin/sh", "-c", relaunchScript(), "sh",
		strconv.Itoa(child.Process.Pid), "/Applications/Tetiva.app", old, "/tmp/tetiva profile")
	script.Env = append(os.Environ(), "PATH="+bin+":"+os.Getenv("PATH"))
	out, err := script.CombinedOutput()
	require.NoError(t, err, string(out))

	exitedAt := <-exited
	info, err := os.Stat(calls)
	require.NoError(t, err)
	assert.True(t, info.ModTime().After(exitedAt), "open ran at %v, child exited at %v", info.ModTime(), exitedAt)
	got, err := os.ReadFile(calls)
	require.NoError(t, err)
	assert.Equal(t, "-n /Applications/Tetiva.app --env TETIVA_DATA_DIR=/tmp/tetiva profile\n", string(got))
	assert.NoDirExists(t, old)
}
