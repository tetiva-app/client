package sync

import (
	"context"
	"crypto/tls"
	"fmt"
	"log/slog"
	"os"
	"runtime"
	"strings"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"

	authv1 "github.com/tetiva-app/proto/go/gophercourier/auth/v1"
	publicationv1 "github.com/tetiva-app/proto/go/gophercourier/publication/v1"
	subscriptionv1 "github.com/tetiva-app/proto/go/gophercourier/subscription/v1"
	syncv1 "github.com/tetiva-app/proto/go/gophercourier/sync/v1"
	workspacev1 "github.com/tetiva-app/proto/go/gophercourier/workspace/v1"

	"github.com/tetiva-app/client/internal/constants"
)

type GRPCClient struct {
	conn         *grpc.ClientConn
	auth         authv1.AuthServiceClient
	sync         syncv1.SyncServiceClient
	workspace    workspacev1.WorkspaceServiceClient
	publication  publicationv1.PublicationServiceClient
	subscription subscriptionv1.SubscriptionServiceClient
}

// Uses TLS for port 443, insecure for localhost/127.0.0.1 addresses.
func NewGRPCClient(serverURL string) (*GRPCClient, error) {
	useTLS := requiresTLS(serverURL)
	var creds grpc.DialOption
	if useTLS {
		creds = grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{}))
	} else {
		creds = grpc.WithTransportCredentials(insecure.NewCredentials())
	}

	// Use passthrough resolver — lets Go's standard net.Dialer handle DNS,
	// which works reliably inside Wails sandbox (gRPC's built-in dns resolver may not).
	target := serverURL
	if !strings.Contains(target, "://") {
		target = "passthrough:///" + target
	}

	slog.Info("grpc: NewGRPCClient", "input", serverURL, "target", target, "tls", useTLS)

	conn, err := grpc.NewClient(target, creds, grpc.WithUserAgent(userAgent()),
		// Pull pages of 100 entities with bodies and docs outgrow the 4 MiB default.
		grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(16<<20)))
	if err != nil {
		return nil, fmt.Errorf("grpc dial: %w", err)
	}

	return &GRPCClient{
		conn:         conn,
		auth:         authv1.NewAuthServiceClient(conn),
		sync:         syncv1.NewSyncServiceClient(conn),
		workspace:    workspacev1.NewWorkspaceServiceClient(conn),
		publication:  publicationv1.NewPublicationServiceClient(conn),
		subscription: subscriptionv1.NewSubscriptionServiceClient(conn),
	}, nil
}

// NewGRPCClientWithStubs builds a client around ready-made stubs, for tests without a live server.
func NewGRPCClientWithStubs(auth authv1.AuthServiceClient, ws workspacev1.WorkspaceServiceClient) *GRPCClient {
	return &GRPCClient{auth: auth, workspace: ws}
}

// NewGRPCClientWithSyncStub adds a sync stub, for tests that drive a syncer.
func NewGRPCClientWithSyncStub(auth authv1.AuthServiceClient, ws workspacev1.WorkspaceServiceClient, sync syncv1.SyncServiceClient) *GRPCClient {
	return &GRPCClient{auth: auth, workspace: ws, sync: sync}
}

// NewGRPCClientWithPublication adds a publication stub, for tests of publication calls.
func NewGRPCClientWithPublication(auth authv1.AuthServiceClient, ws workspacev1.WorkspaceServiceClient, pub publicationv1.PublicationServiceClient) *GRPCClient {
	return &GRPCClient{auth: auth, workspace: ws, publication: pub}
}

// NewGRPCClientWithPlan takes a subscription stub, for tests of the plan lookup.
func NewGRPCClientWithPlan(auth authv1.AuthServiceClient, sub subscriptionv1.SubscriptionServiceClient) *GRPCClient {
	return &GRPCClient{auth: auth, subscription: sub}
}

// userAgent names this device in the server's session list.
func userAgent() string {
	host, _ := os.Hostname()
	return fmt.Sprintf("Tetiva/%s (%s; %s)", constants.AppVersion, runtime.GOOS, host)
}

func (c *GRPCClient) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

func (c *GRPCClient) Auth() authv1.AuthServiceClient { return c.auth }

func (c *GRPCClient) Sync() syncv1.SyncServiceClient { return c.sync }

func (c *GRPCClient) Workspace() workspacev1.WorkspaceServiceClient { return c.workspace }

func (c *GRPCClient) Publication() publicationv1.PublicationServiceClient { return c.publication }

func (c *GRPCClient) Subscription() subscriptionv1.SubscriptionServiceClient { return c.subscription }

func ContextWithAuth(ctx context.Context, token string) context.Context {
	return metadata.AppendToOutgoingContext(ctx, "authorization", "Bearer "+token)
}

func requiresTLS(addr string) bool {
	if strings.HasSuffix(addr, ":443") {
		return true
	}
	host := addr
	if idx := strings.LastIndex(addr, ":"); idx != -1 {
		host = addr[:idx]
	}
	return host != "localhost" && host != "127.0.0.1" && host != "[::1]"
}
