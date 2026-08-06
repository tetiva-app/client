package settings

import (
	"context"
	"fmt"
	"net"
	"strconv"
)

const (
	keyMCPEnabled = "mcp.enabled"
	keyMCPAddr    = "mcp.addr"

	// DefaultMCPAddr is used when no address is persisted.
	DefaultMCPAddr = "127.0.0.1:9300"
)

// NormalizeMCPAddr binds host-less addresses (":9300") to loopback: a bare port
// means 0.0.0.0, and the MCP server has no authentication. An explicit host is
// returned untouched — exposing the server is then a deliberate choice.
func NormalizeMCPAddr(addr string) string {
	host, port, err := net.SplitHostPort(addr)
	if err != nil || host != "" {
		return addr
	}
	return net.JoinHostPort("127.0.0.1", port)
}

// MCPConfig is the persisted MCP server configuration.
type MCPConfig struct {
	Enabled bool
	Addr    string
}

// GetMCPConfig reads the persisted MCP config, falling back to defaults
// (disabled, DefaultMCPAddr) when keys are absent.
func (u *usecase) GetMCPConfig(ctx context.Context) (MCPConfig, error) {
	const funcName = "settings.GetMCPConfig"
	cfg := MCPConfig{Addr: DefaultMCPAddr}

	enStr, found, err := u.repo.Get(ctx, keyMCPEnabled)
	if err != nil {
		return cfg, fmt.Errorf("%s: %w", funcName, err)
	}
	if found {
		cfg.Enabled = enStr == "true"
	}

	addr, found, err := u.repo.Get(ctx, keyMCPAddr)
	if err != nil {
		return cfg, fmt.Errorf("%s: %w", funcName, err)
	}
	if found && addr != "" {
		cfg.Addr = addr
	}
	return cfg, nil
}

// SetMCPConfig persists the MCP config.
func (u *usecase) SetMCPConfig(ctx context.Context, cfg MCPConfig) error {
	const funcName = "settings.SetMCPConfig"
	if err := u.repo.Set(ctx, keyMCPEnabled, strconv.FormatBool(cfg.Enabled)); err != nil {
		return fmt.Errorf("%s: %w", funcName, err)
	}
	if err := u.repo.Set(ctx, keyMCPAddr, cfg.Addr); err != nil {
		return fmt.Errorf("%s: %w", funcName, err)
	}
	return nil
}
