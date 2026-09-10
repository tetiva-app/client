package wails

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/tetiva-app/client/internal/adapters/wails/dto"
	"github.com/tetiva-app/client/internal/domain/entities"
	ws "github.com/tetiva-app/client/internal/domain/usecase/websocket"
)

type fakeWSUsecase struct {
	result    ws.ConnectResult
	connErr   error
	opt       ws.ConnectOpt
	sent      []ws.OutgoingMessage
	disconnID ws.ConnectionID
}

func (f *fakeWSUsecase) Connect(_ context.Context, opt ws.ConnectOpt) (ws.ConnectResult, error) {
	f.opt = opt
	return f.result, f.connErr
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
	fake := &fakeWSUsecase{result: ws.ConnectResult{
		Connected:   true,
		Status:      101,
		Subprotocol: "chat",
		Script:      &entities.ScriptResult{PreConsole: []string{"hi"}},
	}}
	svc := NewWebSocketService(fake, NewWebSocketEventSink())
	connID := uuid.New()
	res := svc.Connect(dto.WSConnectRequest{
		RequestID:    uuid.New().String(),
		WorkspaceID:  uuid.New().String(),
		ConnectionID: connID.String(),
	})
	if res.Error != nil {
		t.Fatalf("unexpected error: %+v", res.Error)
	}
	if !res.Data.Connected || res.Data.Status != 101 || res.Data.Subprotocol != "chat" {
		t.Fatalf("data = %+v", res.Data)
	}
	if res.Data.ConnectionID != connID.String() {
		t.Fatalf("connectionId = %q", res.Data.ConnectionID)
	}
	if res.Data.Script == nil || len(res.Data.Script.PreConsole) != 1 {
		t.Fatalf("script = %+v", res.Data.Script)
	}
	if fake.opt.ConnectionID != connID || fake.opt.UserID != defaultUserID {
		t.Fatalf("usecase got opt %+v", fake.opt)
	}
}

func TestWSServiceConnectFailedHandshakeIsData(t *testing.T) {
	fake := &fakeWSUsecase{result: ws.ConnectResult{
		Status: 401,
		Error:  "handshake rejected",
		Script: &entities.ScriptResult{PreConsole: []string{"hi"}},
	}}
	svc := NewWebSocketService(fake, NewWebSocketEventSink())
	res := svc.Connect(dto.WSConnectRequest{
		RequestID:    uuid.New().String(),
		WorkspaceID:  uuid.New().String(),
		ConnectionID: uuid.New().String(),
	})
	if res.Error != nil {
		t.Fatalf("a rejected handshake must not be a Result error: %+v", res.Error)
	}
	if res.Data.Connected || res.Data.Status != 401 || res.Data.Error != "handshake rejected" {
		t.Fatalf("data = %+v", res.Data)
	}
	if res.Data.Script == nil {
		t.Fatal("expected the script outcome to survive a failed handshake")
	}
}

func TestWSServiceConnectUsecaseErrorIsError(t *testing.T) {
	svc := NewWebSocketService(&fakeWSUsecase{connErr: errors.New("boom")}, NewWebSocketEventSink())
	res := svc.Connect(dto.WSConnectRequest{
		RequestID:    uuid.New().String(),
		WorkspaceID:  uuid.New().String(),
		ConnectionID: uuid.New().String(),
	})
	if res.Error == nil {
		t.Fatal("expected an error result")
	}
}

func TestWSServiceConnectBadUUID(t *testing.T) {
	svc := NewWebSocketService(&fakeWSUsecase{}, NewWebSocketEventSink())
	res := svc.Connect(dto.WSConnectRequest{RequestID: "not-a-uuid", WorkspaceID: uuid.New().String(), ConnectionID: uuid.New().String()})
	if res.Error == nil || res.Error.Code != ErrCodeValidation {
		t.Fatalf("expected validation error, got %+v", res.Error)
	}
	res = svc.Connect(dto.WSConnectRequest{RequestID: uuid.New().String(), WorkspaceID: uuid.New().String(), ConnectionID: "nope"})
	if res.Error == nil || res.Error.Fields["connectionId"] == "" {
		t.Fatalf("expected a connectionId validation error, got %+v", res.Error)
	}
}

func TestWSServiceSendAndDisconnect(t *testing.T) {
	fake := &fakeWSUsecase{}
	svc := NewWebSocketService(fake, NewWebSocketEventSink())
	id := uuid.New()
	if r := svc.Send(dto.WSSendRequest{ConnectionID: id.String(), Data: "ping", MessageType: "text"}); r.Error != nil {
		t.Fatalf("Send error: %+v", r.Error)
	}
	if len(fake.sent) != 1 || string(fake.sent[0].Data) != "ping" || fake.sent[0].Type != ws.MessageText {
		t.Fatalf("usecase.Send not called correctly: %+v", fake.sent)
	}
	if r := svc.Disconnect(dto.WSDisconnectRequest{ConnectionID: id.String()}); r.Error != nil {
		t.Fatalf("Disconnect error: %+v", r.Error)
	}
	if fake.disconnID != id {
		t.Fatalf("disconnect id mismatch")
	}
}

func TestWSServiceSendBinaryDecodesBase64(t *testing.T) {
	fake := &fakeWSUsecase{}
	svc := NewWebSocketService(fake, NewWebSocketEventSink())
	if r := svc.Send(dto.WSSendRequest{ConnectionID: uuid.New().String(), Data: "aGVsbG8=", MessageType: "binary"}); r.Error != nil {
		t.Fatalf("Send error: %+v", r.Error)
	}
	if len(fake.sent) != 1 || fake.sent[0].Type != ws.MessageBinary || string(fake.sent[0].Data) != "hello" {
		t.Fatalf("binary frame = %+v", fake.sent)
	}
}

func TestWSServiceSendRejectsBadBinaryAndType(t *testing.T) {
	svc := NewWebSocketService(&fakeWSUsecase{}, NewWebSocketEventSink())
	r := svc.Send(dto.WSSendRequest{ConnectionID: uuid.New().String(), Data: "not base64", MessageType: "binary"})
	if r.Error == nil || r.Error.Code != ErrCodeValidation {
		t.Fatalf("expected validation error for bad base64, got %+v", r.Error)
	}
	r = svc.Send(dto.WSSendRequest{ConnectionID: uuid.New().String(), Data: "x", MessageType: "protobuf"})
	if r.Error == nil || r.Error.Fields["messageType"] == "" {
		t.Fatalf("expected a messageType validation error, got %+v", r.Error)
	}
}
