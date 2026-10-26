package request_test

import (
	"context"
	"encoding/json"
	"os"
	"slices"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/tetiva-app/client/internal/domain/entities"
	"github.com/tetiva-app/client/internal/domain/usecase/request"
)

func pv(kv ...string) []entities.PathVariable {
	out := []entities.PathVariable{}
	for i := 0; i < len(kv); i += 2 {
		out = append(out, entities.PathVariable{Key: kv[i], Value: kv[i+1]})
	}
	return out
}

func createWithPath(t *testing.T, uc request.Usecase, protocol entities.Protocol, url string, vars []entities.PathVariable) *entities.Request {
	t.Helper()
	created, err := uc.Create(context.Background(), request.Create{
		CollectionID:  testCollectionID,
		Name:          "Get user",
		Protocol:      protocol,
		Method:        entities.MethodGET,
		URL:           url,
		PathVariables: vars,
		BodyType:      entities.BodyTypeNone,
		AuthType:      entities.AuthTypeNone,
	}, request.CreateOpt{UserID: "user-1"})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	return created
}

func TestCreate_PathVariablesFollowTheURL(t *testing.T) {
	uc, repo, _, _ := newTestUsecase()

	created := createWithPath(t, uc, entities.ProtocolHTTP, "{{baseUrl}}/orgs/:org/users/:id",
		pv("id", "{{userId}}", "stale", "x"))

	want := pv("org", "", "id", "{{userId}}")
	if !slices.Equal(created.PathVariables, want) {
		t.Errorf("created = %v, want %v", created.PathVariables, want)
	}
	if !slices.Equal(repo.requests[created.ID].PathVariables, want) {
		t.Errorf("stored = %v, want %v", repo.requests[created.ID].PathVariables, want)
	}
}

func TestCreate_PathVariablesOnlyWhereTheTableExists(t *testing.T) {
	uc, _, _, _ := newTestUsecase()

	for _, p := range []entities.Protocol{entities.ProtocolGRPC, entities.ProtocolGraphQL} {
		created := createWithPath(t, uc, p, "https://api.io/:tenant/graphql", pv("tenant", "acme"))
		if created.PathVariables == nil || len(created.PathVariables) != 0 {
			t.Errorf("%s: path variables = %#v, want an empty list", p, created.PathVariables)
		}
	}
	ws := createWithPath(t, uc, entities.ProtocolWebSocket, "wss://chat.io/rooms/:room", pv("room", "lobby"))
	if !slices.Equal(ws.PathVariables, pv("room", "lobby")) {
		t.Errorf("websocket path variables = %v", ws.PathVariables)
	}
}

func TestEdit_PathVariablesKeepValuesOfKeysStillInTheURL(t *testing.T) {
	uc, _, _, _ := newTestUsecase()
	created := createWithPath(t, uc, entities.ProtocolHTTP, "/users/:id", pv("id", "42"))

	edited, err := uc.Edit(context.Background(), request.Edit{
		Name:          created.Name,
		Method:        entities.MethodGET,
		URL:           "/users/:id/posts/:postId",
		PathVariables: pv("id", "42", "postId", "7", "gone", "1"),
		BodyType:      entities.BodyTypeNone,
		AuthType:      entities.AuthTypeNone,
	}, request.EditOpt{RequestID: created.ID, UserID: "user-1", Version: created.Version})
	if err != nil {
		t.Fatalf("Edit: %v", err)
	}

	if want := pv("id", "42", "postId", "7"); !slices.Equal(edited.PathVariables, want) {
		t.Errorf("edited = %v, want %v", edited.PathVariables, want)
	}
}

func TestEdit_WithoutPathVariablesKeepsTheStoredValues(t *testing.T) {
	uc, _, _, _ := newTestUsecase()
	created := createWithPath(t, uc, entities.ProtocolHTTP, "/users/:id", pv("id", "42"))
	edit := request.Edit{
		Name:     "Renamed",
		Method:   entities.MethodGET,
		URL:      "/users/:id",
		BodyType: entities.BodyTypeNone,
		AuthType: entities.AuthTypeNone,
	}

	kept, err := uc.Edit(context.Background(), edit, request.EditOpt{RequestID: created.ID, UserID: "user-1", Version: created.Version})
	if err != nil {
		t.Fatalf("Edit: %v", err)
	}
	if !slices.Equal(kept.PathVariables, pv("id", "42")) {
		t.Errorf("omitted field = %v, want the stored value", kept.PathVariables)
	}

	edit.PathVariables = pv()
	cleared, err := uc.Edit(context.Background(), edit, request.EditOpt{RequestID: created.ID, UserID: "user-1", Version: kept.Version})
	if err != nil {
		t.Fatalf("Edit: %v", err)
	}
	if !slices.Equal(cleared.PathVariables, pv("id", "")) {
		t.Errorf("explicit empty list = %v, want the value cleared", cleared.PathVariables)
	}
}

func TestDuplicate_ResyncsPathVariablesWithTheNewURL(t *testing.T) {
	repo := newMockRepo()
	original := seedSiblings(repo, "Get user")[0]
	original.URL = "/users/:id"
	original.PathVariables = pv("id", "42")
	uc := newCascadeUsecase(repo, fixtureCollections(), nil, &recordingExampleCleaner{}, &snapshotTx{repo: repo})

	same, err := uc.Duplicate(context.Background(), request.DuplicateOpt{RequestID: original.ID, Name: "Copy", UserID: "u1"})
	if err != nil {
		t.Fatalf("Duplicate: %v", err)
	}
	moved, err := uc.Duplicate(context.Background(), request.DuplicateOpt{
		RequestID: original.ID, Name: "Posts", URL: "/users/:id/posts/:postId", UserID: "u1",
	})
	if err != nil {
		t.Fatalf("Duplicate with URL: %v", err)
	}

	if !slices.Equal(same.PathVariables, pv("id", "42")) {
		t.Errorf("copy = %v", same.PathVariables)
	}
	if !slices.Equal(moved.PathVariables, pv("id", "42", "postId", "")) {
		t.Errorf("copy with a new URL = %v", moved.PathVariables)
	}
}

func TestExecute_PathValuesComeBeforeTheEnvironment(t *testing.T) {
	uc, _, history, requester := newTestUsecaseWithVars(map[string]string{
		"baseUrl": "https://api.io",
		"userId":  "42",
		"id":      "from-env",
	})
	created := createWithPath(t, uc, entities.ProtocolHTTP, "{{baseUrl}}/users/:id/posts/:postId?id=:id",
		pv("id", "{{userId}}"))
	requester.response = &entities.Response{StatusCode: 200, Headers: map[string][]string{}, Duration: time.Millisecond}

	if _, err := uc.Execute(context.Background(), created.ID, request.ExecuteOpt{UserID: "user-1", WorkspaceID: testWorkspaceID}); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	want := "https://api.io/users/42/posts/:postId?id=:id"
	if requester.lastRequest.URL != want {
		t.Errorf("sent %q, want %q", requester.lastRequest.URL, want)
	}
	if got := history.entries[len(history.entries)-1].URL; got != want {
		t.Errorf("history URL %q, want %q", got, want)
	}
}

func TestBuildSnippetInput_AppliesPathVariables(t *testing.T) {
	d := newSnippetDeps(map[string]string{"baseUrl": "https://api.io", "userId": "42"})
	req := snippetRequest(entities.ProtocolHTTP, entities.MethodGET, "{{baseUrl}}/users/:id/:tab")
	req.PathVariables = pv("id", "{{userId}}", "tab", "")

	resolved, _ := d.buildHAR(t, req, true)
	if resolved.URL != "https://api.io/users/42/:tab" {
		t.Errorf("resolved URL = %q", resolved.URL)
	}
	raw, _ := d.buildHAR(t, req, false)
	if raw.URL != "{{baseUrl}}/users/{{userId}}/:tab" {
		t.Errorf("unresolved URL = %q", raw.URL)
	}

	ws := snippetRequest(entities.ProtocolWebSocket, "", "wss://chat.io/rooms/:room")
	ws.PathVariables = pv("room", "lobby")
	if in := d.build(t, ws, true); in.WS == nil || in.WS.URL != "wss://chat.io/rooms/lobby" {
		t.Errorf("websocket snippet = %+v", in.WS)
	}
}

func TestResolveWebSocket_AppliesPathVariables(t *testing.T) {
	reqID := uuid.New()
	repo := newMockRepo()
	r := wsRequest(reqID, "wss://{{host}}/rooms/:room")
	r.PathVariables = pv("room", "{{room}}")
	repo.requests[reqID] = r
	uc := newTestUsecaseWithEnv(t, repo, map[string]string{"host": "chat.io", "room": "lobby"})

	dial, err := uc.ResolveWebSocket(context.Background(), reqID, testWorkspaceID, "user-1")
	if err != nil {
		t.Fatalf("ResolveWebSocket: %v", err)
	}
	if dial.URL != "wss://chat.io/rooms/lobby" {
		t.Errorf("url = %q", dial.URL)
	}
}

func TestExecute_APathValueStaysOneSegment(t *testing.T) {
	uc, _, _, requester := newTestUsecaseWithVars(map[string]string{"userId": "a/b?c#d", "base": "https://api.io"})
	created := createWithPath(t, uc, entities.ProtocolHTTP, "{{base}}/users/:id/posts?limit=1", pv("id", "{{userId}}"))
	requester.response = &entities.Response{StatusCode: 200, Headers: map[string][]string{}, Duration: time.Millisecond}

	if _, err := uc.Execute(context.Background(), created.ID, request.ExecuteOpt{UserID: "user-1", WorkspaceID: testWorkspaceID}); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	if want := "https://api.io/users/a%2Fb%3Fc%23d/posts?limit=1"; requester.lastRequest.URL != want {
		t.Errorf("sent %q, want %q", requester.lastRequest.URL, want)
	}
}

func TestResolveURL_SharedCases(t *testing.T) {
	raw, err := os.ReadFile("../../../../testdata/path-variables.json")
	if err != nil {
		t.Fatal(err)
	}
	var cases struct {
		Resolve []struct {
			URL  string                  `json:"url"`
			Vars []entities.PathVariable `json:"vars"`
			Env  map[string]string       `json:"env"`
			Want string                  `json:"want"`
		} `json:"resolve"`
	}
	if err := json.Unmarshal(raw, &cases); err != nil {
		t.Fatal(err)
	}
	for _, tc := range cases.Resolve {
		resolve := func(s string) string { return request.SubstituteVariables(s, tc.Env) }
		if got := entities.ResolveURL(tc.URL, tc.Vars, resolve); got != tc.Want {
			t.Errorf("ResolveURL(%q) = %q, want %q", tc.URL, got, tc.Want)
		}
	}
}
