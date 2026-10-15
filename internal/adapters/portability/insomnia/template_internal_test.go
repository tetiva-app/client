package insomnia

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTemplates_RewritesVariableReferences(t *testing.T) {
	cases := map[string]string{
		"{{ _.base_url }}/pets":            "{{base_url}}/pets",
		"{{base_url}}/pets":                "{{base_url}}/pets",
		"{{ host }}:{{_.port}}":            "{{host}}:{{port}}",
		"{{ _.auth.token }}":               "{{auth.token}}",
		"{{ _['test-value'] }}":            "{{test-value}}",
		`{{ _["x y"] }}`:                   "{{x y}}",
		"{{ _.vault.api_key }}":            "{{api_key}}",
		"{{ vault.api_key }}":              "{{api_key}}",
		"Bearer {{ _.token }}":             "Bearer {{token}}",
		"no templates":                     "no templates",
		`{"a":{"b":1}}`:                    `{"a":{"b":1}}`,
		"{{ $ts }}":                        "{{$ts}}",
		"{{ _.a }}{{_.b}}{{ c }}":          "{{a}}{{b}}{{c}}",
		"multi\nline {{ _.x }}\n{{ _.y }}": "multi\nline {{x}}\n{{y}}",
	}
	for in, want := range cases {
		var tpl templates
		assert.Equal(t, want, tpl.conv(in), in)
		assert.Empty(t, tpl.warning(), in)
	}
}

func TestTemplates_LeavesTagsAndExpressionsAndCountsThem(t *testing.T) {
	var tpl templates
	in := []string{
		"{% response 'body', 'req_1', 'b64::JC5pZA==::46b', 'never', 60 %}",
		"{%- faker 'randomEmail' -%} and {% faker 'randomFirstName' %}",
		"{{ _.a | upper }}",
		"{{ _.vault[0] }}",
		"{% prompt 'Id', '', '1' %}/{{ _.id }}",
	}
	for _, s := range in[:4] {
		assert.Equal(t, s, tpl.conv(s))
	}
	assert.Equal(t, "{% prompt 'Id', '', '1' %}/{{id}}", tpl.conv(in[4]))

	assert.Equal(t,
		"6 Insomnia template tags were left as-is (faker, prompt, response, {{ … }} expressions): "+
			"Tetiva does not run them, use variables or scripts instead",
		tpl.warning())
}
