package wails

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
