package request_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/tetiva-app/client/internal/adapters/publication/snapshotjson"
	"github.com/tetiva-app/client/internal/adapters/wails/dto"
	"github.com/tetiva-app/client/internal/domain/entities"
	"github.com/tetiva-app/client/internal/domain/usecase/publication"
)

const snapshotHARDir = "testdata/snapshot-har"

type parityCase struct {
	name    string
	secrets []string
	edit    func(r *entities.Request)
}

func parityCases() []parityCase {
	return []parityCase{
		{name: "get_query", edit: func(r *entities.Request) {
			r.URL = "https://api.example.com/users?page=2&q=a+b%20c&tag=x&tag=y"
			r.Headers = []entities.HeaderItem{{Key: "Accept", Value: "application/json", Enabled: true}, {Key: "X-Off", Value: "1"}}
		}},
		{name: "post_json", edit: func(r *entities.Request) {
			r.Method, r.URL, r.BodyType = entities.MethodPOST, "https://api.example.com/users", entities.BodyTypeJSON
			r.Body = "{\n  \"name\": \"neo\",\n  \"role\": \"{{role}}\"\n}"
		}},
		{name: "urlencoded", edit: func(r *entities.Request) {
			r.Method, r.URL, r.BodyType = entities.MethodPOST, "https://api.example.com/search", entities.BodyTypeForm
			r.Body = `[{"key":"q","value":"рыжий кот","type":"text","enabled":true},{"key":"empty","value":"","type":"text","enabled":true},` +
				`{"key":"off","value":"1","type":"text","enabled":false}]`
		}},
		{name: "multipart_file", edit: func(r *entities.Request) {
			r.Method, r.URL, r.BodyType = entities.MethodPOST, "https://api.example.com/upload", entities.BodyTypeForm
			r.Body = `[{"key":"title","value":"Q3","type":"text","enabled":true},{"key":"file","value":"/Users/ivan/report.pdf","type":"file","enabled":true}]`
		}},
		{name: "binary", edit: func(r *entities.Request) {
			r.Method, r.URL, r.BodyType, r.Body = entities.MethodPUT, "https://api.example.com/blobs/1", entities.BodyTypeBinary, `C:\data\payload.bin`
		}},
		{name: "bearer_variable", secrets: []string{"token"}, edit: func(r *entities.Request) {
			r.URL = "{{baseUrl}}/me"
			r.AuthType, r.AuthData = entities.AuthTypeBearer, `{"prefix":"Bearer","token":"{{token}}"}`
		}},
		{name: "api_key_query", secrets: []string{"apiKey"}, edit: func(r *entities.Request) {
			r.URL = "https://api.example.com/pets?limit=10"
			r.AuthType, r.AuthData = entities.AuthTypeAPIKey, `{"key":"api_key","value":"{{apiKey}}","addTo":"query"}`
		}},
		{name: "graphql", edit: func(r *entities.Request) {
			r.Protocol, r.URL = entities.ProtocolGraphQL, "https://api.example.com/graphql"
			r.Headers = []entities.HeaderItem{{Key: "X-Trace", Value: "{{trace}}", Enabled: true}}
			r.GraphQLQuery = "query Pet($id: ID!) {\n  pet(id: $id) { name }\n}"
			r.GraphQLVariables, r.GraphQLOperation = "{\n  \"id\": \"{{petId}}\"\n}", "Pet"
		}},
	}
}

type snapshotHARPair struct {
	Request     json.RawMessage    `json:"request"`
	Environment json.RawMessage    `json:"environment"`
	Auth        json.RawMessage    `json:"auth"`
	HAR         *dto.HARRequestDTO `json:"har"`
}

// buildPair publishes the request as a one-request collection and pairs its snapshot form with the
// snippet HAR built from the saved row; frontend snapshot-har-parity.test.ts replays each pair.
func buildPair(t *testing.T, i int, c parityCase) []byte {
	t.Helper()
	req := snippetRequest(entities.ProtocolHTTP, entities.MethodGET, "https://api.example.com/")
	req.ID = uuid.MustParse(fmt.Sprintf("00000000-0000-4000-b000-%012d", i+1))
	req.Name = c.name
	c.edit(req)

	root := &entities.Collection{ID: testCollectionID, WorkspaceID: testWorkspaceID, Name: "Parity", AuthType: entities.AuthTypeNone, AuthData: "{}"}
	in := publication.BuildInput{Root: root, Collections: []*entities.Collection{root}, Requests: []*entities.Request{req}, Generator: "parity", Locale: "en"}
	if len(c.secrets) > 0 {
		in.Environment = &entities.Environment{ID: uuid.MustParse("00000000-0000-4000-b000-000000000100"), WorkspaceID: testWorkspaceID, Name: "prod"}
		for j, name := range c.secrets {
			in.Variables = append(in.Variables, &entities.Variable{
				ID: uuid.MustParse(fmt.Sprintf("00000000-0000-4000-b000-%012d", 200+j)), EnvironmentID: in.Environment.ID,
				Key: name, Value: "never-published", IsSecret: true, Enabled: true,
			})
		}
	}
	s, report, err := publication.Build(in)
	require.NoError(t, err)
	for _, rd := range report.Redactions {
		require.Equal(t, "file", rd.Category, "a parity case must publish the request unchanged but for file paths")
	}
	raw, err := snapshotjson.Marshal(s)
	require.NoError(t, err)

	var doc struct {
		Collection struct {
			Items []json.RawMessage `json:"items"`
		} `json:"collection"`
		Environment json.RawMessage `json:"environment"`
	}
	require.NoError(t, json.Unmarshal(raw, &doc))
	require.Len(t, doc.Collection.Items, 1)
	request := doc.Collection.Items[0]
	var item struct {
		Auth json.RawMessage `json:"auth"`
	}
	require.NoError(t, json.Unmarshal(request, &item))
	auth := item.Auth
	var authType struct{ Type string }
	require.NoError(t, json.Unmarshal(auth, &authType))
	if authType.Type == "inherit" {
		auth = json.RawMessage("null")
	}

	d := newSnippetDeps(nil)
	har := dto.SnippetInputToDTO(d.build(t, req, false)).HAR
	require.NotNil(t, har)

	out, err := json.MarshalIndent(snapshotHARPair{Request: request, Environment: doc.Environment, Auth: auth, HAR: har}, "", "  ")
	require.NoError(t, err)
	return append(out, '\n')
}

func TestSnapshotHARParityFixturesAreCurrent(t *testing.T) {
	update := os.Getenv("UPDATE_SNAPSHOT_HAR") == "1"
	var names []string
	for i, c := range parityCases() {
		names = append(names, c.name+".json")
		want := buildPair(t, i, c)
		path := filepath.Join(snapshotHARDir, c.name+".json")
		if update {
			require.NoError(t, os.MkdirAll(snapshotHARDir, 0o755))
			require.NoError(t, os.WriteFile(path, want, 0o644))
			continue
		}
		got, err := os.ReadFile(path)
		require.NoError(t, err, "run with UPDATE_SNAPSHOT_HAR=1 to create %s", path)
		require.True(t, bytes.Equal(want, got), "%s is stale; rerun with UPDATE_SNAPSHOT_HAR=1", path)
	}

	entries, err := os.ReadDir(snapshotHARDir)
	require.NoError(t, err)
	var onDisk []string
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".json") {
			onDisk = append(onDisk, e.Name())
		}
	}
	slices.Sort(names)
	require.Equal(t, names, onDisk, "remove fixtures of dropped cases")
}
