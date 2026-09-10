package requester

import (
	"context"
	"fmt"
	"net/http"

	cw "github.com/coder/websocket"
	"github.com/google/uuid"

	ws "github.com/tetiva-app/client/internal/domain/usecase/websocket"
)

// maxWSReadBytes lifts the per-message read limit above the 32KiB library default.
const maxWSReadBytes = 8 << 20 // 8 MiB

// WebSocketRequester dials Raw WebSocket connections via coder/websocket.
type WebSocketRequester struct {
	store CookieStore
}

func NewWebSocketRequester(store CookieStore) *WebSocketRequester {
	return &WebSocketRequester{store: store}
}

// The handshake response is returned even when the upgrade was rejected.
func (r *WebSocketRequester) Dial(ctx context.Context, p ws.DialParams) (ws.Conn, ws.DialInfo, error) {
	const funcName = "requester.WebSocketRequester.Dial"
	opts := &cw.DialOptions{}
	if len(p.Headers) > 0 {
		opts.HTTPHeader = http.Header(p.Headers)
	}
	if len(p.Subprotocols) > 0 {
		opts.Subprotocols = p.Subprotocols
	}
	// coder/websocket rewrites ws→http / wss→https before HTTPClient.Do, so the jar
	// matches Secure cookies and http.Client persists Set-Cookie from the 101 itself.
	if r.store != nil && p.WorkspaceID != uuid.Nil {
		opts.HTTPClient = &http.Client{Jar: newWorkspaceJar(ctx, r.store, p.WorkspaceID)}
	}
	c, resp, err := cw.Dial(ctx, p.URL, opts)
	info := dialInfo(resp)
	if err != nil {
		return nil, info, fmt.Errorf("%s: %w", funcName, err)
	}
	// On success the library nils resp.Body; never close it.
	info.Subprotocol = c.Subprotocol()
	c.SetReadLimit(maxWSReadBytes)
	return &wsConn{c: c}, info, nil
}

func dialInfo(resp *http.Response) ws.DialInfo {
	if resp == nil {
		return ws.DialInfo{}
	}
	headers := make(map[string][]string, len(resp.Header))
	for k, v := range resp.Header {
		headers[k] = v
	}
	return ws.DialInfo{
		StatusCode:      resp.StatusCode,
		ResponseHeaders: headers,
		Subprotocol:     resp.Header.Get("Sec-WebSocket-Protocol"),
	}
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

func (w *wsConn) Ping(ctx context.Context) error {
	return w.c.Ping(ctx)
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
