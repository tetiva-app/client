package snapshotjson_test

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tetiva-app/client/internal/adapters/publication/snapshotjson"
	"github.com/tetiva-app/client/internal/domain"
	"github.com/tetiva-app/client/internal/domain/entities"
	"github.com/tetiva-app/client/internal/domain/usecase/publication"
)

const decodeLimit = 8 << 20

func readFixture(t *testing.T, name string) []byte {
	t.Helper()
	raw, err := os.ReadFile(repoFile(t, "testdata/snapshot/"+name))
	require.NoError(t, err)
	return raw
}

func gzipped(t *testing.T, b []byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := gzip.NewWriter(&buf)
	_, err := zw.Write(b)
	require.NoError(t, err)
	require.NoError(t, zw.Close())
	return buf.Bytes()
}

func requireReason(t *testing.T, err error, reason string) {
	t.Helper()
	var re *domain.ReasonError
	require.ErrorAs(t, err, &re)
	assert.Equal(t, reason, re.Reason)
	var ve *domain.ValidationError
	assert.ErrorAs(t, err, &ve)
}

func snapshotJSON(t *testing.T, items any, extra map[string]any) []byte {
	t.Helper()
	doc := map[string]any{
		"format": "tetiva.collection-snapshot", "version": 1, "generator": "Tetiva 1.2.0", "locale": "en",
		"collection": map[string]any{
			"id": "000000000001", "name": "C", "description": "", "auth": nil, "scripts": nil,
			"grpcMetadata": []any{}, "items": items,
		},
		"environment": nil,
	}
	for k, v := range extra {
		doc[k] = v
	}
	raw, err := json.Marshal(doc)
	require.NoError(t, err)
	return raw
}

func folderChain(depth int) []any {
	var items []any
	for d := depth; d >= 1; d-- {
		items = []any{map[string]any{
			"kind": "folder", "id": "00000000000f", "name": "F", "description": "", "auth": nil, "scripts": nil, "items": items,
		}}
	}
	if items == nil {
		return []any{}
	}
	return items
}

func requests(n int) []any {
	items := make([]any, n)
	for i := range items {
		items[i] = map[string]any{
			"kind": "request", "id": "00000000000a", "name": "R", "description": "", "protocol": "http",
			"http": map[string]any{
				"method": "GET", "url": "https://example.com", "headers": []any{},
				"body": map[string]any{"type": "none", "raw": "", "fields": []any{}, "fileName": ""},
			},
			"auth":    map[string]any{"type": "inherit", "fields": map[string]any{}, "redacted": []any{}},
			"scripts": nil, "examples": []any{},
		}
	}
	return items
}

func TestDecode_AllProtocolsRoundTrips(t *testing.T) {
	raw := readFixture(t, "all-protocols.json")

	s, err := snapshotjson.Decode(bytes.NewReader(raw), decodeLimit)
	require.NoError(t, err)

	assert.Equal(t, "Tetiva 1.2.0", s.Generator)
	assert.Equal(t, "ru", s.Locale)
	assert.Equal(t, "Petstore API — демо", s.Collection.Name)
	require.Len(t, s.Collection.GRPCMetadata, 2)
	require.NotNil(t, s.Environment)
	assert.Equal(t, publication.Variable{Key: "token", Value: "", Secret: true}, s.Environment.Variables[9])

	out, err := snapshotjson.Marshal(s)
	require.NoError(t, err)
	assert.JSONEq(t, string(raw), string(out))
}

func TestDecode_KeepsIntegersExact(t *testing.T) {
	s, err := snapshotjson.Decode(bytes.NewReader(readFixture(t, "all-protocols.json")), decodeLimit)
	require.NoError(t, err)

	admin := s.Collection.Items[0].Folder.Items[0].Folder
	head := admin.Items[1].Request
	require.Equal(t, entities.Protocol("http"), head.Protocol)
	claims, ok := head.Auth.Fields["claims"].(map[string]any)
	require.True(t, ok)
	iat, err := json.Marshal(claims["iat"])
	require.NoError(t, err)
	assert.Equal(t, "1727222400", string(iat))
}

func TestDecode_Gzip(t *testing.T) {
	raw := readFixture(t, "all-protocols.json")

	s, err := snapshotjson.Decode(bytes.NewReader(gzipped(t, raw)), decodeLimit)
	require.NoError(t, err)

	out, err := snapshotjson.Marshal(s)
	require.NoError(t, err)
	assert.JSONEq(t, string(raw), string(out))
}

func TestDecode_MaxSizeFixtureFitsTheLimit(t *testing.T) {
	raw := readFixture(t, "max-size.json")
	require.LessOrEqual(t, len(raw), decodeLimit)

	s, err := snapshotjson.Decode(bytes.NewReader(raw), decodeLimit)
	require.NoError(t, err)
	assert.NotEmpty(t, s.Collection.Items)
}

func TestDecode_TooLarge(t *testing.T) {
	big := snapshotJSON(t, []any{}, map[string]any{"padding": strings.Repeat("a", 4096)})

	_, err := snapshotjson.Decode(bytes.NewReader(big), 1024)
	requireReason(t, err, snapshotjson.ReasonTooLarge)
}

func TestDecode_GzipBomb(t *testing.T) {
	bomb := gzipped(t, snapshotJSON(t, []any{}, map[string]any{"padding": strings.Repeat("a", decodeLimit+1)}))
	require.Less(t, len(bomb), 64<<10)

	_, err := snapshotjson.Decode(bytes.NewReader(bomb), decodeLimit)
	requireReason(t, err, snapshotjson.ReasonTooLarge)
}

func TestDecode_Invalid(t *testing.T) {
	cases := map[string][]byte{
		"garbage":        []byte("not json at all"),
		"array":          []byte(`[1,2,3]`),
		"wrong format":   []byte(`{"format":"postman","version":1}`),
		"no version":     []byte(`{"format":"tetiva.collection-snapshot"}`),
		"broken gzip":    {0x1f, 0x8b, 0x00, 0x01},
		"truncated":      snapshotJSON(t, []any{}, nil)[:40],
		"unknown kind":   snapshotJSON(t, []any{map[string]any{"kind": "script", "id": "000000000002", "name": "x"}}, nil),
		"string status":  []byte(strings.Replace(string(readFixture(t, "all-protocols.json")), `"status": 200`, `"status": "200"`, 1)),
		"too deep":       snapshotJSON(t, folderChain(17), nil),
		"too many items": snapshotJSON(t, requests(5001), nil),
	}
	for name, raw := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := snapshotjson.Decode(bytes.NewReader(raw), decodeLimit)
			requireReason(t, err, snapshotjson.ReasonInvalid)
		})
	}
}

func TestDecode_DepthAndItemBoundaries(t *testing.T) {
	_, err := snapshotjson.Decode(bytes.NewReader(snapshotJSON(t, folderChain(16), nil)), decodeLimit)
	require.NoError(t, err)

	_, err = snapshotjson.Decode(bytes.NewReader(snapshotJSON(t, requests(5000), nil)), decodeLimit)
	require.NoError(t, err)
}

func TestDecode_NewerVersionAsksForUpdate(t *testing.T) {
	_, err := snapshotjson.Decode(bytes.NewReader(snapshotJSON(t, []any{}, map[string]any{"version": 2})), decodeLimit)
	requireReason(t, err, snapshotjson.ReasonUpdateRequired)
}

func TestDecode_IgnoresUnknownFields(t *testing.T) {
	items := requests(1)
	items[0].(map[string]any)["futureField"] = map[string]any{"x": 1}
	raw := snapshotJSON(t, items, map[string]any{"publishedBy": "someone"})

	s, err := snapshotjson.Decode(bytes.NewReader(raw), decodeLimit)
	require.NoError(t, err)
	require.Len(t, s.Collection.Items, 1)
	assert.Equal(t, "https://example.com", s.Collection.Items[0].Request.HTTP.URL)
}

func TestDecode_NullArraysReadAsEmpty(t *testing.T) {
	raw := snapshotJSON(t, nil, nil)

	s, err := snapshotjson.Decode(bytes.NewReader(raw), decodeLimit)
	require.NoError(t, err)
	assert.Empty(t, s.Collection.Items)
	assert.Nil(t, s.Environment)
}

func TestSniff(t *testing.T) {
	raw := readFixture(t, "all-protocols.json")
	late := []byte(`{"version":1,"collection":{"items":[]},"format":"tetiva.collection-snapshot"}`)

	assert.True(t, snapshotjson.Sniff(raw))
	assert.True(t, snapshotjson.Sniff(gzipped(t, raw)))
	assert.True(t, snapshotjson.Sniff(late))
	assert.False(t, snapshotjson.Sniff([]byte(`{"info":{"schema":"v2.1"},"item":[]}`)))
	assert.False(t, snapshotjson.Sniff([]byte(`{"format":"other"}`)))
	assert.False(t, snapshotjson.Sniff([]byte(`["tetiva.collection-snapshot"]`)))
	assert.False(t, snapshotjson.Sniff([]byte(`garbage`)))
}
