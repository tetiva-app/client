package wails

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/google/uuid"
	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"

	"github.com/tetiva-app/client/internal/constants"
	"github.com/tetiva-app/client/internal/domain"
)

// WindowInfo tracks an open child window.
type WindowInfo struct {
	Window   *application.WebviewWindow
	Type     string // "detached-request" or "schema-viewer"
	EntityID string // requestID or schemaID
}

// SchemaData holds definition content for schema viewer windows.
type SchemaData struct {
	Definition string `json:"definition"`
	Source     string `json:"source"`
	Language   string `json:"language"`   // "protobuf" | "graphql"
	SchemaJSON string `json:"schemaJSON"` // serialized GraphQLSchema for interactive browse (GraphQL only)
}

// WindowPrefs stores size/position for a window type.
type WindowPrefs struct {
	Width  int `json:"width"`
	Height int `json:"height"`
	X      int `json:"x"`
	Y      int `json:"y"`
}

// WindowService manages detachable child windows.
type WindowService struct {
	app *application.App
	mu  sync.RWMutex

	windows          map[string]*WindowInfo  // windowName → info
	detachedRequests map[string]string       // requestID → windowName
	schemas          map[string]*SchemaData  // schemaID → content
	prefs            map[string]*WindowPrefs // windowType → size/position
}

// NewWindowService creates a new WindowService instance.
func NewWindowService() *WindowService {
	ws := &WindowService{
		windows:          make(map[string]*WindowInfo),
		detachedRequests: make(map[string]string),
		schemas:          make(map[string]*SchemaData),
		prefs:            make(map[string]*WindowPrefs),
	}
	ws.loadPrefs()
	return ws
}

// SetApp sets the Wails application instance (called after app creation in main.go).
func (ws *WindowService) SetApp(app *application.App) {
	ws.app = app
}

func prefsFilePath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, constants.AppDir, "window-prefs.json")
}

func (ws *WindowService) loadPrefs() {
	data, err := os.ReadFile(prefsFilePath())
	if err != nil {
		return // File doesn't exist yet — use defaults
	}

	var prefs map[string]*WindowPrefs
	if err := json.Unmarshal(data, &prefs); err != nil {
		return
	}

	ws.prefs = prefs
}

func (ws *WindowService) savePrefs() {
	ws.mu.RLock()
	data, err := json.MarshalIndent(ws.prefs, "", "  ")
	ws.mu.RUnlock()

	if err != nil {
		return
	}

	dir := filepath.Dir(prefsFilePath())
	_ = os.MkdirAll(dir, 0o755)
	_ = os.WriteFile(prefsFilePath(), data, 0o644)
}

var defaultPrefs = map[string]*WindowPrefs{
	"detached-request": {Width: 1000, Height: 700},
	"schema-viewer":    {Width: 700, Height: 500},
}

func (ws *WindowService) getPrefsOrDefaults(windowType string) *WindowPrefs {
	ws.mu.RLock()
	defer ws.mu.RUnlock()

	if p, ok := ws.prefs[windowType]; ok {
		return p
	}
	if p, ok := defaultPrefs[windowType]; ok {
		return &WindowPrefs{Width: p.Width, Height: p.Height}
	}
	return &WindowPrefs{Width: 800, Height: 600}
}

// buildWindowOptions applies saved prefs; without a saved position Wails centers the window.
func buildWindowOptions(name, title, url string, prefs *WindowPrefs) application.WebviewWindowOptions {
	opts := application.WebviewWindowOptions{
		Name:             name,
		Title:            title,
		Width:            prefs.Width,
		Height:           prefs.Height,
		URL:              url,
		BackgroundColour: application.NewRGB(26, 26, 46),
	}
	if prefs.X != 0 || prefs.Y != 0 {
		opts.InitialPosition = application.WindowXY
		opts.X = prefs.X
		opts.Y = prefs.Y
	}
	return opts
}

func (ws *WindowService) saveWindowPrefs(window *application.WebviewWindow, windowType string) {
	x, y := window.Position()
	ws.mu.Lock()
	ws.prefs[windowType] = &WindowPrefs{
		Width:  window.Width(),
		Height: window.Height(),
		X:      x,
		Y:      y,
	}
	ws.mu.Unlock()
	ws.savePrefs()
}

// DetachRequest opens a request in a separate native window.
func (ws *WindowService) DetachRequest(requestID, protocol, title string) Result[Empty] {
	const funcName = "WindowService.DetachRequest"

	ws.mu.Lock()
	if _, exists := ws.detachedRequests[requestID]; exists {
		ws.mu.Unlock()
		return ws.FocusDetachedWindow(requestID)
	}
	ws.mu.Unlock()

	if ws.app == nil {
		return Err[Empty](fmt.Errorf("%s: app not initialized", funcName))
	}

	prefs := ws.getPrefsOrDefaults("detached-request")
	windowName := fmt.Sprintf("detached-request-%s", requestID)
	url := fmt.Sprintf("/?mode=detached-request&requestId=%s", requestID)

	window := ws.app.Window.NewWithOptions(buildWindowOptions(windowName, title, url, prefs))

	ws.mu.Lock()
	ws.windows[windowName] = &WindowInfo{
		Window:   window,
		Type:     "detached-request",
		EntityID: requestID,
	}
	ws.detachedRequests[requestID] = windowName
	ws.mu.Unlock()

	window.RegisterHook(events.Common.WindowClosing, func(e *application.WindowEvent) {
		ws.saveWindowPrefs(window, "detached-request")

		ws.mu.Lock()
		delete(ws.windows, windowName)
		delete(ws.detachedRequests, requestID)
		ws.mu.Unlock()
	})

	return OK(Empty{})
}

// OpenSchemaViewer opens a proto schema in a separate read-only window.
func (ws *WindowService) OpenSchemaViewer(definition, source, title, language, schemaJSON string) Result[Empty] {
	const funcName = "WindowService.OpenSchemaViewer"

	if ws.app == nil {
		return Err[Empty](fmt.Errorf("%s: app not initialized", funcName))
	}

	schemaID := uuid.New().String()
	prefs := ws.getPrefsOrDefaults("schema-viewer")
	windowName := fmt.Sprintf("schema-viewer-%s", schemaID)
	url := fmt.Sprintf("/?mode=schema-viewer&schemaId=%s", schemaID)

	ws.mu.Lock()
	if language == "" {
		language = "protobuf"
	}
	ws.schemas[schemaID] = &SchemaData{
		Definition: definition,
		Source:     source,
		Language:   language,
		SchemaJSON: schemaJSON,
	}
	ws.mu.Unlock()

	window := ws.app.Window.NewWithOptions(buildWindowOptions(windowName, title, url, prefs))

	ws.mu.Lock()
	ws.windows[windowName] = &WindowInfo{
		Window:   window,
		Type:     "schema-viewer",
		EntityID: schemaID,
	}
	ws.mu.Unlock()

	window.RegisterHook(events.Common.WindowClosing, func(e *application.WindowEvent) {
		ws.saveWindowPrefs(window, "schema-viewer")

		ws.mu.Lock()
		delete(ws.windows, windowName)
		delete(ws.schemas, schemaID)
		ws.mu.Unlock()
	})

	return OK(Empty{})
}

// IsDetached checks if a request is currently open in a detached window.
func (ws *WindowService) IsDetached(requestID string) Result[bool] {
	ws.mu.RLock()
	defer ws.mu.RUnlock()

	_, exists := ws.detachedRequests[requestID]
	return OK(exists)
}

// FocusDetachedWindow brings the detached window for a request to the front.
func (ws *WindowService) FocusDetachedWindow(requestID string) Result[Empty] {
	ws.mu.RLock()
	windowName, exists := ws.detachedRequests[requestID]
	if !exists {
		ws.mu.RUnlock()
		return OK(Empty{})
	}
	info := ws.windows[windowName]
	ws.mu.RUnlock()

	if info != nil && info.Window != nil {
		info.Window.Focus()
	}
	return OK(Empty{})
}

// GetSchemaContent returns the proto schema data for a schema viewer window.
func (ws *WindowService) GetSchemaContent(schemaID string) Result[SchemaData] {
	ws.mu.RLock()
	defer ws.mu.RUnlock()

	data, exists := ws.schemas[schemaID]
	if !exists {
		return Err[SchemaData](&domain.NotFoundError{
			Entity: "schema",
			ID:     schemaID,
		})
	}
	return OK(*data)
}

// CloseAllChildWindows closes all detached windows (called on main window close).
func (ws *WindowService) CloseAllChildWindows() {
	ws.mu.RLock()
	names := make([]string, 0, len(ws.windows))
	for name := range ws.windows {
		names = append(names, name)
	}
	ws.mu.RUnlock()

	for _, name := range names {
		ws.mu.RLock()
		info, ok := ws.windows[name]
		ws.mu.RUnlock()
		if ok && info.Window != nil {
			info.Window.Close()
		}
	}
}
