package snapshot

import (
	"encoding/json"
	"fmt"
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

// plan is the snapshot mapped onto usecase inputs, normalised to what the usecases accept. Preview and
// Import both build it, so the counts and warnings a user confirms are the ones the import produces.
type plan struct {
	includeScripts bool

	root         folderPlan
	grpcMetadata []entities.HeaderItem
	env          *envPlan

	folders, requests, examples int
	urls                        []string
	publicVars                  map[string]string
	scripts                     []portability.ScriptPreview
	warnings                    []string
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
	label    string
	examples []example.Create
}

type envPlan struct {
	name string
	vars []environment.AddVariable
}

func newPlan(s *publication.Snapshot, includeScripts bool) *plan {
	p := &plan{includeScripts: includeScripts, publicVars: map[string]string{}}
	root := s.Collection
	name := p.name("collection", "", root.Name)
	label := itemLabel("collection", name)
	p.root.in = collection.Create{Name: name, Description: p.description(label, root.Description)}
	p.root.in.AuthType, p.root.in.AuthData = p.folderAuth(label, root.Auth)
	p.root.in.PreScript, p.root.in.PostScript = p.scriptsOf(name, root.Scripts)
	p.grpcMetadata = headerItems(root.GRPCMetadata)
	p.root.items = p.items(name, root.Items)

	if env := s.Environment; env != nil {
		ep := &envPlan{name: p.name("environment", "", env.Name)}
		for _, v := range env.Variables {
			if strings.TrimSpace(v.Key) == "" {
				p.warn(fmt.Sprintf("environment %q: a variable without a name was skipped", ep.name))
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

func (p *plan) warn(msg string) {
	p.warnings = append(p.warnings, msg)
}

func itemLabel(kind, name string) string {
	return fmt.Sprintf("%s %q", kind, name)
}

// name keeps a name the usecases accept: not blank, at most 200 characters (the example limit).
func (p *plan) name(kind, parentPath, raw string) string {
	switch {
	case strings.TrimSpace(raw) == "":
		if parentPath == "" {
			p.warn(fmt.Sprintf("an unnamed %s was imported as %q", kind, untitled))
		} else {
			p.warn(fmt.Sprintf("an unnamed %s in %q was imported as %q", kind, parentPath, untitled))
		}
		return untitled
	case utf8.RuneCountInString(raw) > maxNameRunes:
		p.warn(fmt.Sprintf("%s %q: name longer than %d characters was truncated", kind, string([]rune(raw)[:40])+"…", maxNameRunes))
		return string([]rune(raw)[:maxNameRunes])
	}
	return raw
}

func (p *plan) description(label, s string) string {
	if len(s) <= domain.MaxDescriptionLen {
		return s
	}
	cut := domain.MaxDescriptionLen
	for cut > 0 && !utf8.RuneStart(s[cut]) {
		cut--
	}
	p.warn(fmt.Sprintf("%s: description longer than %d bytes was truncated", label, domain.MaxDescriptionLen))
	return s[:cut]
}

// folderAuth maps null, inherit and none alike to none: on a collection none already passes auth through.
func (p *plan) folderAuth(label string, a *publication.Auth) (entities.AuthType, string) {
	if a == nil {
		return entities.AuthTypeNone, "{}"
	}
	p.authURLs(a)
	switch t := entities.AuthType(a.Type); {
	case t == entities.AuthTypeInherit || t == entities.AuthTypeNone:
		return entities.AuthTypeNone, "{}"
	case !t.IsValid():
		p.warn(fmt.Sprintf("%s: auth type %q is not supported and was imported as no auth", label, a.Type))
		return entities.AuthTypeNone, "{}"
	default:
		return t, authData(a.Fields)
	}
}

func (p *plan) requestAuth(label string, a *publication.Auth) (entities.AuthType, string) {
	if a == nil {
		return entities.AuthTypeInherit, "{}"
	}
	p.authURLs(a)
	switch t := entities.AuthType(a.Type); {
	case t == entities.AuthTypeInherit || t == entities.AuthTypeNone:
		return t, "{}"
	case !t.IsValid():
		p.warn(fmt.Sprintf("%s: auth type %q is not supported and was imported as no auth", label, a.Type))
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

// scriptsOf lists every script for the preview but hands them to the import only when asked:
// a script from someone else's collection runs on Send and can rewrite environment variables.
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
			name := p.name("folder", path, f.Name)
			label := itemLabel("folder", name)
			childPath := path + " / " + name
			fp := &folderPlan{in: collection.Create{Name: name, Description: p.description(label, f.Description)}}
			fp.in.AuthType, fp.in.AuthData = p.folderAuth(label, f.Auth)
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
	name := p.name("request", parentPath, r.Name)
	label := itemLabel("request", name)
	in := request.Create{
		Name: name, Description: p.description(label, r.Description), Protocol: r.Protocol, BodyType: entities.BodyTypeNone,
	}
	skip := func(format string, args ...any) *requestPlan {
		p.warn(fmt.Sprintf("%s: "+format+", the request was skipped", append([]any{label}, args...)...))
		return nil
	}

	switch r.Protocol {
	case entities.ProtocolHTTP:
		if r.HTTP == nil {
			return skip("the http part is missing")
		}
		in.Method = entities.HTTPMethod(r.HTTP.Method)
		if !in.Method.IsValid() {
			p.warn(fmt.Sprintf("%s: method %q is not supported, imported as GET", label, r.HTTP.Method))
			in.Method = entities.MethodGET
		}
		in.URL, in.Headers = r.HTTP.URL, headerItems(r.HTTP.Headers)
		in.BodyType, in.Body = p.body(label, r.HTTP.Body)
	case entities.ProtocolGraphQL:
		if r.GraphQL == nil {
			return skip("the graphql part is missing")
		}
		g := r.GraphQL
		in.Method, in.URL, in.Headers = entities.MethodPOST, g.URL, headerItems(g.Headers)
		in.GraphQLQuery, in.GraphQLVariables, in.GraphQLOperation = g.Query, g.Variables, g.OperationName
	case entities.ProtocolGRPC:
		if r.GRPC == nil {
			return skip("the grpc part is missing")
		}
		g := r.GRPC
		in.Method, in.URL, in.GRPCService, in.GRPCMethod = entities.MethodPOST, g.Target, g.Service, g.Method
		in.BodyType, in.Body = entities.BodyTypeJSON, g.Message
		in.GRPCMetadata = requestMetadata(g.Metadata, p.grpcMetadata)
	case entities.ProtocolWebSocket:
		if r.WebSocket == nil {
			return skip("the websocket part is missing")
		}
		in.Method, in.URL, in.Headers = entities.MethodGET, r.WebSocket.URL, headerItems(r.WebSocket.Headers)
		in.BodyType, in.Body = entities.BodyTypeRaw, p.wsSettings(label, r.WebSocket)
	default:
		return skip("protocol %q is not supported", r.Protocol)
	}

	in.AuthType, in.AuthData = p.requestAuth(label, r.Auth)
	p.urls = append(p.urls, in.URL)
	path := parentPath + " / " + name
	in.PreScript, in.PostScript = p.scriptsOf(path, r.Scripts)
	return &requestPlan{in: in, label: label, examples: p.examplesOf(label, path, r.Protocol, r.Examples)}
}

// formField is the form body the request editor stores in Request.Body.
type formField struct {
	Key     string `json:"key"`
	Value   string `json:"value"`
	Type    string `json:"type"`
	Enabled bool   `json:"enabled"`
}

// body never keeps a file path: a path from someone else's machine must not pick what we upload.
func (p *plan) body(label string, b publication.Body) (entities.BodyType, string) {
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
					p.warn(fmt.Sprintf("%s: file field %q was imported without its file; pick it again", label, f.Key))
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
			p.warn(fmt.Sprintf("%s: the file body was imported without its file; pick it again", label))
		}
		return entities.BodyTypeBinary, ""
	default:
		p.warn(fmt.Sprintf("%s: body type %q is not supported, imported without a body", label, b.Type))
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

// wsSettings writes the version 1 document request.Create validates; snapshot messages carry no id.
func (p *plan) wsSettings(label string, ws *publication.WSPart) string {
	doc := wsSettings{Version: 1, Subprotocols: append([]string{}, ws.Subprotocols...), Messages: []wsMessage{}}
	for _, m := range ws.Messages {
		switch m.Format {
		case "json", "text", "binary":
			doc.Messages = append(doc.Messages, wsMessage{ID: uuid.NewString(), Name: m.Name, Format: m.Format, Data: m.Data})
		default:
			p.warn(fmt.Sprintf("%s: message %q has an unknown format %q and was skipped", label, m.Name, m.Format))
		}
	}
	raw, err := json.Marshal(doc)
	if err != nil {
		return ""
	}
	return string(raw)
}

func (p *plan) examplesOf(label, path string, protocol entities.Protocol, list []publication.Example) []example.Create {
	if len(list) == 0 {
		return nil
	}
	if protocol == entities.ProtocolWebSocket {
		p.warn(fmt.Sprintf("%s: %d example(s) skipped: WebSocket requests have no examples", label, len(list)))
		return nil
	}
	var out []example.Create
	for _, e := range list {
		name := p.name("example", path, e.Name)
		if len(e.Body) > domain.MaxExampleBodyLen {
			p.warn(fmt.Sprintf("%s: example %q skipped: body is larger than %d KB", label, name, domain.MaxExampleBodyLen/1024))
			continue
		}
		status := e.Status
		if status < 0 || status > 999 {
			p.warn(fmt.Sprintf("%s: example %q: status %d is outside 0–999, imported as 0", label, name, status))
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

// requestMetadata undoes the merge Build did: a key whose values are exactly the collection's is left
// to the collection, so editing it there still reaches the request.
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
