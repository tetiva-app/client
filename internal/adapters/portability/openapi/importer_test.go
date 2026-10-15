package openapi_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tetiva-app/client/internal/adapters/portability"
	"github.com/tetiva-app/client/internal/adapters/portability/openapi"
	"github.com/tetiva-app/client/internal/domain"
	"github.com/tetiva-app/client/internal/domain/entities"
	"github.com/tetiva-app/client/internal/domain/usecase/collection"
	"github.com/tetiva-app/client/internal/domain/usecase/environment"
	"github.com/tetiva-app/client/internal/domain/usecase/example"
	"github.com/tetiva-app/client/internal/domain/usecase/request"
)

var workspaceID = uuid.MustParse("00000000-0000-4000-a000-000000000001")

func readFile(t *testing.T, path ...string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(path...))
	require.NoError(t, err)
	return data
}

type recorder struct {
	collections []collection.Create
	requests    []request.Create
	examples    []example.Create
	envs        []environment.Create
	vars        []environment.AddVariable
	existing    []*entities.Environment
	activated   []uuid.UUID
	rejectEx    string
}

func (r *recorder) importer() *openapi.Importer {
	return openapi.NewImporter(collections{r}, requests{r}, examples{r}, environments{r})
}

type collections struct{ *recorder }

func (r collections) Create(_ context.Context, in collection.Create, _ collection.CreateOpt) (*entities.Collection, error) {
	r.collections = append(r.collections, in)
	return &entities.Collection{ID: uuid.New(), Name: in.Name, ParentID: in.ParentID}, nil
}

type requests struct{ *recorder }

func (r requests) Create(_ context.Context, in request.Create, _ request.CreateOpt) (*entities.Request, error) {
	r.requests = append(r.requests, in)
	return &entities.Request{ID: uuid.New(), Name: in.Name}, nil
}

type examples struct{ *recorder }

func (r examples) Create(_ context.Context, in example.Create, _ example.CreateOpt) (*entities.ResponseExample, error) {
	if in.Name == r.rejectEx {
		return nil, &domain.ValidationError{Fields: map[string]string{"body": "too large"}}
	}
	r.examples = append(r.examples, in)
	return &entities.ResponseExample{ID: uuid.New(), Name: in.Name}, nil
}

type environments struct{ *recorder }

func (r environments) Create(_ context.Context, in environment.Create, _ environment.CreateOpt) (*entities.Environment, error) {
	r.envs = append(r.envs, in)
	return &entities.Environment{ID: uuid.New(), Name: in.Name}, nil
}

func (r environments) AddVariable(_ context.Context, in environment.AddVariable, _ environment.AddVariableOpt) (*entities.Variable, error) {
	r.vars = append(r.vars, in)
	return &entities.Variable{ID: uuid.New(), Key: in.Key}, nil
}

func (r environments) List(context.Context, environment.ListOpt) ([]*entities.Environment, error) {
	return r.existing, nil
}

func (r environments) SetActive(_ context.Context, opt environment.SetActiveOpt) error {
	r.activated = append(r.activated, opt.EnvironmentID)
	return nil
}

func TestDetect(t *testing.T) {
	imp := (&recorder{}).importer()
	yes := map[string][]byte{
		"3.0 yaml":         readFile(t, "testdata", "petstore-3.0.yaml"),
		"3.1 yaml":         readFile(t, "testdata", "features-3.1.yaml"),
		"2.0 json":         readFile(t, "testdata", "petstore-2.0.json"),
		"escaped json":     readFile(t, "testdata", "escaped-3.0.json"),
		"3.0 json":         []byte(`{"openapi":"3.0.3","info":{"title":"T","version":"1"},"paths":{}}`),
		"unquoted version": []byte("openapi: 3.1\ninfo: {title: T}\n"),
		"swagger yaml":     []byte("swagger: '2.0'\ninfo:\n  title: T\npaths: {}\n"),
	}
	for name, data := range yes {
		assert.True(t, imp.Detect(data), name)
	}

	no := map[string][]byte{
		"tetiva snapshot":  readFile(t, "..", "..", "..", "..", "testdata", "snapshot", "all-protocols.json"),
		"postman env":      []byte(`{"name":"Dev","values":[{"key":"a","value":"1"}],"_postman_variable_scope":"environment"}`),
		"har":              []byte(`{"log":{"version":"1.2","entries":[]}}`),
		"insomnia":         []byte(`{"_type":"export","__export_format":4,"resources":[]}`),
		"swagger 1.2":      []byte(`{"swaggerVersion":"1.2","apis":[]}`),
		"openapi 4":        []byte(`{"openapi":"4.0.0","info":{"title":"T"}}`),
		"no info":          []byte(`{"openapi":"3.0.0","paths":{}}`),
		"info not object":  []byte("openapi: 3.0.0\ninfo: text\n"),
		"compose file":     []byte("services:\n  web:\n    image: nginx\n"),
		"json array":       []byte(`[{"openapi":"3.0.0","info":{}}]`),
		"empty":            {},
		"binary":           {0x00, 0xff, 0x10, 0x7f},
		"version in value": []byte(`{"info":{"title":"openapi: 3.0.0"},"item":[]}`),
	}
	postmanFiles, err := filepath.Glob(filepath.Join("..", "postman", "testdata", "*.json"))
	require.NoError(t, err)
	require.NotEmpty(t, postmanFiles)
	for _, f := range postmanFiles {
		no[filepath.Base(f)] = readFile(t, f)
	}
	for name, data := range no {
		assert.False(t, imp.Detect(data), name)
	}
}

func TestPreview(t *testing.T) {
	cases := map[string]struct {
		file string
		want portability.ImportPreview
	}{
		"openapi 3.0": {
			file: "petstore-3.0.yaml",
			want: portability.ImportPreview{
				Format: "openapi", Title: "Petstore", Folders: 3, Requests: 8, Examples: 7, EnvironmentNames: []string{"Petstore"},
				Hosts: []string{"api.petstore.example.com"}, Scripts: []portability.ScriptPreview{}, Warnings: []string{},
			},
		},
		"swagger 2.0": {
			file: "petstore-2.0.json",
			want: portability.ImportPreview{
				Format: "openapi", Title: "Swagger Petstore", Folders: 1, Requests: 3, Examples: 2, EnvironmentNames: []string{"Swagger Petstore"},
				Hosts: []string{"petstore.swagger.io"}, Scripts: []portability.ScriptPreview{}, Warnings: []string{},
			},
		},
		"json escapes yaml rejects": {
			file: "escaped-3.0.json",
			want: portability.ImportPreview{
				Format: "openapi", Title: "Emoji \U0001F680 API", Folders: 1, Requests: 1, Examples: 1, EnvironmentNames: []string{"Emoji \U0001F680 API"},
				Hosts: []string{"api.example.com"}, Scripts: []portability.ScriptPreview{}, Warnings: []string{},
			},
		},
		"openapi 3.1": {
			file: "features-3.1.yaml",
			want: portability.ImportPreview{
				Format: "openapi", Title: "Features", Folders: 0, Requests: 2, Examples: 1, EnvironmentNames: []string{"Features"},
				Hosts: []string{}, Scripts: []portability.ScriptPreview{},
				Warnings: []string{
					`references to "other.yaml" are not supported and were left empty`,
					`reference "#/components/schemas/Missing" points to nothing and was left empty`,
					"the document names no server: set baseUrl in the environment",
					`security scheme "oidc" (openIdConnect) is not supported: requests that use it were imported without auth`,
					"GET /nodes/{id}: response links were skipped",
					"POST /mixed: callbacks were skipped",
					"webhooks were skipped: newPet",
				},
			},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			rec := &recorder{}

			got, err := rec.importer().Preview(readFile(t, "testdata", tc.file), "")
			require.NoError(t, err)

			assert.Equal(t, &tc.want, got)
			assert.Empty(t, rec.collections)
			assert.Empty(t, rec.requests)
		})
	}
}

func TestPreview_UnreadableDocument(t *testing.T) {
	_, err := (&recorder{}).importer().Preview([]byte(`{"openapi":"3.0.0","info":{"title":"T"},"paths":[1,2]}`), "")

	var ve *domain.ValidationError
	require.ErrorAs(t, err, &ve)
	assert.Contains(t, ve.Fields["content"], "the OpenAPI document can't be read")
}

func TestImport_WritesThePlan(t *testing.T) {
	rec := &recorder{existing: []*entities.Environment{{Name: "Petstore"}}, rejectEx: "404 Not Found"}
	parent := uuid.New()

	res, err := rec.importer().Import(context.Background(), readFile(t, "testdata", "petstore-3.0.yaml"),
		portability.ImportOpt{WorkspaceID: workspaceID, UserID: "local_user", ParentID: &parent})
	require.NoError(t, err)

	assert.Equal(t, 3, res.Folders)
	assert.Equal(t, 8, res.Requests)
	assert.Equal(t, 6, res.Examples)
	assert.Equal(t, []string{"Petstore (2)"}, res.EnvironmentNames)
	assert.Equal(t, "Petstore (2)", res.ActiveEnvironment)
	assert.Len(t, rec.activated, 1)
	assert.Equal(t, []string{`GET /pets/{petId}: example "404 Not Found" skipped: too large`}, res.Warnings)

	require.Len(t, rec.collections, 4)
	assert.Equal(t, &parent, rec.collections[0].ParentID)
	assert.Equal(t, []environment.Create{{Name: "Petstore (2)"}}, rec.envs)
	assert.Len(t, rec.vars, 6)
	for _, r := range rec.requests {
		assert.NotEqual(t, uuid.Nil, r.CollectionID, r.Name)
	}
}
