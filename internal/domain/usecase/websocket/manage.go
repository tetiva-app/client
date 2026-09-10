package websocket

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// Send resolves {{variables}} in text frames at send time; binary payloads go verbatim.
func (u *usecase) Send(ctx context.Context, connID ConnectionID, msg OutgoingMessage) error {
	const funcName = "websocket.Send"
	u.mu.Lock()
	e, ok := u.conns[connID]
	var live bool
	var workspaceID uuid.UUID
	if ok {
		live = e.state == entryLive
		workspaceID = e.workspaceID
	}
	u.mu.Unlock()
	if !ok {
		return fmt.Errorf("%s: connection not found", funcName)
	}
	if !live {
		return fmt.Errorf("%s: connection is still being established", funcName)
	}
	if msg.Type == MessageText && u.resolver != nil {
		text, err := u.resolver.SubstituteMessage(ctx, workspaceID, string(msg.Data))
		if err != nil {
			return fmt.Errorf("%s: %w", funcName, err)
		}
		msg.Data = []byte(text)
	}
	e.writeMu.Lock()
	defer e.writeMu.Unlock()
	if err := e.conn.Write(ctx, Message(msg)); err != nil {
		return fmt.Errorf("%s: %w", funcName, err)
	}
	return nil
}

// Disconnect cancels a pending attempt or closes a live connection.
func (u *usecase) Disconnect(_ context.Context, connID ConnectionID) error {
	e, won := u.claimEntry(connID, nil)
	if !won {
		return nil // already gone, or another path is finalizing it; idempotent
	}
	// Claiming the entry silenced the read pump, so both terminal events come from
	// here. A pending attempt never announced itself and stays silent.
	live := e.state == entryLive
	if live {
		u.sink.OnStateChange(connID, StateClosing, nil)
	}
	u.finalize(e)
	if live {
		u.sink.OnStateChange(connID, StateClosed, nil)
	}
	return nil
}

// DisconnectAll finalizes every entry, pending attempts included (used on shutdown).
func (u *usecase) DisconnectAll(ctx context.Context) error {
	u.mu.Lock()
	ids := make([]ConnectionID, 0, len(u.conns))
	for id := range u.conns {
		ids = append(ids, id)
	}
	u.mu.Unlock()
	for _, id := range ids {
		_ = u.Disconnect(ctx, id)
	}
	return nil
}

func NewUsecase(dialer Dialer, resolver RequestResolver, sink MessageSink, history HistoryRepository) Usecase {
	return &usecase{
		conns:    make(map[ConnectionID]*entry),
		dialer:   dialer,
		resolver: resolver,
		sink:     sink,
		history:  history,
		now:      time.Now,
	}
}
