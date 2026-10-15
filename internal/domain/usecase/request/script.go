package request

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/tetiva-app/client/internal/domain"
)

const ReasonPreScriptFailed = "pre_script_failed"

func preScriptFailed(err error, script ResolvedScript, result *PreScriptResult) error {
	fields := map[string]string{"message": err.Error()}
	var runErr *ScriptRunError
	if errors.As(err, &runErr) {
		fields["message"] = runErr.Message
		if runErr.Line > 0 {
			fields["line"] = strconv.Itoa(runErr.Line)
			fields["column"] = strconv.Itoa(runErr.Column)
		}
		if runErr.Timeout {
			fields["timeout"] = "true"
		}
	}
	if script.Collection != "" {
		fields["collection"] = script.Collection
	}
	if len(result.ConsoleOutput) > 0 {
		console, _ := json.Marshal(result.ConsoleOutput)
		fields["console"] = string(console)
	}
	return &domain.ReasonError{
		Reason: ReasonPreScriptFailed,
		Err:    fmt.Errorf("pre-request script failed: %w", err),
		Fields: fields,
	}
}

func withLocals(vars, locals map[string]string) map[string]string {
	if len(locals) == 0 {
		return vars
	}
	out := make(map[string]string, len(vars)+len(locals))
	for k, v := range vars {
		out[k] = v
	}
	for k, v := range locals {
		out[k] = v
	}
	return out
}

type ScriptContext struct {
	Variables          map[string]string
	LocalVariables     map[string]string // post-script only
	RequestMethod      string
	RequestURL         string
	RequestHeaders     map[string][]string
	ResponseStatusCode int                 // post-script only
	ResponseBody       string              // post-script only
	ResponseHeaders    map[string][]string // post-script only
	ResponseStatusText string              // post-script only (gRPC status name)
	ResponseTime       time.Duration       // post-script only
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

type PreScriptResult struct {
	Headers   map[string][]string
	Metadata  map[string][]string // gRPC metadata after the script; never the caller's map
	Variables map[string]string
	// LocalVariables are pm.variables: substituted into this request, never persisted.
	LocalVariables map[string]string
	ConsoleOutput  []string
}

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

type ScriptRunError struct {
	Message string
	Line    int
	Column  int
	Timeout bool
}

func (e *ScriptRunError) Error() string {
	if e.Line == 0 {
		return e.Message
	}
	return fmt.Sprintf("%s (line %d, column %d)", e.Message, e.Line, e.Column)
}

// ScriptEngine runs JavaScript scripts in a sandboxed environment.
// A failed run still returns its console output and test results alongside the error.
type ScriptEngine interface {
	RunPreScript(ctx context.Context, script string, sctx ScriptContext) (*PreScriptResult, error)
	RunPostScript(ctx context.Context, script string, sctx ScriptContext) (*PostScriptResult, error)
}
