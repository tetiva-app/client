package postman_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tetiva-app/client/internal/adapters/portability/postman"
	"github.com/tetiva-app/client/internal/domain/entities"
	"github.com/tetiva-app/client/internal/domain/usecase/collection"
	"github.com/tetiva-app/client/internal/domain/usecase/example"
	"github.com/tetiva-app/client/internal/domain/usecase/request"
	"github.com/tetiva-app/client/internal/infrastructure/repository/sqlite"
	"github.com/tetiva-app/client/migrations"
	"github.com/tetiva-app/client/pkg/migrate"

	_ "modernc.org/sqlite"
)

var defaultWorkspaceID = uuid.MustParse("00000000-0000-4000-a000-000000000001")

func setupImportDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "test.db"))
	require.NoError(t, err)
	db.SetMaxOpenConns(1)
	_, err = db.Exec("PRAGMA foreign_keys=ON")
	require.NoError(t, err)
	require.NoError(t, migrate.Run(db, migrations.FS, "."))
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func TestImportCollection_ExamplesThroughRealUsecases(t *testing.T) {
	db := setupImportDB(t)
	ctx := context.Background()
	cols := sqlite.NewCollectionRepo(db)
	reqs := sqlite.NewRequestRepo(db)
	collUC := collection.NewUsecase(cols, nil, nil, nil)
	reqUC := request.NewUsecase(reqs, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
	exUC := example.NewUsecase(sqlite.NewResponseExampleRepo(db), reqs, cols)

	raw, err := json.Marshal(map[string]any{
		"info": map[string]any{"name": "Secrets", "schema": postman.SchemaV21},
		"item": []any{map[string]any{
			"name":    "Login",
			"request": map[string]any{"method": "POST", "url": map[string]any{"raw": "{{host}}/login"}},
			"response": []any{
				map[string]any{
					"name": "Masked", "status": "OK", "code": 200, "body": `{"ok":true}`,
					"header": []any{
						map[string]any{"key": "Content-Type", "value": "application/json"},
						map[string]any{"key": "Authorization", "value": "Bearer abc"},
						map[string]any{"key": "{{hdr}}", "value": "literal"},
						map[string]any{"key": "X-Token", "value": "Bearer {{token}}"},
					},
				},
				map[string]any{
					"name": "Too big", "code": 200, "body": strings.Repeat("a", 200*1024),
					"header": []any{map[string]any{"key": "X-Padding", "value": strings.Repeat("b", 300*1024)}},
				},
				map[string]any{"name": "Kept", "status": "Created", "code": 201},
			},
		}},
	})
	require.NoError(t, err)

	res, err := postman.ImportCollection(ctx, raw, postman.ImportOpts{
		WorkspaceID: defaultWorkspaceID, UserID: "local_user",
	}, collUC, reqUC, exUC)
	require.NoError(t, err)
	assert.Equal(t, []string{`request "Login": example 2 skipped: example is too large (max 480 KB)`}, res.Warnings)

	var requestID uuid.UUID
	require.NoError(t, db.QueryRow(`SELECT id FROM requests WHERE name = 'Login'`).Scan(&requestID))

	stored, err := exUC.ListByRequest(ctx, requestID)
	require.NoError(t, err)
	require.Len(t, stored, 2)

	masked := stored[0]
	assert.Equal(t, "Masked", masked.Name)
	assert.Equal(t, defaultWorkspaceID, masked.WorkspaceID)
	assert.Equal(t, "application/json", masked.ContentType)
	assert.Equal(t, []entities.HeaderItem{
		{Key: "Content-Type", Value: "application/json", Enabled: true},
		{Key: "Authorization", Value: "Bearer <redacted>", Enabled: true},
		{Key: "{{hdr}}", Value: "<redacted>", Enabled: true},
		{Key: "X-Token", Value: "Bearer {{token}}", Enabled: true},
	}, masked.Headers)

	assert.Equal(t, "Kept", stored[1].Name)
	assert.Equal(t, 201, stored[1].StatusCode)
}
