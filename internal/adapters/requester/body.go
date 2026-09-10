package requester

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"

	"github.com/tetiva-app/client/internal/domain"
)

// maxSpooledBody caps the in-memory copy Digest retries and SigV4 hashing need.
const maxSpooledBody = 32 << 20

// emptyPayloadHash is SHA-256 of the empty string, the SigV4 hash for a bodyless request.
const emptyPayloadHash = "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"

// The in-memory copy is what a Digest retry replays and SigV4 hashes; reading limit+1
// bytes separates "at the limit" from "over".
func spoolBody(r io.Reader, limit int64) (*bytes.Reader, error) {
	const funcName = "requester.spoolBody"

	buf, err := io.ReadAll(io.LimitReader(r, limit+1))
	if err != nil {
		return nil, fmt.Errorf("%s: %w", funcName, err)
	}
	if int64(len(buf)) > limit {
		return nil, &domain.ValidationError{Fields: map[string]string{
			"body": fmt.Sprintf("Digest and AWS SigV4 need the whole body in memory; %s limit", limitText(limit)),
		}}
	}
	return bytes.NewReader(buf), nil
}

func limitText(limit int64) string {
	if limit >= 1<<20 {
		return fmt.Sprintf("%d MiB", limit>>20)
	}
	return fmt.Sprintf("%d bytes", limit)
}

// Seeking to the start and reading an in-memory reader cannot fail.
func hashPayload(r *bytes.Reader) string {
	h := sha256.New()
	_, _ = r.Seek(0, io.SeekStart)
	_, _ = io.Copy(h, r)
	_, _ = r.Seek(0, io.SeekStart)
	return hex.EncodeToString(h.Sum(nil))
}
