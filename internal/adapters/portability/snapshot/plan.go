package snapshot

import (
	"encoding/json"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"

	"github.com/tetiva-app/client/internal/adapters/portability"
	"github.com/tetiva-app/client/internal/domain"
	"github.com/tetiva-app/client/internal/domain/entities"
	"github.com/tetiva-app/client/internal/domain/usecase/collection"
	"github.com/tetiva-app/client/internal/domain/usecase/environment"
	"github.com/tetiva-app/client/internal/domain/usecase/example"
	"github.com/tetiva-app/client/internal/domain/usecase/publication"
	"github.com/tetiva-app/client/internal/domain/usecase/request"
)

const (
	maxNameRunes = 200
	untitled     = "Untitled"
)

type plan struct {
	includeScripts bool

	root         folderPlan
	grpcMetadata []entities.HeaderItem
	env          *envPlan

	folders, requests, examples int
	urls                        []string
	publicVars                  map[string]string
	scripts                     []portability.ScriptPreview
	warnings                    []portability.Warning
}

type folderPlan struct {
	in    collection.Create
	items []itemPlan
}

type itemPlan struct {
	folder  *folderPlan
	request *requestPlan
}

type requestPlan struct {
	in       request.Create
	subject  portability.Subject
	examples []example.Create
}

type envPlan struct {
	name string
	vars []environment.AddVariable
}

func newPlan(s *publication.Snapshot, includeScripts bool) *plan {
	p := &plan{includeScripts: includeScripts, publicVars: map[string]string{}}
	root := s.Collection
	name := p.name(portability.KindCollection, "", root.Name)
	subject := portability.Subject{Kind: portability.KindCollection, Name: name}
	p.root.in = collection.Create{Name: name, Description: p.description(subject, root.Description)}
	p.root.in.AuthType, p.root.in.AuthData = p.folderAuth(subject, root.Auth)
	p.root.in.PreScript, p.root.in.PostScript = p.scriptsOf(name, root.Scripts)
	p.grpcMetadata = headerItems(root.GRPCMetadata)
	p.root.items = p.items(name, root.Items)

	if env := s.Environment; env != nil {
		ep := &envPlan{name: p.name(portability.KindEnvironment, "", env.Name)}
		for _, v := range env.Variables {
			if strings.TrimSpace(v.Key) == "" {
				p.warn(portability.WarnVariablesUnnamed, portability.Subject{Kind: portability.KindEnvironment, Name: ep.name}, "n", 1)
				continue
			}
			value := v.Value
			if v.Secret {
				value = ""
			} else {
				p.publicVars[v.Key] = v.Value
			}
			ep.vars = append(ep.vars, environment.AddVariable{Key: v.Key, Value: value, IsSecret: v.Secret})
		}
		p.env = ep
	}
	return p
}

func (p *plan) warn(code portability.WarningCode, subject portability.Subject, kv ...any) {
	p.warnings = append(p.warnings, portability.Warn(code, subject, kv...))
}

func (p *plan) name(kind, parentPath, raw string) string {
	switch {
	case strings.TrimSpace(raw) == "":
		if parentPath == "" {
			p.warn(portability.WarnUnnamedRenamed, portability.Subject{}, "kind", kind, "name", untitled)
		} else {
			p.warn(portability.WarnUnnamedRenamedIn, portability.Subject{}, "kind", kind, "parent", parentPath, "name", untitled)
		}
		return untitled
	case utf8.RuneCountInString(raw) > maxNameRunes:
		p.warn(portability.WarnNameTruncated, portability.Subject{Kind: kind, Name: string([]rune(raw)[:40]) + "…"}, "max", maxNameRunes)
		return string([]rune(raw)[:maxNameRunes])
	}
	return raw
}

func (p *plan) description(subject portability.Subject, s string) string {
	text, warnings := portability.ClampDescription(subject, s)
	p.warnings = append(p.warnings, warnings...)
	return text
}

// On a collection none already passes auth through, so null and inherit map to none.
func (p *plan) folderAuth(subject portability.Subject, a *publication.Auth) (entities.AuthType, string) {
	if a == nil {
		return entities.AuthTypeNone, "{}"
	}
	p.authURLs(a)
	switch t := entities.AuthType(a.Type); {
	case t == entities.AuthTypeInherit || t == entities.AuthTypeNone:
		return entities.AuthTypeNone, "{}"
	case !t.IsValid():
		p.warn(portability.WarnAuthUnsupported, subject, "type", a.Type)
		return entities.AuthTypeNone, "{}"
	default:
		return t, authData(a.Fields)
	}
}

func (p *plan) requestAuth(subject portability.Subject, a *publication.Auth) (entities.AuthType, string) {
	if a == nil {
		return entities.AuthTypeInherit, "{}"
	}
	p.authURLs(a)
	switch t := entities.AuthType(a.Type); {
	case t == entities.AuthTypeInherit || t == entities.AuthTypeNone:
		return t, "{}"
	case !t.IsValid():
		p.warn(portability.WarnAuthUnsupported, subject, "type", a.Type)
		return entities.AuthTypeNone, "{}"
	default:
		return t, authData(a.Fields)
	}
}

func (p *plan) authURLs(a *publication.Auth) {
	for _, key := range []string{"tokenUrl", "authUrl", "deviceAuthUrl"} {
		if v, ok := a.Fields[key].(string); ok && v != "" {
			p.urls = append(p.urls, v)
		}
	}
}

func authData(fields map[string]any) string {
	if len(fields) == 0 {
		return "{}"
	}
	raw, err := json.Marshal(fields)
	if err != nil {
		return "{}"
	}
	return string(raw)
}

func (p *plan) scriptsOf(path string, s *publication.Scripts) (string, string) {
	if s == nil {
		return "", ""
	}
	if strings.TrimSpace(s.Pre) != "" {
		p.scripts = append(p.scripts, portability.ScriptPreview{Path: path, Phase: "pre", Text: s.Pre})
	}
	if strings.TrimSpace(s.Post) != "" {
		p.scripts = append(p.scripts, portability.ScriptPreview{Path: path, Phase: "post", Text: s.Post})
	}
	if !p.includeScripts {
		return "", ""
	}
	return s.Pre, s.Post
}

func (p *plan) items(path string, items []publication.Item) []itemPlan {
	var out []itemPlan
	for _, it := range items {
		switch {
		case it.Folder != nil:
			f := it.Folder
			name := p.name(portability.KindFolder, path, f.Name)
			subject := portability.Subject{Kind: portability.KindFolder, Name: name}
			childPath := path + " / " + name
			fp := &folderPlan{in: collection.Create{Name: name, Description: p.description(subject, f.Description)}}
			fp.in.AuthType, fp.in.AuthData = p.folderAuth(subject, f.Auth)
			fp.in.PreScript, fp.in.PostScript = p.scriptsOf(childPath, f.Scripts)
			fp.items = p.items(childPath, f.Items)
			p.folders++
			out = append(out, itemPlan{folder: fp})
		case it.Request != nil:
			if rp := p.request(path, it.Request); rp != nil {
				p.requests++
				out = append(out, itemPlan{request: rp})
			}
		}
	}
	return out
}

func (p *plan) request(parentPath string, r *publication.Request) *requestPlan {
	name := p.name(portability.KindRequest, parentPath, r.Name)
	subject := portability.Subject{Kind: portability.KindRequest, Name: name}
	in := request.Create{
		Name: name, Description: p.description(subject, r.Description), Protocol: r.Protocol, BodyType: entities.BodyTypeNone,
	}
	skip := func(code portability.WarningCode, kv ...any) *requestPlan {
		p.warn(code, subject, kv...)
		return nil
	}

	switch r.Protocol {
	case entities.ProtocolHTTP:
		if r.HTTP == nil {
			return skip(portability.WarnSnapshotPartMissing, "part", "http")
		}
		in.Method = entities.HTTPMethod(r.HTTP.Method)
		if !in.Method.IsValid() {
			p.warn(portability.WarnSnapshotMethodUnsupported, subject, "method", r.HTTP.Method)
			in.Method = entities.MethodGET
		}
		in.URL, in.Headers = r.HTTP.URL, headerItems(r.HTTP.Headers)
		in.BodyType, in.Body = p.body(subject, r.HTTP.Body)
	case entities.ProtocolGraphQL:
		if r.GraphQL == nil {
			return skip(portability.WarnSnapshotPartMissing, "part", "graphql")
		}
		g := r.GraphQL
		in.Method, in.URL, in.Headers = entities.MethodPOST, g.URL, headerItems(g.Headers)
		in.GraphQLQuery, in.GraphQLVariables, in.GraphQLOperation = g.Query, g.Variables, g.OperationName
	case entities.ProtocolGRPC:
		if r.GRPC == nil {
			return skip(portability.WarnSnapshotPartMissing, "part", "grpc")
		}
		g := r.GRPC
		in.Method, in.URL, in.GRPCService, in.GRPCMethod = entities.MethodPOST, g.Target, g.Service, g.Method
		in.BodyType, in.Body = entities.BodyTypeJSON, g.Message
		in.GRPCMetadata = requestMetadata(g.Metadata, p.grpcMetadata)
	case entities.ProtocolWebSocket:
		if r.WebSocket == nil {
			return skip(portability.WarnSnapshotPartMissing, "part", "websocket")
		}
		in.Method, in.URL, in.Headers = entities.MethodGET, r.WebSocket.URL, headerItems(r.WebSocket.Headers)
		in.BodyType, in.Body = entities.BodyTypeRaw, p.wsSettings(subject, r.WebSocket)
	default:
		return skip(portability.WarnSnapshotProtocolUnsupported, "protocol", string(r.Protocol))
	}

	in.AuthType, in.AuthData = p.requestAuth(subject, r.Auth)
	p.urls = append(p.urls, in.URL)
	path := parentPath + " / " + name
	in.PreScript, in.PostScript = p.scriptsOf(path, r.Scripts)
	return &requestPlan{in: in, subject: subject, examples: p.examplesOf(subject, path, r.Protocol, r.Examples)}
}

type formField struct {
	Key     string `json:"key"`
	Value   string `json:"value"`
	Type    string `json:"type"`
	Enabled bool   `json:"enabled"`
}

// body never keeps a file path: a foreign path must not pick what we upload.
func (p *plan) body(subject portability.Subject, b publication.Body) (entities.BodyType, string) {
	switch t := entities.BodyType(b.Type); t {
	case entities.BodyTypeNone, "":
		return entities.BodyTypeNone, ""
	case entities.BodyTypeJSON, entities.BodyTypeXML, entities.BodyTypeRaw:
		return t, b.Raw
	case entities.BodyTypeForm:
		fields := make([]formField, 0, len(b.Fields))
		for _, f := range b.Fields {
			field := formField{Key: f.Key, Value: f.Value, Type: "text", Enabled: f.Enabled}
			if f.Type == "file" {
				if f.Value != "" {
					p.warn(portability.WarnFileFieldDropped, subject, "field", f.Key)
				}
				field.Type, field.Value = "file", ""
			}
			fields = append(fields, field)
		}
		raw, err := json.Marshal(fields)
		if err != nil {
			return entities.BodyTypeForm, "[]"
		}
		return entities.BodyTypeForm, string(raw)
	case entities.BodyTypeBinary:
		if b.FileName != "" {
			p.warn(portability.WarnFileBodyDropped, subject)
		}
		return entities.BodyTypeBinary, ""
	default:
		p.warn(portability.WarnSnapshotBodyUnsupported, subject, "type", b.Type)
		return entities.BodyTypeNone, ""
	}
}

type wsSettings struct {
	Version         int         `json:"version"`
	PingIntervalSec int         `json:"pingIntervalSec"`
	Subprotocols    []string    `json:"subprotocols"`
	Messages        []wsMessage `json:"messages"`
}

type wsMessage struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Format string `json:"format"`
	Data   string `json:"data"`
}

func (p *plan) wsSettings(subject portability.Subject, ws *publication.WSPart) string {
	doc := wsSettings{Version: 1, Subprotocols: append([]string{}, ws.Subprotocols...), Messages: []wsMessage{}}
	for _, m := range ws.Messages {
		switch m.Format {
		case "json", "text", "binary":
			doc.Messages = append(doc.Messages, wsMessage{ID: uuid.NewString(), Name: m.Name, Format: m.Format, Data: m.Data})
		default:
			p.warn(portability.WarnSnapshotWSMessageFormat, subject, "message", m.Name, "format", m.Format)
		}
	}
	raw, err := json.Marshal(doc)
	if err != nil {
		return ""
	}
	return string(raw)
}

func (p *plan) examplesOf(subject portability.Subject, path string, protocol entities.Protocol, list []publication.Example) []example.Create {
	if len(list) == 0 {
		return nil
	}
	if protocol == entities.ProtocolWebSocket {
		p.warn(portability.WarnSnapshotWSExamplesSkipped, subject, "n", len(list))
		return nil
	}
	var out []example.Create
	for _, e := range list {
		name := p.name(portability.KindExample, path, e.Name)
		if len(e.Body) > domain.MaxExampleBodyLen {
			p.warn(portability.WarnExampleTooLarge, subject, "example", name, "kb", domain.MaxExampleBodyLen/1024)
			continue
		}
		status := e.Status
		if status < 0 || status > 999 {
			p.warn(portability.WarnSnapshotExampleStatus, subject, "example", name, "status", status)
			status = 0
		}
		out = append(out, example.Create{
			Name: name, StatusCode: status, StatusText: e.StatusText, Headers: headerItems(e.Headers),
			Body: e.Body, ContentType: e.ContentType, Protocol: protocol,
		})
		p.examples++
	}
	return out
}

func headerItems(headers []publication.Header) []entities.HeaderItem {
	out := make([]entities.HeaderItem, 0, len(headers))
	for _, h := range headers {
		out = append(out, entities.HeaderItem{Key: h.Key, Value: h.Value, Enabled: h.Enabled})
	}
	return out
}

// requestMetadata undoes Build's merge: keys equal to the collection's stay inherited.
func requestMetadata(headers []publication.Header, root []entities.HeaderItem) map[string][]string {
	rootValues := map[string][]string{}
	for _, h := range root {
		if k := strings.ToLower(strings.TrimSpace(h.Key)); h.Enabled && k != "" {
			rootValues[k] = append(rootValues[k], h.Value)
		}
	}
	values := map[string][]string{}
	spelling := map[string]string{}
	for _, h := range headers {
		k := strings.ToLower(strings.TrimSpace(h.Key))
		if !h.Enabled || k == "" {
			continue
		}
		if _, ok := spelling[k]; !ok {
			spelling[k] = h.Key
		}
		values[k] = append(values[k], h.Value)
	}
	out := map[string][]string{}
	for k, vals := range values {
		if !slices.Equal(vals, rootValues[k]) {
			out[spelling[k]] = vals
		}
	}
	return out
}
