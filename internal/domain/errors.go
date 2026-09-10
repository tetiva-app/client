package domain

import "fmt"

type ValidationError struct {
	Fields map[string]string
}

func (e *ValidationError) Error() string {
	return "validation failed"
}

type NotFoundError struct {
	Entity string
	ID     string
}

func (e *NotFoundError) Error() string {
	return fmt.Sprintf("%s not found: %s", e.Entity, e.ID)
}

type ConflictError struct {
	Entity string
	ID     string
}

func (e *ConflictError) Error() string {
	return fmt.Sprintf("%s version conflict: %s", e.Entity, e.ID)
}
