package sqlite

import (
	"context"
	"testing"
)

func TestSyncConfigRepo_GetOrCreate(t *testing.T) {
	db := setupTestDB(t)
	repo := NewSyncConfigRepo(db)
	ctx := context.Background()

	cfg, err := repo.GetOrCreate(ctx)
	if err != nil {
		t.Fatalf("GetOrCreate (first) failed: %v", err)
	}
	if cfg == nil {
		t.Fatal("GetOrCreate returned nil config")
	}
	if cfg.ClientID == "" {
		t.Error("ClientID should not be empty after GetOrCreate")
	}
	if cfg.Enabled {
		t.Error("Enabled should be false by default")
	}

	firstClientID := cfg.ClientID

	cfg2, err := repo.GetOrCreate(ctx)
	if err != nil {
		t.Fatalf("GetOrCreate (second) failed: %v", err)
	}
	if cfg2 == nil {
		t.Fatal("GetOrCreate second call returned nil config")
	}
	if cfg2.ClientID != firstClientID {
		t.Errorf("ClientID changed between calls: got %q, want %q", cfg2.ClientID, firstClientID)
	}
}

func TestSyncConfigRepo_Update(t *testing.T) {
	db := setupTestDB(t)
	repo := NewSyncConfigRepo(db)
	ctx := context.Background()

	_, err := repo.GetOrCreate(ctx)
	if err != nil {
		t.Fatalf("GetOrCreate failed: %v", err)
	}

	original, err := repo.Get(ctx)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}

	updated := &SyncConfig{
		ClientID:  original.ClientID,
		ServerURL: "https://sync.example.com",
		UserEmail: "user@example.com",
		Enabled:   true,
	}

	if err := repo.Update(ctx, updated); err != nil {
		t.Fatalf("Update failed: %v", err)
	}

	got, err := repo.Get(ctx)
	if err != nil {
		t.Fatalf("Get after Update failed: %v", err)
	}
	if got == nil {
		t.Fatal("Get after Update returned nil")
	}

	if got.ServerURL != "https://sync.example.com" {
		t.Errorf("ServerURL mismatch: got %q, want %q", got.ServerURL, "https://sync.example.com")
	}
	if got.UserEmail != "user@example.com" {
		t.Errorf("UserEmail mismatch: got %q, want %q", got.UserEmail, "user@example.com")
	}
	if !got.Enabled {
		t.Error("Enabled should be true after Update")
	}
	if got.ClientID != original.ClientID {
		t.Errorf("ClientID changed on Update: got %q, want %q", got.ClientID, original.ClientID)
	}
}

func TestSyncConfigRepo_GetWhenNotExists(t *testing.T) {
	db := setupTestDB(t)
	repo := NewSyncConfigRepo(db)
	ctx := context.Background()

	cfg, err := repo.Get(ctx)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if cfg != nil {
		t.Errorf("Get should return nil when no config exists, got %+v", cfg)
	}
}

func TestSyncConfigRepo_GetOrCreate_AuthGenerationDefaults(t *testing.T) {
	db := setupTestDB(t)
	repo := NewSyncConfigRepo(db)
	ctx := context.Background()

	cfg, err := repo.GetOrCreate(ctx)
	if err != nil {
		t.Fatalf("GetOrCreate failed: %v", err)
	}
	if cfg.AuthGeneration != 0 {
		t.Errorf("AuthGeneration = %d, want 0", cfg.AuthGeneration)
	}
	if cfg.ReauthRequired {
		t.Error("ReauthRequired should be false on a fresh install")
	}
	if cfg.RefreshToken != "" {
		t.Errorf("RefreshToken = %q, want empty", cfg.RefreshToken)
	}
}

// A row written before migration 019 reads back with the new columns at their
// defaults, which is what the forced re-login keys on.
func TestSyncConfigRepo_Get_RowFromBeforeMigration(t *testing.T) {
	db := setupTestDB(t)
	repo := NewSyncConfigRepo(db)
	ctx := context.Background()

	_, err := db.ExecContext(ctx,
		`INSERT INTO sync_config (id, client_id, server_url, user_email, enabled) VALUES (1, 'client-1', 'sync.example.com', 'a@b.c', 1)`)
	if err != nil {
		t.Fatalf("insert legacy row: %v", err)
	}

	cfg, err := repo.Get(ctx)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if cfg.AuthGeneration != 0 || cfg.ReauthRequired {
		t.Errorf("legacy row read as generation=%d reauth=%v, want 0/false", cfg.AuthGeneration, cfg.ReauthRequired)
	}
	if !cfg.Enabled {
		t.Error("Enabled should survive the migration")
	}
}

func TestSyncConfigRepo_Update_WritesAuthGenerationAndReauth(t *testing.T) {
	db := setupTestDB(t)
	repo := NewSyncConfigRepo(db)
	ctx := context.Background()

	cfg, err := repo.GetOrCreate(ctx)
	if err != nil {
		t.Fatalf("GetOrCreate failed: %v", err)
	}

	cfg.AuthGeneration = 1
	cfg.ReauthRequired = true
	if err := repo.Update(ctx, cfg); err != nil {
		t.Fatalf("Update failed: %v", err)
	}

	got, err := repo.Get(ctx)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if got.AuthGeneration != 1 {
		t.Errorf("AuthGeneration = %d, want 1", got.AuthGeneration)
	}
	if !got.ReauthRequired {
		t.Error("ReauthRequired should be true after Update")
	}
}

// Regression: a partial read-modify-write (storeRefreshTokenDB on every refresh)
// must not zero the new columns and re-arm the forced re-login.
func TestSyncConfigRepo_Update_RefreshTokenKeepsAuthGeneration(t *testing.T) {
	db := setupTestDB(t)
	repo := NewSyncConfigRepo(db)
	ctx := context.Background()

	cfg, err := repo.GetOrCreate(ctx)
	if err != nil {
		t.Fatalf("GetOrCreate failed: %v", err)
	}
	cfg.AuthGeneration = 1
	cfg.ReauthRequired = false
	cfg.Enabled = true
	if err := repo.Update(ctx, cfg); err != nil {
		t.Fatalf("Update failed: %v", err)
	}

	reread, err := repo.Get(ctx)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	reread.RefreshToken = "refresh-2"
	if err := repo.Update(ctx, reread); err != nil {
		t.Fatalf("Update (refresh token) failed: %v", err)
	}

	got, err := repo.Get(ctx)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if got.RefreshToken != "refresh-2" {
		t.Errorf("RefreshToken = %q, want %q", got.RefreshToken, "refresh-2")
	}
	if got.AuthGeneration != 1 {
		t.Errorf("AuthGeneration = %d, want 1 after a refresh-token write", got.AuthGeneration)
	}
	if got.ReauthRequired {
		t.Error("ReauthRequired should stay false after a refresh-token write")
	}
}
