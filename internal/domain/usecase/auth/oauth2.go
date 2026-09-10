package auth

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"

	"github.com/tetiva-app/client/internal/domain"
	"github.com/tetiva-app/client/internal/domain/entities"
)

// defaultRedirectPort mirrors the Auth tab's default (frontend/src/lib/auth-data.ts).
const defaultRedirectPort = "21830"

// OAuth 2.0 grants. The two interactive ones acquire their tokens only through
// the browser flows.
const (
	GrantClientCredentials = "client_credentials"
	GrantPassword          = "password"
	GrantAuthorizationCode = "authorization_code"
	GrantDeviceCode        = "device_code"
)

// How the client authenticates at the token endpoint (RFC 6749 §2.3.1).
const (
	ClientAuthBasic = "basic"
	ClientAuthBody  = "body"
	ClientAuthNone  = "none"
)

// OAuth2Config holds the acquisition fields of an oauth2 auth_data document, variables already
// substituted. Presentation fields stay out — they must not change the hash — and field order is canonical.
type OAuth2Config struct {
	Grant         string
	TokenURL      string
	AuthURL       string
	DeviceAuthURL string
	ClientID      string
	ClientSecret  string
	ClientAuth    string
	Scope         string
	Audience      string
	Username      string
	Password      string
}

// OAuth2ConfigFromFields defaults unset grant and clientAuth to what the Auth tab displays, so an
// import that omits them behaves the way the form reads. Secrets keep their whitespace, the rest is trimmed.
func OAuth2ConfigFromFields(f Fields) OAuth2Config {
	cfg := OAuth2Config{
		Grant:         strings.TrimSpace(f.Str("grant")),
		TokenURL:      strings.TrimSpace(f.Str("tokenUrl")),
		AuthURL:       strings.TrimSpace(f.Str("authUrl")),
		DeviceAuthURL: strings.TrimSpace(f.Str("deviceAuthUrl")),
		ClientID:      strings.TrimSpace(f.Str("clientId")),
		ClientSecret:  f.Str("clientSecret"),
		ClientAuth:    strings.TrimSpace(f.Str("clientAuth")),
		Scope:         strings.TrimSpace(f.Str("scope")),
		Audience:      strings.TrimSpace(f.Str("audience")),
		Username:      strings.TrimSpace(f.Str("username")),
		Password:      f.Str("password"),
	}
	if cfg.Grant == "" {
		cfg.Grant = GrantClientCredentials
	}
	if cfg.ClientAuth == "" {
		cfg.ClientAuth = ClientAuthBasic
	}

	return cfg
}

// AcquisitionChanged reports whether a committed edit invalidates a stored token: the owner left oauth2,
// or its raw fields differ. Variables stay unresolved — the config hash catches the rest at use time.
func AcquisitionChanged(oldType, newType entities.AuthType, oldData, newData string) bool {
	if oldType != entities.AuthTypeOAuth2 {
		return false
	}
	if newType != entities.AuthTypeOAuth2 {
		return true
	}
	oldCfg, oldOK := acquisitionConfig(oldData)
	newCfg, newOK := acquisitionConfig(newData)

	return !oldOK || !newOK || oldCfg != newCfg
}

// AcquisitionHash is the config hash a stored token must carry to have been acquired with this document;
// it hashes the document as written, so one still holding {{variables}} never matches the substituted hash.
func AcquisitionHash(authType entities.AuthType, data string) string {
	if authType != entities.AuthTypeOAuth2 {
		return ""
	}
	cfg, ok := acquisitionConfig(data)
	if !ok {
		return ""
	}

	return ConfigHash(cfg)
}

// acquisitionConfig reports false for a document that does not parse; the caller treats that as a change,
// because an unreadable configuration cannot be proven equal to the one that produced the token.
func acquisitionConfig(raw string) (OAuth2Config, bool) {
	f, err := ParseFields(raw)
	if err != nil {
		return OAuth2Config{}, false
	}

	return OAuth2ConfigFromFields(f), true
}

// validate checks what must hold before any token endpoint call; its messages
// reach the user as validation errors.
func (c OAuth2Config) validate() error {
	switch c.Grant {
	case GrantClientCredentials, GrantPassword, GrantAuthorizationCode, GrantDeviceCode:
	case "":
		return errors.New("oauth2: grant is required")
	default:
		return fmt.Errorf("oauth2: unsupported grant %q", c.Grant)
	}

	switch c.ClientAuth {
	case ClientAuthBasic, ClientAuthBody, ClientAuthNone:
	default:
		return fmt.Errorf("oauth2: unsupported clientAuth %q", c.ClientAuth)
	}

	if c.ClientID == "" {
		return errors.New("oauth2: clientId is required")
	}
	if _, err := ValidateEndpointURL(c.TokenURL); err != nil {
		return fmt.Errorf("oauth2: tokenUrl: %w", err)
	}
	if c.Grant == GrantPassword && (c.Username == "" || c.Password == "") {
		return errors.New("oauth2: username and password are required for the password grant")
	}

	return nil
}

// ValidateFlowConfig checks everything a browser flow needs inside the typed boundary — validate()
// returns plain errors that Err[T] files as "internal". Only the named grant's endpoints are checked.
func ValidateFlowConfig(cfg OAuth2Config, grant string) error {
	fields := map[string]string{}

	switch {
	case grant != GrantAuthorizationCode && grant != GrantDeviceCode:
		fields["grant"] = fmt.Sprintf("%q has no browser flow", grant)
	case cfg.Grant != grant:
		// A token stored under the hash of a different grant would be served for it.
		fields["grant"] = fmt.Sprintf("the configuration uses the %q grant, not %q", cfg.Grant, grant)
	}

	switch cfg.ClientAuth {
	case ClientAuthBasic, ClientAuthBody, ClientAuthNone:
	default:
		fields["clientAuth"] = fmt.Sprintf("unsupported client authentication %q", cfg.ClientAuth)
	}
	if cfg.ClientID == "" {
		fields["clientId"] = "clientId is required"
	}
	if _, err := ValidateEndpointURL(cfg.TokenURL); err != nil {
		fields["tokenUrl"] = err.Error()
	}
	switch grant {
	case GrantAuthorizationCode:
		if _, err := ValidateEndpointURL(cfg.AuthURL); err != nil {
			fields["authUrl"] = err.Error()
		}
	case GrantDeviceCode:
		if _, err := ValidateEndpointURL(cfg.DeviceAuthURL); err != nil {
			fields["deviceAuthUrl"] = err.Error()
		}
	}

	if len(fields) > 0 {
		return &domain.ValidationError{Fields: fields}
	}

	return nil
}

// ValidateRedirectPort normalises the loopback listener port: empty takes the
// Auth tab's default, "0" asks the OS for a free one.
func ValidateRedirectPort(raw string) (string, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return defaultRedirectPort, nil
	}

	port, err := strconv.ParseUint(trimmed, 10, 16)
	if err != nil {
		return "", &domain.ValidationError{Fields: map[string]string{
			"redirectPort": "must be a port number between 0 and 65535; 0 picks a free one",
		}}
	}

	return strconv.FormatUint(port, 10), nil
}

// ValidateEndpointURL accepts an absolute https URL, or http only for a loopback host: secrets and
// tokens must not cross the network in the clear. Stage B validates authUrl and deviceAuthUrl the same way.
func ValidateEndpointURL(raw string) (*url.URL, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil, errors.New("URL is required")
	}

	u, err := url.Parse(trimmed)
	if err != nil {
		return nil, fmt.Errorf("invalid URL: %w", err)
	}
	if !u.IsAbs() || u.Host == "" {
		return nil, errors.New("URL must be absolute, like https://idp.example/oauth/token")
	}
	if u.User != nil {
		return nil, errors.New("URL must not carry userinfo")
	}
	if u.Fragment != "" || strings.Contains(trimmed, "#") {
		return nil, errors.New("URL must not carry a fragment")
	}

	switch u.Scheme {
	case "https":
	case "http":
		if !isLoopbackHost(u.Hostname()) {
			return nil, errors.New("http is allowed only for localhost; use https")
		}
	default:
		return nil, fmt.Errorf("unsupported URL scheme %q", u.Scheme)
	}

	return u, nil
}

func isLoopbackHost(host string) bool {
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)

	return ip != nil && ip.IsLoopback()
}
