package snapshotjson_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/tetiva-app/client/internal/domain/entities"
	"github.com/tetiva-app/client/internal/domain/usecase/publication"
)

var workspaceID = uuid.MustParse("00000000-0000-4000-a000-000000000001")

func fixedID(n int) uuid.UUID {
	return uuid.MustParse(fmt.Sprintf("00000000-0000-4000-a000-%012d", 1000+n))
}

// petstore mirrors testdata/snapshot/all-protocols.json on the entity side: every protocol,
// body type and auth type, nested folders, examples and an environment.
func petstore() publication.BuildInput {
	at := time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC)
	n := 0
	next := func() (uuid.UUID, time.Time) {
		n++
		return fixedID(n), at.Add(time.Duration(n) * time.Second)
	}
	collection := func(name string, parent *entities.Collection) *entities.Collection {
		id, created := next()
		c := &entities.Collection{
			ID: id, WorkspaceID: workspaceID, Name: name, AuthType: entities.AuthTypeNone, AuthData: "{}",
			GRPCMetadata: []entities.HeaderItem{}, Version: 1, CreatedAt: created, UpdatedAt: created,
		}
		if parent != nil {
			pid := parent.ID
			c.ParentID = &pid
		}
		return c
	}
	request := func(parent *entities.Collection, name string, protocol entities.Protocol) *entities.Request {
		id, created := next()
		return &entities.Request{
			ID: id, CollectionID: parent.ID, Name: name, Protocol: protocol, Method: entities.MethodGET,
			Headers: []entities.HeaderItem{}, BodyType: entities.BodyTypeNone, AuthType: entities.AuthTypeInherit, AuthData: "{}",
			GRPCMetadata: map[string][]string{}, Version: 1, CreatedAt: created, UpdatedAt: created,
		}
	}
	example := func(r *entities.Request, name string, code int, text, body, contentType string, headers ...entities.HeaderItem) *entities.ResponseExample {
		id, created := next()
		return &entities.ResponseExample{
			ID: id, RequestID: r.ID, WorkspaceID: workspaceID, Name: name, StatusCode: code, StatusText: text,
			Headers: headers, Body: body, ContentType: contentType, Protocol: r.Protocol, Version: 1, CreatedAt: created,
		}
	}
	h := func(k, v string) entities.HeaderItem { return entities.HeaderItem{Key: k, Value: v, Enabled: true} }

	root := collection("Petstore API — демо", nil)
	root.Description = "# Petstore\n\nДемо-коллекция: **все протоколы**.\n\n```bash\ncurl {{baseUrl}}/pets\n```"
	root.AuthType, root.AuthData = entities.AuthTypeBearer, `{"prefix":"Bearer","token":"{{token}}"}`
	root.PreScript = `pm.environment.set("ts", Date.now());`
	root.GRPCMetadata = []entities.HeaderItem{h("x-client", "tetiva"), h("x-api-version", "2026-09")}

	pets := collection("Питомцы", root)
	pets.Description = "CRUD по питомцам."
	pets.AuthType = entities.AuthTypeOAuth2
	pets.AuthData = `{"grant":"client_credentials","tokenUrl":"https://auth.example.com/oauth/token","scope":"pets:read pets:write",` +
		`"clientAuth":"basic","addTo":"header","headerPrefix":"Bearer","queryParam":"access_token","clientId":"{{clientId}}","clientSecret":"s3cr3t"}`
	pets.PreScript = `console.log("folder pre");`
	pets.GRPCMetadata = []entities.HeaderItem{h("x-tenant", "pets")}
	admin := collection("Администрирование", pets)
	realtime := collection("Realtime", root)
	realtime.SortOrder = 1

	del := request(admin, "Delete pet", entities.ProtocolHTTP)
	del.Method, del.URL = entities.MethodDELETE, "{{baseUrl}}/pets/{{petId}}"
	head := request(admin, "Check pet exists", entities.ProtocolHTTP)
	head.Method, head.URL = entities.MethodHEAD, "{{baseUrl}}/pets/{{petId}}"
	head.AuthType = entities.AuthTypeJWT
	head.AuthData = `{"alg":"HS256","addTo":"header","headerPrefix":"Bearer","queryParam":"token","header":{"kid":"key-2026","typ":"JWT"},` +
		`"claims":{"iss":"https://auth.example.com","sub":"{{userId}}","aud":["petstore","admin"],"iat":1727222400,"admin":true},` +
		`"secret":"{{jwtSecret}}","secretBase64":"false","expiresIn":"3600"}`

	get := request(pets, "Получить питомца", entities.ProtocolHTTP)
	get.Description = "Возвращает питомца по `id`."
	get.URL = "{{baseUrl}}/pets/{{petId}}?expand=owner&api_key=legacy-key"
	get.Headers = []entities.HeaderItem{
		h("Accept", "application/json"), h("Authorization", "Bearer {{token}}"), h("X-Api-Key", "live-key"),
		h("X-{{tenantHeader}}", "acme"), {Key: "X-Debug", Value: "1"},
	}
	create := request(pets, "Create pet", entities.ProtocolHTTP)
	create.Method, create.URL, create.BodyType = entities.MethodPOST, "{{baseUrl}}/pets", entities.BodyTypeJSON
	create.Headers = []entities.HeaderItem{h("Content-Type", "application/json")}
	create.Body = "{\n  \"name\": \"Барсик\",\n  \"ownerId\": \"{{userId}}\"\n}"
	create.AuthType, create.AuthData = entities.AuthTypeBearer, `{"prefix":"Bearer","token":"literal-token"}`
	create.PreScript = `pm.environment.set("requestId", crypto.randomUUID());`
	upload := request(pets, "Загрузить фото", entities.ProtocolHTTP)
	upload.Method, upload.URL, upload.BodyType = entities.MethodPOST, "{{baseUrl}}/pets/{{petId}}/photos", entities.BodyTypeForm
	upload.Body = `[{"key":"title","value":"Отчёт","type":"text","enabled":true},{"key":"file","value":"/Users/ivan/report.pdf","type":"file","enabled":true},` +
		`{"key":"draft","value":"true","type":"text","enabled":false}]`
	upload.AuthType, upload.AuthData = entities.AuthTypeBasic, `{"username":"{{user}}","password":"hunter2"}`
	search := request(pets, "Search (urlencoded)", entities.ProtocolHTTP)
	search.Method, search.URL, search.BodyType = entities.MethodPOST, "{{baseUrl}}/pets/search", entities.BodyTypeForm
	search.Headers = []entities.HeaderItem{h("Content-Type", "application/x-www-form-urlencoded")}
	search.Body = `[{"key":"q","value":"рыжий кот","type":"text","enabled":true},{"key":"limit","value":"20","type":"text","enabled":true}]`
	search.AuthType = entities.AuthTypeNone
	avatar := request(pets, "Replace avatar", entities.ProtocolHTTP)
	avatar.Method, avatar.URL, avatar.BodyType, avatar.Body = entities.MethodPUT, "{{baseUrl}}/pets/{{petId}}/avatar", entities.BodyTypeBinary, `C:\Users\ivan\avatar.png`
	avatar.Headers = []entities.HeaderItem{h("Content-Type", "image/png")}
	avatar.AuthType = entities.AuthTypeAWSSigV4
	avatar.AuthData = `{"region":"eu-central-1","service":"execute-api","accessKeyId":"{{awsKeyId}}","secretAccessKey":"wJalrXUtnFEMI","sessionToken":""}`
	xml := request(pets, "Import XML", entities.ProtocolHTTP)
	xml.Method, xml.URL, xml.BodyType = entities.MethodPATCH, "{{baseUrl}}/pets/import", entities.BodyTypeXML
	xml.Body = `<?xml version="1.0"?><pets><pet id="42">Барсик</pet></pets>`
	xml.AuthType, xml.AuthData = entities.AuthTypeDigest, `{"username":"admin","password":"admin"}`
	ping := request(pets, "Ping", entities.ProtocolHTTP)
	ping.Method, ping.URL, ping.BodyType, ping.Body = entities.MethodOPTIONS, "{{baseUrl}}/ping", entities.BodyTypeRaw, "ping"
	ping.AuthType, ping.AuthData = entities.AuthTypeAPIKey, `{"key":"X-Api-Key","value":"{{apiKey}}","addTo":"header"}`

	gql := request(root, "Pet by id (GraphQL)", entities.ProtocolGraphQL)
	gql.URL = "{{baseUrl}}/graphql"
	gql.Headers = []entities.HeaderItem{h("Authorization", "Bearer {{token}}")}
	gql.GraphQLQuery = "query Pet($id: ID!) {\n  pet(id: $id) { id name }\n}"
	gql.GraphQLVariables, gql.GraphQLOperation = "{\n  \"id\": \"{{petId}}\"\n}", "Pet"
	grpc := request(root, "GetPet (gRPC)", entities.ProtocolGRPC)
	grpc.URL, grpc.GRPCService, grpc.GRPCMethod = "{{grpcHost}}", "petstore.v1.PetService", "GetPet"
	grpc.Body = "{\n  \"id\": \"42\"\n}"
	grpc.GRPCMetadata = map[string][]string{"x-request-id": {"{{requestId}}"}, "authorization": {"Bearer {{token}}"}}
	grpc.GRPCProtoPath = "/Users/ivan/petstore.proto"
	ws := request(realtime, "Чат питомника", entities.ProtocolWebSocket)
	ws.URL = "{{wsUrl}}/events?room=general"
	ws.Headers = []entities.HeaderItem{h("Origin", "https://app.example.com")}
	ws.Body = `{"version":1,"pingIntervalSec":30,"subprotocols":["events.v1","json"],"messages":[` +
		`{"id":"a","name":"Подписка","format":"json","data":"{\"type\":\"subscribe\"}"},` +
		`{"id":"b","name":"Привет","format":"text","data":"Привет, мир"},{"id":"c","name":"Ping frame","format":"binary","data":"3q2+7w=="}]}`
	ws.PostScript = `console.log(pm.response.text());`

	examples := map[uuid.UUID][]*entities.ResponseExample{
		get.ID: {
			example(get, "200 Успех", 200, "OK", "{\n  \"id\": 42,\n  \"name\": \"Барсик\"\n}", "application/json",
				h("Content-Type", "application/json; charset=utf-8"), h("Set-Cookie", "sid=abc")),
			example(get, "404 Не найден", 404, "Not Found", `{"error":"pet not found"}`, "application/json"),
		},
		create.ID: {example(create, "201 Created", 201, "Created", `{"id":43}`, "application/json", h("Location", "/pets/43"))},
		gql.ID:    {example(gql, "200 OK", 200, "OK", `{"data":{"pet":{"id":"42"}}}`, "application/json")},
		grpc.ID: {
			example(grpc, "OK", 0, "OK", `{"id":"42"}`, "application/json", h("content-type", "application/grpc")),
			example(grpc, "NOT_FOUND", 5, "NOT_FOUND", "pet 7 not found", "text/plain"),
		},
	}

	envID, envCreated := next()
	env := &entities.Environment{ID: envID, WorkspaceID: workspaceID, Name: "prod", Version: 1, CreatedAt: envCreated}
	var vars []*entities.Variable
	for _, kv := range []struct {
		key, value string
		secret     bool
	}{
		{"baseUrl", "https://petstore.example.com/v1", false}, {"grpcHost", "grpc.petstore.example.com:443", false},
		{"wsUrl", "wss://ws.petstore.example.com", false}, {"petId", "42", false}, {"userId", "u-1001", false},
		{"tenantHeader", "Tenant", false}, {"user", "demo", false}, {"clientId", "tetiva-demo", false},
		{"token", "", true}, {"apiKey", "", true}, {"jwtSecret", "", true}, {"awsKeyId", "", true},
	} {
		id, created := next()
		vars = append(vars, &entities.Variable{
			ID: id, EnvironmentID: env.ID, Key: kv.key, Value: kv.value, IsSecret: kv.secret, Enabled: true,
			Version: 1, CreatedAt: created,
		})
	}

	return publication.BuildInput{
		Root:           root,
		Collections:    []*entities.Collection{root, pets, admin, realtime},
		Requests:       []*entities.Request{del, head, get, create, upload, search, avatar, xml, ping, gql, grpc, ws},
		Examples:       examples,
		Environment:    env,
		Variables:      vars,
		IncludeScripts: true,
		Generator:      "Tetiva 1.2.0",
		Locale:         "ru",
	}
}

func buildSnapshot(t *testing.T, in publication.BuildInput) (*publication.Snapshot, publication.Report) {
	t.Helper()
	s, report, err := publication.Build(in)
	require.NoError(t, err)
	return s, report
}
