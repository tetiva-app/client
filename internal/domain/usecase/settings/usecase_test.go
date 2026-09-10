package settings

import (
	"context"
	"errors"
	"testing"
)

// fakeRepo is an in-memory Repository for tests. SetMany is all-or-nothing, the
// contract the SQLite transaction gives the usecase.
type fakeRepo struct {
	data        map[string]string
	setCalls    int
	setManyErr  error
	setManyKeys []map[string]string
}

func newFakeRepo() *fakeRepo { return &fakeRepo{data: map[string]string{}} }

func (r *fakeRepo) Get(_ context.Context, key string) (string, bool, error) {
	v, ok := r.data[key]
	return v, ok, nil
}
func (r *fakeRepo) Set(_ context.Context, key, value string) error {
	r.setCalls++
	r.data[key] = value
	return nil
}
func (r *fakeRepo) SetMany(_ context.Context, values map[string]string) error {
	r.setManyKeys = append(r.setManyKeys, values)
	if r.setManyErr != nil {
		return r.setManyErr
	}
	for k, v := range values {
		r.data[k] = v
	}
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

func TestEnsureMCPToken_GeneratesOnceAndSurvivesConfigSave(t *testing.T) {
	repo := newFakeRepo()
	uc := NewUsecase(repo)
	ctx := context.Background()

	first, err := uc.EnsureMCPToken(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(first) < 40 {
		t.Errorf("token looks too short: %q", first)
	}

	second, err := uc.EnsureMCPToken(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if second != first {
		t.Errorf("token rotated on second call: %q -> %q", first, second)
	}

	// SetMCPConfig writes the whole config; the token must not be collateral damage.
	if err := uc.SetMCPConfig(ctx, MCPConfig{Enabled: true, Addr: ":9400"}); err != nil {
		t.Fatal(err)
	}
	after, err := uc.GetMCPToken(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if after != first {
		t.Errorf("SetMCPConfig clobbered the token: %q -> %q", first, after)
	}
}

func TestRegenerateMCPToken_ReplacesValue(t *testing.T) {
	uc := NewUsecase(newFakeRepo())
	ctx := context.Background()

	first, err := uc.EnsureMCPToken(ctx)
	if err != nil {
		t.Fatal(err)
	}
	second, err := uc.RegenerateMCPToken(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if second == first || second == "" {
		t.Errorf("expected a fresh token, got %q", second)
	}
}

func TestGetMCPToken_AbsentIsEmpty(t *testing.T) {
	uc := NewUsecase(newFakeRepo())
	token, err := uc.GetMCPToken(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if token != "" {
		t.Errorf("expected empty token, got %q", token)
	}
}

func TestMCPRequireToken_DefaultsToOn(t *testing.T) {
	repo := newFakeRepo()
	uc := NewUsecase(repo)
	ctx := context.Background()

	require, err := uc.GetMCPRequireToken(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !require {
		t.Error("expected the token to be required by default")
	}

	if err := uc.SetMCPConfig(ctx, MCPConfig{Enabled: true, Addr: ":9400", RequireToken: false}); err != nil {
		t.Fatal(err)
	}
	require, err = uc.GetMCPRequireToken(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if require {
		t.Error("expected the requirement to be off after opting out")
	}
}

// Enabling the server and requiring a token are one decision: a crash between
// two writes would bring the server up on the new address with the old guard.
func TestSetMCPConfig_WritesEveryKeyInOneCall(t *testing.T) {
	repo := newFakeRepo()
	uc := NewUsecase(repo)

	if err := uc.SetMCPConfig(context.Background(), MCPConfig{
		Enabled: true, Addr: ":9400", RequireToken: true,
	}); err != nil {
		t.Fatal(err)
	}

	if repo.setCalls != 0 {
		t.Errorf("config written key by key: %d individual Set calls", repo.setCalls)
	}
	if len(repo.setManyKeys) != 1 {
		t.Fatalf("expected exactly one atomic write, got %d", len(repo.setManyKeys))
	}
	written := repo.setManyKeys[0]
	for _, key := range []string{keyMCPEnabled, keyMCPAddr, keyMCPRequireToken} {
		if _, ok := written[key]; !ok {
			t.Errorf("%q missing from the atomic write: %v", key, written)
		}
	}
}

func TestSetMCPConfig_FailedWriteLeavesNothingBehind(t *testing.T) {
	repo := newFakeRepo()
	repo.setManyErr = errors.New("disk full")
	uc := NewUsecase(repo)

	err := uc.SetMCPConfig(context.Background(), MCPConfig{
		Enabled: true, Addr: ":9400", RequireToken: true,
	})
	if err == nil {
		t.Fatal("expected the write to fail")
	}
	if len(repo.data) != 0 {
		t.Errorf("partial config survived a failed write: %v", repo.data)
	}
}

func TestGetMCPConfig_CarriesRequireToken(t *testing.T) {
	repo := newFakeRepo()
	uc := NewUsecase(repo)
	ctx := context.Background()

	cfg, err := uc.GetMCPConfig(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.RequireToken {
		t.Error("expected the token to be required by default")
	}

	if err := uc.SetMCPConfig(ctx, MCPConfig{Enabled: true, Addr: ":9400", RequireToken: false}); err != nil {
		t.Fatal(err)
	}
	cfg, err = uc.GetMCPConfig(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.RequireToken {
		t.Error("expected the stored requirement to come back off")
	}
}
