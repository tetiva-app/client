package dto

type ConnectRequest struct {
	ServerURL string `json:"serverUrl"`
	Email     string `json:"email"`
	Password  string `json:"password"`
}

type RegisterRequest struct {
	ServerURL string `json:"serverUrl"`
	Email     string `json:"email"`
	Password  string `json:"password"`
	Name      string `json:"name"`
	Locale    string `json:"locale"`
}

type AuthStateResult struct {
	Email                     string `json:"email"`
	RequiresEmailVerification bool   `json:"requiresEmailVerification"`
}

type MeResult struct {
	Email         string `json:"email"`
	EmailVerified bool   `json:"emailVerified"`
}

type SessionInfo struct {
	ID         string `json:"id"`
	ClientID   string `json:"clientId"`
	UserAgent  string `json:"userAgent"`
	IP         string `json:"ip"`
	LastUsedAt string `json:"lastUsedAt"`
	IsCurrent  bool   `json:"isCurrent"`
}

type RevokeSessionRequest struct {
	SessionID string `json:"sessionId"`
}

type LogoutAllResult struct {
	RevokedCount int `json:"revokedCount"`
}

type SyncStatusResponse struct {
	Enabled   bool   `json:"enabled"`
	State     string `json:"state"`
	ServerURL string `json:"serverUrl"`
	UserEmail string `json:"userEmail"`
	Pending   int    `json:"pending"`
	Parked    int    `json:"parked"`
	// TooLarge is the size-refused entries, which no plan change releases.
	TooLarge             int  `json:"tooLarge"`
	AwaitingVerification bool `json:"awaitingVerification"`
	// ReauthRequired is the one-shot ask after an update dropped the session.
	ReauthRequired bool `json:"reauthRequired"`
}

type WorkspaceMapping struct {
	LocalID    string `json:"localId"`
	RemoteID   string `json:"remoteId"`
	RemoteName string `json:"remoteName"`
	State      string `json:"state"`
}

type RemoteWorkspace struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type LinkWorkspaceRequest struct {
	LocalWorkspaceID  string `json:"localWorkspaceId"`
	RemoteWorkspaceID string `json:"remoteWorkspaceId"`
}

type UnlinkWorkspaceRequest struct {
	LocalWorkspaceID string `json:"localWorkspaceId"`
}

type ServerCapabilitiesRequest struct {
	ServerURL string `json:"serverUrl"`
}

// ServerCapabilities is the discovery answer the modal branches on. SignInHost is
// the hostname of the cabinet page, for the "Opens {host} in your browser" line.
type ServerCapabilities struct {
	ServerVersion    string `json:"serverVersion"`
	DesktopSignIn    bool   `json:"desktopSignIn"`
	SignInHost       string `json:"signInHost"`
	RegistrationOpen bool   `json:"registrationOpen"`
}

// flowId is minted by the frontend so it can subscribe before the RPC runs.
type StartBrowserSignInRequest struct {
	FlowID    string `json:"flowId"`
	ServerURL string `json:"serverUrl"`
	Intent    string `json:"intent"` // "" | "signin" | "register"
	Locale    string `json:"locale"`
}

// LoginURL carries the claim secret in its fragment and never leaves this process
// except through openExternal and the clipboard, both at the user's request.
type BrowserSignInInfo struct {
	FlowID    string `json:"flowId"`
	LoginURL  string `json:"loginUrl"`
	Host      string `json:"host"`
	ExpiresAt string `json:"expiresAt"` // RFC 3339, empty when unknown
}

// Mirrors FlowStatusDTO for the sign-in manager. State is empty when the manager
// never had the id: "gone", not "still pending". Auth is set only with done.
type BrowserSignInStatus struct {
	State                    string            `json:"state"`
	Error                    string            `json:"error"`
	Info                     BrowserSignInInfo `json:"info"`
	EmailVerificationPending bool              `json:"emailVerificationPending"`
	// No omitempty: a nil pointer has to reach the frontend as an explicit null,
	// or the Wails path yields undefined where the mock returns null.
	Auth *AuthStateResult `json:"auth"`
}

// Carried on sync:signin:<flowId>. Info is not sent (the frontend holds it from the
// start call), but the deadline is: the server extends it while the user confirms.
type BrowserSignInEvent struct {
	State                    string `json:"state"`
	Error                    string `json:"error"`
	EmailVerificationPending bool   `json:"emailVerificationPending"`
	ExpiresAt                string `json:"expiresAt"` // RFC 3339, empty when unknown
}

// FlowIDRequest addresses a running or retained browser sign-in.
type FlowIDRequest struct {
	FlowID string `json:"flowId"`
}
