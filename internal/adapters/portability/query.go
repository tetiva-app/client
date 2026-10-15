package portability

import (
	"net/url"
	"regexp"
	"strings"
)

func QueryPair(key, value string) string {
	return queryEscape(key) + "=" + queryEscape(value)
}

var keptInQuery = regexp.MustCompile(varPattern.String() + `|%[0-9A-Fa-f]{2}`)

func queryEscape(s string) string {
	var b strings.Builder
	last := 0
	for _, m := range keptInQuery.FindAllStringIndex(s, -1) {
		b.WriteString(url.QueryEscape(s[last:m[0]]))
		b.WriteString(s[m[0]:m[1]])
		last = m[1]
	}
	b.WriteString(url.QueryEscape(s[last:]))
	return b.String()
}

func WithQuery(rawURL string, pairs []string) string {
	if len(pairs) == 0 {
		return rawURL
	}
	base, fragment, hasFragment := strings.Cut(rawURL, "#")
	sep := "?"
	if strings.Contains(base, "?") {
		sep = "&"
	}
	u := base + sep + strings.Join(pairs, "&")
	if hasFragment {
		u += "#" + fragment
	}
	return u
}
