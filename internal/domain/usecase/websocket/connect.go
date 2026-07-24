package websocket

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"

	"github.com/tetiva-app/client/internal/domain/entities"
)

// Connect resolves the request, dials, registers the connection, starts the read
// pump, and records history. The connection outlives the caller's ctx.
func (u *usecase) Connect(ctx context.Context, opt ConnectOpt) (ConnectionID, error) {
	const funcName = "websocket.Connect"

	url, headers, err := u.resolver.ResolveWebSocket(ctx, opt.RequestID, opt.WorkspaceID, opt.UserID)
	if err != nil {
		return uuid.Nil, fmt.Errorf("%s: %w", funcName, err)
	}

	// Dial with the caller ctx plus a handshake timeout so a cancelled RPC
	// aborts a hanging handshake — never dial on a detached background ctx.
	dialCtx, dialCancel := context.WithTimeout(ctx, 30*time.Second)
	conn, err := u.dialer.Dial(dialCtx, DialParams{URL: url, Headers: headers})
	dialCancel()
	if err != nil {
		u.recordHistory(ctx, opt, url, 0, err)
		return uuid.Nil, fmt.Errorf("%s: %w", funcName, err)
	}

	// Only AFTER a successful dial, detach from the caller ctx so the live
	// connection + read pump survive the RPC return.
	connCtx, cancel := context.WithCancel(context.Background())

	connID := u.newID()
	u.mu.Lock()
	u.conns[connID] = &activeConn{conn: conn, cancel: cancel}
	u.mu.Unlock()

	u.sink.OnStateChange(connID, StateConnected, nil)
	u.recordHistory(ctx, opt, url, 101, nil)

	go u.readPump(connCtx, connID, conn)

	return connID, nil
}

// readPump reads frames until the connection errors or is cancelled.
func (u *usecase) readPump(ctx context.Context, connID ConnectionID, conn Conn) {
	for {
		m, err := conn.Read(ctx)
		if err != nil {
			// A normal peer close (ErrClosed) or our own cancellation is a
			// clean shutdown, not an error state.
			st := StateError
			if errors.Is(err, context.Canceled) || errors.Is(err, ErrClosed) {
				st = StateClosed
			}
			u.sink.OnStateChange(connID, st, err)
			u.removeConn(connID)
			return
		}
		u.sink.OnMessage(connID, InboundMessage{Type: m.Type, Data: m.Data, At: time.Now()})
	}
}

// removeConn deletes a connection from the registry (idempotent).
func (u *usecase) removeConn(connID ConnectionID) {
	u.mu.Lock()
	defer u.mu.Unlock()
	delete(u.conns, connID)
}

// recordHistory writes a single connection-fact record. Non-fatal on error.
func (u *usecase) recordHistory(ctx context.Context, opt ConnectOpt, url string, status int, dialErr error) {
	if u.history == nil {
		return
	}
	rec := &entities.History{
		ID:             uuid.New(),
		RequestID:      opt.RequestID,
		WorkspaceID:    opt.WorkspaceID,
		Protocol:       entities.ProtocolWebSocket,
		Method:         "",
		URL:            url,
		ResponseStatus: status,
		CreatedAt:      time.Now(),
	}
	if dialErr != nil {
		rec.ErrorMessage = dialErr.Error()
	}
	if err := u.history.Create(ctx, rec); err != nil {
		slog.Warn("websocket: failed to record connection history", "err", err)
	}
}
