package appupdate_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tetiva-app/client/internal/infrastructure/appupdate"
)

func newSource(t *testing.T, h http.HandlerFunc) *appupdate.Source {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	s := appupdate.NewSource(t.TempDir(), "1.2.1")
	s.URL = srv.URL + "/updates/latest.json"
	return s
}

func TestSource_Fetch(t *testing.T) {
	var got *http.Request
	s := newSource(t, func(w http.ResponseWriter, r *http.Request) {
		got = r
		_, _ = w.Write([]byte(`{"version":"1.2.2"}`))
	})

	raw, err := s.Fetch(context.Background())

	require.NoError(t, err)
	assert.Equal(t, `{"version":"1.2.2"}`, string(raw))
	assert.Equal(t, "/updates/latest.json", got.URL.Path)
	assert.Equal(t, "1.2.1", got.URL.Query().Get("v"))
	assert.Equal(t, runtime.GOOS, got.URL.Query().Get("os"))
	assert.Len(t, got.URL.Query(), 2)
	assert.True(t, strings.HasPrefix(got.UserAgent(), "Tetiva/1.2.1"), got.UserAgent())
}

func TestSource_Failures(t *testing.T) {
	cases := map[string]http.HandlerFunc{
		"redirect": func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/elsewhere" {
				_, _ = w.Write([]byte(`{"version":"1.2.2"}`))
				return
			}
			http.Redirect(w, r, "/elsewhere", http.StatusFound)
		},
		"too large": func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write([]byte(strings.Repeat("x", appupdate.MaxManifestBytes+1)))
		},
		"server error": func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		},
	}
	for name, h := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := newSource(t, h).Fetch(context.Background())
			assert.Error(t, err)
		})
	}
}
