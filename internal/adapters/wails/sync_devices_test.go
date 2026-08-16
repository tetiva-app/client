package wails

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/timestamppb"

	authv1 "github.com/tetiva-app/proto/go/gophercourier/auth/v1"

	"github.com/tetiva-app/client/internal/adapters/wails/dto"
)

func connectedFixture(t *testing.T) *verificationFixture {
	t.Helper()
	f := newVerificationFixture(t)
	f.seedSession(t)
	f.svc.grpcClient = f.client
	return f
}

func TestSyncService_ListSessions_MapsDevices(t *testing.T) {
	f := connectedFixture(t)
	lastUsed := time.Date(2026, 8, 15, 10, 0, 0, 0, time.UTC)
	f.authStub.meResp = authv1.MeResponse_builder{
		Sessions: []*authv1.SessionView{
			authv1.SessionView_builder{
				Id: "sess_1", ClientId: "client-1", UserAgent: "Tetiva/0.17.0 (darwin; mac) grpc-go/1.79.3",
				Ip: "1.2.3.4", LastUsedAt: timestamppb.New(lastUsed), IsCurrent: true,
			}.Build(),
			authv1.SessionView_builder{Id: "sess_2", ClientId: "client-2"}.Build(),
		},
	}.Build()

	res := f.svc.ListSessions()

	require.Nil(t, res.Error)
	require.Len(t, res.Data, 2)
	assert.Equal(t, dto.SessionInfo{
		ID: "sess_1", ClientID: "client-1", UserAgent: "Tetiva/0.17.0 (darwin; mac) grpc-go/1.79.3",
		IP: "1.2.3.4", LastUsedAt: "2026-08-15T10:00:00Z", IsCurrent: true,
	}, res.Data[0])
	assert.Empty(t, res.Data[1].LastUsedAt)
}

func TestSyncService_Devices_NotConnected(t *testing.T) {
	svc := &SyncService{}

	errs := []*ResultError{
		svc.ListSessions().Error,
		svc.RevokeSession(dto.RevokeSessionRequest{SessionID: "sess_2"}).Error,
		svc.LogoutAll().Error,
	}

	for _, e := range errs {
		require.NotNil(t, e)
		assert.Equal(t, ErrCodeNotConnected, e.Code)
		assert.Contains(t, e.Message, "not connected to sync server")
	}
}

func TestSyncService_RevokeSession_SendsSessionID(t *testing.T) {
	f := connectedFixture(t)

	res := f.svc.RevokeSession(dto.RevokeSessionRequest{SessionID: "sess_2"})

	require.Nil(t, res.Error)
	assert.Equal(t, []string{"sess_2"}, f.authStub.revokedIDs)
}

func TestSyncService_LogoutAll_ReturnsRevokedCount(t *testing.T) {
	f := connectedFixture(t)
	f.authStub.logoutAllN = 2

	res := f.svc.LogoutAll()

	require.Nil(t, res.Error)
	assert.Equal(t, 2, res.Data.RevokedCount)
}

func TestSyncService_Logout_RevokesServerSession(t *testing.T) {
	f := connectedFixture(t)

	require.Nil(t, f.svc.Logout().Error)

	assert.Equal(t, 1, f.authStub.logoutCalls)
}
