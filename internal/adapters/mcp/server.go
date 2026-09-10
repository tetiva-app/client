package mcp

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"sort"
	"strings"
	"time"

	mcplib "github.com/mark3labs/mcp-go/mcp"
	mcpserver "github.com/mark3labs/mcp-go/server"

	"github.com/tetiva-app/client/internal/domain"
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

	// shutdownTimeout caps the graceful phase even when the caller passes a
	// context without a deadline, so quitting the app never waits on a client.
	shutdownTimeout = 5 * time.Second
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
	auth      *TokenAuth
	httpSrv   *http.Server
}

func NewServer(
	engine *syncsvc.SyncEngine,
	syncQueue sqlite.SyncQueueRepository,
	colUC collection.Usecase,
	reqUC request.Usecase,
	envUC environment.Usecase,
	wsUC workspace.Usecase,
	addr string,
	auth *TokenAuth,
) *Server {
	if auth == nil {
		auth = NewTokenAuth("", false)
	}
	s := &Server{
		addr:      addr,
		auth:      auth,
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

func (s *Server) Start(_ context.Context) error {
	if !isLoopbackAddr(s.addr) && !s.auth.Enabled() {
		slog.Warn("MCP DevTools is reachable from the network without a token: "+
			"anyone who can reach this address can read collections, environment variables and send requests",
			"addr", s.addr)
	}

	// The handler closes over s.sse, which the constructor below assigns; it is
	// only dereferenced once the goroutine starts serving.
	s.httpSrv = &http.Server{
		Addr: s.addr,
		Handler: s.auth.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			s.sse.ServeHTTP(w, r)
		})),
		ReadHeaderTimeout: 10 * time.Second,
	}

	s.sse = mcpserver.NewSSEServer(s.mcp,
		mcpserver.WithSSEEndpoint("/sse"),
		mcpserver.WithMessageEndpoint("/message"),
		// Without this the endpoint event drops the ?token= of the /sse request and
		// every follow-up POST /message would be rejected as unauthenticated.
		mcpserver.WithAppendQueryToMessageEndpoint(),
		mcpserver.WithHTTPServer(s.httpSrv),
	)

	go func() {
		slog.Info("MCP DevTools server starting", "addr", s.addr)
		if err := s.sse.Start(s.addr); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("MCP DevTools server failed", "err", err)
		}
	}()

	return nil
}

// Exposed for test helpers only.
func (s *Server) Engine() *syncsvc.SyncEngine {
	return s.engine
}

// Graceful shutdown gets the caller's deadline capped at shutdownTimeout: a half-sent
// POST /message parks in the JSON decoder forever, so the connections are then dropped.
func (s *Server) Stop(ctx context.Context) error {
	if s.sse == nil {
		return nil
	}
	slog.Info("MCP DevTools server stopping")

	ctx, cancel := context.WithTimeout(ctx, shutdownTimeout)
	defer cancel()

	shutdownSSE(ctx, s.sse)
	if s.httpSrv == nil {
		return nil
	}
	// Repeated after shutdownSSE because a panic there leaves the port bound and
	// skips whatever sessions the panic cut the loop short of.
	err := s.httpSrv.Shutdown(ctx)
	if err == nil {
		return nil
	}
	slog.Warn("MCP DevTools graceful shutdown timed out, dropping connections", "err", err)
	return s.httpSrv.Close()
}

// shutdownSSE contains the double close of a session's done channel that mcp-go
// v0.45.0 panics on when a client is still attached at quit.
func shutdownSSE(ctx context.Context, sse *mcpserver.SSEServer) {
	defer func() {
		if r := recover(); r != nil {
			slog.Warn("MCP DevTools shutdown panicked", "err", r)
		}
	}()
	if err := sse.Shutdown(ctx); err != nil {
		slog.Warn("MCP DevTools shutdown failed", "err", err)
	}
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
	return mcplib.NewToolResultError("error: " + errText(err))
}

// errText spells out per-field reasons: a bare "validation failed" gives the agent
// nothing to correct.
func errText(err error) string {
	var valErr *domain.ValidationError
	if !errors.As(err, &valErr) || len(valErr.Fields) == 0 {
		return err.Error()
	}

	fields := make([]string, 0, len(valErr.Fields))
	for name := range valErr.Fields {
		fields = append(fields, name)
	}
	sort.Strings(fields)

	parts := make([]string, 0, len(fields))
	for _, name := range fields {
		parts = append(parts, name+": "+valErr.Fields[name])
	}
	return valErr.Error() + " (" + strings.Join(parts, "; ") + ")"
}

func textResult(text string) *mcplib.CallToolResult {
	return mcplib.NewToolResultText(text)
}
