package entities

import "time"

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

type ScriptResult struct {
	PreConsole  []string
	PostConsole []string
	Tests       []ScriptTestResult
	Errors      []ScriptError
}

type ScriptTestResult struct {
	Name   string
	Passed bool
	Error  string
}

type ScriptError struct {
	Phase   string
	Message string
}
