package scriptengine_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tetiva-app/client/internal/infrastructure/scriptengine"
)

// What a collection script can reach. Anything new appearing here is a decision, not an accident.
func TestSandbox_GlobalsVisibleToScripts(t *testing.T) {
	engine := scriptengine.NewGojaEngine()
	res, err := engine.RunPreScript(context.Background(),
		`console.log(Object.getOwnPropertyNames(globalThis).sort().join(","))`, newContext())
	require.NoError(t, err)
	require.Len(t, res.ConsoleOutput, 1)

	globals := strings.Split(res.ConsoleOutput[0], ",")
	assert.Subset(t, globals, []string{"pm", "console", "JSON", "Math", "Promise"})

	for _, forbidden := range []string{"process", "require", "module", "exports", "Buffer",
		"fetch", "XMLHttpRequest", "WebSocket", "setTimeout", "setInterval", "__dirname"} {
		assert.NotContains(t, globals, forbidden)
	}
}

func TestSandbox_FunctionConstructorFindsNoHost(t *testing.T) {
	engine := scriptengine.NewGojaEngine()
	res, err := engine.RunPreScript(context.Background(), `
		try { console.log(String(this.constructor.constructor('return process')())) }
		catch (e) { console.log('threw: ' + e) }
	`, newContext())
	require.NoError(t, err)
	require.Len(t, res.ConsoleOutput, 1)
	assert.Contains(t, res.ConsoleOutput[0], "process is not defined")
}

func TestSandbox_NoStateCarriesBetweenRuns(t *testing.T) {
	engine := scriptengine.NewGojaEngine()
	_, err := engine.RunPreScript(context.Background(), `globalThis.__leak = "planted"`, newContext())
	require.NoError(t, err)

	res, err := engine.RunPreScript(context.Background(), `console.log(String(globalThis.__leak))`, newContext())
	require.NoError(t, err)
	assert.Equal(t, []string{"undefined"}, res.ConsoleOutput)
}

func TestSandbox_TryCatchDoesNotSwallowInterrupt(t *testing.T) {
	engine := scriptengine.NewGojaEngine()
	start := time.Now()
	_, err := engine.RunPreScript(context.Background(), `while (true) { try { while (true) {} } catch (e) {} }`, newContext())

	require.Error(t, err)
	assert.Contains(t, err.Error(), "timeout")
	assert.Less(t, time.Since(start), 10*time.Second)
}

// A regexp that falls back to backtracking never checks for the interrupt. The run is abandoned
// and the caller must still be released.
func TestSandbox_BacktrackingRegexpDoesNotHoldCaller(t *testing.T) {
	engine := scriptengine.NewGojaEngine()
	before := scriptengine.AbandonedScripts()

	start := time.Now()
	_, err := engine.RunPreScript(context.Background(),
		`var s = "a".repeat(40) + "!"; /^(?=(a+)+$)a*$/.test(s)`, newContext())
	elapsed := time.Since(start)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "timeout")
	assert.Less(t, elapsed, 10*time.Second)
	assert.Greater(t, scriptengine.AbandonedScripts(), before)
}

func TestSandbox_CancellationReleasesCallerOnBacktrackingRegexp(t *testing.T) {
	engine := scriptengine.NewGojaEngine()
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	start := time.Now()
	_, err := engine.RunPreScript(ctx, `var s = "a".repeat(40) + "!"; /(a+)+\1$/.test(s)`, newContext())

	require.Error(t, err)
	assert.Contains(t, err.Error(), "cancelled")
	assert.Less(t, time.Since(start), 5*time.Second)
}

func TestSandbox_MetadataIsNotTheCallersMap(t *testing.T) {
	engine := scriptengine.NewGojaEngine()
	sctx := newContext()
	sctx.Protocol = "grpc"
	sctx.Metadata = map[string][]string{"authorization": {"Bearer original"}}

	res, err := engine.RunPreScript(context.Background(),
		`pm.request.metadata.set("authorization", "Bearer from-script")`, sctx)
	require.NoError(t, err)

	assert.Equal(t, []string{"Bearer original"}, sctx.Metadata["authorization"])
	assert.Equal(t, []string{"Bearer from-script"}, res.Metadata["authorization"])
}

func TestSandbox_MetadataSetWithoutExistingMetadata(t *testing.T) {
	engine := scriptengine.NewGojaEngine()
	sctx := newContext()
	sctx.Protocol = "grpc"

	res, err := engine.RunPreScript(context.Background(),
		`pm.request.metadata.set("x-trace", "1")`, sctx)
	require.NoError(t, err)
	assert.Equal(t, []string{"1"}, res.Metadata["x-trace"])
}
