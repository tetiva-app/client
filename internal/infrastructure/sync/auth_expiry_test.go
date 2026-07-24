package sync

import (
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// The monotonic clock stops during macOS sleep, so expiresAt must be wall-clock
// only; time.Time's String() shows "m=±…" while a monotonic reading is attached.
func TestStoreTokensStripsMonotonicClock(t *testing.T) {
	a := NewSyncAuthManager(nil)
	a.storeTokens("access", "", "")

	a.mu.RLock()
	defer a.mu.RUnlock()
	if got := a.expiresAt.String(); strings.Contains(got, "m=") {
		t.Fatalf("expiresAt still carries a monotonic reading: %s", got)
	}
	if a.expiresAt.Before(time.Now()) {
		t.Fatal("expiresAt must be in the future right after storeTokens")
	}
}

func TestInvalidateAccessToken(t *testing.T) {
	a := NewSyncAuthManager(nil)
	a.storeTokens("access", "", "org-1")

	a.InvalidateAccessToken()

	a.mu.RLock()
	defer a.mu.RUnlock()
	if a.accessToken != "" || !a.expiresAt.IsZero() {
		t.Fatalf("token not invalidated: token=%q expiresAt=%v", a.accessToken, a.expiresAt)
	}
	if a.activeOrgID != "org-1" {
		t.Fatal("activeOrgID must survive invalidation")
	}
}

func TestIsUnauthenticatedErr(t *testing.T) {
	unauth := status.Error(codes.Unauthenticated, "token expired")
	wrapped := fmt.Errorf("push entry: %w", fmt.Errorf("rpc: %w", unauth))

	if !isUnauthenticatedErr(wrapped) {
		t.Fatal("wrapped Unauthenticated must be detected")
	}
	if isUnauthenticatedErr(status.Error(codes.Unavailable, "down")) {
		t.Fatal("Unavailable must not be treated as auth failure")
	}
	if isUnauthenticatedErr(errors.New("plain")) {
		t.Fatal("plain error must not be treated as auth failure")
	}
	if isUnauthenticatedErr(nil) {
		t.Fatal("nil must not be treated as auth failure")
	}
}
