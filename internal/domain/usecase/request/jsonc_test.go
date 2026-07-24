package request

import "testing"

func TestStripJSONC(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "empty",
			in:   "",
			want: "",
		},
		{
			name: "no comments",
			in:   `{"a": 1, "b": "x"}`,
			want: `{"a": 1, "b": "x"}`,
		},
		{
			name: "line comment at end of line",
			in: `{
  "a": 1, // trailing
  "b": 2
}`,
			want: `{
  "a": 1, ` + `
  "b": 2
}`,
		},
		{
			name: "line comment whole line",
			in: `{
  // skip this key
  "a": 1
}`,
			want: `{
  ` + `
  "a": 1
}`,
		},
		{
			name: "block comment single line",
			in:   `{"a": /* note */ 1}`,
			want: `{"a":  1}`,
		},
		{
			name: "block comment multi line keeps newlines",
			in: `{
  "a": 1 /* multi
  line
  comment */,
  "b": 2
}`,
			want: `{
  "a": 1 ` + "\n\n" + `,
  "b": 2
}`,
		},
		{
			name: "// inside string is preserved",
			in:   `{"url": "https://example.com/path"}`,
			want: `{"url": "https://example.com/path"}`,
		},
		{
			name: "/* inside string is preserved",
			in:   `{"glob": "/* ignore */"}`,
			want: `{"glob": "/* ignore */"}`,
		},
		{
			name: "escaped quote in string",
			in:   `{"a": "he said \"hi\" // not a comment"}`,
			want: `{"a": "he said \"hi\" // not a comment"}`,
		},
		{
			name: "line comment at EOF without newline",
			in:   `{"a": 1} // tail`,
			want: `{"a": 1} `,
		},
		{
			name: "unterminated block comment is stripped, newlines kept",
			in: `{"a": 1 /* never closed
"b": 2`,
			want: `{"a": 1 ` + "\n",
		},
		{
			name: "preserves newlines in line comment removal",
			in: `// header
{"a": 1}`,
			want: `
{"a": 1}`,
		},
		{
			name: "lone slash is preserved",
			in:   `{"path": "a/b"}`,
			want: `{"path": "a/b"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := stripJSONC(tt.in)
			if got != tt.want {
				t.Errorf("stripJSONC(%q)\n  got:  %q\n  want: %q", tt.in, got, tt.want)
			}
		})
	}
}
