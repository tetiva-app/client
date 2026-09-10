package app

import (
	"io/fs"

	"go.uber.org/fx"
)

func NewApp(migrationsFS fs.FS) fx.Option {
	return fx.Options(
		NewStorages(migrationsFS),
		SyncModule(),
		NewUsecases(),
		MCPModule(),
		WebSocketModule(),
	)
}
