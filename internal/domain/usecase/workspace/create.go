package workspace

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/tetiva-app/client/internal/domain"
	"github.com/tetiva-app/client/internal/domain/entities"
)

// Create holds data for creating a new workspace.
type Create struct {
	Name              string
	RemoteWorkspaceID *string
}

// CreateOpt holds contextual options for workspace creation.
type CreateOpt struct {
	UserID string
}

// Validate checks the Create input.
func (c *Create) Validate() error {
	errs := make(map[string]string)
	if c.Name == "" {
		errs["name"] = "required"
	}
	if len(c.Name) > 100 {
		errs["name"] = "must be 100 characters or less"
	}
	if len(errs) > 0 {
		return &domain.ValidationError{Fields: errs}
	}
	return nil
}

func (u *usecase) Create(ctx context.Context, input Create, opt CreateOpt) (*entities.Workspace, error) {
	const funcName = "workspace.Create"

	if err := input.Validate(); err != nil {
		return nil, err
	}

	now := time.Now()
	w := &entities.Workspace{
		ID:                uuid.New(),
		Name:              input.Name,
		IsActive:          false,
		Version:           1,
		IsDelete:          false,
		RemoteWorkspaceID: input.RemoteWorkspaceID,
		CreatedBy:         opt.UserID,
		CreatedAt:         now,
		UpdatedBy:         opt.UserID,
		UpdatedAt:         now,
	}

	if err := u.repo.Create(ctx, w); err != nil {
		return nil, fmt.Errorf("%s: %w", funcName, err)
	}

	return w, nil
}
