//go:build !windows

package instance

import (
	"bufio"
	"errors"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"testing"
	"time"
)

const (
	helperModeEnv = "TETIVA_INSTANCE_HELPER"
	helperDirEnv  = "TETIVA_INSTANCE_DIR"
	helperArgEnv  = "TETIVA_INSTANCE_ARG"
)

func TestMain(m *testing.M) {
	switch os.Getenv(helperModeEnv) {
	case "forward":
		os.Exit(runForwardHelper())
	case "hold":
		os.Exit(runHoldHelper())
	}
	os.Exit(m.Run())
}

// Exit codes mirror main.go: 0 forwarded, 2 not responding.
func runForwardHelper() int {
	inst, err := acquire(os.Getenv(helperDirEnv), []string{os.Getenv(helperArgEnv)}, nil, defaultOptions)
	switch {
	case errors.Is(err, ErrForwarded):
		return 0
	case errors.Is(err, ErrNotResponding):
		return 2
	case err == nil:
		_ = inst.Close()
		return 3
	default:
		_, _ = os.Stderr.WriteString(err.Error() + "\n")
		return 1
	}
}

// Holds the profile lock with no listener until stdin closes: a hung first instance.
func runHoldHelper() int {
	lock, err := lockFile(filepath.Join(os.Getenv(helperDirEnv), lockFileName))
	if err != nil {
		_, _ = os.Stderr.WriteString(err.Error() + "\n")
		return 1
	}
	defer func() { _ = unlockFile(lock) }()
	_, _ = os.Stdout.WriteString("locked\n")
	_, _ = io.Copy(io.Discard, os.Stdin)
	return 0
}

func helper(t *testing.T, mode, dir, arg string) *exec.Cmd {
	t.Helper()

	cmd := exec.Command(os.Args[0], "-test.run=^$")
	cmd.Env = append(os.Environ(), helperModeEnv+"="+mode, helperDirEnv+"="+dir, helperArgEnv+"="+arg)
	cmd.Stderr = os.Stderr
	return cmd
}

// startHolder returns once the helper owns the lock; closing the returned writer releases it.
func startHolder(t *testing.T, dir string) (io.WriteCloser, *exec.Cmd) {
	t.Helper()

	cmd := helper(t, "hold", dir, "")
	stdin, err := cmd.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = stdin.Close()
		_ = cmd.Wait()
	})
	line, err := bufio.NewReader(stdout).ReadString('\n')
	if err != nil || line != "locked\n" {
		t.Fatalf("holder did not take the lock: %q, %v", line, err)
	}
	return stdin, cmd
}

func closedPort(t *testing.T) int {
	t.Helper()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	_ = ln.Close()
	return port
}

func TestSecondProcessForwardsToFirst(t *testing.T) {
	dir := t.TempDir()
	got := make(chan []string, 1)
	inst, err := Acquire(dir, nil, func(args []string) { got <- args })
	if err != nil {
		t.Fatalf("Acquire: %v", err)
	}
	t.Cleanup(func() { _ = inst.Close() })

	cmd := helper(t, "forward", dir, testLink)
	if err := cmd.Run(); err != nil {
		t.Fatalf("second process: %v (want exit 0, ErrForwarded)", err)
	}

	select {
	case args := <-got:
		if len(args) != 1 || args[0] != testLink {
			t.Fatalf("first instance got %q, want [%q]", args, testLink)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("first instance never received the link")
	}
}

func TestLockedWithoutListenerIsNotResponding(t *testing.T) {
	dir := t.TempDir()
	startHolder(t, dir)
	if err := writeInfo(filepath.Join(dir, infoFileName), closedPort(t), "stale-token"); err != nil {
		t.Fatal(err)
	}

	start := time.Now()
	inst, err := acquire(dir, []string{testLink}, nil, fastOptions)
	if err == nil {
		_ = inst.Close()
	}

	if !errors.Is(err, ErrNotResponding) {
		t.Fatalf("err = %v, want ErrNotResponding", err)
	}
	if elapsed := time.Since(start); elapsed < fastOptions.wait {
		t.Fatalf("gave up after %v, want at least %v of retries", elapsed, fastOptions.wait)
	}
}

func TestLockFreedDuringRetriesTakesOver(t *testing.T) {
	dir := t.TempDir()
	release, cmd := startHolder(t, dir)
	go func() {
		time.Sleep(100 * time.Millisecond)
		_ = release.Close()
	}()

	inst, err := acquire(dir, []string{testLink}, nil, options{wait: 3 * time.Second, retry: 50 * time.Millisecond, dial: 200 * time.Millisecond})
	if err != nil {
		t.Fatalf("acquire after the holder exited: %v", err)
	}
	_ = cmd.Wait()
	_ = inst.Close()
}

func TestInstanceFileIsPrivate(t *testing.T) {
	dir := t.TempDir()
	inst, err := Acquire(dir, nil, nil)
	if err != nil {
		t.Fatalf("Acquire: %v", err)
	}
	t.Cleanup(func() { _ = inst.Close() })

	info, err := os.Stat(filepath.Join(dir, infoFileName))
	if err != nil {
		t.Fatal(err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Fatalf("instance.json mode = %s, want 0600", strconv.FormatUint(uint64(perm), 8))
	}
}
