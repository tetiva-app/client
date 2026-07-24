package domain

import "fmt"

// ValidationError represents a validation failure with per-field details.
type ValidationError struct {
	Fields map[string]string
}

func (e *ValidationError) Error() string {
	return "validation failed"
}

// NotFoundError represents an entity not found by ID.
type NotFoundError struct {
	Entity string
	ID     string
}

func (e *NotFoundError) Error() string {
	return fmt.Sprintf("%s not found: %s", e.Entity, e.ID)
}

// ConflictError represents an optimistic locking version conflict.
type ConflictError struct {
	Entity string
	ID     string
}

func (e *ConflictError) Error() string {
	return fmt.Sprintf("%s version conflict: %s", e.Entity, e.ID)
}
