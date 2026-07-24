package postman_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tetiva-app/client/internal/adapters/portability/postman"
	"github.com/tetiva-app/client/internal/domain/entities"
	"github.com/tetiva-app/client/internal/domain/usecase/environment"
)

type stubEnvironmentUC struct {
	createdEnv  *environment.Create
	createdVars []environment.AddVariable
	env         *entities.Environment
}

func (s *stubEnvironmentUC) Create(_ context.Context, input environment.Create, _ environment.CreateOpt) (*entities.Environment, error) {
	s.createdEnv = &input
	s.env = &entities.Environment{ID: uuid.New(), Name: input.Name}
	return s.env, nil
}

func (s *stubEnvironmentUC) AddVariable(_ context.Context, input environment.AddVariable, _ environment.AddVariableOpt) (*entities.Variable, error) {
	s.createdVars = append(s.createdVars, input)
	return &entities.Variable{ID: uuid.New(), Key: input.Key, Value: input.Value}, nil
}

func (s *stubEnvironmentUC) Duplicate(context.Context, environment.DuplicateOpt) (*entities.Environment, error) {
	return nil, nil
}
func (s *stubEnvironmentUC) GetByID(context.Context, uuid.UUID) (*entities.Environment, error) {
	return nil, nil
}
func (s *stubEnvironmentUC) List(context.Context, environment.ListOpt) ([]*entities.Environment, error) {
	return nil, nil
}
func (s *stubEnvironmentUC) Edit(context.Context, environment.Edit, environment.EditOpt) (*entities.Environment, error) {
	return nil, nil
}
func (s *stubEnvironmentUC) Delete(context.Context, environment.DeleteOpt) error { return nil }
func (s *stubEnvironmentUC) SetActive(context.Context, environment.SetActiveOpt) error {
	return nil
}
func (s *stubEnvironmentUC) EditVariable(context.Context, environment.EditVariable, environment.EditVariableOpt) (*entities.Variable, error) {
	return nil, nil
}
func (s *stubEnvironmentUC) DeleteVariable(context.Context, environment.DeleteVariableOpt) error {
	return nil
}
func (s *stubEnvironmentUC) ListVariables(context.Context, uuid.UUID) ([]*entities.Variable, error) {
	return nil, nil
}
func (s *stubEnvironmentUC) ResolveVariables(context.Context, uuid.UUID) (map[string]string, error) {
	return nil, nil
}
func (s *stubEnvironmentUC) PersistVariableChanges(context.Context, uuid.UUID, string, map[string]string) error {
	return nil
}

func TestImportEnvironment(t *testing.T) {
	data := postman.PostmanEnvironment{
		Name: "Dev",
		Values: []postman.PostmanEnvValue{
			{Key: "host", Value: "https://api.dev.example.com", Enabled: true},
			{Key: "token", Value: "secret123", Enabled: true, Type: "secret"},
			{Key: "disabled_var", Value: "unused", Enabled: false},
		},
	}

	raw, err := json.Marshal(data)
	require.NoError(t, err)

	envUC := &stubEnvironmentUC{}

	result, err := postman.ImportEnvironment(context.Background(), raw, postman.ImportEnvOpts{
		WorkspaceID: uuid.New(),
		UserID:      "local_user",
	}, envUC)

	require.NoError(t, err)
	assert.Equal(t, "Dev", result.EnvironmentName)
	assert.Equal(t, 3, result.VariablesCreated)

	require.NotNil(t, envUC.createdEnv)
	assert.Equal(t, "Dev", envUC.createdEnv.Name)

	require.Equal(t, 3, len(envUC.createdVars))
	assert.Equal(t, "host", envUC.createdVars[0].Key)
	assert.False(t, envUC.createdVars[0].IsSecret)
	assert.Equal(t, "token", envUC.createdVars[1].Key)
	assert.True(t, envUC.createdVars[1].IsSecret)
	assert.Equal(t, "disabled_var", envUC.createdVars[2].Key)
}

func TestImportEnvironment_InvalidJSON(t *testing.T) {
	_, err := postman.ImportEnvironment(context.Background(), []byte("nope"), postman.ImportEnvOpts{
		WorkspaceID: uuid.New(),
		UserID:      "local_user",
	}, &stubEnvironmentUC{})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid JSON")
}

func TestImportEnvironment_EmptyName(t *testing.T) {
	raw, _ := json.Marshal(postman.PostmanEnvironment{Name: ""})

	_, err := postman.ImportEnvironment(context.Background(), raw, postman.ImportEnvOpts{
		WorkspaceID: uuid.New(),
		UserID:      "local_user",
	}, &stubEnvironmentUC{})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "empty")
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
