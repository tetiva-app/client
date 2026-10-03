//go:build updatetest

package appupdate_test

import (
	"crypto/ed25519"
	"encoding/base64"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tetiva-app/client/internal/infrastructure/appupdate"
)

func TestUpdatetestConfig_KeyMatchesTestdata(t *testing.T) {
	b, err := os.ReadFile(filepath.Join("..", "..", "..", "build", "updatesign", "testdata", "test-public.b64"))
	require.NoError(t, err)
	pub, err := base64.StdEncoding.DecodeString(string(b))
	require.NoError(t, err)

	assert.Equal(t, []ed25519.PublicKey{pub}, appupdate.ReleaseKeys())
}

func TestUpdatetestConfig_ReadsDataDir(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "updatetest.json"),
		[]byte(`{"manifestUrl":"http://127.0.0.1:8765/latest.json","autoApply":true}`), 0o600))

	assert.Equal(t, "http://127.0.0.1:8765/latest.json", appupdate.ManifestURL(dir))
	assert.True(t, appupdate.AutoApply(dir))
	assert.True(t, appupdate.AllowLoopbackHTTP())
	assert.Empty(t, appupdate.ManifestURL(t.TempDir()))
	assert.False(t, appupdate.AutoApply(t.TempDir()))
}
