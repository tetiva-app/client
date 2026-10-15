package openapi

import (
	"fmt"
	"strings"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func sampleOf(t *testing.T, schemas string, request bool) string {
	t.Helper()
	doc := "openapi: 3.1.0\ninfo: {title: T, version: '1'}\npaths: {}\ncomponents:\n  schemas:\n" + indent(schemas, "    ")
	s, err := load([]byte(doc))
	require.NoError(t, err)
	ref := &openapi3.SchemaRef{Ref: "#/components/schemas/Root", Value: s.doc.Components.Schemas["Root"].Value}
	return newSampler(s.src, request).json(ref, "")
}

func indent(s, prefix string) string {
	lines := strings.Split(strings.TrimSpace(s), "\n")
	for i, l := range lines {
		lines[i] = prefix + l
	}
	return strings.Join(lines, "\n") + "\n"
}

func compact(s string) string {
	return strings.Join(strings.Fields(s), "")
}

func TestSampler(t *testing.T) {
	cases := map[string]struct {
		schemas string
		request bool
		want    string
	}{
		"properties keep document order": {
			schemas: `
Root:
  type: object
  properties:
    zeta: {type: string}
    alpha: {type: integer}
    beta: {type: boolean}`,
			want: `{"zeta":"string","alpha":0,"beta":true}`,
		},
		"formats, enums and examples": {
			schemas: `
Root:
  type: object
  properties:
    id: {type: string, format: uuid}
    at: {type: string, format: date-time}
    mail: {type: string, format: email}
    state: {type: string, enum: [on, off]}
    count: {type: integer, default: 5}
    name: {type: string, example: Rex}
    tags: {type: array, items: {type: string}}
    nothing: {type: "null"}`,
			want: `{"id":"3fa85f64-5717-4562-b3fc-2c963f66afa6","at":"2024-01-01T12:00:00Z","mail":"user@example.com",` +
				`"state":"on","count":5,"name":"Rex","tags":["string"],"nothing":null}`,
		},
		"object example keeps its key order": {
			schemas: `
Root:
  type: object
  example: {z: 1, a: {y: 2, b: 3}}`,
			want: `{"z":1,"a":{"y":2,"b":3}}`,
		},
		"a cycle ends in an empty object": {
			schemas: `
Root:
  type: object
  properties:
    name: {type: string}
    children: {type: array, items: {$ref: "#/components/schemas/Root"}}
    parent: {$ref: "#/components/schemas/Root"}`,
			want: `{"name":"string","children":[{}],"parent":{}}`,
		},
		"allOf merges and oneOf takes the first": {
			schemas: `
Base:
  type: object
  properties:
    id: {type: integer}
Root:
  allOf:
    - $ref: "#/components/schemas/Base"
    - type: object
      properties:
        pet:
          oneOf:
            - {type: object, properties: {meow: {type: boolean}}}
            - {type: object, properties: {bark: {type: boolean}}}`,
			want: `{"id":0,"pet":{"meow":true}}`,
		},
		"request skips readOnly": {
			schemas: `
Root:
  type: object
  properties:
    id: {type: integer, readOnly: true}
    secret: {type: string, writeOnly: true}`,
			request: true,
			want:    `{"secret":"string"}`,
		},
		"response skips writeOnly": {
			schemas: `
Root:
  type: object
  properties:
    id: {type: integer, readOnly: true}
    secret: {type: string, writeOnly: true}`,
			want: `{"id":0}`,
		},
		"map values": {
			schemas: `
Root:
  type: object
  additionalProperties: {type: integer}`,
			want: `{"key":0}`,
		},
		"3.1 type arrays and examples": {
			schemas: `
Root:
  type: object
  properties:
    note: {type: [string, "null"]}
    level: {type: integer, examples: [7, 8]}
    kind: {const: cat}`,
			want: `{"note":"string","level":7,"kind":"cat"}`,
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, tc.want, compact(sampleOf(t, tc.schemas, tc.request)))
		})
	}
}

func TestSampler_DepthLimit(t *testing.T) {
	var b strings.Builder
	b.WriteString("Root: {$ref: '#/components/schemas/L0'}\n")
	for i := range 10 {
		fmt.Fprintf(&b, "L%d:\n  type: object\n  properties:\n    next: {$ref: '#/components/schemas/L%d'}\n", i, i+1)
	}
	b.WriteString("L10: {type: string}\n")

	got := compact(sampleOf(t, b.String(), false))

	assert.Equal(t, `{"next":{"next":{"next":{"next":{"next":{"next":{}}}}}}}`, got)
}

func TestSampler_IsIndented(t *testing.T) {
	got := sampleOf(t, "Root:\n  type: object\n  properties:\n    a: {type: integer}", false)

	assert.Equal(t, "{\n  \"a\": 0\n}", got)
}
