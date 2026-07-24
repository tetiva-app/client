package entities

import (
	"time"

	"github.com/google/uuid"
)

// History represents an immutable record of a request execution.
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
	CreatedAt       time.Time
}
