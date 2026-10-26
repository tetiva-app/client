package request

import (
	"regexp"
	"strings"

	"github.com/tetiva-app/client/internal/domain/entities"
)

var varPattern = regexp.MustCompile(`\{\{([^}]+)\}\}`)

// substituteVariables replaces {{key}} with values from vars; unresolved keys are left as-is.
func substituteVariables(text string, vars map[string]string) string {
	if len(vars) == 0 || !strings.Contains(text, "{{") {
		return text
	}
	return varPattern.ReplaceAllStringFunc(text, func(match string) string {
		key := match[2 : len(match)-2] // strip {{ and }}
		if val, ok := vars[key]; ok {
			return val
		}
		return match
	})
}

func SubstituteVariables(text string, vars map[string]string) string {
	return substituteVariables(text, vars)
}

func pathVariablesFor(p entities.Protocol, rawURL string, vars []entities.PathVariable) []entities.PathVariable {
	// Checked before the protocol: turning unknown into [] would clear the server copy.
	if vars == nil {
		return nil
	}
	if !p.HasPathVariables() {
		return []entities.PathVariable{}
	}
	return entities.SyncPathVariables(rawURL, vars)
}

func resolveURL(req *entities.Request, vars map[string]string) string {
	resolve := func(s string) string { return substituteVariables(s, vars) }
	if !req.Protocol.HasPathVariables() {
		return resolve(req.URL)
	}
	return entities.ResolveURL(req.URL, req.PathVariables, resolve)
}

// substituteMetadata applies variable substitution to gRPC metadata values.
var substituteMetadata = substituteHeaders

// substituteHeaders applies variable substitution to both header keys and values.
func substituteHeaders(headers map[string][]string, vars map[string]string) map[string][]string {
	if len(vars) == 0 {
		return headers
	}
	result := make(map[string][]string, len(headers))
	for k, values := range headers {
		newKey := substituteVariables(k, vars)
		newValues := make([]string, len(values))
		for i, v := range values {
			newValues[i] = substituteVariables(v, vars)
		}
		result[newKey] = newValues
	}
	return result
}
