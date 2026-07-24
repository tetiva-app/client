package dto

// MCPSettingsResponse separates the desired config (Enabled/Addr, persisted)
// from the running server (Running/SSEURL); the server only changes on restart.
type MCPSettingsResponse struct {
	Enabled         bool   `json:"enabled"` // desired (persisted, or env when envManaged)
	Addr            string `json:"addr"`    // desired addr
	EnvManaged      bool   `json:"envManaged"`
	Running         bool   `json:"running"`
	SSEURL          string `json:"sseUrl"`          // endpoint of the running server (or desired addr if not running)
	RestartRequired bool   `json:"restartRequired"` // desired config differs from the running server
}

// SetMCPSettingsRequest updates the persisted MCP settings.
type SetMCPSettingsRequest struct {
	Enabled bool   `json:"enabled"`
	Addr    string `json:"addr"`
}
