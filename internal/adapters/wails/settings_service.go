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
}

// NewSettingsService creates a new SettingsService instance.
func NewSettingsService(uc settings.Usecase, status *mcpadapter.RuntimeStatus) *SettingsService {
	return &SettingsService{uc: uc, status: status}
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
		desiredAddr = cfg.Addr
	}
	// SSE URL points at the live endpoint when running; otherwise it previews
	// the desired addr.
	urlAddr := desiredAddr
	if s.status.Running {
		urlAddr = s.status.Addr
	}
	restart := desiredEnabled != s.status.Running ||
		(s.status.Running && desiredAddr != s.status.Addr)
	return OK(dto.MCPSettingsResponse{
		Enabled:         desiredEnabled,
		Addr:            desiredAddr,
		EnvManaged:      s.status.EnvManaged,
		Running:         s.status.Running,
		SSEURL:          buildSSEURL(urlAddr),
		RestartRequired: restart,
	})
}

// SetMCPSettings persists MCP settings. It does NOT restart the server — the
// change takes effect on the next app launch. Rejected when env-managed.
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
	if err := s.uc.SetMCPConfig(context.Background(), settings.MCPConfig{Enabled: req.Enabled, Addr: addr}); err != nil {
		return Err[dto.MCPSettingsResponse](err)
	}
	return s.GetMCPSettings()
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
