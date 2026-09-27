package request_test

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tetiva-app/client/internal/domain/usecase/request"
)

func decodeBody(t *testing.T, body string) map[string]any {
	t.Helper()
	dec := json.NewDecoder(bytes.NewReader([]byte(body)))
	dec.UseNumber()
	var got map[string]any
	require.NoError(t, dec.Decode(&got))
	return got
}

func TestGraphQLBody(t *testing.T) {
	tests := []struct {
		name      string
		query     string
		variables string
		operation string
		want      map[string]any
	}{
		{
			name:  "query_only",
			query: "{ me { id } }",
			want:  map[string]any{"query": "{ me { id } }"},
		},
		{
			name:      "empty_object_variables_omitted",
			query:     "{ me { id } }",
			variables: "{}",
			want:      map[string]any{"query": "{ me { id } }"},
		},
		{
			name:      "blank_variables_omitted",
			query:     "{ me { id } }",
			variables: " \n",
			want:      map[string]any{"query": "{ me { id } }"},
		},
		{
			name:      "variables_raw_json_kept",
			query:     "query Q($id: ID!) { user(id: $id) { name } }",
			variables: "{\n  \"id\": 12345678901234567890,\n  \"tags\": [\"a & b\"]\n}",
			want: map[string]any{
				"query": "query Q($id: ID!) { user(id: $id) { name } }",
				"variables": map[string]any{
					"id":   json.Number("12345678901234567890"),
					"tags": []any{"a & b"},
				},
			},
		},
		{
			name:      "operation_name_included",
			query:     "query GetUser { me { id } }",
			operation: "GetUser",
			want:      map[string]any{"query": "query GetUser { me { id } }", "operationName": "GetUser"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body, err := request.GraphQLBody(tt.query, tt.variables, tt.operation)
			require.NoError(t, err)
			assert.Equal(t, tt.want, decodeBody(t, body))
		})
	}

	t.Run("invalid_variables_error", func(t *testing.T) {
		body, err := request.GraphQLBody("{ me { id } }", `{"id": {{n}}}`, "")
		require.Error(t, err)
		assert.Empty(t, body)
	})
}

func TestGraphQLBodyLenient(t *testing.T) {
	t.Run("lenient_invalid_variables_text_and_warning", func(t *testing.T) {
		body, warnings := request.GraphQLBodyLenient(
			"query Q($n: Int!) { item(n: $n) { name } }", `{"n": {{n}}}`, "Q")
		assert.Equal(t,
			`{"query":"query Q($n: Int!) { item(n: $n) { name } }","variables":{"n": {{n}}},"operationName":"Q"}`,
			body)
		assert.Equal(t, []string{"GraphQL variables are not valid JSON; shown as written"}, warnings)
	})

	t.Run("html_characters_stay_literal", func(t *testing.T) {
		const want = `{"query":"{ find(q: \"a & b <c>\") { id } }","variables":{"q":"a & b"},"operationName":"<Op>"}`

		strict, err := request.GraphQLBody(`{ find(q: "a & b <c>") { id } }`, `{"q":"a & b"}`, "<Op>")
		require.NoError(t, err)
		lenient, _ := request.GraphQLBodyLenient(`{ find(q: "a & b <c>") { id } }`, `{"q":"a & b"}`, "<Op>")

		assert.Equal(t, want, strict)
		assert.Equal(t, want, lenient)
	})

	t.Run("lenient_valid_variables_no_warning", func(t *testing.T) {
		body, warnings := request.GraphQLBodyLenient("{ me { id } }", `{ "n": 1 }`, "")
		assert.Equal(t, `{"query":"{ me { id } }","variables":{"n":1}}`, body)
		assert.Empty(t, warnings)
	})
}
