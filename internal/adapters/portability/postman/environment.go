package postman

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"

	"github.com/tetiva-app/client/internal/domain/entities"
	"github.com/tetiva-app/client/internal/domain/usecase/environment"
)

type ImportEnvOpts struct {
	WorkspaceID uuid.UUID
	UserID      string
}

type ImportEnvResult struct {
	EnvironmentName  string
	VariablesCreated int
}

func ImportEnvironment(
	ctx context.Context,
	data []byte,
	opts ImportEnvOpts,
	envUC environment.Usecase,
) (*ImportEnvResult, error) {
	const funcName = "postman.ImportEnvironment"

	var pe PostmanEnvironment
	if err := json.Unmarshal(data, &pe); err != nil {
		return nil, fmt.Errorf("%s: invalid JSON: %w", funcName, err)
	}

	if pe.Name == "" {
		return nil, fmt.Errorf("%s: environment name is empty", funcName)
	}

	env, err := envUC.Create(ctx, environment.Create{Name: pe.Name}, environment.CreateOpt{
		UserID:      opts.UserID,
		WorkspaceID: opts.WorkspaceID,
	})
	if err != nil {
		return nil, fmt.Errorf("%s: failed to create environment: %w", funcName, err)
	}

	result := &ImportEnvResult{
		EnvironmentName: pe.Name,
	}

	for _, v := range pe.Values {
		_, err := envUC.AddVariable(ctx, environment.AddVariable{
			EnvironmentID: env.ID,
			Key:           v.Key,
			Value:         v.Value,
			IsSecret:      v.Type == "secret",
		}, environment.AddVariableOpt{
			UserID: opts.UserID,
		})
		if err != nil {
			return nil, fmt.Errorf("%s: failed to add variable %q: %w", funcName, v.Key, err)
		}
		result.VariablesCreated++
	}

	return result, nil
}

func ExportEnvironment(env *entities.Environment, vars []*entities.Variable) ([]byte, error) {
	const funcName = "postman.ExportEnvironment"

	pe := PostmanEnvironment{
		ID:    env.ID.String(),
		Name:  env.Name,
		Scope: "environment",
	}

	for _, v := range vars {
		pv := PostmanEnvValue{
			Key:     v.Key,
			Value:   v.Value,
			Enabled: v.Enabled,
		}
		if v.IsSecret {
			pv.Type = "secret"
		}
		pe.Values = append(pe.Values, pv)
	}

	if pe.Values == nil {
		pe.Values = []PostmanEnvValue{}
	}

	data, err := json.MarshalIndent(pe, "", "\t")
	if err != nil {
		return nil, fmt.Errorf("%s: failed to marshal: %w", funcName, err)
	}

	return data, nil
}
