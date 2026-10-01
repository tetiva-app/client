package postman

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/google/uuid"

	"github.com/tetiva-app/client/internal/adapters/portability"
	"github.com/tetiva-app/client/internal/adapters/publication/snapshotjson"
	"github.com/tetiva-app/client/internal/domain"
	"github.com/tetiva-app/client/internal/domain/entities"
	"github.com/tetiva-app/client/internal/domain/usecase/environment"
)

type ImportEnvOpts struct {
	WorkspaceID uuid.UUID
	UserID      string
}

type ImportEnvResult struct {
	EnvironmentID    uuid.UUID
	EnvironmentName  string
	VariablesCreated int
	Warnings         []string
}

type EnvironmentCreator interface {
	Create(ctx context.Context, input environment.Create, opt environment.CreateOpt) (*entities.Environment, error)
	AddVariable(ctx context.Context, input environment.AddVariable, opt environment.AddVariableOpt) (*entities.Variable, error)
	List(ctx context.Context, opt environment.ListOpt) ([]*entities.Environment, error)
}

const (
	environmentScope = "environment"
	globalsScope     = "globals"
)

// Callers run it in a transaction: a failing variable leaves a half-built environment.
func ImportEnvironment(
	ctx context.Context,
	data []byte,
	opts ImportEnvOpts,
	envUC EnvironmentCreator,
) (*ImportEnvResult, error) {
	const funcName = "postman.ImportEnvironment"
	const globalsName = "Globals"

	if !json.Valid(data) {
		return nil, fmt.Errorf("%s: %w", funcName, &domain.ValidationError{Fields: map[string]string{"content": "invalid JSON"}})
	}
	if isCollectionFile(data) {
		return nil, fmt.Errorf("%s: %w", funcName, &domain.ReasonError{
			Reason: portability.ReasonCollectionFile,
			Err:    &domain.ValidationError{Fields: map[string]string{"content": "a collection, not an environment"}},
		})
	}
	var pe postmanEnvironmentIn
	if err := json.Unmarshal(data, &pe); err != nil {
		return nil, fmt.Errorf("%s: %w", funcName, &domain.ValidationError{Fields: map[string]string{"content": "not a Postman environment"}})
	}

	name := strings.TrimSpace(pe.Name)
	globals := pe.Scope == globalsScope
	if name == "" {
		if !globals {
			return nil, fmt.Errorf("%s: %w", funcName, &domain.ValidationError{Fields: map[string]string{"name": "environment name is empty"}})
		}
		name = globalsName
	}

	vars := make([]environment.AddVariable, 0, len(pe.Values))
	unnamed := 0
	for _, v := range pe.Values {
		if strings.TrimSpace(v.Key) == "" {
			unnamed++
			continue
		}
		vars = append(vars, environment.AddVariable{
			Key:      v.Key,
			Value:    string(v.Value),
			IsSecret: v.Type == "secret",
			Disabled: v.Enabled != nil && !*v.Enabled,
		})
	}

	env, err := createEnvironment(ctx, envUC, name, vars, opts.WorkspaceID, opts.UserID)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", funcName, err)
	}

	result := &ImportEnvResult{EnvironmentID: env.ID, EnvironmentName: env.Name, VariablesCreated: len(vars)}
	if globals {
		result.Warnings = append(result.Warnings,
			fmt.Sprintf("Tetiva has no global variables: they were imported as the environment %q", env.Name))
	}
	if unnamed > 0 {
		result.Warnings = append(result.Warnings, unnamedVariablesWarning(unnamed))
	}
	return result, nil
}

func createEnvironment(
	ctx context.Context,
	envUC EnvironmentCreator,
	name string,
	vars []environment.AddVariable,
	workspaceID uuid.UUID,
	userID string,
) (*entities.Environment, error) {
	const funcName = "postman.createEnvironment"

	existing, err := envUC.List(ctx, environment.ListOpt{WorkspaceID: workspaceID})
	if err != nil {
		return nil, fmt.Errorf("%s: %w", funcName, err)
	}
	taken := make(map[string]bool, len(existing))
	for _, e := range existing {
		taken[e.Name] = true
	}

	env, err := envUC.Create(ctx, environment.Create{Name: portability.FreeName(name, taken)},
		environment.CreateOpt{UserID: userID, WorkspaceID: workspaceID})
	if err != nil {
		return nil, fmt.Errorf("%s: create environment %q: %w", funcName, name, err)
	}
	for _, v := range vars {
		v.EnvironmentID = env.ID
		if _, err := envUC.AddVariable(ctx, v, environment.AddVariableOpt{UserID: userID}); err != nil {
			return nil, fmt.Errorf("%s: add variable %q: %w", funcName, v.Key, err)
		}
	}
	return env, nil
}

func unnamedVariablesWarning(n int) string {
	if n == 1 {
		return "a variable without a name was skipped"
	}
	return fmt.Sprintf("%d variables without a name were skipped", n)
}

func IsEnvironment(data []byte) bool {
	var probe map[string]json.RawMessage
	if err := json.Unmarshal(data, &probe); err != nil {
		return false
	}
	if _, ok := probe["info"]; ok || !bytes.HasPrefix(probe["values"], []byte("[")) {
		return false
	}
	var scope string
	if json.Unmarshal(probe["_postman_variable_scope"], &scope) == nil && (scope == environmentScope || scope == globalsScope) {
		return true
	}
	return bytes.HasPrefix(probe["name"], []byte(`"`))
}

func isCollectionFile(data []byte) bool {
	if snapshotjson.Sniff(data) {
		return true
	}
	var probe struct {
		Info json.RawMessage `json:"info"`
		Item json.RawMessage `json:"item"`
	}
	if err := json.Unmarshal(data, &probe); err != nil {
		return false
	}
	return bytes.HasPrefix(probe.Info, []byte("{")) && bytes.HasPrefix(probe.Item, []byte("["))
}

func ExportEnvironment(env *entities.Environment, vars []*entities.Variable) ([]byte, error) {
	const funcName = "postman.ExportEnvironment"

	pe := PostmanEnvironment{
		ID:    env.ID.String(),
		Name:  env.Name,
		Scope: environmentScope,
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
