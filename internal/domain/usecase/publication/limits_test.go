package publication_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tetiva-app/client/internal/adapters/publication/snapshotjson"
	"github.com/tetiva-app/client/internal/domain/entities"
	"github.com/tetiva-app/client/internal/domain/usecase/publication"
)

func errorPaths(r publication.Report) []string {
	var out []string
	for _, e := range r.Errors {
		out = append(out, e.Path)
	}
	return out
}

func TestLimits_HeaderNames(t *testing.T) {
	f := newFixture()
	folder := f.folder(f.in.Root, "A")
	r := f.request(folder, "Get user")
	r.Headers = []entities.HeaderItem{
		{Key: "   ", Value: "dropped", Enabled: true},
		{Key: " X-Trimmed ", Value: "kept", Enabled: true},
		{Key: "X Api", Value: "1", Enabled: true},
		{Key: "X-{{tenant}}", Value: "1", Enabled: true},
	}

	s, report := f.build(t)

	h := findRequest(s.Collection.Items, "Get user").HTTP.Headers
	require.Len(t, h, 3)
	assert.Equal(t, "X-Trimmed", h[0].Key)
	assert.Equal(t, []string{"A / Get user / headers / X Api"}, errorPaths(report))
	assert.Contains(t, report.Errors[0].Message, "X Api")
}

func TestLimits_StringURLAndEnums(t *testing.T) {
	f := newFixture()
	big := f.request(f.in.Root, "big")
	big.Description = strings.Repeat("a", 1<<20+1)
	longURL := f.request(f.in.Root, "long url")
	longURL.URL = "https://api.example.com/" + strings.Repeat("p", 8<<10)
	ctrl := f.request(f.in.Root, "control")
	ctrl.URL = "https://api.example.com/\nx"
	method := f.request(f.in.Root, "method")
	method.Method = "TRACE"
	status := f.request(f.in.Root, "status")
	f.example(status, "weird").StatusCode = 1000
	proto := f.request(f.in.Root, "proto")
	proto.Protocol = "smtp"

	_, report := f.build(t)

	assert.ElementsMatch(t, []string{
		"big / description", "long url / url", "control / url", "method / method", "status / examples / weird / status", "proto / protocol",
	}, errorPaths(report))
}

func TestLimits_DepthAndItemCount(t *testing.T) {
	t.Run("depth", func(t *testing.T) {
		f := newFixture()
		parent := f.in.Root
		for i := 1; i <= 17; i++ {
			parent = f.folder(parent, fmt.Sprintf("L%d", i))
		}
		_, report := f.build(t)
		require.Len(t, report.Errors, 1)
		assert.True(t, strings.HasSuffix(report.Errors[0].Path, "L17"), report.Errors[0].Path)
	})
	t.Run("depth 16 passes", func(t *testing.T) {
		f := newFixture()
		parent := f.in.Root
		for i := 1; i <= 16; i++ {
			parent = f.folder(parent, fmt.Sprintf("L%d", i))
		}
		_, report := f.build(t)
		assert.Empty(t, report.Errors)
	})
	t.Run("items", func(t *testing.T) {
		f := newFixture()
		for i := 0; i < 5001; i++ {
			f.request(f.in.Root, "r")
		}
		_, report := f.build(t)
		require.Len(t, report.Errors, 1)
		assert.Equal(t, "Petstore", report.Errors[0].Path)
		assert.Equal(t, 5001, report.Requests)
	})
}

func headerRows(n int, value string) []entities.HeaderItem {
	rows := make([]entities.HeaderItem, n)
	for i := range rows {
		rows[i] = entities.HeaderItem{Key: fmt.Sprintf("x-h%d", i), Value: value, Enabled: true}
	}
	return rows
}

func formRequest(f *fixture, n int, key, value string) *entities.Request {
	fields := make([]map[string]any, n)
	for i := range fields {
		fields[i] = map[string]any{"key": key, "value": value, "type": "text", "enabled": true}
	}
	raw, _ := json.Marshal(fields)
	r := f.request(f.in.Root, "r")
	r.Method, r.BodyType, r.Body = entities.MethodPOST, entities.BodyTypeForm, string(raw)
	return r
}

func wsRequest(f *fixture, subprotocols, messages int) *entities.Request {
	subs := make([]string, subprotocols)
	for i := range subs {
		subs[i] = fmt.Sprintf("p%d", i)
	}
	msgs := make([]map[string]string, messages)
	for i := range msgs {
		msgs[i] = map[string]string{"id": fmt.Sprint(i), "name": "m", "format": "text", "data": "ping"}
	}
	raw, _ := json.Marshal(map[string]any{"version": 1, "subprotocols": subs, "messages": msgs})
	r := f.request(f.in.Root, "w")
	r.Protocol, r.URL, r.Body = entities.ProtocolWebSocket, "wss://ws.example.com", string(raw)
	return r
}

func authRequest(f *fixture, t entities.AuthType, fields map[string]any) *entities.Request {
	raw, _ := json.Marshal(fields)
	r := f.request(f.in.Root, "r")
	r.AuthType, r.AuthData = t, string(raw)
	return r
}

func bearerFields(n int) map[string]any {
	fields := map[string]any{"prefix": "Bearer", "token": "{{t}}"}
	for i := len(fields); i < n; i++ {
		fields[fmt.Sprintf("f%d", i)] = "1"
	}
	return fields
}

func nested(depth int) any {
	var v any = 1
	for range depth {
		v = map[string]any{"a": v}
	}
	return v
}

func claims(n int) map[string]any {
	out := make(map[string]any, n)
	for i := range n {
		out[fmt.Sprintf("c%d", i)] = i
	}
	return out
}

type limitCase struct {
	setup func(f *fixture)
	path  string
}

func runLimitCases(t *testing.T, cases map[string]limitCase, message string) {
	t.Helper()
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			f := newFixture()
			tc.setup(f)
			_, report := f.build(t)
			require.Equal(t, []string{tc.path}, errorPaths(report))
			assert.Contains(t, report.Errors[0].Message, message)
		})
	}
}

func TestLimits_Names(t *testing.T) {
	name := func(n int) string { return strings.Repeat("я", n) }
	token := func(n int) string { return strings.Repeat("a", n) }
	setups := func(n int) map[string]func(f *fixture) {
		return map[string]func(f *fixture){
			"collection":  func(f *fixture) { f.in.Root.Name = name(n) },
			"environment": func(f *fixture) { f.env(name(n)) },
			"folder":      func(f *fixture) { f.folder(f.in.Root, name(n)) },
			"request":     func(f *fixture) { f.request(f.in.Root, name(n)) },
			"example":     func(f *fixture) { f.example(f.request(f.in.Root, "r"), name(n)) },
			"header": func(f *fixture) {
				f.request(f.in.Root, "r").Headers = []entities.HeaderItem{{Key: token(n), Value: "1", Enabled: true}}
			},
			"form field": func(f *fixture) { formRequest(f, 1, name(n), "1") },
			"variable":   func(f *fixture) { f.variable(name(n), "1", false) },
			"auth field": func(f *fixture) { authRequest(f, entities.AuthTypeBearer, map[string]any{"token": "x", name(n): "v"}) },
			"key in an auth value": func(f *fixture) {
				authRequest(f, entities.AuthTypeJWT, map[string]any{"alg": "HS256", "claims": map[string]any{name(n): "x"}})
			},
		}
	}

	over := setups(513)
	runLimitCases(t, map[string]limitCase{
		"collection":           {over["collection"], name(513) + " / name"},
		"environment":          {over["environment"], "Environment " + name(513)},
		"folder":               {over["folder"], name(513) + " / name"},
		"request":              {over["request"], name(513) + " / name"},
		"example":              {over["example"], "r / examples / " + name(513) + " / name"},
		"header":               {over["header"], "r / headers / " + token(513)},
		"form field":           {over["form field"], "r / body / " + name(513)},
		"variable":             {over["variable"], "Environment prod / " + name(513)},
		"auth field":           {over["auth field"], "r / auth"},
		"key in an auth value": {over["key in an auth value"], "r / auth / claims"},
	}, "512 characters")

	for label, setup := range setups(512) {
		t.Run(label+" at 512 characters", func(t *testing.T) {
			f := newFixture()
			setup(f)
			_, report := f.build(t)
			assert.Empty(t, report.Errors)
		})
	}
}

func TestLimits_BlankCollectionName(t *testing.T) {
	for _, blank := range []string{"", "   ", "\t\n", "\u00a0\u3000"} {
		t.Run(fmt.Sprintf("%q", blank), func(t *testing.T) {
			f := newFixture()
			f.in.Root.Name = blank
			_, report := f.build(t)
			require.Equal(t, []string{"name"}, errorPaths(report))
			assert.Contains(t, report.Errors[0].Message, "name")
		})
	}

	f := newFixture()
	f.in.Root.Name = "  Petstore  "
	_, report := f.build(t)
	assert.Empty(t, report.Errors)
}

func TestLimits_ValueSizes(t *testing.T) {
	setups := func(pair, variable string) map[string]func(f *fixture) {
		return map[string]func(f *fixture){
			"http header": func(f *fixture) {
				f.request(f.in.Root, "r").Headers = []entities.HeaderItem{{Key: "X-A", Value: pair, Enabled: true}}
			},
			"graphql header": func(f *fixture) {
				r := f.request(f.in.Root, "q")
				r.Protocol, r.Headers = entities.ProtocolGraphQL, []entities.HeaderItem{{Key: "X-A", Value: pair, Enabled: true}}
			},
			"websocket header": func(f *fixture) {
				wsRequest(f, 0, 0).Headers = []entities.HeaderItem{{Key: "X-A", Value: pair, Enabled: true}}
			},
			"example header": func(f *fixture) {
				f.example(f.request(f.in.Root, "r"), "e").Headers = []entities.HeaderItem{{Key: "X-A", Value: pair, Enabled: true}}
			},
			"collection metadata": func(f *fixture) {
				f.in.Root.GRPCMetadata = []entities.HeaderItem{{Key: "x-a", Value: pair, Enabled: true}}
			},
			"grpc metadata": func(f *fixture) {
				r := f.request(f.in.Root, "g")
				r.Protocol, r.URL, r.GRPCMetadata = entities.ProtocolGRPC, "grpc.example.com:443", map[string][]string{"x-a": {pair}}
			},
			"form field": func(f *fixture) { formRequest(f, 1, "k", pair) },
			"variable":   func(f *fixture) { f.variable("v", variable, false) },
		}
	}

	over := setups(strings.Repeat("x", 16<<10+1), strings.Repeat("x", 64<<10+1))
	runLimitCases(t, map[string]limitCase{
		"http header":         {over["http header"], "r / headers / X-A"},
		"graphql header":      {over["graphql header"], "q / headers / X-A"},
		"websocket header":    {over["websocket header"], "w / headers / X-A"},
		"example header":      {over["example header"], "r / examples / e / headers / X-A"},
		"collection metadata": {over["collection metadata"], "Petstore / metadata / x-a"},
		"grpc metadata":       {over["grpc metadata"], "g / metadata / x-a"},
		"form field":          {over["form field"], "r / body / k"},
		"variable":            {over["variable"], "Environment prod / v"},
		"bytes, not characters": {func(f *fixture) {
			f.request(f.in.Root, "r").Headers = []entities.HeaderItem{{Key: "X-A", Value: strings.Repeat("я", 8193), Enabled: true}}
		}, "r / headers / X-A"},
	}, "KiB")

	for label, setup := range setups(strings.Repeat("я", 8192), strings.Repeat("я", 32<<10)) {
		t.Run(label+" at the limit", func(t *testing.T) {
			f := newFixture()
			setup(f)
			_, report := f.build(t)
			assert.Empty(t, report.Errors)
		})
	}
}

func TestLimits_ListLengths(t *testing.T) {
	setups := func(pairs, examples, messages, subprotocols, variables, authFields int) map[string]func(f *fixture) {
		return map[string]func(f *fixture){
			"headers":     func(f *fixture) { f.request(f.in.Root, "r").Headers = headerRows(pairs, "1") },
			"form fields": func(f *fixture) { formRequest(f, pairs, "k", "1") },
			"examples": func(f *fixture) {
				r := f.request(f.in.Root, "r")
				for range examples {
					f.example(r, "e")
				}
			},
			"example headers":     func(f *fixture) { f.example(f.request(f.in.Root, "r"), "e").Headers = headerRows(pairs, "1") },
			"collection metadata": func(f *fixture) { f.in.Root.GRPCMetadata = headerRows(pairs, "1") },
			"merged metadata": func(f *fixture) {
				f.in.Root.GRPCMetadata = headerRows(150, "1")
				r := f.request(f.in.Root, "g")
				r.Protocol, r.URL = entities.ProtocolGRPC, "grpc.example.com:443"
				for i := range pairs - 150 {
					r.GRPCMetadata[fmt.Sprintf("x-r%d", i)] = []string{"1"}
				}
			},
			"messages":     func(f *fixture) { wsRequest(f, 0, messages) },
			"subprotocols": func(f *fixture) { wsRequest(f, subprotocols, 0) },
			"variables": func(f *fixture) {
				for i := range variables {
					f.variable(fmt.Sprintf("v%d", i), "1", false)
				}
			},
			"auth fields": func(f *fixture) { authRequest(f, entities.AuthTypeBearer, bearerFields(authFields)) },
		}
	}

	over := setups(201, 51, 201, 65, 1001, 65)
	runLimitCases(t, map[string]limitCase{
		"headers":             {over["headers"], "r / headers"},
		"form fields":         {over["form fields"], "r / body"},
		"examples":            {over["examples"], "r / examples"},
		"example headers":     {over["example headers"], "r / examples / e / headers"},
		"collection metadata": {over["collection metadata"], "Petstore / metadata"},
		"merged metadata":     {over["merged metadata"], "g / metadata"},
		"messages":            {over["messages"], "w / messages"},
		"subprotocols":        {over["subprotocols"], "w / subprotocols"},
		"variables":           {over["variables"], "Environment prod"},
		"auth fields":         {over["auth fields"], "r / auth"},
	}, "at most")

	for label, setup := range setups(200, 50, 200, 64, 1000, 64) {
		t.Run(label+" at the limit", func(t *testing.T) {
			f := newFixture()
			setup(f)
			_, report := f.build(t)
			assert.Empty(t, report.Errors)
		})
	}
}

func TestLimits_AuthValueShape(t *testing.T) {
	jwt := func(c any) func(f *fixture) {
		return func(f *fixture) { authRequest(f, entities.AuthTypeJWT, map[string]any{"alg": "HS256", "claims": c}) }
	}
	spread := func(a, b int) map[string]any {
		return map[string]any{"a": claims(a), "b": []any{claims(b)}}
	}

	runLimitCases(t, map[string]limitCase{
		"five levels": {jwt(nested(5)), "r / auth / claims"},
	}, "4 levels")
	runLimitCases(t, map[string]limitCase{
		"257 values":             {jwt(claims(257)), "r / auth / claims"},
		"257 values in children": {jwt(spread(127, 127)), "r / auth / claims"},
	}, "256")

	for label, c := range map[string]any{"four levels": nested(4), "256 values": claims(256), "256 values in children": spread(127, 126)} {
		t.Run(label+" pass", func(t *testing.T) {
			f := newFixture()
			jwt(c)(f)
			_, report := f.build(t)
			assert.Empty(t, report.Errors)
		})
	}
}

func TestLimits_ValueCount(t *testing.T) {
	f := newFixture()
	for i := range 10 {
		r := f.request(f.in.Root, fmt.Sprintf("r%d", i))
		for range 50 {
			f.example(r, "e").Headers = headerRows(200, "")
		}
	}

	s, report := f.build(t)

	require.Greater(t, publication.CountValues(s), 500_000)
	require.Equal(t, []string{"Petstore"}, errorPaths(report))
	assert.Contains(t, report.Errors[0].Message, "500000")
}

func jsonValues(v any) int {
	n := 1
	switch t := v.(type) {
	case map[string]any:
		for _, sub := range t {
			n += jsonValues(sub)
		}
	case []any:
		for _, sub := range t {
			n += jsonValues(sub)
		}
	}
	return n
}

func TestCountValues_MatchesTheCanonicalJSON(t *testing.T) {
	snapshots := map[string]*publication.Snapshot{}
	for _, name := range []string{"all-protocols.json", "max-size.json"} {
		raw, err := os.ReadFile(filepath.Join("..", "..", "..", "..", "testdata", "snapshot", name))
		require.NoError(t, err)
		s, err := snapshotjson.Decode(bytes.NewReader(raw), 8<<20)
		require.NoError(t, err)
		snapshots[name] = s
	}
	f := newFixture()
	f.in.Root.GRPCMetadata = headerRows(2, "1")
	formRequest(f, 3, "k", "v")
	wsRequest(f, 2, 3).Headers = headerRows(1, "1")
	authRequest(f, entities.AuthTypeJWT, map[string]any{"alg": "HS256", "claims": map[string]any{"a": []any{1, true, nil, map[string]any{"b": "c"}}}})
	f.example(f.request(f.in.Root, "e"), "ok").Headers = headerRows(2, "1")
	g := f.request(f.in.Root, "g")
	g.Protocol, g.URL, g.GRPCMetadata = entities.ProtocolGRPC, "grpc.example.com:443", map[string][]string{"x-g": {"1"}}
	q := f.request(f.in.Root, "q")
	q.Protocol, q.PreScript = entities.ProtocolGraphQL, "pm.test()"
	f.in.IncludeScripts = true
	f.variable("v", "1", false)
	snapshots["built"], _ = f.build(t)
	snapshots["no environment"], _ = newFixture().build(t)

	for name, s := range snapshots {
		t.Run(name, func(t *testing.T) {
			out, err := snapshotjson.Marshal(s)
			require.NoError(t, err)
			var doc any
			require.NoError(t, json.Unmarshal(out, &doc))
			assert.Equal(t, jsonValues(doc), publication.CountValues(s))
		})
	}
}
