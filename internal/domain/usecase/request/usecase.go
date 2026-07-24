package request

import (
	"context"
	"io"
	"net/http"

	"github.com/google/uuid"

	"github.com/tetiva-app/client/internal/domain/entities"
)

// Repository defines the persistence contract for Request usecase (ISP).
type Repository interface {
	Create(ctx context.Context, r *entities.Request) error
	GetByID(ctx context.Context, id uuid.UUID) (*entities.Request, error)
	List(ctx context.Context, filter Filter) ([]*entities.Request, error)
	Update(ctx context.Context, r *entities.Request) error
	UpdateSortOrder(ctx context.Context, id uuid.UUID, sortOrder int) error
	DeleteHard(ctx context.Context, id uuid.UUID) error
	CleanupDrafts(ctx context.Context) (int, error)
}

// HistoryRepository persists execution history (immutable, append-only).
type HistoryRepository interface {
	Create(ctx context.Context, h *entities.History) error
	GetByID(ctx context.Context, id uuid.UUID) (*entities.History, error)
}

// HTTPRequester sends HTTP requests.
type HTTPRequester interface {
	Execute(ctx context.Context, req HTTPExecuteRequest) (*entities.Response, error)
}

// GRPCRequester sends gRPC requests.
type GRPCRequester interface {
	Execute(ctx context.Context, req GRPCExecuteRequest) (*entities.Response, error)
	ListServices(ctx context.Context, req GRPCConnectRequest) (*GRPCSchema, error)
}

// GRPCExecuteRequest contains resolved gRPC request data ready to send.
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

// GRPCConnectRequest is used to connect and list services.
type GRPCConnectRequest struct {
	Host        string
	UseTLS      bool
	ProtoPath   string
	WorkspaceID uuid.UUID
}

// GRPCSchema holds discovered services and methods.
type GRPCSchema struct {
	Services []GRPCService
	Source   string // "reflection" | "proto_file" | "proto_directory"
}

// GRPCService represents a single gRPC service.
type GRPCService struct {
	FullName string
	Methods  []GRPCMethodInfo
}

// GRPCMethodInfo holds info about a single gRPC method.
type GRPCMethodInfo struct {
	Name            string
	InputType       string
	OutputType      string
	IsServerStream  bool
	IsClientStream  bool
	ProtoDefinition string
	ExampleJSON     string
}

// GraphQLRequester sends GraphQL requests and performs introspection.
type GraphQLRequester interface {
	Execute(ctx context.Context, req GraphQLExecuteRequest) (*entities.Response, error)
	Introspect(ctx context.Context, req GraphQLIntrospectRequest) (*GraphQLSchema, error)
	GenerateExampleQuery(schema *GraphQLSchema, operationName string) (*GraphQLExampleResponse, error)
}

// GraphQLExecuteRequest contains resolved GraphQL request data ready to send.
type GraphQLExecuteRequest struct {
	Endpoint      string
	Query         string
	Variables     string // JSON
	OperationName string
	Headers       map[string][]string
}

// GraphQLIntrospectRequest is used to load schema via introspection or file.
// Exactly one of Endpoint or SchemaPath must be non-empty.
type GraphQLIntrospectRequest struct {
	Endpoint   string
	SchemaPath string
	Headers    map[string][]string
}

// GraphQLSchema holds discovered queries, mutations, and types.
type GraphQLSchema struct {
	Queries   []GraphQLOperation
	Mutations []GraphQLOperation
	Types     []GraphQLType
	Source    string // "introspection" | "schema_file"
}

// GraphQLOperation represents a single query or mutation.
type GraphQLOperation struct {
	Name       string
	Args       []GraphQLArg
	ReturnType string
	Definition string // SDL snippet for schema viewer
}

// GraphQLArg represents a GraphQL argument.
type GraphQLArg struct {
	Name         string
	Type         string // e.g. "ID!", "[String]", "CreateUserInput!"
	DefaultValue string
}

// GraphQLType represents a GraphQL type definition.
type GraphQLType struct {
	Name          string
	Kind          string // OBJECT, INPUT_OBJECT, ENUM, INTERFACE, UNION, SCALAR
	Fields        []GraphQLField
	EnumValues    []string
	PossibleTypes []string // for INTERFACE and UNION
	Definition    string   // SDL for this type
}

// GraphQLField represents a field within a GraphQL type.
type GraphQLField struct {
	Name string
	Type string // e.g. "String!", "[Order!]"
	Args []GraphQLArg
}

// GraphQLExampleResponse holds a generated example query and variables.
type GraphQLExampleResponse struct {
	Query     string
	Variables string // JSON
}

// EnvironmentResolver resolves environment variables for a workspace.
type EnvironmentResolver interface {
	ResolveVariables(ctx context.Context, workspaceID uuid.UUID) (map[string]string, error)
}

// CookieReader returns cookies to send with a request URL. workspaceID is ignored
// by the in-memory jar but kept in the signature for a per-workspace persisted jar.
type CookieReader interface {
	CookiesFor(ctx context.Context, workspaceID uuid.UUID, rawURL string) []*http.Cookie
}

// CollectionReader provides read access to collections for script resolution
// and draft creation.
type CollectionReader interface {
	GetByID(ctx context.Context, id uuid.UUID) (*entities.Collection, error)
	// ListByWorkspace returns all non-deleted collections in the workspace,
	// sorted by sort_order ASC. Used to pick a host collection for drafts.
	ListByWorkspace(ctx context.Context, workspaceID uuid.UUID) ([]*entities.Collection, error)
}

// ScriptResolver resolves the effective pre/post script for a request,
// walking up the collection hierarchy if the request has no script of its own.
type ScriptResolver interface {
	ResolvePreScript(ctx context.Context, req *entities.Request) (string, error)
	ResolvePostScript(ctx context.Context, req *entities.Request) (string, error)
}

// VariablePersister saves script-modified variables back to the active environment.
type VariablePersister interface {
	PersistVariableChanges(ctx context.Context, workspaceID uuid.UUID, userID string, newVars map[string]string) error
}

// HTTPExecuteRequest contains resolved request data ready to send.
type HTTPExecuteRequest struct {
	Method      entities.HTTPMethod
	URL         string
	Headers     map[string][]string
	Body        string
	BodyReader  io.Reader
	WorkspaceID uuid.UUID // required for per-workspace cookie jar
}

// ExecuteOpt holds contextual options for the Execute operation.
type ExecuteOpt struct {
	UserID      string
	WorkspaceID uuid.UUID
}

// BuildCurlOpt holds contextual options for the BuildCurl operation.
type BuildCurlOpt struct {
	WorkspaceID uuid.UUID
}

// Usecase defines the public API for Request operations.
type Usecase interface {
	Create(ctx context.Context, input Create, opt CreateOpt) (*entities.Request, error)
	GetByID(ctx context.Context, id uuid.UUID) (*entities.Request, error)
	List(ctx context.Context, opt ListOpt) ([]*entities.Request, error)
	Edit(ctx context.Context, input Edit, opt EditOpt) (*entities.Request, error)
	Delete(ctx context.Context, opt DeleteOpt) error
	Reorder(ctx context.Context, id uuid.UUID, sortOrder int) error
	Execute(ctx context.Context, id uuid.UUID, opt ExecuteOpt) (*entities.Response, error)
	BuildCurl(ctx context.Context, id uuid.UUID, opt BuildCurlOpt) (string, *entities.ScriptResult, error)
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
	ResolveWebSocket(ctx context.Context, requestID, workspaceID uuid.UUID, userID string) (url string, headers map[string][]string, err error)
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
}

// NewUsecase creates a new Request usecase instance.
func NewUsecase(repo Repository, historyRepo HistoryRepository, requester HTTPRequester, grpcRequester GRPCRequester, graphqlRequester GraphQLRequester, envResolver EnvironmentResolver, scriptEngine ScriptEngine, scriptResolver ScriptResolver, varPersister VariablePersister, authResolver AuthResolver, cookieReader CookieReader, collectionReader CollectionReader) Usecase {
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
	}
}
