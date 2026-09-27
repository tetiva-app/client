package publication

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"

	"github.com/google/uuid"
)

// OpaqueID keeps anchors stable across revisions without revealing the UUIDs: knowing them, anyone
// could push entities with those ids first and make the author's sync fail with ID_CONFLICT.
func OpaqueID(collectionID, entityID uuid.UUID) string {
	mac := hmac.New(sha256.New, []byte(collectionID.String()))
	mac.Write([]byte(entityID.String()))
	return hex.EncodeToString(mac.Sum(nil))[:12]
}
