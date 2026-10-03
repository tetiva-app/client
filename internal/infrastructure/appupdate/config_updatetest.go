//go:build updatetest

package appupdate

import (
	"crypto/ed25519"
	"encoding/json"
	"os"
	"path/filepath"
)

const testPublicKey = "OTdyImZHMazEqQVWv9Qawa99brR3HPlYBZqo4MU5bME="

func ReleaseKeys() []ed25519.PublicKey { return decodeKeys(testPublicKey) }

func ManifestURL(dataDir string) string { return readUpdatetest(dataDir).ManifestURL }

func AutoApply(dataDir string) bool { return readUpdatetest(dataDir).AutoApply }

func AllowLoopbackHTTP() bool { return true }

type updatetestDTO struct {
	ManifestURL string `json:"manifestUrl"`
	AutoApply   bool   `json:"autoApply"`
}

// A file rather than env vars, because open(1) on macOS drops the environment.
func readUpdatetest(dataDir string) updatetestDTO {
	var cfg updatetestDTO
	if b, err := os.ReadFile(filepath.Join(dataDir, "updatetest.json")); err == nil {
		_ = json.Unmarshal(b, &cfg)
	}
	return cfg
}
