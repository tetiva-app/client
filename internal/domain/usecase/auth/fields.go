// Package auth holds the auth-scheme logic shared by the request usecase and the Wails auth service.
// It imports only entities and the domain error types, so both sides can depend on it.
package auth

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

// Fields is a parsed auth_data document: string leaves for credentials, nested
// objects for JWT claims and headers.
type Fields map[string]any

// ParseFields parses an auth_data JSON object; "" and "{}" yield empty Fields.
func ParseFields(raw string) (Fields, error) {
	const funcName = "auth.ParseFields"

	trimmed := strings.TrimSpace(raw)
	if trimmed == "" || trimmed == "{}" {
		return Fields{}, nil
	}
	var f Fields
	if err := json.Unmarshal([]byte(trimmed), &f); err != nil {
		return nil, fmt.Errorf("%s: invalid auth_data JSON: %w", funcName, err)
	}
	if f == nil {
		return Fields{}, nil
	}
	return f, nil
}

// Str returns the string at key, or "" when absent or not a string.
func (f Fields) Str(key string) string {
	s, _ := f[key].(string)
	return s
}

// Obj returns the object at key, or nil when absent or not an object.
func (f Fields) Obj(key string) map[string]any {
	o, _ := f[key].(map[string]any)
	return o
}

// Substitute replaces {{variables}} in every string leaf at any depth. Working on the parsed document
// keeps quotes and braces inside values from corrupting the JSON.
func Substitute(f Fields, vars map[string]string) Fields {
	if len(f) == 0 {
		return f
	}
	out := make(Fields, len(f))
	for k, v := range f {
		out[k] = substituteValue(v, vars)
	}
	return out
}

func substituteValue(v any, vars map[string]string) any {
	switch t := v.(type) {
	case string:
		return substituteString(t, vars)
	case map[string]any:
		obj := make(map[string]any, len(t))
		for k, sub := range t {
			obj[k] = substituteValue(sub, vars)
		}
		return obj
	case []any:
		arr := make([]any, len(t))
		for i, sub := range t {
			arr[i] = substituteValue(sub, vars)
		}
		return arr
	default:
		return v
	}
}

// Same pattern as the request usecase; duplicated because auth must not import it.
var varPattern = regexp.MustCompile(`\{\{([^}]+)\}\}`)

func substituteString(text string, vars map[string]string) string {
	if len(vars) == 0 || !strings.Contains(text, "{{") {
		return text
	}
	return varPattern.ReplaceAllStringFunc(text, func(match string) string {
		if val, ok := vars[match[2:len(match)-2]]; ok {
			return val
		}
		return match
	})
}
