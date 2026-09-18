package request

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/tetiva-app/client/internal/domain"
	"github.com/tetiva-app/client/internal/domain/entities"
	"github.com/tetiva-app/client/internal/domain/usecase/auth"
	"github.com/tetiva-app/client/internal/domain/usecase/websocket"
)

type Edit struct {
	Name              string
	Description       string
	Protocol          entities.Protocol // set from existing entity before validation
	Method            entities.HTTPMethod
	URL               string
	Headers           []entities.HeaderItem
	Body              string
	BodyType          entities.BodyType
	AuthType          entities.AuthType
	AuthData          string
	PreScript         string
	PostScript        string
	GRPCService       string
	GRPCMethod        string
	GRPCProtoPath     string
	GRPCMetadata      map[string][]string
	GraphQLQuery      string
	GraphQLVariables  string
	GraphQLSchemaPath string
	GraphQLOperation  string
}

type EditOpt struct {
	RequestID uuid.UUID
	UserID    string
	Version   int
}

func (e *Edit) Validate() error {
	errs := make(map[string]string)

	if e.Name == "" {
		errs["name"] = "required"
	}
	if e.Protocol == entities.ProtocolHTTP && !e.Method.IsValid() {
		errs["method"] = "invalid"
	}
	if !e.BodyType.IsValid() {
		errs["bodyType"] = "invalid"
	}
	if !e.AuthType.IsValid() {
		errs["authType"] = "invalid"
	}

	if len(errs) > 0 {
		return &domain.ValidationError{Fields: errs}
	}
	return nil
}

func (u *usecase) Edit(ctx context.Context, input Edit, opt EditOpt) (*entities.Request, error) {
	const funcName = "request.Edit"

	existing, err := u.repo.GetByID(ctx, opt.RequestID)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", funcName, err)
	}
	if existing == nil {
		return nil, &domain.NotFoundError{Entity: "request", ID: opt.RequestID.String()}
	}
	if existing.Version != opt.Version {
		return nil, &domain.ConflictError{Entity: "request", ID: opt.RequestID.String()}
	}

	input.Protocol = existing.Protocol

	if err := input.Validate(); err != nil {
		return nil, err
	}

	// Only growth is rejected: a pre-cap description must not lock the entity.
	if len(input.Description) > domain.MaxDescriptionLen && len(input.Description) > len(existing.Description) {
		return nil, &domain.ValidationError{Fields: map[string]string{
			"description": fmt.Sprintf("must be at most %d bytes", domain.MaxDescriptionLen),
		}}
	}

	if input.Protocol == entities.ProtocolWebSocket {
		if err := websocket.ValidateSettings(input.Body); err != nil {
			return nil, err
		}
		// The body of a WS request is never sent: it carries the settings document.
		input.BodyType = entities.BodyTypeRaw
	}

	headers := input.Headers
	if headers == nil {
		headers = []entities.HeaderItem{}
	}

	prevAuthType, prevAuthData := existing.AuthType, existing.AuthData

	existing.Name = input.Name
	existing.Description = input.Description
	existing.Method = input.Method
	existing.URL = input.URL
	existing.Headers = headers
	existing.Body = input.Body
	existing.BodyType = input.BodyType
	existing.AuthType = input.AuthType
	authData := input.AuthData
	if authData == "" {
		authData = "{}"
	}
	existing.AuthData = authData
	existing.PreScript = input.PreScript
	existing.PostScript = input.PostScript
	existing.GRPCService = input.GRPCService
	existing.GRPCMethod = input.GRPCMethod
	existing.GRPCProtoPath = input.GRPCProtoPath
	if input.GRPCMetadata != nil {
		existing.GRPCMetadata = input.GRPCMetadata
	}
	existing.GraphQLQuery = input.GraphQLQuery
	existing.GraphQLVariables = input.GraphQLVariables
	existing.GraphQLSchemaPath = input.GraphQLSchemaPath
	existing.GraphQLOperation = input.GraphQLOperation
	existing.Version++
	existing.UpdatedBy = opt.UserID
	existing.UpdatedAt = time.Now()

	if err := u.repo.Update(ctx, existing); err != nil {
		return nil, fmt.Errorf("%s: %w", funcName, err)
	}

	if auth.AcquisitionChanged(prevAuthType, existing.AuthType, prevAuthData, existing.AuthData) {
		// The saved configuration can no longer produce the stored token — unless
		// it is the one the token was acquired with, from the same editor buffer.
		keep := auth.AcquisitionHash(existing.AuthType, existing.AuthData)
		if err := u.tokens().ClearOwnersUnlessHash(ctx, entities.AuthOwnerKindRequest, []uuid.UUID{existing.ID}, keep); err != nil {
			return nil, fmt.Errorf("%s: clear tokens: %w", funcName, err)
		}
	}

	return existing, nil
}
