package wails

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"

	mcpadapter "github.com/tetiva-app/client/internal/adapters/mcp"
	"github.com/tetiva-app/client/internal/adapters/wails/dto"
	"github.com/tetiva-app/client/internal/domain"
	"github.com/tetiva-app/client/internal/domain/usecase/settings"
)

// SettingsService exposes app settings (currently MCP) to the Wails frontend.
type SettingsService struct {
	uc     settings.Usecase
	status *mcpadapter.RuntimeStatus
	auth   *mcpadapter.TokenAuth
}

func NewSettingsService(uc settings.Usecase, status *mcpadapter.RuntimeStatus, auth *mcpadapter.TokenAuth) *SettingsService {
	return &SettingsService{uc: uc, status: status, auth: auth}
}

// GetMCPSettings returns the desired MCP settings (DB config, or env when
// env-managed) plus live runtime status; RestartRequired means they differ.
func (s *SettingsService) GetMCPSettings() Result[dto.MCPSettingsResponse] {
	desiredEnabled := s.status.Enabled
	desiredAddr := s.status.Addr
	if !s.status.EnvManaged {
		cfg, err := s.uc.GetMCPConfig(context.Background())
		if err != nil {
			return Err[dto.MCPSettingsResponse](err)
		}
		desiredEnabled = cfg.Enabled
		// Report the address the server will actually bind, otherwise a persisted
		// ":9300" reads as differing from the resolved one and RestartRequired sticks.
		desiredAddr = settings.NormalizeMCPAddr(cfg.Addr)
	}
	// SSE URL points at the live endpoint when running; otherwise it previews
	// the desired addr.
	urlAddr := desiredAddr
	if s.status.Running {
		urlAddr = s.status.Addr
	}
	restart := desiredEnabled != s.status.Running ||
		(s.status.Running && desiredAddr != s.status.Addr)

	token, err := s.uc.EnsureMCPToken(context.Background())
	if err != nil {
		return Err[dto.MCPSettingsResponse](err)
	}
	requireToken, err := s.uc.GetMCPRequireToken(context.Background())
	if err != nil {
		return Err[dto.MCPSettingsResponse](err)
	}

	return OK(dto.MCPSettingsResponse{
		Enabled:         desiredEnabled,
		Addr:            desiredAddr,
		EnvManaged:      s.status.EnvManaged,
		Running:         s.status.Running,
		SSEURL:          buildSSEURL(urlAddr),
		RestartRequired: restart,
		Token:           token,
		RequireToken:    requireToken,
	})
}

// Takes effect on the next app launch, not immediately; rejected when env-managed.
func (s *SettingsService) SetMCPSettings(req dto.SetMCPSettingsRequest) Result[dto.MCPSettingsResponse] {
	if s.status.EnvManaged {
		return Err[dto.MCPSettingsResponse](&domain.ValidationError{
			Fields: map[string]string{"_": "MCP is managed by environment variables"},
		})
	}
	addr := strings.TrimSpace(req.Addr)
	if err := validateAddr(addr); err != nil {
		return Err[dto.MCPSettingsResponse](&domain.ValidationError{
			Fields: map[string]string{"addr": err.Error()},
		})
	}
	ctx := context.Background()
	requireToken, err := s.resolveRequireToken(ctx, req.RequireToken)
	if err != nil {
		return Err[dto.MCPSettingsResponse](err)
	}

	if err := s.uc.SetMCPConfig(ctx, settings.MCPConfig{
		Enabled:      req.Enabled,
		Addr:         addr,
		RequireToken: requireToken,
	}); err != nil {
		return Err[dto.MCPSettingsResponse](err)
	}

	// Live auth only after the write committed, so the running guard never
	// enforces a requirement that the next launch will not find persisted.
	token, err := s.uc.EnsureMCPToken(ctx)
	if err != nil {
		return Err[dto.MCPSettingsResponse](err)
	}
	s.setAuth(token, requireToken)
	return s.GetMCPSettings()
}

// resolveRequireToken carries the stored flag into the single config write when
// the caller omitted it; omission means "leave it alone", not "turn it off".
func (s *SettingsService) resolveRequireToken(ctx context.Context, requested *bool) (bool, error) {
	if requested != nil {
		return *requested, nil
	}
	return s.uc.GetMCPRequireToken(ctx)
}

// RegenerateMCPToken issues a new token and invalidates every client config
// that carries the old one. Takes effect immediately on the running server.
func (s *SettingsService) RegenerateMCPToken() Result[dto.MCPSettingsResponse] {
	ctx := context.Background()
	token, err := s.uc.RegenerateMCPToken(ctx)
	if err != nil {
		return Err[dto.MCPSettingsResponse](err)
	}

	require, err := s.uc.GetMCPRequireToken(ctx)
	if err != nil {
		// The old token is already dead in storage, so revoke it on the running server
		// too and fail closed instead of reporting a rotation the caller cannot trust.
		s.setAuth(token, true)
		return Err[dto.MCPSettingsResponse](
			fmt.Errorf("mcp token rotated but its requirement flag could not be read: %w", err))
	}
	s.setAuth(token, require)
	return s.GetMCPSettings()
}

// setAuth pushes credentials into the already-running server. The token is the
// value just persisted, not a re-read, so a rotation cannot half apply.
func (s *SettingsService) setAuth(token string, require bool) {
	if s.auth == nil {
		return
	}
	s.auth.Set(token, require)
}

func validateAddr(addr string) error {
	_, portStr, err := net.SplitHostPort(addr)
	if err != nil {
		return errors.New("must be host:port (e.g. :9300)")
	}
	port, err := strconv.Atoi(portStr)
	if err != nil || port < 1 || port > 65535 {
		return errors.New("port must be between 1 and 65535")
	}
	return nil
}

// buildSSEURL turns a listen address into the SSE URL the frontend shows;
// empty/wildcard hosts become localhost, IPv6 hosts get brackets.
func buildSSEURL(addr string) string {
	host, portStr, err := net.SplitHostPort(addr)
	if err != nil {
		return ""
	}
	if host == "" || host == "0.0.0.0" || host == "::" {
		host = "localhost"
	}
	if strings.Contains(host, ":") {
		host = "[" + host + "]"
	}
	return fmt.Sprintf("http://%s:%s/sse", host, portStr)
}
