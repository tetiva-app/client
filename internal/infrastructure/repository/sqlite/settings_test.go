package sqlite

import (
	"context"
	"testing"
)

func TestSettingsRepo_GetMissing(t *testing.T) {
	db := setupTestDB(t)
	repo := NewSettingsRepo(db)
	_, found, err := repo.Get(context.Background(), "nope")
	if err != nil {
		t.Fatal(err)
	}
	if found {
		t.Errorf("expected not found")
	}
}

func TestSettingsRepo_SetGetUpsert(t *testing.T) {
	db := setupTestDB(t)
	repo := NewSettingsRepo(db)
	ctx := context.Background()

	if err := repo.Set(ctx, "mcp.addr", ":9300"); err != nil {
		t.Fatal(err)
	}
	v, found, err := repo.Get(ctx, "mcp.addr")
	if err != nil {
		t.Fatal(err)
	}
	if !found || v != ":9300" {
		t.Errorf("got found=%v value=%q", found, v)
	}

	if err := repo.Set(ctx, "mcp.addr", ":9400"); err != nil {
		t.Fatal(err)
	}
	v, _, _ = repo.Get(ctx, "mcp.addr")
	if v != ":9400" {
		t.Errorf("expected upsert to :9400, got %q", v)
	}
}
