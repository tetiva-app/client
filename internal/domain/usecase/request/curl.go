package request

import (
	"context"
	"fmt"
	"net/http"
	"sort"
	"strings"

	"github.com/google/uuid"

	"github.com/tetiva-app/client/internal/domain"
	"github.com/tetiva-app/client/internal/domain/entities"
)

// BuildCurl renders a request as a shell-ready curl command using the same prep
// pipeline as Execute, but without sending, persisting variables, or writing history.
// HTTP only — gRPC/GraphQL return ValidationError; pre-script errors do not abort.
func (u *usecase) BuildCurl(ctx context.Context, id uuid.UUID, opt BuildCurlOpt) (string, *entities.ScriptResult, error) {
	const funcName = "request.BuildCurl"

	req, err := u.repo.GetByID(ctx, id)
	if err != nil {
		return "", nil, fmt.Errorf("%s: %w", funcName, err)
	}
	if req == nil {
		return "", nil, &domain.NotFoundError{Entity: "request", ID: id.String()}
	}
	if req.Protocol != entities.ProtocolHTTP {
		return "", nil, &domain.ValidationError{Fields: map[string]string{
			"protocol": "Copy as cURL is only supported for HTTP requests",
		}}
	}

	vars, err := u.envResolver.ResolveVariables(ctx, opt.WorkspaceID)
	if err != nil {
		return "", nil, fmt.Errorf("%s: %w", funcName, err)
	}

	prep, scriptResult, err := u.prepareHTTP(ctx, req, vars, prepareOpt{})
	if err != nil {
		return "", scriptResult, fmt.Errorf("%s: %w", funcName, err)
	}

	var cookies []*http.Cookie
	if u.cookieReader != nil {
		cookies = u.cookieReader.CookiesFor(ctx, opt.WorkspaceID, prep.URL)
	}
	return buildCurlText(prep, cookies), scriptResult, nil
}

// buildCurlText formats a prepared HTTP request as a multi-line curl command.
// Pure function; no IO. Arguments are single-quoted via shellQuote.
func buildCurlText(prep preparedHTTP, cookies []*http.Cookie) string {
	parts := []string{"curl"}

	if prep.Method != entities.MethodGET && prep.Method != "" {
		parts = append(parts, "-X "+string(prep.Method))
	}

	parts = append(parts, shellQuote(prep.URL))

	// Stable header order; multi-value headers emit one -H per value.
	keys := make([]string, 0, len(prep.Headers))
	for k := range prep.Headers {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		for _, v := range prep.Headers[k] {
			parts = append(parts, "-H "+shellQuote(k+": "+v))
		}
	}

	if len(cookies) > 0 {
		pairs := make([]string, 0, len(cookies))
		for _, c := range cookies {
			pairs = append(pairs, c.Name+"="+c.Value)
		}
		parts = append(parts, "-b "+shellQuote(strings.Join(pairs, "; ")))
	}

	switch prep.BodyType {
	case entities.BodyTypeForm:
		// -F only when prepareHTTP produced a multipart body (files involved);
		// urlencoded forms use -d so the explicit Content-Type header is honored.
		if prep.BodyReader != nil {
			for _, f := range prep.FormFields {
				if !f.Enabled || f.Key == "" {
					continue
				}
				if f.Type == "file" && f.Value != "" {
					parts = append(parts, "-F "+shellQuote(f.Key+"=@"+f.Value))
				} else {
					parts = append(parts, "-F "+shellQuote(f.Key+"="+f.Value))
				}
			}
		} else if prep.Body != "" {
			parts = append(parts, "-d "+shellQuote(prep.Body))
		}
	case entities.BodyTypeBinary:
		if prep.BinaryPath != "" {
			parts = append(parts, "--data-binary "+shellQuote("@"+prep.BinaryPath))
		}
	case entities.BodyTypeNone:
	default:
		if prep.Body != "" {
			parts = append(parts, "-d "+shellQuote(prep.Body))
		}
	}

	return strings.Join(parts, " \\\n  ")
}

// shellQuote wraps s in single quotes, escaping embedded single quotes
// with the standard quote/backslash-quote/quote shell pattern.
func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}
