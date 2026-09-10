package entities

import (
	"time"

	"github.com/google/uuid"
)

type Collection struct {
	ID           uuid.UUID
	WorkspaceID  uuid.UUID
	ParentID     *uuid.UUID
	Name         string
	PreScript    string
	PostScript   string
	Description  string
	AuthType     AuthType
	AuthData     string
	GRPCMetadata []HeaderItem
	SortOrder    int
	Version      int
	IsDelete     bool
	CreatedBy    string
	CreatedAt    time.Time
	UpdatedBy    string
	UpdatedAt    time.Time
}
