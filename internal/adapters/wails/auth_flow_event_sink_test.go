package wails

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tetiva-app/client/internal/adapters/wails/dto"
	"github.com/tetiva-app/client/internal/domain/usecase/auth"
)

func TestAuthFlowEventSinkEmitsState(t *testing.T) {
	var name string
	var payload any
	sink := NewAuthFlowEventSink()
	sink.SetEmit(func(n string, data any) { name, payload = n, data })

	sink.OnFlowState("f1", auth.FlowPending, nil)

	assert.Equal(t, "auth:flow:f1", name)
	d, ok := payload.(dto.FlowStatusDTO)
	require.True(t, ok)
	assert.Equal(t, string(auth.FlowPending), d.State)
	assert.Empty(t, d.Error)
}

func TestAuthFlowEventSinkCarriesTheError(t *testing.T) {
	var payload any
	sink := NewAuthFlowEventSink()
	sink.SetEmit(func(_ string, data any) { payload = data })

	sink.OnFlowState("f2", auth.FlowError, errors.New("the flow timed out"))

	d, ok := payload.(dto.FlowStatusDTO)
	require.True(t, ok)
	assert.Equal(t, string(auth.FlowError), d.State)
	assert.Equal(t, "the flow timed out", d.Error)
}

func TestAuthFlowEventSinkNilEmitSafe(t *testing.T) {
	sink := NewAuthFlowEventSink() // emit never set
	sink.OnFlowState("f3", auth.FlowCancelled, nil)
	sink.OnFlowState("f3", auth.FlowError, errors.New("boom")) // must not panic
}
