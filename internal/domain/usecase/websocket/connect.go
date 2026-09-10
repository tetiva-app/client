package websocket

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"

	"github.com/tetiva-app/client/internal/domain"
	"github.com/tetiva-app/client/internal/domain/entities"
)

// handshakeTimeout bounds one dial attempt.
const handshakeTimeout = 30 * time.Second

// pingTimeout bounds a single keepalive round-trip; a var so tests can shorten it.
var pingTimeout = 10 * time.Second

// Connect promotes a reserved id into a live connection, which outlives the caller's ctx.
func (u *usecase) Connect(ctx context.Context, opt ConnectOpt) (ConnectResult, error) {
	const funcName = "websocket.Connect"

	connID := opt.ConnectionID
	if connID == uuid.Nil {
		return ConnectResult{}, fmt.Errorf("%s: %w", funcName,
			&domain.ValidationError{Fields: map[string]string{"connectionId": "must not be empty"}})
	}

	// The live connection is detached from the caller's ctx, but stays cancellable
	// through the registry so Disconnect can abort an attempt that is still dialing.
	connCtx, cancel := context.WithCancel(context.Background())
	e := &entry{state: entryPending, cancel: cancel, done: make(chan struct{})}

	u.mu.Lock()
	if _, exists := u.conns[connID]; exists {
		u.mu.Unlock()
		cancel()
		return ConnectResult{}, fmt.Errorf("%s: %w", funcName,
			&domain.ValidationError{Fields: map[string]string{"connectionId": "already registered"}})
	}
	u.conns[connID] = e
	u.mu.Unlock()

	// Rolled back on every failure path; disarmed only after promotion. Entry-aware,
	// so a late-landing attempt never tears down a connection that reused the id.
	armed := true
	defer func() {
		if armed {
			u.closeEntry(connID, e)
		}
	}()

	resolved, err := u.resolver.ResolveWebSocket(connCtx, opt.RequestID, opt.WorkspaceID, opt.UserID)
	if err != nil {
		return ConnectResult{}, fmt.Errorf("%s: %w", funcName, err)
	}
	if resolved.Failed != "" {
		return ConnectResult{Error: resolved.Failed, Script: resolved.Script}, nil
	}

	dialCtx, dialCancel := context.WithTimeout(connCtx, handshakeTimeout)
	start := u.now()
	conn, info, dialErr := u.dialer.Dial(dialCtx, DialParams{
		URL:          resolved.URL,
		Headers:      resolved.Headers,
		Subprotocols: resolved.Subprotocols,
		WorkspaceID:  opt.WorkspaceID,
	})
	elapsed := u.now().Sub(start)
	dialCancel()

	u.recordHistory(ctx, opt, resolved, info, elapsed, dialErr)

	if dialErr != nil {
		return ConnectResult{Error: dialErr.Error(), Status: info.StatusCode, Script: resolved.Script}, nil
	}

	u.mu.Lock()
	current, ok := u.conns[connID]
	promoted := ok && current == e
	if promoted {
		e.state = entryLive
		e.conn = conn
		e.workspaceID = opt.WorkspaceID
		e.pingInterval = resolved.PingInterval
		armed = false
		// Announced under the registry lock: a Disconnect racing the promotion
		// waits here, so its closing/closed can never precede this event.
		u.sink.OnStateChange(connID, StateConnected, nil)
	}
	u.mu.Unlock()

	if !promoted {
		// Cancelled while dialing: close on the spot, never register.
		_ = conn.Close(1000, "client disconnect")
		return ConnectResult{Error: "connection cancelled", Status: info.StatusCode, Script: resolved.Script}, nil
	}

	go u.readPump(connCtx, connID, e, conn)
	if resolved.PingInterval > 0 {
		go u.pingLoop(connCtx, connID, e, resolved.PingInterval)
	}

	return ConnectResult{
		Connected:   true,
		Status:      info.StatusCode,
		Subprotocol: info.Subprotocol,
		Script:      resolved.Script,
	}, nil
}

func (u *usecase) readPump(ctx context.Context, connID ConnectionID, e *entry, conn Conn) {
	for {
		m, err := conn.Read(ctx)
		if err != nil {
			owned, won := u.claimEntry(connID, e)
			if !won {
				return // another path owns the shutdown and reports it
			}
			// A normal peer close (ErrClosed) or our own cancellation is a
			// clean shutdown, not an error state.
			st := StateError
			if errors.Is(err, context.Canceled) || errors.Is(err, ErrClosed) || ctx.Err() != nil {
				st = StateClosed
			}
			u.finalize(owned)
			u.sink.OnStateChange(connID, st, err)
			return
		}
		u.sink.OnMessage(connID, InboundMessage{Type: m.Type, Data: m.Data, At: time.Now()})
	}
}

// pingLoop keeps the connection alive. Ping is called outside writeMu: the
// library serialises control frames itself, and holding it would stall Send.
func (u *usecase) pingLoop(ctx context.Context, connID ConnectionID, e *entry, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-e.done:
			return
		case <-ticker.C:
		}

		pingCtx, cancel := context.WithTimeout(ctx, pingTimeout)
		err := e.conn.Ping(pingCtx)
		timedOut := errors.Is(pingCtx.Err(), context.DeadlineExceeded)
		cancel()
		if err == nil {
			continue
		}
		// Our own shutdown cancelled the ping; whoever started it reports the close.
		if ctx.Err() != nil {
			return
		}
		owned, won := u.claimEntry(connID, e)
		if !won {
			return
		}
		if timedOut {
			u.sink.OnSystem(connID, "keepalive ping timed out")
		} else {
			u.sink.OnSystem(connID, "keepalive ping failed: "+err.Error())
		}
		u.finalize(owned)
		u.sink.OnStateChange(connID, StateClosed, nil)
		return
	}
}

// claimEntry hands a shutdown to exactly one caller: unregistering the entry is the claim, so racing
// paths cannot both report a terminal state. A non-nil want keeps a lost attempt off its successor.
func (u *usecase) claimEntry(connID ConnectionID, want *entry) (*entry, bool) {
	u.mu.Lock()
	defer u.mu.Unlock()
	e, ok := u.conns[connID]
	if !ok || (want != nil && e != want) {
		return nil, false
	}
	delete(u.conns, connID)
	return e, true
}

// finalize is only ever run by the caller that claimed the entry.
func (u *usecase) finalize(e *entry) {
	close(e.done)
	e.cancel()
	if e.conn != nil {
		_ = e.conn.Close(1000, "client disconnect")
	}
}

// closeEntry finalizes silently — a rolled-back attempt has nothing to report. Idempotent.
func (u *usecase) closeEntry(connID ConnectionID, want *entry) {
	if e, ok := u.claimEntry(connID, want); ok {
		u.finalize(e)
	}
}

// recordHistory is non-fatal: a lost history row must not abort a live connection.
func (u *usecase) recordHistory(ctx context.Context, opt ConnectOpt, resolved ResolvedDial, info DialInfo, elapsed time.Duration, dialErr error) {
	if u.history == nil {
		return
	}
	rec := &entities.History{
		ID:              uuid.New(),
		RequestID:       opt.RequestID,
		WorkspaceID:     opt.WorkspaceID,
		Protocol:        entities.ProtocolWebSocket,
		Method:          "",
		URL:             resolved.URL,
		RequestHeaders:  resolved.Headers,
		ResponseStatus:  info.StatusCode,
		ResponseHeaders: info.ResponseHeaders,
		AuthQueryKeys:   resolved.AuthQueryKeys,
		DurationMs:      elapsed.Milliseconds(),
		CreatedAt:       u.now(),
	}
	if dialErr != nil {
		rec.ErrorMessage = dialErr.Error()
	}
	if err := u.history.Create(ctx, rec); err != nil {
		slog.Warn("websocket: failed to record connection history", "err", err)
	}
}
