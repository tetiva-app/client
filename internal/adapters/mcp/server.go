package mcp

import (
	"context"
	"fmt"
	"log/slog"
	"net"

	mcplib "github.com/mark3labs/mcp-go/mcp"
	mcpserver "github.com/mark3labs/mcp-go/server"

	"github.com/tetiva-app/client/internal/domain/usecase/collection"
	"github.com/tetiva-app/client/internal/domain/usecase/environment"
	"github.com/tetiva-app/client/internal/domain/usecase/request"
	"github.com/tetiva-app/client/internal/domain/usecase/workspace"
	"github.com/tetiva-app/client/internal/infrastructure/repository/sqlite"
	syncsvc "github.com/tetiva-app/client/internal/infrastructure/sync"
)

const (
	defaultWorkspaceID = "00000000-0000-4000-a000-000000000001"
	mcpUserID          = "mcp-devtools"
)

// Server is the MCP DevTools server embedded in the Wails application.
type Server struct {
	mcp       *mcpserver.MCPServer
	sse       *mcpserver.SSEServer
	addr      string
	engine    *syncsvc.SyncEngine
	syncQueue sqlite.SyncQueueRepository
	colUC     collection.Usecase
	reqUC     request.Usecase
	envUC     environment.Usecase
	wsUC      workspace.Usecase
}

// NewServer creates a new MCP DevTools server.
func NewServer(
	engine *syncsvc.SyncEngine,
	syncQueue sqlite.SyncQueueRepository,
	colUC collection.Usecase,
	reqUC request.Usecase,
	envUC environment.Usecase,
	wsUC workspace.Usecase,
	addr string,
) *Server {
	s := &Server{
		addr:      addr,
		engine:    engine,
		syncQueue: syncQueue,
		colUC:     colUC,
		reqUC:     reqUC,
		envUC:     envUC,
		wsUC:      wsUC,
	}

	s.mcp = mcpserver.NewMCPServer(
		"Tetiva DevTools",
		"1.0.0",
		mcpserver.WithToolCapabilities(true),
	)

	s.registerSyncTools()
	s.registerWorkspaceTools()
	s.registerCollectionTools()
	s.registerRequestTools()
	s.registerEnvironmentTools()
	s.registerVariableTools()

	return s
}

// Start starts the SSE server on the configured address.
func (s *Server) Start(_ context.Context) error {
	if !isLoopbackAddr(s.addr) {
		slog.Warn("MCP DevTools is reachable from the network and has no authentication: "+
			"anyone who can reach this address can read collections, environment variables and send requests",
			"addr", s.addr)
	}

	s.sse = mcpserver.NewSSEServer(s.mcp,
		mcpserver.WithSSEEndpoint("/sse"),
		mcpserver.WithMessageEndpoint("/message"),
	)

	go func() {
		slog.Info("MCP DevTools server starting", "addr", s.addr)
		if err := s.sse.Start(s.addr); err != nil {
			slog.Error("MCP DevTools server failed", "err", err)
		}
	}()

	return nil
}

// Engine returns the underlying SyncEngine. Exposed for test helpers only.
func (s *Server) Engine() *syncsvc.SyncEngine {
	return s.engine
}

// Stop gracefully shuts down the SSE server.
func (s *Server) Stop(_ context.Context) error {
	if s.sse != nil {
		slog.Info("MCP DevTools server stopping")
		return s.sse.Shutdown(context.Background())
	}
	return nil
}

// isLoopbackAddr reports whether the listen address is reachable only from this
// machine. A host-less address (":9300") binds every interface.
func isLoopbackAddr(addr string) bool {
	host, _, err := net.SplitHostPort(addr)
	if err != nil || host == "" {
		return false
	}
	if host == "localhost" {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func errResult(err error) *mcplib.CallToolResult {
	return mcplib.NewToolResultError(fmt.Sprintf("error: %v", err))
}

func textResult(text string) *mcplib.CallToolResult {
	return mcplib.NewToolResultText(text)
}
