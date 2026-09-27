package publication

import (
	"fmt"
	"slices"
	"strconv"

	"github.com/google/uuid"

	"github.com/tetiva-app/client/internal/domain/entities"
	"github.com/tetiva-app/client/internal/domain/usecase/websocket"
)

// Hidden variable reasons.
const (
	HiddenSecret     = "secret"
	HiddenReferenced = "referenced"
	HiddenSuspicious = "suspicious"
)

// BuildInput is the collection as stored. Collections and Requests may hold more than the
// subtree of Root; deleted rows, drafts and anything outside the live tree are skipped.
type BuildInput struct {
	Root           *entities.Collection
	Collections    []*entities.Collection
	Requests       []*entities.Request
	Examples       map[uuid.UUID][]*entities.ResponseExample
	Environment    *entities.Environment
	Variables      []*entities.Variable
	IncludeScripts bool
	Generator      string
	Locale         string
	// PublishAsIs holds the selectors the author chose to publish unmasked.
	PublishAsIs []string
}

// Redaction.Path is for display only and is never parsed: "Folder A / Get user / headers / X-Api-Key".
type Redaction struct {
	Selector    string
	Path        string
	Category    string
	Reason      string
	Overridable bool
	Overridden  bool
}

// Warning is a scan hit; Excerpt is its first four characters and an ellipsis.
type Warning struct {
	Selector   string
	Path       string
	Rule       string
	Excerpt    string
	Overridden bool
}

type HiddenVar struct {
	VariableID  uuid.UUID
	Key         string
	Reason      string
	Selector    string
	Overridable bool
	Overridden  bool
}

// BlockingError is a problem the server would reject the snapshot for.
type BlockingError struct {
	Path    string
	Code    string
	Params  map[string]string
	Message string
}

type Report struct {
	Folders           int
	Requests          int
	Examples          int
	PublishedVars     []string
	HiddenVars        []HiddenVar
	Redactions        []Redaction
	Warnings          []Warning
	Errors            []BlockingError
	AcceptedOverrides []string
	IgnoredOverrides  []string
}

// Build returns an error only for input it cannot read (no root, unparsable stored JSON); what the
// server would refuse goes to Report.Errors.
func Build(in BuildInput) (*Snapshot, Report, error) {
	const funcName = "publication.Build"

	if in.Root == nil {
		return nil, Report{}, fmt.Errorf("%s: root collection is required", funcName)
	}
	overrides := make(map[string]bool, len(in.PublishAsIs))
	for _, s := range in.PublishAsIs {
		overrides[s] = true
	}
	t := newTree(in)
	vars := newVarState(t.variables, func(v *entities.Variable) string { return OpaqueID(in.Root.ID, v.ID) }, overrides)

	// Hiding a variable can leave a header name unresolved, which makes more places sensitive, so
	// passes repeat until the referenced set stops growing; it only grows, so this ends.
	referenced := map[string]bool{}
	var p *pass
	for {
		p = newPass(in, t, vars, referenced)
		if err := p.run(); err != nil {
			return nil, Report{}, fmt.Errorf("%s: %w", funcName, err)
		}
		if !vars.expand(referenced, p.refs) {
			break
		}
	}

	report := p.report
	warnings, scanAccepted := scan(p.snap, p.owners, overrides, p.public)
	report.Warnings = warnings
	report.Errors = append(report.Errors, checkLimits(p.snap, p.owners)...)
	report.AcceptedOverrides = sortedUnique(append(p.acceptedVars, scanAccepted...))
	var ignored []string
	for _, s := range in.PublishAsIs {
		if !slices.Contains(report.AcceptedOverrides, s) {
			ignored = append(ignored, s)
		}
	}
	report.IgnoredOverrides = sortedUnique(ignored)
	return p.snap, report, nil
}

// envOwners are the selector owners of the environment and its published rows, in snapshot order.
type envOwners struct {
	env  string
	vars []string
}

type pass struct {
	in         BuildInput
	tree       *tree
	vars       *varState
	referenced map[string]bool
	public     map[string]string

	snap         *Snapshot
	owners       envOwners
	refs         map[string]bool
	hostRefs     map[string]bool
	report       Report
	acceptedVars []string
	visited      map[uuid.UUID]bool
	redacted     map[string]bool
	fragments    map[string]bool
	items        int
}

func newPass(in BuildInput, t *tree, vars *varState, referenced map[string]bool) *pass {
	return &pass{
		in: in, tree: t, vars: vars, referenced: referenced, public: vars.publicValues(referenced),
		refs: map[string]bool{}, visited: map[uuid.UUID]bool{}, redacted: map[string]bool{}, hostRefs: map[string]bool{},
		fragments: map[string]bool{},
	}
}

func (p *pass) id(id uuid.UUID) string {
	return OpaqueID(p.in.Root.ID, id)
}

func (p *pass) run() error {
	root := p.in.Root
	p.visited[root.ID] = true
	folder, err := p.folder(root, nil, nil)
	if err != nil {
		return err
	}
	if p.items > maxItems {
		p.fail(root.Name, codeTooManyItems, countParams(p.items, maxItems))
	}
	owner := p.id(root.ID)
	p.snap = &Snapshot{
		Generator: p.in.Generator,
		Locale:    p.in.Locale,
		Collection: Root{
			Folder:       *folder,
			GRPCMetadata: p.headers(owner, root.Name, root.GRPCMetadata, CategoryMetadata),
		},
	}
	p.snap.Environment = p.environment()
	return nil
}

// folder walks c and its live descendants; names is the display path below the root.
func (p *pass) folder(c *entities.Collection, chain []*entities.Collection, names []string) (*Folder, error) {
	owner := p.id(c.ID)
	path := joinPath(names...)
	if len(names) == 0 {
		path = c.Name
	}
	auth, err := p.auth(owner, path, c.AuthType, c.AuthData, false)
	if err != nil {
		return nil, fmt.Errorf("collection %q: %w", c.Name, err)
	}
	p.bodySecret(c.Description)
	f := &Folder{
		ID: owner, Name: c.Name, Description: c.Description,
		Auth: auth, Scripts: p.scripts(owner, path, c.PreScript, c.PostScript),
	}

	chain = append(slices.Clip(chain), c)
	for _, child := range p.tree.children[c.ID] {
		if p.visited[child.ID] {
			continue
		}
		p.visited[child.ID] = true
		p.items++
		p.report.Folders++
		childNames := append(slices.Clip(names), child.Name)
		if len(childNames) == maxFolderDepth+1 {
			p.fail(joinPath(childNames...), codeFoldersTooDeep, limitParams(maxFolderDepth))
		}
		sub, err := p.folder(child, chain, childNames)
		if err != nil {
			return nil, err
		}
		f.Items = append(f.Items, Item{Folder: sub})
	}
	for _, r := range p.tree.requests[c.ID] {
		p.items++
		p.report.Requests++
		req, err := p.request(r, chain, append(slices.Clip(names), r.Name))
		if err != nil {
			return nil, err
		}
		f.Items = append(f.Items, Item{Request: req})
	}
	return f, nil
}

func (p *pass) request(r *entities.Request, chain []*entities.Collection, names []string) (*Request, error) {
	owner := p.id(r.ID)
	path := joinPath(names...)
	auth, err := p.auth(owner, path, r.AuthType, r.AuthData, true)
	if err != nil {
		return nil, fmt.Errorf("request %q: %w", r.Name, err)
	}
	p.bodySecret(r.Description)
	out := &Request{
		ID: owner, Name: r.Name, Description: r.Description, Protocol: r.Protocol,
		Auth: auth, Scripts: p.scripts(owner, path, r.PreScript, r.PostScript),
	}
	urlPath := joinPath(path, "url")

	switch r.Protocol {
	case entities.ProtocolHTTP:
		if !r.Method.IsValid() {
			p.fail(joinPath(path, "method"), codeMethodUnsupported, valueParams(string(r.Method)))
		}
		body, err := p.body(owner, path, r)
		if err != nil {
			return nil, fmt.Errorf("request %q: %w", r.Name, err)
		}
		out.HTTP = &HTTPPart{
			Method: string(r.Method), URL: p.url(owner, urlPath, r.URL, "0", "", true),
			Headers: p.headers(owner, path, r.Headers, CategoryHeader), Body: body,
		}
	case entities.ProtocolGraphQL:
		// Schema and proto paths are left out; a published variable in one would still show it.
		p.addRefs(r.GraphQLSchemaPath)
		p.bodyRefs(r.GraphQLVariables)
		p.graphQLSecret(r.GraphQLQuery)
		p.fragmentRefs(r.GraphQLQuery)
		out.GraphQL = &GraphQLPart{
			URL: p.url(owner, urlPath, r.URL, "0", "", true), Headers: p.headers(owner, path, r.Headers, CategoryHeader),
			Query: r.GraphQLQuery, Variables: r.GraphQLVariables, OperationName: r.GraphQLOperation,
		}
	case entities.ProtocolGRPC:
		p.addRefs(r.GRPCProtoPath)
		p.bodyRefs(r.Body)
		out.GRPC = &GRPCPart{
			Target: p.url(owner, urlPath, r.URL, "0", "", true), Service: r.GRPCService, Method: r.GRPCMethod, Message: r.Body,
			Metadata: p.headers(owner, path, effectiveMetadata(chain, r.GRPCMetadata), CategoryMetadata),
		}
	case entities.ProtocolWebSocket:
		settings := websocket.ParseSettings(r.Body)
		p.subprotocolRefs(settings.Subprotocols)
		ws := &WSPart{
			URL: p.url(owner, urlPath, r.URL, "0", "", true), Headers: p.headers(owner, path, r.Headers, CategoryHeader),
			Subprotocols: slices.Clone(settings.Subprotocols),
		}
		for _, m := range settings.Messages {
			if m.Format != "binary" {
				p.bodyRefs(m.Data)
			}
			ws.Messages = append(ws.Messages, WSMessage{Name: m.Name, Format: m.Format, Data: m.Data})
		}
		out.WebSocket = ws
	default:
		p.fail(joinPath(path, "protocol"), codeProtocolUnsupported, valueParams(string(r.Protocol)))
	}

	for _, e := range p.tree.examples[r.ID] {
		p.report.Examples++
		out.Examples = append(out.Examples, p.example(e, path))
	}
	return out, nil
}

func (p *pass) body(owner, path string, r *entities.Request) (Body, error) {
	switch r.BodyType {
	case entities.BodyTypeNone, "":
		return Body{Type: string(entities.BodyTypeNone)}, nil
	case entities.BodyTypeJSON:
		p.bodyRefs(r.Body)
		return Body{Type: string(r.BodyType), Raw: r.Body}, nil
	case entities.BodyTypeXML:
		p.bodyRefs(r.Body)
		return Body{Type: string(r.BodyType), Raw: r.Body}, nil
	case entities.BodyTypeRaw:
		p.bodyRefs(r.Body)
		raw := r.Body
		if formLike(raw) {
			n := 0
			raw = p.params(owner, joinPath(path, "body"), raw, "body.", CategoryForm, &n)
		}
		return Body{Type: string(r.BodyType), Raw: raw}, nil
	case entities.BodyTypeBinary:
		return Body{Type: string(r.BodyType), FileName: p.fileName(owner, joinPath(path, "body"), r.Body, "0")}, nil
	case entities.BodyTypeForm:
		fields, err := p.form(owner, path, r.Body)
		return Body{Type: string(r.BodyType), Fields: fields}, err
	default:
		p.fail(joinPath(path, "body"), codeBodyTypeUnsupported, valueParams(string(r.BodyType)))
		return Body{Type: string(r.BodyType)}, nil
	}
}

func (p *pass) example(e *entities.ResponseExample, requestPath string) Example {
	owner := p.id(e.ID)
	path := joinPath(requestPath, "examples", e.Name)
	if e.StatusCode < 0 || e.StatusCode > 999 {
		p.fail(joinPath(path, "status"), codeStatusOutOfRange, valueParams(strconv.Itoa(e.StatusCode)))
	}
	p.bodyRefs(e.Body)
	return Example{
		ID: owner, Name: e.Name, Status: e.StatusCode, StatusText: e.StatusText,
		Headers: p.headers(owner, path, e.Headers, CategoryHeader), Body: e.Body, ContentType: e.ContentType,
	}
}

func (p *pass) scripts(owner, path, pre, post string) *Scripts {
	if pre == "" && post == "" {
		return nil
	}
	if !p.in.IncludeScripts {
		p.redact(Redaction{Selector: selector(owner, CategoryScript, "0"), Path: joinPath(path, "scripts"), Category: CategoryScript, Reason: reasonScript})
		return nil
	}
	return &Scripts{Pre: pre, Post: post}
}

func (p *pass) environment() *Environment {
	env := p.in.Environment
	if env == nil {
		return nil
	}
	p.owners.env = p.id(env.ID)
	out := &Environment{Name: env.Name}
	p.hostVars()
	for i, row := range p.vars.rows {
		key := row.v.Key
		path := joinPath("Environment "+env.Name, key)
		v := Variable{Key: key, Secret: true}
		if p.vars.published(i, p.referenced) {
			v = Variable{Key: key, Value: p.url(row.owner, path, row.v.Value, "0", "", p.hostRefs[key])}
			p.report.PublishedVars = append(p.report.PublishedVars, key)
		}
		out.Variables = append(out.Variables, v)
		p.owners.vars = append(p.owners.vars, row.owner)

		reason := p.vars.hiddenReason(i, p.referenced)
		if reason == "" {
			continue
		}
		overridable := reason != HiddenSecret
		overridden := overridable && row.force
		p.report.HiddenVars = append(p.report.HiddenVars, HiddenVar{
			VariableID: row.v.ID, Key: key, Reason: reason, Selector: row.selector, Overridable: overridable, Overridden: overridden,
		})
		if reason == HiddenReferenced {
			p.redact(Redaction{Selector: row.selector, Path: path, Category: CategoryVar, Reason: reasonReferenced, Overridable: true, Overridden: overridden})
		}
		if overridden {
			p.acceptedVars = append(p.acceptedVars, row.selector)
		}
	}
	return out
}

func (p *pass) redact(r Redaction) {
	if p.redacted[r.Selector] {
		return
	}
	p.redacted[r.Selector] = true
	p.report.Redactions = append(p.report.Redactions, r)
}

func (p *pass) fail(path, code string, params map[string]string) {
	p.report.Errors = append(p.report.Errors, newBlockingError(path, code, params))
}

func (p *pass) addRefs(texts ...string) {
	for _, t := range texts {
		for _, name := range refNames(t) {
			p.refs[name] = true
		}
	}
}
