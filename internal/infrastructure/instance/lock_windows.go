//go:build windows

package instance

import (
	"errors"
	"fmt"
	"os"

	"golang.org/x/sys/windows"
)

func lockFile(path string) (*os.File, error) {
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0o600)
	if err != nil {
		return nil, fmt.Errorf("open lock: %w", err)
	}
	flags := uint32(windows.LOCKFILE_EXCLUSIVE_LOCK | windows.LOCKFILE_FAIL_IMMEDIATELY)
	if err := windows.LockFileEx(windows.Handle(f.Fd()), flags, 0, 1, 0, new(windows.Overlapped)); err != nil {
		_ = f.Close()
		if errors.Is(err, windows.ERROR_LOCK_VIOLATION) {
			return nil, errLocked
		}
		return nil, fmt.Errorf("lock: %w", err)
	}
	return f, nil
}

func unlockFile(f *os.File) error {
	err := windows.UnlockFileEx(windows.Handle(f.Fd()), 0, 1, 0, new(windows.Overlapped))
	return errors.Join(err, f.Close())
}

var procAllowSetForegroundWindow = windows.NewLazySystemDLL("user32.dll").NewProc("AllowSetForegroundWindow")

// Only the process the user just launched may take the foreground; this lends that right
// to the running instance, otherwise its window only flashes in the taskbar.
func yieldForeground() {
	const asfwAny = 0xFFFFFFFF
	_, _, _ = procAllowSetForegroundWindow.Call(asfwAny)
}
