package entities

import (
	"time"

	"github.com/google/uuid"
)

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
