package wails

import (
	"context"
	"sync"

	"google.golang.org/grpc"

	authv1 "github.com/tetiva-app/proto/go/gophercourier/auth/v1"
	workspacev1 "github.com/tetiva-app/proto/go/gophercourier/workspace/v1"
)

// stubAuthClient embeds the generated interface so only the RPCs a test
// exercises need bodies.
type stubAuthClient struct {
	authv1.AuthServiceClient

	mu           sync.Mutex
	registerResp *authv1.RegisterResponse
	loginResp    *authv1.LoginResponse
	getMeResp    *authv1.GetMeResponse
	getMeErr     error
	refreshResp  *authv1.RefreshResponse
	refreshErr   error
	resendErr    error
	infoResp     *authv1.GetServerInfoResponse
	infoErr      error
	meResp       *authv1.MeResponse
	logoutAllN   int32

	logoutCalls int
	revokedIDs  []string
}

func (s *stubAuthClient) setGetMe(resp *authv1.GetMeResponse, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.getMeResp, s.getMeErr = resp, err
}

func (s *stubAuthClient) setRefreshErr(err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.refreshErr = err
}

func (s *stubAuthClient) setRefresh(resp *authv1.RefreshResponse) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.refreshResp = resp
}

func (s *stubAuthClient) Refresh(context.Context, *authv1.RefreshRequest, ...grpc.CallOption) (*authv1.RefreshResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.refreshErr != nil {
		return nil, s.refreshErr
	}
	if s.refreshResp != nil {
		return s.refreshResp, nil
	}
	return authv1.RefreshResponse_builder{
		AccessToken: "access-2", RefreshToken: "refresh-2", ActiveOrgId: "org-1",
	}.Build(), nil
}

func (s *stubAuthClient) setServerInfo(resp *authv1.GetServerInfoResponse, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.infoResp, s.infoErr = resp, err
}

func (s *stubAuthClient) GetServerInfo(context.Context, *authv1.GetServerInfoRequest, ...grpc.CallOption) (*authv1.GetServerInfoResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.infoResp, s.infoErr
}

func (s *stubAuthClient) Register(context.Context, *authv1.RegisterRequest, ...grpc.CallOption) (*authv1.RegisterResponse, error) {
	return s.registerResp, nil
}

func (s *stubAuthClient) Login(context.Context, *authv1.LoginRequest, ...grpc.CallOption) (*authv1.LoginResponse, error) {
	return s.loginResp, nil
}

func (s *stubAuthClient) GetMe(context.Context, *authv1.GetMeRequest, ...grpc.CallOption) (*authv1.GetMeResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.getMeResp, s.getMeErr
}

func (s *stubAuthClient) ResendVerification(context.Context, *authv1.ResendVerificationRequest, ...grpc.CallOption) (*authv1.ResendVerificationResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.resendErr != nil {
		return nil, s.resendErr
	}
	return authv1.ResendVerificationResponse_builder{}.Build(), nil
}

// stubWorkspaceClient counts ListByOrg calls: workspace discovery runs exactly
// once per enableSync, which makes it the probe for "sync was enabled N times".
type stubWorkspaceClient struct {
	workspacev1.WorkspaceServiceClient

	mu          sync.Mutex
	listCalls   int
	remoteID    string
	createID    string
	createErr   error
	createName  string
	createOrgID string
}

func (s *stubWorkspaceClient) ListByOrg(context.Context, *workspacev1.ListByOrgRequest, ...grpc.CallOption) (*workspacev1.ListByOrgResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.listCalls++
	return workspacev1.ListByOrgResponse_builder{
		Workspaces: []*workspacev1.Workspace{
			workspacev1.Workspace_builder{Id: s.remoteID, Name: "Remote"}.Build(),
		},
	}.Build(), nil
}

func (s *stubWorkspaceClient) Create(_ context.Context, req *workspacev1.CreateRequest, _ ...grpc.CallOption) (*workspacev1.CreateResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.createName = req.GetName()
	s.createOrgID = req.GetOrgId()
	if s.createErr != nil {
		return nil, s.createErr
	}
	id := s.createID
	if id == "" {
		id = "remote-linked"
	}
	return workspacev1.CreateResponse_builder{
		Workspace: workspacev1.Workspace_builder{Id: id, Name: req.GetName()}.Build(),
	}.Build(), nil
}

func (s *stubWorkspaceClient) setCreate(id string, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.createID, s.createErr = id, err
}

func (s *stubWorkspaceClient) createdName() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.createName
}

func (s *stubWorkspaceClient) createdOrgID() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.createOrgID
}

func (s *stubWorkspaceClient) calls() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.listCalls
}

func (s *stubAuthClient) Me(context.Context, *authv1.MeRequest, ...grpc.CallOption) (*authv1.MeResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.meResp, nil
}

func (s *stubAuthClient) Logout(context.Context, *authv1.LogoutRequest, ...grpc.CallOption) (*authv1.LogoutResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.logoutCalls++
	return authv1.LogoutResponse_builder{}.Build(), nil
}

func (s *stubAuthClient) RevokeSession(_ context.Context, req *authv1.RevokeSessionRequest, _ ...grpc.CallOption) (*authv1.RevokeSessionResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.revokedIDs = append(s.revokedIDs, req.GetSessionId())
	return authv1.RevokeSessionResponse_builder{}.Build(), nil
}

func (s *stubAuthClient) LogoutAll(context.Context, *authv1.LogoutAllRequest, ...grpc.CallOption) (*authv1.LogoutAllResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return authv1.LogoutAllResponse_builder{RevokedCount: s.logoutAllN}.Build(), nil
}
