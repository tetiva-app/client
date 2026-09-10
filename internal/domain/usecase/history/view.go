package history

import (
	"github.com/google/uuid"

	"github.com/tetiva-app/client/internal/domain/entities"
)

// StatusKind groups response outcomes for filtering. A row with both a status
// and an error_message is classified only as Error, never as a 2xx-5xx range.
type StatusKind string

const (
	StatusKind2xx   StatusKind = "2xx" // includes 101, the WebSocket handshake success
	StatusKind3xx   StatusKind = "3xx"
	StatusKind4xx   StatusKind = "4xx"
	StatusKind5xx   StatusKind = "5xx"
	StatusKindError StatusKind = "error" // network error, no HTTP response
)

type Filter struct {
	WorkspaceID uuid.UUID           // required
	RequestID   *uuid.UUID          // optional: limit to single request
	Protocols   []entities.Protocol // empty = any
	StatusKinds []StatusKind        // empty = any
	URLContains string              // case-insensitive substring on URL
	Limit       int                 // 0 = use default page size (200)
	Offset      int
}

type ListOpt struct {
	WorkspaceID uuid.UUID
	Filter      Filter
}

type DeleteOpt struct {
	HistoryID   uuid.UUID
	WorkspaceID uuid.UUID // protects from cross-workspace delete
}

type ClearOpt struct {
	WorkspaceID uuid.UUID
}

const DefaultPageSize = 200
