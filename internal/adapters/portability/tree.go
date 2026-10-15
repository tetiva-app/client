package portability

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"slices"
	"strings"

	"github.com/google/uuid"

	"github.com/tetiva-app/client/internal/domain"
	"github.com/tetiva-app/client/internal/domain/entities"
	"github.com/tetiva-app/client/internal/domain/usecase/auth"
	"github.com/tetiva-app/client/internal/domain/usecase/collection"
	"github.com/tetiva-app/client/internal/domain/usecase/environment"
	"github.com/tetiva-app/client/internal/domain/usecase/example"
	"github.com/tetiva-app/client/internal/domain/usecase/request"
)

type Tree struct {
	Root         Folder
	Environments []Environment
	Warnings     []string
}

type Folder struct {
	In       collection.Create
	Folders  []Folder
	Requests []Request
}

type Request struct {
	In       request.Create
	Examples []example.Create
}

type Environment struct {
	Name string
	Vars []environment.AddVariable
}

type (
	CollectionCreator interface {
		Create(ctx context.Context, in collection.Create, opt collection.CreateOpt) (*entities.Collection, error)
	}
	RequestCreator interface {
		Create(ctx context.Context, in request.Create, opt request.CreateOpt) (*entities.Request, error)
	}
	ExampleCreator interface {
		Create(ctx context.Context, in example.Create, opt example.CreateOpt) (*entities.ResponseExample, error)
	}
	EnvironmentCreator interface {
		Create(ctx context.Context, in environment.Create, opt environment.CreateOpt) (*entities.Environment, error)
		AddVariable(ctx context.Context, in environment.AddVariable, opt environment.AddVariableOpt) (*entities.Variable, error)
		List(ctx context.Context, opt environment.ListOpt) ([]*entities.Environment, error)
	}
)

type Writer struct {
	Collections  CollectionCreator
	Requests     RequestCreator
	Examples     ExampleCreator
	Environments EnvironmentCreator
}

const maxSecretLabels = 5

func (t *Tree) Preview(format string) *ImportPreview {
	p := &ImportPreview{Format: format, Title: t.Root.In.Name, Scripts: []ScriptPreview{}}
	var urls []string
	var walk func(path string, f *Folder)
	walk = func(path string, f *Folder) {
		p.Scripts = appendScripts(p.Scripts, path, f.In.PreScript, f.In.PostScript)
		urls = appendAuthURLs(urls, f.In.AuthType, f.In.AuthData)
		for i := range f.Folders {
			p.Folders++
			walk(path+" / "+f.Folders[i].In.Name, &f.Folders[i])
		}
		for _, r := range f.Requests {
			p.Requests++
			p.Examples += len(r.Examples)
			urls = appendAuthURLs(append(urls, r.In.URL), r.In.AuthType, r.In.AuthData)
			p.Scripts = appendScripts(p.Scripts, path+" / "+r.In.Name, r.In.PreScript, r.In.PostScript)
		}
	}
	walk(t.Root.In.Name, &t.Root)

	var envs []map[string]string
	for _, env := range t.Environments {
		if len(env.Vars) == 0 {
			continue
		}
		p.EnvironmentNames = append(p.EnvironmentNames, env.Name)
		vars := map[string]string{}
		for _, v := range env.Vars {
			if !v.Disabled && !v.IsSecret {
				vars[v.Key] = v.Value
			}
		}
		envs = append(envs, vars)
	}
	p.Hosts = Hosts(urls, envs...)
	p.Warnings = t.warnings()
	return p
}

func appendScripts(out []ScriptPreview, path, pre, post string) []ScriptPreview {
	if strings.TrimSpace(pre) != "" {
		out = append(out, ScriptPreview{Path: path, Phase: "pre", Text: pre})
	}
	if strings.TrimSpace(post) != "" {
		out = append(out, ScriptPreview{Path: path, Phase: "post", Text: post})
	}
	return out
}

func appendAuthURLs(urls []string, authType entities.AuthType, data string) []string {
	if authType != entities.AuthTypeOAuth2 {
		return urls
	}
	fields, err := auth.ParseFields(data)
	if err != nil {
		return urls
	}
	for _, key := range []string{"tokenUrl", "authUrl"} {
		if v := fields.Str(key); v != "" {
			urls = append(urls, v)
		}
	}
	return urls
}

func (t *Tree) warnings() []string {
	out := slices.Clone(t.Warnings)
	var suspects []string
	var walk func(f *Folder)
	walk = func(f *Folder) {
		for i := range f.Folders {
			walk(&f.Folders[i])
		}
		for _, r := range f.Requests {
			for _, ex := range r.Examples {
				if found := example.SuspectedSecrets(ex.Headers, ex.Body); len(found) > 0 {
					suspects = append(suspects, fmt.Sprintf("%q / %q (%s)", r.In.Name, ex.Name, strings.Join(found, ", ")))
				}
			}
		}
	}
	walk(&t.Root)
	if len(suspects) > maxSecretLabels {
		suspects = append(suspects[:maxSecretLabels], fmt.Sprintf("and %d more", len(suspects)-maxSecretLabels))
	}
	if len(suspects) > 0 {
		out = append(out, "examples may contain secrets, check them before sharing: "+strings.Join(suspects, "; "))
	}
	if out == nil {
		return []string{}
	}
	return out
}

func (w Writer) Write(ctx context.Context, t *Tree, opt ImportOpt) (*ImportResult, error) {
	const funcName = "portability.Writer.Write"

	res := &ImportResult{Warnings: t.warnings()}
	id, err := w.folder(ctx, &t.Root, opt.ParentID, opt, res)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", funcName, err)
	}
	res.CollectionID = id
	for i := range t.Environments {
		if len(t.Environments[i].Vars) == 0 {
			continue
		}
		created, err := w.CreateEnvironment(ctx, &t.Environments[i], opt)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", funcName, err)
		}
		res.EnvironmentNames = append(res.EnvironmentNames, created.Name)
	}
	return res, nil
}

func (w Writer) folder(ctx context.Context, f *Folder, parentID *uuid.UUID, opt ImportOpt, res *ImportResult) (uuid.UUID, error) {
	in := f.In
	in.ParentID = parentID
	if !opt.IncludeScripts {
		in.PreScript, in.PostScript = "", ""
	}
	c, err := w.Collections.Create(ctx, in, collection.CreateOpt{UserID: opt.UserID, WorkspaceID: opt.WorkspaceID})
	if err != nil {
		return uuid.Nil, fmt.Errorf("create folder %q: %w", in.Name, err)
	}
	for i := range f.Folders {
		if _, err := w.folder(ctx, &f.Folders[i], &c.ID, opt, res); err != nil {
			return uuid.Nil, err
		}
		res.Folders++
	}
	for _, r := range f.Requests {
		if err := w.request(ctx, c.ID, r, opt, res); err != nil {
			return uuid.Nil, err
		}
	}
	return c.ID, nil
}

func (w Writer) request(ctx context.Context, collectionID uuid.UUID, r Request, opt ImportOpt, res *ImportResult) error {
	in := r.In
	in.CollectionID = collectionID
	if !opt.IncludeScripts {
		in.PreScript, in.PostScript = "", ""
	}
	created, err := w.Requests.Create(ctx, in, request.CreateOpt{UserID: opt.UserID})
	if err != nil {
		return fmt.Errorf("create request %q: %w", in.Name, err)
	}
	res.Requests++
	for _, ex := range r.Examples {
		ex.RequestID = created.ID
		if _, err := w.Examples.Create(ctx, ex, example.CreateOpt{UserID: opt.UserID}); err != nil {
			var ve *domain.ValidationError
			if !errors.As(err, &ve) {
				return fmt.Errorf("create example %q of %q: %w", ex.Name, in.Name, err)
			}
			res.Warnings = append(res.Warnings, fmt.Sprintf("request %q: example %q skipped: %s", in.Name, ex.Name, describe(ve)))
			continue
		}
		res.Examples++
	}
	return nil
}

func (w Writer) CreateEnvironment(ctx context.Context, env *Environment, opt ImportOpt) (*entities.Environment, error) {
	existing, err := w.Environments.List(ctx, environment.ListOpt{WorkspaceID: opt.WorkspaceID})
	if err != nil {
		return nil, fmt.Errorf("list environments: %w", err)
	}
	taken := make(map[string]bool, len(existing))
	for _, e := range existing {
		taken[e.Name] = true
	}
	created, err := w.Environments.Create(ctx, environment.Create{Name: FreeName(env.Name, taken)},
		environment.CreateOpt{UserID: opt.UserID, WorkspaceID: opt.WorkspaceID})
	if err != nil {
		return nil, fmt.Errorf("create environment %q: %w", env.Name, err)
	}
	for _, v := range env.Vars {
		v.EnvironmentID = created.ID
		if _, err := w.Environments.AddVariable(ctx, v, environment.AddVariableOpt{UserID: opt.UserID}); err != nil {
			return nil, fmt.Errorf("add variable %q: %w", v.Key, err)
		}
	}
	return created, nil
}

func describe(ve *domain.ValidationError) string {
	msgs := make([]string, 0, len(ve.Fields))
	for _, msg := range ve.Fields {
		msgs = append(msgs, msg)
	}
	slices.Sort(msgs)
	return strings.Join(msgs, "; ")
}

func FileTitle(fileName, fallback string) string {
	if title := strings.TrimSpace(strings.TrimSuffix(fileName, filepath.Ext(fileName))); title != "" {
		return title
	}
	return fallback
}
