package app

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	"go.uber.org/fx"
	"go.uber.org/fx/fxtest"

	"github.com/tetiva-app/client/internal/constants"
	"github.com/tetiva-app/client/internal/infrastructure/repository/sqlite"
	"github.com/tetiva-app/client/migrations"
)

func TestStorageClosesDBAfterOtherStopHooks(t *testing.T) {
	dir := t.TempDir()
	wal := filepath.Join(dir, constants.DBFileName+"-wal")
	var pingAtStop error

	app := fxtest.New(t,
		fx.Supply(sqlite.DataDir(dir)),
		NewStorages(migrations.FS),
		fx.Invoke(func(lc fx.Lifecycle, db *sql.DB) {
			lc.Append(fx.StopHook(func() { pingAtStop = db.Ping() }))
		}),
	)
	app.RequireStart()
	if _, err := os.Stat(wal); err != nil {
		t.Fatalf("WAL file missing while running: %v", err)
	}

	app.RequireStop()

	if pingAtStop != nil {
		t.Fatalf("DB closed before a dependent stop hook ran: %v", pingAtStop)
	}
	if _, err := os.Stat(wal); !os.IsNotExist(err) {
		t.Fatalf("WAL file left after stop (err=%v)", err)
	}
}
