package main

import (
	"context"
	"encoding/json"
	"errors"
	"html"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"

	"github.com/tetiva-app/client/internal/adapters/requester"
	"github.com/tetiva-app/client/internal/domain"
	"github.com/tetiva-app/client/internal/domain/entities"
	"github.com/tetiva-app/client/internal/domain/usecase/request"
)

func newTestServer(t *testing.T) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(newServer().routes())
	t.Cleanup(srv.Close)

	return srv
}

func newTestServerFor(t *testing.T, opts serverOptions) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(newServerWithOptions(opts).routes())
	t.Cleanup(srv.Close)

	return srv
}

// testClock is read by the handler goroutines, so it carries its own lock.
type testClock struct {
	mu sync.Mutex
	at time.Time
}

func (c *testClock) now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()

	return c.at
}

func (c *testClock) advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.at = c.at.Add(d)
}

func postToken(t *testing.T, srv *httptest.Server, path string, form url.Values, basic []string) (int, map[string]any) {
	t.Helper()
	req, err := http.NewRequest(http.MethodPost, srv.URL+path, strings.NewReader(form.Encode()))
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if basic != nil {
		req.SetBasicAuth(url.QueryEscape(basic[0]), url.QueryEscape(basic[1]))
	}

	resp, err := srv.Client().Do(req)
	if err != nil {
		t.Fatalf("token request: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	var decoded map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&decoded); err != nil {
		t.Fatalf("decode token response: %v", err)
	}

	return resp.StatusCode, decoded
}

func whoami(t *testing.T, srv *httptest.Server, header string) (int, whoamiResponse) {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, srv.URL+"/api/whoami", nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	if header != "" {
		req.Header.Set("Authorization", header)
	}

	resp, err := srv.Client().Do(req)
	if err != nil {
		t.Fatalf("whoami: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	var decoded whoamiResponse
	_ = json.NewDecoder(resp.Body).Decode(&decoded)

	return resp.StatusCode, decoded
}

func TestTokenEndpointClientCredentialsWithBasicAuth(t *testing.T) {
	srv := newTestServer(t)

	status, body := postToken(t, srv, "/oauth2/token",
		url.Values{"grant_type": {"client_credentials"}, "scope": {"read write"}},
		[]string{clientID, clientSecret})
	if status != http.StatusOK {
		t.Fatalf("status = %d, want 200 (%v)", status, body)
	}
	access, _ := body["access_token"].(string)
	if access == "" {
		t.Fatal("no access_token in the response")
	}
	if body["refresh_token"] == "" || body["refresh_token"] == nil {
		t.Fatal("client_credentials answer carried no refresh_token")
	}
	if body["expires_in"] != float64(defaultExpires) {
		t.Fatalf("expires_in = %v, want %d", body["expires_in"], defaultExpires)
	}

	status, who := whoami(t, srv, "Bearer "+access)
	if status != http.StatusOK || who.Scheme != "oauth2" || who.Grant != "client_credentials" {
		t.Fatalf("whoami = %d %+v, want 200 oauth2/client_credentials", status, who)
	}
	if who.Scope != "read write" {
		t.Fatalf("scope = %q, want the requested one", who.Scope)
	}
}

func TestTokenEndpointPasswordGrantWithBodyClientAuth(t *testing.T) {
	srv := newTestServer(t)

	status, body := postToken(t, srv, "/oauth2/token", url.Values{
		"grant_type":    {"password"},
		"username":      {passwordUser},
		"password":      {passwordPass},
		"client_id":     {clientID},
		"client_secret": {clientSecret},
	}, nil)
	if status != http.StatusOK {
		t.Fatalf("status = %d, want 200 (%v)", status, body)
	}

	_, who := whoami(t, srv, "Bearer "+body["access_token"].(string))
	if who.Grant != "password" {
		t.Fatalf("grant = %q, want password", who.Grant)
	}

	status, body = postToken(t, srv, "/oauth2/token", url.Values{
		"grant_type": {"password"},
		"username":   {passwordUser},
		"password":   {"wrong"},
		"client_id":  {clientID},
	}, nil)
	if status != http.StatusBadRequest || body["error"] != "invalid_grant" {
		t.Fatalf("wrong password = %d %v, want 400 invalid_grant", status, body)
	}
}

func TestTokenEndpointRefreshOmitsANewRefreshToken(t *testing.T) {
	srv := newTestServer(t)

	_, first := postToken(t, srv, "/oauth2/token",
		url.Values{"grant_type": {"client_credentials"}}, []string{clientID, clientSecret})
	refresh := first["refresh_token"].(string)

	status, second := postToken(t, srv, "/oauth2/token",
		url.Values{"grant_type": {"refresh_token"}, "refresh_token": {refresh}},
		[]string{clientID, clientSecret})
	if status != http.StatusOK {
		t.Fatalf("refresh status = %d, want 200 (%v)", status, second)
	}
	if _, present := second["refresh_token"]; present {
		t.Fatal("refresh answer carried a new refresh_token; the client must keep the old one")
	}
	if second["access_token"] == first["access_token"] {
		t.Fatal("refresh returned the same access token")
	}

	_, who := whoami(t, srv, "Bearer "+second["access_token"].(string))
	if who.Grant != "refresh_token" {
		t.Fatalf("grant = %q, want refresh_token", who.Grant)
	}

	status, _ = postToken(t, srv, "/oauth2/token",
		url.Values{"grant_type": {"refresh_token"}, "refresh_token": {"nope"}},
		[]string{clientID, clientSecret})
	if status != http.StatusBadRequest {
		t.Fatalf("unknown refresh token = %d, want 400", status)
	}
}

func TestTokenEndpointRejectsAWrongSecretAndAnUnknownGrant(t *testing.T) {
	srv := newTestServer(t)

	status, body := postToken(t, srv, "/oauth2/token",
		url.Values{"grant_type": {"client_credentials"}}, []string{clientID, "nope"})
	if status != http.StatusUnauthorized || body["error"] != "invalid_client" {
		t.Fatalf("wrong secret = %d %v, want 401 invalid_client", status, body)
	}

	status, body = postToken(t, srv, "/oauth2/token",
		url.Values{"grant_type": {"device_code"}}, []string{clientID, clientSecret})
	if status != http.StatusBadRequest || body["error"] != "unsupported_grant_type" {
		t.Fatalf("device_code = %d %v, want 400 unsupported_grant_type", status, body)
	}
}

func TestTokenEndpointExpiresInQueryShortensTheToken(t *testing.T) {
	srv := newTestServer(t)

	status, body := postToken(t, srv, "/oauth2/token?expires_in=45",
		url.Values{"grant_type": {"client_credentials"}}, []string{clientID, clientSecret})
	if status != http.StatusOK || body["expires_in"] != float64(45) {
		t.Fatalf("expires_in = %v (status %d), want 45", body["expires_in"], status)
	}
}

func TestWhoamiAcceptsASignedJWTAndRejectsAForeignOne(t *testing.T) {
	srv := newTestServer(t)

	sign := func(secret string) string {
		token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
			"sub": "user-42",
			"iat": time.Now().Unix(),
			"exp": time.Now().Add(time.Hour).Unix(),
		})
		signed, err := token.SignedString([]byte(secret))
		if err != nil {
			t.Fatalf("sign: %v", err)
		}
		return signed
	}

	status, who := whoami(t, srv, "Bearer "+sign(jwtSecret))
	if status != http.StatusOK || who.Scheme != "jwt" || who.Subject != "user-42" {
		t.Fatalf("whoami = %d %+v, want 200 jwt/user-42", status, who)
	}

	if status, _ = whoami(t, srv, "Bearer "+sign("other-secret")); status != http.StatusUnauthorized {
		t.Fatalf("foreign JWT = %d, want 401", status)
	}
	if status, _ = whoami(t, srv, "Bearer not-a-token"); status != http.StatusUnauthorized {
		t.Fatalf("garbage bearer = %d, want 401", status)
	}
}

func TestWhoamiTakesTheTokenFromTheQueryWhenThereIsNoHeader(t *testing.T) {
	srv := newTestServer(t)

	_, body := postToken(t, srv, "/oauth2/token",
		url.Values{"grant_type": {"client_credentials"}}, []string{clientID, clientSecret})

	resp, err := srv.Client().Get(srv.URL + "/api/whoami?access_token=" + url.QueryEscape(body["access_token"].(string)))
	if err != nil {
		t.Fatalf("whoami: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	var who whoamiResponse
	_ = json.NewDecoder(resp.Body).Decode(&who)
	if resp.StatusCode != http.StatusOK || who.Scheme != "oauth2" {
		t.Fatalf("whoami = %d %+v, want 200 oauth2", resp.StatusCode, who)
	}
}

func TestWhoamiChallengesADigestClientWithoutCredentials(t *testing.T) {
	srv := newTestServer(t)

	resp, err := srv.Client().Get(srv.URL + "/api/whoami")
	if err != nil {
		t.Fatalf("whoami: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", resp.StatusCode)
	}
	challenges := resp.Header.Values("WWW-Authenticate")
	if len(challenges) != 2 || !strings.Contains(challenges[0], "algorithm=SHA-256") {
		t.Fatalf("challenges = %v, want SHA-256 first then MD5", challenges)
	}
	var session bool
	for _, c := range resp.Cookies() {
		session = session || c.Name == sessionCookie
	}
	if !session {
		t.Fatal("the challenge set no session cookie, so its nonce cannot be bound to one")
	}
}

// cookieStore is the requester's CookieStore over an in-memory jar.
type cookieStore struct{ jar *cookiejar.Jar }

func newCookieStore(t *testing.T) *cookieStore {
	t.Helper()
	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatalf("cookiejar: %v", err)
	}

	return &cookieStore{jar: jar}
}

func (c *cookieStore) GetCookiesFor(_ context.Context, _ uuid.UUID, u *url.URL) []*http.Cookie {
	return c.jar.Cookies(u)
}

func (c *cookieStore) SetCookies(_ context.Context, _ uuid.UUID, u *url.URL, cookies []*http.Cookie) error {
	c.jar.SetCookies(u, cookies)

	return nil
}

func send(t *testing.T, store requester.CookieStore, req request.HTTPExecuteRequest) *entities.Response {
	t.Helper()
	resp, err := requester.NewHTTPRequester(store).Execute(context.Background(), req)
	if err != nil {
		t.Fatalf("execute: %v", err)
	}

	return resp
}

func TestWhoamiVerifiesDigestFromTheRealRequester(t *testing.T) {
	srv := newTestServer(t)

	for _, tc := range []struct{ name, query, algorithm string }{
		{name: "server picks SHA-256", query: "", algorithm: "SHA-256"},
		{name: "MD5 only", query: "?alg=MD5", algorithm: "MD5"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			resp := send(t, newCookieStore(t), request.HTTPExecuteRequest{
				Method:      entities.MethodPOST,
				URL:         srv.URL + "/api/whoami" + tc.query,
				Body:        `{"hello":"digest"}`,
				Headers:     map[string][]string{"Content-Type": {"application/json"}},
				WorkspaceID: uuid.New(),
				Auth: &request.RequestAuth{
					Type:   entities.AuthTypeDigest,
					Fields: map[string]any{"username": digestUser, "password": digestPass},
				},
			})

			if resp.StatusCode != http.StatusOK {
				t.Fatalf("status = %d (%s), want 200", resp.StatusCode, resp.Body)
			}
			var who whoamiResponse
			if err := json.Unmarshal([]byte(resp.Body), &who); err != nil {
				t.Fatalf("decode: %v", err)
			}
			if who.Scheme != "digest" || who.Algorithm != tc.algorithm || who.User != digestUser {
				t.Fatalf("answer = %+v, want digest/%s/%s", who, tc.algorithm, digestUser)
			}
		})
	}
}

func TestWhoamiRejectsDigestWhenTheSessionCookieIsDropped(t *testing.T) {
	srv := newTestServer(t)

	// A nil store leaves the digest transport without a jar, so the challenge
	// cookie never comes back and the nonce belongs to nobody.
	resp := send(t, nil, request.HTTPExecuteRequest{
		Method:      entities.MethodGET,
		URL:         srv.URL + "/api/whoami",
		WorkspaceID: uuid.New(),
		Auth: &request.RequestAuth{
			Type:   entities.AuthTypeDigest,
			Fields: map[string]any{"username": digestUser, "password": digestPass},
		},
	})
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %d (%s), want 401", resp.StatusCode, resp.Body)
	}
}

func TestWhoamiReportsAnUnsupportedDigestAlgorithm(t *testing.T) {
	srv := newTestServer(t)

	_, err := requester.NewHTTPRequester(newCookieStore(t)).Execute(context.Background(), request.HTTPExecuteRequest{
		Method:      entities.MethodGET,
		URL:         srv.URL + "/api/whoami?alg=MD5-sess",
		WorkspaceID: uuid.New(),
		Auth: &request.RequestAuth{
			Type:   entities.AuthTypeDigest,
			Fields: map[string]any{"username": digestUser, "password": digestPass},
		},
	})
	var invalid *domain.ValidationError
	if !errors.As(err, &invalid) || !strings.Contains(invalid.Fields["auth"], "MD5-sess") {
		t.Fatalf("err = %v, want a readable MD5-sess rejection", err)
	}
}

func TestWhoamiVerifiesSigV4FromTheRealRequester(t *testing.T) {
	srv := newTestServer(t)
	creds := map[string]any{
		"accessKeyId":     awsAccessKeyID,
		"secretAccessKey": awsSecretAccessKey,
		"region":          "us-east-1",
		"service":         "execute-api",
	}

	cases := []struct {
		name    string
		req     request.HTTPExecuteRequest
		service string
	}{
		{
			name:    "GET with a query string",
			service: "execute-api",
			req: request.HTTPExecuteRequest{
				Method: entities.MethodGET,
				URL:    srv.URL + "/api/whoami?b=2&a=1&x=hello%20world",
			},
		},
		{
			name:    "POST with a body and a content type",
			service: "execute-api",
			req: request.HTTPExecuteRequest{
				Method:  entities.MethodPOST,
				URL:     srv.URL + "/api/whoami",
				Body:    `{"hello":"sigv4"}`,
				Headers: map[string][]string{"Content-Type": {"application/json"}, "X-Amz-Target": {"Demo.Put"}},
			},
		},
		{
			name:    "S3 style with x-amz-content-sha256 and metadata",
			service: "s3",
			req: request.HTTPExecuteRequest{
				Method:  entities.MethodPUT,
				URL:     srv.URL + "/api/whoami",
				Body:    "payload bytes",
				Headers: map[string][]string{"X-Amz-Meta-Foo": {"bar"}},
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fields := map[string]any{}
			for k, v := range creds {
				fields[k] = v
			}
			fields["service"] = tc.service
			fields["sessionToken"] = "FQoGZXIvYXdzEXAMPLE"

			req := tc.req
			req.WorkspaceID = uuid.New()
			req.Auth = &request.RequestAuth{Type: entities.AuthTypeAWSSigV4, Fields: fields}
			resp := send(t, newCookieStore(t), req)

			if resp.StatusCode != http.StatusOK {
				t.Fatalf("status = %d (%s), want 200", resp.StatusCode, resp.Body)
			}
			var who whoamiResponse
			if err := json.Unmarshal([]byte(resp.Body), &who); err != nil {
				t.Fatalf("decode: %v", err)
			}
			if who.Scheme != "aws_sigv4" || who.Service != tc.service || who.Region != "us-east-1" {
				t.Fatalf("answer = %+v, want aws_sigv4/%s/us-east-1", who, tc.service)
			}
		})
	}
}

func TestWhoamiRejectsATamperedSigV4Request(t *testing.T) {
	srv := newTestServer(t)

	resp := send(t, newCookieStore(t), request.HTTPExecuteRequest{
		Method:      entities.MethodPOST,
		URL:         srv.URL + "/api/whoami",
		Body:        `{"hello":"sigv4"}`,
		WorkspaceID: uuid.New(),
		Auth: &request.RequestAuth{
			Type: entities.AuthTypeAWSSigV4,
			Fields: map[string]any{
				"accessKeyId":     awsAccessKeyID,
				"secretAccessKey": "not-the-secret",
				"region":          "us-east-1",
				"service":         "execute-api",
			},
		},
	})
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %d (%s), want 401", resp.StatusCode, resp.Body)
	}
}

// A verifier of the length RFC 7636 §4.1 asks for; the mock only ever hashes it.
const testVerifier = "test-verifier-0123456789012345678901234567890"

const testRedirectURI = "http://127.0.0.1:21830/callback"

func authorizeQuery() url.Values {
	return url.Values{
		"response_type":         {"code"},
		"client_id":             {clientID},
		"redirect_uri":          {testRedirectURI},
		"state":                 {"state-value"},
		"code_challenge":        {pkceChallenge(testVerifier)},
		"code_challenge_method": {"S256"},
	}
}

var hrefPattern = regexp.MustCompile(`href="([^"]+)"`)

// authorize runs one GET /oauth2/authorize: on 200 it returns the target the
// page redirects to, otherwise the decoded error body.
func authorize(t *testing.T, srv *httptest.Server, q url.Values) (int, *url.URL, map[string]string) {
	t.Helper()
	resp, err := srv.Client().Get(srv.URL + "/oauth2/authorize?" + q.Encode())
	if err != nil {
		t.Fatalf("authorize: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read authorize body: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		decoded := map[string]string{}
		if err := json.Unmarshal(body, &decoded); err != nil {
			t.Fatalf("decode authorize error: %v (%s)", err, body)
		}

		return resp.StatusCode, nil, decoded
	}

	match := hrefPattern.FindStringSubmatch(string(body))
	if match == nil {
		t.Fatalf("the authorize page carried no redirect: %s", body)
	}
	target, err := url.Parse(html.UnescapeString(match[1]))
	if err != nil {
		t.Fatalf("parse redirect target: %v", err)
	}

	return resp.StatusCode, target, nil
}

func exchangeForm(code string) url.Values {
	return url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {code},
		"redirect_uri":  {testRedirectURI},
		"code_verifier": {testVerifier},
	}
}

func TestAuthorizeIssuesACodeTheExchangeSpendsOnce(t *testing.T) {
	srv := newTestServer(t)

	status, target, _ := authorize(t, srv, authorizeQuery())
	if status != http.StatusOK {
		t.Fatalf("authorize status = %d, want 200", status)
	}
	if got := target.Scheme + "://" + target.Host + target.Path; got != testRedirectURI {
		t.Fatalf("redirect target = %q, want %q", got, testRedirectURI)
	}
	if target.Query().Get("state") != "state-value" {
		t.Fatalf("state = %q, want the one that was sent", target.Query().Get("state"))
	}
	code := target.Query().Get("code")
	if code == "" {
		t.Fatal("the redirect carried no code")
	}

	status, body := postToken(t, srv, "/oauth2/token", exchangeForm(code), []string{clientID, clientSecret})
	if status != http.StatusOK {
		t.Fatalf("exchange = %d, want 200 (%v)", status, body)
	}
	access, _ := body["access_token"].(string)
	if access == "" {
		t.Fatal("the exchange returned no access_token")
	}

	status, who := whoami(t, srv, "Bearer "+access)
	if status != http.StatusOK || who.Scheme != "oauth2" || who.Grant != "authorization_code" {
		t.Fatalf("whoami = %d %+v, want 200 oauth2/authorization_code", status, who)
	}

	status, body = postToken(t, srv, "/oauth2/token", exchangeForm(code), []string{clientID, clientSecret})
	if status != http.StatusBadRequest || body["error"] != "invalid_grant" {
		t.Fatalf("replay = %d %v, want 400 invalid_grant", status, body)
	}
}

func TestAuthorizeCarriesTheScopeIntoTheToken(t *testing.T) {
	srv := newTestServer(t)

	q := authorizeQuery()
	q.Set("scope", "read write")
	_, target, _ := authorize(t, srv, q)

	_, body := postToken(t, srv, "/oauth2/token", exchangeForm(target.Query().Get("code")),
		[]string{clientID, clientSecret})
	if body["scope"] != "read write" {
		t.Fatalf("scope = %v, want the one the code was issued for", body["scope"])
	}
}

func TestAuthorizeRequiresEachParameterExactlyOnce(t *testing.T) {
	srv := newTestServer(t)

	for _, field := range []string{"response_type", "client_id", "redirect_uri", "state", "code_challenge", "code_challenge_method"} {
		t.Run("missing "+field, func(t *testing.T) {
			q := authorizeQuery()
			q.Del(field)
			status, _, body := authorize(t, srv, q)
			if status != http.StatusBadRequest || body["field"] != field {
				t.Fatalf("status = %d field = %q, want 400 %s", status, body["field"], field)
			}
		})
		t.Run("duplicated "+field, func(t *testing.T) {
			q := authorizeQuery()
			q.Add(field, q.Get(field))
			status, _, body := authorize(t, srv, q)
			if status != http.StatusBadRequest || body["field"] != field {
				t.Fatalf("status = %d field = %q, want 400 %s", status, body["field"], field)
			}
		})
	}

	for _, field := range []string{"scope", "audience", "deny"} {
		t.Run("duplicated "+field, func(t *testing.T) {
			q := authorizeQuery()
			q.Add(field, "one")
			q.Add(field, "two")
			status, _, body := authorize(t, srv, q)
			if status != http.StatusBadRequest || body["field"] != field {
				t.Fatalf("status = %d field = %q, want 400 %s", status, body["field"], field)
			}
		})
	}

	t.Run("empty state", func(t *testing.T) {
		q := authorizeQuery()
		q.Set("state", "  ")
		status, _, body := authorize(t, srv, q)
		if status != http.StatusBadRequest || body["field"] != "state" {
			t.Fatalf("status = %d field = %q, want 400 state", status, body["field"])
		}
	})
}

func TestAuthorizeRejectsAMalformedParameter(t *testing.T) {
	srv := newTestServer(t)

	cases := []struct {
		name  string
		key   string
		value string
	}{
		{name: "implicit flow", key: "response_type", value: "token"},
		{name: "plain PKCE", key: "code_challenge_method", value: "plain"},
		{name: "challenge that is not a digest", key: "code_challenge", value: "not-a-digest"},
		{name: "unknown client", key: "client_id", value: "someone-else"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			q := authorizeQuery()
			q.Set(tc.key, tc.value)
			status, _, body := authorize(t, srv, q)
			if status != http.StatusBadRequest || body["field"] != tc.key {
				t.Fatalf("status = %d field = %q, want 400 %s", status, body["field"], tc.key)
			}
		})
	}
}

func TestAuthorizeAcceptsOnlyALoopbackCallback(t *testing.T) {
	srv := newTestServer(t)

	for _, redirect := range []string{
		"https://example.com/callback",
		"http://example.com/callback",
		"http://127.0.0.1:21830/other",
		"http://127.0.0.1/callback",
		"http://127.0.0.1:21830/callback?next=/admin",
		"http://localhost:21830/callback#fragment",
	} {
		t.Run(redirect, func(t *testing.T) {
			q := authorizeQuery()
			q.Set("redirect_uri", redirect)
			status, _, body := authorize(t, srv, q)
			if status != http.StatusBadRequest || body["field"] != "redirect_uri" {
				t.Fatalf("status = %d field = %q, want 400 redirect_uri", status, body["field"])
			}
		})
	}

	q := authorizeQuery()
	q.Set("redirect_uri", "http://localhost:21830/callback")
	if status, _, body := authorize(t, srv, q); status != http.StatusOK {
		t.Fatalf("localhost callback = %d %v, want 200", status, body)
	}
}

func TestAuthorizeDenyRedirectsWithAccessDenied(t *testing.T) {
	srv := newTestServer(t)

	q := authorizeQuery()
	q.Set("deny", "1")
	status, target, _ := authorize(t, srv, q)
	if status != http.StatusOK {
		t.Fatalf("status = %d, want 200", status)
	}
	if target.Query().Get("error") != "access_denied" || target.Query().Get("code") != "" {
		t.Fatalf("redirect query = %v, want access_denied and no code", target.RawQuery)
	}
	if target.Query().Get("state") != "state-value" {
		t.Fatal("the denial dropped the state")
	}
}

func TestAuthorizeTakesGETOnly(t *testing.T) {
	srv := newTestServer(t)

	status, body := postToken(t, srv, "/oauth2/authorize", url.Values{}, nil)
	if status != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d %v, want 405", status, body)
	}
}

func TestExchangeRejectsAWrongVerifierOrRedirect(t *testing.T) {
	srv := newTestServer(t)

	_, target, _ := authorize(t, srv, authorizeQuery())
	form := exchangeForm(target.Query().Get("code"))
	form.Set("code_verifier", "some-other-verifier-000000000000000000000000")
	status, body := postToken(t, srv, "/oauth2/token", form, []string{clientID, clientSecret})
	if status != http.StatusBadRequest || body["error"] != "invalid_grant" {
		t.Fatalf("wrong verifier = %d %v, want 400 invalid_grant", status, body)
	}

	_, target, _ = authorize(t, srv, authorizeQuery())
	form = exchangeForm(target.Query().Get("code"))
	form.Set("redirect_uri", "http://127.0.0.1:21831/callback")
	status, body = postToken(t, srv, "/oauth2/token", form, []string{clientID, clientSecret})
	if status != http.StatusBadRequest || body["error"] != "invalid_grant" {
		t.Fatalf("mismatched redirect = %d %v, want 400 invalid_grant", status, body)
	}

	status, body = postToken(t, srv, "/oauth2/token",
		url.Values{"grant_type": {"authorization_code"}, "redirect_uri": {testRedirectURI}, "code_verifier": {testVerifier}},
		[]string{clientID, clientSecret})
	if status != http.StatusBadRequest || body["error"] != "invalid_request" {
		t.Fatalf("missing code = %d %v, want 400 invalid_request", status, body)
	}

	form = exchangeForm("code-that-was-never-issued")
	status, body = postToken(t, srv, "/oauth2/token", form, []string{clientID, clientSecret})
	if status != http.StatusBadRequest || body["error"] != "invalid_grant" {
		t.Fatalf("unknown code = %d %v, want 400 invalid_grant", status, body)
	}
}

func deviceForm() url.Values {
	return url.Values{"client_id": {clientID}}
}

func pollForm(deviceCode string) url.Values {
	return url.Values{
		"grant_type":  {deviceCodeGrant},
		"device_code": {deviceCode},
		"client_id":   {clientID},
	}
}

func visit(t *testing.T, srv *httptest.Server, target string) int {
	t.Helper()
	resp, err := srv.Client().Get(target)
	if err != nil {
		t.Fatalf("visit %s: %v", target, err)
	}
	defer func() { _ = resp.Body.Close() }()
	_, _ = io.Copy(io.Discard, resp.Body)

	return resp.StatusCode
}

func TestDeviceRoundTripPendingThenApproved(t *testing.T) {
	srv := newTestServer(t)

	status, body := postToken(t, srv, "/oauth2/device", deviceForm(), []string{clientID, clientSecret})
	if status != http.StatusOK {
		t.Fatalf("device authorization = %d %v, want 200", status, body)
	}
	deviceCode, _ := body["device_code"].(string)
	userCode, _ := body["user_code"].(string)
	complete, _ := body["verification_uri_complete"].(string)
	if deviceCode == "" || userCode == "" {
		t.Fatalf("device authorization carried no codes: %v", body)
	}
	if body["verification_uri"] != srv.URL+"/device" {
		t.Fatalf("verification_uri = %v, want %s/device", body["verification_uri"], srv.URL)
	}
	if !strings.Contains(complete, url.QueryEscape(userCode)) {
		t.Fatalf("verification_uri_complete = %q, want the user code in it", complete)
	}
	if body["interval"] != float64(defaultDeviceInterval) || body["expires_in"] != float64(defaultDeviceExpires) {
		t.Fatalf("interval/expires_in = %v/%v, want %d/%d", body["interval"], body["expires_in"],
			defaultDeviceInterval, defaultDeviceExpires)
	}

	status, body = postToken(t, srv, "/oauth2/token", pollForm(deviceCode), []string{clientID, clientSecret})
	if status != http.StatusBadRequest || body["error"] != "authorization_pending" {
		t.Fatalf("first poll = %d %v, want 400 authorization_pending", status, body)
	}

	if code := visit(t, srv, complete); code != http.StatusOK {
		t.Fatalf("verification page = %d, want 200", code)
	}

	status, body = postToken(t, srv, "/oauth2/token", pollForm(deviceCode), []string{clientID, clientSecret})
	if status != http.StatusOK {
		t.Fatalf("poll after approval = %d %v, want 200", status, body)
	}
	access, _ := body["access_token"].(string)
	status, who := whoami(t, srv, "Bearer "+access)
	if status != http.StatusOK || who.Grant != deviceCodeGrant {
		t.Fatalf("whoami = %d %+v, want 200 with the device grant", status, who)
	}
}

func TestDeviceVerificationPageRejectsAnUnknownUserCode(t *testing.T) {
	srv := newTestServer(t)

	if code := visit(t, srv, srv.URL+"/device?user_code=ZZZZ-ZZZZ"); code != http.StatusNotFound {
		t.Fatalf("unknown user_code = %d, want 404", code)
	}
	if code := visit(t, srv, srv.URL+"/device"); code != http.StatusNotFound {
		t.Fatalf("absent user_code = %d, want 404", code)
	}
}

func TestDeviceDenialEndsThePoll(t *testing.T) {
	srv := newTestServer(t)

	_, body := postToken(t, srv, "/oauth2/device", deviceForm(), []string{clientID, clientSecret})
	deviceCode, _ := body["device_code"].(string)
	complete, _ := body["verification_uri_complete"].(string)

	if code := visit(t, srv, complete+"&deny=1"); code != http.StatusOK {
		t.Fatalf("denial page = %d, want 200", code)
	}

	status, body := postToken(t, srv, "/oauth2/token", pollForm(deviceCode), []string{clientID, clientSecret})
	if status != http.StatusBadRequest || body["error"] != "access_denied" {
		t.Fatalf("poll after denial = %d %v, want 400 access_denied", status, body)
	}
}

func TestDeviceSlowDownIsAnsweredOnce(t *testing.T) {
	srv := newTestServer(t)

	_, body := postToken(t, srv, "/oauth2/device", deviceForm(), []string{clientID, clientSecret})
	deviceCode, _ := body["device_code"].(string)

	status, body := postToken(t, srv, "/oauth2/token?slow=1", pollForm(deviceCode), []string{clientID, clientSecret})
	if status != http.StatusBadRequest || body["error"] != "slow_down" {
		t.Fatalf("first poll = %d %v, want 400 slow_down", status, body)
	}
	status, body = postToken(t, srv, "/oauth2/token?slow=1", pollForm(deviceCode), []string{clientID, clientSecret})
	if status != http.StatusBadRequest || body["error"] != "authorization_pending" {
		t.Fatalf("second poll = %d %v, want 400 authorization_pending", status, body)
	}
}

func TestDeviceCodeExpires(t *testing.T) {
	clock := &testClock{at: time.Date(2026, 9, 5, 12, 0, 0, 0, time.UTC)}
	srv := newTestServerFor(t, serverOptions{Now: clock.now})

	status, body := postToken(t, srv, "/oauth2/device?expires=1", deviceForm(), []string{clientID, clientSecret})
	if status != http.StatusOK || body["expires_in"] != float64(1) {
		t.Fatalf("device authorization = %d %v, want 200 with expires_in 1", status, body)
	}
	deviceCode, _ := body["device_code"].(string)
	complete, _ := body["verification_uri_complete"].(string)
	visit(t, srv, complete)

	clock.advance(2 * time.Second)
	status, body = postToken(t, srv, "/oauth2/token", pollForm(deviceCode), []string{clientID, clientSecret})
	if status != http.StatusBadRequest || body["error"] != "expired_token" {
		t.Fatalf("poll after expiry = %d %v, want 400 expired_token", status, body)
	}
}

func TestDeviceAuthorizationAdvertisesTheRequestedInterval(t *testing.T) {
	srv := newTestServer(t)

	status, body := postToken(t, srv, "/oauth2/device?interval=0", deviceForm(), []string{clientID, clientSecret})
	if status != http.StatusOK || body["interval"] != float64(0) {
		t.Fatalf("device authorization = %d %v, want 200 with interval 0", status, body)
	}

	status, body = postToken(t, srv, "/oauth2/device?interval=soon", deviceForm(), []string{clientID, clientSecret})
	if status != http.StatusBadRequest || body["field"] != "interval" {
		t.Fatalf("non-numeric interval = %d %v, want 400 naming interval", status, body)
	}
}

func TestDeviceAuthorizationRequiresClientIDExactlyOnce(t *testing.T) {
	srv := newTestServer(t)

	status, body := postToken(t, srv, "/oauth2/device", url.Values{}, []string{clientID, clientSecret})
	if status != http.StatusBadRequest || body["field"] != "client_id" {
		t.Fatalf("missing client_id = %d %v, want 400 naming client_id", status, body)
	}

	status, body = postToken(t, srv, "/oauth2/device",
		url.Values{"client_id": {clientID, clientID}}, []string{clientID, clientSecret})
	if status != http.StatusBadRequest || body["field"] != "client_id" {
		t.Fatalf("duplicated client_id = %d %v, want 400 naming client_id", status, body)
	}

	status, body = postToken(t, srv, "/oauth2/device",
		url.Values{"client_id": {clientID}, "scope": {"a", "b"}}, []string{clientID, clientSecret})
	if status != http.StatusBadRequest || body["field"] != "scope" {
		t.Fatalf("duplicated scope = %d %v, want 400 naming scope", status, body)
	}
}

func TestDevicePollRejectsAMissingDeviceCode(t *testing.T) {
	srv := newTestServer(t)

	status, body := postToken(t, srv, "/oauth2/token",
		url.Values{"grant_type": {deviceCodeGrant}, "client_id": {clientID}}, []string{clientID, clientSecret})
	if status != http.StatusBadRequest || body["error"] != "invalid_request" {
		t.Fatalf("missing device_code = %d %v, want 400 invalid_request", status, body)
	}

	status, body = postToken(t, srv, "/oauth2/token", pollForm("device-nobody-issued"), []string{clientID, clientSecret})
	if status != http.StatusBadRequest || body["error"] != "invalid_grant" {
		t.Fatalf("unknown device_code = %d %v, want 400 invalid_grant", status, body)
	}
}

func TestClientAuthModes(t *testing.T) {
	basicHeader := []string{clientID, clientSecret}
	bodyForm := url.Values{"grant_type": {"client_credentials"}, "client_id": {clientID}, "client_secret": {clientSecret}}
	bareForm := url.Values{"grant_type": {"client_credentials"}, "client_id": {clientID}}
	basicForm := url.Values{"grant_type": {"client_credentials"}}

	cases := []struct {
		mode  string
		name  string
		form  url.Values
		basic []string
		want  int
	}{
		{mode: clientAuthBasic, name: "header accepted", form: basicForm, basic: basicHeader, want: http.StatusOK},
		{mode: clientAuthBasic, name: "body refused", form: bodyForm, want: http.StatusUnauthorized},
		{mode: clientAuthBasic, name: "secret in the body refused", form: bodyForm, basic: basicHeader, want: http.StatusUnauthorized},
		{mode: clientAuthBody, name: "body accepted", form: bodyForm, want: http.StatusOK},
		{mode: clientAuthBody, name: "header refused", form: basicForm, basic: basicHeader, want: http.StatusUnauthorized},
		{mode: clientAuthBody, name: "missing secret refused", form: bareForm, want: http.StatusUnauthorized},
		{mode: clientAuthNone, name: "bare client_id accepted", form: bareForm, want: http.StatusOK},
		{mode: clientAuthNone, name: "header refused", form: basicForm, basic: basicHeader, want: http.StatusUnauthorized},
		{mode: clientAuthNone, name: "secret refused", form: bodyForm, want: http.StatusUnauthorized},
		{mode: clientAuthAny, name: "header accepted", form: basicForm, basic: basicHeader, want: http.StatusOK},
		{mode: clientAuthAny, name: "body accepted", form: bodyForm, want: http.StatusOK},
		{mode: clientAuthAny, name: "bare client_id accepted", form: bareForm, want: http.StatusOK},
	}
	for _, tc := range cases {
		t.Run(tc.mode+" "+tc.name, func(t *testing.T) {
			srv := newTestServerFor(t, serverOptions{ClientAuth: tc.mode})
			status, body := postToken(t, srv, "/oauth2/token", tc.form, tc.basic)
			if status != tc.want {
				t.Fatalf("status = %d %v, want %d", status, body, tc.want)
			}
		})
	}
}

func TestDeviceEndpointEnforcesTheClientAuthMode(t *testing.T) {
	srv := newTestServerFor(t, serverOptions{ClientAuth: clientAuthNone})

	status, body := postToken(t, srv, "/oauth2/device", deviceForm(), nil)
	if status != http.StatusOK {
		t.Fatalf("public client = %d %v, want 200", status, body)
	}

	status, body = postToken(t, srv, "/oauth2/device", deviceForm(), []string{clientID, clientSecret})
	if status != http.StatusUnauthorized {
		t.Fatalf("Basic credentials = %d %v, want 401", status, body)
	}
}
