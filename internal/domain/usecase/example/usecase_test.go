package example_test

import (
	"context"
	"errors"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/tetiva-app/client/internal/domain"
	"github.com/tetiva-app/client/internal/domain/entities"
	"github.com/tetiva-app/client/internal/domain/usecase/example"
)

var (
	wsA = uuid.MustParse("00000000-0000-4000-a000-000000000001")
	wsB = uuid.MustParse("00000000-0000-4000-a000-000000000002")
)

type fakeRepo struct {
	rows      map[uuid.UUID]*entities.ResponseExample
	updateErr error
}

func newFakeRepo() *fakeRepo {
	return &fakeRepo{rows: make(map[uuid.UUID]*entities.ResponseExample)}
}

func cloneExample(e *entities.ResponseExample) *entities.ResponseExample {
	cp := *e
	if e.Headers != nil {
		cp.Headers = append([]entities.HeaderItem(nil), e.Headers...)
	}
	return &cp
}

func (r *fakeRepo) Create(_ context.Context, e *entities.ResponseExample) error {
	r.rows[e.ID] = cloneExample(e)
	return nil
}

func (r *fakeRepo) GetByID(_ context.Context, id uuid.UUID) (*entities.ResponseExample, error) {
	e, ok := r.rows[id]
	if !ok || e.IsDelete {
		return nil, nil
	}
	return cloneExample(e), nil
}

func (r *fakeRepo) ListByRequest(_ context.Context, requestID uuid.UUID) ([]*entities.ResponseExample, error) {
	var out []*entities.ResponseExample
	for _, e := range r.rows {
		if e.RequestID == requestID && !e.IsDelete {
			out = append(out, cloneExample(e))
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].SortOrder < out[j].SortOrder })
	return out, nil
}

func (r *fakeRepo) Update(_ context.Context, e *entities.ResponseExample) error {
	if r.updateErr != nil {
		return r.updateErr
	}
	r.rows[e.ID] = cloneExample(e)
	return nil
}

func (r *fakeRepo) UpdateAtVersion(_ context.Context, e *entities.ResponseExample, baseVersion int) error {
	if r.updateErr != nil {
		return r.updateErr
	}
	cur, ok := r.rows[e.ID]
	if !ok || cur.IsDelete || cur.Version != baseVersion {
		return &domain.ConflictError{Entity: "example", ID: e.ID.String()}
	}
	r.rows[e.ID] = cloneExample(e)
	return nil
}

type fakeRequests map[uuid.UUID]*entities.Request

func (f fakeRequests) GetByID(_ context.Context, id uuid.UUID) (*entities.Request, error) {
	r, ok := f[id]
	if !ok || r.IsDelete {
		return nil, nil
	}
	cp := *r
	return &cp, nil
}

type fakeCollections map[uuid.UUID]*entities.Collection

func (f fakeCollections) GetByID(_ context.Context, id uuid.UUID) (*entities.Collection, error) {
	c, ok := f[id]
	if !ok || c.IsDelete {
		return nil, nil
	}
	cp := *c
	return &cp, nil
}

type fixture struct {
	repo        *fakeRepo
	requests    fakeRequests
	collections fakeCollections
	uc          example.Usecase
	root        *entities.Collection
	folder      *entities.Collection
	req         *entities.Request
}

func newFixture() *fixture {
	root := &entities.Collection{ID: uuid.New(), WorkspaceID: wsA}
	folder := &entities.Collection{ID: uuid.New(), WorkspaceID: wsA, ParentID: &root.ID}
	req := &entities.Request{ID: uuid.New(), CollectionID: folder.ID, Protocol: entities.ProtocolHTTP, Version: 1}
	f := &fixture{
		repo:        newFakeRepo(),
		requests:    fakeRequests{req.ID: req},
		collections: fakeCollections{root.ID: root, folder.ID: folder},
		root:        root,
		folder:      folder,
		req:         req,
	}
	f.uc = example.NewUsecase(f.repo, f.requests, f.collections)
	return f
}

func validCreate(requestID uuid.UUID) example.Create {
	return example.Create{
		RequestID:   requestID,
		Name:        "200 OK",
		StatusCode:  200,
		StatusText:  "OK",
		Headers:     []entities.HeaderItem{{Key: "Content-Type", Value: "application/json", Enabled: true}},
		Body:        `{"ok":true}`,
		ContentType: "application/json",
		Protocol:    entities.ProtocolHTTP,
	}
}

func (f *fixture) seed(t *testing.T) *entities.ResponseExample {
	t.Helper()
	e, err := f.uc.Create(context.Background(), validCreate(f.req.ID), example.CreateOpt{UserID: "u1"})
	if err != nil {
		t.Fatalf("seed: %v", err)
	}
	return e
}

func validationFields(t *testing.T, err error) map[string]string {
	t.Helper()
	var ve *domain.ValidationError
	if !errors.As(err, &ve) {
		t.Fatalf("err = %v, want ValidationError", err)
	}
	return ve.Fields
}

func requireNotFound(t *testing.T, err error) {
	t.Helper()
	var nf *domain.NotFoundError
	if !errors.As(err, &nf) {
		t.Fatalf("err = %v, want NotFoundError", err)
	}
}

func headerValue(headers []entities.HeaderItem, key string) string {
	for _, h := range headers {
		if h.Key == key {
			return h.Value
		}
	}
	return "<missing>"
}

func TestCreate_Validation(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(*example.Create)
		field  string
	}{
		{"empty name", func(c *example.Create) { c.Name = "" }, "name"},
		{"blank name", func(c *example.Create) { c.Name = "   " }, "name"},
		{"name over 200 characters", func(c *example.Create) { c.Name = strings.Repeat("я", 201) }, "name"},
		{"negative status", func(c *example.Create) { c.StatusCode = -1 }, "statusCode"},
		{"status over 999", func(c *example.Create) { c.StatusCode = 1000 }, "statusCode"},
		{"websocket protocol", func(c *example.Create) { c.Protocol = entities.ProtocolWebSocket }, "protocol"},
		{"empty protocol", func(c *example.Create) { c.Protocol = "" }, "protocol"},
		{"body over limit", func(c *example.Create) { c.Body = strings.Repeat("a", domain.MaxExampleBodyLen+1) }, "body"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := newFixture()
			in := validCreate(f.req.ID)
			tc.mutate(&in)
			_, err := f.uc.Create(context.Background(), in, example.CreateOpt{UserID: "u1"})
			if _, ok := validationFields(t, err)[tc.field]; !ok {
				t.Fatalf("fields = %v, want %q", validationFields(t, err), tc.field)
			}
			if len(f.repo.rows) != 0 {
				t.Fatal("invalid example was stored")
			}
		})
	}
}

func TestCreate_AcceptsBoundaries(t *testing.T) {
	f := newFixture()
	in := validCreate(f.req.ID)
	in.Name = strings.Repeat("я", 200)
	in.StatusCode = 999
	in.Body = strings.Repeat("a", domain.MaxExampleBodyLen)
	in.Protocol = entities.ProtocolGRPC
	if _, err := f.uc.Create(context.Background(), in, example.CreateOpt{UserID: "u1"}); err != nil {
		t.Fatalf("Create: %v", err)
	}
}

func TestCreate_FillsDerivedFields(t *testing.T) {
	f := newFixture()
	first := f.seed(t)
	second, err := f.uc.Create(context.Background(), validCreate(f.req.ID), example.CreateOpt{UserID: "u2"})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	if first.WorkspaceID != wsA || second.WorkspaceID != wsA {
		t.Errorf("workspace = %s/%s, want %s", first.WorkspaceID, second.WorkspaceID, wsA)
	}
	if second.SortOrder != first.SortOrder+1 {
		t.Errorf("sort order = %d after %d, want max+1", second.SortOrder, first.SortOrder)
	}
	if second.Version != 1 || second.IsDelete {
		t.Errorf("version = %d, deleted = %v", second.Version, second.IsDelete)
	}
	if second.CreatedBy != "u2" || second.UpdatedBy != "u2" || second.CreatedAt.IsZero() || second.UpdatedAt.IsZero() {
		t.Errorf("audit fields not set: %+v", second)
	}
	if _, err := uuid.Parse(second.ID.String()); err != nil || second.ID == first.ID {
		t.Errorf("id = %s", second.ID)
	}
	if f.repo.rows[second.ID] == nil {
		t.Error("example not stored")
	}
}

func TestCreate_MasksHeaders(t *testing.T) {
	f := newFixture()
	in := validCreate(f.req.ID)
	in.Headers = []entities.HeaderItem{
		{Key: "Authorization", Value: "Bearer abc", Enabled: true},
		{Key: "X-Token", Value: "Bearer {{t}}", Enabled: true},
		{Key: "{{hdr}}", Value: "literal", Enabled: true},
		{Key: "Set-Cookie", Value: "sid=42; Path=/", Enabled: true},
		{Key: "Accept", Value: "application/json", Enabled: true},
		{Key: "Location", Value: "https://s3.example/r?X-Amz-Signature=sig&page=1", Enabled: true},
	}
	original := append([]entities.HeaderItem(nil), in.Headers...)

	e, err := f.uc.Create(context.Background(), in, example.CreateOpt{UserID: "u1"})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	for _, headers := range [][]entities.HeaderItem{e.Headers, f.repo.rows[e.ID].Headers} {
		want := map[string]string{
			"Authorization": "Bearer <redacted>",
			"X-Token":       "Bearer {{t}}",
			"{{hdr}}":       "<redacted>",
			"Set-Cookie":    "<redacted>",
			"Accept":        "application/json",
			"Location":      "https://s3.example/r?X-Amz-Signature=<redacted>&page=1",
		}
		for key, value := range want {
			if got := headerValue(headers, key); got != value {
				t.Errorf("%s = %q, want %q", key, got, value)
			}
		}
	}
	for i := range original {
		if in.Headers[i] != original[i] {
			t.Errorf("caller's header %d mutated: %+v", i, in.Headers[i])
		}
	}
}

func TestEdit_MasksHeaders(t *testing.T) {
	f := newFixture()
	e := f.seed(t)

	got, err := f.uc.Edit(context.Background(), example.Edit{
		Name:       "401",
		StatusCode: 401,
		Headers: []entities.HeaderItem{
			{Key: "Authorization", Value: "Bearer abc", Enabled: true},
			{Key: "X-Token", Value: "Bearer {{t}}", Enabled: true},
			{Key: "{{hdr}}", Value: "literal", Enabled: true},
		},
	}, example.EditOpt{ExampleID: e.ID, UserID: "u1", Version: e.Version})
	if err != nil {
		t.Fatalf("Edit: %v", err)
	}

	stored := f.repo.rows[e.ID].Headers
	for key, want := range map[string]string{"Authorization": "Bearer <redacted>", "X-Token": "Bearer {{t}}", "{{hdr}}": "<redacted>"} {
		if v := headerValue(got.Headers, key); v != want {
			t.Errorf("returned %s = %q, want %q", key, v, want)
		}
		if v := headerValue(stored, key); v != want {
			t.Errorf("stored %s = %q, want %q", key, v, want)
		}
	}
}

func TestCreate_HeadersPushPayloadOverLimit(t *testing.T) {
	f := newFixture()
	in := validCreate(f.req.ID)
	in.Body = strings.Repeat("a", domain.MaxExampleBodyLen)
	in.Headers = []entities.HeaderItem{{Key: "X-Trace", Value: strings.Repeat("b", 240*1024), Enabled: true}}

	_, err := f.uc.Create(context.Background(), in, example.CreateOpt{UserID: "u1"})

	fields := validationFields(t, err)
	if fields["example"] != "example is too large (max 480 KB)" {
		t.Fatalf("fields = %v", fields)
	}
}

func TestCreate_PayloadMeasuredAfterMasking(t *testing.T) {
	f := newFixture()
	in := validCreate(f.req.ID)
	in.Body = strings.Repeat("a", domain.MaxExampleBodyLen)
	in.Headers = []entities.HeaderItem{{Key: "Authorization", Value: "Bearer " + strings.Repeat("s", 300*1024), Enabled: true}}

	if _, err := f.uc.Create(context.Background(), in, example.CreateOpt{UserID: "u1"}); err != nil {
		t.Fatalf("Create: %v", err)
	}
}

func TestEdit_HeadersPushPayloadOverLimit(t *testing.T) {
	f := newFixture()
	e := f.seed(t)

	_, err := f.uc.Edit(context.Background(), example.Edit{
		Name:    "big",
		Body:    strings.Repeat("a", domain.MaxExampleBodyLen),
		Headers: []entities.HeaderItem{{Key: "X-Trace", Value: strings.Repeat("b", 240*1024), Enabled: true}},
	}, example.EditOpt{ExampleID: e.ID, UserID: "u1", Version: e.Version})

	if _, ok := validationFields(t, err)["example"]; !ok {
		t.Fatalf("err = %v, want example too large", err)
	}
	if f.repo.rows[e.ID].Version != e.Version {
		t.Error("rejected edit was stored")
	}
}

func TestCreate_DraftRequestRejected(t *testing.T) {
	f := newFixture()
	f.requests[f.req.ID].IsDraft = true

	_, err := f.uc.Create(context.Background(), validCreate(f.req.ID), example.CreateOpt{UserID: "u1"})

	if got := validationFields(t, err)["request"]; got != "save the request before adding examples" {
		t.Fatalf("request field = %q", got)
	}
}

func TestCreate_MissingRequest(t *testing.T) {
	f := newFixture()
	_, err := f.uc.Create(context.Background(), validCreate(uuid.New()), example.CreateOpt{UserID: "u1"})
	requireNotFound(t, err)
}

func TestDeadGrandparentHidesExamples(t *testing.T) {
	f := newFixture()
	e := f.seed(t)
	f.collections[f.root.ID].IsDelete = true
	ctx := context.Background()

	list, err := f.uc.ListByRequest(ctx, f.req.ID)
	if err != nil {
		t.Fatalf("ListByRequest: %v", err)
	}
	if len(list) != 0 {
		t.Errorf("list = %d examples, want none", len(list))
	}

	_, err = f.uc.Create(ctx, validCreate(f.req.ID), example.CreateOpt{UserID: "u1"})
	requireNotFound(t, err)

	_, err = f.uc.Edit(ctx, example.Edit{Name: "x"}, example.EditOpt{ExampleID: e.ID, UserID: "u1", Version: e.Version})
	requireNotFound(t, err)

	err = f.uc.Delete(ctx, example.DeleteOpt{ExampleID: e.ID, UserID: "u1", Version: e.Version})
	requireNotFound(t, err)

	if f.repo.rows[e.ID].IsDelete || f.repo.rows[e.ID].Version != e.Version {
		t.Error("example behind a dead chain was changed")
	}
}

func TestForeignWorkspaceExampleHidden(t *testing.T) {
	f := newFixture()
	live := f.seed(t)
	foreign := cloneExample(live)
	foreign.ID = uuid.New()
	foreign.WorkspaceID = wsB
	f.repo.rows[foreign.ID] = foreign
	ctx := context.Background()

	list, err := f.uc.ListByRequest(ctx, f.req.ID)
	if err != nil {
		t.Fatalf("ListByRequest: %v", err)
	}
	if len(list) != 1 || list[0].ID != live.ID {
		t.Fatalf("list = %v, want only the live example", list)
	}

	_, err = f.uc.Edit(ctx, example.Edit{Name: "x"}, example.EditOpt{ExampleID: foreign.ID, UserID: "u1", Version: 1})
	requireNotFound(t, err)
	err = f.uc.Delete(ctx, example.DeleteOpt{ExampleID: foreign.ID, UserID: "u1", Version: 1})
	requireNotFound(t, err)
}

func TestListByRequest_EmptyNotNil(t *testing.T) {
	f := newFixture()
	list, err := f.uc.ListByRequest(context.Background(), f.req.ID)
	if err != nil {
		t.Fatalf("ListByRequest: %v", err)
	}
	if list == nil || len(list) != 0 {
		t.Fatalf("list = %#v, want empty slice", list)
	}
}

func TestChainWorkspace(t *testing.T) {
	ctx := context.Background()

	t.Run("live chain", func(t *testing.T) {
		f := newFixture()
		ws, ok, err := example.ChainWorkspace(ctx, f.requests, f.collections, f.req.ID)
		if err != nil || !ok || ws != wsA {
			t.Fatalf("ws=%s ok=%v err=%v", ws, ok, err)
		}
	})
	t.Run("ancestor in another workspace", func(t *testing.T) {
		f := newFixture()
		f.collections[f.root.ID].WorkspaceID = wsB
		if _, ok, err := example.ChainWorkspace(ctx, f.requests, f.collections, f.req.ID); err != nil || ok {
			t.Fatalf("ok=%v err=%v, want not ok", ok, err)
		}
	})
	t.Run("missing ancestor", func(t *testing.T) {
		f := newFixture()
		delete(f.collections, f.root.ID)
		if _, ok, err := example.ChainWorkspace(ctx, f.requests, f.collections, f.req.ID); err != nil || ok {
			t.Fatalf("ok=%v err=%v, want not ok", ok, err)
		}
	})
	t.Run("deleted request", func(t *testing.T) {
		f := newFixture()
		f.requests[f.req.ID].IsDelete = true
		if _, ok, err := example.ChainWorkspace(ctx, f.requests, f.collections, f.req.ID); err != nil || ok {
			t.Fatalf("ok=%v err=%v, want not ok", ok, err)
		}
	})
	t.Run("parent cycle", func(t *testing.T) {
		f := newFixture()
		f.collections[f.root.ID].ParentID = &f.folder.ID
		if _, ok, err := example.ChainWorkspace(ctx, f.requests, f.collections, f.req.ID); err != nil || ok {
			t.Fatalf("ok=%v err=%v, want not ok", ok, err)
		}
	})
}

func TestEdit_UpdatesAndChecksVersion(t *testing.T) {
	f := newFixture()
	e := f.seed(t)
	ctx := context.Background()

	_, err := f.uc.Edit(ctx, example.Edit{Name: "x"}, example.EditOpt{ExampleID: e.ID, UserID: "u1", Version: e.Version + 1})
	var conflict *domain.ConflictError
	if !errors.As(err, &conflict) {
		t.Fatalf("err = %v, want ConflictError", err)
	}

	got, err := f.uc.Edit(ctx, example.Edit{
		Name: "404 Not Found", StatusCode: 404, StatusText: "Not Found", Body: "{}", ContentType: "application/json",
	}, example.EditOpt{ExampleID: e.ID, UserID: "u2", Version: e.Version})
	if err != nil {
		t.Fatalf("Edit: %v", err)
	}
	if got.Version != e.Version+1 || got.Name != "404 Not Found" || got.StatusCode != 404 || got.UpdatedBy != "u2" {
		t.Errorf("edited = %+v", got)
	}
	if got.Protocol != e.Protocol || got.SortOrder != e.SortOrder || got.WorkspaceID != e.WorkspaceID || got.CreatedBy != e.CreatedBy {
		t.Errorf("fields outside Edit changed: %+v", got)
	}
	if f.repo.rows[e.ID].Name != "404 Not Found" {
		t.Error("edit not stored")
	}
}

func TestEdit_Validation(t *testing.T) {
	f := newFixture()
	e := f.seed(t)
	_, err := f.uc.Edit(context.Background(), example.Edit{Name: "", StatusCode: 1000},
		example.EditOpt{ExampleID: e.ID, UserID: "u1", Version: e.Version})
	fields := validationFields(t, err)
	if _, ok := fields["name"]; !ok {
		t.Errorf("fields = %v, want name", fields)
	}
	if _, ok := fields["statusCode"]; !ok {
		t.Errorf("fields = %v, want statusCode", fields)
	}
}

func TestDelete_SoftDeletesAndChecksVersion(t *testing.T) {
	f := newFixture()
	e := f.seed(t)
	ctx := context.Background()

	err := f.uc.Delete(ctx, example.DeleteOpt{ExampleID: e.ID, UserID: "u1", Version: e.Version + 1})
	var conflict *domain.ConflictError
	if !errors.As(err, &conflict) {
		t.Fatalf("err = %v, want ConflictError", err)
	}

	if err := f.uc.Delete(ctx, example.DeleteOpt{ExampleID: e.ID, UserID: "u2", Version: e.Version}); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	row := f.repo.rows[e.ID]
	if !row.IsDelete || row.Version != e.Version+1 || row.UpdatedBy != "u2" {
		t.Errorf("row = %+v", row)
	}

	err = f.uc.Delete(ctx, example.DeleteOpt{ExampleID: e.ID, UserID: "u1", Version: row.Version})
	requireNotFound(t, err)
}

func TestDeleteByRequest_IgnoresChainLiveness(t *testing.T) {
	f := newFixture()
	a := f.seed(t)
	b := f.seed(t)
	otherReq := &entities.Request{ID: uuid.New(), CollectionID: f.folder.ID, Version: 1}
	f.requests[otherReq.ID] = otherReq
	other, err := f.uc.Create(context.Background(), validCreate(otherReq.ID), example.CreateOpt{UserID: "u1"})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	f.requests[f.req.ID].IsDelete = true

	if err := f.uc.DeleteByRequest(context.Background(), f.req.ID, "u2"); err != nil {
		t.Fatalf("DeleteByRequest: %v", err)
	}

	for _, e := range []*entities.ResponseExample{a, b} {
		row := f.repo.rows[e.ID]
		if !row.IsDelete || row.Version != e.Version+1 || row.UpdatedBy != "u2" {
			t.Errorf("example %s = %+v, want soft-deleted", e.ID, row)
		}
	}
	if f.repo.rows[other.ID].IsDelete {
		t.Error("another request's example was deleted")
	}
}

func TestDeleteByRequest_ReturnsRepoError(t *testing.T) {
	f := newFixture()
	f.seed(t)
	f.repo.updateErr = errors.New("disk full")

	if err := f.uc.DeleteByRequest(context.Background(), f.req.ID, "u1"); err == nil {
		t.Fatal("want error")
	}
}

func TestMoveToWorkspace_CopiesAndTombstonesOriginals(t *testing.T) {
	f := newFixture()
	a := f.seed(t)
	b := f.seed(t)
	target := &entities.Collection{ID: uuid.New(), WorkspaceID: wsB}
	f.collections[target.ID] = target
	f.requests[f.req.ID].CollectionID = target.ID
	before := time.Now().Add(-time.Second)

	if err := f.uc.MoveToWorkspace(context.Background(), f.req.ID, wsB, "u2"); err != nil {
		t.Fatalf("MoveToWorkspace: %v", err)
	}

	originals := map[uuid.UUID]*entities.ResponseExample{a.ID: a, b.ID: b}
	var copies []*entities.ResponseExample
	for id, row := range f.repo.rows {
		if orig, ok := originals[id]; ok {
			if !row.IsDelete || row.WorkspaceID != wsA || row.Version != orig.Version+1 || row.UpdatedBy != "u2" {
				t.Errorf("original %s = %+v, want soft-deleted in wsA", id, row)
			}
			continue
		}
		copies = append(copies, row)
	}
	if len(copies) != 2 {
		t.Fatalf("copies = %d, want 2", len(copies))
	}
	orders := map[int]bool{}
	for _, c := range copies {
		if c.IsDelete || c.WorkspaceID != wsB || c.RequestID != f.req.ID || c.Version != 1 {
			t.Errorf("copy = %+v", c)
		}
		if c.CreatedBy != "u2" || c.CreatedAt.Before(before) {
			t.Errorf("copy audit fields = %s/%s", c.CreatedBy, c.CreatedAt)
		}
		orders[c.SortOrder] = true
	}
	if !orders[a.SortOrder] || !orders[b.SortOrder] {
		t.Errorf("copies lost sort order: %v", orders)
	}

	list, err := f.uc.ListByRequest(context.Background(), f.req.ID)
	if err != nil || len(list) != 2 {
		t.Fatalf("list after move = %d, err %v; want the two copies", len(list), err)
	}
}

func TestMoveToWorkspace_KeepsExamplesAlreadyThere(t *testing.T) {
	f := newFixture()
	e := f.seed(t)

	if err := f.uc.MoveToWorkspace(context.Background(), f.req.ID, wsA, "u2"); err != nil {
		t.Fatalf("MoveToWorkspace: %v", err)
	}

	if len(f.repo.rows) != 1 || f.repo.rows[e.ID].IsDelete || f.repo.rows[e.ID].Version != e.Version {
		t.Fatalf("rows = %+v, want the example untouched", f.repo.rows)
	}
}
