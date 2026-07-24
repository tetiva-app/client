package app

import (
	"context"
	"log/slog"

	"go.uber.org/fx"

	"github.com/tetiva-app/client/internal/domain/usecase/request"
)

// RegisterCleanupHook wires an OnStart hook that hard-deletes leftover Replay
// drafts — normally removed by the frontend on tab close, but a crash can leave them.
func RegisterCleanupHook(lc fx.Lifecycle, uc request.Usecase) {
	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			n, err := uc.CleanupDrafts(ctx)
			if err != nil {
				slog.Warn("cleanup drafts failed", "err", err)
				return nil // do not block app start
			}
			if n > 0 {
				slog.Info("cleaned leftover drafts", "count", n)
			}
			return nil
		},
	})
}
