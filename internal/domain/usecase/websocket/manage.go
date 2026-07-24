package websocket

import (
	"context"
	"fmt"

	"github.com/google/uuid"
)

// Send writes one frame to a live connection.
func (u *usecase) Send(ctx context.Context, connID ConnectionID, msg OutgoingMessage) error {
	const funcName = "websocket.Send"
	u.mu.Lock()
	ac, ok := u.conns[connID]
	u.mu.Unlock()
	if !ok {
		return fmt.Errorf("%s: connection not found", funcName)
	}
	ac.writeMu.Lock()
	defer ac.writeMu.Unlock()
	if err := ac.conn.Write(ctx, Message(msg)); err != nil {
		return fmt.Errorf("%s: %w", funcName, err)
	}
	return nil
}

// Disconnect cancels the read pump and closes the connection.
func (u *usecase) Disconnect(_ context.Context, connID ConnectionID) error {
	u.mu.Lock()
	ac, ok := u.conns[connID]
	if ok {
		delete(u.conns, connID)
	}
	u.mu.Unlock()
	if !ok {
		return nil // already gone; idempotent
	}
	// Emit only StateClosing here; the cancelled readPump emits StateClosed,
	// so emitting it here too would duplicate the closed event in the UI.
	u.sink.OnStateChange(connID, StateClosing, nil)
	ac.cancel()
	_ = ac.conn.Close(1000, "client disconnect")
	return nil
}

// DisconnectAll closes every live connection (used on app shutdown).
func (u *usecase) DisconnectAll(_ context.Context) error {
	u.mu.Lock()
	all := make([]*activeConn, 0, len(u.conns))
	for id, ac := range u.conns {
		all = append(all, ac)
		delete(u.conns, id)
	}
	u.mu.Unlock()
	for _, ac := range all {
		ac.cancel()
		_ = ac.conn.Close(1000, "shutdown")
	}
	return nil
}

// NewUsecase creates the WebSocket usecase.
func NewUsecase(dialer Dialer, resolver RequestResolver, sink MessageSink, history HistoryRepository) Usecase {
	return &usecase{
		conns:    make(map[ConnectionID]*activeConn),
		dialer:   dialer,
		resolver: resolver,
		sink:     sink,
		history:  history,
		newID:    func() ConnectionID { return uuid.New() },
	}
}
