package dto

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tetiva-app/client/internal/domain"
	"github.com/tetiva-app/client/internal/domain/entities"
	"github.com/tetiva-app/client/internal/domain/har"
	"github.com/tetiva-app/client/internal/domain/usecase/request"
)

func marshalSnippet(t *testing.T, in request.SnippetInput) string {
	t.Helper()
	b, err := json.Marshal(SnippetInputToDTO(in))
	require.NoError(t, err)
	return string(b)
}

func TestSnippetInputToDTO_EmptyGETHasArraysNotNull(t *testing.T) {
	got := marshalSnippet(t, request.SnippetInput{
		Protocol: entities.ProtocolHTTP,
		HAR:      &har.Request{Method: "GET", URL: "https://a.io", HTTPVersion: "HTTP/1.1"},
	})

	assert.JSONEq(t, `{
		"protocol": "http",
		"har": {
			"method": "GET", "url": "https://a.io", "httpVersion": "HTTP/1.1",
			"headers": [], "queryString": [], "cookies": [],
			"headersSize": -1, "bodySize": -1
		},
		"warnings": []
	}`, got)
	assert.NotContains(t, got, "null")
}

func TestSnippetInputToDTO_KeepsEmptyValues(t *testing.T) {
	got := marshalSnippet(t, request.SnippetInput{
		Protocol: entities.ProtocolHTTP,
		HAR: &har.Request{
			Method:      "POST",
			URL:         "https://a.io/login",
			HTTPVersion: "HTTP/1.1",
			QueryString: []har.NameValue{{Name: "flag"}},
			PostData: &har.PostData{
				MimeType: "application/x-www-form-urlencoded",
				Params:   []har.Param{{Name: "remember"}},
			},
		},
	})

	assert.Contains(t, got, `"queryString":[{"name":"flag","value":""}]`)
	assert.Contains(t, got, `"postData":{"mimeType":"application/x-www-form-urlencoded","text":"","params":[{"name":"remember","value":""}]}`)
}

func TestSnippetInputToDTO_PostDataWithoutParamsHasEmptyArray(t *testing.T) {
	got := marshalSnippet(t, request.SnippetInput{
		Protocol: entities.ProtocolHTTP,
		HAR: &har.Request{
			Method: "POST", URL: "https://a.io", HTTPVersion: "HTTP/1.1",
			PostData: &har.PostData{MimeType: "application/json", Text: `{"a":1}`},
		},
	})

	assert.Contains(t, got, `"postData":{"mimeType":"application/json","text":"{\"a\":1}","params":[]}`)
}

func TestSnippetInputToDTO_TetivaOnlyWithBinaryOrAuthNote(t *testing.T) {
	base := func() *har.Request {
		return &har.Request{Method: "POST", URL: "https://a.io", HTTPVersion: "HTTP/1.1"}
	}

	plain := marshalSnippet(t, request.SnippetInput{Protocol: entities.ProtocolHTTP, HAR: base()})
	assert.NotContains(t, plain, "_tetiva")

	binary := base()
	binary.BinaryFile = "a.bin"
	assert.Contains(t,
		marshalSnippet(t, request.SnippetInput{Protocol: entities.ProtocolHTTP, HAR: binary}),
		`"_tetiva":{"binaryFile":"a.bin"}`)

	digest := base()
	digest.AuthNote = "digest"
	assert.Contains(t,
		marshalSnippet(t, request.SnippetInput{Protocol: entities.ProtocolHTTP, HAR: digest}),
		`"_tetiva":{"authNote":"digest"}`)
}

func TestSnippetInputToDTO_GRPCCollectionsNotNull(t *testing.T) {
	got := marshalSnippet(t, request.SnippetInput{
		Protocol: entities.ProtocolGRPC,
		GRPC: &request.GRPCSnippet{
			Target: "localhost:50051", Service: "svc.v1.S", Method: "M", Message: "{}",
			Metadata: map[string][]string{"x-empty": nil},
		},
	})

	assert.JSONEq(t, `{
		"protocol": "grpc",
		"grpc": {"target": "localhost:50051", "service": "svc.v1.S", "method": "M", "message": "{}", "metadata": {"x-empty": []}},
		"warnings": []
	}`, got)

	noMetadata := marshalSnippet(t, request.SnippetInput{Protocol: entities.ProtocolGRPC, GRPC: &request.GRPCSnippet{}})
	assert.Contains(t, noMetadata, `"metadata":{}`)
}

func TestSnippetInputToDTO_WSCollectionsNotNull(t *testing.T) {
	got := marshalSnippet(t, request.SnippetInput{
		Protocol: entities.ProtocolWebSocket,
		WS:       &request.WSSnippet{URL: "wss://a.io/ws"},
		Warnings: []string{"w"},
	})

	assert.JSONEq(t, `{
		"protocol": "websocket",
		"ws": {"url": "wss://a.io/ws", "headers": {}, "subprotocols": [], "messages": []},
		"warnings": ["w"]
	}`, got)
}

func TestSnippetInputToDTO_WSMessages(t *testing.T) {
	dto := SnippetInputToDTO(request.SnippetInput{
		Protocol: entities.ProtocolWebSocket,
		WS: &request.WSSnippet{
			URL:          "wss://a.io/ws",
			Headers:      map[string][]string{"Origin": {"https://a.io"}},
			Subprotocols: []string{"graphql-ws"},
			Messages:     []request.WSSnippetMessage{{Name: "hello", Format: "json", Data: `{"a":1}`}},
		},
	})

	require.NotNil(t, dto.WS)
	assert.Equal(t, []WSSnippetMessageDTO{{Name: "hello", Format: "json", Data: `{"a":1}`}}, dto.WS.Messages)
	assert.Equal(t, []string{"graphql-ws"}, dto.WS.Subprotocols)
	assert.Equal(t, map[string][]string{"Origin": {"https://a.io"}}, dto.WS.Headers)
	assert.Nil(t, dto.HAR)
	assert.Nil(t, dto.GRPC)
}

func validSnippetRequestDTO() SnippetRequestDTO {
	return SnippetRequestDTO{
		ID:               uuid.NewString(),
		CollectionID:     uuid.NewString(),
		Protocol:         "graphql",
		Method:           "POST",
		URL:              "{{baseUrl}}/graphql",
		Headers:          []HeaderItemDTO{{Key: "X-A", Value: "1", Enabled: true}},
		Body:             "b",
		BodyType:         "json",
		AuthType:         "bearer",
		AuthData:         `{"token":"t"}`,
		PreScript:        "pm.x()",
		GRPCService:      "svc",
		GRPCMethod:       "m",
		GRPCMetadata:     map[string][]string{"k": {"v"}},
		GraphQLQuery:     "{ me }",
		GraphQLVariables: `{"n":1}`,
		GraphQLOperation: "Me",
	}
}

func TestSnippetRequestDTO_ToEntity_MapsEditorState(t *testing.T) {
	d := validSnippetRequestDTO()

	got, err := d.ToEntity()

	require.NoError(t, err)
	assert.Equal(t, d.ID, got.ID.String())
	assert.Equal(t, d.CollectionID, got.CollectionID.String())
	assert.Equal(t, entities.ProtocolGraphQL, got.Protocol)
	assert.Equal(t, entities.HTTPMethod("POST"), got.Method)
	assert.Equal(t, d.URL, got.URL)
	assert.Equal(t, []entities.HeaderItem{{Key: "X-A", Value: "1", Enabled: true}}, got.Headers)
	assert.Equal(t, d.Body, got.Body)
	assert.Equal(t, entities.BodyTypeJSON, got.BodyType)
	assert.Equal(t, entities.AuthTypeBearer, got.AuthType)
	assert.Equal(t, d.AuthData, got.AuthData)
	assert.Equal(t, d.PreScript, got.PreScript)
	assert.Equal(t, d.GRPCService, got.GRPCService)
	assert.Equal(t, d.GRPCMethod, got.GRPCMethod)
	assert.Equal(t, d.GRPCMetadata, got.GRPCMetadata)
	assert.Equal(t, d.GraphQLQuery, got.GraphQLQuery)
	assert.Equal(t, d.GraphQLVariables, got.GraphQLVariables)
	assert.Equal(t, d.GraphQLOperation, got.GraphQLOperation)
}

func TestSnippetRequestDTO_ToEntity_InvalidIDs(t *testing.T) {
	for _, tc := range []struct {
		field string
		edit  func(*SnippetRequestDTO)
	}{
		{"collectionId", func(d *SnippetRequestDTO) { d.CollectionID = "nope" }},
		{"id", func(d *SnippetRequestDTO) { d.ID = "" }},
	} {
		t.Run(tc.field, func(t *testing.T) {
			d := validSnippetRequestDTO()
			tc.edit(&d)

			got, err := d.ToEntity()

			assert.Nil(t, got)
			var vErr *domain.ValidationError
			require.True(t, errors.As(err, &vErr), "want ValidationError, got %v", err)
			assert.NotEmpty(t, vErr.Fields[tc.field])
		})
	}
}

// snippetFromHAR mirrors BuildSnippetInput: references are protected before Build and restored after it.
func snippetFromHAR(in har.Input) request.SnippetInput {
	ph := har.NewPlaceholders(harTexts(in)...)
	req, warnings := har.Build(protectHARInput(ph, in))
	ph.RestoreRequest(&req)
	for i, w := range warnings {
		warnings[i] = ph.Restore(w)
	}
	return request.SnippetInput{Protocol: entities.ProtocolHTTP, HAR: &req, Warnings: warnings}
}

func harTexts(in har.Input) []string {
	texts := []string{in.URL, in.Body.Text, in.Body.BinaryFile}
	for k, values := range in.Headers {
		texts = append(texts, k)
		texts = append(texts, values...)
	}
	for _, p := range in.Body.Params {
		texts = append(texts, p.Name, p.Value, p.FileName)
	}
	return texts
}

// protectHARInput covers every field BuildSnippetInput protects: the request body holds the form
// fields and the binary path, so they are tokens by the time Build sees them.
func protectHARInput(ph *har.Placeholders, in har.Input) har.Input {
	out := in
	out.URL = ph.Protect(in.URL, nil)
	out.Headers = make(map[string][]string, len(in.Headers))
	for k, values := range in.Headers {
		vs := make([]string, len(values))
		for i, v := range values {
			vs[i] = ph.Protect(v, nil)
		}
		out.Headers[ph.Protect(k, nil)] = vs
	}
	out.Body.Text = ph.Protect(in.Body.Text, nil)
	out.Body.BinaryFile = ph.Protect(in.Body.BinaryFile, nil)
	out.Body.Params = slices.Clone(in.Body.Params)
	for i := range out.Body.Params {
		p := &out.Body.Params[i]
		p.Name = ph.Protect(p.Name, nil)
		p.Value = ph.Protect(p.Value, nil)
		p.FileName = ph.Protect(p.FileName, nil)
	}
	return out
}

func TestSnippetFromHAR_ProtectsEveryField(t *testing.T) {
	in := har.Input{
		Method:  "POST",
		URL:     "{{base}}/u?q={{q}}",
		Headers: map[string][]string{"{{h}}": {"{{hv}}"}},
		Body: har.Body{
			Kind:       har.BodyMultipart,
			Text:       "{{text}}",
			Params:     []har.Param{{Name: "{{name}}", Value: "{{value}}"}, {Name: "f", FileName: "{{dir}}/a.pdf"}},
			BinaryFile: "{{dir}}/b.bin",
		},
	}
	ph := har.NewPlaceholders(harTexts(in)...)

	protected := protectHARInput(ph, in)

	raw, err := json.Marshal(protected)
	require.NoError(t, err)
	assert.NotContains(t, string(raw), "{{")
	assert.Equal(t, "{{value}}", in.Body.Params[0].Value, "the caller's params must not change")
}

func snippetContractFixtures() map[string]request.SnippetInput {
	return map[string]request.SnippetInput{
		"get_no_query": snippetFromHAR(har.Input{
			Method:  "GET",
			URL:     "https://api.example.com/users",
			Headers: map[string][]string{"Accept": {"application/json"}},
		}),
		"urlencoded_empty_value": snippetFromHAR(har.Input{
			Method:  "POST",
			URL:     "https://api.example.com/login",
			Headers: map[string][]string{"Content-Type": {"application/x-www-form-urlencoded"}},
			Body: har.Body{
				Kind:     har.BodyURLEncoded,
				MimeType: "application/x-www-form-urlencoded",
				Params:   []har.Param{{Name: "username", Value: "alice"}, {Name: "remember", Value: ""}},
			},
		}),
		"raw_urlencoded": snippetFromHAR(har.Input{
			Method:  "POST",
			URL:     "https://api.example.com/login",
			Headers: map[string][]string{"Content-Type": {"application/x-www-form-urlencoded"}},
			Body: har.Body{
				Kind:     har.BodyText,
				MimeType: "application/x-www-form-urlencoded",
				Text:     "user=ann&pass=p%26ss%3Dw+rd&note=%D0%B4",
			},
		}),
		"multipart_file": snippetFromHAR(har.Input{
			Method:  "POST",
			URL:     "https://api.example.com/upload",
			Headers: map[string][]string{"Content-Type": {"multipart/form-data; boundary=x"}},
			Body: har.Body{
				Kind:     har.BodyMultipart,
				MimeType: "multipart/form-data; boundary=x",
				Params: []har.Param{
					{Name: "title", Value: "Q3 report"},
					{Name: "file", FileName: "/Users/me/docs/report.pdf"},
				},
			},
		}),
		"binary": snippetFromHAR(har.Input{
			Method:  "PUT",
			URL:     "https://api.example.com/blobs/1",
			Headers: map[string][]string{"Content-Type": {"application/octet-stream"}},
			Body: har.Body{
				Kind:       har.BodyBinary,
				MimeType:   "application/octet-stream",
				BinaryFile: "/Users/me/data/payload.bin",
			},
		}),
		"digest_note": snippetFromHAR(har.Input{
			Method:   "GET",
			URL:      "https://api.example.com/secure",
			AuthNote: string(entities.AuthTypeDigest),
		}),
		"unresolved_base": snippetFromHAR(har.Input{
			Method: "POST",
			URL:    "{{baseUrl}}/users/{{id}}?limit={{limit}}&sort=name",
			Headers: map[string][]string{
				"Authorization": {"Bearer {{token}}"},
				"Content-Type":  {"application/json"},
			},
			Body: har.Body{Kind: har.BodyText, MimeType: "application/json", Text: `{"name":"{{name}}"}`},
		}),
		"unsafe_names": snippetFromHAR(har.Input{
			Method: "POST",
			URL:    "https://api.example.com/{{x'; printf injected; #}}?q={{a b}}",
			Headers: map[string][]string{
				"Content-Type": {"application/json"},
				"X-Home":       {"{{$HOME}}"},
			},
			Body: har.Body{Kind: har.BodyText, MimeType: "application/json", Text: `{"v":"{{x'; printf injected; #}}"}`},
		}),
		"grpc": {
			Protocol: entities.ProtocolGRPC,
			GRPC: &request.GRPCSnippet{
				Target:   "localhost:50051",
				Service:  "example.v1.UserService",
				Method:   "GetUser",
				Message:  "{\n  \"id\": \"42\"\n}",
				Metadata: map[string][]string{"authorization": {"Bearer {{token}}"}},
			},
		},
		"ws_no_messages": {
			Protocol: entities.ProtocolWebSocket,
			WS:       &request.WSSnippet{URL: "wss://echo.example.com/ws"},
		},
	}
}

func encodeFixture(v any) ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// The snippet bundle tests (frontend/src/lib/snippets) load these files, so the Go
// wire shape and the TS generators are checked against the same inputs.
func TestSnippetContractFixtures(t *testing.T) {
	dir := filepath.Join("testdata", "snippet")
	update := os.Getenv("UPDATE_SNIPPET_FIXTURES") == "1"
	if update {
		require.NoError(t, os.MkdirAll(dir, 0o755))
	}

	for name, in := range snippetContractFixtures() {
		t.Run(name, func(t *testing.T) {
			want, err := encodeFixture(SnippetInputToDTO(in))
			require.NoError(t, err)
			assert.NotContains(t, string(want), "null")
			assert.NotContains(t, string(want), "tetivaph")

			path := filepath.Join(dir, name+".json")
			if update {
				require.NoError(t, os.WriteFile(path, want, 0o644))
				return
			}
			got, err := os.ReadFile(path)
			require.NoError(t, err, "run with UPDATE_SNIPPET_FIXTURES=1 to create %s", path)
			assert.Equal(t, string(want), string(got), "%s is stale; run with UPDATE_SNIPPET_FIXTURES=1", path)
		})
	}
}

func TestSnippetContractFixtures_NoStrayFiles(t *testing.T) {
	known := snippetContractFixtures()
	entries, err := os.ReadDir(filepath.Join("testdata", "snippet"))
	require.NoError(t, err)
	for _, e := range entries {
		name := strings.TrimSuffix(e.Name(), ".json")
		_, ok := known[name]
		assert.True(t, ok, "testdata/snippet/%s has no fixture in snippetContractFixtures", e.Name())
	}
}
