package publication_test

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tetiva-app/client/internal/domain/entities"
)

func TestScan_EveryRuleWarnsAndMasks(t *testing.T) {
	cases := map[string]string{
		"jwt":                "eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiIxIn0.c2lnbmF0dXJlc2lnbmF0dXJl",
		"aws-access-key":     "AKIAIOSFODNN7EXAMPLE",
		"private-key":        "-----BEGIN RSA PRIVATE KEY-----\nMIIBOgIBAAJBAKj34\n-----END RSA PRIVATE KEY-----",
		"github-token":       "ghp_abcdefghijklmnopqrstuvwxyz0123456789",
		"slack-token":        "xoxb-1234567890-abcdefghij",
		"stripe-key":         "sk_live_abcdefghijklmnop1234",
		"google-api-key":     "AIzaSyA1234567890abcdefghijklmnopqrstuv",
		"telegram-bot-token": "123456789:AAEhBOweik6ad9r_QXMENQjcrGbqCr4K-ab",
		"bearer-token":       "Bearer abcdefghijklmnopqrstuvwxyz",
	}
	for rule, secret := range cases {
		t.Run(rule, func(t *testing.T) {
			f := newFixture()
			r := f.request(f.in.Root, "r")
			r.Description = "before " + secret + " after"
			s, report := f.build(t)
			found := warningsOf(report, rule)
			require.Len(t, found, 1, "%+v", report.Warnings)
			desc := findRequest(s.Collection.Items, "r").Description
			assert.True(t, strings.HasPrefix(desc, "before "), desc)
			assert.True(t, strings.HasSuffix(desc, " after"), desc)
			assert.Contains(t, desc, "<redacted>")
			assert.NotContains(t, desc, secret)
		})
	}
}

func TestScan_TelegramTokenInTheURLPath(t *testing.T) {
	f := newFixture()
	r := f.request(f.in.Root, "send")
	r.URL = "https://api.telegram.org/bot123456789:AAEhBOweik6ad9r_QXMENQjcrGbqCr4K-ab/sendMessage"

	s, report := f.build(t)

	require.Len(t, warningsOf(report, "telegram-bot-token"), 1)
	assert.Equal(t, "https://api.telegram.org/bot<redacted>/sendMessage", findRequest(s.Collection.Items, "send").HTTP.URL)
}

func TestScan_JSONSecretKeyKeepsFormatting(t *testing.T) {
	f := newFixture()
	r := f.request(f.in.Root, "login")
	r.Method, r.BodyType = entities.MethodPOST, entities.BodyTypeJSON
	r.Body = "{\n  \"user\": \"ivan\",\n  \"Password\" :  \"hunter2\\\"x\",\n  \"nested\": {\"api_key\": \"k-1\"},\n" +
		"  \"token\": \"{{token}}\",\n  \"secret\": 42\n}"
	gql := f.request(f.in.Root, "gql")
	gql.Protocol, gql.GraphQLVariables = entities.ProtocolGraphQL, `{"client_secret":"abc"}`
	ex := f.example(r, "ok")
	ex.Body = `{"id_token":"xyz","name":"n"}`
	notJSON := f.example(r, "text")
	notJSON.Body = `password: "hunter2"`

	s, report := f.build(t)

	out := findRequest(s.Collection.Items, "login")
	assert.Equal(t, "{\n  \"user\": \"ivan\",\n  \"Password\" :  \"<redacted>\",\n  \"nested\": {\"api_key\": \"<redacted>\"},\n"+
		"  \"token\": \"{{token}}\",\n  \"secret\": 42\n}", out.HTTP.Body.Raw)
	assert.Equal(t, `{"client_secret":"<redacted>"}`, findRequest(s.Collection.Items, "gql").GraphQL.Variables)
	assert.Equal(t, `{"id_token":"<redacted>","name":"n"}`, out.Examples[0].Body)
	assert.Equal(t, `password: "hunter2"`, out.Examples[1].Body)
	assert.Len(t, report.Warnings, 4, "the id_token hit is one warning, whichever rule saw it first")
	assert.Len(t, warningsOf(report, "json-secret-key"), 3)
	for _, w := range report.Warnings {
		assert.False(t, w.Overridden)
	}
}

func TestScan_JSONSecretKeyWithUnquotedReferences(t *testing.T) {
	f := newFixture()
	r := f.request(f.in.Root, "create")
	r.Method, r.BodyType = entities.MethodPOST, entities.BodyTypeJSON
	r.Body = `{"id": {{petId}}, "note": "a \"{{hi}}\" b", "password": "CANARY-pw", "client_secret": "{{cs}}", "token": {{tok}}, {{k}}: "v"}`
	f.variable("petId", "7", false)
	f.variable("cs", "CANARY-cs", false)
	f.variable("tok", "CANARY-tok", false)
	f.variable("k", "name", false)

	s, report := f.build(t)
	out, _ := f.marshal(t)

	assert.NotContains(t, out, "CANARY")
	assert.Equal(t, `{"id": {{petId}}, "note": "a \"{{hi}}\" b", "password": "<redacted>", "client_secret": "{{cs}}", "token": {{tok}}, {{k}}: "v"}`,
		findRequest(s.Collection.Items, "create").HTTP.Body.Raw)
	assert.Len(t, warningsOf(report, "json-secret-key"), 1)
	for _, name := range []string{"cs", "tok"} {
		require.NotNil(t, hiddenVar(report, name), name)
		assert.Equal(t, "referenced", hiddenVar(report, name).Reason, name)
	}
	assert.Equal(t, []string{"petId", "k"}, report.PublishedVars)
}

func TestScan_ReferencesAndMasksAreNotFindings(t *testing.T) {
	f := newFixture()
	r := f.request(f.in.Root, "r")
	r.Headers = []entities.HeaderItem{{Key: "Authorization", Value: "Bearer {{token}}", Enabled: true}}
	r.Description = "Bearer <redacted> and Bearer {{token}}"

	_, report := f.build(t)

	assert.Empty(t, report.Warnings)
}

func TestScan_ReferenceInsideAMatchSurvives(t *testing.T) {
	f := newFixture()
	r := f.request(f.in.Root, "r")
	r.Method, r.BodyType = entities.MethodPOST, entities.BodyTypeJSON
	r.Body = `{"access_token":"abc{{suffix}}"}`

	s, _ := f.build(t)

	assert.Equal(t, `{"access_token":"<redacted>{{suffix}}"}`, findRequest(s.Collection.Items, "r").HTTP.Body.Raw)
}

func TestScan_PublishedVariableValues(t *testing.T) {
	f := newFixture()
	v := f.variable("webhook", "https://hooks.example.com/ghp_abcdefghijklmnopqrstuvwxyz0123456789", false)

	s, report := f.build(t)

	gh := warningsOf(report, "github-token")
	require.Len(t, gh, 1)
	assert.Equal(t, "https://hooks.example.com/<redacted>", findVariable(s, "webhook").Value)
	assert.True(t, strings.HasPrefix(gh[0].Selector, f.opaque(v.ID)+"/scan/"), gh[0].Selector)
}

func TestSuspiciousVariableNames(t *testing.T) {
	suspicious := []string{
		"authHeader", "SESSION_ID", "myCookie", "apikey", "signatureBase", "token2",
		"clientSecret", "password", "passphrase", "dbPwd", "credentials", "privateHost",
	}
	plain := []string{"baseUrl", "petId", "user", "host", "limit"}
	f := newFixture()
	for _, name := range append(append([]string{}, suspicious...), plain...) {
		f.variable(name, "v", false)
	}

	_, report := f.build(t)

	for _, name := range suspicious {
		hv := hiddenVar(report, name)
		if assert.NotNil(t, hv, name) {
			assert.Equal(t, "suspicious", hv.Reason, name)
			assert.True(t, hv.Overridable, name)
		}
	}
	assert.Equal(t, plain, report.PublishedVars)
}

func TestScan_JSONSecretKeyInLooseJSON(t *testing.T) {
	f := newFixture()
	jsonc := jsonBody(f, "{ /* login */ \"password\":\"pw-1\", // note\n\"clientSecret\":\"{{plain}}\",\n\"tokens\": [\"t-1\", {\"id\": \"x\"}],}")
	broken := f.request(f.in.Root, "broken")
	broken.Method, broken.BodyType, broken.Body = entities.MethodPOST, entities.BodyTypeJSON, `{"apiKey": "k-1", oops, "note": "n"`
	f.variable("plain", "p-1", false)

	s, report := f.build(t)

	assert.Equal(t, "{ /* login */ \"password\":\"<redacted>\", // note\n\"clientSecret\":\"{{plain}}\",\n\"tokens\": [\"<redacted>\", {\"id\": \"<redacted>\"}],}",
		findRequest(s.Collection.Items, jsonc.Name).HTTP.Body.Raw)
	assert.Equal(t, `{"apiKey": "<redacted>", oops, "note": "n"`, findRequest(s.Collection.Items, "broken").HTTP.Body.Raw)
	assert.Len(t, warningsOf(report, "json-secret-key"), 4)
	require.NotNil(t, hiddenVar(report, "plain"))
	assert.Equal(t, "referenced", hiddenVar(report, "plain").Reason)
}

func TestScan_UnclosedCDATAStaysLinear(t *testing.T) {
	body := strings.Repeat(`<a><![CDATA[x<b>y</b>`, 128<<10/22)
	f := newFixture()
	r := f.request(f.in.Root, "r")
	r.Method, r.BodyType, r.Body = entities.MethodPOST, entities.BodyTypeXML, body
	f.example(r, "200").Body = body

	start := time.Now()
	s, _ := f.build(t)

	assert.Less(t, time.Since(start), time.Second)
	assert.Equal(t, body, findRequest(s.Collection.Items, "r").HTTP.Body.Raw)
}

func TestScan_URLsInsideTextStayLinear(t *testing.T) {
	for name, text := range map[string]string{
		"templated bases": strings.Repeat("{{a}}/", 128<<10/6),
		"schemes":         strings.Repeat("x://", 128<<10/4) + "?page=1",
		"many urls":       strings.Repeat("see https://a.example.com/x?page=1&b=2 ", 128<<10/40),
	} {
		f := newFixture()
		f.request(f.in.Root, "d").Description = text

		start := time.Now()
		s, _ := f.build(t)

		assert.Less(t, time.Since(start), time.Second, name)
		assert.Equal(t, text, findRequest(s.Collection.Items, "d").Description, name)
	}
}

func TestScan_OrdinaryWordsStayReadable(t *testing.T) {
	f := newFixture()
	body := `{"author":"Ivan","keyword":"pets","monkey":"m","code":"NOT_FOUND","tokenizer":"bpe"}`
	r := jsonBody(f, body)
	r.URL = "https://api.example.com/x?keyword=pets&author=ivan"
	r.Headers = []entities.HeaderItem{{Key: "X-Author", Value: "ivan", Enabled: true}, {Key: "Access-Control-Allow-Credentials", Value: "true", Enabled: true}}
	f.example(r, "404").Body = `{"code":"NOT_FOUND","message":"no pet"}`

	s, report := f.build(t)

	out := findRequest(s.Collection.Items, "r")
	assert.Equal(t, body, out.HTTP.Body.Raw)
	assert.Equal(t, "https://api.example.com/x?keyword=pets&author=ivan", out.HTTP.URL)
	assert.Equal(t, "ivan", out.HTTP.Headers[0].Value)
	assert.Equal(t, "true", out.HTTP.Headers[1].Value)
	assert.Equal(t, `{"code":"NOT_FOUND","message":"no pet"}`, out.Examples[0].Body)
	assert.Empty(t, report.Warnings)
	assert.Empty(t, report.Redactions)
}
