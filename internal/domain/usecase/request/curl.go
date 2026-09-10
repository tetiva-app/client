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
	"github.com/tetiva-app/client/internal/domain/usecase/auth"
)

// BuildCurl reuses the Execute prep pipeline but sends nothing and never acquires a token: an oauth2
// request renders from the cached one or without credentials. HTTP only; a pre-script error does not abort.
func (u *usecase) BuildCurl(ctx context.Context, id uuid.UUID, opt BuildCurlOpt) (CurlResult, error) {
	const funcName = "request.BuildCurl"

	req, err := u.repo.GetByID(ctx, id)
	if err != nil {
		return CurlResult{}, fmt.Errorf("%s: %w", funcName, err)
	}
	if req == nil {
		return CurlResult{}, &domain.NotFoundError{Entity: "request", ID: id.String()}
	}
	if req.Protocol != entities.ProtocolHTTP {
		return CurlResult{}, &domain.ValidationError{Fields: map[string]string{
			"protocol": "Copy as cURL is only supported for HTTP requests",
		}}
	}

	resolvedAuth, err := u.resolveAuthFor(ctx, req, opt.WorkspaceID)
	if err != nil {
		return CurlResult{}, fmt.Errorf("%s: %w", funcName, err)
	}

	vars, err := u.envResolver.ResolveVariables(ctx, opt.WorkspaceID)
	if err != nil {
		return CurlResult{}, fmt.Errorf("%s: %w", funcName, err)
	}

	prep, scriptResult, err := u.prepareHTTP(ctx, req, vars, resolvedAuth, prepareOpt{TokenFromCacheOnly: true})
	if err != nil {
		return CurlResult{ScriptResult: scriptResult}, fmt.Errorf("%s: %w", funcName, err)
	}

	var cookies []*http.Cookie
	if u.cookieReader != nil {
		cookies = u.cookieReader.CookiesFor(ctx, opt.WorkspaceID, prep.URL)
	}

	return CurlResult{
		Command:      buildCurlText(prep, cookies),
		Warnings:     prep.Warnings,
		ScriptResult: scriptResult,
	}, nil
}

// buildCurlText is pure: no IO, and every argument goes through shellQuote.
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

	parts = append(parts, curlAuthFlags(prep.Auth)...)

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

// curlAuthFlags renders the schemes the requester applies on the wire; header ones are in prep.Headers.
func curlAuthFlags(ra *RequestAuth) []string {
	if ra == nil {
		return nil
	}
	f := auth.Fields(ra.Fields)

	switch ra.Type {
	case entities.AuthTypeDigest:
		return []string{"--digest", "-u " + shellQuote(f.Str("username")+":"+f.Str("password"))}
	case entities.AuthTypeAWSSigV4:
		flags := []string{
			"--aws-sigv4 " + shellQuote("aws:amz:"+f.Str("region")+":"+f.Str("service")),
			"-u " + shellQuote(f.Str("accessKeyId")+":"+f.Str("secretAccessKey")),
		}
		if token := f.Str("sessionToken"); token != "" {
			flags = append(flags, "-H "+shellQuote("x-amz-security-token: "+token))
		}
		return flags
	default:
		return nil
	}
}

// shellQuote wraps s in single quotes, escaping embedded ones with the quote/backslash/quote pattern.
func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}
