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

	authHeaders []string
	resendCalls int
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
