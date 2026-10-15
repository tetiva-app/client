package insomnia_test

import (
	"encoding/json"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tetiva-app/client/internal/adapters/portability"
	"github.com/tetiva-app/client/internal/domain/entities"
	"github.com/tetiva-app/client/internal/domain/usecase/environment"
)

func TestImportV4_TreeFollowsInsomniaSortOrder(t *testing.T) {
	s := newStore()
	res := importFile(t, s, fixture(t, "petstore.insomnia.json"), false)

	root, rootID := s.collection(t, "Petstore")
	pets, petsID := s.collection(t, "Pets")
	uploads, _ := s.collection(t, "Uploads")
	assert.Equal(t, rootID, res.CollectionID)
	assert.Nil(t, root.ParentID)
	assert.Equal(t, "Sample Petstore API.", root.Description)
	assert.Equal(t, rootID, *pets.ParentID)
	assert.Equal(t, petsID, *uploads.ParentID)
	assert.Equal(t, "Everything about pets.", pets.Description)

	assert.Equal(t, []string{"Upload photo", "Add pet", "Get pet", "Live feed", "Get feature", "Pets query", "Login"}, s.requestNames())
	assert.Equal(t, 2, res.Folders)
	assert.Equal(t, 7, res.Requests)
	assert.Zero(t, res.Examples)
}

func TestImportV4_HTTPRequest(t *testing.T) {
	s := newStore()
	importFile(t, s, fixture(t, "petstore.insomnia.json"), false)

	get := s.request(t, "Get pet")
	assert.Equal(t, entities.ProtocolHTTP, get.Protocol)
	assert.Equal(t, entities.MethodGET, get.Method)
	assert.Equal(t, "{{base_url}}/pets/{{petId}}?fields=name%2Cstatus", get.URL)
	assert.Equal(t, "Returns a single pet.", get.Description)
	assert.Equal(t, []entities.HeaderItem{
		{Key: "Accept", Value: "application/json", Enabled: true},
		{Key: "X-Trace", Value: "{{trace_id}}", Enabled: false},
	}, get.Headers)
	assert.Equal(t, entities.AuthTypeInherit, get.AuthType)
	assert.Equal(t, entities.BodyTypeNone, get.BodyType)

	add := s.request(t, "Add pet")
	assert.Equal(t, entities.MethodPOST, add.Method)
	assert.Equal(t, entities.BodyTypeJSON, add.BodyType)
	assert.Contains(t, add.Body, `"{% faker 'randomFirstName' %}"`)
	assert.Equal(t, entities.AuthTypeNone, add.AuthType)
	assert.Equal(t, []entities.HeaderItem{{Key: "Content-Type", Value: "application/json", Enabled: true}}, add.Headers)
}

func TestImportV4_Bodies(t *testing.T) {
	s := newStore()
	importFile(t, s, fixture(t, "petstore.insomnia.json"), false)

	upload := s.request(t, "Upload photo")
	assert.Equal(t, entities.BodyTypeForm, upload.BodyType)
	assert.JSONEq(t, `[
		{"key":"caption","value":"{{caption}}","type":"text","enabled":true},
		{"key":"photo","value":"","type":"file","enabled":true},
		{"key":"draft","value":"1","type":"text","enabled":false}
	]`, upload.Body)
	assert.Empty(t, upload.Headers)

	login := s.request(t, "Login")
	assert.Equal(t, entities.BodyTypeForm, login.BodyType)
	assert.JSONEq(t, `[
		{"key":"username","value":"{{user}}","type":"text","enabled":true},
		{"key":"password","value":"{{user-password}}","type":"text","enabled":true},
		{"key":"remember","value":"yes","type":"text","enabled":false}
	]`, login.Body)
	assert.Empty(t, login.Headers)

	gql := s.request(t, "Pets query")
	assert.Equal(t, entities.ProtocolGraphQL, gql.Protocol)
	assert.Equal(t, entities.MethodPOST, gql.Method)
	assert.Equal(t, "query Pets($limit: Int) {\n  pets(limit: $limit, owner: \"{{user}}\") { id name }\n}", gql.GraphQLQuery)
	assert.JSONEq(t, `{"limit":10}`, gql.GraphQLVariables)
	assert.Equal(t, "Pets", gql.GraphQLOperation)
	assert.Equal(t, entities.BodyTypeNone, gql.BodyType)
}

func TestImportV4_GRPCAndWebSocket(t *testing.T) {
	s := newStore()
	importFile(t, s, fixture(t, "petstore.insomnia.json"), false)

	g := s.request(t, "Get feature")
	assert.Equal(t, entities.ProtocolGRPC, g.Protocol)
	assert.Equal(t, "grpcs://{{grpc_host}}", g.URL)
	assert.Equal(t, "routeguide.RouteGuide", g.GRPCService)
	assert.Equal(t, "GetFeature", g.GRPCMethod)
	assert.Equal(t, entities.BodyTypeJSON, g.BodyType)
	assert.JSONEq(t, `{"latitude":409146138,"longitude":-746188906}`, g.Body)
	assert.Equal(t, map[string][]string{"x-tenant": {"{{tenant}}"}}, g.GRPCMetadata)
	assert.Empty(t, g.GRPCProtoPath)

	ws := s.request(t, "Live feed")
	assert.Equal(t, entities.ProtocolWebSocket, ws.Protocol)
	assert.Equal(t, "wss://{{ws_host}}/feed/{{channel}}?since=0", ws.URL)
	assert.Equal(t, []entities.HeaderItem{{Key: "X-Client", Value: "insomnia", Enabled: true}}, ws.Headers)
	assert.Equal(t, entities.AuthTypeBearer, ws.AuthType)
	assert.JSONEq(t, `{"token":"{{token}}","prefix":"Token"}`, ws.AuthData)

	var settings struct {
		Version  int `json:"version"`
		Messages []struct {
			ID, Name, Format, Data string
		} `json:"messages"`
	}
	require.NoError(t, json.Unmarshal([]byte(ws.Body), &settings))
	assert.Equal(t, 1, settings.Version)
	require.Len(t, settings.Messages, 1)
	assert.NotEmpty(t, settings.Messages[0].ID)
	assert.Equal(t, "New Payload", settings.Messages[0].Name)
	assert.Equal(t, "json", settings.Messages[0].Format)
	assert.Equal(t, `{"subscribe": "{{channel_id}}"}`, settings.Messages[0].Data)
}

func TestImportV4_Auth(t *testing.T) {
	s := newStore()
	importFile(t, s, fixture(t, "petstore.insomnia.json"), false)

	pets, _ := s.collection(t, "Pets")
	assert.Equal(t, entities.AuthTypeBearer, pets.AuthType)
	assert.JSONEq(t, `{"token":"{{token}}"}`, pets.AuthData)
	uploads, _ := s.collection(t, "Uploads")
	assert.Equal(t, entities.AuthTypeNone, uploads.AuthType)

	login := s.request(t, "Login")
	assert.Equal(t, entities.AuthTypeBasic, login.AuthType)
	assert.JSONEq(t, `{"username":"{{user}}","password":"secret"}`, login.AuthData)

	upload := s.request(t, "Upload photo")
	assert.Equal(t, entities.AuthTypeAPIKey, upload.AuthType)
	assert.JSONEq(t, `{"key":"X-Api-Key","value":"{{api_key}}","addTo":"header"}`, upload.AuthData)

	gql := s.request(t, "Pets query")
	assert.Equal(t, entities.AuthTypeOAuth2, gql.AuthType)
	assert.JSONEq(t, `{
		"grant":"client_credentials","tokenUrl":"{{auth_url}}/oauth/token","clientId":"petstore-cli",
		"clientSecret":"{{client_secret}}","scope":"pets:read","clientAuth":"body","headerPrefix":""
	}`, gql.AuthData)
}

func TestImportV4_SkipsResourcesItDoesNotRead(t *testing.T) {
	files := map[string]string{
		"json": `{"_type":"export","__export_format":4,"resources":[
			{"_id":"wrk_1","_type":"workspace","name":"Mocked","scope":"collection"},
			{"_id":"req_1","_type":"request","parentId":"wrk_1","name":"Ping","method":"GET","url":"https://api.example.com/ping","body":{}},
			{"_id":"mock_1","_type":"mock","parentId":"wrk_1","name":"Mock","url":"https://mock.insomnia.rest"},
			{"_id":"mock-route_1","_type":"mock_route","parentId":"mock_1","name":"/ping","body":"pong","mimeType":"text/plain","statusCode":200}
		]}`,
		"yaml": `_type: export
__export_format: 4
resources:
  - _id: wrk_1
    _type: workspace
    name: Mocked
  - _id: req_1
    _type: request
    parentId: wrk_1
    name: Ping
    method: GET
    url: https://api.example.com/ping
  - _id: mock-route_1
    _type: mock_route
    parentId: mock_1
    name: /ping
    body: pong
`,
	}
	for name, content := range files {
		t.Run(name, func(t *testing.T) {
			s := newStore()
			res := importFile(t, s, []byte(content), false)

			_, rootID := s.collection(t, "Mocked")
			assert.Equal(t, rootID, res.CollectionID)
			assert.Equal(t, []string{"Ping"}, s.requestNames())
		})
	}
}

func TestImportV4_EnvironmentPerSubEnvironment(t *testing.T) {
	s := newStore()
	res := importFile(t, s, fixture(t, "petstore.insomnia.json"), false)

	assert.Equal(t, []string{"Petstore · Local", "Petstore · Production"}, s.envs)
	assert.Equal(t, []string{"Petstore · Local", "Petstore · Production"}, res.EnvironmentNames)

	base := []environment.AddVariable{
		{Key: "base_url", Value: "{{scheme}}://{{host}}/v2"},
		{Key: "scheme", Value: "https"},
		{Key: "user", Value: "alice"},
		{Key: "user-password", Value: ""},
		{Key: "retries", Value: "3"},
		{Key: "limits.page", Value: "50"},
		{Key: "limits.burst", Value: "true"},
		{Key: "regions", Value: `["eu","us"]`},
		{Key: "api_key", IsSecret: true},
		{Key: "client_secret", IsSecret: true},
	}
	local := append([]environment.AddVariable{}, base...)
	local[1].Value = "http"
	local = append(local,
		environment.AddVariable{Key: "host", Value: "localhost:8080"},
		environment.AddVariable{Key: "auth_url", Value: "http://localhost:9000"},
		environment.AddVariable{Key: "petId", Value: "1"},
		environment.AddVariable{Key: "channel", Value: "pets"},
	)
	assert.Equal(t, local, s.vars["Petstore · Local"])

	prod := append(append([]environment.AddVariable{}, base...),
		environment.AddVariable{Key: "host", Value: "petstore.example.com"},
		environment.AddVariable{Key: "auth_url", Value: "https://auth.example.com"},
		environment.AddVariable{Key: "grpc_host", Value: "grpc.example.com:443"},
		environment.AddVariable{Key: "ws_host", Value: "ws.example.com"},
		environment.AddVariable{Key: "channel", Value: "pets"},
		environment.AddVariable{Key: "petId", Value: "42"},
	)
	assert.Equal(t, prod, s.vars["Petstore · Production"])
}

func TestImportV4_EnvironmentNamesAvoidExistingOnes(t *testing.T) {
	s := newStore()
	s.existing = []string{"Petstore · Local"}
	res := importFile(t, s, fixture(t, "petstore.insomnia.json"), false)

	assert.Equal(t, []string{"Petstore · Local (2)", "Petstore · Production"}, res.EnvironmentNames)
}

func TestImportV4_ScriptsOnlyWhenIncluded(t *testing.T) {
	without := newStore()
	res := importFile(t, without, fixture(t, "petstore.insomnia.json"), false)
	add := without.request(t, "Add pet")
	assert.Empty(t, add.PreScript)
	assert.Empty(t, add.PostScript)
	pets, _ := without.collection(t, "Pets")
	assert.Empty(t, pets.PreScript)
	assert.NotContains(t, portability.Texts(res.Warnings), scriptsWarning)

	with := newStore()
	res = importFile(t, with, fixture(t, "petstore.insomnia.json"), true)
	add = with.request(t, "Add pet")
	assert.Equal(t, "insomnia.environment.set('started', Date.now());", add.PreScript)
	assert.Equal(t, "insomnia.test('created', () => {\n  insomnia.expect(insomnia.response.code).to.eql(201);\n});", add.PostScript)
	pets, _ = with.collection(t, "Pets")
	assert.Equal(t, "insomnia.request.addHeader({ key: 'X-Folder', value: 'pets' });", pets.PreScript)
	assert.Contains(t, portability.Texts(res.Warnings), scriptsWarning)
}

func TestImportV4_Warnings(t *testing.T) {
	s := newStore()
	res := importFile(t, s, fixture(t, "petstore.insomnia.json"), false)

	assert.Equal(t, []string{
		`folder "Uploads": folder environment variables were not imported`,
		`request "Upload photo": file field "photo" was imported without its file; pick it again`,
		`path parameter petId differs between requests: the environment keeps the first value`,
		`1 disabled query parameter was dropped: Tetiva keeps query parameters in the URL`,
		`1 disabled gRPC metadata item was dropped`,
		`gRPC requests were imported without their proto files: pick a .proto file or use server reflection`,
		`2 vault secrets were imported as empty secret variables: fill in their values`,
		`2 Insomnia template tags were left as-is (faker, response): Tetiva does not run them, use variables or scripts instead`,
		`1 unit test was not imported`,
		`1 cookie from the cookie jar was not imported`,
	}, portability.Texts(res.Warnings))
	assert.Equal(t, portability.WarnFileFieldDropped, res.Warnings[1].Code)
	assert.Equal(t, portability.Subject{Kind: portability.KindRequest, Name: "Upload photo"}, res.Warnings[1].Subject)
	assert.Equal(t, map[string]string{"field": "photo"}, res.Warnings[1].Params)
	assert.Equal(t, map[string]string{"n": "2", "names": "faker, response"}, res.Warnings[7].Params)
}

const scriptsWarning = "Insomnia scripts use the insomnia.* API, which Tetiva does not run: rewrite them to pm.* after importing"

func TestImportV4_QueryParametersAreEncoded(t *testing.T) {
	s := newStore()
	importFile(t, s, []byte(`{"_type":"export","__export_format":4,"resources":[`+
		`{"_id":"wrk_1","_type":"workspace","name":"W"},`+
		`{"_id":"req_1","_type":"request","parentId":"wrk_1","name":"Search","method":"GET","url":"{{ _.base }}/search#results",`+
		`"parameters":[{"name":"q","value":"a&b c+d#é"},{"name":"page","value":"{{ _.page }}"}]}]}`), false)

	u := s.request(t, "Search").URL
	assert.Equal(t, "{{base}}/search?q=a%26b+c%2Bd%23%C3%A9&page={{page}}#results", u)
	parsed, err := url.Parse(strings.NewReplacer("{{base}}", "https://e.com", "{{page}}", "2").Replace(u))
	require.NoError(t, err)
	assert.Equal(t, url.Values{"q": {"a&b c+d#é"}, "page": {"2"}}, parsed.Query())
	assert.Equal(t, "results", parsed.Fragment)
}

func TestImportV4_EnvironmentDataWithJSONEscapes(t *testing.T) {
	s := newStore()
	importFile(t, s, []byte(`{"_type":"export","__export_format":4,"resources":[`+
		`{"_id":"wrk_1","_type":"workspace","name":"W"},`+
		`{"_id":"env_1","_type":"environment","parentId":"wrk_1","name":"Base",`+
		`"data":{"zeta":"https:\/\/api.example.com","alpha":"😀","nested":{"b":1,"a":true},"list":["x\/y"]}}]}`), false)

	assert.Equal(t, []environment.AddVariable{
		{Key: "zeta", Value: "https://api.example.com"},
		{Key: "alpha", Value: "😀"},
		{Key: "nested.b", Value: "1"},
		{Key: "nested.a", Value: "true"},
		{Key: "list", Value: `["x/y"]`},
	}, s.vars["W"])
}

func TestImportV4_MultipartWithoutFilesIsReported(t *testing.T) {
	s := newStore()
	res := importFile(t, s, []byte(`{"_type":"export","__export_format":4,"resources":[`+
		`{"_id":"wrk_1","_type":"workspace","name":"W"},`+
		`{"_id":"req_1","_type":"request","parentId":"wrk_1","name":"Sign up","method":"POST","url":"https://e.com/users",`+
		`"body":{"mimeType":"multipart/form-data","params":[{"name":"name","value":"Ann"}]}},`+
		`{"_id":"req_2","_type":"request","parentId":"wrk_1","name":"Profile","method":"POST","url":"https://e.com/me",`+
		`"body":{"mimeType":"multipart/form-data","params":[{"name":"bio","value":"hi"}]}},`+
		`{"_id":"req_3","_type":"request","parentId":"wrk_1","name":"Login","method":"POST","url":"https://e.com/login",`+
		`"body":{"mimeType":"application/x-www-form-urlencoded","params":[{"name":"user","value":"ann"}]}}]}`), false)

	assert.Equal(t, []string{
		`2 multipart requests have no file field and will be sent as a URL-encoded form, not multipart: "Sign up", "Profile"`,
	}, portability.Texts(res.Warnings))
}
