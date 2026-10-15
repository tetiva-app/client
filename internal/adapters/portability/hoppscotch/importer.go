package hoppscotch

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"regexp"
	"slices"
	"strings"

	"github.com/tetiva-app/client/internal/adapters/portability"
	"github.com/tetiva-app/client/internal/domain"
	"github.com/tetiva-app/client/internal/domain/entities"
	"github.com/tetiva-app/client/internal/domain/usecase/collection"
	"github.com/tetiva-app/client/internal/domain/usecase/environment"
	"github.com/tetiva-app/client/internal/domain/usecase/example"
	"github.com/tetiva-app/client/internal/domain/usecase/request"
)

const (
	defaultName = "Hoppscotch import"
	untitled    = "Untitled"
)

var variableRef = regexp.MustCompile(`<<([^>]*)>>`)

type Importer struct {
	w portability.Writer
}

func NewImporter(
	collections portability.CollectionCreator,
	requests portability.RequestCreator,
	examples portability.ExampleCreator,
	environments portability.EnvironmentCreator,
) *Importer {
	return &Importer{w: portability.Writer{
		Collections: collections, Requests: requests, Examples: examples, Environments: environments,
	}}
}

type probe struct {
	Name     *string         `json:"name"`
	Folders  json.RawMessage `json:"folders"`
	Requests json.RawMessage `json:"requests"`
	Order    json.RawMessage `json:"order"`
}

// A Postman v1 collection also has name, folders and requests; its "order" tells it apart.
func (p probe) collection() bool {
	isList := func(raw json.RawMessage) bool { return bytes.HasPrefix(bytes.TrimSpace(raw), []byte("[")) }
	return p.Name != nil && isList(p.Folders) && isList(p.Requests) && p.Order == nil
}

func (i *Importer) Detect(data []byte) bool {
	data = bytes.TrimSpace(data)
	if bytes.HasPrefix(data, []byte("[")) {
		var list []probe
		if json.Unmarshal(data, &list) != nil || len(list) == 0 {
			return false
		}
		return !slices.ContainsFunc(list, func(p probe) bool { return !p.collection() })
	}
	var p probe
	return json.Unmarshal(data, &p) == nil && p.collection()
}

func (i *Importer) Preview(data []byte, fileName string) (*portability.ImportPreview, error) {
	b, err := parse(data, fileName)
	if err != nil {
		return nil, fmt.Errorf("hoppscotch.Importer.Preview: %w", err)
	}
	p := b.tree.Preview(portability.FormatHoppscotch)
	if len(p.Scripts) > 0 {
		p.Warnings = append(p.Warnings, scriptsWarning())
	}
	return p, nil
}

func (i *Importer) Import(ctx context.Context, data []byte, opt portability.ImportOpt) (*portability.ImportResult, error) {
	b, err := parse(data, opt.FileName)
	if err != nil {
		return nil, fmt.Errorf("hoppscotch.Importer.Import: %w", err)
	}
	res, err := i.w.Write(ctx, b.tree, opt)
	if err != nil {
		return nil, err
	}
	if opt.IncludeScripts && b.scripts {
		res.Warnings = append(res.Warnings, scriptsWarning())
	}
	return res, nil
}

func scriptsWarning() portability.Warning {
	return portability.Warn(portability.WarnForeignScripts, portability.Subject{}, "app", "Hoppscotch", "api", "pw.* / hopp.*")
}

type builder struct {
	tree      *portability.Tree
	vars      []environment.AddVariable
	values    map[string]string
	conflicts []string
	secrets   []string
	textForms []string
	authOff   []string
	scripts   bool
	headers   bool
}

func parse(data []byte, fileName string) (*builder, error) {
	var cols []hoppCollection
	if err := json.Unmarshal(asList(data), &cols); err != nil {
		return nil, &domain.ValidationError{Fields: map[string]string{"content": "not a valid Hoppscotch collection: " + err.Error()}}
	}

	b := &builder{tree: &portability.Tree{}, values: map[string]string{}}
	if len(cols) == 1 {
		b.tree.Root = b.collection(cols[0], nil, false)
		b.environment(b.tree.Root.In.Name)
	} else {
		b.tree.Root.In.Name = portability.FileTitle(fileName, defaultName)
		for _, c := range cols {
			f := b.collection(c, nil, false)
			b.tree.Root.Folders = append(b.tree.Root.Folders, f)
			b.environment(f.In.Name)
		}
	}
	none := portability.Subject{}
	if len(b.conflicts) > 0 {
		b.warn(portability.WarnHoppscotchVariableConflicts, none, "keys", strings.Join(b.conflicts, ", "))
	}
	if len(b.secrets) > 0 {
		b.tree.Warnings = append(b.tree.Warnings, secretsWarning(b.secrets))
	}
	if b.headers {
		b.warn(portability.WarnHoppscotchHeadersCopied, none)
	}
	if len(b.textForms) > 0 {
		b.warn(portability.WarnFormsSentURLEncoded, none, "n", len(b.textForms), "requests", portability.List{Items: b.textForms})
	}
	if len(b.authOff) > 0 {
		b.warn(portability.WarnHoppscotchAuthOff, none, "n", len(b.authOff), "folders", portability.List{Items: b.authOff})
	}
	return b, nil
}

func (b *builder) environment(name string) {
	if len(b.vars) > 0 {
		b.tree.Environments = append(b.tree.Environments, portability.Environment{Name: name, Vars: b.vars})
	}
	b.vars, b.values = nil, map[string]string{}
}

func asList(data []byte) []byte {
	data = bytes.TrimSpace(data)
	if bytes.HasPrefix(data, []byte("[")) {
		return data
	}
	return append(append([]byte("["), data...), ']')
}

func (b *builder) warn(code portability.WarningCode, subject portability.Subject, kv ...any) {
	b.tree.Warnings = append(b.tree.Warnings, portability.Warn(code, subject, kv...))
}

func (b *builder) collection(c hoppCollection, inherited []entities.HeaderItem, parentAuth bool) portability.Folder {
	name := nameOr(c.Name)
	subject := portability.Subject{Kind: portability.KindFolder, Name: name}
	description, ws := portability.ClampDescription(subject, deref(c.Description))
	b.tree.Warnings = append(b.tree.Warnings, ws...)
	authType, authData := b.auth(c.Auth, subject, false)
	if authType == entities.AuthTypeInherit {
		authType = entities.AuthTypeNone
	}
	if parentAuth && c.Auth != nil && (c.Auth.AuthType == "none" || c.Auth.AuthActive != nil && !*c.Auth.AuthActive) {
		b.authOff = append(b.authOff, `"`+name+`"`)
	}
	b.scripts = b.scripts || hasScript(c.PreRequestScript, c.TestScript)
	f := portability.Folder{In: collection.Create{
		Name: name, Description: description, AuthType: authType, AuthData: authData,
		PreScript: c.PreRequestScript, PostScript: c.TestScript,
	}}

	for _, v := range c.Variables {
		value := v.CurrentValue
		if value == "" {
			value = v.InitialValue
		}
		if v.Secret && value == "" && strings.TrimSpace(v.Key) != "" && !slices.Contains(b.secrets, v.Key) {
			b.secrets = append(b.secrets, v.Key)
		}
		b.variable(environment.AddVariable{Key: v.Key, Value: vars(value), IsSecret: v.Secret})
	}
	own := headerItems(c.Headers)
	b.headers = b.headers || len(own) > 0
	headers := mergeHeaders(inherited, own)
	for _, sub := range c.Folders {
		f.Folders = append(f.Folders, b.collection(sub, headers, parentAuth || authType != entities.AuthTypeNone))
	}
	for _, r := range c.Requests {
		f.Requests = append(f.Requests, b.request(r, headers))
	}
	return f
}

func (b *builder) variable(v environment.AddVariable) {
	if strings.TrimSpace(v.Key) == "" {
		return
	}
	if seen, ok := b.values[v.Key]; ok {
		if seen != v.Value && !slices.Contains(b.conflicts, v.Key) {
			b.conflicts = append(b.conflicts, v.Key)
		}
		return
	}
	b.values[v.Key] = v.Value
	b.vars = append(b.vars, v)
}

func (b *builder) request(r hoppRequest, inherited []entities.HeaderItem) portability.Request {
	name := nameOr(r.Name)
	subject := portability.Subject{Kind: portability.KindRequest, Name: name}
	description, ws := portability.ClampDescription(subject, deref(r.Description))
	b.tree.Warnings = append(b.tree.Warnings, ws...)
	authType, authData := b.auth(r.Auth, subject, true)
	b.scripts = b.scripts || hasScript(r.PreRequestScript, r.TestScript)
	in := request.Create{
		Name: name, Description: description, AuthType: authType, AuthData: authData,
		Headers: mergeHeaders(inherited, headerItems(r.Headers)), PreScript: r.PreRequestScript, PostScript: r.TestScript,
	}
	for _, v := range r.RequestVariables {
		b.variable(environment.AddVariable{Key: v.Key, Value: vars(v.Value), Disabled: !v.enabled()})
	}

	if r.Endpoint == nil && r.Query != nil {
		in.Protocol = entities.ProtocolGraphQL
		in.Method = entities.MethodPOST
		in.URL = vars(r.URL)
		in.BodyType = entities.BodyTypeNone
		in.GraphQLQuery = *r.Query
		in.GraphQLVariables = vars(r.Variables)
		return portability.Request{In: in}
	}

	in.Protocol = entities.ProtocolHTTP
	in.Method = entities.HTTPMethod(strings.ToUpper(strings.TrimSpace(r.Method)))
	if !in.Method.IsValid() {
		in.Method = entities.MethodGET
	}
	in.URL = withParams(vars(deref(r.Endpoint)), r.Params)
	var contentType string
	in.BodyType, in.Body, contentType = b.body(r.Body, name)
	if in.BodyType == entities.BodyTypeForm {
		in.Headers = slices.DeleteFunc(in.Headers, func(h entities.HeaderItem) bool {
			mt, _, _ := strings.Cut(h.Value, ";")
			return isContentType(h) && strings.EqualFold(strings.TrimSpace(mt), "multipart/form-data")
		})
	}
	if contentType != "" && !slices.ContainsFunc(in.Headers, isContentType) {
		in.Headers = append(in.Headers, entities.HeaderItem{Key: "Content-Type", Value: contentType, Enabled: true})
	}
	return portability.Request{In: in, Examples: examples(r.Responses)}
}

func withParams(endpoint string, params []hoppKV) string {
	var query []string
	for _, p := range params {
		if p.enabled() && p.Key != "" {
			query = append(query, portability.QueryPair(vars(p.Key), vars(p.Value)))
		}
	}
	return portability.WithQuery(endpoint, query)
}

func (b *builder) body(raw json.RawMessage, name string) (bodyType entities.BodyType, body, contentType string) {
	subject := portability.Subject{Kind: portability.KindRequest, Name: name}
	var hb hoppBody
	if json.Unmarshal(raw, &hb) != nil || hb.ContentType == nil {
		return entities.BodyTypeNone, "", ""
	}
	ct := *hb.ContentType
	var text string
	_ = json.Unmarshal(hb.Body, &text)

	switch {
	case ct == "application/x-www-form-urlencoded":
		return entities.BodyTypeForm, formJSON(parseRawKeyValues(text), nil), ""
	case ct == "multipart/form-data":
		var fields []hoppFormField
		_ = json.Unmarshal(hb.Body, &fields)
		for _, f := range fields {
			if f.IsFile {
				b.warn(portability.WarnFileFieldDropped, subject, "field", f.Key)
			}
		}
		if !slices.ContainsFunc(fields, func(f hoppFormField) bool { return f.IsFile }) {
			b.textForms = append(b.textForms, `"`+name+`"`)
		}
		return entities.BodyTypeForm, formJSON(nil, fields), ""
	case ct == "application/octet-stream" || ct == "binary":
		b.warn(portability.WarnFileBodyDropped, subject)
		return entities.BodyTypeBinary, "", ""
	case strings.Contains(ct, "json"):
		bodyType = entities.BodyTypeJSON
	case strings.Contains(ct, "xml"):
		bodyType = entities.BodyTypeXML
	default:
		bodyType = entities.BodyTypeRaw
	}
	if ct != "application/json" && ct != "application/xml" {
		contentType = ct
	}
	return bodyType, vars(text), contentType
}

func formJSON(kvs []hoppKV, fields []hoppFormField) string {
	out := make([]map[string]any, 0, len(kvs)+len(fields))
	for _, kv := range kvs {
		out = append(out, map[string]any{"key": vars(kv.Key), "value": vars(kv.Value), "type": "text", "enabled": kv.enabled()})
	}
	for _, f := range fields {
		field := map[string]any{"key": vars(f.Key), "value": "", "type": "text", "enabled": f.Active == nil || *f.Active}
		if f.IsFile {
			field["type"] = "file"
		} else {
			var v string
			_ = json.Unmarshal(f.Value, &v)
			field["value"] = vars(v)
		}
		out = append(out, field)
	}
	data, _ := json.Marshal(out)
	return string(data)
}

func examples(responses map[string]hoppResponse) []example.Create {
	var out []example.Create
	for _, k := range slices.Sorted(maps.Keys(responses)) {
		r := responses[k]
		name := strings.TrimSpace(r.Name)
		if name == "" {
			name = k
		}
		ex := example.Create{Name: name, StatusText: r.Status, Headers: []entities.HeaderItem{}, Body: r.Body, Protocol: entities.ProtocolHTTP}
		if r.Code != nil {
			ex.StatusCode = *r.Code
		}
		for _, h := range r.Headers {
			item := entities.HeaderItem{Key: h.Key, Value: h.Value, Enabled: true}
			if isContentType(item) {
				ex.ContentType = h.Value
			}
			ex.Headers = append(ex.Headers, item)
		}
		out = append(out, ex)
	}
	return out
}

func headerItems(kvs []hoppKV) []entities.HeaderItem {
	out := make([]entities.HeaderItem, 0, len(kvs))
	for _, kv := range kvs {
		if kv.Key == "" && kv.Value == "" {
			continue
		}
		out = append(out, entities.HeaderItem{Key: vars(kv.Key), Value: vars(kv.Value), Enabled: kv.enabled()})
	}
	return out
}

func mergeHeaders(parent, own []entities.HeaderItem) []entities.HeaderItem {
	out := make([]entities.HeaderItem, 0, len(parent)+len(own))
	for _, p := range parent {
		if !slices.ContainsFunc(own, func(h entities.HeaderItem) bool { return strings.EqualFold(h.Key, p.Key) }) {
			out = append(out, p)
		}
	}
	return append(out, own...)
}

func isContentType(h entities.HeaderItem) bool {
	return strings.EqualFold(strings.TrimSpace(h.Key), "Content-Type")
}

func vars(s string) string {
	return variableRef.ReplaceAllString(s, "{{$1}}")
}

func hasScript(pre, post string) bool {
	return strings.TrimSpace(pre) != "" || strings.TrimSpace(post) != ""
}

func nameOr(name string) string {
	if strings.TrimSpace(name) == "" {
		return untitled
	}
	return name
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
