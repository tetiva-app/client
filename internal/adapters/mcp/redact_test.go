package mcp

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tetiva-app/client/internal/domain/entities"
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
	assert.Equal(t, redactedValue, headers[0].(map[string]any)["Value"])
	assert.Equal(t, "on", headers[1].(map[string]any)["Value"])

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

// headerArgs turns a serialized headers array back into tool arguments, which is
// what an agent editing a request it just read has to do.
func headerArgs(t *testing.T, serialized any) []map[string]any {
	t.Helper()
	items, ok := serialized.([]any)
	require.True(t, ok, "headers is not an array: %v", serialized)
	out := make([]map[string]any, 0, len(items))
	for _, item := range items {
		h := item.(map[string]any)
		out = append(out, map[string]any{"key": h["Key"], "value": h["Value"], "enabled": h["Enabled"]})
	}
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
	echoed := append(headerArgs(t, read["headers"]),
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
		{Key: "Authorization", Value: "Bearer {{token}}"},
	})
	assert.Equal(t, redactedValue, out[0].Value)
	assert.Equal(t, " {{session}} ", out[1].Value)
	assert.Equal(t, redactedValue, out[2].Value)
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
