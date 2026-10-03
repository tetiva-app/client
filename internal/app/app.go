package app

import (
	"io/fs"

	"go.uber.org/fx"

	"github.com/tetiva-app/client/internal/app/deeplink"
	"github.com/tetiva-app/client/internal/infrastructure/repository/sqlite"
)

func NewApp(migrationsFS fs.FS, links *deeplink.Store, dir sqlite.DataDir) fx.Option {
	return fx.Options(
		fx.Supply(links, dir),
		NewStorages(migrationsFS),
		SyncModule(),
		NewUsecases(),
		MCPModule(),
		WebSocketModule(),
		PublicationModule(),
		UpdateModule(),
	)
}
