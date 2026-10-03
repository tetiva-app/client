package app

import (
	"context"
	"runtime"

	"go.uber.org/fx"

	wailsadapter "github.com/tetiva-app/client/internal/adapters/wails"
	"github.com/tetiva-app/client/internal/constants"
	"github.com/tetiva-app/client/internal/domain/usecase/appupdate"
	infraupdate "github.com/tetiva-app/client/internal/infrastructure/appupdate"
	"github.com/tetiva-app/client/internal/infrastructure/repository/sqlite"
)

func UpdateModule() fx.Option {
	return fx.Module("update",
		fx.Provide(wailsadapter.NewUpdateEventSink),
		fx.Provide(func(dir sqlite.DataDir, sink *wailsadapter.UpdateEventSink) (appupdate.Usecase, error) {
			store, err := infraupdate.NewFileStore(string(dir))
			if err != nil {
				return nil, err
			}
			return appupdate.NewUsecase(constants.AppVersion,
				appupdate.Platform{OS: runtime.GOOS, Arch: runtime.GOARCH},
				infraupdate.NewSource(string(dir), constants.AppVersion),
				infraupdate.Codec{Keys: infraupdate.ReleaseKeys()},
				infraupdate.NewDownloader(),
				infraupdate.NewInstaller(string(dir), infraupdate.ProductionBuild),
				store, sink), nil
		}),
		fx.Provide(wailsadapter.NewUpdateService),
		fx.Invoke(func(lc fx.Lifecycle, uc appupdate.Usecase) {
			lc.Append(fx.Hook{OnStart: func(ctx context.Context) error {
				uc.Start(ctx)
				return nil
			}})
		}),
	)
}
