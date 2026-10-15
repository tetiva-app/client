package insomnia

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/tetiva-app/client/internal/adapters/portability"
	"github.com/tetiva-app/client/internal/domain"
	"github.com/tetiva-app/client/internal/domain/entities"
	"github.com/tetiva-app/client/internal/domain/usecase/collection"
	"github.com/tetiva-app/client/internal/domain/usecase/environment"
	"github.com/tetiva-app/client/internal/domain/usecase/request"
)

type Collections interface {
	Create(ctx context.Context, input collection.Create, opt collection.CreateOpt) (*entities.Collection, error)
}

type Requests interface {
	Create(ctx context.Context, input request.Create, opt request.CreateOpt) (*entities.Request, error)
}

type Environments interface {
	Create(ctx context.Context, input environment.Create, opt environment.CreateOpt) (*entities.Environment, error)
	AddVariable(ctx context.Context, input environment.AddVariable, opt environment.AddVariableOpt) (*entities.Variable, error)
	List(ctx context.Context, opt environment.ListOpt) ([]*entities.Environment, error)
}

type Importer struct {
	collections  Collections
	requests     Requests
	environments Environments
}

func NewImporter(collections Collections, requests Requests, environments Environments) *Importer {
	return &Importer{collections: collections, requests: requests, environments: environments}
}

func (i *Importer) Detect(data []byte) bool {
	return isV5(data) || isV4(data)
}

func (i *Importer) Preview(data []byte, _ string) (*portability.ImportPreview, error) {
	p, err := load(data, true)
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(p.envs))
	for _, env := range p.envs {
		names = append(names, env.name)
	}
	return &portability.ImportPreview{
		Format:           portability.FormatInsomnia,
		Title:            p.roots[0].in.Name,
		Folders:          p.folders + len(p.roots) - 1,
		Requests:         p.requests,
		EnvironmentNames: names,
		Hosts:            nonNil(p.hosts()),
		Scripts:          nonNil(p.scripts),
		Warnings:         nonNil(p.warnings),
	}, nil
}

func (i *Importer) Import(ctx context.Context, data []byte, opt portability.ImportOpt) (*portability.ImportResult, error) {
	const funcName = "insomnia.Importer.Import"

	p, err := load(data, opt.IncludeScripts)
	if err != nil {
		return nil, err
	}
	w := &writer{Importer: i, opt: opt}
	res := &portability.ImportResult{
		Folders:  p.folders + len(p.roots) - 1,
		Requests: p.requests,
		Warnings: nonNil(p.warnings),
	}
	for n, root := range p.roots {
		root.in.ParentID = opt.ParentID
		id, err := w.folder(ctx, root)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", funcName, err)
		}
		if n == 0 {
			res.CollectionID = id
		}
	}
	res.EnvironmentNames, err = w.addEnvironments(ctx, p.envs)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", funcName, err)
	}
	return res, nil
}

func load(data []byte, includeScripts bool) (*plan, error) {
	ex, err := parse(data)
	if err != nil {
		return nil, &domain.ValidationError{Fields: map[string]string{"content": "not a readable Insomnia export: " + err.Error()}}
	}
	if len(ex.workspaces) == 0 {
		return nil, &domain.ValidationError{Fields: map[string]string{"content": "the Insomnia export has no collection"}}
	}
	return newPlan(ex, includeScripts), nil
}

const maxNestedExpansion = 1 << 20

func (p *plan) hosts() []string {
	budget := maxNestedExpansion
	envs := make([]map[string]string, 0, len(p.envs))
	for _, env := range p.envs {
		vars := map[string]string{}
		for _, v := range env.vars {
			if !v.IsSecret {
				vars[v.Key] = v.Value
			}
		}
		resolveNested(vars, &budget)
		envs = append(envs, vars)
	}
	return portability.Hosts(p.urls, envs...)
}

func resolveNested(vars map[string]string, budget *int) {
	for range 4 {
		for k, v := range vars {
			vars[k] = portability.ExpandVars(v, vars, budget)
		}
	}
}

type writer struct {
	*Importer
	opt portability.ImportOpt
}

func (w *writer) folder(ctx context.Context, f *folderPlan) (uuid.UUID, error) {
	c, err := w.collections.Create(ctx, f.in, collection.CreateOpt{UserID: w.opt.UserID, WorkspaceID: w.opt.WorkspaceID})
	if err != nil {
		return uuid.Nil, fmt.Errorf("create folder %q: %w", f.in.Name, err)
	}
	for _, sub := range f.folders {
		sub.in.ParentID = &c.ID
		if _, err := w.folder(ctx, sub); err != nil {
			return uuid.Nil, err
		}
	}
	for _, in := range f.requests {
		in.CollectionID = c.ID
		if _, err := w.requests.Create(ctx, in, request.CreateOpt{UserID: w.opt.UserID}); err != nil {
			return uuid.Nil, fmt.Errorf("create request %q: %w", in.Name, err)
		}
	}
	return c.ID, nil
}

func (w *writer) addEnvironments(ctx context.Context, envs []envPlan) ([]string, error) {
	if len(envs) == 0 {
		return nil, nil
	}
	existing, err := w.environments.List(ctx, environment.ListOpt{WorkspaceID: w.opt.WorkspaceID})
	if err != nil {
		return nil, fmt.Errorf("list environments: %w", err)
	}
	taken := make(map[string]bool, len(existing))
	for _, e := range existing {
		taken[e.Name] = true
	}
	names := make([]string, 0, len(envs))
	for _, ep := range envs {
		name := portability.FreeName(ep.name, taken)
		taken[name] = true
		env, err := w.environments.Create(ctx, environment.Create{Name: name},
			environment.CreateOpt{UserID: w.opt.UserID, WorkspaceID: w.opt.WorkspaceID})
		if err != nil {
			return nil, fmt.Errorf("create environment %q: %w", name, err)
		}
		for _, v := range ep.vars {
			v.EnvironmentID = env.ID
			if _, err := w.environments.AddVariable(ctx, v, environment.AddVariableOpt{UserID: w.opt.UserID}); err != nil {
				return nil, fmt.Errorf("add variable %q: %w", v.Key, err)
			}
		}
		names = append(names, env.Name)
	}
	return names, nil
}

func nonNil[T any](s []T) []T {
	if s == nil {
		return []T{}
	}
	return s
}
