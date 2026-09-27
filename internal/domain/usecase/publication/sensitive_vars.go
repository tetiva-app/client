package publication

import (
	"regexp"
	"strings"

	"github.com/tetiva-app/client/internal/domain/entities"
)

// As in request substitution: the name is everything between the braces, untrimmed.
var varRef = regexp.MustCompile(`\{\{([^}]+)\}\}`)

var suspiciousVars = []string{
	"auth", "session", "cookie", "key", "signature", "token", "secret", "password", "pass", "pwd", "credential", "private",
}

func isSuspiciousVar(name string) bool {
	lower := strings.ToLower(name)
	for _, s := range suspiciousVars {
		if strings.Contains(lower, s) {
			return true
		}
	}
	return false
}

func refNames(text string) []string {
	if !strings.Contains(text, "{{") {
		return nil
	}
	var out []string
	for _, m := range varRef.FindAllStringSubmatch(text, -1) {
		out = append(out, m[1])
	}
	return out
}

func onlyRefs(v string) bool {
	return strings.TrimSpace(varRef.ReplaceAllString(v, "")) == ""
}

func substitute(text string, vars map[string]string) string {
	if len(vars) == 0 || !strings.Contains(text, "{{") {
		return text
	}
	return varRef.ReplaceAllStringFunc(text, func(match string) string {
		if v, ok := vars[match[2:len(match)-2]]; ok {
			return v
		}
		return match
	})
}

type varRow struct {
	v        *entities.Variable
	owner    string
	selector string
	force    bool
	refs     []string
}

// Secrecy is per name, not per row: the page substitutes by name.
type varState struct {
	rows       []varRow
	secretKeys map[string]bool
	byKey      map[string][]int
	values     map[string]string
}

func newVarState(rows []*entities.Variable, id func(*entities.Variable) string, overrides map[string]bool) *varState {
	vs := &varState{secretKeys: map[string]bool{}, byKey: map[string][]int{}, values: map[string]string{}}
	for i, v := range rows {
		vs.rows = append(vs.rows, varRow{v: v, owner: id(v), refs: refNames(v.Value)})
		vs.byKey[v.Key] = append(vs.byKey[v.Key], i)
		vs.values[v.Key] = v.Value
		if v.IsSecret {
			vs.secretKeys[v.Key] = true
		}
	}
	for i := range vs.rows {
		row := &vs.rows[i]
		row.selector = selector(row.owner, CategoryVar, vs.valueKey(i))
		row.force = overrides[row.selector] && !row.v.IsSecret
	}
	return vs
}

func (vs *varState) hiddenReason(i int, referenced map[string]bool) string {
	key := vs.rows[i].v.Key
	switch {
	case vs.secretKeys[key]:
		return HiddenSecret
	case referenced[key]:
		return HiddenReferenced
	case isSuspiciousVar(key):
		return HiddenSuspicious
	}
	return ""
}

func (vs *varState) published(i int, referenced map[string]bool) bool {
	reason := vs.hiddenReason(i, referenced)
	return reason == "" || (reason != HiddenSecret && vs.rows[i].force)
}

func (vs *varState) publicValues(referenced map[string]bool) map[string]string {
	out := map[string]string{}
	for i, row := range vs.rows {
		if vs.published(i, referenced) {
			out[row.v.Key] = row.v.Value
		}
	}
	return out
}

func (vs *varState) expand(referenced map[string]bool, seeds map[string]bool) bool {
	before := len(referenced)
	var queue []string
	add := func(name string) {
		if !referenced[name] {
			referenced[name] = true
			queue = append(queue, name)
		}
	}
	for name := range seeds {
		add(name)
	}
	for _, row := range vs.rows {
		if row.v.IsSecret || isSuspiciousVar(row.v.Key) && !row.force {
			for _, name := range refNames(row.v.Value) {
				add(name)
			}
		}
	}
	for len(queue) > 0 {
		name := queue[0]
		queue = queue[1:]
		for _, i := range vs.byKey[name] {
			for _, ref := range refNames(vs.rows[i].v.Value) {
				add(ref)
			}
		}
	}
	return len(referenced) > before
}
