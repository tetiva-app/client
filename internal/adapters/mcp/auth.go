package mcp

import (
	"crypto/subtle"
	"net/http"
	"strings"
	"sync"
)

// unauthorizedBody is read by a human staring at a failing MCP client, so it
// names the exact place the working config comes from.
const unauthorizedBody = "Tetiva MCP: invalid or missing token. " +
	"Open the app, go to Settings -> MCP / DevTools and use \"Copy config\", " +
	"or pass the token as Authorization: Bearer <token> or ?token=<token>."

// TokenAuth guards the MCP HTTP endpoints. It is mutable at runtime so that
// regenerating the token from settings takes effect without an app restart.
type TokenAuth struct {
	mu       sync.RWMutex
	token    string
	required bool
}

// A required guard with an empty token rejects everyone: a broken setup, not an open door.
func NewTokenAuth(token string, required bool) *TokenAuth {
	return &TokenAuth{token: token, required: required}
}

// Set replaces the credentials checked by subsequent requests.
func (a *TokenAuth) Set(token string, required bool) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.token = token
	a.required = required
}

// Enabled reports whether callers must present a token.
func (a *TokenAuth) Enabled() bool {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.required
}

// Allow reports whether the request carries the expected token.
func (a *TokenAuth) Allow(r *http.Request) bool {
	a.mu.RLock()
	token, required := a.token, a.required
	a.mu.RUnlock()

	if !required {
		return true
	}
	if token == "" {
		return false
	}
	presented := presentedToken(r)
	if presented == "" {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(presented), []byte(token)) == 1
}

func (a *TokenAuth) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !a.Allow(r) {
			w.Header().Set("WWW-Authenticate", `Bearer realm="Tetiva MCP"`)
			http.Error(w, unauthorizedBody, http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// presentedToken accepts both transports: a Bearer header (Cursor, Claude Code)
// and a query parameter (clients whose config holds nothing but a URL).
func presentedToken(r *http.Request) string {
	if h := r.Header.Get("Authorization"); h != "" {
		if after, ok := cutBearer(h); ok {
			return after
		}
	}
	return r.URL.Query().Get("token")
}

func cutBearer(header string) (string, bool) {
	const scheme = "bearer "
	if len(header) < len(scheme) || !strings.EqualFold(header[:len(scheme)], scheme) {
		return "", false
	}
	return strings.TrimSpace(header[len(scheme):]), true
}
