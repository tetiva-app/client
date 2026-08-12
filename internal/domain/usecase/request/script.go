package request

import "context"

// ScriptContext provides data to scripts during execution.
type ScriptContext struct {
	Variables          map[string]string
	RequestMethod      string
	RequestURL         string
	RequestHeaders     map[string][]string
	ResponseStatusCode int                 // post-script only
	ResponseBody       string              // post-script only
	ResponseHeaders    map[string][]string // post-script only
	ResponseStatusText string              // post-script only (gRPC status name)
	// gRPC-specific fields (nil/empty for HTTP)
	Protocol   string              // "http" | "grpc" | "graphql"
	Metadata   map[string][]string // gRPC metadata
	Message    string              // gRPC JSON body (alias for request body)
	Service    string              // gRPC fully-qualified service name
	GRPCMethod string              // gRPC method name
	// GraphQL-specific fields (empty for HTTP/gRPC)
	GraphQLVariables string
	GraphQLOperation string
}

// PreScriptResult holds the output of a pre-request script.
type PreScriptResult struct {
	Headers       map[string][]string
	Metadata      map[string][]string // gRPC metadata after the script; never the caller's map
	Variables     map[string]string
	ConsoleOutput []string
}

// PostScriptResult holds the output of a post-response script.
type PostScriptResult struct {
	TestResults   []TestResult
	Variables     map[string]string
	ConsoleOutput []string
}

// TestResult represents a single pm.test() assertion result.
type TestResult struct {
	Name   string
	Passed bool
	Error  string
}

// ScriptEngine runs JavaScript scripts in a sandboxed environment.
type ScriptEngine interface {
	RunPreScript(ctx context.Context, script string, sctx ScriptContext) (*PreScriptResult, error)
	RunPostScript(ctx context.Context, script string, sctx ScriptContext) (*PostScriptResult, error)
}
