package openapi

import (
	"bytes"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tetiva-app/client/internal/domain"
)

func TestSource_PointersEscapeSlashAndTilde(t *testing.T) {
	src, err := parseSource([]byte("paths:\n  /a/{id}:\n    get: {summary: A}\n  x~y:\n    get: {summary: B}\nlist: [zero, one]\n"))
	require.NoError(t, err)

	assert.Equal(t, "A", scalar(src.at(ptr("#", "paths", "/a/{id}", "get")), "summary"))
	assert.Equal(t, "B", scalar(src.at(ptr("#", "paths", "x~y", "get")), "summary"))
	assert.Equal(t, "one", src.at("#/list/1").Value)
	assert.Nil(t, src.at("#/list/2"))
	assert.Nil(t, src.at(ptr("", "paths")))
}

func TestWriteJSON_KeepsOrderAndScalarTypes(t *testing.T) {
	src, err := parseSource([]byte(`
zeta: 1
alpha: "1"
hex: 0x1F
float: 1.5e3
inf: .inf
flag: true
none: null
day: 2024-01-01
html: "<b>&</b>"
nested: {b: [1, two], a: {}}
`))
	require.NoError(t, err)

	var buf bytes.Buffer
	require.NoError(t, writeJSON(&buf, src.root, maxDocumentJSON))
	assert.Equal(t, `{"zeta":1,"alpha":"1","hex":31,"float":1.5e3,"inf":".inf","flag":true,"none":null,`+
		`"day":"2024-01-01","html":"<b>&</b>","nested":{"b":[1,"two"],"a":{}}}`, buf.String())
}

func TestParseSource_JSONKeepsEscapesOrderAndTypes(t *testing.T) {
	src, err := parseSource([]byte(`{"zeta":1,"alpha":"1","float":1.5e3,"big":12345678901234567890,"flag":true,"none":null,` +
		`"slash":"a\/b","emoji":"\ud83d\ude80","nested":{"b":[1,"two"],"a":{}}}`))
	require.NoError(t, err)

	var buf bytes.Buffer
	require.NoError(t, writeJSON(&buf, src.root, maxDocumentJSON))
	assert.Equal(t, `{"zeta":1,"alpha":"1","float":1.5e3,"big":12345678901234567890,"flag":true,"none":null,`+
		`"slash":"a/b","emoji":"🚀","nested":{"b":[1,"two"],"a":{}}}`, buf.String())

	_, err = parseSource([]byte(`[{"openapi":"3.0.0"}]`))
	assert.EqualError(t, err, "the document is not an object")
}

func TestWriteJSON_StopsAnAliasBomb(t *testing.T) {
	src, err := parseSource([]byte(`
a: &a [x, x, x, x, x, x, x, x, x, x]
b: &b [*a, *a, *a, *a, *a, *a, *a, *a, *a, *a]
c: &c [*b, *b, *b, *b, *b, *b, *b, *b, *b, *b]
d: &d [*c, *c, *c, *c, *c, *c, *c, *c, *c, *c]
e: [*d, *d, *d, *d, *d, *d, *d, *d, *d, *d]
`))
	require.NoError(t, err)

	assert.ErrorIs(t, writeJSON(&bytes.Buffer{}, src.root, 1<<16), errTooLarge)
}

func TestStubBrokenRefs_WarnsOncePerTarget(t *testing.T) {
	src, err := parseSource([]byte(`
a: {$ref: "common.yaml#/A"}
b: {$ref: "common.yaml#/B"}
c: {$ref: "#/missing"}
d: {$ref: "#/missing"}
e: {$ref: "#/target"}
target: {type: string}
`))
	require.NoError(t, err)

	warnings := src.stubBrokenRefs()

	assert.Equal(t, []string{
		`references to "common.yaml" are not supported and were left empty`,
		`reference "#/missing" points to nothing and was left empty`,
	}, warnings)
	var buf bytes.Buffer
	require.NoError(t, writeJSON(&buf, src.root, maxDocumentJSON))
	assert.Equal(t, `{"a":{},"b":{},"c":{},"d":{},"e":{"$ref":"#/target"},"target":{"type":"string"}}`, buf.String())
}

func TestRefPointer_MapsSwagger2Sections(t *testing.T) {
	v2 := &source{swagger2: true}
	v3 := &source{}

	assert.Equal(t, "#/definitions/Pet", v2.refPointer("#/components/schemas/Pet"))
	assert.Equal(t, "#/responses/NotFound", v2.refPointer("#/components/responses/NotFound"))
	assert.Equal(t, "#/components/schemas/Pet", v3.refPointer("#/components/schemas/Pet"))
	assert.Equal(t, "#/components/schemas/Big Pet", v3.refPointer("#/components/schemas/Big%20Pet"))
	assert.Empty(t, v3.refPointer("other.yaml#/Pet"))
}

func TestLoad_RefusesAnAnchorThatContainsItself(t *testing.T) {
	_, err := load([]byte("openapi: 3.0.0\ninfo: {title: x, version: '1'}\npaths:\n  /a: &a\n    get:\n      x: *a\n"))

	var ve *domain.ValidationError
	require.ErrorAs(t, err, &ve)
	assert.Equal(t, "the OpenAPI document can't be read: the anchor &a refers to itself", ve.Fields["content"])
}

func TestWriteJSON_StopsAtTheDepthLimit(t *testing.T) {
	src, err := parseSource([]byte("x: " + strings.Repeat("[", maxDocumentDepth+1) + strings.Repeat("]", maxDocumentDepth+1) + "\n"))
	require.NoError(t, err)

	assert.ErrorIs(t, writeJSON(&bytes.Buffer{}, src.root, maxDocumentJSON), errTooDeep)
}
