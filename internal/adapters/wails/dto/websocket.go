package dto

// The connection id comes from the frontend so it can subscribe before the handshake.
type WSConnectRequest struct {
	RequestID    string `json:"requestId"`
	WorkspaceID  string `json:"workspaceId"`
	ConnectionID string `json:"connectionId"`
}

// A failed handshake is reported here, not an error, so pre-connect output survives.
type WSConnectResultDTO struct {
	Connected    bool             `json:"connected"`
	ConnectionID string           `json:"connectionId"`
	Status       int              `json:"status"`
	Subprotocol  string           `json:"subprotocol"`
	Error        string           `json:"error,omitempty"`
	Script       *ScriptResultDTO `json:"script,omitempty"`
}

// Binary payloads travel base64-encoded.
type WSSendRequest struct {
	ConnectionID string `json:"connectionId"`
	Data         string `json:"data"`
	MessageType  string `json:"messageType"` // "text" | "binary"
}

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
