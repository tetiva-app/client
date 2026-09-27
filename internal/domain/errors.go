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

// ReasonError carries a stable machine-readable reason the UI branches on.
type ReasonError struct {
	Reason string
	Err    error
}

func (e *ReasonError) Error() string {
	return e.Err.Error()
}

func (e *ReasonError) Unwrap() error {
	return e.Err
}
