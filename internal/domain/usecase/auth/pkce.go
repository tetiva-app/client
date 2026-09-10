package auth

import (
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"io"
)

// RFC 7636 §4.1 allows a verifier of 43..128 unreserved characters; 64 random
// bytes encode to 86. The state is the 32 bytes RFC 6819 §5.3.5 asks for.
const (
	verifierBytes = 64
	stateBytes    = 32
)

// newPKCE returns a fresh verifier and its S256 challenge. Only the challenge
// travels to the IdP; the verifier never leaves the flow manager.
func newPKCE(rand io.Reader) (string, string, error) {
	const funcName = "auth.newPKCE"

	verifier, err := randomURLSafe(rand, verifierBytes)
	if err != nil {
		return "", "", fmt.Errorf("%s: %w", funcName, err)
	}
	sum := sha256.Sum256([]byte(verifier))

	return verifier, base64.RawURLEncoding.EncodeToString(sum[:]), nil
}

// newState returns the single-use value that binds a callback to its flow.
func newState(rand io.Reader) (string, error) {
	const funcName = "auth.newState"

	state, err := randomURLSafe(rand, stateBytes)
	if err != nil {
		return "", fmt.Errorf("%s: %w", funcName, err)
	}

	return state, nil
}

func randomURLSafe(rand io.Reader, size int) (string, error) {
	buf := make([]byte, size)
	if _, err := io.ReadFull(rand, buf); err != nil {
		return "", err
	}

	return base64.RawURLEncoding.EncodeToString(buf), nil
}
