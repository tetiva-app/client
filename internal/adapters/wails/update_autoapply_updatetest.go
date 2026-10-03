//go:build updatetest

package wails

import (
	"context"
	"log/slog"
	"time"

	"github.com/tetiva-app/client/internal/adapters/wails/dto"
	"github.com/tetiva-app/client/internal/domain/entities"
	infraupdate "github.com/tetiva-app/client/internal/infrastructure/appupdate"
)

func (s *UpdateService) startAutoApply() {
	if !infraupdate.AutoApply(s.dir) {
		return
	}
	go func() {
		time.Sleep(3 * time.Second)
		ctx := context.Background()
		st := s.uc.Check(ctx)
		if st.Phase == entities.UpdateAvailable {
			st = s.uc.Download(ctx)
		}
		// The UI's own startup check may have started the download first.
		for deadline := time.Now().Add(3 * time.Minute); st.Phase == entities.UpdateDownloading && time.Now().Before(deadline); {
			time.Sleep(500 * time.Millisecond)
			st = s.uc.Status()
		}
		slog.Info("appupdate: auto-apply", "phase", st.Phase, "version", st.Version, "reason", st.Reason)
		if st.Phase != entities.UpdateReady {
			return
		}
		if res := s.Apply(dto.RestoreTabsDTO{}); res.Error != nil {
			slog.Warn("appupdate: auto-apply failed", "err", res.Error.Message)
		}
	}()
}
