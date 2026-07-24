package dto

// ConnectRequest holds sync server connection parameters.
type ConnectRequest struct {
	ServerURL string `json:"serverUrl"`
	Email     string `json:"email"`
	Password  string `json:"password"`
}

// RegisterRequest holds sync server registration parameters.
type RegisterRequest struct {
	ServerURL string `json:"serverUrl"`
	Email     string `json:"email"`
	Password  string `json:"password"`
	Name      string `json:"name"`
	Locale    string `json:"locale"`
}

// SyncStatusResponse reports overall sync status.
type SyncStatusResponse struct {
	Enabled   bool   `json:"enabled"`
	State     string `json:"state"`
	ServerURL string `json:"serverUrl"`
	UserEmail string `json:"userEmail"`
	Pending   int    `json:"pending"`
}

// WorkspaceMapping maps a local workspace to a remote one.
type WorkspaceMapping struct {
	LocalID    string `json:"localId"`
	RemoteID   string `json:"remoteId"`
	RemoteName string `json:"remoteName"`
	State      string `json:"state"`
}

// RemoteWorkspace represents a workspace on the sync server.
type RemoteWorkspace struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// LinkWorkspaceRequest maps a local workspace to a remote one.
type LinkWorkspaceRequest struct {
	LocalWorkspaceID  string `json:"localWorkspaceId"`
	RemoteWorkspaceID string `json:"remoteWorkspaceId"`
}

// UnlinkWorkspaceRequest removes a workspace mapping.
type UnlinkWorkspaceRequest struct {
	LocalWorkspaceID string `json:"localWorkspaceId"`
}
