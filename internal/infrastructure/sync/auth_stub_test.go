package sync

import (
	"context"
	"strings"

	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"

	authv1 "github.com/tetiva-app/proto/go/gophercourier/auth/v1"
)

// stubAuthClient embeds the generated interface so only the RPCs a test
// exercises need bodies; anything else panics on a nil embedded value.
type stubAuthClient struct {
	authv1.AuthServiceClient

	registerResp *authv1.RegisterResponse
	loginResp    *authv1.LoginResponse
	getMeResp    *authv1.GetMeResponse
	getMeErr     error
	resendErr    error
	meResp       *authv1.MeResponse
	refreshResp  *authv1.RefreshResponse
	logoutErr    error
	logoutAllN   int32

	authHeaders  []string
	resendCalls  int
	logoutCalls  int
	revokedIDs   []string
	logoutAllRun int
}

func (s *stubAuthClient) recordAuth(ctx context.Context) {
	md, _ := metadata.FromOutgoingContext(ctx)
	s.authHeaders = append(s.authHeaders, strings.Join(md.Get("authorization"), ""))
}

func (s *stubAuthClient) Register(ctx context.Context, _ *authv1.RegisterRequest, _ ...grpc.CallOption) (*authv1.RegisterResponse, error) {
	s.recordAuth(ctx)
	return s.registerResp, nil
}

func (s *stubAuthClient) Login(ctx context.Context, _ *authv1.LoginRequest, _ ...grpc.CallOption) (*authv1.LoginResponse, error) {
	s.recordAuth(ctx)
	return s.loginResp, nil
}

func (s *stubAuthClient) GetMe(ctx context.Context, _ *authv1.GetMeRequest, _ ...grpc.CallOption) (*authv1.GetMeResponse, error) {
	s.recordAuth(ctx)
	return s.getMeResp, s.getMeErr
}

func (s *stubAuthClient) ResendVerification(ctx context.Context, _ *authv1.ResendVerificationRequest, _ ...grpc.CallOption) (*authv1.ResendVerificationResponse, error) {
	s.recordAuth(ctx)
	s.resendCalls++
	if s.resendErr != nil {
		return nil, s.resendErr
	}
	return authv1.ResendVerificationResponse_builder{}.Build(), nil
}

func (s *stubAuthClient) Refresh(ctx context.Context, _ *authv1.RefreshRequest, _ ...grpc.CallOption) (*authv1.RefreshResponse, error) {
	s.recordAuth(ctx)
	return s.refreshResp, nil
}

func (s *stubAuthClient) Me(ctx context.Context, _ *authv1.MeRequest, _ ...grpc.CallOption) (*authv1.MeResponse, error) {
	s.recordAuth(ctx)
	return s.meResp, nil
}

func (s *stubAuthClient) Logout(ctx context.Context, _ *authv1.LogoutRequest, _ ...grpc.CallOption) (*authv1.LogoutResponse, error) {
	s.recordAuth(ctx)
	s.logoutCalls++
	if s.logoutErr != nil {
		return nil, s.logoutErr
	}
	return authv1.LogoutResponse_builder{}.Build(), nil
}

func (s *stubAuthClient) RevokeSession(ctx context.Context, req *authv1.RevokeSessionRequest, _ ...grpc.CallOption) (*authv1.RevokeSessionResponse, error) {
	s.recordAuth(ctx)
	s.revokedIDs = append(s.revokedIDs, req.GetSessionId())
	return authv1.RevokeSessionResponse_builder{}.Build(), nil
}

func (s *stubAuthClient) LogoutAll(ctx context.Context, _ *authv1.LogoutAllRequest, _ ...grpc.CallOption) (*authv1.LogoutAllResponse, error) {
	s.recordAuth(ctx)
	s.logoutAllRun++
	return authv1.LogoutAllResponse_builder{RevokedCount: s.logoutAllN}.Build(), nil
}
