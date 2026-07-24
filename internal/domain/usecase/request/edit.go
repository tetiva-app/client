package request

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/tetiva-app/client/internal/domain"
	"github.com/tetiva-app/client/internal/domain/entities"
)

// Edit holds the data required to edit an existing request.
type Edit struct {
	Name              string
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

// EditOpt holds contextual options for the Edit operation.
type EditOpt struct {
	RequestID uuid.UUID
	UserID    string
	Version   int
}

// Validate checks that all required fields are present and valid.
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

// Edit validates input, applies optimistic locking, updates and persists the request.
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

	headers := input.Headers
	if headers == nil {
		headers = []entities.HeaderItem{}
	}

	existing.Name = input.Name
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

	return existing, nil
}
