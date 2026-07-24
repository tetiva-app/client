package sync

import (
	"context"
	"crypto/tls"
	"fmt"
	"log/slog"
	"strings"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"

	authv1 "github.com/tetiva-app/proto/go/gophercourier/auth/v1"
	syncv1 "github.com/tetiva-app/proto/go/gophercourier/sync/v1"
	workspacev1 "github.com/tetiva-app/proto/go/gophercourier/workspace/v1"
)

// GRPCClient wraps gRPC service stubs for sync server communication.
type GRPCClient struct {
	conn      *grpc.ClientConn
	auth      authv1.AuthServiceClient
	sync      syncv1.SyncServiceClient
	workspace workspacev1.WorkspaceServiceClient
}

// NewGRPCClient creates a new gRPC client connected to the given server URL.
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

	conn, err := grpc.NewClient(target, creds)
	if err != nil {
		return nil, fmt.Errorf("grpc dial: %w", err)
	}

	return &GRPCClient{
		conn:      conn,
		auth:      authv1.NewAuthServiceClient(conn),
		sync:      syncv1.NewSyncServiceClient(conn),
		workspace: workspacev1.NewWorkspaceServiceClient(conn),
	}, nil
}

// Close closes the gRPC connection.
func (c *GRPCClient) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

// Auth returns the auth service client.
func (c *GRPCClient) Auth() authv1.AuthServiceClient { return c.auth }

// Sync returns the sync service client.
func (c *GRPCClient) Sync() syncv1.SyncServiceClient { return c.sync }

// Workspace returns the workspace service client.
func (c *GRPCClient) Workspace() workspacev1.WorkspaceServiceClient { return c.workspace }

// ContextWithAuth creates a context with the authorization header.
func ContextWithAuth(ctx context.Context, token string) context.Context {
	return metadata.AppendToOutgoingContext(ctx, "authorization", "Bearer "+token)
}

// requiresTLS returns true if the server address needs TLS (port 443 or non-local host).
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
