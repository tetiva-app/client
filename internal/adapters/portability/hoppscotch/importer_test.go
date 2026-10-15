package hoppscotch

import (
	"encoding/json"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tetiva-app/client/internal/adapters/portability"
	"github.com/tetiva-app/client/internal/domain/entities"
	"github.com/tetiva-app/client/internal/domain/usecase/auth"
	"github.com/tetiva-app/client/internal/domain/usecase/environment"
)

func readFixture(t *testing.T, path ...string) []byte {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(path...))
	require.NoError(t, err)
	return raw
}

func petstore(t *testing.T) *builder {
	t.Helper()
	b, err := parse(readFixture(t, "testdata", "petstore-v12.json"), "export.json")
	require.NoError(t, err)
	return b
}

func authFields(t *testing.T, data string) map[string]any {
	t.Helper()
	var m map[string]any
	require.NoError(t, json.Unmarshal([]byte(data), &m))
	return m
}

func requestWith(t *testing.T, fields map[string]any) portability.Request {
	t.Helper()
	req := map[string]any{"v": "17", "name": "R", "method": "POST", "endpoint": "https://api.example.com/r", "params": []any{}, "headers": []any{}}
	for k, v := range fields {
		req[k] = v
	}
	raw, err := json.Marshal(map[string]any{"v": 12, "name": "C", "folders": []any{}, "requests": []any{req}})
	require.NoError(t, err)
	b, err := parse(raw, "")
	require.NoError(t, err)
	return b.tree.Root.Requests[0]
}

func TestDetect(t *testing.T) {
	imp := &Importer{}
	cases := map[string]struct {
		data string
		want bool
	}{
		"collection":             {`{"v":12,"name":"C","folders":[],"requests":[]}`, true},
		"legacy collection":      {`{"name":"C","folders":[],"requests":[]}`, true},
		"export all":             {`[{"v":2,"name":"A","folders":[],"requests":[]},{"v":12,"name":"B","folders":[],"requests":[]}]`, true},
		"empty list":             {`[]`, false},
		"one foreign in a list":  {`[{"v":2,"name":"A","folders":[],"requests":[]},{"name":"B"}]`, false},
		"no folders":             {`{"name":"C","requests":[]}`, false},
		"postman v1":             {`{"id":"1","name":"Old","order":[],"folders":[],"requests":[]}`, false},
		"postman v2.1":           {`{"info":{"name":"P","schema":"https://schema.getpostman.com/json/collection/v2.1.0/collection.json"},"item":[]}`, false},
		"postman environment":    {`{"name":"Dev","values":[],"_postman_variable_scope":"environment"}`, false},
		"hoppscotch environment": {`{"v":2,"id":"e1","name":"Dev","variables":[]}`, false},
		"tetiva snapshot":        {`{"format":"tetiva.collection-snapshot","version":1,"collection":{"name":"S","items":[]}}`, false},
		"insomnia":               {`{"_type":"export","__export_format":4,"resources":[]}`, false},
		"openapi":                {`{"openapi":"3.0.0","info":{"title":"x","version":"1"},"paths":{}}`, false},
		"har":                    {`{"log":{"version":"1.2","entries":[]}}`, false},
		"thunder client":         {`{"clientName":"Thunder Client","collectionName":"T","folders":[],"requests":[]}`, false},
		"not json":               {`name: C`, false},
	}
	for name, tc := range cases {
		assert.Equal(t, tc.want, imp.Detect([]byte(tc.data)), name)
	}
	for _, f := range []string{"petstore-v12.json", "collections-v2.json", "graphql-collections.json"} {
		assert.True(t, imp.Detect(readFixture(t, "testdata", f)), f)
	}
	postman, err := filepath.Glob(filepath.Join("..", "postman", "testdata", "*.json"))
	require.NoError(t, err)
	require.NotEmpty(t, postman)
	for _, f := range append(postman, filepath.Join("..", "..", "..", "..", "testdata", "snapshot", "all-protocols.json")) {
		assert.False(t, imp.Detect(readFixture(t, f)), f)
	}
}

func TestBuild_TreeAndRequests(t *testing.T) {
	root := petstore(t).tree.Root

	assert.Equal(t, "Petstore", root.In.Name)
	assert.Equal(t, "Pets API", root.In.Description)
	require.Len(t, root.Folders, 1)
	assert.Equal(t, "Admin", root.Folders[0].In.Name)
	var names []string
	for _, r := range root.Requests {
		names = append(names, r.In.Name)
	}
	assert.Equal(t, []string{"List pets", "Create pet", "Login", "Upload photo", "Pet by id"}, names)

	list := root.Requests[0].In
	assert.Equal(t, entities.ProtocolHTTP, list.Protocol)
	assert.Equal(t, entities.MethodGET, list.Method)
	assert.Equal(t, "{{baseUrl}}/pets?limit={{limit}}", list.URL)
	assert.Equal(t, "Returns all pets", list.Description)
	assert.Equal(t, entities.BodyTypeNone, list.BodyType)

	del := root.Folders[0].Requests[0].In
	assert.Equal(t, entities.MethodDELETE, del.Method)
	assert.Equal(t, "{{baseUrl}}/pets/{{petId}}", del.URL)
}

func TestBuild_CollectionHeadersReachTheirRequests(t *testing.T) {
	b := petstore(t)
	root := b.tree.Root

	assert.Equal(t, []entities.HeaderItem{
		{Key: "X-Client", Value: "hoppscotch", Enabled: true},
		{Key: "Accept", Value: "application/json", Enabled: true},
		{Key: "X-Trace", Value: "on", Enabled: false},
	}, root.Requests[0].In.Headers)
	assert.Equal(t, []entities.HeaderItem{
		{Key: "X-Admin", Value: "true", Enabled: true},
		{Key: "X-Client", Value: "admin-console", Enabled: true},
	}, root.Folders[0].Requests[0].In.Headers)
	assert.Contains(t, portability.Texts(b.tree.Warnings),
		"headers set on Hoppscotch collections and folders were copied into their requests")
}

func TestBuild_Bodies(t *testing.T) {
	root := petstore(t).tree.Root

	create := root.Requests[1].In
	assert.Equal(t, entities.BodyTypeJSON, create.BodyType)
	assert.Equal(t, `{"name":"{{petName}}"}`, create.Body)

	login := root.Requests[2].In
	assert.Equal(t, entities.BodyTypeForm, login.BodyType)
	assert.JSONEq(t, `[
		{"key":"username","value":"admin","type":"text","enabled":true},
		{"key":"password","value":"{{password}}","type":"text","enabled":true},
		{"key":"remember","value":"true","type":"text","enabled":false}
	]`, login.Body)

	upload := root.Requests[3].In
	assert.Equal(t, entities.BodyTypeForm, upload.BodyType)
	assert.JSONEq(t, `[
		{"key":"caption","value":"{{petName}} at home","type":"text","enabled":true},
		{"key":"photo","value":"","type":"file","enabled":true}
	]`, upload.Body)

	cases := []struct {
		contentType string
		body        any
		bodyType    entities.BodyType
		header      string
	}{
		{"application/json", `{}`, entities.BodyTypeJSON, ""},
		{"application/hal+json", `{}`, entities.BodyTypeJSON, "application/hal+json"},
		{"application/xml", `<a/>`, entities.BodyTypeXML, ""},
		{"text/xml", `<a/>`, entities.BodyTypeXML, "text/xml"},
		{"text/plain", `hi`, entities.BodyTypeRaw, "text/plain"},
		{"text/html", `<p/>`, entities.BodyTypeRaw, "text/html"},
		{"application/octet-stream", map[string]any{}, entities.BodyTypeBinary, ""},
	}
	for _, tc := range cases {
		r := requestWith(t, map[string]any{"body": map[string]any{"contentType": tc.contentType, "body": tc.body}})
		assert.Equal(t, tc.bodyType, r.In.BodyType, tc.contentType)
		var header string
		for _, h := range r.In.Headers {
			if h.Key == "Content-Type" {
				header = h.Value
			}
		}
		assert.Equal(t, tc.header, header, tc.contentType)
	}

	own := requestWith(t, map[string]any{
		"headers": []any{map[string]any{"key": "content-type", "value": "text/plain; charset=utf-8", "active": true}},
		"body":    map[string]any{"contentType": "text/plain", "body": "hi"},
	})
	assert.Len(t, own.In.Headers, 1)
}

func TestParseRawKeyValues(t *testing.T) {
	kvs := parseRawKeyValues("a: 1\n\n  # b : two words  \n\"c:d\": \"x#y\"\nflag\n: orphan")

	var got []string
	for _, kv := range kvs {
		got = append(got, kv.Key+"="+kv.Value+map[bool]string{true: "", false: " (off)"}[kv.enabled()])
	}
	assert.Equal(t, []string{"a=1", "b=two words (off)", "c:d=x#y", "flag="}, got)
}

func TestBuild_Auth(t *testing.T) {
	root := petstore(t).tree.Root

	assert.Equal(t, entities.AuthTypeBearer, root.In.AuthType)
	assert.Equal(t, map[string]any{"token": "{{token}}"}, authFields(t, root.In.AuthData))
	assert.Equal(t, entities.AuthTypeInherit, root.Requests[0].In.AuthType)
	assert.Equal(t, entities.AuthTypeNone, root.Requests[2].In.AuthType)

	admin := root.Folders[0].In
	assert.Equal(t, entities.AuthTypeOAuth2, admin.AuthType)
	assert.Equal(t, map[string]any{
		"grant": auth.GrantClientCredentials, "tokenUrl": "https://auth.example.com/oauth/token",
		"clientId": "{{clientId}}", "clientSecret": "{{clientSecret}}", "scope": "pets:write",
		"clientAuth": auth.ClientAuthBody, "addTo": "header",
	}, authFields(t, admin.AuthData))

	cases := []struct {
		name     string
		auth     map[string]any
		authType entities.AuthType
		data     map[string]any
	}{
		{"basic", map[string]any{"authType": "basic", "authActive": true, "username": "u", "password": "<<p>>"},
			entities.AuthTypeBasic, map[string]any{"username": "u", "password": "{{p}}"}},
		{"api key in query", map[string]any{"authType": "api-key", "authActive": true, "key": "k", "value": "v", "addTo": "QUERY_PARAMS"},
			entities.AuthTypeAPIKey, map[string]any{"key": "k", "value": "v", "addTo": "query"}},
		{"legacy api key", map[string]any{"authType": "api-key", "authActive": true, "key": "k", "value": "v", "addTo": "Headers"},
			entities.AuthTypeAPIKey, map[string]any{"key": "k", "value": "v", "addTo": "header"}},
		{"inactive", map[string]any{"authType": "bearer", "authActive": false, "token": "t"}, entities.AuthTypeNone, map[string]any{}},
		{"digest", map[string]any{"authType": "digest", "authActive": true, "username": "u", "password": "p", "algorithm": "MD5"},
			entities.AuthTypeDigest, map[string]any{"username": "u", "password": "p"}},
		{"aws", map[string]any{"authType": "aws-signature", "authActive": true, "accessKey": "AK", "secretKey": "SK", "region": "eu-west-1", "serviceName": "s3", "addTo": "HEADERS"},
			entities.AuthTypeAWSSigV4, map[string]any{"accessKeyId": "AK", "secretAccessKey": "SK", "region": "eu-west-1", "service": "s3"}},
		{"auth code", map[string]any{"authType": "oauth-2", "authActive": true, "addTo": "QUERY_PARAMS", "grantTypeInfo": map[string]any{
			"grantType": "AUTHORIZATION_CODE", "authEndpoint": "https://a.example.com/auth", "tokenEndpoint": "https://a.example.com/token",
			"clientID": "c", "clientSecret": "", "scopes": "openid", "isPKCE": true, "token": "old"}},
			entities.AuthTypeOAuth2, map[string]any{"grant": auth.GrantAuthorizationCode, "authUrl": "https://a.example.com/auth",
				"tokenUrl": "https://a.example.com/token", "clientId": "c", "scope": "openid", "addTo": "query"}},
		{"password", map[string]any{"authType": "oauth-2", "authActive": true, "grantTypeInfo": map[string]any{
			"grantType": "PASSWORD", "authEndpoint": "https://a.example.com/token", "clientID": "c", "username": "u", "password": "p",
			"clientAuthentication": "AS_BASIC_AUTH_HEADERS"}},
			entities.AuthTypeOAuth2, map[string]any{"grant": auth.GrantPassword, "tokenUrl": "https://a.example.com/token",
				"clientId": "c", "username": "u", "password": "p", "clientAuth": auth.ClientAuthBasic, "addTo": "header"}},
		{"jwt", map[string]any{"authType": "jwt", "authActive": true, "secret": "<<s>>", "privateKey": "", "algorithm": "HS512",
			"payload": `{"sub":"1"}`, "addTo": "HEADERS", "isSecretBase64Encoded": true, "headerPrefix": "Bearer ", "paramName": "token", "jwtHeaders": "{}"},
			entities.AuthTypeJWT, map[string]any{"alg": "HS512", "secret": "{{s}}", "claims": map[string]any{"sub": "1"},
				"headerPrefix": "Bearer", "queryParam": "token", "addTo": "header", "secretBase64": "true"}},
	}
	for _, tc := range cases {
		r := requestWith(t, map[string]any{"auth": tc.auth})
		assert.Equal(t, tc.authType, r.In.AuthType, tc.name)
		assert.Equal(t, tc.data, authFields(t, r.In.AuthData), tc.name)
	}
}

func TestBuild_UnsupportedAuthWarns(t *testing.T) {
	raw, err := json.Marshal(map[string]any{"v": 12, "name": "C", "folders": []any{}, "requests": []any{
		map[string]any{"v": "17", "name": "Hawk", "method": "GET", "endpoint": "https://x.example.com", "auth": map[string]any{"authType": "hawk", "authActive": true}},
		map[string]any{"v": "17", "name": "Implicit", "method": "GET", "endpoint": "https://x.example.com", "auth": map[string]any{
			"authType": "oauth-2", "authActive": true, "grantTypeInfo": map[string]any{"grantType": "IMPLICIT", "authEndpoint": "https://a.example.com/auth", "clientID": "c"}}},
		map[string]any{"v": "17", "name": "Bad JWT", "method": "GET", "endpoint": "https://x.example.com", "auth": map[string]any{
			"authType": "jwt", "authActive": true, "payload": "[1]"}},
	}})
	require.NoError(t, err)

	b, err := parse(raw, "")
	require.NoError(t, err)

	assert.Equal(t, entities.AuthTypeNone, b.tree.Root.Requests[0].In.AuthType)
	assert.Equal(t, []string{
		`request "Hawk": auth type "hawk" is not supported and was imported as no auth`,
		`request "Implicit": OAuth 2.0 grant "IMPLICIT" is not supported; pick one in the Auth tab`,
		`request "Bad JWT": the JWT payload is not a JSON object and was dropped`,
	}, portability.Texts(b.tree.Warnings))
}

func TestBuild_VariablesBecomeOneEnvironment(t *testing.T) {
	b := petstore(t)

	require.Len(t, b.tree.Environments, 1)
	assert.Equal(t, "Petstore", b.tree.Environments[0].Name)
	assert.Equal(t, []environment.AddVariable{
		{Key: "baseUrl", Value: "https://petstore.example.com/v1"},
		{Key: "token", IsSecret: true},
		{Key: "clientId", Value: "petstore-admin"},
		{Key: "clientSecret", IsSecret: true},
		{Key: "petId", Value: "42"},
		{Key: "limit", Value: "20", Disabled: true},
		{Key: "petName", Value: "Rex"},
	}, b.tree.Environments[0].Vars)
}

func TestBuild_VariableConflictsKeepTheFirstValue(t *testing.T) {
	raw, err := json.Marshal(map[string]any{"v": 12, "name": "C", "folders": []any{}, "requests": []any{
		map[string]any{"v": "17", "name": "A", "method": "GET", "endpoint": "<<host>>/a", "requestVariables": []any{map[string]any{"key": "id", "value": "1", "active": true}}},
		map[string]any{"v": "17", "name": "B", "method": "GET", "endpoint": "<<host>>/b", "requestVariables": []any{map[string]any{"key": "id", "value": "2", "active": true}}},
		map[string]any{"v": "17", "name": "C", "method": "GET", "endpoint": "<<host>>/c", "requestVariables": []any{map[string]any{"key": "id", "value": "1", "active": true}}},
	}})
	require.NoError(t, err)

	b, err := parse(raw, "")
	require.NoError(t, err)

	assert.Equal(t, []environment.AddVariable{{Key: "id", Value: "1"}}, b.tree.Environments[0].Vars)
	assert.Equal(t, []string{"variables defined more than once kept their first value: id"}, portability.Texts(b.tree.Warnings))
}

func TestBuild_ExportAllGivesEachCollectionItsOwnEnvironment(t *testing.T) {
	data := `[{"v":12,"name":"A","folders":[],"requests":[{"v":"1","name":"r","method":"GET","endpoint":"<<baseUrl>>/a","params":[],"headers":[],` +
		`"auth":{"authType":"inherit","authActive":true},"body":{"contentType":null,"body":null},"requestVariables":[]}],` +
		`"variables":[{"key":"baseUrl","initialValue":"https://a","currentValue":"","secret":false}]},` +
		`{"v":12,"name":"B","folders":[{"v":12,"name":"Staging","folders":[],"requests":[],` +
		`"variables":[{"key":"baseUrl","initialValue":"https://staging","currentValue":"","secret":false}]}],` +
		`"requests":[{"v":"1","name":"r","method":"GET","endpoint":"<<baseUrl>>/b","params":[],"headers":[],` +
		`"auth":{"authType":"inherit","authActive":true},"body":{"contentType":null,"body":null},"requestVariables":[]}],` +
		`"variables":[{"key":"baseUrl","initialValue":"https://b","currentValue":"","secret":false}]},` +
		`{"v":12,"name":"C","folders":[],"requests":[]}]`

	b, err := parse([]byte(data), "all.json")
	require.NoError(t, err)

	assert.Equal(t, []portability.Environment{
		{Name: "A", Vars: []environment.AddVariable{{Key: "baseUrl", Value: "https://a"}}},
		{Name: "B", Vars: []environment.AddVariable{{Key: "baseUrl", Value: "https://b"}}},
	}, b.tree.Environments)
	assert.Equal(t, []string{"variables defined more than once kept their first value: baseUrl"}, portability.Texts(b.tree.Warnings))
}

func TestBuild_NestedVariableReferences(t *testing.T) {
	raw, err := json.Marshal(map[string]any{"v": 12, "name": "C", "folders": []any{}, "requests": []any{
		map[string]any{"v": "17", "name": "A", "method": "GET", "endpoint": "<<path>>",
			"requestVariables": []any{map[string]any{"key": "path", "value": "<<apiBase>>/a", "active": true}}},
		map[string]any{"v": 10, "name": "Q", "url": "<<apiBase>>/graphql", "query": "{ c }", "variables": `{"code":"<<code>>"}`},
	}, "variables": []any{map[string]any{"key": "apiBase", "initialValue": "<<baseUrl>>/v2", "currentValue": "", "secret": false}}})
	require.NoError(t, err)

	b, err := parse(raw, "")
	require.NoError(t, err)

	assert.Equal(t, []environment.AddVariable{
		{Key: "apiBase", Value: "{{baseUrl}}/v2"},
		{Key: "path", Value: "{{apiBase}}/a"},
	}, b.tree.Environments[0].Vars)
	assert.Equal(t, `{"code":"{{code}}"}`, b.tree.Root.Requests[1].In.GraphQLVariables)
}

func TestBuild_ResponsesBecomeExamples(t *testing.T) {
	list := petstore(t).tree.Root.Requests[0]

	require.Len(t, list.Examples, 1)
	ex := list.Examples[0]
	assert.Equal(t, "OK", ex.Name)
	assert.Equal(t, 200, ex.StatusCode)
	assert.Equal(t, "OK", ex.StatusText)
	assert.Equal(t, `[{"id":1,"name":"Rex"}]`, ex.Body)
	assert.Equal(t, "application/json", ex.ContentType)
	assert.Equal(t, entities.ProtocolHTTP, ex.Protocol)
}

func TestBuild_GraphQL(t *testing.T) {
	gql := petstore(t).tree.Root.Requests[4].In

	assert.Equal(t, entities.ProtocolGraphQL, gql.Protocol)
	assert.Equal(t, entities.MethodPOST, gql.Method)
	assert.Equal(t, "{{baseUrl}}/graphql", gql.URL)
	assert.Equal(t, "query Pet($id: ID!) { pet(id: $id) { name } }", gql.GraphQLQuery)
	assert.Equal(t, `{"id": 1}`, gql.GraphQLVariables)
	assert.Equal(t, entities.AuthTypeInherit, gql.AuthType)

	b, err := parse(readFixture(t, "testdata", "graphql-collections.json"), "countries.json")
	require.NoError(t, err)
	countries := b.tree.Root.Requests[0].In
	assert.Equal(t, "Countries", b.tree.Root.In.Name)
	assert.Equal(t, entities.ProtocolGraphQL, countries.Protocol)
	assert.Equal(t, []entities.HeaderItem{{Key: "X-Api-Key", Value: "{{key}}", Enabled: true}}, countries.Headers)
}

func TestBuild_ExportAllNestsCollectionsUnderTheFileName(t *testing.T) {
	b, err := parse(readFixture(t, "testdata", "collections-v2.json"), "hoppscotch-collections.json")
	require.NoError(t, err)
	root := b.tree.Root

	assert.Equal(t, "hoppscotch-collections", root.In.Name)
	require.Len(t, root.Folders, 2)
	billing := root.Folders[0]
	assert.Equal(t, "Billing", billing.In.Name)
	assert.Equal(t, entities.AuthTypeNone, billing.In.AuthType)
	invoices := billing.Requests[0].In
	assert.Equal(t, entities.MethodGET, invoices.Method)
	assert.Equal(t, "https://billing.example.com/invoices?status=open&page=2", invoices.URL)
	assert.Equal(t, entities.AuthTypeBasic, invoices.AuthType)
	assert.Equal(t, entities.AuthTypeAPIKey, billing.Requests[1].In.AuthType)
	ping := root.Folders[1].Requests[0].In
	assert.Equal(t, entities.MethodHEAD, ping.Method)
	assert.Equal(t, entities.AuthTypeNone, ping.AuthType)
	assert.Empty(t, b.tree.Environments)

	unnamed, err := parse(readFixture(t, "testdata", "collections-v2.json"), "")
	require.NoError(t, err)
	assert.Equal(t, "Hoppscotch import", unnamed.tree.Root.In.Name)
}

func TestBuild_LongDescriptionIsTruncated(t *testing.T) {
	r := requestWith(t, map[string]any{"description": strings.Repeat("é", 9000)})

	assert.Len(t, r.In.Description, 16*1024)
}

func TestPreview(t *testing.T) {
	p, err := (&Importer{}).Preview(readFixture(t, "testdata", "petstore-v12.json"), "")
	require.NoError(t, err)

	assert.Equal(t, &portability.ImportPreview{
		Format: portability.FormatHoppscotch, Title: "Petstore", Folders: 1, Requests: 6, Examples: 1,
		EnvironmentNames: []string{"Petstore"},
		Hosts:            []string{"auth.example.com", "petstore.example.com"},
		Scripts: []portability.ScriptPreview{
			{Path: "Petstore", Phase: "post", Text: `pw.test("ok", () => pw.expect(pw.response.status).toBeLevel2xx());`},
			{Path: "Petstore / Admin / Delete pet", Phase: "post", Text: "pw.test(\"deleted\", () => {\n  pw.expect(pw.response.status).toBe(204);\n});"},
			{Path: "Petstore / List pets", Phase: "pre", Text: `pw.env.set("limit", "10");`},
		},
		Warnings: []portability.Warning{
			portability.Warn(portability.WarnFileFieldDropped, portability.Subject{Kind: portability.KindRequest, Name: "Upload photo"}, "field", "photo"),
			portability.Warn(portability.WarnHoppscotchSecretsEmpty, portability.Subject{}, "keys", "token, clientSecret"),
			portability.Warn(portability.WarnHoppscotchHeadersCopied, portability.Subject{}),
			portability.Warn(portability.WarnForeignScripts, portability.Subject{}, "app", "Hoppscotch", "api", "pw.* / hopp.*"),
		},
	}, p)
}

func TestPreview_PmScriptsNeedNoRewrite(t *testing.T) {
	collection := func(script string) []byte {
		raw, err := json.Marshal(map[string]any{"v": 12, "name": "C", "folders": []any{}, "requests": []any{
			map[string]any{"name": "Ping", "method": "GET", "endpoint": "https://api.example.com/ping", "params": []any{}, "headers": []any{},
				"testScript": script},
		}})
		require.NoError(t, err)
		return raw
	}
	rewrite := portability.Warn(portability.WarnForeignScripts, portability.Subject{}, "app", "Hoppscotch", "api", "pw.* / hopp.*").Text

	for _, tc := range []struct {
		script string
		warns  bool
	}{
		{`pm.test("ok", () => pm.response.to.have.status(200));`, false},
		{`pw.test("ok", () => pw.expect(pw.response.status).toBe(200));`, true},
		{`hopp.test("ok", () => hopp.expect(hopp.response.statusCode).toBe(200));`, true},
	} {
		p, err := (&Importer{}).Preview(collection(tc.script), "")
		require.NoError(t, err)
		assert.Len(t, p.Scripts, 1, tc.script)
		assert.Equal(t, tc.warns, slices.Contains(portability.Texts(p.Warnings), rewrite), tc.script)

		b, err := parse(collection(tc.script), "")
		require.NoError(t, err)
		assert.Equal(t, tc.warns, b.scripts, tc.script)
	}
}

func TestPreview_RejectsABrokenFile(t *testing.T) {
	_, err := (&Importer{}).Preview([]byte(`{"name":"C","folders":[],"requests":[{"headers":"Accept: */*"}]}`), "")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "validation")
}

func TestIsEnvironment(t *testing.T) {
	cases := map[string]struct {
		data string
		want bool
	}{
		"v2":                  {`{"v":2,"id":"e","name":"Dev","variables":[{"key":"a","initialValue":"1","currentValue":"","secret":false}]}`, true},
		"v0":                  {`{"name":"Dev","variables":[{"key":"a","value":"1"}]}`, true},
		"export all":          {`[{"name":"Dev","variables":[]},{"name":"Prod","variables":[]}]`, true},
		"empty list":          {`[]`, false},
		"collection":          {`{"v":12,"name":"C","folders":[],"requests":[],"variables":[]}`, false},
		"collections":         {`[{"v":12,"name":"C","folders":[],"requests":[],"variables":[]}]`, false},
		"postman environment": {`{"name":"Dev","values":[],"variables":[]}`, false},
		"no name":             {`{"v":2,"variables":[]}`, false},
		"bruno environment":   {`{"name":"Dev","variables":[{"name":"host","value":"x","enabled":true,"secret":false,"type":"text"}]}`, false},
	}
	for name, tc := range cases {
		assert.Equal(t, tc.want, IsEnvironment([]byte(tc.data)), name)
	}
	assert.True(t, IsEnvironment(readFixture(t, "testdata", "environments.json")))
	assert.False(t, IsEnvironment(readFixture(t, "testdata", "petstore-v12.json")))
}

func TestBuild_SecretsComeWithoutValues(t *testing.T) {
	assert.Contains(t, portability.Texts(petstore(t).tree.Warnings), "Hoppscotch exports secrets without their values: fill in token, clientSecret")
}

func TestBuild_QueryParametersAreEncoded(t *testing.T) {
	r := requestWith(t, map[string]any{
		"endpoint": "<<host>>/search#results",
		"params": []any{
			map[string]any{"key": "q", "value": "a&b c+d#é", "active": true},
			map[string]any{"key": "page", "value": "<<page>>", "active": true},
		},
	})

	u := r.In.URL
	assert.Equal(t, "{{host}}/search?q=a%26b+c%2Bd%23%C3%A9&page={{page}}#results", u)
	parsed, err := url.Parse(strings.NewReplacer("{{host}}", "https://e.com", "{{page}}", "2").Replace(u))
	require.NoError(t, err)
	assert.Equal(t, url.Values{"q": {"a&b c+d#é"}, "page": {"2"}}, parsed.Query())
	assert.Equal(t, "results", parsed.Fragment)
}

func TestBuild_MultipartWithoutFilesIsReported(t *testing.T) {
	raw, err := json.Marshal(map[string]any{"v": 12, "name": "C", "folders": []any{}, "requests": []any{
		map[string]any{"name": "Sign up", "method": "POST", "endpoint": "https://api.example.com/users", "params": []any{},
			"headers": []any{map[string]any{"key": "Content-Type", "value": "multipart/form-data", "active": true}},
			"body": map[string]any{"contentType": "multipart/form-data", "body": []any{
				map[string]any{"key": "name", "value": "Ann", "active": true, "isFile": false},
			}}},
		map[string]any{"name": "Upload", "method": "POST", "endpoint": "https://api.example.com/files", "params": []any{}, "headers": []any{},
			"body": map[string]any{"contentType": "multipart/form-data", "body": []any{
				map[string]any{"key": "file", "value": []any{}, "active": true, "isFile": true},
			}}},
	}})
	require.NoError(t, err)

	b, err := parse(raw, "")
	require.NoError(t, err)

	assert.Empty(t, b.tree.Root.Requests[0].In.Headers)
	assert.Contains(t, portability.Texts(b.tree.Warnings),
		`1 multipart request has no file field and will be sent as a URL-encoded form, not multipart: "Sign up"`)
}

func TestBuild_FoldersThatTurnAuthOffAreReported(t *testing.T) {
	folder := func(name string, auth map[string]any, sub ...any) map[string]any {
		return map[string]any{"v": 12, "name": name, "auth": auth, "folders": append([]any{}, sub...), "requests": []any{}}
	}
	bearer := map[string]any{"authType": "bearer", "authActive": true, "token": "t"}
	none := map[string]any{"authType": "none", "authActive": true}
	off := map[string]any{"authType": "bearer", "authActive": false, "token": "t"}
	inherit := map[string]any{"authType": "inherit", "authActive": true}

	withAuth, err := json.Marshal(folder("API", bearer, folder("Public", none), folder("Health", off), folder("Users", inherit, folder("Status", none))))
	require.NoError(t, err)
	b, err := parse(withAuth, "")
	require.NoError(t, err)
	assert.Contains(t, portability.Texts(b.tree.Warnings), `requests in folders "Public", "Health", "Status" will inherit the parent's auth: `+
		`Tetiva cannot turn auth off at a folder, set them to No Auth if they must go without`)

	withoutAuth, err := json.Marshal(folder("API", none, folder("Public", none)))
	require.NoError(t, err)
	b, err = parse(withoutAuth, "")
	require.NoError(t, err)
	assert.Empty(t, b.tree.Warnings)
}
