package wails

import (
	"errors"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/tetiva-app/client/internal/domain"
)

// Empty is a typed placeholder for operations that return no data (Delete, Reorder).
type Empty struct{}

// Result is a generic wrapper for Wails service responses.
// It bridges Go domain errors to structured JSON for the frontend.
type Result[T any] struct {
	Data  T            `json:"data"`
	Error *ResultError `json:"error,omitempty"`
}

// ResultError represents a structured error returned to the frontend.
type ResultError struct {
	Code    string            `json:"code"`
	Message string            `json:"message"`
	Fields  map[string]string `json:"fields,omitempty"`
}

const (
	ErrCodeValidation  = "validation"
	ErrCodeNotFound    = "not_found"
	ErrCodeConflict    = "conflict"
	ErrCodeRateLimited = "rate_limited"
	ErrCodeInternal    = "internal"
)

// OK creates a successful Result with the given data.
func OK[T any](data T) Result[T] {
	return Result[T]{Data: data}
}

// Err creates a failed Result by mapping domain errors to ResultError codes.
func Err[T any](err error) Result[T] {
	var zero T

	var valErr *domain.ValidationError
	var notFoundErr *domain.NotFoundError
	var conflictErr *domain.ConflictError

	switch {
	case errors.As(err, &valErr):
		return Result[T]{Data: zero, Error: &ResultError{
			Code:    ErrCodeValidation,
			Message: valErr.Error(),
			Fields:  valErr.Fields,
		}}
	case errors.As(err, &notFoundErr):
		return Result[T]{Data: zero, Error: &ResultError{
			Code:    ErrCodeNotFound,
			Message: notFoundErr.Error(),
		}}
	case errors.As(err, &conflictErr):
		return Result[T]{Data: zero, Error: &ResultError{
			Code:    ErrCodeConflict,
			Message: conflictErr.Error(),
		}}
	case status.Code(err) == codes.ResourceExhausted:
		return Result[T]{Data: zero, Error: &ResultError{
			Code:    ErrCodeRateLimited,
			Message: err.Error(),
		}}
	default:
		return Result[T]{Data: zero, Error: &ResultError{
			Code:    ErrCodeInternal,
			Message: err.Error(),
		}}
	}
}
