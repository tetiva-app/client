package environment

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/tetiva-app/client/internal/domain"
	"github.com/tetiva-app/client/internal/domain/entities"
)

// Edit holds the data required to edit an environment.
type Edit struct {
	Name string
}

// EditOpt holds contextual options for the Edit operation.
type EditOpt struct {
	EnvironmentID uuid.UUID
	UserID        string
	Version       int
}

// Validate checks that all required fields are present.
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

// Edit updates an existing environment.
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

// EditVariable holds the data required to edit a variable.
type EditVariable struct {
	Key      string
	Value    string
	IsSecret bool
	Enabled  bool
}

// EditVariableOpt holds contextual options for EditVariable.
type EditVariableOpt struct {
	VariableID uuid.UUID
	UserID     string
	Version    int
}

// EditVariable updates an existing variable.
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
