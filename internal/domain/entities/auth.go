package entities

import "github.com/google/uuid"

// Auth owner kinds; the token store keys rows by (workspace, kind, id).
const (
	AuthOwnerKindRequest    = "request"
	AuthOwnerKindCollection = "collection"
)

// AuthOwner is either the request itself or the collection an inherited config came from.
type AuthOwner struct {
	WorkspaceID uuid.UUID
	Kind        string
	ID          uuid.UUID
}
