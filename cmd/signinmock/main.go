// Command signinmock fakes the sync server for the browser sign-in e2e: AuthService over gRPC
// plus the cabinet page. Every account and token is a fixture; Approve/Deny skip bearer checks.
// Run: `go run ./cmd/signinmock` → gRPC 127.0.0.1:9878, cabinet 127.0.0.1:9879.
package main

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"sync"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	authv1 "github.com/tetiva-app/proto/go/gophercourier/auth/v1"
)

const (
	defaultGRPCAddr = "127.0.0.1:9878"
	defaultHTTPAddr = "127.0.0.1:9879"

	requestTTL = 5 * time.Minute
	// What an approved request gets while the browser confirms the address.
	verifyExtension = 30 * time.Minute
	// How many approved polls the email gate answers with PENDING before the
	// tokens go out.
	verifyGatePolls = 2
	// base64url(SHA-256(x)) without padding is always this long.
	challengeLen = 43

	maxBody = 1 << 16

	fixtureEmail   = "browser@signinmock.local"
	fixtureUserID  = "user-signinmock"
	fixtureOrgID   = "org-1"
	fixtureAccess  = "signinmock-access-token"
	fixtureRefresh = "signinmock-refresh-token"
)

// Lifecycle of one request, a subset of the server's status table.
const (
	statePending  = "pending"
	stateApproved = "approved"
	stateDenied   = "denied"
)

// mode is switched at runtime through POST /__mode: the Playwright project runs
// every scenario against one process it does not start itself.
type mode struct {
	NoRedirect bool `json:"noRedirect"`
	VerifyGate bool `json:"verifyGate"`
}

type signInRequest struct {
	id             string
	clientID       string
	codeChallenge  string
	claimChallenge string
	redirectURI    string
	intent         string

	state      string
	createdAt  time.Time
	expiresAt  time.Time
	approvedAt time.Time
	claimed    bool
	consumed   bool
	gatePolls  int
	extended   bool
}

type server struct {
	authv1.UnimplementedAuthServiceServer

	httpAddr     string
	pollInterval int
	capability   bool

	mu       sync.Mutex
	requests map[string]*signInRequest
	mode     mode
}

func newServer(pollInterval int, capability bool) *server {
	return &server{
		pollInterval: pollInterval,
		capability:   capability,
		requests:     make(map[string]*signInRequest),
	}
}

func main() {
	grpcAddr := flag.String("grpc", defaultGRPCAddr, "gRPC listen address")
	httpAddr := flag.String("http", defaultHTTPAddr, "cabinet listen address")
	pollInterval := flag.Int("poll-interval", 1, "poll_interval_seconds handed to the app")
	noCapability := flag.Bool("no-capability", false, "answer GetServerInfo without desktop_signin")
	flag.Parse()

	srv := newServer(*pollInterval, !*noCapability)
	srv.httpAddr = *httpAddr

	lis, err := net.Listen("tcp", *grpcAddr)
	if err != nil {
		log.Fatalf("signinmock: listen %s: %v", *grpcAddr, err)
	}

	gs := grpc.NewServer()
	authv1.RegisterAuthServiceServer(gs, srv)
	go func() {
		log.Printf("signinmock: gRPC on %s", *grpcAddr)
		if err := gs.Serve(lis); err != nil {
			log.Fatalf("signinmock: grpc serve: %v", err)
		}
	}()

	log.Printf("signinmock: cabinet on http://%s/desktop-signin", *httpAddr)
	httpSrv := &http.Server{
		Addr:              *httpAddr,
		Handler:           srv.httpHandler(),
		ReadHeaderTimeout: 5 * time.Second,
	}
	if err := httpSrv.ListenAndServe(); err != nil {
		log.Fatalf("signinmock: http serve: %v", err)
	}
}

// GetServerInfo is the discovery the modal calls before it offers any button.
func (s *server) GetServerInfo(context.Context, *authv1.GetServerInfoRequest) (*authv1.GetServerInfoResponse, error) {
	b := authv1.GetServerInfoResponse_builder{
		ServerVersion:    "signinmock",
		RegistrationOpen: true,
	}
	if s.capability {
		b.Capabilities = []string{"desktop_signin"}
		b.DesktopSigninUrl = "http://" + s.httpAddr + "/desktop-signin"
	}

	return b.Build(), nil
}

func (s *server) StartDesktopSignIn(_ context.Context, req *authv1.StartDesktopSignInRequest) (*authv1.StartDesktopSignInResponse, error) {
	if req.GetClientId() == "" {
		return nil, status.Error(codes.InvalidArgument, "client_id is required")
	}
	if !validChallenge(req.GetCodeChallenge()) {
		return nil, status.Error(codes.InvalidArgument, "code_challenge must be 43 base64url characters")
	}
	if !validChallenge(req.GetClaimChallenge()) {
		return nil, status.Error(codes.InvalidArgument, "claim_challenge must be 43 base64url characters")
	}
	if err := validateRedirectURI(req.GetRedirectUri()); err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "redirect_uri: %v", err)
	}

	intent := req.GetIntent()
	if intent == "" {
		intent = "signin"
	}

	now := time.Now()
	r := &signInRequest{
		id:             "dsr_" + randomHex(8),
		clientID:       req.GetClientId(),
		codeChallenge:  req.GetCodeChallenge(),
		claimChallenge: req.GetClaimChallenge(),
		redirectURI:    req.GetRedirectUri(),
		intent:         intent,
		state:          statePending,
		createdAt:      now,
		expiresAt:      now.Add(requestTTL),
	}

	s.mu.Lock()
	s.requests[r.id] = r
	s.mu.Unlock()

	login := "http://" + s.httpAddr + "/desktop-signin?request=" + url.QueryEscape(r.id) +
		"&intent=" + url.QueryEscape(intent)

	return authv1.StartDesktopSignInResponse_builder{
		RequestId:           r.id,
		LoginUrl:            login,
		ExpiresAt:           timestamppb.New(r.expiresAt),
		PollIntervalSeconds: int32(s.pollInterval),
	}.Build(), nil
}

func (s *server) PollDesktopSignIn(_ context.Context, req *authv1.PollDesktopSignInRequest) (*authv1.PollDesktopSignInResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	r := s.requests[req.GetRequestId()]
	if r == nil {
		return pollAnswer(authv1.DesktopSignInStatus_DESKTOP_SIGN_IN_STATUS_EXPIRED, now), nil
	}
	if r.clientID != req.GetClientId() || digestOf(req.GetCodeVerifier()) != r.codeChallenge {
		return nil, status.Error(codes.Unauthenticated, "the request belongs to another client")
	}
	if !now.Before(r.expiresAt) {
		return pollAnswer(authv1.DesktopSignInStatus_DESKTOP_SIGN_IN_STATUS_EXPIRED, r.expiresAt), nil
	}

	switch r.state {
	case stateDenied:
		return pollAnswer(authv1.DesktopSignInStatus_DESKTOP_SIGN_IN_STATUS_DENIED, r.expiresAt), nil
	case stateApproved:
		if r.consumed {
			return pollAnswer(authv1.DesktopSignInStatus_DESKTOP_SIGN_IN_STATUS_EXPIRED, r.expiresAt), nil
		}
		if s.mode.VerifyGate && r.gatePolls < verifyGatePolls {
			r.gatePolls++
			if !r.extended {
				r.expiresAt = r.approvedAt.Add(verifyExtension)
				r.extended = true
			}

			return authv1.PollDesktopSignInResponse_builder{
				Status:                   authv1.DesktopSignInStatus_DESKTOP_SIGN_IN_STATUS_PENDING,
				EmailVerificationPending: true,
				ExpiresAt:                timestamppb.New(r.expiresAt),
			}.Build(), nil
		}
		r.consumed = true

		return authv1.PollDesktopSignInResponse_builder{
			Status:       authv1.DesktopSignInStatus_DESKTOP_SIGN_IN_STATUS_APPROVED,
			AccessToken:  fixtureAccess,
			RefreshToken: fixtureRefresh,
			User:         fixtureUser(),
			ActiveOrgId:  fixtureOrgID,
			ExpiresAt:    timestamppb.New(r.expiresAt),
		}.Build(), nil
	default:
		return pollAnswer(authv1.DesktopSignInStatus_DESKTOP_SIGN_IN_STATUS_PENDING, r.expiresAt), nil
	}
}

func (s *server) CancelDesktopSignIn(_ context.Context, req *authv1.CancelDesktopSignInRequest) (*authv1.CancelDesktopSignInResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	r := s.requests[req.GetRequestId()]
	if r == nil {
		return authv1.CancelDesktopSignInResponse_builder{}.Build(), nil
	}
	if r.clientID != req.GetClientId() || digestOf(req.GetCodeVerifier()) != r.codeChallenge {
		return nil, status.Error(codes.Unauthenticated, "the request belongs to another client")
	}
	// Only a pending request goes away; an approved one still owes its tokens.
	if r.state == statePending {
		delete(s.requests, r.id)
	}

	return authv1.CancelDesktopSignInResponse_builder{}.Build(), nil
}

func (s *server) ClaimDesktopSignIn(_ context.Context, req *authv1.ClaimDesktopSignInRequest) (*authv1.ClaimDesktopSignInResponse, error) {
	intent, expiresAt, err := s.claim(req.GetRequestId(), req.GetClaimSecret())
	if err != nil {
		return nil, err
	}

	return authv1.ClaimDesktopSignInResponse_builder{
		Intent:    intent,
		ExpiresAt: timestamppb.New(expiresAt),
	}.Build(), nil
}

func (s *server) GetDesktopSignIn(_ context.Context, req *authv1.GetDesktopSignInRequest) (*authv1.GetDesktopSignInResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	r, err := s.lookup(req.GetRequestId(), req.GetClaimSecret())
	if err != nil {
		return nil, err
	}
	if !r.claimed {
		return nil, status.Error(codes.Unauthenticated, "the request was not opened from its link")
	}

	return authv1.GetDesktopSignInResponse_builder{
		RequestId:  r.id,
		Status:     viewStatus(r),
		DeviceName: "signinmock fixture",
		Ip:         "127.0.0.1",
		SameIp:     true,
		CreatedAt:  timestamppb.New(r.createdAt),
		ExpiresAt:  timestamppb.New(r.expiresAt),
	}.Build(), nil
}

func (s *server) ApproveDesktopSignIn(_ context.Context, req *authv1.ApproveDesktopSignInRequest) (*authv1.ApproveDesktopSignInResponse, error) {
	redirect, err := s.decide(req.GetRequestId(), req.GetClaimSecret(), true)
	if err != nil {
		return nil, err
	}

	return authv1.ApproveDesktopSignInResponse_builder{RedirectUri: redirect}.Build(), nil
}

func (s *server) DenyDesktopSignIn(_ context.Context, req *authv1.DenyDesktopSignInRequest) (*authv1.DenyDesktopSignInResponse, error) {
	if _, err := s.decide(req.GetRequestId(), req.GetClaimSecret(), false); err != nil {
		return nil, err
	}

	return authv1.DenyDesktopSignInResponse_builder{}.Build(), nil
}

// The rest of AuthService is only as complete as enableSync, the Devices list
// and GetStatus need: no session bookkeeping, no bearer checks.

func (s *server) GetMe(context.Context, *authv1.GetMeRequest) (*authv1.GetMeResponse, error) {
	return authv1.GetMeResponse_builder{User: fixtureUser()}.Build(), nil
}

func (s *server) Me(context.Context, *authv1.MeRequest) (*authv1.MeResponse, error) {
	return authv1.MeResponse_builder{
		User: fixtureUser(),
		Sessions: []*authv1.SessionView{
			authv1.SessionView_builder{
				Id:         "session-signinmock",
				ClientId:   "signinmock",
				UserAgent:  "Tetiva desktop (signinmock)",
				Ip:         "127.0.0.1",
				LastUsedAt: timestamppb.New(time.Now()),
				IsCurrent:  true,
			}.Build(),
		},
	}.Build(), nil
}

func (s *server) Refresh(context.Context, *authv1.RefreshRequest) (*authv1.RefreshResponse, error) {
	return authv1.RefreshResponse_builder{
		AccessToken:  fixtureAccess,
		RefreshToken: fixtureRefresh,
		User:         fixtureUser(),
		ActiveOrgId:  fixtureOrgID,
	}.Build(), nil
}

func (s *server) Logout(context.Context, *authv1.LogoutRequest) (*authv1.LogoutResponse, error) {
	return authv1.LogoutResponse_builder{}.Build(), nil
}

func (s *server) LogoutAll(context.Context, *authv1.LogoutAllRequest) (*authv1.LogoutAllResponse, error) {
	return authv1.LogoutAllResponse_builder{}.Build(), nil
}

func (s *server) ResendVerification(context.Context, *authv1.ResendVerificationRequest) (*authv1.ResendVerificationResponse, error) {
	return authv1.ResendVerificationResponse_builder{}.Build(), nil
}

// claim binds a pending request to the tab that opened the link. The RPC and the
// cabinet page both come through here.
func (s *server) claim(id, secret string) (string, time.Time, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	r, err := s.lookup(id, secret)
	if err != nil {
		return "", time.Time{}, err
	}
	if !time.Now().Before(r.expiresAt) {
		return "", time.Time{}, status.Error(codes.FailedPrecondition, "the request expired")
	}
	if r.claimed || r.state != statePending {
		return "", time.Time{}, status.Error(codes.Aborted, "the request is already open elsewhere")
	}
	r.claimed = true

	return r.intent, r.expiresAt, nil
}

// decide approves or denies a claimed request and answers with the loopback the
// browser should visit, empty in the poll-only mode.
func (s *server) decide(id, secret string, approve bool) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	r, err := s.lookup(id, secret)
	if err != nil {
		return "", err
	}
	if !r.claimed {
		return "", status.Error(codes.Unauthenticated, "the request was not opened from its link")
	}
	if !time.Now().Before(r.expiresAt) {
		return "", status.Error(codes.FailedPrecondition, "the request expired")
	}
	if r.state != statePending {
		return "", status.Error(codes.Aborted, "the request was already answered")
	}

	if !approve {
		r.state = stateDenied

		return "", nil
	}
	r.state = stateApproved
	r.approvedAt = time.Now()
	if s.mode.NoRedirect {
		return "", nil
	}

	return r.redirectURI, nil
}

// lookup resolves a request by its claim secret; callers hold s.mu.
func (s *server) lookup(id, secret string) (*signInRequest, error) {
	r := s.requests[id]
	if r == nil {
		return nil, status.Error(codes.NotFound, "no such sign-in request")
	}
	if subtle.ConstantTimeCompare([]byte(digestOf(secret)), []byte(r.claimChallenge)) != 1 {
		return nil, status.Error(codes.Unauthenticated, "the claim secret does not match")
	}

	return r, nil
}

func (s *server) httpHandler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /desktop-signin", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = io.WriteString(w, cabinetPage)
	})
	mux.HandleFunc("POST /api/claim", s.handleClaim)
	mux.HandleFunc("POST /api/approve", s.handleApprove)
	mux.HandleFunc("POST /api/deny", s.handleDeny)
	mux.HandleFunc("POST /__mode", s.handleMode)
	mux.HandleFunc("POST /__reset", s.handleReset)

	return mux
}

// cabinetCall is what the consent page posts back for every action.
type cabinetCall struct {
	RequestID   string `json:"requestId"`
	ClaimSecret string `json:"claimSecret"`
}

func (s *server) handleClaim(w http.ResponseWriter, r *http.Request) {
	call, ok := decodeCall(w, r)
	if !ok {
		return
	}

	intent, expiresAt, err := s.claim(call.RequestID, call.ClaimSecret)
	if err != nil {
		writeStatusError(w, err)

		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"intent":    intent,
		"expiresAt": expiresAt.UTC().Format(time.RFC3339),
	})
}

func (s *server) handleApprove(w http.ResponseWriter, r *http.Request) {
	call, ok := decodeCall(w, r)
	if !ok {
		return
	}

	redirect, err := s.decide(call.RequestID, call.ClaimSecret, true)
	if err != nil {
		writeStatusError(w, err)

		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"redirectUri": redirect})
}

func (s *server) handleDeny(w http.ResponseWriter, r *http.Request) {
	call, ok := decodeCall(w, r)
	if !ok {
		return
	}

	if _, err := s.decide(call.RequestID, call.ClaimSecret, false); err != nil {
		writeStatusError(w, err)

		return
	}
	writeJSON(w, http.StatusOK, map[string]any{})
}

func (s *server) handleMode(w http.ResponseWriter, r *http.Request) {
	var m mode
	if err := json.NewDecoder(io.LimitReader(r.Body, maxBody)).Decode(&m); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "malformed body"})

		return
	}

	s.mu.Lock()
	s.mode = m
	s.mu.Unlock()

	w.WriteHeader(http.StatusNoContent)
}

func (s *server) handleReset(w http.ResponseWriter, _ *http.Request) {
	s.mu.Lock()
	s.requests = make(map[string]*signInRequest)
	s.mu.Unlock()

	w.WriteHeader(http.StatusNoContent)
}

func decodeCall(w http.ResponseWriter, r *http.Request) (cabinetCall, bool) {
	var call cabinetCall
	if err := json.NewDecoder(io.LimitReader(r.Body, maxBody)).Decode(&call); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "malformed body"})

		return cabinetCall{}, false
	}

	return call, true
}

func writeJSON(w http.ResponseWriter, code int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(body)
}

// writeStatusError keeps the gRPC vocabulary readable from the page: the two
// transports share the same refusals.
func writeStatusError(w http.ResponseWriter, err error) {
	st := status.Convert(err)
	code := http.StatusInternalServerError
	switch st.Code() {
	case codes.InvalidArgument:
		code = http.StatusBadRequest
	case codes.Unauthenticated:
		code = http.StatusUnauthorized
	case codes.NotFound:
		code = http.StatusNotFound
	case codes.Aborted:
		code = http.StatusConflict
	case codes.FailedPrecondition:
		code = http.StatusGone
	}
	writeJSON(w, code, map[string]any{"error": st.Message()})
}

func pollAnswer(st authv1.DesktopSignInStatus, expiresAt time.Time) *authv1.PollDesktopSignInResponse {
	return authv1.PollDesktopSignInResponse_builder{
		Status:    st,
		ExpiresAt: timestamppb.New(expiresAt),
	}.Build()
}

func viewStatus(r *signInRequest) authv1.DesktopSignInStatus {
	if !time.Now().Before(r.expiresAt) {
		return authv1.DesktopSignInStatus_DESKTOP_SIGN_IN_STATUS_EXPIRED
	}
	switch r.state {
	case stateApproved:
		return authv1.DesktopSignInStatus_DESKTOP_SIGN_IN_STATUS_APPROVED
	case stateDenied:
		return authv1.DesktopSignInStatus_DESKTOP_SIGN_IN_STATUS_DENIED
	default:
		return authv1.DesktopSignInStatus_DESKTOP_SIGN_IN_STATUS_PENDING
	}
}

func fixtureUser() *authv1.User {
	return authv1.User_builder{
		Id:            fixtureUserID,
		Email:         fixtureEmail,
		Name:          "Browser Fixture",
		EmailVerified: true,
	}.Build()
}

func digestOf(secret string) string {
	sum := sha256.Sum256([]byte(secret))

	return base64.RawURLEncoding.EncodeToString(sum[:])
}

func validChallenge(v string) bool {
	if len(v) != challengeLen {
		return false
	}
	_, err := base64.RawURLEncoding.DecodeString(v)

	return err == nil
}

// validateRedirectURI mirrors the server's rule: loopback IPv4, an explicit
// port, exactly /callback, nothing else attached.
func validateRedirectURI(raw string) error {
	if raw == "" {
		return nil
	}

	u, err := url.Parse(raw)
	if err != nil {
		return errors.New("not a URL")
	}
	if u.Scheme != "http" {
		return errors.New("must be http")
	}
	if u.User != nil {
		return errors.New("must not carry userinfo")
	}
	if u.RawQuery != "" || u.Fragment != "" {
		return errors.New("must carry no query and no fragment")
	}
	if u.Path != "/callback" {
		return errors.New("path must be /callback")
	}

	host, port, err := net.SplitHostPort(u.Host)
	if err != nil {
		return errors.New("must carry a port")
	}
	if host != "127.0.0.1" {
		return errors.New("host must be 127.0.0.1")
	}
	n, err := strconv.Atoi(port)
	if err != nil || n < 1 || n > 65535 {
		return errors.New("port must be 1..65535")
	}

	return nil
}

func randomHex(n int) string {
	b := make([]byte, n)
	// crypto/rand.Read cannot fail: it panics inside rather than returning.
	_, _ = rand.Read(b)

	return hex.EncodeToString(b)
}

// cabinetPage is the consent screen: it reads the claim secret out of the
// fragment, hides it from the address bar, and posts back the decision.
const cabinetPage = `<!doctype html>
<html lang="en"><head><meta charset="utf-8"><title>Tetiva sign-in (signinmock)</title></head>
<body style="font:16px system-ui;padding:2rem;max-width:34rem">
<h1 style="font-size:1.2rem">Approve this sign-in</h1>
<p>Request <code id="request"></code></p>
<p id="error" style="color:#b00020"></p>
<p id="status"></p>
<p><button id="approve" disabled>Approve</button> <button id="deny" disabled>Deny</button></p>
<script>
var params = new URLSearchParams(location.search);
var requestId = params.get('request') || '';
var claimSecret = new URLSearchParams(location.hash.replace(/^#/, '')).get('claim') || '';
document.getElementById('request').textContent = requestId;
history.replaceState(null, '', location.pathname + location.search);

function show(id, text) { document.getElementById(id).textContent = text; }

function call(path) {
  return fetch(path, {
    method: 'POST',
    headers: { 'content-type': 'application/json' },
    body: JSON.stringify({ requestId: requestId, claimSecret: claimSecret })
  }).then(function (res) {
    return res.json().catch(function () { return {}; }).then(function (body) {
      if (!res.ok) {
        show('error', 'Cannot continue: ' + (body.error || res.status));
        return null;
      }
      return body;
    });
  });
}

call('/api/claim').then(function (ok) {
  if (!ok) return;
  document.getElementById('approve').disabled = false;
  document.getElementById('deny').disabled = false;
});

document.getElementById('approve').onclick = function () {
  call('/api/approve').then(function (res) {
    if (!res) return;
    if (res.redirectUri) { location.assign(res.redirectUri); return; }
    show('status', 'Done. Return to Tetiva.');
  });
};

document.getElementById('deny').onclick = function () {
  call('/api/deny').then(function (res) {
    if (!res) return;
    show('status', 'Denied. Return to Tetiva.');
  });
};
</script>
</body></html>`
