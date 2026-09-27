package snapshotjson_test

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/santhosh-tekuri/jsonschema/v6"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tetiva-app/client/internal/adapters/publication/snapshotjson"
	"github.com/tetiva-app/client/internal/domain/entities"
	"github.com/tetiva-app/client/internal/domain/usecase/publication"
)

const schemaURL = "https://tetiva.app/schemas/collection-snapshot.v1.schema.json"

func repoFile(t *testing.T, rel string) string {
	t.Helper()
	_, file, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(file), "..", "..", "..", "..", rel)
}

func compileSchema(t *testing.T) *jsonschema.Schema {
	t.Helper()
	raw, err := os.ReadFile(repoFile(t, "testdata/snapshot/collection-snapshot.v1.schema.json"))
	require.NoError(t, err)
	doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(raw))
	require.NoError(t, err)
	c := jsonschema.NewCompiler()
	require.NoError(t, c.AddResource(schemaURL, doc))
	sch, err := c.Compile(schemaURL)
	require.NoError(t, err)
	return sch
}

func validate(t *testing.T, sch *jsonschema.Schema, out []byte) {
	t.Helper()
	inst, err := jsonschema.UnmarshalJSON(bytes.NewReader(out))
	require.NoError(t, err)
	require.NoError(t, sch.Validate(inst))
}

func TestMarshal_PassesTheServerSchema(t *testing.T) {
	s, report := buildSnapshot(t, petstore())
	require.Empty(t, report.Errors)
	out, err := snapshotjson.Marshal(s)
	require.NoError(t, err)

	validate(t, compileSchema(t), out)
}

func TestMarshal_SchemaSelfCheck(t *testing.T) {
	sch := compileSchema(t)
	fixture, err := os.ReadFile(repoFile(t, "testdata/snapshot/all-protocols.json"))
	require.NoError(t, err)
	validate(t, sch, fixture)

	bad := strings.Replace(string(fixture), `"key": "Accept"`, `"key": "X Api"`, 1)
	inst, err := jsonschema.UnmarshalJSON(strings.NewReader(bad))
	require.NoError(t, err)
	assert.Error(t, sch.Validate(inst))
}

func TestMarshal_Golden(t *testing.T) {
	s, _ := buildSnapshot(t, petstore())
	out, err := snapshotjson.Marshal(s)
	require.NoError(t, err)

	path := filepath.Join("testdata", "canonical.golden.json")
	if os.Getenv("UPDATE_GOLDEN") == "1" {
		require.NoError(t, os.MkdirAll("testdata", 0o755))
		require.NoError(t, os.WriteFile(path, out, 0o644))
	}
	want, err := os.ReadFile(path)
	require.NoError(t, err, "run with UPDATE_GOLDEN=1 to create %s", path)
	assert.Equal(t, string(want), string(out), "canonical bytes changed; rerun with UPDATE_GOLDEN=1 if intended")
}

func TestMarshal_Shape(t *testing.T) {
	s, _ := buildSnapshot(t, petstore())
	out, err := snapshotjson.Marshal(s)
	require.NoError(t, err)
	text := string(out)

	assert.True(t, strings.HasPrefix(text, `{"format":"tetiva.collection-snapshot","version":1,"generator":"Tetiva 1.2.0","locale":"ru","collection":{"id":`))
	assert.Equal(t, 1, strings.Count(text, `"grpcMetadata"`), "only the root carries grpcMetadata")
	assert.NotContains(t, text, "\n")
	assert.NotContains(t, text, "\\u003c", "no HTML escaping")
	assert.NotContains(t, text, `"graphql":null`, "parts other than the protocol's are left out")

	var doc map[string]any
	require.NoError(t, json.Unmarshal(out, &doc))
	coll := doc["collection"].(map[string]any)
	pets := coll["items"].([]any)[0].(map[string]any)
	assert.Equal(t, "folder", pets["kind"])
	admin := pets["items"].([]any)[0].(map[string]any)
	assert.Nil(t, admin["auth"])
	assert.Nil(t, admin["scripts"])
	del := admin["items"].([]any)[0].(map[string]any)
	assert.Equal(t, map[string]any{"type": "inherit", "fields": map[string]any{}, "redacted": []any{}}, del["auth"])
	assert.Equal(t, []any{}, del["examples"])
	assert.Equal(t, []any{}, del["http"].(map[string]any)["headers"])
	assert.NotContains(t, del, "items")
}

func TestMarshal_EmptyAndNilSlices(t *testing.T) {
	s := &publication.Snapshot{
		Generator: "g", Locale: "en",
		Collection: publication.Root{Folder: publication.Folder{ID: "000000000000", Name: "Empty", Items: []publication.Item{
			{Folder: &publication.Folder{ID: "000000000001", Name: "F"}},
			{Request: &publication.Request{ID: "000000000002", Name: "R", Protocol: entities.ProtocolWebSocket,
				WebSocket: &publication.WSPart{URL: "wss://x"}, Auth: &publication.Auth{Type: "none"}}},
		}}},
	}

	out, err := snapshotjson.Marshal(s)
	require.NoError(t, err)

	assert.Equal(t, `{"format":"tetiva.collection-snapshot","version":1,"generator":"g","locale":"en","collection":{`+
		`"id":"000000000000","name":"Empty","description":"","auth":null,"scripts":null,"grpcMetadata":[],"items":[`+
		`{"kind":"folder","id":"000000000001","name":"F","description":"","auth":null,"scripts":null,"items":[]},`+
		`{"kind":"request","id":"000000000002","name":"R","description":"","protocol":"websocket",`+
		`"websocket":{"url":"wss://x","headers":[],"subprotocols":[],"messages":[]},`+
		`"auth":{"type":"none","fields":{},"redacted":[]},"scripts":null,"examples":[]}]},"environment":null}`, string(out))
	validate(t, compileSchema(t), out)
}

func TestMarshal_NoHTMLEscaping(t *testing.T) {
	s := &publication.Snapshot{Generator: "g", Locale: "en", Collection: publication.Root{Folder: publication.Folder{
		ID: "000000000000", Name: "<b>A & B</b>",
	}}}
	out, err := snapshotjson.Marshal(s)
	require.NoError(t, err)
	assert.Contains(t, string(out), `"name":"<b>A & B</b>"`)
}

func TestGzip_RoundTripAndDeterminism(t *testing.T) {
	s, _ := buildSnapshot(t, petstore())
	out, err := snapshotjson.Marshal(s)
	require.NoError(t, err)

	a, err := snapshotjson.Gzip(out)
	require.NoError(t, err)
	b, err := snapshotjson.Gzip(out)
	require.NoError(t, err)
	assert.Equal(t, a, b)

	zr, err := gzip.NewReader(bytes.NewReader(a))
	require.NoError(t, err)
	back, err := io.ReadAll(zr)
	require.NoError(t, err)
	assert.Equal(t, out, back)
}
