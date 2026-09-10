package auth

import "time"

// Token is one acquisition result from an OAuth 2.0 token endpoint.
type Token struct {
	AccessToken  string
	RefreshToken string
	TokenType    string
	Scope        string
	IDToken      string
	// ExpiresAt is zero when the endpoint reported no expiry.
	ExpiresAt  time.Time
	ObtainedAt time.Time
}

// StoredToken is a Token as it sits in the local store: the hash of the configuration that produced it
// decides whether it may be reused, and the generation guards against writers that started before a clear.
type StoredToken struct {
	Token
	ConfigHash string
	Generation int64
}
