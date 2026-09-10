package request

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/tetiva-app/client/internal/domain/entities"
	"github.com/tetiva-app/client/internal/domain/usecase/auth"
)

// preparedHTTP is the fully-resolved request shared by Execute and BuildCurl.
type preparedHTTP struct {
	Method     entities.HTTPMethod
	URL        string
	Headers    map[string][]string
	BodyType   entities.BodyType
	Body       string      // for JSON/XML/RAW/urlencoded form (wire-ready)
	RawBody    string      // body after var substitution but BEFORE encoding (for history/UI)
	BodyReader io.Reader   // for multipart with files (Execute path)
	FormFields []formField // for multipart, when curl needs -F flags
	BinaryPath string      // absolute path for BodyTypeBinary
	Vars       map[string]string

	// Auth is set for the schemes the requester applies itself (digest, aws_sigv4).
	Auth *RequestAuth
	// AuthQueryKeys names the query parameters auth injected, so history replay
	// can strip them; Warnings carry auth problems that did not stop the send.
	AuthQueryKeys []string
	Warnings      []string
}

// prepareOpt tunes the pipeline for callers that must not send anything:
// Copy as cURL renders from cached tokens only, never acquiring one.
type prepareOpt struct {
	TokenFromCacheOnly bool
	UserID             string
}

// prepareHTTP runs env vars → pre-script → auth → auto Content-Type → body encoding; ra is applied
// here so pre-script headers keep their place, and script errors are appended rather than fatal.
func (u *usecase) prepareHTTP(
	ctx context.Context,
	req *entities.Request,
	vars map[string]string,
	ra ResolvedAuth,
	opt prepareOpt,
) (preparedHTTP, *entities.ScriptResult, error) {
	const funcName = "request.prepareHTTP"

	// The script sees the raw placeholders; its header map is substituted once
	// afterwards, with whatever variables the script left behind.
	rawHeaders := entities.EnabledHeadersToMap(req.Headers)
	execURL := substituteVariables(req.URL, vars)
	body := substituteVariables(req.Body, vars)
	headers := substituteHeaders(rawHeaders, vars)

	var scriptResult *entities.ScriptResult
	preScript, preResolveErr := u.scriptResolver.ResolvePreScript(ctx, req)
	if preResolveErr != nil {
		return preparedHTTP{}, nil, fmt.Errorf("%s: %w", funcName, preResolveErr)
	}
	if preScript != "" && u.scriptEngine != nil {
		scriptResult = &entities.ScriptResult{}
		preCtx := ScriptContext{
			Variables:      vars,
			RequestMethod:  string(req.Method),
			RequestURL:     execURL,
			RequestHeaders: rawHeaders,
			Protocol:       string(req.Protocol),
		}
		preResult, preErr := u.scriptEngine.RunPreScript(ctx, preScript, preCtx)
		if preErr != nil {
			scriptResult.Errors = append(scriptResult.Errors, entities.ScriptError{
				Phase: "pre-script", Message: preErr.Error(),
			})
		} else {
			scriptResult.PreConsole = preResult.ConsoleOutput
			if !mapsEqual(vars, preResult.Variables) {
				vars = preResult.Variables
				execURL = substituteVariables(req.URL, vars)
				body = substituteVariables(req.Body, vars)
			}
			headers = substituteHeaders(preResult.Headers, vars)
		}
	}

	authFields, authErr := auth.ParseFields(ra.Data)
	if authErr != nil {
		return preparedHTTP{}, scriptResult, fmt.Errorf("%s: %w", funcName, authErr)
	}
	fields := auth.Substitute(authFields, vars)

	var reqAuth *RequestAuth
	var authQueryKeys, authWarnings []string
	if isRequesterAuth(ra.Type) {
		reqAuth = &RequestAuth{Type: ra.Type, Fields: fields}
	} else {
		var authApplyErr error
		headers, execURL, authQueryKeys, authWarnings, authApplyErr = u.applyResolvedAuth(ctx, ra, fields, headers, execURL, opt)
		if authApplyErr != nil {
			return preparedHTTP{}, scriptResult, fmt.Errorf("%s: %w", funcName, authApplyErr)
		}
	}

	if ct := autoContentType(req.BodyType); ct != "" {
		if http.Header(headers).Get("Content-Type") == "" {
			headers["Content-Type"] = []string{ct}
		}
	}

	prep := preparedHTTP{
		Method:        req.Method,
		URL:           execURL,
		Headers:       headers,
		BodyType:      req.BodyType,
		RawBody:       body,
		Vars:          vars,
		Auth:          reqAuth,
		AuthQueryKeys: authQueryKeys,
		Warnings:      authWarnings,
	}

	switch req.BodyType {
	case entities.BodyTypeForm:
		fields, parseErr := parseFormFields(body)
		if parseErr != nil {
			return preparedHTTP{}, scriptResult, fmt.Errorf("%s: %w", funcName, parseErr)
		}
		if FormHasFiles(body) {
			ct, reader, encErr := encodeMultipartFormBody(body)
			if encErr != nil {
				return preparedHTTP{}, scriptResult, fmt.Errorf("%s: %w", funcName, encErr)
			}
			prep.BodyReader = reader
			prep.FormFields = fields
			prep.Headers["Content-Type"] = []string{ct}
		} else {
			encoded, encErr := encodeFormBody(body)
			if encErr != nil {
				return preparedHTTP{}, scriptResult, fmt.Errorf("%s: %w", funcName, encErr)
			}
			prep.Body = encoded
			prep.FormFields = fields
		}
	case entities.BodyTypeBinary:
		if body != "" {
			cleanPath, pathErr := validateAbsoluteFilePath(body)
			if pathErr != nil {
				return preparedHTTP{}, scriptResult, fmt.Errorf("%s: %w", funcName, pathErr)
			}
			prep.BinaryPath = cleanPath
		}
	case entities.BodyTypeNone:
	case entities.BodyTypeJSON:
		// Strip JSONC comments before sending; RawBody (history) keeps them.
		prep.Body = stripJSONC(body)
	default:
		prep.Body = body
	}

	return prep, scriptResult, nil
}

// validateAbsoluteFilePath returns the cleaned path, rejecting relative paths and traversal.
func validateAbsoluteFilePath(p string) (string, error) {
	if strings.Contains(p, "..") {
		return "", fmt.Errorf("file path traversal is not allowed: %s", p)
	}
	cleanPath := filepath.Clean(p)
	if !filepath.IsAbs(cleanPath) {
		return "", fmt.Errorf("file path must be absolute: %s", p)
	}
	return cleanPath, nil
}

// isRequesterAuth reports whether the scheme is applied on the wire rather than as a header here.
func isRequesterAuth(t entities.AuthType) bool {
	return t == entities.AuthTypeDigest || t == entities.AuthTypeAWSSigV4
}
