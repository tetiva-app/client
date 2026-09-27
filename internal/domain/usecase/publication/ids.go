package publication

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"

	"github.com/google/uuid"
)

// OpaqueID hides the UUIDs: whoever knows them can push those ids first and break sync.
func OpaqueID(collectionID, entityID uuid.UUID) string {
	mac := hmac.New(sha256.New, []byte(collectionID.String()))
	mac.Write([]byte(entityID.String()))
	return hex.EncodeToString(mac.Sum(nil))[:12]
}
