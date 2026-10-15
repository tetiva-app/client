package portability_test

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tetiva-app/client/internal/adapters/portability"
	"github.com/tetiva-app/client/internal/domain"
	"github.com/tetiva-app/client/internal/domain/entities"
	"github.com/tetiva-app/client/internal/domain/usecase/collection"
	"github.com/tetiva-app/client/internal/domain/usecase/environment"
	"github.com/tetiva-app/client/internal/domain/usecase/example"
	"github.com/tetiva-app/client/internal/domain/usecase/request"
)

type memory struct {
	collections []collection.Create
	requests    []request.Create
	examples    []example.Create
	envs        []string
	vars        []environment.AddVariable
	exampleErr  error
}

type memCollections struct{ *memory }

func (m memCollections) Create(_ context.Context, in collection.Create, _ collection.CreateOpt) (*entities.Collection, error) {
	m.collections = append(m.collections, in)
	return &entities.Collection{ID: uuid.New(), Name: in.Name}, nil
}

type memRequests struct{ *memory }

func (m memRequests) Create(_ context.Context, in request.Create, _ request.CreateOpt) (*entities.Request, error) {
	m.requests = append(m.requests, in)
	return &entities.Request{ID: uuid.New(), Name: in.Name}, nil
}

type memExamples struct{ *memory }

func (m memExamples) Create(_ context.Context, in example.Create, _ example.CreateOpt) (*entities.ResponseExample, error) {
	if m.exampleErr != nil && in.Name == "bad" {
		return nil, m.exampleErr
	}
	m.examples = append(m.examples, in)
	return &entities.ResponseExample{ID: uuid.New()}, nil
}

type memEnvironments struct{ *memory }

func (m memEnvironments) Create(_ context.Context, in environment.Create, _ environment.CreateOpt) (*entities.Environment, error) {
	m.envs = append(m.envs, in.Name)
	return &entities.Environment{ID: uuid.New(), Name: in.Name}, nil
}

func (m memEnvironments) AddVariable(_ context.Context, in environment.AddVariable, _ environment.AddVariableOpt) (*entities.Variable, error) {
	m.vars = append(m.vars, in)
	return &entities.Variable{ID: uuid.New()}, nil
}

func (m memEnvironments) List(context.Context, environment.ListOpt) ([]*entities.Environment, error) {
	return []*entities.Environment{{Name: "Shop"}}, nil
}

func writer(m *memory) portability.Writer {
	return portability.Writer{Collections: memCollections{m}, Requests: memRequests{m}, Examples: memExamples{m}, Environments: memEnvironments{m}}
}

func sampleTree() *portability.Tree {
	return &portability.Tree{
		Root: portability.Folder{
			In: collection.Create{Name: "Shop", PostScript: "pm.test()"},
			Folders: []portability.Folder{{
				In:       collection.Create{Name: "Orders"},
				Requests: []portability.Request{{In: request.Create{Name: "List", URL: "{{host}}/orders", PreScript: "pm.x()"}}},
			}},
			Requests: []portability.Request{{
				In:       request.Create{Name: "Ping", URL: "https://shop.example.com/ping"},
				Examples: []example.Create{{Name: "good"}, {Name: "bad"}},
			}},
		},
		Environments: []portability.Environment{{Name: "Shop", Vars: []environment.AddVariable{
			{Key: "host", Value: "https://api.example.com"}, {Key: "secret", Value: "https://hidden.example.com", IsSecret: true},
		}}},
		Warnings: []string{"from the mapping"},
	}
}

func TestTreePreview(t *testing.T) {
	p := sampleTree().Preview("x")

	assert.Equal(t, &portability.ImportPreview{
		Format: "x", Title: "Shop", Folders: 1, Requests: 2, Examples: 2,
		EnvironmentNames: []string{"Shop"},
		Hosts:            []string{"api.example.com", "shop.example.com"},
		Scripts: []portability.ScriptPreview{
			{Path: "Shop", Phase: "post", Text: "pm.test()"},
			{Path: "Shop / Orders / List", Phase: "pre", Text: "pm.x()"},
		},
		Warnings: []string{"from the mapping"},
	}, p)
}

func TestTreeWrite(t *testing.T) {
	m := &memory{exampleErr: &domain.ValidationError{Fields: map[string]string{"name": "too long"}}}

	res, err := writer(m).Write(context.Background(), sampleTree(), portability.ImportOpt{WorkspaceID: uuid.New()})
	require.NoError(t, err)

	assert.Equal(t, 1, res.Folders)
	assert.Equal(t, 2, res.Requests)
	assert.Equal(t, 1, res.Examples)
	assert.Equal(t, []string{"Shop (2)"}, res.EnvironmentNames)
	assert.Equal(t, []string{"from the mapping", `request "Ping": example "bad" skipped: too long`}, res.Warnings)
	assert.Empty(t, m.collections[0].PostScript)
	assert.Empty(t, m.requests[0].PreScript)
	assert.Len(t, m.vars, 2)
	assert.NotNil(t, m.collections[1].ParentID)
	assert.Nil(t, m.collections[0].ParentID)
}

func TestTree_OneEnvironmentPerCollection(t *testing.T) {
	tr := sampleTree()
	tr.Environments = append(tr.Environments,
		portability.Environment{Name: "Billing", Vars: []environment.AddVariable{{Key: "host", Value: "https://billing.example.com"}}},
		portability.Environment{Name: "Empty"},
	)

	p := tr.Preview("x")
	assert.Equal(t, []string{"Shop", "Billing"}, p.EnvironmentNames)
	assert.Equal(t, []string{"api.example.com", "billing.example.com", "shop.example.com"}, p.Hosts)

	m := &memory{}
	res, err := writer(m).Write(context.Background(), tr, portability.ImportOpt{WorkspaceID: uuid.New()})
	require.NoError(t, err)
	assert.Equal(t, []string{"Shop (2)", "Billing"}, res.EnvironmentNames)
	assert.Len(t, m.vars, 3)
}

func TestTreeWrite_KeepsScriptsWhenAskedAndStopsOnStorageErrors(t *testing.T) {
	m := &memory{}
	_, err := writer(m).Write(context.Background(), sampleTree(), portability.ImportOpt{IncludeScripts: true})
	require.NoError(t, err)
	assert.Equal(t, "pm.test()", m.collections[0].PostScript)

	failing := &memory{exampleErr: errors.New("disk full")}
	_, err = writer(failing).Write(context.Background(), sampleTree(), portability.ImportOpt{})
	assert.ErrorContains(t, err, "disk full")
}

func TestTreeWarnings_NameExamplesThatLookSecret(t *testing.T) {
	tr := &portability.Tree{Root: portability.Folder{In: collection.Create{Name: "C"}}}
	for i := range 7 {
		tr.Root.Requests = append(tr.Root.Requests, portability.Request{
			In:       request.Create{Name: fmt.Sprintf("R%d", i)},
			Examples: []example.Create{{Name: "200", Body: `{"key":"AKIAIOSFODNN7EXAMPLE"}`}},
		})
	}

	warnings := tr.Preview("x").Warnings

	require.Len(t, warnings, 1)
	assert.Contains(t, warnings[0], `examples may contain secrets, check them before sharing: "R0" / "200" (`)
	assert.Contains(t, warnings[0], `"R4" / "200"`)
	assert.NotContains(t, warnings[0], `"R5"`)
	assert.Contains(t, warnings[0], "; and 2 more")
}

func TestFileTitle(t *testing.T) {
	cases := map[string]string{
		"api.example.com.har": "api.example.com",
		"hoppscotch.json":     "hoppscotch",
		"no-extension":        "no-extension",
		".har":                "fallback",
		"":                    "fallback",
	}
	for in, want := range cases {
		assert.Equal(t, want, portability.FileTitle(in, "fallback"), in)
	}
}
