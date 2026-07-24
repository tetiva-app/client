package app

import (
	"database/sql"
	"io/fs"

	"go.uber.org/fx"

	"github.com/tetiva-app/client/internal/infrastructure/repository/sqlite"
	"github.com/tetiva-app/client/pkg/migrate"
)

// NewStorages provides SQLite connection and runs migrations on startup.
func NewStorages(migrationsFS fs.FS) fx.Option {
	return fx.Module("storage",
		fx.Provide(sqlite.NewDB),
		fx.Invoke(func(db *sql.DB) error {
			return migrate.Run(db, migrationsFS, "migrations")
		}),
	)
}
