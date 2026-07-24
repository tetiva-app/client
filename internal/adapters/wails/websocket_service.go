package wails

import (
	"context"

	"github.com/google/uuid"

	"github.com/tetiva-app/client/internal/adapters/wails/dto"
	"github.com/tetiva-app/client/internal/domain"
	ws "github.com/tetiva-app/client/internal/domain/usecase/websocket"
)

// WebSocketService exposes WebSocket operations to the Wails frontend.
type WebSocketService struct {
	uc   ws.Usecase
	sink *WebSocketEventSink
}

// NewWebSocketService creates a WebSocketService.
func NewWebSocketService(uc ws.Usecase, sink *WebSocketEventSink) *WebSocketService {
	return &WebSocketService{uc: uc, sink: sink}
}

// SetEventEmitter wires the Wails event system into the sink (called in main.go).
func (s *WebSocketService) SetEventEmitter(fn func(name string, data any)) {
	s.sink.SetEmit(fn)
}

// Connect opens a connection for the given request and returns its id.
func (s *WebSocketService) Connect(req dto.WSConnectRequest) Result[dto.WSConnectionDTO] {
	ctx := context.Background()
	requestID, err := uuid.Parse(req.RequestID)
	if err != nil {
		return Err[dto.WSConnectionDTO](&domain.ValidationError{Fields: map[string]string{"requestId": "invalid UUID"}})
	}
	workspaceID, err := uuid.Parse(req.WorkspaceID)
	if err != nil {
		return Err[dto.WSConnectionDTO](&domain.ValidationError{Fields: map[string]string{"workspaceId": "invalid UUID"}})
	}
	connID, err := s.uc.Connect(ctx, ws.ConnectOpt{RequestID: requestID, WorkspaceID: workspaceID, UserID: req.UserID})
	if err != nil {
		return Err[dto.WSConnectionDTO](err)
	}
	return OK(dto.WSConnectionDTO{ConnectionID: connID.String()})
}

// Send writes one text frame to a live connection.
func (s *WebSocketService) Send(req dto.WSSendRequest) Result[Empty] {
	ctx := context.Background()
	connID, err := uuid.Parse(req.ConnectionID)
	if err != nil {
		return Err[Empty](&domain.ValidationError{Fields: map[string]string{"connectionId": "invalid UUID"}})
	}
	if err := s.uc.Send(ctx, connID, ws.OutgoingMessage{Type: ws.MessageText, Data: []byte(req.Data)}); err != nil {
		return Err[Empty](err)
	}
	return OK(Empty{})
}

// Disconnect closes a live connection.
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
