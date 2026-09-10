package auth

import (
	"context"
	"encoding/base64"
	"errors"
	"maps"
	"net/http"
	"net/http/httptest"
	"net/url"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"
)

var testNow = time.Date(2026, 9, 4, 12, 0, 0, 0, time.UTC)

// idp is a token endpoint that records what it received and answers with a canned body. Every field
// is mutex-guarded: the concurrency test drives several handler goroutines at once.
type idp struct {
	server *httptest.Server

	mu      sync.Mutex
	hits    int
	form    url.Values
	authHdr string
	headers http.Header
	status  int
	body    string
	hold    chan struct{}
	arrived chan struct{}
}

func newIDP(t *testing.T, body string) *idp {
	t.Helper()

	s := &idp{status: http.StatusOK, body: body, arrived: make(chan struct{}, 64)}
	s.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			t.Errorf("parse form: %v", err)
		}

		s.mu.Lock()
		s.hits++
		s.form = r.PostForm
		s.authHdr = r.Header.Get("Authorization")
		s.headers = r.Header.Clone()
		status, body, hold := s.status, s.body, s.hold
		s.mu.Unlock()

		select {
		case s.arrived <- struct{}{}:
		default:
		}
		if hold != nil {
			<-hold
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(s.server.Close)

	return s
}

func (s *idp) url() string { return s.server.URL + "/token" }

func (s *idp) set(status int, body string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.status, s.body = status, body
}

func (s *idp) block() chan struct{} {
	hold := make(chan struct{})
	s.mu.Lock()
	defer s.mu.Unlock()
	s.hold = hold
	return hold
}

func (s *idp) hitCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.hits
}

func (s *idp) lastForm() url.Values {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.form
}

func (s *idp) lastAuth() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.authHdr
}

func (s *idp) lastHeaders() http.Header {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.headers
}

func (s *idp) deviceURL() string { return s.server.URL + "/device" }

func ccForm(t *testing.T, cfg OAuth2Config) url.Values {
	t.Helper()

	form, err := acquisitionForm(cfg)
	if err != nil {
		t.Fatalf("acquisitionForm: %v", err)
	}

	return form
}

// assertForm compares the whole posted form, keys and multiplicities alike: the
// point of postForm is that it adds nothing but client authentication.
func assertForm(t *testing.T, got, want url.Values) {
	t.Helper()

	for key, wantVals := range want {
		if !slices.Equal(got[key], wantVals) {
			t.Errorf("form[%q] = %v, want %v", key, got[key], wantVals)
		}
	}
	for key, gotVals := range got {
		if _, ok := want[key]; !ok {
			t.Errorf("form carries an unexpected %q = %v", key, gotVals)
		}
	}
}

func TestRequestTokenClientCredentialsBasic(t *testing.T) {
	server := newIDP(t, `{"access_token":"at-1","token_type":"Bearer","expires_in":3600,"scope":"read","refresh_token":"rt-1"}`)
	cfg := OAuth2Config{
		Grant: GrantClientCredentials, TokenURL: server.url(), ClientID: "app",
		ClientSecret: "s+p a/ce", ClientAuth: ClientAuthBasic, Scope: "read", Audience: "api://x",
	}

	tok, err := requestToken(context.Background(), NewTokenHTTPClient(), cfg, ccForm(t, cfg), testNow)
	if err != nil {
		t.Fatalf("requestToken: %v", err)
	}

	// RFC 6749 §2.3.1: both halves are form-urlencoded before the base64 step.
	want := "Basic " + base64.StdEncoding.EncodeToString([]byte("app:s%2Bp+a%2Fce"))
	if got := server.lastAuth(); got != want {
		t.Errorf("Authorization: got %q, want %q", got, want)
	}
	form := server.lastForm()
	if got := form.Get("client_secret"); got != "" {
		t.Errorf("basic client auth must not put the secret in the body, got %q", got)
	}
	if got := form.Get("grant_type"); got != GrantClientCredentials {
		t.Errorf("grant_type: got %q", got)
	}
	if got := form.Get("scope"); got != "read" {
		t.Errorf("scope: got %q", got)
	}
	if got := form.Get("audience"); got != "api://x" {
		t.Errorf("audience: got %q", got)
	}
	if ct := server.lastHeaders().Get("Content-Type"); ct != "application/x-www-form-urlencoded" {
		t.Errorf("Content-Type: got %q", ct)
	}
	if tok.AccessToken != "at-1" || tok.RefreshToken != "rt-1" || tok.TokenType != "Bearer" || tok.Scope != "read" {
		t.Errorf("token fields: %+v", tok)
	}
	if !tok.ExpiresAt.Equal(testNow.Add(time.Hour)) {
		t.Errorf("ExpiresAt: got %v", tok.ExpiresAt)
	}
	if !tok.ObtainedAt.Equal(testNow) {
		t.Errorf("ObtainedAt: got %v", tok.ObtainedAt)
	}
}

func TestRequestTokenPasswordBodyAuth(t *testing.T) {
	server := newIDP(t, `{"access_token":"at-2","expires_in":"600"}`)
	cfg := OAuth2Config{
		Grant: GrantPassword, TokenURL: server.url(), ClientID: "app", ClientSecret: "s3cret",
		ClientAuth: ClientAuthBody, Username: "bob", Password: "hunter2",
	}
	form := url.Values{"grant_type": {GrantPassword}, "username": {cfg.Username}, "password": {cfg.Password}}

	tok, err := requestToken(context.Background(), NewTokenHTTPClient(), cfg, form, testNow)
	if err != nil {
		t.Fatalf("requestToken: %v", err)
	}

	if got := server.lastAuth(); got != "" {
		t.Errorf("body client auth must not send an Authorization header, got %q", got)
	}
	sent := server.lastForm()
	if sent.Get("client_id") != "app" || sent.Get("client_secret") != "s3cret" {
		t.Errorf("client credentials missing from the body: %v", sent)
	}
	if sent.Get("username") != "bob" || sent.Get("password") != "hunter2" {
		t.Errorf("resource-owner credentials missing: %v", sent)
	}
	// A string expires_in is common enough in the wild to be worth accepting.
	if !tok.ExpiresAt.Equal(testNow.Add(10 * time.Minute)) {
		t.Errorf("string expires_in ignored: %v", tok.ExpiresAt)
	}
}

func TestRequestTokenPublicClient(t *testing.T) {
	server := newIDP(t, `{"access_token":"at-3"}`)
	cfg := OAuth2Config{
		Grant: GrantClientCredentials, TokenURL: server.url(), ClientID: "public-app",
		ClientSecret: "ignored", ClientAuth: ClientAuthNone,
	}

	tok, err := requestToken(context.Background(), NewTokenHTTPClient(), cfg, ccForm(t, cfg), testNow)
	if err != nil {
		t.Fatalf("requestToken: %v", err)
	}

	if got := server.lastAuth(); got != "" {
		t.Errorf("public client must not authenticate, got %q", got)
	}
	sent := server.lastForm()
	if sent.Get("client_id") != "public-app" {
		t.Errorf("client_id: got %q", sent.Get("client_id"))
	}
	if sent.Has("client_secret") {
		t.Error("public client must not send a secret")
	}
	if !tok.ExpiresAt.IsZero() {
		t.Errorf("a response without expires_in must leave ExpiresAt zero, got %v", tok.ExpiresAt)
	}
}

func TestRequestTokenErrors(t *testing.T) {
	tests := []struct {
		name    string
		status  int
		body    string
		wantErr []string
	}{
		{
			name: "rfc 6749 error", status: http.StatusBadRequest,
			body:    `{"error":"invalid_client","error_description":"client secret is wrong"}`,
			wantErr: []string{"invalid_client", "client secret is wrong"},
		},
		{
			name: "error without description", status: http.StatusBadRequest,
			body: `{"error":"unsupported_grant_type"}`, wantErr: []string{"unsupported_grant_type"},
		},
		{
			name: "html body", status: http.StatusInternalServerError,
			body: `<html>oops</html>`, wantErr: []string{"500"},
		},
		{
			name: "no access_token", status: http.StatusOK,
			body: `{"token_type":"Bearer"}`, wantErr: []string{"access_token"},
		},
		{
			name: "non-json error", status: http.StatusForbidden,
			body: `denied`, wantErr: []string{"403"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := newIDP(t, tt.body)
			server.set(tt.status, tt.body)
			cfg := OAuth2Config{
				Grant: GrantClientCredentials, TokenURL: server.url(),
				ClientID: "app", ClientAuth: ClientAuthBasic,
			}

			_, err := requestToken(context.Background(), NewTokenHTTPClient(), cfg, ccForm(t, cfg), testNow)
			if err == nil {
				t.Fatal("expected an error")
			}
			for _, want := range tt.wantErr {
				if !strings.Contains(err.Error(), want) {
					t.Fatalf("error %q does not mention %q", err, want)
				}
			}
		})
	}
}

func TestRequestTokenDoesNotFollowRedirects(t *testing.T) {
	upstream := newIDP(t, `{"access_token":"leaked"}`)
	redirector := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, upstream.url(), http.StatusTemporaryRedirect)
	}))
	t.Cleanup(redirector.Close)
	cfg := OAuth2Config{
		Grant: GrantClientCredentials, TokenURL: redirector.URL + "/token",
		ClientID: "app", ClientSecret: "s3cret", ClientAuth: ClientAuthBasic,
	}

	_, err := requestToken(context.Background(), NewTokenHTTPClient(), cfg, ccForm(t, cfg), testNow)
	if err == nil {
		t.Fatal("expected a redirect to be reported, not followed")
	}
	if !strings.Contains(err.Error(), "redirect") {
		t.Errorf("error %q does not mention the redirect", err)
	}
	if upstream.hitCount() != 0 {
		t.Errorf("the client secret was replayed at the redirect target (%d hits)", upstream.hitCount())
	}
}

func TestRequestTokenRejectsInsecureEndpoint(t *testing.T) {
	cfg := OAuth2Config{
		Grant: GrantClientCredentials, TokenURL: "http://idp.example/token",
		ClientID: "app", ClientAuth: ClientAuthBasic,
	}

	form := url.Values{"grant_type": {GrantClientCredentials}}
	_, err := requestToken(context.Background(), NewTokenHTTPClient(), cfg, form, testNow)
	if err == nil {
		t.Fatal("expected plain http to a public host to be rejected")
	}
	if !strings.Contains(err.Error(), "tokenUrl") {
		t.Errorf("error %q does not name the field", err)
	}
}

func TestNewTokenHTTPClient(t *testing.T) {
	c := NewTokenHTTPClient()

	if c.Jar != nil {
		t.Error("the token client must not carry workspace cookies")
	}
	if c.Timeout != tokenEndpointTimeout {
		t.Errorf("timeout: got %v", c.Timeout)
	}
	if err := c.CheckRedirect(nil, nil); err != http.ErrUseLastResponse {
		t.Errorf("CheckRedirect: got %v", err)
	}
}

// clientAuthExtras is what postForm may add to a form, and nothing else.
func clientAuthExtras(clientAuth, clientID, clientSecret string) url.Values {
	switch clientAuth {
	case ClientAuthBody:
		return url.Values{"client_id": {clientID}, "client_secret": {clientSecret}}
	case ClientAuthNone:
		return url.Values{"client_id": {clientID}}
	default:
		return url.Values{}
	}
}

func TestPostFormSendsExactlyTheCallersForm(t *testing.T) {
	const (
		clientID     = "app"
		clientSecret = "s3cret"
	)

	grants := []struct {
		name  string
		grant string
		form  func(t *testing.T, cfg OAuth2Config) url.Values
		want  url.Values
	}{
		{
			name:  "client credentials",
			grant: GrantClientCredentials,
			form:  ccForm,
			want: url.Values{
				"grant_type": {GrantClientCredentials},
				"scope":      {"read"}, "audience": {"api://x"},
			},
		},
		{
			name:  "password",
			grant: GrantPassword,
			form:  ccForm,
			want: url.Values{
				"grant_type": {GrantPassword}, "username": {"bob"}, "password": {"hunter2"},
				"scope": {"read"}, "audience": {"api://x"},
			},
		},
		{
			name:  "refresh",
			grant: GrantClientCredentials,
			form: func(_ *testing.T, cfg OAuth2Config) url.Values {
				return refreshForm(cfg, "rt-old")
			},
			want: url.Values{
				"grant_type": {"refresh_token"}, "refresh_token": {"rt-old"},
				"scope": {"read"}, "audience": {"api://x"},
			},
		},
		{
			// The authorize step already fixed scope and audience; repeating them
			// here makes IdPs that compare the two requests reject the exchange.
			name:  "authorization code exchange",
			grant: GrantAuthorizationCode,
			form: func(_ *testing.T, _ OAuth2Config) url.Values {
				return url.Values{
					"grant_type": {GrantAuthorizationCode}, "code": {"the-code"},
					"redirect_uri":  {"http://127.0.0.1:21830/callback"},
					"code_verifier": {"the-verifier"},
				}
			},
			want: url.Values{
				"grant_type": {GrantAuthorizationCode}, "code": {"the-code"},
				"redirect_uri":  {"http://127.0.0.1:21830/callback"},
				"code_verifier": {"the-verifier"},
			},
		},
		{
			name:  "device authorization",
			grant: GrantDeviceCode,
			form: func(_ *testing.T, cfg OAuth2Config) url.Values {
				return url.Values{"scope": {cfg.Scope}, "audience": {cfg.Audience}}
			},
			want: url.Values{"scope": {"read"}, "audience": {"api://x"}},
		},
		{
			name:  "device poll",
			grant: GrantDeviceCode,
			form: func(_ *testing.T, _ OAuth2Config) url.Values {
				return url.Values{
					"grant_type":  {"urn:ietf:params:oauth:grant-type:device_code"},
					"device_code": {"dc-1"},
				}
			},
			want: url.Values{
				"grant_type":  {"urn:ietf:params:oauth:grant-type:device_code"},
				"device_code": {"dc-1"},
			},
		},
	}

	for _, tt := range grants {
		for _, clientAuth := range []string{ClientAuthBasic, ClientAuthBody, ClientAuthNone} {
			t.Run(tt.name+"/"+clientAuth, func(t *testing.T) {
				server := newIDP(t, `{"access_token":"at"}`)
				cfg := OAuth2Config{
					Grant: tt.grant, TokenURL: server.url(), DeviceAuthURL: server.deviceURL(),
					ClientID: clientID, ClientSecret: clientSecret, ClientAuth: clientAuth,
					Scope: "read", Audience: "api://x", Username: "bob", Password: "hunter2",
				}
				endpoint, err := ValidateEndpointURL(cfg.TokenURL)
				if err != nil {
					t.Fatalf("ValidateEndpointURL: %v", err)
				}

				if _, err := postForm(context.Background(), NewTokenHTTPClient(), cfg, endpoint, tt.form(t, cfg)); err != nil {
					t.Fatalf("postForm: %v", err)
				}

				want := url.Values{}
				maps.Copy(want, tt.want)
				maps.Copy(want, clientAuthExtras(clientAuth, clientID, clientSecret))
				assertForm(t, server.lastForm(), want)

				wantAuth := ""
				if clientAuth == ClientAuthBasic {
					wantAuth = "Basic " + base64.StdEncoding.EncodeToString([]byte(clientID+":"+clientSecret))
				}
				if got := server.lastAuth(); got != wantAuth {
					t.Errorf("Authorization: got %q, want %q", got, wantAuth)
				}
			})
		}
	}
}

func TestPostFormRefusesRedirect(t *testing.T) {
	upstream := newIDP(t, `{"device_code":"leaked"}`)
	redirector := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, upstream.deviceURL(), http.StatusTemporaryRedirect)
	}))
	t.Cleanup(redirector.Close)

	cfg := OAuth2Config{
		Grant: GrantDeviceCode, DeviceAuthURL: redirector.URL + "/device",
		ClientID: "app", ClientSecret: "s3cret", ClientAuth: ClientAuthBody,
	}
	endpoint, err := ValidateEndpointURL(cfg.DeviceAuthURL)
	if err != nil {
		t.Fatalf("ValidateEndpointURL: %v", err)
	}

	_, err = postForm(context.Background(), NewTokenHTTPClient(), cfg, endpoint, url.Values{})
	if err == nil {
		t.Fatal("expected a redirect to be reported, not followed")
	}
	if !strings.Contains(err.Error(), "redirect") {
		t.Errorf("error %q does not mention the redirect", err)
	}
	if upstream.hitCount() != 0 {
		t.Errorf("the client secret was replayed at the redirect target (%d hits)", upstream.hitCount())
	}
}

func TestPostFormRejectsOversizedBody(t *testing.T) {
	// Valid JSON well past the cap: a truncating reader would report a parse error
	// instead, which reads to the user as a broken IdP rather than a huge one.
	body := `{"access_token":"` + strings.Repeat("a", maxTokenResponse) + `"}`
	server := newIDP(t, body)
	cfg := OAuth2Config{
		Grant: GrantClientCredentials, TokenURL: server.url(),
		ClientID: "app", ClientAuth: ClientAuthNone,
	}
	endpoint, err := ValidateEndpointURL(cfg.TokenURL)
	if err != nil {
		t.Fatalf("ValidateEndpointURL: %v", err)
	}

	_, err = postForm(context.Background(), NewTokenHTTPClient(), cfg, endpoint, url.Values{})
	if err == nil {
		t.Fatal("expected an oversized body to be rejected")
	}
	if !strings.Contains(err.Error(), "more than") {
		t.Errorf("error %q does not name the size cap", err)
	}
}

func TestRequestTokenTypedOAuthError(t *testing.T) {
	server := newIDP(t, "")
	server.set(http.StatusBadRequest, `{"error":"authorization_pending","error_description":"waiting"}`)
	cfg := OAuth2Config{
		Grant: GrantClientCredentials, TokenURL: server.url(),
		ClientID: "app", ClientAuth: ClientAuthBasic,
	}

	_, err := requestToken(context.Background(), NewTokenHTTPClient(), cfg, ccForm(t, cfg), testNow)
	var oauthErr *OAuthError
	if !errors.As(err, &oauthErr) {
		t.Fatalf("error %v is not an *OAuthError", err)
	}
	if oauthErr.Code != OAuthErrAuthorizationPending || oauthErr.Description != "waiting" {
		t.Errorf("parsed error = %+v", oauthErr)
	}
	// The stage-A message is part of the contract: a plain error must read the same.
	if got := err.Error(); got != "oauth2: authorization_pending: waiting" {
		t.Errorf("Error() = %q", got)
	}

	server.set(http.StatusBadRequest, `{"error":"access_denied"}`)
	_, err = requestToken(context.Background(), NewTokenHTTPClient(), cfg, ccForm(t, cfg), testNow)
	if !errors.As(err, &oauthErr) || oauthErr.Code != OAuthErrAccessDenied {
		t.Fatalf("error %v is not access_denied", err)
	}
	if got := err.Error(); got != "oauth2: access_denied" {
		t.Errorf("Error() = %q", got)
	}
}
