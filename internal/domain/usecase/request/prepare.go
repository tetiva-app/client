package request

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/tetiva-app/client/internal/domain/entities"
)

// preparedHTTP is the fully-resolved HTTP request after env substitution,
// pre-script, auth, and body encoding. Consumed by both Execute and BuildCurl.
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
}

// prepareOpt fields carry no behavior yet; they keep the call signature stable.
type prepareOpt struct {
	PersistVars bool
	UserID      string
}

// prepareHTTP runs the preparation pipeline: env vars → pre-script → auth →
// auto Content-Type → body encoding. scriptResult is non-nil if a pre-script ran
// (script errors are appended, not fatal). The caller decides whether to persist vars.
func (u *usecase) prepareHTTP(
	ctx context.Context,
	req *entities.Request,
	vars map[string]string,
	opt prepareOpt,
) (preparedHTTP, *entities.ScriptResult, error) {
	const funcName = "request.prepareHTTP"
	_ = opt

	headers := entities.EnabledHeadersToMap(req.Headers)
	execURL := substituteVariables(req.URL, vars)
	body := substituteVariables(req.Body, vars)
	headers = substituteHeaders(headers, vars)

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
			RequestHeaders: headers,
			Protocol:       string(req.Protocol),
		}
		preResult, preErr := u.scriptEngine.RunPreScript(ctx, preScript, preCtx)
		if preErr != nil {
			scriptResult.Errors = append(scriptResult.Errors, entities.ScriptError{
				Phase: "pre-script", Message: preErr.Error(),
			})
		} else {
			scriptResult.PreConsole = preResult.ConsoleOutput
			headers = preResult.Headers
			if !mapsEqual(vars, preResult.Variables) {
				vars = preResult.Variables
				execURL = substituteVariables(req.URL, vars)
				body = substituteVariables(req.Body, vars)
				headers = substituteHeaders(headers, vars)
			}
		}
	}

	resolvedAuthType, resolvedAuthData, authErr := u.authResolver.ResolveAuth(ctx, req)
	if authErr != nil {
		return preparedHTTP{}, scriptResult, fmt.Errorf("%s: %w", funcName, authErr)
	}
	if resolvedAuthData != "" && resolvedAuthData != "{}" {
		resolvedAuthData = substituteVariables(resolvedAuthData, vars)
	}
	headers, execURL, err := applyAuth(resolvedAuthType, resolvedAuthData, headers, execURL)
	if err != nil {
		return preparedHTTP{}, scriptResult, fmt.Errorf("%s: %w", funcName, err)
	}

	if ct := autoContentType(req.BodyType); ct != "" {
		if http.Header(headers).Get("Content-Type") == "" {
			headers["Content-Type"] = []string{ct}
		}
	}

	prep := preparedHTTP{
		Method:   req.Method,
		URL:      execURL,
		Headers:  headers,
		BodyType: req.BodyType,
		RawBody:  body,
		Vars:     vars,
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

// validateAbsoluteFilePath ensures path is absolute and free of traversal.
// Returns the cleaned absolute path.
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
