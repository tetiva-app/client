package settings

import (
	"context"
	"fmt"
	"strconv"
)

const (
	keyMCPEnabled = "mcp.enabled"
	keyMCPAddr    = "mcp.addr"

	// DefaultMCPAddr is used when no address is persisted.
	DefaultMCPAddr = ":9300"
)

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
