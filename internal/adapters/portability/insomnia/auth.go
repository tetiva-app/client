package insomnia

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/tetiva-app/client/internal/domain/entities"
	"github.com/tetiva-app/client/internal/domain/usecase/auth"
)

const noPrefix = "NO_PREFIX"

var oauth2Grants = map[string]string{
	"client_credentials": auth.GrantClientCredentials,
	"password":           auth.GrantPassword,
	"authorization_code": auth.GrantAuthorizationCode,
}

func (p *plan) auth(label string, a *authBlock, request bool) (entities.AuthType, string) {
	if a == nil || a.Type == "" {
		if request {
			return entities.AuthTypeInherit, "{}"
		}
		return entities.AuthTypeNone, "{}"
	}
	if a.Disabled {
		p.warn(fmt.Sprintf("%s: %s auth is disabled in Insomnia and was imported as no auth", label, a.Type))
		return entities.AuthTypeNone, "{}"
	}

	c := p.tpl.conv
	switch a.Type {
	case "none":
		return entities.AuthTypeNone, "{}"
	case "basic":
		return entities.AuthTypeBasic, authJSON(map[string]any{"username": c(a.Username), "password": c(a.Password)})
	case "digest":
		return entities.AuthTypeDigest, authJSON(map[string]any{"username": c(a.Username), "password": c(a.Password)})
	case "bearer":
		data := map[string]any{"token": c(a.Token)}
		putIfSet(data, "prefix", c(strings.TrimSpace(a.Prefix)))
		return entities.AuthTypeBearer, authJSON(data)
	case "apikey":
		addTo := "header"
		switch a.AddTo {
		case "", "header":
		case "queryParams":
			addTo = "query"
		default:
			p.warn(fmt.Sprintf("%s: an API key sent as a %s is not supported and was imported as no auth", label, a.AddTo))
			return entities.AuthTypeNone, "{}"
		}
		return entities.AuthTypeAPIKey, authJSON(map[string]any{"key": c(a.Key), "value": c(a.Value), "addTo": addTo})
	case "iam":
		data := map[string]any{
			"accessKeyId":     c(a.AccessKeyID),
			"secretAccessKey": c(a.SecretAccessKey),
			"region":          c(a.Region),
			"service":         c(a.Service),
		}
		putIfSet(data, "sessionToken", c(a.SessionToken))
		return entities.AuthTypeAWSSigV4, authJSON(data)
	case "oauth2":
		return p.oauth2(label, a)
	default:
		p.warn(fmt.Sprintf("%s: auth type %q is not supported and was imported as no auth", label, a.Type))
		return entities.AuthTypeNone, "{}"
	}
}

func (p *plan) oauth2(label string, a *authBlock) (entities.AuthType, string) {
	c := p.tpl.conv
	data := map[string]any{}
	if a.GrantType != "" {
		grant, ok := oauth2Grants[a.GrantType]
		if !ok {
			grant = a.GrantType
			p.warn(fmt.Sprintf("%s: OAuth 2.0 grant %q is not supported; pick one in the Auth tab", label, a.GrantType))
		}
		data["grant"] = grant
	}
	putIfSet(data, "tokenUrl", c(a.AccessTokenURL))
	putIfSet(data, "authUrl", c(a.AuthorizationURL))
	putIfSet(data, "clientId", c(a.ClientID))
	putIfSet(data, "clientSecret", c(a.ClientSecret))
	putIfSet(data, "scope", c(a.Scope))
	putIfSet(data, "audience", c(a.Audience))
	putIfSet(data, "username", c(a.Username))
	putIfSet(data, "password", c(a.Password))
	if a.CredentialsInBody {
		data["clientAuth"] = auth.ClientAuthBody
	}
	if a.TokenPrefix == noPrefix {
		data["headerPrefix"] = ""
	} else {
		putIfSet(data, "headerPrefix", c(a.TokenPrefix))
	}
	for _, key := range []string{"tokenUrl", "authUrl"} {
		if v, ok := data[key].(string); ok {
			p.urls = append(p.urls, v)
		}
	}
	return entities.AuthTypeOAuth2, authJSON(data)
}

func putIfSet(data map[string]any, key, value string) {
	if value != "" {
		data[key] = value
	}
}

func authJSON(data map[string]any) string {
	raw, err := json.Marshal(data)
	if err != nil {
		return "{}"
	}
	return string(raw)
}
