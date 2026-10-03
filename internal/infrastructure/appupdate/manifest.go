package appupdate

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/tetiva-app/client/internal/domain/entities"
	"github.com/tetiva-app/client/internal/domain/usecase/appupdate"
)

const (
	MaxManifestBytes = 64 << 10
	MaxArtifactBytes = 512 << 20
)

const fallbackURL = "https://tetiva.app/download"

type Codec struct{ Keys []ed25519.PublicKey }

func (c Codec) Parse(raw []byte) (entities.Release, error) {
	const funcName = "appupdate.Codec.Parse"
	if len(raw) > MaxManifestBytes {
		return entities.Release{}, fmt.Errorf("%s: manifest too large", funcName)
	}
	var m manifestDTO
	if err := json.Unmarshal(raw, &m); err != nil {
		return entities.Release{}, fmt.Errorf("%s: %w", funcName, err)
	}
	payload, err := base64.StdEncoding.DecodeString(m.Payload)
	if err != nil || len(payload) == 0 {
		return entities.Release{}, fmt.Errorf("%s: bad payload encoding", funcName)
	}
	if !c.verified(payload, m.Signatures) {
		return entities.Release{}, fmt.Errorf("%s: no valid signature", funcName)
	}
	var p payloadDTO
	if err := json.Unmarshal(payload, &p); err != nil {
		return entities.Release{}, fmt.Errorf("%s: %w", funcName, err)
	}
	if !appupdate.ValidVersion(p.Version) {
		return entities.Release{}, fmt.Errorf("%s: bad version %q", funcName, p.Version)
	}
	rel := entities.Release{Version: p.Version, PublishedAt: p.PublishedAt, InAppDisabled: p.InAppDisabled}
	for _, a := range p.Artifacts {
		if !allowedURL(a.URL) || a.Size <= 0 || a.Size > MaxArtifactBytes || len(a.SHA256) != 64 {
			continue
		}
		rel.Artifacts = append(rel.Artifacts, entities.Artifact{OS: a.OS, Arch: a.Arch, Format: a.Format, URL: a.URL, Size: a.Size, SHA256: strings.ToLower(a.SHA256)})
	}
	return rel, nil
}

func allowedURL(u string) bool {
	return strings.HasPrefix(u, "https://") || AllowLoopbackHTTP() && strings.HasPrefix(u, "http://127.0.0.1")
}

func (c Codec) verified(payload []byte, signatures []string) bool {
	for _, s := range signatures {
		sig, err := base64.StdEncoding.DecodeString(s)
		if err != nil {
			continue
		}
		for _, k := range c.Keys {
			if ed25519.Verify(k, payload, sig) {
				return true
			}
		}
	}
	return false
}

// Sign signs with every key, so a client that knows only an older key still verifies it.
func Sign(r entities.Release, legacyVersion string, keys []ed25519.PrivateKey) ([]byte, error) {
	const funcName = "appupdate.Sign"
	if len(keys) == 0 {
		return nil, fmt.Errorf("%s: no signing keys", funcName)
	}
	p := payloadDTO{
		Version:       r.Version,
		PublishedAt:   r.PublishedAt,
		InAppDisabled: append([]string{}, r.InAppDisabled...),
		Artifacts:     make([]artifactDTO, 0, len(r.Artifacts)),
	}
	for _, a := range r.Artifacts {
		p.Artifacts = append(p.Artifacts, artifactDTO{OS: a.OS, Arch: a.Arch, Format: a.Format, URL: a.URL, Size: a.Size, SHA256: a.SHA256})
	}
	payload, err := json.Marshal(p)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", funcName, err)
	}
	m := manifestDTO{
		Version: r.Version,
		URL:     fallbackURL,
		Payload: base64.StdEncoding.EncodeToString(payload),
	}
	if legacyVersion != "" {
		m.Version = legacyVersion
	}
	for _, k := range keys {
		m.Signatures = append(m.Signatures, base64.StdEncoding.EncodeToString(ed25519.Sign(k, payload)))
	}
	out, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("%s: %w", funcName, err)
	}
	return append(out, '\n'), nil
}

func decodeKeys(encoded ...string) []ed25519.PublicKey {
	keys := make([]ed25519.PublicKey, 0, len(encoded))
	for _, e := range encoded {
		k, err := base64.StdEncoding.DecodeString(e)
		if err != nil || len(k) != ed25519.PublicKeySize {
			panic("appupdate: malformed compiled-in public key")
		}
		keys = append(keys, k)
	}
	return keys
}
