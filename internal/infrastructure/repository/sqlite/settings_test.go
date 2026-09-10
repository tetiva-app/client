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

func TestSettingsRepo_SetMany(t *testing.T) {
	db := setupTestDB(t)
	repo := NewSettingsRepo(db)
	ctx := context.Background()

	if err := repo.SetMany(ctx, map[string]string{
		"mcp.enabled": "true", "mcp.addr": ":9400", "mcp.require_token": "true",
	}); err != nil {
		t.Fatal(err)
	}
	for key, want := range map[string]string{
		"mcp.enabled": "true", "mcp.addr": ":9400", "mcp.require_token": "true",
	} {
		v, found, err := repo.Get(ctx, key)
		if err != nil {
			t.Fatal(err)
		}
		if !found || v != want {
			t.Errorf("%s: got found=%v value=%q, want %q", key, found, v, want)
		}
	}
}

// A rejected key must take the whole batch down, or the config comes back half old and
// half new after a restart; the trigger aborts the second write whichever key goes first.
func TestSettingsRepo_SetManyRollsBackOnFailure(t *testing.T) {
	db := setupTestDB(t)
	repo := NewSettingsRepo(db)
	ctx := context.Background()

	if err := repo.Set(ctx, "mcp.addr", ":9300"); err != nil {
		t.Fatal(err)
	}
	if err := repo.Set(ctx, "mcp.enabled", "false"); err != nil {
		t.Fatal(err)
	}
	for _, stmt := range []string{
		`CREATE TABLE writes (n INTEGER NOT NULL)`,
		`INSERT INTO writes VALUES (0)`,
		`CREATE TRIGGER abort_second BEFORE INSERT ON app_settings BEGIN
			UPDATE writes SET n = n + 1;
			SELECT RAISE(ABORT, 'second write rejected') WHERE (SELECT n FROM writes) > 1;
		END`,
	} {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatal(err)
		}
	}

	err := repo.SetMany(ctx, map[string]string{"mcp.addr": ":9400", "mcp.enabled": "true"})
	if err == nil {
		t.Fatal("expected the batch to fail")
	}
	for key, want := range map[string]string{"mcp.addr": ":9300", "mcp.enabled": "false"} {
		v, _, _ := repo.Get(ctx, key)
		if v != want {
			t.Errorf("failed batch left a partial write: %s = %q, want %q", key, v, want)
		}
	}
}
