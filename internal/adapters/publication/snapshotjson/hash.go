package snapshotjson

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"

	"github.com/tetiva-app/client/internal/domain/usecase/publication"
)

// jsonHashed is jsonSnapshot without generator and locale: a client upgrade or another author
// locale must not read as a change to the collection.
type jsonHashed struct {
	Format      string           `json:"format"`
	Version     int              `json:"version"`
	Collection  jsonCollection   `json:"collection"`
	Environment *jsonEnvironment `json:"environment"`
}

// ContentHash is the sha256 hex of the canonical JSON with the generator and locale keys removed.
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
