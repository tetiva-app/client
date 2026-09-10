package wails

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/wailsapp/wails/v3/pkg/application"

	"github.com/tetiva-app/client/internal/constants"
)

func TestWindowService_GetSchemaContent_Happy(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	ws := NewWindowService()

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
	ws := NewWindowService()

	res := ws.GetSchemaContent(uuid.New().String())

	require.NotNil(t, res.Error)
	assert.Equal(t, ErrCodeNotFound, res.Error.Code)
}

func TestWindowService_DetachRequest_Happy_AlreadyDetached(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	ws := NewWindowService()

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
	ws := NewWindowService()

	res := ws.DetachRequest(uuid.New().String(), "http", "New Window")

	require.NotNil(t, res.Error)
	assert.Equal(t, ErrCodeInternal, res.Error.Code)
	assert.Contains(t, res.Error.Message, "app not initialized")
}

// TODO: OpenSchemaViewer happy path needs a real *application.App; covered by manual testing.

func TestWindowService_OpenSchemaViewer_Error_AppNotInitialized(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	ws := NewWindowService()

	res := ws.OpenSchemaViewer("syntax=\"proto3\";", "test.proto", "Schema", "protobuf", "")

	require.NotNil(t, res.Error)
	assert.Equal(t, ErrCodeInternal, res.Error.Code)
	assert.Contains(t, res.Error.Message, "app not initialized")
}

func TestWindowService_IsDetached_Happy_True(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	ws := NewWindowService()

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
	ws := NewWindowService()

	res := ws.IsDetached(uuid.New().String())

	require.Nil(t, res.Error)
	assert.False(t, res.Data)
}

func TestWindowService_FocusDetachedWindow_Happy_NotExists(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	ws := NewWindowService()

	res := ws.FocusDetachedWindow(uuid.New().String())

	require.Nil(t, res.Error)
}

func TestWindowService_FocusDetachedWindow_Happy_Exists_NoFocus(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	ws := NewWindowService()

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

	ws := NewWindowService()

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

	ws := NewWindowService()

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

	ws := NewWindowService()

	assert.Equal(t, &WindowPrefs{Width: 1000, Height: 700}, ws.getPrefsOrDefaults("detached-request"))
	assert.Equal(t, &WindowPrefs{Width: 700, Height: 500}, ws.getPrefsOrDefaults("schema-viewer"))
}

func TestWindowService_LoadPrefs_V2Keeps(t *testing.T) {
	writePrefsFile(t, `{"version":2,"windows":{"detached-request":{"width":900,"height":600,"x":120,"y":80,"hasPosition":true}}}`)

	ws := NewWindowService()

	got := ws.getPrefsOrDefaults("detached-request")
	assert.Equal(t, &WindowPrefs{Width: 900, Height: 600, X: 120, Y: 80, HasPosition: true}, got)
}

func TestWindowService_LoadPrefs_V2SkipsZeroSize(t *testing.T) {
	writePrefsFile(t, `{"version":2,"windows":{"detached-request":{"width":0,"height":600,"hasPosition":true},"schema-viewer":{"width":700,"height":-1,"hasPosition":true}}}`)

	ws := NewWindowService()

	assert.Equal(t, &WindowPrefs{Width: 1000, Height: 700}, ws.getPrefsOrDefaults("detached-request"))
	assert.Equal(t, &WindowPrefs{Width: 700, Height: 500}, ws.getPrefsOrDefaults("schema-viewer"))
}

func TestWindowService_LoadPrefs_V2SkipsOversized(t *testing.T) {
	writePrefsFile(t, `{"version":2,"windows":{"detached-request":{"width":900,"height":600,"x":2147483647,"y":80,"hasPosition":true},"schema-viewer":{"width":99999,"height":500,"hasPosition":true}}}`)

	ws := NewWindowService()

	assert.Equal(t, &WindowPrefs{Width: 1000, Height: 700}, ws.getPrefsOrDefaults("detached-request"))
	assert.Equal(t, &WindowPrefs{Width: 700, Height: 500}, ws.getPrefsOrDefaults("schema-viewer"))
}

func TestWindowService_LoadPrefs_LegacySkipsOversized(t *testing.T) {
	setLegacyPositionsStale(t, false)
	writePrefsFile(t, `{"detached-request":{"width":900,"height":600,"x":-2147483648,"y":80}}`)

	ws := NewWindowService()

	assert.Equal(t, &WindowPrefs{Width: 1000, Height: 700}, ws.getPrefsOrDefaults("detached-request"))
}

func TestWindowService_LoadPrefs_V2NullWindows(t *testing.T) {
	writePrefsFile(t, `{"version":2,"windows":null}`)

	ws := NewWindowService()

	require.NotNil(t, ws.prefs)
	assert.Empty(t, ws.prefs)
	assert.NotPanics(t, ws.savePrefs)
}

func TestWindowService_LoadPrefs_NullEntry_Legacy(t *testing.T) {
	writePrefsFile(t, `{"detached-request":null}`)

	ws := NewWindowService()

	assert.Equal(t, &WindowPrefs{Width: 1000, Height: 700}, ws.getPrefsOrDefaults("detached-request"))
}

func TestWindowService_LoadPrefs_NullEntry_V2(t *testing.T) {
	writePrefsFile(t, `{"version":2,"windows":{"detached-request":null}}`)

	ws := NewWindowService()

	assert.Equal(t, &WindowPrefs{Width: 1000, Height: 700}, ws.getPrefsOrDefaults("detached-request"))
}

func TestWindowService_LoadPrefs_UnknownVersion(t *testing.T) {
	writePrefsFile(t, `{"version":3,"windows":{"detached-request":{"width":900,"height":600,"x":120,"y":80,"hasPosition":true}}}`)

	ws := NewWindowService()

	assert.Empty(t, ws.prefs)
	assert.Equal(t, &WindowPrefs{Width: 1000, Height: 700}, ws.getPrefsOrDefaults("detached-request"))
}

func TestWindowService_LoadPrefs_Corrupt(t *testing.T) {
	writePrefsFile(t, `{"version":2,"windows":`)

	ws := NewWindowService()

	require.NotNil(t, ws.prefs)
	assert.Empty(t, ws.prefs)
}

func TestWindowService_SavePrefs_WritesV2(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	ws := NewWindowService()

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

func TestBuildWindowOptions_NoPositionWhenNotSaved(t *testing.T) {
	areas := []application.Rect{{X: 0, Y: 0, Width: 1920, Height: 1080}}
	prefs := &WindowPrefs{Width: 900, Height: 600, X: 120, Y: 80}

	opts := buildWindowOptions("detached-request-1", "Title", "/?mode=detached-request", prefs, areas)

	assert.Equal(t, application.WindowCentered, opts.InitialPosition)
	assert.Equal(t, 0, opts.X)
	assert.Equal(t, 0, opts.Y)
	assert.Equal(t, 900, opts.Width)
	assert.Equal(t, 600, opts.Height)
}

func TestBuildWindowOptions_PositionAppliedWhenOnScreen(t *testing.T) {
	areas := []application.Rect{{X: 0, Y: 0, Width: 1920, Height: 1080}}
	prefs := &WindowPrefs{Width: 900, Height: 600, X: 120, Y: 80, HasPosition: true}

	opts := buildWindowOptions("detached-request-1", "Title", "/?mode=detached-request", prefs, areas)

	assert.Equal(t, application.WindowXY, opts.InitialPosition)
	assert.Equal(t, 120, opts.X)
	assert.Equal(t, 80, opts.Y)
}

func TestBuildWindowOptions_PositionDroppedWhenOffScreen(t *testing.T) {
	areas := []application.Rect{{X: 0, Y: 0, Width: 1920, Height: 1080}}
	prefs := &WindowPrefs{Width: 900, Height: 600, X: -9999, Y: -9999, HasPosition: true}

	opts := buildWindowOptions("detached-request-1", "Title", "/?mode=detached-request", prefs, areas)

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
		name       string
		w, h, x, y int
		want       *WindowPrefs
	}{
		{name: "valid", w: 900, h: 600, x: 120, y: 80, want: &WindowPrefs{Width: 900, Height: 600, X: 120, Y: 80, HasPosition: true}},
		{name: "negative position is fine", w: 900, h: 600, x: -1200, y: -50, want: &WindowPrefs{Width: 900, Height: 600, X: -1200, Y: -50, HasPosition: true}},
		{name: "destroyed window", w: 0, h: 0, x: 0, y: 0},
		{name: "zero width", w: 0, h: 600},
		{name: "negative height", w: 900, h: -1},
		{name: "oversized width", w: 40000, h: 600},
		{name: "oversized height", w: 900, h: 40000},
		{name: "oversized x", w: 900, h: 600, x: 40000},
		{name: "oversized negative y", w: 900, h: 600, y: -40000},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := prefsFromWindow(tt.w, tt.h, tt.x, tt.y)
			assert.Equal(t, tt.want != nil, ok)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestLegacyPositionsStale_MatchesPlatform(t *testing.T) {
	assert.Equal(t, runtime.GOOS == "darwin", legacyPositionsStale)
}

func TestWindowService_WorkAreas_NilApp(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	ws := NewWindowService()

	assert.Empty(t, ws.workAreas())
}
