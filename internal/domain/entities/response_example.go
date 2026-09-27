package entities

import (
	"time"

	"github.com/google/uuid"
)

type ResponseExample struct {
	ID          uuid.UUID
	RequestID   uuid.UUID
	WorkspaceID uuid.UUID
	Name        string
	StatusCode  int
	StatusText  string
	Headers     []HeaderItem
	Body        string
	ContentType string
	Protocol    Protocol
	SortOrder   int
	Version     int
	IsDelete    bool
	CreatedBy   string
	CreatedAt   time.Time
	UpdatedBy   string
	UpdatedAt   time.Time
}
