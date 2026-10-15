package insomnia_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/tetiva-app/client/internal/adapters/portability"
	"github.com/tetiva-app/client/internal/adapters/portability/insomnia"
	"github.com/tetiva-app/client/internal/domain/entities"
	"github.com/tetiva-app/client/internal/domain/usecase/collection"
	"github.com/tetiva-app/client/internal/domain/usecase/environment"
	"github.com/tetiva-app/client/internal/domain/usecase/request"
)

var workspaceID = uuid.MustParse("00000000-0000-4000-a000-000000000001")

type store struct {
	collections []collection.Create
	collIDs     []uuid.UUID
	requests    []request.Create
	envs        []string
	vars        map[string][]environment.AddVariable
	envNames    map[uuid.UUID]string
	existing    []string
}

func newStore() *store {
	return &store{vars: map[string][]environment.AddVariable{}, envNames: map[uuid.UUID]string{}}
}

func (s *store) importer() *insomnia.Importer {
	return insomnia.NewImporter(collections{s}, requests{s}, environments{s})
}

type collections struct{ *store }

func (c collections) Create(_ context.Context, in collection.Create, _ collection.CreateOpt) (*entities.Collection, error) {
	id := uuid.New()
	c.collections = append(c.collections, in)
	c.collIDs = append(c.collIDs, id)
	return &entities.Collection{ID: id, Name: in.Name, ParentID: in.ParentID}, nil
}

type requests struct{ *store }

func (r requests) Create(_ context.Context, in request.Create, _ request.CreateOpt) (*entities.Request, error) {
	r.requests = append(r.requests, in)
	return &entities.Request{ID: uuid.New(), Name: in.Name}, nil
}

type environments struct{ *store }

func (e environments) Create(_ context.Context, in environment.Create, _ environment.CreateOpt) (*entities.Environment, error) {
	id := uuid.New()
	e.envs = append(e.envs, in.Name)
	e.envNames[id] = in.Name
	return &entities.Environment{ID: id, Name: in.Name}, nil
}

func (e environments) AddVariable(_ context.Context, in environment.AddVariable, _ environment.AddVariableOpt) (*entities.Variable, error) {
	name := e.envNames[in.EnvironmentID]
	in.EnvironmentID = uuid.Nil
	e.vars[name] = append(e.vars[name], in)
	return &entities.Variable{ID: uuid.New(), Key: in.Key}, nil
}

func (e environments) List(context.Context, environment.ListOpt) ([]*entities.Environment, error) {
	out := make([]*entities.Environment, 0, len(e.existing))
	for _, name := range e.existing {
		out = append(out, &entities.Environment{ID: uuid.New(), Name: name})
	}
	return out, nil
}

func (s *store) request(t *testing.T, name string) request.Create {
	t.Helper()
	for _, r := range s.requests {
		if r.Name == name {
			return r
		}
	}
	t.Fatalf("request %q was not created", name)
	return request.Create{}
}

func (s *store) collection(t *testing.T, name string) (collection.Create, uuid.UUID) {
	t.Helper()
	for i, c := range s.collections {
		if c.Name == name {
			return c, s.collIDs[i]
		}
	}
	t.Fatalf("collection %q was not created", name)
	return collection.Create{}, uuid.Nil
}

func (s *store) requestNames() []string {
	out := make([]string, 0, len(s.requests))
	for _, r := range s.requests {
		out = append(out, r.Name)
	}
	return out
}

func fixture(t *testing.T, name string) []byte {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("testdata", name))
	require.NoError(t, err)
	return raw
}

func importFile(t *testing.T, s *store, data []byte, includeScripts bool) *portability.ImportResult {
	t.Helper()
	res, err := s.importer().Import(context.Background(), data, portability.ImportOpt{
		WorkspaceID: workspaceID, UserID: "local_user", IncludeScripts: includeScripts,
	})
	require.NoError(t, err)
	return res
}
