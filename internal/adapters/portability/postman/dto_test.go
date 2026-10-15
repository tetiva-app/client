package postman_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tetiva-app/client/internal/adapters/portability/postman"
)

func TestPostmanURL_UnmarshalJSON(t *testing.T) {
	tests := []struct {
		name string
		url  string
		want postman.PostmanURL
	}{
		{
			name: "string",
			url:  `"https://api.example.com/users?page=2"`,
			want: postman.PostmanURL{Raw: "https://api.example.com/users?page=2"},
		},
		{
			name: "object",
			url:  `{"raw":"{{host}}/users?page=2","host":["{{host}}"],"path":["users"],"query":[{"key":"page","value":"2"}]}`,
			want: postman.PostmanURL{
				Raw:   "{{host}}/users?page=2",
				Host:  []string{"{{host}}"},
				Path:  []string{"users"},
				Query: []postman.PostmanKV{{Key: "page", Value: "2"}},
			},
		},
		{
			name: "null",
			url:  `null`,
			want: postman.PostmanURL{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var req postman.PostmanRequest
			require.NoError(t, json.Unmarshal([]byte(`{"method":"GET","url":`+tt.url+`}`), &req))
			assert.Equal(t, tt.want, req.URL)
		})
	}
}

func TestPostmanURL_UnmarshalJSONRejectsOtherTypes(t *testing.T) {
	var u postman.PostmanURL
	err := json.Unmarshal([]byte(`42`), &u)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "postman.PostmanURL.UnmarshalJSON")
}
