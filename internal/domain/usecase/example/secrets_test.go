package example_test

import (
	"reflect"
	"testing"

	"github.com/tetiva-app/client/internal/domain/entities"
	"github.com/tetiva-app/client/internal/domain/usecase/example"
)

func TestSuspectedSecrets(t *testing.T) {
	const jwt = "eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiIxIn0.c2lnbmF0dXJlLXZhbHVl"
	cases := []struct {
		name    string
		headers []entities.HeaderItem
		body    string
		want    []string
	}{
		{"plain", []entities.HeaderItem{{Key: "Content-Type", Value: "application/json"}}, `{"id":1}`, []string{}},
		{"token in body", nil, `{"access_token":"` + jwt + `"}`, []string{"JWT", "OAuth token"}},
		{"key in a readable header", []entities.HeaderItem{{Key: "X-Debug", Value: "AKIAIOSFODNN7EXAMPLE"}}, "", []string{"AWS access key"}},
		{"masked headers", []entities.HeaderItem{
			{Key: "Authorization", Value: "Bearer " + jwt},
			{Key: "Location", Value: "https://h/cb#id_token=" + jwt},
		}, "", []string{}},
	}
	for _, c := range cases {
		if got := example.SuspectedSecrets(c.headers, c.body); !reflect.DeepEqual(got, c.want) {
			t.Errorf("%s: got %v, want %v", c.name, got, c.want)
		}
	}
}
