package wails

import (
	"log/slog"
	"sync"

	"github.com/tetiva-app/client/internal/adapters/wails/dto"
	"github.com/tetiva-app/client/internal/domain/usecase/auth"
)

// SignInEventSink implements auth.SignInSink by emitting Wails events. Separate
// from AuthFlowEventSink so FX cannot cross the two sinks over.
type SignInEventSink struct {
	mu   sync.RWMutex
	emit func(name string, data any)
}

func NewSignInEventSink() *SignInEventSink { return &SignInEventSink{} }

func (s *SignInEventSink) SetEmit(fn func(name string, data any)) {
	s.mu.Lock()
	s.emit = fn
	s.mu.Unlock()
}

func (s *SignInEventSink) emitter() func(string, any) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.emit
}

// OnSignInState emits sync:signin:<flowID>. The deadline travels because the
// server extends the request while the user confirms their email.
func (s *SignInEventSink) OnSignInState(flowID string, st auth.SignInStatus) {
	emit := s.emitter()
	if emit == nil {
		return
	}

	d := dto.BrowserSignInEvent{
		State:                    string(st.State),
		EmailVerificationPending: st.EmailVerificationPending,
		ExpiresAt:                formatSessionTime(st.Info.ExpiresAt),
	}
	if st.Err != nil {
		// The panel gets the mapped copy; the raw cause stays here, where a
		// commit failure is the only way to learn what went wrong.
		slog.Warn("sync: browser sign-in ended with an error", "err", st.Err)
		d.Error = signInMessage(st.Err)
	}
	emit("sync:signin:"+flowID, d)
}
