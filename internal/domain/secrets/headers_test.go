package secrets

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/tetiva-app/client/internal/domain/entities"
)

func TestIsSensitiveHeader(t *testing.T) {
	sensitive := []string{
		"Authorization", "proxy-authorization", "Cookie", "Set-Cookie", "X-API-Key", "Api-Key",
		"X-Auth-Token", "X-Access-Token", "X-CSRF-Token", "X-Session-Token", "X-Amz-Security-Token",
		"X-Amz-Content-Sha256", "WWW-Authenticate",
		"apikey", "Private-Token", "X-Vault-Token", "X-Goog-Api-Key", "Ocp-Apim-Subscription-Key",
		"X-RapidAPI-Key", "X-Api-Token", "X-Shopify-Access-Token",
		"X-My-Secret", "X-Client-Key", "X-Session-Id", "X-Signature", "X-Password", "X-Passwd",
		"X-Credential", "X-Custom-Cookie", "X-Private-Thing", "X-Auth-User", " authorization ",
		"{{headerName}}", "X-{{env}}-Id", " {{h}} ",
		"Idempotency-Key", "X_Api_Key", "x.api.key", "XApiKey", "APIKey", "X-Tokens", "X-Credentials",
		"X-Original-Authorization", "X-Apikey", "X-Accesstoken", "X-Refreshtoken", "X-Clientsecret",
		"X-Jsessionid", "X-XSRF", "X-Csrftoken", "sessionID", "X-Authtoken", "X-Privatekey",
		"X-{{env}}-Access-Control",
		"X-Apitoken", "X-Accesskey", "X-Authkey", "X-Privkey", "X-Sessiontoken", "X-Idtoken", "X-Secrettoken",
		"X-Userpassword", "X-Clienttoken", "X-Securitytoken", "X-Hubsignature", "X-Sessioncookie",
		"X-Api-Key2", "Set-Cookie2", "Proxy-Authenticate",
	}
	for _, name := range sensitive {
		assert.True(t, IsSensitiveHeader(name), "header %q", name)
	}
	for _, name := range []string{"X-ApiToken", "X-AccessKey", "X-AuthKey", "X-SessionToken", "X-IdToken", "X-UserPassword", "X-BearerToken"} {
		assert.True(t, IsSensitiveHeader(http.CanonicalHeaderKey(name)), "header %q", http.CanonicalHeaderKey(name))
	}

	plain := []string{
		"", "Accept", "X-Request-Id", "Content-Type", "X-Amz-Date", "User-Agent", "X-Trace",
		"Keyword", "X-Author", "X-Authored-By", "X-Monkey", "X-Keyboard-Layout", "X-Tokenizer",
		"Access-Control-Allow-Credentials", "Access-Control-Allow-Headers", "access-control-expose-headers",
		"X-Passage", "X-Sessions-Total", "X-Api-Version2",
	}
	for _, name := range plain {
		assert.False(t, IsSensitiveHeader(name), "header %q", name)
	}
}

func TestIsSensitiveQueryParam(t *testing.T) {
	sensitive := []string{
		"token", "access_token", "refresh_token", "api_key", "apikey", "key", "secret", "password",
		"sig", "signature", "code", "code_verifier", "client_secret", "assertion", "id_token",
		"x-amz-signature", "x-amz-credential", "x-amz-security-token",
		"ACCESS_TOKEN", "access%5Ftoken", "%70assword", " Token ",
		"my_session", "private_data", "user_passwd", "X-Amz-Credential",
		"apiKey", "accessToken", "refreshToken", "clientSecret", "jsessionid", "_csrf", "csrfmiddlewaretoken",
		"xsrf_token", "oauth_token", "idToken", "session.id",
		"apitoken", "accesskey", "authkey", "sessiontoken", "idtoken", "newpassword", "oauth_verifier",
		"token2", "key1", "pagetoken",
	}
	for _, name := range sensitive {
		assert.True(t, IsSensitiveQueryParam(name), "param %q", name)
	}

	plain := []string{
		"", "page", "sort", "state", "flag", "X-Amz-Date", "100%off", "q", "limit",
		"keyword", "keywords", "author", "author_id", "authorName", "oauth_state", "monkey", "turkey", "hockey",
		"passage", "tokenizer", "page2", "v1", "donkey2",
	}
	for _, name := range plain {
		assert.False(t, IsSensitiveQueryParam(name), "param %q", name)
	}
}

func TestContainsSensitiveWord(t *testing.T) {
	sensitive := []string{
		"X-Oauth", "X-Authz", "X-Userauth", "X-Authcode", "X-Tokenid", "X-Secretid", "X-Sessionkey", "X-Apisecretid",
		"authcode", "sessionkey", "tokenid", "secretid", "oauth", "sessionToken", "apiKey", "clientSecret",
		"auth_token", "secretKey", "passkey", "privatedata", "mycookie", "x-signaturev2", "userpasswd", "credentialid",
		"auth%63ode", "pwd", "dbPwd", "X-Pwd", "pass", "user_pass", "pass2", "passphrase", "X-Key-Passphrase",
	}
	for _, name := range sensitive {
		assert.True(t, ContainsSensitiveWord(name), "name %q", name)
	}

	plain := []string{
		"", "page", "user", "id", "name", "passage", "author", "authorName", "author_id", "Authors", "X-Authored-By",
		"authority", "keyword", "keywords", "X-Keyboard-Layout", "monkey", "turkey2", "hockey", "donkey",
		"tokenizer", "secretary", "compass", "bypass", "passport", "passenger", "passthrough",
	}
	for _, name := range plain {
		assert.False(t, ContainsSensitiveWord(name), "name %q", name)
	}
}

func TestNamesSecretContainer(t *testing.T) {
	for _, name := range []string{
		"password", "passwd", "passphrase", "dbPwd", "token", "tokens", "sessionToken", "secret", "clientSecret",
		"credentials", "privateKey", "X-Secret2", "access%5Ftoken",
	} {
		assert.True(t, NamesSecretContainer(name), "name %q", name)
	}
	for _, name := range []string{
		"", "auth", "authorization", "key", "keys", "apiKey", "session", "signature", "cookie", "user",
		"tokenizer", "secretary", "author", "keyword",
	} {
		assert.False(t, NamesSecretContainer(name), "name %q", name)
	}
}

func TestRedactValue(t *testing.T) {
	cases := []struct{ in, want string }{
		{"", ""},
		{"ApiKey abc", "ApiKey <redacted>"},
		{"Api-Key {{k}}", "Api-Key {{k}}"},
		{"apikey", "apikey"},
		{"Bearer abc", "Bearer <redacted>"},
		{"Bearer {{t}}", "Bearer {{t}}"},
		{"bearer abc", "bearer <redacted>"},
		{"Basic dXNlcjpwYXNz", "Basic <redacted>"},
		{"Token abc", "Token <redacted>"},
		{`Digest username="u", nonce="n"`, "Digest <redacted>"},
		{"Bearer   abc", "Bearer   <redacted>"},
		{"Bearer ", "Bearer "},
		{"Bearer", "Bearer"},
		{"Bearer{{t}}", "Bearer{{t}}"},
		{"Bearer abc{{t}}", "Bearer <redacted>{{t}}"},
		{"{{a}}-xyz-{{b}}", "{{a}}<redacted>{{b}}"},
		{"{{token}}", "{{token}}"},
		{" {{session}} ", " {{session}} "},
		{"{{a}} {{b}} x", "{{a}} {{b}}<redacted>"},
		{"abc", "<redacted>"},
		{"session=abc; theme={{t}}", "<redacted>{{t}}"},
		{"Bearer {{a}}.{{b}}", "Bearer {{a}}<redacted>{{b}}"},
	}
	for _, c := range cases {
		assert.Equal(t, c.want, RedactValue(c.in), "input %q", c.in)
	}
}

func TestRedactHeaders(t *testing.T) {
	in := []entities.HeaderItem{
		{Key: "Authorization", Value: "Bearer abc", Enabled: true},
		{Key: "X-Token", Value: "Bearer {{token}}", Enabled: true},
		{Key: "{{h}}", Value: "literal-secret", Enabled: true},
		{Key: "{{h}}", Value: "{{v}}", Enabled: true},
		{Key: "Set-Cookie", Value: "id=1; Path=/", Enabled: false},
		{Key: "Content-Type", Value: "application/json", Enabled: true},
		{Key: "X-Request-Id", Value: "42", Enabled: false},
	}
	before := append([]entities.HeaderItem(nil), in...)

	out := RedactHeaders(in)

	assert.Equal(t, []entities.HeaderItem{
		{Key: "Authorization", Value: "Bearer <redacted>", Enabled: true},
		{Key: "X-Token", Value: "Bearer {{token}}", Enabled: true},
		{Key: "{{h}}", Value: "<redacted>", Enabled: true},
		{Key: "{{h}}", Value: "{{v}}", Enabled: true},
		{Key: "Set-Cookie", Value: "<redacted>", Enabled: false},
		{Key: "Content-Type", Value: "application/json", Enabled: true},
		{Key: "X-Request-Id", Value: "42", Enabled: false},
	}, out)
	assert.Equal(t, before, in)

	out[5].Value = "changed"
	assert.Equal(t, "application/json", in[5].Value)
	assert.Nil(t, RedactHeaders(nil))
}

func TestIsOrdinaryName(t *testing.T) {
	for _, name := range []string{
		"Idempotency-Key", "X-Idempotency-Key", "pageToken", "page_token", "next_token", "NextPageToken", "sort_key",
		"keyId", "key_id", "token_type", "token_endpoint", "authorization_endpoint", "page%5Ftoken",
	} {
		assert.True(t, IsOrdinaryName(name), "name %q", name)
	}
	for _, name := range []string{"", "endpoint", "key", "token", "X-Api-Key", "accessKeyId", "api_key_id", "page_secret", "X-Key", "x"} {
		assert.False(t, IsOrdinaryName(name), "name %q", name)
	}
}
