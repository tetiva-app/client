package request

import "testing"

func TestStripDraftQuery(t *testing.T) {
	tests := []struct {
		name     string
		url      string
		authKeys []string
		want     string
	}{
		{
			name: "an untouched query survives byte for byte",
			url:  "https://api.example.com/x?b=2&a=1&flag&q=a%20b&filter[]=1&filter[]=2",
			want: "https://api.example.com/x?b=2&a=1&flag&q=a%20b&filter[]=1&filter[]=2",
		},
		{
			name: "the remaining pairs keep their order and escaping",
			url:  "https://api.example.com/x?z=1&access_token=secret&q=a%20b&a=2",
			want: "https://api.example.com/x?z=1&q=a%20b&a=2",
		},
		{
			name:     "a recorded auth key is stripped whatever its case",
			url:      "https://api.example.com/x?Sess=abc&page=2",
			authKeys: []string{"sess"},
			want:     "https://api.example.com/x?page=2",
		},
		{
			name: "an escaped credential name is matched decoded",
			url:  "https://api.example.com/x?client%5Fsecret=shh&page=2",
			want: "https://api.example.com/x?page=2",
		},
		{
			name: "no query at all",
			url:  "https://api.example.com/x",
			want: "https://api.example.com/x",
		},
		{
			name: "an unparsable URL is left alone",
			url:  "://nope?access_token=secret",
			want: "://nope?access_token=secret",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := stripDraftQuery(tc.url, tc.authKeys); got != tc.want {
				t.Errorf("stripDraftQuery(%q) = %q, want %q", tc.url, got, tc.want)
			}
		})
	}
}
