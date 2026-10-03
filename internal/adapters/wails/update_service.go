package wails

import (
	"context"
	"os"
	"sync"
	"time"

	"github.com/tetiva-app/client/internal/adapters/wails/dto"
	"github.com/tetiva-app/client/internal/domain/usecase/appupdate"
	ws "github.com/tetiva-app/client/internal/domain/usecase/websocket"
	"github.com/tetiva-app/client/internal/infrastructure/repository/sqlite"
)

// A hung quit would keep the data-dir lock that the new version waits for.
const quitFallback = 10 * time.Second

type UpdateService struct {
	uc   appupdate.Usecase
	wsUC ws.Usecase
	sink *UpdateEventSink
	dir  string
	exit func(code int)

	mu   sync.Mutex
	quit func()
}

func NewUpdateService(uc appupdate.Usecase, wsUC ws.Usecase, sink *UpdateEventSink, dir sqlite.DataDir) *UpdateService {
	return &UpdateService{uc: uc, wsUC: wsUC, sink: sink, dir: string(dir), exit: os.Exit}
}

func (s *UpdateService) Status() Result[dto.UpdateStateDTO] {
	return OK(dto.UpdateStateToDTO(s.uc.Status()))
}

func (s *UpdateService) Check() Result[dto.UpdateStateDTO] {
	return OK(dto.UpdateStateToDTO(s.uc.Check(context.Background())))
}

func (s *UpdateService) Download() Result[Empty] {
	go s.uc.Download(context.Background())
	return OK(Empty{})
}

func (s *UpdateService) Cancel() Result[Empty] {
	s.uc.Cancel()
	return OK(Empty{})
}

func (s *UpdateService) Apply(restore dto.RestoreTabsDTO) Result[Empty] {
	r := dto.RestoreTabsToEntity(restore)
	r.SavedAt = time.Now()
	if err := s.uc.Apply(context.Background(), r); err != nil {
		return Err[Empty](err)
	}
	s.mu.Lock()
	quit := s.quit
	s.mu.Unlock()
	go func() {
		time.AfterFunc(quitFallback, func() { s.exit(0) })
		quit()
	}()
	return OK(Empty{})
}

func (s *UpdateService) TakeRestore() Result[*dto.RestoreTabsDTO] {
	r, ok := s.uc.TakeRestore()
	if !ok {
		return OK[*dto.RestoreTabsDTO](nil)
	}
	d := dto.RestoreTabsToDTO(r)
	return OK(&d)
}

func (s *UpdateService) OpenConnections() Result[int] {
	return OK(s.wsUC.Count())
}

//wails:ignore
func (s *UpdateService) SetEventEmitter(fn func(name string, data any)) {
	s.sink.SetEmit(fn)
}

// SetQuit is the last wiring step in main.go, so auto-apply starts from here.
//
//wails:ignore
func (s *UpdateService) SetQuit(fn func()) {
	s.mu.Lock()
	s.quit = fn
	s.mu.Unlock()
	s.startAutoApply()
}
