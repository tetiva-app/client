package snapshotjson

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"

	"github.com/tetiva-app/client/internal/domain/usecase/publication"
)

type jsonHashed struct {
	Format      string           `json:"format"`
	Version     int              `json:"version"`
	Collection  jsonCollection   `json:"collection"`
	Environment *jsonEnvironment `json:"environment"`
}

// ContentHash hashes the canonical JSON without the generator and locale keys.
func ContentHash(s *publication.Snapshot) (string, error) {
	const funcName = "snapshotjson.ContentHash"

	full := toJSON(s)
	out, err := encode(jsonHashed{Format: full.Format, Version: full.Version, Collection: full.Collection, Environment: full.Environment})
	if err != nil {
		return "", fmt.Errorf("%s: %w", funcName, err)
	}
	sum := sha256.Sum256(out)
	return hex.EncodeToString(sum[:]), nil
}
