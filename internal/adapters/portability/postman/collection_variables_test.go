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
	"github.com/tetiva-app/client/internal/infrastructure/repository/sqlite"
)

func collectionWithVariables(variable string, urls ...string) []byte {
	if len(urls) == 0 {
		urls = []string{"{{baseUrl}}/pets"}
	}
	items := make([]map[string]any, 0, len(urls))
	for i, u := range urls {
		items = append(items, map[string]any{
			"name":    string(rune('A' + i)),
			"request": map[string]any{"method": "GET", "url": map[string]any{"raw": u}},
		})
	}
	doc := map[string]any{
		"info": map[string]any{"name": "Petstore", "schema": postman.SchemaV21},
		"item": items,
	}
	if variable != "" {
		doc["variable"] = json.RawMessage(variable)
	}
	raw, err := json.Marshal(doc)
	if err != nil {
		panic(err)
	}
	return raw
}

func importWithEnvironments(t *testing.T, data []byte, envUC *stubEnvironmentUC) *postman.ImportResult {
	t.Helper()
	res, err := postman.ImportCollection(context.Background(), data, postman.ImportOpts{
		WorkspaceID: uuid.New(), UserID: "local_user",
	}, &stubCollectionUC{}, &stubRequestUC{}, &stubExampleUC{}, envUC)
	require.NoError(t, err)
	return res
}

func TestImportCollection_VariablesBecomeAnEnvironment(t *testing.T) {
	envUC := &stubEnvironmentUC{}

	res := importWithEnvironments(t, collectionWithVariables(`[`+
		`{"key":"baseUrl","value":"https://petstore.example.com/v1"},`+
		`{"key":"token","value":"s3cr3t","type":"secret"},`+
		`{"key":"legacy","value":"old","disabled":true},`+
		`{"key":"retries","value":5},`+
		`{"key":"verbose","value":true},`+
		`{"key":"empty","value":null},`+
		`{"key":"filter","value":{"a": 1}},`+
		`{"id":"region","value":"eu"},`+
		`{"id":"ignored","key":"kept","value":"k"}]`), envUC)

	require.NotNil(t, envUC.createdEnv)
	assert.Equal(t, "Petstore", envUC.createdEnv.Name)
	assert.Equal(t, "Petstore", res.EnvironmentName)
	assert.NotEqual(t, uuid.Nil, res.EnvironmentID)
	envID := res.EnvironmentID
	assert.Equal(t, []environment.AddVariable{
		{EnvironmentID: envID, Key: "baseUrl", Value: "https://petstore.example.com/v1"},
		{EnvironmentID: envID, Key: "token", Value: "s3cr3t", IsSecret: true},
		{EnvironmentID: envID, Key: "legacy", Value: "old", Disabled: true},
		{EnvironmentID: envID, Key: "retries", Value: "5"},
		{EnvironmentID: envID, Key: "verbose", Value: "true"},
		{EnvironmentID: envID, Key: "empty", Value: ""},
		{EnvironmentID: envID, Key: "filter", Value: `{"a":1}`},
		{EnvironmentID: envID, Key: "region", Value: "eu"},
		{EnvironmentID: envID, Key: "kept", Value: "k"},
	}, envUC.createdVars)
	assert.Empty(t, res.Warnings)
	assert.Equal(t, 1, res.RequestsCreated)
}

func TestImportCollection_NoVariablesNoEnvironment(t *testing.T) {
	for name, variable := range map[string]string{"absent": "", "empty": "[]", "null": "null"} {
		t.Run(name, func(t *testing.T) {
			envUC := &stubEnvironmentUC{}

			res := importWithEnvironments(t, collectionWithVariables(variable), envUC)

			assert.Nil(t, envUC.createdEnv)
			assert.Equal(t, uuid.Nil, res.EnvironmentID)
			assert.Empty(t, res.EnvironmentName)
			assert.Empty(t, res.Warnings)
		})
	}
}

func TestImportCollection_UnreadableVariablesNeverFailTheImport(t *testing.T) {
	cases := map[string]struct {
		variable string
		keys     []string
	}{
		"not an array":       {variable: `"oops"`},
		"an object":          {variable: `{"key":"baseUrl","value":"x"}`},
		"bad elements":       {variable: `[1, {"key": 2}, null, "baseUrl"]`},
		"bad next to a good": {variable: `[1, {"key":"baseUrl","value":"x"}, {"key":3}]`, keys: []string{"baseUrl"}},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			envUC := &stubEnvironmentUC{}

			res := importWithEnvironments(t, collectionWithVariables(tc.variable), envUC)

			assert.Equal(t, 1, res.RequestsCreated)
			assert.Equal(t, []string{"collection variables could not be read and were skipped"}, res.Warnings)
			var keys []string
			for _, v := range envUC.createdVars {
				keys = append(keys, v.Key)
			}
			assert.Equal(t, tc.keys, keys)
			assert.Equal(t, tc.keys != nil, envUC.createdEnv != nil)
		})
	}
}

func TestImportCollection_AnOddFieldKeepsItsVariable(t *testing.T) {
	envUC := &stubEnvironmentUC{}

	res := importWithEnvironments(t, collectionWithVariables(`[`+
		`{"key":"a","value":"1","id":7,"type":["secret"],"disabled":"yes"},`+
		`{"key":2,"id":"b","value":"2"}]`), envUC)

	envID := res.EnvironmentID
	assert.Equal(t, []environment.AddVariable{
		{EnvironmentID: envID, Key: "a", Value: "1"},
		{EnvironmentID: envID, Key: "b", Value: "2"},
	}, envUC.createdVars)
	assert.Empty(t, res.Warnings)
}

func TestImportCollection_UnnamedVariablesAreSkipped(t *testing.T) {
	one := &stubEnvironmentUC{}
	res := importWithEnvironments(t, collectionWithVariables(`[{"key":"","value":"1"}]`), one)
	assert.Nil(t, one.createdEnv)
	assert.Equal(t, []string{"a variable without a name was skipped"}, res.Warnings)

	many := &stubEnvironmentUC{}
	res = importWithEnvironments(t, collectionWithVariables(`[{"key":" ","value":"1"},{"value":"2"},{"key":"a","value":"3"}]`), many)
	require.Len(t, many.createdVars, 1)
	assert.Equal(t, "a", many.createdVars[0].Key)
	assert.Equal(t, []string{"2 variables without a name were skipped"}, res.Warnings)
}

func TestImportCollection_EnvironmentName(t *testing.T) {
	taken := &stubEnvironmentUC{existing: []*entities.Environment{{Name: "Petstore"}, {Name: "petstore (2)"}}}
	res := importWithEnvironments(t, collectionWithVariables(`[{"key":"a","value":"1"}]`), taken)
	assert.Equal(t, "Petstore (2)", res.EnvironmentName)

	blank := &stubEnvironmentUC{}
	raw := []byte(`{"info":{"name":"  ","schema":"` + postman.SchemaV21 + `"},"item":[],"variable":[{"key":"a","value":"1"}]}`)
	res = importWithEnvironments(t, raw, blank)
	assert.Equal(t, "Collection variables", res.EnvironmentName)
}

func TestImporter_ImportReportsTheEnvironment(t *testing.T) {
	envUC := &stubEnvironmentUC{existing: []*entities.Environment{{Name: "Petstore"}}}
	imp := postman.NewImporter(&stubCollectionUC{}, &stubRequestUC{}, &stubExampleUC{}, envUC)

	res, err := imp.Import(context.Background(), collectionWithVariables(`[{"key":"a","value":"1"}]`), portability.ImportOpt{
		WorkspaceID: uuid.New(), UserID: "local_user",
	})
	require.NoError(t, err)

	assert.Equal(t, "Petstore (2)", res.EnvironmentName)
}

func TestImportCollection_SQLite_FixtureVariablesBecomeAnEnvironment(t *testing.T) {
	db := setupImportDB(t)
	ctx := context.Background()
	cols := sqlite.NewCollectionRepo(db)
	reqs := sqlite.NewRequestRepo(db)
	collUC := collection.NewUsecase(cols, nil, nil, nil)
	reqUC := request.NewUsecase(reqs, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
	exUC := example.NewUsecase(sqlite.NewResponseExampleRepo(db), reqs, cols)
	envUC := environment.NewUsecase(sqlite.NewEnvironmentRepo(db), sqlite.NewVariableRepo(db))

	res, err := postman.ImportCollection(ctx, fromSnapshotFixture(t), postman.ImportOpts{
		WorkspaceID: defaultWorkspaceID, UserID: "local_user",
	}, collUC, reqUC, exUC, envUC)
	require.NoError(t, err)

	root, err := collUC.GetByID(ctx, res.RootID)
	require.NoError(t, err)
	assert.Equal(t, "Petstore API — демо", root.Name)
	assert.Equal(t, "Petstore API — демо", res.EnvironmentName)
	env, err := envUC.GetByID(ctx, res.EnvironmentID)
	require.NoError(t, err)
	assert.Equal(t, defaultWorkspaceID, env.WorkspaceID)
	assert.False(t, env.IsActive)

	vars, err := envUC.ListVariables(ctx, res.EnvironmentID)
	require.NoError(t, err)
	var got []string
	for _, v := range vars {
		assert.True(t, v.Enabled, v.Key)
		assert.False(t, v.IsSecret, v.Key)
		got = append(got, v.Key+"="+v.Value)
	}
	assert.Equal(t, []string{
		"baseUrl=https://petstore.example.com/v1",
		"grpcHost=grpc.petstore.example.com:443",
		"wsUrl=wss://ws.petstore.example.com",
		"petId=42",
		"userId=u-1001",
		"tenantHeader=Tenant",
		"user=demo",
		"clientId=tetiva-demo",
		"greeting=Здравствуйте",
	}, got)
}
