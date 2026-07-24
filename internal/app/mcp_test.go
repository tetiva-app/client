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

func TestResolveMCPConfig_DBValuesNoEnv(t *testing.T) {
	unsetEnv(t, "GOPHERCOURIER_MCP")
	unsetEnv(t, "GOPHERCOURIER_MCP_ADDR")
	uc := newUC(map[string]string{"mcp.enabled": "true", "mcp.addr": ":9400"})
	st, err := resolveMCPConfig(uc)
	if err != nil {
		t.Fatal(err)
	}
	if !st.Enabled || st.Addr != ":9400" || st.EnvManaged {
		t.Errorf("got %+v", st)
	}
}

func TestResolveMCPConfig_EnvOverrides(t *testing.T) {
	t.Setenv("GOPHERCOURIER_MCP", "1")
	t.Setenv("GOPHERCOURIER_MCP_ADDR", ":9500")
	uc := newUC(map[string]string{"mcp.enabled": "false", "mcp.addr": ":9400"})
	st, err := resolveMCPConfig(uc)
	if err != nil {
		t.Fatal(err)
	}
	if !st.Enabled || st.Addr != ":9500" || !st.EnvManaged {
		t.Errorf("expected env override, got %+v", st)
	}
}

func TestResolveMCPConfig_DefaultsWhenEmpty(t *testing.T) {
	unsetEnv(t, "GOPHERCOURIER_MCP")
	unsetEnv(t, "GOPHERCOURIER_MCP_ADDR")
	st, err := resolveMCPConfig(newUC(map[string]string{}))
	if err != nil {
		t.Fatal(err)
	}
	if st.Enabled || st.Addr != settings.DefaultMCPAddr || st.EnvManaged {
		t.Errorf("got %+v", st)
	}
}
