// Command wsecho is a local WebSocket echo server for testing the client: it greets with a
// welcome frame describing the handshake, echoes every frame and pushes a periodic tick.
// Run: `go run ./cmd/wsecho` → ws://127.0.0.1:9876
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/coder/websocket"
)

const tickInterval = 5 * time.Second

type welcome struct {
	Welcome     bool              `json:"welcome"`
	Headers     map[string]string `json:"headers"`
	Cookie      string            `json:"cookie"`
	Subprotocol string            `json:"subprotocol"`
}

func handler(w http.ResponseWriter, r *http.Request) {
	c, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		InsecureSkipVerify: true,
		Subprotocols:       requestedSubprotocols(r),
	})
	if err != nil {
		return
	}
	defer func() { _ = c.Close(websocket.StatusNormalClosure, "") }()
	ctx := r.Context()

	hello, err := json.Marshal(welcome{
		Welcome:     true,
		Headers:     map[string]string{"X-Test": r.Header.Get("X-Test")},
		Cookie:      r.Header.Get("Cookie"),
		Subprotocol: c.Subprotocol(),
	})
	if err != nil {
		return
	}
	if err := c.Write(ctx, websocket.MessageText, hello); err != nil {
		return
	}

	go tick(ctx, c)

	for {
		mt, data, err := c.Read(ctx)
		if err != nil {
			return
		}
		// binary frames go back verbatim, the client renders them as a bin row
		if mt != websocket.MessageBinary {
			data = append([]byte("echo: "), data...)
		}
		if err := c.Write(ctx, mt, data); err != nil {
			return
		}
	}
}

func tick(ctx context.Context, c *websocket.Conn) {
	t := time.NewTicker(tickInterval)
	defer t.Stop()
	n := 0
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			n++
			if err := c.Write(ctx, websocket.MessageText, []byte(fmt.Sprintf("tick %d", n))); err != nil {
				return
			}
		}
	}
}

// Accept negotiates only what it is offered, so mirror the client list back to it.
func requestedSubprotocols(r *http.Request) []string {
	var out []string
	for _, v := range r.Header.Values("Sec-WebSocket-Protocol") {
		for _, p := range strings.Split(v, ",") {
			if p = strings.TrimSpace(p); p != "" {
				out = append(out, p)
			}
		}
	}
	return out
}

func main() {
	mux := http.NewServeMux()
	mux.HandleFunc("/", handler)
	log.Println("wsecho listening on ws://127.0.0.1:9876")
	if err := http.ListenAndServe("127.0.0.1:9876", mux); err != nil {
		log.Fatal(err)
	}
}
