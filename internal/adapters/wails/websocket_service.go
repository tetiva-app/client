package wails

import (
	"context"
	"encoding/base64"
	"fmt"

	"github.com/google/uuid"
	"github.com/wailsapp/wails/v3/pkg/application"

	"github.com/tetiva-app/client/internal/adapters/wails/dto"
	"github.com/tetiva-app/client/internal/domain"
	ws "github.com/tetiva-app/client/internal/domain/usecase/websocket"
)

// WebSocketService exposes WebSocket operations to the Wails frontend.
type WebSocketService struct {
	uc   ws.Usecase
	sink *WebSocketEventSink
}

func NewWebSocketService(uc ws.Usecase, sink *WebSocketEventSink) *WebSocketService {
	return &WebSocketService{uc: uc, sink: sink}
}

// SetEventEmitter wires the Wails event system into the sink (called in main.go).
func (s *WebSocketService) SetEventEmitter(fn func(name string, data any)) {
	s.sink.SetEmit(fn)
}

// Connect opens a connection under the id the frontend generated, owned by the calling window.
func (s *WebSocketService) Connect(ctx context.Context, req dto.WSConnectRequest) Result[dto.WSConnectResultDTO] {
	requestID, err := uuid.Parse(req.RequestID)
	if err != nil {
		return Err[dto.WSConnectResultDTO](&domain.ValidationError{Fields: map[string]string{"requestId": "invalid UUID"}})
	}
	workspaceID, err := uuid.Parse(req.WorkspaceID)
	if err != nil {
		return Err[dto.WSConnectResultDTO](&domain.ValidationError{Fields: map[string]string{"workspaceId": "invalid UUID"}})
	}
	connID, err := uuid.Parse(req.ConnectionID)
	if err != nil {
		return Err[dto.WSConnectResultDTO](&domain.ValidationError{Fields: map[string]string{"connectionId": "invalid UUID"}})
	}
	res, err := s.uc.Connect(context.Background(), ws.ConnectOpt{
		ConnectionID: connID,
		RequestID:    requestID,
		WorkspaceID:  workspaceID,
		UserID:       defaultUserID,
		Owner:        callerWindow(ctx),
	})
	if err != nil {
		return Err[dto.WSConnectResultDTO](err)
	}
	return OK(dto.WSConnectResultDTO{
		Connected:    res.Connected,
		ConnectionID: connID.String(),
		Status:       res.Status,
		Subprotocol:  res.Subprotocol,
		Error:        res.Error,
		Script:       dto.ScriptResultToDTO(res.Script),
	})
}

func (s *WebSocketService) Send(req dto.WSSendRequest) Result[Empty] {
	ctx := context.Background()
	connID, err := uuid.Parse(req.ConnectionID)
	if err != nil {
		return Err[Empty](&domain.ValidationError{Fields: map[string]string{"connectionId": "invalid UUID"}})
	}
	msg := ws.OutgoingMessage{Type: ws.MessageText, Data: []byte(req.Data)}
	switch req.MessageType {
	case "text":
	case "binary":
		raw, decErr := base64.StdEncoding.DecodeString(req.Data)
		if decErr != nil {
			return Err[Empty](&domain.ValidationError{Fields: map[string]string{"data": "invalid base64"}})
		}
		msg = ws.OutgoingMessage{Type: ws.MessageBinary, Data: raw}
	default:
		return Err[Empty](&domain.ValidationError{Fields: map[string]string{"messageType": "must be text or binary"}})
	}
	if err := s.uc.Send(ctx, connID, msg); err != nil {
		return Err[Empty](err)
	}
	return OK(Empty{})
}

// Disconnect cancels a pending attempt or closes a live connection.
func (s *WebSocketService) Disconnect(req dto.WSDisconnectRequest) Result[Empty] {
	ctx := context.Background()
	connID, err := uuid.Parse(req.ConnectionID)
	if err != nil {
		return Err[Empty](&domain.ValidationError{Fields: map[string]string{"connectionId": "invalid UUID"}})
	}
	if err := s.uc.Disconnect(ctx, connID); err != nil {
		return Err[Empty](err)
	}
	return OK(Empty{})
}

// DisconnectWindow runs on page load, since a reload skips the editors' unmount.
func (s *WebSocketService) DisconnectWindow(ctx context.Context) Result[Empty] {
	const funcName = "WebSocketService.DisconnectWindow"
	owner := callerWindow(ctx)
	if owner == "" {
		return OK(Empty{})
	}
	if err := s.uc.DisconnectOwner(context.Background(), owner); err != nil {
		return Err[Empty](fmt.Errorf("%s: %w", funcName, err))
	}
	return OK(Empty{})
}

func callerWindow(ctx context.Context) string {
	if w, ok := ctx.Value(application.WindowKey).(application.Window); ok {
		return w.Name()
	}
	return ""
}
