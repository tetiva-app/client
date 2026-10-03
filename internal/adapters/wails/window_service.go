package wails

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"

	"github.com/tetiva-app/client/internal/constants"
	"github.com/tetiva-app/client/internal/domain"
)

type WindowInfo struct {
	Window   *application.WebviewWindow
	Type     string // "detached-request" or "schema-viewer"
	EntityID string // requestID or schemaID
}

type SchemaData struct {
	Definition string `json:"definition"`
	Source     string `json:"source"`
	Language   string `json:"language"`   // "protobuf" | "graphql"
	SchemaJSON string `json:"schemaJSON"` // serialized GraphQLSchema for interactive browse (GraphQL only)
}

type WindowPrefs struct {
	Width       int  `json:"width"`
	Height      int  `json:"height"`
	X           int  `json:"x"`
	Y           int  `json:"y"`
	HasPosition bool `json:"hasPosition"`
}

const windowPrefsVersion = 2

type windowPrefsFile struct {
	Version int                     `json:"version"`
	Windows map[string]*WindowPrefs `json:"windows"`
}

// Wails v3 beta changed the macOS coordinate space; positions saved by older builds are garbage there.
var legacyPositionsStale = runtime.GOOS == "darwin"

const defaultCloseGrace = 3 * time.Second

// WindowService manages detachable child windows.
type WindowService struct {
	app  *application.App
	emit func(name string, data any)
	mu   sync.RWMutex

	windows          map[string]*WindowInfo  // windowName → info
	detachedRequests map[string]string       // requestID → windowName
	schemas          map[string]*SchemaData  // schemaID → content
	prefs            map[string]*WindowPrefs // windowType → size/position

	closeGrace   time.Duration
	closeHeld    map[string]*time.Timer // requestID → fallback close while the window saves
	closeAllowed map[string]bool        // requestID → saved, the next close goes through
}

func NewWindowService() *WindowService {
	ws := &WindowService{
		windows:          make(map[string]*WindowInfo),
		detachedRequests: make(map[string]string),
		schemas:          make(map[string]*SchemaData),
		prefs:            make(map[string]*WindowPrefs),
		closeGrace:       defaultCloseGrace,
		closeHeld:        make(map[string]*time.Timer),
		closeAllowed:     make(map[string]bool),
	}
	ws.loadPrefs()
	return ws
}

// Called after app creation in main.go.
func (ws *WindowService) SetApp(app *application.App) {
	ws.app = app
}

func (ws *WindowService) SetEventEmitter(emit func(name string, data any)) {
	ws.mu.Lock()
	ws.emit = emit
	ws.mu.Unlock()
}

func saveAndCloseEvent(requestID string) string {
	return "window:save-and-close:" + requestID
}

func prefsFilePath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, constants.AppDir, "window-prefs.json")
}

func (ws *WindowService) loadPrefs() {
	ws.prefs = readPrefs(prefsFilePath())
}

// Beyond this the prefs file is corrupt, not a display arrangement.
const coordLimit = 32767

func validGeometry(w, h, x, y int) bool {
	if w <= 0 || h <= 0 || w > coordLimit || h > coordLimit {
		return false
	}
	return x >= -coordLimit && x <= coordLimit && y >= -coordLimit && y <= coordLimit
}

// A destroyed window reads back as 0x0; that must not be persisted.
func prefsFromWindow(w, h, x, y int) (*WindowPrefs, bool) {
	if !validGeometry(w, h, x, y) {
		return nil, false
	}
	return &WindowPrefs{Width: w, Height: h, X: x, Y: y, HasPosition: true}, true
}

// Never returns nil: a missing, corrupt or newer file means defaults.
func readPrefs(path string) map[string]*WindowPrefs {
	prefs := make(map[string]*WindowPrefs)

	data, err := os.ReadFile(path)
	if err != nil {
		return prefs
	}

	var file windowPrefsFile
	if err := json.Unmarshal(data, &file); err == nil && file.Version != 0 {
		if file.Version != windowPrefsVersion {
			return prefs
		}
		for name, p := range file.Windows {
			if p == nil || !validGeometry(p.Width, p.Height, p.X, p.Y) {
				continue
			}
			prefs[name] = p
		}
		return prefs
	}

	var legacy map[string]*WindowPrefs
	if err := json.Unmarshal(data, &legacy); err != nil {
		return prefs
	}
	for name, p := range legacy {
		if p == nil || !validGeometry(p.Width, p.Height, p.X, p.Y) {
			continue
		}
		if legacyPositionsStale {
			p.X, p.Y, p.HasPosition = 0, 0, false
		} else {
			p.HasPosition = p.X != 0 || p.Y != 0
		}
		prefs[name] = p
	}
	return prefs
}

func (ws *WindowService) savePrefs() {
	ws.mu.RLock()
	data, err := json.MarshalIndent(windowPrefsFile{Version: windowPrefsVersion, Windows: ws.prefs}, "", "  ")
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

	if p, ok := ws.prefs[windowType]; ok && p != nil {
		return p
	}
	if p, ok := defaultPrefs[windowType]; ok {
		return &WindowPrefs{Width: p.Width, Height: p.Height}
	}
	return &WindowPrefs{Width: 800, Height: 600}
}

// Without a usable saved position Wails centers the window.
func buildWindowOptions(name, title, url string, prefs *WindowPrefs, areas []application.Rect) application.WebviewWindowOptions {
	opts := application.WebviewWindowOptions{
		Name:             name,
		Title:            title,
		Width:            prefs.Width,
		Height:           prefs.Height,
		URL:              url,
		BackgroundColour: application.NewRGB(26, 26, 46),
	}
	if prefs.HasPosition && positionOnScreen(prefs.X, prefs.Y, prefs.Width, areas) {
		opts.InitialPosition = application.WindowXY
		opts.X = prefs.X
		opts.Y = prefs.Y
	}
	return opts
}

// grabMargin is how much of the title bar must stay inside a work area to remain draggable.
const grabMargin = 100

func positionOnScreen(x, y, w int, areas []application.Rect) bool {
	for _, a := range areas {
		if y < a.Y || y >= a.Y+a.Height {
			continue
		}
		if min(x+w, a.X+a.Width)-max(x, a.X) >= min(w, grabMargin) {
			return true
		}
	}
	return false
}

// GetAll releases Wails' lock on return, so a display change can tear this read; the worst case is a misplaced window.
func (ws *WindowService) workAreas() []application.Rect {
	if ws.app == nil || ws.app.Screen == nil {
		return nil
	}
	screens := ws.app.Screen.GetAll()
	areas := make([]application.Rect, 0, len(screens))
	for _, s := range screens {
		if s == nil {
			continue
		}
		areas = append(areas, s.WorkArea)
	}
	return areas
}

func (ws *WindowService) saveWindowPrefs(window *application.WebviewWindow, windowType string) {
	if window == nil {
		return
	}
	x, y := window.Position()
	prefs, ok := prefsFromWindow(window.Width(), window.Height(), x, y)
	if !ok {
		return
	}
	ws.mu.Lock()
	ws.prefs[windowType] = prefs
	ws.mu.Unlock()
	ws.savePrefs()
}

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

	window := ws.app.Window.NewWithOptions(buildWindowOptions(windowName, title, url, prefs, ws.workAreas()))

	ws.mu.Lock()
	ws.windows[windowName] = &WindowInfo{
		Window:   window,
		Type:     "detached-request",
		EntityID: requestID,
	}
	ws.detachedRequests[requestID] = windowName
	ws.mu.Unlock()

	window.RegisterHook(events.Common.WindowClosing, ws.detachedClosingHook(window, windowName, requestID))

	return OK(Empty{})
}

// Wails fires no beforeunload, so the first close waits for the frontend to save.
func (ws *WindowService) detachedClosingHook(window *application.WebviewWindow, windowName, requestID string) func(*application.WindowEvent) {
	return func(e *application.WindowEvent) {
		if ws.holdClose(windowName, requestID) {
			e.Cancel()
			return
		}
		ws.saveWindowPrefs(window, "detached-request")

		ws.mu.Lock()
		delete(ws.windows, windowName)
		delete(ws.detachedRequests, requestID)
		if timer := ws.closeHeld[requestID]; timer != nil {
			timer.Stop()
		}
		delete(ws.closeHeld, requestID)
		delete(ws.closeAllowed, requestID)
		ws.mu.Unlock()
	}
}

func (ws *WindowService) holdClose(windowName, requestID string) bool {
	ws.mu.Lock()
	// A second Close() can land after this window was let go; cancel the second native close.
	if ws.detachedRequests[requestID] != windowName {
		ws.mu.Unlock()
		return true
	}
	if ws.emit == nil || ws.closeAllowed[requestID] {
		ws.mu.Unlock()
		return false
	}
	if _, held := ws.closeHeld[requestID]; held {
		ws.mu.Unlock()
		return true
	}
	var timer *time.Timer
	timer = time.AfterFunc(ws.closeGrace, func() {
		ws.mu.Lock()
		// Stop() can lose to a timer already firing; only the current hold may close the window.
		current := ws.closeHeld[requestID] == timer
		if current {
			delete(ws.closeHeld, requestID)
		}
		ws.mu.Unlock()
		if current {
			ws.CloseDetached(requestID)
		}
	})
	ws.closeHeld[requestID] = timer
	emit := ws.emit
	ws.mu.Unlock()

	emit(saveAndCloseEvent(requestID), nil)
	return true
}

// CloseDetached closes a detached request window without asking it to save again.
func (ws *WindowService) CloseDetached(requestID string) Result[Empty] {
	ws.mu.Lock()
	windowName, ok := ws.detachedRequests[requestID]
	if !ok {
		ws.mu.Unlock()
		return OK(Empty{})
	}
	ws.closeAllowed[requestID] = true
	info := ws.windows[windowName]
	ws.mu.Unlock()

	if info != nil && info.Window != nil {
		info.Window.Close()
	}
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

	window := ws.app.Window.NewWithOptions(buildWindowOptions(windowName, title, url, prefs, ws.workAreas()))

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

func (ws *WindowService) IsDetached(requestID string) Result[bool] {
	ws.mu.RLock()
	defer ws.mu.RUnlock()

	_, exists := ws.detachedRequests[requestID]
	return OK(exists)
}

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

func (ws *WindowService) ChildWindowCount() Result[int] {
	ws.mu.RLock()
	defer ws.mu.RUnlock()
	return OK(len(ws.windows))
}

func (ws *WindowService) CloseChildWindows() Result[Empty] {
	ws.CloseAllChildWindows()
	deadline := time.Now().Add(ws.closeGrace + time.Second)
	for {
		if n := ws.ChildWindowCount().Data; n == 0 || time.Now().After(deadline) {
			return OK(Empty{})
		}
		time.Sleep(100 * time.Millisecond)
	}
}

// Called on main window close.
func (ws *WindowService) CloseAllChildWindows() {
	ws.mu.RLock()
	infos := make([]*WindowInfo, 0, len(ws.windows))
	for _, info := range ws.windows {
		infos = append(infos, info)
	}
	ws.mu.RUnlock()

	for _, info := range infos {
		if info == nil || info.Window == nil {
			continue
		}
		// The per-window WindowClosing hooks queue behind this one, and quit usually wins that race.
		ws.saveWindowPrefs(info.Window, info.Type)
		info.Window.Close()
	}
}
