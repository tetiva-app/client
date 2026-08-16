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

// AuthStateResult reports the account state right after connect or register.
type AuthStateResult struct {
	Email                     string `json:"email"`
	RequiresEmailVerification bool   `json:"requiresEmailVerification"`
}

// MeResult reports the account behind the stored credentials.
type MeResult struct {
	Email         string `json:"email"`
	EmailVerified bool   `json:"emailVerified"`
}

// SessionInfo is one device signed in to the account.
type SessionInfo struct {
	ID         string `json:"id"`
	ClientID   string `json:"clientId"`
	UserAgent  string `json:"userAgent"`
	IP         string `json:"ip"`
	LastUsedAt string `json:"lastUsedAt"`
	IsCurrent  bool   `json:"isCurrent"`
}

// RevokeSessionRequest names the session to sign out.
type RevokeSessionRequest struct {
	SessionID string `json:"sessionId"`
}

// LogoutAllResult reports how many other devices were signed out.
type LogoutAllResult struct {
	RevokedCount int `json:"revokedCount"`
}

// SyncStatusResponse reports overall sync status.
type SyncStatusResponse struct {
	Enabled              bool   `json:"enabled"`
	State                string `json:"state"`
	ServerURL            string `json:"serverUrl"`
	UserEmail            string `json:"userEmail"`
	Pending              int    `json:"pending"`
	Parked               int    `json:"parked"`
	AwaitingVerification bool   `json:"awaitingVerification"`
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
