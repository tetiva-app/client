package publication

import (
	"maps"
	"regexp"
	"slices"
	"strconv"
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

// BlockingError codes and params are a frontend contract: the UI words them itself.
const (
	codeTextTooLong         = "text_too_long"
	codeValueTooLong        = "value_too_long"
	codeNameTooLong         = "name_too_long"
	codeHeaderNameInvalid   = "header_name_invalid"
	codeURLTooLong          = "url_too_long"
	codeURLControlChar      = "url_control_char"
	codeNameBlank           = "name_blank"
	codeTooManyValues       = "too_many_values"
	codeTooMany             = "too_many"
	codeAuthTooDeep         = "auth_too_deep"
	codeAuthTooManyValues   = "auth_too_many_values"
	codeTooManyItems        = "too_many_items"
	codeFoldersTooDeep      = "folders_too_deep"
	codeMethodUnsupported   = "method_unsupported"
	codeProtocolUnsupported = "protocol_unsupported"
	codeBodyTypeUnsupported = "body_type_unsupported"
	codeStatusOutOfRange    = "status_out_of_range"
	codeAuthTypeUnsupported = "auth_type_unsupported"
)

var blockingText = map[string]string{
	codeTextTooLong:         "text longer than 1 MiB",
	codeValueTooLong:        "value longer than {limit} KiB",
	codeNameTooLong:         "name longer than {limit} characters",
	codeHeaderNameInvalid:   `header name "{name}" may contain only token characters and {{variable}} references`,
	codeURLTooLong:          "URL longer than 8 KiB",
	codeURLControlChar:      "URL contains a control character",
	codeNameBlank:           "collection name is blank",
	codeTooManyValues:       "{count} JSON values; at most {limit} can be published",
	codeTooMany:             "{count} {list}; at most {limit} can be published",
	codeAuthTooDeep:         "nested deeper than {limit} levels",
	codeAuthTooManyValues:   "{count} values inside; at most {limit} can be published",
	codeTooManyItems:        "{count} folders and requests; at most {limit} can be published",
	codeFoldersTooDeep:      "folders are nested deeper than {limit} levels",
	codeMethodUnsupported:   `HTTP method "{value}" cannot be published`,
	codeProtocolUnsupported: `protocol "{value}" cannot be published`,
	codeBodyTypeUnsupported: `body type "{value}" cannot be published`,
	codeStatusOutOfRange:    "status {value} is outside 0–999",
	codeAuthTypeUnsupported: `auth type "{value}" cannot be published`,
}

const (
	listHeaders      = "headers"
	listMetadata     = "metadata"
	listFormFields   = "form_fields"
	listSubprotocols = "subprotocols"
	listMessages     = "messages"
	listExamples     = "examples"
	listVariables    = "variables"
	listAuthFields   = "auth_fields"
)

var listNouns = map[string]string{
	listHeaders:      "headers",
	listMetadata:     "metadata entries",
	listFormFields:   "form fields",
	listSubprotocols: "subprotocols",
	listMessages:     "messages",
	listExamples:     "examples",
	listVariables:    "variables",
	listAuthFields:   "auth fields",
}

func newBlockingError(path, code string, params map[string]string) BlockingError {
	if params == nil {
		params = map[string]string{}
	}
	msg := blockingText[code]
	for k, v := range params {
		if k == "list" {
			v = listNouns[v]
		}
		msg = strings.ReplaceAll(msg, "{"+k+"}", v)
	}
	return BlockingError{Path: path, Code: code, Params: params, Message: msg}
}

func limitParams(limit int) map[string]string {
	return map[string]string{"limit": strconv.Itoa(limit)}
}

func countParams(n, limit int) map[string]string {
	return map[string]string{"count": strconv.Itoa(n), "limit": strconv.Itoa(limit)}
}

func valueParams(v string) map[string]string {
	return map[string]string{"value": v}
}

func checkLimits(s *Snapshot, owners envOwners) []BlockingError {
	var errs []BlockingError
	type key struct{ path, message string }
	seen := map[key]bool{}
	fail := func(path, code string, params map[string]string) {
		e := newBlockingError(path, code, params)
		if k := (key{e.Path, e.Message}); !seen[k] {
			seen[k] = true
			errs = append(errs, e)
		}
	}
	walkSnapshot(s, owners, nil, func(f field) {
		switch {
		case len(f.value) > maxStringBytes:
			fail(f.path, codeTextTooLong, nil)
		case f.maxBytes > 0 && len(f.value) > f.maxBytes:
			fail(f.path, codeValueTooLong, limitParams(f.maxBytes>>10))
		case f.maxRunes > 0 && len(f.value) > f.maxRunes && utf8.RuneCountInString(f.value) > f.maxRunes:
			fail(f.path, codeNameTooLong, limitParams(f.maxRunes))
		case f.kind == kindName && !headerName.MatchString(f.value):
			fail(f.path, codeHeaderNameInvalid, map[string]string{"name": shorten(f.value)})
		case f.kind == kindURL && len(f.value) > maxURLBytes:
			fail(f.path, codeURLTooLong, nil)
		case f.kind == kindURL && strings.ContainsFunc(f.value, func(r rune) bool { return r < 0x20 || r == 0x7f }):
			fail(f.path, codeURLControlChar, nil)
		}
	})
	// Publish sends the name as the page title, which the server refuses blank.
	if strings.TrimSpace(s.Collection.Name) == "" {
		fail("name", codeNameBlank, nil)
	}
	c := shape{fail: fail}
	c.snapshot(s)
	if c.values > maxValues {
		fail(s.Collection.Name, codeTooManyValues, countParams(c.values, maxValues))
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
	c := shape{fail: func(string, string, map[string]string) {}}
	c.snapshot(s)
	return c.values
}

// shape counts the JSON values snapshotjson.Marshal writes and checks the list lengths the server
// caps; paths follow walkSnapshot.
type shape struct {
	values int
	fail   func(path, code string, params map[string]string)
}

func (c *shape) list(path, list string, n, limit int) {
	if n > limit {
		params := countParams(n, limit)
		params["list"] = list
		c.fail(path, codeTooMany, params)
	}
}

func (c *shape) snapshot(s *Snapshot) {
	root := &s.Collection
	c.values += 6 // document, format, version, generator, locale, environment
	c.folder(&root.Folder, root.Name, true)
	c.headers(joinPath(root.Name, "metadata"), listMetadata, root.GRPCMetadata)
	if env := s.Environment; env != nil {
		c.list("Environment "+env.Name, listVariables, len(env.Variables), maxVariables)
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
		c.list(joinPath(path, "body"), listFormFields, len(h.Body.Fields), maxPairs)
		c.values += 8 + 5*len(h.Body.Fields) // http, method, url, body, type, raw, fields, fileName
		c.headers(joinPath(path, "headers"), listHeaders, h.Headers)
	}
	if g := r.GraphQL; g != nil {
		c.values += 5 // graphql, url, query, variables, operationName
		c.headers(joinPath(path, "headers"), listHeaders, g.Headers)
	}
	if g := r.GRPC; g != nil {
		c.values += 5 // grpc, target, service, method, message
		c.headers(joinPath(path, "metadata"), listMetadata, g.Metadata)
	}
	if ws := r.WebSocket; ws != nil {
		c.list(joinPath(path, "subprotocols"), listSubprotocols, len(ws.Subprotocols), maxSubprotocols)
		c.list(joinPath(path, "messages"), listMessages, len(ws.Messages), maxMessages)
		c.values += 4 + len(ws.Subprotocols) + 4*len(ws.Messages) // websocket, url, subprotocols, messages
		c.headers(joinPath(path, "headers"), listHeaders, ws.Headers)
	}
	c.auth(joinPath(path, "auth"), r.Auth)
	c.scripts(r.Scripts)
	c.list(joinPath(path, "examples"), listExamples, len(r.Examples), maxExamples)
	for _, e := range r.Examples {
		c.values += 7 // object, id, name, status, statusText, body, contentType
		c.headers(joinPath(path, "examples", e.Name, "headers"), listHeaders, e.Headers)
	}
}

func (c *shape) headers(path, list string, hs []Header) {
	c.list(path, list, len(hs), maxPairs)
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
	c.list(path, listAuthFields, len(a.Fields), maxAuthFields)
	c.values += 3 + len(a.Redacted) // type, fields, redacted
	for _, k := range slices.Sorted(maps.Keys(a.Fields)) {
		n, depth := measure(a.Fields[k])
		c.values += n
		switch {
		case depth > maxAuthDepth:
			c.fail(joinPath(path, k), codeAuthTooDeep, limitParams(maxAuthDepth))
		case n-1 > maxAuthValues:
			c.fail(joinPath(path, k), codeAuthTooManyValues, countParams(n-1, maxAuthValues))
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
