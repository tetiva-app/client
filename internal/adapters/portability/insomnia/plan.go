package insomnia

import (
	"encoding/json"
	"fmt"
	"slices"
	"strings"

	"github.com/google/uuid"

	"github.com/tetiva-app/client/internal/adapters/portability"
	"github.com/tetiva-app/client/internal/domain/entities"
	"github.com/tetiva-app/client/internal/domain/usecase/collection"
	"github.com/tetiva-app/client/internal/domain/usecase/environment"
	"github.com/tetiva-app/client/internal/domain/usecase/request"
	"github.com/tetiva-app/client/internal/domain/usecase/websocket"
)

type plan struct {
	includeScripts bool
	tpl            templates

	roots    []*folderPlan
	envs     []envPlan
	folders  int
	requests int
	urls     []string
	scripts  []portability.ScriptPreview
	warnings []portability.Warning

	pathVars         []environment.AddVariable
	pathConflicts    []string
	textForms        []string
	disabledParams   int
	disabledMetadata int
	grpc             int
	vaultSecrets     int
	scriptsImported  bool
}

type folderPlan struct {
	in       collection.Create
	folders  []*folderPlan
	requests []request.Create
}

type envPlan struct {
	name string
	vars []environment.AddVariable
}

type formField struct {
	Key     string `json:"key"`
	Value   string `json:"value"`
	Type    string `json:"type"`
	Enabled bool   `json:"enabled"`
}

func newPlan(ex *export, includeScripts bool) *plan {
	p := &plan{includeScripts: includeScripts}
	for _, ws := range ex.workspaces {
		p.pathVars, p.pathConflicts = nil, nil
		name := orUntitled(ws.name)
		root := &folderPlan{in: collection.Create{
			Name:        name,
			Description: p.description(portability.Subject{Kind: portability.KindCollection, Name: name}, ws.description),
			AuthType:    entities.AuthTypeNone,
			AuthData:    "{}",
		}}
		p.items(root, name, ws.items)
		p.roots = append(p.roots, root)
		p.environments(ws, name)
		if len(p.pathConflicts) > 0 {
			p.warn(portability.WarnInsomniaPathParamsDiffer, portability.Subject{},
				"n", len(p.pathConflicts), "names", strings.Join(p.pathConflicts, ", "))
		}
	}
	p.finish(ex)
	return p
}

func (p *plan) finish(ex *export) {
	none := portability.Subject{}
	if len(p.roots) > 1 {
		names := make([]string, 0, len(p.roots))
		for _, r := range p.roots {
			names = append(names, `"`+r.in.Name+`"`)
		}
		p.warn(portability.WarnInsomniaWorkspaces, none, "n", len(p.roots), "names", strings.Join(names, ", "))
	}
	if p.disabledParams > 0 {
		p.warn(portability.WarnInsomniaDisabledParams, none, "n", p.disabledParams)
	}
	if p.disabledMetadata > 0 {
		p.warn(portability.WarnInsomniaDisabledMetadata, none, "n", p.disabledMetadata)
	}
	if len(p.textForms) > 0 {
		p.warn(portability.WarnFormsSentURLEncoded, none, "n", len(p.textForms), "requests", portability.List{Items: p.textForms})
	}
	if p.grpc > 0 {
		p.warn(portability.WarnInsomniaGRPCNoProto, none)
	}
	if p.vaultSecrets > 0 {
		p.warn(portability.WarnInsomniaVaultSecrets, none, "n", p.vaultSecrets)
	}
	p.warnings = append(p.warnings, p.tpl.warning()...)
	if p.scriptsImported {
		p.warn(portability.WarnForeignScripts, none, "app", "Insomnia", "api", "insomnia.*")
	}
	for _, skipped := range []struct {
		n    int
		code portability.WarningCode
	}{
		{ex.socketIOs, portability.WarnInsomniaSocketIOSkipped},
		{ex.unitTests, portability.WarnInsomniaUnitTestsSkipped},
		{ex.cookies, portability.WarnInsomniaCookiesSkipped},
		{ex.specs, portability.WarnInsomniaSpecsSkipped},
	} {
		if skipped.n > 0 {
			p.warn(skipped.code, none, "n", skipped.n)
		}
	}
}

func (p *plan) warn(code portability.WarningCode, subject portability.Subject, kv ...any) {
	p.warnings = append(p.warnings, portability.Warn(code, subject, kv...))
}

func (p *plan) items(parent *folderPlan, path string, items []*resource) {
	for _, it := range items {
		if it.Type == typeGroup {
			parent.folders = append(parent.folders, p.folder(it, path))
			continue
		}
		parent.requests = append(parent.requests, p.request(it, path))
	}
}

func (p *plan) folder(r *resource, parentPath string) *folderPlan {
	name := orUntitled(r.Name)
	subject := portability.Subject{Kind: portability.KindFolder, Name: name}
	path := parentPath + " / " + name
	f := &folderPlan{in: collection.Create{Name: name, Description: p.description(subject, r.Description)}}
	f.in.AuthType, f.in.AuthData = p.auth(subject, r.Authentication, false)
	f.in.PreScript, f.in.PostScript = p.scriptsOf(path, r)
	if len(r.Headers) > 0 {
		p.warn(portability.WarnInsomniaFolderHeaders, subject)
	}
	if len(r.Environment) > 0 {
		p.warn(portability.WarnInsomniaFolderEnvironment, subject)
	}
	p.folders++
	p.items(f, path, r.Children)
	return f
}

func (p *plan) request(r *resource, parentPath string) request.Create {
	name := orUntitled(r.Name)
	subject := portability.Subject{Kind: portability.KindRequest, Name: name}
	c := p.tpl.conv
	in := request.Create{Name: name, Description: p.description(subject, r.Description), BodyType: entities.BodyTypeNone}

	switch r.Type {
	case typeGRPC:
		in.Protocol, in.Method, in.URL = entities.ProtocolGRPC, entities.MethodPOST, c(r.URL)
		in.GRPCService, in.GRPCMethod = grpcMethod(r.ProtoMethodName)
		in.BodyType, in.Body = entities.BodyTypeJSON, c(r.Body.Text)
		in.GRPCMetadata = p.metadata(r.Metadata)
		in.AuthType, in.AuthData = entities.AuthTypeNone, "{}"
		p.grpc++
	case typeWebSocket:
		in.Protocol, in.Method, in.URL = entities.ProtocolWebSocket, entities.MethodGET, p.url(r)
		in.Headers = p.headers(r.Headers)
		in.AuthType, in.AuthData = p.auth(subject, r.Authentication, true)
		in.BodyType, in.Body = entities.BodyTypeRaw, p.wsSettings(r)
	default:
		in.Protocol, in.URL = entities.ProtocolHTTP, p.url(r)
		in.Method = entities.HTTPMethod(strings.ToUpper(r.Method))
		if !in.Method.IsValid() {
			if r.Method != "" {
				p.warn(portability.WarnInsomniaMethodUnsupported, subject, "method", r.Method)
			}
			in.Method = entities.MethodGET
		}
		in.Headers = p.headers(r.Headers)
		in.AuthType, in.AuthData = p.auth(subject, r.Authentication, true)
		p.body(&in, subject, r.Body)
	}
	in.PreScript, in.PostScript = p.scriptsOf(parentPath+" / "+name, r)
	p.urls = append(p.urls, in.URL)
	p.requests++
	return in
}

func (p *plan) url(r *resource) string {
	c := p.tpl.conv
	values := map[string]string{}
	for _, pp := range r.PathParameters {
		values[pp.Name] = c(pp.Value)
	}
	u := pathParam.ReplaceAllStringFunc(c(r.URL), func(m string) string {
		name := m[2:]
		p.pathVar(name, values[name])
		return "/{{" + name + "}}"
	})

	var query []string
	for _, q := range r.Parameters {
		switch {
		case q.Disabled:
			p.disabledParams++
		case q.Name != "" || q.Value != "":
			query = append(query, portability.QueryPair(c(q.Name), c(q.Value)))
		}
	}
	return portability.WithQuery(u, query)
}

func (p *plan) pathVar(name, value string) {
	i := slices.IndexFunc(p.pathVars, func(v environment.AddVariable) bool { return v.Key == name })
	switch {
	case i < 0:
		p.pathVars = append(p.pathVars, environment.AddVariable{Key: name, Value: value})
	case p.pathVars[i].Value == "":
		p.pathVars[i].Value = value
	case value != "" && value != p.pathVars[i].Value && !slices.Contains(p.pathConflicts, name):
		p.pathConflicts = append(p.pathConflicts, name)
	}
}

func (p *plan) headers(hs []pair) []entities.HeaderItem {
	out := make([]entities.HeaderItem, 0, len(hs))
	for _, h := range hs {
		if h.Name == "" && h.Value == "" {
			continue
		}
		out = append(out, entities.HeaderItem{Key: p.tpl.conv(h.Name), Value: p.tpl.conv(h.Value), Enabled: !h.Disabled})
	}
	return out
}

func (p *plan) metadata(md []pair) map[string][]string {
	out := map[string][]string{}
	for _, m := range md {
		switch {
		case m.Disabled:
			p.disabledMetadata++
		case m.Name != "":
			key := p.tpl.conv(m.Name)
			out[key] = append(out[key], p.tpl.conv(m.Value))
		}
	}
	return out
}

func (p *plan) body(in *request.Create, subject portability.Subject, b body) {
	c := p.tpl.conv
	mime := mediaType(b.MimeType)
	switch {
	case mime == "application/graphql":
		p.graphQL(in, subject, b.Text)
	case mime == "application/x-www-form-urlencoded" || mime == "multipart/form-data":
		in.Headers = slices.DeleteFunc(in.Headers, func(h entities.HeaderItem) bool {
			return strings.EqualFold(strings.TrimSpace(h.Key), "Content-Type") && mediaType(h.Value) == mime
		})
		fields := make([]formField, 0, len(b.Params))
		files := false
		for _, f := range b.Params {
			field := formField{Key: c(f.Name), Value: c(f.Value), Type: "text", Enabled: !f.Disabled}
			if f.Type == "file" {
				field.Type, field.Value, files = "file", "", true
				p.warn(portability.WarnFileFieldDropped, subject, "field", f.Name)
			}
			fields = append(fields, field)
		}
		if mime == "multipart/form-data" && !files {
			p.textForms = append(p.textForms, `"`+in.Name+`"`)
		}
		raw, _ := json.Marshal(fields)
		in.BodyType, in.Body = entities.BodyTypeForm, string(raw)
	case mime == "application/octet-stream" || b.FileName != "":
		if b.FileName != "" {
			p.warn(portability.WarnFileBodyDropped, subject)
		}
		in.BodyType = entities.BodyTypeBinary
	case mime == "" && b.Text == "":
	case strings.HasSuffix(mime, "json"):
		in.BodyType, in.Body = entities.BodyTypeJSON, c(b.Text)
	case strings.HasSuffix(mime, "xml"):
		in.BodyType, in.Body = entities.BodyTypeXML, c(b.Text)
	default:
		in.BodyType, in.Body = entities.BodyTypeRaw, c(b.Text)
	}
}

func (p *plan) graphQL(in *request.Create, subject portability.Subject, text string) {
	in.Protocol, in.Method, in.BodyType = entities.ProtocolGraphQL, entities.MethodPOST, entities.BodyTypeNone
	var gql struct {
		Query         string          `json:"query"`
		Variables     json.RawMessage `json:"variables"`
		OperationName string          `json:"operationName"`
	}
	if err := json.Unmarshal([]byte(text), &gql); err != nil {
		p.warn(portability.WarnInsomniaGraphQLNotJSON, subject)
		in.GraphQLQuery = p.tpl.conv(text)
		return
	}
	in.GraphQLQuery = p.tpl.conv(gql.Query)
	in.GraphQLOperation = gql.OperationName
	vars := string(gql.Variables)
	var s string
	if json.Unmarshal(gql.Variables, &s) == nil {
		vars = s
	}
	if vars != "null" {
		in.GraphQLVariables = p.tpl.conv(vars)
	}
}

func (p *plan) wsSettings(r *resource) string {
	for _, c := range r.Children {
		if c.Type != typePayload || strings.TrimSpace(c.Value) == "" {
			continue
		}
		format := "text"
		if strings.Contains(c.Mode, "json") {
			format = "json"
		}
		raw, err := json.Marshal(map[string]any{
			"version": websocket.SettingsVersion,
			"messages": []map[string]string{{
				"id": uuid.NewString(), "name": orUntitled(c.Name), "format": format, "data": p.tpl.conv(c.Value),
			}},
		})
		if err != nil {
			return ""
		}
		return string(raw)
	}
	return ""
}

func (p *plan) scriptsOf(path string, r *resource) (string, string) {
	pre, post := r.PreRequestScript, r.AfterResponseScript
	if strings.TrimSpace(pre) == "" {
		pre = ""
	} else {
		p.scripts = append(p.scripts, portability.ScriptPreview{Path: path, Phase: "pre", Text: pre})
	}
	if strings.TrimSpace(post) == "" {
		post = ""
	} else {
		p.scripts = append(p.scripts, portability.ScriptPreview{Path: path, Phase: "post", Text: post})
	}
	if !p.includeScripts {
		return "", ""
	}
	if pre != "" || post != "" {
		p.scriptsImported = true
	}
	return pre, post
}

func (p *plan) environments(ws *workspace, collectionName string) {
	var base []environment.AddVariable
	if ws.base != nil {
		base = p.envVars(ws.base)
	}
	var envs []envPlan
	if len(ws.subs) == 0 && (len(base) > 0 || len(p.pathVars) > 0) {
		envs = append(envs, envPlan{name: collectionName, vars: base})
	}
	for i, s := range ws.subs {
		name := strings.TrimSpace(s.Name)
		if name == "" {
			name = fmt.Sprintf("Environment %d", i+1)
		}
		envs = append(envs, envPlan{name: collectionName + " · " + name, vars: merge(base, p.envVars(s))})
	}
	for _, env := range envs {
		for _, v := range p.pathVars {
			if !slices.ContainsFunc(env.vars, func(e environment.AddVariable) bool { return e.Key == v.Key }) {
				env.vars = append(env.vars, v)
			}
		}
		if len(env.vars) > 0 {
			p.envs = append(p.envs, env)
		}
	}
}

func (p *plan) envVars(r *resource) []environment.AddVariable {
	out := make([]environment.AddVariable, 0, len(r.Data))
	for _, v := range r.Data {
		name, vault := strings.CutPrefix(v.key, "__insomnia_vault.")
		switch {
		case strings.TrimSpace(name) == "":
		case vault:
			p.vaultSecrets++
			out = append(out, environment.AddVariable{Key: name, IsSecret: true})
		default:
			out = append(out, environment.AddVariable{Key: name, Value: p.tpl.conv(v.value)})
		}
	}
	return out
}

func merge(base, over []environment.AddVariable) []environment.AddVariable {
	out := slices.Clone(base)
	for _, v := range over {
		if i := slices.IndexFunc(out, func(b environment.AddVariable) bool { return b.Key == v.Key }); i >= 0 {
			out[i] = v
			continue
		}
		out = append(out, v)
	}
	return out
}

func (p *plan) description(subject portability.Subject, s string) string {
	text, warnings := portability.ClampDescription(subject, s)
	p.warnings = append(p.warnings, warnings...)
	return text
}

func mediaType(contentType string) string {
	mt, _, _ := strings.Cut(contentType, ";")
	return strings.ToLower(strings.TrimSpace(mt))
}

func grpcMethod(full string) (string, string) {
	service, method, ok := strings.Cut(strings.TrimPrefix(full, "/"), "/")
	if !ok {
		return "", ""
	}
	return service, method
}

func orUntitled(name string) string {
	if strings.TrimSpace(name) == "" {
		return "Untitled"
	}
	return name
}
