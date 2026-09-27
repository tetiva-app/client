package publication

import (
	"crypto/sha256"
	"encoding/hex"
	"slices"
	"strconv"
	"strings"
)

const (
	CategoryVar      = "var"
	CategoryScan     = "scan"
	CategoryHeader   = "header"
	CategoryQuery    = "query"
	CategoryMetadata = "metadata"
	CategoryForm     = "form"
	CategoryAuth     = "auth"
	CategoryURL      = "url"
	CategoryFile     = "file"
	CategoryScript   = "script"
	CategoryCookie   = "cookie"
)

func selector(owner, category, key string) string {
	return owner + "/" + category + "/" + key
}

func indexKey(i int) string {
	return strconv.Itoa(i)
}

// scanKey hashes the content, not an ordinal: selectors outlive edits across revisions.
func scanKey(pointer, match string) string {
	sum := sha256.Sum256([]byte(pointer + "\x00" + match))
	return hex.EncodeToString(sum[:])[:12]
}

// valueKey covers every row reachable through references: any change lapses the override.
func (vs *varState) valueKey(row int) string {
	h := sha256.New()
	h.Write([]byte(vs.rows[row].v.Value))
	queue := slices.Clone(vs.rows[row].refs)
	seen := map[string]bool{}
	for len(queue) > 0 {
		name := queue[0]
		queue = queue[1:]
		if seen[name] {
			continue
		}
		seen[name] = true
		for _, i := range vs.byKey[name] {
			h.Write([]byte("\x00" + name + "\x00" + vs.rows[i].v.Value))
			queue = append(queue, vs.rows[i].refs...)
		}
	}
	return hex.EncodeToString(h.Sum(nil))[:12]
}

func pointerToken(s string) string {
	return strings.ReplaceAll(strings.ReplaceAll(s, "~", "~0"), "/", "~1")
}

func joinPath(parts ...string) string {
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p != "" {
			out = append(out, p)
		}
	}
	return strings.Join(out, " / ")
}

func sortedUnique(in []string) []string {
	out := slices.Clone(in)
	slices.Sort(out)
	return slices.Compact(out)
}
