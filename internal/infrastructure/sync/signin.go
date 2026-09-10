package sync

import (
	"context"
	"fmt"
	"slices"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	authv1 "github.com/tetiva-app/proto/go/gophercourier/auth/v1"

	"github.com/tetiva-app/client/internal/domain/usecase/auth"
)

var _ auth.SignInClient = (*GRPCClient)(nil)

// desktopSignInCapability is the feature name a server advertises when its
// cabinet can complete a sign-in for this app.
const desktopSignInCapability = "desktop_signin"

// GetServerInfo asks what this server offers before anyone signs in. A server
// that predates the RPC answers Unimplemented, which is not an error here.
func (c *GRPCClient) GetServerInfo(ctx context.Context) (auth.ServerInfo, error) {
	const funcName = "GRPCClient.GetServerInfo"

	resp, err := c.auth.GetServerInfo(ctx, authv1.GetServerInfoRequest_builder{}.Build())
	if err != nil {
		if status.Code(err) == codes.Unimplemented {
			return auth.ServerInfo{}, fmt.Errorf("%s: %w", funcName, auth.ErrServerInfoUnsupported)
		}
		return auth.ServerInfo{}, fmt.Errorf("%s: get server info rpc: %w", funcName, err)
	}

	return auth.ServerInfo{
		Version:          resp.GetServerVersion(),
		DesktopSignIn:    slices.Contains(resp.GetCapabilities(), desktopSignInCapability),
		DesktopSignInURL: resp.GetDesktopSigninUrl(),
		RegistrationOpen: resp.GetRegistrationOpen(),
	}, nil
}

// StartDesktopSignIn files the request the browser is about to approve.
func (c *GRPCClient) StartDesktopSignIn(ctx context.Context, p auth.SignInParams,
	redirectURI, codeChallenge, claimChallenge string,
) (auth.SignInStart, error) {
	const funcName = "GRPCClient.StartDesktopSignIn"

	req := authv1.StartDesktopSignInRequest_builder{
		ClientId:       p.ClientID,
		CodeChallenge:  codeChallenge,
		DeviceName:     p.DeviceName,
		Platform:       p.Platform,
		AppVersion:     p.AppVersion,
		RedirectUri:    redirectURI,
		Locale:         p.Locale,
		Intent:         p.Intent,
		ClaimChallenge: claimChallenge,
	}.Build()

	resp, err := c.auth.StartDesktopSignIn(ctx, req)
	if err != nil {
		return auth.SignInStart{}, fmt.Errorf("%s: start desktop sign-in rpc: %w", funcName, err)
	}

	start := auth.SignInStart{
		RequestID:    resp.GetRequestId(),
		LoginURL:     resp.GetLoginUrl(),
		PollInterval: time.Duration(resp.GetPollIntervalSeconds()) * time.Second,
	}
	if resp.HasExpiresAt() {
		start.ExpiresAt = resp.GetExpiresAt().AsTime()
	}
	return start, nil
}

// PollDesktopSignIn maps the transport's retryable statuses onto the manager's
// sentinels, so the domain never sees a gRPC code. APPROVED without tokens is malformed.
func (c *GRPCClient) PollDesktopSignIn(ctx context.Context, requestID, clientID, codeVerifier string) (auth.SignInPoll, error) {
	const funcName = "GRPCClient.PollDesktopSignIn"

	req := authv1.PollDesktopSignInRequest_builder{
		RequestId:    requestID,
		ClientId:     clientID,
		CodeVerifier: codeVerifier,
	}.Build()

	resp, err := c.auth.PollDesktopSignIn(ctx, req)
	if err != nil {
		return auth.SignInPoll{}, pollFailure(err)
	}

	res := auth.SignInPoll{
		Status:                   signInStatus(resp.GetStatus()),
		EmailVerificationPending: resp.GetEmailVerificationPending(),
	}
	if resp.HasExpiresAt() {
		res.ExpiresAt = resp.GetExpiresAt().AsTime()
	}

	if res.Status == auth.SignInApproved {
		if resp.GetAccessToken() == "" || resp.GetRefreshToken() == "" {
			return auth.SignInPoll{}, fmt.Errorf("%s: %w", funcName, auth.ErrSignInMalformed)
		}
		res.Tokens = &auth.SignInTokens{
			AccessToken:               resp.GetAccessToken(),
			RefreshToken:              resp.GetRefreshToken(),
			ActiveOrgID:               resp.GetActiveOrgId(),
			Email:                     resp.GetUser().GetEmail(),
			RequiresEmailVerification: resp.GetRequiresEmailVerification(),
		}
	}

	return res, nil
}

// CancelDesktopSignIn drops a pending request instead of leaving it to expire.
func (c *GRPCClient) CancelDesktopSignIn(ctx context.Context, requestID, clientID, codeVerifier string) error {
	const funcName = "GRPCClient.CancelDesktopSignIn"

	req := authv1.CancelDesktopSignInRequest_builder{
		RequestId:    requestID,
		ClientId:     clientID,
		CodeVerifier: codeVerifier,
	}.Build()

	if _, err := c.auth.CancelDesktopSignIn(ctx, req); err != nil {
		return fmt.Errorf("%s: cancel desktop sign-in rpc: %w", funcName, err)
	}
	return nil
}

// Logout revokes one session by its access token; the manager calls it for a
// session the flow decided not to keep.
func (c *GRPCClient) Logout(ctx context.Context, accessToken string) error {
	const funcName = "GRPCClient.Logout"

	if _, err := c.auth.Logout(ContextWithAuth(ctx, accessToken), authv1.LogoutRequest_builder{}.Build()); err != nil {
		return fmt.Errorf("%s: logout rpc: %w", funcName, err)
	}
	return nil
}

// signInStatus maps the protocol enum; unspecified means "still pending", and an
// unknown one this build cannot honour ends the flow.
func signInStatus(s authv1.DesktopSignInStatus) auth.SignInStatusCode {
	switch s {
	case authv1.DesktopSignInStatus_DESKTOP_SIGN_IN_STATUS_UNSPECIFIED,
		authv1.DesktopSignInStatus_DESKTOP_SIGN_IN_STATUS_PENDING:
		return auth.SignInPending
	case authv1.DesktopSignInStatus_DESKTOP_SIGN_IN_STATUS_APPROVED:
		return auth.SignInApproved
	case authv1.DesktopSignInStatus_DESKTOP_SIGN_IN_STATUS_DENIED:
		return auth.SignInDenied
	case authv1.DesktopSignInStatus_DESKTOP_SIGN_IN_STATUS_EXPIRED:
		return auth.SignInExpired
	default:
		return auth.SignInStatusCode(s.String())
	}
}

// pollFailure turns a transport error into the manager's retry vocabulary.
func pollFailure(err error) error {
	if err == nil {
		return nil
	}

	st, ok := status.FromError(err)
	if !ok {
		// No status at all: the connection dropped mid-call.
		return fmt.Errorf("%w: %w", auth.ErrSignInTransient, err)
	}

	switch st.Code() {
	case codes.Unavailable, codes.DeadlineExceeded:
		return fmt.Errorf("%w: %w", auth.ErrSignInTransient, err)
	case codes.ResourceExhausted:
		return fmt.Errorf("%w: %w", auth.ErrSignInThrottled, err)
	}
	return err
}
