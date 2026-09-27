package sync

import (
	"context"
	"fmt"
	"strings"
	gosync "sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	authv1 "github.com/tetiva-app/proto/go/gophercourier/auth/v1"
	publicationv1 "github.com/tetiva-app/proto/go/gophercourier/publication/v1"
	subscriptionv1 "github.com/tetiva-app/proto/go/gophercourier/subscription/v1"

	"github.com/tetiva-app/client/internal/infrastructure/repository/sqlite"
)

type stubPublicationClient struct {
	publicationv1.PublicationServiceClient

	mu          gosync.Mutex
	authHeaders []string
	budgets     []time.Duration
	publishReqs []*publicationv1.PublishRequest
	unpublished []string
	getBatches  [][]string

	publishErr   error
	wasRevoked   bool
	getPubsErrAt int // 1-based call that fails; 0 never
}

func (s *stubPublicationClient) record(ctx context.Context) {
	md, _ := metadata.FromOutgoingContext(ctx)
	s.authHeaders = append(s.authHeaders, strings.Join(md.Get("authorization"), ""))
	var budget time.Duration
	if deadline, ok := ctx.Deadline(); ok {
		budget = time.Until(deadline)
	}
	s.budgets = append(s.budgets, budget)
}

func (s *stubPublicationClient) Publish(ctx context.Context, in *publicationv1.PublishRequest, _ ...grpc.CallOption) (*publicationv1.PublishResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.record(ctx)
	s.publishReqs = append(s.publishReqs, in)
	if s.publishErr != nil {
		return nil, s.publishErr
	}
	return publicationv1.PublishResponse_builder{Publication: publicationv1.Publication_builder{
		Id:           "pub-1",
		CollectionId: in.GetCollectionId(),
	}.Build()}.Build(), nil
}

func (s *stubPublicationClient) Unpublish(ctx context.Context, in *publicationv1.UnpublishRequest, _ ...grpc.CallOption) (*publicationv1.UnpublishResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.record(ctx)
	s.unpublished = append(s.unpublished, in.GetPublicationId())
	return publicationv1.UnpublishResponse_builder{WasAlreadyRevoked: s.wasRevoked}.Build(), nil
}

func (s *stubPublicationClient) GetPublications(ctx context.Context, in *publicationv1.GetPublicationsRequest, _ ...grpc.CallOption) (*publicationv1.GetPublicationsResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.record(ctx)
	s.getBatches = append(s.getBatches, in.GetCollectionIds())
	if len(s.getBatches) == s.getPubsErrAt {
		return nil, status.Error(codes.Unavailable, "gone")
	}
	pubs := make([]*publicationv1.Publication, 0, len(in.GetCollectionIds()))
	for _, id := range in.GetCollectionIds() {
		pubs = append(pubs, publicationv1.Publication_builder{Id: "pub-" + id, CollectionId: id}.Build())
	}
	return publicationv1.GetPublicationsResponse_builder{Publications: pubs}.Build(), nil
}

type fakeClientProvider struct {
	mu     gosync.Mutex
	client *GRPCClient
}

func (p *fakeClientProvider) EnsureClient(context.Context) (*GRPCClient, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.client, nil
}

func (p *fakeClientProvider) set(client *GRPCClient) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.client = client
}

func signedInAuth(t *testing.T) *SyncAuthManager {
	t.Helper()
	mgr := NewSyncAuthManager(sqlite.NewSyncConfigRepo(setupTestDB(t)))
	mgr.storeTokens("access-1", "", "")
	return mgr
}

func newTestPublicationRemote(t *testing.T, pub publicationv1.PublicationServiceClient) PublicationRemote {
	t.Helper()
	client := NewGRPCClientWithPublication(&stubAuthClient{}, nil, pub)
	return NewPublicationRemote(&fakeClientProvider{client: client}, signedInAuth(t))
}

func assertBudget(t *testing.T, want, got time.Duration) {
	t.Helper()
	assert.LessOrEqual(t, got, want)
	assert.Greater(t, got, want-2*time.Second, "deadline %v is not the %v budget", got, want)
}

func TestPublicationRemote_Publish(t *testing.T) {
	stub := &stubPublicationClient{}
	remote := newTestPublicationRemote(t, stub)

	req := publicationv1.PublishRequest_builder{CollectionId: "c-1", ContentHash: strings.Repeat("a", 64)}.Build()
	pub, err := remote.Publish(context.Background(), req)
	require.NoError(t, err)

	assert.Equal(t, "pub-1", pub.GetId())
	assert.Equal(t, "c-1", pub.GetCollectionId())
	require.Len(t, stub.publishReqs, 1)
	assert.Same(t, req, stub.publishReqs[0])
	assert.Equal(t, []string{"Bearer access-1"}, stub.authHeaders)
	assertBudget(t, 30*time.Second, stub.budgets[0])
}

func TestPublicationRemote_Unpublish(t *testing.T) {
	stub := &stubPublicationClient{wasRevoked: true}
	remote := newTestPublicationRemote(t, stub)

	revoked, err := remote.Unpublish(context.Background(), "pub-7")
	require.NoError(t, err)

	assert.True(t, revoked)
	assert.Equal(t, []string{"pub-7"}, stub.unpublished)
	assert.Equal(t, []string{"Bearer access-1"}, stub.authHeaders)
	assertBudget(t, 10*time.Second, stub.budgets[0])
}

func TestPublicationRemote_GetPublications_ChunksOfAHundred(t *testing.T) {
	stub := &stubPublicationClient{}
	remote := newTestPublicationRemote(t, stub)

	ids := make([]string, 250)
	for i := range ids {
		ids[i] = fmt.Sprintf("c-%03d", i)
	}
	pubs, err := remote.GetPublications(context.Background(), ids)
	require.NoError(t, err)

	require.Len(t, stub.getBatches, 3)
	assert.Equal(t, ids[:100], stub.getBatches[0])
	assert.Equal(t, ids[100:200], stub.getBatches[1])
	assert.Equal(t, ids[200:], stub.getBatches[2])
	require.Len(t, pubs, 250)
	assert.Equal(t, "c-249", pubs[249].GetCollectionId())
	for i, header := range stub.authHeaders {
		assert.Equal(t, "Bearer access-1", header)
		assertBudget(t, 10*time.Second, stub.budgets[i])
	}
}

func TestPublicationRemote_GetPublications_FailedChunkFailsTheCall(t *testing.T) {
	stub := &stubPublicationClient{getPubsErrAt: 2}
	remote := newTestPublicationRemote(t, stub)

	ids := make([]string, 250)
	for i := range ids {
		ids[i] = fmt.Sprintf("c-%03d", i)
	}
	pubs, err := remote.GetPublications(context.Background(), ids)

	require.Error(t, err)
	assert.Equal(t, codes.Unavailable, status.Code(err))
	assert.Nil(t, pubs)
	assert.Len(t, stub.getBatches, 2)
}

func TestPublicationRemote_GetPublications_NothingToAsk(t *testing.T) {
	stub := &stubPublicationClient{}
	remote := newTestPublicationRemote(t, stub)

	pubs, err := remote.GetPublications(context.Background(), nil)
	require.NoError(t, err)
	assert.Empty(t, pubs)
	assert.Empty(t, stub.getBatches)
}

func TestPublicationRemote_ErrorKeepsStatusAndReason(t *testing.T) {
	st, err := status.New(codes.ResourceExhausted, "quota").
		WithDetails(&errdetails.ErrorInfo{Reason: "PUBLISH_QUOTA_EXCEEDED"})
	require.NoError(t, err)
	remote := newTestPublicationRemote(t, &stubPublicationClient{publishErr: st.Err()})

	_, err = remote.Publish(context.Background(), publicationv1.PublishRequest_builder{}.Build())

	got, ok := status.FromError(err)
	require.True(t, ok)
	assert.Equal(t, codes.ResourceExhausted, got.Code())
	require.Len(t, got.Details(), 1)
	assert.Equal(t, "PUBLISH_QUOTA_EXCEEDED", got.Details()[0].(*errdetails.ErrorInfo).GetReason())
}

func TestPublicationRemote_ServerSupportsPublish_CachedPerClient(t *testing.T) {
	var budget time.Duration
	first := &capabilityAuthClient{answer: func(ctx context.Context, _ int32) (*authv1.GetServerInfoResponse, error) {
		deadline, _ := ctx.Deadline()
		budget = time.Until(deadline)
		return serverInfoWith("response_examples", "publish"), nil
	}}
	provider := &fakeClientProvider{client: NewGRPCClientWithPublication(first, nil, &stubPublicationClient{})}
	remote := NewPublicationRemote(provider, signedInAuth(t))
	ctx := context.Background()

	for range 2 {
		ok, err := remote.ServerSupportsPublish(ctx)
		require.NoError(t, err)
		assert.True(t, ok)
	}
	assert.Equal(t, int32(1), first.calls.Load(), "a confirmed capability is not asked again")
	assertBudget(t, 5*time.Second, budget)

	second := &capabilityAuthClient{capabilities: []string{"publish"}}
	provider.set(NewGRPCClientWithPublication(second, nil, &stubPublicationClient{}))
	ok, err := remote.ServerSupportsPublish(ctx)
	require.NoError(t, err)
	assert.True(t, ok)
	assert.Equal(t, int32(1), second.calls.Load(), "a reconnected client is asked again")
}

func TestPublicationRemote_ServerSupportsPublish_OnlySupportIsCached(t *testing.T) {
	cases := []struct {
		name    string
		answer  func(context.Context, int32) (*authv1.GetServerInfoResponse, error)
		wantErr bool
	}{
		{"no capability", func(context.Context, int32) (*authv1.GetServerInfoResponse, error) {
			return serverInfoWith("desktop_signin", "response_examples"), nil
		}, false},
		{"server without the rpc", func(context.Context, int32) (*authv1.GetServerInfoResponse, error) {
			return nil, status.Error(codes.Unimplemented, "unknown method")
		}, false},
		{"rpc error", func(context.Context, int32) (*authv1.GetServerInfoResponse, error) {
			return nil, status.Error(codes.Unavailable, "connection refused")
		}, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			info := &capabilityAuthClient{answer: tc.answer}
			remote := NewPublicationRemote(
				&fakeClientProvider{client: NewGRPCClientWithPublication(info, nil, &stubPublicationClient{})},
				signedInAuth(t))

			for range 2 {
				ok, err := remote.ServerSupportsPublish(context.Background())
				assert.False(t, ok)
				if tc.wantErr {
					assert.Error(t, err)
				} else {
					assert.NoError(t, err)
				}
			}
			assert.Equal(t, int32(2), info.calls.Load())
		})
	}
}

type stubSubscriptionClient struct {
	subscriptionv1.SubscriptionServiceClient

	mu      gosync.Mutex
	orgIDs  []string
	headers []string
	budgets []time.Duration
	err     error
}

func (s *stubSubscriptionClient) GetActivePlan(ctx context.Context, in *subscriptionv1.GetActivePlanRequest, _ ...grpc.CallOption) (*subscriptionv1.GetActivePlanResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	md, _ := metadata.FromOutgoingContext(ctx)
	s.headers = append(s.headers, strings.Join(md.Get("authorization"), ""))
	deadline, _ := ctx.Deadline()
	s.budgets = append(s.budgets, time.Until(deadline))
	s.orgIDs = append(s.orgIDs, in.GetOrgId())
	if s.err != nil {
		return nil, s.err
	}
	features := map[string][]string{"org-personal": {"publish.password", "publish.unlisted"}, "org-team": {"invites"}}[in.GetOrgId()]
	return subscriptionv1.GetActivePlanResponse_builder{
		Plan: subscriptionv1.Plan_builder{Tier: "pro", Features: features}.Build(),
	}.Build(), nil
}

type myOrgsAuthClient struct {
	stubAuthClient
	memberships []*authv1.Membership
}

func (s *myOrgsAuthClient) MyOrgs(ctx context.Context, _ *authv1.MyOrgsRequest, _ ...grpc.CallOption) (*authv1.MyOrgsResponse, error) {
	s.recordAuth(ctx)
	return authv1.MyOrgsResponse_builder{Memberships: s.memberships}.Build(), nil
}

func newPlanRemote(t *testing.T, memberships []*authv1.Membership) (PublicationRemote, *stubSubscriptionClient, *myOrgsAuthClient) {
	t.Helper()
	auth := &myOrgsAuthClient{memberships: memberships}
	sub := &stubSubscriptionClient{}
	mgr := NewSyncAuthManager(sqlite.NewSyncConfigRepo(setupTestDB(t)))
	mgr.storeTokens("access-1", "", "org-team")
	return NewPublicationRemote(&fakeClientProvider{client: NewGRPCClientWithPlan(auth, sub)}, mgr), sub, auth
}

func TestPublicationRemote_PlanFeatures_CloudWorkspaceAsksTheActiveOrg(t *testing.T) {
	remote, sub, auth := newPlanRemote(t, nil)

	features, err := remote.PlanFeatures(context.Background(), false)

	require.NoError(t, err)
	assert.Equal(t, []string{"invites"}, features)
	assert.Equal(t, []string{"org-team"}, sub.orgIDs)
	assert.Equal(t, []string{"Bearer access-1"}, sub.headers)
	assert.Empty(t, auth.authHeaders, "a cloud workspace needs no org lookup")
	assertBudget(t, 10*time.Second, sub.budgets[0])
}

// The server counts a local collection against the author's personal org, whatever org is active.
func TestPublicationRemote_PlanFeatures_LocalWorkspaceAsksThePersonalOrg(t *testing.T) {
	remote, sub, auth := newPlanRemote(t, []*authv1.Membership{
		authv1.Membership_builder{OrgId: "org-team"}.Build(),
		authv1.Membership_builder{OrgId: "org-personal", IsPersonal: true}.Build(),
	})

	features, err := remote.PlanFeatures(context.Background(), true)

	require.NoError(t, err)
	assert.Equal(t, []string{"publish.password", "publish.unlisted"}, features)
	assert.Equal(t, []string{"org-personal"}, sub.orgIDs)
	assert.Equal(t, []string{"Bearer access-1"}, auth.authHeaders)
}

func TestPublicationRemote_PlanFeatures_WithoutAPersonalOrgAsksTheActiveOrg(t *testing.T) {
	remote, sub, _ := newPlanRemote(t, []*authv1.Membership{authv1.Membership_builder{OrgId: "org-team"}.Build()})

	_, err := remote.PlanFeatures(context.Background(), true)

	require.NoError(t, err)
	assert.Equal(t, []string{"org-team"}, sub.orgIDs)
}

func TestPublicationRemote_PlanFeatures_KeepsTheStatus(t *testing.T) {
	remote, sub, _ := newPlanRemote(t, nil)
	sub.err = status.Error(codes.Unimplemented, "unknown service")

	_, err := remote.PlanFeatures(context.Background(), false)

	assert.Equal(t, codes.Unimplemented, status.Code(err))
}
