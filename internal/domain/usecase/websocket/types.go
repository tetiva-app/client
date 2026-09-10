package websocket

import (
	"errors"
	"time"

	"github.com/google/uuid"

	"github.com/tetiva-app/client/internal/domain/entities"
)

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

type OutgoingMessage struct {
	Type MessageType
	Data []byte
}

// DialParams is the fully-resolved handshake input (post env-substitution + auth).
type DialParams struct {
	URL          string
	Headers      map[string][]string
	Subprotocols []string
	// WorkspaceID selects the cookie jar used for the handshake; uuid.Nil = no jar.
	WorkspaceID uuid.UUID
}

// DialInfo is the handshake response, filled on both outcomes so a rejected
// upgrade still reports why.
type DialInfo struct {
	StatusCode      int
	ResponseHeaders map[string][]string
	Subprotocol     string
}

// ResolvedDial is the handshake input a stored request resolves to, plus the
// outcome of the pre-connect script.
type ResolvedDial struct {
	URL          string
	Headers      map[string][]string
	Subprotocols []string
	PingInterval time.Duration
	Script       *entities.ScriptResult // nil when no pre-connect script ran
	// AuthQueryKeys names the query parameters auth injected into the URL, recorded
	// with the history row so a replay draft can strip them.
	AuthQueryKeys []string
	// Failed is a preparation failure after the script stage (scheme guard, auth);
	// it rides in ConnectResult so the script output explaining it survives.
	Failed string
}

// ConnectOpt carries a client-chosen id: the UI generates it so it can subscribe before the handshake.
type ConnectOpt struct {
	ConnectionID ConnectionID
	RequestID    uuid.UUID
	WorkspaceID  uuid.UUID
	UserID       string
}

// ConnectResult reports every failure after the pre-connect script instead of a Go error,
// so the script output explaining that failure survives the trip to the UI.
type ConnectResult struct {
	Connected   bool
	Error       string
	Status      int
	Subprotocol string
	Script      *entities.ScriptResult
}
