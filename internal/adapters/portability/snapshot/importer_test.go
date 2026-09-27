package snapshot_test

import (
	"bytes"
	"compress/gzip"
	"context"
	"database/sql"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tetiva-app/client/internal/adapters/portability"
	"github.com/tetiva-app/client/internal/adapters/portability/snapshot"
	"github.com/tetiva-app/client/internal/adapters/publication/snapshotjson"
	"github.com/tetiva-app/client/internal/domain"
	"github.com/tetiva-app/client/internal/domain/entities"
	"github.com/tetiva-app/client/internal/domain/usecase/auth"
	"github.com/tetiva-app/client/internal/domain/usecase/collection"
	"github.com/tetiva-app/client/internal/domain/usecase/environment"
	"github.com/tetiva-app/client/internal/domain/usecase/example"
	"github.com/tetiva-app/client/internal/domain/usecase/request"
	"github.com/tetiva-app/client/internal/domain/usecase/websocket"
	"github.com/tetiva-app/client/internal/infrastructure/repository/sqlite"
	"github.com/tetiva-app/client/migrations"
	"github.com/tetiva-app/client/pkg/migrate"

	_ "modernc.org/sqlite"
)

var workspaceID = uuid.MustParse("00000000-0000-4000-a000-000000000001")

var _ portability.Importer = (*snapshot.Importer)(nil)

type env struct {
	db   *sql.DB
	cols collection.Usecase
	reqs request.Usecase
	exs  example.Usecase
	envs environment.Usecase
	imp  *snapshot.Importer
}

func newEnv(t *testing.T) *env {
	t.Helper()
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "test.db"))
	require.NoError(t, err)
	db.SetMaxOpenConns(1)
	_, err = db.Exec("PRAGMA foreign_keys=ON")
	require.NoError(t, err)
	require.NoError(t, migrate.Run(db, migrations.FS, "."))
	t.Cleanup(func() { _ = db.Close() })

	colRepo := sqlite.NewCollectionRepo(db)
	reqRepo := sqlite.NewRequestRepo(db)
	e := &env{
		db:   db,
		cols: collection.NewUsecase(colRepo, nil, nil, nil),
		reqs: request.NewUsecase(reqRepo, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil),
		exs:  example.NewUsecase(sqlite.NewResponseExampleRepo(db), reqRepo, colRepo),
		envs: environment.NewUsecase(sqlite.NewEnvironmentRepo(db), sqlite.NewVariableRepo(db)),
	}
	e.imp = snapshot.NewImporter(e.cols, e.reqs, e.exs, e.envs)
	return e
}

func fixture(t *testing.T, name string) []byte {
	t.Helper()
	_, file, _, _ := runtime.Caller(0)
	raw, err := os.ReadFile(filepath.Join(filepath.Dir(file), "..", "..", "..", "..", "testdata", "snapshot", name))
	require.NoError(t, err)
	return raw
}

func opt(include bool) portability.ImportOpt {
	return portability.ImportOpt{WorkspaceID: workspaceID, UserID: "local_user", IncludeScripts: include}
}

type tree struct {
	cols map[string]*entities.Collection
	reqs map[string]*entities.Request
}

func (e *env) tree(t *testing.T) tree {
	t.Helper()
	ctx := context.Background()
	all, err := e.cols.List(ctx, collection.ListOpt{WorkspaceID: workspaceID})
	require.NoError(t, err)
	tr := tree{cols: map[string]*entities.Collection{}, reqs: map[string]*entities.Request{}}
	for _, c := range all {
		tr.cols[c.Name] = c
		reqs, err := e.reqs.List(ctx, request.ListOpt{CollectionID: c.ID})
		require.NoError(t, err)
		for _, r := range reqs {
			tr.reqs[r.Name] = r
		}
	}
	return tr
}

func doc(t *testing.T, items []any, collectionExtra map[string]any) []byte {
	t.Helper()
	col := map[string]any{
		"id": "000000000001", "name": "Imported", "description": "", "auth": nil, "scripts": nil,
		"grpcMetadata": []any{}, "items": items,
	}
	for k, v := range collectionExtra {
		col[k] = v
	}
	raw, err := json.Marshal(map[string]any{
		"format": "tetiva.collection-snapshot", "version": 1, "generator": "Tetiva 1.2.0", "locale": "en",
		"collection": col, "environment": nil,
	})
	require.NoError(t, err)
	return raw
}

func httpRequest(name string, extra map[string]any) map[string]any {
	r := map[string]any{
		"kind": "request", "id": "00000000000a", "name": name, "description": "", "protocol": "http",
		"http": map[string]any{
			"method": "GET", "url": "https://api.example.com/x", "headers": []any{},
			"body": map[string]any{"type": "none", "raw": "", "fields": []any{}, "fileName": ""},
		},
		"auth":    map[string]any{"type": "inherit", "fields": map[string]any{}, "redacted": []any{}},
		"scripts": nil, "examples": []any{},
	}
	for k, v := range extra {
		r[k] = v
	}
	return r
}

func folder(name string, auth any, items ...any) map[string]any {
	if items == nil {
		items = []any{}
	}
	return map[string]any{
		"kind": "folder", "id": "00000000000f", "name": name, "description": "", "auth": auth, "scripts": nil, "items": items,
	}
}

func TestImport_AllProtocols(t *testing.T) {
	e := newEnv(t)

	res, err := e.imp.Import(context.Background(), fixture(t, "all-protocols.json"), opt(true))
	require.NoError(t, err)

	assert.Equal(t, 3, res.Folders)
	assert.Equal(t, 12, res.Requests)
	assert.Equal(t, 6, res.Examples)
	assert.Equal(t, []string{
		`request "Загрузить фото": file field "file" was imported without its file; pick it again`,
		`request "Replace avatar": the file body was imported without its file; pick it again`,
	}, res.Warnings)

	tr := e.tree(t)
	root := tr.cols["Petstore API — демо"]
	require.NotNil(t, root)
	assert.Equal(t, res.CollectionID, root.ID)
	assert.Nil(t, root.ParentID)
	assert.Equal(t, entities.AuthTypeBearer, root.AuthType)
	assert.JSONEq(t, `{"prefix":"Bearer","token":"{{token}}"}`, root.AuthData)
	assert.Equal(t, `pm.environment.set("ts", Date.now());`, root.PreScript)
	assert.Equal(t, []entities.HeaderItem{
		{Key: "x-client", Value: "tetiva", Enabled: true},
		{Key: "x-api-version", Value: "2026-09", Enabled: true},
	}, root.GRPCMetadata)

	pets := tr.cols["Питомцы"]
	assert.Equal(t, root.ID, *pets.ParentID)
	assert.Equal(t, entities.AuthTypeOAuth2, pets.AuthType)
	assert.Equal(t, `console.log("folder pre");`, pets.PreScript)
	assert.Equal(t, entities.AuthTypeNone, tr.cols["Администрирование"].AuthType)
	assert.Equal(t, pets.ID, *tr.cols["Администрирование"].ParentID)

	assert.Equal(t, entities.AuthTypeInherit, tr.reqs["Delete pet"].AuthType)
	assert.Equal(t, entities.AuthTypeNone, tr.reqs["Search (urlencoded)"].AuthType)
	jwt, err := auth.ParseFields(tr.reqs["Check pet exists"].AuthData)
	require.NoError(t, err)
	assert.Equal(t, "{{userId}}", jwt.Obj("claims")["sub"])
	assert.Contains(t, tr.reqs["Check pet exists"].AuthData, `"iat":1727222400`)

	upload := tr.reqs["Загрузить фото"]
	assert.Equal(t, entities.BodyTypeForm, upload.BodyType)
	assert.JSONEq(t, `[
		{"key":"title","value":"Отчёт за Q3","type":"text","enabled":true},
		{"key":"file","value":"","type":"file","enabled":true},
		{"key":"draft","value":"true","type":"text","enabled":false}
	]`, upload.Body)
	assert.Equal(t, entities.BodyTypeBinary, tr.reqs["Replace avatar"].BodyType)
	assert.Empty(t, tr.reqs["Replace avatar"].Body)

	create := tr.reqs["Create pet"]
	assert.Equal(t, entities.MethodPOST, create.Method)
	assert.Equal(t, entities.BodyTypeJSON, create.BodyType)
	assert.Equal(t, `pm.test("created", () => pm.response.to.have.status(201));`, create.PostScript)

	gql := tr.reqs["Pet by id (GraphQL)"]
	assert.Equal(t, entities.ProtocolGraphQL, gql.Protocol)
	assert.Equal(t, "Pet", gql.GraphQLOperation)
	assert.Contains(t, gql.GraphQLVariables, "{{petId}}")

	grpc := tr.reqs["GetPet (gRPC)"]
	assert.Equal(t, entities.ProtocolGRPC, grpc.Protocol)
	assert.Equal(t, "{{grpcHost}}", grpc.URL)
	assert.Equal(t, "petstore.v1.PetService", grpc.GRPCService)
	assert.Equal(t, "GetPet", grpc.GRPCMethod)
	assert.Equal(t, entities.BodyTypeJSON, grpc.BodyType)
	assert.Contains(t, grpc.Body, `"id": "42"`)
	assert.Equal(t, map[string][]string{
		"x-request-id": {"{{requestId}}"}, "x-tenant": {"acme"}, "authorization": {"Bearer {{token}}"},
	}, grpc.GRPCMetadata)

	ws := tr.reqs["Чат питомника"]
	assert.Equal(t, entities.ProtocolWebSocket, ws.Protocol)
	require.NoError(t, websocket.ValidateSettings(ws.Body))
	settings := websocket.ParseSettings(ws.Body)
	assert.Equal(t, []string{"events.v1", "json"}, settings.Subprotocols)
	require.Len(t, settings.Messages, 3)
	assert.Equal(t, "binary", settings.Messages[2].Format)
	assert.Equal(t, "3q2+7w==", settings.Messages[2].Data)
	assert.NotEqual(t, settings.Messages[0].ID, settings.Messages[1].ID)

	grpcExamples, err := e.exs.ListByRequest(context.Background(), grpc.ID)
	require.NoError(t, err)
	require.Len(t, grpcExamples, 2)
	assert.Equal(t, 5, grpcExamples[1].StatusCode)
	assert.Equal(t, entities.ProtocolGRPC, grpcExamples[1].Protocol)

	envs, err := e.envs.List(context.Background(), environment.ListOpt{WorkspaceID: workspaceID})
	require.NoError(t, err)
	var prod *entities.Environment
	for _, env := range envs {
		if env.Name == "prod" {
			prod = env
		}
	}
	require.NotNil(t, prod)
	assert.False(t, prod.IsActive)
	vars, err := e.envs.ListVariables(context.Background(), prod.ID)
	require.NoError(t, err)
	require.Len(t, vars, 13)
	byKey := map[string]*entities.Variable{}
	for _, v := range vars {
		byKey[v.Key] = v
	}
	assert.Equal(t, "https://petstore.example.com/v1", byKey["baseUrl"].Value)
	assert.True(t, byKey["token"].IsSecret)
	assert.Empty(t, byKey["token"].Value)
}

func TestImport_RootMetadataIsNotRepeatedOnRequests(t *testing.T) {
	e := newEnv(t)
	meta := func(k, v string) map[string]any {
		return map[string]any{"key": k, "value": v, "enabled": true, "redacted": false}
	}
	grpc := httpRequest("Call", map[string]any{
		"protocol": "grpc", "http": nil,
		"grpc": map[string]any{
			"target": "localhost:50051", "service": "s.S", "method": "M", "message": "{}",
			"metadata": []any{meta("x-client", "tetiva"), meta("x-tenant", "acme"), meta("x-trace", "a"), meta("X-Trace", "b")},
		},
	})
	raw := doc(t, []any{grpc}, map[string]any{"grpcMetadata": []any{meta("X-Client", "tetiva"), meta("x-tenant", "root")}})

	_, err := e.imp.Import(context.Background(), raw, opt(false))
	require.NoError(t, err)

	assert.Equal(t, map[string][]string{"x-tenant": {"acme"}, "x-trace": {"a", "b"}}, e.tree(t).reqs["Call"].GRPCMetadata)
}

func TestImport_ScriptsOffByDefault(t *testing.T) {
	e := newEnv(t)

	_, err := e.imp.Import(context.Background(), fixture(t, "all-protocols.json"), opt(false))
	require.NoError(t, err)

	tr := e.tree(t)
	for name, c := range tr.cols {
		assert.Empty(t, c.PreScript+c.PostScript, name)
	}
	for name, r := range tr.reqs {
		assert.Empty(t, r.PreScript+r.PostScript, name)
	}
}

func TestImport_AlwaysTopLevel(t *testing.T) {
	e := newEnv(t)
	parent, err := e.cols.Create(context.Background(), collection.Create{
		Name: "Mine", AuthType: entities.AuthTypeBearer, AuthData: `{"token":"{{mine}}"}`, PreScript: "pm.environment.set('x', 1)",
	}, collection.CreateOpt{UserID: "local_user", WorkspaceID: workspaceID})
	require.NoError(t, err)
	o := opt(false)
	o.ParentID = &parent.ID

	res, err := e.imp.Import(context.Background(), doc(t, []any{httpRequest("R", nil)}, nil), o)
	require.NoError(t, err)

	got, err := e.cols.GetByID(context.Background(), res.CollectionID)
	require.NoError(t, err)
	assert.Nil(t, got.ParentID)
}

func TestImport_FolderAuthPassThroughAndRequestAuthAsIs(t *testing.T) {
	e := newEnv(t)
	authOf := func(typ string) map[string]any {
		return map[string]any{"type": typ, "fields": map[string]any{}, "redacted": []any{}}
	}
	raw := doc(t, []any{
		folder("Inherit", authOf("inherit")),
		folder("None", authOf("none")),
		folder("Null", nil),
		folder("Unknown", authOf("hawk")),
		httpRequest("Req none", map[string]any{"auth": authOf("none")}),
		httpRequest("Req inherit", map[string]any{"auth": authOf("inherit")}),
		httpRequest("Req null", map[string]any{"auth": nil}),
		httpRequest("Req unknown", map[string]any{"auth": authOf("ntlm")}),
	}, map[string]any{"auth": authOf("inherit")})

	res, err := e.imp.Import(context.Background(), raw, opt(false))
	require.NoError(t, err)

	tr := e.tree(t)
	for _, name := range []string{"Imported", "Inherit", "None", "Null", "Unknown"} {
		assert.Equal(t, entities.AuthTypeNone, tr.cols[name].AuthType, name)
		assert.Equal(t, "{}", tr.cols[name].AuthData, name)
	}
	assert.Equal(t, entities.AuthTypeNone, tr.reqs["Req none"].AuthType)
	assert.Equal(t, entities.AuthTypeInherit, tr.reqs["Req inherit"].AuthType)
	assert.Equal(t, entities.AuthTypeInherit, tr.reqs["Req null"].AuthType)
	assert.Equal(t, entities.AuthTypeNone, tr.reqs["Req unknown"].AuthType)
	assert.Equal(t, []string{
		`folder "Unknown": auth type "hawk" is not supported and was imported as no auth`,
		`request "Req unknown": auth type "ntlm" is not supported and was imported as no auth`,
	}, res.Warnings)
}

func TestImport_MaliciousSnapshot(t *testing.T) {
	e := newEnv(t)
	raw := doc(t, []any{
		httpRequest("Upload", map[string]any{
			"http": map[string]any{
				"method": "POST", "url": "https://x.example.com", "headers": []any{}, "unexpected": true,
				"body": map[string]any{"type": "form", "raw": "", "fileName": "", "fields": []any{
					map[string]any{"key": "key", "value": "/Users/ivan/.ssh/id_rsa", "type": "file", "enabled": true},
					map[string]any{"key": "win", "value": `C:\Users\ivan\key.pem`, "type": "file", "enabled": true},
				}},
			},
			"scripts": map[string]any{"pre": "pm.environment.set('baseUrl', 'https://evil.example.com')", "post": ""},
		}),
		httpRequest("Binary", map[string]any{
			"http": map[string]any{
				"method": "PUT", "url": "https://x.example.com", "headers": []any{},
				"body": map[string]any{"type": "binary", "raw": "", "fields": []any{}, "fileName": "/etc/passwd"},
			},
		}),
	}, map[string]any{"ownerEmail": "someone@example.com"})

	_, err := e.imp.Import(context.Background(), raw, opt(false))
	require.NoError(t, err)

	tr := e.tree(t)
	assert.JSONEq(t, `[{"key":"key","value":"","type":"file","enabled":true},{"key":"win","value":"","type":"file","enabled":true}]`,
		tr.reqs["Upload"].Body)
	assert.Empty(t, tr.reqs["Upload"].PreScript)
	assert.Empty(t, tr.reqs["Binary"].Body)
}

func TestImport_RefusesWhatTheServerWouldRefuse(t *testing.T) {
	e := newEnv(t)
	deep := []any{}
	for range 17 {
		deep = []any{folder("F", nil, deep...)}
	}
	many := make([]any, 5001)
	for i := range many {
		many[i] = httpRequest("R", nil)
	}
	newer, err := json.Marshal(map[string]any{"format": "tetiva.collection-snapshot", "version": 2})
	require.NoError(t, err)

	cases := map[string]struct {
		raw    []byte
		reason string
	}{
		"too deep":  {doc(t, deep, nil), snapshotjson.ReasonInvalid},
		"too many":  {doc(t, many, nil), snapshotjson.ReasonInvalid},
		"version 2": {newer, snapshotjson.ReasonUpdateRequired},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			for _, run := range []func() error{
				func() error { _, err := e.imp.Import(context.Background(), tc.raw, opt(false)); return err },
				func() error { _, err := e.imp.Preview(tc.raw); return err },
			} {
				var re *domain.ReasonError
				require.ErrorAs(t, run(), &re)
				assert.Equal(t, tc.reason, re.Reason)
			}
		})
	}
	assert.Empty(t, e.tree(t).cols)
}

func TestImport_Normalisation(t *testing.T) {
	e := newEnv(t)
	longName := strings.Repeat("н", 300)
	longDesc := strings.Repeat("ё", 10*1024)
	raw := doc(t, []any{
		httpRequest(longName, map[string]any{"description": longDesc}),
		httpRequest("   ", nil),
		httpRequest("Odd", map[string]any{"http": map[string]any{
			"method": "FETCH", "url": "https://x.example.com", "headers": []any{},
			"body": map[string]any{"type": "yaml", "raw": "a: 1", "fields": []any{}, "fileName": ""},
		}}),
		httpRequest("MQTT", map[string]any{"protocol": "mqtt"}),
		httpRequest("Missing part", map[string]any{"protocol": "graphql"}),
		httpRequest("Examples", map[string]any{"examples": []any{
			map[string]any{"id": "0000000000e1", "name": "Huge", "status": 200, "statusText": "OK", "headers": []any{},
				"body": strings.Repeat("a", 300*1024), "contentType": "text/plain"},
			map[string]any{"id": "0000000000e2", "name": "Odd status", "status": 1200, "statusText": "", "headers": []any{},
				"body": "", "contentType": ""},
			map[string]any{"id": "0000000000e3", "name": "", "status": 201, "statusText": "Created", "headers": []any{},
				"body": "{}", "contentType": "application/json"},
		}}),
	}, nil)

	p, err := e.imp.Preview(raw)
	require.NoError(t, err)
	res, err := e.imp.Import(context.Background(), raw, opt(false))
	require.NoError(t, err)

	assert.Equal(t, p.Warnings, res.Warnings)
	assert.Equal(t, 4, res.Requests)
	assert.Equal(t, 2, res.Examples)
	assert.Equal(t, p.Requests, res.Requests)
	assert.Equal(t, p.Examples, res.Examples)
	assert.Equal(t, []string{
		`request "` + strings.Repeat("н", 40) + `…": name longer than 200 characters was truncated`,
		`request "` + strings.Repeat("н", 200) + `": description longer than 16384 bytes was truncated`,
		`an unnamed request in "Imported" was imported as "Untitled"`,
		`request "Odd": method "FETCH" is not supported, imported as GET`,
		`request "Odd": body type "yaml" is not supported, imported without a body`,
		`request "MQTT": protocol "mqtt" is not supported, the request was skipped`,
		`request "Missing part": the graphql part is missing, the request was skipped`,
		`request "Examples": example "Huge" skipped: body is larger than 256 KB`,
		`request "Examples": example "Odd status": status 1200 is outside 0–999, imported as 0`,
		`an unnamed example in "Imported / Examples" was imported as "Untitled"`,
	}, res.Warnings)

	tr := e.tree(t)
	long := tr.reqs[strings.Repeat("н", 200)]
	require.NotNil(t, long)
	assert.LessOrEqual(t, len(long.Description), domain.MaxDescriptionLen)
	assert.True(t, utf8.ValidString(long.Description))
	assert.NotNil(t, tr.reqs["Untitled"])
	assert.Equal(t, entities.MethodGET, tr.reqs["Odd"].Method)
	assert.Equal(t, entities.BodyTypeNone, tr.reqs["Odd"].BodyType)

	exs, err := e.exs.ListByRequest(context.Background(), tr.reqs["Examples"].ID)
	require.NoError(t, err)
	require.Len(t, exs, 2)
	assert.Equal(t, 0, exs[0].StatusCode)
	assert.Equal(t, "Untitled", exs[1].Name)
}

func TestImport_NameConflictGetsASuffix(t *testing.T) {
	e := newEnv(t)
	raw := doc(t, []any{}, nil)

	var names []string
	for range 3 {
		res, err := e.imp.Import(context.Background(), raw, opt(false))
		require.NoError(t, err)
		c, err := e.cols.GetByID(context.Background(), res.CollectionID)
		require.NoError(t, err)
		names = append(names, c.Name)
	}
	assert.Equal(t, []string{"Imported", "Imported (2)", "Imported (3)"}, names)
}

func TestPreview_AllProtocolsWritesNothing(t *testing.T) {
	e := newEnv(t)

	p, err := e.imp.Preview(fixture(t, "all-protocols.json"))
	require.NoError(t, err)

	assert.Equal(t, portability.FormatTetiva, p.Format)
	assert.Equal(t, "Petstore API — демо", p.Title)
	assert.Equal(t, 3, p.Folders)
	assert.Equal(t, 12, p.Requests)
	assert.Equal(t, 6, p.Examples)
	assert.Equal(t, "prod", p.EnvironmentName)
	assert.Equal(t, []string{
		"auth.example.com", "grpc.petstore.example.com:443", "petstore.example.com", "ws.petstore.example.com",
	}, p.Hosts)
	root := "Petstore API — демо"
	assert.Equal(t, []portability.ScriptPreview{
		{Path: root, Phase: "pre", Text: `pm.environment.set("ts", Date.now());`},
		{Path: root, Phase: "post", Text: `pm.test("status is 2xx", () => pm.expect(pm.response.code).to.be.below(300));`},
		{Path: root + " / Питомцы", Phase: "pre", Text: `console.log("folder pre");`},
		{Path: root + " / Питомцы / Create pet", Phase: "pre", Text: `pm.environment.set("requestId", crypto.randomUUID());`},
		{Path: root + " / Питомцы / Create pet", Phase: "post", Text: `pm.test("created", () => pm.response.to.have.status(201));`},
		{Path: root + " / Realtime / Чат питомника", Phase: "post", Text: `console.log(pm.response.text());`},
	}, p.Scripts)
	assert.Len(t, p.Warnings, 2)
	assert.Empty(t, e.tree(t).cols)
}

func TestPreview_HostsFromAuthURLsAndUnresolvedVariables(t *testing.T) {
	e := newEnv(t)
	oauth := map[string]any{"type": "oauth2", "fields": map[string]any{
		"tokenUrl": "{{authHost}}/token", "authUrl": "https://login.example.com/authorize", "deviceAuthUrl": "",
	}, "redacted": []any{}}
	raw := doc(t, []any{httpRequest("R", map[string]any{
		"auth": oauth,
		"http": map[string]any{"method": "GET", "url": "{{baseUrl}}/x", "headers": []any{},
			"body": map[string]any{"type": "none", "raw": "", "fields": []any{}, "fileName": ""}},
	})}, nil)
	var d map[string]any
	require.NoError(t, json.Unmarshal(raw, &d))
	d["environment"] = map[string]any{"name": "", "variables": []any{
		map[string]any{"key": "authHost", "value": "https://auth.example.com", "secret": false},
		map[string]any{"key": "baseUrl", "value": "https://hidden.example.com", "secret": true},
	}}
	raw, err := json.Marshal(d)
	require.NoError(t, err)

	p, err := e.imp.Preview(raw)
	require.NoError(t, err)

	assert.Equal(t, []string{"auth.example.com", "login.example.com", "{{baseUrl}}"}, p.Hosts)
	assert.Equal(t, "Untitled", p.EnvironmentName)
}

func TestPreview_HostsOfGRPCResolverTargets(t *testing.T) {
	e := newEnv(t)
	grpc := func(name, target string) map[string]any {
		return httpRequest(name, map[string]any{"protocol": "grpc", "http": nil, "grpc": map[string]any{
			"target": target, "service": "s.S", "method": "M", "message": "{}", "metadata": []any{},
		}})
	}
	raw := doc(t, []any{
		grpc("A", "dns:///a.example.com:443"),
		grpc("B", "dns://1.1.1.1/b.example.com:443"),
		grpc("C", "passthrough:///c.example.com:443"),
		grpc("D", "dns:///"),
	}, nil)

	p, err := e.imp.Preview(raw)
	require.NoError(t, err)

	assert.Equal(t, []string{"a.example.com:443", "b.example.com:443", "c.example.com:443", "dns:///"}, p.Hosts)
}

func TestDetect(t *testing.T) {
	e := newEnv(t)
	raw := fixture(t, "all-protocols.json")
	var buf bytes.Buffer
	zw := gzip.NewWriter(&buf)
	_, err := zw.Write(raw)
	require.NoError(t, err)
	require.NoError(t, zw.Close())

	assert.True(t, e.imp.Detect(raw))
	assert.True(t, e.imp.Detect(buf.Bytes()))
	assert.False(t, e.imp.Detect([]byte(`{"info":{"schema":"https://schema.getpostman.com/json/collection/v2.1.0/collection.json"},"item":[]}`)))
	assert.False(t, e.imp.Detect([]byte(`nope`)))
}
