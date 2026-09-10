package requester

import (
	"context"
	"crypto/md5"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/tetiva-app/client/internal/domain"
	"github.com/tetiva-app/client/internal/domain/entities"
	"github.com/tetiva-app/client/internal/domain/usecase/request"
)

// memCookieStore is a CookieStore that ignores URL matching: the digest tests
// talk to a single origin and only care about what was stored.
type memCookieStore struct {
	mu     sync.Mutex
	jars   map[uuid.UUID]map[string]string
	writes []string
}

func newMemCookieStore() *memCookieStore {
	return &memCookieStore{jars: map[uuid.UUID]map[string]string{}}
}

func (s *memCookieStore) GetCookiesFor(_ context.Context, ws uuid.UUID, _ *url.URL) []*http.Cookie {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []*http.Cookie
	for name, value := range s.jars[ws] {
		out = append(out, &http.Cookie{Name: name, Value: value})
	}
	return out
}

func (s *memCookieStore) SetCookies(_ context.Context, ws uuid.UUID, _ *url.URL, cookies []*http.Cookie) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.jars[ws] == nil {
		s.jars[ws] = map[string]string{}
	}
	for _, c := range cookies {
		s.jars[ws][c.Name] = c.Value
		s.writes = append(s.writes, c.Name)
	}
	return nil
}

func (s *memCookieStore) has(ws uuid.UUID, name string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, ok := s.jars[ws][name]
	return ok
}

// digestServer issues RFC 7616 challenges whose nonce is bound to a session
// cookie, so a client that loses the cookie can never authenticate.
type digestServer struct {
	algorithm string
	qop       string
	username  string
	password  string

	mu       sync.Mutex
	nonces   map[string]string // session cookie value → nonce
	requests int
	lastBody string
	noCookie bool
}

func newDigestServer(algorithm, qop, username, password string) *digestServer {
	return &digestServer{
		algorithm: algorithm, qop: qop, username: username, password: password,
		nonces: map[string]string{},
	}
}

func (s *digestServer) hash(v string) string {
	if s.algorithm == "SHA-256" {
		sum := sha256.Sum256([]byte(v))
		return hex.EncodeToString(sum[:])
	}
	sum := md5.Sum([]byte(v))
	return hex.EncodeToString(sum[:])
}

func (s *digestServer) challenge(w http.ResponseWriter) {
	s.mu.Lock()
	sid := fmt.Sprintf("sid-%d", len(s.nonces)+1)
	nonce := fmt.Sprintf("nonce-%d", len(s.nonces)+1)
	s.nonces[sid] = nonce
	s.mu.Unlock()

	http.SetCookie(w, &http.Cookie{Name: "sid", Value: sid, Path: "/"})
	w.Header().Set("WWW-Authenticate", fmt.Sprintf(
		`Digest realm="tetiva", qop="%s", nonce="%s", algorithm=%s`, s.qop, nonce, s.algorithm))
	w.WriteHeader(http.StatusUnauthorized)
	// A real challenge carries a body; the library drains it before handing the
	// response back, so every digest test runs against a non-empty one.
	_, _ = w.Write([]byte(challengeBody))
}

const challengeBody = `{"error":"authentication required"}`

func (s *digestServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	s.requests++
	s.mu.Unlock()

	body, _ := io.ReadAll(r.Body)
	auth := r.Header.Get("Authorization")
	if auth == "" {
		s.challenge(w)
		return
	}

	cookie, err := r.Cookie("sid")
	if err != nil {
		s.mu.Lock()
		s.noCookie = true
		s.mu.Unlock()
		http.Error(w, "authenticated request without the session cookie", http.StatusBadRequest)
		return
	}
	s.mu.Lock()
	nonce := s.nonces[cookie.Value]
	s.mu.Unlock()

	p := parseDigestParams(auth)
	ha1 := s.hash(s.username + ":tetiva:" + s.password)
	ha2 := s.hash(r.Method + ":" + p["uri"])
	want := s.hash(strings.Join([]string{ha1, nonce, p["nc"], p["cnonce"], p["qop"], ha2}, ":"))
	if p["response"] != want || p["nonce"] != nonce || nonce == "" {
		s.challenge(w)
		return
	}

	s.mu.Lock()
	s.lastBody = string(body)
	s.mu.Unlock()
	http.SetCookie(w, &http.Cookie{Name: "granted", Value: "yes", Path: "/"})
	_, _ = w.Write([]byte("authenticated"))
}

// parseDigestParams splits a Digest header into its quote-aware key/value pairs.
func parseDigestParams(header string) map[string]string {
	out := map[string]string{}
	rest := strings.TrimSpace(strings.TrimPrefix(header, "Digest "))
	var current strings.Builder
	inQuotes := false
	flush := func() {
		pair := strings.TrimSpace(current.String())
		current.Reset()
		key, value, ok := strings.Cut(pair, "=")
		if !ok {
			return
		}
		out[strings.TrimSpace(key)] = strings.Trim(strings.TrimSpace(value), `"`)
	}
	for _, ch := range rest {
		switch {
		case ch == '"':
			inQuotes = !inQuotes
			current.WriteRune(ch)
		case ch == ',' && !inQuotes:
			flush()
		default:
			current.WriteRune(ch)
		}
	}
	flush()
	return out
}

func digestAuth(username, password string) *request.RequestAuth {
	return &request.RequestAuth{
		Type:   entities.AuthTypeDigest,
		Fields: map[string]any{"username": username, "password": password},
	}
}

func TestHTTPRequester_Digest(t *testing.T) {
	for _, algorithm := range []string{"MD5", "SHA-256"} {
		t.Run(algorithm, func(t *testing.T) {
			ds := newDigestServer(algorithm, "auth", "neo", "trinity")
			server := httptest.NewServer(ds)
			defer server.Close()

			store := newMemCookieStore()
			ws := uuid.New()
			r := NewHTTPRequester(store)
			// A plain reader has no GetBody: the retry can only carry the body
			// because the requester spooled it first.
			resp, err := r.Execute(context.Background(), request.HTTPExecuteRequest{
				Method:      entities.MethodPOST,
				URL:         server.URL + "/secret",
				Headers:     map[string][]string{"Content-Type": {"text/plain"}},
				BodyReader:  struct{ io.Reader }{strings.NewReader("challenge me")},
				WorkspaceID: ws,
				Auth:        digestAuth("neo", "trinity"),
			})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if resp.StatusCode != http.StatusOK {
				t.Fatalf("expected 200, got %d (%s)", resp.StatusCode, resp.Body)
			}
			if ds.noCookie {
				t.Fatal("the retry lost the session cookie bound to the nonce")
			}
			if ds.requests != 2 {
				t.Errorf("expected challenge + retry, got %d requests", ds.requests)
			}
			if ds.lastBody != "challenge me" {
				t.Errorf("body not replayed on the retry: %q", ds.lastBody)
			}
			if !store.has(ws, "granted") {
				t.Errorf("cookie from the final response was not written back: %v", store.writes)
			}
		})
	}
}

func TestHTTPRequester_Digest_UnsupportedChallenge(t *testing.T) {
	tests := []struct {
		name      string
		algorithm string
		qop       string
		want      string
	}{
		{name: "session algorithm", algorithm: "MD5-sess", qop: "auth", want: "MD5-sess"},
		{name: "auth-int only", algorithm: "MD5", qop: "auth-int", want: "auth-int"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(newDigestServer(tc.algorithm, tc.qop, "neo", "trinity"))
			defer server.Close()

			r := NewHTTPRequester(nil)
			_, err := r.Execute(context.Background(), request.HTTPExecuteRequest{
				Method: entities.MethodGET,
				URL:    server.URL + "/secret",
				Auth:   digestAuth("neo", "trinity"),
			})
			var ve *domain.ValidationError
			if !errors.As(err, &ve) {
				t.Fatalf("expected ValidationError, got %v", err)
			}
			if !strings.Contains(ve.Fields["auth"], tc.want) {
				t.Fatalf("expected the message to name %q, got %q", tc.want, ve.Fields["auth"])
			}
		})
	}
}

func TestHTTPRequester_Digest_WrongPasswordReturnsThe401(t *testing.T) {
	server := httptest.NewServer(newDigestServer("MD5", "auth", "neo", "trinity"))
	defer server.Close()

	r := NewHTTPRequester(newMemCookieStore())
	resp, err := r.Execute(context.Background(), request.HTTPExecuteRequest{
		Method:      entities.MethodGET,
		URL:         server.URL + "/secret",
		WorkspaceID: uuid.New(),
		Auth:        digestAuth("neo", "wrong"),
	})
	if err != nil {
		t.Fatalf("a rejected credential is a response, not an error: %v", err)
	}
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", resp.StatusCode)
	}
	if resp.Body != challengeBody {
		t.Fatalf("expected the challenge body, got %q", resp.Body)
	}
}

func TestHTTPRequester_Digest_UnanswerableChallengeReturnsTheResponse(t *testing.T) {
	tests := []struct {
		name       string
		wwwAuth    string
		wantStatus int
	}{
		{name: "basic challenge", wwwAuth: `Basic realm="tetiva"`, wantStatus: http.StatusUnauthorized},
		{name: "malformed digest challenge", wwwAuth: "Digest ,,,", wantStatus: http.StatusUnauthorized},
		{name: "no challenge at all", wwwAuth: "", wantStatus: http.StatusUnauthorized},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			const body = `{"error":"use basic"}`
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				if tc.wwwAuth != "" {
					w.Header().Set("WWW-Authenticate", tc.wwwAuth)
				}
				w.WriteHeader(http.StatusUnauthorized)
				_, _ = w.Write([]byte(body))
			}))
			defer server.Close()

			r := NewHTTPRequester(newMemCookieStore())
			resp, err := r.Execute(context.Background(), request.HTTPExecuteRequest{
				Method:      entities.MethodGET,
				URL:         server.URL + "/secret",
				WorkspaceID: uuid.New(),
				Auth:        digestAuth("neo", "trinity"),
			})
			if err != nil {
				t.Fatalf("a 401 we cannot answer is a response, not an error: %v", err)
			}
			if resp.StatusCode != tc.wantStatus {
				t.Fatalf("expected %d, got %d", tc.wantStatus, resp.StatusCode)
			}
			if resp.Body != body {
				t.Fatalf("expected the server body, got %q", resp.Body)
			}
		})
	}
}

func TestHTTPRequester_Digest_MissingCredentials(t *testing.T) {
	ds := newDigestServer("MD5", "auth", "neo", "trinity")
	server := httptest.NewServer(ds)
	defer server.Close()

	r := NewHTTPRequester(newMemCookieStore())
	_, err := r.Execute(context.Background(), request.HTTPExecuteRequest{
		Method:      entities.MethodGET,
		URL:         server.URL + "/secret",
		WorkspaceID: uuid.New(),
		Auth:        digestAuth("", "  "),
	})
	var ve *domain.ValidationError
	if !errors.As(err, &ve) {
		t.Fatalf("expected ValidationError, got %v", err)
	}
	for _, field := range []string{"username", "password"} {
		if ve.Fields[field] == "" {
			t.Errorf("expected %q to be reported missing, got %v", field, ve.Fields)
		}
	}
	if ds.requests != 0 {
		t.Errorf("nothing should reach the server, got %d requests", ds.requests)
	}
}

func TestDigestTransports_CachedPerWorkspaceOriginAndCredentials(t *testing.T) {
	cache := &digestTransports{}
	u, _ := url.Parse("https://api.example.com/one")
	other, _ := url.Parse("https://api.example.com/two")
	elsewhere, _ := url.Parse("https://other.example.com/one")
	wsA, wsB := uuid.New(), uuid.New()

	first := cache.get(wsA, u, "neo", "trinity", nil)
	if same := cache.get(wsA, other, "neo", "trinity", nil); same != first {
		t.Error("expected the same transport for the same workspace, origin and credentials")
	}
	if diff := cache.get(wsA, u, "neo", "cypher", nil); diff == first {
		t.Error("different credentials must not share a transport")
	}
	if diff := cache.get(wsB, u, "neo", "trinity", nil); diff == first {
		t.Error("different workspaces must not share a transport")
	}
	if diff := cache.get(wsA, elsewhere, "neo", "trinity", nil); diff == first {
		t.Error("different origins must not share a transport")
	}
}

// ncDigestServer hands out one long-lived nonce and refuses a count it has already seen —
// the RFC 7616 replay protection Apache and most appliances implement.
type ncDigestServer struct {
	username, password string

	mu       sync.Mutex
	used     map[string]bool
	replays  int
	requests int
}

func newNCDigestServer(username, password string) *ncDigestServer {
	return &ncDigestServer{username: username, password: password, used: map[string]bool{}}
}

func (s *ncDigestServer) hash(v string) string {
	sum := md5.Sum([]byte(v))
	return hex.EncodeToString(sum[:])
}

func (s *ncDigestServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	s.requests++
	s.mu.Unlock()

	header := r.Header.Get("Authorization")
	if header == "" {
		w.Header().Set("WWW-Authenticate", `Digest realm="tetiva", qop="auth", nonce="fixed-nonce", algorithm=MD5`)
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	p := parseDigestParams(header)
	ha1 := s.hash(s.username + ":tetiva:" + s.password)
	ha2 := s.hash(r.Method + ":" + p["uri"])
	want := s.hash(strings.Join([]string{ha1, "fixed-nonce", p["nc"], p["cnonce"], p["qop"], ha2}, ":"))
	if p["response"] != want {
		w.Header().Set("WWW-Authenticate", `Digest realm="tetiva", qop="auth", nonce="fixed-nonce", algorithm=MD5`)
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	s.mu.Lock()
	replayed := s.used[p["nc"]]
	if replayed {
		s.replays++
	}
	s.used[p["nc"]] = true
	s.mu.Unlock()

	if replayed {
		http.Error(w, "replayed nonce count "+p["nc"], http.StatusUnauthorized)
		return
	}
	_, _ = w.Write([]byte("authenticated"))
}

func (s *ncDigestServer) replayCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.replays
}

func (s *ncDigestServer) requestCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.requests
}

func TestHTTPRequester_Digest_ConcurrentSendsUseDistinctNonceCounts(t *testing.T) {
	ds := newNCDigestServer("neo", "trinity")
	server := httptest.NewServer(ds)
	defer server.Close()

	r := NewHTTPRequester(newMemCookieStore())
	ws := uuid.New()

	const senders = 12
	var wg sync.WaitGroup
	codes := make([]int, senders)
	errs := make([]error, senders)
	for i := range codes {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			resp, err := r.Execute(context.Background(), request.HTTPExecuteRequest{
				Method:      entities.MethodGET,
				URL:         server.URL + "/secret",
				WorkspaceID: ws,
				Auth:        digestAuth("neo", "trinity"),
			})
			if err != nil {
				errs[i] = err
				return
			}
			codes[i] = resp.StatusCode
		}(i)
	}
	wg.Wait()

	for i := range codes {
		if errs[i] != nil {
			t.Fatalf("send %d: %v", i, errs[i])
		}
		if codes[i] != http.StatusOK {
			t.Errorf("send %d: got %d, want 200", i, codes[i])
		}
	}
	if n := ds.replayCount(); n != 0 {
		t.Errorf("%d sends replayed a nonce count", n)
	}
	// One challenge for the first send, then the cached one for every other.
	if n := ds.requestCount(); n != senders+1 {
		t.Errorf("server saw %d requests, want %d (one challenge, then cached)", n, senders+1)
	}
}

func TestDigestEntry_QueuedSendHonoursCancel(t *testing.T) {
	release := make(chan struct{})
	blocked := make(chan struct{}, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		select {
		case blocked <- struct{}{}:
		default:
		}
		<-release
		w.Header().Set("WWW-Authenticate", `Digest realm="r", qop="auth", nonce="n"`)
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer server.Close()

	origin, err := url.Parse(server.URL)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	entry := (&digestTransports{}).get(uuid.New(), origin, "neo", "trinity", nil)

	priming := make(chan struct{})
	go func() {
		defer close(priming)
		req, reqErr := http.NewRequest(http.MethodGet, server.URL, nil)
		if reqErr != nil {
			return
		}
		if resp, rtErr := entry.RoundTrip(req); rtErr == nil {
			_ = resp.Body.Close()
		}
	}()
	<-blocked

	ctx, cancel := context.WithCancel(context.Background())
	queued := make(chan error, 1)
	go func() {
		req, reqErr := http.NewRequestWithContext(ctx, http.MethodGet, server.URL, nil)
		if reqErr != nil {
			queued <- reqErr
			return
		}
		resp, rtErr := entry.RoundTrip(req)
		if resp != nil {
			_ = resp.Body.Close()
		}
		queued <- rtErr
	}()
	cancel()

	select {
	case err := <-queued:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("queued send: %v, want context.Canceled", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("queued send waited for the priming exchange instead of its own context")
	}

	close(release)
	<-priming
}
