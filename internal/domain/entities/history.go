package entities

import (
	"time"

	"github.com/google/uuid"
)

// History is an immutable record of one request execution.
type History struct {
	ID              uuid.UUID
	RequestID       uuid.UUID
	WorkspaceID     uuid.UUID
	Protocol        Protocol
	Method          string
	URL             string
	RequestHeaders  map[string][]string
	RequestBody     string
	ResponseStatus  int
	ResponseHeaders map[string][]string
	ResponseBody    string
	ResponseSize    int64
	DurationMs      int64
	ErrorMessage    string
	// AuthQueryKeys names the query parameters auth injected into the URL, so a
	// replay draft can strip them instead of persisting a token.
	AuthQueryKeys []string
	CreatedAt     time.Time
}
