package app

import (
	"testing"

	"go.uber.org/fx"

	wailsadapter "github.com/tetiva-app/client/internal/adapters/wails"
	"github.com/tetiva-app/client/internal/app/deeplink"
	"github.com/tetiva-app/client/internal/infrastructure/repository/sqlite"
	"github.com/tetiva-app/client/migrations"
)

func TestDIGraphValid(t *testing.T) {
	// Same FS main.go passes; it is not read during validation.
	app := fx.Options(
		NewApp(migrations.FS, deeplink.NewStore(), sqlite.DataDir(t.TempDir())),
		fx.Invoke(func(*wailsadapter.DeepLinkService) {}),
		fx.Invoke(func(*wailsadapter.UpdateService) {}),
	)
	if err := fx.ValidateApp(app, fx.NopLogger); err != nil {
		t.Fatalf("DI graph invalid: %v", err)
	}
}
