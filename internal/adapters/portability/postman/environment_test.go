package postman_test

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tetiva-app/client/internal/adapters/portability"
	"github.com/tetiva-app/client/internal/adapters/portability/postman"
	"github.com/tetiva-app/client/internal/domain"
	"github.com/tetiva-app/client/internal/domain/entities"
	"github.com/tetiva-app/client/internal/domain/usecase/environment"
)

type stubEnvironmentUC struct {
	existing    []*entities.Environment
	createdEnv  *environment.Create
	createdVars []environment.AddVariable
}

func (s *stubEnvironmentUC) Create(_ context.Context, input environment.Create, _ environment.CreateOpt) (*entities.Environment, error) {
	s.createdEnv = &input
	return &entities.Environment{ID: uuid.New(), Name: input.Name}, nil
}

func (s *stubEnvironmentUC) AddVariable(_ context.Context, input environment.AddVariable, _ environment.AddVariableOpt) (*entities.Variable, error) {
	s.createdVars = append(s.createdVars, input)
	return &entities.Variable{ID: uuid.New(), Key: input.Key, Value: input.Value}, nil
}

func (s *stubEnvironmentUC) List(context.Context, environment.ListOpt) ([]*entities.Environment, error) {
	return s.existing, nil
}

func importEnv(t *testing.T, envUC *stubEnvironmentUC, content string) (*postman.ImportEnvResult, error) {
	t.Helper()
	return postman.ImportEnvironment(context.Background(), []byte(content), postman.ImportEnvOpts{
		WorkspaceID: uuid.New(),
		UserID:      "local_user",
	}, envUC)
}

func TestImportEnvironment(t *testing.T) {
	envUC := &stubEnvironmentUC{}

	result, err := importEnv(t, envUC, `{"name":"Dev","values":[
		{"key":"host","value":"https://api.dev.example.com","enabled":true},
		{"key":"token","value":"secret123","type":"secret","enabled":true},
		{"key":"disabled_var","value":"unused","enabled":false},
		{"key":"legacy","value":"no enabled field"}
	],"_postman_variable_scope":"environment"}`)

	require.NoError(t, err)
	assert.Equal(t, "Dev", result.EnvironmentName)
	assert.NotEqual(t, uuid.Nil, result.EnvironmentID)
	assert.Equal(t, 4, result.VariablesCreated)
	assert.Empty(t, result.Warnings)
	require.NotNil(t, envUC.createdEnv)
	assert.Equal(t, "Dev", envUC.createdEnv.Name)
	assert.Equal(t, []environment.AddVariable{
		{EnvironmentID: result.EnvironmentID, Key: "host", Value: "https://api.dev.example.com"},
		{EnvironmentID: result.EnvironmentID, Key: "token", Value: "secret123", IsSecret: true},
		{EnvironmentID: result.EnvironmentID, Key: "disabled_var", Value: "unused", Disabled: true},
		{EnvironmentID: result.EnvironmentID, Key: "legacy", Value: "no enabled field"},
	}, envUC.createdVars)
}

func TestImportEnvironment_ValuesOfAnyType(t *testing.T) {
	envUC := &stubEnvironmentUC{}

	_, err := importEnv(t, envUC, `{"name":"Dev","values":[
		{"key":"n","value":5},
		{"key":"f","value":1.50},
		{"key":"b","value":true},
		{"key":"z","value":null},
		{"key":"missing"},
		{"key":"o","value":{ "a" : 1 }},
		{"key":"a","value":[1, "x"]}
	]}`)

	require.NoError(t, err)
	values := map[string]string{}
	for _, v := range envUC.createdVars {
		values[v.Key] = v.Value
	}
	assert.Equal(t, map[string]string{
		"n": "5", "f": "1.50", "b": "true", "z": "", "missing": "", "o": `{"a":1}`, "a": `[1,"x"]`,
	}, values)
}

func TestImportEnvironment_SkipsVariablesWithoutAName(t *testing.T) {
	one := &stubEnvironmentUC{}
	res, err := importEnv(t, one, `{"name":"Dev","values":[{"key":"","value":"x"},{"key":"kept","value":"y"}]}`)
	require.NoError(t, err)
	assert.Equal(t, 1, res.VariablesCreated)
	assert.Equal(t, []string{"a variable without a name was skipped"}, res.Warnings)
	require.Len(t, one.createdVars, 1)
	assert.Equal(t, "kept", one.createdVars[0].Key)

	many := &stubEnvironmentUC{}
	res, err = importEnv(t, many, `{"name":"Dev","values":[{"key":"  "},{"value":"x"},{"key":"\t"}]}`)
	require.NoError(t, err)
	assert.Zero(t, res.VariablesCreated)
	assert.Equal(t, []string{"3 variables without a name were skipped"}, res.Warnings)
	assert.Empty(t, many.createdVars)
	assert.NotNil(t, many.createdEnv)
}

func TestImportEnvironment_NameTakenGetsASuffix(t *testing.T) {
	envUC := &stubEnvironmentUC{existing: []*entities.Environment{{Name: "Dev"}, {Name: "Dev (2)"}, {Name: "dev"}}}

	res, err := importEnv(t, envUC, `{"name":"  Dev  ","values":[]}`)

	require.NoError(t, err)
	assert.Equal(t, "Dev (3)", res.EnvironmentName)
	assert.Equal(t, "Dev (3)", envUC.createdEnv.Name)
}

func TestImportEnvironment_Globals(t *testing.T) {
	unnamed := &stubEnvironmentUC{existing: []*entities.Environment{{Name: "Globals"}}}
	res, err := importEnv(t, unnamed, `{"name":"","values":[{"key":"k","value":"v"}],"_postman_variable_scope":"globals"}`)
	require.NoError(t, err)
	assert.Equal(t, "Globals (2)", res.EnvironmentName)
	assert.Equal(t, []string{`Tetiva has no global variables: they were imported as the environment "Globals (2)"`}, res.Warnings)

	named := &stubEnvironmentUC{}
	res, err = importEnv(t, named, `{"name":"My Workspace Globals","values":[{"key":""}],"_postman_variable_scope":"globals"}`)
	require.NoError(t, err)
	assert.Equal(t, "My Workspace Globals", res.EnvironmentName)
	assert.Equal(t, []string{
		`Tetiva has no global variables: they were imported as the environment "My Workspace Globals"`,
		"a variable without a name was skipped",
	}, res.Warnings)
}

func TestImportEnvironment_CollectionFileIsRefused(t *testing.T) {
	snapshot, err := os.ReadFile(filepath.Join("..", "..", "..", "..", "testdata", "snapshot", "all-protocols.json"))
	require.NoError(t, err)
	cases := map[string]string{
		"postman collection":          `{"info":{"name":"P","schema":"` + postman.SchemaV21 + `"},"item":[]}`,
		"collection without a schema": `{"info":{"name":"P"},"item":[{"name":"R"}],"name":"P","values":[]}`,
		"tetiva snapshot":             string(snapshot),
	}
	for name, content := range cases {
		t.Run(name, func(t *testing.T) {
			envUC := &stubEnvironmentUC{}

			_, err := importEnv(t, envUC, content)

			var re *domain.ReasonError
			require.ErrorAs(t, err, &re)
			assert.Equal(t, portability.ReasonCollectionFile, re.Reason)
			var ve *domain.ValidationError
			require.ErrorAs(t, err, &ve)
			assert.Equal(t, map[string]string{"content": "a collection, not an environment"}, ve.Fields)
			assert.Nil(t, envUC.createdEnv)
		})
	}
}

func TestIsEnvironment(t *testing.T) {
	snapshot, err := os.ReadFile(filepath.Join("..", "..", "..", "..", "testdata", "snapshot", "all-protocols.json"))
	require.NoError(t, err)
	cases := map[string]struct {
		content string
		want    bool
	}{
		"environment":              {`{"id":"1","name":"Dev","values":[{"key":"a","value":"1"}],"_postman_variable_scope":"environment"}`, true},
		"globals without a name":   {`{"name":"","values":[],"_postman_variable_scope":"globals"}`, true},
		"globals, no name at all":  {`{"values":[],"_postman_variable_scope":"globals"}`, true},
		"old export without scope": {` { "name" : "Dev" , "values" : [ ] } `, true},
		"postman collection":       {`{"info":{"name":"P","schema":"` + postman.SchemaV21 + `"},"item":[]}`, false},
		"collection with values":   {`{"info":{"name":"P"},"item":[],"name":"P","values":[]}`, false},
		"tetiva snapshot":          {string(snapshot), false},
		"values not an array":      {`{"name":"Dev","values":{}}`, false},
		"no values":                {`{"name":"Dev","_postman_variable_scope":"environment"}`, false},
		"unknown scope, no name":   {`{"values":[],"_postman_variable_scope":"collection"}`, false},
		"name not a string":        {`{"name":5,"values":[]}`, false},
		"top-level array":          {`[{"name":"Dev","values":[]}]`, false},
		"null":                     {`null`, false},
		"junk":                     {`not json`, false},
		"empty":                    {``, false},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, tc.want, postman.IsEnvironment([]byte(tc.content)))
		})
	}
}

func TestImportEnvironment_InputErrors(t *testing.T) {
	cases := map[string]struct {
		content string
		fields  map[string]string
	}{
		"invalid JSON":     {`nope`, map[string]string{"content": "invalid JSON"}},
		"truncated JSON":   {`{"name":"Dev","values":[`, map[string]string{"content": "invalid JSON"}},
		"not an object":    {`[{"name":"Dev"}]`, map[string]string{"content": "not a Postman environment"}},
		"values not array": {`{"name":"Dev","values":{}}`, map[string]string{"content": "not a Postman environment"}},
		"empty name":       {`{"name":"","values":[]}`, map[string]string{"name": "environment name is empty"}},
		"blank name":       {`{"name":"   ","values":[{"key":"k"}],"_postman_variable_scope":"environment"}`, map[string]string{"name": "environment name is empty"}},
		"no name":          {`{"values":[]}`, map[string]string{"name": "environment name is empty"}},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			envUC := &stubEnvironmentUC{}

			_, err := importEnv(t, envUC, tc.content)

			var ve *domain.ValidationError
			require.ErrorAs(t, err, &ve)
			assert.Equal(t, tc.fields, ve.Fields)
			var re *domain.ReasonError
			assert.False(t, errors.As(err, &re))
			assert.Nil(t, envUC.createdEnv)
		})
	}
}

func TestExportEnvironment(t *testing.T) {
	env := &entities.Environment{
		ID:   uuid.New(),
		Name: "Production",
	}
	vars := []*entities.Variable{
		{ID: uuid.New(), Key: "host", Value: "https://api.prod.com", Enabled: true, IsSecret: false},
		{ID: uuid.New(), Key: "api_key", Value: "key123", Enabled: true, IsSecret: true},
		{ID: uuid.New(), Key: "debug", Value: "false", Enabled: false, IsSecret: false},
	}

	data, err := postman.ExportEnvironment(env, vars)
	require.NoError(t, err)

	var pe postman.PostmanEnvironment
	require.NoError(t, json.Unmarshal(data, &pe))

	assert.Equal(t, "Production", pe.Name)
	require.Equal(t, 3, len(pe.Values))

	assert.Equal(t, "host", pe.Values[0].Key)
	assert.True(t, pe.Values[0].Enabled)
	assert.Empty(t, pe.Values[0].Type)

	assert.Equal(t, "api_key", pe.Values[1].Key)
	assert.Equal(t, "secret", pe.Values[1].Type)

	assert.Equal(t, "debug", pe.Values[2].Key)
	assert.False(t, pe.Values[2].Enabled)
}

func TestExportEnvironment_EmptyVars(t *testing.T) {
	env := &entities.Environment{ID: uuid.New(), Name: "Empty"}

	data, err := postman.ExportEnvironment(env, nil)
	require.NoError(t, err)

	var pe postman.PostmanEnvironment
	require.NoError(t, json.Unmarshal(data, &pe))

	assert.Equal(t, "Empty", pe.Name)
	assert.Equal(t, 0, len(pe.Values))
}

func TestExportEnvironment_Golden(t *testing.T) {
	id := uuid.MustParse("6f1c2d3e-4a5b-4c6d-8e7f-901234567890")
	vars := []*entities.Variable{
		{Key: "host", Value: "https://api.prod.com", Enabled: true},
		{Key: "api_key", Value: "key123", Enabled: true, IsSecret: true},
		{Key: "debug", Value: "false", Enabled: false},
	}

	data, err := postman.ExportEnvironment(&entities.Environment{ID: id, Name: "Production"}, vars)
	require.NoError(t, err)
	assert.Equal(t, `{
	"id": "6f1c2d3e-4a5b-4c6d-8e7f-901234567890",
	"name": "Production",
	"values": [
		{
			"key": "host",
			"value": "https://api.prod.com",
			"enabled": true
		},
		{
			"key": "api_key",
			"value": "key123",
			"type": "secret",
			"enabled": true
		},
		{
			"key": "debug",
			"value": "false",
			"enabled": false
		}
	],
	"_postman_variable_scope": "environment"
}`, string(data))

	empty, err := postman.ExportEnvironment(&entities.Environment{ID: id, Name: "Empty"}, nil)
	require.NoError(t, err)
	assert.Equal(t, `{
	"id": "6f1c2d3e-4a5b-4c6d-8e7f-901234567890",
	"name": "Empty",
	"values": [],
	"_postman_variable_scope": "environment"
}`, string(empty))
}
