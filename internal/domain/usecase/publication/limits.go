package publication

import (
	"fmt"
	"maps"
	"regexp"
	"slices"
	"strings"
	"unicode/utf8"
)

// Limits of the server's snapshot validator, of which the schema (proto/snapshot/collection-snapshot.v1.schema.json)
// expresses only part. The 8 MiB / 3.5 MiB gzip size limits are checked by the caller on the encoded snapshot.
const (
	maxStringBytes = 1 << 20
	maxURLBytes    = 8 << 10
	maxFolderDepth = 16
	maxItems       = 5000
	maxValues      = 500_000

	maxNameRunes      = 512
	maxPairValueBytes = 16 << 10
	maxVariableBytes  = 64 << 10
	maxPairs          = 200
	maxMessages       = 200
	maxSubprotocols   = 64
	maxExamples       = 50
	maxVariables      = 1000
	maxAuthFields     = 64
	maxAuthDepth      = 4
	maxAuthValues     = 256
)

var headerName = regexp.MustCompile("^([!#$%&'*+.^_`|~0-9A-Za-z-]|\\{\\{[^}\\x00-\\x1f\\x7f]+\\}\\})+$")

func checkLimits(s *Snapshot, owners envOwners) []BlockingError {
	var errs []BlockingError
	seen := map[BlockingError]bool{}
	fail := func(path, format string, args ...any) {
		e := BlockingError{Path: path, Message: fmt.Sprintf(format, args...)}
		if !seen[e] {
			seen[e] = true
			errs = append(errs, e)
		}
	}
	walkSnapshot(s, owners, nil, func(f field) {
		switch {
		case len(f.value) > maxStringBytes:
			fail(f.path, "text longer than 1 MiB")
		case f.maxBytes > 0 && len(f.value) > f.maxBytes:
			fail(f.path, "value longer than %d KiB", f.maxBytes>>10)
		case f.maxRunes > 0 && len(f.value) > f.maxRunes && utf8.RuneCountInString(f.value) > f.maxRunes:
			fail(f.path, "name longer than %d characters", f.maxRunes)
		case f.kind == kindName && !headerName.MatchString(f.value):
			fail(f.path, "header name %q may contain only token characters and {{variable}} references", shorten(f.value))
		case f.kind == kindURL && len(f.value) > maxURLBytes:
			fail(f.path, "URL longer than 8 KiB")
		case f.kind == kindURL && strings.ContainsFunc(f.value, func(r rune) bool { return r < 0x20 || r == 0x7f }):
			fail(f.path, "URL contains a control character")
		}
	})
	// Publish sends the name as the page title, which the server refuses blank.
	if strings.TrimSpace(s.Collection.Name) == "" {
		fail("name", "collection name is blank")
	}
	c := shape{fail: fail}
	c.snapshot(s)
	if c.values > maxValues {
		fail(s.Collection.Name, "%d JSON values; at most %d can be published", c.values, maxValues)
	}
	return errs
}

func shorten(s string) string {
	if r := []rune(s); len(r) > 64 {
		return string(r[:64]) + "…"
	}
	return s
}

func countValues(s *Snapshot) int {
	c := shape{fail: func(string, string, ...any) {}}
	c.snapshot(s)
	return c.values
}

// shape counts the JSON values snapshotjson.Marshal writes and checks the list lengths the server
// caps; paths follow walkSnapshot.
type shape struct {
	values int
	fail   func(path, format string, args ...any)
}

func (c *shape) list(path, what string, n, limit int) {
	if n > limit {
		c.fail(path, "%d %s; at most %d can be published", n, what, limit)
	}
}

func (c *shape) snapshot(s *Snapshot) {
	root := &s.Collection
	c.values += 6 // document, format, version, generator, locale, environment
	c.folder(&root.Folder, root.Name, true)
	c.headers(joinPath(root.Name, "metadata"), "metadata entries", root.GRPCMetadata)
	if env := s.Environment; env != nil {
		c.list("Environment "+env.Name, "variables", len(env.Variables), maxVariables)
		c.values += 2 + 4*len(env.Variables)
	}
}

func (c *shape) folder(f *Folder, path string, isRoot bool) {
	c.values += 5 // object, id, name, description, items
	prefix := path
	if isRoot {
		prefix = ""
	} else {
		c.values++ // kind
	}
	c.auth(joinPath(path, "auth"), f.Auth)
	c.scripts(f.Scripts)
	for _, it := range f.Items {
		if it.Folder != nil {
			c.folder(it.Folder, joinPath(prefix, it.Folder.Name), false)
		} else if it.Request != nil {
			c.request(it.Request, joinPath(prefix, it.Request.Name))
		}
	}
}

func (c *shape) request(r *Request, path string) {
	c.values += 6 // object, kind, id, name, description, examples
	if r.Protocol != "" {
		c.values++
	}
	if h := r.HTTP; h != nil {
		c.list(joinPath(path, "body"), "form fields", len(h.Body.Fields), maxPairs)
		c.values += 8 + 5*len(h.Body.Fields) // http, method, url, body, type, raw, fields, fileName
		c.headers(joinPath(path, "headers"), "headers", h.Headers)
	}
	if g := r.GraphQL; g != nil {
		c.values += 5 // graphql, url, query, variables, operationName
		c.headers(joinPath(path, "headers"), "headers", g.Headers)
	}
	if g := r.GRPC; g != nil {
		c.values += 5 // grpc, target, service, method, message
		c.headers(joinPath(path, "metadata"), "metadata entries", g.Metadata)
	}
	if ws := r.WebSocket; ws != nil {
		c.list(joinPath(path, "subprotocols"), "subprotocols", len(ws.Subprotocols), maxSubprotocols)
		c.list(joinPath(path, "messages"), "messages", len(ws.Messages), maxMessages)
		c.values += 4 + len(ws.Subprotocols) + 4*len(ws.Messages) // websocket, url, subprotocols, messages
		c.headers(joinPath(path, "headers"), "headers", ws.Headers)
	}
	c.auth(joinPath(path, "auth"), r.Auth)
	c.scripts(r.Scripts)
	c.list(joinPath(path, "examples"), "examples", len(r.Examples), maxExamples)
	for _, e := range r.Examples {
		c.values += 7 // object, id, name, status, statusText, body, contentType
		c.headers(joinPath(path, "examples", e.Name, "headers"), "headers", e.Headers)
	}
}

func (c *shape) headers(path, what string, hs []Header) {
	c.list(path, what, len(hs), maxPairs)
	c.values += 1 + 5*len(hs)
}

func (c *shape) scripts(s *Scripts) {
	c.values++
	if s != nil {
		c.values += 2
	}
}

// auth needs no separate check of Redacted: Build lists only keys of Fields there.
func (c *shape) auth(path string, a *Auth) {
	c.values++
	if a == nil {
		return
	}
	c.list(path, "auth fields", len(a.Fields), maxAuthFields)
	c.values += 3 + len(a.Redacted) // type, fields, redacted
	for _, k := range slices.Sorted(maps.Keys(a.Fields)) {
		n, depth := measure(a.Fields[k])
		c.values += n
		switch {
		case depth > maxAuthDepth:
			c.fail(joinPath(path, k), "nested deeper than %d levels", maxAuthDepth)
		case n-1 > maxAuthValues:
			c.fail(joinPath(path, k), "%d values inside; at most %d can be published", n-1, maxAuthValues)
		}
	}
}

// measure returns the JSON values in v, v included, and how many arrays and objects deep it nests.
func measure(v any) (values, depth int) {
	var children []any
	switch t := v.(type) {
	case map[string]any:
		children = slices.Collect(maps.Values(t))
	case []any:
		children = t
	default:
		return 1, 0
	}
	values = 1
	for _, sub := range children {
		n, d := measure(sub)
		values += n
		depth = max(depth, d)
	}
	return values, depth + 1
}
