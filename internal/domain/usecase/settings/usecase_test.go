package settings

import (
	"context"
	"testing"
)

// fakeRepo is an in-memory Repository for tests.
type fakeRepo struct {
	data map[string]string
}

func newFakeRepo() *fakeRepo { return &fakeRepo{data: map[string]string{}} }

func (r *fakeRepo) Get(_ context.Context, key string) (string, bool, error) {
	v, ok := r.data[key]
	return v, ok, nil
}
func (r *fakeRepo) Set(_ context.Context, key, value string) error {
	r.data[key] = value
	return nil
}

func TestGetMCPConfig_Defaults(t *testing.T) {
	uc := NewUsecase(newFakeRepo())
	cfg, err := uc.GetMCPConfig(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Enabled {
		t.Errorf("expected disabled by default, got enabled")
	}
	if cfg.Addr != DefaultMCPAddr {
		t.Errorf("expected addr %q, got %q", DefaultMCPAddr, cfg.Addr)
	}
}

func TestSetThenGetMCPConfig(t *testing.T) {
	uc := NewUsecase(newFakeRepo())
	ctx := context.Background()
	if err := uc.SetMCPConfig(ctx, MCPConfig{Enabled: true, Addr: ":9400"}); err != nil {
		t.Fatal(err)
	}
	cfg, err := uc.GetMCPConfig(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.Enabled || cfg.Addr != ":9400" {
		t.Errorf("got %+v", cfg)
	}
}

func TestGetMCPConfig_EmptyAddrFallsBackToDefault(t *testing.T) {
	repo := newFakeRepo()
	repo.data[keyMCPAddr] = ""
	uc := NewUsecase(repo)
	cfg, err := uc.GetMCPConfig(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Addr != DefaultMCPAddr {
		t.Errorf("expected fallback to %q, got %q", DefaultMCPAddr, cfg.Addr)
	}
}
