// Package instance keeps one running app per profile directory.
package instance

import (
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"net"
	"os"
	"path/filepath"
	"sync"
	"time"
)

const (
	lockFileName = "tetiva.lock"
	infoFileName = "instance.json"
)

// ErrForwarded means the arguments reached the running instance; the caller exits 0.
var ErrForwarded = errors.New("instance: arguments forwarded to the running instance")

// ErrNotResponding means the profile is locked but nobody answers on the loopback port.
var ErrNotResponding = errors.New("instance: profile is locked but the running instance does not answer")

var errLocked = errors.New("instance: profile is locked")

type Instance struct {
	lock      *os.File
	listener  net.Listener
	token     string
	infoPath  string
	onArgs    func(args []string)
	wg        sync.WaitGroup
	stopOnce  sync.Once
	stopErr   error
	closeOnce sync.Once
	closeErr  error
}

type options struct {
	wait   time.Duration
	retry  time.Duration
	dial   time.Duration
	listen func(network, address string) (net.Listener, error)
}

var defaultOptions = options{wait: 5 * time.Second, retry: 100 * time.Millisecond, dial: time.Second, listen: net.Listen}

// Acquire owns dataDir or hands args to its owner (ErrForwarded); onArgs must not block.
func Acquire(dataDir string, args []string, onArgs func(args []string)) (*Instance, error) {
	return acquire(dataDir, args, onArgs, defaultOptions)
}

func acquire(dataDir string, args []string, onArgs func(args []string), opt options) (*Instance, error) {
	const funcName = "instance.Acquire"

	lockPath := filepath.Join(dataDir, lockFileName)
	infoPath := filepath.Join(dataDir, infoFileName)
	deadline := time.Now().Add(opt.wait)
	for {
		lock, err := lockFile(lockPath)
		if err == nil {
			if err := os.Remove(infoPath); err != nil && !errors.Is(err, fs.ErrNotExist) {
				slog.Warn("instance: stale instance.json left in place", "err", err)
			}
			listen := opt.listen
			if listen == nil {
				listen = net.Listen
			}
			inst, err := serve(lock, infoPath, onArgs, listen)
			if err != nil {
				return nil, fmt.Errorf("%s: %w", funcName, err)
			}
			return inst, nil
		}
		if !errors.Is(err, errLocked) {
			return nil, fmt.Errorf("%s: %w", funcName, err)
		}
		if forward(infoPath, args, opt.dial) == nil {
			return nil, ErrForwarded
		}
		if !time.Now().Before(deadline) {
			return nil, ErrNotResponding
		}
		time.Sleep(opt.retry)
	}
}

// StopServing removes instance.json first, so no launch dials a freed port.
func (i *Instance) StopServing() error {
	i.stopOnce.Do(func() {
		err := os.Remove(i.infoPath)
		if errors.Is(err, fs.ErrNotExist) {
			err = nil
		}
		err = errors.Join(err, i.listener.Close())
		i.wg.Wait()
		i.stopErr = err
	})
	return i.stopErr
}

// Close unlocks only after StopServing, so a successor's instance.json is never deleted.
func (i *Instance) Close() error {
	i.closeOnce.Do(func() {
		i.closeErr = errors.Join(i.StopServing(), unlockFile(i.lock))
	})
	return i.closeErr
}
