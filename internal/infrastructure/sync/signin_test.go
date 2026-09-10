package sync

import (
	"context"
	"errors"
	"io"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	authv1 "github.com/tetiva-app/proto/go/gophercourier/auth/v1"

	"github.com/tetiva-app/client/internal/domain/usecase/auth"
)

func signInParams() auth.SignInParams {
	return auth.SignInParams{
		ServerURL:  "sync.example.com:443",
		ClientID:   "client-1",
		DeviceName: "Mac",
		Platform:   "darwin",
		AppVersion: "0.17.0",
		Locale:     "ru",
		Intent:     "register",
	}
}

func TestGRPCClient_GetServerInfo_MapsAnswer(t *testing.T) {
	stub := &stubAuthClient{serverInfoResp: authv1.GetServerInfoResponse_builder{
		ServerVersion:    "0.20.0",
		Capabilities:     []string{"password_reset", "desktop_signin"},
		DesktopSigninUrl: "https://app.tetiva.app/desktop-signin",
		RegistrationOpen: true,
	}.Build()}

	info, err := NewGRPCClientWithStubs(stub, nil).GetServerInfo(context.Background())
	require.NoError(t, err)

	assert.Equal(t, "0.20.0", info.Version)
	assert.True(t, info.DesktopSignIn)
	assert.Equal(t, "https://app.tetiva.app/desktop-signin", info.DesktopSignInURL)
	assert.True(t, info.RegistrationOpen)
}

func TestGRPCClient_GetServerInfo_WithoutCapability(t *testing.T) {
	stub := &stubAuthClient{serverInfoResp: authv1.GetServerInfoResponse_builder{
		ServerVersion: "0.19.0",
		Capabilities:  []string{"password_reset"},
	}.Build()}

	info, err := NewGRPCClientWithStubs(stub, nil).GetServerInfo(context.Background())
	require.NoError(t, err)
	assert.False(t, info.DesktopSignIn)
}

func TestGRPCClient_GetServerInfo_UnimplementedIsNotAnError(t *testing.T) {
	stub := &stubAuthClient{serverInfoErr: status.Error(codes.Unimplemented, "unknown method")}

	_, err := NewGRPCClientWithStubs(stub, nil).GetServerInfo(context.Background())
	assert.ErrorIs(t, err, auth.ErrServerInfoUnsupported)
}

func TestGRPCClient_StartDesktopSignIn_SendsEveryField(t *testing.T) {
	expires := time.Now().Add(10 * time.Minute).Truncate(time.Second)
	stub := &stubAuthClient{startResp: authv1.StartDesktopSignInResponse_builder{
		RequestId:           "req-1",
		LoginUrl:            "https://app.tetiva.app/desktop-signin?request_id=req-1",
		ExpiresAt:           timestamppb.New(expires),
		PollIntervalSeconds: 5,
	}.Build()}

	start, err := NewGRPCClientWithStubs(stub, nil).
		StartDesktopSignIn(context.Background(), signInParams(), "http://127.0.0.1:51234/callback", "challenge", "claim")
	require.NoError(t, err)

	req := stub.startReq
	require.NotNil(t, req)
	assert.Equal(t, "client-1", req.GetClientId())
	assert.Equal(t, "challenge", req.GetCodeChallenge())
	assert.Equal(t, "Mac", req.GetDeviceName())
	assert.Equal(t, "darwin", req.GetPlatform())
	assert.Equal(t, "0.17.0", req.GetAppVersion())
	assert.Equal(t, "http://127.0.0.1:51234/callback", req.GetRedirectUri())
	assert.Equal(t, "ru", req.GetLocale())
	assert.Equal(t, "register", req.GetIntent())
	assert.Equal(t, "claim", req.GetClaimChallenge())

	assert.Equal(t, "req-1", start.RequestID)
	assert.Equal(t, "https://app.tetiva.app/desktop-signin?request_id=req-1", start.LoginURL)
	assert.True(t, start.ExpiresAt.Equal(expires))
	assert.Equal(t, 5*time.Second, start.PollInterval)
}

func TestGRPCClient_StartDesktopSignIn_NoIntervalNoDeadline(t *testing.T) {
	stub := &stubAuthClient{startResp: authv1.StartDesktopSignInResponse_builder{
		RequestId: "req-1",
		LoginUrl:  "https://app.tetiva.app/desktop-signin",
	}.Build()}

	start, err := NewGRPCClientWithStubs(stub, nil).
		StartDesktopSignIn(context.Background(), signInParams(), "", "challenge", "claim")
	require.NoError(t, err)

	assert.Zero(t, start.PollInterval, "the manager falls back to its own interval")
	assert.True(t, start.ExpiresAt.IsZero())
}

func TestGRPCClient_PollDesktopSignIn_Approved(t *testing.T) {
	expires := time.Now().Add(30 * time.Minute).Truncate(time.Second)
	stub := &stubAuthClient{pollResp: authv1.PollDesktopSignInResponse_builder{
		Status:                    authv1.DesktopSignInStatus_DESKTOP_SIGN_IN_STATUS_APPROVED,
		AccessToken:               "access-1",
		RefreshToken:              "refresh-1",
		User:                      authv1.User_builder{Email: "a@b.c"}.Build(),
		ActiveOrgId:               "org-1",
		RequiresEmailVerification: true,
		ExpiresAt:                 timestamppb.New(expires),
	}.Build()}

	res, err := NewGRPCClientWithStubs(stub, nil).
		PollDesktopSignIn(context.Background(), "req-1", "client-1", "verifier")
	require.NoError(t, err)

	require.NotNil(t, stub.pollReq)
	assert.Equal(t, "req-1", stub.pollReq.GetRequestId())
	assert.Equal(t, "client-1", stub.pollReq.GetClientId())
	assert.Equal(t, "verifier", stub.pollReq.GetCodeVerifier())

	assert.Equal(t, auth.SignInApproved, res.Status)
	require.NotNil(t, res.Tokens)
	assert.Equal(t, "access-1", res.Tokens.AccessToken)
	assert.Equal(t, "refresh-1", res.Tokens.RefreshToken)
	assert.Equal(t, "org-1", res.Tokens.ActiveOrgID)
	assert.Equal(t, "a@b.c", res.Tokens.Email)
	assert.True(t, res.Tokens.RequiresEmailVerification)
	assert.True(t, res.ExpiresAt.Equal(expires))
}

func TestGRPCClient_PollDesktopSignIn_NonApprovedStatuses(t *testing.T) {
	cases := []struct {
		name string
		in   authv1.DesktopSignInStatus
		want auth.SignInStatusCode
	}{
		{"pending", authv1.DesktopSignInStatus_DESKTOP_SIGN_IN_STATUS_PENDING, auth.SignInPending},
		{"unspecified", authv1.DesktopSignInStatus_DESKTOP_SIGN_IN_STATUS_UNSPECIFIED, auth.SignInPending},
		{"denied", authv1.DesktopSignInStatus_DESKTOP_SIGN_IN_STATUS_DENIED, auth.SignInDenied},
		{"expired", authv1.DesktopSignInStatus_DESKTOP_SIGN_IN_STATUS_EXPIRED, auth.SignInExpired},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			stub := &stubAuthClient{pollResp: authv1.PollDesktopSignInResponse_builder{
				Status:                   tc.in,
				EmailVerificationPending: true,
			}.Build()}

			res, err := NewGRPCClientWithStubs(stub, nil).
				PollDesktopSignIn(context.Background(), "req-1", "client-1", "verifier")
			require.NoError(t, err)

			assert.Equal(t, tc.want, res.Status)
			assert.True(t, res.EmailVerificationPending)
			assert.Nil(t, res.Tokens)
		})
	}
}

// A status this build does not know must not be read as pending: the manager
// ends the flow on it.
func TestGRPCClient_PollDesktopSignIn_UnknownStatus(t *testing.T) {
	stub := &stubAuthClient{pollResp: authv1.PollDesktopSignInResponse_builder{
		Status: authv1.DesktopSignInStatus(42),
	}.Build()}

	res, err := NewGRPCClientWithStubs(stub, nil).
		PollDesktopSignIn(context.Background(), "req-1", "client-1", "verifier")
	require.NoError(t, err)

	assert.NotEqual(t, auth.SignInPending, res.Status)
	assert.NotEqual(t, auth.SignInApproved, res.Status)
}

func TestGRPCClient_PollDesktopSignIn_ApprovedWithoutTokens(t *testing.T) {
	cases := map[string]authv1.PollDesktopSignInResponse_builder{
		"no access token": {
			Status:       authv1.DesktopSignInStatus_DESKTOP_SIGN_IN_STATUS_APPROVED,
			RefreshToken: "refresh-1",
		},
		"no refresh token": {
			Status:      authv1.DesktopSignInStatus_DESKTOP_SIGN_IN_STATUS_APPROVED,
			AccessToken: "access-1",
		},
	}

	for name, b := range cases {
		t.Run(name, func(t *testing.T) {
			stub := &stubAuthClient{pollResp: b.Build()}

			_, err := NewGRPCClientWithStubs(stub, nil).
				PollDesktopSignIn(context.Background(), "req-1", "client-1", "verifier")
			assert.ErrorIs(t, err, auth.ErrSignInMalformed)
		})
	}
}

func TestPollFailure_Classification(t *testing.T) {
	unauthenticated := status.Error(codes.Unauthenticated, "bad verifier")
	invalid := status.Error(codes.InvalidArgument, "no request id")

	cases := []struct {
		name string
		in   error
		want error
	}{
		{"unavailable", status.Error(codes.Unavailable, "connection refused"), auth.ErrSignInTransient},
		{"deadline exceeded", status.Error(codes.DeadlineExceeded, "timeout"), auth.ErrSignInTransient},
		{"network", io.EOF, auth.ErrSignInTransient},
		{"rate limited", status.Error(codes.ResourceExhausted, "slow down"), auth.ErrSignInThrottled},
		{"unauthenticated", unauthenticated, unauthenticated},
		{"invalid argument", invalid, invalid},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.ErrorIs(t, pollFailure(tc.in), tc.want)
		})
	}

	assert.NoError(t, pollFailure(nil))
	assert.NotErrorIs(t, pollFailure(unauthenticated), auth.ErrSignInTransient)
}

func TestGRPCClient_PollDesktopSignIn_TransportErrorIsClassified(t *testing.T) {
	stub := &stubAuthClient{pollErr: status.Error(codes.ResourceExhausted, "slow down")}

	_, err := NewGRPCClientWithStubs(stub, nil).
		PollDesktopSignIn(context.Background(), "req-1", "client-1", "verifier")
	assert.ErrorIs(t, err, auth.ErrSignInThrottled)
}

func TestGRPCClient_CancelDesktopSignIn(t *testing.T) {
	stub := &stubAuthClient{}

	require.NoError(t, NewGRPCClientWithStubs(stub, nil).
		CancelDesktopSignIn(context.Background(), "req-1", "client-1", "verifier"))

	require.NotNil(t, stub.cancelReq)
	assert.Equal(t, "req-1", stub.cancelReq.GetRequestId())
	assert.Equal(t, "client-1", stub.cancelReq.GetClientId())
	assert.Equal(t, "verifier", stub.cancelReq.GetCodeVerifier())

	stub.cancelErr = errors.New("boom")
	assert.Error(t, NewGRPCClientWithStubs(stub, nil).
		CancelDesktopSignIn(context.Background(), "req-1", "client-1", "verifier"))
}

func TestGRPCClient_Logout_SendsBearer(t *testing.T) {
	stub := &stubAuthClient{}

	require.NoError(t, NewGRPCClientWithStubs(stub, nil).Logout(context.Background(), "access-1"))

	assert.Equal(t, 1, stub.logoutCalls)
	require.Len(t, stub.authHeaders, 1)
	assert.Equal(t, "Bearer access-1", stub.authHeaders[0])
}

func TestGRPCClient_ImplementsSignInClient(t *testing.T) {
	var _ auth.SignInClient = NewGRPCClientWithStubs(&stubAuthClient{}, nil)
}
