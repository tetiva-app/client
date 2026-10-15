package hoppscotch_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tetiva-app/client/internal/adapters/portability"
	"github.com/tetiva-app/client/internal/adapters/portability/hoppscotch"
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

var workspaceID = uuid.MustParse("00000000-0000-4000-a000-000000000001")

var _ portability.Importer = (*hoppscotch.Importer)(nil)

type stack struct {
	db   *sql.DB
	cols collection.Usecase
	reqs request.Usecase
	exs  example.Usecase
	envs environment.Usecase
	imp  *hoppscotch.Importer
}

func newStack(t *testing.T) *stack {
	t.Helper()
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "test.db"))
	require.NoError(t, err)
	db.SetMaxOpenConns(1)
	_, err = db.Exec("PRAGMA foreign_keys=ON")
	require.NoError(t, err)
	require.NoError(t, migrate.Run(db, migrations.FS, "."))
	t.Cleanup(func() { _ = db.Close() })

	colRepo := sqlite.NewCollectionRepo(db)
	reqRepo := sqlite.NewRequestRepo(db)
	s := &stack{
		db:   db,
		cols: collection.NewUsecase(colRepo, nil, nil, nil),
		reqs: request.NewUsecase(reqRepo, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil),
		exs:  example.NewUsecase(sqlite.NewResponseExampleRepo(db), reqRepo, colRepo),
		envs: environment.NewUsecase(sqlite.NewEnvironmentRepo(db), sqlite.NewVariableRepo(db)),
	}
	s.imp = hoppscotch.NewImporter(s.cols, s.reqs, s.exs, s.envs)
	return s
}

func (s *stack) requests(t *testing.T) map[string]*entities.Request {
	t.Helper()
	ctx := context.Background()
	all, err := s.cols.List(ctx, collection.ListOpt{WorkspaceID: workspaceID})
	require.NoError(t, err)
	out := map[string]*entities.Request{}
	for _, c := range all {
		reqs, err := s.reqs.List(ctx, request.ListOpt{CollectionID: c.ID})
		require.NoError(t, err)
		for _, r := range reqs {
			out[r.Name] = r
		}
	}
	return out
}

func petstore(t *testing.T) []byte {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("testdata", "petstore-v12.json"))
	require.NoError(t, err)
	return raw
}

func TestImport_IntoSQLite(t *testing.T) {
	s := newStack(t)
	ctx := context.Background()

	res, err := s.imp.Import(ctx, petstore(t), portability.ImportOpt{WorkspaceID: workspaceID, UserID: "local_user"})
	require.NoError(t, err)

	assert.Equal(t, 1, res.Folders)
	assert.Equal(t, 6, res.Requests)
	assert.Equal(t, 1, res.Examples)
	assert.Equal(t, []string{"Petstore"}, res.EnvironmentNames)
	assert.NotContains(t, portability.Texts(res.Warnings), "Hoppscotch scripts use the pw.* / hopp.* API, which Tetiva does not run: rewrite them to pm.* after importing")

	root, err := s.cols.GetByID(ctx, res.CollectionID)
	require.NoError(t, err)
	assert.Equal(t, entities.AuthTypeBearer, root.AuthType)
	assert.Empty(t, root.PostScript)

	reqs := s.requests(t)
	require.Len(t, reqs, 6)
	assert.Equal(t, "{{baseUrl}}/pets?limit={{limit}}", reqs["List pets"].URL)
	assert.Empty(t, reqs["List pets"].PreScript)
	assert.Equal(t, entities.ProtocolGraphQL, reqs["Pet by id"].Protocol)
	examples, err := s.exs.ListByRequest(ctx, reqs["List pets"].ID)
	require.NoError(t, err)
	require.Len(t, examples, 1)
	assert.Equal(t, 200, examples[0].StatusCode)

	envs, err := s.envs.List(ctx, environment.ListOpt{WorkspaceID: workspaceID})
	require.NoError(t, err)
	var env *entities.Environment
	for _, e := range envs {
		if e.Name == "Petstore" {
			env = e
		}
	}
	require.NotNil(t, env)
	vars, err := s.envs.ListVariables(ctx, env.ID)
	require.NoError(t, err)
	byKey := map[string]*entities.Variable{}
	for _, v := range vars {
		byKey[v.Key] = v
	}
	assert.Len(t, vars, 7)
	assert.Equal(t, "https://petstore.example.com/v1", byKey["baseUrl"].Value)
	assert.True(t, byKey["token"].IsSecret)
	assert.False(t, byKey["limit"].Enabled)
}

func TestImport_ScriptsOnlyWhenAskedAndIntoAFolder(t *testing.T) {
	s := newStack(t)
	ctx := context.Background()
	parent, err := s.cols.Create(ctx, collection.Create{Name: "Imports"}, collection.CreateOpt{UserID: "local_user", WorkspaceID: workspaceID})
	require.NoError(t, err)

	res, err := s.imp.Import(ctx, petstore(t), portability.ImportOpt{
		WorkspaceID: workspaceID, UserID: "local_user", ParentID: &parent.ID, IncludeScripts: true,
	})
	require.NoError(t, err)

	root, err := s.cols.GetByID(ctx, res.CollectionID)
	require.NoError(t, err)
	assert.Equal(t, parent.ID, *root.ParentID)
	assert.Contains(t, root.PostScript, "pw.test")
	assert.Equal(t, `pw.env.set("limit", "10");`, s.requests(t)["List pets"].PreScript)
	assert.Contains(t, portability.Texts(res.Warnings), "Hoppscotch scripts use the pw.* / hopp.* API, which Tetiva does not run: rewrite them to pm.* after importing")

	again, err := s.imp.Import(ctx, petstore(t), portability.ImportOpt{WorkspaceID: workspaceID, UserID: "local_user"})
	require.NoError(t, err)
	assert.Equal(t, []string{"Petstore (2)"}, again.EnvironmentNames)
}

func TestImportEnvironments_IntoSQLite(t *testing.T) {
	s := newStack(t)
	ctx := context.Background()
	raw, err := os.ReadFile(filepath.Join("testdata", "environments.json"))
	require.NoError(t, err)

	res, err := hoppscotch.ImportEnvironments(ctx, raw, s.envs, portability.ImportOpt{WorkspaceID: workspaceID, UserID: "local_user"})
	require.NoError(t, err)

	assert.Equal(t, []string{"Petstore dev", "Petstore prod"}, res.Names)
	assert.Equal(t, 5, res.Variables)
	assert.Equal(t, []string{"Hoppscotch exports secrets without their values: fill in token"}, portability.Texts(res.Warnings))
	vars, err := s.envs.ListVariables(ctx, res.ID)
	require.NoError(t, err)
	byKey := map[string]string{}
	for _, v := range vars {
		byKey[v.Key] = v.Value
	}
	assert.Equal(t, map[string]string{
		"baseUrl": "https://dev.petstore.example.com/v1", "pets": "{{baseUrl}}/pets", "token": "",
	}, byKey)
}

func TestImport_LargeCollectionImportsInSeconds(t *testing.T) {
	if race.Enabled {
		t.Skip("timings mean nothing under the race detector")
	}
	const folders, perFolder = 50, 100
	var list []map[string]any
	for f := range folders {
		var reqs []any
		for r := range perFolder {
			reqs = append(reqs, map[string]any{
				"v": "17", "name": fmt.Sprintf("Request %d", r), "method": "POST", "endpoint": "<<baseUrl>>/items/" + fmt.Sprint(r),
				"params":  []any{map[string]any{"key": "page", "value": "1", "active": true}},
				"headers": []any{map[string]any{"key": "Accept", "value": "application/json", "active": true}},
				"body":    map[string]any{"contentType": "application/json", "body": `{"name":"<<name>>"}`},
				"auth":    map[string]any{"authType": "inherit", "authActive": true},
				"responses": map[string]any{"OK": map[string]any{
					"name": "OK", "status": "OK", "code": 200, "headers": []any{}, "body": `{"ok":true}`,
				}},
			})
		}
		list = append(list, map[string]any{"v": 12, "name": fmt.Sprintf("Folder %d", f), "folders": []any{}, "requests": reqs})
	}
	data, err := json.Marshal(map[string]any{
		"v": 12, "name": "Big", "folders": list, "requests": []any{},
		"variables": []any{map[string]any{"key": "baseUrl", "initialValue": "https://api.example.com", "currentValue": "", "secret": false}},
	})
	require.NoError(t, err)
	s := newStack(t)

	var res *portability.ImportResult
	start := time.Now()
	err = sqlite.NewTxRunner(s.db).Run(context.Background(), func(ctx context.Context) error {
		var err error
		res, err = s.imp.Import(ctx, data, portability.ImportOpt{WorkspaceID: workspaceID, UserID: "local_user"})
		return err
	})
	elapsed := time.Since(start)
	require.NoError(t, err)

	assert.Equal(t, folders, res.Folders)
	assert.Equal(t, folders*perFolder, res.Requests)
	assert.Equal(t, folders*perFolder, res.Examples)
	assert.Less(t, elapsed, 10*time.Second)
	t.Logf("%d requests imported in %s", res.Requests, elapsed)
}
