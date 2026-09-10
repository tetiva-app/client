package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"

	authv1 "github.com/tetiva-app/proto/go/gophercourier/auth/v1"
)

const (
	testClientID = "client-fixture"
	testVerifier = "verifier-fixture-for-the-signin-mock-tests"
	testClaim    = "claim-secret-fixture-for-the-signin-mock"
)

func digest(secret string) string {
	sum := sha256.Sum256([]byte(secret))

	return base64.RawURLEncoding.EncodeToString(sum[:])
}

type harness struct {
	srv  *server
	http *httptest.Server
	auth authv1.AuthServiceClient
}

func newHarness(t *testing.T) *harness {
	t.Helper()

	srv := newServer(1, true)
	hs := httptest.NewUnstartedServer(srv.httpHandler())
	// The login URL the mock hands out points back at this listener, so the
	// address has to be known before anything can serve.
	srv.httpAddr = hs.Listener.Addr().String()
	hs.Start()
	t.Cleanup(hs.Close)

	lis := bufconn.Listen(1 << 20)
	gs := grpc.NewServer()
	authv1.RegisterAuthServiceServer(gs, srv)
	go func() { _ = gs.Serve(lis) }()
	t.Cleanup(gs.Stop)

	conn, err := grpc.NewClient("passthrough:///bufnet",
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) { return lis.DialContext(ctx) }),
		grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatalf("dial bufconn: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })

	return &harness{srv: srv, http: hs, auth: authv1.NewAuthServiceClient(conn)}
}

func (h *harness) start(t *testing.T, redirectURI string) *authv1.StartDesktopSignInResponse {
	t.Helper()

	resp, err := h.auth.StartDesktopSignIn(t.Context(), authv1.StartDesktopSignInRequest_builder{
		ClientId:       testClientID,
		CodeChallenge:  digest(testVerifier),
		ClaimChallenge: digest(testClaim),
		RedirectUri:    redirectURI,
		Intent:         "signin",
	}.Build())
	if err != nil {
		t.Fatalf("start: %v", err)
	}

	return resp
}

func (h *harness) poll(t *testing.T, id string) *authv1.PollDesktopSignInResponse {
	t.Helper()

	resp, err := h.auth.PollDesktopSignIn(t.Context(), authv1.PollDesktopSignInRequest_builder{
		RequestId:    id,
		ClientId:     testClientID,
		CodeVerifier: testVerifier,
	}.Build())
	if err != nil {
		t.Fatalf("poll: %v", err)
	}
	if !resp.HasExpiresAt() {
		t.Fatal("poll answer carries no expires_at")
	}

	return resp
}

func (h *harness) claim(t *testing.T, id string) error {
	t.Helper()

	_, err := h.auth.ClaimDesktopSignIn(t.Context(), authv1.ClaimDesktopSignInRequest_builder{
		RequestId:   id,
		ClaimSecret: testClaim,
	}.Build())

	return err
}

func (h *harness) post(t *testing.T, path string, body any) (int, map[string]any) {
	t.Helper()

	raw, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal body: %v", err)
	}
	resp, err := h.http.Client().Post(h.http.URL+path, "application/json", bytes.NewReader(raw))
	if err != nil {
		t.Fatalf("post %s: %v", path, err)
	}
	defer func() { _ = resp.Body.Close() }()

	out := map[string]any{}
	_ = json.NewDecoder(resp.Body).Decode(&out)

	return resp.StatusCode, out
}

func TestStartValidatesRedirectURI(t *testing.T) {
	h := newHarness(t)

	tests := []struct {
		redirect string
		ok       bool
	}{
		{"", true},
		{"http://127.0.0.1:54321/callback", true},
		{"http://[::1]:54321/callback", false},
		{"http://localhost:8080/callback", false},
		{"https://127.0.0.1/callback", false},
		{"http://127.0.0.1:0/callback", false},
		{"http://127.0.0.1:8080/callback?x=1", false},
		{"http://127.0.0.1:8080/cb", false},
		{"http://user@127.0.0.1:8080/callback", false},
		{"http://127.1:8080/callback", false},
		{"evil://x", false},
	}

	for _, tc := range tests {
		t.Run(tc.redirect, func(t *testing.T) {
			_, err := h.auth.StartDesktopSignIn(t.Context(), authv1.StartDesktopSignInRequest_builder{
				ClientId:       testClientID,
				CodeChallenge:  digest(testVerifier),
				ClaimChallenge: digest(testClaim),
				RedirectUri:    tc.redirect,
			}.Build())

			if tc.ok {
				if err != nil {
					t.Fatalf("want accepted, got %v", err)
				}

				return
			}
			if status.Code(err) != codes.InvalidArgument {
				t.Fatalf("want InvalidArgument, got %v", err)
			}
		})
	}
}

func TestStartValidatesChallengesAndClientID(t *testing.T) {
	h := newHarness(t)

	tests := []struct {
		name                  string
		clientID              string
		codeChall, claimChall string
	}{
		{"empty client id", "", digest(testVerifier), digest(testClaim)},
		{"short code challenge", testClientID, "too-short", digest(testClaim)},
		{"long code challenge", testClientID, digest(testVerifier) + "a", digest(testClaim)},
		{"not base64url", testClientID, strings.Repeat("*", 43), digest(testClaim)},
		{"short claim challenge", testClientID, digest(testVerifier), "too-short"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := h.auth.StartDesktopSignIn(t.Context(), authv1.StartDesktopSignInRequest_builder{
				ClientId:       tc.clientID,
				CodeChallenge:  tc.codeChall,
				ClaimChallenge: tc.claimChall,
			}.Build())
			if status.Code(err) != codes.InvalidArgument {
				t.Fatalf("want InvalidArgument, got %v", err)
			}
		})
	}
}

func TestStartAnswersALoginURLWithoutAFragment(t *testing.T) {
	h := newHarness(t)

	resp := h.start(t, "http://127.0.0.1:54321/callback")
	if resp.GetRequestId() == "" {
		t.Fatal("no request id")
	}
	login := resp.GetLoginUrl()
	if !strings.Contains(login, "request="+resp.GetRequestId()) {
		t.Fatalf("login url %q carries no request id", login)
	}
	if !strings.Contains(login, "intent=signin") {
		t.Fatalf("login url %q carries no intent", login)
	}
	if strings.Contains(login, "#") {
		t.Fatalf("login url %q carries a fragment", login)
	}
	if !resp.HasExpiresAt() {
		t.Fatal("no expires_at")
	}
	if resp.GetPollIntervalSeconds() != 1 {
		t.Fatalf("poll interval = %d, want 1", resp.GetPollIntervalSeconds())
	}
}

func TestGetServerInfoDropsTheCapabilityOnDemand(t *testing.T) {
	h := newHarness(t)

	info, err := h.auth.GetServerInfo(t.Context(), authv1.GetServerInfoRequest_builder{}.Build())
	if err != nil {
		t.Fatalf("get server info: %v", err)
	}
	if len(info.GetCapabilities()) != 1 || info.GetCapabilities()[0] != "desktop_signin" {
		t.Fatalf("capabilities = %v", info.GetCapabilities())
	}
	if !strings.HasSuffix(info.GetDesktopSigninUrl(), "/desktop-signin") {
		t.Fatalf("desktop sign-in url = %q", info.GetDesktopSigninUrl())
	}

	h.srv.capability = false
	info, err = h.auth.GetServerInfo(t.Context(), authv1.GetServerInfoRequest_builder{}.Build())
	if err != nil {
		t.Fatalf("get server info: %v", err)
	}
	if len(info.GetCapabilities()) != 0 || info.GetDesktopSigninUrl() != "" {
		t.Fatalf("capability was not dropped: %v %q", info.GetCapabilities(), info.GetDesktopSigninUrl())
	}
}

func TestClaimBindsTheRequestOnce(t *testing.T) {
	h := newHarness(t)
	id := h.start(t, "").GetRequestId()

	if err := h.claim(t, id); err != nil {
		t.Fatalf("first claim: %v", err)
	}
	if err := h.claim(t, id); status.Code(err) != codes.Aborted {
		t.Fatalf("second claim: want Aborted, got %v", err)
	}

	_, err := h.auth.ClaimDesktopSignIn(t.Context(), authv1.ClaimDesktopSignInRequest_builder{
		RequestId:   h.start(t, "").GetRequestId(),
		ClaimSecret: "wrong",
	}.Build())
	if status.Code(err) != codes.Unauthenticated {
		t.Fatalf("wrong secret: want Unauthenticated, got %v", err)
	}

	_, err = h.auth.ClaimDesktopSignIn(t.Context(), authv1.ClaimDesktopSignInRequest_builder{
		RequestId:   "dsr_nope",
		ClaimSecret: testClaim,
	}.Build())
	if status.Code(err) != codes.NotFound {
		t.Fatalf("unknown request: want NotFound, got %v", err)
	}
}

func TestPollRefusesAForeignVerifierAndKeepsTheRequest(t *testing.T) {
	h := newHarness(t)
	id := h.start(t, "").GetRequestId()

	_, err := h.auth.PollDesktopSignIn(t.Context(), authv1.PollDesktopSignInRequest_builder{
		RequestId:    id,
		ClientId:     testClientID,
		CodeVerifier: "someone-elses-verifier",
	}.Build())
	if status.Code(err) != codes.Unauthenticated {
		t.Fatalf("want Unauthenticated, got %v", err)
	}

	_, err = h.auth.PollDesktopSignIn(t.Context(), authv1.PollDesktopSignInRequest_builder{
		RequestId:    id,
		ClientId:     "another-install",
		CodeVerifier: testVerifier,
	}.Build())
	if status.Code(err) != codes.Unauthenticated {
		t.Fatalf("foreign client id: want Unauthenticated, got %v", err)
	}

	if got := h.poll(t, id).GetStatus(); got != authv1.DesktopSignInStatus_DESKTOP_SIGN_IN_STATUS_PENDING {
		t.Fatalf("status = %v, want PENDING", got)
	}
}

func TestApprovedPollIssuesTokensExactlyOnce(t *testing.T) {
	h := newHarness(t)
	resp := h.start(t, "http://127.0.0.1:54321/callback")
	id := resp.GetRequestId()

	if got := h.poll(t, id).GetStatus(); got != authv1.DesktopSignInStatus_DESKTOP_SIGN_IN_STATUS_PENDING {
		t.Fatalf("status = %v, want PENDING", got)
	}

	if err := h.claim(t, id); err != nil {
		t.Fatalf("claim: %v", err)
	}
	approved, err := h.auth.ApproveDesktopSignIn(t.Context(), authv1.ApproveDesktopSignInRequest_builder{
		RequestId:   id,
		ClaimSecret: testClaim,
	}.Build())
	if err != nil {
		t.Fatalf("approve: %v", err)
	}
	if approved.GetRedirectUri() != "http://127.0.0.1:54321/callback" {
		t.Fatalf("redirect uri = %q", approved.GetRedirectUri())
	}

	first := h.poll(t, id)
	if first.GetStatus() != authv1.DesktopSignInStatus_DESKTOP_SIGN_IN_STATUS_APPROVED {
		t.Fatalf("status = %v, want APPROVED", first.GetStatus())
	}
	if first.GetAccessToken() == "" || first.GetRefreshToken() == "" {
		t.Fatal("approved answer carries no tokens")
	}
	if first.GetUser().GetEmail() != fixtureEmail || first.GetActiveOrgId() != fixtureOrgID {
		t.Fatalf("account = %q / %q", first.GetUser().GetEmail(), first.GetActiveOrgId())
	}
	if first.GetRequiresEmailVerification() {
		t.Fatal("the mock must hand out a confirmed account")
	}

	if got := h.poll(t, id).GetStatus(); got != authv1.DesktopSignInStatus_DESKTOP_SIGN_IN_STATUS_EXPIRED {
		t.Fatalf("second poll = %v, want EXPIRED", got)
	}
}

func TestDeniedRequestPollsDenied(t *testing.T) {
	h := newHarness(t)
	id := h.start(t, "").GetRequestId()

	if err := h.claim(t, id); err != nil {
		t.Fatalf("claim: %v", err)
	}
	if _, err := h.auth.DenyDesktopSignIn(t.Context(), authv1.DenyDesktopSignInRequest_builder{
		RequestId:   id,
		ClaimSecret: testClaim,
	}.Build()); err != nil {
		t.Fatalf("deny: %v", err)
	}

	if got := h.poll(t, id).GetStatus(); got != authv1.DesktopSignInStatus_DESKTOP_SIGN_IN_STATUS_DENIED {
		t.Fatalf("status = %v, want DENIED", got)
	}
}

func TestApproveRequiresAClaimedRequest(t *testing.T) {
	h := newHarness(t)
	id := h.start(t, "").GetRequestId()

	_, err := h.auth.ApproveDesktopSignIn(t.Context(), authv1.ApproveDesktopSignInRequest_builder{
		RequestId:   id,
		ClaimSecret: testClaim,
	}.Build())
	if status.Code(err) != codes.Unauthenticated {
		t.Fatalf("want Unauthenticated, got %v", err)
	}
}

func TestCancelDropsAPendingRequest(t *testing.T) {
	h := newHarness(t)
	id := h.start(t, "").GetRequestId()

	if _, err := h.auth.CancelDesktopSignIn(t.Context(), authv1.CancelDesktopSignInRequest_builder{
		RequestId:    id,
		ClientId:     testClientID,
		CodeVerifier: testVerifier,
	}.Build()); err != nil {
		t.Fatalf("cancel: %v", err)
	}

	if got := h.poll(t, id).GetStatus(); got != authv1.DesktopSignInStatus_DESKTOP_SIGN_IN_STATUS_EXPIRED {
		t.Fatalf("status = %v, want EXPIRED", got)
	}

	// A request nobody filed is not an error to cancel.
	if _, err := h.auth.CancelDesktopSignIn(t.Context(), authv1.CancelDesktopSignInRequest_builder{
		RequestId:    "dsr_nope",
		ClientId:     testClientID,
		CodeVerifier: testVerifier,
	}.Build()); err != nil {
		t.Fatalf("cancel unknown: %v", err)
	}
}

func TestExpiredRequestPollsExpired(t *testing.T) {
	h := newHarness(t)
	id := h.start(t, "").GetRequestId()

	h.srv.mu.Lock()
	h.srv.requests[id].expiresAt = time.Now().Add(-time.Minute)
	h.srv.mu.Unlock()

	if got := h.poll(t, id).GetStatus(); got != authv1.DesktopSignInStatus_DESKTOP_SIGN_IN_STATUS_EXPIRED {
		t.Fatalf("status = %v, want EXPIRED", got)
	}
	if err := h.claim(t, id); status.Code(err) != codes.FailedPrecondition {
		t.Fatalf("claim expired: want FailedPrecondition, got %v", err)
	}
}

func TestUnknownRequestPollsExpiredWithADeadline(t *testing.T) {
	h := newHarness(t)

	resp := h.poll(t, "dsr_never-existed")
	if resp.GetStatus() != authv1.DesktopSignInStatus_DESKTOP_SIGN_IN_STATUS_EXPIRED {
		t.Fatalf("status = %v, want EXPIRED", resp.GetStatus())
	}
}

func TestVerifyGateHoldsTheTokensAndExtendsTheDeadline(t *testing.T) {
	h := newHarness(t)
	id := h.start(t, "").GetRequestId()
	original := h.poll(t, id).GetExpiresAt().AsTime()

	if code, _ := h.post(t, "/__mode", map[string]bool{"verifyGate": true}); code != http.StatusNoContent {
		t.Fatalf("set mode: %d", code)
	}
	if err := h.claim(t, id); err != nil {
		t.Fatalf("claim: %v", err)
	}
	if _, err := h.auth.ApproveDesktopSignIn(t.Context(), authv1.ApproveDesktopSignInRequest_builder{
		RequestId:   id,
		ClaimSecret: testClaim,
	}.Build()); err != nil {
		t.Fatalf("approve: %v", err)
	}

	for attempt := range verifyGatePolls {
		resp := h.poll(t, id)
		if resp.GetStatus() != authv1.DesktopSignInStatus_DESKTOP_SIGN_IN_STATUS_PENDING {
			t.Fatalf("poll %d = %v, want PENDING", attempt, resp.GetStatus())
		}
		if !resp.GetEmailVerificationPending() {
			t.Fatalf("poll %d carries no email_verification_pending", attempt)
		}
		if !resp.GetExpiresAt().AsTime().After(original) {
			t.Fatalf("poll %d did not extend the deadline", attempt)
		}
	}

	final := h.poll(t, id)
	if final.GetStatus() != authv1.DesktopSignInStatus_DESKTOP_SIGN_IN_STATUS_APPROVED {
		t.Fatalf("final poll = %v, want APPROVED", final.GetStatus())
	}
	if final.GetRequiresEmailVerification() {
		t.Fatal("the server gate is already passed by the time tokens are issued")
	}
	if final.GetAccessToken() == "" {
		t.Fatal("no tokens after the gate")
	}
}

func TestCabinetAPIMirrorsTheRPCs(t *testing.T) {
	h := newHarness(t)
	id := h.start(t, "http://127.0.0.1:54321/callback").GetRequestId()

	if code, body := h.post(t, "/api/claim", map[string]string{
		"requestId": id, "claimSecret": "wrong",
	}); code != http.StatusUnauthorized || body["error"] == nil {
		t.Fatalf("wrong secret: %d %v", code, body)
	}

	if code, _ := h.post(t, "/api/claim", map[string]string{
		"requestId": id, "claimSecret": testClaim,
	}); code != http.StatusOK {
		t.Fatalf("claim: %d", code)
	}

	code, body := h.post(t, "/api/approve", map[string]string{"requestId": id, "claimSecret": testClaim})
	if code != http.StatusOK {
		t.Fatalf("approve: %d %v", code, body)
	}
	if body["redirectUri"] != "http://127.0.0.1:54321/callback" {
		t.Fatalf("redirect uri = %v", body["redirectUri"])
	}
}

func TestNoRedirectModeSilencesTheRedirect(t *testing.T) {
	h := newHarness(t)

	if code, _ := h.post(t, "/__mode", map[string]bool{"noRedirect": true}); code != http.StatusNoContent {
		t.Fatalf("set mode: %d", code)
	}

	id := h.start(t, "http://127.0.0.1:54321/callback").GetRequestId()
	if err := h.claim(t, id); err != nil {
		t.Fatalf("claim: %v", err)
	}
	approved, err := h.auth.ApproveDesktopSignIn(t.Context(), authv1.ApproveDesktopSignInRequest_builder{
		RequestId:   id,
		ClaimSecret: testClaim,
	}.Build())
	if err != nil {
		t.Fatalf("approve: %v", err)
	}
	if approved.GetRedirectUri() != "" {
		t.Fatalf("redirect uri = %q, want empty", approved.GetRedirectUri())
	}
}

func TestResetForgetsTheRequests(t *testing.T) {
	h := newHarness(t)
	id := h.start(t, "").GetRequestId()

	if code, _ := h.post(t, "/__reset", map[string]any{}); code != http.StatusNoContent {
		t.Fatalf("reset: %d", code)
	}

	if got := h.poll(t, id).GetStatus(); got != authv1.DesktopSignInStatus_DESKTOP_SIGN_IN_STATUS_EXPIRED {
		t.Fatalf("status = %v, want EXPIRED", got)
	}
}

func TestCabinetPageCarriesTheConsentControls(t *testing.T) {
	h := newHarness(t)

	resp, err := h.http.Client().Get(h.http.URL + "/desktop-signin?request=dsr_x")
	if err != nil {
		t.Fatalf("get page: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body := make([]byte, 8192)
	n, _ := resp.Body.Read(body)
	page := string(body[:n])
	for _, want := range []string{`id="approve"`, `id="deny"`, "location.hash", "history.replaceState"} {
		if !strings.Contains(page, want) {
			t.Fatalf("page carries no %q", want)
		}
	}
}

func TestConcurrentClaimApproveAndPoll(t *testing.T) {
	h := newHarness(t)
	id := h.start(t, "http://127.0.0.1:54321/callback").GetRequestId()

	const workers = 8

	var (
		mu       sync.Mutex
		claims   int
		approves int
		tokens   int
	)

	var wg sync.WaitGroup
	for range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := h.claim(t, id); err == nil {
				mu.Lock()
				claims++
				mu.Unlock()
			}
		}()
	}
	wg.Wait()

	for range workers {
		wg.Add(2)
		go func() {
			defer wg.Done()
			_, err := h.auth.ApproveDesktopSignIn(t.Context(), authv1.ApproveDesktopSignInRequest_builder{
				RequestId:   id,
				ClaimSecret: testClaim,
			}.Build())
			if err == nil {
				mu.Lock()
				approves++
				mu.Unlock()
			}
		}()
		go func() {
			defer wg.Done()
			resp, err := h.auth.PollDesktopSignIn(t.Context(), authv1.PollDesktopSignInRequest_builder{
				RequestId:    id,
				ClientId:     testClientID,
				CodeVerifier: testVerifier,
			}.Build())
			if err == nil && resp.GetAccessToken() != "" {
				mu.Lock()
				tokens++
				mu.Unlock()
			}
		}()
	}
	wg.Wait()

	if h.poll(t, id).GetAccessToken() != "" {
		tokens++
	}

	if claims != 1 {
		t.Fatalf("successful claims = %d, want 1", claims)
	}
	if approves != 1 {
		t.Fatalf("successful approvals = %d, want 1", approves)
	}
	if tokens != 1 {
		t.Fatalf("token issuances = %d, want 1", tokens)
	}
}
