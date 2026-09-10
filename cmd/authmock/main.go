// Command authmock is a local IdP: OAuth 2.0 token, authorize and device endpoints plus
// /api/whoami behind bearer, JWT, Digest or SigV4. Every credential here is a fixture.
// Run: `go run ./cmd/authmock [-client-auth basic|body|none]` → http://127.0.0.1:9877
package main

import (
	"crypto/hmac"
	"crypto/md5"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"hash"
	"html"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const (
	defaultAddr = "127.0.0.1:9877"

	// OAuth 2.0 client fixture.
	clientID     = "test-client"
	clientSecret = "test-secret"
	// Resource-owner password grant fixture.
	passwordUser = "demo"
	passwordPass = "demo-password"
	// Shared secret for the JWT scheme (HS256/384/512).
	jwtSecret = "authmock-hs256-secret"
	// Digest fixture.
	digestUser  = "digest-user"
	digestPass  = "digest-password"
	digestRealm = "authmock"
	// AWS published test-suite credentials.
	awsAccessKeyID     = "AKIDEXAMPLE"
	awsSecretAccessKey = "wJalrXUtnFEMI/K7MDENG+bPxRfiCYEXAMPLEKEY"

	// Client-authentication modes the token and device endpoints enforce; they
	// mirror auth.ClientAuth* on the client side.
	clientAuthAny   = "any"
	clientAuthBasic = "basic"
	clientAuthBody  = "body"
	clientAuthNone  = "none"

	// deviceCodeGrant is the grant_type of an RFC 8628 poll.
	deviceCodeGrant = "urn:ietf:params:oauth:grant-type:device_code"
	// What a device authorization advertises unless ?expires= / ?interval= say
	// otherwise.
	defaultDeviceExpires  = 300
	defaultDeviceInterval = 5

	sessionCookie   = "authmock_session"
	defaultExpires  = 3600
	maxRequestBody  = 1 << 20
	sigv4Algorithm  = "AWS4-HMAC-SHA256"
	unsignedPayload = "UNSIGNED-PAYLOAD"
)

// issuedToken is one access token handed out by the token endpoint.
type issuedToken struct {
	ID        string
	Grant     string
	Scope     string
	ExpiresAt time.Time
}

// authCode is one authorization code handed out by /oauth2/authorize.
type authCode struct {
	Challenge   string
	RedirectURI string
	Scope       string
}

// deviceGrant is one device authorization; the verification page is what flips
// it to approved or denied.
type deviceGrant struct {
	Scope      string
	ExpiresAt  time.Time
	Approved   bool
	Denied     bool
	SlowedDown bool
}

type server struct {
	clientAuth string

	mu        sync.Mutex
	seq       int
	tokens    map[string]issuedToken  // access token → what minted it
	refresh   map[string]string       // refresh token → grant it came from
	sessions  map[string]string       // session id → digest nonce
	codes     map[string]*authCode    // authorization code → what it was issued for
	devices   map[string]*deviceGrant // device code → its approval state
	userCodes map[string]string       // user code → device code
	now       func() time.Time
}

// serverOptions is what a test or the command line varies; the zero value is the
// permissive server every stage-A test runs against.
type serverOptions struct {
	ClientAuth string
	Now        func() time.Time
}

func newServer() *server {
	return newServerWithOptions(serverOptions{})
}

func newServerWithOptions(opts serverOptions) *server {
	if opts.ClientAuth == "" {
		opts.ClientAuth = clientAuthAny
	}
	if opts.Now == nil {
		opts.Now = time.Now
	}

	return &server{
		clientAuth: opts.ClientAuth,
		tokens:     map[string]issuedToken{},
		refresh:    map[string]string{},
		sessions:   map[string]string{},
		codes:      map[string]*authCode{},
		devices:    map[string]*deviceGrant{},
		userCodes:  map[string]string{},
		now:        opts.Now,
	}
}

func (s *server) routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/oauth2/token", s.handleToken)
	mux.HandleFunc("/oauth2/authorize", s.handleAuthorize)
	mux.HandleFunc("/oauth2/device", s.handleDeviceAuth)
	mux.HandleFunc("/device", s.handleVerification)
	mux.HandleFunc("/api/whoami", s.handleWhoami)

	return mux
}

func main() {
	addr := flag.String("addr", defaultAddr, "listen address")
	clientAuth := flag.String("client-auth", clientAuthAny,
		"client authentication the token and device endpoints require: any, basic, body or none")
	flag.Parse()

	switch *clientAuth {
	case clientAuthAny, clientAuthBasic, clientAuthBody, clientAuthNone:
	default:
		log.Fatalf("unknown -client-auth %q: use any, basic, body or none", *clientAuth)
	}

	srv := &http.Server{
		Addr:              *addr,
		Handler:           newServerWithOptions(serverOptions{ClientAuth: *clientAuth}).routes(),
		ReadHeaderTimeout: 10 * time.Second,
	}
	log.Printf("authmock listening on http://%s (token: /oauth2/token, authorize: /oauth2/authorize, "+
		"device: /oauth2/device, verification: /device, resource: /api/whoami; client-auth: %s)", *addr, *clientAuth)
	if err := srv.ListenAndServe(); err != nil {
		log.Fatal(err)
	}
}

func (s *server) handleToken(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		oauthError(w, http.StatusMethodNotAllowed, "invalid_request", "the token endpoint takes POST")
		return
	}
	if err := r.ParseForm(); err != nil {
		oauthError(w, http.StatusBadRequest, "invalid_request", "malformed form body")
		return
	}
	if err := s.checkClient(r); err != nil {
		w.Header().Set("WWW-Authenticate", `Basic realm="authmock"`)
		oauthError(w, http.StatusUnauthorized, "invalid_client", err.Error())
		return
	}

	grant := r.PostFormValue("grant_type")
	// The interactive grants carry the scope from what they were issued for, so
	// the client cannot widen it at the exchange.
	scope := r.PostFormValue("scope")
	switch grant {
	case "client_credentials":
	case "password":
		if r.PostFormValue("username") != passwordUser || r.PostFormValue("password") != passwordPass {
			oauthError(w, http.StatusBadRequest, "invalid_grant", "unknown resource owner credentials")
			return
		}
	case "refresh_token":
		s.mu.Lock()
		_, known := s.refresh[r.PostFormValue("refresh_token")]
		s.mu.Unlock()
		if !known {
			oauthError(w, http.StatusBadRequest, "invalid_grant", "unknown refresh token")
			return
		}
	case "authorization_code":
		redeemed, failure := s.redeemCode(r)
		if failure != nil {
			oauthError(w, http.StatusBadRequest, failure.Code, failure.Description)
			return
		}
		scope = redeemed
	case deviceCodeGrant:
		redeemed, failure := s.pollDevice(r)
		if failure != nil {
			oauthError(w, http.StatusBadRequest, failure.Code, failure.Description)
			return
		}
		scope = redeemed
	default:
		oauthError(w, http.StatusBadRequest, "unsupported_grant_type", "grant "+grant+" is not implemented")
		return
	}

	expiresIn := defaultExpires
	if raw := r.URL.Query().Get("expires_in"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed <= 0 {
			oauthError(w, http.StatusBadRequest, "invalid_request", "expires_in must be a positive number of seconds")
			return
		}
		expiresIn = parsed
	}

	body := s.issue(grant, scope, expiresIn)
	log.Printf("token: grant=%s expires_in=%d", grant, expiresIn)
	writeJSON(w, http.StatusOK, body)
}

// issue mints an access token; a refresh grant deliberately returns no new
// refresh token, so the client has to keep the one it already holds.
func (s *server) issue(grant, scope string, expiresIn int) map[string]any {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.seq++
	id := fmt.Sprintf("token-%d", s.seq)
	access := id + "." + randomHex(16)
	s.tokens[access] = issuedToken{
		ID:        id,
		Grant:     grant,
		Scope:     scope,
		ExpiresAt: s.now().Add(time.Duration(expiresIn) * time.Second),
	}

	body := map[string]any{
		"access_token": access,
		"token_type":   "Bearer",
		"expires_in":   expiresIn,
	}
	if scope != "" {
		body["scope"] = scope
	}
	if grant != "refresh_token" {
		refresh := "refresh-" + randomHex(16)
		s.refresh[refresh] = grant
		body["refresh_token"] = refresh
	}

	return body
}

// "any" accepts the Basic header (RFC 6749 §2.3.1 form-urlencodes both halves), the
// body, or a bare client_id; the three strict modes each refuse what the others take.
func (s *server) checkClient(r *http.Request) error {
	basicID, basicSecret, hasBasic := r.BasicAuth()
	if hasBasic {
		basicID, basicSecret = unescapeCredential(basicID), unescapeCredential(basicSecret)
	}
	bodyID, bodySecret := r.PostFormValue("client_id"), r.PostFormValue("client_secret")

	switch s.clientAuth {
	case clientAuthBasic:
		if !hasBasic {
			return errors.New("this client must authenticate with the Basic header")
		}
		if bodySecret != "" {
			return errors.New("client_secret must not travel in the body")
		}
		if basicID != clientID || basicSecret != clientSecret {
			return errors.New("wrong Basic credentials")
		}
	case clientAuthBody:
		if hasBasic {
			return errors.New("this client must authenticate in the body")
		}
		if bodyID != clientID {
			return errors.New("unknown client_id")
		}
		if bodySecret != clientSecret {
			return errors.New("wrong client_secret")
		}
	case clientAuthNone:
		// A public client has no secret at all — asking for one would defeat the
		// point of the mode.
		if hasBasic {
			return errors.New("a public client must not send Basic credentials")
		}
		if bodySecret != "" {
			return errors.New("a public client must not send a client_secret")
		}
		if bodyID != clientID {
			return errors.New("unknown client_id")
		}
	default:
		id, secret := bodyID, bodySecret
		if hasBasic {
			id, secret = basicID, basicSecret
		}
		if id != clientID {
			return errors.New("unknown client_id")
		}
		if secret != "" && secret != clientSecret {
			return errors.New("wrong client_secret")
		}
	}

	return nil
}

func unescapeCredential(v string) string {
	unescaped, err := url.QueryUnescape(v)
	if err != nil {
		return v
	}

	return unescaped
}

// grantError is the RFC 6749 §5.2 body a failed grant answers with.
type grantError struct {
	Code        string
	Description string
}

// handleAuthorize validates the whole parameter set before minting anything, then answers
// a page that returns the browser to the loopback listener; ?deny=1 sends access_denied.
func (s *server) handleAuthorize(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		fieldError(w, http.StatusMethodNotAllowed, "invalid_request", "", "the authorize endpoint takes GET")
		return
	}
	q, err := url.ParseQuery(r.URL.RawQuery)
	if err != nil {
		fieldError(w, http.StatusBadRequest, "invalid_request", "", "malformed query string")
		return
	}
	params, field, err := requiredParams(q, "response_type", "client_id", "redirect_uri",
		"state", "code_challenge", "code_challenge_method")
	if err != nil {
		fieldError(w, http.StatusBadRequest, "invalid_request", field, err.Error())
		return
	}
	if field, err = atMostOnce(q, "scope", "audience", "deny"); err != nil {
		fieldError(w, http.StatusBadRequest, "invalid_request", field, err.Error())
		return
	}

	switch {
	case params["response_type"] != "code":
		fieldError(w, http.StatusBadRequest, "unsupported_response_type", "response_type",
			"only response_type=code is implemented")
		return
	case params["code_challenge_method"] != "S256":
		fieldError(w, http.StatusBadRequest, "invalid_request", "code_challenge_method", "only S256 is accepted")
		return
	case !validChallenge(params["code_challenge"]):
		fieldError(w, http.StatusBadRequest, "invalid_request", "code_challenge",
			"code_challenge must be a base64url-encoded SHA-256 digest")
		return
	case params["client_id"] != clientID:
		fieldError(w, http.StatusBadRequest, "invalid_client", "client_id", "unknown client_id")
		return
	}

	redirect, err := loopbackRedirect(params["redirect_uri"])
	if err != nil {
		fieldError(w, http.StatusBadRequest, "invalid_request", "redirect_uri", err.Error())
		return
	}

	answer := url.Values{"state": {params["state"]}}
	verdict := "approved"
	if q.Get("deny") != "" {
		verdict = "denied"
		answer.Set("error", "access_denied")
		answer.Set("error_description", "the user denied the request")
	} else {
		answer.Set("code", s.mintCode(params["code_challenge"], params["redirect_uri"], q.Get("scope")))
	}
	redirect.RawQuery = answer.Encode()

	log.Printf("authorize: %s", verdict)
	writeHTML(w, http.StatusOK, redirectPage(redirect.String()))
}

// loopbackRedirect accepts only the loopback callback a native client is allowed
// to listen on (RFC 8252 §7.3), and no query of its own — the answer owns it.
func loopbackRedirect(raw string) (*url.URL, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return nil, errors.New("redirect_uri is not a URL")
	}
	if u.Scheme != "http" {
		return nil, errors.New("redirect_uri must be http on the loopback interface")
	}
	host, port, err := net.SplitHostPort(u.Host)
	if err != nil || port == "" {
		return nil, errors.New("redirect_uri must carry an explicit port")
	}
	if host != "127.0.0.1" && host != "localhost" && host != "::1" {
		return nil, errors.New("redirect_uri must point at the loopback interface")
	}
	if u.Path != "/callback" {
		return nil, errors.New("redirect_uri path must be /callback")
	}
	if u.RawQuery != "" || u.Fragment != "" {
		return nil, errors.New("redirect_uri must carry no query or fragment")
	}

	return u, nil
}

func (s *server) mintCode(challenge, redirectURI, scope string) string {
	code := "code-" + randomHex(16)

	s.mu.Lock()
	defer s.mu.Unlock()
	s.codes[code] = &authCode{Challenge: challenge, RedirectURI: redirectURI, Scope: scope}

	return code
}

// redeemCode consumes an authorization code: it is revoked the moment it is presented,
// so neither a replay nor a retry with a corrected RFC 7636 verifier works.
func (s *server) redeemCode(r *http.Request) (string, *grantError) {
	code, verifier := r.PostFormValue("code"), r.PostFormValue("code_verifier")
	redirect := r.PostFormValue("redirect_uri")
	switch {
	case code == "":
		return "", &grantError{Code: "invalid_request", Description: "code is required"}
	case verifier == "":
		return "", &grantError{Code: "invalid_request", Description: "code_verifier is required"}
	case redirect == "":
		return "", &grantError{Code: "invalid_request", Description: "redirect_uri is required"}
	}

	s.mu.Lock()
	stored, known := s.codes[code]
	delete(s.codes, code)
	s.mu.Unlock()

	switch {
	case !known:
		return "", &grantError{Code: "invalid_grant", Description: "unknown or already redeemed authorization code"}
	case stored.RedirectURI != redirect:
		return "", &grantError{Code: "invalid_grant", Description: "redirect_uri differs from the one the code was issued for"}
	case subtle.ConstantTimeCompare([]byte(pkceChallenge(verifier)), []byte(stored.Challenge)) != 1:
		return "", &grantError{Code: "invalid_grant", Description: "code_verifier does not match the code_challenge"}
	}

	return stored.Scope, nil
}

func pkceChallenge(verifier string) string {
	sum := sha256.Sum256([]byte(verifier))

	return base64.RawURLEncoding.EncodeToString(sum[:])
}

func validChallenge(v string) bool {
	sum, err := base64.RawURLEncoding.DecodeString(v)

	return err == nil && len(sum) == sha256.Size
}

// handleDeviceAuth answers the device authorization request; ?expires= and ?interval=
// override what is advertised, so a client can be shown values it must refuse.
func (s *server) handleDeviceAuth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		fieldError(w, http.StatusMethodNotAllowed, "invalid_request", "", "the device endpoint takes POST")
		return
	}
	if err := r.ParseForm(); err != nil {
		fieldError(w, http.StatusBadRequest, "invalid_request", "", "malformed form body")
		return
	}
	if err := s.checkClient(r); err != nil {
		w.Header().Set("WWW-Authenticate", `Basic realm="authmock"`)
		fieldError(w, http.StatusUnauthorized, "invalid_client", "", err.Error())
		return
	}
	params, field, err := requiredParams(r.PostForm, "client_id")
	if err != nil {
		fieldError(w, http.StatusBadRequest, "invalid_request", field, err.Error())
		return
	}
	if field, err = atMostOnce(r.PostForm, "scope", "audience"); err != nil {
		fieldError(w, http.StatusBadRequest, "invalid_request", field, err.Error())
		return
	}
	if params["client_id"] != clientID {
		fieldError(w, http.StatusBadRequest, "invalid_client", "client_id", "unknown client_id")
		return
	}

	query := r.URL.Query()
	expiresIn, ok := intQuery(query, "expires", defaultDeviceExpires)
	if !ok {
		fieldError(w, http.StatusBadRequest, "invalid_request", "expires", "expires must be a number of seconds")
		return
	}
	interval, ok := intQuery(query, "interval", defaultDeviceInterval)
	if !ok {
		fieldError(w, http.StatusBadRequest, "invalid_request", "interval", "interval must be a number of seconds")
		return
	}

	deviceCode, userCode := s.mintDevice(r.PostFormValue("scope"), expiresIn)
	verification := "http://" + r.Host + "/device"
	log.Printf("device: authorization requested (expires_in=%d interval=%d)", expiresIn, interval)
	writeJSON(w, http.StatusOK, map[string]any{
		"device_code":               deviceCode,
		"user_code":                 userCode,
		"verification_uri":          verification,
		"verification_uri_complete": verification + "?user_code=" + url.QueryEscape(userCode),
		"interval":                  interval,
		"expires_in":                expiresIn,
	})
}

func (s *server) mintDevice(scope string, expiresIn int) (string, string) {
	deviceCode, userCode := "device-"+randomHex(16), randomUserCode()

	s.mu.Lock()
	defer s.mu.Unlock()
	s.devices[deviceCode] = &deviceGrant{
		Scope:     scope,
		ExpiresAt: s.now().Add(time.Duration(expiresIn) * time.Second),
	}
	s.userCodes[userCode] = deviceCode

	return deviceCode, userCode
}

// pollDevice answers one poll of the token endpoint for a device code.
func (s *server) pollDevice(r *http.Request) (string, *grantError) {
	deviceCode := r.PostFormValue("device_code")
	if deviceCode == "" {
		return "", &grantError{Code: "invalid_request", Description: "device_code is required"}
	}
	if r.PostFormValue("client_id") != clientID {
		return "", &grantError{Code: "invalid_request", Description: "the poll must carry client_id"}
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	grant, known := s.devices[deviceCode]
	switch {
	case !known:
		return "", &grantError{Code: "invalid_grant", Description: "unknown device_code"}
	case s.now().After(grant.ExpiresAt):
		return "", &grantError{Code: "expired_token", Description: "the device code expired"}
	case grant.Denied:
		return "", &grantError{Code: "access_denied", Description: "the user denied the request"}
	case r.URL.Query().Get("slow") != "" && !grant.SlowedDown:
		grant.SlowedDown = true

		return "", &grantError{Code: "slow_down", Description: "poll less often"}
	case !grant.Approved:
		return "", &grantError{Code: "authorization_pending", Description: "the user has not approved the request yet"}
	}

	return grant.Scope, nil
}

// handleVerification is the page the user opens on the second device: a known
// user_code approves the grant it belongs to, ?deny=1 denies it.
func (s *server) handleVerification(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		fieldError(w, http.StatusMethodNotAllowed, "invalid_request", "", "the verification page takes GET")
		return
	}
	userCode := strings.ToUpper(strings.TrimSpace(r.URL.Query().Get("user_code")))
	deny := r.URL.Query().Get("deny") != ""

	s.mu.Lock()
	grant := s.devices[s.userCodes[userCode]]
	if grant != nil {
		grant.Approved, grant.Denied = !deny, deny
	}
	s.mu.Unlock()

	if userCode == "" || grant == nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "unknown user_code"})
		return
	}

	verdict := "approved"
	if deny {
		verdict = "denied"
	}
	log.Printf("device: %s", verdict)
	writeHTML(w, http.StatusOK, verificationPage(verdict))
}

// randomUserCode uses the RFC 8628 §6.1 alphabet: no vowels, no look-alikes.
func randomUserCode() string {
	const alphabet = "BCDFGHJKLMNPQRSTVWXZ23456789"

	buf := make([]byte, 8)
	if _, err := rand.Read(buf); err != nil {
		panic(err)
	}
	out := make([]byte, 0, len(buf)+1)
	for i, b := range buf {
		if i == 4 {
			out = append(out, '-')
		}
		out = append(out, alphabet[int(b)%len(alphabet)])
	}

	return string(out)
}

// requiredParams reads each key exactly once and returns the offending key with
// the error, so the answer can name the field the client got wrong.
func requiredParams(values url.Values, keys ...string) (map[string]string, string, error) {
	out := make(map[string]string, len(keys))
	for _, key := range keys {
		found := values[key]
		switch {
		case len(found) == 0:
			return nil, key, errors.New(key + " is required")
		case len(found) > 1:
			return nil, key, fmt.Errorf("%s was given %d times", key, len(found))
		case strings.TrimSpace(found[0]) == "":
			return nil, key, errors.New(key + " is empty")
		}
		out[key] = found[0]
	}

	return out, "", nil
}

func atMostOnce(values url.Values, keys ...string) (string, error) {
	for _, key := range keys {
		if len(values[key]) > 1 {
			return key, fmt.Errorf("%s was given %d times", key, len(values[key]))
		}
	}

	return "", nil
}

func intQuery(values url.Values, key string, fallback int) (int, bool) {
	raw := values.Get(key)
	if raw == "" {
		return fallback, true
	}
	parsed, err := strconv.Atoi(raw)
	if err != nil {
		return 0, false
	}

	return parsed, true
}

type whoamiResponse struct {
	Scheme    string `json:"scheme"`
	TokenID   string `json:"tokenId,omitempty"`
	Grant     string `json:"grant,omitempty"`
	Scope     string `json:"scope,omitempty"`
	Subject   string `json:"subject,omitempty"`
	User      string `json:"user,omitempty"`
	Algorithm string `json:"algorithm,omitempty"`
	Region    string `json:"region,omitempty"`
	Service   string `json:"service,omitempty"`
}

func (s *server) handleWhoami(w http.ResponseWriter, r *http.Request) {
	body, err := readBody(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	authz := r.Header.Get("Authorization")
	scheme, _, _ := strings.Cut(authz, " ")
	var (
		answer *whoamiResponse
		reason error
	)
	switch {
	case strings.EqualFold(scheme, "bearer"):
		answer, reason = s.checkBearer(strings.TrimSpace(authz[len(scheme):]))
	case strings.EqualFold(scheme, "digest"):
		answer, reason = s.checkDigest(r)
	case strings.EqualFold(scheme, sigv4Algorithm):
		answer, reason = verifySigV4(r, body)
	case authz == "":
		if token := queryToken(r); token != "" {
			answer, reason = s.checkBearer(token)
			break
		}
		s.challenge(w, r, "no credentials")
		return
	default:
		reason = fmt.Errorf("unsupported authorization scheme %q", scheme)
	}

	if reason != nil {
		if strings.EqualFold(scheme, "digest") {
			s.challenge(w, r, reason.Error())
			return
		}
		log.Printf("whoami: rejected (%s): %v", r.Method, reason)
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": reason.Error()})
		return
	}

	log.Printf("whoami: %s accepted (%s %s)", answer.Scheme, r.Method, r.URL.Path)
	// A cookie on the success response proves the client persisted the final
	// response's cookies, not only the challenge's.
	http.SetCookie(w, &http.Cookie{Name: "authmock_seen", Value: answer.Scheme, Path: "/"})
	writeJSON(w, http.StatusOK, answer)
}

func queryToken(r *http.Request) string {
	q := r.URL.Query()
	for _, key := range []string{"access_token", "token"} {
		if v := q.Get(key); v != "" {
			return v
		}
	}

	return ""
}

// checkBearer answers for an access token this server issued, otherwise it tries
// to read the value as a JWT signed with the shared secret.
func (s *server) checkBearer(token string) (*whoamiResponse, error) {
	s.mu.Lock()
	issued, ok := s.tokens[token]
	s.mu.Unlock()
	if ok {
		if s.now().After(issued.ExpiresAt) {
			return nil, errors.New("access token expired")
		}
		return &whoamiResponse{Scheme: "oauth2", TokenID: issued.ID, Grant: issued.Grant, Scope: issued.Scope}, nil
	}

	claims := jwt.MapClaims{}
	parsed, err := jwt.ParseWithClaims(token, claims, func(*jwt.Token) (any, error) {
		return []byte(jwtSecret), nil
	}, jwt.WithValidMethods([]string{"HS256", "HS384", "HS512"}))
	if err != nil {
		return nil, fmt.Errorf("neither an issued access token nor a valid JWT: %w", err)
	}

	subject, _ := claims["sub"].(string)

	return &whoamiResponse{Scheme: "jwt", Subject: subject, Algorithm: parsed.Method.Alg()}, nil
}

// challenge answers 401 with the algorithms of ?alg= (both, newest first, by default)
// and binds the nonce to a session cookie: a client that drops cookies cannot answer.
func (s *server) challenge(w http.ResponseWriter, r *http.Request, reason string) {
	session := sessionID(r)
	nonce := randomHex(16)
	s.mu.Lock()
	s.sessions[session] = nonce
	s.mu.Unlock()

	algorithms := []string{"SHA-256", "MD5"}
	switch strings.ToUpper(r.URL.Query().Get("alg")) {
	case "":
	case "MD5":
		algorithms = []string{"MD5"}
	case "SHA-256":
		algorithms = []string{"SHA-256"}
	default:
		// Anything else (MD5-sess, SHA-512-256-sess, …) is offered verbatim so a
		// client can prove it reports an unanswerable challenge.
		algorithms = []string{r.URL.Query().Get("alg")}
	}

	opaque := randomHex(8)
	for _, alg := range algorithms {
		w.Header().Add("WWW-Authenticate", fmt.Sprintf(
			`Digest realm=%q, qop="auth", algorithm=%s, nonce=%q, opaque=%q`, digestRealm, alg, nonce, opaque))
	}
	http.SetCookie(w, &http.Cookie{Name: sessionCookie, Value: session, Path: "/"})
	log.Printf("whoami: digest challenge (%s)", reason)
	writeJSON(w, http.StatusUnauthorized, map[string]string{"error": reason})
}

func sessionID(r *http.Request) string {
	if c, err := r.Cookie(sessionCookie); err == nil && c.Value != "" {
		return c.Value
	}

	return randomHex(12)
}

func (s *server) checkDigest(r *http.Request) (*whoamiResponse, error) {
	params := parseDigestParams(r.Header.Get("Authorization"))
	if params["username"] != digestUser {
		return nil, errors.New("unknown user")
	}
	if params["uri"] != r.URL.RequestURI() {
		return nil, errors.New("uri does not match the request")
	}

	cookie, err := r.Cookie(sessionCookie)
	if err != nil {
		return nil, errors.New("session cookie missing — the challenge cookie was not sent back")
	}
	s.mu.Lock()
	nonce, known := s.sessions[cookie.Value]
	s.mu.Unlock()
	if !known || nonce != params["nonce"] {
		return nil, errors.New("nonce does not belong to this session")
	}

	newHash, err := digestHasher(params["algorithm"])
	if err != nil {
		return nil, err
	}
	ha1 := digestHex(newHash, digestUser+":"+digestRealm+":"+digestPass)
	ha2 := digestHex(newHash, r.Method+":"+params["uri"])
	expected := digestHex(newHash, strings.Join([]string{
		ha1, params["nonce"], params["nc"], params["cnonce"], params["qop"], ha2,
	}, ":"))
	if subtle.ConstantTimeCompare([]byte(expected), []byte(params["response"])) != 1 {
		return nil, errors.New("digest response does not match")
	}

	algorithm := params["algorithm"]
	if algorithm == "" {
		algorithm = "MD5"
	}

	return &whoamiResponse{Scheme: "digest", User: digestUser, Algorithm: algorithm}, nil
}

func digestHasher(algorithm string) (func() hash.Hash, error) {
	switch strings.ToUpper(algorithm) {
	case "", "MD5":
		return md5.New, nil
	case "SHA-256":
		return sha256.New, nil
	default:
		return nil, fmt.Errorf("unsupported digest algorithm %q", algorithm)
	}
}

func digestHex(newHash func() hash.Hash, s string) string {
	h := newHash()
	h.Write([]byte(s))

	return hex.EncodeToString(h.Sum(nil))
}

// parseDigestParams reads the comma-separated key=value list of a Digest header;
// unquoted values (algorithm, qop, nc) are kept as they arrive.
func parseDigestParams(header string) map[string]string {
	_, rest, found := strings.Cut(header, " ")
	if !found {
		return map[string]string{}
	}

	out := map[string]string{}
	for _, part := range splitDigestParts(rest) {
		key, value, ok := strings.Cut(strings.TrimSpace(part), "=")
		if !ok {
			continue
		}
		out[strings.ToLower(strings.TrimSpace(key))] = strings.Trim(strings.TrimSpace(value), `"`)
	}

	return out
}

// splitDigestParts splits on commas outside quotes: a quoted value may hold one.
func splitDigestParts(s string) []string {
	var (
		parts   []string
		current strings.Builder
		quoted  bool
	)
	for i := 0; i < len(s); i++ {
		switch c := s[i]; {
		case c == '"':
			quoted = !quoted
			current.WriteByte(c)
		case c == ',' && !quoted:
			parts = append(parts, current.String())
			current.Reset()
		default:
			current.WriteByte(c)
		}
	}
	if current.Len() > 0 {
		parts = append(parts, current.String())
	}

	return parts
}

// verifySigV4 recomputes the signature from the request itself, using only the
// SignedHeaders list the client declared — an independent check of the signer.
func verifySigV4(r *http.Request, body []byte) (*whoamiResponse, error) {
	params := parseSigV4Header(r.Header.Get("Authorization"))
	credential := strings.Split(params["credential"], "/")
	if len(credential) != 5 {
		return nil, errors.New("malformed Credential")
	}
	accessKey, dateStamp, region, service := credential[0], credential[1], credential[2], credential[3]
	if accessKey != awsAccessKeyID {
		return nil, fmt.Errorf("unknown access key %q", accessKey)
	}

	amzDate := r.Header.Get("X-Amz-Date")
	if amzDate == "" {
		return nil, errors.New("X-Amz-Date is missing")
	}
	if !strings.HasPrefix(amzDate, dateStamp) {
		return nil, errors.New("X-Amz-Date does not match the credential scope")
	}

	payloadHash := hexSHA256(body)
	if declared := r.Header.Get("X-Amz-Content-Sha256"); declared != "" {
		if declared != unsignedPayload && declared != payloadHash {
			return nil, errors.New("x-amz-content-sha256 does not match the body")
		}
		payloadHash = declared
	}

	signed := strings.Split(params["signedheaders"], ";")
	canonicalRequest := strings.Join([]string{
		r.Method,
		canonicalSigV4Path(r.URL, service),
		canonicalSigV4Query(r.URL),
		canonicalSigV4Headers(r, signed),
		params["signedheaders"],
		payloadHash,
	}, "\n")

	scope := strings.Join([]string{dateStamp, region, service, "aws4_request"}, "/")
	stringToSign := strings.Join([]string{
		sigv4Algorithm, amzDate, scope, hexSHA256([]byte(canonicalRequest)),
	}, "\n")

	key := hmacSHA256([]byte("AWS4"+awsSecretAccessKey), dateStamp)
	key = hmacSHA256(key, region)
	key = hmacSHA256(key, service)
	key = hmacSHA256(key, "aws4_request")
	expected := hex.EncodeToString(hmacSHA256(key, stringToSign))
	if subtle.ConstantTimeCompare([]byte(expected), []byte(params["signature"])) != 1 {
		return nil, errors.New("signature does not match")
	}

	return &whoamiResponse{Scheme: "aws_sigv4", Region: region, Service: service}, nil
}

func parseSigV4Header(header string) map[string]string {
	_, rest, found := strings.Cut(header, " ")
	if !found {
		return map[string]string{}
	}

	out := map[string]string{}
	for _, part := range strings.Split(rest, ",") {
		key, value, ok := strings.Cut(strings.TrimSpace(part), "=")
		if !ok {
			continue
		}
		out[strings.ToLower(strings.TrimSpace(key))] = strings.TrimSpace(value)
	}

	return out
}

// canonicalSigV4Headers rebuilds the header block for the declared names; Host
// never reaches r.Header, so it is read from the request line.
func canonicalSigV4Headers(r *http.Request, names []string) string {
	var b strings.Builder
	for _, name := range names {
		values := r.Header.Values(name)
		if strings.EqualFold(name, "host") {
			values = []string{r.Host}
		}
		cleaned := make([]string, 0, len(values))
		for _, v := range values {
			cleaned = append(cleaned, collapseSpaces(v))
		}
		b.WriteString(strings.ToLower(name))
		b.WriteByte(':')
		b.WriteString(strings.Join(cleaned, ","))
		b.WriteByte('\n')
	}

	return b.String()
}

func canonicalSigV4Path(u *url.URL, service string) string {
	path := u.EscapedPath()
	if path == "" {
		return "/"
	}
	if strings.HasPrefix(strings.ToLower(service), "s3") {
		return path
	}

	return awsEscape(path, true)
}

func canonicalSigV4Query(u *url.URL) string {
	if u.RawQuery == "" {
		return ""
	}

	type pair struct{ key, value string }
	var pairs []pair
	for _, raw := range strings.Split(u.RawQuery, "&") {
		if raw == "" {
			continue
		}
		key, value, _ := strings.Cut(raw, "=")
		pairs = append(pairs, pair{key: awsEscape(unescape(key), false), value: awsEscape(unescape(value), false)})
	}
	sort.Slice(pairs, func(i, j int) bool {
		if pairs[i].key != pairs[j].key {
			return pairs[i].key < pairs[j].key
		}
		return pairs[i].value < pairs[j].value
	})

	parts := make([]string, 0, len(pairs))
	for _, p := range pairs {
		parts = append(parts, p.key+"="+p.value)
	}

	return strings.Join(parts, "&")
}

func unescape(s string) string {
	decoded, err := url.PathUnescape(s)
	if err != nil {
		return s
	}

	return decoded
}

func awsEscape(s string, keepSlash bool) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		ch := s[i]
		switch {
		case ch >= 'A' && ch <= 'Z', ch >= 'a' && ch <= 'z', ch >= '0' && ch <= '9',
			ch == '-', ch == '_', ch == '.', ch == '~':
			b.WriteByte(ch)
		case ch == '/' && keepSlash:
			b.WriteByte('/')
		default:
			fmt.Fprintf(&b, "%%%02X", ch)
		}
	}

	return b.String()
}

func collapseSpaces(v string) string {
	out := strings.TrimSpace(v)
	for strings.Contains(out, "  ") {
		out = strings.ReplaceAll(out, "  ", " ")
	}

	return out
}

func hexSHA256(b []byte) string {
	sum := sha256.Sum256(b)

	return hex.EncodeToString(sum[:])
}

func hmacSHA256(key []byte, data string) []byte {
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(data))

	return mac.Sum(nil)
}

func readBody(r *http.Request) ([]byte, error) {
	if r.Body == nil {
		return nil, nil
	}
	defer func() { _ = r.Body.Close() }()

	buf, err := io.ReadAll(io.LimitReader(r.Body, maxRequestBody+1))
	if err != nil {
		return nil, err
	}
	if len(buf) > maxRequestBody {
		return nil, errors.New("request body too large")
	}

	return buf, nil
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func writeHTML(w http.ResponseWriter, status int, body string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_, _ = io.WriteString(w, body)
}

// redirectPage stands in for a consent screen: the browser follows the refresh
// back to the client's loopback listener.
func redirectPage(target string) string {
	escaped := html.EscapeString(target)

	return `<!doctype html><meta charset="utf-8"><title>authmock</title>` +
		`<meta http-equiv="refresh" content="0;url=` + escaped + `">` +
		`<body style="font:16px system-ui;padding:3rem;text-align:center">` +
		`<a href="` + escaped + `">Returning to the application…</a></body>`
}

func verificationPage(verdict string) string {
	return `<!doctype html><meta charset="utf-8"><title>authmock</title>` +
		`<body style="font:16px system-ui;padding:3rem;text-align:center">Device ` +
		html.EscapeString(verdict) + `.</body>`
}

func fieldError(w http.ResponseWriter, status int, code, field, description string) {
	body := map[string]string{"error": code, "error_description": description}
	if field != "" {
		body["field"] = field
	}
	writeJSON(w, status, body)
}

func oauthError(w http.ResponseWriter, status int, code, description string) {
	writeJSON(w, status, map[string]string{"error": code, "error_description": description})
}

func randomHex(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}

	return hex.EncodeToString(b)
}
