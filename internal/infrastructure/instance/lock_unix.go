//go:build unix

package instance

import (
	"errors"
	"fmt"
	"os"

	"golang.org/x/sys/unix"
)

func lockFile(path string) (*os.File, error) {
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0o600)
	if err != nil {
		return nil, fmt.Errorf("open lock: %w", err)
	}
	if err := unix.Flock(int(f.Fd()), unix.LOCK_EX|unix.LOCK_NB); err != nil {
		_ = f.Close()
		if errors.Is(err, unix.EWOULDBLOCK) {
			return nil, errLocked
		}
		return nil, fmt.Errorf("lock: %w", err)
	}
	return f, nil
}

func unlockFile(f *os.File) error {
	err := unix.Flock(int(f.Fd()), unix.LOCK_UN)
	return errors.Join(err, f.Close())
}

func yieldForeground() {}
