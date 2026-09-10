package app

import (
	"context"
	"errors"
	"log/slog"

	"go.uber.org/fx"

	"github.com/tetiva-app/client/internal/domain/usecase/auth"
	"github.com/tetiva-app/client/internal/domain/usecase/request"
)

// RegisterCleanupHook hard-deletes Replay drafts the frontend could not remove on tab close
// (a crash), and sweeps token rows whose owner went away while the app was down.
func RegisterCleanupHook(lc fx.Lifecycle, uc request.Usecase, tokens request.TokenCleaner) {
	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			n, err := uc.CleanupDrafts(ctx)
			if err != nil {
				slog.Warn("cleanup drafts failed", "err", err)
			} else if n > 0 {
				slog.Info("cleaned leftover drafts", "count", n)
			}

			swept, err := tokens.DeleteOrphans(ctx)
			if err != nil {
				slog.Warn("sweep orphan auth tokens failed", "err", err)
			} else if swept > 0 {
				slog.Info("swept orphan auth tokens", "count", swept)
			}

			return nil // never block app start
		},
	})
}

// RegisterFlowShutdownHook stops loopback listeners and pollers of browser flows still running at quit.
func RegisterFlowShutdownHook(lc fx.Lifecycle, flows auth.FlowManager, signIn auth.SignInManager) {
	lc.Append(fx.Hook{
		OnStop: func(ctx context.Context) error {
			err := errors.Join(flows.Shutdown(ctx), signIn.Shutdown(ctx))
			if err != nil {
				// main.go runs fx with a NopLogger, so the survivors are logged here.
				slog.Warn("auth: flow manager shutdown", "err", err)
			}

			return err
		},
	})
}
