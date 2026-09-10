package app

import (
	"context"
	"log/slog"
	"time"

	"go.uber.org/fx"

	wailsadapter "github.com/tetiva-app/client/internal/adapters/wails"
	"github.com/tetiva-app/client/internal/domain/usecase/auth"
	"github.com/tetiva-app/client/internal/infrastructure/repository/sqlite"
	syncsvc "github.com/tetiva-app/client/internal/infrastructure/sync"
)

func SyncModule() fx.Option {
	return fx.Module("sync",
		fx.Provide(sqlite.NewSyncConfigRepo),
		fx.Provide(sqlite.NewSyncQueueRepo),
		fx.Provide(syncsvc.NewSyncAuthManager),
		// SyncEngine — uses inner (non-decorated) repos
		fx.Provide(fx.Annotate(
			syncsvc.NewSyncEngine,
			fx.ParamTags(``, ``, ``, ``, `name:"innerCollectionRepo"`, `name:"innerRequestRepo"`, `name:"innerEnvironmentRepo"`, `name:"innerVariableRepo"`, ``),
		)),
		fx.Provide(wailsadapter.NewSignInEventSink),
		fx.Provide(func(s *wailsadapter.SignInEventSink) auth.SignInSink { return s }),
		fx.Provide(func(sink auth.SignInSink) auth.SignInManager {
			// A longer join than the OAuth manager's 5 s: the commit is a 1.5 s
			// config write plus a 2 s keychain write.
			return auth.NewSignInManager(sink, auth.FlowOptions{JoinCap: 10 * time.Second})
		}),
		fx.Provide(wailsadapter.NewSyncService),
		fx.Invoke(func(lc fx.Lifecycle, svc *wailsadapter.SyncService) {
			lc.Append(fx.Hook{
				OnStart: func(ctx context.Context) error {
					if err := svc.ResumeOnStartup(ctx); err != nil {
						slog.Warn("sync: resume on startup failed", "err", err)
					}
					return nil // non-fatal
				},
			})
		}),
	)
}
