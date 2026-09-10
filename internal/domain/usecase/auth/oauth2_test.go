package auth

import (
	"errors"
	"strings"
	"testing"

	"github.com/tetiva-app/client/internal/domain"
)

func TestValidateEndpointURL(t *testing.T) {
	tests := []struct {
		name    string
		raw     string
		wantErr string
	}{
		{name: "https", raw: "https://idp.example/token"},
		{name: "https with port and query", raw: "https://idp.example:8443/oauth/token?tenant=a"},
		{name: "loopback ipv4", raw: "http://127.0.0.1:9/token"},
		{name: "loopback ipv6", raw: "http://[::1]:9/token"},
		{name: "localhost", raw: "http://localhost:8080/token"},
		{name: "localhost uppercase", raw: "http://LocalHost:8080/token"},
		{name: "plain http", raw: "http://idp.example/token", wantErr: "https"},
		{name: "private ip over http", raw: "http://10.0.0.5/token", wantErr: "https"},
		{name: "ftp", raw: "ftp://idp.example/token", wantErr: "scheme"},
		{name: "relative", raw: "/oauth/token", wantErr: "absolute"},
		{name: "schemeless host", raw: "idp.example/token", wantErr: "absolute"},
		{name: "empty", raw: "   ", wantErr: "required"},
		{name: "userinfo", raw: "https://user:pass@idp.example/token", wantErr: "userinfo"},
		{name: "fragment", raw: "https://idp.example/token#frag", wantErr: "fragment"},
		{name: "no host", raw: "https:///token", wantErr: "absolute"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			u, err := ValidateEndpointURL(tt.raw)
			if tt.wantErr != "" {
				if err == nil {
					t.Fatalf("ValidateEndpointURL(%q): expected an error", tt.raw)
				}
				if !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("error %q does not mention %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("ValidateEndpointURL(%q): %v", tt.raw, err)
			}
			if u == nil {
				t.Fatal("expected a parsed URL")
			}
		})
	}
}

func TestOAuth2ConfigFromFields(t *testing.T) {
	f, err := ParseFields(`{
		"grant":"password","tokenUrl":" https://idp.example/token ","clientId":"app",
		"clientSecret":"s3cret","scope":"read write","audience":"api://x",
		"username":"bob","password":"hunter2","addTo":"query","headerPrefix":"Token",
		"expiresIn":3600
	}`)
	if err != nil {
		t.Fatalf("ParseFields: %v", err)
	}

	cfg := OAuth2ConfigFromFields(f)
	if cfg.Grant != GrantPassword {
		t.Errorf("Grant: got %q", cfg.Grant)
	}
	if bare := OAuth2ConfigFromFields(Fields{"clientId": "app"}); bare.Grant != GrantClientCredentials {
		t.Errorf("unset grant: got %q, want %q", bare.Grant, GrantClientCredentials)
	}
	if cfg.TokenURL != "https://idp.example/token" {
		t.Errorf("TokenURL not trimmed: %q", cfg.TokenURL)
	}
	if cfg.ClientAuth != ClientAuthBasic {
		t.Errorf("ClientAuth default: got %q, want %q", cfg.ClientAuth, ClientAuthBasic)
	}
	if cfg.Scope != "read write" || cfg.Audience != "api://x" {
		t.Errorf("scope/audience: %q / %q", cfg.Scope, cfg.Audience)
	}
	if cfg.Username != "bob" || cfg.Password != "hunter2" || cfg.ClientSecret != "s3cret" {
		t.Errorf("credentials not carried: %+v", cfg)
	}
}

func TestConfigHash(t *testing.T) {
	base := OAuth2Config{
		Grant: GrantClientCredentials, TokenURL: "https://idp.example/token",
		ClientID: "app", ClientSecret: "s3cret", ClientAuth: ClientAuthBasic, Scope: "read",
	}

	if ConfigHash(base) != ConfigHash(base) {
		t.Fatal("hash is not stable")
	}

	presentation, err := ParseFields(`{"grant":"client_credentials","tokenUrl":"https://idp.example/token",
		"clientId":"app","clientSecret":"s3cret","scope":"read","addTo":"query","queryParam":"at",
		"headerPrefix":"Token","redirectPort":"0"}`)
	if err != nil {
		t.Fatalf("ParseFields: %v", err)
	}
	if got := ConfigHash(OAuth2ConfigFromFields(presentation)); got != ConfigHash(base) {
		t.Error("presentation fields must not change the hash")
	}

	changed := base
	changed.Scope = "read write"
	if ConfigHash(changed) == ConfigHash(base) {
		t.Error("a different scope must change the hash")
	}

	if ConfigHash(OAuth2ConfigFromFields(Fields{
		"grant": GrantClientCredentials, "tokenUrl": "https://idp.example/token",
		"clientId": "app", "clientSecret": "s3cret", "scope": "read",
	})) != ConfigHash(base) {
		t.Error("an unset clientAuth must hash like the basic default")
	}
	// A Postman block without grant_type: the form shows client_credentials, and
	// the send path must agree instead of failing on an empty grant.
	if ConfigHash(OAuth2ConfigFromFields(Fields{
		"tokenUrl": "https://idp.example/token", "clientId": "app",
		"clientSecret": "s3cret", "scope": "read",
	})) != ConfigHash(base) {
		t.Error("an unset grant must hash like the client_credentials default")
	}
}

func TestOAuth2ConfigValidate(t *testing.T) {
	valid := OAuth2Config{
		Grant: GrantClientCredentials, TokenURL: "https://idp.example/token",
		ClientID: "app", ClientAuth: ClientAuthBasic,
	}
	if err := valid.validate(); err != nil {
		t.Fatalf("valid config rejected: %v", err)
	}

	tests := []struct {
		name    string
		mutate  func(*OAuth2Config)
		wantErr string
	}{
		{name: "no grant", mutate: func(c *OAuth2Config) { c.Grant = "" }, wantErr: "grant"},
		{name: "unknown grant", mutate: func(c *OAuth2Config) { c.Grant = "hawk" }, wantErr: "hawk"},
		{name: "unknown clientAuth", mutate: func(c *OAuth2Config) { c.ClientAuth = "digest" }, wantErr: "clientAuth"},
		{name: "no clientId", mutate: func(c *OAuth2Config) { c.ClientID = "" }, wantErr: "clientId"},
		{name: "bad tokenUrl", mutate: func(c *OAuth2Config) { c.TokenURL = "http://idp.example/token" }, wantErr: "tokenUrl"},
		{name: "password without credentials", mutate: func(c *OAuth2Config) { c.Grant = GrantPassword }, wantErr: "password grant"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := valid
			tt.mutate(&cfg)
			err := cfg.validate()
			if err == nil {
				t.Fatal("expected an error")
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("error %q does not mention %q", err, tt.wantErr)
			}
		})
	}
}

func codeConfig() OAuth2Config {
	return OAuth2Config{
		Grant: GrantAuthorizationCode, TokenURL: "https://idp.example/token",
		AuthURL: "https://idp.example/authorize", ClientID: "app", ClientAuth: ClientAuthNone,
	}
}

func deviceConfig() OAuth2Config {
	return OAuth2Config{
		Grant: GrantDeviceCode, TokenURL: "https://idp.example/token",
		DeviceAuthURL: "https://idp.example/device", ClientID: "app", ClientAuth: ClientAuthNone,
	}
}

func TestValidateFlowConfig(t *testing.T) {
	tests := []struct {
		name       string
		cfg        OAuth2Config
		grant      string
		wantFields []string
	}{
		{name: "authorization code", cfg: codeConfig(), grant: GrantAuthorizationCode},
		{name: "device code", cfg: deviceConfig(), grant: GrantDeviceCode},
		{
			// The endpoint the grant does not use is nobody's business.
			name: "unused endpoint may be empty", grant: GrantDeviceCode,
			cfg: func() OAuth2Config { c := deviceConfig(); c.AuthURL = ""; return c }(),
		},
		{
			name: "missing authUrl", grant: GrantAuthorizationCode,
			cfg:        func() OAuth2Config { c := codeConfig(); c.AuthURL = ""; return c }(),
			wantFields: []string{"authUrl"},
		},
		{
			name: "missing deviceAuthUrl", grant: GrantDeviceCode,
			cfg:        func() OAuth2Config { c := deviceConfig(); c.DeviceAuthURL = ""; return c }(),
			wantFields: []string{"deviceAuthUrl"},
		},
		{
			name: "remote http authUrl", grant: GrantAuthorizationCode,
			cfg:        func() OAuth2Config { c := codeConfig(); c.AuthURL = "http://idp.example/authorize"; return c }(),
			wantFields: []string{"authUrl"},
		},
		{
			name: "remote http tokenUrl", grant: GrantAuthorizationCode,
			cfg:        func() OAuth2Config { c := codeConfig(); c.TokenURL = "http://idp.example/token"; return c }(),
			wantFields: []string{"tokenUrl"},
		},
		{
			name: "unsupported clientAuth", grant: GrantAuthorizationCode,
			cfg:        func() OAuth2Config { c := codeConfig(); c.ClientAuth = "mtls"; return c }(),
			wantFields: []string{"clientAuth"},
		},
		{
			name: "empty clientId", grant: GrantAuthorizationCode,
			cfg:        func() OAuth2Config { c := codeConfig(); c.ClientID = ""; return c }(),
			wantFields: []string{"clientId"},
		},
		{
			name: "grant mismatch", grant: GrantAuthorizationCode,
			cfg: deviceConfig(), wantFields: []string{"grant", "authUrl"},
		},
		{
			// A non-interactive grant has no browser flow, so its endpoints are not
			// checked at all.
			name: "grant without a flow", grant: GrantClientCredentials,
			cfg: codeConfig(), wantFields: []string{"grant"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateFlowConfig(tt.cfg, tt.grant)
			if len(tt.wantFields) == 0 {
				if err != nil {
					t.Fatalf("ValidateFlowConfig: %v", err)
				}
				return
			}

			var valErr *domain.ValidationError
			if !errors.As(err, &valErr) {
				t.Fatalf("error %v is not a *domain.ValidationError", err)
			}
			if len(valErr.Fields) != len(tt.wantFields) {
				t.Fatalf("fields = %v, want exactly %v", valErr.Fields, tt.wantFields)
			}
			for _, key := range tt.wantFields {
				if valErr.Fields[key] == "" {
					t.Errorf("no message under %q: %v", key, valErr.Fields)
				}
			}
		})
	}
}

func TestValidateRedirectPort(t *testing.T) {
	tests := []struct {
		raw     string
		want    string
		wantErr bool
	}{
		{raw: "", want: defaultRedirectPort},
		{raw: "   ", want: defaultRedirectPort},
		{raw: "0", want: "0"},
		{raw: "1", want: "1"},
		{raw: " 21830 ", want: "21830"},
		{raw: "65535", want: "65535"},
		{raw: "65536", wantErr: true},
		{raw: "-1", wantErr: true},
		{raw: "+80", wantErr: true},
		{raw: "8o80", wantErr: true},
		{raw: "1e3", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.raw, func(t *testing.T) {
			got, err := ValidateRedirectPort(tt.raw)
			if tt.wantErr {
				var valErr *domain.ValidationError
				if !errors.As(err, &valErr) {
					t.Fatalf("ValidateRedirectPort(%q) = %q, %v; want a *domain.ValidationError", tt.raw, got, err)
				}
				if valErr.Fields["redirectPort"] == "" {
					t.Errorf("no message under redirectPort: %v", valErr.Fields)
				}
				return
			}
			if err != nil {
				t.Fatalf("ValidateRedirectPort(%q): %v", tt.raw, err)
			}
			if got != tt.want {
				t.Errorf("ValidateRedirectPort(%q) = %q, want %q", tt.raw, got, tt.want)
			}
		})
	}
}
