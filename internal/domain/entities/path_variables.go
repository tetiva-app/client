package entities

import (
	"slices"
	"strings"
)

type PathVariable struct {
	Key   string
	Value string
}

// GraphQL has no table to edit path values in; gRPC targets have no path.
func (p Protocol) HasPathVariables() bool {
	return p == ProtocolHTTP || p == ProtocolWebSocket
}

func pathSpan(rawURL string) (start, end int) {
	end = len(rawURL)
	if i := strings.IndexAny(rawURL, "?#"); i >= 0 {
		end = i
	}
	if i := strings.Index(rawURL[:end], "://"); i >= 0 {
		start = i + 3
	}
	if i := strings.IndexByte(rawURL[start:end], '/'); i >= 0 {
		return start + i, end
	}
	return end, end
}

func segmentKey(segment string) (string, bool) {
	if len(segment) < 2 || segment[0] != ':' {
		return "", false
	}
	return segment[1:], true
}

type pathSegment struct {
	start, end int
	key        string
}

func pathVariableSegments(rawURL string) []pathSegment {
	start, end := pathSpan(rawURL)
	var out []pathSegment
	offset := start
	for _, segment := range strings.Split(rawURL[start:end], "/") {
		if key, ok := segmentKey(segment); ok {
			out = append(out, pathSegment{start: offset, end: offset + len(segment), key: key})
		}
		offset += len(segment) + 1
	}
	return out
}

func PathVariableKeys(rawURL string) []string {
	var keys []string
	for _, s := range pathVariableSegments(rawURL) {
		if !slices.Contains(keys, s.key) {
			keys = append(keys, s.key)
		}
	}
	return keys
}

// Only / ? # are escaped, so an already encoded value is sent as written.
var pathValueEscaper = strings.NewReplacer("/", "%2F", "?", "%3F", "#", "%23")

// ResolveURL resolves each path value once, before escaping, so it cannot add a segment.
func ResolveURL(rawURL string, vars []PathVariable, resolve func(string) string) string {
	var b strings.Builder
	last := 0
	for _, s := range pathVariableSegments(rawURL) {
		j := slices.IndexFunc(vars, func(v PathVariable) bool { return v.Key == s.key })
		if j < 0 {
			continue
		}
		value := resolve(vars[j].Value)
		if value == "" {
			continue
		}
		b.WriteString(resolve(rawURL[last:s.start]))
		b.WriteString(pathValueEscaper.Replace(value))
		last = s.end
	}
	b.WriteString(resolve(rawURL[last:]))
	return b.String()
}

func ApplyPathVariables(rawURL string, vars []PathVariable) string {
	return ResolveURL(rawURL, vars, func(s string) string { return s })
}

func SyncPathVariables(rawURL string, vars []PathVariable) []PathVariable {
	keys := PathVariableKeys(rawURL)
	out := make([]PathVariable, 0, len(keys))
	for _, key := range keys {
		v := PathVariable{Key: key}
		if j := slices.IndexFunc(vars, func(p PathVariable) bool { return p.Key == key }); j >= 0 {
			v.Value = vars[j].Value
		}
		out = append(out, v)
	}
	return out
}
