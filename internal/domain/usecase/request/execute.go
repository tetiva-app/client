package request

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/tetiva-app/client/internal/domain"
	"github.com/tetiva-app/client/internal/domain/entities"
)

// reservedHeaders cannot be overwritten by API Key auth.
var reservedHeaders = map[string]bool{
	"host":              true,
	"content-length":    true,
	"authorization":     true,
	"connection":        true,
	"transfer-encoding": true,
}

// applyAuth adds authentication headers/query params based on auth config.
func applyAuth(authType entities.AuthType, authData string, headers map[string][]string, rawURL string) (map[string][]string, string, error) {
	if authType == entities.AuthTypeNone || authType == entities.AuthTypeInherit || authType == "" {
		return headers, rawURL, nil
	}
	if authData == "" || authData == "{}" {
		return headers, rawURL, nil
	}

	var data map[string]string
	if err := json.Unmarshal([]byte(authData), &data); err != nil {
		return headers, rawURL, fmt.Errorf("invalid auth_data JSON: %w", err)
	}

	switch authType {
	case entities.AuthTypeBasic:
		username := data["username"]
		password := data["password"]
		creds := base64.StdEncoding.EncodeToString([]byte(username + ":" + password))
		headers["Authorization"] = []string{"Basic " + creds}

	case entities.AuthTypeBearer:
		token := data["token"]
		prefix, hasPrefix := data["prefix"]
		if !hasPrefix {
			prefix = "Bearer"
		}
		if prefix == "" {
			headers["Authorization"] = []string{token}
		} else {
			headers["Authorization"] = []string{prefix + " " + token}
		}

	case entities.AuthTypeAPIKey:
		key := data["key"]
		value := data["value"]
		addTo := data["addTo"]
		if addTo == "query" {
			parsed, err := url.Parse(rawURL)
			if err != nil {
				return headers, rawURL, fmt.Errorf("invalid URL for API key: %w", err)
			}
			q := parsed.Query()
			q.Set(key, value)
			parsed.RawQuery = q.Encode()
			rawURL = parsed.String()
		} else {
			if reservedHeaders[strings.ToLower(key)] {
				return headers, rawURL, fmt.Errorf("API key header %q is reserved and cannot be overwritten", key)
			}
			headers[key] = []string{value}
		}
	}

	return headers, rawURL, nil
}

// formField's JSON tags describe the wire format the frontend writes into
// Request.Body for BodyTypeForm — not a domain contract; it never escapes this package.
type formField struct {
	Key     string `json:"key"`
	Value   string `json:"value"`
	Type    string `json:"type"`
	Enabled bool   `json:"enabled"`
}

// parseFormFields parses JSON array of form fields.
func parseFormFields(body string) ([]formField, error) {
	if body == "" || body == "[]" {
		return nil, nil
	}
	var fields []formField
	if err := json.Unmarshal([]byte(body), &fields); err != nil {
		return nil, fmt.Errorf("invalid form body JSON: %w", err)
	}
	return fields, nil
}

// FormHasFiles returns true if any enabled field has type "file".
func FormHasFiles(body string) bool {
	fields, err := parseFormFields(body)
	if err != nil {
		return false
	}
	for _, f := range fields {
		if f.Enabled && f.Type == "file" && f.Key != "" {
			return true
		}
	}
	return false
}

// encodeFormBody converts JSON array of form fields to url-encoded string.
func encodeFormBody(body string) (string, error) {
	fields, err := parseFormFields(body)
	if err != nil {
		return "", err
	}

	values := url.Values{}
	for _, f := range fields {
		if f.Enabled && f.Key != "" {
			values.Add(f.Key, f.Value)
		}
	}
	return values.Encode(), nil
}

// encodeMultipartFormBody encodes form fields as multipart/form-data, including file fields.
// Returns the Content-Type (with boundary) and an io.Reader for the body.
func encodeMultipartFormBody(body string) (string, io.Reader, error) {
	fields, err := parseFormFields(body)
	if err != nil {
		return "", nil, err
	}

	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)

	for _, f := range fields {
		if !f.Enabled || f.Key == "" {
			continue
		}

		if f.Type == "file" && f.Value != "" {
			// Security: reject path traversal
			if strings.Contains(f.Value, "..") {
				return "", nil, fmt.Errorf("form file path traversal is not allowed: %s", f.Key)
			}
			cleanPath := filepath.Clean(f.Value)
			if !filepath.IsAbs(cleanPath) {
				return "", nil, fmt.Errorf("form file path must be absolute: %s", f.Key)
			}

			file, openErr := os.Open(cleanPath)
			if openErr != nil {
				return "", nil, fmt.Errorf("form file not found %q: %w", f.Key, openErr)
			}

			part, partErr := writer.CreateFormFile(f.Key, filepath.Base(cleanPath))
			if partErr != nil {
				_ = file.Close()
				return "", nil, fmt.Errorf("failed to create form file part %q: %w", f.Key, partErr)
			}

			if _, copyErr := io.Copy(part, file); copyErr != nil {
				_ = file.Close()
				return "", nil, fmt.Errorf("failed to copy file %q: %w", f.Key, copyErr)
			}
			if closeErr := file.Close(); closeErr != nil {
				return "", nil, fmt.Errorf("failed to close form file %q: %w", f.Key, closeErr)
			}
		} else {
			if writeErr := writer.WriteField(f.Key, f.Value); writeErr != nil {
				return "", nil, fmt.Errorf("failed to write field %q: %w", f.Key, writeErr)
			}
		}
	}

	if err := writer.Close(); err != nil {
		return "", nil, fmt.Errorf("failed to close multipart writer: %w", err)
	}

	return writer.FormDataContentType(), &buf, nil
}

// autoContentType returns the Content-Type for a body type, or empty if none.
func autoContentType(bodyType entities.BodyType) string {
	switch bodyType {
	case entities.BodyTypeJSON:
		return "application/json"
	case entities.BodyTypeXML:
		return "application/xml"
	case entities.BodyTypeForm:
		return "application/x-www-form-urlencoded"
	case entities.BodyTypeBinary:
		return "application/octet-stream"
	default:
		return ""
	}
}

// Execute loads a request by ID, applies auth, encodes body, sends it via HTTPRequester,
// saves history, and returns the response.
func (u *usecase) Execute(ctx context.Context, id uuid.UUID, opt ExecuteOpt) (*entities.Response, error) {
	const funcName = "request.Execute"

	req, err := u.repo.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", funcName, err)
	}
	if req == nil {
		return nil, &domain.NotFoundError{Entity: "request", ID: id.String()}
	}

	vars, err := u.envResolver.ResolveVariables(ctx, opt.WorkspaceID)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", funcName, err)
	}

	var resp *entities.Response
	var execErr error
	var scriptResult *entities.ScriptResult
	var execURL string
	var historyHeaders map[string][]string
	var historyBody string
	var grpcRequestMetadata map[string][]string

	switch req.Protocol {
	case entities.ProtocolWebSocket:
		return nil, &domain.ValidationError{Fields: map[string]string{"protocol": "use Connect for websocket, not Execute"}}

	case entities.ProtocolGRPC:
		pre := u.runPreScriptForNonHTTP(ctx, req, vars)
		scriptResult, vars, execURL = pre.ScriptResult, pre.Vars, pre.URL
		grpcMetadata := substituteMetadata(pre.Metadata, vars)
		grpcReq := GRPCExecuteRequest{
			Host:      execURL,
			Service:   req.GRPCService,
			Method:    req.GRPCMethod,
			Message:   pre.Body,
			Metadata:  grpcMetadata,
			ProtoPath: req.GRPCProtoPath,
		}
		resp, execErr = u.grpcRequester.Execute(ctx, grpcReq)
		historyHeaders = pre.Headers
		historyBody = pre.Body
		grpcRequestMetadata = grpcMetadata

	case entities.ProtocolGraphQL:
		pre := u.runPreScriptForNonHTTP(ctx, req, vars)
		scriptResult, vars, execURL = pre.ScriptResult, pre.Vars, pre.URL
		headers, body := pre.Headers, pre.Body
		resolvedAuthType, resolvedAuthData, authErr := u.authResolver.ResolveAuth(ctx, req)
		if authErr != nil {
			return nil, fmt.Errorf("%s: %w", funcName, authErr)
		}
		if resolvedAuthData != "" && resolvedAuthData != "{}" {
			resolvedAuthData = substituteVariables(resolvedAuthData, vars)
		}
		headers, execURL, err = applyAuth(resolvedAuthType, resolvedAuthData, headers, execURL)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", funcName, err)
		}
		graphqlQuery := substituteVariables(req.GraphQLQuery, vars)
		graphqlVars := substituteVariables(req.GraphQLVariables, vars)
		graphqlReq := GraphQLExecuteRequest{
			Endpoint:      execURL,
			Query:         graphqlQuery,
			Variables:     graphqlVars,
			OperationName: req.GraphQLOperation,
			Headers:       headers,
		}
		resp, execErr = u.graphqlRequester.Execute(ctx, graphqlReq)
		historyHeaders = headers
		historyBody = req.GraphQLQuery
		_ = body

	default: // HTTP
		prep, sr, prepErr := u.prepareHTTP(ctx, req, vars, prepareOpt{})
		if prepErr != nil {
			return nil, fmt.Errorf("%s: %w", funcName, prepErr)
		}
		scriptResult = sr
		vars = prep.Vars
		execURL = prep.URL
		execReq := HTTPExecuteRequest{
			Method:      prep.Method,
			URL:         prep.URL,
			Headers:     prep.Headers,
			Body:        prep.Body,
			WorkspaceID: opt.WorkspaceID,
		}
		if prep.BodyReader != nil {
			execReq.BodyReader = prep.BodyReader
		}
		if prep.BinaryPath != "" {
			file, openErr := os.Open(prep.BinaryPath)
			if openErr != nil {
				return nil, fmt.Errorf("%s: file not found: %w", funcName, openErr)
			}
			defer func() { _ = file.Close() }()
			execReq.BodyReader = file
		}
		resp, execErr = u.requester.Execute(ctx, execReq)
		historyHeaders = prep.Headers
		historyBody = prep.RawBody
		if req.BodyType == entities.BodyTypeBinary {
			historyBody = "[binary: " + prep.BinaryPath + "]"
		}
	}

	// Post-script (all protocols)
	postScript, postResolveErr := u.scriptResolver.ResolvePostScript(ctx, req)
	if postResolveErr != nil {
		return nil, fmt.Errorf("%s: %w", funcName, postResolveErr)
	}
	if postScript != "" && u.scriptEngine != nil && resp != nil {
		if scriptResult == nil {
			scriptResult = &entities.ScriptResult{}
		}
		postCtx := ScriptContext{
			Variables:          vars,
			RequestMethod:      string(req.Method),
			RequestURL:         execURL,
			RequestHeaders:     historyHeaders,
			ResponseStatusCode: resp.StatusCode,
			ResponseBody:       resp.Body,
			ResponseHeaders:    resp.Headers,
			ResponseStatusText: resp.StatusText,
			Protocol:           string(req.Protocol),
		}
		if req.Protocol == entities.ProtocolGRPC {
			postCtx.Metadata = grpcRequestMetadata
			postCtx.Service = req.GRPCService
			postCtx.GRPCMethod = req.GRPCMethod
		}
		if req.Protocol == entities.ProtocolGraphQL {
			postCtx.GraphQLVariables = substituteVariables(req.GraphQLVariables, vars)
			postCtx.GraphQLOperation = req.GraphQLOperation
		}
		postResult, postErr := u.scriptEngine.RunPostScript(ctx, postScript, postCtx)
		if postErr != nil {
			scriptResult.Errors = append(scriptResult.Errors, entities.ScriptError{
				Phase: "post-script", Message: postErr.Error(),
			})
		} else {
			scriptResult.PostConsole = postResult.ConsoleOutput
			vars = postResult.Variables
			for _, tr := range postResult.TestResults {
				scriptResult.Tests = append(scriptResult.Tests, entities.ScriptTestResult{
					Name: tr.Name, Passed: tr.Passed, Error: tr.Error,
				})
			}
		}
	}

	if resp != nil && scriptResult != nil {
		resp.ScriptResult = scriptResult
	}

	if u.varPersister != nil && scriptResult != nil && len(vars) > 0 {
		if persistErr := u.varPersister.PersistVariableChanges(ctx, opt.WorkspaceID, opt.UserID, vars); persistErr != nil {
			scriptResult.Errors = append(scriptResult.Errors, entities.ScriptError{
				Phase: "variable-persist", Message: persistErr.Error(),
			})
		}
	}

	var historyMethod, historyURL string
	switch req.Protocol {
	case entities.ProtocolGRPC:
		historyMethod = req.GRPCService + "/" + req.GRPCMethod
		historyURL = execURL + "/" + req.GRPCService + "/" + req.GRPCMethod
	case entities.ProtocolGraphQL:
		historyMethod = "GRAPHQL"
		if req.GraphQLOperation != "" {
			historyMethod = req.GraphQLOperation
		}
		historyURL = execURL
	default:
		historyMethod = string(req.Method)
		historyURL = execURL
	}

	history := &entities.History{
		ID:              uuid.New(),
		RequestID:       req.ID,
		WorkspaceID:     opt.WorkspaceID,
		Protocol:        req.Protocol,
		Method:          historyMethod,
		URL:             historyURL,
		RequestHeaders:  historyHeaders,
		RequestBody:     historyBody,
		ResponseHeaders: make(map[string][]string),
		CreatedAt:       time.Now(),
	}

	if resp != nil {
		history.ResponseStatus = resp.StatusCode
		history.ResponseHeaders = resp.Headers
		history.ResponseBody = resp.Body
		history.ResponseSize = resp.Size
		history.DurationMs = resp.Duration.Milliseconds()
	}

	if execErr != nil {
		history.ErrorMessage = execErr.Error()
	}

	if saveErr := u.historyRepo.Create(ctx, history); saveErr != nil {
		if execErr != nil {
			return nil, fmt.Errorf("%s: %w (history save also failed: %v)", funcName, execErr, saveErr)
		}
		return nil, fmt.Errorf("%s: failed to save history: %w", funcName, saveErr)
	}

	if execErr != nil {
		return nil, fmt.Errorf("%s: %w", funcName, execErr)
	}

	return resp, nil
}

func mapsEqual(a, b map[string]string) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if b[k] != v {
			return false
		}
	}
	return true
}

// preScriptOutcome carries what a pre-script may have changed on the gRPC/GraphQL paths.
type preScriptOutcome struct {
	ScriptResult *entities.ScriptResult
	Vars         map[string]string
	URL          string
	Body         string
	Headers      map[string][]string
	Metadata     map[string][]string // gRPC only; never the request's own map
}

// runPreScriptForNonHTTP runs pre-script for gRPC/GraphQL paths (HTTP uses prepareHTTP).
func (u *usecase) runPreScriptForNonHTTP(
	ctx context.Context,
	req *entities.Request,
	vars map[string]string,
) preScriptOutcome {
	headers := entities.EnabledHeadersToMap(req.Headers)
	execURL := substituteVariables(req.URL, vars)
	body := substituteVariables(req.Body, vars)
	headers = substituteHeaders(headers, vars)

	out := preScriptOutcome{Vars: vars, URL: execURL, Body: body, Headers: headers, Metadata: req.GRPCMetadata}

	preScript, preResolveErr := u.scriptResolver.ResolvePreScript(ctx, req)
	if preResolveErr != nil {
		out.ScriptResult = &entities.ScriptResult{
			Errors: []entities.ScriptError{{Phase: "pre-script", Message: preResolveErr.Error()}},
		}
		return out
	}
	if preScript == "" || u.scriptEngine == nil {
		return out
	}
	scriptResult := &entities.ScriptResult{}
	out.ScriptResult = scriptResult
	preCtx := ScriptContext{
		Variables:      vars,
		RequestMethod:  string(req.Method),
		RequestURL:     execURL,
		RequestHeaders: headers,
		Protocol:       string(req.Protocol),
	}
	if req.Protocol == entities.ProtocolGRPC {
		preCtx.Metadata = req.GRPCMetadata
		preCtx.Message = body
		preCtx.Service = req.GRPCService
		preCtx.GRPCMethod = req.GRPCMethod
	}
	if req.Protocol == entities.ProtocolGraphQL {
		preCtx.GraphQLVariables = substituteVariables(req.GraphQLVariables, vars)
		preCtx.GraphQLOperation = req.GraphQLOperation
	}
	preResult, preErr := u.scriptEngine.RunPreScript(ctx, preScript, preCtx)
	if preErr != nil {
		scriptResult.Errors = append(scriptResult.Errors, entities.ScriptError{
			Phase: "pre-script", Message: preErr.Error(),
		})
		return out
	}
	scriptResult.PreConsole = preResult.ConsoleOutput
	out.Headers = preResult.Headers
	if req.Protocol == entities.ProtocolGRPC {
		out.Metadata = preResult.Metadata
	}
	if !mapsEqual(vars, preResult.Variables) {
		out.Vars = preResult.Variables
		out.URL = substituteVariables(req.URL, out.Vars)
		out.Body = substituteVariables(req.Body, out.Vars)
		out.Headers = substituteHeaders(out.Headers, out.Vars)
	}
	return out
}
