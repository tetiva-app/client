package requester_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	cw "github.com/coder/websocket"

	"github.com/tetiva-app/client/internal/adapters/requester"
	ws "github.com/tetiva-app/client/internal/domain/usecase/websocket"
)

func TestWebSocketRequesterEchoRoundTrip(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := cw.Accept(w, r, nil)
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
	defer srv.Close()

	url := "ws" + strings.TrimPrefix(srv.URL, "http")
	d := requester.NewWebSocketRequester()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	conn, err := d.Dial(ctx, ws.DialParams{URL: url})
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	defer func() { _ = conn.Close(1000, "done") }()

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
	d := requester.NewWebSocketRequester()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if _, err := d.Dial(ctx, ws.DialParams{URL: "ws://127.0.0.1:1/nope"}); err == nil {
		t.Fatal("expected dial error on refused connection")
	}
}
