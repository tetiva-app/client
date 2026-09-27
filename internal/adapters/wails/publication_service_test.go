package wails

import (
	"context"
	"database/sql"
	"encoding/base64"
	"errors"
	"fmt"
	"math/rand/v2"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	publicationv1 "github.com/tetiva-app/proto/go/gophercourier/publication/v1"

	"github.com/tetiva-app/client/internal/adapters/wails/dto"
	"github.com/tetiva-app/client/internal/domain/entities"
	"github.com/tetiva-app/client/internal/domain/usecase/collection"
	"github.com/tetiva-app/client/internal/domain/usecase/environment"
	"github.com/tetiva-app/client/internal/domain/usecase/example"
	"github.com/tetiva-app/client/internal/domain/usecase/request"
	"github.com/tetiva-app/client/internal/domain/usecase/workspace"
	"github.com/tetiva-app/client/internal/infrastructure/repository/sqlite"
	syncsvc "github.com/tetiva-app/client/internal/infrastructure/sync"
)

type fakePublicationServer struct {
	mu           sync.Mutex
	supports     bool
	supportsErr  error
	publishErr   error
	unpublishErr error
	getErr       error
	hideSettings bool
	pubs         map[string]*publicationv1.Publication
	publishes    []*publicationv1.PublishRequest
	unpublishes  []string
	seq          int
	features     map[bool][]string
	featuresErr  error
	planAsks     []bool
	supportAsks  int
	gets         [][]string
}

func newFakePublicationServer() *fakePublicationServer {
	return &fakePublicationServer{supports: true, pubs: map[string]*publicationv1.Publication{}}
}

func (f *fakePublicationServer) ServerSupportsPublish(context.Context) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.supportAsks++
	return f.supports, f.supportsErr
}

func (f *fakePublicationServer) Publish(_ context.Context, req *publicationv1.PublishRequest) (*publicationv1.Publication, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.publishes = append(f.publishes, proto.Clone(req).(*publicationv1.PublishRequest))
	if f.publishErr != nil {
		return nil, f.publishErr
	}
	prev := f.pubs[req.GetCollectionId()]
	id, slug, revision := "", "", int32(1)
	if prev != nil && prev.GetStatus() == publicationv1.PublicationStatus_PUBLICATION_STATUS_ACTIVE {
		id, slug, revision = prev.GetId(), prev.GetSlug(), prev.GetRevision()+1
	} else {
		f.seq++
		id, slug = fmt.Sprintf("pub-%d", f.seq), fmt.Sprintf("petstore-%d", f.seq)
	}
	pub := publicationv1.Publication_builder{
		Id: id, Slug: slug, PublicUrl: "https://share.tetiva.app/" + slug,
		CollectionId: req.GetCollectionId(), WorkspaceId: req.GetWorkspaceId(), Title: req.GetTitle(),
		Visibility: req.GetVisibility(), Status: publicationv1.PublicationStatus_PUBLICATION_STATUS_ACTIVE,
		Revision: revision, ContentHash: req.GetContentHash(), UpdatedAt: timestamppb.Now(),
		CanManage: true, Settings: proto.Clone(req.GetSettings()).(*publicationv1.PublishSettings),
	}.Build()
	f.pubs[req.GetCollectionId()] = pub
	return proto.Clone(pub).(*publicationv1.Publication), nil
}

func (f *fakePublicationServer) Unpublish(_ context.Context, publicationID string) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.unpublishes = append(f.unpublishes, publicationID)
	if f.unpublishErr != nil {
		return false, f.unpublishErr
	}
	for _, p := range f.pubs {
		if p.GetId() != publicationID {
			continue
		}
		if p.GetStatus() == publicationv1.PublicationStatus_PUBLICATION_STATUS_REVOKED {
			return true, nil
		}
		p.SetStatus(publicationv1.PublicationStatus_PUBLICATION_STATUS_REVOKED)
		return false, nil
	}
	return false, status.Error(codes.NotFound, "publication not found")
}

func (f *fakePublicationServer) GetPublications(_ context.Context, ids []string) ([]*publicationv1.Publication, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.gets = append(f.gets, append([]string(nil), ids...))
	if f.getErr != nil {
		return nil, f.getErr
	}
	var out []*publicationv1.Publication
	for _, id := range ids {
		p, ok := f.pubs[id]
		if !ok {
			continue
		}
		c := proto.Clone(p).(*publicationv1.Publication)
		if f.hideSettings {
			c.SetCanManage(false)
			c.ClearSettings()
		}
		out = append(out, c)
	}
	return out, nil
}

func (f *fakePublicationServer) PlanFeatures(_ context.Context, personal bool) ([]string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.planAsks = append(f.planAsks, personal)
	return f.features[personal], f.featuresErr
}

func (f *fakePublicationServer) set(fn func(f *fakePublicationServer)) {
	f.mu.Lock()
	defer f.mu.Unlock()
	fn(f)
}

func (f *fakePublicationServer) lastPublish(t *testing.T) *publicationv1.PublishRequest {
	t.Helper()
	f.mu.Lock()
	defer f.mu.Unlock()
	require.NotEmpty(t, f.publishes, "no Publish reached the server")
	return f.publishes[len(f.publishes)-1]
}

func (f *fakePublicationServer) unpublished() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.unpublishes...)
}

type pubHarness struct {
	db          *sql.DB
	remote      *fakePublicationServer
	repo        *sqlite.PublicationRepo
	config      sqlite.SyncConfigRepository
	collections collection.Usecase
	requests    request.Usecase
	examples    example.Usecase
	envs        environment.Usecase
	workspaces  workspace.Usecase
	svc         *PublicationService
	timers      *fakeTimers
	localWS     uuid.UUID
}

func newPubHarness(t *testing.T) *pubHarness {
	t.Helper()
	db := setupSyncTestDB(t)
	h := &pubHarness{db: db, remote: newFakePublicationServer(), localWS: uuid.MustParse(seededWorkspaceID)}
	h.wire(db)
	return h
}

func (h *pubHarness) wire(db *sql.DB) {
	h.repo = sqlite.NewPublicationRepo(db)
	tx := sqlite.NewTxRunner(db)
	colRepo := sqlite.NewCollectionRepo(db)
	reqRepo := sqlite.NewRequestRepo(db)
	h.config = sqlite.NewSyncConfigRepo(db)
	h.collections = collection.NewUsecase(colRepo, nil, tx, h.repo)
	h.requests = request.NewUsecase(reqRepo, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, tx)
	h.examples = example.NewUsecase(sqlite.NewResponseExampleRepo(db), reqRepo, colRepo)
	h.envs = environment.NewUsecase(sqlite.NewEnvironmentRepo(db), sqlite.NewVariableRepo(db))
	h.workspaces = workspace.NewUsecase(sqlite.NewWorkspaceRepo(db), tx, h.repo)
	engine := syncsvc.NewSyncEngine(syncsvc.NewSyncAuthManager(h.config), sqlite.NewSyncQueueRepo(db), h.config, db,
		colRepo, reqRepo, sqlite.NewEnvironmentRepo(db), sqlite.NewVariableRepo(db), sqlite.NewResponseExampleRepo(db), nil)
	h.svc = NewPublicationService(h.collections, h.requests, h.examples, h.envs, h.workspaces, h.remote, h.config, h.repo, engine)
	h.svc.spawn = func(func()) {}
	h.timers = &fakeTimers{}
	h.svc.after = h.timers.after
}

type fakeTimers struct {
	mu      sync.Mutex
	delays  []time.Duration
	fns     []func()
	stopped []bool
}

func (f *fakeTimers) after(d time.Duration, fn func()) func() bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	i := len(f.delays)
	f.delays, f.fns, f.stopped = append(f.delays, d), append(f.fns, fn), append(f.stopped, false)
	return func() bool {
		f.mu.Lock()
		defer f.mu.Unlock()
		was := !f.stopped[i]
		f.stopped[i] = true
		return was
	}
}

func (f *fakeTimers) scheduled() []time.Duration {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]time.Duration(nil), f.delays...)
}

func (f *fakeTimers) live() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	n := 0
	for _, s := range f.stopped {
		if !s {
			n++
		}
	}
	return n
}

func (f *fakeTimers) fire(t *testing.T) {
	t.Helper()
	f.mu.Lock()
	i := len(f.fns) - 1
	require.GreaterOrEqual(t, i, 0, "no retry scheduled")
	require.False(t, f.stopped[i], "the latest retry was cancelled")
	f.stopped[i] = true
	fn := f.fns[i]
	f.mu.Unlock()
	fn()
}

func (h *pubHarness) signIn(t *testing.T, serverURL, email string) {
	t.Helper()
	ctx := context.Background()
	cfg, err := h.config.GetOrCreate(ctx)
	require.NoError(t, err)
	cfg.ServerURL, cfg.UserEmail, cfg.Enabled = serverURL, email, true
	require.NoError(t, h.config.Update(ctx, cfg))
}

func (h *pubHarness) signOut(t *testing.T) {
	t.Helper()
	ctx := context.Background()
	cfg, err := h.config.GetOrCreate(ctx)
	require.NoError(t, err)
	cfg.Enabled = false
	require.NoError(t, h.config.Update(ctx, cfg))
}

func (h *pubHarness) workspace(t *testing.T, name string, remoteID *string) uuid.UUID {
	t.Helper()
	now := time.Now().Truncate(time.Second)
	ws := &entities.Workspace{
		ID: uuid.New(), Name: name, RemoteWorkspaceID: remoteID, Version: 1,
		CreatedBy: defaultUserID, CreatedAt: now, UpdatedBy: defaultUserID, UpdatedAt: now,
	}
	require.NoError(t, sqlite.NewWorkspaceRepo(h.db).Create(context.Background(), ws))
	return ws.ID
}

func (h *pubHarness) collection(t *testing.T, workspaceID uuid.UUID, name string, parentID *uuid.UUID) *entities.Collection {
	t.Helper()
	c, err := h.collections.Create(context.Background(), collection.Create{Name: name, ParentID: parentID},
		collection.CreateOpt{UserID: defaultUserID, WorkspaceID: workspaceID})
	require.NoError(t, err)
	return c
}

func (h *pubHarness) request(t *testing.T, collectionID uuid.UUID, name string, mutate func(*request.Create)) *entities.Request {
	t.Helper()
	in := request.Create{
		CollectionID: collectionID, Name: name, Protocol: entities.ProtocolHTTP, Method: entities.MethodGET,
		URL: "https://api.example.com/users", BodyType: entities.BodyTypeNone, AuthType: entities.AuthTypeInherit,
	}
	if mutate != nil {
		mutate(&in)
	}
	r, err := h.requests.Create(context.Background(), in, request.CreateOpt{UserID: defaultUserID})
	require.NoError(t, err)
	return r
}

func (h *pubHarness) editURL(t *testing.T, r *entities.Request, url string) {
	t.Helper()
	repo := sqlite.NewRequestRepo(h.db)
	stored, err := repo.GetByID(context.Background(), r.ID)
	require.NoError(t, err)
	stored.URL = url
	stored.Version++
	require.NoError(t, repo.Update(context.Background(), stored))
}

func (h *pubHarness) environment(t *testing.T, workspaceID uuid.UUID, name string, vars map[string]string) *entities.Environment {
	t.Helper()
	ctx := context.Background()
	env, err := h.envs.Create(ctx, environment.Create{Name: name}, environment.CreateOpt{UserID: defaultUserID, WorkspaceID: workspaceID})
	require.NoError(t, err)
	for k, v := range vars {
		_, err := h.envs.AddVariable(ctx, environment.AddVariable{EnvironmentID: env.ID, Key: k, Value: v}, environment.AddVariableOpt{UserID: defaultUserID})
		require.NoError(t, err)
	}
	return env
}

func previewRequestFor(c *entities.Collection) dto.PublishPreviewRequest {
	return dto.PublishPreviewRequest{CollectionID: c.ID.String(), WorkspaceID: c.WorkspaceID.String()}
}

func (h *pubHarness) preview(t *testing.T, req dto.PublishPreviewRequest) dto.PublishPreview {
	t.Helper()
	res := h.svc.Preview(req)
	require.Nil(t, res.Error, "Preview: %+v", res.Error)
	return res.Data
}

func (h *pubHarness) publishRequest(t *testing.T, preview dto.PublishPreviewRequest) dto.PublishRequest {
	t.Helper()
	p := h.preview(t, preview)
	return dto.PublishRequest{
		CollectionID: preview.CollectionID, WorkspaceID: preview.WorkspaceID, EnvironmentID: preview.EnvironmentID,
		IncludeScripts: preview.IncludeScripts, PublishAsIs: preview.PublishAsIs,
		Visibility: "public", Locale: "en", AcknowledgedWarnings: len(p.Warnings) > 0, PreviewHash: p.PreviewHash,
	}
}

func (h *pubHarness) publish(t *testing.T, c *entities.Collection) dto.PublicationStatus {
	t.Helper()
	res := h.svc.Publish(h.publishRequest(t, previewRequestFor(c)))
	require.Nil(t, res.Error, "Publish: %+v", res.Error)
	return res.Data
}

func (h *pubHarness) status(t *testing.T, c *entities.Collection) dto.PublicationStatus {
	t.Helper()
	res := h.svc.Status(dto.PublicationStatusRequest{CollectionID: c.ID.String()})
	require.Nil(t, res.Error, "Status: %+v", res.Error)
	return res.Data
}

const testOwner = testServerURL + "\na@b.c"

func (h *pubHarness) row(t *testing.T, id uuid.UUID) *sqlite.PublicationRow {
	t.Helper()
	return h.rowOf(t, testOwner, id)
}

func (h *pubHarness) rowOf(t *testing.T, owner string, id uuid.UUID) *sqlite.PublicationRow {
	t.Helper()
	row, err := h.repo.Get(context.Background(), owner, id)
	require.NoError(t, err)
	return row
}

func (h *pubHarness) deleteCollection(t *testing.T, c *entities.Collection) {
	t.Helper()
	stored, err := h.collections.GetByID(context.Background(), c.ID)
	require.NoError(t, err)
	require.NoError(t, h.collections.Delete(context.Background(), collection.DeleteOpt{CollectionID: c.ID, UserID: defaultUserID, Version: stored.Version}))
}

func (h *pubHarness) deleteWorkspace(t *testing.T, id uuid.UUID) {
	t.Helper()
	ws, err := h.workspaces.GetByID(context.Background(), id)
	require.NoError(t, err)
	require.NoError(t, h.workspaces.Delete(context.Background(), workspace.DeleteOpt{WorkspaceID: id, UserID: defaultUserID, Version: ws.Version}))
}

func strPtr(s string) *string { return &s }

func TestPublicationStatus_Availability(t *testing.T) {
	t.Run("signed out", func(t *testing.T) {
		h := newPubHarness(t)
		root := h.collection(t, h.localWS, "API", nil)

		st := h.status(t, root)
		assert.False(t, st.Available)
		assert.Equal(t, "not_logged_in", st.ReasonUnavailable)
		assert.False(t, st.Published)
	})

	t.Run("expired session", func(t *testing.T) {
		h := newPubHarness(t)
		h.signIn(t, testServerURL, "a@b.c")
		h.remote.set(func(f *fakePublicationServer) { f.supportsErr = fmt.Errorf("get token: %w", syncsvc.ErrAuthExpired) })
		root := h.collection(t, h.localWS, "API", nil)

		assert.Equal(t, "not_logged_in", h.status(t, root).ReasonUnavailable)
	})

	t.Run("server without publishing", func(t *testing.T) {
		h := newPubHarness(t)
		h.signIn(t, testServerURL, "a@b.c")
		h.remote.set(func(f *fakePublicationServer) { f.supports = false })
		root := h.collection(t, h.localWS, "API", nil)

		st := h.status(t, root)
		assert.False(t, st.Available)
		assert.Equal(t, "no_capability", st.ReasonUnavailable)
	})

	t.Run("offline shows the cached row as stale", func(t *testing.T) {
		h := newPubHarness(t)
		h.signIn(t, testServerURL, "a@b.c")
		root := h.collection(t, h.localWS, "API", nil)
		h.request(t, root.ID, "List users", nil)
		published := h.publish(t, root)

		h.remote.set(func(f *fakePublicationServer) { f.supportsErr = status.Error(codes.Unavailable, "no route to host") })
		st := h.status(t, root)
		assert.False(t, st.Available)
		assert.Equal(t, "offline", st.ReasonUnavailable)
		assert.True(t, st.Stale)
		assert.True(t, st.Published)
		assert.Equal(t, published.Slug, st.Slug)
		assert.Equal(t, "unknown", st.HasChanges)
	})

	t.Run("a failed refresh is offline too", func(t *testing.T) {
		h := newPubHarness(t)
		h.signIn(t, testServerURL, "a@b.c")
		root := h.collection(t, h.localWS, "API", nil)
		h.publish(t, root)

		h.remote.set(func(f *fakePublicationServer) { f.getErr = status.Error(codes.DeadlineExceeded, "slow") })
		st := h.status(t, root)
		assert.Equal(t, "offline", st.ReasonUnavailable)
		assert.True(t, st.Stale)
	})

	t.Run("nested collection with a publication", func(t *testing.T) {
		h := newPubHarness(t)
		h.signIn(t, testServerURL, "a@b.c")
		root := h.collection(t, h.localWS, "API", nil)
		folder := h.collection(t, h.localWS, "Folder", nil)
		h.publish(t, root)
		moved, err := h.collections.Move(context.Background(), collection.MoveOpt{CollectionID: root.ID, TargetParentID: &folder.ID, UserID: defaultUserID, Version: root.Version})
		require.NoError(t, err)

		st := h.status(t, moved)
		assert.False(t, st.Available)
		assert.Equal(t, "not_root", st.ReasonUnavailable)
		assert.True(t, st.Published, "the panel still offers Unpublish")
		assert.True(t, st.CanManage)
	})
}

func TestPublicationStatus_RefreshesFromServer(t *testing.T) {
	h := newPubHarness(t)
	h.signIn(t, testServerURL, "a@b.c")
	root := h.collection(t, h.localWS, "API", nil)
	h.request(t, root.ID, "List users", nil)

	st := h.status(t, root)
	assert.True(t, st.Available)
	assert.False(t, st.Published)
	assert.Nil(t, h.row(t, root.ID))

	h.publish(t, root)
	h.remote.set(func(f *fakePublicationServer) {
		p := f.pubs[root.ID.String()]
		p.SetViewsTotal(12)
		p.SetImportsTotal(3)
		p.SetDownloadsTotal(1)
		p.SetBlocked(true)
		p.SetBlockedReason("spam")
		p.SetBadge(true)
	})
	st = h.status(t, root)
	assert.True(t, st.Published)
	assert.Equal(t, dto.PublicationCounters{Views: 12, Imports: 3, Downloads: 1}, st.Counters)
	assert.True(t, st.Blocked)
	assert.Equal(t, "spam", st.BlockedReason)
	assert.True(t, st.Badge)
	assert.Equal(t, 1, st.Revision)
	assert.NotEmpty(t, st.UpdatedAt)
	assert.Equal(t, "public", st.Visibility)

	h.remote.set(func(f *fakePublicationServer) { delete(f.pubs, root.ID.String()) })
	st = h.status(t, root)
	assert.False(t, st.Published)
	assert.Nil(t, h.row(t, root.ID), "a publication the server no longer has leaves the cache")
}

func TestPublicationStatus_WithoutManageRight(t *testing.T) {
	h := newPubHarness(t)
	h.signIn(t, testServerURL, "a@b.c")
	root := h.collection(t, h.localWS, "API", nil)
	h.publish(t, root)
	h.remote.set(func(f *fakePublicationServer) { f.hideSettings = true })

	st := h.status(t, root)
	assert.True(t, st.Published)
	assert.False(t, st.CanManage)
	assert.Nil(t, st.Settings)
	assert.Equal(t, "unknown", st.HasChanges)
}

func TestPublicationStatus_EndsWithABackgroundPass(t *testing.T) {
	h := newPubHarness(t)
	var spawned int
	h.svc.spawn = func(func()) { spawned++ }
	root := h.collection(t, h.localWS, "API", nil)

	h.status(t, root)
	assert.Equal(t, 1, spawned)
}

func TestPublicationPreview_ReportAndSizes(t *testing.T) {
	h := newPubHarness(t)
	root := h.collection(t, h.localWS, "API", nil)
	folder := h.collection(t, h.localWS, "Users", &root.ID)
	r := h.request(t, folder.ID, "Get user", func(c *request.Create) {
		c.Headers = []entities.HeaderItem{{Key: "X-Api-Key", Value: "{{tenant}}", Enabled: true}}
	})
	_, err := h.examples.Create(context.Background(), example.Create{RequestID: r.ID, Name: "OK", StatusCode: 200, Body: `{"id":1}`, Protocol: entities.ProtocolHTTP},
		example.CreateOpt{UserID: defaultUserID})
	require.NoError(t, err)
	env := h.environment(t, h.localWS, "Prod", map[string]string{"tenant": "acme", "baseUrl": "https://api.example.com"})

	req := previewRequestFor(root)
	req.EnvironmentID = env.ID.String()
	p := h.preview(t, req)
	assert.Equal(t, 1, p.Folders)
	assert.Equal(t, 1, p.Requests)
	assert.Equal(t, 1, p.Examples)
	assert.Equal(t, []string{"baseUrl"}, p.PublishedVars)
	require.Len(t, p.HiddenVars, 1)
	assert.Equal(t, "tenant", p.HiddenVars[0].Key)
	assert.Equal(t, "referenced", p.HiddenVars[0].Reason)
	assert.True(t, p.HiddenVars[0].Overridable)
	assert.Positive(t, p.SizeBytes)
	assert.Positive(t, p.GzipBytes)
	assert.Equal(t, 8<<20, p.SizeLimitBytes)
	assert.Equal(t, 7<<19, p.GzipLimitBytes)
	assert.Len(t, p.PreviewHash, 64)
	assert.NotNil(t, p.Warnings)
	assert.NotNil(t, p.Errors)
	assert.NotNil(t, p.IgnoredOverrides)
	assert.NotNil(t, p.LargestExamples)
	assert.Empty(t, p.LargestExamples, "examples are listed only over the limit")

	assert.Equal(t, p.PreviewHash, h.preview(t, req).PreviewHash, "the same content previews to the same hash")

	req.PublishAsIs = []string{p.HiddenVars[0].Selector}
	overridden := h.preview(t, req)
	assert.NotEqual(t, p.PreviewHash, overridden.PreviewHash, "an override changes what was reviewed")
	assert.True(t, overridden.HiddenVars[0].Overridden)

	h.editURL(t, r, "https://api.example.com/users/{{id}}")
	assert.NotEqual(t, overridden.PreviewHash, h.preview(t, req).PreviewHash, "an edited request needs a new review")
}

func TestPublicationPreview_RefusesForeignScope(t *testing.T) {
	h := newPubHarness(t)
	root := h.collection(t, h.localWS, "API", nil)
	nested := h.collection(t, h.localWS, "Nested", &root.ID)
	otherWS := h.workspace(t, "Other", nil)
	foreignEnv := h.environment(t, otherWS, "Foreign", nil)

	cases := map[string]dto.PublishPreviewRequest{
		"environment of another workspace": {CollectionID: root.ID.String(), WorkspaceID: h.localWS.String(), EnvironmentID: foreignEnv.ID.String()},
		"collection of another workspace":  {CollectionID: root.ID.String(), WorkspaceID: otherWS.String()},
		"nested collection":                {CollectionID: nested.ID.String(), WorkspaceID: h.localWS.String()},
		"unknown environment":              {CollectionID: root.ID.String(), WorkspaceID: h.localWS.String(), EnvironmentID: uuid.NewString()},
	}
	for name, req := range cases {
		t.Run(name, func(t *testing.T) {
			res := h.svc.Preview(req)
			require.NotNil(t, res.Error)
			assert.Equal(t, ErrCodeValidation, res.Error.Code)
		})
	}
}

func TestPublicationPublish_SendsTheSnapshotAndCachesTheResult(t *testing.T) {
	h := newPubHarness(t)
	h.signIn(t, testServerURL, "a@b.c")
	root := h.collection(t, h.localWS, "Petstore", nil)
	h.request(t, root.ID, "Get user", func(c *request.Create) {
		c.Headers = []entities.HeaderItem{{Key: "X-Api-Key", Value: "{{tenant}}", Enabled: true}}
	})
	env := h.environment(t, h.localWS, "Prod", map[string]string{"tenant": "acme"})

	req := previewRequestFor(root)
	req.EnvironmentID = env.ID.String()
	req.IncludeScripts = true
	hidden := h.preview(t, req).HiddenVars[0].Selector
	req.PublishAsIs = []string{hidden, "d93148ea5eb1/cookie/0"}
	pr := h.publishRequest(t, req)
	pr.Locale = "ru"
	pr.Visibility = "password"
	pr.Password = strPtr("correct horse")

	res := h.svc.Publish(pr)
	require.Nil(t, res.Error, "%+v", res.Error)
	assert.True(t, res.Data.Published)
	assert.True(t, res.Data.CanManage)
	assert.Equal(t, "no", res.Data.HasChanges)

	sent := h.remote.lastPublish(t)
	assert.Equal(t, root.ID.String(), sent.GetCollectionId())
	assert.Empty(t, sent.GetWorkspaceId(), "a local workspace publishes with an empty workspace_id")
	assert.Equal(t, "ru", sent.GetLocale())
	assert.Equal(t, "Petstore", sent.GetTitle())
	assert.Equal(t, publicationv1.Visibility_VISIBILITY_PASSWORD, sent.GetVisibility())
	assert.Equal(t, "correct horse", sent.GetPassword())
	assert.Len(t, sent.GetContentHash(), 64)
	assert.NotEmpty(t, sent.GetSnapshotGz())
	assert.Equal(t, env.ID.String(), sent.GetSettings().GetEnvironmentId())
	assert.Equal(t, "Prod", sent.GetSettings().GetEnvironmentName())
	assert.True(t, sent.GetSettings().GetIncludeScripts())
	assert.Equal(t, []string{hidden}, sent.GetSettings().GetPublishAsIs(), "only accepted selectors reach the server")

	row := h.row(t, root.ID)
	require.NotNil(t, row)
	assert.Equal(t, testOwner, row.OwnerKey)
	assert.Equal(t, h.localWS, row.WorkspaceID)
	assert.Equal(t, "pub-1", row.PublicationID)
	assert.Equal(t, sent.GetContentHash(), row.ContentHash)
	assert.False(t, row.PendingUnpublish)
	require.NotNil(t, row.Settings)
	assert.Equal(t, []string{hidden}, row.Settings.PublishAsIs)
}

func TestPublicationPublish_CloudWorkspaceSendsItsRemoteID(t *testing.T) {
	h := newPubHarness(t)
	h.signIn(t, testServerURL, "a@b.c")
	cloudWS := h.workspace(t, "Team", strPtr("remote-ws-1"))
	root := h.collection(t, cloudWS, "Team API", nil)

	h.publish(t, root)
	assert.Equal(t, "remote-ws-1", h.remote.lastPublish(t).GetWorkspaceId())
	assert.Equal(t, cloudWS, h.row(t, root.ID).WorkspaceID, "the cache keeps the local workspace id")
}

func TestPublicationPublish_TitleIsTheScrubbedName(t *testing.T) {
	h := newPubHarness(t)
	h.signIn(t, testServerURL, "a@b.c")
	root := h.collection(t, h.localWS, "API ghp_abcdefghijklmnopqrstuvwxyz0123456789", nil)

	h.publish(t, root)

	title := h.remote.lastPublish(t).GetTitle()
	assert.Equal(t, "API <redacted>", title)
}

func TestPublicationPublish_EnvironmentNameIsTheScrubbedName(t *testing.T) {
	h := newPubHarness(t)
	h.signIn(t, testServerURL, "a@b.c")
	root := h.collection(t, h.localWS, "API", nil)
	env := h.environment(t, h.localWS, "Prod ghp_abcdefghijklmnopqrstuvwxyz0123456789", map[string]string{"baseUrl": "https://api.example.com"})

	req := previewRequestFor(root)
	req.EnvironmentID = env.ID.String()
	res := h.svc.Publish(h.publishRequest(t, req))
	require.Nil(t, res.Error, "Publish: %+v", res.Error)

	name := h.remote.lastPublish(t).GetSettings().GetEnvironmentName()
	assert.Equal(t, "Prod <redacted>", name)
}

func TestPublicationPublish_Refusals(t *testing.T) {
	t.Run("blocking errors", func(t *testing.T) {
		h := newPubHarness(t)
		h.signIn(t, testServerURL, "a@b.c")
		root := h.collection(t, h.localWS, "API", nil)
		h.request(t, root.ID, "Bad header", func(c *request.Create) {
			c.Headers = []entities.HeaderItem{{Key: "X Api", Value: "1", Enabled: true}}
		})
		pr := h.publishRequest(t, previewRequestFor(root))
		errs := h.preview(t, previewRequestFor(root)).Errors
		require.NotEmpty(t, errs)
		assert.Equal(t, "header_name_invalid", errs[0].Code)
		assert.Equal(t, map[string]string{"name": "X Api"}, errs[0].Params)

		res := h.svc.Publish(pr)
		require.NotNil(t, res.Error)
		assert.Equal(t, ErrCodeValidation, res.Error.Code)
		assert.Empty(t, h.remote.publishes)
	})

	t.Run("stale preview", func(t *testing.T) {
		h := newPubHarness(t)
		h.signIn(t, testServerURL, "a@b.c")
		root := h.collection(t, h.localWS, "API", nil)
		r := h.request(t, root.ID, "List users", nil)
		pr := h.publishRequest(t, previewRequestFor(root))
		h.editURL(t, r, "https://api.example.com/v2/users")

		res := h.svc.Publish(pr)
		require.NotNil(t, res.Error)
		assert.Equal(t, ErrCodeConflict, res.Error.Code)
		assert.Empty(t, res.Error.Reason)
		assert.Empty(t, h.remote.publishes)
	})

	t.Run("unacknowledged warnings", func(t *testing.T) {
		h := newPubHarness(t)
		h.signIn(t, testServerURL, "a@b.c")
		root := h.collection(t, h.localWS, "API", nil)
		h.request(t, root.ID, "Leaky", func(c *request.Create) {
			c.Method = entities.MethodPOST
			c.BodyType = entities.BodyTypeJSON
			c.Body = `{"note":"AKIAABCDEFGHIJKLMNOP"}`
		})
		pr := h.publishRequest(t, previewRequestFor(root))
		require.True(t, pr.AcknowledgedWarnings, "the fixture must raise a scan warning")
		pr.AcknowledgedWarnings = false

		res := h.svc.Publish(pr)
		require.NotNil(t, res.Error)
		assert.Equal(t, ErrCodeValidation, res.Error.Code)
		assert.Contains(t, res.Error.Fields, "acknowledgedWarnings")
		assert.Empty(t, h.remote.publishes)
	})

	t.Run("bad fields", func(t *testing.T) {
		h := newPubHarness(t)
		h.signIn(t, testServerURL, "a@b.c")
		root := h.collection(t, h.localWS, "API", nil)
		base := h.publishRequest(t, previewRequestFor(root))

		for name, mutate := range map[string]func(*dto.PublishRequest){
			"locale":            func(r *dto.PublishRequest) { r.Locale = "de" },
			"visibility":        func(r *dto.PublishRequest) { r.Visibility = "org_members" },
			"short password":    func(r *dto.PublishRequest) { r.Visibility, r.Password = "password", strPtr("1234567") },
			"long password":     func(r *dto.PublishRequest) { r.Visibility, r.Password = "password", strPtr(strings.Repeat("p", 73)) },
			"collection id":     func(r *dto.PublishRequest) { r.CollectionID = "nope" },
			"environment scope": func(r *dto.PublishRequest) { r.EnvironmentID = uuid.NewString() },
		} {
			pr := base
			mutate(&pr)
			res := h.svc.Publish(pr)
			require.NotNil(t, res.Error, name)
			assert.Equal(t, ErrCodeValidation, res.Error.Code, name)
		}
		assert.Empty(t, h.remote.publishes)
	})

	t.Run("signed out", func(t *testing.T) {
		h := newPubHarness(t)
		root := h.collection(t, h.localWS, "API", nil)

		res := h.svc.Publish(h.publishRequest(t, previewRequestFor(root)))
		require.NotNil(t, res.Error)
		assert.Equal(t, ErrCodeNotConnected, res.Error.Code)
		assert.Equal(t, ReasonServerUnreachable, res.Error.Reason)
	})

	t.Run("network failures read as an unreachable server", func(t *testing.T) {
		for _, err := range []error{
			status.Error(codes.Unavailable, "connection refused"),
			status.Error(codes.DeadlineExceeded, "context deadline exceeded"),
			context.DeadlineExceeded,
		} {
			h := newPubHarness(t)
			h.signIn(t, testServerURL, "a@b.c")
			root := h.collection(t, h.localWS, "API", nil)
			h.remote.set(func(f *fakePublicationServer) { f.publishErr = fmt.Errorf("publish rpc: %w", err) })

			res := h.svc.Publish(h.publishRequest(t, previewRequestFor(root)))
			require.NotNil(t, res.Error, "%v", err)
			assert.Equal(t, ReasonServerUnreachable, res.Error.Reason, "%v", err)
		}
	})

	t.Run("a refusal is not a network failure", func(t *testing.T) {
		h := newPubHarness(t)
		h.signIn(t, testServerURL, "a@b.c")
		root := h.collection(t, h.localWS, "API", nil)
		h.remote.set(func(f *fakePublicationServer) {
			f.publishErr = fmt.Errorf("publish rpc: %w", status.Error(codes.PermissionDenied, "not yours"))
		})

		res := h.svc.Publish(h.publishRequest(t, previewRequestFor(root)))
		require.NotNil(t, res.Error)
		assert.Empty(t, res.Error.Reason)
	})

	t.Run("server reason reaches the result", func(t *testing.T) {
		h := newPubHarness(t)
		h.signIn(t, testServerURL, "a@b.c")
		root := h.collection(t, h.localWS, "API", nil)
		st, err := status.New(codes.ResourceExhausted, "quota").WithDetails(&errdetails.ErrorInfo{Reason: "PUBLISH_QUOTA_EXCEEDED"})
		require.NoError(t, err)
		h.remote.set(func(f *fakePublicationServer) { f.publishErr = fmt.Errorf("publish rpc: %w", st.Err()) })

		res := h.svc.Publish(h.publishRequest(t, previewRequestFor(root)))
		require.NotNil(t, res.Error)
		assert.Equal(t, "PUBLISH_QUOTA_EXCEEDED", res.Error.Reason)
		assert.Nil(t, h.row(t, root.ID))
	})
}

func TestPublicationPublish_TooManyOverrides(t *testing.T) {
	h := newPubHarness(t)
	h.signIn(t, testServerURL, "a@b.c")
	root := h.collection(t, h.localWS, "API", nil)
	vars := make(map[string]string, 501)
	for i := range 501 {
		vars[fmt.Sprintf("token%d", i)] = "v"
	}
	env := h.environment(t, h.localWS, "Many", vars)

	req := previewRequestFor(root)
	req.EnvironmentID = env.ID.String()
	for _, v := range h.preview(t, req).HiddenVars {
		require.True(t, v.Overridable)
		req.PublishAsIs = append(req.PublishAsIs, v.Selector)
	}
	require.Len(t, req.PublishAsIs, 501)

	res := h.svc.Publish(h.publishRequest(t, req))
	require.NotNil(t, res.Error)
	assert.Equal(t, ErrCodeValidation, res.Error.Code)
	assert.Contains(t, res.Error.Fields, "publishAsIs")
	assert.Empty(t, h.remote.publishes)
}

func TestPublicationTooLarge_ListsTheLargestExamples(t *testing.T) {
	h := newPubHarness(t)
	h.signIn(t, testServerURL, "a@b.c")
	root := h.collection(t, h.localWS, "API", nil)
	r := h.request(t, root.ID, "Download", nil)
	rng := rand.New(rand.NewPCG(1, 2))
	exampleRepo := sqlite.NewResponseExampleRepo(h.db)
	now := time.Now().Truncate(time.Second)
	for i := range 6 {
		raw := make([]byte, 750_000-i*10_000)
		for j := range raw {
			raw[j] = byte(rng.UintN(256))
		}
		require.NoError(t, exampleRepo.Create(context.Background(), &entities.ResponseExample{
			ID: uuid.New(), RequestID: r.ID, WorkspaceID: h.localWS, Name: fmt.Sprintf("Blob %d", i), StatusCode: 200,
			Headers: []entities.HeaderItem{}, Body: base64.StdEncoding.EncodeToString(raw), Protocol: entities.ProtocolHTTP,
			SortOrder: i, Version: 1, CreatedBy: defaultUserID, CreatedAt: now, UpdatedBy: defaultUserID, UpdatedAt: now,
		}))
	}

	pr := h.publishRequest(t, previewRequestFor(root))
	p := h.preview(t, previewRequestFor(root))
	require.Greater(t, p.GzipBytes, p.GzipLimitBytes, "the fixture must exceed the compressed limit")
	require.Len(t, p.LargestExamples, 5, "the preview names what to remove")
	for i, e := range p.LargestExamples {
		assert.Equal(t, fmt.Sprintf("Download / Blob %d", i), e.Path)
		assert.Greater(t, e.Bytes, base64.StdEncoding.EncodedLen(750_000-(i+1)*10_000))
	}

	res := h.svc.Publish(pr)
	require.NotNil(t, res.Error)
	assert.Equal(t, ErrCodeValidation, res.Error.Code)
	msg := res.Error.Fields["snapshot"]
	assert.Contains(t, msg, "largest examples")
	for i := range 5 {
		assert.Contains(t, msg, fmt.Sprintf("Download / Blob %d", i))
	}
	assert.NotContains(t, msg, "Blob 5", "only the five largest are listed")
	assert.Less(t, strings.Index(msg, "Blob 0"), strings.Index(msg, "Blob 4"), "largest first")
	assert.Empty(t, h.remote.publishes)
}

func TestPublicationHasChanges_FromServerSettings(t *testing.T) {
	h := newPubHarness(t)
	h.signIn(t, testServerURL, "a@b.c")
	root := h.collection(t, h.localWS, "API", nil)
	r := h.request(t, root.ID, "Get user", func(c *request.Create) {
		c.Headers = []entities.HeaderItem{{Key: "X-Api-Key", Value: "{{tenant}}", Enabled: true}}
	})
	env := h.environment(t, h.localWS, "Prod", map[string]string{"tenant": "acme"})

	req := previewRequestFor(root)
	req.EnvironmentID = env.ID.String()
	req.PublishAsIs = []string{h.preview(t, req).HiddenVars[0].Selector}
	res := h.svc.Publish(h.publishRequest(t, req))
	require.Nil(t, res.Error, "%+v", res.Error)

	require.NoError(t, h.repo.Delete(context.Background(), testOwner, root.ID), "a colleague's device has no cache")
	st := h.status(t, root)
	assert.Equal(t, "no", st.HasChanges, "the override comes back from the server")
	require.NotNil(t, st.Settings)
	assert.Equal(t, req.PublishAsIs, st.Settings.PublishAsIs)
	assert.Equal(t, env.ID.String(), st.Settings.EnvironmentID)
	assert.False(t, st.Settings.EnvironmentMissing)

	h.editURL(t, r, "https://api.example.com/users/{{id}}")
	assert.Equal(t, "yes", h.status(t, root).HasChanges)
}

func TestPublicationHasChanges_WithoutEnvironment(t *testing.T) {
	h := newPubHarness(t)
	h.signIn(t, testServerURL, "a@b.c")
	root := h.collection(t, h.localWS, "API", nil)
	h.request(t, root.ID, "List users", nil)
	h.publish(t, root)

	st := h.status(t, root)
	assert.Equal(t, "no", st.HasChanges)
	require.NotNil(t, st.Settings)
	assert.Empty(t, st.Settings.EnvironmentID)
}

func TestPublicationHasChanges_DeletedEnvironment(t *testing.T) {
	h := newPubHarness(t)
	h.signIn(t, testServerURL, "a@b.c")
	root := h.collection(t, h.localWS, "API", nil)
	env := h.environment(t, h.localWS, "Prod", map[string]string{"baseUrl": "https://api.example.com"})
	req := previewRequestFor(root)
	req.EnvironmentID = env.ID.String()
	require.Nil(t, h.svc.Publish(h.publishRequest(t, req)).Error)
	require.NoError(t, h.envs.Delete(context.Background(), environment.DeleteOpt{EnvironmentID: env.ID, UserID: defaultUserID, Version: env.Version}))

	st := h.status(t, root)
	assert.Equal(t, "unknown", st.HasChanges)
	require.NotNil(t, st.Settings)
	assert.True(t, st.Settings.EnvironmentMissing)
}

func TestPublicationUnpublish(t *testing.T) {
	h := newPubHarness(t)
	h.signIn(t, testServerURL, "a@b.c")
	root := h.collection(t, h.localWS, "API", nil)
	h.publish(t, root)

	h.remote.set(func(f *fakePublicationServer) { f.unpublishErr = status.Error(codes.Unavailable, "offline") })
	res := h.svc.Unpublish(dto.UnpublishRequest{CollectionID: root.ID.String()})
	require.NotNil(t, res.Error, "a manual unpublish reports the failure")
	assert.Equal(t, ReasonServerUnreachable, res.Error.Reason)
	assert.False(t, h.row(t, root.ID).PendingUnpublish, "and leaves no deferred intent")

	h.remote.set(func(f *fakePublicationServer) { f.unpublishErr = nil })
	res = h.svc.Unpublish(dto.UnpublishRequest{CollectionID: root.ID.String()})
	require.Nil(t, res.Error, "%+v", res.Error)
	assert.False(t, res.Data.Published)
	assert.Equal(t, "revoked", h.row(t, root.ID).Status)
	assert.Equal(t, []string{"pub-1", "pub-1"}, h.remote.unpublished())
}

func TestPublicationMarkVariableSecret(t *testing.T) {
	h := newPubHarness(t)
	env := h.environment(t, h.localWS, "Prod", map[string]string{"tenant": "acme"})
	other := h.environment(t, h.localWS, "Other", nil)
	vars, err := h.envs.ListVariables(context.Background(), env.ID)
	require.NoError(t, err)
	before := vars[0]

	res := h.svc.MarkVariableSecret(dto.MarkSecretRequest{EnvironmentID: other.ID.String(), VariableID: before.ID.String()})
	require.NotNil(t, res.Error)
	assert.Equal(t, ErrCodeNotFound, res.Error.Code)

	res = h.svc.MarkVariableSecret(dto.MarkSecretRequest{EnvironmentID: env.ID.String(), VariableID: before.ID.String()})
	require.Nil(t, res.Error, "%+v", res.Error)
	vars, err = h.envs.ListVariables(context.Background(), env.ID)
	require.NoError(t, err)
	after := vars[0]
	assert.True(t, after.IsSecret)
	assert.Equal(t, before.Key, after.Key)
	assert.Equal(t, before.Value, after.Value)
	assert.Equal(t, before.Enabled, after.Enabled)
	assert.Equal(t, before.Version+1, after.Version)
}

func TestPublicationDelete_LocalCollectionUnpublishesAfterRestart(t *testing.T) {
	h := newPubHarness(t)
	h.signIn(t, testServerURL, "a@b.c")
	root := h.collection(t, h.localWS, "API", nil)
	h.publish(t, root)
	h.remote.set(func(f *fakePublicationServer) { f.unpublishErr = status.Error(codes.Unavailable, "offline") })

	res := NewCollectionService(h.collections).Delete(dto.DeleteCollectionRequest{ID: root.ID.String(), Version: root.Version})
	require.Nil(t, res.Error, "%+v", res.Error)
	assert.True(t, h.row(t, root.ID).PendingUnpublish, "the delete records the intent without the network")

	h.svc.ProcessPending(context.Background())
	assert.True(t, h.row(t, root.ID).PendingUnpublish, "a network failure keeps the intent")

	h.remote.set(func(f *fakePublicationServer) { f.unpublishErr = nil })
	h.wire(h.db)
	h.svc.ProcessPending(context.Background())
	assert.Equal(t, []string{"pub-1", "pub-1"}, h.remote.unpublished())
	assert.Nil(t, h.row(t, root.ID), "a dead collection leaves the cache once unpublished")
}

func TestPublicationDelete_MCPPathUsesTheSameUsecase(t *testing.T) {
	h := newPubHarness(t)
	h.signIn(t, testServerURL, "a@b.c")
	root := h.collection(t, h.localWS, "API", nil)
	h.publish(t, root)

	h.deleteCollection(t, root)
	assert.True(t, h.row(t, root.ID).PendingUnpublish)
}

func TestPublicationDelete_RolledBackDeleteLeavesNoIntent(t *testing.T) {
	h := newPubHarness(t)
	h.signIn(t, testServerURL, "a@b.c")
	root := h.collection(t, h.localWS, "API", nil)
	h.publish(t, root)

	boom := errors.New("disk full")
	uc := collection.NewUsecase(sqlite.NewCollectionRepo(h.db), nil, sqlite.NewTxRunner(h.db), failAfterMark{inner: h.repo, err: boom})
	err := uc.Delete(context.Background(), collection.DeleteOpt{CollectionID: root.ID, UserID: defaultUserID, Version: root.Version})
	require.ErrorIs(t, err, boom)

	assert.False(t, h.row(t, root.ID).PendingUnpublish, "the mark rolled back with the delete")
	alive, err := h.collections.GetByID(context.Background(), root.ID)
	require.NoError(t, err)
	assert.False(t, alive.IsDelete)
}

type failAfterMark struct {
	inner *sqlite.PublicationRepo
	err   error
}

func (f failAfterMark) MarkPendingUnpublish(ctx context.Context, ids []uuid.UUID) error {
	if err := f.inner.MarkPendingUnpublish(ctx, ids); err != nil {
		return err
	}
	return f.err
}

func TestPublicationDelete_WithoutRowRecordsNothing(t *testing.T) {
	h := newPubHarness(t)
	h.signIn(t, testServerURL, "a@b.c")
	root := h.collection(t, h.localWS, "API", nil)

	h.deleteCollection(t, root)
	assert.Nil(t, h.row(t, root.ID))
	h.svc.ProcessPending(context.Background())
	assert.Empty(t, h.remote.unpublished())
}

func TestPublicationDelete_LocalWorkspaceMarksEveryRow(t *testing.T) {
	h := newPubHarness(t)
	h.signIn(t, testServerURL, "a@b.c")
	ws := h.workspace(t, "Side project", nil)
	first := h.collection(t, ws, "First", nil)
	second := h.collection(t, ws, "Second", nil)
	h.publish(t, first)
	h.publish(t, second)

	h.deleteWorkspace(t, ws)
	assert.True(t, h.row(t, first.ID).PendingUnpublish)
	assert.True(t, h.row(t, second.ID).PendingUnpublish)

	h.svc.ProcessPending(context.Background())
	assert.ElementsMatch(t, []string{"pub-1", "pub-2"}, h.remote.unpublished())
	assert.Nil(t, h.row(t, first.ID))
	assert.Nil(t, h.row(t, second.ID))
}

func TestPublicationDelete_CloudWorkspaceIsLeftToTheServer(t *testing.T) {
	h := newPubHarness(t)
	h.signIn(t, testServerURL, "a@b.c")
	cloudWS := h.workspace(t, "Team", strPtr("remote-ws-1"))
	deleted := h.collection(t, cloudWS, "Deleted", nil)
	kept := h.collection(t, cloudWS, "Kept", nil)
	h.publish(t, deleted)
	h.publish(t, kept)

	h.deleteCollection(t, deleted)
	assert.False(t, h.row(t, deleted.ID).PendingUnpublish)
	h.svc.ProcessPending(context.Background())
	assert.Nil(t, h.row(t, deleted.ID), "reconciliation drops the row of a dead cloud collection")
	assert.NotNil(t, h.row(t, kept.ID))

	h.deleteWorkspace(t, cloudWS)
	assert.False(t, h.row(t, kept.ID).PendingUnpublish, "the local copy of a cloud workspace goes; the team page stays")
	h.svc.ProcessPending(context.Background())
	assert.Nil(t, h.row(t, kept.ID))

	assert.Empty(t, h.remote.unpublished())
}

func TestPublicationDelete_UnlinkedCloudWorkspaceLeavesTheTeamPage(t *testing.T) {
	h := newPubHarness(t)
	h.signIn(t, testServerURL, "a@b.c")
	cloudWS := h.workspace(t, "Team", strPtr("remote-ws-1"))
	kept := h.collection(t, cloudWS, "Kept", nil)
	h.publish(t, kept)
	require.True(t, h.row(t, kept.ID).Cloud)

	require.NoError(t, unlinkWorkspace(context.Background(), h.db, cloudWS.String()))
	h.deleteWorkspace(t, cloudWS)
	assert.False(t, h.row(t, kept.ID).PendingUnpublish)
	h.svc.ProcessPending(context.Background())
	assert.Empty(t, h.remote.unpublished())
	assert.Nil(t, h.row(t, kept.ID))
}

func TestPublicationDelete_CloudFlagFollowsTheServer(t *testing.T) {
	h := newPubHarness(t)
	h.signIn(t, testServerURL, "a@b.c")
	root := h.collection(t, h.localWS, "API", nil)
	h.publish(t, root)
	require.False(t, h.row(t, root.ID).Cloud)

	h.remote.set(func(f *fakePublicationServer) { f.pubs[root.ID.String()].SetWorkspaceId("remote-ws-1") })
	h.status(t, root)
	require.True(t, h.row(t, root.ID).Cloud, "a refresh takes the server's workspace")

	h.deleteCollection(t, root)
	assert.False(t, h.row(t, root.ID).PendingUnpublish)
	h.svc.ProcessPending(context.Background())
	assert.Empty(t, h.remote.unpublished(), "the server takes a dead cloud collection down itself")
	assert.Nil(t, h.row(t, root.ID))
}

func (h *pubHarness) linkAndMove(t *testing.T, ws uuid.UUID, root *entities.Collection) {
	t.Helper()
	_, err := h.db.Exec(`UPDATE workspaces SET remote_workspace_id = 'remote-ws-1' WHERE id = ?`, ws.String())
	require.NoError(t, err)
	h.remote.set(func(f *fakePublicationServer) { f.pubs[root.ID.String()].SetWorkspaceId("remote-ws-1") })
}

func TestPublicationDelete_LinkedWorkspaceCopyLeavesTheMovedPage(t *testing.T) {
	h := newPubHarness(t)
	h.signIn(t, testServerURL, "a@b.c")
	ws := h.workspace(t, "Side", nil)
	root := h.collection(t, ws, "API", nil)
	h.publish(t, root)
	require.False(t, h.row(t, root.ID).Cloud)
	h.linkAndMove(t, ws, root)

	h.deleteWorkspace(t, ws)
	assert.False(t, h.row(t, root.ID).PendingUnpublish)
	h.svc.ProcessPending(context.Background())
	assert.Empty(t, h.remote.unpublished(), "team page taken down by a stale cloud flag")
	assert.Nil(t, h.row(t, root.ID))
}

func TestPublicationDelete_CollectionOfALinkedWorkspaceIsLeftToTheServer(t *testing.T) {
	h := newPubHarness(t)
	h.signIn(t, testServerURL, "a@b.c")
	ws := h.workspace(t, "Side", nil)
	root := h.collection(t, ws, "API", nil)
	h.publish(t, root)
	h.linkAndMove(t, ws, root)

	h.deleteCollection(t, root)
	assert.False(t, h.row(t, root.ID).PendingUnpublish)
	h.svc.ProcessPending(context.Background())
	assert.Empty(t, h.remote.unpublished())
	assert.Nil(t, h.row(t, root.ID))
}

func TestPublicationReconcile_LocalWorkspaceDeletedDuringRefresh(t *testing.T) {
	h := newPubHarness(t)
	h.signIn(t, testServerURL, "a@b.c")
	ws := h.workspace(t, "Side project", nil)
	root := h.collection(t, ws, "API", nil)
	h.publish(t, root)
	h.svc.remote = &racingRemote{fakePublicationServer: h.remote, during: func() { h.deleteWorkspace(t, ws) }}

	h.status(t, root)
	require.False(t, h.row(t, root.ID).PendingUnpublish, "the refresh wrote back the row it read before the delete")

	h.svc.ProcessPending(context.Background())
	assert.Equal(t, []string{"pub-1"}, h.remote.unpublished())
	assert.Nil(t, h.row(t, root.ID))
}

func TestPublicationReconcile_LocalWorkspaceDeletedDuringPublish(t *testing.T) {
	for name, publishedBefore := range map[string]bool{"first publish": false, "republish": true} {
		t.Run(name, func(t *testing.T) {
			h := newPubHarness(t)
			h.signIn(t, testServerURL, "a@b.c")
			ws := h.workspace(t, "Side project", nil)
			root := h.collection(t, ws, "API", nil)
			if publishedBefore {
				h.publish(t, root)
			}
			h.svc.remote = &racingRemote{fakePublicationServer: h.remote, during: func() { h.deleteWorkspace(t, ws) }}

			h.publish(t, root)
			require.False(t, h.row(t, root.ID).PendingUnpublish)

			h.svc.ProcessPending(context.Background())
			assert.Equal(t, []string{"pub-1"}, h.remote.unpublished())
			assert.Nil(t, h.row(t, root.ID))
		})
	}
}

type racingRemote struct {
	*fakePublicationServer
	once   sync.Once
	during func()
}

func (r *racingRemote) GetPublications(ctx context.Context, ids []string) ([]*publicationv1.Publication, error) {
	r.once.Do(r.during)
	return r.fakePublicationServer.GetPublications(ctx, ids)
}

func (r *racingRemote) Publish(ctx context.Context, req *publicationv1.PublishRequest) (*publicationv1.Publication, error) {
	r.once.Do(r.during)
	return r.fakePublicationServer.Publish(ctx, req)
}

func TestPublicationReconcile_NestedRootIsUnpublished(t *testing.T) {
	h := newPubHarness(t)
	h.signIn(t, testServerURL, "a@b.c")
	root := h.collection(t, h.localWS, "API", nil)
	folder := h.collection(t, h.localWS, "Folder", nil)
	h.publish(t, root)
	_, err := h.collections.Move(context.Background(), collection.MoveOpt{CollectionID: root.ID, TargetParentID: &folder.ID, UserID: defaultUserID, Version: root.Version})
	require.NoError(t, err)

	h.svc.ProcessPending(context.Background())
	assert.Equal(t, []string{"pub-1"}, h.remote.unpublished())
	row := h.row(t, root.ID)
	require.NotNil(t, row, "the collection lives on, so does its row")
	assert.Equal(t, "revoked", row.Status)
	assert.False(t, row.PendingUnpublish)

	h.svc.ProcessPending(context.Background())
	assert.Len(t, h.remote.unpublished(), 1, "a revoked row is not unpublished again")
}

func TestPublicationPending_ServerRefusalSettlesTheIntent(t *testing.T) {
	for name, code := range map[string]codes.Code{"permission denied": codes.PermissionDenied, "not found": codes.NotFound} {
		t.Run(name, func(t *testing.T) {
			h := newPubHarness(t)
			h.signIn(t, testServerURL, "a@b.c")
			root := h.collection(t, h.localWS, "API", nil)
			h.publish(t, root)
			h.deleteCollection(t, root)
			h.remote.set(func(f *fakePublicationServer) {
				f.unpublishErr = fmt.Errorf("unpublish rpc: %w", status.Error(code, "no"))
			})

			h.svc.ProcessPending(context.Background())
			assert.Nil(t, h.row(t, root.ID))
		})
	}
}

func TestPublicationPending_WaitsForASession(t *testing.T) {
	h := newPubHarness(t)
	h.signIn(t, testServerURL, "a@b.c")
	root := h.collection(t, h.localWS, "API", nil)
	h.publish(t, root)
	h.deleteCollection(t, root)
	h.remote.set(func(f *fakePublicationServer) { f.unpublishErr = fmt.Errorf("unpublish: %w", ErrNotConnected) })

	h.svc.ProcessPending(context.Background())
	assert.True(t, h.row(t, root.ID).PendingUnpublish)
	assert.Zero(t, h.row(t, root.ID).UnpublishAttempts, "a network failure is not a refusal")
}

func TestPublicationPending_PermanentRefusalGivesUnpublishBack(t *testing.T) {
	h := newPubHarness(t)
	h.signIn(t, testServerURL, "a@b.c")
	root := h.collection(t, h.localWS, "API", nil)
	folder := h.collection(t, h.localWS, "Folder", nil)
	h.publish(t, root)
	moved, err := h.collections.Move(context.Background(), collection.MoveOpt{CollectionID: root.ID, TargetParentID: &folder.ID, UserID: defaultUserID, Version: root.Version})
	require.NoError(t, err)
	h.remote.set(func(f *fakePublicationServer) {
		f.unpublishErr = fmt.Errorf("unpublish rpc: %w", status.Error(codes.Unimplemented, "unknown service gophercourier.publication.v1.PublicationService"))
	})

	for attempt := 1; attempt <= 2; attempt++ {
		h.svc.ProcessPending(context.Background())
		st := h.status(t, moved)
		require.True(t, st.PendingUnpublish, "attempt %d", attempt)
		assert.Empty(t, st.UnpublishError, "attempt %d: the error waits for the third refusal", attempt)
	}
	h.svc.ProcessPending(context.Background())
	st := h.status(t, moved)
	assert.True(t, st.PendingUnpublish)
	assert.Equal(t, "unknown service gophercourier.publication.v1.PublicationService", st.UnpublishError)
	assert.Equal(t, 3, h.row(t, root.ID).UnpublishAttempts)

	h.remote.set(func(f *fakePublicationServer) { f.unpublishErr = nil })
	res := h.svc.Unpublish(dto.UnpublishRequest{CollectionID: root.ID.String()})
	require.Nil(t, res.Error, "%+v", res.Error)
	assert.False(t, res.Data.PendingUnpublish)
	assert.Empty(t, res.Data.UnpublishError)
	row := h.row(t, root.ID)
	assert.Equal(t, "revoked", row.Status)
	assert.Zero(t, row.UnpublishAttempts)
	assert.Empty(t, row.UnpublishError)
}

func TestPublicationPending_DeleteStartsAPass(t *testing.T) {
	for name, del := range map[string]func(h *pubHarness, root *entities.Collection, ws uuid.UUID){
		"collection": func(h *pubHarness, root *entities.Collection, _ uuid.UUID) { h.deleteCollection(t, root) },
		"workspace":  func(h *pubHarness, _ *entities.Collection, ws uuid.UUID) { h.deleteWorkspace(t, ws) },
	} {
		t.Run(name, func(t *testing.T) {
			h := newPubHarness(t)
			h.signIn(t, testServerURL, "a@b.c")
			ws := h.workspace(t, "Side project", nil)
			root := h.collection(t, ws, "API", nil)
			h.publish(t, root)
			ctx, cancel := context.WithCancel(context.Background())
			done := make(chan struct{})
			go func() {
				h.svc.WatchDeletes(ctx)
				close(done)
			}()
			t.Cleanup(func() {
				cancel()
				<-done
			})

			del(h, root, ws)
			assert.Eventually(t, func() bool {
				row, err := h.repo.Get(context.Background(), testOwner, root.ID)
				return err == nil && row == nil && len(h.remote.unpublished()) == 1
			}, 5*time.Second, 10*time.Millisecond, "the page comes down without waiting for a restart or a Status")
		})
	}
}

func TestPublicationPending_RetriesWithBackoffWhileOffline(t *testing.T) {
	h := newPubHarness(t)
	h.signIn(t, testServerURL, "a@b.c")
	root := h.collection(t, h.localWS, "API", nil)
	h.publish(t, root)
	h.deleteCollection(t, root)
	h.remote.set(func(f *fakePublicationServer) { f.unpublishErr = status.Error(codes.Unavailable, "no route to host") })

	h.svc.ProcessPending(context.Background())
	assert.Equal(t, []time.Duration{time.Minute}, h.timers.scheduled())
	h.timers.fire(t)
	h.timers.fire(t)
	h.timers.fire(t)
	assert.Equal(t, []time.Duration{time.Minute, 5 * time.Minute, 15 * time.Minute, 15 * time.Minute}, h.timers.scheduled())

	h.remote.set(func(f *fakePublicationServer) { f.unpublishErr = nil })
	h.timers.fire(t)
	assert.Len(t, h.timers.scheduled(), 4, "nothing is left to retry")
	assert.Nil(t, h.row(t, root.ID))
}

func TestPublicationPending_PassThatGetsThroughResetsTheBackoff(t *testing.T) {
	h := newPubHarness(t)
	h.signIn(t, testServerURL, "a@b.c")
	first := h.collection(t, h.localWS, "First", nil)
	second := h.collection(t, h.localWS, "Second", nil)
	h.publish(t, first)
	h.publish(t, second)
	offline := func(on bool) {
		h.remote.set(func(f *fakePublicationServer) {
			f.unpublishErr = nil
			if on {
				f.unpublishErr = status.Error(codes.Unavailable, "offline")
			}
		})
	}

	h.deleteCollection(t, first)
	offline(true)
	h.svc.ProcessPending(context.Background())
	h.timers.fire(t)
	require.Equal(t, []time.Duration{time.Minute, 5 * time.Minute}, h.timers.scheduled())

	offline(false)
	h.svc.ProcessPending(context.Background())
	assert.Zero(t, h.timers.live(), "a pass that got through cancels the waiting retry")

	h.deleteCollection(t, second)
	offline(true)
	h.svc.ProcessPending(context.Background())
	assert.Equal(t, time.Minute, h.timers.scheduled()[2], "the backoff starts over")
}

func TestPublicationPending_NoRetryWithoutANetworkFailure(t *testing.T) {
	t.Run("refused", func(t *testing.T) {
		h := newPubHarness(t)
		h.signIn(t, testServerURL, "a@b.c")
		root := h.collection(t, h.localWS, "API", nil)
		h.publish(t, root)
		h.deleteCollection(t, root)
		h.remote.set(func(f *fakePublicationServer) { f.unpublishErr = status.Error(codes.Unimplemented, "unknown service") })

		h.svc.ProcessPending(context.Background())
		assert.Empty(t, h.timers.scheduled())
	})

	t.Run("signed out", func(t *testing.T) {
		h := newPubHarness(t)
		h.signIn(t, testServerURL, "a@b.c")
		root := h.collection(t, h.localWS, "API", nil)
		h.publish(t, root)
		h.deleteCollection(t, root)
		h.signOut(t)

		h.svc.ProcessPending(context.Background())
		assert.Empty(t, h.timers.scheduled(), "signing in starts the next pass")
	})
}

func TestPublicationPending_StoppedWatcherDropsTheRetry(t *testing.T) {
	h := newPubHarness(t)
	h.signIn(t, testServerURL, "a@b.c")
	root := h.collection(t, h.localWS, "API", nil)
	h.publish(t, root)
	h.deleteCollection(t, root)
	h.remote.set(func(f *fakePublicationServer) { f.unpublishErr = status.Error(codes.Unavailable, "offline") })
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		h.svc.WatchDeletes(ctx)
		close(done)
	}()

	h.svc.ProcessPending(context.Background())
	require.Eventually(t, func() bool { return h.timers.live() == 1 }, 5*time.Second, 10*time.Millisecond)
	cancel()
	<-done
	assert.Zero(t, h.timers.live())

	n := len(h.timers.scheduled())
	h.svc.ProcessPending(context.Background())
	assert.Len(t, h.timers.scheduled(), n, "no retry is scheduled after shutdown")
}

func TestPublicationPending_RefusalWithoutAMessageStillShows(t *testing.T) {
	h := newPubHarness(t)
	h.signIn(t, testServerURL, "a@b.c")
	root := h.collection(t, h.localWS, "API", nil)
	folder := h.collection(t, h.localWS, "Folder", nil)
	h.publish(t, root)
	moved, err := h.collections.Move(context.Background(), collection.MoveOpt{CollectionID: root.ID, TargetParentID: &folder.ID, UserID: defaultUserID, Version: root.Version})
	require.NoError(t, err)
	h.remote.set(func(f *fakePublicationServer) {
		f.unpublishErr = fmt.Errorf("unpublish rpc: %w", status.Error(codes.Internal, ""))
	})

	for range 3 {
		h.svc.ProcessPending(context.Background())
	}
	assert.Equal(t, "Internal", h.status(t, moved).UnpublishError)
}

func TestPublicationOwner_OtherAccountCacheIsHidden(t *testing.T) {
	h := newPubHarness(t)
	h.signIn(t, testServerURL, "a@b.c")
	root := h.collection(t, h.localWS, "API", nil)
	h.publish(t, root)

	h.signIn(t, "self-hosted.example:443", "a@b.c")
	h.remote.set(func(f *fakePublicationServer) { f.getErr = status.Error(codes.Unavailable, "offline") })
	st := h.status(t, root)
	assert.False(t, st.Published, "another server's row is not shown")
	assert.False(t, st.Stale)
	h.svc.ProcessPending(context.Background())
	assert.NotNil(t, h.row(t, root.ID), "switching accounts keeps the cache")

	h.signIn(t, testServerURL, "a@b.c")
	st = h.status(t, root)
	assert.True(t, st.Published)
	assert.True(t, st.Stale)
}

func TestPublicationPending_DeleteUnderAnotherAccount(t *testing.T) {
	h := newPubHarness(t)
	h.signIn(t, testServerURL, "a@b.c")
	root := h.collection(t, h.localWS, "API", nil)
	h.publish(t, root)

	h.signIn(t, testServerURL, "b@b.c")
	h.status(t, h.collection(t, h.localWS, "Other", nil))
	h.deleteCollection(t, root)
	h.svc.ProcessPending(context.Background())
	assert.Empty(t, h.remote.unpublished(), "another account does not act on a's intent")

	h.signIn(t, testServerURL, "a@b.c")
	h.svc.ProcessPending(context.Background())
	assert.Equal(t, []string{"pub-1"}, h.remote.unpublished())
	assert.Nil(t, h.row(t, root.ID))
}

func TestPublicationPending_RootNestedUnderAnotherAccount(t *testing.T) {
	h := newPubHarness(t)
	h.signIn(t, testServerURL, "a@b.c")
	cloudWS := h.workspace(t, "Team", strPtr("remote-ws-1"))
	root := h.collection(t, cloudWS, "API", nil)
	folder := h.collection(t, cloudWS, "Folder", nil)
	h.publish(t, root)

	h.signIn(t, testServerURL, "b@b.c")
	h.status(t, folder)
	_, err := h.collections.Move(context.Background(), collection.MoveOpt{CollectionID: root.ID, TargetParentID: &folder.ID, UserID: defaultUserID, Version: root.Version})
	require.NoError(t, err)
	h.svc.ProcessPending(context.Background())
	assert.Empty(t, h.remote.unpublished())

	h.signIn(t, testServerURL, "a@b.c")
	h.svc.ProcessPending(context.Background())
	assert.Equal(t, []string{"pub-1"}, h.remote.unpublished())
}

func TestPublicationOwner_SignOutHidesTheCacheUntilTheAccountReturns(t *testing.T) {
	h := newPubHarness(t)
	h.signIn(t, testServerURL, "a@b.c")
	root := h.collection(t, h.localWS, "API", nil)
	h.publish(t, root)

	h.signOut(t)
	st := h.status(t, root)
	assert.False(t, st.Published, "a signed-out status reads no account's row")
	assert.False(t, st.Stale)
	h.svc.ProcessPending(context.Background())
	assert.NotNil(t, h.row(t, root.ID), "signing out keeps the cache")

	h.signIn(t, testServerURL, "a@b.c")
	h.remote.set(func(f *fakePublicationServer) { f.getErr = status.Error(codes.Unavailable, "offline") })
	st = h.status(t, root)
	assert.True(t, st.Published)
	assert.True(t, st.Stale)
}

func TestPublicationPending_SurvivesSignOut(t *testing.T) {
	h := newPubHarness(t)
	h.signIn(t, testServerURL, "a@b.c")
	root := h.collection(t, h.localWS, "API", nil)
	h.publish(t, root)
	h.remote.set(func(f *fakePublicationServer) { f.unpublishErr = status.Error(codes.Unavailable, "offline") })
	res := NewCollectionService(h.collections).Delete(dto.DeleteCollectionRequest{ID: root.ID.String(), Version: root.Version})
	require.Nil(t, res.Error)
	require.True(t, h.row(t, root.ID).PendingUnpublish)

	h.signOut(t)
	h.svc.ProcessPending(context.Background())

	h.remote.set(func(f *fakePublicationServer) { f.unpublishErr = nil })
	h.signIn(t, testServerURL, "a@b.c")
	h.svc.ProcessPending(context.Background())
	assert.Equal(t, []string{"pub-1"}, h.remote.unpublished())
	assert.Nil(t, h.row(t, root.ID))
}

func TestPublicationPending_WaitsForItsAccount(t *testing.T) {
	h := newPubHarness(t)
	h.signIn(t, testServerURL, "a@b.c")
	root := h.collection(t, h.localWS, "API", nil)
	h.publish(t, root)
	h.deleteCollection(t, root)

	h.signIn(t, testServerURL, "b@b.c")
	h.status(t, h.collection(t, h.localWS, "Other", nil))
	h.svc.ProcessPending(context.Background())
	assert.Empty(t, h.remote.unpublished(), "another account does not act on a's intent")
	require.NotNil(t, h.row(t, root.ID))
	assert.True(t, h.row(t, root.ID).PendingUnpublish)

	h.signIn(t, testServerURL, "a@b.c")
	h.svc.ProcessPending(context.Background())
	assert.Equal(t, []string{"pub-1"}, h.remote.unpublished())
	assert.Nil(t, h.row(t, root.ID))
}

func TestPublicationPending_DeleteWhileSignedOut(t *testing.T) {
	h := newPubHarness(t)
	h.signIn(t, testServerURL, "a@b.c")
	root := h.collection(t, h.localWS, "API", nil)
	h.publish(t, root)

	h.signOut(t)
	h.status(t, root)
	h.svc.ProcessPending(context.Background())
	h.deleteCollection(t, root)
	require.NotNil(t, h.row(t, root.ID))
	assert.True(t, h.row(t, root.ID).PendingUnpublish, "the delete marks the cached row of the signed-out account")

	h.signIn(t, testServerURL, "a@b.c")
	h.svc.ProcessPending(context.Background())
	assert.Equal(t, []string{"pub-1"}, h.remote.unpublished())
	assert.Nil(t, h.row(t, root.ID))
}

func TestPublicationProcessPending_OneRunAtATime(t *testing.T) {
	h := newPubHarness(t)
	h.signIn(t, testServerURL, "a@b.c")
	root := h.collection(t, h.localWS, "API", nil)
	h.publish(t, root)
	h.deleteCollection(t, root)

	blocking := &blockingRemote{fakePublicationServer: h.remote, entered: make(chan struct{}), release: make(chan struct{})}
	h.svc.remote = blocking
	done := make(chan struct{})
	go func() {
		h.svc.ProcessPending(context.Background())
		close(done)
	}()
	<-blocking.entered

	returned := make(chan struct{})
	go func() {
		h.svc.ProcessPending(context.Background())
		close(returned)
	}()
	select {
	case <-returned:
	case <-time.After(time.Second):
		t.Fatal("a second call waited for the running pass")
	}
	close(blocking.release)
	<-done
	assert.Len(t, h.remote.unpublished(), 1, "the rerun found nothing left to unpublish")
}

type blockingRemote struct {
	*fakePublicationServer
	once    sync.Once
	entered chan struct{}
	release chan struct{}
}

func (b *blockingRemote) Unpublish(ctx context.Context, id string) (bool, error) {
	b.once.Do(func() {
		close(b.entered)
		<-b.release
	})
	return b.fakePublicationServer.Unpublish(ctx, id)
}

func TestPublicationUnpublish_SignedOut(t *testing.T) {
	h := newPubHarness(t)
	h.signIn(t, testServerURL, "a@b.c")
	root := h.collection(t, h.localWS, "API", nil)
	h.publish(t, root)
	h.signOut(t)

	res := h.svc.Unpublish(dto.UnpublishRequest{CollectionID: root.ID.String()})
	require.NotNil(t, res.Error)
	assert.Equal(t, ErrCodeNotConnected, res.Error.Code)
	assert.Empty(t, h.remote.unpublished())
}

func pendingKeys(t *testing.T, db *sql.DB, workspaceID uuid.UUID) []string {
	t.Helper()
	rows, err := db.Query(`SELECT entity_type || ':' || entity_id FROM sync_queue WHERE workspace_id = ? AND status = 'pending' ORDER BY id`, workspaceID.String())
	require.NoError(t, err)
	defer func() { _ = rows.Close() }()
	var out []string
	for rows.Next() {
		var key string
		require.NoError(t, rows.Scan(&key))
		out = append(out, key)
	}
	require.NoError(t, rows.Err())
	return out
}

func TestPublish_NotSyncedQueuesTheTreeTheServerNeverGot(t *testing.T) {
	h := newPubHarness(t)
	h.signIn(t, testServerURL, "a@b.c")
	ws := h.workspace(t, "Dogs", strPtr("remote-ws-1"))
	root := h.collection(t, ws, "Dogs API", nil)
	folder := h.collection(t, ws, "Breeds", &root.ID)
	req := h.request(t, folder.ID, "List breeds", nil)
	h.remote.set(func(f *fakePublicationServer) {
		f.publishErr = statusWithReason(t, codes.FailedPrecondition, "PUBLISH_COLLECTION_NOT_SYNCED")
	})

	res := h.svc.Publish(h.publishRequest(t, previewRequestFor(root)))

	require.NotNil(t, res.Error)
	assert.Equal(t, ReasonCollectionSyncing, res.Error.Reason)
	assert.Equal(t, []string{"collection:" + root.ID.String(), "collection:" + folder.ID.String(), "request:" + req.ID.String()},
		pendingKeys(t, h.db, ws))

	res = h.svc.Publish(h.publishRequest(t, previewRequestFor(root)))
	require.NotNil(t, res.Error)
	assert.Equal(t, "PUBLISH_COLLECTION_NOT_SYNCED", res.Error.Reason, "a tree already on its way leaves nothing to queue: sync just hasn't finished")
	assert.Len(t, pendingKeys(t, h.db, ws), 3)
}

func TestPublish_NotSyncedWithTheTreeConfirmedKeepsTheServerReason(t *testing.T) {
	h := newPubHarness(t)
	h.signIn(t, testServerURL, "a@b.c")
	ws := h.workspace(t, "Dogs", strPtr("remote-ws-1"))
	root := h.collection(t, ws, "Dogs API", nil)
	_, err := h.db.Exec(`UPDATE collections SET is_synced = 1 WHERE id = ?`, root.ID.String())
	require.NoError(t, err)
	h.remote.set(func(f *fakePublicationServer) {
		f.publishErr = statusWithReason(t, codes.FailedPrecondition, "PUBLISH_COLLECTION_NOT_SYNCED")
	})

	res := h.svc.Publish(h.publishRequest(t, previewRequestFor(root)))

	require.NotNil(t, res.Error)
	assert.Equal(t, "PUBLISH_COLLECTION_NOT_SYNCED", res.Error.Reason)
	assert.Empty(t, pendingKeys(t, h.db, ws))
}

func TestPublish_NotSyncedInALocalWorkspaceQueuesNothing(t *testing.T) {
	h := newPubHarness(t)
	h.signIn(t, testServerURL, "a@b.c")
	root := h.collection(t, h.localWS, "API", nil)
	h.remote.set(func(f *fakePublicationServer) {
		f.publishErr = statusWithReason(t, codes.FailedPrecondition, "PUBLISH_COLLECTION_NOT_SYNCED")
	})

	res := h.svc.Publish(h.publishRequest(t, previewRequestFor(root)))

	require.NotNil(t, res.Error)
	assert.Equal(t, "PUBLISH_COLLECTION_NOT_SYNCED", res.Error.Reason)
	assert.Empty(t, pendingKeys(t, h.db, h.localWS))
}

func TestPublicationPlan_LocksWhatThePlanLacks(t *testing.T) {
	h := newPubHarness(t)
	h.signIn(t, testServerURL, "a@b.c")
	cloudWS := h.workspace(t, "Team", strPtr("remote-ws-1"))
	local := h.collection(t, h.localWS, "Local API", nil)
	cloud := h.collection(t, cloudWS, "Team API", nil)
	h.remote.set(func(f *fakePublicationServer) {
		f.features = map[bool][]string{true: {"publish.no_badge"}, false: {"invites", "publish.unlisted", "publish.password"}}
	})

	res := h.svc.Plan(dto.PublishPlanRequest{CollectionID: local.ID.String()})
	require.Nil(t, res.Error)
	assert.Equal(t, dto.PublishPlan{Unlisted: false, Password: false}, res.Data, "a local collection counts against the personal org")

	res = h.svc.Plan(dto.PublishPlanRequest{CollectionID: cloud.ID.String()})
	require.Nil(t, res.Error)
	assert.Equal(t, dto.PublishPlan{Unlisted: true, Password: true}, res.Data)
	assert.Equal(t, []bool{true, false}, h.remote.planAsks)
}

func TestPublicationPlan_UnknownPlanIsAnError(t *testing.T) {
	h := newPubHarness(t)
	h.signIn(t, testServerURL, "a@b.c")
	root := h.collection(t, h.localWS, "API", nil)
	h.remote.set(func(f *fakePublicationServer) { f.featuresErr = status.Error(codes.Unimplemented, "unknown service") })

	res := h.svc.Plan(dto.PublishPlanRequest{CollectionID: root.ID.String()})

	require.NotNil(t, res.Error, "the dialog locks nothing it could not read")
}

func TestPublish_NotSyncedAgainAfterARoundThatMadeNoProgressKeepsTheServerReason(t *testing.T) {
	h := newPubHarness(t)
	h.signIn(t, testServerURL, "a@b.c")
	ws := h.workspace(t, "Dogs", strPtr("remote-ws-1"))
	root := h.collection(t, ws, "Dogs API", nil)
	h.collection(t, ws, "Breeds", &root.ID)
	h.remote.set(func(f *fakePublicationServer) {
		f.publishErr = statusWithReason(t, codes.FailedPrecondition, "PUBLISH_COLLECTION_NOT_SYNCED")
	})
	res := h.svc.Publish(h.publishRequest(t, previewRequestFor(root)))
	require.NotNil(t, res.Error)
	require.Equal(t, ReasonCollectionSyncing, res.Error.Reason)

	_, err := h.db.Exec(`DELETE FROM sync_queue WHERE workspace_id = ?`, ws.String())
	require.NoError(t, err)
	res = h.svc.Publish(h.publishRequest(t, previewRequestFor(root)))

	require.NotNil(t, res.Error)
	assert.Equal(t, "PUBLISH_COLLECTION_NOT_SYNCED", res.Error.Reason)
}

func TestPublish_NotSyncedAgainAfterARoundThatMadeProgressSaysSyncing(t *testing.T) {
	h := newPubHarness(t)
	h.signIn(t, testServerURL, "a@b.c")
	ws := h.workspace(t, "Dogs", strPtr("remote-ws-1"))
	root := h.collection(t, ws, "Dogs API", nil)
	h.collection(t, ws, "Breeds", &root.ID)
	h.remote.set(func(f *fakePublicationServer) {
		f.publishErr = statusWithReason(t, codes.FailedPrecondition, "PUBLISH_COLLECTION_NOT_SYNCED")
	})
	res := h.svc.Publish(h.publishRequest(t, previewRequestFor(root)))
	require.NotNil(t, res.Error)
	require.Equal(t, ReasonCollectionSyncing, res.Error.Reason)

	_, err := h.db.Exec(`UPDATE collections SET is_synced = 1 WHERE id = ?`, root.ID.String())
	require.NoError(t, err)
	_, err = h.db.Exec(`DELETE FROM sync_queue WHERE workspace_id = ?`, ws.String())
	require.NoError(t, err)
	res = h.svc.Publish(h.publishRequest(t, previewRequestFor(root)))

	require.NotNil(t, res.Error)
	assert.Equal(t, ReasonCollectionSyncing, res.Error.Reason)
}

func (h *pubHarness) list(t *testing.T, workspaceID uuid.UUID, remote bool) dto.PublicationList {
	t.Helper()
	res := h.svc.List(dto.PublicationListRequest{WorkspaceID: workspaceID.String(), Remote: remote})
	require.Nil(t, res.Error, "List: %+v", res.Error)
	return res.Data
}

func (f *fakePublicationServer) resetCalls() {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.gets, f.supportAsks = nil, 0
}

func (f *fakePublicationServer) getCalls() [][]string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([][]string(nil), f.gets...)
}

func listNames(l dto.PublicationList) []string {
	names := make([]string, 0, len(l.Items))
	for _, it := range l.Items {
		names = append(names, it.Name)
	}
	return names
}

func TestPublicationList_PublishedRootsByNameInOneCall(t *testing.T) {
	h := newPubHarness(t)
	h.signIn(t, testServerURL, "a@b.c")
	beta := h.collection(t, h.localWS, "beta API", nil)
	alpha := h.collection(t, h.localWS, "Alpha", nil)
	draft := h.collection(t, h.localWS, "Draft", nil)
	h.collection(t, h.localWS, "Folder", &beta.ID)
	h.collection(t, h.workspace(t, "Side project", nil), "Elsewhere", nil)
	h.publish(t, beta)
	h.publish(t, alpha)
	h.remote.resetCalls()

	list := h.list(t, h.localWS, true)

	assert.Empty(t, list.Reason)
	assert.Equal(t, []string{"Alpha", "beta API"}, listNames(list))
	assert.Equal(t, alpha.ID.String(), list.Items[0].CollectionID)
	assert.True(t, list.Items[0].Status.Published)
	assert.True(t, list.Items[0].Status.Available)
	assert.False(t, list.Items[0].Status.Stale)
	assert.Equal(t, "no", list.Items[0].Status.HasChanges)
	gets := h.remote.getCalls()
	require.Len(t, gets, 1)
	assert.ElementsMatch(t, []string{beta.ID.String(), alpha.ID.String(), draft.ID.String()}, gets[0])
}

func TestPublicationList_LocalRecountStaysOffTheNetwork(t *testing.T) {
	h := newPubHarness(t)
	h.signIn(t, testServerURL, "a@b.c")
	root := h.collection(t, h.localWS, "API", nil)
	r := h.request(t, root.ID, "List users", nil)
	h.publish(t, root)
	h.editURL(t, r, "https://api.example.com/v2/users")
	h.remote.resetCalls()

	list := h.list(t, h.localWS, false)

	assert.Empty(t, h.remote.getCalls())
	assert.Zero(t, h.remote.supportAsks)
	assert.Empty(t, list.Reason)
	require.Len(t, list.Items, 1)
	assert.Equal(t, "yes", list.Items[0].Status.HasChanges)
	assert.False(t, list.Items[0].Status.Stale)
}

func TestPublicationList_OfflineKeepsTheCacheAndWorksOutChanges(t *testing.T) {
	for name, fail := range map[string]func(f *fakePublicationServer){
		"server unreachable": func(f *fakePublicationServer) { f.supportsErr = status.Error(codes.Unavailable, "no route to host") },
		"refresh failed":     func(f *fakePublicationServer) { f.getErr = status.Error(codes.DeadlineExceeded, "slow") },
	} {
		t.Run(name, func(t *testing.T) {
			h := newPubHarness(t)
			h.signIn(t, testServerURL, "a@b.c")
			root := h.collection(t, h.localWS, "API", nil)
			r := h.request(t, root.ID, "List users", nil)
			h.publish(t, root)
			h.editURL(t, r, "https://api.example.com/v2/users")
			h.remote.set(fail)

			list := h.list(t, h.localWS, true)

			assert.Equal(t, "offline", list.Reason)
			require.Len(t, list.Items, 1)
			st := list.Items[0].Status
			assert.True(t, st.Published)
			assert.True(t, st.Stale)
			assert.False(t, st.Available)
			assert.Equal(t, "yes", st.HasChanges)
			assert.Equal(t, "unknown", h.status(t, root).HasChanges, "Status still says unknown offline")
		})
	}
}

func TestPublicationList_OnlyTheSignedInAccount(t *testing.T) {
	h := newPubHarness(t)
	h.signIn(t, testServerURL, "a@b.c")
	root := h.collection(t, h.localWS, "API", nil)
	h.publish(t, root)

	h.signOut(t)
	h.remote.resetCalls()
	list := h.list(t, h.localWS, true)
	assert.Equal(t, "not_logged_in", list.Reason)
	assert.Equal(t, []dto.PublicationListItem{}, list.Items)
	assert.Empty(t, h.remote.getCalls())
	assert.Zero(t, h.remote.supportAsks)

	h.signIn(t, testServerURL, "b@b.c")
	h.remote.set(func(f *fakePublicationServer) { f.getErr = status.Error(codes.Unavailable, "offline") })
	assert.Empty(t, h.list(t, h.localWS, true).Items, "a's cached row is not b's")
	assert.Empty(t, h.list(t, h.localWS, false).Items)
}

func TestPublicationList_PageFromAnotherDeviceHasUnknownChanges(t *testing.T) {
	h := newPubHarness(t)
	h.signIn(t, testServerURL, "a@b.c")
	root := h.collection(t, h.localWS, "API", nil)
	h.publish(t, root)
	require.NoError(t, h.repo.Delete(context.Background(), testOwner, root.ID), "the other device's cache is not here")
	h.remote.set(func(f *fakePublicationServer) {
		f.pubs[root.ID.String()].GetSettings().SetEnvironmentId(uuid.NewString())
	})

	list := h.list(t, h.localWS, true)

	require.Len(t, list.Items, 1)
	st := list.Items[0].Status
	assert.Equal(t, "unknown", st.HasChanges)
	require.NotNil(t, st.Settings)
	assert.True(t, st.Settings.EnvironmentMissing)
	assert.NotNil(t, h.row(t, root.ID), "the refresh cached the page")
}

func TestPublicationList_CollectionDeletedDuringTheRefreshIsSkipped(t *testing.T) {
	h := newPubHarness(t)
	h.signIn(t, testServerURL, "a@b.c")
	gone := h.collection(t, h.localWS, "Gone", nil)
	kept := h.collection(t, h.localWS, "Kept", nil)
	h.publish(t, gone)
	h.publish(t, kept)
	h.svc.remote = &racingRemote{fakePublicationServer: h.remote, during: func() { h.deleteCollection(t, gone) }}

	list := h.list(t, h.localWS, true)

	assert.Equal(t, []string{"Kept"}, listNames(list))
	assert.True(t, h.row(t, gone.ID).PendingUnpublish, "the refresh left the delete's mark alone")
}

func TestPublicationList_ARootThatCannotBeComparedStaysListed(t *testing.T) {
	h := newPubHarness(t)
	h.signIn(t, testServerURL, "a@b.c")
	broken := h.collection(t, h.localWS, "Broken", nil)
	r := h.request(t, broken.ID, "List users", nil)
	fine := h.collection(t, h.localWS, "Fine", nil)
	h.publish(t, broken)
	h.publish(t, fine)
	_, err := h.db.Exec(`UPDATE requests SET headers = '{}' WHERE id = ?`, r.ID.String())
	require.NoError(t, err)

	for _, remote := range []bool{true, false} {
		list := h.list(t, h.localWS, remote)

		assert.Equal(t, []string{"Broken", "Fine"}, listNames(list))
		assert.Equal(t, "unknown", list.Items[0].Status.HasChanges)
		assert.Equal(t, "no", list.Items[1].Status.HasChanges)
	}
}

func TestPublicationList_KeepsAPendingUnpublishAndDropsARevokedPage(t *testing.T) {
	h := newPubHarness(t)
	h.signIn(t, testServerURL, "a@b.c")
	pending := h.collection(t, h.localWS, "Pending", nil)
	revoked := h.collection(t, h.localWS, "Revoked", nil)
	h.publish(t, pending)
	h.publish(t, revoked)
	require.Nil(t, h.svc.Unpublish(dto.UnpublishRequest{CollectionID: revoked.ID.String()}).Error)
	row := h.row(t, pending.ID)
	row.PendingUnpublish = true
	require.NoError(t, h.repo.Upsert(context.Background(), row))

	list := h.list(t, h.localWS, false)

	assert.Equal(t, []string{"Pending"}, listNames(list))
	assert.True(t, list.Items[0].Status.PendingUnpublish)
}

func TestPublicationList_RejectsABadWorkspaceID(t *testing.T) {
	h := newPubHarness(t)
	res := h.svc.List(dto.PublicationListRequest{WorkspaceID: "nope"})
	require.NotNil(t, res.Error)
	assert.Equal(t, "validation", res.Error.Code)
}
