package wails

import (
	"bytes"
	"compress/gzip"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tetiva-app/client/internal/adapters/portability"
	"github.com/tetiva-app/client/internal/adapters/publication/snapshotjson"
	"github.com/tetiva-app/client/internal/adapters/wails/dto"
	"github.com/tetiva-app/client/internal/domain/entities"
	"github.com/tetiva-app/client/internal/domain/usecase/collection"
	"github.com/tetiva-app/client/internal/domain/usecase/environment"
	"github.com/tetiva-app/client/internal/domain/usecase/example"
	"github.com/tetiva-app/client/internal/domain/usecase/request"
	"github.com/tetiva-app/client/internal/infrastructure/publicapi"
	"github.com/tetiva-app/client/internal/infrastructure/repository/sqlite"
)

const importSlug = "petstore-api-k3f9x2qa"

var importWorkspace = "00000000-0000-4000-a000-000000000001"

func allProtocolsSnapshot(t *testing.T) []byte {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("..", "..", "..", "testdata", "snapshot", "all-protocols.json"))
	require.NoError(t, err)
	return raw
}

const threeRequestPostman = `{"info":{"name":"P","schema":"https://schema.getpostman.com/json/collection/v2.1.0/collection.json"},"item":[` +
	`{"name":"A","request":{"method":"GET","url":{"raw":"https://a.example.com"}}},` +
	`{"name":"B","request":{"method":"GET","url":{"raw":"https://b.example.com"}}},` +
	`{"name":"C","request":{"method":"GET","url":{"raw":"https://c.example.com"}}}]}`

const twoEntryHAR = `{"log":{"version":"1.2","entries":[` +
	`{"request":{"method":"GET","url":"https://a.example.com/x","headers":[]}},` +
	`{"request":{"method":"POST","url":"https://b.example.com/y","headers":[]}}]}}`

const hoppscotchCollection = `{"v":12,"name":"H","folders":[],"requests":[` +
	`{"v":"17","name":"A","method":"GET","endpoint":"<<host>>/a","preRequestScript":"pw.env.set('x','1')"},` +
	`{"v":"17","name":"B","method":"GET","endpoint":"<<host>>/b"}],` +
	`"variables":[{"key":"host","initialValue":"https://h.example.com","currentValue":"","secret":false}]}`

const postmanWithVariables = `{"info":{"name":"P","schema":"https://schema.getpostman.com/json/collection/v2.1.0/collection.json"},` +
	`"variable":[{"key":"host","value":"https://a.example.com"},{"key":"b","value":"2","disabled":true},{"key":"c","value":"3"}],` +
	`"item":[{"name":"A","request":{"method":"GET","url":{"raw":"{{host}}/a"}}}]}`

type failAfterRequests struct {
	request.Usecase
	n int
}

func (f *failAfterRequests) Create(ctx context.Context, in request.Create, opt request.CreateOpt) (*entities.Request, error) {
	if f.n == 0 {
		return nil, errors.New("disk full")
	}
	f.n--
	return f.Usecase.Create(ctx, in, opt)
}

type failAfterVariables struct {
	environment.Usecase
	n int
}

func (f *failAfterVariables) AddVariable(ctx context.Context, in environment.AddVariable, opt environment.AddVariableOpt) (*entities.Variable, error) {
	if f.n == 0 {
		return nil, errors.New("disk full")
	}
	f.n--
	return f.Usecase.AddVariable(ctx, in, opt)
}

type importFixture struct {
	db        *sql.DB
	svc       *PortabilityService
	snapshots atomic.Int32
}

type importFixtureOpts struct {
	wrapRequests     func(request.Usecase) request.Usecase
	wrapEnvironments func(environment.Usecase) environment.Usecase
}

func newImportFixture(t *testing.T, handler http.HandlerFunc, opts importFixtureOpts) *importFixture {
	t.Helper()
	f := &importFixture{db: setupSyncTestDB(t)}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/snapshot.json") {
			f.snapshots.Add(1)
		}
		handler(w, r)
	}))
	t.Cleanup(srv.Close)

	colRepo := sqlite.NewCollectionRepo(f.db)
	reqRepo := sqlite.NewRequestRepo(f.db)
	reqUC := request.NewUsecase(reqRepo, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
	if opts.wrapRequests != nil {
		reqUC = opts.wrapRequests(reqUC)
	}
	envUC := environment.NewUsecase(sqlite.NewEnvironmentRepo(f.db), sqlite.NewVariableRepo(f.db))
	if opts.wrapEnvironments != nil {
		envUC = opts.wrapEnvironments(envUC)
	}
	f.svc = NewPortabilityService(
		collection.NewUsecase(colRepo, nil, nil, nil),
		reqUC,
		envUC,
		example.NewUsecase(sqlite.NewResponseExampleRepo(f.db), reqRepo, colRepo),
		publicapi.New(srv.URL),
		sqlite.NewTxRunner(f.db),
	)
	return f
}

func (f *importFixture) count(t *testing.T, table string) int {
	t.Helper()
	var n int
	require.NoError(t, f.db.QueryRow("SELECT COUNT(*) FROM "+table).Scan(&n))
	return n
}

func serveSnapshot(t *testing.T, body []byte) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/snapshot.json") {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		var buf bytes.Buffer
		zw := gzip.NewWriter(&buf)
		_, err := zw.Write(body)
		require.NoError(t, err)
		require.NoError(t, zw.Close())
		w.Header().Set("Content-Encoding", "gzip")
		_, _ = w.Write(buf.Bytes())
	}
}

func statusOnly(code int) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(code) }
}

func requireReason[T any](t *testing.T, res Result[T], code, reason string) {
	t.Helper()
	require.NotNil(t, res.Error)
	assert.Equal(t, code, res.Error.Code)
	assert.Equal(t, reason, res.Error.Reason)
}

func TestLinkFetch_ConfirmImportsWithoutASecondDownload(t *testing.T) {
	var auth string
	snap := allProtocolsSnapshot(t)
	f := newImportFixture(t, func(w http.ResponseWriter, r *http.Request) {
		auth = r.Header.Get("Authorization")
		serveSnapshot(t, snap)(w, r)
	}, importFixtureOpts{})

	fetched := f.svc.LinkFetch(dto.LinkFetchRequest{Slug: importSlug, Token: "import-token"})
	require.Nil(t, fetched.Error)
	assert.Equal(t, "Bearer import-token", auth)
	assert.NotEmpty(t, fetched.Data.PreviewID)
	assert.Equal(t, "tetiva", fetched.Data.Preview.Format)
	assert.Equal(t, "Petstore API — демо", fetched.Data.Preview.Title)
	assert.Equal(t, 12, fetched.Data.Preview.Requests)
	assert.Len(t, fetched.Data.Preview.Scripts, 6)

	confirmed := f.svc.ImportConfirm(dto.ImportConfirmRequest{PreviewID: fetched.Data.PreviewID, WorkspaceID: importWorkspace})
	require.Nil(t, confirmed.Error)
	assert.NotEmpty(t, confirmed.Data.CollectionID)
	assert.Equal(t, 3, confirmed.Data.Folders)
	assert.Equal(t, 12, confirmed.Data.Requests)
	assert.Equal(t, 6, confirmed.Data.Examples)
	assert.Equal(t, []string{"prod"}, confirmed.Data.EnvironmentNames)
	assert.Equal(t, int32(1), f.snapshots.Load())
	assert.Equal(t, 4, f.count(t, "collections"))

	again := f.svc.ImportConfirm(dto.ImportConfirmRequest{PreviewID: fetched.Data.PreviewID, WorkspaceID: importWorkspace})
	requireReason(t, again, ErrCodeNotFound, portability.ReasonPreviewExpired)
	assert.Equal(t, int32(1), f.snapshots.Load())
}

func TestImportConfirm_UnknownAndExpiredPreview(t *testing.T) {
	f := newImportFixture(t, serveSnapshot(t, allProtocolsSnapshot(t)), importFixtureOpts{})
	now := time.Date(2026, 9, 25, 12, 0, 0, 0, time.UTC)
	f.svc.previews.now = func() time.Time { return now }

	unknown := f.svc.ImportConfirm(dto.ImportConfirmRequest{PreviewID: uuid.NewString(), WorkspaceID: importWorkspace})
	requireReason(t, unknown, ErrCodeNotFound, portability.ReasonPreviewExpired)

	fetched := f.svc.LinkFetch(dto.LinkFetchRequest{Slug: importSlug})
	require.Nil(t, fetched.Error)
	now = now.Add(10*time.Minute + time.Second)

	expired := f.svc.ImportConfirm(dto.ImportConfirmRequest{PreviewID: fetched.Data.PreviewID, WorkspaceID: importWorkspace})
	requireReason(t, expired, ErrCodeNotFound, portability.ReasonPreviewExpired)
	assert.Zero(t, f.count(t, "collections"))
}

func TestPreviewCache_FifthEntryEvictsTheOldest(t *testing.T) {
	c := newPreviewCache()
	var ids []string
	for i := range 5 {
		id, err := c.put([]byte{byte(i)}, "f.json")
		require.NoError(t, err)
		ids = append(ids, id)
	}

	_, ok := c.get(ids[0])
	assert.False(t, ok)
	for i, id := range ids[1:] {
		e, ok := c.get(id)
		require.True(t, ok)
		assert.Equal(t, []byte{byte(i + 1)}, e.data)
		assert.Equal(t, "f.json", e.fileName)
	}
}

func TestPreviewCache_TotalBudgetEvictsTheOldest(t *testing.T) {
	c := newPreviewCache()
	c.maxBytes, c.maxTotal = 10, 16
	first, err := c.put(make([]byte, 8), "")
	require.NoError(t, err)
	second, err := c.put(make([]byte, 8), "")
	require.NoError(t, err)

	third, err := c.put(make([]byte, 1), "")
	require.NoError(t, err)

	_, ok := c.get(first)
	assert.False(t, ok)
	for _, id := range []string{second, third} {
		_, ok := c.get(id)
		assert.True(t, ok)
	}
	_, err = c.put(make([]byte, 11), "")
	require.Error(t, err)
	_, ok = c.get(second)
	assert.True(t, ok)
}

func TestPreviewCache_DropsAnUnconfirmedPreviewWhenItExpires(t *testing.T) {
	c := newPreviewCache()
	now := time.Now()
	c.now = func() time.Time { return now }
	var expire func()
	var delay time.Duration
	c.afterFunc = func(d time.Duration, f func()) { delay, expire = d, f }

	_, err := c.put(make([]byte, 8), "big.har")
	require.NoError(t, err)
	require.NotNil(t, expire)
	assert.Equal(t, previewTTL, delay)

	now = now.Add(previewTTL)
	expire()

	assert.Empty(t, c.entries)
}

func TestPreviewCache_HoldsAFileAtTheImportLimit(t *testing.T) {
	c := newPreviewCache()

	_, err := c.put(make([]byte, portability.MaxFileBytes), "")
	require.NoError(t, err)
	_, err = c.put(make([]byte, portability.MaxFileBytes+1), "")
	require.Error(t, err)
}

func TestImportPreview_ConfirmByPreviewIDTakesTheFileOnce(t *testing.T) {
	f := newImportFixture(t, statusOnly(http.StatusNotFound), importFixtureOpts{})

	previewed := f.svc.ImportPreview(dto.ImportPreviewRequest{Content: threeRequestPostman})
	require.Nil(t, previewed.Error)
	assert.NotEmpty(t, previewed.Data.PreviewID)
	assert.Equal(t, 3, previewed.Data.Preview.Requests)

	confirmed := f.svc.ImportConfirm(dto.ImportConfirmRequest{PreviewID: previewed.Data.PreviewID, WorkspaceID: importWorkspace})
	require.Nil(t, confirmed.Error)
	assert.Equal(t, 3, confirmed.Data.Requests)
	assert.Equal(t, 3, f.count(t, "requests"))

	again := f.svc.ImportConfirm(dto.ImportConfirmRequest{PreviewID: previewed.Data.PreviewID, WorkspaceID: importWorkspace})
	requireReason(t, again, ErrCodeNotFound, portability.ReasonPreviewExpired)
	assert.Equal(t, 3, f.count(t, "requests"))
}

func TestImportPreview_ConfirmByPreviewIDNamesAHARAfterThePreviewedFile(t *testing.T) {
	f := newImportFixture(t, statusOnly(http.StatusNotFound), importFixtureOpts{})
	parent := f.svc.ImportConfirm(dto.ImportConfirmRequest{Content: threeRequestPostman, WorkspaceID: importWorkspace})
	require.Nil(t, parent.Error)

	previewed := f.svc.ImportPreview(dto.ImportPreviewRequest{Content: twoEntryHAR, FileName: "session.har"})
	require.Nil(t, previewed.Error)
	res := f.svc.ImportConfirm(dto.ImportConfirmRequest{
		PreviewID: previewed.Data.PreviewID, WorkspaceID: importWorkspace, ParentID: &parent.Data.CollectionID,
	})

	require.Nil(t, res.Error)
	var name string
	var parentID sql.NullString
	require.NoError(t, f.db.QueryRow(`SELECT name, parent_id FROM collections WHERE id = ?`, res.Data.CollectionID).Scan(&name, &parentID))
	assert.Equal(t, "session", name)
	assert.Equal(t, parent.Data.CollectionID, parentID.String)
}

func TestImportPreview_ConfirmsALargeFileByPreviewID(t *testing.T) {
	f := newImportFixture(t, statusOnly(http.StatusNotFound), importFixtureOpts{})
	big := threeRequestPostman + strings.Repeat(" ", 12<<20)

	previewed := f.svc.ImportPreview(dto.ImportPreviewRequest{Content: big})
	require.Nil(t, previewed.Error)
	res := f.svc.ImportConfirm(dto.ImportConfirmRequest{PreviewID: previewed.Data.PreviewID, WorkspaceID: importWorkspace})

	require.Nil(t, res.Error)
	assert.Equal(t, 3, res.Data.Requests)
}

func TestImportConfirm_RollsBackEveryFormat(t *testing.T) {
	cases := map[string]string{
		"snapshot":   string(allProtocolsSnapshot(t)),
		"postman":    threeRequestPostman,
		"har":        twoEntryHAR,
		"hoppscotch": hoppscotchCollection,
	}
	for name, content := range cases {
		t.Run(name, func(t *testing.T) {
			f := newImportFixture(t, statusOnly(http.StatusNotFound), importFixtureOpts{wrapRequests: func(u request.Usecase) request.Usecase {
				return &failAfterRequests{Usecase: u, n: 1}
			}})

			res := f.svc.ImportConfirm(dto.ImportConfirmRequest{Content: content, WorkspaceID: importWorkspace, IncludeScripts: true})

			require.NotNil(t, res.Error)
			assert.Contains(t, res.Error.Message, "disk full")
			assert.Zero(t, f.count(t, "collections"))
			assert.Zero(t, f.count(t, "requests"))
			assert.Zero(t, f.count(t, "response_examples"))
			assert.Equal(t, 1, f.count(t, "environments"), "only the seeded Default")
		})
	}
}

func TestImportEnvironment_RollsBackOnAFailedVariable(t *testing.T) {
	f := newImportFixture(t, statusOnly(http.StatusNotFound), importFixtureOpts{wrapEnvironments: func(u environment.Usecase) environment.Usecase {
		return &failAfterVariables{Usecase: u, n: 1}
	}})
	envs, vars := f.count(t, "environments"), f.count(t, "variables")

	res := f.svc.ImportEnvironment(dto.ImportEnvironmentRequest{
		Content:     `{"name":"Dev","values":[{"key":"a","value":"1"},{"key":"b","value":"2"},{"key":"c","value":"3"}]}`,
		WorkspaceID: importWorkspace,
	})

	require.NotNil(t, res.Error)
	assert.Contains(t, res.Error.Message, "disk full")
	assert.Equal(t, envs, f.count(t, "environments"))
	assert.Equal(t, vars, f.count(t, "variables"))
}

func TestImportEnvironment_ThroughTheService(t *testing.T) {
	f := newImportFixture(t, statusOnly(http.StatusNotFound), importFixtureOpts{})
	content := `{"name":"Dev","values":[{"key":"a","value":"1"},{"key":"b","value":"2","enabled":false},{"key":"","value":"3"}]}`

	first := f.svc.ImportEnvironment(dto.ImportEnvironmentRequest{Content: content, WorkspaceID: importWorkspace})
	second := f.svc.ImportEnvironment(dto.ImportEnvironmentRequest{Content: content, WorkspaceID: importWorkspace})

	require.Nil(t, first.Error)
	require.Nil(t, second.Error)
	assert.Equal(t, []string{"Dev"}, first.Data.EnvironmentNames)
	assert.Equal(t, []string{"Dev (2)"}, second.Data.EnvironmentNames)
	assert.Equal(t, 2, first.Data.VariablesCreated)
	assert.Equal(t, []string{"a variable without a name was skipped"}, warningTexts(first.Data.Warnings))
	assert.Equal(t, "variables_unnamed", first.Data.Warnings[0].Code)
	assert.Equal(t, map[string]string{"n": "1"}, first.Data.Warnings[0].Params)
	var enabled []bool
	rows, err := f.db.Query(`SELECT enabled FROM variables WHERE environment_id = ? ORDER BY rowid`, first.Data.EnvironmentID)
	require.NoError(t, err)
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var e bool
		require.NoError(t, rows.Scan(&e))
		enabled = append(enabled, e)
	}
	require.NoError(t, rows.Err())
	assert.Equal(t, []bool{true, false}, enabled)
	var active bool
	require.NoError(t, f.db.QueryRow(`SELECT is_active FROM environments WHERE id = ?`, second.Data.EnvironmentID).Scan(&active))
	assert.False(t, active)
}

func TestImportConfirm_PostmanVariablesRollBackWithTheCollection(t *testing.T) {
	f := newImportFixture(t, statusOnly(http.StatusNotFound), importFixtureOpts{wrapEnvironments: func(u environment.Usecase) environment.Usecase {
		return &failAfterVariables{Usecase: u, n: 1}
	}})
	envs, vars := f.count(t, "environments"), f.count(t, "variables")

	res := f.svc.ImportConfirm(dto.ImportConfirmRequest{Content: postmanWithVariables, WorkspaceID: importWorkspace})

	require.NotNil(t, res.Error)
	assert.Contains(t, res.Error.Message, "disk full")
	assert.Zero(t, f.count(t, "collections"))
	assert.Zero(t, f.count(t, "requests"))
	assert.Equal(t, envs, f.count(t, "environments"))
	assert.Equal(t, vars, f.count(t, "variables"))
}

type recordingTx struct {
	calls int
	err   error
}

type insideTxKey struct{}

func (r *recordingTx) Run(ctx context.Context, fn func(ctx context.Context) error) error {
	r.calls++
	r.err = fn(context.WithValue(ctx, insideTxKey{}, true))
	return r.err
}

func TestImportConfirm_OneTransactionForEitherFormat(t *testing.T) {
	cases := map[string]string{
		"snapshot": string(allProtocolsSnapshot(t)),
		"postman":  threeRequestPostman,
	}
	for name, content := range cases {
		t.Run(name, func(t *testing.T) {
			tx := &recordingTx{}
			collUC := &stubCollectionUsecase{
				createFn: func(ctx context.Context, in collection.Create, opt collection.CreateOpt) (*entities.Collection, error) {
					assert.Equal(t, true, ctx.Value(insideTxKey{}))
					return &entities.Collection{ID: uuid.New(), Name: in.Name, WorkspaceID: opt.WorkspaceID, Version: 1}, nil
				},
				listFn: func(context.Context, collection.ListOpt) ([]*entities.Collection, error) { return nil, nil },
				editFn: func(_ context.Context, in collection.Edit, opt collection.EditOpt) (*entities.Collection, error) {
					return &entities.Collection{ID: opt.CollectionID, Name: in.Name}, nil
				},
			}
			reqUC := &stubRequestUsecase{
				createFn: func(context.Context, request.Create, request.CreateOpt) (*entities.Request, error) {
					return nil, errors.New("disk full")
				},
			}
			svc := NewPortabilityService(collUC, reqUC, &stubEnvironmentUsecase{}, &stubExampleUsecase{}, nil, tx)

			res := svc.ImportConfirm(dto.ImportConfirmRequest{Content: content, WorkspaceID: importWorkspace})

			require.NotNil(t, res.Error)
			assert.Equal(t, 1, tx.calls)
			require.Error(t, tx.err)
			assert.Contains(t, tx.err.Error(), "disk full")
		})
	}
}

func TestImportConfirm_ExactlyOneSource(t *testing.T) {
	f := newImportFixture(t, statusOnly(http.StatusNotFound), importFixtureOpts{})

	for _, req := range []dto.ImportConfirmRequest{
		{WorkspaceID: importWorkspace},
		{PreviewID: uuid.NewString(), Content: threeRequestPostman, WorkspaceID: importWorkspace},
	} {
		res := f.svc.ImportConfirm(req)
		require.NotNil(t, res.Error)
		assert.Equal(t, ErrCodeValidation, res.Error.Code)
	}
}

func TestImportConfirm_PostmanKeepsParentAndScriptsFlag(t *testing.T) {
	f := newImportFixture(t, statusOnly(http.StatusNotFound), importFixtureOpts{})
	parent := f.svc.ImportConfirm(dto.ImportConfirmRequest{Content: threeRequestPostman, WorkspaceID: importWorkspace})
	require.Nil(t, parent.Error)
	withScript := strings.Replace(threeRequestPostman, `"item":[`,
		`"event":[{"listen":"prerequest","script":{"exec":["pm.environment.set('x','1')"]}}],"item":[`, 1)

	child := f.svc.ImportConfirm(dto.ImportConfirmRequest{Content: withScript, WorkspaceID: importWorkspace, ParentID: &parent.Data.CollectionID})
	require.Nil(t, child.Error)

	var parentID sql.NullString
	var pre string
	require.NoError(t, f.db.QueryRow(`SELECT parent_id, pre_script FROM collections WHERE id = ?`, child.Data.CollectionID).Scan(&parentID, &pre))
	assert.Equal(t, parent.Data.CollectionID, parentID.String)
	assert.Empty(t, pre)
	assert.Equal(t, 0, child.Data.Folders)
	assert.Equal(t, 3, child.Data.Requests)
}

func TestImportPreview_DetectsTheFormat(t *testing.T) {
	f := newImportFixture(t, statusOnly(http.StatusNotFound), importFixtureOpts{})

	postmanPreview := f.svc.ImportPreview(dto.ImportPreviewRequest{Content: threeRequestPostman})
	require.Nil(t, postmanPreview.Error)
	assert.Equal(t, "postman", postmanPreview.Data.Preview.Format)
	assert.Equal(t, []string{"a.example.com", "b.example.com", "c.example.com"}, postmanPreview.Data.Preview.Hosts)
	assert.NotNil(t, postmanPreview.Data.Preview.Scripts)
	assert.NotNil(t, postmanPreview.Data.Preview.Warnings)

	snapPreview := f.svc.ImportPreview(dto.ImportPreviewRequest{Content: string(allProtocolsSnapshot(t))})
	require.Nil(t, snapPreview.Error)
	assert.Equal(t, "tetiva", snapPreview.Data.Preview.Format)
	assert.Equal(t, []string{"prod"}, snapPreview.Data.Preview.EnvironmentNames)

	harPreview := f.svc.ImportPreview(dto.ImportPreviewRequest{Content: twoEntryHAR, FileName: "session.har"})
	require.Nil(t, harPreview.Error)
	assert.Equal(t, "har", harPreview.Data.Preview.Format)
	assert.Equal(t, "session", harPreview.Data.Preview.Title)
	assert.Equal(t, 2, harPreview.Data.Preview.Folders)

	hoppPreview := f.svc.ImportPreview(dto.ImportPreviewRequest{Content: hoppscotchCollection, FileName: "ignored.json"})
	require.Nil(t, hoppPreview.Error)
	assert.Equal(t, "hoppscotch", hoppPreview.Data.Preview.Format)
	assert.Equal(t, "H", hoppPreview.Data.Preview.Title)
	assert.Equal(t, []string{"H"}, hoppPreview.Data.Preview.EnvironmentNames)
	assert.Equal(t, []string{"h.example.com"}, hoppPreview.Data.Preview.Hosts)
	assert.Len(t, hoppPreview.Data.Preview.Scripts, 1)

	junk := f.svc.ImportPreview(dto.ImportPreviewRequest{Content: `{"hello":"world"}`})
	requireReason(t, junk, ErrCodeValidation, portability.ReasonUnsupportedFile)
	assert.Zero(t, f.count(t, "collections"))
	assert.Equal(t, 1, f.count(t, "environments"), "only the seeded Default")
}

const openAPIYAML = `openapi: 3.0.3
info: {title: Shop, version: "1"}
servers: [{url: "https://shop.example.com"}]
paths:
  /items/{id}:
    get:
      summary: Get item
      responses: {"200": {description: ok}}
`

func TestImportOpenAPI_PreviewThenConfirmIntoAFolder(t *testing.T) {
	f := newImportFixture(t, statusOnly(http.StatusNotFound), importFixtureOpts{})
	parent := f.svc.ImportConfirm(dto.ImportConfirmRequest{Content: threeRequestPostman, WorkspaceID: importWorkspace})
	require.Nil(t, parent.Error)

	preview := f.svc.ImportPreview(dto.ImportPreviewRequest{Content: openAPIYAML})
	require.Nil(t, preview.Error)
	assert.Equal(t, "openapi", preview.Data.Preview.Format)
	assert.Equal(t, "Shop", preview.Data.Preview.Title)
	assert.Equal(t, []string{"shop.example.com"}, preview.Data.Preview.Hosts)

	res := f.svc.ImportConfirm(dto.ImportConfirmRequest{Content: openAPIYAML, WorkspaceID: importWorkspace, ParentID: &parent.Data.CollectionID})
	require.Nil(t, res.Error)
	assert.Equal(t, 1, res.Data.Requests)
	assert.Equal(t, []string{"Shop"}, res.Data.EnvironmentNames)

	var parentID sql.NullString
	require.NoError(t, f.db.QueryRow(`SELECT parent_id FROM collections WHERE id = ?`, res.Data.CollectionID).Scan(&parentID))
	assert.Equal(t, parent.Data.CollectionID, parentID.String)
	var url string
	require.NoError(t, f.db.QueryRow(`SELECT url FROM requests WHERE name = 'Get item'`).Scan(&url))
	assert.Equal(t, "{{baseUrl}}/items/{{id}}", url)

	broken := f.svc.ImportPreview(dto.ImportPreviewRequest{Content: "openapi: 3.0.0\ninfo: {title: T}\npaths: [1]\n"})
	require.NotNil(t, broken.Error)
	assert.Equal(t, ErrCodeValidation, broken.Error.Code)
	assert.Contains(t, broken.Error.Fields["content"], "the OpenAPI document can't be read")
}

func TestImportPreview_HandsEnvironmentFilesBack(t *testing.T) {
	f := newImportFixture(t, statusOnly(http.StatusNotFound), importFixtureOpts{})
	envs, vars := f.count(t, "environments"), f.count(t, "variables")
	files := map[string]string{
		"environment": `{"name":"Dev","values":[{"key":"a","value":"1"}],"_postman_variable_scope":"environment"}`,
		"globals":     `{"name":"","values":[{"key":"a","value":"1"}],"_postman_variable_scope":"globals"}`,
	}
	for name, content := range files {
		t.Run(name, func(t *testing.T) {
			res := f.svc.ImportPreview(dto.ImportPreviewRequest{Content: content})

			requireReason(t, res, ErrCodeValidation, portability.ReasonEnvironmentFile)
			assert.Equal(t, map[string]string{"content": "a Postman environment, not a collection"}, res.Error.Fields)
		})
	}
	assert.Zero(t, f.count(t, "collections"))
	assert.Equal(t, envs, f.count(t, "environments"))
	assert.Equal(t, vars, f.count(t, "variables"))
}

func TestLinkMeta(t *testing.T) {
	f := newImportFixture(t, func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/pub/"+importSlug, r.URL.Path)
		_, _ = io.WriteString(w, `{"slug":"`+importSlug+`","title":"Petstore","locale":"en","visibility":"password",`+
			`"revision":4,"updated_at":"2026-09-20T10:00:00Z","password_required":true}`)
	}, importFixtureOpts{})

	res := f.svc.LinkMeta(dto.LinkMetaRequest{Slug: importSlug})
	require.Nil(t, res.Error)
	assert.Equal(t, dto.LinkMeta{
		Slug: importSlug, Title: "Petstore", PasswordRequired: true, Revision: 4, UpdatedAt: "2026-09-20T10:00:00Z",
	}, res.Data)
}

func TestLinkMeta_Errors(t *testing.T) {
	notFound := newImportFixture(t, statusOnly(http.StatusNotFound), importFixtureOpts{})
	requireReason(t, notFound.svc.LinkMeta(dto.LinkMetaRequest{Slug: importSlug}), ErrCodeNotFound, portability.ReasonLinkNotFound)

	limited := newImportFixture(t, statusOnly(http.StatusTooManyRequests), importFixtureOpts{})
	res := limited.svc.LinkMeta(dto.LinkMetaRequest{Slug: importSlug})
	require.NotNil(t, res.Error)
	assert.Equal(t, portability.ReasonRateLimited, res.Error.Reason)

	down := newImportFixture(t, statusOnly(http.StatusServiceUnavailable), importFixtureOpts{})
	requireReason(t, down.svc.LinkMeta(dto.LinkMetaRequest{Slug: importSlug}), ErrCodeInternal, ReasonServerUnreachable)

	dropped := newImportFixture(t, func(w http.ResponseWriter, _ *http.Request) {
		conn, _, err := w.(http.Hijacker).Hijack()
		require.NoError(t, err)
		_ = conn.Close()
	}, importFixtureOpts{})
	requireReason(t, dropped.svc.LinkMeta(dto.LinkMetaRequest{Slug: importSlug}), ErrCodeInternal, ReasonServerUnreachable)
	requireReason(t, dropped.svc.LinkFetch(dto.LinkFetchRequest{Slug: importSlug}), ErrCodeInternal, ReasonServerUnreachable)
	requireReason(t, dropped.svc.LinkUnlock(dto.LinkUnlockRequest{Slug: importSlug, Password: "password1"}), ErrCodeInternal, ReasonServerUnreachable)

	var hits atomic.Int32
	bad := newImportFixture(t, func(w http.ResponseWriter, _ *http.Request) { hits.Add(1) }, importFixtureOpts{})
	invalid := bad.svc.LinkMeta(dto.LinkMetaRequest{Slug: "../etc"})
	require.NotNil(t, invalid.Error)
	assert.Equal(t, ErrCodeValidation, invalid.Error.Code)
	assert.Zero(t, hits.Load())
}

func TestLinkUnlock(t *testing.T) {
	f := newImportFixture(t, func(w http.ResponseWriter, r *http.Request) {
		var body map[string]string
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		if body["password"] != "correct horse" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		_, _ = io.WriteString(w, `{"token":"view-token","expires_at":"2026-10-25T00:00:00Z"}`)
	}, importFixtureOpts{})

	ok := f.svc.LinkUnlock(dto.LinkUnlockRequest{Slug: importSlug, Password: "correct horse"})
	require.Nil(t, ok.Error)
	assert.Equal(t, "view-token", ok.Data.Token)

	wrong := f.svc.LinkUnlock(dto.LinkUnlockRequest{Slug: importSlug, Password: "battery staple"})
	requireReason(t, wrong, ErrCodeValidation, portability.ReasonPasswordInvalid)

	empty := f.svc.LinkUnlock(dto.LinkUnlockRequest{Slug: importSlug})
	require.NotNil(t, empty.Error)
	assert.Equal(t, ErrCodeValidation, empty.Error.Code)
}

func TestLinkFetch_Errors(t *testing.T) {
	newer, err := json.Marshal(map[string]any{"format": "tetiva.collection-snapshot", "version": 2})
	require.NoError(t, err)
	cases := map[string]struct {
		handler      http.HandlerFunc
		code, reason string
	}{
		"password":  {statusOnly(http.StatusUnauthorized), ErrCodeValidation, portability.ReasonPasswordRequired},
		"not found": {statusOnly(http.StatusNotFound), ErrCodeNotFound, portability.ReasonLinkNotFound},
		"junk":      {serveSnapshot(t, []byte(`<html>maintenance</html>`)), ErrCodeValidation, snapshotjson.ReasonInvalid},
		"newer":     {serveSnapshot(t, newer), ErrCodeValidation, snapshotjson.ReasonUpdateRequired},
		"too large": {serveSnapshot(t, bytes.Repeat([]byte(" "), 8<<20+1)), ErrCodeValidation, snapshotjson.ReasonTooLarge},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			f := newImportFixture(t, tc.handler, importFixtureOpts{})

			requireReason(t, f.svc.LinkFetch(dto.LinkFetchRequest{Slug: importSlug}), tc.code, tc.reason)
		})
	}

	limited := newImportFixture(t, statusOnly(http.StatusTooManyRequests), importFixtureOpts{})
	res := limited.svc.LinkFetch(dto.LinkFetchRequest{Slug: importSlug})
	require.NotNil(t, res.Error)
	assert.Equal(t, portability.ReasonRateLimited, res.Error.Reason)
}

func TestImportCollection_TakesASnapshotFileThroughTheSamePath(t *testing.T) {
	f := newImportFixture(t, statusOnly(http.StatusNotFound), importFixtureOpts{})

	res := f.svc.ImportCollection(dto.ImportCollectionRequest{Content: string(allProtocolsSnapshot(t)), WorkspaceID: importWorkspace})

	require.Nil(t, res.Error)
	assert.Equal(t, 4, res.Data.FoldersCreated)
	assert.Equal(t, 12, res.Data.RequestsCreated)
	var scripts int
	require.NoError(t, f.db.QueryRow(`SELECT COUNT(*) FROM collections WHERE pre_script != '' OR post_script != ''`).Scan(&scripts))
	assert.Zero(t, scripts)
}

func TestImportConfirm_NamesAHARAfterItsFileAndKeepsTheFolder(t *testing.T) {
	f := newImportFixture(t, statusOnly(http.StatusNotFound), importFixtureOpts{})
	parent := f.svc.ImportConfirm(dto.ImportConfirmRequest{Content: threeRequestPostman, WorkspaceID: importWorkspace})
	require.Nil(t, parent.Error)

	res := f.svc.ImportConfirm(dto.ImportConfirmRequest{
		Content: twoEntryHAR, FileName: "capture.har", WorkspaceID: importWorkspace, ParentID: &parent.Data.CollectionID,
	})

	require.Nil(t, res.Error)
	assert.Equal(t, 2, res.Data.Folders)
	assert.Equal(t, 2, res.Data.Requests)
	var name string
	var parentID sql.NullString
	require.NoError(t, f.db.QueryRow(`SELECT name, parent_id FROM collections WHERE id = ?`, res.Data.CollectionID).Scan(&name, &parentID))
	assert.Equal(t, "capture", name)
	assert.Equal(t, parent.Data.CollectionID, parentID.String)
}

func TestImportConfirm_HoppscotchScriptsFollowTheFlag(t *testing.T) {
	f := newImportFixture(t, statusOnly(http.StatusNotFound), importFixtureOpts{})

	without := f.svc.ImportConfirm(dto.ImportConfirmRequest{Content: hoppscotchCollection, WorkspaceID: importWorkspace})
	with := f.svc.ImportConfirm(dto.ImportConfirmRequest{Content: hoppscotchCollection, WorkspaceID: importWorkspace, IncludeScripts: true})

	require.Nil(t, without.Error)
	require.Nil(t, with.Error)
	assert.Equal(t, []string{"H"}, without.Data.EnvironmentNames)
	assert.Equal(t, []string{"H (2)"}, with.Data.EnvironmentNames)
	var scripts int
	require.NoError(t, f.db.QueryRow(`SELECT COUNT(*) FROM requests WHERE pre_script != ''`).Scan(&scripts))
	assert.Equal(t, 1, scripts)
	assert.Contains(t, warningTexts(with.Data.Warnings), "Hoppscotch scripts use the pw.* / hopp.* API, which Tetiva does not run: rewrite them to pm.* after importing")
	assert.NotContains(t, warningTexts(without.Data.Warnings), "Hoppscotch scripts use the pw.* / hopp.* API, which Tetiva does not run: rewrite them to pm.* after importing")
}

func TestImportEnvironment_TakesHoppscotchEnvironments(t *testing.T) {
	f := newImportFixture(t, statusOnly(http.StatusNotFound), importFixtureOpts{})
	single := `{"v":2,"id":"e1","name":"Dev","variables":[` +
		`{"key":"host","initialValue":"https://dev.example.com","currentValue":"","secret":false},` +
		`{"key":"token","initialValue":"","currentValue":"","secret":true}]}`
	list := `[{"v":1,"id":"e2","name":"Prod","variables":[{"key":"host","value":"https://example.com","secret":false}]},` +
		`{"name":"Stage","variables":[{"key":"host","value":"https://stage.example.com"}]}]`

	preview := f.svc.ImportPreview(dto.ImportPreviewRequest{Content: single})
	requireReason(t, preview, ErrCodeValidation, portability.ReasonEnvironmentFile)
	assert.Equal(t, map[string]string{"content": "a Hoppscotch environment, not a collection"}, preview.Error.Fields)

	one := f.svc.ImportEnvironment(dto.ImportEnvironmentRequest{Content: single, WorkspaceID: importWorkspace})
	require.Nil(t, one.Error)
	assert.Equal(t, []string{"Dev"}, one.Data.EnvironmentNames)
	assert.Equal(t, 2, one.Data.VariablesCreated)
	assert.Equal(t, []string{"Hoppscotch exports secrets without their values: fill in token"}, warningTexts(one.Data.Warnings))
	var value string
	var secret bool
	require.NoError(t, f.db.QueryRow(`SELECT value, is_secret FROM variables WHERE environment_id = ? AND key = 'host'`, one.Data.EnvironmentID).Scan(&value, &secret))
	assert.Equal(t, "https://dev.example.com", value)
	assert.False(t, secret)

	many := f.svc.ImportEnvironment(dto.ImportEnvironmentRequest{Content: list, WorkspaceID: importWorkspace})
	require.Nil(t, many.Error)
	assert.Equal(t, []string{"Prod", "Stage"}, many.Data.EnvironmentNames)
	assert.Equal(t, 2, many.Data.VariablesCreated)

	twice := f.svc.ImportEnvironment(dto.ImportEnvironmentRequest{Content: "[" + single + "," + single + "]", WorkspaceID: importWorkspace})
	require.Nil(t, twice.Error)
	assert.Equal(t, []string{"Dev (2)", "Dev (3)"}, twice.Data.EnvironmentNames)
	assert.Equal(t, []string{"Hoppscotch exports secrets without their values: fill in token"}, warningTexts(twice.Data.Warnings))
}

func TestImportEnvironment_RefusesCollectionsOfEveryFormat(t *testing.T) {
	f := newImportFixture(t, statusOnly(http.StatusNotFound), importFixtureOpts{})
	envs := f.count(t, "environments")

	for name, content := range map[string]string{"har": twoEntryHAR, "hoppscotch": hoppscotchCollection, "postman": threeRequestPostman} {
		res := f.svc.ImportEnvironment(dto.ImportEnvironmentRequest{Content: content, WorkspaceID: importWorkspace})
		requireReason(t, res, ErrCodeValidation, portability.ReasonCollectionFile)
		assert.Equal(t, envs, f.count(t, "environments"), name)
	}
}

func TestImport_RefusesFilesOverTheSizeLimit(t *testing.T) {
	f := newImportFixture(t, statusOnly(http.StatusNotFound), importFixtureOpts{})
	big := threeRequestPostman + strings.Repeat(" ", portability.MaxFileBytes)

	requireReason(t, f.svc.ImportPreview(dto.ImportPreviewRequest{Content: big}), ErrCodeValidation, portability.ReasonFileTooLarge)
	requireReason(t, f.svc.ImportConfirm(dto.ImportConfirmRequest{Content: big, WorkspaceID: importWorkspace}), ErrCodeValidation, portability.ReasonFileTooLarge)
	requireReason(t, f.svc.ImportCollection(dto.ImportCollectionRequest{Content: big, WorkspaceID: importWorkspace}), ErrCodeValidation, portability.ReasonFileTooLarge)
	requireReason(t, f.svc.ImportEnvironment(dto.ImportEnvironmentRequest{Content: big, WorkspaceID: importWorkspace}), ErrCodeValidation, portability.ReasonFileTooLarge)
	assert.Zero(t, f.count(t, "requests"))
	assert.Nil(t, f.svc.ImportPreview(dto.ImportPreviewRequest{Content: threeRequestPostman + strings.Repeat(" ", portability.MaxFileBytes-len(threeRequestPostman))}).Error)
}

func warningTexts(ws []dto.ImportWarning) []string {
	out := make([]string, 0, len(ws))
	for _, w := range ws {
		out = append(out, w.Text)
	}
	return out
}
