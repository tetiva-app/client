package postman_test

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tetiva-app/client/internal/adapters/portability/postman"
	"github.com/tetiva-app/client/internal/domain"
	"github.com/tetiva-app/client/internal/domain/entities"
	"github.com/tetiva-app/client/internal/domain/usecase/example"
)

func importWithExamples(t *testing.T, data []byte) (*postman.ImportResult, *stubRequestUC, *stubExampleUC) {
	t.Helper()
	reqUC, exUC := &stubRequestUC{}, &stubExampleUC{}
	res, err := postman.ImportCollection(context.Background(), data, postman.ImportOpts{
		WorkspaceID: uuid.New(), UserID: "local_user",
	}, &stubCollectionUC{}, reqUC, exUC, &stubEnvironmentUC{})
	require.NoError(t, err)
	return res, reqUC, exUC
}

func examplesOf(created []example.Create, requestID uuid.UUID) []example.Create {
	var out []example.Create
	for _, c := range created {
		if c.RequestID == requestID {
			out = append(out, c)
		}
	}
	return out
}

func TestImportCollection_Examples(t *testing.T) {
	data, err := os.ReadFile("testdata/examples.postman_collection.json")
	require.NoError(t, err)

	res, reqUC, exUC := importWithExamples(t, data)
	require.Len(t, reqUC.created, 3)
	assert.Equal(t, 3, res.RequestsCreated)

	getUser := examplesOf(exUC.created, reqUC.ids[0])
	require.Len(t, getUser, 2, "the broken example between the two valid ones is skipped alone")

	found := getUser[0]
	assert.Equal(t, "Found", found.Name)
	assert.Equal(t, 200, found.StatusCode)
	assert.Equal(t, "OK", found.StatusText)
	assert.Equal(t, `{"id":1}`, found.Body)
	assert.Equal(t, "application/json", found.ContentType)
	assert.Equal(t, entities.ProtocolHTTP, found.Protocol)
	assert.Equal(t, []entities.HeaderItem{
		{Key: "content-type", Value: "application/json", Enabled: true},
		{Key: "Set-Cookie", Value: "a=1", Enabled: true},
		{Key: "Set-Cookie", Value: "b=2", Enabled: true},
		{Key: "Vary", Value: "Accept", Enabled: true},
		{Key: "Vary", Value: "Origin", Enabled: true},
	}, found.Headers)

	notFound := getUser[1]
	assert.Equal(t, "404 Not Found", notFound.Name)
	assert.Equal(t, 404, notFound.StatusCode)
	assert.NotNil(t, notFound.Headers)
	assert.Empty(t, notFound.Headers)
	assert.Empty(t, notFound.Body)

	odd := examplesOf(exUC.created, reqUC.ids[1])
	require.Len(t, odd, 3)
	assert.Equal(t, "Example", odd[0].Name)
	assert.Equal(t, "Out of range", odd[1].Name)
	assert.Equal(t, 0, odd[1].StatusCode)
	assert.Equal(t, "Negative", odd[2].Name)
	assert.Equal(t, 0, odd[2].StatusCode)

	gql := examplesOf(exUC.created, reqUC.ids[2])
	require.Len(t, gql, 1)
	assert.Equal(t, entities.ProtocolGraphQL, gql[0].Protocol)
	assert.Equal(t, "application/json; charset=utf-8", gql[0].ContentType)

	warnings := strings.Join(res.Warnings, "\n")
	assert.Contains(t, warnings, `request "Get user": example 2 skipped: "code" must be a whole number, got string`)
	assert.Contains(t, warnings, `request "Odd examples": example 2: status code 1500 is out of range, imported as 0`)
	assert.Contains(t, warnings, `request "Odd examples": example 3: status code -1 is out of range, imported as 0`)
	assert.Contains(t, warnings, `request "Odd examples": example 4 skipped: not an object (got string)`)
	assert.Contains(t, warnings, `request "Odd examples": example 5 skipped: "code" must be a whole number, got number 99999999999999999999999`)
	assert.Len(t, res.Warnings, 5)
}

func singleRequestCollection(t *testing.T, responses ...any) []byte {
	t.Helper()
	raw, err := json.Marshal(map[string]any{
		"info": map[string]any{"name": "Limits", "schema": postman.SchemaV21},
		"item": []any{map[string]any{
			"name":     "Ping",
			"request":  map[string]any{"method": "GET", "url": map[string]any{"raw": "/ping"}},
			"response": responses,
		}},
	})
	require.NoError(t, err)
	return raw
}

func TestImportCollection_ExampleBodyLimitCountsBytes(t *testing.T) {
	atLimit := strings.Repeat("€", domain.MaxExampleBodyLen/3) + strings.Repeat("a", domain.MaxExampleBodyLen%3)
	require.Equal(t, domain.MaxExampleBodyLen, len(atLimit))
	overLimit := strings.Repeat("€", domain.MaxExampleBodyLen/3+1)
	require.Less(t, utf8.RuneCountInString(overLimit), domain.MaxExampleBodyLen)

	res, _, exUC := importWithExamples(t, singleRequestCollection(t,
		map[string]any{"name": "At limit", "code": 200, "body": atLimit},
		map[string]any{"name": "Over limit", "code": 200, "body": overLimit},
	))

	require.Len(t, exUC.created, 1)
	assert.Equal(t, "At limit", exUC.created[0].Name)
	assert.Equal(t, []string{`request "Ping": example 2 skipped: body is larger than 256 KB`}, res.Warnings)
}

func TestImportCollection_WarnsAboutExamplesThatMayHoldSecrets(t *testing.T) {
	res, _, exUC := importWithExamples(t, singleRequestCollection(t,
		map[string]any{"name": "Login", "code": 200, "body": `{"access_token":"2YotnFZFEjr1zCsicMWpAA"}`},
		map[string]any{"name": "Plain", "code": 200, "body": `{"id":1}`},
		map[string]any{"name": "Masked", "code": 200, "header": []any{
			map[string]any{"key": "Authorization", "value": "Bearer abcdefghij0123456789xyz"},
		}},
		map[string]any{"name": "Keyed", "code": 200, "header": []any{
			map[string]any{"key": "X-Debug", "value": "AKIAIOSFODNN7EXAMPLE"},
		}},
	))

	require.Len(t, exUC.created, 4, "a suspected secret warns, it does not block")
	assert.Equal(t, []string{
		`examples may contain secrets, check them before sharing: request "Ping" / "Login" (OAuth token); ` +
			`request "Ping" / "Keyed" (AWS access key)`,
	}, res.Warnings)
}

func TestImportCollection_ExampleNameTruncatedToLimit(t *testing.T) {
	long := strings.Repeat("я", 250)

	res, _, exUC := importWithExamples(t, singleRequestCollection(t,
		map[string]any{"name": long, "code": 200},
	))

	require.Len(t, exUC.created, 1)
	assert.Equal(t, 200, utf8.RuneCountInString(exUC.created[0].Name))
	assert.True(t, utf8.ValidString(exUC.created[0].Name))
	assert.Equal(t, []string{`request "Ping": example 1: name longer than 200 characters was truncated`}, res.Warnings)
}

func TestImportCollection_ExampleErrorNeverAbortsImport(t *testing.T) {
	exUC := &stubExampleUC{failOn: func(in example.Create) error {
		switch in.Name {
		case "Huge":
			return &domain.ValidationError{Fields: map[string]string{"example": "example is too large (max 480 KB)"}}
		case "Storage":
			return errors.New("disk I/O error")
		}
		return nil
	}}
	reqUC := &stubRequestUC{}
	res, err := postman.ImportCollection(context.Background(), singleRequestCollection(t,
		map[string]any{"name": "Huge", "code": 200},
		map[string]any{"name": "Kept", "code": 201},
		map[string]any{"name": "Storage", "code": 500},
	), postman.ImportOpts{WorkspaceID: uuid.New(), UserID: "local_user"}, &stubCollectionUC{}, reqUC, exUC, &stubEnvironmentUC{})
	require.NoError(t, err)

	assert.Equal(t, 1, res.RequestsCreated)
	require.Len(t, exUC.created, 1)
	assert.Equal(t, "Kept", exUC.created[0].Name)
	assert.Equal(t, []string{
		`request "Ping": example 1 skipped: example is too large (max 480 KB)`,
		`request "Ping": example 3 skipped: disk I/O error`,
	}, res.Warnings)
}

func exportedResponses(t *testing.T, item postman.PostmanItem) []postman.PostmanResponse {
	t.Helper()
	out := make([]postman.PostmanResponse, 0, len(item.Response))
	for _, raw := range item.Response {
		var r postman.PostmanResponse
		require.NoError(t, json.Unmarshal(raw, &r))
		out = append(out, r)
	}
	return out
}

func TestExportCollection_Examples(t *testing.T) {
	rootID := uuid.New()
	req := &entities.Request{
		ID: uuid.New(), CollectionID: rootID, Name: "Get user", Description: "Docs.",
		Protocol: entities.ProtocolHTTP, Method: entities.MethodGET, URL: "{{host}}/users/1",
		Headers:  []entities.HeaderItem{{Key: "Accept", Value: "application/json", Enabled: true}},
		BodyType: entities.BodyTypeNone, AuthType: entities.AuthTypeBearer, AuthData: `{"token":"{{token}}"}`,
	}
	examples := map[uuid.UUID][]*entities.ResponseExample{req.ID: {
		{
			ID: uuid.New(), RequestID: req.ID, Name: "Missing", StatusCode: 404, StatusText: "Not Found",
			Headers: []entities.HeaderItem{}, Body: `{"error":"not found"}`, SortOrder: 1,
		},
		{
			ID: uuid.New(), RequestID: req.ID, Name: "Found", StatusCode: 200, StatusText: "OK",
			Headers: []entities.HeaderItem{
				{Key: "Content-Type", Value: "application/json", Enabled: true},
				{Key: "Set-Cookie", Value: "<redacted>", Enabled: true},
			},
			Body: `{"id":1}`, SortOrder: 0,
		},
	}}

	data, warnings, err := postman.ExportCollection(rootID,
		[]*entities.Collection{{ID: rootID, Name: "API"}}, []*entities.Request{req}, examples)
	require.NoError(t, err)
	assert.Empty(t, warnings)

	var pc postman.PostmanCollection
	require.NoError(t, json.Unmarshal(data, &pc))
	require.Len(t, pc.Item, 1)

	responses := exportedResponses(t, pc.Item[0])
	require.Len(t, responses, 2)
	assert.Equal(t, "Found", responses[0].Name, "examples follow their sort order")
	assert.Equal(t, 200, responses[0].Code)
	assert.Equal(t, "OK", responses[0].Status)
	assert.Equal(t, `{"id":1}`, responses[0].Body)
	assert.Equal(t, postman.PostmanHeaders{
		{Key: "Content-Type", Value: "application/json"},
		{Key: "Set-Cookie", Value: "<redacted>"},
	}, responses[0].Header)
	assert.Equal(t, "Missing", responses[1].Name)
	assert.Equal(t, 404, responses[1].Code)
	assert.NotNil(t, responses[1].Header)

	orig := responses[0].OriginalRequest
	require.NotNil(t, orig)
	assert.Equal(t, "GET", orig.Method)
	assert.Equal(t, "{{host}}/users/1", orig.URL.Raw)
	assert.Equal(t, []postman.PostmanKV{{Key: "Accept", Value: "application/json"}}, orig.Header)
}

func TestExportCollection_SkipsGRPCWithWarning(t *testing.T) {
	rootID := uuid.New()
	grpcReq := &entities.Request{
		ID: uuid.New(), CollectionID: rootID, Name: "SayHello", Protocol: entities.ProtocolGRPC,
		URL: "localhost:50051", GRPCService: "helloworld.Greeter", GRPCMethod: "SayHello",
		BodyType: entities.BodyTypeJSON, Body: `{"name":"x"}`, AuthType: entities.AuthTypeNone, AuthData: "{}",
	}
	httpReq := &entities.Request{
		ID: uuid.New(), CollectionID: rootID, Name: "Ping", Protocol: entities.ProtocolHTTP,
		Method: entities.MethodGET, URL: "/ping", BodyType: entities.BodyTypeNone,
		AuthType: entities.AuthTypeInherit, AuthData: "{}",
	}
	examples := map[uuid.UUID][]*entities.ResponseExample{
		grpcReq.ID: {{ID: uuid.New(), RequestID: grpcReq.ID, Name: "OK", Body: `{"message":"hi"}`, Protocol: entities.ProtocolGRPC}},
	}

	data, warnings, err := postman.ExportCollection(rootID,
		[]*entities.Collection{{ID: rootID, Name: "API"}}, []*entities.Request{grpcReq, httpReq}, examples)
	require.NoError(t, err)

	assert.Equal(t, []string{`request "SayHello": gRPC requests have no Postman equivalent and were skipped`}, warnings)
	var pc postman.PostmanCollection
	require.NoError(t, json.Unmarshal(data, &pc))
	require.Len(t, pc.Item, 1)
	assert.Equal(t, "Ping", pc.Item[0].Name)
	assert.NotContains(t, string(data), "SayHello")
}

func TestExportImportRoundTrip_Examples(t *testing.T) {
	rootID := uuid.New()
	req := &entities.Request{
		ID: uuid.New(), CollectionID: rootID, Name: "Users", Protocol: entities.ProtocolGraphQL,
		Method: entities.MethodPOST, URL: "{{host}}/graphql", GraphQLQuery: "{ users { id } }",
		BodyType: entities.BodyTypeNone, AuthType: entities.AuthTypeInherit, AuthData: "{}",
	}
	original := &entities.ResponseExample{
		ID: uuid.New(), RequestID: req.ID, Name: "Two users", StatusCode: 200, StatusText: "OK",
		Headers: []entities.HeaderItem{
			{Key: "Content-Type", Value: "application/json", Enabled: true},
			{Key: "Vary", Value: "Accept", Enabled: true},
			{Key: "Vary", Value: "Origin", Enabled: true},
		},
		Body: `{"data":{"users":[{"id":1},{"id":2}]}}`, ContentType: "application/json", Protocol: entities.ProtocolGraphQL,
	}

	data, _, err := postman.ExportCollection(rootID, []*entities.Collection{{ID: rootID, Name: "API"}},
		[]*entities.Request{req}, map[uuid.UUID][]*entities.ResponseExample{req.ID: {original}})
	require.NoError(t, err)

	res, reqUC, exUC := importWithExamples(t, data)
	assert.Empty(t, res.Warnings)
	require.Len(t, exUC.created, 1)
	assert.Equal(t, example.Create{
		RequestID:   reqUC.ids[0],
		Name:        original.Name,
		StatusCode:  original.StatusCode,
		StatusText:  original.StatusText,
		Headers:     original.Headers,
		Body:        original.Body,
		ContentType: original.ContentType,
		Protocol:    entities.ProtocolGraphQL,
	}, exUC.created[0])
}

func exampleRoundTrip(t *testing.T, ex *entities.ResponseExample) (postman.PostmanResponse, example.Create) {
	t.Helper()
	rootID := uuid.New()
	req := &entities.Request{
		ID: uuid.New(), CollectionID: rootID, Name: "Q", Protocol: entities.ProtocolGraphQL,
		Method: entities.MethodPOST, URL: "https://x/graphql", AuthType: entities.AuthTypeInherit, AuthData: "{}",
	}
	ex.RequestID = req.ID
	data, _, err := postman.ExportCollection(rootID, []*entities.Collection{{ID: rootID, Name: "API"}},
		[]*entities.Request{req}, map[uuid.UUID][]*entities.ResponseExample{req.ID: {ex}})
	require.NoError(t, err)

	var pc postman.PostmanCollection
	require.NoError(t, json.Unmarshal(data, &pc))
	exported := exportedResponses(t, pc.Item[0])
	require.Len(t, exported, 1)

	_, _, exUC := importWithExamples(t, data)
	require.Len(t, exUC.created, 1)
	return exported[0], exUC.created[0]
}

func TestExportImportRoundTrip_ExampleContentTypeWithoutHeader(t *testing.T) {
	exported, imported := exampleRoundTrip(t, &entities.ResponseExample{
		ID: uuid.New(), Name: "New example", StatusCode: 200, StatusText: "OK", Headers: []entities.HeaderItem{},
		Body: `{"data":{}}`, ContentType: "application/json", Protocol: entities.ProtocolGraphQL,
	})

	assert.Equal(t, "json", exported.PreviewLanguage)
	assert.Empty(t, exported.Header, "Postman's language names application/json exactly, so no header is invented")
	assert.Equal(t, "application/json", imported.ContentType)
	assert.Empty(t, imported.Headers)
}

func TestExportImportRoundTrip_ExampleContentTypeBeyondPreviewLanguages(t *testing.T) {
	exported, imported := exampleRoundTrip(t, &entities.ResponseExample{
		ID: uuid.New(), Name: "Report", StatusCode: 200, Headers: []entities.HeaderItem{},
		Body: "a,b", ContentType: "text/csv", Protocol: entities.ProtocolHTTP,
	})

	assert.Equal(t, "text", exported.PreviewLanguage)
	assert.Equal(t, postman.PostmanHeaders{{Key: "Content-Type", Value: "text/csv"}}, exported.Header)
	assert.Equal(t, "text/csv", imported.ContentType)
}

func TestImportCollection_ExampleContentTypeFallsBackToPreviewLanguage(t *testing.T) {
	_, _, exUC := importWithExamples(t, singleRequestCollection(t,
		map[string]any{"name": "Xml", "code": 200, "_postman_previewlanguage": "xml", "body": "<a/>"},
		map[string]any{"name": "Header wins", "code": 200, "_postman_previewlanguage": "html",
			"header": []any{map[string]any{"key": "Content-Type", "value": "text/plain"}}},
		map[string]any{"name": "Unknown", "code": 200, "_postman_previewlanguage": "auto"},
	))

	require.Len(t, exUC.created, 3)
	assert.Equal(t, "application/xml", exUC.created[0].ContentType)
	assert.Equal(t, "text/plain", exUC.created[1].ContentType)
	assert.Empty(t, exUC.created[2].ContentType)
}

func TestImportCollection_ExampleHeaderShapes(t *testing.T) {
	res, _, exUC := importWithExamples(t, singleRequestCollection(t,
		map[string]any{"name": "String", "code": 200, "header": "Content-Type: application/json\r\nX-A: 1\n\n// X-Off: 2\nX-Url: https://h/p"},
		map[string]any{"name": "Mixed", "code": 200, "header": []any{"Content-Type: text/plain", map[string]any{"key": "X-B", "value": "2"}}},
		map[string]any{"name": "Number", "code": 200, "header": 5},
		map[string]any{"name": "Bad item", "code": 200, "header": []any{true}},
	))

	require.Len(t, exUC.created, 2)
	assert.Equal(t, []entities.HeaderItem{
		{Key: "Content-Type", Value: "application/json", Enabled: true},
		{Key: "X-A", Value: "1", Enabled: true},
		{Key: "X-Off", Value: "2", Enabled: false},
		{Key: "X-Url", Value: "https://h/p", Enabled: true},
	}, exUC.created[0].Headers)
	assert.Equal(t, "application/json", exUC.created[0].ContentType)
	assert.Equal(t, []entities.HeaderItem{
		{Key: "Content-Type", Value: "text/plain", Enabled: true},
		{Key: "X-B", Value: "2", Enabled: true},
	}, exUC.created[1].Headers)
	assert.Equal(t, "text/plain", exUC.created[1].ContentType)
	assert.Equal(t, []string{
		`request "Ping": example 3 skipped: "header" must be a list, got number`,
		`request "Ping": example 4 skipped: "header" must be a list, got bool`,
	}, res.Warnings)
}
