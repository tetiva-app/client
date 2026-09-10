package request

import (
	"context"
	"errors"
	"fmt"
	"net/url"

	"github.com/tetiva-app/client/internal/domain"
	"github.com/tetiva-app/client/internal/domain/entities"
	"github.com/tetiva-app/client/internal/domain/usecase/auth"
)

const (
	oauth2DefaultPrefix     = "Bearer"
	oauth2DefaultQueryParam = "access_token"
)

// oauth2NoCachedTokenWarning is shown where a token may not be acquired on the
// spot (Copy as cURL), so the command is rendered without credentials.
const oauth2NoCachedTokenWarning = "no cached OAuth 2.0 token: the command carries no Authorization; get a token in the Auth tab first"

// applyResolvedAuth puts the effective auth on the prepared headers and URL: oauth2 goes through the
// token provider; digest and aws_sigv4 never reach here, the caller puts them on preparedHTTP.Auth.
func (u *usecase) applyResolvedAuth(
	ctx context.Context,
	ra ResolvedAuth,
	f auth.Fields,
	headers map[string][]string,
	rawURL string,
	opt prepareOpt,
) (map[string][]string, string, []string, []string, error) {
	if ra.Type != entities.AuthTypeOAuth2 {
		outHeaders, outURL, keys, err := applyHeaderAuth(ra.Type, f, headers, rawURL)
		return outHeaders, outURL, keys, nil, err
	}

	token, warnings, err := u.oauth2Token(ctx, ra.Owner, f, opt)
	if err != nil {
		return headers, rawURL, nil, nil, err
	}
	if token == "" {
		return headers, rawURL, nil, warnings, nil
	}

	if f.Str("addTo") == "query" {
		param := f.Str("queryParam")
		if param == "" {
			param = oauth2DefaultQueryParam
		}
		parsed, parseErr := url.Parse(rawURL)
		if parseErr != nil {
			return headers, rawURL, nil, warnings, fmt.Errorf("invalid URL for OAuth 2.0 token: %w", parseErr)
		}
		q := parsed.Query()
		q.Set(param, token)
		parsed.RawQuery = q.Encode()
		return headers, parsed.String(), []string{param}, warnings, nil
	}

	prefix := oauth2DefaultPrefix
	if _, ok := f["headerPrefix"]; ok {
		prefix = f.Str("headerPrefix")
	}
	if prefix == "" {
		headers["Authorization"] = []string{token}
	} else {
		headers["Authorization"] = []string{prefix + " " + token}
	}

	return headers, rawURL, nil, warnings, nil
}

// oauth2Token returns "" with a warning when the caller accepts only a cached token and there is none.
func (u *usecase) oauth2Token(ctx context.Context, owner entities.AuthOwner, f auth.Fields, opt prepareOpt) (string, []string, error) {
	if u.authProvider == nil {
		return "", nil, &domain.ValidationError{Fields: map[string]string{
			"auth": "OAuth 2.0 is not available in this build",
		}}
	}

	cfg := auth.OAuth2ConfigFromFields(f)

	if opt.TokenFromCacheOnly {
		token, ok, err := u.authProvider.Peek(ctx, owner, cfg)
		if err != nil {
			return "", nil, err
		}
		if !ok {
			return "", []string{oauth2NoCachedTokenWarning}, nil
		}
		return token, nil, nil
	}

	token, err := u.authProvider.AccessToken(ctx, owner, cfg)
	if err != nil {
		if errors.Is(err, auth.ErrTokenRequired) {
			return "", nil, &domain.ValidationError{Fields: map[string]string{
				"auth": "get a token in the Auth tab first",
			}}
		}
		return "", nil, err
	}

	return token, nil, nil
}
