package request_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/tetiva-app/client/internal/domain"
	"github.com/tetiva-app/client/internal/domain/entities"
	"github.com/tetiva-app/client/internal/domain/usecase/request"
)

// ucWithRequest builds a usecase with a single request loaded into the mock repo.
func ucWithRequest(t *testing.T, r *entities.Request) request.Usecase {
	t.Helper()
	repo := newMockRepo()
	repo.requests[r.ID] = r
	return request.NewUsecase(repo, &mockHistoryRepo{}, &mockRequester{}, nil, nil,
		&mockEnvResolver{}, &noopScriptEngine{}, &noopScriptResolver{},
		&noopVarPersister{}, request.NewAuthResolver(&mockCollectionReader{}), nil, nil)
}

func TestBuildCurl_GETPlain(t *testing.T) {
	id := uuid.New()
	uc := ucWithRequest(t, &entities.Request{
		ID: id, Protocol: entities.ProtocolHTTP, Method: entities.MethodGET,
		URL: "https://api.example.com/x", BodyType: entities.BodyTypeNone, AuthType: entities.AuthTypeNone,
	})
	got, sr, err := uc.BuildCurl(context.Background(), id, request.BuildCurlOpt{WorkspaceID: uuid.New()})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if sr != nil {
		t.Errorf("expected nil scriptResult, got %+v", sr)
	}
	want := "curl \\\n  'https://api.example.com/x'"
	if got != want {
		t.Errorf("curl mismatch:\n got: %q\nwant: %q", got, want)
	}
}

func TestBuildCurl_POSTJSONAddsContentType(t *testing.T) {
	id := uuid.New()
	uc := ucWithRequest(t, &entities.Request{
		ID: id, Protocol: entities.ProtocolHTTP, Method: entities.MethodPOST,
		URL:      "https://api.example.com/users",
		Body:     `{"name":"Alice"}`,
		BodyType: entities.BodyTypeJSON, AuthType: entities.AuthTypeNone,
	})
	got, _, err := uc.BuildCurl(context.Background(), id, request.BuildCurlOpt{WorkspaceID: uuid.New()})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if !strings.Contains(got, "-X POST") {
		t.Errorf("missing -X POST: %s", got)
	}
	if !strings.Contains(got, "-H 'Content-Type: application/json'") {
		t.Errorf("missing Content-Type header: %s", got)
	}
	if !strings.Contains(got, `-d '{"name":"Alice"}'`) {
		t.Errorf("missing -d body: %s", got)
	}
}

func TestBuildCurl_BearerAuth(t *testing.T) {
	id := uuid.New()
	uc := ucWithRequest(t, &entities.Request{
		ID: id, Protocol: entities.ProtocolHTTP, Method: entities.MethodGET,
		URL: "https://api.example.com/me", BodyType: entities.BodyTypeNone,
		AuthType: entities.AuthTypeBearer, AuthData: `{"token":"xyz"}`,
	})
	got, _, err := uc.BuildCurl(context.Background(), id, request.BuildCurlOpt{WorkspaceID: uuid.New()})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if !strings.Contains(got, "-H 'Authorization: Bearer xyz'") {
		t.Errorf("expected Bearer header, got: %s", got)
	}
}

func TestBuildCurl_APIKeyInQuery(t *testing.T) {
	id := uuid.New()
	uc := ucWithRequest(t, &entities.Request{
		ID: id, Protocol: entities.ProtocolHTTP, Method: entities.MethodGET,
		URL: "https://api.example.com/x", BodyType: entities.BodyTypeNone,
		AuthType: entities.AuthTypeAPIKey,
		AuthData: `{"key":"api_key","value":"secret","addTo":"query"}`,
	})
	got, _, err := uc.BuildCurl(context.Background(), id, request.BuildCurlOpt{WorkspaceID: uuid.New()})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if !strings.Contains(got, "api_key=secret") {
		t.Errorf("expected api_key in URL, got: %s", got)
	}
}

func TestBuildCurl_FormUrlencoded(t *testing.T) {
	id := uuid.New()
	uc := ucWithRequest(t, &entities.Request{
		ID: id, Protocol: entities.ProtocolHTTP, Method: entities.MethodPOST,
		URL:      "https://api.example.com/login",
		Body:     `[{"key":"user","value":"alice","type":"text","enabled":true}]`,
		BodyType: entities.BodyTypeForm, AuthType: entities.AuthTypeNone,
	})
	got, _, err := uc.BuildCurl(context.Background(), id, request.BuildCurlOpt{WorkspaceID: uuid.New()})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if !strings.Contains(got, "-d 'user=alice'") {
		t.Errorf("expected -d 'user=alice' for urlencoded form, got: %s", got)
	}
	if strings.Contains(got, "-F ") {
		t.Errorf("urlencoded form must NOT use -F, got: %s", got)
	}
	if !strings.Contains(got, "-H 'Content-Type: application/x-www-form-urlencoded'") {
		t.Errorf("expected explicit urlencoded Content-Type, got: %s", got)
	}
}

func TestBuildCurl_FormMultipartWithFile(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "avatar.jpg")
	if err := os.WriteFile(filePath, []byte("fake-jpg-bytes"), 0644); err != nil {
		t.Fatalf("write temp file: %v", err)
	}

	id := uuid.New()
	body, _ := json.Marshal([]map[string]any{
		{"key": "name", "value": "alice", "type": "text", "enabled": true},
		{"key": "avatar", "value": filePath, "type": "file", "enabled": true},
	})
	uc := ucWithRequest(t, &entities.Request{
		ID: id, Protocol: entities.ProtocolHTTP, Method: entities.MethodPOST,
		URL:  "https://api.example.com/upload",
		Body: string(body), BodyType: entities.BodyTypeForm, AuthType: entities.AuthTypeNone,
	})
	got, _, err := uc.BuildCurl(context.Background(), id, request.BuildCurlOpt{WorkspaceID: uuid.New()})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if !strings.Contains(got, "-F 'name=alice'") {
		t.Errorf("expected text field as -F: %s", got)
	}
	if !strings.Contains(got, "-F 'avatar=@"+filePath+"'") {
		t.Errorf("expected file field as -F with @path: %s", got)
	}
	if strings.Contains(got, "-d ") {
		t.Errorf("multipart must NOT use -d: %s", got)
	}
}

func TestBuildCurl_URLWithSingleQuote(t *testing.T) {
	id := uuid.New()
	uc := ucWithRequest(t, &entities.Request{
		ID: id, Protocol: entities.ProtocolHTTP, Method: entities.MethodGET,
		URL: "https://api.example.com/it's-fine", BodyType: entities.BodyTypeNone, AuthType: entities.AuthTypeNone,
	})
	got, _, err := uc.BuildCurl(context.Background(), id, request.BuildCurlOpt{WorkspaceID: uuid.New()})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if !strings.Contains(got, `'https://api.example.com/it'\''s-fine'`) {
		t.Errorf("expected escaped quote, got: %s", got)
	}
}

func TestBuildCurl_NonHTTPProtocolErrors(t *testing.T) {
	id := uuid.New()
	uc := ucWithRequest(t, &entities.Request{
		ID: id, Protocol: entities.ProtocolGRPC, Method: entities.MethodPOST,
		URL: "grpc.example.com:50051", BodyType: entities.BodyTypeJSON,
		GRPCService: "Foo", GRPCMethod: "Bar",
	})
	_, _, err := uc.BuildCurl(context.Background(), id, request.BuildCurlOpt{WorkspaceID: uuid.New()})
	if err == nil {
		t.Fatal("expected error for gRPC protocol")
	}
	var valErr *domain.ValidationError
	if !errors.As(err, &valErr) {
		t.Fatalf("expected ValidationError, got %T: %v", err, err)
	}
	if _, ok := valErr.Fields["protocol"]; !ok {
		t.Error("expected 'protocol' field in ValidationError")
	}
}

func TestBuildCurl_PreScriptInjectsHeader(t *testing.T) {
	id := uuid.New()
	repo := newMockRepo()
	repo.requests[id] = &entities.Request{
		ID: id, Protocol: entities.ProtocolHTTP, Method: entities.MethodGET,
		URL: "https://api.example.com/x", BodyType: entities.BodyTypeNone, AuthType: entities.AuthTypeNone,
	}
	cap := &captureScriptEngine{
		preHeaders: map[string][]string{"X-Trace": {"abc"}},
		preVars:    map[string]string{},
	}
	uc := request.NewUsecase(repo, &mockHistoryRepo{}, &mockRequester{}, nil, nil,
		&mockEnvResolver{}, cap, &scriptResolverWithPre{pre: "// inject"},
		&noopVarPersister{}, request.NewAuthResolver(&mockCollectionReader{}), nil, nil)

	got, sr, err := uc.BuildCurl(context.Background(), id, request.BuildCurlOpt{WorkspaceID: uuid.New()})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if !strings.Contains(got, "-H 'X-Trace: abc'") {
		t.Errorf("expected pre-script header, got: %s", got)
	}
	if sr == nil {
		t.Fatal("expected non-nil scriptResult")
	}
	if cap.preCalls != 1 {
		t.Errorf("expected pre-script called once, got %d", cap.preCalls)
	}
}

type recordingPersister struct{ calls int }

func (r *recordingPersister) PersistVariableChanges(_ context.Context, _ uuid.UUID, _ string, _ map[string]string) error {
	r.calls++
	return nil
}

func TestBuildCurl_DryRunVsExecuteVarPersist(t *testing.T) {
	id := uuid.New()
	repo := newMockRepo()
	repo.requests[id] = &entities.Request{
		ID: id, Protocol: entities.ProtocolHTTP, Method: entities.MethodGET,
		URL: "https://api.example.com/x", BodyType: entities.BodyTypeNone, AuthType: entities.AuthTypeNone,
	}
	cap := &captureScriptEngine{
		preHeaders: map[string][]string{},
		preVars:    map[string]string{"new_var": "value"},
	}
	persister := &recordingPersister{}
	uc := request.NewUsecase(repo, &mockHistoryRepo{}, &mockRequester{response: &entities.Response{StatusCode: 200}}, nil, nil,
		&mockEnvResolver{}, cap, &scriptResolverWithPre{pre: "// set var"},
		persister, request.NewAuthResolver(&mockCollectionReader{}), nil, nil)

	// positive control — Execute does persist vars
	if _, err := uc.Execute(context.Background(), id, request.ExecuteOpt{WorkspaceID: uuid.New()}); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if persister.calls != 1 {
		t.Fatalf("Execute should persist vars: got %d calls, want 1", persister.calls)
	}

	if _, _, err := uc.BuildCurl(context.Background(), id, request.BuildCurlOpt{WorkspaceID: uuid.New()}); err != nil {
		t.Fatalf("buildcurl: %v", err)
	}
	if persister.calls != 1 {
		t.Errorf("BuildCurl persisted vars (dry-run violated): now %d calls, want still 1", persister.calls)
	}
}

// fakeCookieReader returns canned cookies regardless of URL.
type fakeCookieReader struct{ cookies []*http.Cookie }

func (f *fakeCookieReader) CookiesFor(_ context.Context, _ uuid.UUID, _ string) []*http.Cookie {
	return f.cookies
}

func TestBuildCurl_EmitsCookiesFromJar(t *testing.T) {
	id := uuid.New()
	repo := newMockRepo()
	repo.requests[id] = &entities.Request{
		ID: id, Protocol: entities.ProtocolHTTP, Method: entities.MethodGET,
		URL: "https://api.example.com/me", BodyType: entities.BodyTypeNone, AuthType: entities.AuthTypeNone,
	}
	cookies := []*http.Cookie{
		{Name: "session", Value: "abc123"},
		{Name: "csrf", Value: "tok"},
	}
	uc := request.NewUsecase(repo, &mockHistoryRepo{}, &mockRequester{}, nil, nil,
		&mockEnvResolver{}, &noopScriptEngine{}, &noopScriptResolver{},
		&noopVarPersister{}, request.NewAuthResolver(&mockCollectionReader{}),
		&fakeCookieReader{cookies: cookies}, nil)

	got, _, err := uc.BuildCurl(context.Background(), id, request.BuildCurlOpt{WorkspaceID: uuid.New()})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if !strings.Contains(got, "-b 'session=abc123; csrf=tok'") {
		t.Errorf("expected -b with cookies, got: %s", got)
	}
}

func TestBuildCurl_NoCookieReaderProducesNoB(t *testing.T) {
	id := uuid.New()
	repo := newMockRepo()
	repo.requests[id] = &entities.Request{
		ID: id, Protocol: entities.ProtocolHTTP, Method: entities.MethodGET,
		URL: "https://api.example.com/x", BodyType: entities.BodyTypeNone, AuthType: entities.AuthTypeNone,
	}
	uc := request.NewUsecase(repo, &mockHistoryRepo{}, &mockRequester{}, nil, nil,
		&mockEnvResolver{}, &noopScriptEngine{}, &noopScriptResolver{},
		&noopVarPersister{}, request.NewAuthResolver(&mockCollectionReader{}), nil, nil)

	got, _, err := uc.BuildCurl(context.Background(), id, request.BuildCurlOpt{WorkspaceID: uuid.New()})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if strings.Contains(got, "-b ") {
		t.Errorf("expected no -b when cookieReader is nil, got: %s", got)
	}
}

func TestBuildCurl_EmptyCookieListProducesNoB(t *testing.T) {
	id := uuid.New()
	repo := newMockRepo()
	repo.requests[id] = &entities.Request{
		ID: id, Protocol: entities.ProtocolHTTP, Method: entities.MethodGET,
		URL: "https://api.example.com/x", BodyType: entities.BodyTypeNone, AuthType: entities.AuthTypeNone,
	}
	uc := request.NewUsecase(repo, &mockHistoryRepo{}, &mockRequester{}, nil, nil,
		&mockEnvResolver{}, &noopScriptEngine{}, &noopScriptResolver{},
		&noopVarPersister{}, request.NewAuthResolver(&mockCollectionReader{}),
		&fakeCookieReader{cookies: nil}, nil)

	got, _, err := uc.BuildCurl(context.Background(), id, request.BuildCurlOpt{WorkspaceID: uuid.New()})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if strings.Contains(got, "-b ") {
		t.Errorf("expected no -b for empty cookie list, got: %s", got)
	}
}
