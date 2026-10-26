package wails

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/tetiva-app/client/internal/adapters/wails/dto"
	"github.com/tetiva-app/client/internal/domain/entities"
	"github.com/tetiva-app/client/internal/domain/usecase/request"
	"github.com/tetiva-app/client/internal/infrastructure/repository/sqlite"
)

func TestRequestService_UnknownPathVariablesSurviveTheEditor(t *testing.T) {
	db := setupSyncTestDB(t)
	ctx := context.Background()
	coll := &entities.Collection{
		ID: uuid.New(), WorkspaceID: uuid.MustParse("00000000-0000-4000-a000-000000000001"), Name: "Users",
		AuthType: entities.AuthTypeNone, AuthData: "{}", GRPCMetadata: []entities.HeaderItem{}, Version: 1,
		CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}
	require.NoError(t, sqlite.NewCollectionRepo(db).Create(ctx, coll))
	repo := sqlite.NewRequestRepo(db)
	stored := &entities.Request{
		ID: uuid.New(), CollectionID: coll.ID, Name: "Get user", Protocol: entities.ProtocolHTTP, Method: entities.MethodGET,
		URL: "/users/:id", Headers: []entities.HeaderItem{}, BodyType: entities.BodyTypeNone, AuthType: entities.AuthTypeNone,
		AuthData: "{}", GRPCMetadata: map[string][]string{}, Version: 1, CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}
	require.NoError(t, repo.Create(ctx, stored))
	svc := NewRequestService(request.NewUsecase(repo, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil))

	got := svc.GetByID(stored.ID.String())
	require.Nil(t, got.Error)
	sent, err := json.Marshal(got.Data)
	require.NoError(t, err)
	require.Contains(t, string(sent), `"pathVariables":null`)

	var edit dto.EditRequestRequest
	require.NoError(t, json.Unmarshal(sent, &edit))
	edit.Name = "Get one user"
	renamed := svc.Edit(edit)
	require.Nil(t, renamed.Error)
	vars, err := repo.GetPathVariablesByID(ctx, stored.ID)
	require.NoError(t, err)
	require.Nil(t, vars)

	require.NoError(t, json.Unmarshal([]byte(`{"pathVariables":[{"key":"id","value":"7"}]}`), &edit))
	edit.Version = renamed.Data.Version
	filled := svc.Edit(edit)
	require.Nil(t, filled.Error)
	vars, err = repo.GetPathVariablesByID(ctx, stored.ID)
	require.NoError(t, err)
	require.Equal(t, []entities.PathVariable{{Key: "id", Value: "7"}}, vars)
}
