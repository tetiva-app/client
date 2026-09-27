package secrets

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/tetiva-app/client/internal/domain/entities"
)

func TestMaskQuery(t *testing.T) {
	const mask = "[m]"
	cases := []struct{ in, want string }{
		{"https://api.example.com/users", "https://api.example.com/users"},
		{"https://api.example.com/u?page=2", "https://api.example.com/u?page=2"},
		{"https://api.example.com/u?token=abc123", "https://api.example.com/u?token=[m]"},
		{"https://api.example.com/u?page=2&api_key=sk_1&sort=asc", "https://api.example.com/u?page=2&api_key=[m]&sort=asc"},
		{"https://api.example.com/u?signature=x#frag", "https://api.example.com/u?signature=[m]#frag"},
		{"https://api.example.com/u?token={{authToken}}", "https://api.example.com/u?token={{authToken}}"},
		{"https://api.example.com/u?key=", "https://api.example.com/u?key="},
		{"https://api.example.com/u?flag", "https://api.example.com/u?flag"},
		{"https://api.example.com/u?access%5Ftoken=a", "https://api.example.com/u?access%5Ftoken=[m]"},
		{"https://user:pw@api.example.com/u#access_token=a", "https://user:pw@api.example.com/u#access_token=a"},
	}
	for _, c := range cases {
		assert.Equal(t, c.want, MaskQuery(c.in, mask), "input %q", c.in)
	}
}

func TestRedactURL(t *testing.T) {
	cases := []struct{ in, want string }{
		{"https://api.example.com/u?page=2", "https://api.example.com/u?page=2"},
		{"/relative/path", "/relative/path"},
		{"", ""},
		{
			"https://bucket.s3.amazonaws.com/r.csv?X-Amz-Algorithm=AWS4-HMAC-SHA256&X-Amz-Credential=AKIAIOSFODNN7EXAMPLE%2F20260925&X-Amz-Signature=deadbeef",
			"https://bucket.s3.amazonaws.com/r.csv?X-Amz-Algorithm=AWS4-HMAC-SHA256&X-Amz-Credential=<redacted>&X-Amz-Signature=<redacted>",
		},
		{"https://app.example.com/cb?code=SPLxlOBeZQQ&state=xyz", "https://app.example.com/cb?code=<redacted>&state=xyz"},
		{
			"https://app.example.com/cb#access_token=eyJhbGciOi.payload.sig&token_type=bearer&state=xyz",
			"https://app.example.com/cb#access_token=<redacted>&token_type=<redacted>&state=xyz",
		},
		{"https://app.example.com/docs#section-2", "https://app.example.com/docs#section-2"},
		{"https://user:hunter2@git.example.com/repo.git", "https://<redacted>@git.example.com/repo.git"},
		{"https://ghp_abc@github.com/o/r", "https://<redacted>@github.com/o/r"},
		{"https://{{user}}:{{pass}}@host/p", "https://{{user}}:{{pass}}@host/p"},
		{"https://host/p?next=a@b", "https://host/p?next=a@b"},
		{"/cb?code=abc#id_token=x", "/cb?code=<redacted>#id_token=<redacted>"},
		{"https://host/p?token={{t}}#access_token={{t}}", "https://host/p?token={{t}}#access_token={{t}}"},
	}
	for _, c := range cases {
		assert.Equal(t, c.want, RedactURL(c.in), "input %q", c.in)
	}
}

func TestRedactHeaders_URLValues(t *testing.T) {
	in := []entities.HeaderItem{
		{Key: "Location", Value: "https://bucket.s3.example/r.csv?X-Amz-Credential=AKIA&X-Amz-Signature=sig", Enabled: true},
		{Key: "location", Value: "https://app.example.com/cb#access_token=eyJ.a.b", Enabled: true},
		{Key: "Content-Location", Value: "https://u:p@api.example.com/r?page=1", Enabled: true},
		{Key: "Link", Value: `<https://api.example.com/r?page=2&access_token=abc>; rel="next", <https://api.example.com/r?page=9>; rel="last"`, Enabled: true},
		{Key: "Refresh", Value: "5; url=https://app.example.com/cb?code=abc", Enabled: true},
		{Key: "Refresh", Value: `0;URL='https://app.example.com/?sig=abc'`, Enabled: true},
		{Key: "Location", Value: "/users/42", Enabled: true},
		{Key: "X-Next", Value: "https://api.example.com/r?token=abc", Enabled: true},
	}

	out := RedactHeaders(in)

	assert.Equal(t, []string{
		"https://bucket.s3.example/r.csv?X-Amz-Credential=<redacted>&X-Amz-Signature=<redacted>",
		"https://app.example.com/cb#access_token=<redacted>",
		"https://<redacted>@api.example.com/r?page=1",
		`<https://api.example.com/r?page=2&access_token=<redacted>>; rel="next", <https://api.example.com/r?page=9>; rel="last"`,
		"5; url=https://app.example.com/cb?code=<redacted>",
		`0;URL='https://app.example.com/?sig=<redacted>'`,
		"/users/42",
		"https://api.example.com/r?token=abc",
	}, headerValues(out))
	assert.Equal(t, "https://app.example.com/cb#access_token=eyJ.a.b", in[1].Value)
}

func TestRedactHeaders_Idempotent(t *testing.T) {
	in := []entities.HeaderItem{
		{Key: "Authorization", Value: "Bearer abc", Enabled: true},
		{Key: "Location", Value: "https://u:p@h/cb?code=a#access_token=b", Enabled: true},
		{Key: "Link", Value: `<https://h/r?token=a>; rel="next"`, Enabled: true},
		{Key: "Refresh", Value: "1; url=https://h/?sig=a", Enabled: true},
	}
	once := RedactHeaders(in)
	assert.Equal(t, once, RedactHeaders(once))
}

func headerValues(h []entities.HeaderItem) []string {
	out := make([]string, len(h))
	for i, item := range h {
		out[i] = item.Value
	}
	return out
}
