package har_test

import (
	"net/url"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tetiva-app/client/internal/domain/har"
)

func TestPlaceholders(t *testing.T) {
	t.Run("protect_restore_roundtrip", func(t *testing.T) {
		texts := []string{
			"{{baseUrl}}/users/{{id}}?q={{q}}",
			"Bearer {{token}}",
			`{"a":"{{v}}","b":"{{id}}"}`,
		}
		p := har.NewPlaceholders(texts...)
		for _, s := range texts {
			protected := p.Protect(s, nil)
			assert.NotContains(t, protected, "{{")
			assert.Equal(t, s, p.Restore(protected))
		}
	})

	t.Run("keep_skips_resolvable", func(t *testing.T) {
		s := "https://{{host}}/users/{{id}}"
		p := har.NewPlaceholders(s)
		protected := p.Protect(s, func(name string) bool { return name == "host" })
		assert.True(t, strings.HasPrefix(protected, "https://{{host}}/users/"))
		assert.NotContains(t, protected, "{{id}}")
		assert.Equal(t, s, p.Restore(protected))
	})

	t.Run("port_gets_numeric_token", func(t *testing.T) {
		s := "https://{{h}}:{{p}}/a"
		p := har.NewPlaceholders(s)
		u, err := url.Parse(p.Protect(s, nil))
		require.NoError(t, err)
		assert.Equal(t, "59990", u.Port())
		assert.Equal(t, s, p.Restore(u.String()))
		assert.Equal(t, `{"id":59990}`, p.Restore(`{"id":59990}`))
	})

	t.Run("reference_right_after_port", func(t *testing.T) {
		withQuery := func(t *testing.T, protected string) string {
			t.Helper()
			u, err := url.Parse(protected)
			require.NoError(t, err)
			q := u.Query()
			q.Set("api_key", "k")
			u.RawQuery = q.Encode()
			return u.String()
		}
		resolved := map[string]string{"basePath": "/v1", "port": "8443"}
		keep := func(names ...string) func(string) bool {
			return func(name string) bool { return slices.Contains(names, name) }
		}
		substitute := func(s string) string {
			for k, v := range resolved {
				s = strings.ReplaceAll(s, "{{"+k+"}}", v)
			}
			return s
		}
		cases := []struct {
			name, in string
			keep     []string
			want     string
		}{
			{"all_protected", "https://{{host}}:{{port}}{{basePath}}/u", nil, "https://{{host}}:{{port}}{{basePath}}/u?api_key=k"},
			{"path_resolved", "https://{{host}}:{{port}}{{basePath}}/u", []string{"basePath"}, "https://{{host}}:{{port}}/v1/u?api_key=k"},
			{"port_resolved", "https://{{host}}:{{port}}{{basePath}}/u", []string{"port"}, "https://{{host}}:8443{{basePath}}/u?api_key=k"},
			{"literal_port", "http://localhost:8080{{basePath}}/u", nil, "http://localhost:8080{{basePath}}/u?api_key=k"},
			{"path_only", "http://localhost:8080{{basePath}}", nil, "http://localhost:8080{{basePath}}?api_key=k"},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				p := har.NewPlaceholders(tc.in)
				protected := substitute(p.Protect(tc.in, keep(tc.keep...)))

				assert.Equal(t, tc.want, p.Restore(withQuery(t, protected)))
			})
		}
	})

	t.Run("reference_after_port_of_a_nested_url", func(t *testing.T) {
		s := "https://api.io/x?next=http://h:8080{{p}}"
		p := har.NewPlaceholders(s)
		u, err := url.Parse(p.Protect(s, nil))
		require.NoError(t, err)
		q := u.Query()
		q.Set("api_key", "k")
		u.RawQuery = q.Encode()

		got, err := url.QueryUnescape(p.Restore(u.RawQuery))
		require.NoError(t, err)
		assert.Equal(t, "api_key=k&next=http://h:8080{{p}}", got)
	})

	t.Run("prefix_collision", func(t *testing.T) {
		s := "tetivaph0x/{{a}}"
		p := har.NewPlaceholders(s)
		protected := p.Protect(s, nil)
		assert.True(t, strings.HasPrefix(protected, "tetivaph0x/"))
		assert.NotContains(t, protected, "{{")
		assert.Equal(t, s, p.Restore(protected))
		assert.Equal(t, "tetivaph0x", p.Restore("tetivaph0x"))
	})

	t.Run("prefix_outgrows_longest_z_run", func(t *testing.T) {
		p := har.NewPlaceholders("tetivaphz", "TETIVAPHZZZ tetivaph", "tetivaphtetivaphzz")
		assert.Equal(t, "tetivaphzzzz0x", p.Protect("{{a}}", nil))
	})

	t.Run("prefix_linear_in_z_run", func(t *testing.T) {
		run := 1 << 20
		start := time.Now()
		p := har.NewPlaceholders("tetivaph" + strings.Repeat("z", run))
		elapsed := time.Since(start)

		tok := p.Protect("{{a}}", nil)
		assert.Equal(t, "tetivaph"+strings.Repeat("z", run+1)+"0x", tok)
		assert.Less(t, elapsed, 200*time.Millisecond)
		assert.Equal(t, "https://{{a}}/", p.Restore("https://"+tok+"/"))
	})

	t.Run("unsafe_names_survive_url_parse", func(t *testing.T) {
		s := "https://api.io/{{x'; printf injected; #}}/{{a b}}?home={{$HOME}}&n={{a b}}"
		p := har.NewPlaceholders(s)
		u, err := url.Parse(p.Protect(s, nil))
		require.NoError(t, err)
		assert.Equal(t, s, p.Restore(u.String()))

		q := u.Query()
		q.Set("api_key", "k")
		u.RawQuery = q.Encode()
		assert.Equal(t,
			"https://api.io/{{x'; printf injected; #}}/{{a b}}?api_key=k&home={{$HOME}}&n={{a b}}",
			p.Restore(u.String()))
	})

	t.Run("encoded_braces_restored", func(t *testing.T) {
		p := har.NewPlaceholders("{{baseUrl}}/u", "{{a b}}")
		p.Protect("{{baseUrl}}/u", nil)
		p.Protect("{{a b}}", nil)
		assert.Equal(t, "{{baseUrl}}/u", p.Restore("%7b%7bbaseUrl%7d%7d/u"))
		assert.Equal(t, "/{{a b}}", p.Restore("/%7B%7Ba%20b%7D%7D"))
		assert.Equal(t, "%7B%7Bother%7D%7D", p.Restore("%7B%7Bother%7D%7D"))
	})

	t.Run("restore_request_all_fields", func(t *testing.T) {
		p := har.NewPlaceholders("{{v}}")
		tok := p.Protect("{{v}}", nil)
		r := har.Request{
			Method:      tok,
			URL:         "https://" + tok + "/a",
			HTTPVersion: "HTTP/1.1",
			Headers:     []har.NameValue{{Name: tok, Value: tok}},
			QueryString: []har.NameValue{{Name: tok, Value: tok}},
			PostData: &har.PostData{
				MimeType: tok,
				Text:     tok,
				Params:   []har.Param{{Name: tok, Value: tok, FileName: tok, ContentType: tok}},
			},
			BinaryFile: tok,
			AuthNote:   tok,
		}
		p.RestoreRequest(&r)
		assert.Equal(t, har.Request{
			Method:      "{{v}}",
			URL:         "https://{{v}}/a",
			HTTPVersion: "HTTP/1.1",
			Headers:     []har.NameValue{{Name: "{{v}}", Value: "{{v}}"}},
			QueryString: []har.NameValue{{Name: "{{v}}", Value: "{{v}}"}},
			PostData: &har.PostData{
				MimeType: "{{v}}",
				Text:     "{{v}}",
				Params:   []har.Param{{Name: "{{v}}", Value: "{{v}}", FileName: "{{v}}", ContentType: "{{v}}"}},
			},
			BinaryFile: "{{v}}",
			AuthNote:   "{{v}}",
		}, r)
	})
}
