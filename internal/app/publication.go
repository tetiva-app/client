package app

import (
	"context"

	"go.uber.org/fx"

	wailsadapter "github.com/tetiva-app/client/internal/adapters/wails"
	"github.com/tetiva-app/client/internal/domain/usecase/collection"
	"github.com/tetiva-app/client/internal/domain/usecase/workspace"
	"github.com/tetiva-app/client/internal/infrastructure/repository/sqlite"
	syncsvc "github.com/tetiva-app/client/internal/infrastructure/sync"
)

func PublicationModule() fx.Option {
	return fx.Module("publication",
		fx.Provide(func(s *wailsadapter.SyncService) syncsvc.ClientProvider { return s }),
		fx.Provide(syncsvc.NewPublicationRemote),
		fx.Provide(sqlite.NewPublicationRepo),
		fx.Provide(func(r *sqlite.PublicationRepo) collection.PublicationMarker { return r }),
		fx.Provide(func(r *sqlite.PublicationRepo) workspace.PublicationMarker { return r }),
		fx.Provide(func(e *syncsvc.SyncEngine) wailsadapter.CollectionUploader { return e }),
		fx.Provide(wailsadapter.NewPublicationService),
		fx.Invoke(func(lc fx.Lifecycle, svc *wailsadapter.PublicationService, engine *syncsvc.SyncEngine) {
			engine.OnConnected(func() { svc.ProcessPending(context.Background()) })
			watch, stop := context.WithCancel(context.Background())
			lc.Append(fx.Hook{
				OnStart: func(context.Context) error {
					go svc.ProcessPending(context.Background())
					go svc.WatchDeletes(watch)
					return nil
				},
				OnStop: func(context.Context) error {
					stop()
					return nil
				},
			})
		}),
	)
}
