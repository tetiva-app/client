package auth

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestParseFields(t *testing.T) {
	tests := []struct {
		name    string
		raw     string
		wantLen int
		wantErr bool
	}{
		{name: "empty string", raw: "", wantLen: 0},
		{name: "empty object", raw: "{}", wantLen: 0},
		{name: "whitespace", raw: "  \n", wantLen: 0},
		{name: "null", raw: "null", wantLen: 0},
		{name: "flat", raw: `{"username":"admin","password":"secret"}`, wantLen: 2},
		{name: "nested", raw: `{"claims":{"sub":"42","roles":["a","b"]},"alg":"HS256"}`, wantLen: 2},
		{name: "broken json", raw: `{"username":`, wantErr: true},
		{name: "array", raw: `["a"]`, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f, err := ParseFields(tt.raw)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error for %q", tt.raw)
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseFields(%q): %v", tt.raw, err)
			}
			if len(f) != tt.wantLen {
				t.Fatalf("len: got %d, want %d", len(f), tt.wantLen)
			}
		})
	}
}

func TestFieldsAccessors(t *testing.T) {
	f, err := ParseFields(`{"alg":"HS256","claims":{"sub":"42"},"expiresIn":3600,"flag":true}`)
	if err != nil {
		t.Fatalf("ParseFields: %v", err)
	}
	if got := f.Str("alg"); got != "HS256" {
		t.Errorf("Str(alg): got %q", got)
	}
	if got := f.Str("missing"); got != "" {
		t.Errorf("Str(missing): got %q, want empty", got)
	}
	if got := f.Str("expiresIn"); got != "" {
		t.Errorf("Str on a number must be empty, got %q", got)
	}
	if got := f.Obj("claims")["sub"]; got != "42" {
		t.Errorf("Obj(claims)[sub]: got %v", got)
	}
	if f.Obj("alg") != nil {
		t.Error("Obj on a string must be nil")
	}
}

func TestSubstituteLeavesAtAnyDepth(t *testing.T) {
	f, err := ParseFields(`{
		"clientId":"{{id}}",
		"claims":{"sub":"{{user}}","nested":{"aud":"{{aud}}"},"roles":["{{role}}","fixed"]},
		"expiresIn":3600,
		"{{key}}":"{{id}}"
	}`)
	if err != nil {
		t.Fatalf("ParseFields: %v", err)
	}
	vars := map[string]string{"id": "abc", "user": "u-1", "aud": "api", "role": "admin", "key": "renamed"}

	got := Substitute(f, vars)

	if got.Str("clientId") != "abc" {
		t.Errorf("clientId: got %q", got.Str("clientId"))
	}
	claims := got.Obj("claims")
	if claims["sub"] != "u-1" {
		t.Errorf("claims.sub: got %v", claims["sub"])
	}
	if nested, _ := claims["nested"].(map[string]any); nested["aud"] != "api" {
		t.Errorf("claims.nested.aud: got %v", nested["aud"])
	}
	roles, _ := claims["roles"].([]any)
	if len(roles) != 2 || roles[0] != "admin" || roles[1] != "fixed" {
		t.Errorf("claims.roles: got %v", roles)
	}
	if got["expiresIn"] != float64(3600) {
		t.Errorf("non-string leaf changed: %v", got["expiresIn"])
	}
	if _, ok := got["{{key}}"]; !ok {
		t.Errorf("keys must stay untouched: %v", got)
	}
}

func TestSubstituteKeepsUnresolvedAndSurvivesQuotes(t *testing.T) {
	f, err := ParseFields(`{"username":"admin","password":"{{pw}}","token":"{{missing}}"}`)
	if err != nil {
		t.Fatalf("ParseFields: %v", err)
	}
	got := Substitute(f, map[string]string{"pw": `p"a}}ss\n`})

	if got.Str("password") != `p"a}}ss\n` {
		t.Errorf("quoted value corrupted: %q", got.Str("password"))
	}
	if got.Str("token") != "{{missing}}" {
		t.Errorf("unresolved variable must survive: %q", got.Str("token"))
	}
	if _, err := json.Marshal(got); err != nil {
		t.Fatalf("substituted fields must still marshal: %v", err)
	}
}

func TestSubstituteDoesNotMutateInput(t *testing.T) {
	f, err := ParseFields(`{"clientId":"{{id}}","claims":{"sub":"{{id}}"}}`)
	if err != nil {
		t.Fatalf("ParseFields: %v", err)
	}
	before := map[string]any{"clientId": "{{id}}", "claims": map[string]any{"sub": "{{id}}"}}

	_ = Substitute(f, map[string]string{"id": "abc"})

	if !reflect.DeepEqual(map[string]any(f), before) {
		t.Errorf("input mutated: %v", f)
	}
}
