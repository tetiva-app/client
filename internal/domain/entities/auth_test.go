package entities

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestAuthTypeIsValid(t *testing.T) {
	for _, at := range ValidAuthTypes() {
		if !at.IsValid() {
			t.Errorf("%q is in the registry but IsValid says otherwise", at)
		}
	}
	for _, at := range []AuthType{"", "hawk", "ntlm", "OAuth2"} {
		if AuthType(at).IsValid() {
			t.Errorf("%q must not be valid", at)
		}
	}
}

func TestValidAuthTypesIncludesNewSchemes(t *testing.T) {
	want := map[AuthType]bool{
		AuthTypeOAuth2: false, AuthTypeJWT: false, AuthTypeDigest: false, AuthTypeAWSSigV4: false,
	}
	for _, at := range ValidAuthTypes() {
		if _, ok := want[at]; ok {
			want[at] = true
		}
	}
	for at, found := range want {
		if !found {
			t.Errorf("%q missing from ValidAuthTypes()", at)
		}
	}
}

func TestValidCollectionAuthTypesExcludesInherit(t *testing.T) {
	types := ValidCollectionAuthTypes()
	if len(types) != len(ValidAuthTypes())-1 {
		t.Fatalf("collection types: got %d, want %d", len(types), len(ValidAuthTypes())-1)
	}
	for _, at := range types {
		if at == AuthTypeInherit {
			t.Fatal("inherit must not be offered to collections")
		}
	}
}

// The frontend registry test (constants/auth.ts) reads this fixture, so the file
// is regenerated here and committed whenever the Go registry changes.
func TestAuthTypesFixtureIsCurrent(t *testing.T) {
	path := filepath.Join("testdata", "auth_types.json")
	want, err := json.MarshalIndent(map[string][]AuthType{
		"authTypes":           ValidAuthTypes(),
		"collectionAuthTypes": ValidCollectionAuthTypes(),
	}, "", "  ")
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	want = append(want, '\n')

	got, readErr := os.ReadFile(path)
	if readErr == nil && bytes.Equal(got, want) {
		return
	}
	if mkErr := os.MkdirAll("testdata", 0o755); mkErr != nil {
		t.Fatalf("mkdir testdata: %v", mkErr)
	}
	if writeErr := os.WriteFile(path, want, 0o644); writeErr != nil {
		t.Fatalf("write %s: %v", path, writeErr)
	}
	t.Fatalf("%s regenerated from ValidAuthTypes(); commit it", path)
}
