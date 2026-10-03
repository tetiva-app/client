package main

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/tetiva-app/client/internal/domain/entities"
	infraupdate "github.com/tetiva-app/client/internal/infrastructure/appupdate"
)

const (
	keychainAccount = "tetiva"
	keychainService = "tetiva-update-signing"
	releaseSigned   = "internal/infrastructure/appupdate/testdata/release-signed.json"
)

type keyStore interface {
	Save(privateKey string) error
	Load() (string, error)
}

type keychain struct{}

// Save passes -T "" so no app is trusted and macOS asks the owner on every read.
func (keychain) Save(privateKey string) error {
	const funcName = "keychain.Save"
	out, err := exec.Command("security", "add-generic-password", "-a", keychainAccount, "-s", keychainService, "-T", "", "-w", privateKey).CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s: %w: %s", funcName, err, strings.TrimSpace(string(out)))
	}
	return nil
}

func (keychain) Load() (string, error) {
	const funcName = "keychain.Load"
	cmd := exec.Command("security", "find-generic-password", "-a", keychainAccount, "-s", keychainService, "-w")
	cmd.Stderr = os.Stderr
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("%s: %w", funcName, err)
	}
	return strings.TrimSpace(string(out)), nil
}

func (t tool) keygen() error {
	const funcName = "updatesign.keygen"
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return fmt.Errorf("%s: %w", funcName, err)
	}
	signed, err := infraupdate.Sign(entities.Release{Version: "0.0.1", PublishedAt: time.Now().UTC().Truncate(time.Second)}, "", []ed25519.PrivateKey{priv})
	if err != nil {
		return fmt.Errorf("%s: %w", funcName, err)
	}
	// Mkdir, not MkdirAll: outside the module root it fails before the keychain is touched.
	if err := os.Mkdir(filepath.Dir(releaseSigned), 0o755); err != nil && !os.IsExist(err) {
		return fmt.Errorf("%s: %w", funcName, err)
	}
	if err := t.keys.Save(base64.StdEncoding.EncodeToString(priv)); err != nil {
		return fmt.Errorf("%s: %w", funcName, err)
	}
	if err := os.WriteFile(releaseSigned, signed, 0o644); err != nil {
		return fmt.Errorf("%s: %w", funcName, err)
	}
	_, _ = fmt.Fprintf(t.stdout, "var releaseKeys = []string{%q}\n", base64.StdEncoding.EncodeToString(pub))
	_, _ = fmt.Fprintf(t.stderr, "key saved to the keychain as %s; wrote %s\n", keychainService, releaseSigned)
	return nil
}
