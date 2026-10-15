package request

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/tetiva-app/client/internal/domain"
	"github.com/tetiva-app/client/internal/domain/entities"
	"github.com/tetiva-app/client/internal/domain/usecase/auth"
)

// reservedHeaders cannot be overwritten by API Key auth.
var reservedHeaders = map[string]bool{
	"host":              true,
	"content-length":    true,
	"authorization":     true,
	"connection":        true,
	"transfer-encoding": true,
}

// applyHeaderAuth applies the header-layer schemes and names the query parameters it injected, so
// history replay can strip them; oauth2, digest and aws_sigv4 live in other layers.
func applyHeaderAuth(authType entities.AuthType, f auth.Fields, headers map[string][]string, rawURL string) (map[string][]string, string, []string, error) {
	switch authType {
	case entities.AuthTypeNone, entities.AuthTypeInherit, "":
		return headers, rawURL, nil, nil
	case entities.AuthTypeBasic, entities.AuthTypeBearer, entities.AuthTypeAPIKey, entities.AuthTypeJWT:
	default:
		return headers, rawURL, nil, fmt.Errorf("unsupported auth type %q", authType)
	}
	if len(f) == 0 {
		return headers, rawURL, nil, nil
	}

	var queryKeys []string

	switch authType {
	case entities.AuthTypeBasic:
		creds := base64.StdEncoding.EncodeToString([]byte(f.Str("username") + ":" + f.Str("password")))
		headers["Authorization"] = []string{"Basic " + creds}

	case entities.AuthTypeBearer:
		token := f.Str("token")
		prefix := "Bearer"
		if _, ok := f["prefix"]; ok {
			prefix = f.Str("prefix")
		}
		if prefix == "" {
			headers["Authorization"] = []string{token}
		} else {
			headers["Authorization"] = []string{prefix + " " + token}
		}

	case entities.AuthTypeAPIKey:
		key := f.Str("key")
		value := f.Str("value")
		if apiKeyAddTo(f) == "query" {
			parsed, err := url.Parse(rawURL)
			if err != nil {
				return headers, rawURL, nil, fmt.Errorf("invalid URL for API key: %w", err)
			}
			q := parsed.Query()
			q.Set(key, value)
			parsed.RawQuery = q.Encode()
			rawURL = parsed.String()
			if key != "" {
				queryKeys = append(queryKeys, key)
			}
		} else {
			if reservedHeaders[strings.ToLower(key)] {
				return headers, rawURL, nil, fmt.Errorf("API key header %q is reserved and cannot be overwritten", key)
			}
			headers[key] = []string{value}
		}

	case entities.AuthTypeJWT:
		return applyJWT(f, headers, rawURL, time.Now())
	}

	return headers, rawURL, queryKeys, nil
}

func apiKeyAddTo(f auth.Fields) string {
	if addTo := f.Str("addTo"); addTo != "" {
		return addTo
	}
	return f.Str("in") // legacy key from Postman imports
}

func authHeaderKeys(authType entities.AuthType, f auth.Fields) []string {
	if authType != entities.AuthTypeAPIKey || apiKeyAddTo(f) == "query" || f.Str("key") == "" {
		return nil
	}
	return []string{f.Str("key")}
}

// formField's JSON tags describe the wire format the frontend writes into
// Request.Body for BodyTypeForm — not a domain contract; it never escapes this package.
type formField struct {
	Key     string `json:"key"`
	Value   string `json:"value"`
	Type    string `json:"type"`
	Enabled bool   `json:"enabled"`
}

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

func hasFileField(fields []formField) bool {
	for _, f := range fields {
		if f.Enabled && f.Type == "file" && f.Key != "" {
			return true
		}
	}
	return false
}

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

func (u *usecase) Execute(ctx context.Context, id uuid.UUID, opt ExecuteOpt) (*entities.Response, error) {
	const funcName = "request.Execute"

	req, err := u.repo.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", funcName, err)
	}
	if req == nil {
		return nil, &domain.NotFoundError{Entity: "request", ID: id.String()}
	}

	resolvedAuth, err := u.resolveAuthFor(ctx, req, opt.WorkspaceID)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", funcName, err)
	}
	if req.Protocol != entities.ProtocolHTTP {
		if authErr := rejectNonHTTPAuth(resolvedAuth.Type); authErr != nil {
			return nil, fmt.Errorf("%s: %w", funcName, authErr)
		}
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
	var historyAuthQueryKeys, historyAuthHeaderKeys []string
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
		authFields, fieldsErr := auth.ParseFields(resolvedAuth.Data)
		if fieldsErr != nil {
			return nil, fmt.Errorf("%s: %w", funcName, fieldsErr)
		}
		authFields = auth.Substitute(authFields, vars)
		headers, execURL, historyAuthQueryKeys, _, err = u.applyResolvedAuth(ctx, resolvedAuth, authFields, headers, execURL, prepareOpt{UserID: opt.UserID})
		if err != nil {
			return nil, fmt.Errorf("%s: %w", funcName, err)
		}
		historyAuthHeaderKeys = authHeaderKeys(resolvedAuth.Type, authFields)
		graphqlQuery := substituteVariables(req.GraphQLQuery, vars)
		graphqlVars := substituteVariables(req.GraphQLVariables, vars)
		graphqlReq := GraphQLExecuteRequest{
			Endpoint:      execURL,
			Query:         graphqlQuery,
			Variables:     graphqlVars,
			OperationName: req.GraphQLOperation,
			Headers:       headers,
			WorkspaceID:   opt.WorkspaceID,
			AuthQueryKeys: historyAuthQueryKeys,
		}
		resp, execErr = u.graphqlRequester.Execute(ctx, graphqlReq)
		historyHeaders = headers
		historyBody = req.GraphQLQuery
		_ = body

	default: // HTTP
		prep, sr, prepErr := u.prepareHTTP(ctx, req, vars, resolvedAuth, prepareOpt{UserID: opt.UserID})
		if prepErr != nil {
			return nil, fmt.Errorf("%s: %w", funcName, prepErr)
		}
		scriptResult = sr
		vars = prep.Vars
		execURL = prep.URL
		execReq := HTTPExecuteRequest{
			Method:        prep.Method,
			URL:           prep.URL,
			Headers:       prep.Headers,
			Body:          prep.Body,
			WorkspaceID:   opt.WorkspaceID,
			Auth:          prep.Auth,
			AuthQueryKeys: prep.AuthQueryKeys,
		}
		if mp := prep.Multipart; mp != nil {
			body := mp.open()
			defer func() { _ = body.Close() }()
			execReq.BodyReader = body
			execReq.BodyLength = mp.size
			execReq.GetBody = func() (io.ReadCloser, error) { return mp.open(), nil }
		}
		if prep.BinaryPath != "" {
			file, openErr := openBodyFile(prep.BinaryPath)
			if openErr != nil {
				return nil, fmt.Errorf("%s: %w", funcName, openErr)
			}
			defer func() { _ = file.Close() }()
			info, statErr := file.Stat()
			if statErr != nil {
				return nil, fmt.Errorf("%s: %w", funcName, statErr)
			}
			execReq.BodyReader = file
			execReq.BodyLength = info.Size()
			execReq.GetBody = func() (io.ReadCloser, error) { return openBodyFile(prep.BinaryPath) }
		}
		resp, execErr = u.requester.Execute(ctx, execReq)
		historyHeaders = prep.Headers
		historyBody = prep.RawBody
		historyAuthQueryKeys = prep.AuthQueryKeys
		historyAuthHeaderKeys = prep.AuthHeaderKeys
		if req.BodyType == entities.BodyTypeBinary {
			historyBody = "[binary: " + prep.BinaryPath + "]"
		}
	}

	// gRPC reports a cancel as a Canceled response rather than an error, so ctx decides.
	if ctxErr := ctx.Err(); ctxErr != nil {
		removeBinaryBody(resp)
		return nil, fmt.Errorf("%s: %w", funcName, ctxErr)
	}

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
			ResponseTime:       resp.Duration,
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

	if ctxErr := ctx.Err(); ctxErr != nil {
		removeBinaryBody(resp)
		return nil, fmt.Errorf("%s: %w", funcName, ctxErr)
	}
	// A cancel from here on lets the run commit whole instead of half.
	commitCtx := context.WithoutCancel(ctx)

	if resp != nil && scriptResult != nil {
		resp.ScriptResult = scriptResult
	}
	if resp != nil {
		sentHeaders := historyHeaders
		if req.Protocol == entities.ProtocolGRPC {
			sentHeaders = grpcRequestMetadata
		}
		resp.Sent = &entities.SentRequest{
			URL: execURL, Headers: sentHeaders, AuthQueryKeys: historyAuthQueryKeys, AuthHeaderKeys: historyAuthHeaderKeys,
		}
	}

	if u.varPersister != nil && scriptResult != nil && len(vars) > 0 {
		if persistErr := u.varPersister.PersistVariableChanges(commitCtx, opt.WorkspaceID, opt.UserID, vars); persistErr != nil {
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
		AuthQueryKeys:   historyAuthQueryKeys,
		AuthHeaderKeys:  historyAuthHeaderKeys,
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

	if saveErr := u.historyRepo.Create(commitCtx, history); saveErr != nil {
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

func removeBinaryBody(resp *entities.Response) {
	if resp != nil && resp.BinaryPath != "" {
		_ = os.Remove(resp.BinaryPath)
	}
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
	// The script sees the raw placeholders; its header map is substituted once
	// afterwards, with whatever variables the script left behind.
	rawHeaders := entities.EnabledHeadersToMap(req.Headers)
	execURL := substituteVariables(req.URL, vars)
	body := substituteVariables(req.Body, vars)

	out := preScriptOutcome{Vars: vars, URL: execURL, Body: body, Headers: substituteHeaders(rawHeaders, vars), Metadata: req.GRPCMetadata}

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
		RequestHeaders: rawHeaders,
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
	if req.Protocol == entities.ProtocolGRPC {
		out.Metadata = preResult.Metadata
	}
	if !mapsEqual(vars, preResult.Variables) {
		out.Vars = preResult.Variables
		out.URL = substituteVariables(req.URL, out.Vars)
		out.Body = substituteVariables(req.Body, out.Vars)
	}
	out.Headers = substituteHeaders(preResult.Headers, out.Vars)
	return out
}
