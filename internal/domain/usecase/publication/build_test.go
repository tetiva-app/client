package publication_test

import (
	"regexp"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tetiva-app/client/internal/domain/entities"
	"github.com/tetiva-app/client/internal/domain/usecase/publication"
)

func TestBuild_RejectsMissingRoot(t *testing.T) {
	_, _, err := publication.Build(publication.BuildInput{})
	require.Error(t, err)
}

func TestBuild_RejectsBrokenStoredJSON(t *testing.T) {
	f := newFixture()
	r := f.request(f.in.Root, "r")
	r.AuthType, r.AuthData = entities.AuthTypeBearer, "{not json"
	_, _, err := publication.Build(f.in)
	require.Error(t, err)

	r.AuthType, r.AuthData = entities.AuthTypeNone, "{}"
	r.BodyType, r.Body = entities.BodyTypeForm, "[{broken"
	_, _, err = publication.Build(f.in)
	require.Error(t, err)
}

func TestBuild_LiveTreeOnly(t *testing.T) {
	f := newFixture()
	live := f.folder(f.in.Root, "Live")
	dead := f.folder(f.in.Root, "Dead")
	dead.IsDelete = true
	orphanParent := f.folder(dead, "Under dead")
	f.request(orphanParent, "under dead")
	kept := f.request(live, "kept")
	deleted := f.request(live, "deleted")
	deleted.IsDelete = true
	draft := f.request(live, "draft")
	draft.IsDraft = true
	f.example(kept, "kept example")
	gone := f.example(kept, "gone example")
	gone.IsDelete = true
	f.example(deleted, "example of a deleted request")
	other := &entities.Collection{ID: uuid.New(), WorkspaceID: workspaceID, Name: "Other root"}
	f.in.Collections = append(f.in.Collections, other)
	f.in.Requests = append(f.in.Requests, &entities.Request{ID: uuid.New(), CollectionID: other.ID, Name: "elsewhere", Protocol: entities.ProtocolHTTP})

	s, report := f.build(t)

	require.Len(t, s.Collection.Items, 1)
	liveFolder := s.Collection.Items[0].Folder
	require.NotNil(t, liveFolder)
	assert.Equal(t, "Live", liveFolder.Name)
	require.Len(t, liveFolder.Items, 1)
	req := liveFolder.Items[0].Request
	assert.Equal(t, "kept", req.Name)
	require.Len(t, req.Examples, 1)
	assert.Equal(t, "kept example", req.Examples[0].Name)
	assert.Equal(t, 1, report.Folders)
	assert.Equal(t, 1, report.Requests)
	assert.Equal(t, 1, report.Examples)
}

func TestBuild_OrderIsSortOrderThenCreatedAtThenID(t *testing.T) {
	f := newFixture()
	r1 := f.request(f.in.Root, "second by created_at")
	r0 := f.request(f.in.Root, "first by sort order")
	r0.SortOrder = -1
	tieA := f.request(f.in.Root, "tie a")
	tieB := f.request(f.in.Root, "tie b")
	tieB.CreatedAt, tieB.SortOrder = r1.CreatedAt, r1.SortOrder
	tieA.CreatedAt, tieA.SortOrder = r1.CreatedAt, r1.SortOrder
	folder := f.folder(f.in.Root, "folder after requests in storage order")
	folder.SortOrder = 99

	s, _ := f.build(t)

	var names []string
	for _, it := range s.Collection.Items {
		if it.Folder != nil {
			names = append(names, it.Folder.Name)
		} else {
			names = append(names, it.Request.Name)
		}
	}
	assert.Equal(t, []string{"folder after requests in storage order", "first by sort order", "second by created_at", "tie a", "tie b"}, names)
}

func TestBuild_OpaqueIDs(t *testing.T) {
	f := newFixture()
	r := f.request(f.in.Root, "r")
	ex := f.example(r, "e")

	s, _ := f.build(t)

	idPattern := regexp.MustCompile(`^[0-9a-f]{12}$`)
	out := findRequest(s.Collection.Items, "r")
	assert.Equal(t, f.opaque(f.in.Root.ID), s.Collection.ID)
	assert.Equal(t, f.opaque(r.ID), out.ID)
	assert.Equal(t, f.opaque(ex.ID), out.Examples[0].ID)
	for _, id := range []string{s.Collection.ID, out.ID, out.Examples[0].ID} {
		assert.Regexp(t, idPattern, id)
	}
}

func TestOpaqueID_Vector(t *testing.T) {
	collectionID := uuid.MustParse("00000000-0000-4000-a000-000000000001")
	entityID := uuid.MustParse("00000000-0000-4000-a000-000000000002")

	// printf '%s' <entity> | openssl dgst -sha256 -hmac <collection>
	got := publication.OpaqueID(collectionID, entityID)

	assert.Equal(t, "5b06586a22dc", got)
	for _, part := range strings.Split(entityID.String(), "-") {
		assert.NotContains(t, got, part)
	}
	assert.NotEqual(t, got, publication.OpaqueID(entityID, collectionID))
}

func TestBuild_ReferencesInPublishedFieldsSurvive(t *testing.T) {
	f := newFixture()
	f.in.IncludeScripts = true
	f.in.Root.Description = "Base {{baseUrl}}"
	f.in.Root.GRPCMetadata = []entities.HeaderItem{{Key: "x-{{metaKey}}", Value: "{{metaValue}}", Enabled: true}}
	f.in.Root.PreScript = "pm.variables.get('{{scriptVar}}')"
	folder := f.folder(f.in.Root, "F")
	folder.AuthType = entities.AuthTypeOAuth2
	folder.AuthData = `{"tokenUrl":"{{idp}}/token?client_secret={{cs}}","clientId":"{{cid}}","scope":"{{scope}}"}`

	h := f.request(folder, "http")
	h.URL = "https://{{u}}:{{p}}@{{host}}/pets/{{petId}}?api_key={{k}}&q={{q}}"
	h.Headers = []entities.HeaderItem{
		{Key: "Authorization", Value: "Bearer {{token}}", Enabled: true},
		{Key: "X-{{hdr}}", Value: "{{hv}}", Enabled: true},
	}
	h.BodyType, h.Body = entities.BodyTypeJSON, `{"owner":"{{userId}}","token":"{{bodyToken}}"}`
	h.Description = "{{descVar}}"
	ex := f.example(h, "{{exName}}")
	ex.Body = `{"id":"{{exId}}"}`
	ex.Headers = []entities.HeaderItem{{Key: "Location", Value: "/pets/{{loc}}", Enabled: true}}

	form := f.request(folder, "form")
	form.BodyType = entities.BodyTypeForm
	form.Body = `[{"key":"{{fk}}","value":"{{fv}}","type":"text","enabled":true},{"key":"password","value":"{{fpw}}","type":"text","enabled":true}]`

	gql := f.request(folder, "gql")
	gql.Protocol, gql.URL = entities.ProtocolGraphQL, "{{gqlUrl}}"
	gql.GraphQLQuery, gql.GraphQLVariables, gql.GraphQLOperation = "query { {{gq}} }", `{"id":"{{gv}}"}`, "{{gop}}"

	grpc := f.request(folder, "grpc")
	grpc.Protocol, grpc.URL, grpc.Body = entities.ProtocolGRPC, "{{grpcHost}}", `{"id":"{{gm}}"}`
	grpc.GRPCMetadata = map[string][]string{"authorization": {"Bearer {{gtoken}}"}}

	ws := f.request(folder, "ws")
	ws.Protocol, ws.URL = entities.ProtocolWebSocket, "{{wsUrl}}/events"
	ws.Body = `{"version":1,"subprotocols":["{{sub}}"],"messages":[{"id":"1","name":"{{mn}}","format":"text","data":"{{md}}"}]}`

	out, _ := f.marshal(t)

	for _, ref := range []string{
		"{{baseUrl}}", "{{metaKey}}", "{{metaValue}}", "{{scriptVar}}", "{{idp}}", "{{cs}}", "{{cid}}", "{{scope}}",
		"{{u}}", "{{p}}", "{{host}}", "{{petId}}", "{{k}}", "{{q}}", "{{token}}", "{{hdr}}", "{{hv}}", "{{userId}}",
		"{{bodyToken}}", "{{descVar}}", "{{exName}}", "{{exId}}", "{{loc}}", "{{fk}}", "{{fv}}", "{{fpw}}",
		"{{gqlUrl}}", "{{gq}}", "{{gv}}", "{{gop}}", "{{grpcHost}}", "{{gm}}", "{{gtoken}}", "{{wsUrl}}", "{{sub}}",
		"{{mn}}", "{{md}}",
	} {
		assert.Contains(t, out, ref)
	}
}

func TestBuild_EffectiveGRPCMetadata(t *testing.T) {
	f := newFixture()
	f.in.Root.GRPCMetadata = []entities.HeaderItem{
		{Key: "x-client", Value: "tetiva", Enabled: true},
		{Key: "x-tenant", Value: "root", Enabled: true},
		{Key: "x-off", Value: "disabled", Enabled: false},
	}
	outer := f.folder(f.in.Root, "Outer")
	outer.GRPCMetadata = []entities.HeaderItem{{Key: "X-Tenant", Value: "outer", Enabled: true}, {Key: "x-region", Value: "eu", Enabled: true}}
	inner := f.folder(outer, "Inner")
	r := f.request(inner, "call")
	r.Protocol, r.URL, r.GRPCService, r.GRPCMethod, r.Body = entities.ProtocolGRPC, "grpc.example.com:443", "pets.v1.Pets", "Get", `{"id":1}`
	r.GRPCMetadata = map[string][]string{"x-region": {"us", "ca"}, "a-first": {"1"}}
	r.GRPCProtoPath = "/Users/ivan/pets.proto"

	s, _ := f.build(t)

	grpc := findRequest(s.Collection.Items, "call").GRPC
	require.NotNil(t, grpc)
	assert.Equal(t, publication.GRPCPart{
		Target: "grpc.example.com:443", Service: "pets.v1.Pets", Method: "Get", Message: `{"id":1}`,
		Metadata: []publication.Header{
			{Key: "a-first", Value: "1", Enabled: true},
			{Key: "x-client", Value: "tetiva", Enabled: true},
			{Key: "x-region", Value: "us", Enabled: true},
			{Key: "x-region", Value: "ca", Enabled: true},
			{Key: "X-Tenant", Value: "outer", Enabled: true},
		},
	}, *grpc)
	assert.Len(t, s.Collection.GRPCMetadata, 3, "the root keeps its own rows, disabled ones included")
}

func TestBuild_ProtocolParts(t *testing.T) {
	f := newFixture()
	h := f.request(f.in.Root, "http")
	h.Method, h.BodyType, h.Body = entities.MethodPOST, entities.BodyTypeXML, "<a/>"
	stale := f.request(f.in.Root, "none body")
	stale.BodyType, stale.Body = entities.BodyTypeNone, "left over from json"
	form := f.request(f.in.Root, "form")
	form.BodyType = entities.BodyTypeForm
	form.Body = `[{"key":"q","value":"1","type":"text","enabled":true},{"key":"f","value":"/tmp/a.txt","type":"file","enabled":false},{"key":"x","value":"y","type":"weird","enabled":true}]`
	ws := f.request(f.in.Root, "ws")
	ws.Protocol, ws.URL = entities.ProtocolWebSocket, "wss://ws.example.com"
	ws.Body = `{"version":1,"pingIntervalSec":5,"subprotocols":["json"],"messages":[{"id":"m1","name":"Hi","format":"json","data":"{}"},{"id":"m2","name":"Bin","format":"binary","data":"AAE="}]}`
	gql := f.request(f.in.Root, "gql")
	gql.Protocol, gql.URL, gql.GraphQLQuery, gql.GraphQLVariables, gql.GraphQLOperation = entities.ProtocolGraphQL, "https://g.example.com", "{ a }", "{}", "Op"
	gql.GraphQLSchemaPath = "/Users/ivan/schema.graphql"

	s, report := f.build(t)

	out := findRequest(s.Collection.Items, "http")
	assert.Equal(t, publication.Body{Type: "xml", Raw: "<a/>"}, out.HTTP.Body)
	assert.Nil(t, out.GraphQL)
	assert.Nil(t, out.GRPC)
	assert.Nil(t, out.WebSocket)
	assert.Equal(t, publication.Body{Type: "none"}, findRequest(s.Collection.Items, "none body").HTTP.Body)
	assert.Equal(t, []publication.FormField{
		{Key: "q", Value: "1", Type: "text", Enabled: true},
		{Key: "f", Value: "a.txt", Type: "file"},
		{Key: "x", Value: "y", Type: "text", Enabled: true},
	}, findRequest(s.Collection.Items, "form").HTTP.Body.Fields)
	assert.Equal(t, &publication.WSPart{
		URL: "wss://ws.example.com", Subprotocols: []string{"json"},
		Messages: []publication.WSMessage{{Name: "Hi", Format: "json", Data: "{}"}, {Name: "Bin", Format: "binary", Data: "AAE="}},
	}, findRequest(s.Collection.Items, "ws").WebSocket)
	assert.Equal(t, &publication.GraphQLPart{URL: "https://g.example.com", Query: "{ a }", Variables: "{}", OperationName: "Op"},
		findRequest(s.Collection.Items, "gql").GraphQL)
	assert.Empty(t, report.Errors)
}
