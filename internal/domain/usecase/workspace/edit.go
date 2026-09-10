package workspace

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/tetiva-app/client/internal/domain"
	"github.com/tetiva-app/client/internal/domain/entities"
)

type Edit struct {
	Name string
}

type EditOpt struct {
	WorkspaceID uuid.UUID
	UserID      string
	Version     int
}

func (e *Edit) Validate() error {
	errs := make(map[string]string)
	if e.Name == "" {
		errs["name"] = "required"
	}
	if len(e.Name) > 100 {
		errs["name"] = "must be 100 characters or less"
	}
	if len(errs) > 0 {
		return &domain.ValidationError{Fields: errs}
	}
	return nil
}

func (u *usecase) Edit(ctx context.Context, input Edit, opt EditOpt) (*entities.Workspace, error) {
	const funcName = "workspace.Edit"

	if err := input.Validate(); err != nil {
		return nil, err
	}

	existing, err := u.repo.GetByID(ctx, opt.WorkspaceID)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", funcName, err)
	}
	if existing == nil {
		return nil, &domain.NotFoundError{Entity: "workspace", ID: opt.WorkspaceID.String()}
	}
	if existing.Version != opt.Version {
		return nil, &domain.ConflictError{Entity: "workspace", ID: opt.WorkspaceID.String()}
	}

	existing.Name = input.Name
	existing.Version++
	existing.UpdatedBy = opt.UserID
	existing.UpdatedAt = time.Now()

	if err := u.repo.Update(ctx, existing); err != nil {
		return nil, fmt.Errorf("%s: %w", funcName, err)
	}

	return existing, nil
}
