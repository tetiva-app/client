package portability

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWarn_PicksTheFormByCount(t *testing.T) {
	assert.Equal(t, "1 page asset entry skipped: images, scripts, stylesheets, fonts and media are not imported",
		Warn(WarnHARAssetsSkipped, Subject{}, "n", 1).Text)
	assert.Equal(t, "2 page asset entries skipped: images, scripts, stylesheets, fonts and media are not imported",
		Warn(WarnHARAssetsSkipped, Subject{}, "n", 2).Text)
	assert.Equal(t, "a variable without a name was skipped", Warn(WarnVariablesUnnamed, Subject{}, "n", 1).Text)
	assert.Equal(t, "3 variables without a name were skipped", Warn(WarnVariablesUnnamed, Subject{}, "n", 3).Text)
	assert.Equal(t, "path parameter id differs between requests: the environment keeps the first value",
		Warn(WarnInsomniaPathParamsDiffer, Subject{}, "n", 1, "names", "id").Text)
}

func TestWarn_ListOverflow(t *testing.T) {
	w := Warn(WarnExampleSecrets, Subject{}, "examples", List{Items: []string{"a", "b", "c", "d", "e", "f", "g"}, Sep: "; "})

	assert.Equal(t, map[string]string{"examples": "a; b; c; d; e", "examplesMore": "2"}, w.Params)
	assert.Equal(t, "examples may contain secrets, check them before sharing: a; b; c; d; e and 2 more", w.Text)
}

func TestWarn_ShortListHasNoOverflow(t *testing.T) {
	w := Warn(WarnOpenAPIAuthHeaderParam, Subject{}, "operations", List{Items: []string{"GET /a", "GET /b"}})

	assert.Equal(t, map[string]string{"operations": "GET /a, GET /b"}, w.Params)
	assert.Equal(t, "the Authorization header parameter of GET /a, GET /b was not imported: "+
		"set the auth on the collection or the request instead", w.Text)
}

func TestWarn_FillsInOnePass(t *testing.T) {
	w := Warn(WarnExampleSkipped, Subject{Kind: KindRequest, Name: "{example}"}, "example", "{reason}", "reason", "too long")

	assert.Equal(t, `request "{example}": example "{reason}" skipped: too long`, w.Text)
}

func TestWarn_KeepsDoubleBraces(t *testing.T) {
	w := Warn(WarnOpenAPIOwnServer, Subject{Kind: KindOperation, Name: "GET /users"}, "url", "https://other.example.com")

	assert.Equal(t, `GET /users: its own server "https://other.example.com" was ignored, the request uses {{baseUrl}}`, w.Text)
}

func TestWarn_SubjectForms(t *testing.T) {
	const text = "the file body was imported without its file; pick it again"
	for _, tc := range []struct {
		subject Subject
		want    string
	}{
		{Subject{}, text},
		{Subject{Kind: KindRequest, Name: `Say "hi" \ bye`}, `request "Say "hi" \ bye": ` + text},
		{Subject{Kind: KindRequest}, "request: " + text},
		{Subject{Kind: KindOperation, Name: "POST /upload"}, "POST /upload: " + text},
	} {
		assert.Equal(t, tc.want, Warn(WarnFileBodyDropped, tc.subject).Text)
	}
}

func TestWarn_IntParams(t *testing.T) {
	w := Warn(WarnDescriptionTruncated, Subject{Kind: KindCollection, Name: "API"}, "max", 16384)

	assert.Equal(t, map[string]string{"max": "16384"}, w.Params)
	assert.Equal(t, `collection "API": description longer than 16,384 bytes was truncated`, w.Text)
}

func TestWarn_GroupsThousandsInCounts(t *testing.T) {
	w := Warn(WarnHARBodiesLeftOut, Subject{}, "n", 1234, "kb", 256)

	assert.Equal(t, map[string]string{"n": "1234", "kb": "256"}, w.Params)
	assert.Equal(t, "1,234 response bodies were left out of the examples: "+
		"only JSON, XML and plain-text bodies up to 256 KB are kept", w.Text)
	assert.Equal(t, "example 1: status code 1500 is out of range, imported as 0",
		Warn(WarnPostmanExampleStatus, Subject{}, "index", 1, "status", 1500).Text)
}

func TestTexts(t *testing.T) {
	assert.Equal(t, []string{}, Texts(nil))
	assert.Equal(t, []string{"headers set on Hoppscotch collections and folders were copied into their requests"},
		Texts([]Warning{Warn(WarnHoppscotchHeadersCopied, Subject{})}))
}

type fixtureSubject struct {
	Kind string `json:"kind"`
	Name string `json:"name"`
}

type fixtureSample struct {
	Code    WarningCode       `json:"code"`
	Subject fixtureSubject    `json:"subject"`
	Params  map[string]string `json:"params"`
	Text    string            `json:"text"`
}

func placeholders(tmpl string) []string {
	var out []string
	for _, m := range placeholder.FindAllStringSubmatchIndex(tmpl, -1) {
		if m[0] > 0 && tmpl[m[0]-1] == '{' {
			continue
		}
		if key := tmpl[m[2]:m[3]]; !slices.Contains(out, key) {
			out = append(out, key)
		}
	}
	return out
}

func sampleArgs(code WarningCode, n int) []any {
	forms := warningTexts[code]
	var args []any
	for _, key := range placeholders(forms.one + " " + forms.other) {
		switch key {
		case "n":
		case "kind":
			args = append(args, key, KindFolder)
		default:
			args = append(args, key, "v-"+key)
		}
	}
	if forms.other != "" {
		args = append(args, "n", n)
	}
	return args
}

func fixtureSamples() []fixtureSample {
	var ws []Warning
	codes := make([]WarningCode, 0, len(warningTexts))
	for code := range warningTexts {
		codes = append(codes, code)
	}
	slices.Sort(codes)
	for _, code := range codes {
		ws = append(ws, Warn(code, Subject{}, sampleArgs(code, 1)...))
		if warningTexts[code].other != "" {
			ws = append(ws, Warn(code, Subject{}, sampleArgs(code, 2)...), Warn(code, Subject{}, sampleArgs(code, 1234)...))
		}
	}
	for _, kind := range []string{KindCollection, KindFolder, KindRequest, KindEnvironment, KindExample} {
		ws = append(ws, Warn(WarnFileBodyDropped, Subject{Kind: kind, Name: "v-name"}), Warn(WarnFileBodyDropped, Subject{Kind: kind}))
	}
	ws = append(ws, Warn(WarnFileBodyDropped, Subject{Kind: KindOperation, Name: "POST /v-path"}))
	items := make([]string, 7)
	for i := range items {
		items[i] = "v-" + strconv.Itoa(i+1)
	}
	ws = append(ws, Warn(WarnFormsSentURLEncoded, Subject{}, "n", len(items), "requests", List{Items: items}),
		Warn(WarnDescriptionTruncated, Subject{}, "max", 16384),
		Warn(WarnPostmanExampleTooLarge, Subject{}, "index", 1200, "kb", 1024))

	out := make([]fixtureSample, 0, len(ws))
	for _, w := range ws {
		params := w.Params
		if params == nil {
			params = map[string]string{}
		}
		out = append(out, fixtureSample{
			Code: w.Code, Subject: fixtureSubject{Kind: w.Subject.Kind, Name: w.Subject.Name}, Params: params, Text: w.Text,
		})
	}
	return out
}

func TestWarningFixture(t *testing.T) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	require.NoError(t, enc.Encode(fixtureSamples()))
	want := buf.Bytes()

	path := filepath.Join("testdata", "warnings.json")
	if os.Getenv("UPDATE_WARNING_FIXTURES") == "1" {
		require.NoError(t, os.MkdirAll("testdata", 0o755))
		require.NoError(t, os.WriteFile(path, want, 0o644))
		return
	}
	got, err := os.ReadFile(path)
	require.NoError(t, err, "run with UPDATE_WARNING_FIXTURES=1 to create %s", path)
	assert.Equal(t, string(want), string(got), "%s is stale; run with UPDATE_WARNING_FIXTURES=1", path)
}

func TestWarningFixture_FillsEveryPlaceholder(t *testing.T) {
	for _, s := range fixtureSamples() {
		assert.NotRegexp(t, `(^|[^{])\{\w+\}`, s.Text, s.Code)
	}
}
