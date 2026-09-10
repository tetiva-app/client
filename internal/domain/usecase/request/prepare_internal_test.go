package request

import (
	"context"
	"encoding/base64"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/tetiva-app/client/internal/domain/entities"
	"github.com/tetiva-app/client/internal/domain/usecase/auth"
)

type internalCollectionReader struct{}

func (internalCollectionReader) GetByID(_ context.Context, _ uuid.UUID) (*entities.Collection, error) {
	return nil, nil
}

func (internalCollectionReader) ListByWorkspace(_ context.Context, _ uuid.UUID) ([]*entities.Collection, error) {
	return nil, nil
}

type internalScriptResolver struct{}

func (internalScriptResolver) ResolvePreScript(_ context.Context, _ *entities.Request) (string, error) {
	return "", nil
}
func (internalScriptResolver) ResolvePostScript(_ context.Context, _ *entities.Request) (string, error) {
	return "", nil
}

// ownAuth is what the resolver returns for a request carrying its own auth.
func ownAuth(req *entities.Request) ResolvedAuth {
	return ResolvedAuth{Type: req.AuthType, Data: req.AuthData}
}

func newInternalUsecase() *usecase {
	uc := NewUsecase(nil, nil, nil, nil, nil, nil, nil, &internalScriptResolver{}, nil, NewAuthResolver(internalCollectionReader{}), nil, nil, nil, nil).(*usecase)
	return uc
}

func TestPrepareHTTP_BasicGET(t *testing.T) {
	uc := newInternalUsecase()
	req := &entities.Request{
		ID:       uuid.New(),
		Protocol: entities.ProtocolHTTP,
		Method:   entities.MethodGET,
		URL:      "https://api.example.com/users",
		BodyType: entities.BodyTypeNone,
		AuthType: entities.AuthTypeNone,
	}

	prep, sr, err := uc.prepareHTTP(context.Background(), req, map[string]string{}, ownAuth(req), prepareOpt{})
	if err != nil {
		t.Fatalf("prepareHTTP: %v", err)
	}
	if sr != nil {
		t.Errorf("expected nil scriptResult for no-script req, got %+v", sr)
	}
	if prep.URL != "https://api.example.com/users" {
		t.Errorf("URL: got %q", prep.URL)
	}
	if prep.Method != entities.MethodGET {
		t.Errorf("method: got %q", prep.Method)
	}
	if prep.Body != "" {
		t.Errorf("body: got %q, want empty", prep.Body)
	}
	if prep.BodyReader != nil {
		t.Errorf("BodyReader: got non-nil")
	}
}

func TestPrepareHTTP_VarSubstitution(t *testing.T) {
	uc := newInternalUsecase()
	req := &entities.Request{
		ID:       uuid.New(),
		Protocol: entities.ProtocolHTTP,
		Method:   entities.MethodGET,
		URL:      "https://{{host}}/items",
		BodyType: entities.BodyTypeNone,
		AuthType: entities.AuthTypeNone,
	}

	prep, _, err := uc.prepareHTTP(context.Background(), req, map[string]string{"host": "api.example.com"}, ownAuth(req), prepareOpt{})
	if err != nil {
		t.Fatalf("prepareHTTP: %v", err)
	}
	if prep.URL != "https://api.example.com/items" {
		t.Errorf("URL: got %q, want substituted", prep.URL)
	}
}

func TestPrepareHTTP_JSONBodyAddsContentType(t *testing.T) {
	uc := newInternalUsecase()
	req := &entities.Request{
		ID:       uuid.New(),
		Protocol: entities.ProtocolHTTP,
		Method:   entities.MethodPOST,
		URL:      "https://api.example.com/users",
		Body:     `{"name":"Alice"}`,
		BodyType: entities.BodyTypeJSON,
		AuthType: entities.AuthTypeNone,
	}

	prep, _, err := uc.prepareHTTP(context.Background(), req, map[string]string{}, ownAuth(req), prepareOpt{})
	if err != nil {
		t.Fatalf("prepareHTTP: %v", err)
	}
	if got := prep.Headers["Content-Type"]; len(got) != 1 || got[0] != "application/json" {
		t.Errorf("Content-Type: got %v, want [application/json]", got)
	}
	if prep.Body != `{"name":"Alice"}` {
		t.Errorf("body: got %q", prep.Body)
	}
}

func TestPrepareHTTP_BinaryPathValidated(t *testing.T) {
	uc := newInternalUsecase()
	req := &entities.Request{
		ID:       uuid.New(),
		Protocol: entities.ProtocolHTTP,
		Method:   entities.MethodPOST,
		URL:      "https://api.example.com/upload",
		Body:     "../etc/passwd",
		BodyType: entities.BodyTypeBinary,
		AuthType: entities.AuthTypeNone,
	}

	_, _, err := uc.prepareHTTP(context.Background(), req, map[string]string{}, ownAuth(req), prepareOpt{})
	if err == nil {
		t.Fatal("expected error for path traversal, got nil")
	}
}

func mustFields(t *testing.T, raw string) auth.Fields {
	t.Helper()
	f, err := auth.ParseFields(raw)
	if err != nil {
		t.Fatalf("ParseFields(%q): %v", raw, err)
	}
	return f
}

func TestApplyHeaderAuth_Basic(t *testing.T) {
	headers, _, keys, err := applyHeaderAuth(entities.AuthTypeBasic,
		mustFields(t, `{"username":"admin","password":"secret"}`),
		map[string][]string{}, "https://api.example.com/data")
	if err != nil {
		t.Fatalf("applyHeaderAuth: %v", err)
	}
	want := "Basic " + base64.StdEncoding.EncodeToString([]byte("admin:secret"))
	if got := headers["Authorization"]; len(got) != 1 || got[0] != want {
		t.Errorf("Authorization: got %v, want [%s]", got, want)
	}
	if keys != nil {
		t.Errorf("query keys: got %v, want none", keys)
	}
}

func TestApplyHeaderAuth_BearerPrefixes(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want string
	}{
		{name: "default prefix", raw: `{"token":"t0"}`, want: "Bearer t0"},
		{name: "custom prefix", raw: `{"token":"t0","prefix":"Token"}`, want: "Token t0"},
		{name: "empty prefix", raw: `{"token":"t0","prefix":""}`, want: "t0"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			headers, _, _, err := applyHeaderAuth(entities.AuthTypeBearer, mustFields(t, tt.raw),
				map[string][]string{}, "https://api.example.com/data")
			if err != nil {
				t.Fatalf("applyHeaderAuth: %v", err)
			}
			if got := headers["Authorization"]; len(got) != 1 || got[0] != tt.want {
				t.Errorf("Authorization: got %v, want [%s]", got, tt.want)
			}
		})
	}
}

func TestApplyHeaderAuth_APIKeyLegacyInKey(t *testing.T) {
	headers, rawURL, keys, err := applyHeaderAuth(entities.AuthTypeAPIKey,
		mustFields(t, `{"key":"api_key","value":"sk_live","in":"query"}`),
		map[string][]string{}, "https://api.example.com/data")
	if err != nil {
		t.Fatalf("applyHeaderAuth: %v", err)
	}
	if !strings.Contains(rawURL, "api_key=sk_live") {
		t.Errorf("legacy \"in\" key must still route to query: %q", rawURL)
	}
	if len(headers) != 0 {
		t.Errorf("expected no headers, got %v", headers)
	}
	if len(keys) != 1 || keys[0] != "api_key" {
		t.Errorf("query keys: got %v, want [api_key]", keys)
	}
}

func TestApplyHeaderAuth_APIKeyAddToWinsOverIn(t *testing.T) {
	headers, rawURL, keys, err := applyHeaderAuth(entities.AuthTypeAPIKey,
		mustFields(t, `{"key":"X-Key","value":"sk_live","addTo":"header","in":"query"}`),
		map[string][]string{}, "https://api.example.com/data")
	if err != nil {
		t.Fatalf("applyHeaderAuth: %v", err)
	}
	if got := headers["X-Key"]; len(got) != 1 || got[0] != "sk_live" {
		t.Errorf("X-Key header: got %v", got)
	}
	if strings.Contains(rawURL, "X-Key") {
		t.Errorf("URL must stay untouched: %q", rawURL)
	}
	if keys != nil {
		t.Errorf("query keys: got %v, want none", keys)
	}
}

func TestApplyHeaderAuth_APIKeyReservedHeader(t *testing.T) {
	_, _, _, err := applyHeaderAuth(entities.AuthTypeAPIKey,
		mustFields(t, `{"key":"Authorization","value":"sk_live"}`),
		map[string][]string{}, "https://api.example.com/data")
	if err == nil {
		t.Fatal("expected error for a reserved header name")
	}
}

func TestApplyHeaderAuth_PassthroughTypes(t *testing.T) {
	for _, at := range []entities.AuthType{entities.AuthTypeNone, entities.AuthTypeInherit, ""} {
		headers, rawURL, keys, err := applyHeaderAuth(at, mustFields(t, `{"token":"t0"}`),
			map[string][]string{}, "https://api.example.com/data")
		if err != nil {
			t.Fatalf("applyHeaderAuth(%q): %v", at, err)
		}
		if len(headers) != 0 || rawURL != "https://api.example.com/data" || keys != nil {
			t.Errorf("%q must not touch the request: %v %q %v", at, headers, rawURL, keys)
		}
	}
}

func TestApplyHeaderAuth_EmptyFieldsAreNoop(t *testing.T) {
	headers, _, _, err := applyHeaderAuth(entities.AuthTypeBearer, auth.Fields{},
		map[string][]string{}, "https://api.example.com/data")
	if err != nil {
		t.Fatalf("applyHeaderAuth: %v", err)
	}
	if len(headers) != 0 {
		t.Errorf("expected no headers, got %v", headers)
	}
}

// oauth2, digest and aws_sigv4 are applied by other layers; an unknown type from
// a newer client must not silently send an unauthenticated request.
func TestApplyHeaderAuth_UnsupportedTypes(t *testing.T) {
	for _, at := range []entities.AuthType{
		entities.AuthTypeOAuth2, entities.AuthTypeDigest, entities.AuthTypeAWSSigV4, entities.AuthType("hawk"),
	} {
		_, _, _, err := applyHeaderAuth(at, mustFields(t, `{"token":"t0"}`),
			map[string][]string{}, "https://api.example.com/data")
		if err == nil {
			t.Errorf("%q: expected an error", at)
		}
	}
}

func TestPrepareHTTP_AuthValueWithQuotesSurvivesSubstitution(t *testing.T) {
	uc := newInternalUsecase()
	req := &entities.Request{
		ID:       uuid.New(),
		Protocol: entities.ProtocolHTTP,
		Method:   entities.MethodGET,
		URL:      "https://api.example.com/users",
		BodyType: entities.BodyTypeNone,
		AuthType: entities.AuthTypeBasic,
		AuthData: `{"username":"admin","password":"{{pw}}"}`,
	}

	prep, _, err := uc.prepareHTTP(context.Background(), req, map[string]string{"pw": `p"a}}ss`}, ownAuth(req), prepareOpt{})
	if err != nil {
		t.Fatalf("prepareHTTP: %v", err)
	}
	got := prep.Headers["Authorization"]
	want := "Basic " + base64.StdEncoding.EncodeToString([]byte(`admin:p"a}}ss`))
	if len(got) != 1 || got[0] != want {
		t.Errorf("Authorization: got %v, want [%s]", got, want)
	}
}

// digest and aws_sigv4 are handed to the requester on preparedHTTP.Auth: they
// can only be applied once the final request exists.
func TestPrepareHTTP_RequesterLayerAuth(t *testing.T) {
	tests := []struct {
		name     string
		authType entities.AuthType
		data     string
		wantKey  string
		wantVal  string
	}{
		{
			name: "digest", authType: entities.AuthTypeDigest,
			data: `{"username":"neo","password":"{{pw}}"}`, wantKey: "password", wantVal: `p"ass`,
		},
		{
			name: "aws_sigv4", authType: entities.AuthTypeAWSSigV4,
			data:    `{"accessKeyId":"AKID","secretAccessKey":"{{pw}}","region":"us-east-1","service":"s3"}`,
			wantKey: "secretAccessKey", wantVal: `p"ass`,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			uc := newInternalUsecase()
			req := &entities.Request{
				ID:       uuid.New(),
				Protocol: entities.ProtocolHTTP,
				Method:   entities.MethodGET,
				URL:      "https://api.example.com/users",
				BodyType: entities.BodyTypeNone,
				AuthType: tc.authType,
				AuthData: tc.data,
			}

			prep, _, err := uc.prepareHTTP(context.Background(), req, map[string]string{"pw": `p"ass`}, ownAuth(req), prepareOpt{})
			if err != nil {
				t.Fatalf("prepareHTTP: %v", err)
			}
			if prep.Auth == nil {
				t.Fatal("expected preparedHTTP.Auth to carry the scheme")
			}
			if prep.Auth.Type != tc.authType {
				t.Errorf("auth type: got %q, want %q", prep.Auth.Type, tc.authType)
			}
			if got := prep.Auth.Fields[tc.wantKey]; got != tc.wantVal {
				t.Errorf("%s: got %v, want %q", tc.wantKey, got, tc.wantVal)
			}
			if _, ok := prep.Headers["Authorization"]; ok {
				t.Errorf("no header must be set for %s: %v", tc.authType, prep.Headers)
			}
			if prep.URL != "https://api.example.com/users" {
				t.Errorf("URL must be untouched, got %q", prep.URL)
			}
		})
	}
}

func TestPrepareHTTP_HeaderAuthLeavesNoRequesterAuth(t *testing.T) {
	uc := newInternalUsecase()
	req := &entities.Request{
		ID:       uuid.New(),
		Protocol: entities.ProtocolHTTP,
		Method:   entities.MethodGET,
		URL:      "https://api.example.com/users",
		BodyType: entities.BodyTypeNone,
		AuthType: entities.AuthTypeAPIKey,
		AuthData: `{"key":"api_key","value":"secret","addTo":"query"}`,
	}

	prep, _, err := uc.prepareHTTP(context.Background(), req, map[string]string{}, ownAuth(req), prepareOpt{})
	if err != nil {
		t.Fatalf("prepareHTTP: %v", err)
	}
	if prep.Auth != nil {
		t.Errorf("header-layer auth must not reach the requester: %+v", prep.Auth)
	}
	if len(prep.AuthQueryKeys) != 1 || prep.AuthQueryKeys[0] != "api_key" {
		t.Errorf("AuthQueryKeys: got %v, want [api_key]", prep.AuthQueryKeys)
	}
}
