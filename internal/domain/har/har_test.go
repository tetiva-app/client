package har_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/tetiva-app/client/internal/domain/har"
)

func TestBuild(t *testing.T) {
	tests := []struct {
		name     string
		in       har.Input
		want     har.Request
		warnings []string
	}{
		{
			name: "query_moves_out_of_url",
			in:   har.Input{Method: "GET", URL: "https://a.io/p?x=1&y=two%20words&z=a+b"},
			want: har.Request{
				Method: "GET", URL: "https://a.io/p", HTTPVersion: "HTTP/1.1",
				QueryString: []har.NameValue{{Name: "x", Value: "1"}, {Name: "y", Value: "two words"}, {Name: "z", Value: "a b"}},
			},
		},
		{
			name: "placeholder_base_survives",
			in:   har.Input{Method: "GET", URL: "{{baseUrl}}/users?id={{id}}"},
			want: har.Request{
				Method: "GET", URL: "{{baseUrl}}/users", HTTPVersion: "HTTP/1.1",
				QueryString: []har.NameValue{{Name: "id", Value: "{{id}}"}},
			},
		},
		{
			name: "hash_inside_placeholder_not_fragment",
			in:   har.Input{Method: "GET", URL: "https://a.io/{{x#y}}?q=1"},
			want: har.Request{
				Method: "GET", URL: "https://a.io/{{x#y}}", HTTPVersion: "HTTP/1.1",
				QueryString: []har.NameValue{{Name: "q", Value: "1"}},
			},
		},
		{
			name: "placeholder_in_query_not_unescaped",
			in:   har.Input{Method: "GET", URL: "https://a.io/?q={{a+b&c=d}}%20x"},
			want: har.Request{
				Method: "GET", URL: "https://a.io/", HTTPVersion: "HTTP/1.1",
				QueryString: []har.NameValue{{Name: "q", Value: "{{a+b&c=d}} x"}},
			},
		},
		{
			name: "repeated_query_keys_kept",
			in:   har.Input{Method: "GET", URL: "https://a.io/p?a=1&b=2&a=3"},
			want: har.Request{
				Method: "GET", URL: "https://a.io/p", HTTPVersion: "HTTP/1.1",
				QueryString: []har.NameValue{{Name: "a", Value: "1"}, {Name: "b", Value: "2"}, {Name: "a", Value: "3"}},
			},
		},
		{
			name: "flag_without_value",
			in:   har.Input{Method: "GET", URL: "https://a.io/p?flag&&x=&y=%zz&"},
			want: har.Request{
				Method: "GET", URL: "https://a.io/p", HTTPVersion: "HTTP/1.1",
				QueryString: []har.NameValue{{Name: "flag", Value: ""}, {Name: "x", Value: ""}, {Name: "y", Value: "%zz"}},
			},
		},
		{
			name: "empty_query_dropped",
			in:   har.Input{Method: "GET", URL: "https://a.io/p?"},
			want: har.Request{Method: "GET", URL: "https://a.io/p", HTTPVersion: "HTTP/1.1"},
		},
		{
			name: "userinfo_stripped",
			in:   har.Input{Method: "GET", URL: "https://user:p@ss@a.io/p?q=1"},
			want: har.Request{
				Method: "GET", URL: "https://a.io/p", HTTPVersion: "HTTP/1.1",
				QueryString: []har.NameValue{{Name: "q", Value: "1"}},
			},
		},
		{
			name: "at_in_path_is_not_userinfo",
			in:   har.Input{Method: "GET", URL: "https://a.io/@me"},
			want: har.Request{Method: "GET", URL: "https://a.io/@me", HTTPVersion: "HTTP/1.1"},
		},
		{
			name: "fragment_dropped",
			in:   har.Input{Method: "GET", URL: "https://a.io/p#frag?x=1"},
			want: har.Request{Method: "GET", URL: "https://a.io/p", HTTPVersion: "HTTP/1.1"},
		},
		{
			name: "headers_sorted_and_folded",
			in: har.Input{Method: "GET", URL: "https://a.io", Headers: map[string][]string{
				"X-B":    {"1", "2"},
				"Accept": {"a"},
				"X-None": {},
			}},
			want: har.Request{
				Method: "GET", URL: "https://a.io", HTTPVersion: "HTTP/1.1",
				Headers: []har.NameValue{{Name: "Accept", Value: "a"}, {Name: "X-B", Value: "1, 2"}},
			},
		},
		{
			name: "cookie_folded_with_semicolon",
			in: har.Input{Method: "GET", URL: "https://a.io", Headers: map[string][]string{
				"cookie": {"a=1", "b=2"},
			}},
			want: har.Request{
				Method: "GET", URL: "https://a.io", HTTPVersion: "HTTP/1.1",
				Headers: []har.NameValue{{Name: "cookie", Value: "a=1; b=2"}},
			},
		},
		{
			name: "text_body_kept",
			in: har.Input{Method: "POST", URL: "https://a.io", Body: har.Body{
				Kind: har.BodyText, MimeType: "application/json", Text: `{"a":1}`,
			}},
			want: har.Request{
				Method: "POST", URL: "https://a.io", HTTPVersion: "HTTP/1.1",
				PostData: &har.PostData{MimeType: "application/json", Text: `{"a":1}`},
			},
		},
		{
			name: "multipart_drops_content_type",
			in: har.Input{
				Method: "POST", URL: "https://a.io",
				Headers: map[string][]string{
					"content-type": {"multipart/form-data; boundary=x"},
					"X":            {"1"},
				},
				Body: har.Body{
					Kind: har.BodyMultipart, MimeType: "multipart/form-data; boundary=x",
					Params: []har.Param{{Name: "a", Value: "1"}},
				},
			},
			want: har.Request{
				Method: "POST", URL: "https://a.io", HTTPVersion: "HTTP/1.1",
				Headers:  []har.NameValue{{Name: "X", Value: "1"}},
				PostData: &har.PostData{MimeType: "multipart/form-data", Params: []har.Param{{Name: "a", Value: "1"}}},
			},
		},
		{
			name: "non_ascii_header_warns",
			in: har.Input{Method: "GET", URL: "https://a.io", Headers: map[string][]string{
				"X-Name": {"Привет"},
			}},
			want: har.Request{
				Method: "GET", URL: "https://a.io", HTTPVersion: "HTTP/1.1",
				Headers: []har.NameValue{{Name: "X-Name", Value: "Привет"}},
			},
			warnings: []string{`Header "X-Name" has non-ASCII characters; some languages cannot send it`},
		},
		{
			name: "urlencoded_repeated_key_warns",
			in: har.Input{Method: "POST", URL: "https://a.io", Body: har.Body{
				Kind:   har.BodyURLEncoded,
				Params: []har.Param{{Name: "a", Value: "1"}, {Name: "a", Value: "2"}, {Name: "b", Value: ""}, {Name: "a", Value: "3"}},
			}},
			want: har.Request{
				Method: "POST", URL: "https://a.io", HTTPVersion: "HTTP/1.1",
				PostData: &har.PostData{
					MimeType: "application/x-www-form-urlencoded",
					Params:   []har.Param{{Name: "a", Value: "1"}, {Name: "a", Value: "2"}, {Name: "b", Value: ""}, {Name: "a", Value: "3"}},
				},
			},
			warnings: []string{`Repeated form key "a": some languages keep only the last value`},
		},
		{
			name: "multipart_file_basename",
			in: har.Input{Method: "POST", URL: "https://a.io", Body: har.Body{
				Kind: har.BodyMultipart, MimeType: "multipart/form-data",
				Params: []har.Param{
					{Name: "f", Value: "ignored", FileName: "/Users/me/docs/report.pdf", ContentType: "application/pdf"},
					{Name: "w", FileName: `C:\Users\me\photo.png`},
					{Name: "t", Value: "v"},
				},
			}},
			want: har.Request{
				Method: "POST", URL: "https://a.io", HTTPVersion: "HTTP/1.1",
				PostData: &har.PostData{MimeType: "multipart/form-data", Params: []har.Param{
					{Name: "f", FileName: "report.pdf", ContentType: "application/pdf"},
					{Name: "w", FileName: "photo.png"},
					{Name: "t", Value: "v"},
				}},
			},
		},
		{
			name: "binary_basename_and_warning",
			in: har.Input{Method: "PUT", URL: "https://a.io", Body: har.Body{
				Kind: har.BodyBinary, MimeType: "application/octet-stream", BinaryFile: "{{dir}}/a.bin",
			}},
			want: har.Request{
				Method: "PUT", URL: "https://a.io", HTTPVersion: "HTTP/1.1",
				PostData:   &har.PostData{MimeType: "application/octet-stream"},
				BinaryFile: "a.bin",
			},
			warnings: []string{"Binary body is shown as a file reference"},
		},
		{
			name: "binary_placeholder_with_slash_kept_whole",
			in: har.Input{Method: "PUT", URL: "https://a.io", Body: har.Body{
				Kind: har.BodyBinary, BinaryFile: "{{a/b}}",
			}},
			want: har.Request{
				Method: "PUT", URL: "https://a.io", HTTPVersion: "HTTP/1.1",
				PostData:   &har.PostData{},
				BinaryFile: "{{a/b}}",
			},
			warnings: []string{"Binary body is shown as a file reference"},
		},
		{
			name: "http_version_fixed",
			in:   har.Input{Method: "DELETE", URL: "http://a.io/x"},
			want: har.Request{Method: "DELETE", URL: "http://a.io/x", HTTPVersion: "HTTP/1.1"},
		},
		{
			name: "auth_note_copied",
			in:   har.Input{Method: "GET", URL: "https://a.io", AuthNote: "digest"},
			want: har.Request{Method: "GET", URL: "https://a.io", HTTPVersion: "HTTP/1.1", AuthNote: "digest"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, warnings := har.Build(tt.in)
			assert.Equal(t, tt.want, got)
			assert.Equal(t, tt.warnings, warnings)
		})
	}
}
