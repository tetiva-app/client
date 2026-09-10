package app

import (
	"context"
	"log/slog"
	"net"
	"os"

	"github.com/google/uuid"
	"go.uber.org/fx"

	mcpadapter "github.com/tetiva-app/client/internal/adapters/mcp"
	"github.com/tetiva-app/client/internal/domain/usecase/collection"
	"github.com/tetiva-app/client/internal/domain/usecase/environment"
	"github.com/tetiva-app/client/internal/domain/usecase/request"
	"github.com/tetiva-app/client/internal/domain/usecase/settings"
	"github.com/tetiva-app/client/internal/domain/usecase/workspace"
	"github.com/tetiva-app/client/internal/infrastructure/repository/sqlite"
	syncsvc "github.com/tetiva-app/client/internal/infrastructure/sync"
)

// resolveMCPConfig merges persisted settings with env vars, which win and mark the config env-managed.
func resolveMCPConfig(uc settings.Usecase) (*mcpadapter.RuntimeStatus, error) {
	// A DB read failure must not crash app boot — fall back to disabled defaults.
	// Runs during fx graph construction, after storage migrations, so app_settings exists.
	persisted, err := uc.GetMCPConfig(context.Background())
	if err != nil {
		slog.Error("MCP: failed to read settings, defaulting to disabled", "err", err)
		persisted = settings.MCPConfig{Enabled: false, Addr: settings.DefaultMCPAddr}
	}
	st := &mcpadapter.RuntimeStatus{
		Enabled: persisted.Enabled,
		Addr:    persisted.Addr,
	}
	if v, ok := lookupEnvWithLegacy("TETIVA_MCP", "GOPHERCOURIER_MCP"); ok {
		st.Enabled = v == "1"
		st.EnvManaged = true
	}
	if v := getEnvWithLegacy("TETIVA_MCP_ADDR", "GOPHERCOURIER_MCP_ADDR"); v != "" {
		st.Addr = v
		st.EnvManaged = true
	}
	if st.Addr == "" {
		st.Addr = settings.DefaultMCPAddr
	}
	// Single choke point for every source (persisted, env, default) — configs
	// written before the loopback default would otherwise still bind 0.0.0.0.
	st.Addr = settings.NormalizeMCPAddr(st.Addr)
	return st, nil
}

// resolveMCPAuth builds the token guard, generating the token on first launch.
// A storage failure falls back to an ephemeral token nobody can know.
func resolveMCPAuth(uc settings.Usecase) *mcpadapter.TokenAuth {
	ctx := context.Background()
	token, err := uc.EnsureMCPToken(ctx)
	if err != nil {
		slog.Error("MCP: failed to read token, refusing every client until restart", "err", err)
		return mcpadapter.NewTokenAuth(uuid.NewString(), true)
	}
	require, err := uc.GetMCPRequireToken(ctx)
	if err != nil {
		slog.Error("MCP: failed to read token requirement, defaulting to required", "err", err)
		require = true
	}
	return mcpadapter.NewTokenAuth(token, require)
}

// MCPModule is always wired, but the server only starts when the resolved config is enabled.
func MCPModule() fx.Option {
	return fx.Module("mcp",
		fx.Provide(resolveMCPConfig),
		fx.Provide(resolveMCPAuth),
		fx.Provide(func(
			engine *syncsvc.SyncEngine,
			syncQueue sqlite.SyncQueueRepository,
			colUC collection.Usecase,
			reqUC request.Usecase,
			envUC environment.Usecase,
			wsUC workspace.Usecase,
			st *mcpadapter.RuntimeStatus,
			auth *mcpadapter.TokenAuth,
		) *mcpadapter.Server {
			return mcpadapter.NewServer(engine, syncQueue, colUC, reqUC, envUC, wsUC, st.Addr, auth)
		}),
		fx.Invoke(func(lc fx.Lifecycle, srv *mcpadapter.Server, st *mcpadapter.RuntimeStatus) {
			lc.Append(fx.Hook{
				OnStart: func(ctx context.Context) error {
					if !st.Enabled {
						return nil
					}
					// Pre-bind probe: the SSE server binds asynchronously and would only
					// log a port conflict. A failed probe leaves Running=false, boot continues.
					ln, err := net.Listen("tcp", st.Addr)
					if err != nil {
						slog.Error("MCP port unavailable", "addr", st.Addr, "err", err)
						return nil
					}
					_ = ln.Close()
					if err := srv.Start(ctx); err != nil {
						return err
					}
					st.Running = true
					return nil
				},
				OnStop: srv.Stop,
			})
		}),
	)
}

// lookupEnvWithLegacy falls back to the pre-rebrand GopherCourier name so old setups keep working.
func lookupEnvWithLegacy(name, legacy string) (string, bool) {
	if v, ok := os.LookupEnv(name); ok {
		return v, true
	}
	return os.LookupEnv(legacy)
}

// getEnvWithLegacy is the Getenv counterpart of lookupEnvWithLegacy.
func getEnvWithLegacy(name, legacy string) string {
	if v := os.Getenv(name); v != "" {
		return v
	}
	return os.Getenv(legacy)
}
