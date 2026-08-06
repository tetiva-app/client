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
	"path/filepath"
	"syscall"

	"github.com/google/uuid"
	_ "modernc.org/sqlite"

	mcpadapter "github.com/tetiva-app/client/internal/adapters/mcp"
	"github.com/tetiva-app/client/internal/domain/entities"
	"github.com/tetiva-app/client/internal/domain/usecase/collection"
	"github.com/tetiva-app/client/internal/domain/usecase/environment"
	"github.com/tetiva-app/client/internal/domain/usecase/request"
	"github.com/tetiva-app/client/internal/domain/usecase/settings"
	"github.com/tetiva-app/client/internal/domain/usecase/workspace"
	"github.com/tetiva-app/client/internal/infrastructure/repository/sqlite"
	syncsvc "github.com/tetiva-app/client/internal/infrastructure/sync"
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

	colUC := collection.NewUsecase(colRepo)
	envUC := environment.NewUsecase(envRepo, varRepo)
	wsUC := workspace.NewUsecase(wsRepo)
	reqUC := request.NewUsecase(
		reqRepo, &noopHistoryRepo{}, &noopHTTPRequester{},
		&noopGRPCRequester{}, &noopGraphQLRequester{},
		&noopEnvResolver{}, &noopScriptEngine{},
		&noopScriptResolver{}, &noopVarPersister{},
		&noopAuthResolver{}, &noopCookieReader{}, nil,
	)

	auth := syncsvc.NewSyncAuthManager(syncConfigRepo)
	engine := syncsvc.NewSyncEngine(auth, syncQueueRepo, syncConfigRepo, db,
		colRepo, reqRepo, envRepo, varRepo)

	srv := mcpadapter.NewServer(engine, syncQueueRepo, colUC, reqUC, envUC, wsUC, addr)
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
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		log.Fatalf("open db: %v", err)
	}
	if _, err := db.Exec("PRAGMA foreign_keys=ON"); err != nil {
		log.Fatalf("pragma: %v", err)
	}
	return db
}

func applyMigrations(db *sql.DB) {
	migrationsDir := filepath.Join(getMigrationsDir(), "migrations")
	entries, err := os.ReadDir(migrationsDir)
	if err != nil {
		log.Fatalf("read migrations dir %s: %v", migrationsDir, err)
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		data, err := os.ReadFile(filepath.Join(migrationsDir, e.Name()))
		if err != nil {
			log.Fatalf("read migration %s: %v", e.Name(), err)
		}
		if _, err := db.Exec(string(data)); err != nil {
			log.Fatalf("apply migration %s: %v", e.Name(), err)
		}
	}
	slog.Info("migrations applied")
}

func getMigrationsDir() string {
	if _, err := os.Stat("migrations"); err == nil {
		return "."
	}
	exe, _ := os.Executable()
	dir := filepath.Dir(exe)
	if _, err := os.Stat(filepath.Join(dir, "migrations")); err == nil {
		return dir
	}
	log.Fatal("cannot find migrations directory")
	return ""
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

type noopAuthResolver struct{}

func (n *noopAuthResolver) ResolveAuth(_ context.Context, _ *entities.Request) (entities.AuthType, string, error) {
	return entities.AuthTypeNone, "{}", nil
}

type noopCookieReader struct{}

func (noopCookieReader) CookiesFor(_ context.Context, _ uuid.UUID, _ string) []*http.Cookie {
	return nil
}
