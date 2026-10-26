package sync

import (
	"context"
	"database/sql"
	"io/fs"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/tetiva-app/client/internal/domain/entities"
	"github.com/tetiva-app/client/internal/domain/usecase/request"
	"github.com/tetiva-app/client/internal/infrastructure/repository/sqlite"
	"github.com/tetiva-app/client/migrations"
	"github.com/tetiva-app/client/pkg/migrate"
	syncv1 "github.com/tetiva-app/proto/go/gophercourier/sync/v1"
)

func migrationsUpTo(t *testing.T, last int) fs.FS {
	t.Helper()
	entries, err := fs.ReadDir(migrations.FS, ".")
	require.NoError(t, err)
	out := fstest.MapFS{}
	for _, e := range entries {
		version, err := strconv.Atoi(strings.SplitN(e.Name(), "_", 2)[0])
		if err != nil || version > last {
			continue
		}
		data, err := fs.ReadFile(migrations.FS, e.Name())
		require.NoError(t, err)
		out[e.Name()] = &fstest.MapFile{Data: data}
	}
	return out
}

func TestUpgrade_PathVariablesThisDeviceNeverHadAreNotSentAsAClear(t *testing.T) {
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "test.db"))
	require.NoError(t, err)
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = db.Close() })
	require.NoError(t, migrate.Run(db, migrationsUpTo(t, 27), "."))

	collID, reqID := uuid.New(), uuid.New()
	for _, stmt := range []string{
		`INSERT INTO collections (id, workspace_id, name) VALUES ('` + collID.String() + `', '` + testWorkspaceID.String() + `', 'Users')`,
		`INSERT INTO requests (id, collection_id, name, url, headers) VALUES ('` + reqID.String() + `', '` + collID.String() + `', 'Get user', '/users/:id', '[]')`,
	} {
		_, err := db.Exec(stmt)
		require.NoError(t, err)
	}
	require.NoError(t, migrate.Run(db, migrations.FS, "."))

	ctx := context.Background()
	repo := sqlite.NewRequestRepo(db)
	uc := request.NewUsecase(repo, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)

	stored, err := repo.GetByID(ctx, reqID)
	require.NoError(t, err)
	require.Nil(t, stored.PathVariables)

	renamed, err := uc.Edit(ctx, request.Edit{
		Name: "Get one user", Method: entities.MethodGET, URL: stored.URL,
		BodyType: entities.BodyTypeNone, AuthType: entities.AuthTypeNone,
	}, request.EditOpt{RequestID: reqID, UserID: "u", Version: stored.Version})
	require.NoError(t, err)
	require.Nil(t, renamed.PathVariables)
	require.False(t, RequestToProto(renamed, "op").GetRequest().HasPathVariables(), "the queued push must leave the field out")

	engine := NewSyncEngine(NewSyncAuthManager(sqlite.NewSyncConfigRepo(db)), sqlite.NewSyncQueueRepo(db), sqlite.NewSyncConfigRepo(db), db,
		newStubCollectionRepo(), repo, newStubEnvironmentRepo(), newStubVariableRepo(), sqlite.NewResponseExampleRepo(db), nil)
	ws, cancel := newSyncer(engine, testWorkspaceID.String(), StateConnected)
	defer cancel()
	pulled := func(vars *syncv1.PathVariables) *syncv1.SyncEntity {
		e := RequestToProto(renamed, "")
		e.SetVersion(int32(renamed.Version + 1))
		e.GetRequest().SetPathVariables(vars)
		return e
	}

	require.NoError(t, ws.applyEntity(ctx, pulled(nil)))
	got, err := repo.GetPathVariablesByID(ctx, reqID)
	require.NoError(t, err)
	require.Nil(t, got, "a pull from an older peer keeps the values unknown")

	remote := syncv1.PathVariables_builder{Items: []*syncv1.PathVariable{syncv1.PathVariable_builder{Key: "id", Value: "42"}.Build()}}.Build()
	require.NoError(t, ws.applyEntity(ctx, pulled(remote)))
	stored, err = repo.GetByID(ctx, reqID)
	require.NoError(t, err)
	require.Equal(t, []entities.PathVariable{{Key: "id", Value: "42"}}, stored.PathVariables)

	again, err := uc.Edit(ctx, request.Edit{
		Name: "Get user", Method: entities.MethodGET, URL: stored.URL,
		BodyType: entities.BodyTypeNone, AuthType: entities.AuthTypeNone,
	}, request.EditOpt{RequestID: reqID, UserID: "u", Version: stored.Version})
	require.NoError(t, err)
	sent := RequestToProto(again, "op").GetRequest()
	require.True(t, sent.HasPathVariables())
	require.Equal(t, "42", sent.GetPathVariables().GetItems()[0].GetValue())
}
