package sync

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"sync"
	"time"

	authv1 "github.com/tetiva-app/proto/go/gophercourier/auth/v1"
	publicationv1 "github.com/tetiva-app/proto/go/gophercourier/publication/v1"
	subscriptionv1 "github.com/tetiva-app/proto/go/gophercourier/subscription/v1"

	"github.com/tetiva-app/client/internal/domain/usecase/auth"
)

const (
	publishTimeout        = 30 * time.Second
	publicationRPCTimeout = 10 * time.Second
	// maxPublicationIDs is the server's cap per GetPublications; more is InvalidArgument.
	maxPublicationIDs = 100
)

// ClientProvider's connection is swapped on reconnect: ask on every call, never keep one.
type ClientProvider interface {
	EnsureClient(ctx context.Context) (*GRPCClient, error)
}

// PublicationRemote talks to the publication API of the user's own sync server.
type PublicationRemote interface {
	ServerSupportsPublish(ctx context.Context) (bool, error)
	Publish(ctx context.Context, req *publicationv1.PublishRequest) (*publicationv1.Publication, error)
	Unpublish(ctx context.Context, publicationID string) (wasAlreadyRevoked bool, err error)
	GetPublications(ctx context.Context, collectionIDs []string) ([]*publicationv1.Publication, error)
	// personal asks the author's personal org, for a local collection; else the active org.
	PlanFeatures(ctx context.Context, personal bool) ([]string, error)
}

type publicationRemote struct {
	clients ClientProvider
	auth    *SyncAuthManager

	mu            sync.Mutex
	publishClient *GRPCClient
}

func NewPublicationRemote(clients ClientProvider, auth *SyncAuthManager) PublicationRemote {
	return &publicationRemote{clients: clients, auth: auth}
}

// Only a yes is cached, per connection: an operator may turn publishing on at runtime.
func (r *publicationRemote) ServerSupportsPublish(ctx context.Context) (bool, error) {
	const funcName = "publicationRemote.ServerSupportsPublish"

	ctx, cancel := context.WithTimeout(ctx, serverInfoTimeout)
	defer cancel()

	client, err := r.clients.EnsureClient(ctx)
	if err != nil {
		return false, fmt.Errorf("%s: %w", funcName, err)
	}
	if r.confirmed(client) {
		return true, nil
	}

	info, err := client.GetServerInfo(ctx)
	switch {
	case errors.Is(err, auth.ErrServerInfoUnsupported):
		return false, nil
	case err != nil:
		return false, fmt.Errorf("%s: %w", funcName, err)
	case !info.Publish:
		return false, nil
	}

	r.mu.Lock()
	r.publishClient = client
	r.mu.Unlock()
	return true, nil
}

func (r *publicationRemote) confirmed(client *GRPCClient) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.publishClient == client
}

func (r *publicationRemote) Publish(ctx context.Context, req *publicationv1.PublishRequest) (*publicationv1.Publication, error) {
	const funcName = "publicationRemote.Publish"

	ctx, cancel := context.WithTimeout(ctx, publishTimeout)
	defer cancel()

	stub, ctx, err := r.connect(ctx)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", funcName, err)
	}
	resp, err := stub.Publish(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("%s: publish rpc: %w", funcName, err)
	}
	return resp.GetPublication(), nil
}

func (r *publicationRemote) Unpublish(ctx context.Context, publicationID string) (bool, error) {
	const funcName = "publicationRemote.Unpublish"

	ctx, cancel := context.WithTimeout(ctx, publicationRPCTimeout)
	defer cancel()

	stub, ctx, err := r.connect(ctx)
	if err != nil {
		return false, fmt.Errorf("%s: %w", funcName, err)
	}
	resp, err := stub.Unpublish(ctx, publicationv1.UnpublishRequest_builder{PublicationId: publicationID}.Build())
	if err != nil {
		return false, fmt.Errorf("%s: unpublish rpc: %w", funcName, err)
	}
	return resp.GetWasAlreadyRevoked(), nil
}

func (r *publicationRemote) GetPublications(ctx context.Context, collectionIDs []string) ([]*publicationv1.Publication, error) {
	const funcName = "publicationRemote.GetPublications"

	var out []*publicationv1.Publication
	for ids := range slices.Chunk(collectionIDs, maxPublicationIDs) {
		pubs, err := r.getPublications(ctx, ids)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", funcName, err)
		}
		out = append(out, pubs...)
	}
	return out, nil
}

func (r *publicationRemote) getPublications(ctx context.Context, ids []string) ([]*publicationv1.Publication, error) {
	ctx, cancel := context.WithTimeout(ctx, publicationRPCTimeout)
	defer cancel()

	stub, ctx, err := r.connect(ctx)
	if err != nil {
		return nil, err
	}
	resp, err := stub.GetPublications(ctx, publicationv1.GetPublicationsRequest_builder{CollectionIds: ids}.Build())
	if err != nil {
		return nil, fmt.Errorf("get publications rpc: %w", err)
	}
	return resp.GetPublications(), nil
}

func (r *publicationRemote) PlanFeatures(ctx context.Context, personal bool) ([]string, error) {
	const funcName = "publicationRemote.PlanFeatures"

	ctx, cancel := context.WithTimeout(ctx, publicationRPCTimeout)
	defer cancel()

	client, err := r.clients.EnsureClient(ctx)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", funcName, err)
	}
	token, err := r.auth.GetAccessToken(ctx, client)
	if err != nil {
		return nil, fmt.Errorf("%s: get access token: %w", funcName, err)
	}
	ctx = ContextWithAuth(ctx, token)

	orgID := r.auth.GetActiveOrgID()
	if personal {
		resp, err := client.Auth().MyOrgs(ctx, authv1.MyOrgsRequest_builder{}.Build())
		if err != nil {
			return nil, fmt.Errorf("%s: my orgs rpc: %w", funcName, err)
		}
		for _, m := range resp.GetMemberships() {
			if m.GetIsPersonal() {
				orgID = m.GetOrgId()
				break
			}
		}
	}
	if orgID == "" {
		return nil, fmt.Errorf("%s: no organization to ask", funcName)
	}

	resp, err := client.Subscription().GetActivePlan(ctx, subscriptionv1.GetActivePlanRequest_builder{OrgId: orgID}.Build())
	if err != nil {
		return nil, fmt.Errorf("%s: get active plan rpc: %w", funcName, err)
	}
	return resp.GetPlan().GetFeatures(), nil
}

func (r *publicationRemote) connect(ctx context.Context) (publicationv1.PublicationServiceClient, context.Context, error) {
	client, err := r.clients.EnsureClient(ctx)
	if err != nil {
		return nil, nil, err
	}
	token, err := r.auth.GetAccessToken(ctx, client)
	if err != nil {
		return nil, nil, fmt.Errorf("get access token: %w", err)
	}
	return client.Publication(), ContextWithAuth(ctx, token), nil
}
