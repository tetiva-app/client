package main

import (
	"context"
	"embed"
	"errors"
	"fmt"
	"log"
	"log/slog"
	"os"
	"runtime"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
	"go.uber.org/fx"

	wailsadapter "github.com/tetiva-app/client/internal/adapters/wails"
	appmodule "github.com/tetiva-app/client/internal/app"
	"github.com/tetiva-app/client/internal/app/deeplink"
	"github.com/tetiva-app/client/internal/constants"
	"github.com/tetiva-app/client/internal/infrastructure/instance"
	"github.com/tetiva-app/client/internal/infrastructure/repository/sqlite"
	"github.com/tetiva-app/client/migrations"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	dataDir, err := sqlite.ResolveDataDir()
	if err != nil {
		log.Fatalf("data dir: %v", err)
	}
	links := deeplink.NewStore()
	inst, err := instance.Acquire(dataDir, os.Args[1:], func(args []string) {
		if !links.Deliver(args) {
			links.Activate()
		}
	})
	switch {
	case errors.Is(err, instance.ErrForwarded):
		msg := fmt.Sprintf("Tetiva is already running with %s; the launch was passed to it (set TETIVA_DATA_DIR to run a separate copy)", dataDir)
		fmt.Fprintln(os.Stderr, msg)
		slog.Info(msg)
		os.Exit(0)
	case errors.Is(err, instance.ErrNotResponding):
		msg := fmt.Sprintf("Tetiva holds %s but does not respond; close it or set TETIVA_DATA_DIR", dataDir)
		fmt.Fprintln(os.Stderr, msg)
		slog.Error(msg)
		os.Exit(2)
	case err != nil:
		log.Fatalf("instance: %v", err)
	}
	defer func() { _ = inst.Close() }()
	links.Deliver(os.Args[1:])

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
	var authService *wailsadapter.AuthService
	var exampleService *wailsadapter.ExampleService
	var publicationService *wailsadapter.PublicationService
	var deepLinkService *wailsadapter.DeepLinkService

	fxApp := fx.New(
		appmodule.NewApp(migrations.FS, links, sqlite.DataDir(dataDir)),
		fx.Populate(&collectionService, &requestService, &environmentService, &portabilityService, &workspaceService, &windowService, &syncService, &websocketService, &searchService, &cookieService, &historyService, &settingsService, &authService, &exampleService, &publicationService, &deepLinkService),
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
			application.NewService(authService),
			application.NewService(exampleService),
			application.NewService(publicationService),
			application.NewService(deepLinkService),
		},
	})

	// macOS exits inside Run and never reaches the deferred Close.
	wailsApp.OnShutdown(func() { _ = inst.StopServing() })

	// Elsewhere Wails raises this event from the argv links.Deliver already took.
	if runtime.GOOS == "darwin" {
		wailsApp.Event.OnApplicationEvent(events.Common.ApplicationLaunchedWithUrl, func(e *application.ApplicationEvent) {
			links.Deliver([]string{e.Context().URL()})
		})
	}

	windowService.SetApp(wailsApp)
	portabilityService.SetApp(wailsApp)

	windowService.SetEventEmitter(func(name string, data any) {
		wailsApp.Event.Emit(name, data)
	})

	syncService.SetEventEmitter(func(name string, data any) {
		wailsApp.Event.Emit(name, data)
	})

	websocketService.SetEventEmitter(func(name string, data any) {
		wailsApp.Event.Emit(name, data)
	})

	authService.SetEventEmitter(func(name string, data any) {
		wailsApp.Event.Emit(name, data)
	})

	// Linux draws the menu as a GTK bar with unreadable labels; the frontend handles Ctrl+W.
	if runtime.GOOS != "linux" {
		// The default Cmd+W closes the window and quits the app; rebind it to close the tab.
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
	}

	mainWindow := wailsApp.Window.NewWithOptions(application.WebviewWindowOptions{
		Name:             "main",
		Title:            constants.AppTitle(),
		Width:            1280,
		Height:           800,
		BackgroundColour: application.NewRGB(26, 26, 46),
		URL:              "/",
	})

	// The sign-in loopback fires from a browser tab, so the window has to come
	// back on its own.
	syncService.SetFocusMain(func() { mainWindow.Focus() })

	mainWindow.RegisterHook(events.Common.WindowClosing, func(e *application.WindowEvent) {
		windowService.CloseAllChildWindows()
	})

	wailsApp.Event.OnApplicationEvent(events.Common.ApplicationStarted, func(*application.ApplicationEvent) {
		deepLinkService.Attach(mainWindow, func(name string, data any) {
			wailsApp.Event.Emit(name, data)
		})
	})

	if err := wailsApp.Run(); err != nil {
		log.Fatalf("wails run: %v", err)
	}
}
