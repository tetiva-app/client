// Package snapshot imports a published Tetiva collection (CollectionSnapshot v1) from a link or a file.
package snapshot

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/google/uuid"

	"github.com/tetiva-app/client/internal/adapters/portability"
	"github.com/tetiva-app/client/internal/adapters/publication/snapshotjson"
	"github.com/tetiva-app/client/internal/domain"
	"github.com/tetiva-app/client/internal/domain/entities"
	"github.com/tetiva-app/client/internal/domain/usecase/collection"
	"github.com/tetiva-app/client/internal/domain/usecase/environment"
	"github.com/tetiva-app/client/internal/domain/usecase/example"
	"github.com/tetiva-app/client/internal/domain/usecase/request"
)

// maxBytes is the decompressed size limit of a snapshot, the same the server accepts.
const maxBytes = 8 << 20

type Collections interface {
	Create(ctx context.Context, input collection.Create, opt collection.CreateOpt) (*entities.Collection, error)
	Edit(ctx context.Context, input collection.Edit, opt collection.EditOpt) (*entities.Collection, error)
	List(ctx context.Context, opt collection.ListOpt) ([]*entities.Collection, error)
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
	return snapshotjson.Sniff(data)
}

func (i *Importer) Preview(data []byte) (*portability.ImportPreview, error) {
	s, err := snapshotjson.Decode(bytes.NewReader(data), maxBytes)
	if err != nil {
		return nil, err
	}
	p := newPlan(s, false)
	preview := &portability.ImportPreview{
		Format:   portability.FormatTetiva,
		Title:    p.root.in.Name,
		Folders:  p.folders,
		Requests: p.requests,
		Examples: p.examples,
		Hosts:    portability.Hosts(p.urls, p.publicVars),
		Scripts:  p.scripts,
		Warnings: p.warnings,
	}
	if p.env != nil {
		preview.EnvironmentName = p.env.name
	}
	return preview, nil
}

// Import always creates a top-level collection and ignores opt.ParentID (see portability.ImportOpt).
func (i *Importer) Import(ctx context.Context, data []byte, opt portability.ImportOpt) (*portability.ImportResult, error) {
	const funcName = "snapshot.Importer.Import"

	s, err := snapshotjson.Decode(bytes.NewReader(data), maxBytes)
	if err != nil {
		return nil, err
	}
	p := newPlan(s, opt.IncludeScripts)

	in := p.root.in
	if in.Name, err = i.freeName(ctx, opt.WorkspaceID, in.Name); err != nil {
		return nil, fmt.Errorf("%s: %w", funcName, err)
	}
	root, err := i.collections.Create(ctx, in, collection.CreateOpt{UserID: opt.UserID, WorkspaceID: opt.WorkspaceID})
	if err != nil {
		return nil, fmt.Errorf("%s: create collection %q: %w", funcName, in.Name, err)
	}
	// collection.Create takes no gRPC metadata; only Edit sets it.
	if len(p.grpcMetadata) > 0 {
		_, err := i.collections.Edit(ctx, collection.Edit{
			Name: root.Name, PreScript: root.PreScript, PostScript: root.PostScript, Description: root.Description,
			AuthType: root.AuthType, AuthData: root.AuthData, GRPCMetadata: p.grpcMetadata,
		}, collection.EditOpt{CollectionID: root.ID, UserID: opt.UserID, Version: root.Version})
		if err != nil {
			return nil, fmt.Errorf("%s: gRPC metadata of %q: %w", funcName, root.Name, err)
		}
	}

	w := &writer{Importer: i, opt: opt, res: &portability.ImportResult{CollectionID: root.ID, Warnings: p.warnings}}
	if err := w.items(ctx, root.ID, p.root.items); err != nil {
		return nil, fmt.Errorf("%s: %w", funcName, err)
	}
	if p.env != nil {
		if err := w.environment(ctx, p.env); err != nil {
			return nil, fmt.Errorf("%s: %w", funcName, err)
		}
	}
	return w.res, nil
}

func (i *Importer) freeName(ctx context.Context, workspaceID uuid.UUID, name string) (string, error) {
	all, err := i.collections.List(ctx, collection.ListOpt{WorkspaceID: workspaceID})
	if err != nil {
		return "", err
	}
	taken := map[string]bool{}
	for _, c := range all {
		if c.ParentID == nil {
			taken[c.Name] = true
		}
	}
	candidate := name
	for n := 2; taken[candidate]; n++ {
		candidate = fmt.Sprintf("%s (%d)", name, n)
	}
	return candidate, nil
}

type writer struct {
	*Importer
	opt portability.ImportOpt
	res *portability.ImportResult
}

func (w *writer) items(ctx context.Context, parentID uuid.UUID, items []itemPlan) error {
	for _, it := range items {
		if f := it.folder; f != nil {
			in := f.in
			in.ParentID = &parentID
			c, err := w.collections.Create(ctx, in, collection.CreateOpt{UserID: w.opt.UserID, WorkspaceID: w.opt.WorkspaceID})
			if err != nil {
				return fmt.Errorf("create folder %q: %w", in.Name, err)
			}
			w.res.Folders++
			if err := w.items(ctx, c.ID, f.items); err != nil {
				return err
			}
			continue
		}
		if err := w.request(ctx, parentID, it.request); err != nil {
			return err
		}
	}
	return nil
}

// request skips an example the usecase rejects (a payload over the sync limit) instead of failing the import.
func (w *writer) request(ctx context.Context, collectionID uuid.UUID, rp *requestPlan) error {
	in := rp.in
	in.CollectionID = collectionID
	r, err := w.requests.Create(ctx, in, request.CreateOpt{UserID: w.opt.UserID})
	if err != nil {
		return fmt.Errorf("create request %q: %w", in.Name, err)
	}
	w.res.Requests++
	for _, ex := range rp.examples {
		ex.RequestID = r.ID
		if _, err := w.examples.Create(ctx, ex, example.CreateOpt{UserID: w.opt.UserID}); err != nil {
			var ve *domain.ValidationError
			if !errors.As(err, &ve) {
				return fmt.Errorf("create example %q of %s: %w", ex.Name, rp.label, err)
			}
			w.res.Warnings = append(w.res.Warnings, fmt.Sprintf("%s: example %q skipped: %s", rp.label, ex.Name, describe(ve)))
			continue
		}
		w.res.Examples++
	}
	return nil
}

func (w *writer) environment(ctx context.Context, ep *envPlan) error {
	env, err := w.environments.Create(ctx, environment.Create{Name: ep.name},
		environment.CreateOpt{UserID: w.opt.UserID, WorkspaceID: w.opt.WorkspaceID})
	if err != nil {
		return fmt.Errorf("create environment %q: %w", ep.name, err)
	}
	for _, v := range ep.vars {
		v.EnvironmentID = env.ID
		if _, err := w.environments.AddVariable(ctx, v, environment.AddVariableOpt{UserID: w.opt.UserID}); err != nil {
			return fmt.Errorf("add variable %q: %w", v.Key, err)
		}
	}
	return nil
}

func describe(ve *domain.ValidationError) string {
	msgs := make([]string, 0, len(ve.Fields))
	for _, msg := range ve.Fields {
		msgs = append(msgs, msg)
	}
	slices.Sort(msgs)
	return strings.Join(msgs, "; ")
}
