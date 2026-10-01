package postman_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tetiva-app/client/internal/adapters/portability"
	"github.com/tetiva-app/client/internal/adapters/portability/postman"
	"github.com/tetiva-app/client/internal/domain/entities"
	"github.com/tetiva-app/client/internal/domain/usecase/collection"
	"github.com/tetiva-app/client/internal/domain/usecase/environment"
	"github.com/tetiva-app/client/internal/domain/usecase/example"
	"github.com/tetiva-app/client/internal/domain/usecase/request"
)

var _ portability.Importer = (*postman.Importer)(nil)

func adapterFixture(t *testing.T) []byte {
	t.Helper()
	script := func(listen string, lines ...string) map[string]any {
		return map[string]any{"listen": listen, "script": map[string]any{"type": "text/javascript", "exec": lines}}
	}
	raw, err := json.Marshal(map[string]any{
		"info":  map[string]any{"name": "Shop", "schema": postman.SchemaV21},
		"event": []any{script("prerequest", "pm.environment.set('a', 1)"), script("test", " ")},
		"auth": map[string]any{"type": "oauth2", "oauth2": []any{
			map[string]any{"key": "accessTokenUrl", "value": "https://auth.example.com/token"},
		}},
		"item": []any{
			map[string]any{
				"name":  "Orders",
				"event": []any{script("test", "pm.test('ok', () => {})")},
				"item": []any{
					map[string]any{
						"name":    "List",
						"request": map[string]any{"method": "GET", "url": map[string]any{"raw": "https://shop.example.com/orders"}},
						"response": []any{
							map[string]any{"name": "OK", "code": 200, "body": "[]"},
							map[string]any{"name": "Empty", "code": 204},
						},
					},
					map[string]any{"name": "Nested", "item": []any{}},
				},
			},
			map[string]any{
				"name":  "Upload",
				"event": []any{script("prerequest", "console.log('up')")},
				"request": map[string]any{
					"method": "POST", "url": map[string]any{"raw": "{{host}}/upload"},
					"body": map[string]any{"mode": "formdata", "formdata": []any{
						map[string]any{"key": "file", "type": "file", "src": "/Users/ivan/secret.pem"},
					}},
				},
			},
		},
	})
	require.NoError(t, err)
	return raw
}

type failingCollections struct{}

func (failingCollections) Create(context.Context, collection.Create, collection.CreateOpt) (*entities.Collection, error) {
	panic("preview must not write")
}

type failingRequests struct{}

func (failingRequests) Create(context.Context, request.Create, request.CreateOpt) (*entities.Request, error) {
	panic("preview must not write")
}

type failingExamples struct{}

func (failingExamples) Create(context.Context, example.Create, example.CreateOpt) (*entities.ResponseExample, error) {
	panic("preview must not write")
}

type failingEnvironments struct{}

func (failingEnvironments) Create(context.Context, environment.Create, environment.CreateOpt) (*entities.Environment, error) {
	panic("preview must not write")
}

func (failingEnvironments) AddVariable(context.Context, environment.AddVariable, environment.AddVariableOpt) (*entities.Variable, error) {
	panic("preview must not write")
}

func (failingEnvironments) List(context.Context, environment.ListOpt) ([]*entities.Environment, error) {
	panic("preview must not read the workspace")
}

func TestImporter_Detect(t *testing.T) {
	imp := postman.NewImporter(failingCollections{}, failingRequests{}, failingExamples{}, failingEnvironments{})

	assert.True(t, imp.Detect(adapterFixture(t)))
	assert.True(t, imp.Detect([]byte(`{"info":{"schema":"https://schema.getpostman.com/json/collection/v2.0.0/collection.json"},"item":[]}`)))
	assert.False(t, imp.Detect([]byte(`{"format":"tetiva.collection-snapshot","version":1,"collection":{"items":[]}}`)))
	assert.False(t, imp.Detect([]byte(`{"info":{"schema":"x"},"item":{}}`)))
	assert.False(t, imp.Detect([]byte(`{"info":{},"item":[]}`)))
	assert.False(t, imp.Detect([]byte(`not json`)))
}

func TestImporter_PreviewWritesNothing(t *testing.T) {
	imp := postman.NewImporter(failingCollections{}, failingRequests{}, failingExamples{}, failingEnvironments{})

	p, err := imp.Preview(adapterFixture(t))
	require.NoError(t, err)

	assert.Equal(t, portability.FormatPostman, p.Format)
	assert.Equal(t, "Shop", p.Title)
	assert.Equal(t, 2, p.Folders)
	assert.Equal(t, 2, p.Requests)
	assert.Equal(t, 2, p.Examples)
	assert.Empty(t, p.EnvironmentName)
	assert.Equal(t, []string{"auth.example.com", "shop.example.com", "{{host}}"}, p.Hosts)
	assert.Equal(t, []portability.ScriptPreview{
		{Path: "Shop", Phase: "pre", Text: "pm.environment.set('a', 1)"},
		{Path: "Shop / Orders", Phase: "post", Text: "pm.test('ok', () => {})"},
		{Path: "Shop / Upload", Phase: "pre", Text: "console.log('up')"},
	}, p.Scripts)
	assert.Equal(t, []string{`request "Upload": file field "file" was imported without its file; pick it again`}, p.Warnings)
}

func TestImporter_PreviewResolvesCollectionVariables(t *testing.T) {
	imp := postman.NewImporter(failingCollections{}, failingRequests{}, failingExamples{}, failingEnvironments{})
	data := collectionWithVariables(`[`+
		`{"id":"baseUrl","value":"https://petstore.example.com/v1"},`+
		`{"key":"vault","value":"https://vault.example.com","type":"secret"},`+
		`{"key":"legacy","value":"https://legacy.example.com","disabled":true}]`,
		"{{baseUrl}}/pets", "{{vault}}/keys", "{{legacy}}/old")

	p, err := imp.Preview(data)
	require.NoError(t, err)

	assert.Equal(t, "Petstore", p.EnvironmentName)
	assert.Equal(t, []string{"petstore.example.com", "{{legacy}}", "{{vault}}"}, p.Hosts)
	assert.Empty(t, p.Warnings)
}

func TestImporter_PreviewRejectsAnOlderSchema(t *testing.T) {
	imp := postman.NewImporter(failingCollections{}, failingRequests{}, failingExamples{}, failingEnvironments{})

	_, err := imp.Preview([]byte(`{"info":{"name":"x","schema":"https://schema.getpostman.com/json/collection/v2.0.0/collection.json"},"item":[]}`))
	require.Error(t, err)
}

func TestImporter_ImportPassesParentAndScriptsFlag(t *testing.T) {
	for _, include := range []bool{false, true} {
		collUC := &stubCollectionUC{}
		reqUC := &stubRequestUC{}
		exUC := &stubExampleUC{}
		parent := uuid.New()
		imp := postman.NewImporter(collUC, reqUC, exUC, &stubEnvironmentUC{})

		res, err := imp.Import(context.Background(), adapterFixture(t), portability.ImportOpt{
			WorkspaceID: uuid.New(), ParentID: &parent, UserID: "local_user", IncludeScripts: include,
		})
		require.NoError(t, err)

		require.Len(t, collUC.created, 3)
		assert.Equal(t, &parent, collUC.created[0].ParentID)
		assert.NotEqual(t, uuid.Nil, res.CollectionID)
		assert.Equal(t, 2, res.Folders)
		assert.Equal(t, 2, res.Requests)
		assert.Equal(t, 2, res.Examples)
		if include {
			assert.Equal(t, "pm.environment.set('a', 1)", collUC.created[0].PreScript)
			assert.Equal(t, "console.log('up')", reqUC.created[1].PreScript)
		} else {
			for _, c := range collUC.created {
				assert.Empty(t, c.PreScript+c.PostScript)
			}
			for _, r := range reqUC.created {
				assert.Empty(t, r.PreScript+r.PostScript)
			}
		}
	}
}

func TestImportCollection_ReportsRootAndExamples(t *testing.T) {
	collUC := &stubCollectionUC{}
	exUC := &stubExampleUC{}

	res, err := postman.ImportCollection(context.Background(), adapterFixture(t), postman.ImportOpts{
		WorkspaceID: uuid.New(), UserID: "local_user",
	}, collUC, &stubRequestUC{}, exUC, &stubEnvironmentUC{})
	require.NoError(t, err)

	assert.NotEqual(t, uuid.Nil, res.RootID)
	assert.Equal(t, 2, res.ExamplesCreated)
}
