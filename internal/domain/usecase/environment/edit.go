package environment

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
	EnvironmentID uuid.UUID
	UserID        string
	Version       int
}

func (e *Edit) Validate() error {
	errs := make(map[string]string)
	if e.Name == "" {
		errs["name"] = "required"
	}
	if len(errs) > 0 {
		return &domain.ValidationError{Fields: errs}
	}
	return nil
}

func (u *usecase) Edit(ctx context.Context, input Edit, opt EditOpt) (*entities.Environment, error) {
	const funcName = "environment.Edit"

	if err := input.Validate(); err != nil {
		return nil, err
	}

	existing, err := u.repo.GetByID(ctx, opt.EnvironmentID)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", funcName, err)
	}
	if existing == nil {
		return nil, &domain.NotFoundError{Entity: "environment", ID: opt.EnvironmentID.String()}
	}
	if existing.Version != opt.Version {
		return nil, &domain.ConflictError{Entity: "environment", ID: opt.EnvironmentID.String()}
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

type EditVariable struct {
	Key      string
	Value    string
	IsSecret bool
	Enabled  bool
}

type EditVariableOpt struct {
	VariableID uuid.UUID
	UserID     string
	Version    int
}

func (u *usecase) EditVariable(ctx context.Context, input EditVariable, opt EditVariableOpt) (*entities.Variable, error) {
	const funcName = "environment.EditVariable"

	existing, err := u.varRepo.GetByID(ctx, opt.VariableID)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", funcName, err)
	}
	if existing == nil {
		return nil, &domain.NotFoundError{Entity: "variable", ID: opt.VariableID.String()}
	}
	if existing.Version != opt.Version {
		return nil, &domain.ConflictError{Entity: "variable", ID: opt.VariableID.String()}
	}

	existing.Key = input.Key
	existing.Value = input.Value
	existing.IsSecret = input.IsSecret
	existing.Enabled = input.Enabled
	existing.Version++
	existing.UpdatedBy = opt.UserID
	existing.UpdatedAt = time.Now()

	if err := u.varRepo.Update(ctx, existing); err != nil {
		return nil, fmt.Errorf("%s: %w", funcName, err)
	}

	return existing, nil
}
