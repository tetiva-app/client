package environment

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/tetiva-app/client/internal/domain"
	"github.com/tetiva-app/client/internal/domain/entities"
)

// Filter defines criteria for listing environments.
type Filter struct {
	WorkspaceID uuid.UUID
}

// ListOpt holds contextual options for the List operation.
type ListOpt struct {
	WorkspaceID uuid.UUID
}

// DeleteOpt holds contextual options for the Delete operation.
type DeleteOpt struct {
	EnvironmentID uuid.UUID
	UserID        string
	Version       int
}

// SetActiveOpt holds options for setting the active environment.
type SetActiveOpt struct {
	WorkspaceID   uuid.UUID
	EnvironmentID uuid.UUID
}

// DeleteVariableOpt holds options for deleting a variable.
type DeleteVariableOpt struct {
	VariableID uuid.UUID
}

// GetByID retrieves a single environment by its ID.
func (u *usecase) GetByID(ctx context.Context, id uuid.UUID) (*entities.Environment, error) {
	const funcName = "environment.GetByID"

	e, err := u.repo.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", funcName, err)
	}
	if e == nil {
		return nil, &domain.NotFoundError{Entity: "environment", ID: id.String()}
	}

	return e, nil
}

// List returns environments matching the given options.
func (u *usecase) List(ctx context.Context, opt ListOpt) ([]*entities.Environment, error) {
	const funcName = "environment.List"

	envs, err := u.repo.List(ctx, Filter(opt))
	if err != nil {
		return nil, fmt.Errorf("%s: %w", funcName, err)
	}

	return envs, nil
}

// Delete performs a soft delete on the environment.
func (u *usecase) Delete(ctx context.Context, opt DeleteOpt) error {
	const funcName = "environment.Delete"

	existing, err := u.repo.GetByID(ctx, opt.EnvironmentID)
	if err != nil {
		return fmt.Errorf("%s: %w", funcName, err)
	}
	if existing == nil {
		return &domain.NotFoundError{Entity: "environment", ID: opt.EnvironmentID.String()}
	}
	if existing.Version != opt.Version {
		return &domain.ConflictError{Entity: "environment", ID: opt.EnvironmentID.String()}
	}

	existing.IsDelete = true
	existing.Version++
	existing.UpdatedBy = opt.UserID
	existing.UpdatedAt = time.Now()

	if err := u.repo.Update(ctx, existing); err != nil {
		return fmt.Errorf("%s: %w", funcName, err)
	}

	return nil
}

// SetActive sets the active environment for a workspace.
func (u *usecase) SetActive(ctx context.Context, opt SetActiveOpt) error {
	const funcName = "environment.SetActive"

	if err := u.repo.SetActive(ctx, opt.WorkspaceID, opt.EnvironmentID); err != nil {
		return fmt.Errorf("%s: %w", funcName, err)
	}

	return nil
}

// ListVariables returns all non-deleted variables for an environment.
func (u *usecase) ListVariables(ctx context.Context, environmentID uuid.UUID) ([]*entities.Variable, error) {
	const funcName = "environment.ListVariables"

	vars, err := u.varRepo.List(ctx, environmentID)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", funcName, err)
	}

	return vars, nil
}

// DeleteVariable removes a variable (hard delete — variables cascade with environment).
func (u *usecase) DeleteVariable(ctx context.Context, opt DeleteVariableOpt) error {
	const funcName = "environment.DeleteVariable"

	if err := u.varRepo.Delete(ctx, opt.VariableID); err != nil {
		return fmt.Errorf("%s: %w", funcName, err)
	}

	return nil
}

// ResolveVariables returns a map of key→value for the active environment's enabled variables.
func (u *usecase) ResolveVariables(ctx context.Context, workspaceID uuid.UUID) (map[string]string, error) {
	const funcName = "environment.ResolveVariables"

	active, err := u.repo.GetActive(ctx, workspaceID)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", funcName, err)
	}
	if active == nil {
		return map[string]string{}, nil
	}

	vars, err := u.varRepo.List(ctx, active.ID)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", funcName, err)
	}

	result := make(map[string]string, len(vars))
	for _, v := range vars {
		if v.Enabled {
			result[v.Key] = v.Value
		}
	}

	return result, nil
}
