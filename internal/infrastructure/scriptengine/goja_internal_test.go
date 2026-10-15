package scriptengine

import (
	"context"
	"testing"
	"time"

	"github.com/dop251/goja"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRunWithTimeout_AHostFunctionPanicIsAnError(t *testing.T) {
	vm := goja.New()
	_ = vm.Set("broken", func(goja.FunctionCall) goja.Value {
		var v goja.Value
		return vm.ToValue(v.String())
	})

	err := runWithTimeout(context.Background(), vm, "broken()", time.Second)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "script engine failed")
}
