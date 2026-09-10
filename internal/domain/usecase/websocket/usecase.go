package websocket

import (
	"context"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/tetiva-app/client/internal/domain/entities"
)

type Usecase interface {
	// Connect returns an error only for failures before the pre-connect script
	// (bad or duplicate id, request not found); everything later is in ConnectResult.
	Connect(ctx context.Context, opt ConnectOpt) (ConnectResult, error)
	Send(ctx context.Context, connID ConnectionID, msg OutgoingMessage) error
	Disconnect(ctx context.Context, connID ConnectionID) error
	DisconnectAll(ctx context.Context) error
}

// Dialer is implemented by adapters/requester; DialInfo carries the handshake response on both outcomes.
type Dialer interface {
	Dial(ctx context.Context, p DialParams) (Conn, DialInfo, error)
}

// Conn is a single live connection. Implemented by adapters/requester.
type Conn interface {
	Read(ctx context.Context) (Message, error) // blocks until a frame, error, or close
	Write(ctx context.Context, m Message) error
	Ping(ctx context.Context) error // blocks until the pong is read by the read pump
	Close(code int, reason string) error
}

// MessageSink is called with the registry lock held: implementations must not call back into the usecase.
type MessageSink interface {
	OnMessage(connID ConnectionID, m InboundMessage)
	OnSystem(connID ConnectionID, text string) // client-side notice, e.g. a keepalive failure
	OnStateChange(connID ConnectionID, st ConnState, err error)
}

// RequestResolver turns a stored request into handshake input (env substitution, pre-connect script,
// auth) and resolves variables in outgoing messages; an error means the attempt never started.
type RequestResolver interface {
	ResolveWebSocket(ctx context.Context, requestID, workspaceID uuid.UUID, userID string) (ResolvedDial, error)
	SubstituteMessage(ctx context.Context, workspaceID uuid.UUID, text string) (string, error)
}

// HistoryRepository persists the connection fact (append-only).
type HistoryRepository interface {
	Create(ctx context.Context, h *entities.History) error
}

// entryState separates a connection attempt from a live connection: a pending
// entry has no conn yet, so Send and DisconnectAll must not dereference it.
type entryState int

const (
	entryPending entryState = iota
	entryLive
)

// entry is a registry slot for one connection, reserved before the handshake.
type entry struct {
	state        entryState
	cancel       context.CancelFunc
	conn         Conn
	writeMu      sync.Mutex // coder/websocket allows one Write at a time
	workspaceID  uuid.UUID
	pingInterval time.Duration
	done         chan struct{} // closed by finalize; stops the ping loop
}

type usecase struct {
	mu       sync.Mutex
	conns    map[ConnectionID]*entry
	dialer   Dialer
	resolver RequestResolver
	sink     MessageSink
	history  HistoryRepository
	now      func() time.Time
}
