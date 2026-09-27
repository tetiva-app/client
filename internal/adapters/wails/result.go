package wails

import (
	"context"
	"errors"

	"google.golang.org/genproto/googleapis/rpc/errdetails"
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
	Reason  string            `json:"reason,omitempty"`
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

// ReasonServerUnreachable labels a publication or public link failure the user can only retry.
const ReasonServerUnreachable = "SERVER_UNREACHABLE"

func OK[T any](data T) Result[T] {
	return Result[T]{Data: data}
}

// Err maps domain errors to ResultError codes.
func Err[T any](err error) Result[T] {
	res := resultError(err)
	res.Reason = errorReason(err)
	var zero T
	return Result[T]{Data: zero, Error: res}
}

func resultError(err error) *ResultError {
	var valErr *domain.ValidationError
	var notFoundErr *domain.NotFoundError
	var conflictErr *domain.ConflictError

	switch {
	case errors.As(err, &valErr):
		return &ResultError{
			Code:    ErrCodeValidation,
			Message: valErr.Error(),
			Fields:  valErr.Fields,
		}
	case errors.As(err, &notFoundErr):
		return &ResultError{
			Code:    ErrCodeNotFound,
			Message: notFoundErr.Error(),
		}
	case errors.As(err, &conflictErr):
		return &ResultError{
			Code:    ErrCodeConflict,
			Message: conflictErr.Error(),
		}
	case errors.Is(err, ErrNotConnected):
		return &ResultError{
			Code:    ErrCodeNotConnected,
			Message: err.Error(),
		}
	// Above ResourceExhausted on purpose: discovery refused by a rate limit is
	// still "no answer to branch on", and rate_limited would hide the Retry.
	case errors.Is(err, ErrServerUnreachable):
		return &ResultError{
			Code:    ErrCodeServerUnreachable,
			Message: err.Error(),
		}
	case status.Code(err) == codes.ResourceExhausted:
		return &ResultError{
			Code:    ErrCodeRateLimited,
			Message: err.Error(),
		}
	default:
		return &ResultError{
			Code:    ErrCodeInternal,
			Message: err.Error(),
		}
	}
}

// unreachable gives a failed connection, a timeout or a missing session transport ReasonServerUnreachable,
// unless the server already named a reason.
func unreachable(err error) error {
	if err == nil || errorReason(err) != "" {
		return err
	}
	code := status.Code(err)
	if errors.Is(err, ErrNotConnected) || errors.Is(err, ErrServerUnreachable) || errors.Is(err, context.DeadlineExceeded) ||
		code == codes.Unavailable || code == codes.DeadlineExceeded {
		return &domain.ReasonError{Reason: ReasonServerUnreachable, Err: err}
	}
	return err
}

// errorReason prefers a client-side reason over the server's, so a wrapped RPC
// failure can be re-labelled by the caller.
func errorReason(err error) string {
	var reasonErr *domain.ReasonError
	if errors.As(err, &reasonErr) {
		return reasonErr.Reason
	}
	st, ok := status.FromError(err)
	if !ok {
		return ""
	}
	for _, detail := range st.Details() {
		if info, ok := detail.(*errdetails.ErrorInfo); ok {
			return info.GetReason()
		}
	}
	return ""
}
