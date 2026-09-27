package publication

import (
	"slices"

	"github.com/tetiva-app/client/internal/domain/entities"
)

type authFields struct {
	public []string
	secret []string
}

// Only public fields are published as stored; secret lists feed the frontend fixture.
var authFieldTable = map[entities.AuthType]authFields{
	entities.AuthTypeNone:    {},
	entities.AuthTypeInherit: {},
	entities.AuthTypeBasic:   {secret: []string{"username", "password"}},
	entities.AuthTypeBearer:  {public: []string{"prefix"}, secret: []string{"token"}},
	entities.AuthTypeAPIKey:  {public: []string{"key", "addTo", "in"}, secret: []string{"value"}},
	entities.AuthTypeOAuth2: {
		public: []string{"grant", "tokenUrl", "authUrl", "deviceAuthUrl", "scope", "audience", "clientAuth", "addTo", "headerPrefix", "queryParam"},
		secret: []string{"clientId", "clientSecret", "username", "password", "redirectPort"},
	},
	entities.AuthTypeJWT: {
		public: []string{"alg", "addTo", "headerPrefix", "queryParam", "header", "claims"},
		secret: []string{"secret", "secretBase64", "privateKey", "expiresIn"},
	},
	entities.AuthTypeDigest:   {secret: []string{"username", "password"}},
	entities.AuthTypeAWSSigV4: {public: []string{"region", "service"}, secret: []string{"accessKeyId", "secretAccessKey", "sessionToken"}},
}

var authURLFields = map[string]bool{"tokenUrl": true, "authUrl": true, "deviceAuthUrl": true}

var authObjectFields = map[string]bool{"claims": true, "header": true}

func isPublicAuthField(t entities.AuthType, key string) bool {
	return slices.Contains(authFieldTable[t].public, key)
}
