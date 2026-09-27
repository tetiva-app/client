package publication_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tetiva-app/client/internal/domain/entities"
	"github.com/tetiva-app/client/internal/domain/usecase/publication"
)

var secretAuthFields = map[entities.AuthType]string{
	entities.AuthTypeBasic:    "password",
	entities.AuthTypeBearer:   "token",
	entities.AuthTypeAPIKey:   "value",
	entities.AuthTypeOAuth2:   "clientSecret",
	entities.AuthTypeJWT:      "secret",
	entities.AuthTypeDigest:   "password",
	entities.AuthTypeAWSSigV4: "secretAccessKey",
}

func TestBuild_NoCanaryReachesTheSnapshot(t *testing.T) {
	f := newFixture()
	f.in.Root.GRPCMetadata = []entities.HeaderItem{{Key: "authorization", Value: "CANARY-root-meta", Enabled: true}}
	folder := f.folder(f.in.Root, "Folder")
	folder.GRPCMetadata = []entities.HeaderItem{{Key: "x-session-token", Value: "CANARY-folder-meta", Enabled: true}}
	f.env("prod")

	literalAuth := map[entities.AuthType]string{
		entities.AuthTypeBasic:  `{"username":"CANARY-basic-user","password":"CANARY-basic-pass"}`,
		entities.AuthTypeBearer: `{"prefix":"Bearer","token":"CANARY-bearer","extra":"CANARY-bearer-extra"}`,
		entities.AuthTypeAPIKey: `{"key":"X-Key","value":"CANARY-apikey","addTo":"header"}`,
		entities.AuthTypeOAuth2: `{"grant":"password","tokenUrl":"https://idp.example.com/token?client_secret=CANARY-oauth-query",` +
			`"authUrl":"https://CANARY-oauth-ui:x@idp.example.com/auth","clientId":"CANARY-oauth-id","clientSecret":"CANARY-oauth-secret",` +
			`"username":"CANARY-oauth-user","password":"CANARY-oauth-pass","redirectPort":"CANARY-oauth-port","expires_in":123456789012}`,
		entities.AuthTypeJWT: `{"alg":"HS256","secret":"CANARY-jwt-secret","secretBase64":"CANARY-jwt-b64","privateKey":"CANARY-jwt-key",` +
			`"expiresIn":"CANARY-jwt-exp","claims":{"aud":"https://aud.example.com/?token=CANARY-jwt-claim","n":1}}`,
		entities.AuthTypeDigest:   `{"username":"CANARY-digest-user","password":"CANARY-digest-pass"}`,
		entities.AuthTypeAWSSigV4: `{"region":"eu-central-1","accessKeyId":"CANARY-aws-id","secretAccessKey":"CANARY-aws-secret","sessionToken":"CANARY-aws-session"}`,
		"hawk":                    `{"id":"CANARY-hawk"}`,
	}
	for authType, data := range literalAuth {
		r := f.request(folder, "literal "+string(authType))
		r.AuthType, r.AuthData = authType, data
	}
	f.in.Root.AuthType = entities.AuthTypeBearer
	f.in.Root.AuthData = `{"token":"CANARY-root-auth"}`

	n := 0
	ref := func(place string) string {
		n++
		name := fmt.Sprintf("ref%d", n)
		f.variable(name, "CANARY-ref-"+place, false)
		return "{{" + name + "}}"
	}
	for authType, field := range secretAuthFields {
		r := f.request(folder, "ref "+string(authType))
		r.AuthType = authType
		r.AuthData = fmt.Sprintf(`{%q:%q}`, field, ref("auth-"+string(authType)))
	}

	headers := f.request(folder, "headers")
	headers.URL = "https://admin:CANARY-url-userinfo@api.example.com/x?token=CANARY-query&access_token=" + ref("query") +
		"&page=2#id_token=CANARY-fragment"
	headers.Headers = []entities.HeaderItem{
		{Key: "X-Api-Key", Value: "CANARY-h-list", Enabled: true},
		{Key: "X-Shared-Secret", Value: "CANARY-h-sub", Enabled: true},
		{Key: "X-{{nowhere}}", Value: "CANARY-h-unresolved", Enabled: true},
		{Key: "X-Upstream", Value: "Bearer CANARY-h-bearer", Enabled: false},
		{Key: "Cookie", Value: "sid=CANARY-cookie; " + ref("cookie"), Enabled: true},
		{Key: "X-Api-Key", Value: ref("h-list"), Enabled: true},
		{Key: "X-Shared-Secret", Value: ref("h-sub"), Enabled: true},
		{Key: "X-{{nowhere}}", Value: ref("h-unresolved"), Enabled: true},
		{Key: "X-Upstream", Value: "Bearer " + ref("h-bearer"), Enabled: true},
		{Key: "Authorization", Value: "{{chainA}}", Enabled: true},
	}
	f.variable("chainA", "{{chainB}}", false)
	f.variable("chainB", "CANARY-chain", false)
	ex := f.example(headers, "200")
	ex.Headers = []entities.HeaderItem{{Key: "Set-Cookie", Value: "sid=CANARY-set-cookie", Enabled: true}}
	ex.Body = `{"refresh_token": "CANARY-example-json"}`

	form := f.request(folder, "form")
	form.Method, form.BodyType = entities.MethodPOST, entities.BodyTypeForm
	form.Body = `[{"key":"password","value":"CANARY-form","type":"text","enabled":true},` +
		`{"key":"client_secret","value":"` + ref("form") + `","type":"text","enabled":true}]`

	multipart := f.request(folder, "multipart")
	multipart.Method, multipart.BodyType = entities.MethodPOST, entities.BodyTypeForm
	multipart.Body = `[{"key":"api_key","value":"CANARY-multipart","type":"text","enabled":true},` +
		`{"key":"win","value":"C:\\Users\\CANARYwin\\key.pem","type":"file","enabled":true},` +
		`{"key":"unc","value":"\\\\CANARYunc\\share\\key.pem","type":"file","enabled":true}]`

	binary := f.request(folder, "binary")
	binary.Method, binary.BodyType, binary.Body = entities.MethodPUT, entities.BodyTypeBinary, "/home/CANARYbin/data.bin"

	jsonBody := f.request(folder, "json")
	jsonBody.Method, jsonBody.BodyType = entities.MethodPOST, entities.BodyTypeJSON
	jsonBody.Body = `{"access_token": "CANARY-json", "token": "` + ref("json") + `"}`
	jsonBody.PreScript = "pm.environment.set('x', 'CANARY-script')"

	grpc := f.request(folder, "grpc")
	grpc.Protocol, grpc.URL, grpc.GRPCProtoPath = entities.ProtocolGRPC, "grpc.example.com:443", "/Users/CANARY-proto/api.proto"
	grpc.GRPCMetadata = map[string][]string{"authorization": {"CANARY-meta"}, "x-auth-token": {ref("meta")}}

	gql := f.request(folder, "graphql")
	gql.Protocol, gql.GraphQLSchemaPath = entities.ProtocolGraphQL, "/Users/CANARY-schema/schema.graphql"
	gql.Headers = []entities.HeaderItem{{Key: "X-Api-Key", Value: "CANARY-gql", Enabled: true}}

	ws := f.request(folder, "ws")
	ws.Protocol, ws.URL = entities.ProtocolWebSocket, "wss://ws.example.com/?signature=CANARY-ws-query"
	ws.Headers = []entities.HeaderItem{{Key: "Authorization", Value: "Token CANARY-ws", Enabled: true}}

	userinfo := f.request(folder, "userinfo ref")
	userinfo.URL = "https://" + ref("userinfo") + ":x@api.example.com/"

	f.variable("baseUrl", "https://admin:CANARY-var-userinfo@api.example.com/v1?key=CANARY-var-query", false)
	f.variable("token", "CANARY-secret-var", true)
	f.variable("apiToken", "CANARY-suspicious", false)
	disabled := f.variable("unused", "CANARY-disabled", false)
	disabled.Enabled = false

	out, report := f.marshal(t)

	assert.NotContains(t, out, "CANARY")
	assert.NotContains(t, out, "123456789012")
	assert.Contains(t, out, `"value":"key.pem"`)
	assert.Contains(t, out, `"fileName":"data.bin"`)
	for i := 1; i <= n; i++ {
		name := fmt.Sprintf("ref%d", i)
		hv := hiddenVar(report, name)
		require.NotNil(t, hv, name)
		assert.Equal(t, "referenced", hv.Reason, name)
		assert.True(t, hv.Overridable, name)
	}
	for _, name := range []string{"chainA", "chainB"} {
		require.NotNil(t, hiddenVar(report, name), name)
		assert.Equal(t, "referenced", hiddenVar(report, name).Reason, name)
	}
	assert.Equal(t, "secret", hiddenVar(report, "token").Reason)
	assert.False(t, hiddenVar(report, "token").Overridable)
	assert.Equal(t, "suspicious", hiddenVar(report, "apiToken").Reason)
	assert.Nil(t, hiddenVar(report, "unused"))
	assert.Contains(t, report.PublishedVars, "baseUrl")
	assert.NotEmpty(t, report.Errors, "an unknown auth type blocks publishing")
}

func TestBuild_AuthAllowlist(t *testing.T) {
	f := newFixture()
	r := f.request(f.in.Root, "oauth")
	r.AuthType = entities.AuthTypeOAuth2
	r.AuthData = `{"grant":"client_credentials","tokenUrl":"https://idp.example.com/token","scope":"read",` +
		`"clientId":"{{clientId}}","clientSecret":"lit{{x}}","redirectPort":"21830","enabled":true,"clientAuth":"basic"}`
	s, report := f.build(t)

	auth := findRequest(s.Collection.Items, "oauth").Auth
	require.NotNil(t, auth)
	assert.Equal(t, "oauth2", auth.Type)
	assert.Equal(t, "client_credentials", auth.Fields["grant"])
	assert.Equal(t, "https://idp.example.com/token", auth.Fields["tokenUrl"])
	assert.Equal(t, "read", auth.Fields["scope"])
	assert.Equal(t, "{{clientId}}", auth.Fields["clientId"])
	assert.Equal(t, "", auth.Fields["clientSecret"])
	assert.Equal(t, "", auth.Fields["redirectPort"])
	assert.Equal(t, "", auth.Fields["enabled"])
	assert.Equal(t, []string{"clientSecret", "enabled", "redirectPort"}, auth.Redacted)

	authRedactions := redactionsOf(report, "auth")
	require.Len(t, authRedactions, 3)
	for _, rd := range authRedactions {
		assert.False(t, rd.Overridable)
		assert.True(t, strings.HasPrefix(rd.Selector, f.opaque(r.ID)+"/auth/"), rd.Selector)
	}
}

func TestBuild_AuthLevels(t *testing.T) {
	f := newFixture()
	folder := f.folder(f.in.Root, "Folder")
	folder.AuthType = entities.AuthTypeInherit
	none := f.request(folder, "none")
	none.AuthType = entities.AuthTypeNone
	f.request(folder, "inherit")

	s, _ := f.build(t)

	assert.Nil(t, s.Collection.Auth)
	assert.Nil(t, s.Collection.Items[0].Folder.Auth)
	for _, name := range []string{"none", "inherit"} {
		auth := findRequest(s.Collection.Items, name).Auth
		require.NotNil(t, auth, name)
		assert.Equal(t, name, auth.Type)
		assert.Empty(t, auth.Fields)
		assert.Empty(t, auth.Redacted)
	}
}

func TestBuild_HeaderNameFromPublicVariable(t *testing.T) {
	f := newFixture()
	r := f.request(f.in.Root, "r")
	r.Headers = []entities.HeaderItem{
		{Key: "X-{{tenantHeader}}", Value: "acme", Enabled: true},
		{Key: "{{hdrName}}", Value: "literal-secret", Enabled: true},
		{Key: "Access-Control-Request-Headers", Value: "Authorization", Enabled: true},
	}
	f.variable("tenantHeader", "Tenant", false)
	f.variable("hdrName", "X-Api-Key", false)

	s, report := f.build(t)

	h := findRequest(s.Collection.Items, "r").HTTP.Headers
	require.Len(t, h, 3)
	assert.Equal(t, publication.Header{Key: "X-{{tenantHeader}}", Value: "acme", Enabled: true}, h[0])
	assert.Equal(t, publication.Header{Key: "{{hdrName}}", Value: "<redacted>", Enabled: true, Redacted: true}, h[1])
	assert.Equal(t, "Authorization", h[2].Value)
	assert.Contains(t, report.PublishedVars, "tenantHeader")
	assert.Contains(t, report.PublishedVars, "hdrName")
}

func TestBuild_ReferencedFromHiddenNameBecomesSensitive(t *testing.T) {
	f := newFixture()
	r := f.request(f.in.Root, "r")
	r.Headers = []entities.HeaderItem{{Key: "X-{{secretHeader}}", Value: "{{plain}}", Enabled: true}}
	f.variable("secretHeader", "Tenant", false)
	f.variable("plain", "value", false)

	s, report := f.build(t)

	require.NotNil(t, hiddenVar(report, "secretHeader"))
	assert.Equal(t, "referenced", hiddenVar(report, "plain").Reason)
	assert.Equal(t, &publication.Variable{Key: "plain", Secret: true}, findVariable(s, "plain"))

	f.in.PublishAsIs = []string{hiddenVar(report, "secretHeader").Selector}
	s, report = f.build(t)
	assert.Equal(t, "Tenant", findVariable(s, "secretHeader").Value)
	assert.Equal(t, "value", findVariable(s, "plain").Value, "a resolved, harmless name leaves its value public")
	assert.Nil(t, hiddenVar(report, "plain"))
}

func TestBuild_AuthSchemeFromAVariable(t *testing.T) {
	for name, v := range map[string]struct {
		key    string
		secret bool
	}{
		"public":     {"scheme", false},
		"suspicious": {"authScheme", false},
		"secret":     {"s", true},
	} {
		t.Run(name, func(t *testing.T) {
			f := newFixture()
			r := f.request(f.in.Root, "r")
			r.Headers = []entities.HeaderItem{{Key: "X-Upstream", Value: "{{" + v.key + "}} CANARYabcdefghijklmnopqrst", Enabled: true}}
			f.variable(v.key, "Bearer", v.secret)

			out, report := f.marshal(t)

			assert.NotContains(t, out, "CANARY")
			assert.Len(t, redactionsOf(report, "header"), 1)
			assert.NotNil(t, hiddenVar(report, v.key))
		})
	}
}

func TestBuild_UserinfoInAHostVariable(t *testing.T) {
	f := newFixture()
	f.request(f.in.Root, "direct").URL = "https://{{host}}/x"
	f.request(f.in.Root, "chain").URL = "{{baseUrl}}/pets"
	f.request(f.in.Root, "schemeless chain").URL = "{{gateway}}/x"
	f.request(f.in.Root, "literal").URL = "admin:CANARY-literal@api.example.com/x"
	f.variable("apiHost", "svc:CANARY-chain@api.example.com", false)
	f.variable("baseUrl", "https://{{apiHost}}/v1", false)
	f.variable("host", "admin:CANARY-host@api.example.com", false)
	f.variable("gateway", "{{gatewayHost}}/v2", false)
	f.variable("gatewayHost", "ops@gw.example.com:8443", false)
	f.variable("contact", "ivan:x@example.com", false)

	s, report := f.build(t)
	out, _ := f.marshal(t)

	assert.NotContains(t, out, "CANARY")
	assert.Equal(t, "api.example.com", findVariable(s, "host").Value)
	assert.Equal(t, "api.example.com", findVariable(s, "apiHost").Value)
	assert.Equal(t, "gw.example.com:8443", findVariable(s, "gatewayHost").Value)
	assert.Equal(t, "ivan:x@example.com", findVariable(s, "contact").Value, "a variable outside a URL host keeps its value")
	assert.Equal(t, "api.example.com/x", findRequest(s.Collection.Items, "literal").HTTP.URL)
	assert.Len(t, redactionsOf(report, "url"), 4)
}

func TestBuild_FilePathsKeepOnlyTheBaseName(t *testing.T) {
	for name, path := range map[string]string{
		"unix":          "/home/ivan/keys/key.pem",
		"windows":       `C:\Users\ivan\key.pem`,
		"unc":           `\\srv\share\key.pem`,
		"mixed":         `C:\Users/ivan\key.pem`,
		"reference dir": "{{dir}}/key.pem",
	} {
		t.Run(name, func(t *testing.T) {
			f := newFixture()
			r := f.request(f.in.Root, "r")
			r.BodyType, r.Body = entities.BodyTypeBinary, path
			s, report := f.build(t)
			assert.Equal(t, "key.pem", findRequest(s.Collection.Items, "r").HTTP.Body.FileName)
			require.Len(t, redactionsOf(report, "file"), 1)
		})
	}
}

func TestBuild_ScriptsFollowTheFlag(t *testing.T) {
	f := newFixture()
	f.in.Root.PreScript = "console.log(1)"
	r := f.request(f.in.Root, "r")
	r.PostScript = "pm.test('ok')"

	s, report := f.build(t)
	assert.Nil(t, s.Collection.Scripts)
	assert.Nil(t, findRequest(s.Collection.Items, "r").Scripts)
	assert.Len(t, redactionsOf(report, "script"), 2)

	f.in.IncludeScripts = true
	s, report = f.build(t)
	assert.Equal(t, &publication.Scripts{Pre: "console.log(1)"}, s.Collection.Scripts)
	assert.Equal(t, &publication.Scripts{Post: "pm.test('ok')"}, findRequest(s.Collection.Items, "r").Scripts)
	assert.Empty(t, redactionsOf(report, "script"))
}

func TestBuild_DisabledAndSecretVariables(t *testing.T) {
	f := newFixture()
	f.variable("baseUrl", "https://api.example.com", false)
	off := f.variable("off", "x", false)
	off.Enabled = false
	f.variable("dup", "public-looking", false)
	f.variable("dup", "", true)

	s, report := f.build(t)

	require.NotNil(t, s.Environment)
	assert.Equal(t, "prod", s.Environment.Name)
	assert.Nil(t, findVariable(s, "off"))
	assert.Equal(t, &publication.Variable{Key: "baseUrl", Value: "https://api.example.com"}, findVariable(s, "baseUrl"))
	for _, v := range s.Environment.Variables {
		if v.Key == "dup" {
			assert.Equal(t, publication.Variable{Key: "dup", Secret: true}, v, "one secret row hides every row of that name")
		}
	}
	assert.Equal(t, []string{"baseUrl"}, report.PublishedVars)
}

func TestBuild_VariableValuesFollowTheURLRule(t *testing.T) {
	f := newFixture()
	f.variable("baseUrl", "https://user:pw@api.example.com/v1?sig=abc&page=1", false)
	f.variable("withRefs", "https://{{u}}:{{p}}@api.example.com/", false)

	s, report := f.build(t)

	assert.Equal(t, "https://api.example.com/v1?sig=<redacted>&page=1", findVariable(s, "baseUrl").Value)
	assert.Equal(t, "https://{{u}}:{{p}}@api.example.com/", findVariable(s, "withRefs").Value)
	assert.NotEmpty(t, redactionsOf(report, "url"))
	assert.NotEmpty(t, redactionsOf(report, "query"))
}

func jsonBody(f *fixture, body string) *entities.Request {
	r := f.request(f.in.Root, "r")
	r.Method, r.BodyType, r.Body = entities.MethodPOST, entities.BodyTypeJSON, body
	return r
}

func jwtAuth(f *fixture, data string) *entities.Request {
	r := f.request(f.in.Root, "r")
	r.AuthType, r.AuthData = entities.AuthTypeJWT, data
	return r
}

func assertNoCanary(t *testing.T, cases map[string]func(f *fixture)) {
	t.Helper()
	for name, setup := range cases {
		t.Run(name, func(t *testing.T) {
			f := newFixture()
			setup(f)
			out, report := f.marshal(t)
			assert.NotContains(t, out, "CANARY")
			assert.NotZero(t, len(report.Warnings)+len(report.Redactions)+len(report.HiddenVars), "nothing in the preview")
		})
	}
}

func TestBuild_SecretJSONKeys(t *testing.T) {
	assertNoCanary(t, map[string]func(f *fixture){
		"comments": func(f *fixture) {
			jsonBody(f, "{ /* login */ \"password\":\"CANARY-pw\", // note\n\"client_secret\":\"{{plain}}\" }")
			f.variable("plain", "CANARY-ref", false)
		},
		"trailing comma": func(f *fixture) { jsonBody(f, `{"password":"CANARY-pw",}`) },
		"broken json":    func(f *fixture) { jsonBody(f, `{"password": "CANARY-pw", oops}`) },
		"camelCase and words inside": func(f *fixture) {
			jsonBody(f, `{"apiKey":"CANARY-a","secretKey":"CANARY-b","auth_token":"CANARY-c","api_token":"CANARY-d","X-Oauth":"CANARY-e",`+
				`"authcode":"CANARY-f","sessionkey":"CANARY-g","tokenid":"CANARY-h","api_keys":["CANARY-i"]}`)
		},
		"session token in an example": func(f *fixture) {
			f.example(f.request(f.in.Root, "login"), "200").Body = `{"sessionToken":"CANARY-opaque"}`
		},
		"reference under a camelCase key": func(f *fixture) {
			jsonBody(f, `{"clientSecret":"{{cs}}"}`)
			f.variable("cs", "CANARY-cs", false)
		},
		"header names with a word inside": func(f *fixture) {
			r := f.request(f.in.Root, "r")
			for _, name := range []string{"X-Oauth", "X-Authz", "X-Userauth", "X-Authcode", "X-Tokenid", "X-Secretid", "X-Sessionkey", "X-Apisecretid"} {
				r.Headers = append(r.Headers, entities.HeaderItem{Key: name, Value: "CANARY-" + name, Enabled: true})
			}
		},
		"query and form names with a word inside": func(f *fixture) {
			r := f.request(f.in.Root, "r")
			r.URL = "https://api.example.com/x?authcode=CANARY-1&sessionkey=CANARY-2&tokenid=CANARY-3&secretid=CANARY-4&oauth=CANARY-5"
			r.Method, r.BodyType = entities.MethodPOST, entities.BodyTypeForm
			r.Body = `[{"key":"authcode","value":"CANARY-6","type":"text","enabled":true}]`
		},
	})
}

func TestBuild_JWTClaimsFollowTheirKeys(t *testing.T) {
	assertNoCanary(t, map[string]func(f *fixture){
		"token": func(f *fixture) { jwtAuth(f, `{"alg":"HS256","claims":{"access_token":"CANARY-token"}}`) },
		"nested and array": func(f *fixture) {
			jwtAuth(f, `{"alg":"HS256","claims":{"ctx":{"password":"CANARY-pw"},"sessions":["CANARY-s"]}}`)
		},
		"reference": func(f *fixture) {
			jwtAuth(f, `{"alg":"HS256","claims":{"secret":"{{s}}"}}`)
			f.variable("s", "CANARY-claim", false)
		},
		"templated key": func(f *fixture) {
			jwtAuth(f, `{"alg":"HS256","claims":{"{{claim}}":"CANARY-pw"}}`)
			f.variable("claim", "password", false)
		},
		"jwt header": func(f *fixture) { jwtAuth(f, `{"alg":"HS256","header":{"kid":"k1","x-api-key":"CANARY-h"}}`) },
		"json in a claim string": func(f *fixture) {
			jwtAuth(f, `{"alg":"HS256","claims":{"ctx":"{\"password\":\"CANARY-pw\"}"}}`)
		},
		"redirect path in a claim": func(f *fixture) {
			jwtAuth(f, `{"alg":"HS256","claims":{"redirect":"/cb?access_token=CANARY-t"}}`)
		},
		"reference in json in a claim string": func(f *fixture) {
			jwtAuth(f, `{"alg":"HS256","header":{"ctx":"{\"password\":\"{{pw}}\"}"}}`)
			f.variable("pw", "CANARY-ref", false)
		},
	})

	f := newFixture()
	r := jwtAuth(f, `{"alg":"HS256","queryParam":"access_token","claims":{"sub":"ivan","password":"pw-1","aud":"https://a.example.com/"}}`)
	s, report := f.build(t)
	claims := findRequest(s.Collection.Items, r.Name).Auth.Fields["claims"]
	assert.Equal(t, map[string]any{"sub": "ivan", "password": "<redacted>", "aud": "https://a.example.com/"}, claims)
	assert.Equal(t, "access_token", findRequest(s.Collection.Items, r.Name).Auth.Fields["queryParam"])
	w := warningsOf(report, "json-secret-key")
	require.Len(t, w, 1)
	assert.Equal(t, "r / auth / claims / password", w[0].Path)

	f.in.PublishAsIs = []string{w[0].Selector}
	s, _ = f.build(t)
	assert.Equal(t, "pw-1", findRequest(s.Collection.Items, r.Name).Auth.Fields["claims"].(map[string]any)["password"])
}

func TestBuild_JSONAfterSubstitution(t *testing.T) {
	assertNoCanary(t, map[string]func(f *fixture){
		"templated key": func(f *fixture) {
			jsonBody(f, `{"{{field}}":"CANARY-password"}`)
			f.variable("field", "password", false)
		},
		"key with a hidden reference": func(f *fixture) {
			jsonBody(f, `{"x_{{authField}}":"CANARY-v"}`)
			f.variable("authField", "id", false)
		},
		"body from a variable": func(f *fixture) {
			jsonBody(f, "{{payload}}")
			f.variable("payload", `{"password":"CANARY-password"}`, false)
		},
		"object value from a variable": func(f *fixture) {
			jsonBody(f, `{"data": {{obj}}}`)
			f.variable("obj", `{"token":"CANARY-token"}`, false)
		},
		"fragment through another variable": func(f *fixture) {
			jsonBody(f, "{{wrap}}")
			f.variable("wrap", "{{payload}}", false)
			f.variable("payload", `{"password":"CANARY-password"}`, false)
		},
		"reference inside a fragment": func(f *fixture) {
			jsonBody(f, "{{payload}}")
			f.variable("payload", `{"password":"{{pw}}","user":"ivan"}`, false)
			f.variable("pw", "CANARY-pw", false)
		},
		"graphql variables from a variable": func(f *fixture) {
			r := f.request(f.in.Root, "r")
			r.Protocol, r.GraphQLVariables = entities.ProtocolGraphQL, "{{vars}}"
			f.variable("vars", `{"clientSecret":"CANARY-cs"}`, false)
		},
	})

	f := newFixture()
	r := jsonBody(f, `{"{{field}}":"pw-1","data":{{obj}},"name":{{name}}}`)
	f.variable("field", "password", false)
	f.variable("obj", `{"user":"ivan"}`, false)
	f.variable("name", `"Rex"`, false)
	s, report := f.build(t)
	assert.Equal(t, `{"{{field}}":"<redacted>","data":{{obj}},"name":{{name}}}`, findRequest(s.Collection.Items, r.Name).HTTP.Body.Raw)
	assert.Equal(t, []string{"field", "obj", "name"}, report.PublishedVars)
}

func TestBuild_QueryFromAVariable(t *testing.T) {
	assertNoCanary(t, map[string]func(f *fixture){
		"whole query": func(f *fixture) {
			f.request(f.in.Root, "r").URL = "https://example.com/?{{params}}"
			f.variable("params", "password=CANARY-password", false)
		},
		"query tail": func(f *fixture) {
			f.request(f.in.Root, "r").URL = "https://example.com/x?page=1&{{p}}"
			f.variable("p", "api_key=CANARY-key", false)
		},
		"value that adds a parameter": func(f *fixture) {
			f.request(f.in.Root, "r").URL = "https://example.com/x?page={{page}}"
			f.variable("page", "1&token=CANARY-token", false)
		},
		"fragment": func(f *fixture) {
			f.request(f.in.Root, "r").URL = "https://example.com/cb#{{frag}}"
			f.variable("frag", "access_token=CANARY-token", false)
		},
		"oauth token url": func(f *fixture) {
			r := f.request(f.in.Root, "r")
			r.AuthType, r.AuthData = entities.AuthTypeOAuth2, `{"grant":"client_credentials","tokenUrl":"https://idp.example.com/token?{{qs}}"}`
			f.variable("qs", "client_secret=CANARY-cs", false)
		},
		"whole query with a later question mark": func(f *fixture) {
			f.request(f.in.Root, "r").URL = "https://api.example.com/items?{{params}}"
			f.variable("params", "access_token=CANARY-live&redirect_uri=https://app.example.com/cb?state=1", false)
		},
		"value that adds a parameter before a question mark": func(f *fixture) {
			f.request(f.in.Root, "r").URL = "https://api.example.com/items?page={{page}}"
			f.variable("page", "1&token=CANARY&r=/a?b=c", false)
		},
		"query tail with a later question mark": func(f *fixture) {
			f.request(f.in.Root, "r").URL = "https://api.example.com/items?page=1&{{p}}"
			f.variable("p", "api_key=CANARY&cb=/x?y", false)
		},
		"literal redirect path with its own query": func(f *fixture) {
			f.request(f.in.Root, "r").URL = "https://api.example.com/x?redirect_uri=/cb?access_token=CANARY"
		},
		"literal redirect url with its own query": func(f *fixture) {
			f.request(f.in.Root, "r").URL = "https://api.example.com/login?next=https://app.example.com/cb?token=CANARY"
		},
		"literal query tail with a later question mark": func(f *fixture) {
			f.request(f.in.Root, "r").URL = "https://api.example.com/items?page=1&cb=/x?api_key=CANARY"
		},
		"oauth auth url with a nested query": func(f *fixture) {
			r := f.request(f.in.Root, "r")
			r.AuthType, r.AuthData = entities.AuthTypeOAuth2,
				`{"grant":"authorization_code","authUrl":"https://idp.example.com/authorize?redirect_uri=https://app.example.com/cb?client_secret=CANARY"}`
		},
		"variable value with a nested query": func(f *fixture) {
			f.variable("cb", "https://x.example.com/?next=https://y.example.com/?token=CANARY", false)
		},
	})

	f := newFixture()
	f.request(f.in.Root, "r").URL = "https://example.com/x?page={{page}}&{{filters}}"
	f.variable("page", "2", false)
	f.variable("filters", "sort=name&limit=10", false)
	_, report := f.build(t)
	assert.Equal(t, []string{"page", "filters"}, report.PublishedVars)

	f = newFixture()
	f.request(f.in.Root, "nested").URL = "https://api.example.com/x?redirect_uri=/cb?access_token=t-1&page=2#/home?tab=1"
	s, _ := f.build(t)
	assert.Equal(t, "https://api.example.com/x?redirect_uri=/cb?access_token=<redacted>&page=2#/home?tab=1", findRequest(s.Collection.Items, "nested").HTTP.URL)

	for name, value := range map[string]string{
		"params":  "access_token=live-1&redirect_uri=https://app.example.com/cb?state=1",
		"prose":   "see https://a.example.com/?x=1 and https://b.example.com/?token=t-1",
		"withUrl": "https://idp.example.com/?token=t-1",
	} {
		f = newFixture()
		f.request(f.in.Root, "r").URL = "https://api.example.com/items?{{" + name + "}}"
		f.variable(name, value, false)
		_, report = f.build(t)
		require.NotNil(t, hiddenVar(report, name), name)
		assert.Equal(t, "referenced", hiddenVar(report, name).Reason, name)
	}
}

func TestBuild_FilePathVariablesAreHidden(t *testing.T) {
	assertNoCanary(t, map[string]func(f *fixture){
		"binary body": func(f *fixture) {
			r := f.request(f.in.Root, "r")
			r.Method, r.BodyType, r.Body = entities.MethodPUT, entities.BodyTypeBinary, "{{upload}}"
			f.variable("upload", "/home/CANARY-private-dir/data.bin", false)
		},
		"file field": func(f *fixture) {
			r := f.request(f.in.Root, "r")
			r.Method, r.BodyType = entities.MethodPOST, entities.BodyTypeForm
			r.Body = `[{"key":"doc","value":"{{dir}}/key.pem","type":"file","enabled":true}]`
			f.variable("dir", "/home/CANARY-private-dir", false)
		},
		"through another variable": func(f *fixture) {
			r := f.request(f.in.Root, "r")
			r.Method, r.BodyType, r.Body = entities.MethodPUT, entities.BodyTypeBinary, "{{upload}}"
			f.variable("upload", "{{home}}/data.bin", false)
			f.variable("home", "/Users/CANARY-ivan", false)
		},
	})

	f := newFixture()
	r := f.request(f.in.Root, "r")
	r.Method, r.BodyType, r.Body = entities.MethodPUT, entities.BodyTypeBinary, "{{upload}}"
	f.variable("upload", "/home/ivan/data.bin", false)
	s, report := f.build(t)
	assert.Equal(t, "{{upload}}", findRequest(s.Collection.Items, "r").HTTP.Body.FileName)
	require.NotNil(t, hiddenVar(report, "upload"))
	assert.Equal(t, "referenced", hiddenVar(report, "upload").Reason)
}

func TestBuild_RawFormGraphQLAndSubprotocols(t *testing.T) {
	rawBody := func(f *fixture, body string) *entities.Request {
		r := f.request(f.in.Root, "r")
		r.Method, r.BodyType, r.Body = entities.MethodPOST, entities.BodyTypeRaw, body
		return r
	}
	graphQL := func(f *fixture, query string) *entities.Request {
		r := f.request(f.in.Root, "r")
		r.Protocol, r.URL, r.GraphQLQuery = entities.ProtocolGraphQL, "https://api.example.com/graphql", query
		return r
	}
	ws := func(f *fixture, subprotocol string) *entities.Request {
		r := f.request(f.in.Root, "r")
		r.Protocol, r.URL = entities.ProtocolWebSocket, "wss://ws.example.com/"
		r.Body = `{"version":1,"subprotocols":["` + subprotocol + `"]}`
		return r
	}
	assertNoCanary(t, map[string]func(f *fixture){
		"form-like raw body": func(f *fixture) { rawBody(f, "grant_type=password&password=CANARY-pw") },
		"form-like raw body with a reference": func(f *fixture) {
			rawBody(f, "grant_type=password&client_secret={{cs}}")
			f.variable("cs", "CANARY-cs", false)
		},
		"raw body from a variable": func(f *fixture) {
			rawBody(f, "{{payload}}")
			f.variable("payload", "grant_type=password&password=CANARY-pw", false)
		},
		"literal graphql argument": func(f *fixture) {
			graphQL(f, `mutation { login(user: "ivan", password: "CANARY-pw") { token } }`)
		},
		"graphql argument reference": func(f *fixture) {
			graphQL(f, `mutation { login(user: "ivan", apiKey: "{{k}}") { ok } }`)
			f.variable("k", "CANARY-k", false)
		},
		"bearer subprotocol": func(f *fixture) { ws(f, "base64url.bearer.authorization.k8s.io.CANARYdG9rZW4") },
		"bearer subprotocol reference": func(f *fixture) {
			ws(f, "base64url.bearer.authorization.k8s.io.{{t}}")
			f.variable("t", "CANARY-t", false)
		},
		"ApiKey scheme": func(f *fixture) {
			f.request(f.in.Root, "r").Headers = []entities.HeaderItem{
				{Key: "X-Custom", Value: "ApiKey CANARY-literal", Enabled: true},
				{Key: "X-Other", Value: "Api-Key {{k}}", Enabled: true},
			}
			f.variable("k", "CANARY-ref", false)
		},
	})

	f := newFixture()
	raw := rawBody(f, "grant_type=password&username=ivan&password=pw-1")
	gql := graphQL(f, `mutation { login(user: "ivan", password: "pw-2") { token } }`)
	sock := ws(f, "base64url.bearer.authorization.k8s.io.dG9rZW4tMQ")
	raw.Name, gql.Name, sock.Name = "raw", "gql", "ws"
	f.request(f.in.Root, "h").Headers = []entities.HeaderItem{{Key: "X-Custom", Value: "ApiKey k-1", Enabled: true}}
	xml := f.request(f.in.Root, "xml")
	xml.Method, xml.BodyType, xml.Body = entities.MethodPOST, entities.BodyTypeXML, `<a b="c=d&e=f"/>`
	s, report := f.build(t)

	assert.Equal(t, "grant_type=password&username=ivan&password=<redacted>", findRequest(s.Collection.Items, raw.Name).HTTP.Body.Raw)
	assert.Equal(t, `mutation { login(user: "ivan", password: "<redacted>") { token } }`, findRequest(s.Collection.Items, gql.Name).GraphQL.Query)
	assert.Equal(t, []string{"base64url.bearer.authorization.k8s.io.<redacted>"}, findRequest(s.Collection.Items, sock.Name).WebSocket.Subprotocols)
	assert.Equal(t, "ApiKey <redacted>", findRequest(s.Collection.Items, "h").HTTP.Headers[0].Value)
	assert.Equal(t, `<a b="c=d&e=f"/>`, findRequest(s.Collection.Items, "xml").HTTP.Body.Raw)
	form := redactionsOf(report, "form")
	require.Len(t, form, 1)
	assert.Equal(t, "raw / body / password", form[0].Path)
	gw := warningsOf(report, "graphql-secret-argument")
	require.Len(t, gw, 1)
	require.Len(t, warningsOf(report, "bearer-token"), 1)

	f.in.PublishAsIs = []string{gw[0].Selector, warningsOf(report, "bearer-token")[0].Selector}
	s, _ = f.build(t)
	assert.Contains(t, findRequest(s.Collection.Items, gql.Name).GraphQL.Query, `"pw-2"`)
	assert.Equal(t, []string{"base64url.bearer.authorization.k8s.io.dG9rZW4tMQ"}, findRequest(s.Collection.Items, sock.Name).WebSocket.Subprotocols)
}

func TestBuild_JSONInsideRawBodiesFormValuesAndTextMessages(t *testing.T) {
	assertNoCanary(t, map[string]func(f *fixture){
		"raw body": func(f *fixture) {
			r := f.request(f.in.Root, "r")
			r.Method, r.BodyType, r.Body = entities.MethodPOST, entities.BodyTypeRaw, `{"password": "CANARY-pw"}`
		},
		"form field value": func(f *fixture) {
			r := f.request(f.in.Root, "r")
			r.Method, r.BodyType = entities.MethodPOST, entities.BodyTypeForm
			r.Body = `[{"key":"meta","value":"{\"clientSecret\":\"CANARY-cs\"}","type":"text","enabled":true}]`
		},
		"form field value from a variable": func(f *fixture) {
			r := f.request(f.in.Root, "r")
			r.Method, r.BodyType = entities.MethodPOST, entities.BodyTypeForm
			r.Body = `[{"key":"meta","value":"{{meta}}","type":"text","enabled":true}]`
			f.variable("meta", `{"token":"CANARY-t"}`, false)
		},
		"text message": func(f *fixture) {
			r := f.request(f.in.Root, "r")
			r.Protocol, r.URL = entities.ProtocolWebSocket, "wss://ws.example.com/"
			r.Body = `{"version":1,"messages":[{"id":"1","name":"auth","format":"text","data":"{\"type\":\"auth\",\"token\":\"CANARY-t\"}"}]}`
		},
		"text message reference": func(f *fixture) {
			r := f.request(f.in.Root, "r")
			r.Protocol, r.URL = entities.ProtocolWebSocket, "wss://ws.example.com/"
			r.Body = `{"version":1,"messages":[{"id":"1","name":"auth","format":"text","data":"{\"type\":\"auth\",\"token\":\"{{t}}\"}"}]}`
			f.variable("t", "CANARY-t", false)
		},
	})
}

func TestBuild_JSONInsideOtherText(t *testing.T) {
	htmlExample := func(f *fixture, body string) {
		e := f.example(f.request(f.in.Root, "callback"), "200")
		e.Body, e.ContentType = body, "text/html"
	}
	assertNoCanary(t, map[string]func(f *fixture){
		"html example on one line": func(f *fixture) {
			htmlExample(f, `<p>Signed in. Return to https://app.example.com now.</p>`+
				`<script>window.opener.postMessage({"sessionToken":"CANARY-opaque"},"*")</script>`)
		},
		"http dump with a glob accept": func(f *fixture) {
			f.example(f.request(f.in.Root, "echo"), "200").Body = "GET /login HTTP/1.1\r\nAccept: */*\r\n\r\n{\"password\":\"CANARY-pw\"}"
		},
		"stray quote before the json": func(f *fixture) {
			f.example(f.request(f.in.Root, "echo"), "200").Body = "Display: 15\" wide\n{\"password\":\"CANARY-pw\"}"
		},
		"reference after a stray quote": func(f *fixture) {
			f.example(f.request(f.in.Root, "echo"), "200").Body = "Display: 15\" wide\n{\"password\":\"{{pw}}\"}"
			f.variable("pw", "CANARY-pw", false)
		},
		"line comment in prose": func(f *fixture) {
			htmlExample(f, `<p>Mount //srv/share first.</p><script>send({"apiKey":"CANARY-k"})</script>`)
		},
		"unquoted url before a secret": func(f *fixture) {
			jsonBody(f, `{"callback": https://app.example.com/cb, "client_secret": "CANARY"}`)
		},
		"reference after an unquoted url": func(f *fixture) {
			jsonBody(f, `{"callback": https://app.example.com/cb, "token": {{t}}}`)
			f.variable("t", "CANARY-t", false)
		},
		"reference after a glob": func(f *fixture) {
			jsonBody(f, `{"accept": */*, "token": {{t}}, "note": "a */ b"}`)
			f.variable("t", "CANARY-t", false)
		},
		"reference after an unterminated block comment": func(f *fixture) {
			f.example(f.request(f.in.Root, "echo"), "200").Body = "Accept: /*\r\n\r\n{\"token\": {{t}}}"
			f.variable("t", "CANARY-t", false)
		},
		"quote inside a block comment": func(f *fixture) {
			jsonBody(f, `{ /* don't "quote */ "password": "CANARY", "token": {{t}} }`)
			f.variable("t", "CANARY-t", false)
		},
		"quote inside a line comment": func(f *fixture) {
			jsonBody(f, "{ // don't \"quote\n \"token\": {{t}} }")
			f.variable("t", "CANARY-t", false)
		},
	})
}

func TestBuild_JSONStoredInAString(t *testing.T) {
	assertNoCanary(t, map[string]func(f *fixture){
		"lambda envelope example": func(f *fixture) {
			f.example(f.request(f.in.Root, "fn"), "200").Body = `{"statusCode":200,"body":"{\"password\":\"CANARY-pw\"}"}`
		},
		"payload in a body": func(f *fixture) { jsonBody(f, `{"payload":"{\"access_token\":\"CANARY-t\"}"}`) },
		"two levels": func(f *fixture) {
			jsonBody(f, `{"a":"{\"b\":\"{\\\"clientSecret\\\":\\\"CANARY-cs\\\"}\"}"}`)
		},
		"reference inside": func(f *fixture) {
			jsonBody(f, `{"payload":"{\"token\":\"{{t}}\"}"}`)
			f.variable("t", "CANARY-t", false)
		},
	})

	f := newFixture()
	r := jsonBody(f, `{"body":"{\"password\":\"pw-1\",\"user\":\"ivan\"}","meta":"{\"user\":\"ivan\"}","n":"say \"hi\""}`)
	s, report := f.build(t)
	assert.Equal(t, `{"body":"<redacted>","meta":"{\"user\":\"ivan\"}","n":"say \"hi\""}`, findRequest(s.Collection.Items, r.Name).HTTP.Body.Raw)
	assert.Len(t, warningsOf(report, "json-secret-key"), 1)
}

func TestBuild_GraphQLBlockStringsDefaultsAndQueryVariables(t *testing.T) {
	graphQL := func(f *fixture, query string) *entities.Request {
		r := f.request(f.in.Root, "r")
		r.Protocol, r.URL, r.GraphQLQuery = entities.ProtocolGraphQL, "https://api.example.com/graphql", query
		return r
	}
	assertNoCanary(t, map[string]func(f *fixture){
		"block string": func(f *fixture) { graphQL(f, `mutation { login(password: """CANARY-pw""") { ok } }`) },
		"block string reference": func(f *fixture) {
			graphQL(f, `mutation { login(password: """{{pw}}""") { ok } }`)
			f.variable("pw", "CANARY-pw", false)
		},
		"variable default": func(f *fixture) {
			graphQL(f, `mutation($password: String! = "CANARY-pw") { login(password: $password) { ok } }`)
		},
		"query from a variable": func(f *fixture) {
			graphQL(f, "{{q}}")
			f.variable("q", `mutation { login(password: "CANARY-pw") { ok } }`, false)
		},
	})

	f := newFixture()
	r := graphQL(f, "mutation($apiKey: [String] = \"k-1\", $limit: Int = 5) {\n  login(password: \"\"\"pw-1\"\"\", note: \"\"\"hi\"\"\") { ok }\n}")
	s, report := f.build(t)
	assert.Equal(t, "mutation($apiKey: [String] = \"<redacted>\", $limit: Int = 5) {\n  login(password: \"\"\"<redacted>\"\"\", note: \"\"\"hi\"\"\") { ok }\n}",
		findRequest(s.Collection.Items, r.Name).GraphQL.Query)
	assert.Len(t, warningsOf(report, "graphql-secret-argument"), 2)
}

func TestBuild_SubprotocolTokens(t *testing.T) {
	ws := func(f *fixture, subprotocols ...string) *entities.Request {
		r := f.request(f.in.Root, "r")
		r.Protocol, r.URL = entities.ProtocolWebSocket, "wss://ws.example.com/"
		quoted := make([]string, len(subprotocols))
		for i, s := range subprotocols {
			quoted[i] = fmt.Sprintf("%q", s)
		}
		r.Body = `{"version":1,"subprotocols":[` + strings.Join(quoted, ",") + `]}`
		return r
	}
	header := func(f *fixture, value string) {
		r := ws(f)
		r.Headers = []entities.HeaderItem{{Key: "Sec-WebSocket-Protocol", Value: value, Enabled: true}}
	}
	assertNoCanary(t, map[string]func(f *fixture){
		"entry after access_token": func(f *fixture) { ws(f, "access_token", "CANARYopaque") },
		"entry after bearer":       func(f *fixture) { ws(f, "graphql-ws", "Bearer", "CANARYopaque") },
		"reference after access_token": func(f *fixture) {
			ws(f, "access_token", "{{t}}")
			f.variable("t", "CANARY-t", false)
		},
		"whole entry from a variable": func(f *fixture) {
			ws(f, "{{sub}}")
			f.variable("sub", "base64url.bearer.authorization.k8s.io.CANARYdG9r", false)
		},
		"header":      func(f *fixture) { header(f, "base64url.bearer.authorization.k8s.io.CANARYdG9r, base64.binary.k8s.io") },
		"header list": func(f *fixture) { header(f, "access_token, CANARYopaque") },
		"header from a variable": func(f *fixture) {
			header(f, "{{sub}}")
			f.variable("sub", "base64url.bearer.authorization.k8s.io.CANARYdG9r", false)
		},
	})

	f := newFixture()
	r := ws(f, "v1.token.example", "chat", "access_token", "t-1")
	r.Headers = []entities.HeaderItem{{Key: "Sec-WebSocket-Protocol", Value: "graphql-ws, chat", Enabled: true}}
	s, report := f.build(t)
	out := findRequest(s.Collection.Items, r.Name).WebSocket
	assert.Equal(t, []string{"v1.token.example", "chat", "access_token", "<redacted>"}, out.Subprotocols)
	assert.Equal(t, "graphql-ws, chat", out.Headers[0].Value)
	assert.Len(t, warningsOf(report, "bearer-token"), 1)
}

func TestBuild_FormLikeRawBodyOnSeveralLines(t *testing.T) {
	rawBody := func(f *fixture, body string) *entities.Request {
		r := f.request(f.in.Root, "r")
		r.Method, r.BodyType, r.Body = entities.MethodPOST, entities.BodyTypeRaw, body
		return r
	}
	assertNoCanary(t, map[string]func(f *fixture){
		"break after an ampersand": func(f *fixture) { rawBody(f, "user=ivan&\npassword=CANARY-pw") },
		"space in a value":         func(f *fixture) { rawBody(f, "user=ivan&password=my CANARY-pw") },
		"one pair per line":        func(f *fixture) { rawBody(f, "token=CANARY-t\nuser=ivan\n") },
		"reference on a later line": func(f *fixture) {
			rawBody(f, "user=ivan\nclient_secret={{cs}}")
			f.variable("cs", "CANARY-cs", false)
		},
		"variable holding lines": func(f *fixture) {
			rawBody(f, "{{payload}}")
			f.variable("payload", "user=ivan\npassword=CANARY-pw", false)
		},
		"variable with a later question mark": func(f *fixture) {
			rawBody(f, "{{payload}}")
			f.variable("payload", "password=CANARY&return=/x?y=1", false)
		},
		"form field from a variable with a later question mark": func(f *fixture) {
			r := f.request(f.in.Root, "r")
			r.Method, r.BodyType = entities.MethodPOST, entities.BodyTypeForm
			r.Body = `[{"key":"q","value":"{{q}}","type":"text","enabled":true}]`
			f.variable("q", "token=CANARY&u=/a?b", false)
		},
	})

	f := newFixture()
	r := rawBody(f, "user=ivan\r\npassword=pw-1\r\n")
	prose := rawBody(f, "Dear team,\nplease use key = 42 & more")
	prose.Name = "prose"
	s, report := f.build(t)
	assert.Equal(t, "user=ivan\r\npassword=<redacted>\r\n", findRequest(s.Collection.Items, r.Name).HTTP.Body.Raw)
	assert.Equal(t, prose.Body, findRequest(s.Collection.Items, "prose").HTTP.Body.Raw)
	require.Len(t, redactionsOf(report, "form"), 1)
	assert.Equal(t, "r / body / password", redactionsOf(report, "form")[0].Path)
}

func TestBuild_ShortPasswordNames(t *testing.T) {
	assertNoCanary(t, map[string]func(f *fixture){
		"pwd json key":  func(f *fixture) { jsonBody(f, `{"login":"u","pwd":"CANARY-pw"}`) },
		"pass json key": func(f *fixture) { jsonBody(f, `{"user":"u","pass":"CANARY-pw"}`) },
		"pass query": func(f *fixture) {
			f.request(f.in.Root, "r").URL = "https://api.example.com/login?user=u&pass=CANARY-pw"
		},
		"passphrase head": func(f *fixture) {
			f.request(f.in.Root, "r").Headers = []entities.HeaderItem{{Key: "X-Passphrase", Value: "CANARY-pw", Enabled: true}}
		},
	})

	f := newFixture()
	body := `{"compass":"north","bypass":"cache","passport":"visa"}`
	r := jsonBody(f, body)
	s, _ := f.build(t)
	assert.Equal(t, body, findRequest(s.Collection.Items, r.Name).HTTP.Body.Raw)
}

func TestBuild_XMLAndVariableValues(t *testing.T) {
	xmlBody := func(f *fixture, body string) *entities.Request {
		r := f.request(f.in.Root, "r")
		r.Method, r.BodyType, r.Body = entities.MethodPOST, entities.BodyTypeXML, body
		return r
	}
	assertNoCanary(t, map[string]func(f *fixture){
		"ws-security password": func(f *fixture) {
			xmlBody(f, `<wsse:UsernameToken><wsse:Username>ivan</wsse:Username>`+
				`<wsse:Password Type="#PasswordText">CANARY-pw</wsse:Password></wsse:UsernameToken>`)
		},
		"attribute": func(f *fixture) { xmlBody(f, `<login user="ivan" password="CANARY-pw"/>`) },
		"soap response": func(f *fixture) {
			f.example(f.request(f.in.Root, "soap"), "200").Body = "<LoginResult>\n  <AuthToken>CANARY-t</AuthToken>\n</LoginResult>"
		},
		"element reference": func(f *fixture) {
			xmlBody(f, `<Password>{{pw}}</Password>`)
			f.variable("pw", "CANARY-pw", false)
		},
		"xml from a variable": func(f *fixture) {
			xmlBody(f, `{{envelope}}`)
			f.variable("envelope", `<Password>CANARY-pw</Password>`, false)
		},
		"json in a header variable": func(f *fixture) {
			f.request(f.in.Root, "r").Headers = []entities.HeaderItem{{Key: "X-Meta", Value: "{{meta}}", Enabled: true}}
			f.variable("meta", `{"token":"CANARY-t"}`, false)
		},
		"json in an unused variable": func(f *fixture) { f.variable("settings", `{"password":"CANARY-pw"}`, false) },
	})

	f := newFixture()
	r := xmlBody(f, `<wsse:Password Type="x">pw-1</wsse:Password><note>hi</note><Key>photos/cat.jpg</Key><creds password='pw-2'/>`)
	s, report := f.build(t)
	assert.Equal(t, `<wsse:Password Type="x"><redacted></wsse:Password><note>hi</note><Key>photos/cat.jpg</Key><creds password='<redacted>'/>`,
		findRequest(s.Collection.Items, r.Name).HTTP.Body.Raw)
	assert.Len(t, warningsOf(report, "xml-secret-field"), 2)

	f = newFixture()
	placeholder := `{"hint":"send <api_key> here","usage":"Bearer <token>","name":"x"}`
	r = jsonBody(f, placeholder)
	s, report = f.build(t)
	assert.Equal(t, placeholder, findRequest(s.Collection.Items, r.Name).HTTP.Body.Raw)
	assert.Empty(t, report.Warnings)
}

func TestBuild_OrdinaryNamesStayReadable(t *testing.T) {
	f := newFixture()
	r := f.request(f.in.Root, "r")
	r.URL = "https://api.example.com/items?pageToken=CgRwYWdl&next_token=abc&sort_key=name"
	r.Headers = []entities.HeaderItem{{Key: "Idempotency-Key", Value: "5f1c7c1e-2d3a-4b5c-9d8e-7f6a5b4c3d2e", Enabled: true}}
	r.Method, r.BodyType = entities.MethodPOST, entities.BodyTypeForm
	r.Body = `[{"key":"sort_key","value":"name","type":"text","enabled":true}]`
	discovery := `{"token_endpoint":"https://idp.example.com/token","keyId":"k1","token_type":"Bearer"}`
	f.example(r, "200").Body = discovery

	s, report := f.build(t)

	out := findRequest(s.Collection.Items, "r")
	assert.Equal(t, r.URL, out.HTTP.URL)
	assert.Equal(t, "5f1c7c1e-2d3a-4b5c-9d8e-7f6a5b4c3d2e", out.HTTP.Headers[0].Value)
	assert.Equal(t, "name", out.HTTP.Body.Fields[0].Value)
	assert.Equal(t, discovery, out.Examples[0].Body)
	assert.Empty(t, report.Redactions)
	assert.Empty(t, report.Warnings)
}

func TestBuild_FormEncodedExamplesAndValues(t *testing.T) {
	assertNoCanary(t, map[string]func(f *fixture){
		"oauth1 token response": func(f *fixture) {
			f.example(f.request(f.in.Root, "request_token"), "200").Body = "oauth_token=CANARY-t&oauth_token_secret=CANARY-s&oauth_callback_confirmed=true"
		},
		"token response": func(f *fixture) {
			f.example(f.request(f.in.Root, "token"), "200").Body = "access_token=CANARY-opaque&scope=repo&token_type=bearer"
		},
		"reference in an example": func(f *fixture) {
			f.example(f.request(f.in.Root, "token"), "200").Body = "access_token={{t}}&scope=repo"
			f.variable("t", "CANARY-t", false)
		},
		"form field value": func(f *fixture) {
			r := f.request(f.in.Root, "r")
			r.Method, r.BodyType = entities.MethodPOST, entities.BodyTypeForm
			r.Body = `[{"key":"data","value":"user=u&password=CANARY-pw","type":"text","enabled":true}]`
		},
		"text message": func(f *fixture) {
			r := f.request(f.in.Root, "r")
			r.Protocol, r.URL = entities.ProtocolWebSocket, "wss://ws.example.com/"
			r.Body = `{"version":1,"messages":[{"id":"1","name":"a","format":"text","data":"token=CANARY-t&x=1"}]}`
		},
		"unused variable": func(f *fixture) { f.variable("login", "user=ivan&password=CANARY-pw", false) },
		"value with its own query": func(f *fixture) {
			f.example(f.request(f.in.Root, "login"), "302").Body = "next=/cb?token=CANARY&state=s"
		},
		"raw body value with its own query": func(f *fixture) {
			r := f.request(f.in.Root, "r")
			r.Method, r.BodyType, r.Body = entities.MethodPOST, entities.BodyTypeRaw, "next=/cb?token=CANARY&state=s"
		},
	})

	f := newFixture()
	r := f.request(f.in.Root, "token")
	f.example(r, "200").Body = "access_token=tok-1&scope=repo&token_type=bearer"
	f.example(r, "lines").Body = "access_token=tok-2\nscope=repo"
	s, report := f.build(t)
	assert.Equal(t, "access_token=<redacted>&scope=repo&token_type=bearer", findRequest(s.Collection.Items, r.Name).Examples[0].Body)
	assert.Equal(t, "access_token=<redacted>\nscope=repo", findRequest(s.Collection.Items, r.Name).Examples[1].Body)
	w := warningsOf(report, "form-secret-field")
	require.Len(t, w, 2)
	assert.Equal(t, "token / examples / 200 / body", w[0].Path)

	f.in.PublishAsIs = []string{w[0].Selector}
	s, _ = f.build(t)
	assert.Equal(t, "access_token=tok-1&scope=repo&token_type=bearer", findRequest(s.Collection.Items, r.Name).Examples[0].Body)
}

func TestBuild_URLsInsideBodies(t *testing.T) {
	example := func(f *fixture, body string) { f.example(f.request(f.in.Root, "echo"), "200").Body = body }
	assertNoCanary(t, map[string]func(f *fixture){
		"echo example": func(f *fixture) {
			example(f, `{"args":{"api_key":"k-1"},"url":"https://httpbin.org/get?api_key=CANARY-k"}`)
		},
		"path":     func(f *fixture) { example(f, `{"next":"/v1/items?cursor=c1&access_token=CANARY"}`) },
		"fragment": func(f *fixture) { example(f, `{"redirect":"https://app.example.com/cb#access_token=CANARY&state=s"}`) },
		"after a base url reference": func(f *fixture) {
			jsonBody(f, `{"callback":"{{host}}/cb?token=CANARY"}`)
			f.variable("host", "https://app.example.com", false)
		},
		"html link": func(f *fixture) {
			example(f, `<p><a href="https://x.example.com/?token=CANARY">continue</a></p>`)
		},
		"json stored in a string": func(f *fixture) {
			example(f, `{"body":"{\"url\":\"https://x.example.com/?token=CANARY\"}"}`)
		},
		"whole body": func(f *fixture) { example(f, "https://app.example.com/cb?code=CANARY&state=s1") },
		"reference": func(f *fixture) {
			jsonBody(f, `{"url":"https://x.example.com/?api_key={{k}}"}`)
			f.variable("k", "CANARY-k", false)
		},
		"url with a nested query": func(f *fixture) {
			jsonBody(f, `{"url":"https://x.example.com/?next=https://y.example.com/?token=CANARY"}`)
		},
		"path with a nested query": func(f *fixture) { example(f, `{"next":"/login?redirect=/cb?access_token=CANARY"}`) },
	})

	f := newFixture()
	r := f.request(f.in.Root, "echo")
	f.example(r, "200").Body = `{"args":{"api_key":"k-1"},"url":"https://httpbin.org/get?api_key=k-2&page=2"}`
	f.example(r, "text").Body = "https://app.example.com/cb?code=c-1 (expires in 60 s)"
	ordinary := []string{
		`{"url":"https://api.example.com/items?page=2&sort=name","next":"/v1/items?cursor=abc","q":"why? token=a b"}`,
		`{"u":"https://x.example.com/cb","q":"a?token=b"}`,
		"Read https://x.example.com/docs and ask?token=abc",
	}
	for i, body := range ordinary {
		f.example(r, fmt.Sprint("ordinary ", i)).Body = body
	}
	s, report := f.build(t)
	out := findRequest(s.Collection.Items, r.Name)
	assert.Equal(t, `{"args":{"api_key":"<redacted>"},"url":"https://httpbin.org/get?api_key=<redacted>&page=2"}`, out.Examples[0].Body)
	assert.Equal(t, "https://app.example.com/cb?code=<redacted> (expires in 60 s)", out.Examples[1].Body)
	for i, body := range ordinary {
		assert.Equal(t, body, out.Examples[i+2].Body)
	}
	assert.Len(t, warningsOf(report, "json-secret-key"), 1)
	assert.Len(t, warningsOf(report, "url-secret-parameter"), 2)
}

func TestBuild_JSONInHeadersMetadataAndDescriptions(t *testing.T) {
	header := func(f *fixture, key, value string) {
		f.request(f.in.Root, "r").Headers = []entities.HeaderItem{{Key: key, Value: value, Enabled: true}}
	}
	assertNoCanary(t, map[string]func(f *fixture){
		"header": func(f *fixture) { header(f, "X-Context", `{"session_token":"CANARY"}`) },
		"header reference": func(f *fixture) {
			header(f, "X-Context", `{"session_token":"{{st}}"}`)
			f.variable("st", "CANARY-st", false)
		},
		"grpc metadata": func(f *fixture) {
			r := f.request(f.in.Root, "g")
			r.Protocol, r.URL = entities.ProtocolGRPC, "grpc://h:1"
			r.GRPCMetadata = map[string][]string{"x-ctx": {`{"password":"CANARY"}`}}
		},
		"collection metadata": func(f *fixture) {
			f.in.Root.GRPCMetadata = []entities.HeaderItem{{Key: "x-ctx", Value: `{"password":"CANARY"}`, Enabled: true}}
		},
		"example header": func(f *fixture) {
			f.example(f.request(f.in.Root, "r"), "200").Headers = []entities.HeaderItem{{Key: "X-Debug", Value: `{"token":"CANARY"}`, Enabled: true}}
		},
		"location header": func(f *fixture) {
			f.example(f.request(f.in.Root, "r"), "302").Headers = []entities.HeaderItem{
				{Key: "Location", Value: "https://app.example.com/cb#access_token=CANARY&state=s", Enabled: true},
			}
		},
		"request description": func(f *fixture) {
			f.request(f.in.Root, "d").Description = "Example:\n```json\n{\"password\": \"CANARY\"}\n```"
		},
		"folder description": func(f *fixture) { f.folder(f.in.Root, "auth").Description = `Send {"apiKey":"CANARY"} first.` },
		"description reference": func(f *fixture) {
			f.request(f.in.Root, "d").Description = `{"password":"{{pw}}"}`
			f.variable("pw", "CANARY-pw", false)
		},
		"folder description reference": func(f *fixture) {
			f.folder(f.in.Root, "auth").Description = `Send {"apiKey":"{{k}}"} first.`
			f.variable("k", "CANARY-k", false)
		},
		"url in description prose": func(f *fixture) {
			f.request(f.in.Root, "d").Description = "Open https://api.example.com/x?api_key=CANARY in a browser."
		},
		"curl in a description": func(f *fixture) {
			f.request(f.in.Root, "d").Description = "Try:\n\ncurl 'https://api.example.com/x?page=1&access_token=CANARY'"
		},
		"templated curl in a description": func(f *fixture) {
			f.request(f.in.Root, "d").Description = "curl \"{{baseUrl}}/pets?api_key={{demo}}\""
			f.variable("demo", "CANARY-demo", false)
		},
		"link header": func(f *fixture) {
			f.example(f.request(f.in.Root, "l"), "200").Headers = []entities.HeaderItem{
				{Key: "Link", Value: `<https://api.example.com/x?page=2&access_token=CANARY>; rel="next"`, Enabled: true},
			}
		},
		"url in a text example": func(f *fixture) {
			f.example(f.request(f.in.Root, "e"), "302").Body = "Redirecting to https://app.example.com/cb?token=CANARY ..."
		},
	})

	f := newFixture()
	r := f.request(f.in.Root, "r")
	r.Headers = []entities.HeaderItem{
		{Key: "X-Context", Value: `{"session_token":"s-1","locale":"en"}`, Enabled: true},
		{Key: "Cache-Control", Value: "max-age=0", Enabled: true},
		{Key: "Accept", Value: "application/json; q=0.9", Enabled: true},
	}
	r.Description = "Returns pets.\n\nSee https://docs.example.com/pets?lang=en for details."
	s, report := f.build(t)
	out := findRequest(s.Collection.Items, "r")
	assert.Equal(t, `{"session_token":"<redacted>","locale":"en"}`, out.HTTP.Headers[0].Value)
	assert.Equal(t, "max-age=0", out.HTTP.Headers[1].Value)
	assert.Equal(t, "application/json; q=0.9", out.HTTP.Headers[2].Value)
	assert.Equal(t, r.Description, out.Description)
	assert.Len(t, report.Warnings, 1)
}

func TestBuild_ObjectsUnderSecretKeys(t *testing.T) {
	assertNoCanary(t, map[string]func(f *fixture){
		"password change": func(f *fixture) { jsonBody(f, `{"password":{"old":"CANARY-1","new":"CANARY-2"}}`) },
		"token object in an example": func(f *fixture) {
			f.example(f.request(f.in.Root, "login"), "200").Body = `{"token":{"value":"CANARY","expiresAt":"2026-10-01"}}`
		},
		"list of credential objects": func(f *fixture) { jsonBody(f, `{"credentials":[{"type":"password","value":"CANARY"}]}`) },
		"deeper object":              func(f *fixture) { jsonBody(f, `{"clientSecret":{"current":{"v":"CANARY"}}}`) },
		"reference inside": func(f *fixture) {
			jsonBody(f, `{"password":{"new":"{{pw}}"}}`)
			f.variable("pw", "CANARY-pw", false)
		},
		"templated key": func(f *fixture) {
			jsonBody(f, `{"{{field}}":{"new":"CANARY"}}`)
			f.variable("field", "password", false)
		},
		"json stored in a string": func(f *fixture) {
			f.example(f.request(f.in.Root, "r"), "200").Body = `{"body":"{\"secret\":{\"v\":\"CANARY\"}}"}`
		},
		"jwt claims":      func(f *fixture) { jwtAuth(f, `{"alg":"HS256","claims":{"password":{"old":"CANARY"}}}`) },
		"jwt claims list": func(f *fixture) { jwtAuth(f, `{"alg":"HS256","claims":{"tokens":[{"v":"CANARY"}]}}`) },
		"jwt claims reference": func(f *fixture) {
			jwtAuth(f, `{"alg":"HS256","claims":{"token":{"value":"{{t}}"}}}`)
			f.variable("t", "CANARY-t", false)
		},
	})

	f := newFixture()
	readable := `,"auth":{"type":"bearer"},"keys":[{"kid":"k1","kty":"RSA"}],"session":{"user":"ivan"},"tokenizer":{"name":"bpe"},"pageToken":{"v":"p"}}`
	body := jsonBody(f, `{"password":{"old":"pw-1","new":"pw-2"}`+readable)
	jwt := jwtAuth(f, `{"alg":"HS256","claims":{"password":{"old":"pw-3"},"auth":{"role":"admin"}}}`)
	jwt.Name = "jwt"
	s, report := f.build(t)
	assert.Equal(t, `{"password":{"old":"<redacted>","new":"<redacted>"}`+readable, findRequest(s.Collection.Items, body.Name).HTTP.Body.Raw)
	assert.Equal(t, map[string]any{"password": map[string]any{"old": "<redacted>"}, "auth": map[string]any{"role": "admin"}},
		findRequest(s.Collection.Items, "jwt").Auth.Fields["claims"])
	assert.Len(t, warningsOf(report, "json-secret-key"), 3)
}

func TestBuild_KeyValueLists(t *testing.T) {
	example := func(f *fixture, body string) { f.example(f.request(f.in.Root, "r"), "200").Body = body }
	assertNoCanary(t, map[string]func(f *fixture){
		"postman environment": func(f *fixture) { example(f, `{"values":[{"key":"apiKey","value":"CANARY","enabled":true}]}`) },
		"har headers":         func(f *fixture) { example(f, `{"headers":[{"name":"Authorization","value":"Bearer CANARY-opaque"}]}`) },
		"kubernetes env":      func(f *fixture) { example(f, `{"env":[{"name":"DB_PASSWORD","value":"CANARY"}]}`) },
		"value first":         func(f *fixture) { example(f, `[{"value":"CANARY","Key":"client_secret"}]`) },
		"reference": func(f *fixture) {
			jsonBody(f, `[{"key":"apiKey","value":"{{k}}"}]`)
			f.variable("k", "CANARY-k", false)
		},
	})

	f := newFixture()
	r := f.request(f.in.Root, "r")
	f.example(r, "list").Body = `[{"key":"apiKey","value":"k-1"}]`
	ordinary := []string{
		`{"key":"PROJ-123","fields":{"summary":"x"}}`,
		`{"Tags":[{"Key":"Name","Value":"web"}]}`,
		`[{"key":"Content-Type","value":"application/json"},{"name":"page","value":"2"}]`,
	}
	for i, body := range ordinary {
		f.example(r, fmt.Sprint("ordinary ", i)).Body = body
	}
	s, report := f.build(t)
	out := findRequest(s.Collection.Items, "r")
	assert.Equal(t, `[{"key":"apiKey","value":"<redacted>"}]`, out.Examples[0].Body)
	for i, body := range ordinary {
		assert.Equal(t, body, out.Examples[i+1].Body)
	}
	assert.Len(t, report.Warnings, 1)
}

func TestBuild_GraphQLListsCDATASubprotocolValuesAndSchemaPaths(t *testing.T) {
	graphQL := func(f *fixture, query string) *entities.Request {
		r := f.request(f.in.Root, "r")
		r.Protocol, r.URL, r.GraphQLQuery = entities.ProtocolGraphQL, "https://api.example.com/graphql", query
		return r
	}
	xmlBody := func(f *fixture, body string) {
		r := f.request(f.in.Root, "r")
		r.Method, r.BodyType, r.Body = entities.MethodPOST, entities.BodyTypeXML, body
	}
	ws := func(f *fixture, subprotocols ...string) *entities.Request {
		r := f.request(f.in.Root, "r")
		r.Protocol, r.URL = entities.ProtocolWebSocket, "wss://ws.example.com/"
		quoted := make([]string, len(subprotocols))
		for i, s := range subprotocols {
			quoted[i] = fmt.Sprintf("%q", s)
		}
		r.Body = `{"version":1,"subprotocols":[` + strings.Join(quoted, ",") + `]}`
		return r
	}
	assertNoCanary(t, map[string]func(f *fixture){
		"graphql list": func(f *fixture) { graphQL(f, `mutation { login(tokens: ["CANARY-a", "CANARY-b"]) { ok } }`) },
		"graphql list reference": func(f *fixture) {
			graphQL(f, `mutation { login(tokens: ["x", {{t}}]) { ok } }`)
			f.variable("t", "CANARY-t", false)
		},
		"graphql list default": func(f *fixture) {
			graphQL(f, `mutation($keys: [String] = ["CANARY"]) { f(ids: $keys) { ok } }`)
		},
		"xml cdata": func(f *fixture) { xmlBody(f, `<Password><![CDATA[CANARY-pw]]></Password>`) },
		"xml cdata reference": func(f *fixture) {
			xmlBody(f, `<Password><![CDATA[{{pw}}]]></Password>`)
			f.variable("pw", "CANARY-pw", false)
		},
		"subprotocol header from a variable list": func(f *fixture) {
			ws(f).Headers = []entities.HeaderItem{{Key: "Sec-WebSocket-Protocol", Value: "{{sub}}", Enabled: true}}
			f.variable("sub", "access_token, CANARYopaque", false)
		},
		"entry after a variable ending in access_token": func(f *fixture) {
			ws(f, "{{first}}", "{{tok}}")
			f.variable("first", "graphql-ws, access_token", false)
			f.variable("tok", "CANARY-t", false)
		},
		"access_token entry": func(f *fixture) { ws(f, "access_token.CANARYopaque") },
		"access_token entry reference": func(f *fixture) {
			ws(f, "access_token.{{t}}")
			f.variable("t", "CANARY-t", false)
		},
		"proto path": func(f *fixture) {
			r := f.request(f.in.Root, "r")
			r.Protocol, r.URL, r.GRPCProtoPath = entities.ProtocolGRPC, "grpc://h:1", "{{protoDir}}/api.proto"
			f.variable("protoDir", "/home/CANARY/protos", false)
		},
		"schema path": func(f *fixture) {
			graphQL(f, "{ pets { id } }").GraphQLSchemaPath = "{{schemaDir}}/schema.graphql"
			f.variable("schemaDir", "/home/CANARY", false)
		},
	})

	f := newFixture()
	gql := graphQL(f, `mutation { login(tokens: ["t-1", "t-2"], tags: ["a"], empty: []) { ok } }`)
	gql.Name = "gql"
	xml := f.request(f.in.Root, "xml")
	xml.Method, xml.BodyType, xml.Body = entities.MethodPOST, entities.BodyTypeXML, `<Password><![CDATA[pw-1]]></Password><Note><![CDATA[hi]]></Note>`
	sock := ws(f, "access_token.t-3", "graphql-ws")
	sock.Name = "ws"
	s, report := f.build(t)
	assert.Equal(t, `mutation { login(tokens: ["<redacted>", "<redacted>"], tags: ["a"], empty: []) { ok } }`, findRequest(s.Collection.Items, "gql").GraphQL.Query)
	assert.Equal(t, `<Password><![CDATA[<redacted>]]></Password><Note><![CDATA[hi]]></Note>`, findRequest(s.Collection.Items, "xml").HTTP.Body.Raw)
	assert.Equal(t, []string{"access_token.<redacted>", "graphql-ws"}, findRequest(s.Collection.Items, "ws").WebSocket.Subprotocols)
	assert.Len(t, warningsOf(report, "graphql-secret-argument"), 2)
	assert.Len(t, warningsOf(report, "xml-secret-field"), 1)
	assert.Len(t, warningsOf(report, "bearer-token"), 1)
}
