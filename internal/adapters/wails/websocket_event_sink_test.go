package wails

import (
	"encoding/base64"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/tetiva-app/client/internal/adapters/wails/dto"
	ws "github.com/tetiva-app/client/internal/domain/usecase/websocket"
)

func TestEventSinkEmitsMessage(t *testing.T) {
	var mu sync.Mutex
	events := map[string]any{}
	sink := NewWebSocketEventSink()
	sink.SetEmit(func(name string, data any) {
		mu.Lock()
		events[name] = data
		mu.Unlock()
	})

	id := uuid.New()
	sink.OnMessage(id, ws.InboundMessage{Type: ws.MessageText, Data: []byte("hello"), At: time.Now()})

	mu.Lock()
	defer mu.Unlock()
	if _, ok := events["ws:message:"+id.String()]; !ok {
		t.Fatalf("expected ws:message event, got keys %v", keysOf(events))
	}
}

func TestEventSinkEmitsState(t *testing.T) {
	var got string
	sink := NewWebSocketEventSink()
	sink.SetEmit(func(name string, _ any) { got = name })
	id := uuid.New()
	sink.OnStateChange(id, ws.StateError, errors.New("boom"))
	if got != "ws:state:"+id.String() {
		t.Fatalf("state event name = %q", got)
	}
}

func TestEventSinkNilEmitSafe(t *testing.T) {
	sink := NewWebSocketEventSink() // emit never set
	sink.OnMessage(uuid.New(), ws.InboundMessage{Data: []byte("x")})
	sink.OnStateChange(uuid.New(), ws.StateClosed, nil) // must not panic
}

func TestEventSinkBase64EncodesBinary(t *testing.T) {
	var captured any
	sink := NewWebSocketEventSink()
	sink.SetEmit(func(_ string, data any) { captured = data })

	raw := []byte{0x00, 0xff, 0x10} // bytes that would corrupt under string()
	sink.OnMessage(uuid.New(), ws.InboundMessage{Type: ws.MessageBinary, Data: raw, At: time.Now()})

	msg, ok := captured.(dto.WSMessageDTO)
	if !ok {
		t.Fatalf("expected WSMessageDTO, got %T", captured)
	}
	if msg.Type != "binary" {
		t.Fatalf("type = %q, want binary", msg.Type)
	}
	if msg.Data != base64.StdEncoding.EncodeToString(raw) {
		t.Fatalf("data = %q, want base64 of raw bytes", msg.Data)
	}
}

func keysOf(m map[string]any) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
