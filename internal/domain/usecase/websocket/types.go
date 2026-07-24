package websocket

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

// ConnectionID identifies a live WebSocket connection within the registry.
type ConnectionID = uuid.UUID

// MessageType distinguishes text and binary WebSocket frames. The values have no
// numeric parity with coder/websocket's wire constants; the requester adapter maps them.
type MessageType int

const (
	MessageText MessageType = iota
	MessageBinary
)

func (t MessageType) String() string {
	if t == MessageBinary {
		return "binary"
	}
	return "text"
}

// ConnState is the lifecycle state of a connection, pushed to the UI.
type ConnState int

const (
	StateConnecting ConnState = iota
	StateConnected
	StateClosing
	StateClosed
	StateError
)

func (s ConnState) String() string {
	switch s {
	case StateConnecting:
		return "connecting"
	case StateConnected:
		return "connected"
	case StateClosing:
		return "closing"
	case StateClosed:
		return "closed"
	case StateError:
		return "error"
	default:
		return "unknown"
	}
}

// ErrClosed is returned by Conn.Read when the peer closed normally (NormalClosure /
// GoingAway); readPump maps it — and cancellation — to StateClosed, not StateError.
var ErrClosed = errors.New("websocket: connection closed")

// Message is a frame read from or written to a connection.
type Message struct {
	Type MessageType
	Data []byte
}

// InboundMessage is a frame received from the server, timestamped on arrival.
type InboundMessage struct {
	Type MessageType
	Data []byte
	At   time.Time
}

// OutgoingMessage is a frame the user sends.
type OutgoingMessage struct {
	Type MessageType
	Data []byte
}

// DialParams is the fully-resolved handshake input (post env-substitution + auth).
type DialParams struct {
	URL          string
	Headers      map[string][]string
	Subprotocols []string // empty in MVP
}

// ConnectOpt carries the request reference and caller identity.
type ConnectOpt struct {
	RequestID   uuid.UUID
	WorkspaceID uuid.UUID
	UserID      string
}
