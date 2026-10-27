package sync

import (
	"context"
	"testing"

	"github.com/zalando/go-keyring"

	"github.com/tetiva-app/client/internal/constants/flavortest"
)

func TestKeyring_PreviewStaysOutOfTheRegularAppsEntries(t *testing.T) {
	keyring.MockInit()
	flavortest.Preview(t)
	ctx := context.Background()
	repo := newTestSyncConfigRepo(setupTestDB(t))
	if _, err := repo.GetOrCreate(ctx); err != nil {
		t.Fatalf("GetOrCreate: %v", err)
	}
	account := keyringKeyFor("client-1")
	regular := map[[2]string]string{
		{keyringService, account}:                "regular",
		{keyringService, keyringLegacyKey}:       "regular-unscoped",
		{keyringLegacyService, account}:          "legacy",
		{keyringLegacyService, keyringLegacyKey}: "legacy-unscoped",
	}
	for k, v := range regular {
		if err := keyring.Set(k[0], k[1], v); err != nil {
			t.Fatal(err)
		}
	}

	mgr := NewSyncAuthManager(repo)
	mgr.SetClientID("client-1")

	if got := mgr.loadRefreshToken(); got != "" {
		t.Fatalf("preview read %q from the regular app's keychain", got)
	}

	mgr.storeTokens("access", "preview-refresh", "org-1")
	if got, err := keyring.Get(keyringPreviewService, account); err != nil || got != "preview-refresh" {
		t.Fatalf("preview token = %q, %v", got, err)
	}
	if got := mgr.loadRefreshToken(); got != "preview-refresh" {
		t.Fatalf("loadRefreshToken = %q", got)
	}

	mgr.deleteRefreshToken()
	if _, err := keyring.Get(keyringPreviewService, account); err == nil {
		t.Fatal("preview token survived the sign-out")
	}
	for k, v := range regular {
		if got, err := keyring.Get(k[0], k[1]); err != nil || got != v {
			t.Errorf("%s/%s = %q, %v; the regular app's entry must stay", k[0], k[1], got, err)
		}
	}
}

func TestKeyring_StableStillMigratesTheLegacyEntry(t *testing.T) {
	keyring.MockInit()
	repo := newTestSyncConfigRepo(setupTestDB(t))
	account := keyringKeyFor("client-1")
	if err := keyring.Set(keyringLegacyService, account, "legacy"); err != nil {
		t.Fatal(err)
	}

	mgr := NewSyncAuthManager(repo)
	mgr.SetClientID("client-1")

	if got := mgr.loadRefreshToken(); got != "legacy" {
		t.Fatalf("loadRefreshToken = %q", got)
	}
	if got, err := keyring.Get(keyringService, account); err != nil || got != "legacy" {
		t.Fatalf("migrated entry = %q, %v", got, err)
	}
}
