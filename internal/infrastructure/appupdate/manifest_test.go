package appupdate_test

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tetiva-app/client/internal/domain/entities"
	"github.com/tetiva-app/client/internal/infrastructure/appupdate"
)

var testSHA = strings.Repeat("ab", 32)

func newKey(t *testing.T) ed25519.PrivateKey {
	t.Helper()
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	return priv
}

func publicOf(priv ed25519.PrivateKey) ed25519.PublicKey {
	return priv.Public().(ed25519.PublicKey)
}

func payloadBytes(t *testing.T, payload any) []byte {
	t.Helper()
	if b, ok := payload.([]byte); ok {
		return b
	}
	b, err := json.Marshal(payload)
	require.NoError(t, err)
	return b
}

func manifest(t *testing.T, payload []byte, signatures ...string) []byte {
	t.Helper()
	b, err := json.Marshal(map[string]any{
		"version":    "1.2.2",
		"url":        "https://tetiva.app/download",
		"payload":    base64.StdEncoding.EncodeToString(payload),
		"signatures": signatures,
	})
	require.NoError(t, err)
	return b
}

func sig(priv ed25519.PrivateKey, payload []byte) string {
	return base64.StdEncoding.EncodeToString(ed25519.Sign(priv, payload))
}

func signed(t *testing.T, priv ed25519.PrivateKey, payload any) []byte {
	t.Helper()
	b := payloadBytes(t, payload)
	return manifest(t, b, sig(priv, b))
}

func artifact(overrides map[string]any) map[string]any {
	a := map[string]any{
		"os":     "darwin",
		"arch":   "universal",
		"format": "zip",
		"url":    "https://s3.twcstorage.ru/ccquota/releases/Tetiva-1.2.2-macos-universal.zip",
		"size":   41234567,
		"sha256": testSHA,
	}
	for k, v := range overrides {
		a[k] = v
	}
	return a
}

func payload(artifacts ...map[string]any) map[string]any {
	return map[string]any{
		"version":       "1.2.2",
		"publishedAt":   "2026-10-20T12:00:00Z",
		"inAppDisabled": []string{"1.2.1"},
		"artifacts":     artifacts,
	}
}

func TestParse_Valid(t *testing.T) {
	priv := newKey(t)
	codec := appupdate.Codec{Keys: []ed25519.PublicKey{publicOf(priv)}}

	rel, err := codec.Parse(signed(t, priv, payload(
		artifact(nil),
		artifact(map[string]any{"os": "windows", "arch": "amd64", "format": "nsis",
			"url": "https://s3.twcstorage.ru/ccquota/releases/Tetiva-1.2.2-windows-amd64-installer.exe", "size": 15234567,
			"sha256": strings.ToUpper(testSHA)}),
	)))

	require.NoError(t, err)
	assert.Equal(t, "1.2.2", rel.Version)
	assert.True(t, rel.PublishedAt.Equal(time.Date(2026, 10, 20, 12, 0, 0, 0, time.UTC)))
	assert.Equal(t, []string{"1.2.1"}, rel.InAppDisabled)
	assert.Equal(t, []entities.Artifact{
		{OS: "darwin", Arch: "universal", Format: "zip", URL: "https://s3.twcstorage.ru/ccquota/releases/Tetiva-1.2.2-macos-universal.zip", Size: 41234567, SHA256: testSHA},
		{OS: "windows", Arch: "amd64", Format: "nsis", URL: "https://s3.twcstorage.ru/ccquota/releases/Tetiva-1.2.2-windows-amd64-installer.exe", Size: 15234567, SHA256: testSHA},
	}, rel.Artifacts)
}

func TestParse_PayloadChangedAfterSigning(t *testing.T) {
	priv := newKey(t)
	codec := appupdate.Codec{Keys: []ed25519.PublicKey{publicOf(priv)}}
	b := payloadBytes(t, payload(artifact(nil)))
	s := sig(priv, b)
	tampered := []byte(strings.Replace(string(b), "1.2.2", "1.2.3", 1))
	require.Equal(t, len(b), len(tampered))

	_, err := codec.Parse(manifest(t, tampered, s))

	assert.Error(t, err)
}

func TestParse_OtherKey(t *testing.T) {
	codec := appupdate.Codec{Keys: []ed25519.PublicKey{publicOf(newKey(t))}}

	_, err := codec.Parse(signed(t, newKey(t), payload(artifact(nil))))

	assert.Error(t, err)
}

func TestParse_SecondSignatureValid(t *testing.T) {
	priv := newKey(t)
	codec := appupdate.Codec{Keys: []ed25519.PublicKey{publicOf(priv)}}
	b := payloadBytes(t, payload(artifact(nil)))

	rel, err := codec.Parse(manifest(t, b, "garbage", sig(newKey(t), b), sig(priv, b)))

	require.NoError(t, err)
	assert.Equal(t, "1.2.2", rel.Version)
}

func paddedTo(t *testing.T, priv ed25519.PrivateKey, size int) []byte {
	t.Helper()
	b := payloadBytes(t, payload(artifact(nil)))
	m := map[string]any{
		"payload":    base64.StdEncoding.EncodeToString(b),
		"signatures": []string{sig(priv, b)},
		"pad":        "",
	}
	base, err := json.Marshal(m)
	require.NoError(t, err)
	m["pad"] = strings.Repeat("x", size-len(base))
	raw, err := json.Marshal(m)
	require.NoError(t, err)
	require.Len(t, raw, size)
	return raw
}

func TestParse_SizeLimit(t *testing.T) {
	priv := newKey(t)
	codec := appupdate.Codec{Keys: []ed25519.PublicKey{publicOf(priv)}}

	_, err := codec.Parse(paddedTo(t, priv, appupdate.MaxManifestBytes))
	require.NoError(t, err)

	_, err = codec.Parse(paddedTo(t, priv, appupdate.MaxManifestBytes+1))
	assert.Error(t, err)
}

func TestParse_Malformed(t *testing.T) {
	priv := newKey(t)
	codec := appupdate.Codec{Keys: []ed25519.PublicKey{publicOf(priv)}}
	notBase64, err := json.Marshal(map[string]any{"payload": "%%%", "signatures": []string{sig(priv, []byte("%%%"))}})
	require.NoError(t, err)

	cases := map[string][]byte{
		"not json":           []byte("{"),
		"payload not base64": notBase64,
		"payload not json":   signed(t, priv, []byte("not json")),
		"empty payload":      manifest(t, nil, sig(priv, nil)),
		"no signatures":      manifest(t, payloadBytes(t, payload(artifact(nil)))),
	}
	for name, raw := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := codec.Parse(raw)
			assert.Error(t, err)
		})
	}
}

func TestParse_BadVersion(t *testing.T) {
	priv := newKey(t)
	codec := appupdate.Codec{Keys: []ed25519.PublicKey{publicOf(priv)}}
	p := payload(artifact(nil))
	p["version"] = "1.2"

	_, err := codec.Parse(signed(t, priv, p))

	assert.Error(t, err)
}

func TestParse_UnknownFieldsKept(t *testing.T) {
	priv := newKey(t)
	codec := appupdate.Codec{Keys: []ed25519.PublicKey{publicOf(priv)}}
	p := payload(artifact(map[string]any{"os": "plan9", "delta": true}))
	p["channel"] = "stable"
	raw, err := json.Marshal(map[string]any{
		"version":    "1.2.2",
		"payload":    base64.StdEncoding.EncodeToString(payloadBytes(t, p)),
		"signatures": []string{sig(priv, payloadBytes(t, p))},
		"mirror":     "https://example.com",
	})
	require.NoError(t, err)

	rel, err := codec.Parse(raw)

	require.NoError(t, err)
	require.Len(t, rel.Artifacts, 1)
	assert.Equal(t, "plan9", rel.Artifacts[0].OS)
}

func TestParse_InvalidArtifactsDropped(t *testing.T) {
	priv := newKey(t)
	codec := appupdate.Codec{Keys: []ed25519.PublicKey{publicOf(priv)}}

	rel, err := codec.Parse(signed(t, priv, payload(
		artifact(map[string]any{"url": "http://s3.twcstorage.ru/ccquota/releases/Tetiva-1.2.2-macos-universal.zip"}),
		artifact(map[string]any{"size": 0}),
		artifact(map[string]any{"size": appupdate.MaxArtifactBytes + 1}),
		artifact(map[string]any{"sha256": testSHA[:63]}),
		artifact(map[string]any{"arch": "kept"}),
	)))

	require.NoError(t, err)
	require.Len(t, rel.Artifacts, 1)
	assert.Equal(t, "kept", rel.Artifacts[0].Arch)
}

func testRelease() entities.Release {
	return entities.Release{
		Version:       "1.2.2",
		PublishedAt:   time.Date(2026, 10, 20, 12, 0, 0, 0, time.UTC),
		InAppDisabled: []string{"1.2.0"},
		Artifacts: []entities.Artifact{
			{OS: "darwin", Arch: "universal", Format: "zip", URL: "https://s3.twcstorage.ru/ccquota/releases/Tetiva-1.2.2-macos-universal.zip", Size: 41234567, SHA256: testSHA},
		},
	}
}

func TestSign_RoundTrip(t *testing.T) {
	priv := newKey(t)
	codec := appupdate.Codec{Keys: []ed25519.PublicKey{publicOf(priv)}}

	raw, err := appupdate.Sign(testRelease(), "", []ed25519.PrivateKey{priv})
	require.NoError(t, err)
	rel, err := codec.Parse(raw)

	require.NoError(t, err)
	assert.Equal(t, testRelease(), rel)
}

func TestSign_EveryKeyVerifies(t *testing.T) {
	keys := []ed25519.PrivateKey{newKey(t), newKey(t)}

	raw, err := appupdate.Sign(testRelease(), "", keys)
	require.NoError(t, err)

	for _, k := range keys {
		_, err := appupdate.Codec{Keys: []ed25519.PublicKey{publicOf(k)}}.Parse(raw)
		assert.NoError(t, err)
	}
}

func TestSign_LegacyTopLevel(t *testing.T) {
	priv := newKey(t)

	raw, err := appupdate.Sign(testRelease(), "1.2.0", []ed25519.PrivateKey{priv})
	require.NoError(t, err)

	var top map[string]any
	require.NoError(t, json.Unmarshal(raw, &top))
	assert.Equal(t, "1.2.0", top["version"])
	assert.Equal(t, "https://tetiva.app/download", top["url"])
	rel, err := appupdate.Codec{Keys: []ed25519.PublicKey{publicOf(priv)}}.Parse(raw)
	require.NoError(t, err)
	assert.Equal(t, "1.2.2", rel.Version)
}

func TestSign_TopLevelDefaultsToReleaseVersion(t *testing.T) {
	raw, err := appupdate.Sign(testRelease(), "", []ed25519.PrivateKey{newKey(t)})
	require.NoError(t, err)

	var top map[string]any
	require.NoError(t, json.Unmarshal(raw, &top))
	assert.Equal(t, "1.2.2", top["version"])
	assert.Equal(t, "https://tetiva.app/download", top["url"])
}

func TestSign_NoKeys(t *testing.T) {
	_, err := appupdate.Sign(testRelease(), "", nil)

	assert.Error(t, err)
}
