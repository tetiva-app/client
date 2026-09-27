package publication_test

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/tetiva-app/client/internal/adapters/publication/snapshotjson"
	"github.com/tetiva-app/client/internal/domain/entities"
	"github.com/tetiva-app/client/internal/domain/usecase/publication"
)

var workspaceID = uuid.MustParse("00000000-0000-4000-a000-000000000001")

type fixture struct {
	in  publication.BuildInput
	seq int
	at  time.Time
}

func newFixture() *fixture {
	f := &fixture{at: time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC)}
	f.in.Examples = map[uuid.UUID][]*entities.ResponseExample{}
	f.in.Generator = "Tetiva test"
	f.in.Locale = "en"
	f.in.Root = f.collection("Petstore", nil)
	return f
}

func (f *fixture) id() uuid.UUID {
	f.seq++
	return uuid.MustParse(fmt.Sprintf("00000000-0000-4000-a000-%012d", 100+f.seq))
}

func (f *fixture) tick() time.Time {
	f.at = f.at.Add(time.Second)
	return f.at
}

func (f *fixture) collection(name string, parent *entities.Collection) *entities.Collection {
	c := &entities.Collection{
		ID: f.id(), WorkspaceID: workspaceID, Name: name,
		AuthType: entities.AuthTypeNone, AuthData: "{}", GRPCMetadata: []entities.HeaderItem{},
		Version: 1, CreatedAt: f.tick(),
	}
	if parent != nil {
		pid := parent.ID
		c.ParentID = &pid
	}
	f.in.Collections = append(f.in.Collections, c)
	return c
}

func (f *fixture) folder(parent *entities.Collection, name string) *entities.Collection {
	return f.collection(name, parent)
}

func (f *fixture) request(parent *entities.Collection, name string) *entities.Request {
	r := &entities.Request{
		ID: f.id(), CollectionID: parent.ID, Name: name,
		Protocol: entities.ProtocolHTTP, Method: entities.MethodGET, URL: "https://api.example.com/x",
		Headers: []entities.HeaderItem{}, BodyType: entities.BodyTypeNone,
		AuthType: entities.AuthTypeInherit, AuthData: "{}", GRPCMetadata: map[string][]string{},
		Version: 1, CreatedAt: f.tick(),
	}
	f.in.Requests = append(f.in.Requests, r)
	return r
}

func (f *fixture) example(r *entities.Request, name string) *entities.ResponseExample {
	e := &entities.ResponseExample{
		ID: f.id(), RequestID: r.ID, WorkspaceID: workspaceID, Name: name,
		StatusCode: 200, StatusText: "OK", Headers: []entities.HeaderItem{}, Protocol: r.Protocol,
		Version: 1, CreatedAt: f.tick(),
	}
	f.in.Examples[r.ID] = append(f.in.Examples[r.ID], e)
	return e
}

func (f *fixture) env(name string) *entities.Environment {
	f.in.Environment = &entities.Environment{ID: f.id(), WorkspaceID: workspaceID, Name: name, Version: 1, CreatedAt: f.tick()}
	return f.in.Environment
}

func (f *fixture) variable(key, value string, secret bool) *entities.Variable {
	if f.in.Environment == nil {
		f.env("prod")
	}
	v := &entities.Variable{
		ID: f.id(), EnvironmentID: f.in.Environment.ID, Key: key, Value: value,
		IsSecret: secret, Enabled: true, Version: 1, CreatedAt: f.tick(),
	}
	f.in.Variables = append(f.in.Variables, v)
	return v
}

func (f *fixture) build(t *testing.T) (*publication.Snapshot, publication.Report) {
	t.Helper()
	s, report, err := publication.Build(f.in)
	require.NoError(t, err)
	return s, report
}

func (f *fixture) marshal(t *testing.T) (string, publication.Report) {
	t.Helper()
	s, report := f.build(t)
	out, err := snapshotjson.Marshal(s)
	require.NoError(t, err)
	return string(out), report
}

func (f *fixture) opaque(id uuid.UUID) string {
	return publication.OpaqueID(f.in.Root.ID, id)
}

func (f *fixture) varSelector(v *entities.Variable) string {
	sum := sha256.Sum256([]byte(v.Value))
	return f.opaque(v.ID) + "/var/" + hex.EncodeToString(sum[:])[:12]
}

func findRequest(items []publication.Item, name string) *publication.Request {
	for _, it := range items {
		if it.Request != nil && it.Request.Name == name {
			return it.Request
		}
		if it.Folder != nil {
			if r := findRequest(it.Folder.Items, name); r != nil {
				return r
			}
		}
	}
	return nil
}

func findVariable(s *publication.Snapshot, key string) *publication.Variable {
	if s.Environment == nil {
		return nil
	}
	for i := range s.Environment.Variables {
		if s.Environment.Variables[i].Key == key {
			return &s.Environment.Variables[i]
		}
	}
	return nil
}

func hiddenVar(r publication.Report, key string) *publication.HiddenVar {
	for i := range r.HiddenVars {
		if r.HiddenVars[i].Key == key {
			return &r.HiddenVars[i]
		}
	}
	return nil
}

func redactionsOf(r publication.Report, category string) []publication.Redaction {
	var out []publication.Redaction
	for _, rd := range r.Redactions {
		if rd.Category == category {
			out = append(out, rd)
		}
	}
	return out
}

func warningsOf(r publication.Report, rule string) []publication.Warning {
	var out []publication.Warning
	for _, w := range r.Warnings {
		if w.Rule == rule {
			out = append(out, w)
		}
	}
	return out
}
