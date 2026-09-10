package wails

import (
	"sync"

	"github.com/tetiva-app/client/internal/adapters/wails/dto"
	"github.com/tetiva-app/client/internal/domain/usecase/auth"
)

// AuthFlowEventSink implements auth.FlowSink by emitting Wails events. The emit
// func is injected post-construction in main.go (same pattern as websocket).
type AuthFlowEventSink struct {
	mu   sync.RWMutex
	emit func(name string, data any)
}

func NewAuthFlowEventSink() *AuthFlowEventSink { return &AuthFlowEventSink{} }

func (s *AuthFlowEventSink) SetEmit(fn func(name string, data any)) {
	s.mu.Lock()
	s.emit = fn
	s.mu.Unlock()
}

func (s *AuthFlowEventSink) emitter() func(string, any) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.emit
}

// Emits a state transition on auth:flow:<flowID>. Info stays empty: the frontend
// already holds it from the start call, or reads it from FlowStatus after a reload.
func (s *AuthFlowEventSink) OnFlowState(flowID string, st auth.FlowState, err error) {
	emit := s.emitter()
	if emit == nil {
		return
	}
	d := dto.FlowStatusDTO{State: string(st)}
	if err != nil {
		d.Error = err.Error()
	}
	emit("auth:flow:"+flowID, d)
}
