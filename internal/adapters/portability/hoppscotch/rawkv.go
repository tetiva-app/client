package hoppscotch

import (
	"encoding/json"
	"strings"
)

func parseRawKeyValues(text string) []hoppKV {
	var out []hoppKV
	for line := range strings.SplitSeq(text, "\n") {
		line = strings.TrimSpace(line)
		active := true
		if rest, ok := strings.CutPrefix(line, "#"); ok {
			active, line = false, strings.TrimSpace(rest)
		}
		key, rest := token(line, ":")
		if key == "" {
			continue
		}
		value := ""
		if after, ok := strings.CutPrefix(strings.TrimSpace(rest), ":"); ok {
			value, _ = token(strings.TrimSpace(after), "")
		}
		out = append(out, hoppKV{Key: key, Value: value, Active: &active})
	}
	return out
}

func token(s, stop string) (string, string) {
	if strings.HasPrefix(s, `"`) {
		dec := json.NewDecoder(strings.NewReader(s))
		var v string
		if dec.Decode(&v) == nil {
			return v, s[dec.InputOffset():]
		}
	}
	if i := strings.Index(s, stop); stop != "" && i >= 0 {
		return strings.TrimSpace(s[:i]), s[i:]
	}
	return strings.TrimSpace(s), ""
}
