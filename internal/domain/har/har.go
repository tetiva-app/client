// Package har turns a prepared request into the normalised HAR 1.2 request the
// code-snippet generators consume.
package har

import (
	"fmt"
	"net/url"
	"regexp"
	"slices"
	"strings"
	"unicode/utf8"
)

// varPattern mirrors the client's variable syntax: the name is taken verbatim, without trimming.
var varPattern = regexp.MustCompile(`\{\{([^}]+)\}\}`)

const httpVersion = "HTTP/1.1"

type NameValue struct{ Name, Value string }

// Param is a form field; FileName != "" marks a file part, whose Value is empty.
type Param struct{ Name, Value, FileName, ContentType string }

type PostData struct {
	MimeType string
	Text     string
	Params   []Param
}

type Request struct {
	Method      string
	URL         string // without query, userinfo and fragment
	HTTPVersion string // always "HTTP/1.1"
	Headers     []NameValue
	QueryString []NameValue
	PostData    *PostData
	BinaryFile  string // base name; rendered by the snippet wrapper
	AuthNote    string // "digest" | "aws_sigv4" | ""; the wrapper prints a comment
}

type BodyKind int

const (
	BodyNone BodyKind = iota
	BodyText
	BodyURLEncoded
	BodyMultipart
	BodyBinary
)

type Body struct {
	Kind       BodyKind
	MimeType   string
	Text       string
	Params     []Param
	BinaryFile string // may be a full path or contain placeholders; Build keeps the base name
}

type Input struct {
	Method   string
	URL      string
	Headers  map[string][]string
	Body     Body
	AuthNote string
}

// Build normalises in into a HAR request and returns warnings about what some
// target languages cannot reproduce faithfully.
func Build(in Input) (Request, []string) {
	base, rawQuery := splitURL(in.URL)
	req := Request{
		Method:      in.Method,
		URL:         base,
		HTTPVersion: httpVersion,
		QueryString: parseQuery(rawQuery),
		AuthNote:    in.AuthNote,
	}

	var warnings []string
	req.Headers, warnings = buildHeaders(in.Headers, in.Body.Kind == BodyMultipart)

	switch in.Body.Kind {
	case BodyText:
		req.PostData = &PostData{MimeType: in.Body.MimeType, Text: in.Body.Text}
	case BodyURLEncoded:
		req.PostData = &PostData{
			MimeType: orDefault(in.Body.MimeType, "application/x-www-form-urlencoded"),
			Params:   slices.Clone(in.Body.Params),
		}
		warnings = append(warnings, repeatedKeyWarnings(in.Body.Params)...)
	case BodyMultipart:
		var params []Param
		for _, p := range in.Body.Params {
			if p.FileName != "" {
				p.FileName = baseName(p.FileName)
				p.Value = ""
			}
			params = append(params, p)
		}
		// The boundary belongs to the client library that builds the body.
		mime, _, _ := strings.Cut(in.Body.MimeType, ";")
		req.PostData = &PostData{
			MimeType: orDefault(strings.TrimSpace(mime), "multipart/form-data"),
			Params:   params,
		}
	case BodyBinary:
		req.BinaryFile = baseName(in.Body.BinaryFile)
		req.PostData = &PostData{MimeType: in.Body.MimeType}
		warnings = append(warnings, "Binary body is shown as a file reference")
	}

	return req, warnings
}

// splitURL separates the query and drops the fragment and userinfo without
// looking inside {{…}}, so a '#' or '?' in a variable name is not a delimiter.
func splitURL(raw string) (base, query string) {
	spans := varPattern.FindAllStringIndex(raw, -1)
	if i := indexOutside(raw, "#", 0, spans); i >= 0 {
		raw = raw[:i]
	}
	if i := indexOutside(raw, "?", 0, spans); i >= 0 {
		raw, query = raw[:i], raw[i+1:]
	}

	scheme := indexOutside(raw, "://", 0, spans)
	if scheme < 0 {
		return raw, query
	}
	start := scheme + len("://")
	end := indexOutside(raw, "/", start, spans)
	if end < 0 {
		end = len(raw)
	}
	for i := end - 1; i >= start; i-- {
		if raw[i] == '@' && outside(i, spans) {
			return raw[:start] + raw[i+1:], query
		}
	}
	return raw, query
}

func parseQuery(q string) []NameValue {
	var out []NameValue
	for _, seg := range splitOutside(q, '&') {
		if seg == "" {
			continue
		}
		name, value := seg, ""
		if i := indexOutside(seg, "=", 0, varPattern.FindAllStringIndex(seg, -1)); i >= 0 {
			name, value = seg[:i], seg[i+1:]
		}
		out = append(out, NameValue{Name: unescape(name), Value: unescape(value)})
	}
	return out
}

// unescape decodes query escapes outside {{…}}; a piece that fails to decode stays raw.
func unescape(s string) string {
	var b strings.Builder
	last := 0
	for _, sp := range varPattern.FindAllStringIndex(s, -1) {
		b.WriteString(queryUnescape(s[last:sp[0]]))
		b.WriteString(s[sp[0]:sp[1]])
		last = sp[1]
	}
	b.WriteString(queryUnescape(s[last:]))
	return b.String()
}

func queryUnescape(s string) string {
	if v, err := url.QueryUnescape(s); err == nil {
		return v
	}
	return s
}

func buildHeaders(h map[string][]string, dropContentType bool) ([]NameValue, []string) {
	keys := make([]string, 0, len(h))
	for k, values := range h {
		if len(values) == 0 || (dropContentType && strings.EqualFold(k, "Content-Type")) {
			continue
		}
		keys = append(keys, k)
	}
	slices.Sort(keys)

	var headers []NameValue
	var warnings []string
	for _, k := range keys {
		sep := ", "
		if strings.EqualFold(k, "Cookie") {
			sep = "; "
		}
		v := strings.Join(h[k], sep)
		if !isASCII(v) {
			warnings = append(warnings, fmt.Sprintf("Header %q has non-ASCII characters; some languages cannot send it", k))
		}
		headers = append(headers, NameValue{Name: k, Value: v})
	}
	return headers, warnings
}

func repeatedKeyWarnings(params []Param) []string {
	var warnings []string
	seen := make(map[string]int, len(params))
	for _, p := range params {
		seen[p.Name]++
		if seen[p.Name] == 2 {
			warnings = append(warnings, fmt.Sprintf("Repeated form key %q: some languages keep only the last value", p.Name))
		}
	}
	return warnings
}

// baseName splits on both separators because a collection synced from Windows
// may carry backslash paths onto macOS or Linux.
func baseName(path string) string {
	spans := varPattern.FindAllStringIndex(path, -1)
	path = strings.TrimRight(path, `/\`)
	for i := len(path) - 1; i >= 0; i-- {
		if (path[i] == '/' || path[i] == '\\') && outside(i, spans) {
			return path[i+1:]
		}
	}
	return path
}

func splitOutside(s string, sep byte) []string {
	spans := varPattern.FindAllStringIndex(s, -1)
	var parts []string
	last := 0
	for i := 0; i < len(s); i++ {
		if s[i] == sep && outside(i, spans) {
			parts = append(parts, s[last:i])
			last = i + 1
		}
	}
	return append(parts, s[last:])
}

func indexOutside(s, sub string, from int, spans [][]int) int {
	for i := from; i+len(sub) <= len(s); i++ {
		if strings.HasPrefix(s[i:], sub) && outside(i, spans) {
			return i
		}
	}
	return -1
}

func outside(i int, spans [][]int) bool {
	for _, sp := range spans {
		if i >= sp[0] && i < sp[1] {
			return false
		}
	}
	return true
}

func isASCII(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] >= utf8.RuneSelf {
			return false
		}
	}
	return true
}

func orDefault(s, def string) string {
	if s == "" {
		return def
	}
	return s
}
