package wails

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/wailsapp/wails/v3/pkg/application"

	"github.com/tetiva-app/client/internal/constants"
	"github.com/tetiva-app/client/internal/domain/usecase/websocket"
)

func TestWindowService_GetSchemaContent_Happy(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	ws := NewWindowService(nil)

	schemaID := uuid.New().String()
	want := &SchemaData{
		Definition: "syntax = \"proto3\";",
		Source:     "test.proto",
		Language:   "protobuf",
		SchemaJSON: "",
	}
	ws.mu.Lock()
	ws.schemas[schemaID] = want
	ws.mu.Unlock()

	res := ws.GetSchemaContent(schemaID)

	require.Nil(t, res.Error)
	assert.Equal(t, want.Definition, res.Data.Definition)
	assert.Equal(t, want.Source, res.Data.Source)
	assert.Equal(t, "protobuf", res.Data.Language)
}

func TestWindowService_GetSchemaContent_Error_NotFound(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	ws := NewWindowService(nil)

	res := ws.GetSchemaContent(uuid.New().String())

	require.NotNil(t, res.Error)
	assert.Equal(t, ErrCodeNotFound, res.Error.Code)
}

func TestWindowService_DetachRequest_Happy_AlreadyDetached(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	ws := NewWindowService(nil)

	requestID := uuid.New().String()
	// Pre-populate detachedRequests so DetachRequest takes the FocusDetachedWindow path.
	// Leave ws.windows empty so info is nil → Focus is skipped (no app needed).
	ws.mu.Lock()
	ws.detachedRequests[requestID] = "detached-request-" + requestID
	ws.mu.Unlock()

	res := ws.DetachRequest(requestID, "http", "Already Open")

	require.Nil(t, res.Error)
}

func TestWindowService_DetachRequest_Error_AppNotInitialized(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	ws := NewWindowService(nil)

	res := ws.DetachRequest(uuid.New().String(), "http", "New Window")

	require.NotNil(t, res.Error)
	assert.Equal(t, ErrCodeInternal, res.Error.Code)
	assert.Contains(t, res.Error.Message, "app not initialized")
}

func detachedForTest(t *testing.T) (*WindowService, string, *[]string) {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	ws := NewWindowService(nil)
	emitted := []string{}
	ws.SetEventEmitter(func(name string, _ any) { emitted = append(emitted, name) })
	requestID := uuid.New().String()
	ws.mu.Lock()
	ws.detachedRequests[requestID] = "detached-request-" + requestID
	ws.mu.Unlock()
	return ws, requestID, &emitted
}

func TestWindowService_DetachedClosing_WaitsForSaveOnce(t *testing.T) {
	ws, requestID, emitted := detachedForTest(t)
	hook := ws.detachedClosingHook(nil, "detached-request-"+requestID, requestID)

	first := application.NewWindowEvent()
	hook(first)
	second := application.NewWindowEvent()
	hook(second)

	assert.True(t, first.IsCancelled())
	assert.True(t, second.IsCancelled())
	assert.Equal(t, []string{"window:save-and-close:" + requestID}, *emitted)
	assert.True(t, ws.IsDetached(requestID).Data)

	require.Nil(t, ws.CloseDetached(requestID).Error)
	third := application.NewWindowEvent()
	hook(third)

	assert.False(t, third.IsCancelled())
	assert.False(t, ws.IsDetached(requestID).Data)
}

func TestWindowService_DetachedClosing_ClosesWhenTheFrontendNeverAnswers(t *testing.T) {
	ws, requestID, _ := detachedForTest(t)
	ws.closeGrace = 10 * time.Millisecond
	hook := ws.detachedClosingHook(nil, "detached-request-"+requestID, requestID)

	held := application.NewWindowEvent()
	hook(held)
	require.True(t, held.IsCancelled())

	require.Eventually(t, func() bool {
		next := application.NewWindowEvent()
		hook(next)
		return !next.IsCancelled()
	}, time.Second, 5*time.Millisecond)
}

func TestWindowService_DetachedClosing_NoEmitterClosesAtOnce(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	ws := NewWindowService(nil)
	requestID := uuid.New().String()
	ws.mu.Lock()
	ws.detachedRequests[requestID] = "detached-request-" + requestID
	ws.mu.Unlock()

	e := application.NewWindowEvent()
	ws.detachedClosingHook(nil, "detached-request-"+requestID, requestID)(e)

	assert.False(t, e.IsCancelled())
	assert.False(t, ws.IsDetached(requestID).Data)
}

func TestWindowService_DetachedClosing_ReopenedWindowSavesAgain(t *testing.T) {
	ws, requestID, emitted := detachedForTest(t)
	windowName := "detached-request-" + requestID
	hook := ws.detachedClosingHook(nil, windowName, requestID)
	hook(application.NewWindowEvent())
	require.Nil(t, ws.CloseDetached(requestID).Error)
	hook(application.NewWindowEvent())

	ws.mu.Lock()
	ws.detachedRequests[requestID] = windowName
	ws.mu.Unlock()
	reopened := application.NewWindowEvent()
	ws.detachedClosingHook(nil, windowName, requestID)(reopened)

	assert.True(t, reopened.IsCancelled())
	assert.Len(t, *emitted, 2)
}

func TestWindowService_DetachedClosing_LateEventAfterCloseLeavesNoHold(t *testing.T) {
	ws, requestID, emitted := detachedForTest(t)
	windowName := "detached-request-" + requestID
	hook := ws.detachedClosingHook(nil, windowName, requestID)
	hook(application.NewWindowEvent())
	require.Nil(t, ws.CloseDetached(requestID).Error)
	allowed := application.NewWindowEvent()
	hook(allowed)
	require.False(t, allowed.IsCancelled())

	late := application.NewWindowEvent()
	hook(late)

	assert.True(t, late.IsCancelled())
	assert.Len(t, *emitted, 1)
	ws.mu.RLock()
	assert.Empty(t, ws.closeHeld)
	ws.mu.RUnlock()

	ws.mu.Lock()
	ws.detachedRequests[requestID] = windowName
	ws.mu.Unlock()
	reopened := application.NewWindowEvent()
	ws.detachedClosingHook(nil, windowName, requestID)(reopened)

	assert.True(t, reopened.IsCancelled())
	assert.Equal(t, []string{saveAndCloseEvent(requestID), saveAndCloseEvent(requestID)}, *emitted)
}

func TestWindowService_DetachedClosing_FallbackReleasesItsHold(t *testing.T) {
	ws, requestID, _ := detachedForTest(t)
	ws.closeGrace = 10 * time.Millisecond
	held := application.NewWindowEvent()
	ws.detachedClosingHook(nil, "detached-request-"+requestID, requestID)(held)
	require.True(t, held.IsCancelled())

	require.Eventually(t, func() bool {
		ws.mu.RLock()
		defer ws.mu.RUnlock()
		return ws.closeAllowed[requestID]
	}, time.Second, 5*time.Millisecond)
	ws.mu.RLock()
	assert.Empty(t, ws.closeHeld)
	ws.mu.RUnlock()
}

func TestWindowService_CloseDetached_UnknownRequest(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	ws := NewWindowService(nil)

	res := ws.CloseDetached(uuid.New().String())

	require.Nil(t, res.Error)
	assert.Empty(t, ws.closeAllowed)
}

// TODO: OpenSchemaViewer happy path needs a real *application.App; covered by manual testing.

func TestWindowService_OpenSchemaViewer_Error_AppNotInitialized(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	ws := NewWindowService(nil)

	res := ws.OpenSchemaViewer("syntax=\"proto3\";", "test.proto", "Schema", "protobuf", "")

	require.NotNil(t, res.Error)
	assert.Equal(t, ErrCodeInternal, res.Error.Code)
	assert.Contains(t, res.Error.Message, "app not initialized")
}

func TestWindowService_IsDetached_Happy_True(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	ws := NewWindowService(nil)

	requestID := uuid.New().String()
	ws.mu.Lock()
	ws.detachedRequests[requestID] = "detached-request-" + requestID
	ws.mu.Unlock()

	res := ws.IsDetached(requestID)

	require.Nil(t, res.Error)
	assert.True(t, res.Data)
}

func TestWindowService_IsDetached_Happy_False(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	ws := NewWindowService(nil)

	res := ws.IsDetached(uuid.New().String())

	require.Nil(t, res.Error)
	assert.False(t, res.Data)
}

func TestWindowService_FocusDetachedWindow_Happy_NotExists(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	ws := NewWindowService(nil)

	res := ws.FocusDetachedWindow(uuid.New().String())

	require.Nil(t, res.Error)
}

func TestWindowService_FocusDetachedWindow_Happy_Exists_NoFocus(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	ws := NewWindowService(nil)

	requestID := uuid.New().String()
	windowName := "x"
	// Pre-populate with a WindowInfo whose Window is nil, so Focus() is skipped.
	ws.mu.Lock()
	ws.detachedRequests[requestID] = windowName
	ws.windows[windowName] = &WindowInfo{Window: nil, Type: "detached-request", EntityID: requestID}
	ws.mu.Unlock()

	res := ws.FocusDetachedWindow(requestID)

	require.Nil(t, res.Error)
}

func TestWindowService_ChildWindowCount(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	ws := NewWindowService(nil)
	ws.mu.Lock()
	ws.windows["detached-request-a"] = &WindowInfo{Type: "detached-request"}
	ws.windows["schema-viewer-b"] = &WindowInfo{Type: "schema-viewer"}
	ws.mu.Unlock()

	res := ws.ChildWindowCount()

	require.Nil(t, res.Error)
	assert.Equal(t, 2, res.Data)
}

func TestWindowService_CloseChildWindows_NoneReturnsAtOnce(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	ws := NewWindowService(nil)
	start := time.Now()

	res := ws.CloseChildWindows()

	require.Nil(t, res.Error)
	assert.Less(t, time.Since(start), 50*time.Millisecond)
}

func writePrefsFile(t *testing.T, content string) {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	dir := filepath.Join(home, constants.AppDir)
	require.NoError(t, os.MkdirAll(dir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "window-prefs.json"), []byte(content), 0o644))
}

func setLegacyPositionsStale(t *testing.T, stale bool) {
	t.Helper()
	prev := legacyPositionsStale
	legacyPositionsStale = stale
	t.Cleanup(func() { legacyPositionsStale = prev })
}

func TestWindowService_LoadPrefs_LegacyFlat_Darwin(t *testing.T) {
	setLegacyPositionsStale(t, true)
	writePrefsFile(t, `{"detached-request":{"width":900,"height":600,"x":-9999,"y":-9999}}`)

	ws := NewWindowService(nil)

	got := ws.getPrefsOrDefaults("detached-request")
	assert.Equal(t, 900, got.Width)
	assert.Equal(t, 600, got.Height)
	assert.Equal(t, 0, got.X)
	assert.Equal(t, 0, got.Y)
	assert.False(t, got.HasPosition)
}

func TestWindowService_LoadPrefs_LegacyFlat_OtherOS(t *testing.T) {
	setLegacyPositionsStale(t, false)
	writePrefsFile(t, `{"detached-request":{"width":900,"height":600,"x":120,"y":80},"schema-viewer":{"width":700,"height":500,"x":0,"y":0},"only-y":{"width":700,"height":500,"x":0,"y":50},"only-x":{"width":700,"height":500,"x":50,"y":0}}`)

	ws := NewWindowService(nil)

	detached := ws.getPrefsOrDefaults("detached-request")
	assert.Equal(t, 120, detached.X)
	assert.Equal(t, 80, detached.Y)
	assert.True(t, detached.HasPosition)

	schema := ws.getPrefsOrDefaults("schema-viewer")
	assert.False(t, schema.HasPosition)

	assert.True(t, ws.getPrefsOrDefaults("only-y").HasPosition)
	assert.True(t, ws.getPrefsOrDefaults("only-x").HasPosition)
}

func TestWindowService_LoadPrefs_LegacySkipsZeroSize(t *testing.T) {
	setLegacyPositionsStale(t, false)
	writePrefsFile(t, `{"detached-request":{"width":0,"height":600,"x":120,"y":80},"schema-viewer":{"width":700,"height":-1}}`)

	ws := NewWindowService(nil)

	assert.Equal(t, &WindowPrefs{Width: 1000, Height: 700}, ws.getPrefsOrDefaults("detached-request"))
	assert.Equal(t, &WindowPrefs{Width: 700, Height: 500}, ws.getPrefsOrDefaults("schema-viewer"))
}

func TestWindowService_LoadPrefs_V2Keeps(t *testing.T) {
	writePrefsFile(t, `{"version":2,"windows":{"detached-request":{"width":900,"height":600,"x":120,"y":80,"hasPosition":true}}}`)

	ws := NewWindowService(nil)

	got := ws.getPrefsOrDefaults("detached-request")
	assert.Equal(t, &WindowPrefs{Width: 900, Height: 600, X: 120, Y: 80, HasPosition: true}, got)
}

func TestWindowService_LoadPrefs_V2SkipsZeroSize(t *testing.T) {
	writePrefsFile(t, `{"version":2,"windows":{"detached-request":{"width":0,"height":600,"hasPosition":true},"schema-viewer":{"width":700,"height":-1,"hasPosition":true}}}`)

	ws := NewWindowService(nil)

	assert.Equal(t, &WindowPrefs{Width: 1000, Height: 700}, ws.getPrefsOrDefaults("detached-request"))
	assert.Equal(t, &WindowPrefs{Width: 700, Height: 500}, ws.getPrefsOrDefaults("schema-viewer"))
}

func TestWindowService_LoadPrefs_V2SkipsOversized(t *testing.T) {
	writePrefsFile(t, `{"version":2,"windows":{"detached-request":{"width":900,"height":600,"x":2147483647,"y":80,"hasPosition":true},"schema-viewer":{"width":99999,"height":500,"hasPosition":true}}}`)

	ws := NewWindowService(nil)

	assert.Equal(t, &WindowPrefs{Width: 1000, Height: 700}, ws.getPrefsOrDefaults("detached-request"))
	assert.Equal(t, &WindowPrefs{Width: 700, Height: 500}, ws.getPrefsOrDefaults("schema-viewer"))
}

func TestWindowService_LoadPrefs_LegacySkipsOversized(t *testing.T) {
	setLegacyPositionsStale(t, false)
	writePrefsFile(t, `{"detached-request":{"width":900,"height":600,"x":-2147483648,"y":80}}`)

	ws := NewWindowService(nil)

	assert.Equal(t, &WindowPrefs{Width: 1000, Height: 700}, ws.getPrefsOrDefaults("detached-request"))
}

func TestWindowService_LoadPrefs_V2NullWindows(t *testing.T) {
	writePrefsFile(t, `{"version":2,"windows":null}`)

	ws := NewWindowService(nil)

	require.NotNil(t, ws.prefs)
	assert.Empty(t, ws.prefs)
	assert.NotPanics(t, ws.savePrefs)
}

func TestWindowService_LoadPrefs_NullEntry_Legacy(t *testing.T) {
	writePrefsFile(t, `{"detached-request":null}`)

	ws := NewWindowService(nil)

	assert.Equal(t, &WindowPrefs{Width: 1000, Height: 700}, ws.getPrefsOrDefaults("detached-request"))
}

func TestWindowService_LoadPrefs_NullEntry_V2(t *testing.T) {
	writePrefsFile(t, `{"version":2,"windows":{"detached-request":null}}`)

	ws := NewWindowService(nil)

	assert.Equal(t, &WindowPrefs{Width: 1000, Height: 700}, ws.getPrefsOrDefaults("detached-request"))
}

func TestWindowService_LoadPrefs_UnknownVersion(t *testing.T) {
	writePrefsFile(t, `{"version":3,"windows":{"detached-request":{"width":900,"height":600,"x":120,"y":80,"hasPosition":true}}}`)

	ws := NewWindowService(nil)

	assert.Empty(t, ws.prefs)
	assert.Equal(t, &WindowPrefs{Width: 1000, Height: 700}, ws.getPrefsOrDefaults("detached-request"))
}

func TestWindowService_LoadPrefs_Corrupt(t *testing.T) {
	writePrefsFile(t, `{"version":2,"windows":`)

	ws := NewWindowService(nil)

	require.NotNil(t, ws.prefs)
	assert.Empty(t, ws.prefs)
}

func TestWindowService_SavePrefs_WritesV2(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	ws := NewWindowService(nil)

	ws.mu.Lock()
	ws.prefs["detached-request"] = &WindowPrefs{Width: 900, Height: 600, X: 120, Y: 80, HasPosition: true}
	ws.mu.Unlock()
	ws.savePrefs()

	raw, err := os.ReadFile(prefsFilePath())
	require.NoError(t, err)
	assert.Contains(t, string(raw), `"hasPosition"`)

	var file windowPrefsFile
	require.NoError(t, json.Unmarshal(raw, &file))
	assert.Equal(t, 2, file.Version)
	require.Contains(t, file.Windows, "detached-request")
	assert.Equal(t, &WindowPrefs{Width: 900, Height: 600, X: 120, Y: 80, HasPosition: true}, file.Windows["detached-request"])
}

func TestWindowService_SavePrefs_ConcurrentThemeChangeKeepsLatest(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	ws := NewWindowService(nil)

	var wg sync.WaitGroup
	for i := range 200 {
		wg.Go(ws.savePrefs)
		wg.Go(func() {
			effective := "light"
			if i%2 == 0 {
				effective = "dark"
			}
			ws.SetTheme("system", effective)
		})
	}
	wg.Wait()

	raw, err := os.ReadFile(prefsFilePath())
	require.NoError(t, err)
	var file windowPrefsFile
	require.NoError(t, json.Unmarshal(raw, &file))
	assert.Equal(t, ws.theme, file.Theme)
}

func TestBuildWindowOptions_NoPositionWhenNotSaved(t *testing.T) {
	areas := []application.Rect{{X: 0, Y: 0, Width: 1920, Height: 1080}}
	prefs := &WindowPrefs{Width: 900, Height: 600, X: 120, Y: 80}

	opts := buildWindowOptions("detached-request-1", "Title", "/?mode=detached-request", prefs, areas, windowBackground(false))

	assert.Equal(t, application.WindowCentered, opts.InitialPosition)
	assert.Equal(t, 0, opts.X)
	assert.Equal(t, 0, opts.Y)
	assert.Equal(t, 900, opts.Width)
	assert.Equal(t, 600, opts.Height)
}

func TestBuildWindowOptions_PositionAppliedWhenOnScreen(t *testing.T) {
	areas := []application.Rect{{X: 0, Y: 0, Width: 1920, Height: 1080}}
	prefs := &WindowPrefs{Width: 900, Height: 600, X: 120, Y: 80, HasPosition: true}

	opts := buildWindowOptions("detached-request-1", "Title", "/?mode=detached-request", prefs, areas, windowBackground(false))

	assert.Equal(t, application.WindowXY, opts.InitialPosition)
	assert.Equal(t, 120, opts.X)
	assert.Equal(t, 80, opts.Y)
}

func TestBuildWindowOptions_PositionDroppedWhenOffScreen(t *testing.T) {
	areas := []application.Rect{{X: 0, Y: 0, Width: 1920, Height: 1080}}
	prefs := &WindowPrefs{Width: 900, Height: 600, X: -9999, Y: -9999, HasPosition: true}

	opts := buildWindowOptions("detached-request-1", "Title", "/?mode=detached-request", prefs, areas, windowBackground(false))

	assert.Equal(t, application.WindowCentered, opts.InitialPosition)
	assert.Equal(t, 0, opts.X)
	assert.Equal(t, 0, opts.Y)
}

func TestPositionOnScreen(t *testing.T) {
	primary := application.Rect{X: 0, Y: 0, Width: 1920, Height: 1080}
	withMenuBar := application.Rect{X: 0, Y: 25, Width: 1920, Height: 1055}
	above := application.Rect{X: 0, Y: -1080, Width: 1920, Height: 1080}
	small := application.Rect{X: 0, Y: 0, Width: 800, Height: 600}
	right := application.Rect{X: 1920, Y: 0, Width: 1920, Height: 1080}

	tests := []struct {
		name    string
		x, y, w int
		areas   []application.Rect
		want    bool
	}{
		{name: "no screens", x: 100, y: 100, w: 900, areas: nil, want: false},
		{name: "inside primary", x: 100, y: 100, w: 400, areas: []application.Rect{primary}, want: true},
		{name: "origin of primary", x: 0, y: 0, w: 1000, areas: []application.Rect{primary}, want: true},
		{name: "above work area", x: 100, y: 0, w: 900, areas: []application.Rect{withMenuBar}, want: false},
		{name: "past right edge", x: 1870, y: 100, w: 500, areas: []application.Rect{primary}, want: false},
		{name: "exactly enough overlap", x: 1820, y: 100, w: 500, areas: []application.Rect{primary}, want: true},
		{name: "narrow window fully inside", x: 100, y: 100, w: 80, areas: []application.Rect{primary}, want: true},
		{name: "screen above primary", x: 100, y: -1000, w: 900, areas: []application.Rect{primary, above}, want: true},
		{name: "window wider than screen", x: -200, y: 10, w: 1200, areas: []application.Rect{small}, want: true},
		{name: "below work area", x: 100, y: 1080, w: 900, areas: []application.Rect{primary}, want: false},
		{name: "left of a non-origin screen", x: 0, y: 10, w: 200, areas: []application.Rect{right}, want: false},
		{name: "left overlap exactly 100", x: 1820, y: 10, w: 200, areas: []application.Rect{right}, want: true},
		{name: "left overlap 99", x: 1819, y: 10, w: 200, areas: []application.Rect{right}, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, positionOnScreen(tt.x, tt.y, tt.w, tt.areas))
		})
	}
}

func TestPrefsFromWindow(t *testing.T) {
	tests := []struct {
		name   string
		bounds application.Rect
		offset frameOffset
		want   *WindowPrefs
	}{
		{name: "valid", bounds: application.Rect{X: 120, Y: 80, Width: 900, Height: 600}, want: &WindowPrefs{Width: 900, Height: 600, X: 120, Y: 80, HasPosition: true}},
		{name: "negative position is fine", bounds: application.Rect{X: -1200, Y: -50, Width: 900, Height: 600}, want: &WindowPrefs{Width: 900, Height: 600, X: -1200, Y: -50, HasPosition: true}},
		{name: "frame offset is taken off", bounds: application.Rect{X: 120, Y: 80, Width: 899, Height: 631}, offset: frameOffset{width: -1, height: 31}, want: &WindowPrefs{Width: 900, Height: 600, X: 120, Y: 80, HasPosition: true}},
		{name: "destroyed window", bounds: application.Rect{}},
		{name: "destroyed window with an offset", bounds: application.Rect{}, offset: frameOffset{width: -1, height: 31}},
		{name: "zero width", bounds: application.Rect{Height: 600}},
		{name: "negative height", bounds: application.Rect{Width: 900, Height: -1}},
		{name: "oversized width", bounds: application.Rect{Width: 40000, Height: 600}},
		{name: "oversized height", bounds: application.Rect{Width: 900, Height: 40000}},
		{name: "oversized x", bounds: application.Rect{X: 40000, Width: 900, Height: 600}},
		{name: "oversized negative y", bounds: application.Rect{Y: -40000, Width: 900, Height: 600}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := prefsFromWindow(tt.bounds, tt.offset)
			assert.Equal(t, tt.want != nil, ok)
			assert.Equal(t, tt.want, got)
		})
	}
}

// macFrame is what Wails beta.28 reports on macOS: a content area of (w-1)x(h-1) under a 32 pt title bar.
func macFrame(opts application.WebviewWindowOptions) (int, int) {
	return opts.Width - 1, opts.Height - 1 + 32
}

func setMeasuresFrame(t *testing.T, measures bool) {
	t.Helper()
	prev := measuresFrame
	measuresFrame = measures
	t.Cleanup(func() { measuresFrame = prev })
}

func TestWindowGeometry_ReopensAtTheSameSize(t *testing.T) {
	setMeasuresFrame(t, true)
	prefs := &WindowPrefs{Width: 900, Height: 600}

	for range 3 {
		opts := buildWindowOptions("detached-request-1", "Title", "/", prefs, nil, windowBackground(false))
		w, h := macFrame(opts)
		next, ok := prefsFromWindow(application.Rect{X: 40, Y: 60, Width: w, Height: h}, measureFrameOffset(opts, w, h))

		require.True(t, ok)
		assert.Equal(t, 900, next.Width)
		assert.Equal(t, 600, next.Height)
		prefs = next
	}
}

func TestWindowGeometry_ReopensAResizedWindowAtItsFrame(t *testing.T) {
	setMeasuresFrame(t, true)
	opts := buildWindowOptions("detached-request-1", "Title", "/", &WindowPrefs{Width: 900, Height: 600}, nil, windowBackground(false))
	w, h := macFrame(opts)
	saved, ok := prefsFromWindow(application.Rect{Width: 1200, Height: 832}, measureFrameOffset(opts, w, h))
	require.True(t, ok)

	reopenedW, reopenedH := macFrame(buildWindowOptions("detached-request-1", "Title", "/", saved, nil, windowBackground(false)))

	assert.Equal(t, 1200, reopenedW)
	assert.Equal(t, 832, reopenedH)
}

func TestMeasureFrameOffset(t *testing.T) {
	setMeasuresFrame(t, true)
	opts := application.WebviewWindowOptions{Width: 900, Height: 600}

	tests := []struct {
		name           string
		frameW, frameH int
		want           frameOffset
	}{
		{name: "destroyed window", want: frameOffset{}},
		{name: "title bar", frameW: 899, frameH: 631, want: frameOffset{width: -1, height: 31}},
		{name: "largest plausible frame", frameW: 902, frameH: 700, want: frameOffset{width: 2, height: 100}},
		{name: "smallest plausible frame", frameW: 898, frameH: 600, want: frameOffset{width: -2}},
		{name: "width clamped to the screen", frameW: 880, frameH: 631, want: frameOffset{}},
		{name: "height clamped to the screen", frameW: 899, frameH: 590, want: frameOffset{}},
		{name: "wider than requested", frameW: 903, frameH: 631, want: frameOffset{}},
		{name: "taller than a title bar", frameW: 899, frameH: 701, want: frameOffset{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, measureFrameOffset(opts, tt.frameW, tt.frameH))
		})
	}
}

func TestMeasureFrameOffset_OnlyOnMacOS(t *testing.T) {
	setMeasuresFrame(t, false)

	got := measureFrameOffset(application.WebviewWindowOptions{Width: 900, Height: 600}, 899, 631)

	assert.Equal(t, frameOffset{}, got)
}

func TestMeasuresFrame_MatchesPlatform(t *testing.T) {
	assert.Equal(t, runtime.GOOS == "darwin", measuresFrame)
}

type geometryWindow struct {
	application.Window
	bounds application.Rect
	closed bool
}

func (w *geometryWindow) Bounds() application.Rect { return w.bounds }
func (w *geometryWindow) Close()                   { w.closed = true }

func TestWindowService_SaveWindowPrefs_StoresTheRequestedSize(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	ws := NewWindowService(nil)
	info := &WindowInfo{
		Window: &geometryWindow{bounds: application.Rect{X: 120, Y: 80, Width: 899, Height: 631}},
		Type:   "detached-request",
		offset: frameOffset{width: -1, height: 31},
	}

	ws.saveWindowPrefs(info)

	assert.Equal(t, &WindowPrefs{Width: 900, Height: 600, X: 120, Y: 80, HasPosition: true}, NewWindowService(nil).getPrefsOrDefaults("detached-request"))
}

func childWindowForTest(ws *WindowService, requestID string, bounds application.Rect) *WindowInfo {
	name := "detached-request-" + requestID
	info := &WindowInfo{Window: &geometryWindow{bounds: bounds}, Type: "detached-request", EntityID: requestID}
	ws.mu.Lock()
	ws.windows[name] = info
	ws.detachedRequests[requestID] = name
	ws.mu.Unlock()
	ws.windowFocused(name)
	return info
}

var (
	boundsA = application.Rect{X: 10, Y: 20, Width: 800, Height: 500}
	boundsB = application.Rect{X: 30, Y: 40, Width: 1100, Height: 700}
	boundsC = application.Rect{X: 50, Y: 60, Width: 640, Height: 480}
)

func prefsOf(b application.Rect) *WindowPrefs {
	return &WindowPrefs{Width: b.Width, Height: b.Height, X: b.X, Y: b.Y, HasPosition: true}
}

func TestWindowService_CloseAllChildWindows_SavesTheLastFocusedWindow(t *testing.T) {
	for range 20 {
		t.Setenv("HOME", t.TempDir())
		ws := NewWindowService(nil)
		a := childWindowForTest(ws, "a", boundsA)
		b := childWindowForTest(ws, "b", boundsB)
		c := childWindowForTest(ws, "c", boundsC)
		ws.windowFocused("detached-request-b")
		ws.windowFocused("detached-request-a")

		ws.CloseAllChildWindows()

		assert.Equal(t, prefsOf(boundsA), NewWindowService(nil).getPrefsOrDefaults("detached-request"))
		for _, info := range []*WindowInfo{a, b, c} {
			assert.True(t, info.Window.(*geometryWindow).closed)
		}
	}
}

func TestWindowService_ClosingWithTheAppKeepsTheLastFocusedGeometry(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	ws := NewWindowService(nil)
	a := childWindowForTest(ws, "a", boundsA)
	childWindowForTest(ws, "b", boundsB)

	ws.CloseAllChildWindows()
	closing := application.NewWindowEvent()
	ws.detachedClosingHook(a, "detached-request-a", "a")(closing)

	assert.False(t, closing.IsCancelled())
	assert.Equal(t, prefsOf(boundsB), NewWindowService(nil).getPrefsOrDefaults("detached-request"))
}

func TestWindowService_ServiceShutdown_SavesTheLastFocusedWindow(t *testing.T) {
	for range 20 {
		t.Setenv("HOME", t.TempDir())
		ws := NewWindowService(nil)
		childWindowForTest(ws, "a", boundsA)
		childWindowForTest(ws, "b", boundsB)
		childWindowForTest(ws, "c", boundsC)
		ws.windowFocused("detached-request-b")
		ws.windowFocused("detached-request-a")

		require.NoError(t, ws.ServiceShutdown())

		assert.Equal(t, prefsOf(boundsA), NewWindowService(nil).getPrefsOrDefaults("detached-request"))
	}
}

func TestWindowService_WindowsClosingAfterShutdownKeepTheLastFocusedGeometry(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	ws := NewWindowService(nil)
	a := childWindowForTest(ws, "a", boundsA)
	childWindowForTest(ws, "b", boundsB)
	ws.windowFocused("detached-request-a")

	require.NoError(t, ws.ServiceShutdown())
	ws.detachedClosingHook(a, "detached-request-a", "a")(application.NewWindowEvent())
	ws.CloseAllChildWindows()

	assert.Equal(t, prefsOf(boundsA), NewWindowService(nil).getPrefsOrDefaults("detached-request"))
}

type slowBoundsWindow struct {
	application.Window
	bounds  application.Rect
	reading chan struct{}
	release chan struct{}
}

func (w *slowBoundsWindow) Bounds() application.Rect {
	close(w.reading)
	<-w.release
	return w.bounds
}

func TestWindowService_WindowClosingAsTheAppQuitsKeepsTheLastFocusedGeometry(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	ws := NewWindowService(nil)
	slow := &slowBoundsWindow{bounds: boundsB, reading: make(chan struct{}), release: make(chan struct{})}
	b := &WindowInfo{Window: slow, Type: "detached-request", EntityID: "b"}
	ws.mu.Lock()
	ws.windows["detached-request-b"] = b
	ws.detachedRequests["b"] = "detached-request-b"
	ws.mu.Unlock()
	childWindowForTest(ws, "a", boundsA)

	closed := make(chan struct{})
	go func() {
		ws.detachedClosingHook(b, "detached-request-b", "b")(application.NewWindowEvent())
		close(closed)
	}()
	select {
	case <-slow.reading:
	case <-time.After(time.Second):
		t.Fatal("the closing window never read its bounds")
	}
	require.NoError(t, ws.ServiceShutdown())
	close(slow.release)
	<-closed

	assert.Equal(t, prefsOf(boundsA), NewWindowService(nil).getPrefsOrDefaults("detached-request"))
}

type ownerSockets struct {
	websocket.Usecase
	closed chan string
}

func (s *ownerSockets) DisconnectOwner(_ context.Context, owner string) error {
	s.closed <- owner
	return nil
}

func TestWindowService_ClosingADetachedWindowClosesItsWebSockets(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	sockets := &ownerSockets{closed: make(chan string, 1)}
	ws := NewWindowService(sockets)
	a := childWindowForTest(ws, "a", boundsA)

	ws.detachedClosingHook(a, "detached-request-a", "a")(application.NewWindowEvent())

	select {
	case owner := <-sockets.closed:
		assert.Equal(t, "detached-request-a", owner)
	case <-time.After(time.Second):
		t.Fatal("the window's websockets were left open")
	}
}

func TestWindowService_UserCloseSavesThatWindow(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	ws := NewWindowService(nil)
	a := childWindowForTest(ws, "a", boundsA)
	childWindowForTest(ws, "b", boundsB)

	ws.detachedClosingHook(a, "detached-request-a", "a")(application.NewWindowEvent())

	assert.Equal(t, prefsOf(boundsA), NewWindowService(nil).getPrefsOrDefaults("detached-request"))
}

func TestLegacyPositionsStale_MatchesPlatform(t *testing.T) {
	assert.Equal(t, runtime.GOOS == "darwin", legacyPositionsStale)
}

func TestWindowService_WorkAreas_NilApp(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	ws := NewWindowService(nil)

	assert.Empty(t, ws.workAreas())
}

func TestWindowBackground_MatchesThemePageBackground(t *testing.T) {
	assert.Equal(t, application.NewRGB(255, 255, 255), windowBackground(false))
	assert.Equal(t, application.NewRGB(15, 17, 23), windowBackground(true))
}

func TestBuildWindowOptions_UsesThemeBackground(t *testing.T) {
	prefs := &WindowPrefs{Width: 900, Height: 600}

	opts := buildWindowOptions("schema-viewer-1", "Title", "/?mode=schema-viewer", prefs, nil, windowBackground(true))

	assert.Equal(t, application.NewRGB(15, 17, 23), opts.BackgroundColour)
}

type backgroundWindow struct {
	application.Window
	colour application.RGBA
}

func (w *backgroundWindow) SetBackgroundColour(colour application.RGBA) application.Window {
	w.colour = colour
	return w
}

func TestPaintBackground_SetsEveryWindow(t *testing.T) {
	first, second := &backgroundWindow{}, &backgroundWindow{}

	paintBackground([]application.Window{first, second}, windowBackground(true))

	assert.Equal(t, application.NewRGB(15, 17, 23), first.colour)
	assert.Equal(t, application.NewRGB(15, 17, 23), second.colour)
}

func TestWindowService_ServiceStartupWithoutAppIsNoop(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	assert.NoError(t, NewWindowService(nil).ServiceStartup(context.Background(), application.ServiceOptions{}))
}

func TestDarkWindow(t *testing.T) {
	cases := []struct {
		name       string
		theme      windowTheme
		osDark     bool
		osReliable bool
		want       bool
	}{
		{"light wins over a dark OS", windowTheme{Preference: "light", LastDark: true}, true, true, false},
		{"dark wins over a light OS", windowTheme{Preference: "dark"}, false, true, true},
		{"system follows the OS", windowTheme{Preference: "system"}, true, true, true},
		{"system ignores the last theme when the OS answers", windowTheme{Preference: "system", LastDark: true}, false, true, false},
		{"never reported follows the OS", windowTheme{}, true, true, true},
		{"system on Linux uses the last theme", windowTheme{Preference: "system", LastDark: true}, false, false, true},
		{"never reported on Linux stays light", windowTheme{}, true, false, false},
		{"explicit dark on Linux", windowTheme{Preference: "dark"}, false, false, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, darkWindow(tc.theme, tc.osDark, tc.osReliable))
		})
	}
}

func setOSThemeReliable(t *testing.T, reliable bool) {
	t.Helper()
	prev := osThemeReliable
	osThemeReliable = reliable
	t.Cleanup(func() { osThemeReliable = prev })
}

func TestWindowService_Background_FollowsThemeAndOS(t *testing.T) {
	setOSThemeReliable(t, true)
	t.Setenv("HOME", t.TempDir())
	ws := NewWindowService(nil)
	osDark := true
	ws.osDark = func() bool { return osDark }

	require.Nil(t, ws.SetTheme("system", "light").Error)
	assert.Equal(t, windowBackground(true), ws.background())

	osDark = false
	assert.Equal(t, windowBackground(false), ws.background())

	require.Nil(t, ws.SetTheme("dark", "dark").Error)
	assert.Equal(t, windowBackground(true), ws.background())
}

func TestWindowService_Background_UsesLastThemeWhenOSIsUnreliable(t *testing.T) {
	setOSThemeReliable(t, false)
	t.Setenv("HOME", t.TempDir())
	ws := NewWindowService(nil)
	ws.osDark = func() bool { return false }

	require.Nil(t, ws.SetTheme("system", "dark").Error)

	assert.Equal(t, windowBackground(true), ws.background())
}

func TestWindowService_SetTheme_SurvivesRestart(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	ws := NewWindowService(nil)
	ws.mu.Lock()
	ws.prefs["schema-viewer"] = &WindowPrefs{Width: 700, Height: 500}
	ws.mu.Unlock()

	res := ws.SetTheme("system", "dark")

	require.Nil(t, res.Error)
	restarted := NewWindowService(nil)
	assert.Equal(t, windowTheme{Preference: "system", LastDark: true}, restarted.theme)
	assert.Equal(t, &WindowPrefs{Width: 700, Height: 500}, restarted.prefs["schema-viewer"])
}

func TestWindowService_SetTheme_RejectsUnknownValues(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	ws := NewWindowService(nil)

	for _, args := range [][2]string{{"sepia", "dark"}, {"dark", "system"}, {"", ""}} {
		res := ws.SetTheme(args[0], args[1])

		require.NotNil(t, res.Error, "%v", args)
		assert.Equal(t, ErrCodeValidation, res.Error.Code)
	}
	assert.Equal(t, windowTheme{}, ws.theme)
	assert.NoFileExists(t, prefsFilePath())
}
