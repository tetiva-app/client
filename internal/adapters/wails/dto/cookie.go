package dto

import "github.com/tetiva-app/client/internal/domain/entities"

type CookieResponse struct {
	ID        string `json:"id"`
	Domain    string `json:"domain"`
	HostOnly  bool   `json:"hostOnly"`
	Path      string `json:"path"`
	Name      string `json:"name"`
	Value     string `json:"value"`
	ExpiresAt *int64 `json:"expiresAt"` // unix seconds; nil = session
	HTTPOnly  bool   `json:"httpOnly"`
	Secure    bool   `json:"secure"`
	SameSite  string `json:"sameSite"` // "", "Lax", "Strict", "None"
}

type AddCookieRequest struct {
	WorkspaceID string `json:"workspaceId"`
	Domain      string `json:"domain"`
	HostOnly    bool   `json:"hostOnly"`
	Path        string `json:"path"`
	Name        string `json:"name"`
	Value       string `json:"value"`
	ExpiresAt   *int64 `json:"expiresAt"`
	HTTPOnly    bool   `json:"httpOnly"`
	Secure      bool   `json:"secure"`
	SameSite    string `json:"sameSite"`
}

// Replace semantics: the UI must send the full current state of the cookie.
type EditCookieRequest struct {
	ID        string `json:"id"`
	Domain    string `json:"domain"`
	HostOnly  bool   `json:"hostOnly"`
	Path      string `json:"path"`
	Name      string `json:"name"`
	Value     string `json:"value"`
	ExpiresAt *int64 `json:"expiresAt"`
	HTTPOnly  bool   `json:"httpOnly"`
	Secure    bool   `json:"secure"`
	SameSite  string `json:"sameSite"`
}

func CookieToResponse(c *entities.Cookie) CookieResponse {
	r := CookieResponse{
		ID:     c.ID.String(),
		Domain: c.Domain, HostOnly: c.HostOnly,
		Path: c.Path,
		Name: c.Name, Value: c.Value,
		HTTPOnly: c.HTTPOnly, Secure: c.Secure, SameSite: c.SameSite,
	}
	if c.ExpiresAt != nil {
		t := c.ExpiresAt.Unix()
		r.ExpiresAt = &t
	}
	return r
}

func CookiesToResponse(cs []*entities.Cookie) []CookieResponse {
	out := make([]CookieResponse, len(cs))
	for i, c := range cs {
		out[i] = CookieToResponse(c)
	}
	return out
}
