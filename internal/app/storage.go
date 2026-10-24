package app

import (
	"database/sql"
	"io/fs"

	"go.uber.org/fx"

	"github.com/tetiva-app/client/internal/infrastructure/repository/sqlite"
	"github.com/tetiva-app/client/pkg/migrate"
)

func NewStorages(migrationsFS fs.FS) fx.Option {
	return fx.Module("storage",
		fx.Provide(newDB),
		fx.Invoke(func(db *sql.DB) error {
			return migrate.Run(db, migrationsFS, ".")
		}),
	)
}

// Appended at construction: fx stops in reverse, so every DB user stops before Close.
func newDB(lc fx.Lifecycle, dir sqlite.DataDir) (*sql.DB, error) {
	db, err := sqlite.NewDB(dir)
	if err != nil {
		return nil, err
	}
	lc.Append(fx.StopHook(db.Close))

	return db, nil
}
