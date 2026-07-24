package wails

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"github.com/tetiva-app/client/internal/adapters/wails/dto"
	ws "github.com/tetiva-app/client/internal/domain/usecase/websocket"
)

type fakeWSUsecase struct {
	connID    ws.ConnectionID
	connErr   error
	sent      []ws.OutgoingMessage
	disconnID ws.ConnectionID
}

func (f *fakeWSUsecase) Connect(_ context.Context, _ ws.ConnectOpt) (ws.ConnectionID, error) {
	return f.connID, f.connErr
}
func (f *fakeWSUsecase) Send(_ context.Context, _ ws.ConnectionID, m ws.OutgoingMessage) error {
	f.sent = append(f.sent, m)
	return nil
}
func (f *fakeWSUsecase) Disconnect(_ context.Context, id ws.ConnectionID) error {
	f.disconnID = id
	return nil
}
func (f *fakeWSUsecase) DisconnectAll(_ context.Context) error { return nil }

func TestWSServiceConnectOK(t *testing.T) {
	id := uuid.New()
	svc := NewWebSocketService(&fakeWSUsecase{connID: id}, NewWebSocketEventSink())
	res := svc.Connect(dto.WSConnectRequest{RequestID: uuid.New().String(), WorkspaceID: uuid.New().String()})
	if res.Error != nil {
		t.Fatalf("unexpected error: %+v", res.Error)
	}
	if res.Data.ConnectionID != id.String() {
		t.Fatalf("connectionId = %q", res.Data.ConnectionID)
	}
}

func TestWSServiceConnectBadUUID(t *testing.T) {
	svc := NewWebSocketService(&fakeWSUsecase{}, NewWebSocketEventSink())
	res := svc.Connect(dto.WSConnectRequest{RequestID: "not-a-uuid", WorkspaceID: uuid.New().String()})
	if res.Error == nil || res.Error.Code != ErrCodeValidation {
		t.Fatalf("expected validation error, got %+v", res.Error)
	}
}

func TestWSServiceSendAndDisconnect(t *testing.T) {
	fake := &fakeWSUsecase{}
	svc := NewWebSocketService(fake, NewWebSocketEventSink())
	id := uuid.New()
	if r := svc.Send(dto.WSSendRequest{ConnectionID: id.String(), Data: "ping"}); r.Error != nil {
		t.Fatalf("Send error: %+v", r.Error)
	}
	if len(fake.sent) != 1 || string(fake.sent[0].Data) != "ping" {
		t.Fatalf("usecase.Send not called correctly: %+v", fake.sent)
	}
	if r := svc.Disconnect(dto.WSDisconnectRequest{ConnectionID: id.String()}); r.Error != nil {
		t.Fatalf("Disconnect error: %+v", r.Error)
	}
	if fake.disconnID != id {
		t.Fatalf("disconnect id mismatch")
	}
}
