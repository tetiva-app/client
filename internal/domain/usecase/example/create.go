package example

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"

	"github.com/tetiva-app/client/internal/domain"
	"github.com/tetiva-app/client/internal/domain/entities"
	"github.com/tetiva-app/client/internal/domain/secrets"
)

const maxNameLen = 200

type Create struct {
	RequestID   uuid.UUID
	Name        string
	StatusCode  int
	StatusText  string
	Headers     []entities.HeaderItem
	Body        string
	ContentType string
	Protocol    entities.Protocol
}

type CreateOpt struct {
	UserID string
}

func (c *Create) Validate() error {
	errs := validateFields(c.Name, c.StatusCode, c.Body)
	switch c.Protocol {
	case entities.ProtocolHTTP, entities.ProtocolGraphQL, entities.ProtocolGRPC:
	default:
		errs["protocol"] = "must be http, graphql or grpc"
	}
	if len(errs) > 0 {
		return &domain.ValidationError{Fields: errs}
	}
	return nil
}

func (u *usecase) Create(ctx context.Context, in Create, opt CreateOpt) (*entities.ResponseExample, error) {
	const funcName = "example.Create"

	if err := in.Validate(); err != nil {
		return nil, err
	}
	name := strings.TrimSpace(in.Name)
	headers := secrets.RedactHeaders(in.Headers)
	if err := validatePayload(name, in.StatusText, headers, in.Body, in.ContentType); err != nil {
		return nil, err
	}

	req, err := u.requests.GetByID(ctx, in.RequestID)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", funcName, err)
	}
	if req == nil || req.IsDelete {
		return nil, &domain.NotFoundError{Entity: "request", ID: in.RequestID.String()}
	}
	if req.IsDraft {
		return nil, &domain.ValidationError{Fields: map[string]string{"request": "save the request before adding examples"}}
	}
	workspaceID, ok, err := chainWorkspaceFrom(ctx, u.collections, req)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", funcName, err)
	}
	if !ok {
		return nil, &domain.NotFoundError{Entity: "request", ID: in.RequestID.String()}
	}

	sortOrder, err := u.nextSortOrder(ctx, in.RequestID)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", funcName, err)
	}

	now := time.Now()
	e := &entities.ResponseExample{
		ID:          uuid.New(),
		RequestID:   in.RequestID,
		WorkspaceID: workspaceID,
		Name:        name,
		StatusCode:  in.StatusCode,
		StatusText:  in.StatusText,
		Headers:     headers,
		Body:        in.Body,
		ContentType: in.ContentType,
		Protocol:    in.Protocol,
		SortOrder:   sortOrder,
		Version:     1,
		CreatedBy:   opt.UserID,
		CreatedAt:   now,
		UpdatedBy:   opt.UserID,
		UpdatedAt:   now,
	}
	if err := u.repo.Create(ctx, e); err != nil {
		return nil, fmt.Errorf("%s: %w", funcName, err)
	}
	return e, nil
}

func (u *usecase) nextSortOrder(ctx context.Context, requestID uuid.UUID) (int, error) {
	existing, err := u.repo.ListByRequest(ctx, requestID)
	if err != nil {
		return 0, err
	}
	next := 0
	for _, e := range existing {
		if e.SortOrder >= next {
			next = e.SortOrder + 1
		}
	}
	return next, nil
}

func validateFields(name string, statusCode int, body string) map[string]string {
	errs := make(map[string]string)
	switch n := utf8.RuneCountInString(strings.TrimSpace(name)); {
	case n == 0:
		errs["name"] = "required"
	case n > maxNameLen:
		errs["name"] = fmt.Sprintf("must be at most %d characters", maxNameLen)
	}
	if statusCode < 0 || statusCode > 999 {
		errs["statusCode"] = "must be between 0 and 999"
	}
	if len(body) > domain.MaxExampleBodyLen {
		errs["body"] = fmt.Sprintf("body is too large (max %d KB)", domain.MaxExampleBodyLen/1024)
	}
	return errs
}

// validatePayload runs on masked headers: the server's 512 KiB cap applies to what is synced.
func validatePayload(name, statusText string, headers []entities.HeaderItem, body, contentType string) error {
	headersJSON, err := json.Marshal(headers)
	if err != nil {
		return fmt.Errorf("example.validatePayload: %w", err)
	}
	if len(body)+len(headersJSON)+len(name)+len(statusText)+len(contentType) > domain.MaxExamplePayloadLen {
		return &domain.ValidationError{Fields: map[string]string{
			"example": fmt.Sprintf("example is too large (max %d KB)", domain.MaxExamplePayloadLen/1024),
		}}
	}
	return nil
}
