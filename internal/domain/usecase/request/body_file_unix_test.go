//go:build unix

package request_test

import (
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/tetiva-app/client/internal/domain/usecase/request"
)

func TestExecute_RefusesSpecialBodyFiles(t *testing.T) {
	fifo := filepath.Join(t.TempDir(), "payload.bin")
	if err := syscall.Mkfifo(fifo, 0o600); err != nil {
		t.Fatal(err)
	}

	for _, path := range []string{fifo, "/dev/zero"} {
		for _, bodyType := range bodyFileTypes {
			uc, id := bodyFileRequest(t, &mockRequester{}, bodyType, path)
			done := make(chan error, 1)
			go func() {
				_, err := uc.Execute(t.Context(), id, request.ExecuteOpt{WorkspaceID: testWorkspaceID})
				done <- err
			}()

			select {
			case err := <-done:
				if err == nil || !strings.Contains(err.Error(), "is not a regular file") {
					t.Errorf("%s body %q: err = %v", bodyType, path, err)
				}
			case <-time.After(5 * time.Second):
				t.Fatalf("%s body %q blocked the send", bodyType, path)
			}
		}
	}
}
