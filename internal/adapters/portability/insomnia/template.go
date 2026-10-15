package insomnia

import (
	"regexp"
	"slices"
	"strings"

	"github.com/tetiva-app/client/internal/adapters/portability"
)

var (
	expression = regexp.MustCompile(`\{\{([^{}]*)\}\}`)
	tag        = regexp.MustCompile(`\{%-?\s*(\w*)[^%]*%\}`)
	identPath  = regexp.MustCompile(`^[A-Za-z_$][\w$]*(?:\.[A-Za-z_$][\w$]*)*$`)
	bracketKey = regexp.MustCompile(`^_\[\s*(?:'([^']*)'|"([^"]*)")\s*\]$`)
	pathParam  = regexp.MustCompile(`/:([^/?#:]+)`)
)

type templates struct {
	left map[string]int
}

func (t *templates) conv(s string) string {
	if !strings.Contains(s, "{") {
		return s
	}
	for _, m := range tag.FindAllStringSubmatch(s, -1) {
		t.leave(m[1])
	}
	return expression.ReplaceAllStringFunc(s, func(m string) string {
		if name, ok := variableName(m[2 : len(m)-2]); ok {
			return "{{" + name + "}}"
		}
		t.leave("{{ … }} expressions")
		return m
	})
}

func (t *templates) leave(name string) {
	if name == "" {
		name = "unnamed"
	}
	if t.left == nil {
		t.left = map[string]int{}
	}
	t.left[name]++
}

func (t *templates) warning() []portability.Warning {
	if len(t.left) == 0 {
		return nil
	}
	total := 0
	names := make([]string, 0, len(t.left))
	for name, n := range t.left {
		total += n
		names = append(names, name)
	}
	slices.Sort(names)
	return []portability.Warning{
		portability.Warn(portability.WarnInsomniaTemplateTags, portability.Subject{}, "n", total, "names", strings.Join(names, ", ")),
	}
}

func variableName(expr string) (string, bool) {
	e := strings.TrimSpace(expr)
	if m := bracketKey.FindStringSubmatch(e); m != nil {
		return m[1] + m[2], m[1]+m[2] != ""
	}
	e = strings.TrimPrefix(e, "_.")
	if !identPath.MatchString(e) {
		return "", false
	}
	return strings.TrimPrefix(e, "vault."), true
}
