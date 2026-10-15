package hoppscotch

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"strings"

	"github.com/google/uuid"

	"github.com/tetiva-app/client/internal/adapters/portability"
	"github.com/tetiva-app/client/internal/domain"
	"github.com/tetiva-app/client/internal/domain/usecase/environment"
)

type hoppEnvironment struct {
	Name      string `json:"name"`
	Variables []struct {
		Key          string  `json:"key"`
		Value        *string `json:"value"`
		InitialValue string  `json:"initialValue"`
		CurrentValue string  `json:"currentValue"`
		Secret       bool    `json:"secret"`
	} `json:"variables"`
}

type EnvironmentResult struct {
	ID        uuid.UUID
	Names     []string
	Variables int
	Warnings  []portability.Warning
}

func IsEnvironment(data []byte) bool {
	type variable struct {
		Key *string `json:"key"`
	}
	type envProbe struct {
		Name      *string         `json:"name"`
		Variables *[]variable     `json:"variables"`
		Folders   json.RawMessage `json:"folders"`
		Values    json.RawMessage `json:"values"`
	}
	is := func(p envProbe) bool {
		return p.Name != nil && p.Variables != nil && p.Folders == nil && p.Values == nil &&
			!slices.ContainsFunc(*p.Variables, func(v variable) bool { return v.Key == nil })
	}
	data = bytes.TrimSpace(data)
	if bytes.HasPrefix(data, []byte("[")) {
		var list []envProbe
		return json.Unmarshal(data, &list) == nil && len(list) > 0 && !slices.ContainsFunc(list, func(p envProbe) bool { return !is(p) })
	}
	var p envProbe
	return json.Unmarshal(data, &p) == nil && is(p)
}

func ImportEnvironments(ctx context.Context, data []byte, envs portability.EnvironmentCreator, opt portability.ImportOpt) (*EnvironmentResult, error) {
	const funcName = "hoppscotch.ImportEnvironments"

	var list []hoppEnvironment
	if err := json.Unmarshal(asList(data), &list); err != nil {
		return nil, fmt.Errorf("%s: %w", funcName, &domain.ValidationError{Fields: map[string]string{
			"content": "not a valid Hoppscotch environment: " + err.Error(),
		}})
	}

	w := portability.Writer{Environments: envs}
	res := &EnvironmentResult{}
	var secrets []string
	for _, e := range list {
		env := &portability.Environment{Name: nameOr(e.Name)}
		for _, v := range e.Variables {
			if strings.TrimSpace(v.Key) == "" {
				continue
			}
			value := v.CurrentValue
			switch {
			case v.Value != nil:
				value = *v.Value
			case value == "":
				value = v.InitialValue
			}
			if v.Secret && value == "" && !slices.Contains(secrets, v.Key) {
				secrets = append(secrets, v.Key)
			}
			env.Vars = append(env.Vars, environment.AddVariable{Key: v.Key, Value: vars(value), IsSecret: v.Secret})
		}
		created, err := w.CreateEnvironment(ctx, env, opt)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", funcName, err)
		}
		if res.ID == uuid.Nil {
			res.ID = created.ID
		}
		res.Names = append(res.Names, created.Name)
		res.Variables += len(env.Vars)
	}
	if len(secrets) > 0 {
		res.Warnings = append(res.Warnings, secretsWarning(secrets))
	}
	return res, nil
}

func secretsWarning(keys []string) portability.Warning {
	return portability.Warn(portability.WarnHoppscotchSecretsEmpty, portability.Subject{}, "keys", strings.Join(keys, ", "))
}
