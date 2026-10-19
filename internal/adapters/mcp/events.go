package mcp

import "github.com/google/uuid"

func (s *Server) SetEventEmitter(fn func(name string, data any)) {
	s.emitMu.Lock()
	s.emitFn = fn
	s.emitMu.Unlock()
}

func (s *Server) emitter() func(string, any) {
	s.emitMu.RLock()
	defer s.emitMu.RUnlock()
	return s.emitFn
}

func (s *Server) emit(name string, data any) {
	if fn := s.emitter(); fn != nil {
		fn(name, data)
	}
}

func (s *Server) emitRequestsChanged(collectionIDs ...uuid.UUID) {
	ids := make([]string, 0, len(collectionIDs))
	for _, id := range collectionIDs {
		ids = append(ids, id.String())
	}
	s.emit("collection:updated", map[string]any{"collectionIds": ids})
}

func (s *Server) emitCollectionsChanged(workspaceID uuid.UUID) {
	s.emit("collection:updated", map[string]any{"workspaceId": workspaceID.String()})
}

func (s *Server) emitEnvChanged(workspaceID uuid.UUID) {
	s.emit("env:changed", map[string]any{"workspaceId": workspaceID.String()})
}
