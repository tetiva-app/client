package entities

import "time"

// Response represents the result of executing an HTTP or gRPC request.
type Response struct {
	StatusCode        int
	StatusText        string
	URL               string // Final, env-resolved URL the request actually went to.
	Headers           map[string][]string
	Body              string
	Size              int64
	Duration          time.Duration
	Protocol          Protocol
	ScriptResult      *ScriptResult
	IsBinary          bool
	BinaryPath        string
	SuggestedFilename string
}

// ScriptResult aggregates results from pre/post script execution.
type ScriptResult struct {
	PreConsole  []string
	PostConsole []string
	Tests       []ScriptTestResult
	Errors      []ScriptError
}

// ScriptTestResult represents a single test assertion result.
type ScriptTestResult struct {
	Name   string
	Passed bool
	Error  string
}

// ScriptError represents a script execution error.
type ScriptError struct {
	Phase   string
	Message string
}
