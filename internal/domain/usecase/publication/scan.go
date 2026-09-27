package publication

import (
	"cmp"
	"encoding/json"
	"maps"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/tetiva-app/client/internal/domain/secrets"
)

const (
	ruleJSONSecretKey    = "json-secret-key"
	ruleGraphQLSecretArg = "graphql-secret-argument"
	ruleBearerToken      = "bearer-token"
	ruleXMLSecretField   = "xml-secret-field"
	ruleFormSecretField  = "form-secret-field"
	ruleURLSecretParam   = "url-secret-parameter"
)

const graphQLString = `"""[\s\S]*?"""|"(?:[^"\\]|\\.)*"|\{\{[^}]+\}\}`

const graphQLValue = `(\[\s*(?:(?:` + graphQLString + `)\s*,?\s*)*\]|` + graphQLString + `)`

var (
	graphQLArg        = regexp.MustCompile(`([_A-Za-z][_0-9A-Za-z]*)\s*:\s*` + graphQLValue)
	graphQLDefault    = regexp.MustCompile(`\$([_A-Za-z][_0-9A-Za-z]*)\s*:[\s\[\]!_0-9A-Za-z]*=\s*` + graphQLValue)
	graphQLItem       = regexp.MustCompile(graphQLString)
	bearerSubprotocol = regexp.MustCompile(`(?i)\b(?:bearer(?:\.authorization\.k8s\.io)?[.,_ \t-]+|access_token\.)(.+)$`)
	quotedPair        = regexp.MustCompile(`"((?:[^"\\]|\\.)*)"\s*:\s*"((?:[^"\\]|\\.)*)"`)
	xmlTag            = regexp.MustCompile(`<[^<>]*>`)
	xmlOpen           = regexp.MustCompile(`<(?:[\w.-]+:)?([\w.-]+)(?:\s[^<>]*)?>`)
	xmlAttr           = regexp.MustCompile(`\s(?:[\w.-]+:)?([\w.-]+)\s*=\s*(?:"([^"]*)"|'([^']*)')`)
)

type fieldKind int

const (
	kindText fieldKind = iota
	kindBody
	kindURL
	kindName
	kindKey
	kindGraphQL
	kindSubprotocol
)

type field struct {
	owner    string
	pointer  string
	path     string
	kind     fieldKind
	value    string
	set      func(string)
	jsonKey  string
	keyed    bool
	maxBytes int
	maxRunes int
}

type span struct{ start, end int }

func scan(s *Snapshot, owners envOwners, overrides map[string]bool, public map[string]string) ([]Warning, []string) {
	var (
		warnings []Warning
		accepted []string
	)
	walkSnapshot(s, owners, public, func(f field) {
		if f.kind == kindName || f.kind == kindKey {
			return
		}
		var mask []span
		seen := map[string]bool{}
		for _, hit := range findSecrets(f, public) {
			match := f.value[hit.start:hit.end]
			sel := selector(f.owner, CategoryScan, scanKey(f.pointer, match))
			overridden := overrides[sel]
			if !seen[sel] {
				seen[sel] = true
				warnings = append(warnings, Warning{Selector: sel, Path: f.path, Rule: hit.rule, Excerpt: excerpt(match), Overridden: overridden})
				if overridden {
					accepted = append(accepted, sel)
				}
			}
			if !overridden {
				mask = append(mask, span{hit.start, hit.end})
			}
		}
		if len(mask) > 0 {
			f.set(applyMask(f.value, mask))
		}
	})
	return warnings, accepted
}

type hit struct {
	span
	rule string
}

func findSecrets(f field, public map[string]string) []hit {
	text := f.value
	var hits []hit
	for _, s := range secrets.Scan(text) {
		hits = append(hits, hit{span{s.Start, s.End}, s.Rule})
	}
	literal := func(sp span, rule string) {
		if isLiteralSecret(text[sp.start:sp.end]) {
			hits = append(hits, hit{sp, rule})
		}
	}
	keySensitive := func(key string) bool {
		sensitive, _ := jsonKeySensitive(key, public)
		return sensitive
	}
	if f.keyed && keySensitive(f.jsonKey) {
		literal(span{0, len(text)}, ruleJSONSecretKey)
	}
	switch f.kind {
	case kindBody:
		// The decoded value decides: a JSON string holding only a reference is not a literal.
		for _, sp := range secretPairs(text, public) {
			if isLiteralSecret(sp.value) {
				hits = append(hits, hit{span{sp.start, sp.end}, sp.rule})
			}
		}
	case kindGraphQL:
		for _, a := range graphQLArgs(text) {
			if keySensitive(a.key) {
				literal(span{a.start, a.end}, ruleGraphQLSecretArg)
			}
		}
	case kindSubprotocol:
		prev := f.jsonKey
		for _, e := range subprotocolEntries(text) {
			if m := bearerSubprotocol.FindStringSubmatchIndex(text[e.start:e.end]); m != nil {
				literal(span{e.start + m[2], e.start + m[3]}, ruleBearerToken)
			} else if carriesToken(prev) {
				literal(e, ruleBearerToken)
			}
			prev = text[e.start:e.end]
		}
	}
	slices.SortStableFunc(hits, func(a, b hit) int { return a.start - b.start })
	return slices.CompactFunc(hits, func(a, b hit) bool { return a.span == b.span })
}

func carriesToken(entry string) bool {
	entry = strings.TrimSpace(entry)
	return strings.EqualFold(entry, "access_token") || strings.EqualFold(entry, "bearer")
}

func isSubprotocolHeader(name string) bool {
	return strings.EqualFold(strings.TrimSpace(name), "sec-websocket-protocol")
}

func subprotocolEntries(text string) []span {
	var out []span
	start := 0
	for i := 0; i <= len(text); i++ {
		if i < len(text) && text[i] != ',' {
			continue
		}
		s, e := start, i
		for s < e && (text[s] == ' ' || text[s] == '\t') {
			s++
		}
		for e > s && (text[e-1] == ' ' || text[e-1] == '\t') {
			e--
		}
		if e > s {
			out = append(out, span{s, e})
		}
		start = i + 1
	}
	return out
}

func applyMask(text string, spans []span) string {
	slices.SortFunc(spans, func(a, b span) int { return a.start - b.start })
	var b strings.Builder
	last := 0
	for i := 0; i < len(spans); {
		start, end := spans[i].start, spans[i].end
		for i++; i < len(spans) && spans[i].start < end; i++ {
			end = max(end, spans[i].end)
		}
		b.WriteString(text[last:start])
		b.WriteString(maskLiterals(text[start:end]))
		last = end
	}
	b.WriteString(text[last:])
	return b.String()
}

func maskLiterals(s string) string {
	var b strings.Builder
	last := 0
	for _, ref := range varRef.FindAllStringIndex(s, -1) {
		if ref[0] > last {
			b.WriteString(redactedMark)
		}
		b.WriteString(s[ref[0]:ref[1]])
		last = ref[1]
	}
	if last < len(s) {
		b.WriteString(redactedMark)
	}
	return b.String()
}

func excerpt(match string) string {
	r := []rune(match)
	return string(r[:min(len(r), 4)]) + "…"
}

// For JSON stored in a string, start:end spans that whole string and value is decoded.
type jsonPair struct {
	key, value string
	start, end int
}

func jsonPairs(text string, public map[string]string) []jsonPair {
	if !strings.Contains(text, ":") {
		return nil
	}
	out := lexedPairs(text, public)
	seen := make(map[jsonPair]bool, len(out))
	for _, p := range out {
		seen[p] = true
	}
	add := func(p jsonPair) {
		if !seen[p] {
			seen[p] = true
			out = append(out, p)
		}
	}
	// JSON inside HTML or an HTTP dump can throw the lexer off; the regex still sees its pairs.
	for _, m := range quotedPair.FindAllStringSubmatchIndex(text, -1) {
		add(jsonPair{key: jsonKeyText(text, jsonToken{'"', m[2] - 1, m[3] + 1}), value: text[m[4]:m[5]], start: m[4], end: m[5]})
	}
	for _, p := range slices.Clone(out) {
		for _, inner := range storedPairs(text, p, public) {
			add(inner)
		}
	}
	return out
}

type secretPair struct {
	jsonPair
	rule     string
	nameRefs bool
}

func secretPairs(text string, public map[string]string) []secretPair {
	var out []secretPair
	add := func(kv jsonPair, rule string, names func(string) bool) {
		if sensitive, nameRefs := nameSensitive(kv.key, public, names); sensitive {
			out = append(out, secretPair{kv, rule, nameRefs})
		}
	}
	pairs, xml := jsonPairs(text, public), xmlPairs(text)
	for _, kv := range pairs {
		add(kv, ruleJSONSecretKey, sensitiveJSONKey)
	}
	for _, kv := range xml {
		add(kv, ruleXMLSecretField, sensitiveJSONKey)
	}
	whole := jsonPair{value: text, end: len(text)}
	for _, kv := range slices.Concat(pairs, xml, []jsonPair{whole}, embeddedURLs(text)) {
		for _, q := range urlParams(text, kv) {
			add(q, ruleURLSecretParam, sensitiveParamName)
		}
	}
	if urlQuery(text) < 0 && formLike(text) {
		for _, kv := range formPairs(text, "&\n?") {
			add(kv, ruleFormSecretField, sensitiveParamName)
		}
	}
	return out
}

func embeddedURLs(text string) []jsonPair {
	const stops = " \t\r\n\"'<>`"
	var out []jsonPair
	scheme, templated := -2, -2
	for from := 0; from < len(text); {
		if scheme != -1 && scheme < from {
			scheme = nextIndex(text, "://", from)
		}
		if templated != -1 && templated < from {
			templated = nextIndex(text, "}}/", from)
		}
		mark := scheme
		if mark < 0 || templated >= 0 && templated < mark {
			mark = templated
		}
		if mark < 0 {
			break
		}
		start, end := mark, len(text)
		for start > from && strings.IndexByte(stops, text[start-1]) < 0 {
			start--
		}
		if e := strings.IndexAny(text[mark:], stops); e >= 0 {
			end = mark + e
		}
		if urlQuery(text[start:end]) >= 0 {
			out = append(out, jsonPair{value: text[start:end], start: start, end: end})
		}
		from = end + 1
	}
	return out
}

func nextIndex(text, sub string, from int) int {
	if i := strings.Index(text[from:], sub); i >= 0 {
		return from + i
	}
	return -1
}

func urlQuery(v string) int {
	i := strings.IndexAny(v, "?#")
	if i < 0 {
		return -1
	}
	head := varRef.ReplaceAllString(v[:i], "x")
	if strings.ContainsAny(head, " \t\r\n\"'<>{}") {
		return -1
	}
	if !strings.Contains(head, "://") && !strings.HasPrefix(head, "/") && !strings.HasPrefix(v, "{{") {
		return -1
	}
	return i + 1
}

func urlParams(text string, kv jsonPair) []jsonPair {
	q := urlQuery(kv.value)
	if q < 0 {
		return nil
	}
	params := kv.value[q:]
	if end := strings.IndexAny(params, " \t\r\n\"'<>"); end >= 0 {
		params = params[:end]
	}
	out := formPairs(params, "&#?")
	direct := text[kv.start:kv.end] == kv.value
	for i := range out {
		if direct {
			out[i].start, out[i].end = kv.start+q+out[i].start, kv.start+q+out[i].end
		} else {
			out[i].start, out[i].end = kv.start, kv.end
		}
	}
	return out
}

func storedPairs(text string, p jsonPair, public map[string]string) []jsonPair {
	if p.start == 0 || p.end >= len(text) || text[p.start-1] != '"' || text[p.end] != '"' || !strings.Contains(p.value, `\"`) {
		return nil
	}
	if lead := strings.TrimLeft(p.value, " \t\r\n"); lead == "" || lead[0] != '{' && lead[0] != '[' {
		return nil
	}
	var decoded string
	if json.Unmarshal([]byte(text[p.start-1:p.end+1]), &decoded) != nil {
		return nil
	}
	inner := jsonPairs(decoded, public)
	for i := range inner {
		inner[i].start, inner[i].end = p.start, p.end
	}
	return inner
}

func lexedPairs(text string, public map[string]string) []jsonPair {
	type frame struct {
		array      bool
		key        string
		keyed      bool
		secret     string
		label, val *jsonPair
	}
	var (
		toks  = lexJSON(text)
		stack []frame
		out   []jsonPair
		key   string
		keyed bool
	)
	for i := 0; i < len(toks); i++ {
		t := toks[i]
		value := t.kind == '"' || t.kind == 'r'
		secret := ""
		if n := len(stack); n > 0 {
			secret = stack[n-1].secret
		}
		if (t.kind == '[' || t.kind == '{') && secret == "" && keyed && secretContainer(key, public) {
			secret = key
		}
		switch {
		case value && i+1 < len(toks) && toks[i+1].kind == ':':
			key, keyed = jsonKeyText(text, t), true
			i++
			continue
		case value && keyed:
			p := t.pair(text, cmp.Or(secret, key))
			out = append(out, p)
			if n := len(stack); n > 0 {
				if strings.EqualFold(key, "value") {
					stack[n-1].val = &p
				} else if strings.EqualFold(key, "key") || strings.EqualFold(key, "name") {
					stack[n-1].label = &p
				}
			}
		case value && len(stack) > 0 && stack[len(stack)-1].array && stack[len(stack)-1].keyed:
			out = append(out, t.pair(text, cmp.Or(secret, stack[len(stack)-1].key)))
		case t.kind == '[':
			f := frame{array: true, key: key, keyed: keyed, secret: secret}
			if n := len(stack); !keyed && n > 0 && stack[n-1].array {
				f.key, f.keyed = stack[n-1].key, stack[n-1].keyed
			}
			stack = append(stack, f)
		case t.kind == '{':
			stack = append(stack, frame{secret: secret})
		case t.kind == '}' || t.kind == ']':
			if n := len(stack); n > 0 {
				if f := stack[n-1]; f.label != nil && f.val != nil {
					out = append(out, jsonPair{key: f.label.value, value: f.val.value, start: f.val.start, end: f.val.end})
				}
				stack = stack[:n-1]
			}
		}
		keyed = false
	}
	return out
}

// xmlPairs skips a bare <Key>: far more often an S3 or plist key than a secret.
func xmlPairs(text string) []jsonPair {
	if !strings.Contains(text, "<") {
		return nil
	}
	out := xmlTexts(text)
	for _, tag := range xmlTag.FindAllStringIndex(text, -1) {
		for _, m := range xmlAttr.FindAllStringSubmatchIndex(text[tag[0]:tag[1]], -1) {
			g := 4
			if m[g] < 0 {
				g = 6
			}
			start, end := tag[0]+m[g], tag[0]+m[g+1]
			out = append(out, jsonPair{key: text[tag[0]+m[2] : tag[0]+m[3]], value: text[start:end], start: start, end: end})
		}
	}
	return out
}

// xmlTexts needs the closing tag right after, or an "<api_key>" placeholder eats text.
func xmlTexts(text string) []jsonPair {
	var out []jsonPair
	// cdataEnd caches the first "]]></" past the last lookup: unclosed sections stay linear.
	cdataEnd, pos := -1, 0
	for _, m := range xmlOpen.FindAllStringSubmatchIndex(text, -1) {
		if m[0] < pos {
			continue
		}
		start, end := m[1], -1
		if rest := text[m[1]:]; strings.HasPrefix(rest, "<![CDATA[") {
			start += len("<![CDATA[")
			if cdataEnd < start {
				cdataEnd = len(text)
				if i := strings.Index(text[start:], "]]></"); i >= 0 {
					cdataEnd = start + i
				}
			}
			if cdataEnd < len(text) {
				end, pos = cdataEnd, cdataEnd+len("]]></")
			}
		} else if i := strings.IndexByte(rest, '<'); i >= 0 && strings.HasPrefix(rest[i:], "</") {
			end, pos = m[1]+i, m[1]+i+len("</")
		}
		if name := text[m[2]:m[3]]; end >= 0 && !strings.EqualFold(name, "key") {
			out = append(out, jsonPair{key: name, value: text[start:end], start: start, end: end})
		}
	}
	return out
}

func graphQLArgs(text string) []jsonPair {
	if !strings.Contains(text, ":") {
		return nil
	}
	var out []jsonPair
	for _, re := range []*regexp.Regexp{graphQLArg, graphQLDefault} {
		for _, m := range re.FindAllStringSubmatchIndex(text, -1) {
			items := [][]int{{m[4], m[5]}}
			if text[m[4]] == '[' {
				items = graphQLItem.FindAllStringIndex(text[m[4]:m[5]], -1)
			}
			for _, it := range items {
				start, end := it[0], it[1]
				if text[m[4]] == '[' {
					start, end = m[4]+start, m[4]+end
				}
				quotes := 0
				if strings.HasPrefix(text[start:end], `"""`) {
					quotes = 3
				} else if text[start] == '"' {
					quotes = 1
				}
				start, end = start+quotes, end-quotes
				out = append(out, jsonPair{key: text[m[2]:m[3]], value: text[start:end], start: start, end: end})
			}
		}
	}
	return out
}

// jsonToken.kind: '"' string, 'r' {{reference}}, 'x' other run, else the punctuation.
type jsonToken struct {
	kind       byte
	start, end int
}

func (t jsonToken) pair(text, key string) jsonPair {
	if t.kind == 'r' {
		return jsonPair{key: key, value: text[t.start:t.end], start: t.start, end: t.end}
	}
	end := t.end
	if end-t.start >= 2 && text[end-1] == '"' {
		end--
	}
	return jsonPair{key: key, value: text[t.start+1 : end], start: t.start + 1, end: end}
}

func jsonKeyText(text string, t jsonToken) string {
	if t.kind == 'r' {
		return text[t.start:t.end]
	}
	raw := text[t.start:t.end]
	var decoded string
	if strings.Contains(raw, `\`) && json.Unmarshal([]byte(raw), &decoded) == nil {
		return decoded
	}
	return strings.Trim(raw, `"`)
}

func lexJSON(text string) []jsonToken {
	var toks []jsonToken
	// nextClose caches the first '}' at or after the scan position: runs of "{{" stay linear.
	nextClose := -1
	lastBlockEnd := strings.LastIndex(text, "*/")
	for i := 0; i < len(text); {
		c := text[i]
		if c == '/' {
			if end := commentEnd(text, i, lastBlockEnd); end > i {
				i = end
				continue
			}
		}
		switch {
		case c == '"':
			j := i + 1
			for j < len(text) && text[j] != '"' {
				if text[j] == '\\' {
					j++
				}
				j++
			}
			end := min(j+1, len(text))
			toks = append(toks, jsonToken{'"', i, end})
			i = end
		case strings.HasPrefix(text[i:], "{{"):
			if nextClose < i+2 {
				nextClose = len(text)
				if j := strings.IndexByte(text[i+2:], '}'); j >= 0 {
					nextClose = i + 2 + j
				}
			}
			// Same match as varRef: the name runs to the first '}', which must be doubled.
			if nextClose > i+2 && nextClose+1 < len(text) && text[nextClose+1] == '}' {
				toks = append(toks, jsonToken{'r', i, nextClose + 2})
				i = nextClose + 2
				continue
			}
			toks = append(toks, jsonToken{c, i, i + 1})
			i++
		case strings.IndexByte("{}[]:,", c) >= 0:
			toks = append(toks, jsonToken{c, i, i + 1})
			i++
		case c == ' ' || c == '\t' || c == '\n' || c == '\r':
			i++
		default:
			j := i + 1
			for j < len(text) && strings.IndexByte("{}[]:,\" \t\n\r/", text[j]) < 0 {
				j++
			}
			toks = append(toks, jsonToken{'x', i, j})
			i = j
		}
	}
	return toks
}

// Comments open only where a token could: "https://", "*/*" and unclosed "/*" are text.
func commentEnd(text string, i, lastBlockEnd int) int {
	prev := byte(' ')
	if i > 0 {
		prev = text[i-1]
	}
	if strings.IndexByte(" \t\r\n{}[],\":", prev) < 0 {
		return i
	}
	switch {
	case prev != ':' && strings.HasPrefix(text[i:], "//"):
		if nl := strings.IndexByte(text[i:], '\n'); nl >= 0 {
			return i + nl
		}
		return len(text)
	case lastBlockEnd >= i+2 && strings.HasPrefix(text[i:], "/*"):
		return i + 2 + strings.Index(text[i+2:], "*/") + 2
	}
	return i
}

func walkSnapshot(s *Snapshot, owners envOwners, public map[string]string, visit func(field)) {
	w := walker{visit: visit, public: public}
	root := &s.Collection
	w.folder(&root.Folder, root.Name, true)
	w.headers(root.ID, "/grpcMetadata", joinPath(root.Name, "metadata"), root.GRPCMetadata)
	if env := s.Environment; env != nil {
		envPath := "Environment " + env.Name
		w.sized(owners.env, "/name", envPath, kindText, &env.Name, 0, maxNameRunes)
		for i := range env.Variables {
			v := &env.Variables[i]
			path := joinPath(envPath, v.Key)
			w.key(owners.vars[i], "/key", path, v.Key)
			w.sized(owners.vars[i], "/value", path, kindBody, &v.Value, maxVariableBytes, 0)
		}
	}
}

type walker struct {
	visit  func(field)
	public map[string]string
}

func (w walker) text(owner, pointer, path string, kind fieldKind, p *string) {
	w.sized(owner, pointer, path, kind, p, 0, 0)
}

func (w walker) sized(owner, pointer, path string, kind fieldKind, p *string, maxBytes, maxRunes int) {
	w.visit(field{
		owner: owner, pointer: pointer, path: path, kind: kind, value: *p, set: func(v string) { *p = v },
		maxBytes: maxBytes, maxRunes: maxRunes,
	})
}

func (w walker) key(owner, pointer, path, value string) {
	w.visit(field{owner: owner, pointer: pointer, path: path, kind: kindKey, value: value, maxRunes: maxNameRunes})
}

func (w walker) folder(f *Folder, path string, isRoot bool) {
	w.sized(f.ID, "/name", joinPath(path, "name"), kindText, &f.Name, 0, maxNameRunes)
	w.text(f.ID, "/description", joinPath(path, "description"), kindBody, &f.Description)
	w.auth(f.ID, joinPath(path, "auth"), f.Auth)
	w.scripts(f.ID, joinPath(path, "scripts"), f.Scripts)
	prefix := path
	if isRoot {
		prefix = ""
	}
	for _, it := range f.Items {
		if it.Folder != nil {
			w.folder(it.Folder, joinPath(prefix, it.Folder.Name), false)
		} else if it.Request != nil {
			w.request(it.Request, joinPath(prefix, it.Request.Name))
		}
	}
}

func (w walker) request(r *Request, path string) {
	id := r.ID
	w.sized(id, "/name", joinPath(path, "name"), kindText, &r.Name, 0, maxNameRunes)
	w.text(id, "/description", joinPath(path, "description"), kindBody, &r.Description)
	urlPath := joinPath(path, "url")
	switch {
	case r.HTTP != nil:
		h := r.HTTP
		w.text(id, "/http/url", urlPath, kindURL, &h.URL)
		w.headers(id, "/http/headers", joinPath(path, "headers"), h.Headers)
		bodyPath := joinPath(path, "body")
		w.text(id, "/http/body/raw", bodyPath, kindBody, &h.Body.Raw)
		w.text(id, "/http/body/fileName", bodyPath, kindText, &h.Body.FileName)
		for i := range h.Body.Fields {
			f := &h.Body.Fields[i]
			ptr := "/http/body/fields/" + strconv.Itoa(i)
			fieldPath := joinPath(bodyPath, f.Key)
			w.key(id, ptr+"/key", fieldPath, f.Key)
			w.sized(id, ptr+"/value", fieldPath, kindBody, &f.Value, maxPairValueBytes, 0)
		}
	case r.GraphQL != nil:
		g := r.GraphQL
		w.text(id, "/graphql/url", urlPath, kindURL, &g.URL)
		w.headers(id, "/graphql/headers", joinPath(path, "headers"), g.Headers)
		w.text(id, "/graphql/query", joinPath(path, "query"), kindGraphQL, &g.Query)
		w.text(id, "/graphql/variables", joinPath(path, "variables"), kindBody, &g.Variables)
		w.text(id, "/graphql/operationName", joinPath(path, "operation name"), kindText, &g.OperationName)
	case r.GRPC != nil:
		g := r.GRPC
		w.text(id, "/grpc/target", urlPath, kindURL, &g.Target)
		w.text(id, "/grpc/service", joinPath(path, "service"), kindText, &g.Service)
		w.text(id, "/grpc/method", joinPath(path, "method"), kindText, &g.Method)
		w.text(id, "/grpc/message", joinPath(path, "message"), kindBody, &g.Message)
		w.headers(id, "/grpc/metadata", joinPath(path, "metadata"), g.Metadata)
	case r.WebSocket != nil:
		ws := r.WebSocket
		w.text(id, "/websocket/url", urlPath, kindURL, &ws.URL)
		w.headers(id, "/websocket/headers", joinPath(path, "headers"), ws.Headers)
		prev := ""
		for i := range ws.Subprotocols {
			slot, entry := &ws.Subprotocols[i], ws.Subprotocols[i]
			w.visit(field{
				owner: id, pointer: "/websocket/subprotocols/" + strconv.Itoa(i), path: joinPath(path, "subprotocols"),
				kind: kindSubprotocol, value: entry, set: func(v string) { *slot = v }, jsonKey: prev,
			})
			prev = entry
		}
		for i := range ws.Messages {
			m := &ws.Messages[i]
			ptr := "/websocket/messages/" + strconv.Itoa(i)
			msgPath := joinPath(path, "messages", m.Name)
			w.text(id, ptr+"/name", msgPath, kindText, &m.Name)
			kind := kindBody
			if m.Format == "binary" {
				kind = kindText
			}
			w.text(id, ptr+"/data", msgPath, kind, &m.Data)
		}
	}
	w.auth(id, joinPath(path, "auth"), r.Auth)
	w.scripts(id, joinPath(path, "scripts"), r.Scripts)
	for i := range r.Examples {
		e := &r.Examples[i]
		exPath := joinPath(path, "examples", e.Name)
		w.sized(e.ID, "/name", joinPath(exPath, "name"), kindText, &e.Name, 0, maxNameRunes)
		w.text(e.ID, "/statusText", joinPath(exPath, "status text"), kindText, &e.StatusText)
		w.headers(e.ID, "/headers", joinPath(exPath, "headers"), e.Headers)
		w.text(e.ID, "/body", joinPath(exPath, "body"), kindBody, &e.Body)
		w.text(e.ID, "/contentType", joinPath(exPath, "content type"), kindText, &e.ContentType)
	}
}

func (w walker) headers(owner, pointer, path string, headers []Header) {
	for i := range headers {
		h := &headers[i]
		ptr := pointer + "/" + strconv.Itoa(i)
		rowPath := joinPath(path, h.Key)
		w.visit(field{owner: owner, pointer: ptr + "/key", path: rowPath, kind: kindName, value: h.Key, maxRunes: maxNameRunes})
		kind := kindBody
		if isSubprotocolHeader(h.Key) {
			kind = kindSubprotocol
		}
		w.sized(owner, ptr+"/value", rowPath, kind, &h.Value, maxPairValueBytes, 0)
	}
}

func (w walker) scripts(owner, path string, s *Scripts) {
	if s == nil {
		return
	}
	w.text(owner, "/scripts/pre", joinPath(path, "pre"), kindText, &s.Pre)
	w.text(owner, "/scripts/post", joinPath(path, "post"), kindText, &s.Post)
}

func (w walker) auth(owner, path string, a *Auth) {
	if a == nil {
		return
	}
	for _, name := range a.Redacted {
		w.key(owner, "/auth/redacted", path, name)
	}
	for _, k := range slices.Sorted(maps.Keys(a.Fields)) {
		w.key(owner, "/auth/fields", path, k)
		w.value(owner, "/auth/fields/"+pointerToken(k), joinPath(path, k), a.Fields[k], func(x any) { a.Fields[k] = x }, "", authObjectFields[k])
	}
}

func (w walker) value(owner, pointer, path string, v any, set func(any), key string, keyed bool) {
	switch t := v.(type) {
	case string:
		kind := kindText
		if keyed {
			kind = kindBody
		}
		w.visit(field{owner: owner, pointer: pointer, path: path, kind: kind, value: t, set: func(s string) { set(s) }, jsonKey: key, keyed: keyed})
	case map[string]any:
		for _, k := range slices.Sorted(maps.Keys(t)) {
			w.key(owner, pointer, path, k)
			w.value(owner, pointer+"/"+pointerToken(k), joinPath(path, k), t[k], func(x any) { t[k] = x }, memberKey(key, k, keyed, w.public), keyed)
		}
	case []any:
		for i := range t {
			w.value(owner, pointer+"/"+strconv.Itoa(i), path, t[i], func(x any) { t[i] = x }, key, keyed)
		}
	}
}
