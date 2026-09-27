package request

import (
	"encoding/json"
	"maps"
	"slices"
	"strings"

	"github.com/golang-jwt/jwt/v5"

	"github.com/tetiva-app/client/internal/domain/entities"
	"github.com/tetiva-app/client/internal/domain/har"
	"github.com/tetiva-app/client/internal/domain/usecase/auth"
)

const (
	basicPlaceholderWarning = "Basic auth is not encoded because these variables are not substituted: "
	jwtPlaceholderWarning   = "JWT is not signed because these variables are not substituted: "
)

// oauth2PlacementFields are the only OAuth 2.0 fields a snippet prints; the rest key the token cache.
var oauth2PlacementFields = []string{"addTo", "queryParam", "headerPrefix"}

// prepareSnippetAuth protects the auth fields a snippet prints, which may only use the shown variables.
// Basic and JWT fields that still hold a reference become a placeholder: encoding or signing would bake {{name}} in.
func prepareSnippetAuth(ra ResolvedAuth, shown, all map[string]string, ph *har.Placeholders, keep func(string) bool) (ResolvedAuth, []string, error) {
	var placeholder func(auth.Fields) (entities.AuthType, auth.Fields, string)
	switch ra.Type {
	case entities.AuthTypeAPIKey, entities.AuthTypeBearer, entities.AuthTypeOAuth2:
	case entities.AuthTypeBasic:
		placeholder = basicPlaceholder
	case entities.AuthTypeJWT:
		placeholder = jwtPlaceholder
	default:
		// Digest and SigV4 are left out of snippets.
		return ra, nil, nil
	}
	f, err := auth.ParseFields(ra.Data)
	if err != nil {
		return ra, nil, err
	}

	var warnings []string
	if placeholder != nil {
		placeholderType, fields, warning := placeholder(auth.Substitute(f, shown))
		if warning == "" {
			return ra, nil, nil
		}
		ra.Type, f, warnings = placeholderType, fields, []string{warning}
		// Whatever reference is left in a placeholder has nothing to resolve to.
		keep = nil
	}

	printed := slices.Collect(maps.Keys(f))
	if ra.Type == entities.AuthTypeOAuth2 {
		printed = oauth2PlacementFields
	}
	for _, k := range printed {
		if s, ok := f[k].(string); ok {
			f[k] = ph.Protect(s, keep)
		}
	}
	if ra.Type == entities.AuthTypeOAuth2 {
		// The cached token was acquired with the config resolved from every variable, secrets included.
		f = auth.Substitute(f, all)
	}

	data, err := json.Marshal(f)
	if err != nil {
		return ra, nil, err
	}
	ra.Data = string(data)
	return ra, warnings, nil
}

// basicPlaceholder stands a readable Bearer-shaped header in for credentials that hold a reference.
func basicPlaceholder(f auth.Fields) (entities.AuthType, auth.Fields, string) {
	user, pass := f.Str("username"), f.Str("password")
	refs := references(user, pass)
	if len(refs) == 0 {
		return "", nil, ""
	}
	placeholder := auth.Fields{"prefix": "Basic", "token": "<base64 of " + user + ":" + pass + ">"}
	return entities.AuthTypeBearer, placeholder, basicPlaceholderWarning + strings.Join(refs, ", ")
}

// jwtPlaceholder puts a stand-in where applyJWT would put the signed token; it names only
// the references of the key, never the key itself.
func jwtPlaceholder(f auth.Fields) (entities.AuthType, auth.Fields, string) {
	keyFields, otherFields := jwtTokenFields(f)
	values := []any{f["alg"]}
	for _, k := range slices.Concat(keyFields, otherFields) {
		values = append(values, f[k])
	}
	refs := references(values...)
	if len(refs) == 0 {
		return "", nil, ""
	}
	warning := jwtPlaceholderWarning + strings.Join(refs, ", ")

	token := "<JWT>"
	var keys []any
	for _, k := range keyFields {
		keys = append(keys, f[k])
	}
	if keyRefs := references(keys...); len(keyRefs) > 0 {
		token = "<JWT signed with " + strings.Join(keyRefs, ", ") + ">"
	}

	if f.Str("addTo") == "query" {
		param := f.Str("queryParam")
		if param == "" {
			param = jwtDefaultQueryParam
		}
		return entities.AuthTypeAPIKey, auth.Fields{"key": param, "value": token, "addTo": "query"}, warning
	}
	placeholder := auth.Fields{"token": token}
	if _, ok := f["headerPrefix"]; ok {
		placeholder["prefix"] = f.Str("headerPrefix")
	}
	return entities.AuthTypeBearer, placeholder, warning
}

// jwtTokenFields splits the fields signJWT reads, besides alg, into the key and the rest;
// an algorithm that is still a reference could need either key.
func jwtTokenFields(f auth.Fields) (key, other []string) {
	alg := f.Str("alg")
	if alg == "" {
		alg = jwtDefaultAlg
	}
	other = []string{"expiresIn", "claims", "header"}
	switch jwt.GetSigningMethod(alg).(type) {
	case *jwt.SigningMethodHMAC:
		return []string{"secret"}, append(other, "secretBase64")
	case nil:
		return []string{"secret", "privateKey"}, append(other, "secretBase64")
	default:
		return []string{"privateKey"}, other
	}
}

// references lists the distinct {{name}} references in string leaves, in order; object keys are walked sorted.
func references(values ...any) []string {
	var refs []string
	var walk func(v any)
	walk = func(v any) {
		switch t := v.(type) {
		case string:
			for _, ref := range varPattern.FindAllString(t, -1) {
				if !slices.Contains(refs, ref) {
					refs = append(refs, ref)
				}
			}
		case map[string]any:
			keys := make([]string, 0, len(t))
			for k := range t {
				keys = append(keys, k)
			}
			slices.Sort(keys)
			for _, k := range keys {
				walk(t[k])
			}
		case []any:
			for _, sub := range t {
				walk(sub)
			}
		}
	}
	for _, v := range values {
		walk(v)
	}
	return refs
}
