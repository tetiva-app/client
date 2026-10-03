package appupdate_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tetiva-app/client/internal/domain/entities"
	"github.com/tetiva-app/client/internal/infrastructure/appupdate"
)

func testBody(n int) []byte {
	b := make([]byte, n)
	for i := range b {
		b[i] = byte(i % 251)
	}
	return b
}

func serveArtifact(t *testing.T, h http.HandlerFunc) string {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	return srv.URL + "/Tetiva-1.2.2-macos-universal.zip"
}

func waitForClient(r *http.Request) {
	select {
	case <-r.Context().Done():
	case <-time.After(5 * time.Second):
	}
}

func partPath(t *testing.T) string {
	return filepath.Join(t.TempDir(), "Tetiva-1.2.2-macos-universal.zip.part")
}

func TestDownload_WritesBody(t *testing.T) {
	data := testBody(200<<10 + 7)
	url := serveArtifact(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(data)
	})
	dst := partPath(t)
	var seen []int64

	err := appupdate.NewDownloader().Download(context.Background(),
		entities.Artifact{URL: url, Size: int64(len(data))}, dst, func(n int64) { seen = append(seen, n) })

	require.NoError(t, err)
	got, err := os.ReadFile(dst)
	require.NoError(t, err)
	assert.Equal(t, data, got)
	require.NotEmpty(t, seen)
	assert.IsIncreasing(t, seen)
	assert.Equal(t, int64(len(data)), seen[len(seen)-1])
}

func TestDownload_LongerThanDeclared(t *testing.T) {
	data := testBody(1000)
	url := serveArtifact(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(data)
	})
	dst := partPath(t)

	err := appupdate.NewDownloader().Download(context.Background(),
		entities.Artifact{URL: url, Size: 999}, dst, func(int64) {})

	assert.Error(t, err)
	assert.NoFileExists(t, dst)
}

func TestDownload_Cancel(t *testing.T) {
	url := serveArtifact(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(testBody(128 << 10))
		w.(http.Flusher).Flush()
		waitForClient(r)
	})
	dst := partPath(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err := appupdate.NewDownloader().Download(ctx,
		entities.Artifact{URL: url, Size: 1 << 20}, dst, func(int64) { cancel() })

	assert.Error(t, err)
	assert.NoFileExists(t, dst)
}

func TestDownload_NotFound(t *testing.T) {
	url := serveArtifact(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})
	dst := partPath(t)

	err := appupdate.NewDownloader().Download(context.Background(),
		entities.Artifact{URL: url, Size: 10}, dst, func(int64) {})

	assert.Error(t, err)
	assert.NoFileExists(t, dst)
}

func TestDownload_Stalled(t *testing.T) {
	url := serveArtifact(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(testBody(10))
		w.(http.Flusher).Flush()
		waitForClient(r)
	})
	dst := partPath(t)
	d := appupdate.NewDownloader()
	d.IdleTimeout = 200 * time.Millisecond
	start := time.Now()

	err := d.Download(context.Background(), entities.Artifact{URL: url, Size: 1 << 20}, dst, func(int64) {})

	assert.Error(t, err)
	assert.Less(t, time.Since(start), 3*time.Second)
	assert.NoFileExists(t, dst)
}

func TestDownload_SlowButSteady(t *testing.T) {
	chunk := testBody(100)
	url := serveArtifact(t, func(w http.ResponseWriter, _ *http.Request) {
		for range 10 {
			_, _ = w.Write(chunk)
			w.(http.Flusher).Flush()
			time.Sleep(50 * time.Millisecond)
		}
	})
	dst := partPath(t)
	d := appupdate.NewDownloader()
	d.IdleTimeout = 300 * time.Millisecond

	err := d.Download(context.Background(), entities.Artifact{URL: url, Size: 1000}, dst, func(int64) {})

	require.NoError(t, err)
	info, err := os.Stat(dst)
	require.NoError(t, err)
	assert.Equal(t, int64(1000), info.Size())
}
