package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
)

func dialEcho(t *testing.T, opts *websocket.DialOptions) (context.Context, *websocket.Conn) {
	t.Helper()

	srv := httptest.NewServer(http.HandlerFunc(handler))
	t.Cleanup(srv.Close)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	t.Cleanup(cancel)

	c, _, err := websocket.Dial(ctx, "ws"+strings.TrimPrefix(srv.URL, "http"), opts)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	t.Cleanup(func() { _ = c.CloseNow() })
	return ctx, c
}

func readWelcome(t *testing.T, ctx context.Context, c *websocket.Conn) welcome {
	t.Helper()

	mt, data, err := c.Read(ctx)
	if err != nil {
		t.Fatalf("read welcome: %v", err)
	}
	if mt != websocket.MessageText {
		t.Fatalf("welcome type = %v, want text", mt)
	}
	var w welcome
	if err := json.Unmarshal(data, &w); err != nil {
		t.Fatalf("welcome %q is not json: %v", data, err)
	}
	if !w.Welcome {
		t.Errorf("welcome flag = false, want true")
	}
	return w
}

func TestWelcomeFrameReportsHandshake(t *testing.T) {
	h := http.Header{}
	h.Set("X-Test", "from-script")
	h.Set("Cookie", "session=abc")

	ctx, c := dialEcho(t, &websocket.DialOptions{HTTPHeader: h, Subprotocols: []string{"graphql-ws", "json"}})

	if got := c.Subprotocol(); got != "graphql-ws" {
		t.Errorf("negotiated subprotocol = %q, want graphql-ws", got)
	}

	w := readWelcome(t, ctx, c)
	if got := w.Headers["X-Test"]; got != "from-script" {
		t.Errorf("headers[X-Test] = %q, want from-script", got)
	}
	if w.Cookie != "session=abc" {
		t.Errorf("cookie = %q, want session=abc", w.Cookie)
	}
	if w.Subprotocol != "graphql-ws" {
		t.Errorf("subprotocol = %q, want graphql-ws", w.Subprotocol)
	}
}

func TestWelcomeFrameWithoutHeaders(t *testing.T) {
	ctx, c := dialEcho(t, nil)

	w := readWelcome(t, ctx, c)
	got, ok := w.Headers["X-Test"]
	if !ok {
		t.Errorf("headers has no X-Test key: %v", w.Headers)
	}
	if got != "" {
		t.Errorf("headers[X-Test] = %q, want empty", got)
	}
	if w.Cookie != "" {
		t.Errorf("cookie = %q, want empty", w.Cookie)
	}
	if w.Subprotocol != "" {
		t.Errorf("subprotocol = %q, want empty", w.Subprotocol)
	}
}

func TestEchoKeepsFrameType(t *testing.T) {
	ctx, c := dialEcho(t, nil)
	readWelcome(t, ctx, c)

	if err := c.Write(ctx, websocket.MessageText, []byte(`{"hello":"from-script"}`)); err != nil {
		t.Fatalf("write text: %v", err)
	}
	mt, data, err := c.Read(ctx)
	if err != nil {
		t.Fatalf("read text echo: %v", err)
	}
	if mt != websocket.MessageText {
		t.Errorf("text echo type = %v, want text", mt)
	}
	if string(data) != `echo: {"hello":"from-script"}` {
		t.Errorf("text echo = %q", data)
	}

	if err := c.Write(ctx, websocket.MessageBinary, []byte("hello")); err != nil {
		t.Fatalf("write binary: %v", err)
	}
	mt, data, err = c.Read(ctx)
	if err != nil {
		t.Fatalf("read binary echo: %v", err)
	}
	if mt != websocket.MessageBinary {
		t.Errorf("binary echo type = %v, want binary", mt)
	}
	if string(data) != "hello" {
		t.Errorf("binary echo = %q, want verbatim hello", data)
	}
}
