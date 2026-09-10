package auth

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
)

// ConfigHash fingerprints the acquisition configuration. A stored token may be reused only while this
// value matches, so switching environment, client or scope switches tokens too.
func ConfigHash(cfg OAuth2Config) string {
	// The struct's field order is the canonical form; a struct of strings cannot
	// fail to marshal.
	canonical, _ := json.Marshal(cfg)
	sum := sha256.Sum256(canonical)

	return hex.EncodeToString(sum[:])
}
