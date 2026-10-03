//go:build !updatetest

package appupdate_test

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tetiva-app/client/internal/infrastructure/appupdate"
)

func TestReleaseConfig(t *testing.T) {
	assert.Equal(t, "https://api.tetiva.app/updates/latest.json", appupdate.ManifestURL(""))
	assert.False(t, appupdate.AutoApply(""))
	assert.False(t, appupdate.AllowLoopbackHTTP())
}

func TestReleaseKeyVerifiesFixture(t *testing.T) {
	require.NotEmpty(t, appupdate.ReleaseKeys())
	raw, err := os.ReadFile("testdata/release-signed.json")
	require.NoError(t, err)
	_, err = appupdate.Codec{Keys: appupdate.ReleaseKeys()}.Parse(raw)
	require.NoError(t, err)
}
