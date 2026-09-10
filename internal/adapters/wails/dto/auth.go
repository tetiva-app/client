package dto

// Carries the Auth tab's editor buffer, not the saved row: auth edits live in
// local state until Cmd+S. The workspace is derived from the owner, not sent.
type AuthConfigRequest struct {
	OwnerKind string `json:"ownerKind"`
	OwnerID   string `json:"ownerId"`
	AuthType  string `json:"authType"`
	AuthData  string `json:"authData"`
}

// TokenStatusDTO feeds the status line of the OAuth 2.0 block. ExpiresAt is
// RFC 3339, empty when the token endpoint reported no expiry.
type TokenStatusDTO struct {
	State     string `json:"state"`
	ExpiresAt string `json:"expiresAt"`
}

// ResolveOwnerRequest asks which entity holds the auth a request effectively uses.
type ResolveOwnerRequest struct {
	RequestID string `json:"requestId"`
}

// OwnerKind and OwnerID are empty when the inherit chain ends without a configured
// ancestor. AuthData comes back unmasked — it is the user's own configuration.
type ResolvedOwnerDTO struct {
	OwnerKind string `json:"ownerKind"`
	OwnerID   string `json:"ownerId"`
	AuthType  string `json:"authType"`
	AuthData  string `json:"authData"`
}

// Fields are spelled out rather than embedded: the TS binding generator would
// surface an embedded struct as a nested field. FlowID is minted by the frontend.
type StartFlowRequest struct {
	OwnerKind string `json:"ownerKind"`
	OwnerID   string `json:"ownerId"`
	AuthType  string `json:"authType"`
	AuthData  string `json:"authData"`
	FlowID    string `json:"flowId"`
}

// FlowRequest addresses a running or retained flow.
type FlowRequest struct {
	FlowID string `json:"flowId"`
}

// FlowInfoDTO carries both grants; the fields of the other one are empty.
// ExpiresAt is RFC 3339, empty when the flow has no deadline to show.
type FlowInfoDTO struct {
	FlowID                  string `json:"flowId"`
	AuthorizeURL            string `json:"authorizeUrl"`
	UserCode                string `json:"userCode"`
	VerificationURI         string `json:"verificationUri"`
	VerificationURIComplete string `json:"verificationUriComplete"`
	IntervalSec             int    `json:"intervalSec"`
	ExpiresAt               string `json:"expiresAt"`
}

// Carries the info alongside the state, so a reloaded page restores the pending
// panel from it alone. An unknown flow answers with an empty State: gone, not pending.
type FlowStatusDTO struct {
	State string      `json:"state"`
	Error string      `json:"error"`
	Info  FlowInfoDTO `json:"info"`
}
