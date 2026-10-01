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
	assert.Equal(t, "prod", confirmed.Data.EnvironmentName)
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
		id, err := c.put([]byte{byte(i)})
		require.NoError(t, err)
		ids = append(ids, id)
	}

	_, ok := c.get(ids[0])
	assert.False(t, ok)
	for i, id := range ids[1:] {
		data, ok := c.get(id)
		require.True(t, ok)
		assert.Equal(t, []byte{byte(i + 1)}, data)
	}

	_, err := c.put(make([]byte, 8<<20+1))
	require.Error(t, err)
}

func TestImportConfirm_RollsBackBothFormats(t *testing.T) {
	cases := map[string]string{
		"snapshot": string(allProtocolsSnapshot(t)),
		"postman":  threeRequestPostman,
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
	assert.Equal(t, "Dev", first.Data.EnvironmentName)
	assert.Equal(t, "Dev (2)", second.Data.EnvironmentName)
	assert.Equal(t, 2, first.Data.VariablesCreated)
	assert.Equal(t, []string{"a variable without a name was skipped"}, first.Data.Warnings)
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
	assert.Equal(t, "postman", postmanPreview.Data.Format)
	assert.Equal(t, []string{"a.example.com", "b.example.com", "c.example.com"}, postmanPreview.Data.Hosts)
	assert.NotNil(t, postmanPreview.Data.Scripts)
	assert.NotNil(t, postmanPreview.Data.Warnings)

	snapPreview := f.svc.ImportPreview(dto.ImportPreviewRequest{Content: string(allProtocolsSnapshot(t))})
	require.Nil(t, snapPreview.Error)
	assert.Equal(t, "tetiva", snapPreview.Data.Format)
	assert.Equal(t, "prod", snapPreview.Data.EnvironmentName)

	junk := f.svc.ImportPreview(dto.ImportPreviewRequest{Content: `{"hello":"world"}`})
	requireReason(t, junk, ErrCodeValidation, portability.ReasonUnsupportedFile)
	assert.Zero(t, f.count(t, "collections"))
	assert.Equal(t, 1, f.count(t, "environments"), "only the seeded Default")
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
