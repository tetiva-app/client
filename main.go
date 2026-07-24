package main

import (
	"context"
	"embed"
	"log"
	"log/slog"
	"os"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
	"go.uber.org/fx"

	wailsadapter "github.com/tetiva-app/client/internal/adapters/wails"
	appmodule "github.com/tetiva-app/client/internal/app"
	"github.com/tetiva-app/client/internal/constants"
)

//go:embed all:frontend/dist
var assets embed.FS

//go:embed migrations/*.sql
var migrationsFS embed.FS

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	var collectionService *wailsadapter.CollectionService
	var requestService *wailsadapter.RequestService
	var environmentService *wailsadapter.EnvironmentService
	var portabilityService *wailsadapter.PortabilityService
	var workspaceService *wailsadapter.WorkspaceService
	var windowService *wailsadapter.WindowService
	var syncService *wailsadapter.SyncService
	var websocketService *wailsadapter.WebSocketService
	var searchService *wailsadapter.SearchService
	var cookieService *wailsadapter.CookieService
	var historyService *wailsadapter.HistoryService
	var settingsService *wailsadapter.SettingsService

	fxApp := fx.New(
		appmodule.NewApp(migrationsFS),
		fx.Populate(&collectionService, &requestService, &environmentService, &portabilityService, &workspaceService, &windowService, &syncService, &websocketService, &searchService, &cookieService, &historyService, &settingsService),
		fx.NopLogger,
	)

	if err := fxApp.Start(context.Background()); err != nil {
		log.Fatalf("fx start: %v", err)
	}
	defer func() { _ = fxApp.Stop(context.Background()) }()

	slog.Info("dependencies initialized, starting wails app")

	wailsApp := application.New(application.Options{
		Name: constants.AppName,
		Assets: application.AssetOptions{
			Handler: application.AssetFileServerFS(assets),
		},
		Mac: application.MacOptions{
			ApplicationShouldTerminateAfterLastWindowClosed: true,
		},
		Services: []application.Service{
			application.NewService(collectionService),
			application.NewService(requestService),
			application.NewService(environmentService),
			application.NewService(portabilityService),
			application.NewService(workspaceService),
			application.NewService(windowService),
			application.NewService(syncService),
			application.NewService(websocketService),
			application.NewService(searchService),
			application.NewService(cookieService),
			application.NewService(historyService),
			application.NewService(settingsService),
		},
	})

	windowService.SetApp(wailsApp)
	portabilityService.SetApp(wailsApp)

	syncService.SetEventEmitter(func(name string, data any) {
		wailsApp.Event.Emit(name, data)
	})

	websocketService.SetEventEmitter(func(name string, data any) {
		wailsApp.Event.Emit(name, data)
	})

	// The default File menu binds Cmd+W to close-window, which quits the app
	// (ShouldTerminateAfterLastWindowClosed); rebind it to close the active tab instead.
	appMenu := application.NewMenu()
	appMenu.AddRole(application.AppMenu)
	fileMenu := appMenu.AddSubmenu("File")
	fileMenu.Add("Close Tab").SetAccelerator("CmdOrCtrl+W").OnClick(func(_ *application.Context) {
		win := wailsApp.Window.Current()
		if win != nil && win.Name() != "main" {
			win.Close()
			return
		}
		wailsApp.Event.Emit("app:close-tab", nil)
	})
	appMenu.AddRole(application.EditMenu)
	appMenu.AddRole(application.ViewMenu)
	appMenu.AddRole(application.WindowMenu)
	wailsApp.Menu.SetApplicationMenu(appMenu)

	mainWindow := wailsApp.Window.NewWithOptions(application.WebviewWindowOptions{
		Name:             "main",
		Title:            constants.AppTitle(),
		Width:            1280,
		Height:           800,
		BackgroundColour: application.NewRGB(26, 26, 46),
		URL:              "/",
	})

	mainWindow.RegisterHook(events.Common.WindowClosing, func(e *application.WindowEvent) {
		windowService.CloseAllChildWindows()
	})

	if err := wailsApp.Run(); err != nil {
		log.Fatalf("wails run: %v", err)
	}
}
