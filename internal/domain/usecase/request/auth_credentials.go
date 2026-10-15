package request

import (
	"context"
	"encoding/base64"
	"fmt"

	"github.com/google/uuid"

	"github.com/tetiva-app/client/internal/domain"
	"github.com/tetiva-app/client/internal/domain/entities"
	"github.com/tetiva-app/client/internal/domain/usecase/auth"
)

var authCredentialFields = map[entities.AuthType][]string{
	entities.AuthTypeBasic:    {"password"},
	entities.AuthTypeBearer:   {"token"},
	entities.AuthTypeAPIKey:   {"value"},
	entities.AuthTypeOAuth2:   {"clientSecret", "password"},
	entities.AuthTypeJWT:      {"secret", "privateKey"},
	entities.AuthTypeDigest:   {"password"},
	entities.AuthTypeAWSSigV4: {"secretAccessKey", "sessionToken"},
}

func (u *usecase) AuthCredentials(ctx context.Context, requestID, workspaceID uuid.UUID) ([]string, error) {
	const funcName = "request.AuthCredentials"

	req, err := u.repo.GetByID(ctx, requestID)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", funcName, err)
	}
	if req == nil {
		return nil, &domain.NotFoundError{Entity: "request", ID: requestID.String()}
	}
	ra, err := u.resolveAuthFor(ctx, req, workspaceID)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", funcName, err)
	}
	fields, err := auth.ParseFields(ra.Data)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", funcName, err)
	}
	vars, err := u.envResolver.ResolveVariables(ctx, workspaceID)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", funcName, err)
	}
	fields = auth.Substitute(fields, vars)

	var out []string
	for _, key := range authCredentialFields[ra.Type] {
		if v := fields.Str(key); v != "" {
			out = append(out, v)
		}
	}
	if ra.Type == entities.AuthTypeBasic && fields.Str("password") != "" {
		out = append(out, base64.StdEncoding.EncodeToString([]byte(fields.Str("username")+":"+fields.Str("password"))))
	}
	if ra.Type == entities.AuthTypeOAuth2 && u.authProvider != nil {
		token, ok, err := u.authProvider.Peek(ctx, ra.Owner, auth.OAuth2ConfigFromFields(fields))
		if err != nil {
			return nil, fmt.Errorf("%s: %w", funcName, err)
		}
		if ok {
			out = append(out, token)
		}
	}
	return out, nil
}
