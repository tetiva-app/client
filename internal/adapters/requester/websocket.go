package requester

import (
	"context"
	"fmt"
	"net/http"

	cw "github.com/coder/websocket"

	ws "github.com/tetiva-app/client/internal/domain/usecase/websocket"
)

// maxWSReadBytes raises the per-message read limit above the 32KiB default
// so larger JSON frames aren't truncated.
const maxWSReadBytes = 8 << 20 // 8 MiB

// WebSocketRequester dials Raw WebSocket connections via coder/websocket.
type WebSocketRequester struct{}

// NewWebSocketRequester creates a WebSocketRequester.
func NewWebSocketRequester() *WebSocketRequester { return &WebSocketRequester{} }

// Dial opens a connection and returns a ws.Conn wrapper.
func (r *WebSocketRequester) Dial(ctx context.Context, p ws.DialParams) (ws.Conn, error) {
	const funcName = "requester.WebSocketRequester.Dial"
	opts := &cw.DialOptions{}
	if len(p.Headers) > 0 {
		opts.HTTPHeader = http.Header(p.Headers)
	}
	if len(p.Subprotocols) > 0 {
		opts.Subprotocols = p.Subprotocols
	}
	c, _, err := cw.Dial(ctx, p.URL, opts)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", funcName, err)
	}
	c.SetReadLimit(maxWSReadBytes)
	return &wsConn{c: c}, nil
}

// wsConn adapts *coder/websocket.Conn to the ws.Conn interface.
type wsConn struct {
	c *cw.Conn
}

func (w *wsConn) Read(ctx context.Context) (ws.Message, error) {
	mt, data, err := w.c.Read(ctx)
	if err != nil {
		// Map a normal / going-away peer close to the domain ErrClosed sentinel
		// so the usecase reports StateClosed rather than StateError.
		switch cw.CloseStatus(err) {
		case cw.StatusNormalClosure, cw.StatusGoingAway, cw.StatusNoStatusRcvd:
			return ws.Message{}, ws.ErrClosed
		}
		return ws.Message{}, err
	}
	return ws.Message{Type: fromCoderType(mt), Data: data}, nil
}

func (w *wsConn) Write(ctx context.Context, m ws.Message) error {
	return w.c.Write(ctx, toCoderType(m.Type), m.Data)
}

func (w *wsConn) Close(code int, reason string) error {
	return w.c.Close(cw.StatusCode(code), reason)
}

func fromCoderType(mt cw.MessageType) ws.MessageType {
	if mt == cw.MessageBinary {
		return ws.MessageBinary
	}
	return ws.MessageText
}

func toCoderType(mt ws.MessageType) cw.MessageType {
	if mt == ws.MessageBinary {
		return cw.MessageBinary
	}
	return cw.MessageText
}
