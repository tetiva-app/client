package wails

import (
	"encoding/base64"
	"sync"

	"github.com/tetiva-app/client/internal/adapters/wails/dto"
	ws "github.com/tetiva-app/client/internal/domain/usecase/websocket"
)

// WebSocketEventSink implements websocket.MessageSink by emitting Wails events.
// The emit func is injected post-construction in main.go (same pattern as sync).
type WebSocketEventSink struct {
	mu   sync.RWMutex
	emit func(name string, data any)
}

// NewWebSocketEventSink creates a sink with no emitter yet.
func NewWebSocketEventSink() *WebSocketEventSink { return &WebSocketEventSink{} }

// SetEmit wires the Wails event emitter.
func (s *WebSocketEventSink) SetEmit(fn func(name string, data any)) {
	s.mu.Lock()
	s.emit = fn
	s.mu.Unlock()
}

func (s *WebSocketEventSink) emitter() func(string, any) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.emit
}

// OnMessage emits an inbound frame on ws:message:<connID>.
func (s *WebSocketEventSink) OnMessage(connID ws.ConnectionID, m ws.InboundMessage) {
	emit := s.emitter()
	if emit == nil {
		return
	}
	// Text frames pass through as-is; binary frames are base64-encoded so
	// arbitrary bytes survive the JSON event boundary without corruption.
	data := string(m.Data)
	if m.Type == ws.MessageBinary {
		data = base64.StdEncoding.EncodeToString(m.Data)
	}
	emit("ws:message:"+connID.String(), dto.WSMessageDTO{
		Dir:  "in",
		Data: data,
		Type: m.Type.String(),
		At:   m.At.UnixMilli(),
	})
}

// OnStateChange emits a state transition on ws:state:<connID>.
func (s *WebSocketEventSink) OnStateChange(connID ws.ConnectionID, st ws.ConnState, err error) {
	emit := s.emitter()
	if emit == nil {
		return
	}
	d := dto.WSStateDTO{State: st.String()}
	if err != nil {
		d.Error = err.Error()
	}
	emit("ws:state:"+connID.String(), d)
}
