package appupdate

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/tetiva-app/client/internal/domain/entities"
	"github.com/tetiva-app/client/internal/domain/usecase/appupdate"
)

func TestDarwinBundleFromExecutable(t *testing.T) {
	bundle, ok := bundleFromExecutable("/Applications/Tetiva.app/Contents/MacOS/client")
	assert.True(t, ok)
	assert.Equal(t, "/Applications/Tetiva.app", bundle)

	for _, exe := range []string{
		"/Users/me/GolandProjects/client/bin/client",
		"/Applications/Tetiva/Contents/MacOS/client",
		"/Applications/Tetiva.app/Contents/Resources/client",
	} {
		_, ok := bundleFromExecutable(exe)
		assert.False(t, ok, exe)
	}
}

func TestDarwinKindFromPath(t *testing.T) {
	writableExcept := func(denied string) func(string) bool {
		return func(p string) bool { return p != denied }
	}
	tests := []struct {
		name       string
		bundle     string
		writable   func(string) bool
		wantKind   entities.InstallKind
		wantReason string
	}{
		{
			name:       "translocated",
			bundle:     "/private/var/folders/x/AppTranslocation/ABC/d/Tetiva.app",
			writable:   writableExcept(""),
			wantKind:   entities.InstallUnsupported,
			wantReason: appupdate.ReasonTranslocated,
		},
		{
			name:       "mounted image",
			bundle:     "/Volumes/Tetiva/Tetiva.app",
			writable:   writableExcept(""),
			wantKind:   entities.InstallUnsupported,
			wantReason: appupdate.ReasonTranslocated,
		},
		{
			name:       "bundle not writable",
			bundle:     "/Applications/Tetiva.app",
			writable:   writableExcept("/Applications/Tetiva.app"),
			wantKind:   entities.InstallUnsupported,
			wantReason: appupdate.ReasonReadOnly,
		},
		{
			name:       "parent not writable",
			bundle:     "/Applications/Tetiva.app",
			writable:   writableExcept("/Applications"),
			wantKind:   entities.InstallUnsupported,
			wantReason: appupdate.ReasonReadOnly,
		},
		{
			name:     "writable",
			bundle:   "/Applications/Tetiva.app",
			writable: writableExcept(""),
			wantKind: entities.InstallInApp,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			kind, reason := darwinKindFromPath(tt.bundle, tt.writable)
			assert.Equal(t, tt.wantKind, kind)
			assert.Equal(t, tt.wantReason, reason)
		})
	}
}
