package openapi

import (
	"bytes"
	"cmp"
	"encoding/json"
	"fmt"
	"maps"
	"net/http"
	"net/url"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/getkin/kin-openapi/openapi3"
	"gopkg.in/yaml.v3"

	"github.com/tetiva-app/client/internal/adapters/portability"
	"github.com/tetiva-app/client/internal/domain"
	"github.com/tetiva-app/client/internal/domain/entities"
	"github.com/tetiva-app/client/internal/domain/usecase/collection"
	"github.com/tetiva-app/client/internal/domain/usecase/environment"
	"github.com/tetiva-app/client/internal/domain/usecase/example"
	"github.com/tetiva-app/client/internal/domain/usecase/request"
)

const (
	untitled          = "Untitled API"
	maxExampleNameLen = 200
)

var methods = []string{
	http.MethodGet, http.MethodPut, http.MethodPost, http.MethodDelete,
	http.MethodOptions, http.MethodHead, http.MethodPatch,
}

var pathParam = regexp.MustCompile(`\{([^{}]+)\}`)

var reservedVars = map[string]bool{
	"baseUrl": true, "token": true, "username": true, "password": true, "apiKey": true, "clientId": true, "clientSecret": true,
}

type plan struct {
	root     collection.Create
	folders  []*folderPlan
	requests []*requestPlan
	envName  string
	baseURL  string
	vars     []environment.AddVariable
	urls     []string
	warnings []string

	requestCount, exampleCount int
}

type folderPlan struct {
	in       collection.Create
	requests []*requestPlan
}

type requestPlan struct {
	in       request.Create
	label    string
	examples []example.Create
}

type builder struct {
	*spec
	p           *plan
	warned      map[string]bool
	vars        map[string]int
	paramKeys   map[string]string
	global      authChoice
	authHeaders []string
	textForms   []string
}

func newPlan(s *spec) *plan {
	b := &builder{spec: s, p: &plan{}, warned: map[string]bool{}, vars: map[string]int{}, paramKeys: paramKeys(s.doc.Paths)}
	for _, w := range s.warnings {
		b.warn(w)
	}
	b.p.root = b.collection()
	b.p.envName = b.p.root.Name
	b.p.baseURL = b.baseURL()
	b.addVar("baseUrl", b.p.baseURL, false)
	b.global = b.security(s.doc.Security)
	b.p.root.AuthType, b.p.root.AuthData = b.global.typ, b.global.data
	b.operations()
	if len(b.textForms) > 0 {
		b.warn(portability.URLEncodedForms(b.textForms))
	}
	if len(b.authHeaders) > 0 {
		b.warn(fmt.Sprintf("the Authorization header parameter of %s was not imported: set the auth on the collection or the request instead",
			portability.Listing(b.authHeaders)))
	}
	if len(s.doc.Webhooks) > 0 {
		b.warn("webhooks were skipped: " + strings.Join(orderedKeys(s.doc.Webhooks, s.src.at("#/webhooks")), ", "))
	}
	return b.p
}

func paramKeys(paths *openapi3.Paths) map[string]string {
	taken := map[string]bool{}
	if paths != nil {
		for path := range paths.Map() {
			for _, m := range pathParam.FindAllStringSubmatch(path, -1) {
				taken[m[1]] = true
			}
		}
	}
	keys := make(map[string]string, len(reservedVars))
	for name := range reservedVars {
		key := name + "Param"
		for n := 2; taken[key]; n++ {
			key = fmt.Sprintf("%sParam%d", name, n)
		}
		keys[name] = key
	}
	return keys
}

func (b *builder) warn(msg string) {
	if !b.warned[msg] {
		b.warned[msg] = true
		b.p.warnings = append(b.p.warnings, msg)
	}
}

func (b *builder) addVar(key, value string, secret bool) {
	if i, ok := b.vars[key]; ok {
		if v := &b.p.vars[i]; v.Value == "" && !v.IsSecret {
			v.Value = value
		}
		return
	}
	b.vars[key] = len(b.p.vars)
	b.p.vars = append(b.p.vars, environment.AddVariable{Key: key, Value: value, IsSecret: secret})
}

func (p *plan) hostVars() map[string]string {
	if strings.Contains(p.baseURL, "://") {
		return map[string]string{"baseUrl": p.baseURL}
	}
	return nil
}

func (b *builder) collection() collection.Create {
	in := collection.Create{Name: untitled}
	info := b.doc.Info
	if info == nil {
		return in
	}
	if t := strings.TrimSpace(info.Title); t != "" {
		in.Name = t
	}
	var parts []string
	if d := strings.TrimSpace(info.Description); d != "" {
		parts = append(parts, d)
	}
	if v := strings.TrimSpace(info.Version); v != "" {
		parts = append(parts, "Version: "+v)
	}
	in.Description = b.clamp(fmt.Sprintf("collection %q", in.Name), strings.Join(parts, "\n\n"))
	return in
}

func (b *builder) clamp(label, s string) string {
	text, warning := portability.ClampDescription(label, s)
	if warning != "" {
		b.warn(warning)
	}
	return text
}

func (b *builder) baseURL() string {
	raw := b.serverURL()
	if raw == "" {
		b.warn("the document names no server: set baseUrl in the environment")
		return ""
	}
	if !strings.Contains(raw, "://") {
		b.warn(fmt.Sprintf("the server URL %q is relative: set baseUrl in the environment to the full address", raw))
	}
	return strings.TrimRight(raw, "/")
}

func (b *builder) serverURL() string {
	if len(b.doc.Servers) == 0 || b.doc.Servers[0] == nil {
		if b.src.swagger2 {
			return scalar(b.src.root, "basePath")
		}
		return ""
	}
	return resolveServer(b.doc.Servers[0])
}

func resolveServer(server *openapi3.Server) string {
	u := server.URL
	for name, v := range server.Variables {
		if v != nil {
			u = strings.ReplaceAll(u, "{"+name+"}", v.Default)
		}
	}
	return u
}

func (b *builder) ownServer(label string, item *openapi3.PathItem, op *openapi3.Operation) {
	servers := item.Servers
	if op.Servers != nil {
		servers = *op.Servers
	}
	if len(servers) == 0 || servers[0] == nil {
		return
	}
	if u := strings.TrimRight(resolveServer(servers[0]), "/"); u != b.p.baseURL {
		b.warn(fmt.Sprintf("%s: its own server %q was ignored, the request uses {{baseUrl}}", label, u))
	}
}

func (b *builder) operations() {
	folders := map[string]*folderPlan{}
	var order []string
	folder := func(name, description string) *folderPlan {
		f, ok := folders[name]
		if !ok {
			f = &folderPlan{in: collection.Create{Name: name, Description: b.clamp(fmt.Sprintf("folder %q", name), description)}}
			folders[name] = f
			order = append(order, name)
		}
		return f
	}
	for _, tag := range b.doc.Tags {
		if tag != nil && strings.TrimSpace(tag.Name) != "" {
			folder(tag.Name, strings.TrimSpace(tag.Description))
		}
	}
	if b.doc.Paths == nil {
		return
	}
	paths := b.doc.Paths.Map()
	for _, path := range orderedKeys(paths, b.src.at("#/paths")) {
		item := paths[path]
		if item == nil {
			continue
		}
		if item.Trace != nil {
			b.warn(fmt.Sprintf("TRACE %s was skipped: TRACE requests are not supported", path))
		}
		for _, method := range methods {
			op := item.GetOperation(method)
			if op == nil {
				continue
			}
			rp := b.request(path, method, item, op)
			if len(op.Tags) == 0 || strings.TrimSpace(op.Tags[0]) == "" {
				b.p.requests = append(b.p.requests, rp)
				continue
			}
			f := folder(op.Tags[0], "")
			f.requests = append(f.requests, rp)
		}
	}
	for _, name := range order {
		if f := folders[name]; len(f.requests) > 0 {
			b.p.folders = append(b.p.folders, f)
		}
	}
}

func (b *builder) request(path, method string, item *openapi3.PathItem, op *openapi3.Operation) *requestPlan {
	label := method + " " + path
	opPtr := ptr("#", "paths", path, strings.ToLower(method))
	params := mergeParams(item.Parameters, op.Parameters)

	in := request.Create{
		Name:        requestName(op, label),
		Description: b.clamp(label, description(op, params)),
		Protocol:    entities.ProtocolHTTP,
		Method:      entities.HTTPMethod(method),
		URL:         b.url(label, path, params),
		Headers:     headerParams(params),
	}
	var contentType string
	in.BodyType, in.Body, contentType = b.body(label, op.RequestBody, ptr(opPtr, "requestBody"))
	if contentType != "" && in.BodyType != entities.BodyTypeForm {
		in.Headers = append(in.Headers, entities.HeaderItem{Key: "Content-Type", Value: contentType, Enabled: true})
	}
	in.AuthType, in.AuthData = entities.AuthTypeInherit, "{}"
	if op.Security != nil {
		if c := b.security(*op.Security); c.typ == entities.AuthTypeNone || c.key != b.global.key {
			in.AuthType, in.AuthData = c.typ, c.data
		}
	}
	unauthenticated := in.AuthType == entities.AuthTypeNone || in.AuthType == entities.AuthTypeInherit && b.global.typ == entities.AuthTypeNone
	if b.src.swagger2 && unauthenticated && slices.ContainsFunc(params, isAuthorizationHeader) {
		b.authHeaders = append(b.authHeaders, label)
	}
	b.ownServer(label, item, op)
	if len(op.Callbacks) > 0 {
		b.warn(label + ": callbacks were skipped")
	}

	b.p.urls = append(b.p.urls, in.URL)
	b.p.requestCount++
	return &requestPlan{in: in, label: label, examples: b.examples(op, opPtr, label)}
}

func requestName(op *openapi3.Operation, label string) string {
	for _, s := range []string{op.Summary, op.OperationID} {
		if s = strings.TrimSpace(s); s != "" {
			return s
		}
	}
	return label
}

func mergeParams(lists ...openapi3.Parameters) []*openapi3.Parameter {
	var out []*openapi3.Parameter
	index := map[string]int{}
	for _, list := range lists {
		for _, ref := range list {
			if ref == nil || ref.Value == nil {
				continue
			}
			p := ref.Value
			key := p.In + "\x00" + p.Name
			if i, ok := index[key]; ok {
				out[i] = p
				continue
			}
			index[key] = len(out)
			out = append(out, p)
		}
	}
	return out
}

func (b *builder) url(label, path string, params []*openapi3.Parameter) string {
	values := map[string]string{}
	for _, p := range params {
		if p.In == openapi3.ParameterInPath {
			values[p.Name], _ = paramValue(p)
		}
	}
	u := "{{baseUrl}}" + pathParam.ReplaceAllStringFunc(path, func(m string) string {
		name := m[1 : len(m)-1]
		key := name
		if renamed, ok := b.paramKeys[name]; ok {
			key = renamed
		}
		b.addVar(key, values[name], false)
		return "{{" + key + "}}"
	})
	var query []string
	for _, p := range params {
		if p.In != openapi3.ParameterInQuery {
			continue
		}
		v, ok := paramExample(p)
		if !ok && !p.Required {
			continue
		}
		pairs, exact := queryPairs(p, v)
		if !exact {
			b.warn(fmt.Sprintf("%s: query parameter %q uses style %s, which was not reproduced", label, p.Name, p.Style))
		}
		query = append(query, pairs...)
	}
	if len(query) > 0 {
		u += "?" + strings.Join(query, "&")
	}
	return u
}

var delimiters = map[string]string{
	openapi3.SerializationForm:           ",",
	openapi3.SerializationSpaceDelimited: "%20",
	openapi3.SerializationPipeDelimited:  "|",
}

func queryPairs(p *openapi3.Parameter, v any) (pairs []string, exact bool) {
	style := cmp.Or(p.Style, openapi3.SerializationForm)
	explode := style == openapi3.SerializationForm
	if p.Explode != nil {
		explode = *p.Explode
	}
	name := url.QueryEscape(p.Name)
	pair := func(key string, v any) string { return key + "=" + url.QueryEscape(text(v)) }
	switch t := v.(type) {
	case nil:
		return []string{name + "="}, true
	case []any:
		sep, ok := delimiters[style]
		if explode && ok {
			for _, item := range t {
				pairs = append(pairs, pair(name, item))
			}
			return pairs, true
		}
		items := make([]string, 0, len(t))
		for _, item := range t {
			items = append(items, url.QueryEscape(text(item)))
		}
		return []string{name + "=" + strings.Join(items, cmp.Or(sep, ","))}, ok
	case map[string]any:
		keys := slices.Sorted(maps.Keys(t))
		switch {
		case style == openapi3.SerializationDeepObject:
			for _, k := range keys {
				pairs = append(pairs, pair(name+"["+url.QueryEscape(k)+"]", t[k]))
			}
		case style == openapi3.SerializationForm && explode:
			for _, k := range keys {
				pairs = append(pairs, pair(url.QueryEscape(k), t[k]))
			}
		case style == openapi3.SerializationForm:
			parts := make([]string, 0, 2*len(keys))
			for _, k := range keys {
				parts = append(parts, url.QueryEscape(k), url.QueryEscape(text(t[k])))
			}
			return []string{name + "=" + strings.Join(parts, ",")}, true
		default:
			return []string{pair(name, t)}, false
		}
		return pairs, true
	}
	return []string{pair(name, v)}, true
}

func isAuthorizationHeader(p *openapi3.Parameter) bool {
	return p.In == openapi3.ParameterInHeader && strings.EqualFold(p.Name, "Authorization")
}

// OpenAPI ignores header parameters named Accept, Content-Type and Authorization.
func headerParams(params []*openapi3.Parameter) []entities.HeaderItem {
	var out []entities.HeaderItem
	for _, p := range params {
		if p.In != openapi3.ParameterInHeader {
			continue
		}
		switch strings.ToLower(p.Name) {
		case "accept", "content-type", "authorization":
			continue
		}
		if v, ok := paramValue(p); ok || p.Required {
			out = append(out, entities.HeaderItem{Key: p.Name, Value: v, Enabled: true})
		}
	}
	return out
}

func paramValue(p *openapi3.Parameter) (string, bool) {
	if v, ok := paramExample(p); ok {
		return text(v), true
	}
	return "", false
}

func paramExample(p *openapi3.Parameter) (any, bool) {
	if p.Example != nil {
		return p.Example, true
	}
	for _, name := range orderedKeys(p.Examples, nil) {
		if ex := p.Examples[name]; ex != nil && ex.Value != nil && ex.Value.Value != nil {
			return ex.Value.Value, true
		}
	}
	if p.Schema == nil || p.Schema.Value == nil {
		return nil, false
	}
	if v, ok := schemaExample(p.Schema.Value); ok {
		return v, true
	}
	if items := p.Schema.Value.Items; items != nil && items.Value != nil {
		return schemaExample(items.Value)
	}
	return nil, false
}

func schemaValue(s *openapi3.Schema) (string, bool) {
	if v, ok := schemaExample(s); ok {
		return text(v), true
	}
	return "", false
}

func schemaExample(s *openapi3.Schema) (any, bool) {
	switch {
	case s.Example != nil:
		return s.Example, true
	case len(s.Examples) > 0:
		return s.Examples[0], true
	case s.Default != nil:
		return s.Default, true
	case len(s.Enum) > 0:
		return s.Enum[0], true
	}
	return nil, false
}

func text(v any) string {
	switch t := v.(type) {
	case string:
		return t
	case []any:
		parts := make([]string, 0, len(t))
		for _, item := range t {
			parts = append(parts, text(item))
		}
		return strings.Join(parts, ",")
	default:
		raw, err := json.Marshal(t)
		if err != nil {
			return ""
		}
		return string(raw)
	}
}

func description(op *openapi3.Operation, params []*openapi3.Parameter) string {
	var parts []string
	if d := strings.TrimSpace(op.Description); d != "" {
		parts = append(parts, d)
	}
	var lines []string
	for _, p := range params {
		facts := []string{p.In}
		if t := schemaType(p.Schema); t != "" {
			facts = append(facts, t)
		}
		if p.Required {
			facts = append(facts, "required")
		}
		line := fmt.Sprintf("- `%s` (%s)", p.Name, strings.Join(facts, ", "))
		if d := strings.TrimSpace(p.Description); d != "" {
			line += ": " + strings.ReplaceAll(d, "\n", "\n  ")
		}
		lines = append(lines, line)
	}
	if len(lines) > 0 {
		parts = append(parts, "**Parameters**\n\n"+strings.Join(lines, "\n"))
	}
	return strings.Join(parts, "\n\n")
}

func schemaType(ref *openapi3.SchemaRef) string {
	return seenSchemas{}.schemaType(ref)
}

func (seen seenSchemas) schemaType(ref *openapi3.SchemaRef) string {
	if ref == nil || !seen.visit(ref.Value) {
		return ""
	}
	t := typeOf(ref.Value)
	if t == "array" {
		if item := seen.schemaType(ref.Value.Items); item != "" {
			return "array of " + item
		}
	}
	return t
}

type mediaKind int

const (
	mediaJSON mediaKind = iota
	mediaForm
	mediaMultipart
	mediaXML
	mediaText
	mediaBinary
)

func kindOf(mediaType string) mediaKind {
	mt, _, _ := strings.Cut(strings.ToLower(mediaType), ";")
	mt = strings.TrimSpace(mt)
	switch {
	case mt == "*/*" || strings.HasSuffix(mt, "/json") || strings.HasSuffix(mt, "+json"):
		return mediaJSON
	case mt == "application/x-www-form-urlencoded":
		return mediaForm
	case strings.HasPrefix(mt, "multipart/"):
		return mediaMultipart
	case strings.HasSuffix(mt, "/xml") || strings.HasSuffix(mt, "+xml"):
		return mediaXML
	case mt == "application/octet-stream" || mt == "application/pdf" || mt == "application/zip",
		strings.HasPrefix(mt, "image/"), strings.HasPrefix(mt, "audio/"), strings.HasPrefix(mt, "video/"):
		return mediaBinary
	}
	return mediaText
}

func headerType(mediaType string) string {
	if strings.TrimSpace(mediaType) == "*/*" {
		return "application/json"
	}
	return mediaType
}

func pickMedia(content openapi3.Content) (string, *openapi3.MediaType) {
	names := make([]string, 0, len(content))
	for name, mt := range content {
		if mt != nil {
			names = append(names, name)
		}
	}
	if len(names) == 0 {
		return "", nil
	}
	rank := func(name string) int {
		if strings.EqualFold(name, "application/json") {
			return -1
		}
		return int(kindOf(name))
	}
	slices.SortFunc(names, func(a, c string) int {
		if d := rank(a) - rank(c); d != 0 {
			return d
		}
		return strings.Compare(a, c)
	})
	return names[0], content[names[0]]
}

func (b *builder) body(label string, ref *openapi3.RequestBodyRef, pointer string) (entities.BodyType, string, string) {
	if ref == nil || ref.Value == nil {
		return entities.BodyTypeNone, "", ""
	}
	pointer = b.src.follow(ref.Ref, pointer)
	name, mt := pickMedia(ref.Value.Content)
	if mt == nil {
		return entities.BodyTypeNone, "", ""
	}
	mtPtr := ptr(pointer, "content", name)
	ct := headerType(name)
	switch kind := kindOf(name); kind {
	case mediaJSON:
		if bodies := b.mediaBodies(kind, mt, mtPtr); len(bodies) > 0 {
			return entities.BodyTypeJSON, bodies[0].body, ct
		}
		if mt.Schema == nil {
			return entities.BodyTypeJSON, "", ct
		}
		return entities.BodyTypeJSON, newSampler(b.src, true).json(mt.Schema, ptr(mtPtr, "schema")), ct
	case mediaForm, mediaMultipart:
		fields := b.formFields(mt, ptr(mtPtr, "schema"), kind == mediaMultipart)
		if kind == mediaMultipart && !slices.ContainsFunc(fields, func(f formField) bool { return f.Type == "file" }) {
			b.textForms = append(b.textForms, label)
		}
		raw, err := json.Marshal(fields)
		if err != nil {
			return entities.BodyTypeForm, "[]", ct
		}
		return entities.BodyTypeForm, string(raw), ct
	case mediaBinary:
		return entities.BodyTypeBinary, "", ct
	case mediaXML:
		return entities.BodyTypeXML, firstBody(b.mediaBodies(kind, mt, mtPtr)), ct
	default:
		return entities.BodyTypeRaw, firstBody(b.mediaBodies(kind, mt, mtPtr)), ct
	}
}

type formField struct {
	Key     string `json:"key"`
	Value   string `json:"value"`
	Type    string `json:"type"`
	Enabled bool   `json:"enabled"`
}

func (b *builder) formFields(mt *openapi3.MediaType, pointer string, multipart bool) []formField {
	fields := []formField{}
	if mt.Schema != nil && mt.Schema.Value != nil {
		example, _ := mt.Example.(map[string]any)
		g := newSampler(b.src, true)
		for _, p := range g.properties(mt.Schema.Value, b.src.follow(mt.Schema.Ref, pointer), 0, seenSchemas{}) {
			f := formField{Key: p.name, Type: "text", Enabled: true}
			s := p.ref.Value
			if v, ok := example[p.name]; ok {
				f.Value = text(v)
			} else if s != nil {
				f.Value, _ = schemaValue(s)
			}
			if multipart && s != nil && isFile(s) {
				f.Type, f.Value = "file", ""
			}
			fields = append(fields, f)
		}
	}
	return fields
}

func isFile(s *openapi3.Schema) bool {
	if typeOf(s) == "array" && s.Items != nil && s.Items.Value != nil {
		s = s.Items.Value
	}
	return s.Format == "binary" || s.Format == "base64" || s.ContentMediaType != "" || typeOf(s) == "file"
}

type namedBody struct {
	name string
	body string
}

func firstBody(bodies []namedBody) string {
	if len(bodies) == 0 {
		return ""
	}
	return bodies[0].body
}

func (b *builder) mediaBodies(kind mediaKind, mt *openapi3.MediaType, pointer string) []namedBody {
	render := func(at string, v any) (string, bool) {
		if kind == mediaJSON {
			return b.jsonText(at, v), true
		}
		s, ok := v.(string)
		return s, ok
	}
	var out []namedBody
	if mt.Example != nil {
		if body, ok := render(ptr(pointer, "example"), mt.Example); ok {
			out = append(out, namedBody{body: body})
		}
	}
	for _, key := range orderedKeys(mt.Examples, b.src.at(ptr(pointer, "examples"))) {
		ex := mt.Examples[key]
		if ex == nil || ex.Value == nil || ex.Value.Value == nil {
			continue
		}
		name := key
		if s := strings.TrimSpace(ex.Value.Summary); s != "" {
			name = s
		}
		if body, ok := render(ptr(b.src.follow(ex.Ref, ptr(pointer, "examples", key)), "value"), ex.Value.Value); ok {
			out = append(out, namedBody{name: name, body: body})
		}
	}
	return out
}

func (b *builder) jsonText(pointer string, v any) string {
	if s, ok := v.(string); ok {
		if t := strings.TrimSpace(s); (strings.HasPrefix(t, "{") || strings.HasPrefix(t, "[")) && json.Valid([]byte(t)) {
			return s
		}
	}
	var buf bytes.Buffer
	writeValue(&buf, b.src, pointer, v)
	return indentJSON(buf.Bytes())
}

func (b *builder) examples(op *openapi3.Operation, opPtr, label string) []example.Create {
	if op.Responses == nil {
		return nil
	}
	responses := op.Responses.Map()
	var out []example.Create
	links := false
	for _, code := range responseCodes(responses) {
		ref := responses[code]
		if ref == nil || ref.Value == nil {
			continue
		}
		r := ref.Value
		links = links || len(r.Links) > 0
		rPtr := b.src.follow(ref.Ref, ptr(opPtr, "responses", code))
		status, statusName := statusOf(code)
		name, mt := pickMedia(r.Content)

		var bodies []namedBody
		contentType := headerType(name)
		if b.src.swagger2 {
			bodies, contentType = b.swagger2Examples(rPtr, contentType)
		} else if mt != nil {
			bodies = b.mediaBodies(kindOf(name), mt, ptr(rPtr, "content", name))
		}
		if len(bodies) == 0 && mt != nil && mt.Schema != nil && kindOf(name) == mediaJSON && status >= 200 && status < 300 {
			if body := newSampler(b.src, false).json(mt.Schema, ptr(rPtr, "content", name, "schema")); body != "{}" {
				bodies = []namedBody{{body: body}}
			}
		}
		for _, nb := range bodies {
			exName := statusName
			if nb.name != "" && len(bodies) > 1 {
				exName += " (" + nb.name + ")"
			}
			if utf8.RuneCountInString(exName) > maxExampleNameLen {
				exName = string([]rune(exName)[:maxExampleNameLen])
			}
			if len(nb.body) > domain.MaxExampleBodyLen {
				b.warn(fmt.Sprintf("%s: example %q skipped: body is larger than %d KB", label, exName, domain.MaxExampleBodyLen/1024))
				continue
			}
			out = append(out, example.Create{
				Name:        exName,
				StatusCode:  status,
				StatusText:  http.StatusText(status),
				Headers:     []entities.HeaderItem{{Key: "Content-Type", Value: contentType, Enabled: true}},
				Body:        nb.body,
				ContentType: contentType,
				Protocol:    entities.ProtocolHTTP,
			})
			b.p.exampleCount++
		}
	}
	if links {
		b.warn(label + ": response links were skipped")
	}
	return out
}

// swagger2Examples reads the source: the 2.0 to 3.0 conversion drops response examples.
func (b *builder) swagger2Examples(responsePtr, contentType string) ([]namedBody, string) {
	n := b.src.at(ptr(responsePtr, "examples"))
	mimes := keys(n)
	if len(mimes) == 0 {
		return nil, contentType
	}
	var out []namedBody
	for _, mime := range mimes {
		v := field(n, mime)
		if kindOf(mime) == mediaJSON {
			var buf bytes.Buffer
			if writeJSON(&buf, v, maxDocumentJSON) == nil {
				out = append(out, namedBody{name: mime, body: indentJSON(buf.Bytes())})
			}
		} else if v.Kind == yaml.ScalarNode {
			out = append(out, namedBody{name: mime, body: v.Value})
		}
	}
	return out, headerType(mimes[0])
}

func responseCodes(responses map[string]*openapi3.ResponseRef) []string {
	codes := slices.Collect(maps.Keys(responses))
	rank := func(code string) int {
		if n, err := strconv.Atoi(code); err == nil {
			return n * 10
		}
		if len(code) == 3 && strings.EqualFold(code[1:], "XX") {
			return int(code[0]-'0')*1000 + 999
		}
		return 1 << 30
	}
	slices.SortFunc(codes, func(a, c string) int {
		if d := rank(a) - rank(c); d != 0 {
			return d
		}
		return strings.Compare(a, c)
	})
	return codes
}

func statusOf(code string) (int, string) {
	if n, err := strconv.Atoi(code); err == nil && n >= 0 && n <= 999 {
		if t := http.StatusText(n); t != "" {
			return n, fmt.Sprintf("%d %s", n, t)
		}
		return n, code
	}
	if len(code) == 3 && code[0] >= '1' && code[0] <= '5' && strings.EqualFold(code[1:], "XX") {
		return int(code[0]-'0') * 100, strings.ToUpper(code)
	}
	return 0, "Default"
}
