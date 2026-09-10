// Standalone MCP DevTools server for testing without Wails.
package main

import (
	"context"
	"database/sql"
	"log"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/google/uuid"
	_ "modernc.org/sqlite"

	mcpadapter "github.com/tetiva-app/client/internal/adapters/mcp"
	"github.com/tetiva-app/client/internal/domain/entities"
	"github.com/tetiva-app/client/internal/domain/usecase/auth"
	"github.com/tetiva-app/client/internal/domain/usecase/collection"
	"github.com/tetiva-app/client/internal/domain/usecase/environment"
	"github.com/tetiva-app/client/internal/domain/usecase/request"
	"github.com/tetiva-app/client/internal/domain/usecase/settings"
	"github.com/tetiva-app/client/internal/domain/usecase/workspace"
	"github.com/tetiva-app/client/internal/infrastructure/repository/sqlite"
	syncsvc "github.com/tetiva-app/client/internal/infrastructure/sync"
	"github.com/tetiva-app/client/migrations"
	"github.com/tetiva-app/client/pkg/migrate"
)

func main() {
	addr := settings.DefaultMCPAddr
	if a := os.Getenv("MCP_ADDR"); a != "" {
		addr = settings.NormalizeMCPAddr(a)
	}

	db := openDB()
	applyMigrations(db)

	colRepo := sqlite.NewCollectionRepo(db)
	reqRepo := sqlite.NewRequestRepo(db)
	envRepo := sqlite.NewEnvironmentRepo(db)
	varRepo := sqlite.NewVariableRepo(db)
	wsRepo := sqlite.NewWorkspaceRepo(db)
	syncQueueRepo := sqlite.NewSyncQueueRepo(db)
	syncConfigRepo := sqlite.NewSyncConfigRepo(db)
	tokenRepo := sqlite.NewAuthTokenRepo(db)

	colUC := collection.NewUsecase(colRepo, tokenRepo)
	envUC := environment.NewUsecase(envRepo, varRepo)
	wsUC := workspace.NewUsecase(wsRepo)
	reqUC := request.NewUsecase(
		reqRepo, &noopHistoryRepo{}, &noopHTTPRequester{},
		&noopGRPCRequester{}, &noopGraphQLRequester{},
		&noopEnvResolver{}, &noopScriptEngine{},
		&noopScriptResolver{}, &noopVarPersister{},
		request.NewAuthResolver(collectionReader{repo: colRepo}),
		&noopCookieReader{}, nil, tokenRepo, auth.NewProvider(tokenRepo, nil, nil),
	)

	syncAuth := syncsvc.NewSyncAuthManager(syncConfigRepo)
	engine := syncsvc.NewSyncEngine(syncAuth, syncQueueRepo, syncConfigRepo, db,
		colRepo, reqRepo, envRepo, varRepo, tokenRepo)

	// Debug binary: no UI to copy a token from, so MCP_TOKEN is the only source
	// and an empty one leaves the server open.
	token := os.Getenv("MCP_TOKEN")
	mcpAuth := mcpadapter.NewTokenAuth(token, token != "")

	srv := mcpadapter.NewServer(engine, syncQueueRepo, colUC, reqUC, envUC, wsUC, addr, mcpAuth)
	ctx, cancel := signal.NotifyContext(
		signalContext(), syscall.SIGINT, syscall.SIGTERM,
	)
	defer cancel()

	if err := srv.Start(ctx); err != nil {
		log.Fatal(err)
	}

	slog.Info("MCP DevTools server ready", "addr", addr)
	<-ctx.Done()
	slog.Info("shutting down")
	_ = srv.Stop(ctx)
}

func openDB() *sql.DB {
	dbPath := os.Getenv("MCP_DB")
	if dbPath == "" {
		dbPath = ":memory:"
	}
	// The shared DSN carries busy_timeout to every pooled connection; ":memory:"
	// comes back unchanged, so the single-connection branch below still applies.
	db, err := sql.Open("sqlite", sqlite.DSN(dbPath))
	if err != nil {
		log.Fatalf("open db: %v", err)
	}
	if dbPath == ":memory:" {
		// every extra pooled connection would open its own empty in-memory database
		db.SetMaxOpenConns(1)
	}
	if _, err := db.Exec("PRAGMA foreign_keys=ON"); err != nil {
		log.Fatalf("pragma: %v", err)
	}
	return db
}

func applyMigrations(db *sql.DB) {
	// Databases created by the old per-file loop have no schema_migrations rows;
	// the runner would replay every migration on top of an existing schema.
	var legacy int
	if err := db.QueryRow(`SELECT EXISTS(SELECT 1 FROM sqlite_master WHERE type='table' AND name='collections')
		AND NOT EXISTS(SELECT 1 FROM sqlite_master WHERE type='table' AND name='schema_migrations')`).Scan(&legacy); err != nil {
		log.Fatalf("probe schema: %v", err)
	}
	if legacy == 1 {
		log.Fatal("this database predates schema_migrations: delete it or point MCP_DB at a fresh path")
	}
	if err := migrate.Run(db, migrations.FS, "."); err != nil {
		log.Fatalf("apply migrations: %v", err)
	}
	slog.Info("migrations applied")
}

func signalContext() context.Context {
	return context.Background()
}

// Noop stubs for request usecase dependencies (not needed for MCP CRUD).

type noopHistoryRepo struct{}

func (n *noopHistoryRepo) Create(_ context.Context, _ *entities.History) error { return nil }
func (n *noopHistoryRepo) GetByID(_ context.Context, _ uuid.UUID) (*entities.History, error) {
	return nil, nil
}

type noopHTTPRequester struct{}

func (n *noopHTTPRequester) Execute(_ context.Context, _ request.HTTPExecuteRequest) (*entities.Response, error) {
	return nil, nil
}

type noopGRPCRequester struct{}

func (n *noopGRPCRequester) Execute(_ context.Context, _ request.GRPCExecuteRequest) (*entities.Response, error) {
	return nil, nil
}
func (n *noopGRPCRequester) ListServices(_ context.Context, _ request.GRPCConnectRequest) (*request.GRPCSchema, error) {
	return nil, nil
}

type noopGraphQLRequester struct{}

func (n *noopGraphQLRequester) Execute(_ context.Context, _ request.GraphQLExecuteRequest) (*entities.Response, error) {
	return nil, nil
}
func (n *noopGraphQLRequester) Introspect(_ context.Context, _ request.GraphQLIntrospectRequest) (*request.GraphQLSchema, error) {
	return nil, nil
}
func (n *noopGraphQLRequester) GenerateExampleQuery(_ *request.GraphQLSchema, _ string) (*request.GraphQLExampleResponse, error) {
	return nil, nil
}

type noopEnvResolver struct{}

func (n *noopEnvResolver) ResolveVariables(_ context.Context, _ uuid.UUID) (map[string]string, error) {
	return nil, nil
}

type noopScriptEngine struct{}

func (n *noopScriptEngine) RunPreScript(_ context.Context, _ string, _ request.ScriptContext) (*request.PreScriptResult, error) {
	return &request.PreScriptResult{}, nil
}
func (n *noopScriptEngine) RunPostScript(_ context.Context, _ string, _ request.ScriptContext) (*request.PostScriptResult, error) {
	return &request.PostScriptResult{}, nil
}

type noopScriptResolver struct{}

func (n *noopScriptResolver) ResolvePreScript(_ context.Context, _ *entities.Request) (string, error) {
	return "", nil
}
func (n *noopScriptResolver) ResolvePostScript(_ context.Context, _ *entities.Request) (string, error) {
	return "", nil
}

type noopVarPersister struct{}

func (n *noopVarPersister) PersistVariableChanges(_ context.Context, _ uuid.UUID, _ string, _ map[string]string) error {
	return nil
}

type noopCookieReader struct{}

func (noopCookieReader) CookiesFor(_ context.Context, _ uuid.UUID, _ string) []*http.Cookie {
	return nil
}

// collectionReader adapts the collection repo to request.CollectionReader; kept
// local so this dev binary stays free of the fx/Wails graph.
type collectionReader struct{ repo collection.Repository }

func (c collectionReader) GetByID(ctx context.Context, id uuid.UUID) (*entities.Collection, error) {
	return c.repo.GetByID(ctx, id)
}

func (c collectionReader) ListByWorkspace(ctx context.Context, workspaceID uuid.UUID) ([]*entities.Collection, error) {
	return c.repo.List(ctx, collection.Filter{WorkspaceID: workspaceID})
}
