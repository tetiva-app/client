package scriptengine_test

import (
	"context"
	"testing"
	"time"

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

func TestPreScript_CollectionVariablesShareTheEnvironment(t *testing.T) {
	engine := scriptengine.NewGojaEngine()

	result, err := engine.RunPreScript(context.Background(), `
		pm.collectionVariables.set("k", "v");
		console.log(pm.environment.get("k"));
		console.log(pm.collectionVariables.get("host"));
		pm.collectionVariables.unset("token");
		console.log(pm.environment.get("token"));
	`, newContext())

	require.NoError(t, err)
	assert.Equal(t, []string{"v", "https://api.example.com", "undefined"}, result.ConsoleOutput)
	assert.Equal(t, map[string]string{"host": "https://api.example.com", "k": "v"}, result.Variables)
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

	var runErr *request.ScriptRunError
	require.ErrorAs(t, err, &runErr)
	assert.True(t, runErr.Timeout)
}

func TestPreScript_SyntaxError(t *testing.T) {
	engine := scriptengine.NewGojaEngine()
	_, err := engine.RunPreScript(context.Background(), `
		var x = {{{;
	`, newContext())

	require.Error(t, err)
}

func TestPreScript_BtoaAtob(t *testing.T) {
	engine := scriptengine.NewGojaEngine()
	result, err := engine.RunPreScript(context.Background(), `
		pm.request.headers.upsert({key: 'Authorization', value: 'Basic ' + btoa('user:pass')});
		pm.request.headers.upsert({key: 'X-Decoded', value: atob('dXNlcjpwYXNz')});
	`, newContext())

	require.NoError(t, err)
	assert.Equal(t, []string{"Basic dXNlcjpwYXNz"}, result.Headers["Authorization"])
	assert.Equal(t, []string{"user:pass"}, result.Headers["X-Decoded"])
}

func TestPreScript_BtoaRejectsNonLatin1(t *testing.T) {
	engine := scriptengine.NewGojaEngine()
	_, err := engine.RunPreScript(context.Background(), `btoa('пароль')`, newContext())

	require.Error(t, err)
	assert.Contains(t, err.Error(), "Latin1")
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

func TestPreScript_AtobIsForgivingLikeBrowsersAndPostman(t *testing.T) {
	engine := scriptengine.NewGojaEngine()
	result, err := engine.RunPreScript(context.Background(), `
		const jwt = 'eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiIxMjM0NTY3ODkwIiwibmFtZSI6IkpvaG4ifQ.sig';
		console.log(JSON.parse(atob(jwt.split('.')[1])).sub);
		console.log(atob(' dXNl\ncjpw YXNz '));
		console.log(atob('PDw_Pz8-Pg'));
	`, newContext())

	require.NoError(t, err)
	assert.Equal(t, []string{"1234567890", "user:pass", "<<???>>"}, result.ConsoleOutput)
}

func TestPreScript_AtobRejectsImpossibleLength(t *testing.T) {
	_, err := scriptengine.NewGojaEngine().RunPreScript(context.Background(), `atob('abcde')`, newContext())

	require.Error(t, err)
	assert.Contains(t, err.Error(), "atob")
}

func TestPreScript_HeaderNamesAreCaseInsensitive(t *testing.T) {
	sctx := newContext()
	sctx.RequestHeaders = map[string][]string{"Content-Type": {"text/plain"}, "X-Keep": {"1"}}

	result, err := scriptengine.NewGojaEngine().RunPreScript(context.Background(), `
		pm.request.headers.upsert({ key: 'content-type', value: 'application/json' });
		pm.request.headers.remove('x-keep');
	`, sctx)

	require.NoError(t, err)
	assert.Equal(t, map[string][]string{"Content-Type": {"application/json"}}, result.Headers)
}

func TestPreScript_HeaderCallsWithoutAValueOrKey(t *testing.T) {
	result, err := scriptengine.NewGojaEngine().RunPreScript(context.Background(), `
		pm.request.headers.add({ key: 'X-A' });
		pm.request.headers.upsert({ key: 'X-B' });
	`, newContext())
	require.NoError(t, err)
	assert.Equal(t, []string{""}, result.Headers["X-A"])
	assert.Equal(t, []string{""}, result.Headers["X-B"])

	for _, script := range []string{`pm.request.headers.upsert({ value: '1' })`, `pm.request.headers.add()`, `pm.request.headers.upsert(null)`} {
		_, err := scriptengine.NewGojaEngine().RunPreScript(context.Background(), script, newContext())
		require.Error(t, err, script)
		assert.Contains(t, err.Error(), "{ key, value }", script)
	}
}

func TestPreScript_ThrownObjectNamedSyntaxError(t *testing.T) {
	_, err := scriptengine.NewGojaEngine().RunPreScript(context.Background(), `throw { name: 'SyntaxError' }`, newContext())

	var runErr *request.ScriptRunError
	require.ErrorAs(t, err, &runErr)
	assert.Equal(t, 1, runErr.Line)
}

func TestPostScript_SeesPmVariablesFromThePreRequestScript(t *testing.T) {
	sctx := newContext()
	sctx.Variables = map[string]string{"id": "old"}
	sctx.LocalVariables = map[string]string{"id": "new"}

	result, err := scriptengine.NewGojaEngine().RunPostScript(context.Background(), `
		console.log(pm.variables.get('id'), pm.environment.get('id'));
		pm.variables.set('id', 'changed');
	`, sctx)

	require.NoError(t, err)
	assert.Equal(t, []string{"new old"}, result.ConsoleOutput)
	assert.Equal(t, map[string]string{"id": "new"}, sctx.LocalVariables)
}

func TestPreScript_HeadersAddAppends(t *testing.T) {
	result, err := scriptengine.NewGojaEngine().RunPreScript(context.Background(), `
		pm.request.headers.add({ key: 'X-Added', value: '1' });
		pm.request.headers.add({ key: 'x-added', value: '2' });
		pm.request.headers.add({ key: 'content-type', value: 'text/plain' });
	`, newContext())

	require.NoError(t, err)
	assert.Equal(t, []string{"1", "2"}, result.Headers["X-Added"])
	assert.Equal(t, []string{"application/json", "text/plain"}, result.Headers["Content-Type"])
}

func TestPreScript_PmVariablesAreLocalToTheRun(t *testing.T) {
	sctx := newContext()

	result, err := scriptengine.NewGojaEngine().RunPreScript(context.Background(), `
		pm.variables.set('reqId', '42');
		pm.variables.set('host', 'https://local.example.com');
		console.log(pm.variables.get('reqId'), pm.variables.get('host'), pm.variables.get('token'));
		console.log(pm.variables.has('reqId'), pm.variables.has('missing'));
		pm.variables.unset('host');
		console.log(pm.variables.get('host'));
	`, sctx)

	require.NoError(t, err)
	assert.Equal(t, []string{
		"42 https://local.example.com abc123",
		"true false",
		"https://api.example.com",
	}, result.ConsoleOutput)
	assert.Equal(t, map[string]string{"reqId": "42"}, result.LocalVariables)
	assert.Equal(t, sctx.Variables, result.Variables)
}

func TestPreScript_UnsupportedPmAPIsNameTheAlternative(t *testing.T) {
	for script, want := range map[string]string{
		`pm.globals.set('g', '1')`:      "pm.globals is not supported in Tetiva; use pm.environment",
		`pm.globals.get('g')`:           "pm.globals is not supported in Tetiva; use pm.environment",
		`pm.sendRequest('https://x.y')`: "pm.sendRequest is not supported in Tetiva",
	} {
		_, err := scriptengine.NewGojaEngine().RunPreScript(context.Background(), script, newContext())

		require.Error(t, err, script)
		assert.Contains(t, err.Error(), want, script)
	}
}

func TestPreScript_ErrorCarriesMessageLineAndColumn(t *testing.T) {
	_, err := scriptengine.NewGojaEngine().RunPreScript(context.Background(), "var a = 1\n\nnotDefinedFn()", newContext())

	var runErr *request.ScriptRunError
	require.ErrorAs(t, err, &runErr)
	assert.Equal(t, "ReferenceError: notDefinedFn is not defined", runErr.Message)
	assert.Equal(t, 3, runErr.Line)
	assert.Positive(t, runErr.Column)
	assert.NotContains(t, err.Error(), "<eval>")
}

func TestPreScript_SyntaxErrorIsReadable(t *testing.T) {
	_, err := scriptengine.NewGojaEngine().RunPreScript(context.Background(), "\nif (token {\n  x()\n}", newContext())

	var runErr *request.ScriptRunError
	require.ErrorAs(t, err, &runErr)
	assert.Equal(t, request.ScriptRunError{Message: "SyntaxError: Unexpected token {", Line: 2, Column: 11}, *runErr)
}

func TestPreScript_FailureKeepsTheConsoleWrittenBeforeIt(t *testing.T) {
	result, err := scriptengine.NewGojaEngine().RunPreScript(context.Background(), `
		console.log('step 1');
		console.log('step 2');
		computeSignature();
	`, newContext())

	require.Error(t, err)
	require.NotNil(t, result)
	assert.Equal(t, []string{"step 1", "step 2"}, result.ConsoleOutput)
}

func TestPreScript_ConsoleIsCappedForRunawayLoops(t *testing.T) {
	result, err := scriptengine.NewGojaEngine().RunPreScript(context.Background(), `
		let i = 0;
		while (true) console.log(i++);
	`, newContext())

	require.Error(t, err)
	require.NotNil(t, result)
	assert.LessOrEqual(t, len(result.ConsoleOutput), 1001)
	assert.Contains(t, result.ConsoleOutput[len(result.ConsoleOutput)-1], "more lines not shown")
}

func TestPostScript_FailureKeepsTestsThatAlreadyRan(t *testing.T) {
	sctx := newContext()
	sctx.ResponseStatusCode = 200
	sctx.ResponseBody = `{}`

	result, err := scriptengine.NewGojaEngine().RunPostScript(context.Background(), `
		pm.test('status is 200', () => pm.response.to.have.status(200));
		console.log('before');
		pm.response.json().items.forEach(i => console.log(i));
	`, sctx)

	require.Error(t, err)
	require.NotNil(t, result)
	assert.Equal(t, []request.TestResult{{Name: "status is 200", Passed: true}}, result.TestResults)
	assert.Equal(t, []string{"before"}, result.ConsoleOutput)
}

func TestPreScript_PmVariablesReplaceInAndToObject(t *testing.T) {
	result, err := scriptengine.NewGojaEngine().RunPreScript(context.Background(), `
		pm.variables.set('reqId', '42');
		console.log(pm.variables.replaceIn('{{host}}/items/{{reqId}}?x={{missing}}'));
		const all = pm.variables.toObject();
		console.log(Object.keys(all).sort().join(','), all.reqId, all.token);
	`, newContext())

	require.NoError(t, err)
	assert.Equal(t, []string{
		"https://api.example.com/items/42?x={{missing}}",
		"host,reqId,token 42 abc123",
	}, result.ConsoleOutput)
}

func TestPreScript_ConsoleLogPrintsObjectsAsJSON(t *testing.T) {
	result, err := scriptengine.NewGojaEngine().RunPreScript(context.Background(), `
		console.log('user =', 'alice', 'obj =', { a: 1 }, [1, 2], null, undefined, 3);
		console.log(new Error('boom'), function f() {});
		const loop = {}; loop.self = loop;
		console.log(loop);
	`, newContext())

	require.NoError(t, err)
	require.Len(t, result.ConsoleOutput, 3)
	assert.Equal(t, `user = alice obj = {"a":1} [1,2] null undefined 3`, result.ConsoleOutput[0])
	assert.Equal(t, "Error: boom function f() {}", result.ConsoleOutput[1])
	assert.Equal(t, "[object Object]", result.ConsoleOutput[2])
}

func TestPreScript_ConsoleLogStopsOnHugeObjects(t *testing.T) {
	start := time.Now()
	result, err := scriptengine.NewGojaEngine().RunPreScript(context.Background(), `
		let x = {};
		for (let i = 0; i < 30; i++) x = { a: x, b: x };
		console.log(x);
	`, newContext())

	require.NoError(t, err)
	assert.Equal(t, []string{"[object Object]"}, result.ConsoleOutput)
	assert.Less(t, time.Since(start), 2*time.Second)
}

func TestPreScript_ConsoleLogIgnoresAReplacedJSONStringify(t *testing.T) {
	result, err := scriptengine.NewGojaEngine().RunPreScript(context.Background(), `
		JSON.stringify = null;
		console.log({ a: 1 });
	`, newContext())

	require.NoError(t, err)
	assert.Equal(t, []string{`{"a":1}`}, result.ConsoleOutput)
}
