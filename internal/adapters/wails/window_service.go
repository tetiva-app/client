package wails

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
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
	"github.com/tetiva-app/client/internal/domain/usecase/websocket"
)

type WindowInfo struct {
	Window   application.Window
	Type     string // "detached-request" or "schema-viewer"
	EntityID string // requestID or schemaID
	offset   frameOffset

	focusedAt      uint64
	closingWithApp bool
}

type frameOffset struct{ width, height int }

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
	Theme   windowTheme             `json:"theme"`
}

type windowTheme struct {
	Preference string `json:"preference,omitempty"`
	LastDark   bool   `json:"lastDark,omitempty"`
}

// Wails v3 beta changed the macOS coordinate space; positions saved by older builds are garbage there.
var legacyPositionsStale = runtime.GOOS == "darwin"

const defaultCloseGrace = 3 * time.Second

// WindowService manages detachable child windows.
type WindowService struct {
	app     *application.App
	osDark  func() bool
	emit    func(name string, data any)
	sockets websocket.Usecase
	mu      sync.RWMutex
	saveMu  sync.Mutex

	windows          map[string]*WindowInfo  // windowName → info
	detachedRequests map[string]string       // requestID → windowName
	schemas          map[string]*SchemaData  // schemaID → content
	prefs            map[string]*WindowPrefs // windowType → size/position
	theme            windowTheme
	focusSeq         uint64

	closeGrace   time.Duration
	closeHeld    map[string]*time.Timer // requestID → fallback close while the window saves
	closeAllowed map[string]bool        // requestID → saved, the next close goes through
}

func NewWindowService(sockets websocket.Usecase) *WindowService {
	ws := &WindowService{
		sockets:          sockets,
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
	ws.osDark = app.Env.IsDarkMode
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
	ws.prefs, ws.theme = readPrefs(prefsFilePath())
}

// Beyond this the prefs file is corrupt, not a display arrangement.
const coordLimit = 32767

func validGeometry(w, h, x, y int) bool {
	if w <= 0 || h <= 0 || w > coordLimit || h > coordLimit {
		return false
	}
	return x >= -coordLimit && x <= coordLimit && y >= -coordLimit && y <= coordLimit
}

// Wails on macOS opens the content area at the requested size but reports the whole frame.
var measuresFrame = runtime.GOOS == "darwin"

// Anything outside a title bar's range is the OS clamping the requested size, not a frame.
func measureFrameOffset(opts application.WebviewWindowOptions, frameW, frameH int) frameOffset {
	if !measuresFrame || frameW <= 0 || frameH <= 0 {
		return frameOffset{}
	}
	off := frameOffset{width: frameW - opts.Width, height: frameH - opts.Height}
	if off.width < -2 || off.width > 2 || off.height < 0 || off.height > 100 {
		return frameOffset{}
	}
	return off
}

// A destroyed window reads back as 0x0; that must not be persisted.
func prefsFromWindow(b application.Rect, off frameOffset) (*WindowPrefs, bool) {
	if b.Width <= 0 || b.Height <= 0 {
		return nil, false
	}
	w, h := b.Width-off.width, b.Height-off.height
	if !validGeometry(w, h, b.X, b.Y) {
		return nil, false
	}
	return &WindowPrefs{Width: w, Height: h, X: b.X, Y: b.Y, HasPosition: true}, true
}

func readPrefs(path string) (map[string]*WindowPrefs, windowTheme) {
	prefs := make(map[string]*WindowPrefs)

	data, err := os.ReadFile(path)
	if err != nil {
		return prefs, windowTheme{}
	}

	var file windowPrefsFile
	if err := json.Unmarshal(data, &file); err == nil && file.Version != 0 {
		if file.Version != windowPrefsVersion {
			return prefs, windowTheme{}
		}
		for name, p := range file.Windows {
			if p == nil || !validGeometry(p.Width, p.Height, p.X, p.Y) {
				continue
			}
			prefs[name] = p
		}
		return prefs, file.Theme
	}

	var legacy map[string]*WindowPrefs
	if err := json.Unmarshal(data, &legacy); err != nil {
		return prefs, windowTheme{}
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
	return prefs, windowTheme{}
}

// saveMu spans snapshot and write, so a slower save can never overwrite a newer snapshot.
func (ws *WindowService) savePrefs() {
	ws.saveMu.Lock()
	defer ws.saveMu.Unlock()

	ws.mu.RLock()
	data, err := json.MarshalIndent(windowPrefsFile{Version: windowPrefsVersion, Windows: ws.prefs, Theme: ws.theme}, "", "  ")
	ws.mu.RUnlock()

	if err != nil {
		return
	}

	path := prefsFilePath()
	_ = os.MkdirAll(filepath.Dir(path), 0o755)
	_ = writeFileAtomic(path, data)
}

func writeFileAtomic(path string, data []byte) error {
	f, err := os.CreateTemp(filepath.Dir(path), filepath.Base(path)+"-*.tmp")
	if err != nil {
		return err
	}
	_, err = f.Write(data)
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err == nil {
		err = os.Rename(f.Name(), path)
	}
	if err != nil {
		_ = os.Remove(f.Name())
	}
	return err
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

// windowBackground matches --background of each theme in frontend/src/assets/index.css.
func windowBackground(dark bool) application.RGBA {
	if dark {
		return application.NewRGB(15, 17, 23)
	}
	return application.NewRGB(255, 255, 255)
}

// Wails' GTK3 build reports dark only when the GTK theme name contains "dark".
var osThemeReliable = runtime.GOOS != "linux"

func darkWindow(theme windowTheme, osDark, osReliable bool) bool {
	switch theme.Preference {
	case "light":
		return false
	case "dark":
		return true
	}
	if !osReliable {
		return theme.LastDark
	}
	return osDark
}

func (ws *WindowService) background() application.RGBA {
	ws.mu.RLock()
	theme := ws.theme
	ws.mu.RUnlock()
	return windowBackground(darkWindow(theme, ws.osDark(), osThemeReliable))
}

// SetTheme takes effect for windows opened afterwards, the main one on the next start.
func (ws *WindowService) SetTheme(preference, effective string) Result[Empty] {
	const funcName = "WindowService.SetTheme"
	validPreference := preference == "light" || preference == "dark" || preference == "system"
	if !validPreference || (effective != "light" && effective != "dark") {
		return Err[Empty](fmt.Errorf("%s: %w", funcName, &domain.ValidationError{
			Fields: map[string]string{"theme": "unknown theme " + preference + "/" + effective},
		}))
	}
	theme := windowTheme{Preference: preference, LastDark: effective == "dark"}

	ws.mu.Lock()
	changed := ws.theme != theme
	ws.theme = theme
	ws.mu.Unlock()

	if changed {
		ws.savePrefs()
	}
	return OK(Empty{})
}

// Wails reads the OS theme only after Run starts; pre-Run windows are built after this.
func (ws *WindowService) ServiceStartup(context.Context, application.ServiceOptions) error {
	if ws.app == nil {
		return nil
	}
	paintBackground(ws.app.Window.GetAll(), ws.background())
	return nil
}

func paintBackground(windows []application.Window, colour application.RGBA) {
	for _, w := range windows {
		w.SetBackgroundColour(colour)
	}
}

// Without a usable saved position Wails centers the window.
func buildWindowOptions(name, title, url string, prefs *WindowPrefs, areas []application.Rect, background application.RGBA) application.WebviewWindowOptions {
	opts := application.WebviewWindowOptions{
		Name:             name,
		Title:            title,
		Width:            prefs.Width,
		Height:           prefs.Height,
		URL:              url,
		BackgroundColour: background,
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

// Wails creates the native window before NewWithOptions returns, so its frame is read before the user can resize it.
func (ws *WindowService) openWindow(opts application.WebviewWindowOptions) (*application.WebviewWindow, frameOffset) {
	window := ws.app.Window.NewWithOptions(opts)
	w, h := window.Size()
	return window, measureFrameOffset(opts, w, h)
}

func (ws *WindowService) trackFocus(window application.Window, windowName string) {
	ws.windowFocused(windowName)
	window.OnWindowEvent(events.Common.WindowFocus, func(*application.WindowEvent) {
		ws.windowFocused(windowName)
	})
}

func (ws *WindowService) windowFocused(windowName string) {
	ws.mu.Lock()
	defer ws.mu.Unlock()
	if info := ws.windows[windowName]; info != nil {
		ws.focusSeq++
		info.focusedAt = ws.focusSeq
	}
}

// Windows closed along with the app are saved by saveLastFocusedWindows instead.
func (ws *WindowService) saveClosedWindowPrefs(info *WindowInfo) {
	ws.storeWindowPrefs(info, true)
}

func (ws *WindowService) saveWindowPrefs(info *WindowInfo) {
	ws.storeWindowPrefs(info, false)
}

// The quit check shares the commit's lock: the app can start quitting while Bounds() runs.
func (ws *WindowService) storeWindowPrefs(info *WindowInfo, unlessClosingWithApp bool) {
	if info == nil || info.Window == nil {
		return
	}
	prefs, ok := prefsFromWindow(info.Window.Bounds(), info.offset)
	if !ok {
		return
	}
	ws.mu.Lock()
	if unlessClosingWithApp && info.closingWithApp {
		ws.mu.Unlock()
		return
	}
	ws.prefs[info.Type] = prefs
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

	window, offset := ws.openWindow(buildWindowOptions(windowName, title, url, prefs, ws.workAreas(), ws.background()))
	info := &WindowInfo{
		Window:   window,
		Type:     "detached-request",
		EntityID: requestID,
		offset:   offset,
	}

	ws.mu.Lock()
	ws.windows[windowName] = info
	ws.detachedRequests[requestID] = windowName
	ws.mu.Unlock()

	ws.trackFocus(window, windowName)
	window.RegisterHook(events.Common.WindowClosing, ws.detachedClosingHook(info, windowName, requestID))

	return OK(Empty{})
}

// Wails fires no beforeunload, so the first close waits for the frontend to save.
func (ws *WindowService) detachedClosingHook(info *WindowInfo, windowName, requestID string) func(*application.WindowEvent) {
	return func(e *application.WindowEvent) {
		if ws.holdClose(windowName, requestID) {
			e.Cancel()
			return
		}
		ws.closeWebSockets(windowName)
		ws.saveClosedWindowPrefs(info)

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

// A close handshake with a server that stopped answering takes seconds; the window does not wait for it.
func (ws *WindowService) closeWebSockets(windowName string) {
	if ws.sockets == nil {
		return
	}
	go func() {
		if err := ws.sockets.DisconnectOwner(context.Background(), windowName); err != nil {
			slog.Warn("window: failed to close the websockets of a closed window", "window", windowName, "err", err)
		}
	}()
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

	window, offset := ws.openWindow(buildWindowOptions(windowName, title, url, prefs, ws.workAreas(), ws.background()))
	info := &WindowInfo{
		Window:   window,
		Type:     "schema-viewer",
		EntityID: schemaID,
		offset:   offset,
	}

	ws.mu.Lock()
	ws.windows[windowName] = info
	ws.mu.Unlock()

	ws.trackFocus(window, windowName)
	window.RegisterHook(events.Common.WindowClosing, func(e *application.WindowEvent) {
		ws.saveClosedWindowPrefs(info)

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
	ws.saveLastFocusedWindows()

	ws.mu.RLock()
	infos := make([]*WindowInfo, 0, len(ws.windows))
	for _, info := range ws.windows {
		if info != nil && info.Window != nil {
			infos = append(infos, info)
		}
	}
	ws.mu.RUnlock()

	for _, info := range infos {
		info.Window.Close()
	}
}

// At quit Wails calls this before it closes the windows, and no WindowClosing hook runs for that close.
func (ws *WindowService) ServiceShutdown() error {
	ws.saveLastFocusedWindows()
	return nil
}

// Windows marked here skip their own save and any later call, so the first save at quit stands.
func (ws *WindowService) saveLastFocusedWindows() {
	ws.mu.Lock()
	lastFocused := make(map[string]*WindowInfo)
	for _, info := range ws.windows {
		if info == nil || info.Window == nil || info.closingWithApp {
			continue
		}
		info.closingWithApp = true
		if last := lastFocused[info.Type]; last == nil || info.focusedAt > last.focusedAt {
			lastFocused[info.Type] = info
		}
	}
	ws.mu.Unlock()

	for _, info := range lastFocused {
		ws.saveWindowPrefs(info)
	}
}
