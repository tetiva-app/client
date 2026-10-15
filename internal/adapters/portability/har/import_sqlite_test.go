package har_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tetiva-app/client/internal/adapters/portability"
	"github.com/tetiva-app/client/internal/adapters/portability/har"
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

var _ portability.Importer = (*har.Importer)(nil)

type stack struct {
	db   *sql.DB
	cols collection.Usecase
	reqs request.Usecase
	exs  example.Usecase
	imp  *har.Importer
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
	}
	s.imp = har.NewImporter(s.cols, s.reqs, s.exs, environment.NewUsecase(sqlite.NewEnvironmentRepo(db), sqlite.NewVariableRepo(db)))
	return s
}

func (s *stack) count(t *testing.T, table string) int {
	t.Helper()
	var n int
	require.NoError(t, s.db.QueryRow("SELECT COUNT(*) FROM "+table).Scan(&n))
	return n
}

func TestImport_IntoSQLite(t *testing.T) {
	s := newStack(t)
	ctx := context.Background()
	data, err := os.ReadFile(filepath.Join("testdata", "two-hosts.har"))
	require.NoError(t, err)

	res, err := s.imp.Import(ctx, data, portability.ImportOpt{WorkspaceID: workspaceID, UserID: "local_user", FileName: "shop.har"})
	require.NoError(t, err)

	assert.Equal(t, 2, res.Folders)
	assert.Equal(t, 3, res.Requests)
	assert.Equal(t, 3, res.Examples)
	assert.Empty(t, res.EnvironmentNames)
	root, err := s.cols.GetByID(ctx, res.CollectionID)
	require.NoError(t, err)
	assert.Equal(t, "shop", root.Name)
	assert.Equal(t, workspaceID, root.WorkspaceID)

	all, err := s.cols.List(ctx, collection.ListOpt{WorkspaceID: workspaceID})
	require.NoError(t, err)
	var api *entities.Collection
	for _, c := range all {
		if c.Name == "api.example.com" {
			api = c
		}
	}
	require.NotNil(t, api)
	assert.Equal(t, res.CollectionID, *api.ParentID)
	reqs, err := s.reqs.List(ctx, request.ListOpt{CollectionID: api.ID})
	require.NoError(t, err)
	require.Len(t, reqs, 2)
	byName := map[string]*entities.Request{}
	for _, r := range reqs {
		byName[r.Name] = r
		assert.Equal(t, 1, r.Version)
		assert.False(t, r.CreatedAt.IsZero())
	}
	add := byName["POST /v1/cart/items"]
	require.NotNil(t, add)
	assert.Equal(t, entities.BodyTypeJSON, add.BodyType)
	assert.JSONEq(t, `{"sku":"A-1","quantity":2}`, add.Body)

	stored, err := s.exs.ListByRequest(ctx, byName["GET /v1/cart"].ID)
	require.NoError(t, err)
	require.Len(t, stored, 1)
	assert.Equal(t, "200 OK", stored[0].Name)
	assert.Equal(t, workspaceID, stored[0].WorkspaceID)
}

func TestImport_LargeCaptureImportsInSeconds(t *testing.T) {
	if race.Enabled {
		t.Skip("timings mean nothing under the race detector")
	}
	const entries = 3000
	contents := []map[string]any{
		{"mimeType": "application/json", "text": `{"items":[` + strings.Repeat(`{"id":1,"name":"product","price":12.5},`, 50) + `{}]}`},
		{"mimeType": "application/javascript", "text": strings.Repeat("function a(){return 1};", 500)},
		{"mimeType": "image/png", "text": strings.Repeat("QUJD", 1500), "encoding": "base64"},
	}
	list := make([]map[string]any, 0, entries)
	for i := range entries {
		list = append(list, map[string]any{
			"request": map[string]any{
				"method": "POST", "url": fmt.Sprintf("https://api%d.example.com/v1/items/%d?page=%d", i%7, i, i),
				"headers":  []any{map[string]any{"name": "Content-Type", "value": "application/json"}},
				"postData": map[string]any{"mimeType": "application/json", "text": fmt.Sprintf(`{"id":%d}`, i)},
			},
			"response": map[string]any{"status": 200, "statusText": "OK", "content": contents[i%len(contents)]},
		})
	}
	data, err := json.Marshal(map[string]any{"log": map[string]any{"version": "1.2", "entries": list}})
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

	assert.Equal(t, entries, res.Requests)
	assert.Equal(t, entries, res.Examples)
	assert.Equal(t, 7, res.Folders)
	assert.Equal(t, entries, s.count(t, "requests"))
	assert.Less(t, elapsed, 10*time.Second)
	t.Logf("%d entries (%d MB) imported in %s", entries, len(data)>>20, elapsed)
}
