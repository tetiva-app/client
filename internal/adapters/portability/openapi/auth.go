package openapi

import (
	"encoding/json"
	"slices"
	"strings"

	"github.com/getkin/kin-openapi/openapi3"

	"github.com/tetiva-app/client/internal/adapters/portability"
	"github.com/tetiva-app/client/internal/domain/entities"
	"github.com/tetiva-app/client/internal/domain/usecase/auth"
)

type authChoice struct {
	typ  entities.AuthType
	data string
	key  string
}

var noAuth = authChoice{typ: entities.AuthTypeNone, data: "{}"}

func (b *builder) security(reqs openapi3.SecurityRequirements) authChoice {
	for _, req := range reqs {
		if len(req) == 0 {
			continue
		}
		names := make([]string, 0, len(req))
		for name := range req {
			names = append(names, name)
		}
		slices.Sort(names)
		if len(names) > 1 {
			b.warn(portability.WarnOpenAPISchemesTogether, portability.Subject{}, "schemes", strings.Join(names, " + "), "scheme", names[0])
		}
		return b.scheme(names[0], req[names[0]])
	}
	return noAuth
}

func (b *builder) scheme(name string, scopes []string) authChoice {
	var ref *openapi3.SecuritySchemeRef
	if c := b.doc.Components; c != nil {
		ref = c.SecuritySchemes[name]
	}
	if ref == nil || ref.Value == nil {
		b.warn(portability.WarnOpenAPISchemeUndefined, portability.Subject{}, "scheme", name)
		return noAuth
	}
	s := ref.Value
	choice := func(t entities.AuthType, data map[string]any) authChoice {
		raw, _ := json.Marshal(data)
		return authChoice{typ: t, data: string(raw), key: name + " " + strings.Join(scopes, " ")}
	}
	switch strings.ToLower(s.Type) {
	case "http":
		switch strings.ToLower(s.Scheme) {
		case "bearer":
			b.addVar("token", "", true)
			return choice(entities.AuthTypeBearer, map[string]any{"token": "{{token}}"})
		case "basic", "digest":
			b.addVar("username", "", false)
			b.addVar("password", "", true)
			t := entities.AuthTypeBasic
			if strings.EqualFold(s.Scheme, "digest") {
				t = entities.AuthTypeDigest
			}
			return choice(t, map[string]any{"username": "{{username}}", "password": "{{password}}"})
		}
	case "apikey":
		if s.In == "header" && strings.EqualFold(s.Name, "Authorization") {
			b.addVar("apiKey", "", true)
			return choice(entities.AuthTypeBearer, map[string]any{"token": "{{apiKey}}", "prefix": ""})
		}
		if s.In == "header" || s.In == "query" {
			b.addVar("apiKey", "", true)
			return choice(entities.AuthTypeAPIKey, map[string]any{"key": s.Name, "value": "{{apiKey}}", "addTo": s.In})
		}
	case "oauth2":
		if data, ok := b.oauth2(name, s.Flows); ok {
			return choice(entities.AuthTypeOAuth2, withScope(data, scopes))
		}
	}
	kind := s.Type
	if s.Scheme != "" {
		kind += " " + s.Scheme
	}
	if s.In != "" {
		b.warn(portability.WarnOpenAPISchemeLocationUnsupported, portability.Subject{}, "scheme", name, "type", kind, "in", s.In)
	} else {
		b.warn(portability.WarnOpenAPISchemeUnsupported, portability.Subject{}, "scheme", name, "type", kind)
	}
	return noAuth
}

func (b *builder) oauth2(name string, flows *openapi3.OAuthFlows) (map[string]any, bool) {
	if flows == nil {
		return nil, false
	}
	data := map[string]any{"clientId": "{{clientId}}", "clientSecret": "{{clientSecret}}"}
	switch {
	case flows.ClientCredentials != nil:
		data["grant"] = auth.GrantClientCredentials
		data["tokenUrl"] = b.endpoint(flows.ClientCredentials.TokenURL)
	case flows.Password != nil:
		data["grant"] = auth.GrantPassword
		data["tokenUrl"] = b.endpoint(flows.Password.TokenURL)
		data["username"], data["password"] = "{{username}}", "{{password}}"
	case flows.AuthorizationCode != nil:
		data["grant"] = auth.GrantAuthorizationCode
		data["authUrl"] = b.endpoint(flows.AuthorizationCode.AuthorizationURL)
		data["tokenUrl"] = b.endpoint(flows.AuthorizationCode.TokenURL)
	case flows.Implicit != nil:
		data["grant"] = auth.GrantAuthorizationCode
		data["authUrl"] = b.endpoint(flows.Implicit.AuthorizationURL)
		b.warn(portability.WarnOpenAPIImplicitFlow, portability.Subject{}, "scheme", name)
	default:
		return nil, false
	}
	b.addVar("clientId", "", false)
	b.addVar("clientSecret", "", true)
	if data["grant"] == auth.GrantPassword {
		b.addVar("username", "", false)
		b.addVar("password", "", true)
	}
	return data, true
}

func (b *builder) endpoint(u string) string {
	if u == "" {
		return ""
	}
	if !strings.Contains(u, "://") {
		u = "{{baseUrl}}/" + strings.TrimPrefix(u, "/")
	}
	b.p.urls = append(b.p.urls, u)
	return u
}

func withScope(data map[string]any, scopes []string) map[string]any {
	if len(scopes) > 0 {
		data["scope"] = strings.Join(scopes, " ")
	}
	return data
}
