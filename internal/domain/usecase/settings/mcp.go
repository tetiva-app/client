package settings

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"net"
	"strconv"
)

const (
	keyMCPEnabled      = "mcp.enabled"
	keyMCPAddr         = "mcp.addr"
	keyMCPToken        = "mcp.token"
	keyMCPRequireToken = "mcp.require_token"

	DefaultMCPAddr = "127.0.0.1:9300"

	mcpTokenBytes = 32
)

// NormalizeMCPAddr binds host-less addresses (":9300") to loopback, since a bare port means 0.0.0.0.
// An explicit host is left untouched — exposing the server is then a deliberate choice.
func NormalizeMCPAddr(addr string) string {
	host, port, err := net.SplitHostPort(addr)
	if err != nil || host != "" {
		return addr
	}
	return net.JoinHostPort("127.0.0.1", port)
}

type MCPConfig struct {
	Enabled      bool
	Addr         string
	RequireToken bool
}

// GetMCPConfig falls back to disabled + DefaultMCPAddr when the keys are absent.
func (u *usecase) GetMCPConfig(ctx context.Context) (MCPConfig, error) {
	const funcName = "settings.GetMCPConfig"
	cfg := MCPConfig{Addr: DefaultMCPAddr, RequireToken: true}

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

	require, err := u.GetMCPRequireToken(ctx)
	if err != nil {
		return cfg, fmt.Errorf("%s: %w", funcName, err)
	}
	cfg.RequireToken = require
	return cfg, nil
}

// SetMCPConfig persists the config in one write: split across three keys a crash could leave it
// half applied — the server up on the new address with the old token requirement.
func (u *usecase) SetMCPConfig(ctx context.Context, cfg MCPConfig) error {
	const funcName = "settings.SetMCPConfig"
	err := u.repo.SetMany(ctx, map[string]string{
		keyMCPEnabled:      strconv.FormatBool(cfg.Enabled),
		keyMCPAddr:         cfg.Addr,
		keyMCPRequireToken: strconv.FormatBool(cfg.RequireToken),
	})
	if err != nil {
		return fmt.Errorf("%s: %w", funcName, err)
	}
	return nil
}

// GetMCPToken returns the persisted bearer token, or "" when none was generated yet.
func (u *usecase) GetMCPToken(ctx context.Context) (string, error) {
	const funcName = "settings.GetMCPToken"
	token, found, err := u.repo.Get(ctx, keyMCPToken)
	if err != nil {
		return "", fmt.Errorf("%s: %w", funcName, err)
	}
	if !found {
		return "", nil
	}
	return token, nil
}

// EnsureMCPToken generates a token on first use; an existing one is never rotated.
func (u *usecase) EnsureMCPToken(ctx context.Context) (string, error) {
	const funcName = "settings.EnsureMCPToken"
	token, err := u.GetMCPToken(ctx)
	if err != nil {
		return "", fmt.Errorf("%s: %w", funcName, err)
	}
	if token != "" {
		return token, nil
	}
	return u.RegenerateMCPToken(ctx)
}

// RegenerateMCPToken invalidates every MCP client config that carries the old token.
func (u *usecase) RegenerateMCPToken(ctx context.Context) (string, error) {
	const funcName = "settings.RegenerateMCPToken"
	token, err := generateMCPToken()
	if err != nil {
		return "", fmt.Errorf("%s: %w", funcName, err)
	}
	if err := u.repo.Set(ctx, keyMCPToken, token); err != nil {
		return "", fmt.Errorf("%s: %w", funcName, err)
	}
	return token, nil
}

// GetMCPRequireToken treats an absent key as enabled — the safe default for an existing install.
func (u *usecase) GetMCPRequireToken(ctx context.Context) (bool, error) {
	const funcName = "settings.GetMCPRequireToken"
	v, found, err := u.repo.Get(ctx, keyMCPRequireToken)
	if err != nil {
		return true, fmt.Errorf("%s: %w", funcName, err)
	}
	if !found {
		return true, nil
	}
	return v != "false", nil
}

func generateMCPToken() (string, error) {
	buf := make([]byte, mcpTokenBytes)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}
