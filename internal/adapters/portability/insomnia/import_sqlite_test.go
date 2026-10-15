package insomnia_test

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
	"github.com/tetiva-app/client/internal/adapters/portability/insomnia"
	"github.com/tetiva-app/client/internal/domain/entities"
	"github.com/tetiva-app/client/internal/domain/usecase/collection"
	"github.com/tetiva-app/client/internal/domain/usecase/environment"
	"github.com/tetiva-app/client/internal/domain/usecase/request"
	"github.com/tetiva-app/client/internal/infrastructure/repository/sqlite"
	"github.com/tetiva-app/client/migrations"
	"github.com/tetiva-app/client/pkg/migrate"

	_ "modernc.org/sqlite"
)

type sqliteStack struct {
	db    *sql.DB
	colls collection.Usecase
	reqs  request.Usecase
	envs  environment.Usecase
	imp   *insomnia.Importer
}

func newSQLiteStack(t *testing.T) *sqliteStack {
	t.Helper()
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "test.db"))
	require.NoError(t, err)
	db.SetMaxOpenConns(1)
	_, err = db.Exec("PRAGMA foreign_keys=ON")
	require.NoError(t, err)
	require.NoError(t, migrate.Run(db, migrations.FS, "."))
	t.Cleanup(func() { _ = db.Close() })

	s := &sqliteStack{
		db:    db,
		colls: collection.NewUsecase(sqlite.NewCollectionRepo(db), nil, nil, nil),
		reqs:  request.NewUsecase(sqlite.NewRequestRepo(db), nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil),
		envs:  environment.NewUsecase(sqlite.NewEnvironmentRepo(db), sqlite.NewVariableRepo(db)),
	}
	s.imp = insomnia.NewImporter(s.colls, s.reqs, s.envs)
	return s
}

func (s *sqliteStack) importInTx(t *testing.T, data []byte, includeScripts bool) *portability.ImportResult {
	t.Helper()
	var res *portability.ImportResult
	err := sqlite.NewTxRunner(s.db).Run(context.Background(), func(ctx context.Context) error {
		var err error
		res, err = s.imp.Import(ctx, data, portability.ImportOpt{WorkspaceID: workspaceID, UserID: "local_user", IncludeScripts: includeScripts})
		return err
	})
	require.NoError(t, err)
	return res
}

func (s *sqliteStack) requestsByName(t *testing.T) map[string]*entities.Request {
	t.Helper()
	all, err := s.colls.List(context.Background(), collection.ListOpt{WorkspaceID: workspaceID})
	require.NoError(t, err)
	out := map[string]*entities.Request{}
	for _, c := range all {
		reqs, err := s.reqs.List(context.Background(), request.ListOpt{CollectionID: c.ID})
		require.NoError(t, err)
		for _, r := range reqs {
			out[r.Name] = r
		}
	}
	return out
}

func TestImport_SQLite_PetstoreEndToEnd(t *testing.T) {
	s := newSQLiteStack(t)
	res := s.importInTx(t, fixture(t, "petstore.insomnia.json"), true)

	root, err := s.colls.GetByID(context.Background(), res.CollectionID)
	require.NoError(t, err)
	assert.Equal(t, "Petstore", root.Name)
	assert.Equal(t, workspaceID, root.WorkspaceID)
	assert.False(t, root.CreatedAt.IsZero())

	reqs := s.requestsByName(t)
	require.Len(t, reqs, 7)
	assert.Equal(t, entities.ProtocolWebSocket, reqs["Live feed"].Protocol)
	assert.Equal(t, entities.BodyTypeRaw, reqs["Live feed"].BodyType)
	assert.Equal(t, entities.ProtocolGRPC, reqs["Get feature"].Protocol)
	assert.Equal(t, map[string][]string{"x-tenant": {"{{tenant}}"}}, reqs["Get feature"].GRPCMetadata)
	assert.Equal(t, entities.ProtocolGraphQL, reqs["Pets query"].Protocol)
	assert.Equal(t, entities.AuthTypeInherit, reqs["Get pet"].AuthType)
	assert.Equal(t, "insomnia.environment.set('started', Date.now());", reqs["Add pet"].PreScript)
	assert.Equal(t, 1, reqs["Get pet"].Version)

	var fields []map[string]any
	require.NoError(t, json.Unmarshal([]byte(reqs["Upload photo"].Body), &fields))
	assert.Equal(t, false, fields[2]["enabled"])

	envs, err := s.envs.List(context.Background(), environment.ListOpt{WorkspaceID: workspaceID})
	require.NoError(t, err)
	byName := map[string]*entities.Environment{}
	for _, e := range envs {
		byName[e.Name] = e
	}
	require.Contains(t, byName, "Petstore · Production")
	require.Contains(t, byName, "Petstore · Local")
	vars, err := s.envs.ListVariables(context.Background(), byName["Petstore · Production"].ID)
	require.NoError(t, err)
	keys := make([]string, 0, len(vars))
	for _, v := range vars {
		keys = append(keys, v.Key)
		if v.Key == "api_key" {
			assert.True(t, v.IsSecret)
			assert.Empty(t, v.Value)
		}
	}
	assert.Equal(t, []string{
		"base_url", "scheme", "user", "user-password", "retries", "limits.page", "limits.burst", "regions",
		"api_key", "client_secret", "host", "auth_url", "grpc_host", "ws_host", "channel", "petId",
	}, keys)
}

func TestImport_SQLite_ImportsUnderAFolderAndTwice(t *testing.T) {
	s := newSQLiteStack(t)
	parent, err := s.colls.Create(context.Background(), collection.Create{Name: "Imports"},
		collection.CreateOpt{UserID: "local_user", WorkspaceID: workspaceID})
	require.NoError(t, err)

	var res *portability.ImportResult
	err = sqlite.NewTxRunner(s.db).Run(context.Background(), func(ctx context.Context) error {
		res, err = s.imp.Import(ctx, fixture(t, "airlines.insomnia.yaml"), portability.ImportOpt{
			WorkspaceID: workspaceID, UserID: "local_user", ParentID: &parent.ID,
		})
		return err
	})
	require.NoError(t, err)
	root, err := s.colls.GetByID(context.Background(), res.CollectionID)
	require.NoError(t, err)
	assert.Equal(t, parent.ID, *root.ParentID)

	again := s.importInTx(t, fixture(t, "airlines.insomnia.yaml"), false)
	assert.Equal(t, []string{"Insomnia Airlines · Staging (2)", "Insomnia Airlines · Empty Env (2)"}, again.EnvironmentNames)
}

func TestImport_SQLite_SkipsBlankEnvironmentKeys(t *testing.T) {
	files := map[string]string{
		"v4 yaml": `_type: export
__export_format: 4
resources:
  - _id: wrk_1
    _type: workspace
    name: Vault
  - _id: env_base
    _type: environment
    parentId: wrk_1
    name: Base Environment
    data:
      "": ""
  - _id: env_blank
    _type: environment
    parentId: env_base
    name: Blank
    metaSortKey: 1
    data:
      "": ""
  - _id: env_prod
    _type: environment
    parentId: env_base
    name: Prod
    metaSortKey: 2
    data:
      host: api.example.com
      "  ": x
`,
		"v5": `type: collection.insomnia.rest/5.0
name: Vault
environments:
  name: Base Environment
  data:
    "": ""
  subEnvironments:
    - name: Blank
      data:
        "": ""
    - name: Prod
      data:
        host: api.example.com
        "  ": x
`,
	}
	for name, content := range files {
		t.Run(name, func(t *testing.T) {
			s := newSQLiteStack(t)
			res := s.importInTx(t, []byte(content), false)

			assert.Equal(t, []string{"Vault · Prod"}, res.EnvironmentNames)
			envs, err := s.envs.List(context.Background(), environment.ListOpt{WorkspaceID: workspaceID})
			require.NoError(t, err)
			names := make([]string, 0, len(envs))
			for _, e := range envs {
				names = append(names, e.Name)
			}
			assert.Equal(t, []string{"Default", "Vault · Prod"}, names)
			vars, err := s.envs.ListVariables(context.Background(), envs[1].ID)
			require.NoError(t, err)
			require.Len(t, vars, 1)
			assert.Equal(t, "host", vars[0].Key)
		})
	}
}

func TestImport_LargeExportIsFast(t *testing.T) {
	const folders, perFolder = 100, 30
	resources := []map[string]any{
		{"_id": "wrk_big", "_type": "workspace", "name": "Big"},
		{"_id": "env_base", "_type": "environment", "parentId": "wrk_big", "data": map[string]any{"base_url": "https://api.example.com"}},
	}
	for f := range folders {
		folderID := fmt.Sprintf("fld_%d", f)
		resources = append(resources, map[string]any{"_id": folderID, "_type": "request_group", "parentId": "wrk_big", "name": folderID, "metaSortKey": -f})
		for r := range perFolder {
			resources = append(resources, map[string]any{
				"_id": fmt.Sprintf("req_%d_%d", f, r), "_type": "request", "parentId": folderID,
				"name": fmt.Sprintf("Request %d", r), "method": "POST", "url": "{{ _.base_url }}/items/:id",
				"metaSortKey": -r,
				"headers":     []map[string]any{{"name": "Authorization", "value": "Bearer {{ _.token }}"}},
				"parameters":  []map[string]any{{"name": "page", "value": "1"}},
				"body":        map[string]any{"mimeType": "application/json", "text": `{"name":"{{ _.name }}","n":1}`},
			})
		}
	}
	data, err := json.Marshal(map[string]any{"_type": "export", "__export_format": 4, "resources": resources})
	require.NoError(t, err)
	s := newSQLiteStack(t)

	start := time.Now()
	preview, err := s.imp.Preview(data, "")
	require.NoError(t, err)
	res := s.importInTx(t, data, false)
	elapsed := time.Since(start)

	assert.Equal(t, folders*perFolder, preview.Requests)
	assert.Equal(t, folders*perFolder, res.Requests)
	assert.Equal(t, folders, res.Folders)
	var stored int
	require.NoError(t, s.db.QueryRow(`SELECT COUNT(*) FROM requests`).Scan(&stored))
	assert.Equal(t, folders*perFolder, stored)
	assert.Less(t, elapsed, 10*time.Second)
}
