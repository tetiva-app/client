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
