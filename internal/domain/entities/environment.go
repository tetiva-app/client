package entities

import (
	"time"

	"github.com/google/uuid"
)

// Environment groups variables for a specific deployment target (dev, staging, prod).
type Environment struct {
	ID          uuid.UUID
	WorkspaceID uuid.UUID
	Name        string
	IsActive    bool
	Version     int
	IsDelete    bool
	CreatedBy   string
	CreatedAt   time.Time
	UpdatedBy   string
	UpdatedAt   time.Time
}

// Variable is a key-value pair within an environment.
type Variable struct {
	ID            uuid.UUID
	EnvironmentID uuid.UUID
	Key           string
	Value         string
	IsSecret      bool
	Enabled       bool
	SortOrder     int
	Version       int
	IsDelete      bool
	CreatedBy     string
	CreatedAt     time.Time
	UpdatedBy     string
	UpdatedAt     time.Time
}
