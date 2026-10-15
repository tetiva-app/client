package hoppscotch

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/tetiva-app/client/internal/domain/entities"
	"github.com/tetiva-app/client/internal/domain/usecase/auth"
)

func (b *builder) auth(a *hoppAuth, label string, isRequest bool) (entities.AuthType, string) {
	if a == nil {
		if isRequest {
			return entities.AuthTypeInherit, "{}"
		}
		return entities.AuthTypeNone, "{}"
	}
	if a.AuthActive != nil && !*a.AuthActive {
		return entities.AuthTypeNone, "{}"
	}

	switch a.AuthType {
	case "", "none":
		return entities.AuthTypeNone, "{}"
	case "inherit":
		return entities.AuthTypeInherit, "{}"
	case "basic":
		return entities.AuthTypeBasic, authData(map[string]any{"username": a.Username, "password": a.Password})
	case "bearer":
		return entities.AuthTypeBearer, authData(map[string]any{"token": a.Token})
	case "digest":
		return entities.AuthTypeDigest, authData(map[string]any{"username": a.Username, "password": a.Password})
	case "api-key":
		return entities.AuthTypeAPIKey, authData(map[string]any{"key": a.Key, "value": a.Value, "addTo": addTo(a.AddTo)})
	case "aws-signature":
		return entities.AuthTypeAWSSigV4, authData(map[string]any{
			"accessKeyId": a.AccessKey, "secretAccessKey": a.SecretKey, "region": a.Region,
			"service": a.ServiceName, "sessionToken": a.ServiceToken,
		})
	case "oauth-2":
		return entities.AuthTypeOAuth2, b.oauth2(a, label)
	case "jwt":
		return entities.AuthTypeJWT, b.jwt(a, label)
	default:
		b.warn(fmt.Sprintf("%s: auth type %q is not supported and was imported as no auth", label, a.AuthType))
		return entities.AuthTypeNone, "{}"
	}
}

func (b *builder) oauth2(a *hoppAuth, label string) string {
	data := map[string]any{"addTo": addTo(a.AddTo)}
	g := a.GrantTypeInfo
	if g == nil {
		return authData(data)
	}
	data["clientId"], data["clientSecret"], data["scope"] = g.ClientID, g.ClientSecret, g.Scopes
	// Hoppscotch's authEndpoint is the token URL for the grants that never open a browser.
	switch g.GrantType {
	case "AUTHORIZATION_CODE":
		data["grant"], data["authUrl"], data["tokenUrl"] = auth.GrantAuthorizationCode, g.AuthEndpoint, g.TokenEndpoint
	case "CLIENT_CREDENTIALS":
		data["grant"], data["tokenUrl"] = auth.GrantClientCredentials, g.AuthEndpoint
	case "PASSWORD":
		data["grant"], data["tokenUrl"] = auth.GrantPassword, g.AuthEndpoint
		data["username"], data["password"] = g.Username, g.Password
	default:
		data["grant"], data["authUrl"] = strings.ToLower(g.GrantType), g.AuthEndpoint
		b.warn(fmt.Sprintf("%s: OAuth 2.0 grant %q is not supported; pick one in the Auth tab", label, g.GrantType))
	}
	switch g.ClientAuthentication {
	case "AS_BASIC_AUTH_HEADERS":
		data["clientAuth"] = auth.ClientAuthBasic
	case "IN_BODY":
		data["clientAuth"] = auth.ClientAuthBody
	}
	return authData(data)
}

func (b *builder) jwt(a *hoppAuth, label string) string {
	data := map[string]any{
		"alg": a.Algorithm, "secret": a.Secret, "privateKey": a.PrivateKey,
		"headerPrefix": strings.TrimSpace(a.HeaderPrefix), "queryParam": a.ParamName, "addTo": addTo(a.AddTo),
	}
	if a.IsSecretBase64Encoded {
		data["secretBase64"] = "true"
	}
	for _, field := range []struct{ name, text, ours string }{{"payload", a.Payload, "claims"}, {"headers", a.JWTHeaders, "header"}} {
		if strings.TrimSpace(field.text) == "" {
			continue
		}
		var obj map[string]any
		if err := json.Unmarshal([]byte(field.text), &obj); err != nil || obj == nil {
			b.warn(fmt.Sprintf("%s: the JWT %s is not a JSON object and was dropped", label, field.name))
			continue
		}
		if len(obj) > 0 {
			data[field.ours] = obj
		}
	}
	return authData(data)
}

func addTo(v string) string {
	if v == "QUERY_PARAMS" || v == "Query params" {
		return "query"
	}
	return "header"
}

func authData(fields map[string]any) string {
	out := make(map[string]any, len(fields))
	for k, v := range fields {
		s, isString := v.(string)
		switch {
		case !isString:
			out[k] = v
		case s != "":
			out[k] = vars(s)
		}
	}
	raw, _ := json.Marshal(out)
	return string(raw)
}
