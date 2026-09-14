package request

import (
	"context"
	"io"
	"net/http"

	"github.com/google/uuid"

	"github.com/tetiva-app/client/internal/domain/entities"
	"github.com/tetiva-app/client/internal/domain/usecase/auth"
	"github.com/tetiva-app/client/internal/domain/usecase/websocket"
)

type Repository interface {
	Create(ctx context.Context, r *entities.Request) error
	GetByID(ctx context.Context, id uuid.UUID) (*entities.Request, error)
	List(ctx context.Context, filter Filter) ([]*entities.Request, error)
	Update(ctx context.Context, r *entities.Request) error
	// GetDescriptionByID reads one field ignoring is_delete; the sync engine needs it
	// to keep a description a peer too old to send the field would otherwise wipe.
	GetDescriptionByID(ctx context.Context, id uuid.UUID) (string, error)
	UpdateSortOrder(ctx context.Context, id uuid.UUID, sortOrder int) error
	DeleteHard(ctx context.Context, id uuid.UUID) error
	CleanupDrafts(ctx context.Context) (int, error)
}

// HistoryRepository persists execution history (immutable, append-only).
type HistoryRepository interface {
	Create(ctx context.Context, h *entities.History) error
	GetByID(ctx context.Context, id uuid.UUID) (*entities.History, error)
}

type HTTPRequester interface {
	Execute(ctx context.Context, req HTTPExecuteRequest) (*entities.Response, error)
}

type GRPCRequester interface {
	Execute(ctx context.Context, req GRPCExecuteRequest) (*entities.Response, error)
	ListServices(ctx context.Context, req GRPCConnectRequest) (*GRPCSchema, error)
}

type GRPCExecuteRequest struct {
	Host        string
	UseTLS      bool
	Service     string
	Method      string
	Message     string
	Metadata    map[string][]string
	ProtoPath   string
	WorkspaceID uuid.UUID
}

type GRPCConnectRequest struct {
	Host        string
	UseTLS      bool
	ProtoPath   string
	WorkspaceID uuid.UUID
}

type GRPCSchema struct {
	Services []GRPCService
	Source   string // "reflection" | "proto_file" | "proto_directory"
}

type GRPCService struct {
	FullName string
	Methods  []GRPCMethodInfo
}

type GRPCMethodInfo struct {
	Name            string
	InputType       string
	OutputType      string
	IsServerStream  bool
	IsClientStream  bool
	ProtoDefinition string
	ExampleJSON     string
}

type GraphQLRequester interface {
	Execute(ctx context.Context, req GraphQLExecuteRequest) (*entities.Response, error)
	Introspect(ctx context.Context, req GraphQLIntrospectRequest) (*GraphQLSchema, error)
	GenerateExampleQuery(schema *GraphQLSchema, operationName string) (*GraphQLExampleResponse, error)
}

type GraphQLExecuteRequest struct {
	Endpoint      string
	Query         string
	Variables     string // JSON
	OperationName string
	Headers       map[string][]string
	WorkspaceID   uuid.UUID // selects the cookie jar; Nil sends no cookies
}

// GraphQLIntrospectRequest needs exactly one of Endpoint or SchemaPath.
type GraphQLIntrospectRequest struct {
	Endpoint    string
	SchemaPath  string
	Headers     map[string][]string
	WorkspaceID uuid.UUID // selects the cookie jar; Nil sends no cookies
}

type GraphQLSchema struct {
	Queries   []GraphQLOperation
	Mutations []GraphQLOperation
	Types     []GraphQLType
	Source    string // "introspection" | "schema_file"
}

type GraphQLOperation struct {
	Name       string
	Args       []GraphQLArg
	ReturnType string
	Definition string // SDL snippet for schema viewer
}

type GraphQLArg struct {
	Name         string
	Type         string // e.g. "ID!", "[String]", "CreateUserInput!"
	DefaultValue string
}

type GraphQLType struct {
	Name          string
	Kind          string // OBJECT, INPUT_OBJECT, ENUM, INTERFACE, UNION, SCALAR
	Fields        []GraphQLField
	EnumValues    []string
	PossibleTypes []string // for INTERFACE and UNION
	Definition    string   // SDL for this type
}

type GraphQLField struct {
	Name string
	Type string // e.g. "String!", "[Order!]"
	Args []GraphQLArg
}

type GraphQLExampleResponse struct {
	Query     string
	Variables string // JSON
}

type EnvironmentResolver interface {
	ResolveVariables(ctx context.Context, workspaceID uuid.UUID) (map[string]string, error)
}

// CookieReader returns cookies to send with a request URL. workspaceID is ignored
// by the in-memory jar but kept in the signature for a per-workspace persisted jar.
type CookieReader interface {
	CookiesFor(ctx context.Context, workspaceID uuid.UUID, rawURL string) []*http.Cookie
}

// CollectionReader is the read side used by script resolution and draft creation.
type CollectionReader interface {
	GetByID(ctx context.Context, id uuid.UUID) (*entities.Collection, error)
	// ListByWorkspace returns all non-deleted collections in the workspace,
	// sorted by sort_order ASC. Used to pick a host collection for drafts.
	ListByWorkspace(ctx context.Context, workspaceID uuid.UUID) ([]*entities.Collection, error)
}

// TokenCleaner invalidates OAuth 2.0 tokens whose owner was deleted, moved to another workspace
// or rewritten; the request usecase never reads tokens itself.
type TokenCleaner interface {
	Clear(ctx context.Context, owner entities.AuthOwner) error
	ClearOwners(ctx context.Context, kind string, ids []uuid.UUID) error
	ClearOwnersUnlessHash(ctx context.Context, kind string, ids []uuid.UUID, keepHash string) error
	DeleteOrphans(ctx context.Context) (int, error)
}

// noopTokenCleaner stands in for test constructors built without a token store.
type noopTokenCleaner struct{}

func (noopTokenCleaner) Clear(context.Context, entities.AuthOwner) error { return nil }

func (noopTokenCleaner) ClearOwners(context.Context, string, []uuid.UUID) error { return nil }

func (noopTokenCleaner) ClearOwnersUnlessHash(context.Context, string, []uuid.UUID, string) error {
	return nil
}

func (noopTokenCleaner) DeleteOrphans(context.Context) (int, error) { return 0, nil }

// ScriptResolver walks up the collection hierarchy when the request has no script of its own.
type ScriptResolver interface {
	ResolvePreScript(ctx context.Context, req *entities.Request) (string, error)
	ResolvePostScript(ctx context.Context, req *entities.Request) (string, error)
}

// VariablePersister saves script-modified variables back to the active environment.
type VariablePersister interface {
	PersistVariableChanges(ctx context.Context, workspaceID uuid.UUID, userID string, newVars map[string]string) error
}

// RequestAuth carries a scheme the requester applies to the final request:
// digest needs the server's challenge, aws_sigv4 signs the assembled request.
type RequestAuth struct {
	Type   entities.AuthType
	Fields map[string]any
}

type HTTPExecuteRequest struct {
	Method      entities.HTTPMethod
	URL         string
	Headers     map[string][]string
	Body        string
	BodyReader  io.Reader
	WorkspaceID uuid.UUID // required for per-workspace cookie jar
	Auth        *RequestAuth
}

type ExecuteOpt struct {
	UserID      string
	WorkspaceID uuid.UUID
}

type BuildCurlOpt struct {
	WorkspaceID uuid.UUID
}

// CurlResult is a rendered curl command plus what the renderer could not do:
// an oauth2 request with no cached token ships without its Authorization header.
type CurlResult struct {
	Command      string
	Warnings     []string
	ScriptResult *entities.ScriptResult
}

type Usecase interface {
	Create(ctx context.Context, input Create, opt CreateOpt) (*entities.Request, error)
	GetByID(ctx context.Context, id uuid.UUID) (*entities.Request, error)
	List(ctx context.Context, opt ListOpt) ([]*entities.Request, error)
	Edit(ctx context.Context, input Edit, opt EditOpt) (*entities.Request, error)
	Delete(ctx context.Context, opt DeleteOpt) error
	Reorder(ctx context.Context, id uuid.UUID, sortOrder int) error
	Execute(ctx context.Context, id uuid.UUID, opt ExecuteOpt) (*entities.Response, error)
	BuildCurl(ctx context.Context, id uuid.UUID, opt BuildCurlOpt) (CurlResult, error)
	Move(ctx context.Context, opt MoveOpt) (*entities.Request, error)
	GRPCListServices(ctx context.Context, req GRPCConnectRequest) (*GRPCSchema, error)
	GRPCGenerateExample(ctx context.Context, req GRPCConnectRequest, service, method string) (string, error)
	GRPCGetProtoDefinition(ctx context.Context, req GRPCConnectRequest, service, method string) (string, error)
	GraphQLIntrospect(ctx context.Context, req GraphQLIntrospectRequest) (*GraphQLSchema, error)
	GraphQLGenerateExample(ctx context.Context, req GraphQLIntrospectRequest, operationName string) (*GraphQLExampleResponse, error)
	GraphQLGetTypeDefinition(ctx context.Context, req GraphQLIntrospectRequest, typeName string) (string, error)
	CreateDraftFromHistory(ctx context.Context, opt CreateDraftFromHistoryOpt) (*entities.Request, error)
	DeleteDraft(ctx context.Context, id uuid.UUID) error
	PromoteDraft(ctx context.Context, opt PromoteDraftOpt) (*entities.Request, error)
	CleanupDrafts(ctx context.Context) (int, error)
	ResolveWebSocket(ctx context.Context, requestID, workspaceID uuid.UUID, userID string) (websocket.ResolvedDial, error)
	SubstituteMessage(ctx context.Context, workspaceID uuid.UUID, text string) (string, error)
}

type usecase struct {
	repo             Repository
	historyRepo      HistoryRepository
	requester        HTTPRequester
	grpcRequester    GRPCRequester
	graphqlRequester GraphQLRequester
	envResolver      EnvironmentResolver
	scriptEngine     ScriptEngine
	scriptResolver   ScriptResolver
	varPersister     VariablePersister
	authResolver     AuthResolver
	cookieReader     CookieReader
	collectionReader CollectionReader
	tokenCleaner     TokenCleaner
	authProvider     auth.Provider
}

func NewUsecase(repo Repository, historyRepo HistoryRepository, requester HTTPRequester, grpcRequester GRPCRequester, graphqlRequester GraphQLRequester, envResolver EnvironmentResolver, scriptEngine ScriptEngine, scriptResolver ScriptResolver, varPersister VariablePersister, authResolver AuthResolver, cookieReader CookieReader, collectionReader CollectionReader, tokenCleaner TokenCleaner, authProvider auth.Provider) Usecase {
	return &usecase{
		repo:             repo,
		historyRepo:      historyRepo,
		requester:        requester,
		grpcRequester:    grpcRequester,
		graphqlRequester: graphqlRequester,
		envResolver:      envResolver,
		scriptEngine:     scriptEngine,
		scriptResolver:   scriptResolver,
		varPersister:     varPersister,
		authResolver:     authResolver,
		cookieReader:     cookieReader,
		collectionReader: collectionReader,
		tokenCleaner:     tokenCleaner,
		authProvider:     authProvider,
	}
}

// tokens returns the injected cleaner, or a no-op for usecases built without one.
func (u *usecase) tokens() TokenCleaner {
	if u.tokenCleaner == nil {
		return noopTokenCleaner{}
	}
	return u.tokenCleaner
}
