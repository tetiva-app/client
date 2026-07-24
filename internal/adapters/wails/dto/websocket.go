package dto

// WSConnectRequest is the Connect RPC input.
type WSConnectRequest struct {
	RequestID   string `json:"requestId"`
	WorkspaceID string `json:"workspaceId"`
	UserID      string `json:"userId"`
}

// WSConnectionDTO is the Connect RPC output.
type WSConnectionDTO struct {
	ConnectionID string `json:"connectionId"`
}

// WSSendRequest is the Send RPC input.
type WSSendRequest struct {
	ConnectionID string `json:"connectionId"`
	Data         string `json:"data"`
}

// WSDisconnectRequest is the Disconnect RPC input.
type WSDisconnectRequest struct {
	ConnectionID string `json:"connectionId"`
}

// WSMessageDTO is emitted on `ws:message:<connID>`.
type WSMessageDTO struct {
	Dir  string `json:"dir"` // "in" | "out" | "system"
	Data string `json:"data"`
	Type string `json:"type"` // "text" | "binary"
	At   int64  `json:"at"`   // unix milliseconds
}

// WSStateDTO is emitted on `ws:state:<connID>`.
type WSStateDTO struct {
	State string `json:"state"` // connecting|connected|closing|closed|error
	Error string `json:"error,omitempty"`
}
