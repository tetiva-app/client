package wails

import (
	"log/slog"
	"sync/atomic"

	"github.com/wailsapp/wails/v3/pkg/application"

	"github.com/tetiva-app/client/internal/adapters/wails/dto"
	"github.com/tetiva-app/client/internal/app/deeplink"
)

const DeepLinkReceivedEvent = "deeplink:received"

// DeepLinkService hands queued tetiva:// import links to the main window.
type DeepLinkService struct {
	store  *deeplink.Store
	window atomic.Pointer[application.WebviewWindow]
}

func NewDeepLinkService(store *deeplink.Store) *DeepLinkService {
	return &DeepLinkService{store: store}
}

func (s *DeepLinkService) TakePending() Result[[]dto.DeepLink] {
	links := s.store.Take()
	out := make([]dto.DeepLink, 0, len(links))
	for _, l := range links {
		out = append(out, dto.DeepLink{Slug: l.Slug, Token: l.Token})
	}
	return OK(out)
}

// Attach runs on ApplicationStarted: before Run, InvokeAsync has no main-thread loop.
//
//wails:ignore
func (s *DeepLinkService) Attach(w *application.WebviewWindow, emit func(name string, data any)) {
	s.window.Store(w)
	s.store.SetHooks(func() { emit(DeepLinkReceivedEvent, nil) }, s.raise)
}

// UnMinimise rather than Restore: Restore would also un-maximise a maximised window.
func (s *DeepLinkService) raise() {
	w := s.window.Load()
	if w == nil {
		return
	}
	application.InvokeAsync(func() {
		// Wails' own handler treats a panic here as fatal.
		defer func() {
			if r := recover(); r != nil {
				slog.Warn("deeplink: could not raise the main window", "panic", r)
			}
		}()
		w.UnMinimise()
		w.Show()
		w.Focus()
	})
}
