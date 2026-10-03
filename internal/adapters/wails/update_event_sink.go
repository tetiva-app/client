package wails

import (
	"sync"

	"github.com/tetiva-app/client/internal/adapters/wails/dto"
	"github.com/tetiva-app/client/internal/domain/entities"
)

const UpdateStateEvent = "update:state"

type UpdateEventSink struct {
	mu   sync.RWMutex
	emit func(name string, data any)
}

func NewUpdateEventSink() *UpdateEventSink { return &UpdateEventSink{} }

func (s *UpdateEventSink) SetEmit(fn func(name string, data any)) {
	s.mu.Lock()
	s.emit = fn
	s.mu.Unlock()
}

func (s *UpdateEventSink) Publish(st entities.UpdateState) {
	s.mu.RLock()
	emit := s.emit
	s.mu.RUnlock()
	if emit == nil {
		return
	}
	emit(UpdateStateEvent, dto.UpdateStateToDTO(st))
}
