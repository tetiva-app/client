package insomnia

import (
	"bytes"
	"encoding/json"
	"fmt"
	"slices"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/tetiva-app/client/internal/adapters/portability"
)

const (
	typeWorkspace = "workspace"
	typeGroup     = "request_group"
	typeRequest   = "request"
	typeGRPC      = "grpc_request"
	typeWebSocket = "websocket_request"
	typePayload   = "websocket_payload"
	typeSocketIO  = "socketio_request"
	typeEnv       = "environment"
	typeCookieJar = "cookie_jar"
	typeUnitTest  = "unit_test"
	typeAPISpec   = "api_spec"
)

type resource struct {
	ID                  string      `json:"_id" yaml:"_id"`
	Type                string      `json:"_type" yaml:"_type"`
	ParentID            string      `json:"parentId" yaml:"parentId"`
	Name                string      `json:"name" yaml:"name"`
	Description         string      `json:"description" yaml:"description"`
	SortKey             float64     `json:"metaSortKey" yaml:"metaSortKey"`
	URL                 string      `json:"url" yaml:"url"`
	Method              string      `json:"method" yaml:"method"`
	Body                body        `json:"body" yaml:"body"`
	Parameters          []pair      `json:"parameters" yaml:"parameters"`
	PathParameters      []pair      `json:"pathParameters" yaml:"pathParameters"`
	Headers             []pair      `json:"headers" yaml:"headers"`
	Metadata            []pair      `json:"metadata" yaml:"metadata"`
	Authentication      *authBlock  `json:"authentication" yaml:"authentication"`
	PreRequestScript    string      `json:"preRequestScript" yaml:"preRequestScript"`
	AfterResponseScript string      `json:"afterResponseScript" yaml:"afterResponseScript"`
	ProtoMethodName     string      `json:"protoMethodName" yaml:"protoMethodName"`
	Environment         envData     `json:"environment" yaml:"environment"`
	Data                envData     `json:"data" yaml:"data"`
	Cookies             []any       `json:"cookies" yaml:"cookies"`
	Value               string      `json:"value" yaml:"value"`
	Mode                string      `json:"mode" yaml:"mode"`
	Meta                *meta       `json:"-" yaml:"meta"`
	Scripts             *scripts    `json:"-" yaml:"scripts"`
	ReflectionAPI       *yaml.Node  `json:"-" yaml:"reflectionApi"`
	Children            []*resource `json:"-" yaml:"children"`
}

type body struct {
	MimeType string `json:"mimeType" yaml:"mimeType"`
	Text     string `json:"text" yaml:"text"`
	FileName string `json:"fileName" yaml:"fileName"`
	Params   []pair `json:"params" yaml:"params"`
}

type pair struct {
	Name     string `json:"name" yaml:"name"`
	Value    string `json:"value" yaml:"value"`
	Disabled bool   `json:"disabled" yaml:"disabled"`
	Type     string `json:"type" yaml:"type"`
	FileName string `json:"fileName" yaml:"fileName"`
}

type authBlock struct {
	Type              string `json:"type" yaml:"type"`
	Disabled          bool   `json:"disabled" yaml:"disabled"`
	Username          string `json:"username" yaml:"username"`
	Password          string `json:"password" yaml:"password"`
	Token             string `json:"token" yaml:"token"`
	Prefix            string `json:"prefix" yaml:"prefix"`
	Key               string `json:"key" yaml:"key"`
	Value             string `json:"value" yaml:"value"`
	AddTo             string `json:"addTo" yaml:"addTo"`
	GrantType         string `json:"grantType" yaml:"grantType"`
	AccessTokenURL    string `json:"accessTokenUrl" yaml:"accessTokenUrl"`
	AuthorizationURL  string `json:"authorizationUrl" yaml:"authorizationUrl"`
	ClientID          string `json:"clientId" yaml:"clientId"`
	ClientSecret      string `json:"clientSecret" yaml:"clientSecret"`
	Audience          string `json:"audience" yaml:"audience"`
	Scope             string `json:"scope" yaml:"scope"`
	TokenPrefix       string `json:"tokenPrefix" yaml:"tokenPrefix"`
	CredentialsInBody bool   `json:"credentialsInBody" yaml:"credentialsInBody"`
	AccessKeyID       string `json:"accessKeyId" yaml:"accessKeyId"`
	SecretAccessKey   string `json:"secretAccessKey" yaml:"secretAccessKey"`
	SessionToken      string `json:"sessionToken" yaml:"sessionToken"`
	Region            string `json:"region" yaml:"region"`
	Service           string `json:"service" yaml:"service"`
}

type meta struct {
	ID          string  `yaml:"id"`
	Description string  `yaml:"description"`
	SortKey     float64 `yaml:"sortKey"`
}

type scripts struct {
	PreRequest    string `yaml:"preRequest"`
	AfterResponse string `yaml:"afterResponse"`
}

type exportV4 struct {
	Resources []v4Resource `json:"resources" yaml:"resources"`
}

type v4Resource struct{ *resource }

func (r *v4Resource) UnmarshalJSON(b []byte) error {
	return r.decode(func(v any) error { return json.Unmarshal(b, v) })
}

func (r *v4Resource) UnmarshalYAML(n *yaml.Node) error {
	return r.decode(n.Decode)
}

func (r *v4Resource) decode(into func(any) error) error {
	var head struct {
		Type string `json:"_type" yaml:"_type"`
	}
	if err := into(&head); err != nil {
		return err
	}
	r.resource = &resource{Type: head.Type}
	switch head.Type {
	case typeWorkspace, typeGroup, typeRequest, typeGRPC, typeWebSocket, typePayload, typeEnv, typeCookieJar:
		return into(r.resource)
	}
	return nil
}

type fileV5 struct {
	Type         string      `yaml:"type"`
	Name         string      `yaml:"name"`
	Meta         *meta       `yaml:"meta"`
	Collection   []*resource `yaml:"collection"`
	Environments *envV5      `yaml:"environments"`
	CookieJar    *struct {
		Cookies []any `yaml:"cookies"`
	} `yaml:"cookieJar"`
	TestSuites []struct {
		Tests []any `yaml:"tests"`
	} `yaml:"testSuites"`
	Spec *yaml.Node `yaml:"spec"`
}

type envV5 struct {
	Name            string  `yaml:"name"`
	Meta            *meta   `yaml:"meta"`
	Data            envData `yaml:"data"`
	SubEnvironments []envV5 `yaml:"subEnvironments"`
}

type envData []envVar

type envVar struct {
	key, value string
}

const (
	maxEnvVars  = 10000
	maxEnvDepth = 32
)

func (d *envData) UnmarshalJSON(b []byte) error {
	n, err := portability.JSONNode(b)
	if err != nil {
		return err
	}
	*d = nil
	return d.flatten("", n, 0)
}

// UnmarshalYAML decodes once just for yaml.v3's alias cycle and expansion checks.
func (d *envData) UnmarshalYAML(n *yaml.Node) error {
	var checked any
	if err := n.Decode(&checked); err != nil {
		return err
	}
	*d = nil
	return d.flatten("", n, 0)
}

func (d *envData) flatten(prefix string, n *yaml.Node, depth int) error {
	if depth > maxEnvDepth {
		return fmt.Errorf("the environment data nests deeper than %d levels", maxEnvDepth)
	}
	if n.Kind == yaml.AliasNode {
		n = n.Alias
	}
	if n.Kind != yaml.MappingNode {
		return nil
	}
	for i := 0; i+1 < len(n.Content); i += 2 {
		if len(*d) >= maxEnvVars {
			return fmt.Errorf("the environment has more than %d variables", maxEnvVars)
		}
		key, v := prefix+n.Content[i].Value, n.Content[i+1]
		if v.Kind == yaml.AliasNode {
			v = v.Alias
		}
		switch v.Kind {
		case yaml.MappingNode:
			if err := d.flatten(key+".", v, depth+1); err != nil {
				return err
			}
		case yaml.ScalarNode:
			value := v.Value
			if v.Tag == "!!null" {
				value = ""
			}
			*d = append(*d, envVar{key: key, value: value})
		default:
			var x any
			if err := v.Decode(&x); err != nil {
				return err
			}
			raw, err := json.Marshal(x)
			if err != nil {
				return err
			}
			*d = append(*d, envVar{key: key, value: string(raw)})
		}
	}
	return nil
}

type workspace struct {
	name, description string
	items             []*resource
	base              *resource
	subs              []*resource
}

type export struct {
	workspaces                           []*workspace
	cookies, unitTests, specs, socketIOs int
}

func isV5(data []byte) bool {
	t := header(data, 1)["type"]
	return t == "collection.insomnia.rest/5.0" || t == "spec.insomnia.rest/5.0"
}

func isV4(data []byte) bool {
	trimmed := trimBOM(data)
	if bytes.HasPrefix(trimmed, []byte("{")) {
		var probe struct {
			Type   string          `json:"_type"`
			Format json.RawMessage `json:"__export_format"`
		}
		return json.Unmarshal(trimmed, &probe) == nil && probe.Type == "export" && string(probe.Format) == "4"
	}
	h := header(trimmed, 3)
	return h["_type"] == "export" && h["__export_format"] == "4"
}

func header(data []byte, n int) map[string]string {
	out := map[string]string{}
	rest := trimBOM(data)
	for len(rest) > 0 && len(out) < n {
		var line []byte
		line, rest, _ = bytes.Cut(rest, []byte("\n"))
		s := strings.TrimRight(string(line), " \t\r")
		if s == "" || s == "---" || s[0] == '#' || s[0] == ' ' || s[0] == '\t' {
			continue
		}
		key, value, ok := strings.Cut(s, ":")
		if !ok {
			break
		}
		out[key] = strings.Trim(strings.TrimSpace(value), `"'`)
	}
	return out
}

func trimBOM(data []byte) []byte {
	return bytes.TrimLeft(bytes.TrimPrefix(data, []byte("\xef\xbb\xbf")), " \t\r\n")
}

func parse(data []byte) (*export, error) {
	if isV5(data) {
		var f fileV5
		if err := yaml.Unmarshal(data, &f); err != nil {
			return nil, err
		}
		return fromV5(&f), nil
	}
	var e exportV4
	trimmed := trimBOM(data)
	var err error
	if bytes.HasPrefix(trimmed, []byte("{")) {
		err = json.Unmarshal(trimmed, &e)
	} else {
		err = yaml.Unmarshal(trimmed, &e)
	}
	if err != nil {
		return nil, err
	}
	resources := make([]*resource, 0, len(e.Resources))
	for _, r := range e.Resources {
		resources = append(resources, r.resource)
	}
	return fromV4(resources), nil
}

func fromV5(f *fileV5) *export {
	ex := &export{}
	w := &workspace{name: f.Name}
	if f.Meta != nil {
		w.description = f.Meta.Description
	}
	w.items = v5Items(f.Collection, ex)
	if env := f.Environments; env != nil {
		w.base = &resource{Name: env.Name, Data: env.Data}
		for _, s := range env.SubEnvironments {
			sub := &resource{Name: s.Name, Data: s.Data}
			if s.Meta != nil {
				sub.SortKey = s.Meta.SortKey
			}
			w.subs = append(w.subs, sub)
		}
		sortByKey(w.subs)
	}
	if f.CookieJar != nil {
		ex.cookies = len(f.CookieJar.Cookies)
	}
	for _, s := range f.TestSuites {
		ex.unitTests += len(s.Tests)
	}
	if f.Spec != nil {
		ex.specs = 1
	}
	ex.workspaces = []*workspace{w}
	return ex
}

func v5Items(items []*resource, ex *export) []*resource {
	out := make([]*resource, 0, len(items))
	for _, it := range items {
		if it == nil {
			continue
		}
		if it.Meta != nil {
			it.ID, it.Description, it.SortKey = it.Meta.ID, it.Meta.Description, it.Meta.SortKey
		}
		if it.Scripts != nil {
			it.PreRequestScript, it.AfterResponseScript = it.Scripts.PreRequest, it.Scripts.AfterResponse
		}
		switch {
		case len(it.Children) > 0:
			it.Type = typeGroup
		case it.Method != "":
			it.Type = typeRequest
		case it.ReflectionAPI != nil || it.ProtoMethodName != "":
			it.Type = typeGRPC
		case strings.HasPrefix(it.ID, "socketio-req"):
			ex.socketIOs++
			continue
		case it.URL != "" || strings.HasPrefix(it.ID, "ws-req"):
			it.Type = typeWebSocket
		default:
			it.Type = typeGroup
		}
		it.Children = v5Items(it.Children, ex)
		out = append(out, it)
	}
	sortByKey(out)
	return out
}

func fromV4(resources []*resource) *export {
	ex := &export{}
	children := map[string][]*resource{}
	var workspaces []*resource
	for _, r := range resources {
		if r == nil {
			continue
		}
		switch r.Type {
		case typeWorkspace:
			workspaces = append(workspaces, r)
			continue
		case typeCookieJar:
			ex.cookies += len(r.Cookies)
		case typeUnitTest:
			ex.unitTests++
		case typeAPISpec:
			ex.specs++
		case typeSocketIO:
			ex.socketIOs++
		}
		children[r.ParentID] = append(children[r.ParentID], r)
	}

	seen := map[string]bool{}
	var tree func(parentID string) []*resource
	tree = func(parentID string) []*resource {
		var out []*resource
		for _, c := range children[parentID] {
			switch c.Type {
			case typeGroup, typeRequest, typeGRPC, typeWebSocket, typePayload:
			default:
				continue
			}
			if seen[c.ID] {
				continue
			}
			seen[c.ID] = true
			c.Children = tree(c.ID)
			out = append(out, c)
		}
		sortByKey(out)
		return out
	}

	for _, wr := range workspaces {
		w := &workspace{name: wr.Name, description: wr.Description}
		for _, c := range children[wr.ID] {
			if c.Type == typeEnv && w.base == nil {
				w.base = c
				for _, s := range children[c.ID] {
					if s.Type == typeEnv {
						w.subs = append(w.subs, s)
					}
				}
				sortByKey(w.subs)
			}
		}
		for _, it := range tree(wr.ID) {
			if it.Type != typePayload {
				w.items = append(w.items, it)
			}
		}
		ex.workspaces = append(ex.workspaces, w)
	}
	return ex
}

func sortByKey(rs []*resource) {
	slices.SortStableFunc(rs, func(a, b *resource) int {
		switch {
		case a.SortKey < b.SortKey:
			return -1
		case a.SortKey > b.SortKey:
			return 1
		}
		return 0
	})
}
