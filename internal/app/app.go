package app

import (
	"io/fs"

	"go.uber.org/fx"
)

// NewApp composes all FX modules for the application.
func NewApp(migrationsFS fs.FS) fx.Option {
	return fx.Options(
		NewStorages(migrationsFS),
		SyncModule(),
		NewUsecases(),
		MCPModule(),
		WebSocketModule(),
	)
}
