package scriptengine_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tetiva-app/client/internal/domain/usecase/request"
	"github.com/tetiva-app/client/internal/infrastructure/scriptengine"
)

func userResponseContext() request.ScriptContext {
	return request.ScriptContext{
		Variables:          map[string]string{},
		RequestMethod:      "GET",
		RequestURL:         "https://api.example.com/users/42",
		RequestHeaders:     map[string][]string{},
		ResponseStatusCode: 200,
		ResponseStatusText: "200 OK",
		ResponseTime:       120 * time.Millisecond,
		ResponseHeaders: map[string][]string{
			"Content-Type": {"application/json; charset=utf-8"},
			"X-Request-Id": {"abc-123"},
		},
		ResponseBody: `{"id":42,"name":"Ada","email":"ada@example.com","roles":["admin","dev"],` +
			`"address":{"city":"London","zip":"N1"},"active":true,"deleted_at":null}`,
		Protocol: "http",
	}
}

func TestExpect_PostmanSnippets(t *testing.T) {
	cases := []struct {
		name    string
		snippet string
		wantErr string
	}{
		{"status code", `pm.expect(pm.response.code).to.equal(200)`, ""},
		{"response status", `pm.response.to.have.status(200)`, ""},
		{"status reason", `pm.response.to.have.status("OK")`, ""},
		{"json property", `pm.expect(pm.response.json().name).to.eql("Ada")`, ""},
		{"array length", `pm.expect(pm.response.json().roles).to.have.lengthOf(2)`, ""},
		{"header contains", `pm.expect(pm.response.headers.get("content-type")).to.include("application/json")`, ""},
		{"response time below", `pm.expect(pm.response.responseTime).to.be.below(200)`, ""},
		{"deep equal", `pm.expect(pm.response.json().address).to.deep.equal({ city: "London", zip: "N1" })`, ""},
		{"not", `pm.expect(pm.response.json().name).to.not.equal("Bob")`, ""},
		{"body includes", `pm.expect(pm.response.text()).to.include("ada@example.com")`, ""},
		{"one of", `pm.expect(pm.response.code).to.be.oneOf([200, 201, 202])`, ""},
		{"match regexp", `pm.expect(pm.response.json().email).to.match(/^\S+@\S+\.\S+$/)`, ""},
		{"nested property", `pm.expect(pm.response.json()).to.have.nested.property("address.city", "London")`, ""},
		{"all keys", `pm.expect(pm.response.json().address).to.be.an("object").that.has.all.keys("city", "zip")`, ""},
		{"property with value", `pm.expect(pm.response.json()).to.have.property("id", 42)`, ""},
		{"true and null", `pm.expect(pm.response.json().active).to.be.true; pm.expect(pm.response.json().deleted_at).to.be.null`, ""},
		{"array includes", `pm.expect(pm.response.json().roles).to.be.an("array").that.includes("admin")`, ""},
		{"range", `pm.expect(pm.response.json().id).to.be.above(40).and.below(50)`, ""},
		{"truthy value", `pm.expect("x").to.be.ok; pm.expect(0).to.not.be.ok`, ""},
		{"response header", `pm.response.to.have.header("X-Request-Id"); pm.response.to.have.header("x-request-id", "abc-123")`, ""},
		{"json body", `pm.response.to.have.jsonBody(); pm.response.to.have.jsonBody("address.city"); pm.response.to.have.jsonBody("roles[0]", "admin")`, ""},
		{"response classes", `pm.response.to.be.ok; pm.response.to.be.success; pm.response.to.be.json; pm.response.to.not.be.error`, ""},
		{"response body", `pm.response.to.have.body(); pm.response.to.have.body(/"Ada"/)`, ""},
		{"json body object", `pm.response.to.have.body(JSON.parse(pm.response.text())); pm.response.to.not.have.body({ id: 42 })`, ""},
		{"body text", `pm.response.to.have.body(pm.response.text()); pm.response.to.not.have.body("Ada")`, ""},
		{"expect on response", `pm.expect(pm.response).to.have.status(200)`, ""},
		{"status is the reason phrase", `pm.expect(pm.response.status).to.equal("OK"); pm.expect(pm.response.code).to.equal(200)`, ""},

		{"wrong status code", `pm.expect(pm.response.code).to.equal(404)`,
			"AssertionError: expected 200 to equal 404"},
		{"wrong response status", `pm.response.to.have.status(201)`,
			"AssertionError: expected response to have status code 201 but got 200"},
		{"wrong status reason", `pm.response.to.have.status("Created")`,
			"AssertionError: expected response to have status reason 'Created' but got 'OK'"},
		{"wrong length", `pm.expect(pm.response.json().roles).to.have.lengthOf(3)`,
			"AssertionError: expected [ 'admin', 'dev' ] to have a length of 3 but got 2"},
		{"missing header", `pm.response.to.have.header("X-Missing")`,
			"AssertionError: expected response to have header with key 'X-Missing'"},
		{"wrong header value", `pm.response.to.have.header("Content-Type", "text/html")`,
			"AssertionError: expected response header 'Content-Type' to be 'text/html' but got 'application/json; charset=utf-8'"},
		{"missing json path", `pm.response.to.have.jsonBody("address.country")`,
			"AssertionError: expected response body json to have path 'address.country'"},
		{"wrong json value", `pm.response.to.have.jsonBody("address.city", "Paris")`,
			"AssertionError: expected response body json at 'address.city' to deep equal 'Paris' but got 'London'"},
		{"not a client error", `pm.response.to.be.clientError`,
			"AssertionError: expected response code to be 4XX but found 200"},
		{"negated ok", `pm.response.to.not.be.ok`,
			"AssertionError: expected response code to not be 200 but found 200"},
		{"not one of", `pm.expect(pm.response.json().name).to.be.oneOf(["Bob", "Eve"])`,
			"AssertionError: expected 'Ada' to be one of [ 'Bob', 'Eve' ]"},
		{"too slow", `pm.expect(pm.response.responseTime).to.be.below(100)`,
			"AssertionError: expected 120 to be below 100"},
		{"custom message", `pm.expect(pm.response.code, "status").to.equal(201)`,
			"AssertionError: status: expected 200 to equal 201"},
		{"deep equal mismatch", `pm.expect(pm.response.json().address).to.eql({ city: "Paris" })`,
			"AssertionError: expected { city: 'London', zip: 'N1' } to deeply equal { city: 'Paris' }"},
		{"json body mismatch", `pm.response.to.have.body({ id: 42 })`,
			"AssertionError: expected response body json to equal { id: 42 } but got { id: 42, name: 'Ada', …(5) }"},
		{"negated json body", `pm.response.to.not.have.body(JSON.parse(pm.response.text()))`,
			"AssertionError: expected response body json to not equal { id: 42, name: 'Ada', …(5) }"},
	}

	engine := scriptengine.NewGojaEngine()
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			res, err := engine.RunPostScript(context.Background(),
				"pm.test('t', function () {\n"+tc.snippet+"\n})", userResponseContext())
			require.NoError(t, err)
			require.Len(t, res.TestResults, 1)
			assert.Equal(t, tc.wantErr, res.TestResults[0].Error)
			assert.Equal(t, tc.wantErr == "", res.TestResults[0].Passed)
		})
	}
}

func TestExpect_OutsideTestFailsTheScriptAtTheUsersLine(t *testing.T) {
	engine := scriptengine.NewGojaEngine()
	_, err := engine.RunPostScript(context.Background(), "var code = pm.response.code\npm.expect(code).to.equal(201)", userResponseContext())

	require.Error(t, err)
	assert.Contains(t, err.Error(), "AssertionError: expected 200 to equal 201 (line 2, column")
	assert.NotContains(t, err.Error(), "chai")
}

func TestExpect_ResponseOkMeansExactly200(t *testing.T) {
	sctx := userResponseContext()
	sctx.ResponseStatusCode = 201
	sctx.ResponseStatusText = "201 Created"

	res, err := scriptengine.NewGojaEngine().RunPostScript(context.Background(), `
		pm.test("success", function () { pm.response.to.be.success })
		pm.test("ok", function () { pm.response.to.be.ok })
	`, sctx)

	require.NoError(t, err)
	require.Len(t, res.TestResults, 2)
	assert.True(t, res.TestResults[0].Passed)
	assert.False(t, res.TestResults[1].Passed)
	assert.Equal(t, "AssertionError: expected response code to be 200 but found 201", res.TestResults[1].Error)
}

func TestExpect_WorksInPreScript(t *testing.T) {
	engine := scriptengine.NewGojaEngine()

	res, err := engine.RunPreScript(context.Background(), `
		pm.expect(pm.request.method).to.be.oneOf(["GET", "HEAD"]);
		pm.environment.set("checked", "yes");
	`, newContext())
	require.NoError(t, err)
	assert.Equal(t, "yes", res.Variables["checked"])

	_, err = engine.RunPreScript(context.Background(), `pm.expect(pm.environment.get("token")).to.equal("other")`, newContext())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "AssertionError: expected 'abc123' to equal 'other'")
}

func TestExpect_PreScriptHasNoResponseAssertions(t *testing.T) {
	engine := scriptengine.NewGojaEngine()
	res, err := engine.RunPreScript(context.Background(), `console.log(typeof pm.response)`, newContext())
	require.NoError(t, err)
	assert.Equal(t, []string{"undefined"}, res.ConsoleOutput)
}

func TestExpect_ChaiStaysOffTheGlobalScope(t *testing.T) {
	engine := scriptengine.NewGojaEngine()
	res, err := engine.RunPostScript(context.Background(), `
		pm.expect(1).to.equal(1);
		pm.response.to.be.ok;
		console.log(Object.getOwnPropertyNames(globalThis).join(","));
	`, userResponseContext())
	require.NoError(t, err)
	require.Len(t, res.ConsoleOutput, 1)

	globals := strings.Split(res.ConsoleOutput[0], ",")
	for _, leaked := range []string{"chai", "module", "exports", "define", "require", "global", "window", "self", "loupe", "typeDetect"} {
		assert.NotContains(t, globals, leaked)
	}
}

func TestExpect_TimeoutStillFiresAfterChaiLoads(t *testing.T) {
	engine := scriptengine.NewGojaEngine()
	before := scriptengine.AbandonedScripts()
	start := time.Now()

	_, err := engine.RunPostScript(context.Background(),
		`pm.test("spin", function () { pm.expect(1).to.equal(1); pm.response.to.be.ok; while (true) {} })`,
		userResponseContext())

	require.Error(t, err)
	assert.Contains(t, err.Error(), "timed out")
	assert.Less(t, time.Since(start), 10*time.Second)
	assert.Equal(t, before, scriptengine.AbandonedScripts())
}

func TestPostScript_ResponseHeadersAndTime(t *testing.T) {
	engine := scriptengine.NewGojaEngine()
	res, err := engine.RunPostScript(context.Background(), `
		console.log(pm.response.headers.get("x-request-id"));
		console.log(String(pm.response.headers.get("X-Missing")));
		console.log(pm.response.headers.has("CONTENT-TYPE"), pm.response.headers.has("X-Missing"));
		console.log(pm.response.responseTime);
	`, userResponseContext())

	require.NoError(t, err)
	assert.Equal(t, []string{"abc-123", "undefined", "true false", "120"}, res.ConsoleOutput)
}

func TestExpect_OkOnUndefinedIsAPlainAssertion(t *testing.T) {
	engine := scriptengine.NewGojaEngine()

	_, err := engine.RunPreScript(context.Background(), `pm.expect(pm.environment.get("missing")).to.not.be.ok`, newContext())
	require.NoError(t, err)

	_, err = engine.RunPreScript(context.Background(), `pm.expect(pm.environment.get("missing")).to.be.ok`, newContext())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "AssertionError: expected undefined to be truthy (line 1, column")

	res, err := engine.RunPostScript(context.Background(), `
		pm.test("missing", function () { pm.expect(undefined).to.be.ok })
		pm.test("negated", function () { pm.expect(undefined).to.not.be.ok })
	`, userResponseContext())
	require.NoError(t, err)
	require.Len(t, res.TestResults, 2)
	assert.Equal(t, "AssertionError: expected undefined to be truthy", res.TestResults[0].Error)
	assert.True(t, res.TestResults[1].Passed)
}

func TestExpect_TestFailuresKeepTheLineUnlessTheyAreAssertions(t *testing.T) {
	res, err := scriptengine.NewGojaEngine().RunPostScript(context.Background(), `pm.test("assert", function () {
  pm.expect(1).to.equal(2)
})
pm.test("type", function () {
  pm.response.json().missing.field
})
pm.test("typo", function () {
  pm.expect(1).to.eqaul(1)
})
pm.test("plain object", function () { throw {} })`, userResponseContext())

	require.NoError(t, err)
	require.Len(t, res.TestResults, 4)
	assert.Equal(t, "AssertionError: expected 1 to equal 2", res.TestResults[0].Error)
	assert.Regexp(t, `^TypeError: Cannot read property 'field' of undefined \(line 5, column \d+\)$`, res.TestResults[1].Error)
	assert.Regexp(t, `^Error: Invalid Chai property: eqaul\. Did you mean "equal"\? \(line 8, column \d+\)$`, res.TestResults[2].Error)
	assert.Regexp(t, `^\[object Object\] \(line 10, column \d+\)$`, res.TestResults[3].Error)
}

func TestExpect_GRPCResponseIsOkOnCodeZero(t *testing.T) {
	sctx := userResponseContext()
	sctx.Protocol = "grpc"
	sctx.ResponseStatusCode = 0
	sctx.ResponseStatusText = "OK"

	res, err := scriptengine.NewGojaEngine().RunPostScript(context.Background(), `
		pm.test("ok", function () { pm.response.to.be.ok; pm.response.to.be.success; pm.response.to.not.be.error })
		pm.test("status", function () { pm.response.to.have.status(0); pm.response.to.have.status("OK") })
	`, sctx)
	require.NoError(t, err)
	for _, tr := range res.TestResults {
		assert.True(t, tr.Passed, "%s: %s", tr.Name, tr.Error)
	}

	sctx.ResponseStatusCode = 14
	sctx.ResponseStatusText = "Unavailable"
	res, err = scriptengine.NewGojaEngine().RunPostScript(context.Background(), `
		pm.test("ok", function () { pm.response.to.be.ok })
		pm.test("error", function () { pm.response.to.be.error })
		pm.test("status", function () { pm.expect(pm.response.status).to.equal("Unavailable") })
	`, sctx)
	require.NoError(t, err)
	require.Len(t, res.TestResults, 3)
	assert.Equal(t, "AssertionError: expected response code to be 0 (OK) but found 14", res.TestResults[0].Error)
	assert.True(t, res.TestResults[1].Passed)
	assert.True(t, res.TestResults[2].Passed, res.TestResults[2].Error)
}
