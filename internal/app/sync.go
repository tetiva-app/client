package app

import (
	"context"
	"log/slog"

	"go.uber.org/fx"

	wailsadapter "github.com/tetiva-app/client/internal/adapters/wails"
	"github.com/tetiva-app/client/internal/infrastructure/repository/sqlite"
	syncsvc "github.com/tetiva-app/client/internal/infrastructure/sync"
)

// SyncModule provides sync infrastructure: repos, auth, engine, Wails service.
func SyncModule() fx.Option {
	return fx.Module("sync",
		fx.Provide(sqlite.NewSyncConfigRepo),
		fx.Provide(sqlite.NewSyncQueueRepo),
		fx.Provide(syncsvc.NewSyncAuthManager),
		// SyncEngine — uses inner (non-decorated) repos
		fx.Provide(fx.Annotate(
			syncsvc.NewSyncEngine,
			fx.ParamTags(``, ``, ``, ``, `name:"innerCollectionRepo"`, `name:"innerRequestRepo"`, `name:"innerEnvironmentRepo"`, `name:"innerVariableRepo"`),
		)),
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
