package entities

import (
	"time"

	"github.com/google/uuid"
)

// Workspace represents an isolated container for collections, environments, and history.
type Workspace struct {
	ID                uuid.UUID
	Name              string
	IsActive          bool
	Version           int
	IsDelete          bool
	CreatedBy         string
	CreatedAt         time.Time
	UpdatedBy         string
	UpdatedAt         time.Time
	RemoteWorkspaceID *string // nil = local workspace, non-nil = synced with remote server
}
