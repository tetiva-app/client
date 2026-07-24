package websocket

import (
	"context"
	"sync"

	"github.com/google/uuid"

	"github.com/tetiva-app/client/internal/domain/entities"
)

// Usecase is the public API for WebSocket connection management.
type Usecase interface {
	Connect(ctx context.Context, opt ConnectOpt) (ConnectionID, error)
	Send(ctx context.Context, connID ConnectionID, msg OutgoingMessage) error
	Disconnect(ctx context.Context, connID ConnectionID) error
	DisconnectAll(ctx context.Context) error
}

// Dialer opens a connection. Implemented by adapters/requester.
type Dialer interface {
	Dial(ctx context.Context, p DialParams) (Conn, error)
}

// Conn is a single live connection. Implemented by adapters/requester.
type Conn interface {
	Read(ctx context.Context) (Message, error) // blocks until a frame, error, or close
	Write(ctx context.Context, m Message) error
	Close(code int, reason string) error
}

// MessageSink receives inbound frames and state changes for the UI.
// Implemented by adapters/wails (emits Wails events).
type MessageSink interface {
	OnMessage(connID ConnectionID, m InboundMessage)
	OnStateChange(connID ConnectionID, st ConnState, err error)
}

// RequestResolver resolves the final handshake URL + headers from a stored request
// (env substitution + auth). Returns primitives to avoid a dependency on request.
type RequestResolver interface {
	ResolveWebSocket(ctx context.Context, requestID, workspaceID uuid.UUID, userID string) (url string, headers map[string][]string, err error)
}

// HistoryRepository persists the connection fact (append-only).
type HistoryRepository interface {
	Create(ctx context.Context, h *entities.History) error
}

// activeConn is a registry entry for one live connection.
type activeConn struct {
	conn    Conn
	cancel  context.CancelFunc
	writeMu sync.Mutex // coder/websocket allows one Write at a time
}

type usecase struct {
	mu       sync.Mutex
	conns    map[ConnectionID]*activeConn
	dialer   Dialer
	resolver RequestResolver
	sink     MessageSink
	history  HistoryRepository
	newID    func() ConnectionID
}
