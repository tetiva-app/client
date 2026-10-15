package mcp

import (
	"context"
	"encoding/json"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tetiva-app/client/internal/domain/entities"
	"github.com/tetiva-app/client/internal/domain/usecase/environment"
	"github.com/tetiva-app/client/internal/domain/usecase/request"
	"github.com/tetiva-app/client/internal/infrastructure/scriptengine"
	"github.com/tetiva-app/client/internal/race"
)

func TestMaskURLSecrets(t *testing.T) {
	cases := []struct{ in, want string }{
		{"https://api.example.com/users", "https://api.example.com/users"},
		{"https://api.example.com/u?page=2", "https://api.example.com/u?page=2"},
		{"https://api.example.com/u?token=abc123", "https://api.example.com/u?token=" + redactedValue},
		{
			"https://api.example.com/u?page=2&api_key=sk_live_1&sort=asc",
			"https://api.example.com/u?page=2&api_key=" + redactedValue + "&sort=asc",
		},
		{"https://api.example.com/u?ACCESS_TOKEN=abc", "https://api.example.com/u?ACCESS_TOKEN=" + redactedValue},
		{"https://api.example.com/u?signature=x#frag", "https://api.example.com/u?signature=" + redactedValue + "#frag"},
		// A variable reference is a pointer, not a secret — the agent needs it.
		{"https://api.example.com/u?token={{authToken}}", "https://api.example.com/u?token={{authToken}}"},
		{"https://{{host}}/u?key=", "https://{{host}}/u?key="},
		{"https://api.example.com/u?flag", "https://api.example.com/u?flag"},
	}
	for _, c := range cases {
		assert.Equal(t, c.want, maskURLSecrets(c.in), "input %q", c.in)
	}
}

func TestMaskHeaderItems(t *testing.T) {
	in := []entities.HeaderItem{
		{Key: "Authorization", Value: "Bearer sk_live_1", Enabled: true},
		{Key: "cookie", Value: "session=abc", Enabled: true},
		{Key: "X-API-Key", Value: "k1", Enabled: false},
		{Key: "Content-Type", Value: "application/json", Enabled: true},
		{Key: "X-Auth-Token", Value: "{{token}}", Enabled: true},
		{Key: "X-Trace", Value: "on", Enabled: true},
	}
	out := maskHeaderItems(in)

	assert.Equal(t, redactedValue, out[0].Value)
	assert.Equal(t, redactedValue, out[1].Value)
	assert.Equal(t, redactedValue, out[2].Value)
	assert.Equal(t, "application/json", out[3].Value)
	assert.Equal(t, "{{token}}", out[4].Value)
	assert.Equal(t, "on", out[5].Value)
	assert.False(t, out[2].Enabled, "masking must not touch other fields")

	// Masking a copy, never the caller's slice — the stored request stays intact.
	assert.Equal(t, "Bearer sk_live_1", in[0].Value)
	assert.Nil(t, maskHeaderItems(nil))
}

func TestMaskResponseHeaders(t *testing.T) {
	out := maskResponseHeaders(map[string][]string{
		"Set-Cookie":   {"a=1", "b=2"},
		"Content-Type": {"application/json"},
	})
	assert.Equal(t, []string{redactedValue, redactedValue}, out["Set-Cookie"])
	assert.Equal(t, []string{"application/json"}, out["Content-Type"])
	assert.Nil(t, maskResponseHeaders(nil))
}

func TestServer_SerializedRequest_MasksSecrets(t *testing.T) {
	srv := setupTestServer(t)

	colRes := callTool(t, srv, "create_collection", map[string]any{"name": "Redact"})
	var col map[string]any
	require.NoError(t, json.Unmarshal([]byte(colRes), &col))

	reqRes := callTool(t, srv, "create_request", map[string]any{
		"collection_id": col["id"].(string),
		"name":          "Secretive",
		"method":        "GET",
		"url":           "https://api.example.com/u?api_key=sk_live_1&page=2",
		"headers": []map[string]any{
			{"key": "Authorization", "value": "Bearer sk_live_1", "enabled": true},
			{"key": "X-Trace", "value": "on", "enabled": true},
		},
	})
	var created map[string]any
	require.NoError(t, json.Unmarshal([]byte(reqRes), &created))

	assert.Equal(t, "https://api.example.com/u?api_key="+redactedValue+"&page=2", created["url"])
	headers := created["headers"].([]any)
	assert.Equal(t, redactedValue, headers[0].(map[string]any)["value"])
	assert.Equal(t, "on", headers[1].(map[string]any)["value"])

	listRes := callTool(t, srv, "list_requests", map[string]any{"collection_id": col["id"].(string)})
	var listed map[string]any
	require.NoError(t, json.Unmarshal([]byte(listRes), &listed))
	first := listed["requests"].([]any)[0].(map[string]any)
	assert.Equal(t, "https://api.example.com/u?api_key="+redactedValue+"&page=2", first["url"])

	// The redaction must stay in the tool output: an agent that echoes the
	// request back without the header argument must not write "[redacted]" in.
	callTool(t, srv, "update_request", map[string]any{
		"id":      created["id"].(string),
		"version": created["version"].(float64),
		"name":    "Secretive v2",
	})
	stored, err := srv.reqUC.GetByID(context.Background(), uuid.MustParse(created["id"].(string)))
	require.NoError(t, err)
	assert.Equal(t, "Bearer sk_live_1", stored.Headers[0].Value)
	assert.Equal(t, "https://api.example.com/u?api_key=sk_live_1&page=2", stored.URL)
}

func callToolJSON(t *testing.T, srv *Server, name string, args map[string]any) map[string]any {
	t.Helper()
	raw := callTool(t, srv, name, args)
	var out map[string]any
	require.NoError(t, json.Unmarshal([]byte(raw), &out), "tool %q returned %s", name, raw)
	return out
}

func TestUpdateRequest_EchoedMaskKeepsStoredSecrets(t *testing.T) {
	srv := setupTestServer(t)
	col := callToolJSON(t, srv, "create_collection", map[string]any{"name": "Echo"})

	created := callToolJSON(t, srv, "create_request", map[string]any{
		"collection_id": col["id"],
		"name":          "Secretive",
		"method":        "GET",
		"url":           "https://api.example.com/u?api_key=sk_live_1&page=2",
		"auth_data":     `{"token":"sk_live_2"}`,
		"headers": []map[string]any{
			{"key": "Authorization", "value": "Bearer sk_live_3", "enabled": true},
			{"key": "X-Trace", "value": "on", "enabled": true},
		},
	})

	read := callToolJSON(t, srv, "get_request", map[string]any{"id": created["id"]})
	require.Equal(t, redactedValue, read["auth_data"])
	require.Contains(t, read["url"], redactedValue)

	// Adding one header means resending the whole array, masks included.
	echoed := append(read["headers"].([]any),
		map[string]any{"key": "X-New", "value": "added", "enabled": true})
	callToolJSON(t, srv, "update_request", map[string]any{
		"id":        created["id"],
		"version":   created["version"],
		"name":      "Secretive",
		"url":       read["url"],
		"auth_data": read["auth_data"],
		"headers":   echoed,
	})

	stored, err := srv.reqUC.GetByID(context.Background(), uuid.MustParse(created["id"].(string)))
	require.NoError(t, err)
	assert.Equal(t, "https://api.example.com/u?api_key=sk_live_1&page=2", stored.URL)
	assert.Equal(t, `{"token":"sk_live_2"}`, stored.AuthData)
	require.Len(t, stored.Headers, 3)
	assert.Equal(t, "Bearer sk_live_3", stored.Headers[0].Value)
	assert.Equal(t, "on", stored.Headers[1].Value)
	assert.Equal(t, "added", stored.Headers[2].Value)
}

func TestUpdateRequest_RealValueOverwritesStoredSecret(t *testing.T) {
	srv := setupTestServer(t)
	col := callToolJSON(t, srv, "create_collection", map[string]any{"name": "Rotate"})

	created := callToolJSON(t, srv, "create_request", map[string]any{
		"collection_id": col["id"],
		"name":          "Secretive",
		"method":        "GET",
		"url":           "https://api.example.com/u?api_key=sk_live_1",
		"auth_data":     `{"token":"sk_live_2"}`,
		"headers": []map[string]any{
			{"key": "Authorization", "value": "Bearer sk_live_3", "enabled": true},
		},
	})

	callToolJSON(t, srv, "update_request", map[string]any{
		"id":        created["id"],
		"version":   created["version"],
		"name":      "Secretive",
		"url":       "https://api.example.com/u?api_key=sk_live_ROTATED",
		"auth_data": `{"token":"sk_live_ROTATED"}`,
		"headers": []map[string]any{
			{"key": "Authorization", "value": "Bearer sk_live_ROTATED", "enabled": true},
		},
	})

	stored, err := srv.reqUC.GetByID(context.Background(), uuid.MustParse(created["id"].(string)))
	require.NoError(t, err)
	assert.Equal(t, "https://api.example.com/u?api_key=sk_live_ROTATED", stored.URL)
	assert.Equal(t, `{"token":"sk_live_ROTATED"}`, stored.AuthData)
	assert.Equal(t, "Bearer sk_live_ROTATED", stored.Headers[0].Value)
}

func TestUpdateRequest_MaskWithNothingBehindItIsRejected(t *testing.T) {
	srv := setupTestServer(t)
	col := callToolJSON(t, srv, "create_collection", map[string]any{"name": "Unknown"})

	created := callToolJSON(t, srv, "create_request", map[string]any{
		"collection_id": col["id"],
		"name":          "Plain",
		"method":        "GET",
		"url":           "https://api.example.com/u",
	})

	res := callTool(t, srv, "update_request", map[string]any{
		"id":      created["id"],
		"version": created["version"],
		"name":    "Plain",
		"headers": []map[string]any{{"key": "X-API-Key", "value": redactedValue, "enabled": true}},
	})
	assert.Contains(t, res, "pass the real value or omit the field")

	stored, err := srv.reqUC.GetByID(context.Background(), uuid.MustParse(created["id"].(string)))
	require.NoError(t, err)
	assert.Empty(t, stored.Headers)
}

func TestCreateRequest_RejectsMask(t *testing.T) {
	srv := setupTestServer(t)
	col := callToolJSON(t, srv, "create_collection", map[string]any{"name": "NoMask"})

	cases := map[string]map[string]any{
		"url":           {"url": "https://api.example.com/u?api_key=" + redactedValue},
		"auth_data":     {"auth_data": redactedValue},
		"header":        {"headers": []map[string]any{{"key": "Authorization", "value": redactedValue}}},
		"grpc_metadata": {"grpc_metadata": map[string]any{"authorization": []any{redactedValue}}},
	}
	for name, extra := range cases {
		args := map[string]any{"collection_id": col["id"], "name": "Nope", "method": "GET"}
		for k, v := range extra {
			args[k] = v
		}
		res := callTool(t, srv, "create_request", args)
		assert.Contains(t, res, "pass the real value or omit the field", "case %s", name)
	}

	listed := callToolJSON(t, srv, "list_requests", map[string]any{"collection_id": col["id"]})
	assert.Equal(t, float64(0), listed["count"])
}

func TestCreateCollection_RejectsMaskedAuthData(t *testing.T) {
	srv := setupTestServer(t)
	res := callTool(t, srv, "create_collection", map[string]any{"name": "Nope", "auth_data": redactedValue})
	assert.Contains(t, res, "pass the real value or omit the field")
}

func TestUpdateRequest_GRPCMetadataMaskedAndRestored(t *testing.T) {
	srv := setupTestServer(t)
	col := callToolJSON(t, srv, "create_collection", map[string]any{"name": "GRPC"})

	created := callToolJSON(t, srv, "create_request", map[string]any{
		"collection_id": col["id"],
		"name":          "Streamer",
		"protocol":      "grpc",
		"url":           "localhost:50051",
		"grpc_service":  "svc.Greeter",
		"grpc_method":   "SayHello",
		"grpc_metadata": map[string]any{
			"authorization": []any{"Bearer grpc_secret"},
			"x-trace":       []any{"on"},
		},
	})

	md := created["grpc_metadata"].(map[string]any)
	assert.Equal(t, []any{redactedValue}, md["authorization"], "gRPC metadata carries the same token as a header")
	assert.Equal(t, []any{"on"}, md["x-trace"])

	callToolJSON(t, srv, "update_request", map[string]any{
		"id":            created["id"],
		"version":       created["version"],
		"name":          "Streamer",
		"grpc_metadata": md,
	})

	stored, err := srv.reqUC.GetByID(context.Background(), uuid.MustParse(created["id"].(string)))
	require.NoError(t, err)
	assert.Equal(t, []string{"Bearer grpc_secret"}, stored.GRPCMetadata["authorization"])
	assert.Equal(t, []string{"on"}, stored.GRPCMetadata["x-trace"])
}

func TestMaskHeaderItems_PartialTemplateStaysMasked(t *testing.T) {
	out := maskHeaderItems([]entities.HeaderItem{
		{Key: "Cookie", Value: "session=REALSECRET; theme={{theme}}"},
		{Key: "Cookie", Value: " {{session}} "},
		{Key: "Authorization", Value: "Bearer abc{{suffix}}"},
		{Key: "Authorization", Value: "Bearer {{a}}.{{b}}"},
	})
	assert.Equal(t, redactedValue, out[0].Value)
	assert.Equal(t, " {{session}} ", out[1].Value)
	assert.Equal(t, redactedValue, out[2].Value)
	assert.Equal(t, redactedValue, out[3].Value)
}

func TestMaskHeaderItems_SchemeWithReferenceStaysVisible(t *testing.T) {
	out := maskHeaderItems([]entities.HeaderItem{
		{Key: "Authorization", Value: "Bearer {{token}}"},
		{Key: "Authorization", Value: "Basic {{credentials}}"},
		{Key: "X-API-Key", Value: "ApiKey {{key}}"},
		{Key: "Authorization", Value: "Bearer abc123"},
		{Key: "Authorization", Value: "Basic dXNlcjpwYXNz"},
		{Key: "Authorization", Value: "Bearer {{a}}{{b}}"},
	})
	assert.Equal(t, "Bearer {{token}}", out[0].Value)
	assert.Equal(t, "Basic {{credentials}}", out[1].Value)
	assert.Equal(t, "ApiKey {{key}}", out[2].Value)
	assert.Equal(t, redactedValue, out[3].Value)
	assert.Equal(t, redactedValue, out[4].Value)
	assert.Equal(t, "Bearer {{a}}{{b}}", out[5].Value)

	md := maskResponseHeaders(map[string][]string{"authorization": {"Bearer {{token}}", "Bearer abc123"}})
	assert.Equal(t, []string{"Bearer {{token}}", redactedValue}, md["authorization"])
}

func TestUpdateCollection_KeepsOmittedFields(t *testing.T) {
	srv := setupTestServer(t)

	created := callToolJSON(t, srv, "create_collection", map[string]any{
		"name":        "Keep",
		"description": "docs",
		"pre_script":  "pre()",
		"post_script": "post()",
		"auth_type":   "bearer",
		"auth_data":   `{"token":"sk_live_1"}`,
	})
	require.Equal(t, redactedValue, created["auth_data"])

	callToolJSON(t, srv, "update_collection", map[string]any{
		"id":      created["id"],
		"version": created["version"],
		"name":    "Renamed",
	})

	stored, err := srv.colUC.GetByID(context.Background(), uuid.MustParse(created["id"].(string)))
	require.NoError(t, err)
	assert.Equal(t, "Renamed", stored.Name)
	assert.Equal(t, "docs", stored.Description)
	assert.Equal(t, "pre()", stored.PreScript)
	assert.Equal(t, "post()", stored.PostScript)
	assert.Equal(t, entities.AuthTypeBearer, stored.AuthType)
	assert.Equal(t, `{"token":"sk_live_1"}`, stored.AuthData)

	// The mask is what get_collection showed; echoing it back must not persist it.
	callToolJSON(t, srv, "update_collection", map[string]any{
		"id":        created["id"],
		"version":   stored.Version,
		"name":      "Renamed",
		"auth_data": redactedValue,
	})
	stored, err = srv.colUC.GetByID(context.Background(), uuid.MustParse(created["id"].(string)))
	require.NoError(t, err)
	assert.Equal(t, `{"token":"sk_live_1"}`, stored.AuthData)
}

// The mask replaces the whole auth_data field, so a marker buried in JSON is an
// agent guessing at a shape it never saw — and it used to be stored verbatim.
func TestCreateRequest_RejectsMaskNestedInAuthData(t *testing.T) {
	srv := setupTestServer(t)
	col := callToolJSON(t, srv, "create_collection", map[string]any{"name": "Nested"})

	res := callTool(t, srv, "create_request", map[string]any{
		"collection_id": col["id"],
		"name":          "Nope",
		"auth_type":     "bearer",
		"auth_data":     `{"token":"` + redactedValue + `"}`,
	})
	assert.Contains(t, res, "pass the real value or omit the field")

	listed := callToolJSON(t, srv, "list_requests", map[string]any{"collection_id": col["id"]})
	assert.Equal(t, float64(0), listed["count"])
}

func TestCreateCollection_RejectsMaskNestedInAuthData(t *testing.T) {
	srv := setupTestServer(t)
	res := callTool(t, srv, "create_collection", map[string]any{
		"name":      "Nope",
		"auth_type": "bearer",
		"auth_data": `{"token":"` + redactedValue + `"}`,
	})
	assert.Contains(t, res, "pass the real value or omit the field")

	listed := callToolJSON(t, srv, "list_collections", map[string]any{})
	assert.Equal(t, float64(0), listed["count"])
}

func TestUpdateRequest_MaskNestedInAuthDataKeepsStoredSecret(t *testing.T) {
	srv := setupTestServer(t)
	col := callToolJSON(t, srv, "create_collection", map[string]any{"name": "Nested"})
	created := callToolJSON(t, srv, "create_request", map[string]any{
		"collection_id": col["id"],
		"name":          "Bearer",
		"auth_type":     "bearer",
		"auth_data":     `{"token":"sk_live_1"}`,
	})
	require.Equal(t, redactedValue, created["auth_data"])

	res := callTool(t, srv, "update_request", map[string]any{
		"id":        created["id"],
		"version":   created["version"],
		"auth_data": `{"token":"` + redactedValue + `"}`,
	})
	assert.Contains(t, res, "pass the real value or omit the field")

	stored, err := srv.reqUC.GetByID(context.Background(), uuid.MustParse(created["id"].(string)))
	require.NoError(t, err)
	assert.Equal(t, `{"token":"sk_live_1"}`, stored.AuthData)
}

func TestUpdateCollection_MaskNestedInAuthDataKeepsStoredSecret(t *testing.T) {
	srv := setupTestServer(t)
	created := callToolJSON(t, srv, "create_collection", map[string]any{
		"name":      "Nested",
		"auth_type": "bearer",
		"auth_data": `{"token":"sk_live_1"}`,
	})
	require.Equal(t, redactedValue, created["auth_data"])

	res := callTool(t, srv, "update_collection", map[string]any{
		"id":        created["id"],
		"version":   created["version"],
		"auth_data": `{"token":"` + redactedValue + `"}`,
	})
	assert.Contains(t, res, "pass the real value or omit the field")

	stored, err := srv.colUC.GetByID(context.Background(), uuid.MustParse(created["id"].(string)))
	require.NoError(t, err)
	assert.Equal(t, `{"token":"sk_live_1"}`, stored.AuthData)
}

// Same hole one level down: "Bearer [redacted]" is not the mask itself, so it
// slipped past the equality check and replaced the credential.
func TestUpdateRequest_MaskEmbeddedInHeaderKeepsStoredSecret(t *testing.T) {
	srv := setupTestServer(t)
	col := callToolJSON(t, srv, "create_collection", map[string]any{"name": "Embedded"})
	created := callToolJSON(t, srv, "create_request", map[string]any{
		"collection_id": col["id"],
		"name":          "Headered",
		"headers": []map[string]any{
			{"key": "Authorization", "value": "Bearer sk_live_1", "enabled": true},
		},
	})

	res := callTool(t, srv, "update_request", map[string]any{
		"id":      created["id"],
		"version": created["version"],
		"headers": []map[string]any{
			{"key": "Authorization", "value": "Bearer " + redactedValue, "enabled": true},
		},
	})
	assert.Contains(t, res, "pass the real value or omit the field")

	stored, err := srv.reqUC.GetByID(context.Background(), uuid.MustParse(created["id"].(string)))
	require.NoError(t, err)
	assert.Equal(t, "Bearer sk_live_1", stored.Headers[0].Value)
}

func TestUpdateRequest_MaskEmbeddedInMetadataKeepsStoredSecret(t *testing.T) {
	srv := setupTestServer(t)
	col := callToolJSON(t, srv, "create_collection", map[string]any{"name": "EmbeddedMD"})
	created := callToolJSON(t, srv, "create_request", map[string]any{
		"collection_id": col["id"],
		"name":          "Streamer",
		"protocol":      "grpc",
		"url":           "localhost:50051",
		"grpc_metadata": map[string]any{"authorization": []any{"Bearer grpc_secret"}},
	})

	res := callTool(t, srv, "update_request", map[string]any{
		"id":            created["id"],
		"version":       created["version"],
		"grpc_metadata": map[string]any{"authorization": []any{"Bearer " + redactedValue}},
	})
	assert.Contains(t, res, "pass the real value or omit the field")

	stored, err := srv.reqUC.GetByID(context.Background(), uuid.MustParse(created["id"].(string)))
	require.NoError(t, err)
	assert.Equal(t, []string{"Bearer grpc_secret"}, stored.GRPCMetadata["authorization"])
}

// The receiver decodes "access%5Ftoken" as "access_token", so classification has
// to decode too — otherwise the credential is handed to the agent in full.
func TestMaskURLSecrets_PercentEncodedNames(t *testing.T) {
	cases := []struct{ in, want string }{
		{"https://api.example.com/u?access%5Ftoken=sk_live_1",
			"https://api.example.com/u?access%5Ftoken=" + redactedValue},
		{"https://api.example.com/u?API%5FKEY=sk_live_1",
			"https://api.example.com/u?API%5FKEY=" + redactedValue},
		{"https://api.example.com/u?%70assword=hunter2",
			"https://api.example.com/u?%70assword=" + redactedValue},
		// A stray percent is not an encoding; the name stays as written.
		{"https://api.example.com/u?100%off=yes", "https://api.example.com/u?100%off=yes"},
	}
	for _, c := range cases {
		assert.Equal(t, c.want, maskURLSecrets(c.in), "input %q", c.in)
	}
}

func TestUpdateRequest_PercentEncodedSecretParamRoundTrips(t *testing.T) {
	srv := setupTestServer(t)
	col := callToolJSON(t, srv, "create_collection", map[string]any{"name": "Encoded"})
	created := callToolJSON(t, srv, "create_request", map[string]any{
		"collection_id": col["id"],
		"name":          "Encoded",
		"url":           "https://api.example.com/u?access%5Ftoken=sk_live_1&page=2",
	})
	require.Equal(t, "https://api.example.com/u?access%5Ftoken="+redactedValue+"&page=2", created["url"])

	callToolJSON(t, srv, "update_request", map[string]any{
		"id":      created["id"],
		"version": created["version"],
		"url":     created["url"],
	})

	stored, err := srv.reqUC.GetByID(context.Background(), uuid.MustParse(created["id"].(string)))
	require.NoError(t, err)
	assert.Equal(t, "https://api.example.com/u?access%5Ftoken=sk_live_1&page=2", stored.URL)
}

func TestMaskSecrets_NewAuthSchemeKeys(t *testing.T) {
	headers := maskHeaderItems([]entities.HeaderItem{
		{Key: "X-Amz-Security-Token", Value: "FQoGZXIvYXdz", Enabled: true},
		{Key: "x-amz-content-sha256", Value: "e3b0c442", Enabled: true},
		{Key: "X-Amz-Date", Value: "20260904T000000Z", Enabled: true},
	})
	assert.Equal(t, redactedValue, headers[0].Value)
	assert.Equal(t, redactedValue, headers[1].Value)
	assert.Equal(t, "20260904T000000Z", headers[2].Value, "the signing date is not a credential")

	resp := maskResponseHeaders(map[string][]string{
		"Www-Authenticate": {`Digest realm="x", nonce="abc"`},
	})
	assert.Equal(t, []string{redactedValue}, resp["Www-Authenticate"])

	cases := []struct{ in, want string }{
		{"https://idp.example/cb?code=abc&state=s", "https://idp.example/cb?code=" + redactedValue + "&state=s"},
		{"https://idp.example/t?code_verifier=v", "https://idp.example/t?code_verifier=" + redactedValue},
		{"https://idp.example/t?client_secret=s", "https://idp.example/t?client_secret=" + redactedValue},
		{"https://idp.example/t?assertion=jwt", "https://idp.example/t?assertion=" + redactedValue},
		{
			"https://s3.example/o?X-Amz-Credential=AKIA%2F20260904&X-Amz-Signature=deadbeef",
			"https://s3.example/o?X-Amz-Credential=" + redactedValue + "&X-Amz-Signature=" + redactedValue,
		},
		{
			"https://s3.example/o?X-Amz-Security-Token=FQoGZXIvYXdz&X-Amz-Date=20260904T000000Z",
			"https://s3.example/o?X-Amz-Security-Token=" + redactedValue + "&X-Amz-Date=20260904T000000Z",
		},
		{"https://idp.example/ti?id_token=eyJhbGci", "https://idp.example/ti?id_token=" + redactedValue},
	}
	for _, c := range cases {
		assert.Equal(t, c.want, maskURLSecrets(c.in), "input %q", c.in)
	}
}

func TestMaskSecrets_SharedClassification(t *testing.T) {
	headers := maskHeaderItems([]entities.HeaderItem{
		{Key: "X-Vault-Token", Value: "s.abc", Enabled: true},
		{Key: "X-Client-Secret-V2", Value: "abc", Enabled: true},
		{Key: "{{authHeader}}", Value: "literal", Enabled: true},
		{Key: "{{authHeader}}", Value: "{{authValue}}", Enabled: true},
	})
	assert.Equal(t, redactedValue, headers[0].Value)
	assert.Equal(t, redactedValue, headers[1].Value)
	assert.Equal(t, redactedValue, headers[2].Value)
	assert.Equal(t, "{{authValue}}", headers[3].Value)

	assert.Equal(t, "https://api.example.com/u?my_session="+redactedValue+"&page=2",
		maskURLSecrets("https://api.example.com/u?my_session=abc&page=2"))
}

func TestMaskSecrets_WholeNameTokensOnly(t *testing.T) {
	assert.Equal(t, "https://api.example.com/search?keyword=shoes&author=bob&page=2",
		maskURLSecrets("https://api.example.com/search?keyword=shoes&author=bob&page=2"))

	resp := maskResponseHeaders(map[string][]string{
		"Access-Control-Allow-Credentials": {"true"},
		"Idempotency-Key":                  {"k-1"},
	})
	assert.Equal(t, []string{"true"}, resp["Access-Control-Allow-Credentials"])
	assert.Equal(t, []string{redactedValue}, resp["Idempotency-Key"])
}

func TestSendRequest_ScriptOutputHidesSecretsMaskedElsewhere(t *testing.T) {
	const token = "tok-0123456789abcdefghijklmnopqrstuvwxyz"
	const cookie = "sid=8f14e45fceea167a"
	srv := setupExecServer(t,
		&stubHTTPRequester{response: &entities.Response{
			StatusCode: 200, StatusText: "200 OK", Protocol: entities.ProtocolHTTP,
			Headers: map[string][]string{"Set-Cookie": {cookie}, "X-Trace": {"t-1"}},
		}},
		&stubEnvResolver{vars: map[string]string{"token": token, "region": "eu-west"}},
		scriptengine.NewGojaEngine(),
		&stubScriptResolver{post: `
			console.log("token " + pm.environment.get("token"));
			pm.test("token", function () { pm.expect(pm.environment.get("token")).to.equal("other") });
			pm.test("cookie", function () { pm.response.to.have.header("Set-Cookie", "sid=other") });
			pm.test("region", function () { pm.expect(pm.environment.get("region")).to.equal("us") });
		`},
	)

	env := callToolJSON(t, srv, "create_environment", map[string]any{"name": "Prod"})
	callToolJSON(t, srv, "create_variable", map[string]any{"environment_id": env["id"], "key": "token", "value": token, "is_secret": true})
	callToolJSON(t, srv, "create_variable", map[string]any{"environment_id": env["id"], "key": "region", "value": "eu-west"})
	require.NoError(t, srv.envUC.SetActive(context.Background(), environment.SetActiveOpt{
		WorkspaceID: uuid.MustParse(defaultWorkspaceID), EnvironmentID: uuid.MustParse(env["id"].(string)),
	}))

	col := callToolJSON(t, srv, "create_collection", map[string]any{"name": "API"})
	created := callToolJSON(t, srv, "create_request", map[string]any{"collection_id": col["id"], "name": "Me", "url": "https://api.example.com/me"})
	raw := callTool(t, srv, "send_request", map[string]any{"id": created["id"]})

	assert.NotContains(t, raw, "0123456789")
	assert.NotContains(t, raw, "8f14e45f")
	assert.Contains(t, raw, "eu-west")
	assert.Contains(t, raw, "t-1")

	var sent struct {
		ScriptResult entities.ScriptResult `json:"script_result"`
	}
	require.NoError(t, json.Unmarshal([]byte(raw), &sent))
	assert.Equal(t, []string{"token [redacted]"}, sent.ScriptResult.PostConsole)
	require.Len(t, sent.ScriptResult.Tests, 3)
	assert.Equal(t, "AssertionError: expected '[redacted]' to equal 'other'", sent.ScriptResult.Tests[0].Error)
	assert.Equal(t, "AssertionError: expected response header 'Set-Cookie' to be 'sid=other' but got '[redacted]'", sent.ScriptResult.Tests[1].Error)
	assert.Equal(t, "AssertionError: expected 'eu-west' to equal 'us'", sent.ScriptResult.Tests[2].Error)
}

func TestSendRequest_ScriptOutputHidesTheRequestsOwnAuth(t *testing.T) {
	const bearer = "tok_live_0123456789abcdef"
	const apiKey = "ak_live_fedcba9876543210"
	srv := setupExecServer(t,
		&stubHTTPRequester{response: &entities.Response{
			StatusCode: 200, StatusText: "200 OK", Protocol: entities.ProtocolHTTP,
			Headers: map[string][]string{}, Body: `{"auth":"Bearer ` + bearer + `"}`,
		}},
		&stubEnvResolver{vars: map[string]string{"apiKey": apiKey}},
		scriptengine.NewGojaEngine(),
		&stubScriptResolver{post: `
			console.log("key " + pm.environment.get("apiKey"));
			pm.test("echo", function () { pm.expect(pm.response.json().auth).to.equal("Bearer other") });
		`},
	)

	keyed := callToolJSON(t, srv, "create_collection", map[string]any{
		"name": "Keyed", "auth_type": "api_key", "auth_data": `{"key":"appid","value":"{{apiKey}}","addTo":"header"}`,
	})
	inherits := callToolJSON(t, srv, "create_request", map[string]any{"collection_id": keyed["id"], "name": "Inherits", "url": "https://api.example.com/me"})
	own := callToolJSON(t, srv, "create_request", map[string]any{
		"collection_id": keyed["id"], "name": "Own", "url": "https://api.example.com/me",
		"auth_type": "bearer", "auth_data": `{"token":"` + bearer + `"}`,
	})

	var sent struct {
		ScriptResult entities.ScriptResult `json:"script_result"`
	}
	require.NoError(t, json.Unmarshal([]byte(callTool(t, srv, "send_request", map[string]any{"id": inherits["id"]})), &sent))
	assert.Equal(t, []string{"key [redacted]"}, sent.ScriptResult.PostConsole)

	require.NoError(t, json.Unmarshal([]byte(callTool(t, srv, "send_request", map[string]any{"id": own["id"]})), &sent))
	require.Len(t, sent.ScriptResult.Tests, 1)
	assert.Equal(t, "AssertionError: expected 'Bearer [redacted]' to equal 'Bearer other'", sent.ScriptResult.Tests[0].Error)
	assert.Equal(t, []string{"key " + apiKey}, sent.ScriptResult.PostConsole)
}

type echoAuthRequester struct{}

func (echoAuthRequester) Execute(_ context.Context, req request.HTTPExecuteRequest) (*entities.Response, error) {
	return &entities.Response{
		StatusCode: 200, StatusText: "200 OK", Protocol: entities.ProtocolHTTP,
		Headers: map[string][]string{}, Body: strings.Join(append(req.Headers["Authorization"], req.Headers["Cookie"]...), ","),
	}, nil
}

func TestSendRequest_ScriptOutputHidesTheCredentialsActuallySent(t *testing.T) {
	srv := setupExecServer(t, echoAuthRequester{},
		&stubEnvResolver{vars: map[string]string{"token": "tok_stored_0123456789"}},
		scriptengine.NewGojaEngine(),
		&stubScriptResolver{
			pre:  `if (pm.request.url.indexOf("rotate") >= 0) pm.environment.set("token", "tok_rotated_9876543210");`,
			post: `console.log("url " + pm.request.url); console.log("sent " + pm.response.text());`,
		},
	)
	col := callToolJSON(t, srv, "create_collection", map[string]any{"name": "C"})
	send := func(fields map[string]any) []string {
		t.Helper()
		fields["collection_id"] = col["id"]
		r := callToolJSON(t, srv, "create_request", fields)
		var sent struct {
			ScriptResult entities.ScriptResult `json:"script_result"`
		}
		out := callTool(t, srv, "send_request", map[string]any{"id": r["id"]})
		require.NoError(t, json.Unmarshal([]byte(out), &sent), out)
		return sent.ScriptResult.PostConsole
	}
	const jwt = `{"alg":"HS256","secret":"supersecretsigningkey123","payload":"{\"sub\":\"1\"}"`

	assert.Equal(t, []string{"url https://api.example.com/me", "sent Bearer [redacted]"}, send(map[string]any{
		"name": "JWT", "url": "https://api.example.com/me", "auth_type": "jwt", "auth_data": jwt + `}`,
	}))
	assert.Equal(t, []string{"url https://api.example.com/me?token=[redacted]", "sent "}, send(map[string]any{
		"name": "JWT in query", "url": "https://api.example.com/me", "auth_type": "jwt", "auth_data": jwt + `,"addTo":"query"}`,
	}))
	assert.Equal(t, []string{"url https://api.example.com/rotate", "sent Bearer [redacted]"}, send(map[string]any{
		"name": "Rotated", "url": "https://api.example.com/rotate", "auth_type": "bearer", "auth_data": `{"token":"{{token}}"}`,
	}))
	assert.Equal(t, []string{"url https://api.example.com/me?appid=[redacted]", "sent "}, send(map[string]any{
		"name": "Query key", "url": "https://api.example.com/me",
		"auth_type": "api_key", "auth_data": `{"key":"appid","value":"k+y/z=1&2 ok","addTo":"query"}`,
	}))
	assert.Equal(t, []string{"url https://api.example.com/me", "sent [redacted]"}, send(map[string]any{
		"name": "Cookie", "url": "https://api.example.com/me",
		"headers": []map[string]any{{"key": "Cookie", "value": "sid=abc123; theme=dark", "enabled": true}},
	}))
}

func TestRedactScriptResult_MasksEachCookieSent(t *testing.T) {
	sent := &entities.SentRequest{Headers: map[string][]string{"Cookie": {"sid=manual_secret_123456; theme=dark; id=7"}}}

	r := redactScriptResult(&entities.ScriptResult{PostConsole: []string{"sid manual_secret_123456", "id 7"}}, sentSecrets(sent))

	assert.Equal(t, []string{"sid [redacted]", "id 7"}, r.PostConsole)
}

func TestRedactScriptResult_LargeSecretStaysCheap(t *testing.T) {
	secret := strings.Repeat("0123456789abcdef", 4<<10)
	r := &entities.ScriptResult{PostConsole: []string{"whole " + secret, "cut '" + secret[:37] + "…'"}}

	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	start := time.Now()
	out := redactScriptResult(r, []string{secret})
	elapsed := time.Since(start)
	runtime.ReadMemStats(&after)

	assert.Equal(t, []string{"whole [redacted]", "cut '[redacted]'"}, out.PostConsole)
	assert.Less(t, elapsed, time.Second*race.Slowdown)
	assert.Less(t, after.TotalAlloc-before.TotalAlloc, uint64(8<<20))
}
