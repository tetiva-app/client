package request

import (
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
)

func TestMultipartBody_FailsTheStreamWhenAFileGoesAway(t *testing.T) {
	path := filepath.Join(t.TempDir(), "gone.bin")
	if err := os.WriteFile(path, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	body, err := newMultipartBody([]formField{{Key: "upload", Value: path, Type: "file", Enabled: true}})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}

	_, err = io.ReadAll(body.open())
	if !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("err = %v, want a not-exist error", err)
	}
}
