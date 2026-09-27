package publicapi_test

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tetiva-app/client/internal/domain"
	"github.com/tetiva-app/client/internal/infrastructure/publicapi"
)

const slug = "petstore-api-k3f9x2qa"

func gz(t *testing.T, b []byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := gzip.NewWriter(&buf)
	_, err := zw.Write(b)
	require.NoError(t, err)
	require.NoError(t, zw.Close())
	return buf.Bytes()
}

func serve(t *testing.T, h http.HandlerFunc) (*publicapi.Client, *atomic.Int32) {
	t.Helper()
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		h(w, r)
	}))
	t.Cleanup(srv.Close)
	return publicapi.New(srv.URL + "/"), &hits
}

func status(code int) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(code) }
}

func TestValidSlug(t *testing.T) {
	assert.True(t, publicapi.ValidSlug(slug))
	assert.True(t, publicapi.ValidSlug("a-12345678"))
	for _, bad := range []string{"", "petstore", "Petstore-api-k3f9x2qa", "petstore-api-k3f9x2q", "../x-12345678", "x-12345678/y",
		strings.Repeat("a", 41) + "-12345678"} {
		assert.False(t, publicapi.ValidSlug(bad), bad)
	}
}

func TestInvalidSlugNeverReachesTheServer(t *testing.T) {
	c, hits := serve(t, status(http.StatusOK))
	ctx := context.Background()

	_, errMeta := c.Meta(ctx, "../admin")
	_, errUnlock := c.Unlock(ctx, "bad slug", "password1")
	_, errSnap := c.Snapshot(ctx, "", "")

	for _, err := range []error{errMeta, errUnlock, errSnap} {
		var ve *domain.ValidationError
		assert.ErrorAs(t, err, &ve)
	}
	assert.Zero(t, hits.Load())
}

func TestMeta(t *testing.T) {
	c, _ := serve(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method)
		assert.Equal(t, "/pub/"+slug, r.URL.Path)
		assert.Contains(t, r.UserAgent(), "Tetiva/")
		_, _ = io.WriteString(w, `{"slug":"`+slug+`","title":"Petstore","locale":"ru","visibility":"password","badge":true,`+
			`"revision":3,"updated_at":"2026-09-20T10:00:00Z","password_required":true,"noindex":true,"future":1}`)
	})

	m, err := c.Meta(context.Background(), slug)
	require.NoError(t, err)
	assert.Equal(t, &publicapi.Meta{
		Slug: slug, Title: "Petstore", Locale: "ru", Visibility: "password", Revision: 3,
		UpdatedAt: time.Date(2026, 9, 20, 10, 0, 0, 0, time.UTC), PasswordRequired: true,
	}, m)
}

func TestMeta_Errors(t *testing.T) {
	cases := map[int]error{http.StatusNotFound: publicapi.ErrNotFound, http.StatusTooManyRequests: publicapi.ErrRateLimited}
	for code, want := range cases {
		c, _ := serve(t, status(code))
		_, err := c.Meta(context.Background(), slug)
		assert.ErrorIs(t, err, want, code)
	}

	c, _ := serve(t, status(http.StatusInternalServerError))
	_, err := c.Meta(context.Background(), slug)
	require.Error(t, err)
	assert.NotErrorIs(t, err, publicapi.ErrNotFound)
}

func TestUnreachableServer(t *testing.T) {
	for _, code := range []int{http.StatusBadGateway, http.StatusServiceUnavailable, http.StatusGatewayTimeout} {
		c, _ := serve(t, status(code))
		_, err := c.Meta(context.Background(), slug)
		assert.ErrorIs(t, err, publicapi.ErrUnreachable, code)
		_, err = c.Unlock(context.Background(), slug, "password1")
		assert.ErrorIs(t, err, publicapi.ErrUnreachable, code)
		_, err = c.Snapshot(context.Background(), slug, "")
		assert.ErrorIs(t, err, publicapi.ErrUnreachable, code)
	}

	c, _ := serve(t, status(http.StatusInternalServerError))
	_, err := c.Meta(context.Background(), slug)
	require.Error(t, err)
	assert.NotErrorIs(t, err, publicapi.ErrUnreachable, "a server bug is not a network problem")

	srv := httptest.NewServer(http.NotFoundHandler())
	srv.Close()
	closed := publicapi.New(srv.URL)
	_, err = closed.Meta(context.Background(), slug)
	assert.ErrorIs(t, err, publicapi.ErrUnreachable)
	_, err = closed.Snapshot(context.Background(), slug, "")
	assert.ErrorIs(t, err, publicapi.ErrUnreachable)
}

func dropMidBody(t *testing.T, encoding string, head []byte) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		if encoding != "" {
			w.Header().Set("Content-Encoding", encoding)
		}
		w.Header().Set("Content-Length", "100000")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(head)
		w.(http.Flusher).Flush()
		conn, _, err := w.(http.Hijacker).Hijack()
		require.NoError(t, err)
		_ = conn.Close()
	}
}

func TestConnectionDroppedMidBody(t *testing.T) {
	zipped := gz(t, []byte(`{"format":"tetiva","collection":{"name":"`+strings.Repeat("a", 4096)+`"}}`))
	for name, h := range map[string]http.HandlerFunc{
		"plain": dropMidBody(t, "", []byte(`{"format":"tetiva`)),
		"gzip":  dropMidBody(t, "gzip", zipped[:len(zipped)/2]),
	} {
		c, _ := serve(t, h)
		_, err := c.Snapshot(context.Background(), slug, "")
		assert.ErrorIs(t, err, publicapi.ErrUnreachable, name)
		_, err = c.Meta(context.Background(), slug)
		assert.ErrorIs(t, err, publicapi.ErrUnreachable, name)
	}

	c, _ := serve(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Encoding", "gzip")
		_, _ = w.Write([]byte("not gzip at all"))
	})
	_, err := c.Snapshot(context.Background(), slug, "")
	require.Error(t, err)
	assert.NotErrorIs(t, err, publicapi.ErrUnreachable, "a broken body the server sent in full is not a network problem")
}

func TestMeta_BodyOverLimit(t *testing.T) {
	c, _ := serve(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, `{"title":"`+strings.Repeat("a", 64<<10)+`"}`)
	})

	_, err := c.Meta(context.Background(), slug)
	require.Error(t, err)
}

func TestUnlock(t *testing.T) {
	c, _ := serve(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "/pub/"+slug+"/unlock", r.URL.Path)
		var body map[string]string
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		assert.Equal(t, map[string]string{"password": "correct horse"}, body)
		_, _ = io.WriteString(w, `{"token":"view-token","expires_at":"2026-10-20T10:00:00Z"}`)
	})

	token, err := c.Unlock(context.Background(), slug, "correct horse")
	require.NoError(t, err)
	assert.Equal(t, "view-token", token)
}

func TestUnlock_Errors(t *testing.T) {
	cases := map[int]error{
		http.StatusUnauthorized:          publicapi.ErrWrongPassword,
		http.StatusRequestEntityTooLarge: publicapi.ErrWrongPassword,
		http.StatusBadRequest:            publicapi.ErrWrongPassword,
		http.StatusNotFound:              publicapi.ErrNotFound,
		http.StatusTooManyRequests:       publicapi.ErrRateLimited,
	}
	for code, want := range cases {
		c, _ := serve(t, status(code))
		_, err := c.Unlock(context.Background(), slug, "password1")
		assert.ErrorIs(t, err, want, code)
	}
}

func TestSnapshot_Plain(t *testing.T) {
	c, _ := serve(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/pub/"+slug+"/snapshot.json", r.URL.Path)
		assert.Equal(t, "app", r.URL.Query().Get("src"))
		assert.Equal(t, "gzip", r.Header.Get("Accept-Encoding"))
		assert.Empty(t, r.Header.Get("Authorization"))
		_, _ = io.WriteString(w, `{"format":"tetiva.collection-snapshot"}`)
	})

	got, err := c.Snapshot(context.Background(), slug, "")
	require.NoError(t, err)
	assert.JSONEq(t, `{"format":"tetiva.collection-snapshot"}`, string(got))
}

func TestSnapshot_GzipAndBearer(t *testing.T) {
	body := []byte(`{"format":"tetiva.collection-snapshot","version":1}`)
	c, _ := serve(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "Bearer import-token", r.Header.Get("Authorization"))
		w.Header().Set("Content-Encoding", "gzip")
		_, _ = w.Write(gz(t, body))
	})

	got, err := c.Snapshot(context.Background(), slug, "import-token")
	require.NoError(t, err)
	assert.Equal(t, body, got)
}

func TestSnapshot_TooLarge(t *testing.T) {
	huge := bytes.Repeat([]byte("a"), 8<<20+1)
	plain, _ := serve(t, func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write(huge) })
	bomb := gz(t, huge)
	zipped, _ := serve(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Encoding", "gzip")
		_, _ = w.Write(bomb)
	})

	_, err := plain.Snapshot(context.Background(), slug, "")
	assert.ErrorIs(t, err, publicapi.ErrTooLarge)
	_, err = zipped.Snapshot(context.Background(), slug, "")
	assert.ErrorIs(t, err, publicapi.ErrTooLarge)
}

func TestSnapshot_Errors(t *testing.T) {
	cases := map[int]error{
		http.StatusUnauthorized:    publicapi.ErrPasswordRequired,
		http.StatusNotFound:        publicapi.ErrNotFound,
		http.StatusTooManyRequests: publicapi.ErrRateLimited,
	}
	for code, want := range cases {
		c, _ := serve(t, status(code))
		_, err := c.Snapshot(context.Background(), slug, "tok")
		assert.ErrorIs(t, err, want, code)
	}

	c, _ := serve(t, status(http.StatusServiceUnavailable))
	_, err := c.Snapshot(context.Background(), slug, "")
	require.Error(t, err)
	assert.NotErrorIs(t, err, publicapi.ErrNotFound)
}

func TestSnapshot_DoesNotFollowRedirects(t *testing.T) {
	var followed atomic.Int32
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		followed.Add(1)
		_, _ = io.WriteString(w, `{}`)
	}))
	t.Cleanup(target.Close)
	c, _ := serve(t, func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, target.URL+"/leak", http.StatusFound)
	})

	_, err := c.Snapshot(context.Background(), slug, "import-token")
	require.Error(t, err)
	assert.Zero(t, followed.Load())
}

func TestDefaultBaseURL_DevOverride(t *testing.T) {
	t.Setenv("TETIVA_PUBLIC_API", "")
	assert.Equal(t, "https://api.tetiva.app", publicapi.DefaultBaseURL())

	t.Setenv("TETIVA_PUBLIC_API", "http://127.0.0.1:8081")
	assert.Equal(t, "http://127.0.0.1:8081", publicapi.DefaultBaseURL())
}
