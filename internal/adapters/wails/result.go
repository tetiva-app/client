package wails

import (
	"errors"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/tetiva-app/client/internal/domain"
)

// Empty is a typed placeholder for operations that return no data (Delete, Reorder).
type Empty struct{}

// Result bridges Go domain errors to structured JSON for the frontend.
type Result[T any] struct {
	Data  T            `json:"data"`
	Error *ResultError `json:"error,omitempty"`
}

type ResultError struct {
	Code    string            `json:"code"`
	Message string            `json:"message"`
	Fields  map[string]string `json:"fields,omitempty"`
}

const (
	ErrCodeValidation   = "validation"
	ErrCodeNotFound     = "not_found"
	ErrCodeConflict     = "conflict"
	ErrCodeRateLimited  = "rate_limited"
	ErrCodeNotConnected = "not_connected"
	// ErrCodeServerUnreachable lets the connect modal offer Retry instead of
	// falling back to the in-app form.
	ErrCodeServerUnreachable = "server_unreachable"
	ErrCodeInternal          = "internal"
)

// ErrNotConnected marks the sync RPCs the UI has to explain differently:
// the account is signed in, the transport is not up.
var ErrNotConnected = errors.New("not connected to sync server")

// ErrServerUnreachable is discovery that never reached a usable server.
var ErrServerUnreachable = errors.New("cannot reach the sync server")

func OK[T any](data T) Result[T] {
	return Result[T]{Data: data}
}

// Err maps domain errors to ResultError codes.
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
	case errors.Is(err, ErrNotConnected):
		return Result[T]{Data: zero, Error: &ResultError{
			Code:    ErrCodeNotConnected,
			Message: err.Error(),
		}}
	// Above ResourceExhausted on purpose: discovery refused by a rate limit is
	// still "no answer to branch on", and rate_limited would hide the Retry.
	case errors.Is(err, ErrServerUnreachable):
		return Result[T]{Data: zero, Error: &ResultError{
			Code:    ErrCodeServerUnreachable,
			Message: err.Error(),
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
