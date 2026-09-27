package publication

import (
	"encoding/json"
	"fmt"
	"maps"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/tetiva-app/client/internal/domain/entities"
	"github.com/tetiva-app/client/internal/domain/secrets"
	"github.com/tetiva-app/client/internal/domain/usecase/auth"
)

const redactedMark = "<redacted>"

const (
	reasonReferenced = "referenced from a secret field"
	reasonAuth       = "secret auth field; keep its value in a secret variable"
	reasonHeader     = "sensitive header"
	reasonMetadata   = "sensitive metadata"
	reasonQuery      = "sensitive query parameter"
	reasonUserinfo   = "credentials in the URL"
	reasonForm       = "sensitive form field"
	reasonFile       = "only the file name is published"
	reasonScript     = "scripts are left out"
	reasonCookie     = "cookies are never published"
)

var authScheme = regexp.MustCompile(`(?i)^(?:bearer|basic|token|digest|api-?key)[ \t]+`)

// headers scrubs one header list; the redaction key is the row's index in the stored slice.
func (p *pass) headers(owner, path string, rows []entities.HeaderItem, category string) []Header {
	section, reason := "headers", reasonHeader
	if category == CategoryMetadata {
		section, reason = "metadata", reasonMetadata
	}
	var out []Header
	for i, h := range rows {
		key := strings.TrimSpace(h.Key)
		if key == "" {
			continue
		}
		rowPath := joinPath(path, section, key)
		if isCookieHeader(key) || isCookieHeader(substitute(key, p.public)) {
			p.addRefs(h.Value)
			p.redact(Redaction{Selector: selector(owner, CategoryCookie, indexKey(i)), Path: rowPath, Category: CategoryCookie, Reason: reasonCookie})
			continue
		}
		if isSubprotocolHeader(key) {
			var entries []string
			for _, e := range subprotocolEntries(h.Value) {
				entries = append(entries, h.Value[e.start:e.end])
			}
			p.subprotocolRefs(entries)
		}
		p.bodySecret(h.Value)
		value, redacted := h.Value, false
		if sensitive, nameRefs := p.headerSensitive(key, h.Value); sensitive {
			p.addRefs(h.Value)
			if nameRefs {
				p.addRefs(key)
			}
			value = secrets.RedactValue(h.Value)
			redacted = value != h.Value || strings.Contains(value, redactedMark)
			if value != h.Value {
				p.redact(Redaction{Selector: selector(owner, category, indexKey(i)), Path: rowPath, Category: category, Reason: reason})
			}
		}
		out = append(out, Header{Key: key, Value: value, Enabled: h.Enabled, Redacted: redacted})
	}
	return out
}

// formLike is a=b&c=d text, as an urlencoded body is sent or typed with a pair per line: no name
// has spaces in it, which keeps prose out.
func formLike(s string) bool {
	lit := strings.TrimSpace(varRef.ReplaceAllString(s, "x"))
	if !strings.Contains(lit, "=") || strings.ContainsAny(lit[:1], `{["<`) {
		return false
	}
	for _, part := range strings.FieldsFunc(lit, func(r rune) bool { return r == '&' || r == '\n' }) {
		name, _, _ := strings.Cut(part, "=")
		if strings.ContainsAny(strings.TrimSpace(name), " \t\r") {
			return false
		}
	}
	return true
}

// graphQLSecret is bodySecret for GraphQL arguments and variable defaults.
func (p *pass) graphQLSecret(query string) bool {
	found := false
	for _, a := range graphQLArgs(query) {
		sensitive, nameRefs := jsonKeySensitive(a.key, p.public)
		if !sensitive {
			continue
		}
		p.addRefs(a.value)
		if nameRefs {
			p.addRefs(a.key)
		}
		found = found || isLiteralSecret(a.value)
	}
	return found
}

// subprotocolRefs hides what fills a token's place in a subprotocol list: bearer.<token>, the entry
// after access_token, or a whole entry whose value has that shape. A variable may hold several entries.
func (p *pass) subprotocolRefs(entries []string) {
	prev := ""
	for _, e := range entries {
		resolved := substitute(e, p.vars.values)
		var inner []string
		for _, sp := range subprotocolEntries(resolved) {
			inner = append(inner, resolved[sp.start:sp.end])
		}
		if m := bearerSubprotocol.FindStringSubmatch(e); m != nil {
			p.addRefs(m[1])
		} else if carriesToken(prev) || slices.ContainsFunc(inner, bearerSubprotocol.MatchString) ||
			len(inner) > 1 && slices.ContainsFunc(inner[:len(inner)-1], carriesToken) {
			p.addRefs(e)
		}
		prev = ""
		if len(inner) > 0 {
			prev = inner[len(inner)-1]
		}
	}
}

func isCookieHeader(name string) bool {
	name = strings.ToLower(strings.TrimSpace(name))
	return name == "cookie" || name == "set-cookie"
}

// headerSensitive checks the name as written and as the page will resolve it; a name the public
// variables leave unresolved could be anything, so it counts as sensitive and so do its references.
func (p *pass) headerSensitive(key, value string) (sensitive, nameRefs bool) {
	resolved := substitute(key, p.public)
	if varRef.MatchString(resolved) {
		return true, true
	}
	if sensitiveHeaderName(resolved) || anyLiteral(key, sensitiveHeaderName) {
		return true, false
	}
	// Hidden values count too: hiding the variable that holds "Bearer" must not unmask the literal after it.
	return authScheme.MatchString(strings.TrimSpace(value)) || authScheme.MatchString(strings.TrimSpace(substitute(value, p.vars.values))), false
}

func (p *pass) paramSensitive(name string) (sensitive, nameRefs bool) {
	return nameSensitive(name, p.public, sensitiveParamName)
}

// jsonKeySensitive is paramSensitive for a JSON key; the scan calls it too, with the same public values.
func jsonKeySensitive(key string, public map[string]string) (sensitive, nameRefs bool) {
	return nameSensitive(key, public, sensitiveJSONKey)
}

// secretContainer is a key whose whole object or list is secret, {"password": {"old": …, "new": …}}:
// what sits inside is judged by it rather than by its own keys.
func secretContainer(key string, public map[string]string) bool {
	sensitive, _ := nameSensitive(key, public, func(k string) bool {
		return sensitiveJSONKey(k) && secrets.NamesSecretContainer(k)
	})
	return sensitive
}

// memberKey is the key the name rules judge member k of an object under key by.
func memberKey(key, k string, keyed bool, public map[string]string) string {
	if keyed && secretContainer(key, public) {
		return key
	}
	return k
}

// nameSensitive applies rule to a name as written and as the page will resolve it; a name the public
// variables leave unresolved could be anything, so it counts as sensitive and so do its references.
func nameSensitive(name string, public map[string]string, rule func(string) bool) (sensitive, nameRefs bool) {
	resolved := substitute(name, public)
	if varRef.MatchString(resolved) {
		return true, true
	}
	return rule(resolved) || anyLiteral(name, rule), false
}

// The name rules of a public page add the spec's words inside tokens to the shared ones, and let
// through the few names that only look sensitive.
func sensitiveHeaderName(name string) bool {
	if secrets.IsOrdinaryName(name) {
		return false
	}
	return secrets.IsSensitiveHeader(name) || !secrets.IsCORSHeader(name) && secrets.ContainsSensitiveWord(name)
}

func sensitiveParamName(name string) bool {
	return !secrets.IsOrdinaryName(name) && (secrets.IsSensitiveQueryParam(name) || secrets.ContainsSensitiveWord(name))
}

// A bare "code" in a body or an example is an error code far more often than an OAuth one, and a
// bare "key" an issue key or the name half of a key/value entry, which lexedPairs reads as a pair.
func sensitiveJSONKey(key string) bool {
	k := strings.TrimSpace(key)
	return !strings.EqualFold(k, "code") && !strings.EqualFold(k, "key") && sensitiveParamName(key)
}

func anyLiteral(name string, sensitive func(string) bool) bool {
	if !strings.Contains(name, "{{") {
		return false
	}
	for _, lit := range varRef.Split(name, -1) {
		if strings.TrimSpace(lit) != "" && sensitive(lit) {
			return true
		}
	}
	return false
}

// url drops userinfo unless it is only references and keeps only the references of a sensitive
// query or fragment parameter. urlKey and queryPrefix keep selectors apart when one owner has several URLs.
func (p *pass) url(owner, path, raw, urlKey, queryPrefix string, hostFirst bool) string {
	out := raw
	if start, end, ok := hostSpan(raw, hostFirst); ok {
		for _, name := range refNames(raw[start:end]) {
			p.hostRefs[name] = true
		}
		if at := strings.LastIndexByte(raw[start:end], '@'); at >= 0 {
			info := raw[start : start+at]
			p.addRefs(info)
			if !onlyRefs(strings.ReplaceAll(info, ":", "")) {
				out = raw[:start] + raw[start+at+1:]
				p.redact(Redaction{Selector: selector(owner, CategoryURL, urlKey), Path: path, Category: CategoryURL, Reason: reasonUserinfo})
			}
		}
	}

	base, fragment, hasFragment := strings.Cut(out, "#")
	base, query, hasQuery := strings.Cut(base, "?")
	n := 0
	query = p.params(owner, path, query, queryPrefix, CategoryQuery, &n)
	fragment = p.params(owner, path, fragment, queryPrefix, CategoryQuery, &n)
	if hasQuery {
		base += "?" + query
	}
	if hasFragment {
		base += "#" + fragment
	}
	return base
}

// hostSpan is the authority of raw: after "://", or from the start of a scheme-less raw when
// hostFirst is set, as in a request URL without a scheme or a variable used as a host.
func hostSpan(raw string, hostFirst bool) (start, end int, ok bool) {
	if i := strings.Index(raw, "://"); i >= 0 {
		start = i + 3
	} else if !hostFirst {
		return 0, 0, false
	}
	end = len(raw)
	if i := strings.IndexAny(raw[start:], "/?#"); i >= 0 {
		end = start + i
	}
	return start, end, true
}

// hostVars extends hostRefs through variable values: baseUrl = "https://{{host}}" adds host.
func (p *pass) hostVars() {
	for grew := true; grew; {
		grew = false
		for _, row := range p.vars.rows {
			start, end, ok := hostSpan(row.v.Value, p.hostRefs[row.v.Key])
			if !ok {
				continue
			}
			for _, name := range refNames(row.v.Value[start:end]) {
				if !p.hostRefs[name] {
					p.hostRefs[name], grew = true, true
				}
			}
		}
	}
}

// params masks the values of sensitive a=b parameters of a URL query or fragment, or of a
// form-like raw body with CategoryForm.
func (p *pass) params(owner, path, params, queryPrefix, category string, n *int) string {
	if params == "" {
		return params
	}
	reason := reasonQuery
	if category == CategoryForm {
		reason = reasonForm
	}
	// A form-like raw body may put a pair on each line, and a redirect_uri may carry its own query.
	var b strings.Builder
	for rest := params; ; {
		i := strings.IndexAny(rest, "&\n?")
		part := rest
		if i >= 0 {
			part = rest[:i]
		}
		part, cr := strings.CutSuffix(part, "\r")
		b.WriteString(p.param(owner, path, part, queryPrefix+indexKey(*n), category, reason))
		*n++
		if cr {
			b.WriteByte('\r')
		}
		if i < 0 {
			return b.String()
		}
		b.WriteByte(rest[i])
		rest = rest[i+1:]
	}
}

func (p *pass) param(owner, path, part, key, category, reason string) string {
	// A variable here may insert whole parameters: "?{{params}}" with params = "password=…".
	p.fragmentRefs(part)
	name, value, ok := strings.Cut(part, "=")
	if !ok || value == "" {
		return part
	}
	sensitive, nameRefs := p.paramSensitive(name)
	if !sensitive {
		return part
	}
	p.addRefs(value)
	if nameRefs {
		p.addRefs(name)
	}
	masked := secrets.RedactValue(value)
	if masked == value {
		return part
	}
	p.redact(Redaction{Selector: selector(owner, category, key), Path: joinPath(path, name), Category: category, Reason: reason})
	return name + "=" + masked
}

func (p *pass) form(owner, path, raw string) ([]FormField, error) {
	if raw == "" || raw == "[]" {
		return nil, nil
	}
	// The stored document is [{key, value, type, enabled}]; field names match case-insensitively.
	var stored []struct {
		Key, Value, Type string
		Enabled          bool
	}
	if err := json.Unmarshal([]byte(raw), &stored); err != nil {
		return nil, fmt.Errorf("form body: %w", err)
	}
	var out []FormField
	for i, f := range stored {
		fieldPath := joinPath(path, "body", f.Key)
		if f.Type == "file" {
			out = append(out, FormField{Key: f.Key, Value: p.fileName(owner, fieldPath, f.Value, indexKey(i)), Type: "file", Enabled: f.Enabled})
			continue
		}
		value := f.Value
		p.bodyRefs(f.Value)
		if sensitive, nameRefs := p.paramSensitive(f.Key); sensitive {
			p.addRefs(f.Value)
			if nameRefs {
				p.addRefs(f.Key)
			}
			if value = secrets.RedactValue(f.Value); value != f.Value {
				p.redact(Redaction{Selector: selector(owner, CategoryForm, indexKey(i)), Path: fieldPath, Category: CategoryForm, Reason: reasonForm})
			}
		}
		out = append(out, FormField{Key: f.Key, Value: value, Type: "text", Enabled: f.Enabled})
	}
	return out, nil
}

// fileName also hides the variables of the stored path: a published value would show the full path.
func (p *pass) fileName(owner, path, stored, key string) string {
	p.addRefs(stored)
	name := baseName(stored)
	if name != stored {
		p.redact(Redaction{Selector: selector(owner, CategoryFile, key), Path: path, Category: CategoryFile, Reason: reasonFile})
	}
	return name
}

// baseName splits on both separators outside {{…}}: a collection synced from Windows carries
// backslash paths onto macOS and Linux. Same as har.baseName, which this package may not import.
func baseName(path string) string {
	spans := varRef.FindAllStringIndex(path, -1)
	path = strings.TrimRight(path, `/\`)
	for i := len(path) - 1; i >= 0; i-- {
		if (path[i] == '/' || path[i] == '\\') && outside(i, spans) {
			return path[i+1:]
		}
	}
	return path
}

func outside(i int, spans [][]int) bool {
	for _, sp := range spans {
		if i >= sp[0] && i < sp[1] {
			return false
		}
	}
	return true
}

// auth publishes allowlisted fields as stored; a secret field keeps a value made only of references
// and is emptied otherwise, since a literal there belongs in a secret variable.
func (p *pass) auth(owner, path string, t entities.AuthType, raw string, isRequest bool) (*Auth, error) {
	switch t {
	case entities.AuthTypeNone, entities.AuthTypeInherit, "":
		if !isRequest {
			return nil, nil
		}
		if t == "" {
			t = entities.AuthTypeNone
		}
		return &Auth{Type: string(t), Fields: map[string]any{}}, nil
	}
	fields, err := auth.ParseFields(raw)
	if err != nil {
		return nil, err
	}
	authPath := joinPath(path, "auth")
	known := t.IsValid()
	if !known {
		p.fail(authPath, "auth type %q cannot be published", t)
	}
	out := &Auth{Type: string(t), Fields: make(map[string]any, len(fields))}
	for _, key := range slices.Sorted(maps.Keys(fields)) {
		val := fields[key]
		fieldPath := joinPath(authPath, key)
		if known && isPublicAuthField(t, key) {
			out.Fields[key] = p.publicAuthValue(owner, fieldPath, key, val)
			continue
		}
		p.addRefs(stringLeaves(val)...)
		if s, ok := val.(string); val == nil || ok && onlyRefs(s) {
			out.Fields[key] = val
			continue
		}
		out.Fields[key] = ""
		out.Redacted = append(out.Redacted, key)
		p.redact(Redaction{Selector: selector(owner, CategoryAuth, key), Path: fieldPath, Category: CategoryAuth, Reason: reasonAuth})
	}
	return out, nil
}

func (p *pass) publicAuthValue(owner, path, key string, v any) any {
	if s, ok := v.(string); ok && authURLFields[key] {
		return p.url(owner, path, s, key, key+".", true)
	}
	if authObjectFields[key] {
		return p.authObject(owner, path, key, v, "", false)
	}
	return v
}

// authObject applies the URL rule to URL-looking strings inside JWT claims and header, and marks the
// references under a secret-looking key; the scan masks the literals there.
func (p *pass) authObject(owner, path, selKey string, v any, jsonKey string, keyed bool) any {
	switch t := v.(type) {
	case string:
		if keyed {
			if sensitive, nameRefs := jsonKeySensitive(jsonKey, p.public); sensitive {
				p.addRefs(t)
				if nameRefs {
					p.addRefs(jsonKey)
				}
			}
			p.bodyRefs(t)
		}
		if strings.Contains(t, "://") {
			return p.url(owner, path, t, selKey, selKey+".", true)
		}
		return t
	case map[string]any:
		out := make(map[string]any, len(t))
		for _, k := range slices.Sorted(maps.Keys(t)) {
			out[k] = p.authObject(owner, joinPath(path, k), selKey+"."+k, t[k], memberKey(jsonKey, k, keyed, p.public), true)
		}
		return out
	case []any:
		out := make([]any, len(t))
		for i := range t {
			out[i] = p.authObject(owner, path, selKey+"."+strconv.Itoa(i), t[i], jsonKey, keyed)
		}
		return out
	}
	return v
}

func stringLeaves(v any) []string {
	switch t := v.(type) {
	case string:
		return []string{t}
	case map[string]any:
		var out []string
		for _, sub := range t {
			out = append(out, stringLeaves(sub)...)
		}
		return out
	case []any:
		var out []string
		for _, sub := range t {
			out = append(out, stringLeaves(sub)...)
		}
		return out
	}
	return nil
}

// bodyRefs marks the references under sensitive names in body-like text; the scan masks the literals.
func (p *pass) bodyRefs(text string) {
	p.bodySecret(text)
	p.fragmentRefs(text)
}

// bodySecret marks the references in the values secretPairs finds and reports a literal there.
func (p *pass) bodySecret(text string) bool {
	found := false
	for _, sp := range secretPairs(text, p.public) {
		p.addRefs(sp.value)
		if sp.nameRefs {
			p.addRefs(sp.key)
		}
		found = found || isLiteralSecret(sp.value)
	}
	return found
}

// formSecret is bodySecret for any a=b&c=d text. Splitting at '?' too reads a query after a URL or a
// path, and the pairs after a redirect_uri that carries its own query.
func (p *pass) formSecret(text string) bool {
	found := false
	for _, kv := range formPairs(text, "&#\n?") {
		sensitive, nameRefs := p.paramSensitive(kv.key)
		if !sensitive {
			continue
		}
		p.addRefs(kv.value)
		if nameRefs {
			p.addRefs(kv.key)
		}
		found = found || isLiteralSecret(kv.value)
	}
	return found
}

// formPairs are the name=value parts of text split at any byte of seps; a name with spaces is prose
// and is skipped. start:end is the value, less a trailing '\r'.
func formPairs(text, seps string) []jsonPair {
	var out []jsonPair
	start := 0
	for i := 0; i <= len(text); i++ {
		if i < len(text) && strings.IndexByte(seps, text[i]) < 0 {
			continue
		}
		part := text[start:i]
		if eq := strings.IndexByte(part, '='); eq >= 0 {
			name := strings.TrimSpace(part[:eq])
			vs, ve := start+eq+1, start+len(strings.TrimSuffix(part, "\r"))
			if name != "" && !strings.ContainsAny(name, " \t") && ve > vs {
				out = append(out, jsonPair{key: name, value: text[vs:ve], start: vs, end: ve})
			}
		}
		start = i + 1
	}
	return out
}

// fragmentRefs hides a variable whose value, put in place of its reference, carries a secret by the
// JSON, form or GraphQL argument rule: a body of {{payload}}, an object value, a query tail, a whole
// query. References inside such a value are followed, as the page substitutes them too.
func (p *pass) fragmentRefs(text string) {
	for _, name := range refNames(text) {
		if p.fragments[name] {
			continue
		}
		p.fragments[name] = true
		for _, i := range p.vars.byKey[name] {
			value := p.vars.rows[i].v.Value
			inBody := p.bodySecret(value)
			inForm := p.formSecret(value)
			if inGraphQL := p.graphQLSecret(value); inBody || inForm || inGraphQL {
				p.refs[name] = true
			}
			p.fragmentRefs(value)
		}
	}
}

func isLiteralSecret(v string) bool {
	return v != "" && !onlyRefs(v) && v != redactedMark
}

// effectiveMetadata merges collection, folder and request metadata; the nearest level wins per
// lowercased key and the result is sorted by key, so map order never shows.
func effectiveMetadata(chain []*entities.Collection, request map[string][]string) []entities.HeaderItem {
	merged := map[string][]entities.HeaderItem{}
	apply := func(level map[string][]entities.HeaderItem) {
		for k, rows := range level {
			merged[k] = rows
		}
	}
	for _, c := range chain {
		level := map[string][]entities.HeaderItem{}
		for _, h := range c.GRPCMetadata {
			if k := strings.ToLower(strings.TrimSpace(h.Key)); h.Enabled && k != "" {
				level[k] = append(level[k], h)
			}
		}
		apply(level)
	}
	keys := slices.SortedFunc(maps.Keys(request), func(a, b string) int {
		if c := strings.Compare(strings.ToLower(a), strings.ToLower(b)); c != 0 {
			return c
		}
		return strings.Compare(a, b)
	})
	level := map[string][]entities.HeaderItem{}
	for _, k := range keys {
		lk := strings.ToLower(strings.TrimSpace(k))
		for _, v := range request[k] {
			level[lk] = append(level[lk], entities.HeaderItem{Key: k, Value: v, Enabled: true})
		}
	}
	apply(level)

	var out []entities.HeaderItem
	for _, k := range slices.Sorted(maps.Keys(merged)) {
		out = append(out, merged[k]...)
	}
	return out
}
