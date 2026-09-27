package publication

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/tetiva-app/client/internal/domain/entities"
)

func TestAuthFieldsFixtureIsCurrent(t *testing.T) {
	public := map[entities.AuthType][]string{}
	secret := map[entities.AuthType][]string{}
	for _, at := range entities.ValidAuthTypes() {
		fields := authFieldTable[at]
		public[at] = append([]string{}, slices.Sorted(slices.Values(fields.public))...)
		secret[at] = append([]string{}, slices.Sorted(slices.Values(fields.secret))...)
		for _, k := range fields.public {
			if slices.Contains(fields.secret, k) {
				t.Errorf("%s.%s is listed as both public and secret", at, k)
			}
		}
	}
	for at := range authFieldTable {
		if !at.IsValid() {
			t.Errorf("authFieldTable lists unknown type %q", at)
		}
	}

	path := filepath.Join("testdata", "auth_fields.json")
	want, err := json.MarshalIndent(map[string]map[entities.AuthType][]string{"public": public, "secret": secret}, "", "  ")
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
	t.Fatalf("%s regenerated from authFieldTable; commit it", path)
}
