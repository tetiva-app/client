package scriptengine_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tetiva-app/client/internal/domain/usecase/request"
	"github.com/tetiva-app/client/internal/infrastructure/scriptengine"
)

func newContext() request.ScriptContext {
	return request.ScriptContext{
		Variables:      map[string]string{"host": "https://api.example.com", "token": "abc123"},
		RequestMethod:  "GET",
		RequestURL:     "https://api.example.com/ping",
		RequestHeaders: map[string][]string{"Content-Type": {"application/json"}},
	}
}

func TestPreScript_EnvironmentGet(t *testing.T) {
	engine := scriptengine.NewGojaEngine()
	sctx := newContext()

	result, err := engine.RunPreScript(context.Background(), `
		var host = pm.environment.get("host");
		console.log("host is", host);
	`, sctx)

	require.NoError(t, err)
	assert.Contains(t, result.ConsoleOutput, "host is https://api.example.com")
	assert.Equal(t, sctx.Variables, result.Variables)
}

func TestPreScript_EnvironmentSet(t *testing.T) {
	engine := scriptengine.NewGojaEngine()
	sctx := newContext()

	result, err := engine.RunPreScript(context.Background(), `
		pm.environment.set("new_var", "new_value");
		pm.environment.set("host", "https://changed.com");
	`, sctx)

	require.NoError(t, err)
	assert.Equal(t, "new_value", result.Variables["new_var"])
	assert.Equal(t, "https://changed.com", result.Variables["host"])
}

func TestPreScript_EnvironmentUnset(t *testing.T) {
	engine := scriptengine.NewGojaEngine()
	sctx := newContext()

	result, err := engine.RunPreScript(context.Background(), `
		pm.environment.unset("token");
	`, sctx)

	require.NoError(t, err)
	_, exists := result.Variables["token"]
	assert.False(t, exists)
}

func TestPreScript_ConsoleLog(t *testing.T) {
	engine := scriptengine.NewGojaEngine()
	result, err := engine.RunPreScript(context.Background(), `
		console.log("hello", 42, true);
		console.log("second line");
	`, newContext())

	require.NoError(t, err)
	require.Equal(t, 2, len(result.ConsoleOutput))
	assert.Equal(t, "hello 42 true", result.ConsoleOutput[0])
	assert.Equal(t, "second line", result.ConsoleOutput[1])
}

func TestPreScript_Timeout(t *testing.T) {
	engine := scriptengine.NewGojaEngine()
	_, err := engine.RunPreScript(context.Background(), `
		while(true) {}
	`, newContext())

	require.Error(t, err)
	assert.Contains(t, err.Error(), "timeout")
}

func TestPreScript_SyntaxError(t *testing.T) {
	engine := scriptengine.NewGojaEngine()
	_, err := engine.RunPreScript(context.Background(), `
		var x = {{{;
	`, newContext())

	require.Error(t, err)
}

func TestPreScript_HeadersPassthrough(t *testing.T) {
	engine := scriptengine.NewGojaEngine()
	sctx := newContext()

	result, err := engine.RunPreScript(context.Background(), `
		// no modifications
	`, sctx)

	require.NoError(t, err)
	assert.Equal(t, sctx.RequestHeaders, result.Headers)
}

func postContext() request.ScriptContext {
	return request.ScriptContext{
		Variables:          map[string]string{"host": "https://api.example.com"},
		RequestMethod:      "POST",
		RequestURL:         "https://api.example.com/login",
		RequestHeaders:     map[string][]string{"Content-Type": {"application/json"}},
		ResponseStatusCode: 201,
		ResponseBody:       `{"access_token":"xyz","user_id":"u123"}`,
		ResponseHeaders:    map[string][]string{"Content-Type": {"application/json"}},
	}
}

func TestPostScript_ResponseJson(t *testing.T) {
	engine := scriptengine.NewGojaEngine()
	result, err := engine.RunPostScript(context.Background(), `
		var data = pm.response.json();
		pm.environment.set("access_token", data.access_token);
		pm.environment.set("user_id", data.user_id);
	`, postContext())

	require.NoError(t, err)
	assert.Equal(t, "xyz", result.Variables["access_token"])
	assert.Equal(t, "u123", result.Variables["user_id"])
}

func TestPostScript_ResponseCode(t *testing.T) {
	engine := scriptengine.NewGojaEngine()
	result, err := engine.RunPostScript(context.Background(), `
		console.log("status:", pm.response.code);
	`, postContext())

	require.NoError(t, err)
	assert.Contains(t, result.ConsoleOutput, "status: 201")
}

func TestPostScript_ResponseText(t *testing.T) {
	engine := scriptengine.NewGojaEngine()
	result, err := engine.RunPostScript(context.Background(), `
		var body = pm.response.text();
		console.log(body);
	`, postContext())

	require.NoError(t, err)
	assert.Contains(t, result.ConsoleOutput[0], "access_token")
}

func TestPostScript_PmTestPass(t *testing.T) {
	engine := scriptengine.NewGojaEngine()
	result, err := engine.RunPostScript(context.Background(), `
		pm.test("status is 201", function() {
			if (pm.response.code !== 201) throw new Error("expected 201");
		});
	`, postContext())

	require.NoError(t, err)
	require.Equal(t, 1, len(result.TestResults))
	assert.Equal(t, "status is 201", result.TestResults[0].Name)
	assert.True(t, result.TestResults[0].Passed)
}

func TestPostScript_PmTestFail(t *testing.T) {
	engine := scriptengine.NewGojaEngine()
	result, err := engine.RunPostScript(context.Background(), `
		pm.test("status is 200", function() {
			if (pm.response.code !== 200) throw new Error("expected 200, got " + pm.response.code);
		});
	`, postContext())

	require.NoError(t, err)
	require.Equal(t, 1, len(result.TestResults))
	assert.False(t, result.TestResults[0].Passed)
	assert.Contains(t, result.TestResults[0].Error, "expected 200")
}

func TestPostScript_MultipleTests(t *testing.T) {
	engine := scriptengine.NewGojaEngine()
	result, err := engine.RunPostScript(context.Background(), `
		pm.test("has token", function() {
			var data = pm.response.json();
			if (!data.access_token) throw new Error("no token");
		});
		pm.test("status ok", function() {
			if (pm.response.code >= 400) throw new Error("bad status");
		});
		pm.test("intentional fail", function() {
			throw new Error("oops");
		});
	`, postContext())

	require.NoError(t, err)
	require.Equal(t, 3, len(result.TestResults))
	assert.True(t, result.TestResults[0].Passed)
	assert.True(t, result.TestResults[1].Passed)
	assert.False(t, result.TestResults[2].Passed)
}

func TestPreScript_HeadersUpsertAndRemove(t *testing.T) {
	engine := scriptengine.NewGojaEngine()
	sctx := newContext()

	result, err := engine.RunPreScript(context.Background(), `
		pm.request.headers.upsert({key: "X-Custom", value: "hello"});
		pm.request.headers.remove("Content-Type");
	`, sctx)

	require.NoError(t, err)
	assert.Equal(t, []string{"hello"}, result.Headers["X-Custom"])
	_, hasCT := result.Headers["Content-Type"]
	assert.False(t, hasCT)
}
