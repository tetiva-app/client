package openapi

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"

	"github.com/tetiva-app/client/internal/adapters/portability"
	"github.com/tetiva-app/client/internal/domain"
	"github.com/tetiva-app/client/internal/domain/entities"
	"github.com/tetiva-app/client/internal/domain/usecase/collection"
	"github.com/tetiva-app/client/internal/domain/usecase/environment"
	"github.com/tetiva-app/client/internal/domain/usecase/example"
	"github.com/tetiva-app/client/internal/domain/usecase/request"
)

type Collections interface {
	Create(ctx context.Context, input collection.Create, opt collection.CreateOpt) (*entities.Collection, error)
}

type Requests interface {
	Create(ctx context.Context, input request.Create, opt request.CreateOpt) (*entities.Request, error)
}

type Examples interface {
	Create(ctx context.Context, in example.Create, opt example.CreateOpt) (*entities.ResponseExample, error)
}

type Environments interface {
	Create(ctx context.Context, input environment.Create, opt environment.CreateOpt) (*entities.Environment, error)
	AddVariable(ctx context.Context, input environment.AddVariable, opt environment.AddVariableOpt) (*entities.Variable, error)
	List(ctx context.Context, opt environment.ListOpt) ([]*entities.Environment, error)
	SetActive(ctx context.Context, opt environment.SetActiveOpt) error
}

type Importer struct {
	collections  Collections
	requests     Requests
	examples     Examples
	environments Environments
}

func NewImporter(collections Collections, requests Requests, examples Examples, environments Environments) *Importer {
	return &Importer{collections: collections, requests: requests, examples: examples, environments: environments}
}

func (i *Importer) Detect(data []byte) bool {
	return detect(data)
}

func (i *Importer) Preview(data []byte, _ string) (*portability.ImportPreview, error) {
	s, err := load(data)
	if err != nil {
		return nil, err
	}
	p := newPlan(s)
	return &portability.ImportPreview{
		Format:           portability.FormatOpenAPI,
		Title:            p.root.Name,
		Folders:          len(p.folders),
		Requests:         p.requestCount,
		Examples:         p.exampleCount,
		EnvironmentNames: []string{p.envName},
		Hosts:            portability.Hosts(p.urls, p.hostVars()),
		Scripts:          []portability.ScriptPreview{},
		Warnings:         nonNil(p.warnings),
	}, nil
}

func (i *Importer) Import(ctx context.Context, data []byte, opt portability.ImportOpt) (*portability.ImportResult, error) {
	const funcName = "openapi.Importer.Import"

	s, err := load(data)
	if err != nil {
		return nil, err
	}
	p := newPlan(s)

	in := p.root
	in.ParentID = opt.ParentID
	root, err := i.collections.Create(ctx, in, collection.CreateOpt{UserID: opt.UserID, WorkspaceID: opt.WorkspaceID})
	if err != nil {
		return nil, fmt.Errorf("%s: create collection %q: %w", funcName, in.Name, err)
	}
	w := &writer{Importer: i, opt: opt, res: &portability.ImportResult{CollectionID: root.ID, Warnings: nonNil(p.warnings)}}
	for _, f := range p.folders {
		if err := w.folder(ctx, root.ID, f); err != nil {
			return nil, fmt.Errorf("%s: %w", funcName, err)
		}
	}
	for _, rp := range p.requests {
		if err := w.request(ctx, root.ID, rp); err != nil {
			return nil, fmt.Errorf("%s: %w", funcName, err)
		}
	}
	if err := w.environment(ctx, p.envName, p.vars); err != nil {
		return nil, fmt.Errorf("%s: %w", funcName, err)
	}
	return w.res, nil
}

type writer struct {
	*Importer
	opt portability.ImportOpt
	res *portability.ImportResult
}

func (w *writer) folder(ctx context.Context, parentID uuid.UUID, f *folderPlan) error {
	in := f.in
	in.ParentID = &parentID
	c, err := w.collections.Create(ctx, in, collection.CreateOpt{UserID: w.opt.UserID, WorkspaceID: w.opt.WorkspaceID})
	if err != nil {
		return fmt.Errorf("create folder %q: %w", in.Name, err)
	}
	w.res.Folders++
	for _, rp := range f.requests {
		if err := w.request(ctx, c.ID, rp); err != nil {
			return err
		}
	}
	return nil
}

func (w *writer) request(ctx context.Context, collectionID uuid.UUID, rp *requestPlan) error {
	in := rp.in
	in.CollectionID = collectionID
	r, err := w.requests.Create(ctx, in, request.CreateOpt{UserID: w.opt.UserID})
	if err != nil {
		return fmt.Errorf("create request %q: %w", rp.label, err)
	}
	w.res.Requests++
	for _, ex := range rp.examples {
		ex.RequestID = r.ID
		if _, err := w.examples.Create(ctx, ex, example.CreateOpt{UserID: w.opt.UserID}); err != nil {
			var ve *domain.ValidationError
			if !errors.As(err, &ve) {
				return fmt.Errorf("create example %q of %s: %w", ex.Name, rp.label, err)
			}
			w.res.Warnings = append(w.res.Warnings, portability.ExampleRejected(operation(rp.label), ex.Name, ve))
			continue
		}
		w.res.Examples++
	}
	return nil
}

func (w *writer) environment(ctx context.Context, name string, vars []environment.AddVariable) error {
	existing, err := w.environments.List(ctx, environment.ListOpt{WorkspaceID: w.opt.WorkspaceID})
	if err != nil {
		return fmt.Errorf("list environments: %w", err)
	}
	taken := make(map[string]bool, len(existing))
	for _, e := range existing {
		taken[e.Name] = true
	}
	env, err := w.environments.Create(ctx, environment.Create{Name: portability.FreeName(name, taken)},
		environment.CreateOpt{UserID: w.opt.UserID, WorkspaceID: w.opt.WorkspaceID})
	if err != nil {
		return fmt.Errorf("create environment %q: %w", name, err)
	}
	w.res.EnvironmentNames = []string{env.Name}
	for _, v := range vars {
		v.EnvironmentID = env.ID
		if _, err := w.environments.AddVariable(ctx, v, environment.AddVariableOpt{UserID: w.opt.UserID}); err != nil {
			return fmt.Errorf("add variable %q: %w", v.Key, err)
		}
	}
	if err := w.environments.SetActive(ctx, environment.SetActiveOpt{WorkspaceID: w.opt.WorkspaceID, EnvironmentID: env.ID}); err != nil {
		return fmt.Errorf("activate environment %q: %w", env.Name, err)
	}
	w.res.ActiveEnvironment = env.Name
	return nil
}

func nonNil[T any](s []T) []T {
	if s == nil {
		return []T{}
	}
	return s
}
