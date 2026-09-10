package auth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const (
	// tokenEndpointTimeout bounds one acquisition; the IdP is not the user's server.
	tokenEndpointTimeout = 30 * time.Second
	// maxTokenResponse caps what a token endpoint may hand back.
	maxTokenResponse = 1 << 20
	// maxErrorSnippet is how much of a non-standard error body reaches the user.
	maxErrorSnippet = 200
)

// NewTokenHTTPClient builds the client used for token endpoints only: no cookie jar, so IdP calls never
// carry workspace cookies, and no redirects, because a 307 would replay a client secret at another host.
func NewTokenHTTPClient() *http.Client {
	return &http.Client{
		Timeout: tokenEndpointTimeout,
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
}

// RFC 6749 §5.2 / RFC 8628 §3.5 error codes the device-code flow branches on.
const (
	OAuthErrAuthorizationPending = "authorization_pending"
	OAuthErrSlowDown             = "slow_down"
	OAuthErrExpiredToken         = "expired_token"
	OAuthErrAccessDenied         = "access_denied"
	OAuthErrInvalidGrant         = "invalid_grant"
)

// OAuthError is an error object an OAuth 2.0 endpoint returned in its body.
type OAuthError struct {
	Code        string
	Description string
}

func (e *OAuthError) Error() string {
	if e.Description != "" {
		return "oauth2: " + e.Code + ": " + e.Description
	}

	return "oauth2: " + e.Code
}

// postForm sends one form-encoded POST and owns the transport rules for both endpoints: the client
// authentication of cfg.ClientAuth, the refusal of a 3xx, the size cap and the *OAuthError mapping.
func postForm(ctx context.Context, client *http.Client, cfg OAuth2Config, endpoint *url.URL, form url.Values) (Fields, error) {
	const funcName = "auth.postForm"

	values := url.Values{}
	for key, vals := range form {
		values[key] = vals
	}
	switch cfg.ClientAuth {
	case ClientAuthBody:
		values.Set("client_id", cfg.ClientID)
		if cfg.ClientSecret != "" {
			values.Set("client_secret", cfg.ClientSecret)
		}
	case ClientAuthNone:
		values.Set("client_id", cfg.ClientID)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint.String(), strings.NewReader(values.Encode()))
	if err != nil {
		return nil, fmt.Errorf("%s: %w", funcName, err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	if cfg.ClientAuth == ClientAuthBasic {
		// RFC 6749 §2.3.1: both halves are form-urlencoded before the base64 step.
		req.SetBasicAuth(url.QueryEscape(cfg.ClientID), url.QueryEscape(cfg.ClientSecret))
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("oauth2: token endpoint unreachable: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode >= http.StatusMultipleChoices && resp.StatusCode < http.StatusBadRequest {
		return nil, fmt.Errorf("oauth2: token endpoint answered with a redirect (%s), which is not followed", resp.Status)
	}

	// One byte past the cap, so an oversized body is detected instead of parsed.
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxTokenResponse+1))
	if err != nil {
		return nil, fmt.Errorf("oauth2: reading the token response: %w", err)
	}
	if len(body) > maxTokenResponse {
		return nil, fmt.Errorf("oauth2: the token endpoint returned more than %d bytes", maxTokenResponse)
	}

	return parseFormResponse(resp.Status, resp.StatusCode, body)
}

func parseFormResponse(status string, code int, body []byte) (Fields, error) {
	var decoded map[string]any
	if err := json.Unmarshal(body, &decoded); err != nil {
		return nil, fmt.Errorf("oauth2: token endpoint returned %s with a non-JSON body: %s", status, snippet(body))
	}

	f := Fields(decoded)
	if errCode := f.Str("error"); errCode != "" {
		return nil, &OAuthError{Code: errCode, Description: f.Str("error_description")}
	}
	if code < http.StatusOK || code >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("oauth2: token endpoint returned %s: %s", status, snippet(body))
	}

	return f, nil
}

// requestToken posts one grant to the token endpoint and decodes the token it
// answers with. form is sent as given; only client authentication comes from cfg.
func requestToken(ctx context.Context, client *http.Client, cfg OAuth2Config, form url.Values, now time.Time) (*Token, error) {
	endpoint, err := ValidateEndpointURL(cfg.TokenURL)
	if err != nil {
		return nil, fmt.Errorf("oauth2: tokenUrl: %w", err)
	}

	f, err := postForm(ctx, client, cfg, endpoint, form)
	if err != nil {
		return nil, err
	}

	return tokenFromFields(f, now)
}

func tokenFromFields(f Fields, now time.Time) (*Token, error) {
	access := f.Str("access_token")
	if access == "" {
		return nil, errors.New("oauth2: token response carried no access_token")
	}

	tok := &Token{
		AccessToken:  access,
		RefreshToken: f.Str("refresh_token"),
		TokenType:    f.Str("token_type"),
		Scope:        f.Str("scope"),
		IDToken:      f.Str("id_token"),
		ObtainedAt:   now,
	}
	if seconds, ok := expiresInSeconds(f["expires_in"]); ok {
		tok.ExpiresAt = now.Add(time.Duration(seconds) * time.Second)
	}

	return tok, nil
}

// expiresInSeconds accepts a number or a numeric string; some IdPs still send
// the latter. A non-positive value is treated as "not stated".
func expiresInSeconds(v any) (int64, bool) {
	var seconds int64
	switch t := v.(type) {
	case float64:
		seconds = int64(t)
	case string:
		parsed, err := strconv.ParseInt(strings.TrimSpace(t), 10, 64)
		if err != nil {
			return 0, false
		}
		seconds = parsed
	default:
		return 0, false
	}

	return seconds, seconds > 0
}

func snippet(body []byte) string {
	text := strings.TrimSpace(string(body))
	if len(text) > maxErrorSnippet {
		return text[:maxErrorSnippet] + "…"
	}

	return text
}
