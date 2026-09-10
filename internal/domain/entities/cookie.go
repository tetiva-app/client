package entities

import (
	"time"

	"github.com/google/uuid"
)

type Cookie struct {
	ID          uuid.UUID
	WorkspaceID uuid.UUID
	Domain      string // normalized lowercase host
	HostOnly    bool   // true = match exact host only (no Domain= attr in Set-Cookie)
	Path        string
	Name        string
	Value       string
	ExpiresAt   *time.Time // nil = session cookie (kept until manually deleted)
	HTTPOnly    bool
	Secure      bool
	SameSite    string // "", "Lax", "Strict", "None"
	CreatedAt   time.Time
	UpdatedAt   time.Time
}
