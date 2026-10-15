package openapi

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/url"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/tetiva-app/client/internal/adapters/portability"
)

const (
	maxDocumentJSON  = 64 << 20
	maxDocumentDepth = 512
)

var (
	errTooLarge = fmt.Errorf("the document expands to more than %d MB", maxDocumentJSON>>20)
	errTooDeep  = fmt.Errorf("the document nests deeper than %d levels", maxDocumentDepth)
)

// source keeps the parsed YAML because kin-openapi loses key order.
type source struct {
	root     *yaml.Node
	swagger2 bool
	nodes    map[string]*yaml.Node
}

func parseSource(data []byte) (*source, error) {
	root, err := parseTree(data)
	if err != nil {
		return nil, err
	}
	if root == nil || root.Kind != yaml.MappingNode {
		return nil, errors.New("the document is not an object")
	}
	return &source{root: root, swagger2: scalar(root, "swagger") == "2.0", nodes: map[string]*yaml.Node{}}, nil
}

func parseTree(data []byte) (*yaml.Node, error) {
	if json.Valid(data) {
		return portability.JSONNode(data)
	}
	var doc yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil || len(doc.Content) == 0 {
		return nil, err
	}
	return doc.Content[0], nil
}

func scalar(n *yaml.Node, key string) string {
	if v := field(n, key); v != nil && v.Kind == yaml.ScalarNode {
		return v.Value
	}
	return ""
}

func field(n *yaml.Node, key string) *yaml.Node {
	if n == nil {
		return nil
	}
	if n.Kind == yaml.AliasNode {
		n = n.Alias
	}
	switch n.Kind {
	case yaml.MappingNode:
		for i := 0; i+1 < len(n.Content); i += 2 {
			if n.Content[i].Value == key {
				return n.Content[i+1]
			}
		}
	case yaml.SequenceNode:
		if i, err := strconv.Atoi(key); err == nil && i >= 0 && i < len(n.Content) {
			return n.Content[i]
		}
	}
	return nil
}

func keys(n *yaml.Node) []string {
	if n == nil || n.Kind != yaml.MappingNode {
		return nil
	}
	out := make([]string, 0, len(n.Content)/2)
	for i := 0; i+1 < len(n.Content); i += 2 {
		out = append(out, n.Content[i].Value)
	}
	return out
}

func ptr(base string, tokens ...string) string {
	if base == "" {
		return ""
	}
	var b strings.Builder
	b.WriteString(base)
	for _, t := range tokens {
		b.WriteByte('/')
		b.WriteString(strings.ReplaceAll(strings.ReplaceAll(t, "~", "~0"), "/", "~1"))
	}
	return b.String()
}

func (s *source) at(pointer string) *yaml.Node {
	if pointer == "" || pointer == "#" {
		return nil
	}
	if n, ok := s.nodes[pointer]; ok {
		return n
	}
	n := s.root
	for _, tok := range strings.Split(strings.TrimPrefix(pointer, "#/"), "/") {
		tok = strings.ReplaceAll(strings.ReplaceAll(tok, "~1", "/"), "~0", "~")
		if n = field(n, tok); n == nil {
			break
		}
	}
	s.nodes[pointer] = n
	return n
}

func (s *source) refPointer(ref string) string {
	frag, ok := strings.CutPrefix(ref, "#")
	if !ok {
		return ""
	}
	if unescaped, err := url.PathUnescape(frag); err == nil {
		frag = unescaped
	}
	if s.swagger2 {
		for v3, v2 := range swagger2Sections {
			if name, ok := strings.CutPrefix(frag, v3); ok {
				frag = v2 + name
				break
			}
		}
	}
	return "#" + frag
}

var swagger2Sections = map[string]string{
	"/components/schemas/":    "/definitions/",
	"/components/responses/":  "/responses/",
	"/components/parameters/": "/parameters/",
}

func (s *source) follow(ref, pointer string) string {
	if ref != "" {
		return s.refPointer(ref)
	}
	return pointer
}

func (s *source) stubBrokenRefs() []portability.Warning {
	var warnings []portability.Warning
	warned := map[string]bool{}
	warn := func(key string, code portability.WarningCode, param string) {
		if !warned[key] {
			warned[key] = true
			warnings = append(warnings, portability.Warn(code, portability.Subject{}, param, key))
		}
	}
	var walk func(n *yaml.Node)
	walk = func(n *yaml.Node) {
		if n.Kind == yaml.MappingNode {
			if ref := scalar(n, "$ref"); ref != "" {
				target, _, _ := strings.Cut(ref, "#")
				switch {
				case target != "":
					warn(target, portability.WarnOpenAPIExternalRef, "target")
					n.Content = nil
					return
				case s.at(s.refPointer(ref)) == nil:
					warn(ref, portability.WarnOpenAPIBrokenRef, "ref")
					n.Content = nil
					return
				}
			}
		}
		for _, c := range n.Content {
			walk(c)
		}
	}
	walk(s.root)
	return warnings
}

// keepText keeps unquoted YAML numbers such as `openapi: 3.1` as text for the loader.
func (s *source) keepText() {
	info := field(s.root, "info")
	for _, n := range []*yaml.Node{field(s.root, "openapi"), field(s.root, "swagger"), field(info, "title"), field(info, "version")} {
		asText(n)
	}
	for _, server := range children(field(s.root, "servers")) {
		for _, v := range children(field(server, "variables")) {
			asText(field(v, "default"))
			for _, e := range children(field(v, "enum")) {
				asText(e)
			}
		}
	}
	for _, tag := range children(field(s.root, "tags")) {
		asText(field(tag, "name"))
	}
	for _, item := range children(field(s.root, "paths")) {
		for _, method := range methods {
			for _, tag := range children(field(field(item, strings.ToLower(method)), "tags")) {
				asText(tag)
			}
		}
	}
}

func children(n *yaml.Node) []*yaml.Node {
	if n != nil && n.Kind == yaml.AliasNode {
		n = n.Alias
	}
	switch {
	case n == nil:
		return nil
	case n.Kind == yaml.SequenceNode:
		return n.Content
	case n.Kind == yaml.MappingNode:
		out := make([]*yaml.Node, 0, len(n.Content)/2)
		for i := 1; i < len(n.Content); i += 2 {
			out = append(out, n.Content[i])
		}
		return out
	}
	return nil
}

func asText(n *yaml.Node) {
	if n != nil && n.Kind == yaml.AliasNode {
		n = n.Alias
	}
	if n != nil && n.Kind == yaml.ScalarNode && n.ShortTag() != "!!null" {
		n.Tag = "!!str"
	}
}

func writeJSON(buf *bytes.Buffer, n *yaml.Node, budget int) error {
	w := &jsonWriter{buf: buf, budget: budget, open: map[*yaml.Node]bool{}}
	return w.write(n, 0)
}

type jsonWriter struct {
	buf    *bytes.Buffer
	budget int
	open   map[*yaml.Node]bool
}

func (w *jsonWriter) write(n *yaml.Node, depth int) error {
	switch {
	case w.buf.Len() > w.budget:
		return errTooLarge
	case depth > maxDocumentDepth:
		return errTooDeep
	}
	if n.Anchor != "" {
		if w.open[n] {
			return fmt.Errorf("the anchor &%s refers to itself", n.Anchor)
		}
		w.open[n] = true
		defer delete(w.open, n)
	}
	buf := w.buf
	switch n.Kind {
	case yaml.DocumentNode:
		if len(n.Content) == 0 {
			buf.WriteString("null")
			return nil
		}
		return w.write(n.Content[0], depth)
	case yaml.AliasNode:
		return w.write(n.Alias, depth+1)
	case yaml.MappingNode:
		buf.WriteByte('{')
		for i := 0; i+1 < len(n.Content); i += 2 {
			if i > 0 {
				buf.WriteByte(',')
			}
			writeString(buf, n.Content[i].Value)
			buf.WriteByte(':')
			if err := w.write(n.Content[i+1], depth+1); err != nil {
				return err
			}
		}
		buf.WriteByte('}')
	case yaml.SequenceNode:
		buf.WriteByte('[')
		for i, c := range n.Content {
			if i > 0 {
				buf.WriteByte(',')
			}
			if err := w.write(c, depth+1); err != nil {
				return err
			}
		}
		buf.WriteByte(']')
	default:
		writeScalar(buf, n)
	}
	return nil
}

func writeScalar(buf *bytes.Buffer, n *yaml.Node) {
	switch n.ShortTag() {
	case "!!null":
		buf.WriteString("null")
		return
	case "!!bool":
		if b, err := strconv.ParseBool(n.Value); err == nil {
			buf.WriteString(strconv.FormatBool(b))
			return
		}
	case "!!int", "!!float":
		if json.Valid([]byte(n.Value)) {
			buf.WriteString(n.Value)
			return
		}
		if i, err := strconv.ParseInt(strings.ReplaceAll(n.Value, "_", ""), 0, 64); err == nil {
			buf.WriteString(strconv.FormatInt(i, 10))
			return
		}
		if f, err := strconv.ParseFloat(n.Value, 64); err == nil && !math.IsInf(f, 0) && !math.IsNaN(f) {
			buf.WriteString(strconv.FormatFloat(f, 'g', -1, 64))
			return
		}
	}
	writeString(buf, n.Value)
}

func writeString(buf *bytes.Buffer, s string) {
	enc := json.NewEncoder(buf)
	enc.SetEscapeHTML(false)
	_ = enc.Encode(s)
	buf.Truncate(buf.Len() - 1)
}
