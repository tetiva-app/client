package entities

import "testing"

func TestProtocolWebSocketIsValid(t *testing.T) {
	if !ProtocolWebSocket.IsValid() {
		t.Fatal("expected ProtocolWebSocket to be valid")
	}
	if string(ProtocolWebSocket) != "websocket" {
		t.Fatalf("expected wire value 'websocket', got %q", ProtocolWebSocket)
	}
}

func TestValidProtocolsIncludesWebSocket(t *testing.T) {
	found := false
	for _, p := range ValidProtocols() {
		if p == ProtocolWebSocket {
			found = true
		}
	}
	if !found {
		t.Fatal("ValidProtocols() must include ProtocolWebSocket")
	}
}
