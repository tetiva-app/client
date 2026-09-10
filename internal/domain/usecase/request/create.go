package request

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/tetiva-app/client/internal/domain"
	"github.com/tetiva-app/client/internal/domain/entities"
	"github.com/tetiva-app/client/internal/domain/usecase/websocket"
)

type Create struct {
	CollectionID      uuid.UUID
	Name              string
	Description       string
	Protocol          entities.Protocol
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

type CreateOpt struct {
	UserID string
}

func (c *Create) Validate() error {
	errs := make(map[string]string)

	if c.Name == "" {
		errs["name"] = "required"
	}
	if c.CollectionID == uuid.Nil {
		errs["collectionId"] = "required"
	}
	if !c.Protocol.IsValid() {
		errs["protocol"] = "invalid"
	}
	if c.Protocol == entities.ProtocolHTTP && !c.Method.IsValid() {
		errs["method"] = "invalid"
	}
	if !c.BodyType.IsValid() {
		errs["bodyType"] = "invalid"
	}
	if !c.AuthType.IsValid() {
		errs["authType"] = "invalid"
	}

	if len(errs) > 0 {
		return &domain.ValidationError{Fields: errs}
	}
	return nil
}

func (u *usecase) Create(ctx context.Context, input Create, opt CreateOpt) (*entities.Request, error) {
	const funcName = "request.Create"

	if err := input.Validate(); err != nil {
		return nil, err
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

	authData := input.AuthData
	if authData == "" {
		authData = "{}"
	}

	grpcMetadata := input.GRPCMetadata
	if grpcMetadata == nil {
		grpcMetadata = make(map[string][]string)
	}

	now := time.Now()
	r := &entities.Request{
		ID:                uuid.New(),
		CollectionID:      input.CollectionID,
		Name:              input.Name,
		Description:       input.Description,
		Protocol:          input.Protocol,
		Method:            input.Method,
		URL:               input.URL,
		Headers:           headers,
		Body:              input.Body,
		BodyType:          input.BodyType,
		AuthType:          input.AuthType,
		AuthData:          authData,
		PreScript:         input.PreScript,
		PostScript:        input.PostScript,
		GRPCService:       input.GRPCService,
		GRPCMethod:        input.GRPCMethod,
		GRPCProtoPath:     input.GRPCProtoPath,
		GRPCMetadata:      grpcMetadata,
		GraphQLQuery:      input.GraphQLQuery,
		GraphQLVariables:  input.GraphQLVariables,
		GraphQLSchemaPath: input.GraphQLSchemaPath,
		GraphQLOperation:  input.GraphQLOperation,
		SortOrder:         0,
		Version:           1,
		IsDelete:          false,
		CreatedBy:         opt.UserID,
		CreatedAt:         now,
		UpdatedBy:         opt.UserID,
		UpdatedAt:         now,
	}

	if err := u.repo.Create(ctx, r); err != nil {
		return nil, fmt.Errorf("%s: %w", funcName, err)
	}

	return r, nil
}
