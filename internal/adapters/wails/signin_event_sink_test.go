package wails

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tetiva-app/client/internal/adapters/wails/dto"
	"github.com/tetiva-app/client/internal/domain/usecase/auth"
)

func TestSignInEventSink_EmitsThePerFlowEvent(t *testing.T) {
	sink := NewSignInEventSink()
	deadline := time.Now().Add(30 * time.Minute).UTC().Truncate(time.Second)

	var name string
	var data any
	sink.SetEmit(func(n string, d any) { name, data = n, d })

	sink.OnSignInState("flow-1", auth.SignInStatus{
		State:                    auth.FlowPending,
		Info:                     auth.SignInInfo{LoginURL: "https://app.tetiva.app/x#claim=s", ExpiresAt: deadline},
		EmailVerificationPending: true,
	})

	assert.Equal(t, "sync:signin:flow-1", name)
	ev, ok := data.(dto.BrowserSignInEvent)
	require.True(t, ok)
	assert.Equal(t, "pending", ev.State)
	assert.True(t, ev.EmailVerificationPending)
	assert.Equal(t, deadline.Format(time.RFC3339), ev.ExpiresAt)
	assert.Empty(t, ev.Error)
}

func TestSignInEventSink_ErrorTravelsWithTheTerminalState(t *testing.T) {
	sink := NewSignInEventSink()

	var data any
	sink.SetEmit(func(_ string, d any) { data = d })

	sink.OnSignInState("flow-1", auth.SignInStatus{State: auth.FlowError, Err: auth.ErrSignInExpired})

	ev := data.(dto.BrowserSignInEvent)
	assert.Equal(t, "error", ev.State)
	assert.Equal(t, "Sign-in link expired, try again", ev.Error,
		"the panel shows §2.5 copy, not the Go sentinel")
	assert.Empty(t, ev.ExpiresAt)
}

func TestSignInEventSink_CommitFailureDoesNotLeakInternals(t *testing.T) {
	sink := NewSignInEventSink()

	var data any
	sink.SetEmit(func(_ string, d any) { data = d })

	sink.OnSignInState("flow-1", auth.SignInStatus{
		State: auth.FlowError,
		Err:   fmt.Errorf("SyncService.signInHooks: %w", errors.New("update config: database is locked")),
	})

	ev := data.(dto.BrowserSignInEvent)
	assert.Equal(t, "Sign-in failed, try again", ev.Error)
}

func TestSignInEventSink_SilentBeforeTheEmitterArrives(t *testing.T) {
	sink := NewSignInEventSink()

	// The manager can reach a terminal state before main.go wires the emitter.
	sink.OnSignInState("flow-1", auth.SignInStatus{State: auth.FlowCancelled})
}
