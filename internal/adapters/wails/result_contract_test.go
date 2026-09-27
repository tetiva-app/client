package wails

// These tests pin the Result/ResultError/Empty JSON wire format the frontend
// depends on (exact field names + omitempty); any change must be intentional.

import (
	"encoding/json"
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/tetiva-app/client/internal/adapters/wails/dto"
	"github.com/tetiva-app/client/internal/domain"
)

func TestContract_OK_String(t *testing.T) {
	b, err := json.Marshal(OK("hello"))
	require.NoError(t, err)
	require.Equal(t, `{"data":"hello"}`, string(b))
}

func TestContract_OK_EmptyStruct(t *testing.T) {
	b, err := json.Marshal(OK(struct{}{}))
	require.NoError(t, err)
	require.Equal(t, `{"data":{}}`, string(b))
}

func TestContract_OK_EmptySlice(t *testing.T) {
	// A nil slice would marshal to null — callers must construct empty slices
	// explicitly to preserve the array shape.
	b, err := json.Marshal(OK([]int{}))
	require.NoError(t, err)
	require.Equal(t, `{"data":[]}`, string(b))
}

func TestContract_Err_Validation_WithFields(t *testing.T) {
	verr := &domain.ValidationError{Fields: map[string]string{"name": "required"}}
	b, err := json.Marshal(Err[string](verr))
	require.NoError(t, err)
	require.Equal(t, `{"data":"","error":{"code":"validation","message":"validation failed","fields":{"name":"required"}}}`, string(b))
}

func TestContract_Err_NotFound_NoFields(t *testing.T) {
	nferr := &domain.NotFoundError{Entity: "booking", ID: "abc-123"}
	b, err := json.Marshal(Err[int](nferr))
	require.NoError(t, err)

	got := string(b)
	require.Equal(t, `{"data":0,"error":{"code":"not_found","message":"booking not found: abc-123"}}`, got)
	assert.NotContains(t, got, `"fields"`, `"fields" must be omitted when empty (omitempty)`)
}

func TestContract_Err_Conflict(t *testing.T) {
	cerr := &domain.ConflictError{Entity: "booking", ID: "abc-123"}
	b, err := json.Marshal(Err[string](cerr))
	require.NoError(t, err)

	got := string(b)
	require.Equal(t, `{"data":"","error":{"code":"conflict","message":"booking version conflict: abc-123"}}`, got)
	assert.NotContains(t, got, `"fields"`)
}

func TestContract_Err_NotConnected(t *testing.T) {
	b, err := json.Marshal(Err[string](fmt.Errorf("listSessions: %w", ErrNotConnected)))
	require.NoError(t, err)
	require.Equal(t, `{"data":"","error":{"code":"not_connected","message":"listSessions: not connected to sync server"}}`, string(b))
}

func TestContract_Err_ServerUnreachable(t *testing.T) {
	b, err := json.Marshal(Err[string](fmt.Errorf("getServerCapabilities: %w", ErrServerUnreachable)))
	require.NoError(t, err)
	require.Equal(t,
		`{"data":"","error":{"code":"server_unreachable","message":"getServerCapabilities: cannot reach the sync server"}}`,
		string(b))
}

func TestContract_Err_ServerUnreachable_WinsOverRateLimited(t *testing.T) {
	// Discovery refused by a rate limit is still no answer to branch on; the
	// modal must offer Retry instead of the in-app form.
	err := fmt.Errorf("%w: %w", ErrServerUnreachable, status.Error(codes.ResourceExhausted, "slow down"))
	res := Err[string](err)
	require.NotNil(t, res.Error)
	require.Equal(t, ErrCodeServerUnreachable, res.Error.Code)
}

func statusWithReason(t *testing.T, code codes.Code, reason string) error {
	t.Helper()
	st, err := status.New(code, "refused").WithDetails(&errdetails.ErrorInfo{Reason: reason, Domain: "tetiva.app"})
	require.NoError(t, err)
	return st.Err()
}

func TestContract_Err_ReasonFromErrorInfo(t *testing.T) {
	reasons := []string{
		"PUBLISH_QUOTA_EXCEEDED", "PUBLISH_FEATURE_REQUIRED", "PUBLISH_BLOCKED", "PUBLISH_SUSPENDED",
		"PUBLISH_COLLECTION_NOT_SYNCED", "PUBLISH_CONFIRM_REQUIRED", "PUBLISH_CONFLICT", "PUBLISH_DISABLED",
		"PUBLISH_EMAIL_UNVERIFIED", "SNAPSHOT_TOO_LARGE", "SNAPSHOT_INVALID", "PASSWORD_INVALID", "RATE_LIMITED",
	}
	for _, reason := range reasons {
		t.Run(reason, func(t *testing.T) {
			res := Err[string](fmt.Errorf("publish: %w", statusWithReason(t, codes.FailedPrecondition, reason)))
			require.NotNil(t, res.Error)
			assert.Equal(t, ErrCodeInternal, res.Error.Code)
			assert.Equal(t, reason, res.Error.Reason)
		})
	}
}

func TestContract_Err_ReasonWireFormat(t *testing.T) {
	err := fmt.Errorf("publish: %w", statusWithReason(t, codes.ResourceExhausted, "PUBLISH_QUOTA_EXCEEDED"))
	b, mErr := json.Marshal(Err[string](err))
	require.NoError(t, mErr)
	require.Equal(t,
		`{"data":"","error":{"code":"rate_limited","message":"publish: rpc error: code = ResourceExhausted desc = refused","reason":"PUBLISH_QUOTA_EXCEEDED"}}`,
		string(b))
}

func TestContract_Err_ReasonSitsBeforeFields(t *testing.T) {
	err := &domain.ReasonError{Reason: "LINK_INVALID", Err: &domain.ValidationError{Fields: map[string]string{"slug": "invalid"}}}
	b, mErr := json.Marshal(Err[string](err))
	require.NoError(t, mErr)
	require.Equal(t,
		`{"data":"","error":{"code":"validation","message":"validation failed","reason":"LINK_INVALID","fields":{"slug":"invalid"}}}`,
		string(b))
}

func TestContract_Err_ReasonFromReasonError(t *testing.T) {
	inner := &domain.NotFoundError{Entity: "publication", ID: "abc"}
	err := fmt.Errorf("importLink: %w", &domain.ReasonError{Reason: "LINK_NOT_FOUND", Err: inner})

	b, mErr := json.Marshal(Err[string](err))
	require.NoError(t, mErr)
	require.Equal(t,
		`{"data":"","error":{"code":"not_found","message":"publication not found: abc","reason":"LINK_NOT_FOUND"}}`,
		string(b))
}

func TestContract_Err_ReasonErrorWinsOverErrorInfo(t *testing.T) {
	err := &domain.ReasonError{Reason: "LINK_EXPIRED", Err: statusWithReason(t, codes.Unauthenticated, "PASSWORD_INVALID")}
	res := Err[string](err)
	require.NotNil(t, res.Error)
	assert.Equal(t, "LINK_EXPIRED", res.Error.Reason)
}

func TestContract_Err_ServerUnreachable_KeepsReason(t *testing.T) {
	err := fmt.Errorf("%w: %w", ErrServerUnreachable, statusWithReason(t, codes.ResourceExhausted, "RATE_LIMITED"))
	res := Err[string](err)
	require.NotNil(t, res.Error)
	assert.Equal(t, ErrCodeServerUnreachable, res.Error.Code)
	assert.Equal(t, "RATE_LIMITED", res.Error.Reason)
}

func TestContract_Err_StatusWithoutErrorInfoHasNoReason(t *testing.T) {
	b, err := json.Marshal(Err[string](status.Error(codes.ResourceExhausted, "slow down")))
	require.NoError(t, err)
	require.Equal(t, `{"data":"","error":{"code":"rate_limited","message":"rpc error: code = ResourceExhausted desc = slow down"}}`, string(b))
}

func TestContract_Err_Internal_PlainError(t *testing.T) {
	b, err := json.Marshal(Err[string](errors.New("boom")))
	require.NoError(t, err)
	require.Equal(t, `{"data":"","error":{"code":"internal","message":"boom"}}`, string(b))
}

func TestContract_OK_HistoryRecord(t *testing.T) {
	rec := dto.HistoryRecord{
		ID:              "h-1",
		WorkspaceID:     "ws-1",
		Protocol:        "http",
		Method:          "GET",
		URL:             "https://api.example.com",
		RequestHeaders:  map[string][]string{},
		RequestBody:     "",
		ResponseStatus:  200,
		ResponseHeaders: map[string][]string{},
		ResponseBody:    "",
		ResponseSize:    0,
		DurationMs:      0,
		CreatedAt:       "2026-01-02T03:04:05Z",
	}
	b, err := json.Marshal(OK(rec))
	require.NoError(t, err)
	got := string(b)
	require.Equal(t,
		`{"data":{"id":"h-1","workspaceId":"ws-1","protocol":"http","method":"GET","url":"https://api.example.com","requestHeaders":{},"requestBody":"","responseStatus":200,"responseHeaders":{},"responseBody":"","responseSize":0,"durationMs":0,"createdAt":"2026-01-02T03:04:05Z"}}`,
		got,
	)
	assert.NotContains(t, got, `"requestId"`, `"requestId" must be omitted when empty (omitempty)`)
	assert.NotContains(t, got, `"errorMessage"`, `"errorMessage" must be omitted when empty (omitempty)`)
}

func TestContract_OK_ListHistoryResponse_Empty(t *testing.T) {
	resp := dto.ListHistoryResponse{Items: []dto.HistoryRecord{}, TotalCount: 0}
	b, err := json.Marshal(OK(resp))
	require.NoError(t, err)
	require.Equal(t, `{"data":{"items":[],"totalCount":0}}`, string(b))
}

func TestContract_Empty_RoundTrip(t *testing.T) {
	b, err := json.Marshal(Empty{})
	require.NoError(t, err)
	require.Equal(t, `{}`, string(b))

	var back Empty
	require.NoError(t, json.Unmarshal(b, &back))
	require.Equal(t, Empty{}, back)
}
