//go:build unix

package requester

import (
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/tetiva-app/client/internal/domain/usecase/request"
)

func within(t *testing.T, fn func() error) error {
	t.Helper()
	done := make(chan error, 1)
	go func() { done <- fn() }()
	select {
	case err := <-done:
		return err
	case <-time.After(5 * time.Second):
		t.Fatal("read blocked")
		return nil
	}
}

func fifo(t *testing.T, dir, name string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := syscall.Mkfifo(path, 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestReadSchemaFileRefusesSpecialFiles(t *testing.T) {
	for _, path := range []string{fifo(t, t.TempDir(), "api.graphql"), "/dev/zero"} {
		err := within(t, func() error {
			_, err := readSchemaFile(path)
			return err
		})
		if err == nil || !strings.Contains(err.Error(), "not a regular file") {
			t.Errorf("readSchemaFile(%q) err = %v", path, err)
		}
	}
}

func TestGRPCProtoDirectoryWithFIFODoesNotBlock(t *testing.T) {
	dir := t.TempDir()
	fifo(t, dir, "evil.proto")

	err := within(t, func() error {
		_, err := NewGRPCRequester().ListServices(t.Context(), request.GRPCConnectRequest{ProtoPath: dir})
		return err
	})
	if err == nil || !strings.Contains(err.Error(), "not a regular file") {
		t.Fatalf("err = %v", err)
	}
}

func TestGRPCProtoFileFIFODoesNotBlock(t *testing.T) {
	path := fifo(t, t.TempDir(), "evil.proto")

	err := within(t, func() error {
		_, err := NewGRPCRequester().ListServices(t.Context(), request.GRPCConnectRequest{ProtoPath: path})
		return err
	})
	if err == nil || !strings.Contains(err.Error(), "not a regular file") {
		t.Fatalf("err = %v", err)
	}
}
