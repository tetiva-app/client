package entities

import (
	"encoding/json"
	"os"
	"slices"
	"testing"
)

type pathVariableCases struct {
	Keys []struct {
		URL  string   `json:"url"`
		Keys []string `json:"keys"`
	} `json:"keys"`
	Apply []struct {
		URL  string `json:"url"`
		Vars []struct {
			Key   string `json:"key"`
			Value string `json:"value"`
		} `json:"vars"`
		Want string `json:"want"`
	} `json:"apply"`
}

func loadPathVariableCases(t *testing.T) pathVariableCases {
	t.Helper()
	raw, err := os.ReadFile("../../../testdata/path-variables.json")
	if err != nil {
		t.Fatal(err)
	}
	var cases pathVariableCases
	if err := json.Unmarshal(raw, &cases); err != nil {
		t.Fatal(err)
	}
	return cases
}

func TestPathVariableKeys(t *testing.T) {
	for _, tc := range loadPathVariableCases(t).Keys {
		got := PathVariableKeys(tc.URL)
		if !slices.Equal(got, tc.Keys) {
			t.Errorf("PathVariableKeys(%q) = %q, want %q", tc.URL, got, tc.Keys)
		}
	}
}

func TestApplyPathVariables(t *testing.T) {
	for _, tc := range loadPathVariableCases(t).Apply {
		vars := make([]PathVariable, 0, len(tc.Vars))
		for _, v := range tc.Vars {
			vars = append(vars, PathVariable{Key: v.Key, Value: v.Value})
		}
		if got := ApplyPathVariables(tc.URL, vars); got != tc.Want {
			t.Errorf("ApplyPathVariables(%q) = %q, want %q", tc.URL, got, tc.Want)
		}
	}
}

func TestSyncPathVariables(t *testing.T) {
	got := SyncPathVariables("https://api.io/orgs/:org/users/:id/:org", []PathVariable{
		{Key: "id", Value: "42"},
		{Key: "gone", Value: "x"},
		{Key: "id", Value: "second"},
	})
	want := []PathVariable{{Key: "org"}, {Key: "id", Value: "42"}}
	if !slices.Equal(got, want) {
		t.Fatalf("SyncPathVariables = %v, want %v", got, want)
	}

	if got := SyncPathVariables("https://api.io/users", nil); got == nil || len(got) != 0 {
		t.Fatalf("a URL without variables must give an empty, non-nil list: %#v", got)
	}
}

func TestProtocolHasPathVariables(t *testing.T) {
	for _, p := range ValidProtocols() {
		want := p == ProtocolHTTP || p == ProtocolWebSocket
		if p.HasPathVariables() != want {
			t.Errorf("%s.HasPathVariables() = %v, want %v", p, !want, want)
		}
	}
}
