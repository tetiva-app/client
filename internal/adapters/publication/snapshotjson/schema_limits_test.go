package snapshotjson_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/santhosh-tekuri/jsonschema/v6"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tetiva-app/client/internal/adapters/publication/snapshotjson"
	"github.com/tetiva-app/client/internal/domain/entities"
	"github.com/tetiva-app/client/internal/domain/usecase/publication"
)

func requestNamed(t *testing.T, in publication.BuildInput, name string) *entities.Request {
	t.Helper()
	for _, r := range in.Requests {
		if r.Name == name {
			return r
		}
	}
	t.Fatalf("no request %q", name)
	return nil
}

func rows(n int, key func(int) string, value string) []entities.HeaderItem {
	out := make([]entities.HeaderItem, n)
	for i := range out {
		out[i] = entities.HeaderItem{Key: key(i), Value: value, Enabled: true}
	}
	return out
}

func mustJSON(v any) string {
	raw, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return string(raw)
}

func formFields(n int, key, value string) string {
	fields := make([]map[string]any, n)
	for i := range fields {
		fields[i] = map[string]any{"key": key, "value": value, "type": "text", "enabled": true}
	}
	return mustJSON(fields)
}

func wsSettings(subprotocols, messages int) string {
	subs := make([]string, subprotocols)
	for i := range subs {
		subs[i] = fmt.Sprintf("p%d", i)
	}
	msgs := make([]map[string]string, messages)
	for i := range msgs {
		msgs[i] = map[string]string{"id": fmt.Sprint(i), "name": "m", "format": "text", "data": "ping"}
	}
	return mustJSON(map[string]any{"version": 1, "subprotocols": subs, "messages": msgs})
}

func addExamples(in *publication.BuildInput, r *entities.Request, n int) {
	for i := range n {
		id := fixedID(5000 + i)
		in.Examples[r.ID] = append(in.Examples[r.ID], &entities.ResponseExample{
			ID: id, RequestID: r.ID, WorkspaceID: workspaceID, Name: fmt.Sprintf("e%d", i), StatusCode: 200,
			Headers: []entities.HeaderItem{}, Protocol: r.Protocol, Version: 1, CreatedAt: time.Date(2026, 9, 2, 0, 0, i, 0, time.UTC),
		})
	}
}

func addVariables(in *publication.BuildInput, total int) {
	for i := len(in.Variables); i < total; i++ {
		in.Variables = append(in.Variables, &entities.Variable{
			ID: fixedID(7000 + i), EnvironmentID: in.Environment.ID, Key: fmt.Sprintf("pad%d", i), Value: "1",
			Enabled: true, Version: 1, CreatedAt: time.Date(2026, 9, 3, 0, 0, i, 0, time.UTC),
		})
	}
}

func bearerWith(n int) string {
	fields := map[string]any{"prefix": "Bearer", "token": "literal-token"}
	for i := len(fields); i < n; i++ {
		fields[fmt.Sprintf("f%d", i)] = "1"
	}
	return mustJSON(fields)
}

func jwtWithClaims(claims any) string {
	return mustJSON(map[string]any{"alg": "HS256", "claims": claims})
}

func claimKeys(n int) map[string]any {
	out := make(map[string]any, n)
	for i := range n {
		out[fmt.Sprintf("c%d", i)] = i
	}
	return out
}

func schemaAccepts(t *testing.T, sch *jsonschema.Schema, out []byte) bool {
	t.Helper()
	inst, err := jsonschema.UnmarshalJSON(bytes.NewReader(out))
	require.NoError(t, err)
	return sch.Validate(inst) == nil
}

func TestMarshal_AtEveryLimitPassesTheServerSchema(t *testing.T) {
	in := petstore()
	name, token := strings.Repeat("я", 512), strings.Repeat("a", 512)
	in.Root.Name = name
	in.Collections[1].Name = name

	get := requestNamed(t, in, "Получить питомца")
	get.Name = name
	get.Headers = rows(200, func(i int) string { return fmt.Sprintf("X-%0510d", i) }, strings.Repeat("x", 16<<10))
	addExamples(&in, get, 50-len(in.Examples[get.ID]))
	in.Examples[get.ID][0].Name = name
	in.Examples[get.ID][0].Headers = rows(200, func(int) string { return token }, strings.Repeat("я", 8<<10))

	upload := requestNamed(t, in, "Загрузить фото")
	upload.Body = formFields(200, name, strings.Repeat("я", 8<<10))
	requestNamed(t, in, "Чат питомника").Body = wsSettings(64, 200)

	grpc := requestNamed(t, in, "GetPet (gRPC)")
	for i := range 200 - len(in.Root.GRPCMetadata) - len(grpc.GRPCMetadata) {
		grpc.GRPCMetadata[fmt.Sprintf("x-m%d", i)] = []string{"1"}
	}

	requestNamed(t, in, "Check pet exists").AuthData = jwtWithClaims(map[string]any{
		strings.Repeat("k", 512): map[string]any{"b": map[string]any{"c": make([]any, 253)}},
	})
	requestNamed(t, in, "Create pet").AuthData = bearerWith(64)

	addVariables(&in, 1000)
	in.Variables[len(in.Variables)-1].Key = name
	in.Variables[len(in.Variables)-1].Value = strings.Repeat("x", 64<<10)

	s, report := buildSnapshot(t, in)
	require.Empty(t, report.Errors)
	out, err := snapshotjson.Marshal(s)
	require.NoError(t, err)

	validate(t, compileSchema(t), out)
}

func TestBuild_ReportsWhatTheServerSchemaRejects(t *testing.T) {
	sch := compileSchema(t)
	cases := map[string]func(t *testing.T, in *publication.BuildInput){
		"request name": func(t *testing.T, in *publication.BuildInput) {
			requestNamed(t, *in, "Ping").Name = strings.Repeat("я", 513)
		},
		"header name": func(t *testing.T, in *publication.BuildInput) {
			requestNamed(t, *in, "Ping").Headers = rows(1, func(int) string { return strings.Repeat("a", 513) }, "1")
		},
		"header value": func(t *testing.T, in *publication.BuildInput) {
			requestNamed(t, *in, "Ping").Headers = rows(1, func(int) string { return "X-A" }, strings.Repeat("x", 16<<10+1))
		},
		"variable value": func(t *testing.T, in *publication.BuildInput) {
			in.Variables[0].Value = strings.Repeat("x", 64<<10+1)
		},
		"headers": func(t *testing.T, in *publication.BuildInput) {
			requestNamed(t, *in, "Ping").Headers = rows(201, func(i int) string { return fmt.Sprintf("X-%d", i) }, "1")
		},
		"form fields": func(t *testing.T, in *publication.BuildInput) {
			requestNamed(t, *in, "Загрузить фото").Body = formFields(201, "k", "1")
		},
		"examples": func(t *testing.T, in *publication.BuildInput) {
			addExamples(in, requestNamed(t, *in, "Ping"), 51)
		},
		"subprotocols": func(t *testing.T, in *publication.BuildInput) {
			requestNamed(t, *in, "Чат питомника").Body = wsSettings(65, 0)
		},
		"messages": func(t *testing.T, in *publication.BuildInput) {
			requestNamed(t, *in, "Чат питомника").Body = wsSettings(0, 201)
		},
		"variables": func(t *testing.T, in *publication.BuildInput) { addVariables(in, 1001) },
		"auth fields": func(t *testing.T, in *publication.BuildInput) {
			requestNamed(t, *in, "Create pet").AuthData = bearerWith(65)
		},
		"auth depth": func(t *testing.T, in *publication.BuildInput) {
			requestNamed(t, *in, "Check pet exists").AuthData = jwtWithClaims(map[string]any{"a": map[string]any{"b": map[string]any{"c": []any{[]any{1}}}}})
		},
		"auth object keys": func(t *testing.T, in *publication.BuildInput) {
			requestNamed(t, *in, "Check pet exists").AuthData = jwtWithClaims(claimKeys(257))
		},
		"auth key": func(t *testing.T, in *publication.BuildInput) {
			requestNamed(t, *in, "Check pet exists").AuthData = jwtWithClaims(map[string]any{strings.Repeat("k", 513): 1})
		},
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			in := petstore()
			mutate(t, &in)
			s, report := buildSnapshot(t, in)
			out, err := snapshotjson.Marshal(s)
			require.NoError(t, err)

			assert.NotEmpty(t, report.Errors)
			assert.False(t, schemaAccepts(t, sch, out))
		})
	}
}
