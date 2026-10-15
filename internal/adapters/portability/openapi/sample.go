package openapi

import (
	"bytes"
	"encoding/json"
	"slices"
	"strconv"

	"github.com/getkin/kin-openapi/openapi3"
	"gopkg.in/yaml.v3"
)

const (
	maxSampleDepth = 6
	maxSampleBytes = 64 << 10
)

var stringSamples = map[string]string{
	"date-time": "2024-01-01T12:00:00Z",
	"date":      "2024-01-01",
	"time":      "12:00:00",
	"uuid":      "3fa85f64-5717-4562-b3fc-2c963f66afa6",
	"email":     "user@example.com",
	"uri":       "https://example.com",
	"url":       "https://example.com",
	"hostname":  "example.com",
	"ipv4":      "192.168.0.1",
	"ipv6":      "::1",
	"byte":      "c3RyaW5n",
}

type sampler struct {
	src     *source
	request bool
	active  seenSchemas
	buf     bytes.Buffer
}

func newSampler(src *source, request bool) *sampler {
	return &sampler{src: src, request: request, active: seenSchemas{}}
}

type seenSchemas map[*openapi3.Schema]bool

func (seen seenSchemas) visit(s *openapi3.Schema) bool {
	if s == nil || seen[s] {
		return false
	}
	seen[s] = true
	return true
}

func (g *sampler) json(ref *openapi3.SchemaRef, pointer string) string {
	g.buf.Reset()
	g.write(ref, pointer, 0)
	return indentJSON(g.buf.Bytes())
}

func (g *sampler) write(ref *openapi3.SchemaRef, pointer string, depth int) {
	if ref == nil || ref.Value == nil {
		g.buf.WriteString("{}")
		return
	}
	pointer = g.src.follow(ref.Ref, pointer)
	s := ref.Value
	if g.literal(s, pointer) {
		return
	}
	if depth >= maxSampleDepth || g.buf.Len() > maxSampleBytes || !g.active.visit(s) {
		g.buf.WriteString(placeholder(s))
		return
	}
	defer delete(g.active, s)

	switch {
	case len(s.OneOf) > 0:
		g.write(s.OneOf[0], ptr(pointer, "oneOf", "0"), depth)
	case len(s.AnyOf) > 0:
		g.write(s.AnyOf[0], ptr(pointer, "anyOf", "0"), depth)
	case typeOf(s) == "object":
		g.object(s, pointer, depth)
	case typeOf(s) == "array":
		g.buf.WriteByte('[')
		if s.Items != nil {
			g.write(s.Items, ptr(pointer, "items"), depth+1)
		}
		g.buf.WriteByte(']')
	default:
		g.buf.WriteString(placeholder(s))
	}
}

func (g *sampler) literal(s *openapi3.Schema, pointer string) bool {
	scalar := typeOf(s) != "object" && typeOf(s) != "array"
	switch {
	case s.Example != nil:
		writeValue(&g.buf, g.src, ptr(pointer, "example"), s.Example)
	case len(s.Examples) > 0:
		writeValue(&g.buf, g.src, ptr(pointer, "examples", "0"), s.Examples[0])
	case s.Const != nil:
		writeValue(&g.buf, g.src, ptr(pointer, "const"), s.Const)
	case scalar && s.Default != nil:
		writeValue(&g.buf, g.src, ptr(pointer, "default"), s.Default)
	case scalar && len(s.Enum) > 0:
		writeValue(&g.buf, g.src, ptr(pointer, "enum", "0"), s.Enum[0])
	default:
		return false
	}
	return true
}

func (g *sampler) object(s *openapi3.Schema, pointer string, depth int) {
	g.buf.WriteByte('{')
	n := 0
	for _, p := range g.properties(s, pointer, 0, seenSchemas{}) {
		if v := p.ref.Value; v != nil && (g.request && v.ReadOnly || !g.request && v.WriteOnly) {
			continue
		}
		if n > 0 {
			g.buf.WriteByte(',')
		}
		writeString(&g.buf, p.name)
		g.buf.WriteByte(':')
		g.write(p.ref, p.pointer, depth+1)
		n++
	}
	if extra := s.AdditionalProperties.Schema; n == 0 && extra != nil {
		g.buf.WriteString(`"key":`)
		g.write(extra, ptr(pointer, "additionalProperties"), depth+1)
	}
	g.buf.WriteByte('}')
}

type property struct {
	name    string
	ref     *openapi3.SchemaRef
	pointer string
}

func (g *sampler) properties(s *openapi3.Schema, pointer string, depth int, merged seenSchemas) []property {
	var out []property
	index := map[string]int{}
	add := func(p property) {
		if i, ok := index[p.name]; ok {
			out[i] = p
			return
		}
		index[p.name] = len(out)
		out = append(out, p)
	}
	if depth < maxSampleDepth {
		for i, part := range s.AllOf {
			if part == nil || !merged.visit(part.Value) {
				continue
			}
			for _, p := range g.properties(part.Value, g.src.follow(part.Ref, ptr(pointer, "allOf", strconv.Itoa(i))), depth+1, merged) {
				add(p)
			}
		}
	}
	for _, name := range orderedKeys(s.Properties, g.src.at(ptr(pointer, "properties"))) {
		if ref := s.Properties[name]; ref != nil {
			add(property{name: name, ref: ref, pointer: ptr(pointer, "properties", name)})
		}
	}
	return out
}

func typeOf(s *openapi3.Schema) string {
	return seenSchemas{}.typeOf(s)
}

func (seen seenSchemas) typeOf(s *openapi3.Schema) string {
	if !seen.visit(s) {
		return ""
	}
	if s.Type != nil {
		for _, t := range s.Type.Slice() {
			if t != "null" {
				return t
			}
		}
		if s.Type.Includes("null") {
			return "null"
		}
	}
	switch {
	case len(s.Properties) > 0 || s.AdditionalProperties.Schema != nil:
		return "object"
	case s.Items != nil:
		return "array"
	}
	for _, part := range s.AllOf {
		if part != nil {
			if t := seen.typeOf(part.Value); t != "" {
				return t
			}
		}
	}
	return ""
}

func placeholder(s *openapi3.Schema) string {
	switch typeOf(s) {
	case "integer", "number":
		return "0"
	case "boolean":
		return "true"
	case "null":
		return "null"
	case "array":
		return "[]"
	case "string":
		sample, ok := stringSamples[s.Format]
		if !ok {
			sample = "string"
		}
		return strconv.Quote(sample)
	}
	return "{}"
}

func writeValue(buf *bytes.Buffer, src *source, pointer string, v any) {
	if n := src.at(pointer); n != nil {
		var raw bytes.Buffer
		if writeJSON(&raw, n, maxSampleBytes) == nil {
			buf.Write(raw.Bytes())
			return
		}
	}
	enc := json.NewEncoder(buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		buf.WriteString("null")
		return
	}
	buf.Truncate(buf.Len() - 1)
}

func indentJSON(raw []byte) string {
	var out bytes.Buffer
	if err := json.Indent(&out, raw, "", "  "); err != nil {
		return string(raw)
	}
	return out.String()
}

func orderedKeys[V any](m map[string]V, n *yaml.Node) []string {
	out := make([]string, 0, len(m))
	seen := make(map[string]bool, len(m))
	for _, k := range keys(n) {
		if _, ok := m[k]; ok && !seen[k] {
			seen[k] = true
			out = append(out, k)
		}
	}
	var rest []string
	for k := range m {
		if !seen[k] {
			rest = append(rest, k)
		}
	}
	slices.Sort(rest)
	return append(out, rest...)
}
