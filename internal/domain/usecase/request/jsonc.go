package request

import "strings"

// stripJSONC drops comments outside string literals, keeping the newlines of stripped blocks so
// error positions still map to the original lines. Postman-compat: JSON bodies only.
func stripJSONC(src string) string {
	if src == "" {
		return ""
	}

	var b strings.Builder
	b.Grow(len(src))

	i := 0
	n := len(src)
	for i < n {
		c := src[i]

		// String literal — copy verbatim, honoring \" escapes.
		if c == '"' {
			b.WriteByte(c)
			i++
			for i < n {
				ch := src[i]
				b.WriteByte(ch)
				i++
				if ch == '\\' && i < n {
					b.WriteByte(src[i])
					i++
					continue
				}
				if ch == '"' {
					break
				}
			}
			continue
		}

		// Line comment: drop until newline (newline itself stays).
		if c == '/' && i+1 < n && src[i+1] == '/' {
			i += 2
			for i < n && src[i] != '\n' {
				i++
			}
			continue
		}

		// Block comment: drop until */ ; preserve newlines for line accuracy.
		if c == '/' && i+1 < n && src[i+1] == '*' {
			i += 2
			for i < n {
				if src[i] == '\n' {
					b.WriteByte('\n')
					i++
					continue
				}
				if src[i] == '*' && i+1 < n && src[i+1] == '/' {
					i += 2
					break
				}
				i++
			}
			continue
		}

		b.WriteByte(c)
		i++
	}

	return b.String()
}
