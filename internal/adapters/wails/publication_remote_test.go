package wails

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	publicationv1 "github.com/tetiva-app/proto/go/gophercourier/publication/v1"

	syncsvc "github.com/tetiva-app/client/internal/infrastructure/sync"
)

type stubPublicationClient struct {
	publicationv1.PublicationServiceClient

	mu          sync.Mutex
	authHeaders []string
	publishErr  error
}

func (s *stubPublicationClient) Publish(ctx context.Context, in *publicationv1.PublishRequest, _ ...grpc.CallOption) (*publicationv1.PublishResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	md, _ := metadata.FromOutgoingContext(ctx)
	s.authHeaders = append(s.authHeaders, strings.Join(md.Get("authorization"), ""))
	if s.publishErr != nil {
		return nil, s.publishErr
	}
	return publicationv1.PublishResponse_builder{Publication: publicationv1.Publication_builder{
		Id: "pub-1", CollectionId: in.GetCollectionId(),
	}.Build()}.Build(), nil
}

func (s *stubPublicationClient) calls() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]string(nil), s.authHeaders...)
}

func publishRequest() *publicationv1.PublishRequest {
	return publicationv1.PublishRequest_builder{CollectionId: "c-1"}.Build()
}

func TestPublicationRemote_FollowsTheSyncServiceConnection(t *testing.T) {
	f := newVerificationFixture(t)
	ctx := context.Background()
	first := &stubPublicationClient{}
	f.svc.newClient = func(string) (*syncsvc.GRPCClient, error) {
		return syncsvc.NewGRPCClientWithPublication(f.authStub, f.wsStub, first), nil
	}
	remote := syncsvc.NewPublicationRemote(f.svc, f.auth)

	_, err := remote.Publish(ctx, publishRequest())
	require.ErrorIs(t, err, ErrNotConnected, "signed out")
	assert.Empty(t, first.calls())

	f.seedSession(t)
	pub, err := remote.Publish(ctx, publishRequest())
	require.NoError(t, err)
	assert.Equal(t, "pub-1", pub.GetId())
	assert.Equal(t, []string{"Bearer access-1"}, first.calls())

	second := &stubPublicationClient{}
	f.svc.setClient(syncsvc.NewGRPCClientWithPublication(f.authStub, f.wsStub, second))
	_, err = remote.Publish(ctx, publishRequest())
	require.NoError(t, err)
	assert.Len(t, second.calls(), 1, "a reconnect moves the calls to the new client")
	assert.Len(t, first.calls(), 1)

	require.Nil(t, f.svc.Logout().Error)
	_, err = remote.Publish(ctx, publishRequest())
	require.Error(t, err, "no token after sign-out")
	assert.Len(t, second.calls(), 1)
}

func TestResultReasonFixtureIsCurrent(t *testing.T) {
	f := newVerificationFixture(t)
	st, err := status.New(codes.ResourceExhausted, "quota").
		WithDetails(&errdetails.ErrorInfo{Reason: "PUBLISH_QUOTA_EXCEEDED"})
	require.NoError(t, err)
	client := syncsvc.NewGRPCClientWithPublication(f.authStub, f.wsStub, &stubPublicationClient{publishErr: st.Err()})
	f.svc.newClient = func(string) (*syncsvc.GRPCClient, error) { return client, nil }
	f.seedSession(t)

	_, err = syncsvc.NewPublicationRemote(f.svc, f.auth).Publish(context.Background(), publishRequest())
	require.Error(t, err)

	got, err := json.MarshalIndent(Err[Empty](err), "", "  ")
	require.NoError(t, err)
	got = append(got, '\n')

	path := filepath.Join("testdata", "result_reason.json")
	if os.Getenv("UPDATE_FIXTURES") == "1" {
		require.NoError(t, os.MkdirAll("testdata", 0o755))
		require.NoError(t, os.WriteFile(path, got, 0o644))
		return
	}
	want, err := os.ReadFile(path)
	require.NoError(t, err, "run with UPDATE_FIXTURES=1 to create %s", path)
	require.True(t, bytes.Equal(want, got), "%s is stale; rerun with UPDATE_FIXTURES=1\n%s", path, got)
}
