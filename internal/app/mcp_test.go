package app

import (
	"context"
	"os"
	"testing"

	"github.com/tetiva-app/client/internal/domain/usecase/settings"
)

type fakeSettingsRepo struct{ data map[string]string }

func (r *fakeSettingsRepo) Get(_ context.Context, k string) (string, bool, error) {
	v, ok := r.data[k]
	return v, ok, nil
}
func (r *fakeSettingsRepo) Set(_ context.Context, k, v string) error {
	r.data[k] = v
	return nil
}

func newUC(data map[string]string) settings.Usecase {
	return settings.NewUsecase(&fakeSettingsRepo{data: data})
}

// unsetEnv removes key for the duration of the test, restoring it on cleanup.
func unsetEnv(t *testing.T, key string) {
	t.Helper()
	orig, had := os.LookupEnv(key)
	_ = os.Unsetenv(key)
	if had {
		t.Cleanup(func() { _ = os.Setenv(key, orig) })
	}
}

// clearMCPEnv drops both the Tetiva and legacy GopherCourier variables so a
// developer's own MCP env cannot leak into the resolution tests.
func clearMCPEnv(t *testing.T) {
	t.Helper()
	for _, k := range []string{"TETIVA_MCP", "TETIVA_MCP_ADDR", "GOPHERCOURIER_MCP", "GOPHERCOURIER_MCP_ADDR"} {
		unsetEnv(t, k)
	}
}

func TestResolveMCPConfig_DBValuesNoEnv(t *testing.T) {
	clearMCPEnv(t)
	uc := newUC(map[string]string{"mcp.enabled": "true", "mcp.addr": "127.0.0.1:9400"})
	st, err := resolveMCPConfig(uc)
	if err != nil {
		t.Fatal(err)
	}
	if !st.Enabled || st.Addr != "127.0.0.1:9400" || st.EnvManaged {
		t.Errorf("got %+v", st)
	}
}

func TestResolveMCPConfig_EnvOverrides(t *testing.T) {
	clearMCPEnv(t)
	t.Setenv("GOPHERCOURIER_MCP", "1")
	t.Setenv("GOPHERCOURIER_MCP_ADDR", "127.0.0.1:9500")
	uc := newUC(map[string]string{"mcp.enabled": "false", "mcp.addr": "127.0.0.1:9400"})
	st, err := resolveMCPConfig(uc)
	if err != nil {
		t.Fatal(err)
	}
	if !st.Enabled || st.Addr != "127.0.0.1:9500" || !st.EnvManaged {
		t.Errorf("expected env override, got %+v", st)
	}
}

func TestResolveMCPConfig_DefaultsWhenEmpty(t *testing.T) {
	clearMCPEnv(t)
	st, err := resolveMCPConfig(newUC(map[string]string{}))
	if err != nil {
		t.Fatal(err)
	}
	if st.Enabled || st.Addr != settings.DefaultMCPAddr || st.EnvManaged {
		t.Errorf("got %+v", st)
	}
}

func TestResolveMCPConfig_DefaultIsLoopback(t *testing.T) {
	clearMCPEnv(t)
	st, err := resolveMCPConfig(newUC(map[string]string{}))
	if err != nil {
		t.Fatal(err)
	}
	if st.Addr != "127.0.0.1:9300" {
		t.Errorf("default must bind loopback only, got %q", st.Addr)
	}
}

// Pre-fix installs persisted ":9300", which binds every interface.
func TestResolveMCPConfig_NormalizesPersistedBarePort(t *testing.T) {
	clearMCPEnv(t)
	uc := newUC(map[string]string{"mcp.enabled": "true", "mcp.addr": ":9300"})
	st, err := resolveMCPConfig(uc)
	if err != nil {
		t.Fatal(err)
	}
	if st.Addr != "127.0.0.1:9300" {
		t.Errorf("persisted bare port must resolve to loopback, got %q", st.Addr)
	}
}

func TestResolveMCPConfig_NormalizesEnvBarePort(t *testing.T) {
	clearMCPEnv(t)
	t.Setenv("TETIVA_MCP_ADDR", ":9400")
	st, err := resolveMCPConfig(newUC(map[string]string{}))
	if err != nil {
		t.Fatal(err)
	}
	if st.Addr != "127.0.0.1:9400" {
		t.Errorf("env bare port must resolve to loopback, got %q", st.Addr)
	}
}

func TestResolveMCPConfig_KeepsExplicitHost(t *testing.T) {
	clearMCPEnv(t)
	for _, addr := range []string{"0.0.0.0:9300", "192.168.1.5:9300", "[::]:9300"} {
		uc := newUC(map[string]string{"mcp.addr": addr})
		st, err := resolveMCPConfig(uc)
		if err != nil {
			t.Fatal(err)
		}
		if st.Addr != addr {
			t.Errorf("explicit host %q must be kept, got %q", addr, st.Addr)
		}
	}
}
