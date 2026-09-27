package app

import (
	"io/fs"

	"go.uber.org/fx"

	"github.com/tetiva-app/client/internal/app/deeplink"
	"github.com/tetiva-app/client/internal/infrastructure/repository/sqlite"
)

// NewApp takes the link store and the profile directory from main.go: both exist before
// fx, because the instance lock and its IPC listener start first.
func NewApp(migrationsFS fs.FS, links *deeplink.Store, dir sqlite.DataDir) fx.Option {
	return fx.Options(
		fx.Supply(links, dir),
		NewStorages(migrationsFS),
		SyncModule(),
		NewUsecases(),
		MCPModule(),
		WebSocketModule(),
		PublicationModule(),
	)
}
