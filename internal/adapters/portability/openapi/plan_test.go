package openapi

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tetiva-app/client/internal/adapters/portability"
	"github.com/tetiva-app/client/internal/domain"
	"github.com/tetiva-app/client/internal/domain/entities"
	"github.com/tetiva-app/client/internal/domain/usecase/environment"
	"github.com/tetiva-app/client/internal/race"
)

func planOf(t *testing.T, data []byte) *plan {
	t.Helper()
	s, err := load(data)
	require.NoError(t, err)
	return newPlan(s)
}

func planFile(t *testing.T, name string) *plan {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", name))
	require.NoError(t, err)
	return planOf(t, data)
}

func requestByLabel(t *testing.T, p *plan, label string) *requestPlan {
	t.Helper()
	all := p.requests
	for _, f := range p.folders {
		all = append(all, f.requests...)
	}
	for _, rp := range all {
		if rp.label == label {
			return rp
		}
	}
	require.Failf(t, "no request", label)
	return nil
}

func varKeys(vars []environment.AddVariable) []string {
	out := make([]string, 0, len(vars))
	for _, v := range vars {
		out = append(out, v.Key)
	}
	return out
}

func TestPlan_FoldersFollowTagsAndUntaggedStayAtRoot(t *testing.T) {
	p := planFile(t, "petstore-3.0.yaml")

	var folders []string
	for _, f := range p.folders {
		folders = append(folders, f.in.Name)
	}
	assert.Equal(t, []string{"pet", "user", "store"}, folders)
	assert.Equal(t, "Everything about your pets", p.folders[0].in.Description)

	var pets []string
	for _, rp := range p.folders[0].requests {
		pets = append(pets, rp.in.Name)
	}
	assert.Equal(t, []string{"List pets", "addPet", "Get a pet", "Delete a pet"}, pets)

	var root []string
	for _, rp := range p.requests {
		root = append(root, rp.in.Name)
	}
	assert.Equal(t, []string{"Upload a file", "GET /health"}, root)
	assert.Equal(t, 8, p.requestCount)
}

func TestPlan_CollectionFromInfo(t *testing.T) {
	p := planFile(t, "petstore-3.0.yaml")

	assert.Equal(t, "Petstore", p.root.Name)
	assert.Equal(t, "A sample pet store.\n\nVersion: 1.4.0", p.root.Description)
	assert.Equal(t, "Petstore", p.envName)
}

func TestPlan_UntitledDocument(t *testing.T) {
	p := planOf(t, []byte(`{"openapi":"3.0.0","info":{"title":" ","version":""},"paths":{}}`))

	assert.Equal(t, "Untitled API", p.root.Name)
	assert.Empty(t, p.root.Description)
}

func TestPlan_URLParametersAndDescription(t *testing.T) {
	p := planFile(t, "petstore-3.0.yaml")

	list := requestByLabel(t, p, "GET /pets")
	assert.Equal(t, "{{baseUrl}}/pets?limit=20&status=available", list.in.URL)
	assert.Equal(t, []entities.HeaderItem{{Key: "X-Request-ID", Value: "", Enabled: true}}, list.in.Headers)
	assert.Equal(t, "Returns pets by status.\n\n**Parameters**\n\n"+
		"- `limit` (query, integer): How many pets to return\n"+
		"- `status` (query, array of string, required)\n"+
		"- `X-Request-ID` (header, string, required)\n"+
		"- `Accept` (header, string)\n"+
		"- `Authorization` (header, string, required)\n"+
		"- `Content-Type` (header, string)\n"+
		"- `offset` (query, integer)", list.in.Description)

	get := requestByLabel(t, p, "GET /pets/{petId}")
	assert.Equal(t, "{{baseUrl}}/pets/{{petId}}", get.in.URL)
	assert.Equal(t, entities.MethodGET, get.in.Method)
	assert.Equal(t, entities.ProtocolHTTP, get.in.Protocol)
}

func TestPlan_JSONEscapes(t *testing.T) {
	p := planFile(t, "escaped-3.0.json")

	rp := requestByLabel(t, p, "GET /users/{id}")
	assert.Equal(t, "Launch \U0001F680", rp.in.Name)
	assert.Equal(t, "{{baseUrl}}/users/{{id}}?fields=", rp.in.URL)
	assert.Equal(t, []environment.AddVariable{{Key: "baseUrl", Value: "https://api.example.com/v1"}, {Key: "id", Value: "7"}}, p.vars)
	require.Len(t, rp.examples, 1)
	assert.JSONEq(t, `{"name":"J\u00f6rg","url":"https://x.io"}`, rp.examples[0].Body)
	assert.Equal(t, "application/json", rp.examples[0].ContentType)
}

func TestPlan_EnvironmentVariables(t *testing.T) {
	p := planFile(t, "petstore-3.0.yaml")

	assert.Equal(t, []environment.AddVariable{
		{Key: "baseUrl", Value: "https://api.petstore.example.com/api/v3"},
		{Key: "apiKey", IsSecret: true},
		{Key: "clientId"},
		{Key: "clientSecret", IsSecret: true},
		{Key: "petId", Value: "42"},
		{Key: "token", IsSecret: true},
	}, p.vars)
}

func TestPlan_BaseURL(t *testing.T) {
	cases := map[string]struct {
		doc     string
		want    string
		warning string
	}{
		"server variables": {
			doc:  `{"openapi":"3.0.0","info":{"title":"T"},"servers":[{"url":"{scheme}://{host}/v1","variables":{"scheme":{"default":"https"},"host":{"default":"x.example.com"}}}],"paths":{}}`,
			want: "https://x.example.com/v1",
		},
		"relative server": {
			doc:     `{"openapi":"3.0.0","info":{"title":"T"},"servers":[{"url":"/api"}],"paths":{}}`,
			want:    "/api",
			warning: `the server URL "/api" is relative: set baseUrl in the environment to the full address`,
		},
		"no server": {
			doc:     `{"openapi":"3.0.0","info":{"title":"T"},"paths":{}}`,
			warning: "the document names no server: set baseUrl in the environment",
		},
		"swagger 2 host": {
			doc:  `{"swagger":"2.0","info":{"title":"T","version":"1"},"host":"api.example.com","basePath":"/v2","schemes":["https"],"paths":{}}`,
			want: "https://api.example.com/v2",
		},
		"swagger 2 without host": {
			doc:     `{"swagger":"2.0","info":{"title":"T","version":"1"},"basePath":"/v2","paths":{}}`,
			want:    "/v2",
			warning: `the server URL "/v2" is relative: set baseUrl in the environment to the full address`,
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			p := planOf(t, []byte(tc.doc))

			assert.Equal(t, "baseUrl", p.vars[0].Key)
			assert.Equal(t, tc.want, p.vars[0].Value)
			if tc.warning == "" {
				assert.Empty(t, p.warnings)
			} else {
				assert.Equal(t, []string{tc.warning}, portability.Texts(p.warnings))
			}
		})
	}
}

func TestPlan_Bodies(t *testing.T) {
	p := planFile(t, "petstore-3.0.yaml")

	add := requestByLabel(t, p, "POST /pets")
	assert.Equal(t, entities.BodyTypeJSON, add.in.BodyType)
	assert.JSONEq(t, `{"name":"doggie","status":"available","tags":[{"name":"string","id":0}],"createdAt":"2024-01-01T12:00:00Z"}`, add.in.Body)
	assert.True(t, strings.HasPrefix(add.in.Body, "{\n  \"name\""), "readOnly id is left out and order is kept")
	assert.Equal(t, []entities.HeaderItem{{Key: "Content-Type", Value: "application/json", Enabled: true}}, add.in.Headers)

	login := requestByLabel(t, p, "POST /user/login")
	assert.Equal(t, entities.BodyTypeForm, login.in.BodyType)
	assert.JSONEq(t, `[{"key":"username","value":"alice","type":"text","enabled":true},{"key":"password","value":"","type":"text","enabled":true},`+
		`{"key":"avatar","value":"","type":"text","enabled":true}]`, login.in.Body)
	assert.Empty(t, login.in.Headers)

	upload := requestByLabel(t, p, "POST /upload")
	assert.JSONEq(t, `[{"key":"note","value":"","type":"text","enabled":true},{"key":"file","value":"","type":"file","enabled":true}]`, upload.in.Body)

	health := requestByLabel(t, p, "GET /health")
	assert.Equal(t, entities.BodyTypeNone, health.in.BodyType)
}

func TestPlan_BodyMediaTypes(t *testing.T) {
	doc := func(content string) []byte {
		return []byte(`{"openapi":"3.0.0","info":{"title":"T"},"paths":{"/x":{"post":{"requestBody":{"content":` + content + `},"responses":{}}}}}`)
	}
	cases := map[string]struct {
		content string
		typ     entities.BodyType
		body    string
		ct      string
	}{
		"json example wins over the schema": {
			content: `{"application/json":{"example":{"b":1,"a":2},"schema":{"type":"object","properties":{"z":{"type":"string"}}}}}`,
			typ:     entities.BodyTypeJSON, body: "{\n  \"b\": 1,\n  \"a\": 2\n}", ct: "application/json",
		},
		"json written as a string": {
			content: `{"application/json":{"example":"{\"a\": 1}"}}`,
			typ:     entities.BodyTypeJSON, body: `{"a": 1}`, ct: "application/json",
		},
		"json preferred over xml": {
			content: `{"application/xml":{"example":"<a/>"},"application/vnd.api+json":{"example":{"a":1}}}`,
			typ:     entities.BodyTypeJSON, body: "{\n  \"a\": 1\n}", ct: "application/vnd.api+json",
		},
		"xml keeps a text example": {
			content: `{"application/xml":{"example":"<pet><name>Rex</name></pet>"}}`,
			typ:     entities.BodyTypeXML, body: "<pet><name>Rex</name></pet>", ct: "application/xml",
		},
		"plain text": {
			content: `{"text/plain":{"examples":{"a":{"value":"hello"}}}}`,
			typ:     entities.BodyTypeRaw, body: "hello", ct: "text/plain",
		},
		"binary": {
			content: `{"application/octet-stream":{"schema":{"type":"string","format":"binary"}}}`,
			typ:     entities.BodyTypeBinary, ct: "application/octet-stream",
		},
		"any type is sent as json": {
			content: `{"*/*":{"schema":{"type":"object","properties":{"a":{"type":"integer"}}}}}`,
			typ:     entities.BodyTypeJSON, body: "{\n  \"a\": 0\n}", ct: "application/json",
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			rp := requestByLabel(t, planOf(t, doc(tc.content)), "POST /x")

			assert.Equal(t, tc.typ, rp.in.BodyType)
			assert.Equal(t, tc.body, rp.in.Body)
			assert.Equal(t, []entities.HeaderItem{{Key: "Content-Type", Value: tc.ct, Enabled: true}}, rp.in.Headers)
		})
	}
}

func TestPlan_Auth(t *testing.T) {
	p := planFile(t, "petstore-3.0.yaml")

	assert.Equal(t, entities.AuthTypeAPIKey, p.root.AuthType)
	assert.JSONEq(t, `{"key":"X-API-Key","value":"{{apiKey}}","addTo":"header"}`, p.root.AuthData)

	list := requestByLabel(t, p, "GET /pets")
	assert.Equal(t, entities.AuthTypeInherit, list.in.AuthType)

	add := requestByLabel(t, p, "POST /pets")
	assert.Equal(t, entities.AuthTypeOAuth2, add.in.AuthType)
	assert.JSONEq(t, `{"grant":"client_credentials","tokenUrl":"{{baseUrl}}/oauth/token","clientId":"{{clientId}}",`+
		`"clientSecret":"{{clientSecret}}","scope":"write:pets"}`, add.in.AuthData)

	del := requestByLabel(t, p, "DELETE /pets/{petId}")
	assert.Equal(t, entities.AuthTypeNone, del.in.AuthType)

	inventory := requestByLabel(t, p, "GET /store/inventory")
	assert.Equal(t, entities.AuthTypeBearer, inventory.in.AuthType)
	assert.JSONEq(t, `{"token":"{{token}}"}`, inventory.in.AuthData)
}

func TestPlan_SecuritySchemes(t *testing.T) {
	doc := func(scheme string) []byte {
		return []byte(`{"openapi":"3.0.0","info":{"title":"T"},"servers":[{"url":"https://a.example.com"}],"paths":{},` +
			`"security":[{"s":["read","write"]}],"components":{"securitySchemes":{"s":` + scheme + `}}}`)
	}
	cases := map[string]struct {
		scheme  string
		typ     entities.AuthType
		data    string
		vars    []string
		warning string
	}{
		"basic": {
			scheme: `{"type":"http","scheme":"basic"}`,
			typ:    entities.AuthTypeBasic, data: `{"username":"{{username}}","password":"{{password}}"}`,
			vars: []string{"baseUrl", "username", "password"},
		},
		"api key in query": {
			scheme: `{"type":"apiKey","in":"query","name":"key"}`,
			typ:    entities.AuthTypeAPIKey, data: `{"key":"key","value":"{{apiKey}}","addTo":"query"}`,
			vars: []string{"baseUrl", "apiKey"},
		},
		"api key in the authorization header": {
			scheme: `{"type":"apiKey","in":"header","name":"authorization"}`,
			typ:    entities.AuthTypeBearer, data: `{"token":"{{apiKey}}","prefix":""}`,
			vars: []string{"baseUrl", "apiKey"},
		},
		"oauth2 password with a relative token url": {
			scheme: `{"type":"oauth2","flows":{"password":{"tokenUrl":"token","scopes":{}}}}`,
			typ:    entities.AuthTypeOAuth2,
			data: `{"grant":"password","tokenUrl":"{{baseUrl}}/token","clientId":"{{clientId}}","clientSecret":"{{clientSecret}}",` +
				`"username":"{{username}}","password":"{{password}}","scope":"read write"}`,
			vars: []string{"baseUrl", "clientId", "clientSecret", "username", "password"},
		},
		"oauth2 authorization code": {
			scheme: `{"type":"oauth2","flows":{"authorizationCode":{"authorizationUrl":"https://id.example.com/auth","tokenUrl":"https://id.example.com/token","scopes":{}}}}`,
			typ:    entities.AuthTypeOAuth2,
			data: `{"grant":"authorization_code","authUrl":"https://id.example.com/auth","tokenUrl":"https://id.example.com/token",` +
				`"clientId":"{{clientId}}","clientSecret":"{{clientSecret}}","scope":"read write"}`,
			vars: []string{"baseUrl", "clientId", "clientSecret"},
		},
		"oauth2 implicit": {
			scheme: `{"type":"oauth2","flows":{"implicit":{"authorizationUrl":"https://id.example.com/auth","scopes":{}}}}`,
			typ:    entities.AuthTypeOAuth2,
			data: `{"grant":"authorization_code","authUrl":"https://id.example.com/auth",` +
				`"clientId":"{{clientId}}","clientSecret":"{{clientSecret}}","scope":"read write"}`,
			vars:    []string{"baseUrl", "clientId", "clientSecret"},
			warning: `security scheme "s": the OAuth 2.0 implicit flow is not supported, imported as authorization code without a token URL`,
		},
		"openid connect": {
			scheme: `{"type":"openIdConnect","openIdConnectUrl":"https://id.example.com/.well-known/openid-configuration"}`,
			typ:    entities.AuthTypeNone, data: `{}`,
			vars:    []string{"baseUrl"},
			warning: `security scheme "s" (openIdConnect) is not supported: requests that use it were imported without auth`,
		},
		"api key in a cookie": {
			scheme: `{"type":"apiKey","in":"cookie","name":"sid"}`,
			typ:    entities.AuthTypeNone, data: `{}`,
			vars:    []string{"baseUrl"},
			warning: `security scheme "s" (apiKey in cookie) is not supported: requests that use it were imported without auth`,
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			p := planOf(t, doc(tc.scheme))

			assert.Equal(t, tc.typ, p.root.AuthType)
			assert.JSONEq(t, tc.data, p.root.AuthData)
			assert.Equal(t, tc.vars, varKeys(p.vars))
			for _, v := range p.vars[1:] {
				assert.Empty(t, v.Value, v.Key)
			}
			if tc.warning == "" {
				assert.Empty(t, p.warnings)
			} else {
				assert.Equal(t, []string{tc.warning}, portability.Texts(p.warnings))
			}
		})
	}
}

func TestPlan_UnsupportedAPIKeyLocationIsAParam(t *testing.T) {
	p := planOf(t, []byte(`{"openapi":"3.0.0","info":{"title":"T"},"servers":[{"url":"https://a.example.com"}],"paths":{},`+
		`"security":[{"s":[]}],"components":{"securitySchemes":{"s":{"type":"apiKey","in":"cookie","name":"sid"}}}}`))

	require.Len(t, p.warnings, 1)
	assert.Equal(t, portability.WarnOpenAPISchemeLocationUnsupported, p.warnings[0].Code)
	assert.Equal(t, map[string]string{"scheme": "s", "type": "apiKey", "in": "cookie"}, p.warnings[0].Params)
}

func TestPlan_SecretVariables(t *testing.T) {
	p := planFile(t, "petstore-2.0.json")

	secret := map[string]bool{}
	for _, v := range p.vars {
		secret[v.Key] = v.IsSecret
	}
	assert.Equal(t, map[string]bool{
		"baseUrl": false, "username": false, "password": true, "petId": false, "clientId": false, "clientSecret": true,
	}, secret)
}

func TestPlan_ResponseExamples(t *testing.T) {
	p := planFile(t, "petstore-3.0.yaml")

	get := requestByLabel(t, p, "GET /pets/{petId}")
	require.Len(t, get.examples, 2)
	ok, notFound := get.examples[0], get.examples[1]
	assert.Equal(t, "200 OK", ok.Name)
	assert.Equal(t, 200, ok.StatusCode)
	assert.Equal(t, "OK", ok.StatusText)
	assert.Equal(t, "application/json", ok.ContentType)
	assert.Equal(t, []entities.HeaderItem{{Key: "Content-Type", Value: "application/json", Enabled: true}}, ok.Headers)
	assert.Equal(t, entities.ProtocolHTTP, ok.Protocol)
	assert.JSONEq(t, `{"id":0,"name":"doggie","status":"available","tags":[{"name":"string","id":0}],"createdAt":"2024-01-01T12:00:00Z"}`, ok.Body)
	assert.Equal(t, "404 Not Found", notFound.Name)
	assert.Equal(t, "{\n  \"code\": 404,\n  \"message\": \"Pet not found\"\n}", notFound.Body)

	inventory := requestByLabel(t, p, "GET /store/inventory")
	require.Len(t, inventory.examples, 2)
	assert.Equal(t, "200 OK (Small store)", inventory.examples[0].Name)
	assert.Equal(t, "200 OK (big)", inventory.examples[1].Name)
	assert.JSONEq(t, `{"sold":100,"available":200}`, inventory.examples[1].Body)

	health := requestByLabel(t, p, "GET /health")
	require.Len(t, health.examples, 1)
	assert.Equal(t, "OK", health.examples[0].Body)
	assert.Equal(t, "text/plain", health.examples[0].ContentType)

	assert.Empty(t, requestByLabel(t, p, "DELETE /pets/{petId}").examples)
	assert.Equal(t, 7, p.exampleCount)
}

func TestPlan_GeneratesExamplesFor2xxOnly(t *testing.T) {
	p := planOf(t, []byte(`{"openapi":"3.0.0","info":{"title":"T"},"paths":{"/x":{"get":{"responses":{`+
		`"2XX":{"description":"ok","content":{"application/json":{"schema":{"type":"object","properties":{"a":{"type":"integer"}}}}}},`+
		`"400":{"description":"bad","content":{"application/json":{"schema":{"type":"object","properties":{"e":{"type":"string"}}}}}},`+
		`"default":{"description":"other","content":{"application/json":{"example":{"e":"x"}}}},`+
		`"201":{"description":"empty","content":{"application/json":{"schema":{"type":"object"}}}}}}}}}`))

	examples := requestByLabel(t, p, "GET /x").examples
	require.Len(t, examples, 2)
	assert.Equal(t, "2XX", examples[0].Name)
	assert.Equal(t, 200, examples[0].StatusCode)
	assert.Equal(t, "Default", examples[1].Name)
	assert.Equal(t, 0, examples[1].StatusCode)
}

func TestPlan_OversizedExampleIsSkipped(t *testing.T) {
	big := strings.Repeat("a", domain.MaxExampleBodyLen+1)
	p := planOf(t, []byte(`{"openapi":"3.0.0","info":{"title":"T"},"paths":{"/x":{"get":{"responses":{`+
		`"200":{"description":"ok","content":{"text/plain":{"example":"`+big+`"}}}}}}}}`))

	assert.Empty(t, requestByLabel(t, p, "GET /x").examples)
	assert.Contains(t, portability.Texts(p.warnings), `GET /x: example "200 OK" skipped: body is larger than 256 KB`)
	assert.Zero(t, p.exampleCount)
}

func TestPlan_Swagger2ExampleSizes(t *testing.T) {
	doc := func(items int) []byte {
		list := strings.Repeat(`"abcdefghijklmnopqrstuvwxyz",`, items) + `"z"`
		return []byte(`{"swagger":"2.0","info":{"title":"T","version":"1"},"paths":{"/x":{"get":{"produces":["application/json"],` +
			`"responses":{"200":{"description":"ok","examples":{"application/json":[` + list + `]}}}}}}}`)
	}

	mid := requestByLabel(t, planOf(t, doc(4000)), "GET /x")
	require.Len(t, mid.examples, 1)
	assert.Greater(t, len(mid.examples[0].Body), maxSampleBytes)
	assert.True(t, strings.HasSuffix(mid.examples[0].Body, "\"z\"\n]"))

	big := planOf(t, doc(10000))
	assert.Empty(t, requestByLabel(t, big, "GET /x").examples)
	assert.Contains(t, portability.Texts(big.warnings), `GET /x: example "200 OK" skipped: body is larger than 256 KB`)
}

func TestPlan_UnquotedNumbersInTextFields(t *testing.T) {
	p := planOf(t, []byte(`
openapi: 3.1
info: {title: 2048, version: 1.10}
servers:
  - url: "https://a.example.com:{port}"
    variables:
      port: {default: 8443, enum: [8443, 443]}
tags: [{name: 2024, description: Year}]
paths:
  /x:
    get: {tags: [2024], responses: {}}
`))

	assert.Equal(t, "2048", p.root.Name)
	assert.Equal(t, "Version: 1.10", p.root.Description)
	assert.Equal(t, "https://a.example.com:8443", p.baseURL)
	require.Len(t, p.folders, 1)
	assert.Equal(t, "2024", p.folders[0].in.Name)

	v2 := planOf(t, []byte("swagger: 2.0\ninfo: {title: T, version: 1}\nhost: a.example.com\npaths: {}\n"))
	assert.Equal(t, "Version: 1", v2.root.Description)
	assert.Equal(t, "https://a.example.com", v2.baseURL)
}

func TestPlan_Swagger2(t *testing.T) {
	p := planFile(t, "petstore-2.0.json")

	assert.Equal(t, "Swagger Petstore", p.root.Name)
	assert.Equal(t, entities.AuthTypeBasic, p.root.AuthType)
	require.Len(t, p.folders, 1)
	assert.Equal(t, "Everything about your Pets", p.folders[0].in.Description)
	assert.Equal(t, "https://petstore.swagger.io/v2", p.vars[0].Value)

	get := requestByLabel(t, p, "GET /pet/{petId}")
	assert.Equal(t, "{{baseUrl}}/pet/{{petId}}", get.in.URL)
	require.Len(t, get.examples, 1)
	assert.Equal(t, "{\n  \"name\": \"Rex\",\n  \"id\": 5\n}", get.examples[0].Body)

	add := requestByLabel(t, p, "POST /pet")
	assert.Equal(t, "{\n  \"name\": \"doggie\",\n  \"id\": 0,\n  \"photoUrls\": [\n    \"string\"\n  ]\n}", add.in.Body)
	assert.Equal(t, entities.AuthTypeOAuth2, add.in.AuthType)
	assert.JSONEq(t, `{"grant":"password","tokenUrl":"https://petstore.swagger.io/oauth/token","clientId":"{{clientId}}",`+
		`"clientSecret":"{{clientSecret}}","username":"{{username}}","password":"{{password}}","scope":"read"}`, add.in.AuthData)

	upload := requestByLabel(t, p, "POST /pet/{petId}/uploadImage")
	assert.Equal(t, entities.BodyTypeForm, upload.in.BodyType)
	assert.JSONEq(t, `[{"key":"additionalMetadata","value":"","type":"text","enabled":true},{"key":"file","value":"","type":"file","enabled":true}]`, upload.in.Body)
	assert.Empty(t, p.warnings)
}

func TestPlan_SkippedPartsAreReported(t *testing.T) {
	p := planFile(t, "features-3.1.yaml")

	assert.Equal(t, []string{
		`references to "other.yaml" are not supported and were left empty`,
		`reference "#/components/schemas/Missing" points to nothing and was left empty`,
		"the document names no server: set baseUrl in the environment",
		`security scheme "oidc" (openIdConnect) is not supported: requests that use it were imported without auth`,
		"GET /nodes/{id}: response links were skipped",
		"POST /mixed: callbacks were skipped",
		"webhooks were skipped: newPet",
	}, portability.Texts(p.warnings))

	node := requestByLabel(t, p, "GET /nodes/{id}")
	assert.Equal(t, "getNode", node.in.Name)
	assert.Equal(t, entities.AuthTypeNone, node.in.AuthType)
	require.Len(t, node.examples, 1)
	assert.JSONEq(t, `{"name":"string","children":[{}],"parent":{}}`, node.examples[0].Body)

	mixed := requestByLabel(t, p, "POST /mixed")
	assert.JSONEq(t, `{"version":3,"created":"2024-01-01","kind":"mixed","shape":{"radius":0},"labels":{"key":"string"},"note":"string"}`, mixed.in.Body)
	assert.Empty(t, mixed.examples)
	assert.Equal(t, []environment.AddVariable{{Key: "baseUrl"}, {Key: "id", Value: "n-1"}}, p.vars)
}

func TestPlan_TraceAndPathItemParameters(t *testing.T) {
	p := planOf(t, []byte(`
openapi: 3.0.0
info: {title: T}
paths:
  /items/{id}:
    parameters:
      - {name: id, in: path, required: true, schema: {type: string}}
      - {name: q, in: query, schema: {type: string, default: a}}
    trace: {responses: {}}
    get:
      parameters:
        - {name: q, in: query, schema: {type: string, default: b}}
      responses: {}
`))

	get := requestByLabel(t, p, "GET /items/{id}")
	assert.Equal(t, "{{baseUrl}}/items/{{id}}?q=b", get.in.URL)
	assert.Equal(t, 1, p.requestCount)
	assert.Contains(t, portability.Texts(p.warnings), "TRACE /items/{id} was skipped: TRACE requests are not supported")
}

func TestPlan_OwnServersAreReported(t *testing.T) {
	p := planOf(t, []byte(`
openapi: 3.0.0
info: {title: T}
servers: [{url: "https://api.example.com/"}]
paths:
  /same:
    servers: [{url: "https://api.example.com"}]
    get: {responses: {}}
  /items:
    servers: [{url: "https://{region}.example.com", variables: {region: {default: eu}}}]
    get: {responses: {}}
    post:
      servers: [{url: "https://upload.example.com"}]
      responses: {}
`))

	assert.Equal(t, []string{
		`GET /items: its own server "https://eu.example.com" was ignored, the request uses {{baseUrl}}`,
		`POST /items: its own server "https://upload.example.com" was ignored, the request uses {{baseUrl}}`,
	}, portability.Texts(p.warnings))
	assert.Equal(t, portability.WarnOpenAPIOwnServer, p.warnings[1].Code)
	assert.Equal(t, portability.Subject{Kind: portability.KindOperation, Name: "POST /items"}, p.warnings[1].Subject)
	assert.Equal(t, map[string]string{"url": "https://upload.example.com"}, p.warnings[1].Params)
	assert.Equal(t, "{{baseUrl}}/items", requestByLabel(t, p, "POST /items").in.URL)
}

func TestPlan_LongDescriptionIsTruncated(t *testing.T) {
	long := strings.Repeat("é", domain.MaxDescriptionLen)
	p := planOf(t, []byte(`{"openapi":"3.0.0","info":{"title":"T","description":"`+long+`"},"paths":{}}`))

	assert.LessOrEqual(t, len(p.root.Description), domain.MaxDescriptionLen)
	assert.True(t, strings.HasSuffix(p.root.Description, "é"))
	assert.Contains(t, portability.Texts(p.warnings), `collection "T": description longer than 16,384 bytes was truncated`)
}

func TestPlan_SelfReferencingSchemas(t *testing.T) {
	p := planOf(t, []byte(`openapi: 3.0.0
info: {title: T, version: '1'}
paths:
  /loop:
    post:
      parameters:
        - {name: ids, in: query, schema: {$ref: '#/components/schemas/List'}}
        - {name: loop, in: query, schema: {$ref: '#/components/schemas/Loop'}}
      requestBody:
        content:
          application/json:
            schema: {$ref: '#/components/schemas/Loop'}
      responses:
        '200':
          description: ok
          content:
            application/json:
              schema: {$ref: '#/components/schemas/List'}
components:
  schemas:
    Loop:
      allOf: [{$ref: '#/components/schemas/Loop'}]
    List:
      type: array
      items: {$ref: '#/components/schemas/List'}
`))

	rp := requestByLabel(t, p, "POST /loop")
	assert.Equal(t, "**Parameters**\n\n- `ids` (query, array)\n- `loop` (query)", rp.in.Description)
	assert.Equal(t, "{}", rp.in.Body)
	require.Len(t, rp.examples, 1)
	assert.Equal(t, "[\n  []\n]", rp.examples[0].Body)

	start := time.Now()
	p = planOf(t, []byte(`openapi: 3.0.0
info: {title: T, version: '1'}
paths:
  /fan:
    post:
      requestBody:
        content:
          application/json:
            schema: {$ref: '#/components/schemas/Fan'}
      responses: {'200': {description: ok}}
components:
  schemas:
    Fan:
      type: object
      properties: {a: {type: string}}
      allOf: [`+strings.Join(slices.Repeat([]string{"{$ref: '#/components/schemas/Fan'}"}, 15), ", ")+`]
`))
	assert.Less(t, time.Since(start), time.Second*race.Slowdown)
	assert.Equal(t, "{\n  \"a\": \"string\"\n}", requestByLabel(t, p, "POST /fan").in.Body)
}

func TestPlan_RepeatedAllOfPartsAreMergedOnce(t *testing.T) {
	var schemas strings.Builder
	for i := range 6 {
		ref := fmt.Sprintf("{$ref: '#/components/schemas/S%d'}", i+1)
		fmt.Fprintf(&schemas, "    S%d:\n      properties: {p%d: {type: string}}\n      allOf: [%s]\n", i, i, strings.Join(slices.Repeat([]string{ref}, 15), ", "))
	}
	start := time.Now()
	p := planOf(t, []byte(`openapi: 3.0.0
info: {title: T, version: '1'}
paths:
  /chain:
    post:
      requestBody:
        content:
          application/json:
            schema: {$ref: '#/components/schemas/S0'}
      responses: {'200': {description: ok}}
components:
  schemas:
`+schemas.String()+`    S6:
      properties: {p6: {type: string}}
`))

	assert.Less(t, time.Since(start), time.Second*race.Slowdown)
	assert.Equal(t, `{
  "p6": "string",
  "p5": "string",
  "p4": "string",
  "p3": "string",
  "p2": "string",
  "p1": "string",
  "p0": "string"
}`, requestByLabel(t, p, "POST /chain").in.Body)
}

func TestPlan_PathParametersKeepClearOfCredentialVariables(t *testing.T) {
	p := planOf(t, []byte(`openapi: 3.0.0
info: {title: T, version: '1'}
servers: [{url: 'https://api.example.com'}]
security: [{bearer: []}]
paths:
  /tokens/{token}:
    get:
      parameters: [{name: token, in: path, required: true, example: abc}]
      responses: {'200': {description: ok}}
  /users/{username}/keys/{apiKey}:
    get:
      security: [{basic: []}]
      parameters:
        - {name: username, in: path, required: true, example: ann}
        - {name: apiKey, in: path, required: true}
      responses: {'200': {description: ok}}
components:
  securitySchemes:
    bearer: {type: http, scheme: bearer}
    basic: {type: http, scheme: basic}
`))

	assert.Equal(t, "{{baseUrl}}/tokens/{{tokenParam}}", requestByLabel(t, p, "GET /tokens/{token}").in.URL)
	assert.Equal(t, "{{baseUrl}}/users/{{usernameParam}}/keys/{{apiKeyParam}}", requestByLabel(t, p, "GET /users/{username}/keys/{apiKey}").in.URL)
	assert.Equal(t, []environment.AddVariable{
		{Key: "baseUrl", Value: "https://api.example.com"},
		{Key: "token", IsSecret: true},
		{Key: "tokenParam", Value: "abc"},
		{Key: "usernameParam", Value: "ann"},
		{Key: "apiKeyParam"},
		{Key: "username"},
		{Key: "password", IsSecret: true},
	}, p.vars)
}

func TestPlan_RenamedPathParametersKeepClearOfOtherPathParameters(t *testing.T) {
	p := planOf(t, []byte(`openapi: 3.0.0
info: {title: T, version: '1'}
paths:
  /keys/{token}:
    get:
      parameters: [{name: token, in: path, required: true, example: zero}]
      responses: {'200': {description: ok}}
  /items/{token}/{tokenParam}:
    get:
      parameters:
        - {name: token, in: path, required: true, example: first}
        - {name: tokenParam, in: path, required: true, example: second}
      responses: {'200': {description: ok}}
`))

	assert.Equal(t, "{{baseUrl}}/keys/{{tokenParam2}}", requestByLabel(t, p, "GET /keys/{token}").in.URL)
	assert.Equal(t, "{{baseUrl}}/items/{{tokenParam2}}/{{tokenParam}}", requestByLabel(t, p, "GET /items/{token}/{tokenParam}").in.URL)
	assert.Equal(t, []environment.AddVariable{
		{Key: "baseUrl"},
		{Key: "tokenParam2", Value: "zero"},
		{Key: "tokenParam", Value: "second"},
	}, p.vars)
}

func TestPlan_QueryParameterStyles(t *testing.T) {
	p := planOf(t, []byte(`openapi: 3.0.0
info: {title: T, version: '1'}
paths:
  /items:
    get:
      parameters:
        - {name: id, in: query, schema: {type: array, items: {type: integer}}, example: [1, 2]}
        - {name: tag, in: query, explode: false, schema: {type: array, items: {type: string}}, example: ['a b', 'c,d']}
        - {name: word, in: query, style: spaceDelimited, schema: {type: array, items: {type: string}}, example: [x, y]}
        - {name: pipe, in: query, style: pipeDelimited, schema: {type: array, items: {type: string}}, example: [x, y]}
        - {name: many, in: query, style: pipeDelimited, explode: true, schema: {type: array, items: {type: string}}, example: [x, y]}
        - {name: filter, in: query, style: deepObject, explode: true, schema: {type: object}, example: {team: ops, role: admin}}
        - {name: point, in: query, schema: {type: object}, example: {y: 2, x: 1}}
        - {name: sort, in: query, explode: false, schema: {type: object}, example: {by: name}}
        - {name: odd, in: query, style: pipeDelimited, schema: {type: object}, example: {a: 1}}
      responses: {'200': {description: ok}}
`))

	rp := requestByLabel(t, p, "GET /items")
	assert.Equal(t, "{{baseUrl}}/items?id=1&id=2&tag=a+b,c%2Cd&word=x%20y&pipe=x|y&many=x&many=y"+
		"&filter[role]=admin&filter[team]=ops&x=1&y=2&sort=by,name&odd=%7B%22a%22%3A1%7D", rp.in.URL)
	assert.Contains(t, portability.Texts(p.warnings), `GET /items: query parameter "odd" uses style pipeDelimited, which was not reproduced`)
}

func TestPlan_Swagger2AuthorizationHeaderParameterIsReported(t *testing.T) {
	p := planOf(t, []byte(`{"swagger":"2.0","info":{"title":"T","version":"1"},"host":"api.example.com",
"securityDefinitions":{"key":{"type":"apiKey","in":"header","name":"X-Key"}},
"paths":{
 "/a":{"get":{"parameters":[{"name":"Authorization","in":"header","required":true,"type":"string"}],"responses":{"200":{"description":"ok"}}}},
 "/b":{"post":{"parameters":[{"name":"authorization","in":"header","type":"string"}],"responses":{"200":{"description":"ok"}}}},
 "/c":{"get":{"security":[{"key":[]}],"parameters":[{"name":"Authorization","in":"header","type":"string"}],"responses":{"200":{"description":"ok"}}}},
 "/d":{"get":{"parameters":[{"name":"X-Trace","in":"header","type":"string"}],"responses":{"200":{"description":"ok"}}}}
}}`))

	assert.Empty(t, requestByLabel(t, p, "GET /a").in.Headers)
	assert.Equal(t, entities.AuthTypeAPIKey, requestByLabel(t, p, "GET /c").in.AuthType)
	assert.Equal(t, []string{
		"the Authorization header parameter of GET /a, POST /b was not imported: set the auth on the collection or the request instead",
	}, portability.Texts(p.warnings))
}

func TestPlan_OpenAPI3AuthorizationHeaderParameterStaysQuiet(t *testing.T) {
	p := planOf(t, []byte(`openapi: 3.0.0
info: {title: T, version: '1'}
servers: [{url: 'https://api.example.com'}]
paths:
  /a:
    get:
      parameters: [{name: Authorization, in: header, required: true, schema: {type: string}}]
      responses: {'200': {description: ok}}
`))

	assert.Empty(t, p.warnings)
}

func TestPlan_MultipartWithoutFilesIsReported(t *testing.T) {
	p := planOf(t, []byte(`openapi: 3.0.0
info: {title: T, version: '1'}
servers: [{url: 'https://api.example.com'}]
paths:
  /users:
    post:
      requestBody:
        content:
          multipart/form-data:
            schema: {type: object, properties: {name: {type: string}}}
      responses: {'200': {description: ok}}
  /files:
    post:
      requestBody:
        content:
          multipart/form-data:
            schema: {type: object, properties: {file: {type: string, format: binary}}}
      responses: {'200': {description: ok}}
`))

	assert.JSONEq(t, `[{"key":"name","value":"","type":"text","enabled":true}]`, requestByLabel(t, p, "POST /users").in.Body)
	assert.Equal(t, []string{
		"1 multipart request has no file field and will be sent as a URL-encoded form, not multipart: POST /users",
	}, portability.Texts(p.warnings))
}
