package example

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/tetiva-app/client/internal/domain"
	"github.com/tetiva-app/client/internal/domain/entities"
	"github.com/tetiva-app/client/internal/domain/secrets"
)

type Edit struct {
	Name        string
	StatusCode  int
	StatusText  string
	Headers     []entities.HeaderItem
	Body        string
	ContentType string
}

type EditOpt struct {
	ExampleID uuid.UUID
	UserID    string
	Version   int
}

func (e *Edit) Validate() error {
	if errs := validateFields(e.Name, e.StatusCode, e.Body); len(errs) > 0 {
		return &domain.ValidationError{Fields: errs}
	}
	return nil
}

func (u *usecase) Edit(ctx context.Context, in Edit, opt EditOpt) (*entities.ResponseExample, error) {
	const funcName = "example.Edit"

	if err := in.Validate(); err != nil {
		return nil, err
	}
	name := strings.TrimSpace(in.Name)
	headers := secrets.RedactHeaders(in.Headers)
	if err := validatePayload(name, in.StatusText, headers, in.Body, in.ContentType); err != nil {
		return nil, err
	}

	e, err := u.getLive(ctx, opt.ExampleID)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", funcName, err)
	}
	if e.Version != opt.Version {
		return nil, &domain.ConflictError{Entity: "example", ID: opt.ExampleID.String()}
	}

	e.Name = name
	e.StatusCode = in.StatusCode
	e.StatusText = in.StatusText
	e.Headers = headers
	e.Body = in.Body
	e.ContentType = in.ContentType
	e.Version++
	e.UpdatedBy = opt.UserID
	e.UpdatedAt = time.Now()
	if err := u.repo.UpdateAtVersion(ctx, e, opt.Version); err != nil {
		return nil, fmt.Errorf("%s: %w", funcName, err)
	}
	return e, nil
}
