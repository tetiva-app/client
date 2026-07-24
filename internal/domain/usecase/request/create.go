package request

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/tetiva-app/client/internal/domain"
	"github.com/tetiva-app/client/internal/domain/entities"
)

// Create holds the data required to create a new request.
type Create struct {
	CollectionID      uuid.UUID
	Name              string
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

// CreateOpt holds contextual options for the Create operation.
type CreateOpt struct {
	UserID string
}

// Validate checks that all required fields are present and valid.
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

// Create validates input, builds a Request entity and persists it.
func (u *usecase) Create(ctx context.Context, input Create, opt CreateOpt) (*entities.Request, error) {
	const funcName = "request.Create"

	if err := input.Validate(); err != nil {
		return nil, err
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
