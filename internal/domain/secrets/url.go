package secrets

import (
	"net/url"
	"regexp"
	"strings"
)

var (
	// Non-greedy, so a "<redacted>" inside the URL survives a second pass.
	linkValue  = regexp.MustCompile(`<(.*?)>(\s*(?:[;,]|$))`)
	refreshURL = regexp.MustCompile(`(?i)(url\s*=\s*)(.*)$`)
	urlHeaders = map[string]func(string) string{
		"location":         RedactURL,
		"content-location": RedactURL,
		"link":             redactLink,
		"refresh":          redactRefresh,
	}
)

func ParamKey(name string) string {
	if decoded, err := url.QueryUnescape(name); err == nil {
		name = decoded
	}
	return strings.ToLower(strings.TrimSpace(name))
}

// SplitQuery is hand-written, not net/url, so {{var}} placeholders survive untouched.
func SplitQuery(raw string) (prefix, query, suffix string, ok bool) {
	q := strings.IndexByte(raw, '?')
	if q < 0 {
		return "", "", "", false
	}
	prefix, query = raw[:q+1], raw[q+1:]
	if frag := strings.IndexByte(query, '#'); frag >= 0 {
		suffix, query = query[frag:], query[:frag]
	}
	if query == "" {
		return "", "", "", false
	}
	return prefix, query, suffix, true
}

// MaskQuery touches only sensitive values: MCP restores echoed masks by position.
func MaskQuery(raw, mask string) string {
	prefix, query, suffix, ok := SplitQuery(raw)
	if !ok {
		return raw
	}
	masked, changed := maskParams(query, mask)
	if !changed {
		return raw
	}
	return prefix + masked + suffix
}

func RedactURL(raw string) string {
	base, frag, hasFrag := strings.Cut(raw, "#")
	base = MaskQuery(redactUserinfo(base), redactedValue)
	if !hasFrag {
		return base
	}
	masked, _ := maskParams(frag, redactedValue)
	return base + "#" + masked
}

func maskParams(params, mask string) (string, bool) {
	parts := strings.Split(params, "&")
	changed := false
	for i, part := range parts {
		name, value, ok := strings.Cut(part, "=")
		if !ok || value == "" || onlyRefs(value) || !IsSensitiveQueryParam(name) {
			continue
		}
		if value != mask {
			parts[i] = name + "=" + mask
			changed = true
		}
	}
	return strings.Join(parts, "&"), changed
}

func redactUserinfo(raw string) string {
	scheme := strings.Index(raw, "://")
	if scheme < 0 {
		return raw
	}
	start := scheme + 3
	end := len(raw)
	if i := strings.IndexAny(raw[start:], "/?"); i >= 0 {
		end = start + i
	}
	at := strings.LastIndexByte(raw[start:end], '@')
	if at < 0 {
		return raw
	}
	userinfo := raw[start : start+at]
	if onlyRefs(strings.ReplaceAll(userinfo, ":", "")) {
		return raw
	}
	return raw[:start] + redactedValue + raw[start+at:]
}

func redactLink(value string) string {
	return linkValue.ReplaceAllStringFunc(value, func(m string) string {
		sub := linkValue.FindStringSubmatch(m)
		return "<" + RedactURL(sub[1]) + ">" + sub[2]
	})
}

func redactRefresh(value string) string {
	loc := refreshURL.FindStringSubmatchIndex(value)
	if loc == nil {
		return value
	}
	target := value[loc[4]:loc[5]]
	quote := ""
	if len(target) >= 2 && (target[0] == '\'' || target[0] == '"') && target[len(target)-1] == target[0] {
		quote, target = target[:1], target[1:len(target)-1]
	}
	return value[:loc[4]] + quote + RedactURL(target) + quote
}

func onlyRefs(v string) bool {
	return strings.TrimSpace(varRef.ReplaceAllString(v, "")) == ""
}
