package postman_test

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tetiva-app/client/internal/adapters/portability/postman"
	"github.com/tetiva-app/client/internal/domain/usecase/environment"
	"github.com/tetiva-app/client/internal/infrastructure/repository/sqlite"
)

func TestImportEnvironment_SQLite_KeepsFileOrderAndFlags(t *testing.T) {
	ctx := context.Background()
	db := setupImportDB(t)
	envUC := environment.NewUsecase(sqlite.NewEnvironmentRepo(db), sqlite.NewVariableRepo(db))
	var keys []string
	values := make([]map[string]any, 0, 20)
	for i := range 20 {
		key := fmt.Sprintf("var_%02d", (i*7)%20)
		keys = append(keys, key)
		v := map[string]any{"key": key, "value": fmt.Sprint(i)}
		switch i {
		case 3:
			v["enabled"] = false
		case 4:
			v["enabled"] = true
			v["type"] = "secret"
		case 5:
		default:
			v["enabled"] = true
		}
		values = append(values, v)
	}
	raw, err := json.Marshal(map[string]any{"name": "Dev", "values": values, "_postman_variable_scope": "environment"})
	require.NoError(t, err)
	opts := postman.ImportEnvOpts{WorkspaceID: defaultWorkspaceID, UserID: "local_user"}

	res, err := postman.ImportEnvironment(ctx, raw, opts, envUC)
	require.NoError(t, err)

	env, err := envUC.GetByID(ctx, res.EnvironmentID)
	require.NoError(t, err)
	assert.Equal(t, "Dev", env.Name)
	assert.False(t, env.IsActive)
	vars, err := envUC.ListVariables(ctx, res.EnvironmentID)
	require.NoError(t, err)
	var got []string
	for _, v := range vars {
		got = append(got, v.Key)
	}
	assert.Equal(t, keys, got)
	assert.False(t, vars[3].Enabled)
	assert.True(t, vars[4].Enabled)
	assert.True(t, vars[4].IsSecret)
	assert.True(t, vars[5].Enabled)

	again, err := postman.ImportEnvironment(ctx, raw, opts, envUC)
	require.NoError(t, err)
	assert.Equal(t, "Dev (2)", again.EnvironmentName)
	envs, err := envUC.List(ctx, environment.ListOpt{WorkspaceID: defaultWorkspaceID})
	require.NoError(t, err)
	var names []string
	for _, e := range envs {
		names = append(names, e.Name)
	}
	assert.ElementsMatch(t, []string{"Default", "Dev", "Dev (2)"}, names)
}
