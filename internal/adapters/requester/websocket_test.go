package requester_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	cw "github.com/coder/websocket"
	"github.com/google/uuid"

	"github.com/tetiva-app/client/internal/adapters/requester"
	ws "github.com/tetiva-app/client/internal/domain/usecase/websocket"
)

// fakeCookieStore is a requester.CookieStore that serves a fixed cookie set and
// records what the jar persisted from the handshake response.
type fakeCookieStore struct {
	mu       sync.Mutex
	send     []*http.Cookie
	setURL   *url.URL
	setWS    uuid.UUID
	received []*http.Cookie
}

func (s *fakeCookieStore) GetCookiesFor(_ context.Context, _ uuid.UUID, _ *url.URL) []*http.Cookie {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.send
}

func (s *fakeCookieStore) SetCookies(_ context.Context, wsID uuid.UUID, u *url.URL, cookies []*http.Cookie) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.setWS, s.setURL = wsID, u
	s.received = append(s.received, cookies...)
	return nil
}

func (s *fakeCookieStore) snapshot() (uuid.UUID, *url.URL, []*http.Cookie) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.setWS, s.setURL, append([]*http.Cookie(nil), s.received...)
}

func wsURL(t *testing.T, srv *httptest.Server) string {
	t.Helper()
	return "ws" + strings.TrimPrefix(srv.URL, "http")
}

func echoServer(t *testing.T, opts *cw.AcceptOptions, before func(http.ResponseWriter, *http.Request)) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if before != nil {
			before(w, r)
		}
		c, err := cw.Accept(w, r, opts)
		if err != nil {
			return
		}
		defer func() { _ = c.Close(cw.StatusNormalClosure, "") }()
		for {
			mt, data, err := c.Read(r.Context())
			if err != nil {
				return
			}
			_ = c.Write(r.Context(), mt, data)
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestWebSocketRequesterEchoRoundTrip(t *testing.T) {
	srv := echoServer(t, nil, nil)

	d := requester.NewWebSocketRequester(nil)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	conn, info, err := d.Dial(ctx, ws.DialParams{URL: wsURL(t, srv)})
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	defer func() { _ = conn.Close(1000, "done") }()

	if info.StatusCode != http.StatusSwitchingProtocols {
		t.Fatalf("handshake status = %d, want 101", info.StatusCode)
	}
	if err := conn.Write(ctx, ws.Message{Type: ws.MessageText, Data: []byte("hi")}); err != nil {
		t.Fatalf("Write: %v", err)
	}
	got, err := conn.Read(ctx)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if string(got.Data) != "hi" || got.Type != ws.MessageText {
		t.Fatalf("echo = %q/%v, want hi/text", got.Data, got.Type)
	}
}

func TestWebSocketRequesterDialErrorOnBadURL(t *testing.T) {
	d := requester.NewWebSocketRequester(nil)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if _, _, err := d.Dial(ctx, ws.DialParams{URL: "ws://127.0.0.1:1/nope"}); err == nil {
		t.Fatal("expected dial error on refused connection")
	}
}

func TestWebSocketRequesterSendsWorkspaceCookies(t *testing.T) {
	var mu sync.Mutex
	var sent string
	srv := echoServer(t, nil, func(_ http.ResponseWriter, r *http.Request) {
		mu.Lock()
		sent = r.Header.Get("Cookie")
		mu.Unlock()
	})

	store := &fakeCookieStore{send: []*http.Cookie{{Name: "session", Value: "abc"}}}
	d := requester.NewWebSocketRequester(store)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	conn, _, err := d.Dial(ctx, ws.DialParams{URL: wsURL(t, srv), WorkspaceID: uuid.New()})
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	defer func() { _ = conn.Close(1000, "done") }()

	mu.Lock()
	defer mu.Unlock()
	if sent != "session=abc" {
		t.Fatalf("upgrade Cookie header = %q, want session=abc", sent)
	}
}

func TestWebSocketRequesterStoresSetCookieFromHandshake(t *testing.T) {
	srv := echoServer(t, nil, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Set-Cookie", "sid=xyz; Path=/")
	})

	store := &fakeCookieStore{}
	wsID := uuid.New()
	d := requester.NewWebSocketRequester(store)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	conn, _, err := d.Dial(ctx, ws.DialParams{URL: wsURL(t, srv), WorkspaceID: wsID})
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	defer func() { _ = conn.Close(1000, "done") }()

	gotWS, gotURL, cookies := store.snapshot()
	if len(cookies) != 1 || cookies[0].Name != "sid" || cookies[0].Value != "xyz" {
		t.Fatalf("persisted cookies = %+v, want sid=xyz", cookies)
	}
	if gotWS != wsID {
		t.Fatalf("persisted for workspace %s, want %s", gotWS, wsID)
	}
	if gotURL == nil || gotURL.Scheme != "http" {
		t.Fatalf("jar saw URL %v, want http scheme", gotURL)
	}
}

func TestWebSocketRequesterKeepsRejectedHandshakeResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("X-Reason", "nope")
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	d := requester.NewWebSocketRequester(nil)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	conn, info, err := d.Dial(ctx, ws.DialParams{URL: wsURL(t, srv)})
	if err == nil {
		_ = conn.Close(1000, "")
		t.Fatal("expected an error for a rejected upgrade")
	}
	if info.StatusCode != http.StatusUnauthorized {
		t.Fatalf("DialInfo.StatusCode = %d, want 401", info.StatusCode)
	}
	if got := info.ResponseHeaders["X-Reason"]; len(got) != 1 || got[0] != "nope" {
		t.Fatalf("DialInfo.ResponseHeaders[X-Reason] = %v, want [nope]", got)
	}
}

func TestWebSocketRequesterReportsNegotiatedSubprotocol(t *testing.T) {
	srv := echoServer(t, &cw.AcceptOptions{Subprotocols: []string{"graphql-ws"}}, nil)

	d := requester.NewWebSocketRequester(nil)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	conn, info, err := d.Dial(ctx, ws.DialParams{URL: wsURL(t, srv), Subprotocols: []string{"graphql-ws"}})
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	defer func() { _ = conn.Close(1000, "done") }()

	if info.Subprotocol != "graphql-ws" {
		t.Fatalf("DialInfo.Subprotocol = %q, want graphql-ws", info.Subprotocol)
	}
}

func TestWebSocketRequesterPing(t *testing.T) {
	srv := echoServer(t, nil, nil)

	d := requester.NewWebSocketRequester(nil)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	conn, _, err := d.Dial(ctx, ws.DialParams{URL: wsURL(t, srv)})
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	defer func() { _ = conn.Close(1000, "done") }()

	// Pongs are only processed by the read loop, so a lone Ping would block.
	readCtx, stopRead := context.WithCancel(context.Background())
	defer stopRead()
	go func() {
		for {
			if _, err := conn.Read(readCtx); err != nil {
				return
			}
		}
	}()

	pingCtx, pingCancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer pingCancel()
	if err := conn.Ping(pingCtx); err != nil {
		t.Fatalf("Ping: %v", err)
	}
}
