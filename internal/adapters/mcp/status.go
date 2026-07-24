package mcp

// RuntimeStatus holds the live MCP server state, set by boot wiring and read by
// the Wails SettingsService; it lives here to avoid an import cycle with wails.
type RuntimeStatus struct {
	Enabled    bool   // resolved config: should the server run this session
	Running    bool   // server actually bound and started
	Addr       string // effective listen address
	EnvManaged bool   // true when env vars determined the config
}
