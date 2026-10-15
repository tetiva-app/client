package openapi_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tetiva-app/client/internal/adapters/portability"
	"github.com/tetiva-app/client/internal/adapters/portability/openapi"
	"github.com/tetiva-app/client/internal/domain/entities"
	"github.com/tetiva-app/client/internal/domain/usecase/collection"
	"github.com/tetiva-app/client/internal/domain/usecase/environment"
	"github.com/tetiva-app/client/internal/domain/usecase/example"
	"github.com/tetiva-app/client/internal/domain/usecase/request"
	"github.com/tetiva-app/client/internal/infrastructure/repository/sqlite"
	"github.com/tetiva-app/client/internal/race"
	"github.com/tetiva-app/client/migrations"
	"github.com/tetiva-app/client/pkg/migrate"

	_ "modernc.org/sqlite"
)

type store struct {
	db    *sql.DB
	tx    portability.TxRunner
	colls collection.Usecase
	reqs  request.Usecase
	exs   example.Usecase
	envs  environment.Usecase
	imp   *openapi.Importer
}

func newStore(t *testing.T) *store {
	t.Helper()
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "test.db"))
	require.NoError(t, err)
	db.SetMaxOpenConns(1)
	_, err = db.Exec("PRAGMA foreign_keys=ON")
	require.NoError(t, err)
	require.NoError(t, migrate.Run(db, migrations.FS, "."))
	t.Cleanup(func() { _ = db.Close() })

	cols := sqlite.NewCollectionRepo(db)
	reqRepo := sqlite.NewRequestRepo(db)
	s := &store{
		db:    db,
		tx:    sqlite.NewTxRunner(db),
		colls: collection.NewUsecase(cols, nil, nil, nil),
		reqs:  request.NewUsecase(reqRepo, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil),
		exs:   example.NewUsecase(sqlite.NewResponseExampleRepo(db), reqRepo, cols),
		envs:  environment.NewUsecase(sqlite.NewEnvironmentRepo(db), sqlite.NewVariableRepo(db)),
	}
	s.imp = openapi.NewImporter(s.colls, s.reqs, s.exs, s.envs)
	return s
}

func (s *store) importFile(t *testing.T, data []byte) *portability.ImportResult {
	t.Helper()
	var res *portability.ImportResult
	require.NoError(t, s.tx.Run(context.Background(), func(ctx context.Context) error {
		var err error
		res, err = s.imp.Import(ctx, data, portability.ImportOpt{WorkspaceID: workspaceID, UserID: "local_user"})
		return err
	}))
	return res
}

func TestImport_SQLite_Petstore(t *testing.T) {
	ctx := context.Background()
	s := newStore(t)

	res := s.importFile(t, readFile(t, "testdata", "petstore-3.0.yaml"))
	assert.Equal(t, 3, res.Folders)
	assert.Equal(t, 8, res.Requests)
	assert.Equal(t, 7, res.Examples)
	assert.Equal(t, []string{"Petstore"}, res.EnvironmentNames)
	assert.Equal(t, "Petstore", res.ActiveEnvironment)
	assert.Empty(t, res.Warnings)

	all, err := s.colls.List(ctx, collection.ListOpt{WorkspaceID: workspaceID})
	require.NoError(t, err)
	byName := map[string]*entities.Collection{}
	for _, c := range all {
		byName[c.Name] = c
	}
	root := byName["Petstore"]
	require.NotNil(t, root)
	assert.Equal(t, res.CollectionID, root.ID)
	assert.Nil(t, root.ParentID)
	assert.Equal(t, entities.AuthTypeAPIKey, root.AuthType)
	assert.Equal(t, workspaceID, root.WorkspaceID)
	for _, name := range []string{"pet", "user", "store"} {
		require.NotNil(t, byName[name], name)
		assert.Equal(t, root.ID, *byName[name].ParentID, name)
	}

	pets, err := s.reqs.List(ctx, request.ListOpt{CollectionID: byName["pet"].ID})
	require.NoError(t, err)
	require.Len(t, pets, 4)
	var get *entities.Request
	for _, r := range pets {
		if r.Name == "Get a pet" {
			get = r
		}
	}
	require.NotNil(t, get)
	assert.Equal(t, "{{baseUrl}}/pets/{{petId}}", get.URL)
	assert.Equal(t, entities.AuthTypeInherit, get.AuthType)
	assert.Equal(t, 1, get.Version)
	assert.False(t, get.CreatedAt.IsZero())

	stored, err := s.exs.ListByRequest(ctx, get.ID)
	require.NoError(t, err)
	require.Len(t, stored, 2)
	assert.Equal(t, "200 OK", stored[0].Name)
	assert.Equal(t, "404 Not Found", stored[1].Name)
	var body map[string]any
	require.NoError(t, json.Unmarshal([]byte(stored[1].Body), &body))
	assert.Equal(t, "Pet not found", body["message"])

	envs, err := s.envs.List(ctx, environment.ListOpt{WorkspaceID: workspaceID})
	require.NoError(t, err)
	var env *entities.Environment
	for _, e := range envs {
		if e.Name == "Petstore" {
			env = e
		}
	}
	require.NotNil(t, env)
	assert.True(t, env.IsActive)
	vars, err := s.envs.ListVariables(ctx, env.ID)
	require.NoError(t, err)
	got := map[string]string{}
	secret := map[string]bool{}
	for _, v := range vars {
		got[v.Key] = v.Value
		secret[v.Key] = v.IsSecret
	}
	assert.Equal(t, map[string]string{
		"baseUrl": "https://api.petstore.example.com/api/v3", "apiKey": "", "clientId": "", "clientSecret": "", "petId": "42", "token": "",
	}, got)
	assert.True(t, secret["apiKey"])
	assert.True(t, secret["token"])
	assert.False(t, secret["petId"])

	again := s.importFile(t, readFile(t, "testdata", "petstore-3.0.yaml"))
	assert.Equal(t, []string{"Petstore (2)"}, again.EnvironmentNames)
	assert.Equal(t, "Petstore (2)", again.ActiveEnvironment)
	envs, err = s.envs.List(ctx, environment.ListOpt{WorkspaceID: workspaceID})
	require.NoError(t, err)
	active := map[string]bool{}
	for _, e := range envs {
		active[e.Name] = e.IsActive
	}
	assert.Equal(t, map[string]bool{"Default": false, "Petstore": false, "Petstore (2)": true}, active)
}

func TestImport_SQLite_AllFixturesPassValidation(t *testing.T) {
	for _, file := range []string{"petstore-2.0.json", "features-3.1.yaml", "escaped-3.0.json"} {
		t.Run(file, func(t *testing.T) {
			s := newStore(t)
			data := readFile(t, "testdata", file)
			preview, err := s.imp.Preview(data, "")
			require.NoError(t, err)

			res := s.importFile(t, data)

			assert.Equal(t, preview.Requests, res.Requests)
			assert.Equal(t, preview.Examples, res.Examples)
			assert.Equal(t, preview.Folders, res.Folders)
			assert.Equal(t, preview.Warnings, res.Warnings)
		})
	}
}

func largeSpec(paths, schemas int) []byte {
	components := map[string]any{}
	for i := range schemas {
		props := map[string]any{
			"id":      map[string]any{"type": "string", "format": "uuid"},
			"name":    map[string]any{"type": "string"},
			"created": map[string]any{"type": "string", "format": "date-time"},
			"tags":    map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
			"next":    map[string]any{"$ref": fmt.Sprintf("#/components/schemas/S%d", (i+1)%schemas)},
		}
		for j := range 10 {
			props[fmt.Sprintf("field%d", j)] = map[string]any{"type": "integer"}
		}
		components[fmt.Sprintf("S%d", i)] = map[string]any{"type": "object", "properties": props}
	}
	ref := func(i int) map[string]any {
		return map[string]any{"$ref": fmt.Sprintf("#/components/schemas/S%d", i%schemas)}
	}
	pathMap := map[string]any{}
	for i := range paths {
		op := func(method string) map[string]any {
			o := map[string]any{
				"tags":        []string{fmt.Sprintf("tag%d", i%40)},
				"summary":     fmt.Sprintf("%s item %d", method, i),
				"operationId": fmt.Sprintf("%s%d", method, i),
				"responses": map[string]any{
					"200": map[string]any{"description": "ok", "content": map[string]any{"application/json": map[string]any{"schema": ref(i)}}},
					"404": map[string]any{"description": "missing"},
				},
			}
			if method != "get" {
				o["requestBody"] = map[string]any{"content": map[string]any{"application/json": map[string]any{"schema": ref(i + 1)}}}
			}
			return o
		}
		pathMap[fmt.Sprintf("/resource%d/{id}", i)] = map[string]any{
			"parameters": []any{map[string]any{"name": "id", "in": "path", "required": true, "schema": map[string]any{"type": "string"}}},
			"get":        op("get"),
			"put":        op("put"),
			"delete":     op("delete"),
		}
	}
	doc, _ := json.Marshal(map[string]any{
		"openapi":    "3.0.3",
		"info":       map[string]any{"title": "Large", "version": "1"},
		"servers":    []any{map[string]any{"url": "https://api.example.com"}},
		"paths":      pathMap,
		"components": map[string]any{"schemas": components},
	})
	return doc
}

func TestImport_SQLite_LargeDocument(t *testing.T) {
	if race.Enabled {
		t.Skip("timing means nothing under -race")
	}
	data := largeSpec(1500, 300)
	s := newStore(t)

	start := time.Now()
	preview, err := s.imp.Preview(data, "")
	require.NoError(t, err)
	res := s.importFile(t, data)
	elapsed := time.Since(start)

	assert.Equal(t, 4500, preview.Requests)
	assert.Equal(t, 4500, res.Requests)
	assert.Equal(t, 40, res.Folders)
	t.Logf("%d KB document: preview and import of %d requests and %d examples took %s", len(data)/1024, res.Requests, res.Examples, elapsed)
	assert.Less(t, elapsed, 20*time.Second)
}
