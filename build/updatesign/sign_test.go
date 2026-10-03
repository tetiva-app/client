package main

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tetiva-app/client/internal/domain/entities"
	infraupdate "github.com/tetiva-app/client/internal/infrastructure/appupdate"
)

type memKeys struct{ key string }

func (m *memKeys) Save(key string) error {
	m.key = key
	return nil
}

func (m *memKeys) Load() (string, error) {
	if m.key == "" {
		return "", errors.New("no key")
	}
	return m.key, nil
}

func newTool(t *testing.T) (tool, ed25519.PublicKey) {
	t.Helper()
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	return tool{
		keys:       &memKeys{key: base64.StdEncoding.EncodeToString(priv)},
		publicKeys: []ed25519.PublicKey{pub},
		client:     http.DefaultClient,
		stdout:     io.Discard,
		stderr:     io.Discard,
	}, pub
}

var artifactNames = map[string]string{
	"Tetiva-1.2.2-macos-universal.zip":         "darwin/universal/zip",
	"Tetiva-1.2.2-windows-amd64-installer.exe": "windows/amd64/nsis",
	"Tetiva-1.2.2-windows-arm64-installer.exe": "windows/arm64/nsis",
}

func writeArtifacts(t *testing.T, names ...string) string {
	t.Helper()
	dir := t.TempDir()
	for _, n := range names {
		require.NoError(t, os.WriteFile(filepath.Join(dir, n), []byte("contents of "+n), 0o644))
	}
	return dir
}

func allArtifacts(t *testing.T) string {
	t.Helper()
	names := make([]string, 0, len(artifactNames))
	for n := range artifactNames {
		names = append(names, n)
	}
	return writeArtifacts(t, names...)
}

func signTo(t *testing.T, tl tool, dir string, extra ...string) string {
	t.Helper()
	out := filepath.Join(t.TempDir(), "latest.json")
	args := append([]string{"sign", "--version", "1.2.2", "--dir", dir, "--out", out}, extra...)
	require.NoError(t, tl.run(args))
	return out
}

func readManifest(t *testing.T, file string) (top map[string]any, payload []byte) {
	t.Helper()
	raw, err := os.ReadFile(file)
	require.NoError(t, err)
	require.NoError(t, json.Unmarshal(raw, &top))
	payload, err = base64.StdEncoding.DecodeString(top["payload"].(string))
	require.NoError(t, err)
	return top, payload
}

func parse(t *testing.T, file string, pub ed25519.PublicKey) entities.Release {
	t.Helper()
	raw, err := os.ReadFile(file)
	require.NoError(t, err)
	rel, err := infraupdate.Codec{Keys: []ed25519.PublicKey{pub}}.Parse(raw)
	require.NoError(t, err)
	return rel
}

func TestSign_ManifestParsesWithMatchingKey(t *testing.T) {
	tl, pub := newTool(t)
	dir := allArtifacts(t)

	out := signTo(t, tl, dir)

	rel := parse(t, out, pub)
	assert.Equal(t, "1.2.2", rel.Version)
	assert.Empty(t, rel.InAppDisabled)
	require.Len(t, rel.Artifacts, 3)
	for _, a := range rel.Artifacts {
		name := strings.TrimPrefix(a.URL, "https://s3.twcstorage.ru/ccquota/releases/")
		content := []byte("contents of " + name)
		sum := sha256.Sum256(content)
		assert.Equal(t, artifactNames[name], a.OS+"/"+a.Arch+"/"+a.Format, name)
		assert.Equal(t, int64(len(content)), a.Size, name)
		assert.Equal(t, hex.EncodeToString(sum[:]), a.SHA256, name)
	}
	top, _ := readManifest(t, out)
	assert.Equal(t, "1.2.2", top["version"])
}

func TestSign_SkipsMissingArtifacts(t *testing.T) {
	tl, pub := newTool(t)

	out := signTo(t, tl, writeArtifacts(t, "Tetiva-1.2.2-windows-amd64-installer.exe"))

	rel := parse(t, out, pub)
	require.Len(t, rel.Artifacts, 1)
	assert.Equal(t, "amd64", rel.Artifacts[0].Arch)
}

func TestSign_NoArtifacts(t *testing.T) {
	tl, _ := newTool(t)
	out := filepath.Join(t.TempDir(), "latest.json")

	err := tl.run([]string{"sign", "--version", "1.2.2", "--dir", writeArtifacts(t, "Tetiva-1.2.1-macos-universal.zip"), "--out", out})

	require.Error(t, err)
	assert.NoFileExists(t, out)
}

func TestSign_LegacyVersion(t *testing.T) {
	tl, pub := newTool(t)

	out := signTo(t, tl, allArtifacts(t), "--legacy-version", "1.2.0")

	top, _ := readManifest(t, out)
	assert.Equal(t, "1.2.0", top["version"])
	assert.Equal(t, "1.2.2", parse(t, out, pub).Version)
}

func TestSign_DisableTrimsSpaces(t *testing.T) {
	tl, pub := newTool(t)

	out := signTo(t, tl, allArtifacts(t), "--disable", " 1.2.1, 1.2.3 ")

	assert.Equal(t, []string{"1.2.1", "1.2.3"}, parse(t, out, pub).InAppDisabled)
}

func TestSign_DisableRejectsBadVersion(t *testing.T) {
	for _, disable := range []string{"v1.2.1", "1.2.1,", "1.2", "1.2.1;1.2.3"} {
		t.Run(disable, func(t *testing.T) {
			tl, _ := newTool(t)
			out := filepath.Join(t.TempDir(), "latest.json")

			err := tl.run([]string{"sign", "--version", "1.2.2", "--dir", allArtifacts(t), "--disable", disable, "--out", out})

			require.Error(t, err)
			assert.Contains(t, err.Error(), "--disable")
			assert.NoFileExists(t, out)
		})
	}
}

func TestSign_KeyFileAndBaseURL(t *testing.T) {
	tl, _ := newTool(t)
	tl.keys = &memKeys{}
	encoded, err := os.ReadFile("testdata/test-public.b64")
	require.NoError(t, err)
	testPub, err := base64.StdEncoding.DecodeString(strings.TrimSpace(string(encoded)))
	require.NoError(t, err)

	out := signTo(t, tl, allArtifacts(t), "--key-file", "testdata/test-key.b64", "--base-url", "http://127.0.0.1:8765/")

	top, payload := readManifest(t, out)
	sig, err := base64.StdEncoding.DecodeString(top["signatures"].([]any)[0].(string))
	require.NoError(t, err)
	assert.True(t, ed25519.Verify(testPub, payload, sig))
	var p struct{ Artifacts []struct{ URL string } }
	require.NoError(t, json.Unmarshal(payload, &p))
	require.Len(t, p.Artifacts, 3)
	for _, a := range p.Artifacts {
		name := strings.TrimPrefix(a.URL, "http://127.0.0.1:8765/")
		assert.Contains(t, artifactNames, name)
	}
}

func TestVerify_Files(t *testing.T) {
	tl, _ := newTool(t)
	dir := allArtifacts(t)
	out := signTo(t, tl, dir)
	require.NoError(t, tl.run([]string{"verify", "--manifest", out, "--files", dir}))

	file := filepath.Join(dir, "Tetiva-1.2.2-windows-arm64-installer.exe")
	b, err := os.ReadFile(file)
	require.NoError(t, err)
	b[0] ^= 1
	require.NoError(t, os.WriteFile(file, b, 0o644))

	assert.Error(t, tl.run([]string{"verify", "--manifest", out, "--files", dir}))
}

func TestVerify_ArtifactDroppedByParser(t *testing.T) {
	tl, _ := newTool(t)
	priv, err := base64.StdEncoding.DecodeString(tl.keys.(*memKeys).key)
	require.NoError(t, err)
	kept := entities.Artifact{OS: "windows", Arch: "amd64", Format: "nsis", URL: "https://s3.twcstorage.ru/ccquota/releases/a.exe", Size: 1, SHA256: strings.Repeat("a", 64)}
	dropped := kept
	dropped.URL = "ftp://s3.twcstorage.ru/ccquota/releases/b.exe"
	write := func(artifacts ...entities.Artifact) string {
		signed, err := infraupdate.Sign(entities.Release{Version: "1.2.2", Artifacts: artifacts}, "", []ed25519.PrivateKey{priv})
		require.NoError(t, err)
		out := filepath.Join(t.TempDir(), "latest.json")
		require.NoError(t, os.WriteFile(out, signed, 0o644))
		return out
	}

	require.NoError(t, tl.run([]string{"verify", "--manifest", write(kept)}))
	assert.Error(t, tl.run([]string{"verify", "--manifest", write(kept, dropped)}))
}

func TestVerify_Remote(t *testing.T) {
	tl, _ := newTool(t)
	dir := allArtifacts(t)
	srv := httptest.NewTLSServer(http.FileServer(http.Dir(dir)))
	defer srv.Close()
	tl.client = srv.Client()
	out := signTo(t, tl, dir, "--base-url", srv.URL+"/")
	require.NoError(t, tl.run([]string{"verify", "--manifest", out, "--remote"}))

	f, err := os.OpenFile(filepath.Join(dir, "Tetiva-1.2.2-macos-universal.zip"), os.O_APPEND|os.O_WRONLY, 0)
	require.NoError(t, err)
	_, err = f.Write([]byte("x"))
	require.NoError(t, err)
	require.NoError(t, f.Close())

	assert.Error(t, tl.run([]string{"verify", "--manifest", out, "--remote"}))
}

func TestKeygen(t *testing.T) {
	tl, _ := newTool(t)
	keys := &memKeys{}
	tl.keys = keys
	var buf bytes.Buffer
	tl.stdout = &buf
	t.Chdir(t.TempDir())
	require.NoError(t, os.MkdirAll("internal/infrastructure/appupdate", 0o755))

	require.NoError(t, tl.run([]string{"keygen"}))

	printed := strings.TrimSpace(buf.String())
	require.True(t, strings.HasPrefix(printed, `var releaseKeys = []string{"`), printed)
	pub, err := base64.StdEncoding.DecodeString(strings.TrimSuffix(strings.TrimPrefix(printed, `var releaseKeys = []string{"`), `"}`))
	require.NoError(t, err)
	parse(t, "internal/infrastructure/appupdate/testdata/release-signed.json", pub)
	priv, err := base64.StdEncoding.DecodeString(keys.key)
	require.NoError(t, err)
	assert.Equal(t, ed25519.PublicKey(pub), ed25519.PrivateKey(priv).Public())
}
