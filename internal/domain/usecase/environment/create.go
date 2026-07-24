package environment

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/tetiva-app/client/internal/domain"
	"github.com/tetiva-app/client/internal/domain/entities"
)

// Create holds the data required to create a new environment.
type Create struct {
	Name string
}

// CreateOpt holds contextual options for the Create operation.
type CreateOpt struct {
	UserID      string
	WorkspaceID uuid.UUID
}

// Validate checks that all required fields are present.
func (c *Create) Validate() error {
	errs := make(map[string]string)
	if c.Name == "" {
		errs["name"] = "required"
	}
	if len(errs) > 0 {
		return &domain.ValidationError{Fields: errs}
	}
	return nil
}

// Create validates input, builds an Environment entity and persists it.
func (u *usecase) Create(ctx context.Context, input Create, opt CreateOpt) (*entities.Environment, error) {
	const funcName = "environment.Create"

	if err := input.Validate(); err != nil {
		return nil, err
	}

	now := time.Now()
	e := &entities.Environment{
		ID:          uuid.New(),
		WorkspaceID: opt.WorkspaceID,
		Name:        input.Name,
		IsActive:    false,
		Version:     1,
		IsDelete:    false,
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

// DuplicateOpt holds contextual options for the Duplicate operation.
type DuplicateOpt struct {
	SourceID    uuid.UUID
	NewName     string
	UserID      string
	WorkspaceID uuid.UUID
}

// Duplicate creates a copy of an existing environment with all its variables.
func (u *usecase) Duplicate(ctx context.Context, opt DuplicateOpt) (*entities.Environment, error) {
	const funcName = "environment.Duplicate"

	if opt.NewName == "" {
		return nil, &domain.ValidationError{Fields: map[string]string{"name": "required"}}
	}

	source, err := u.repo.GetByID(ctx, opt.SourceID)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", funcName, err)
	}
	if source == nil {
		return nil, &domain.NotFoundError{Entity: "environment", ID: opt.SourceID.String()}
	}

	now := time.Now()
	newEnv := &entities.Environment{
		ID:          uuid.New(),
		WorkspaceID: opt.WorkspaceID,
		Name:        opt.NewName,
		IsActive:    false,
		Version:     1,
		IsDelete:    false,
		CreatedBy:   opt.UserID,
		CreatedAt:   now,
		UpdatedBy:   opt.UserID,
		UpdatedAt:   now,
	}

	if err := u.repo.Create(ctx, newEnv); err != nil {
		return nil, fmt.Errorf("%s: %w", funcName, err)
	}

	vars, err := u.varRepo.List(ctx, opt.SourceID)
	if err != nil {
		return nil, fmt.Errorf("%s: failed to list source variables: %w", funcName, err)
	}

	for _, v := range vars {
		newVar := &entities.Variable{
			ID:            uuid.New(),
			EnvironmentID: newEnv.ID,
			Key:           v.Key,
			Value:         v.Value,
			IsSecret:      v.IsSecret,
			Enabled:       v.Enabled,
			SortOrder:     v.SortOrder,
			Version:       1,
			IsDelete:      false,
			CreatedBy:     opt.UserID,
			CreatedAt:     now,
			UpdatedBy:     opt.UserID,
			UpdatedAt:     now,
		}
		if err := u.varRepo.Create(ctx, newVar); err != nil {
			return nil, fmt.Errorf("%s: failed to copy variable %s: %w", funcName, v.Key, err)
		}
	}

	return newEnv, nil
}

// AddVariable holds the data required to add a variable to an environment.
type AddVariable struct {
	EnvironmentID uuid.UUID
	Key           string
	Value         string
	IsSecret      bool
}

// AddVariableOpt holds contextual options for AddVariable.
type AddVariableOpt struct {
	UserID string
}

// Validate checks that all required fields are present.
func (a *AddVariable) Validate() error {
	errs := make(map[string]string)
	if a.Key == "" {
		errs["key"] = "required"
	}
	if len(errs) > 0 {
		return &domain.ValidationError{Fields: errs}
	}
	return nil
}

// AddVariable creates a new variable in the given environment.
func (u *usecase) AddVariable(ctx context.Context, input AddVariable, opt AddVariableOpt) (*entities.Variable, error) {
	const funcName = "environment.AddVariable"

	if err := input.Validate(); err != nil {
		return nil, err
	}

	now := time.Now()
	v := &entities.Variable{
		ID:            uuid.New(),
		EnvironmentID: input.EnvironmentID,
		Key:           input.Key,
		Value:         input.Value,
		IsSecret:      input.IsSecret,
		Enabled:       true,
		SortOrder:     0,
		Version:       1,
		IsDelete:      false,
		CreatedBy:     opt.UserID,
		CreatedAt:     now,
		UpdatedBy:     opt.UserID,
		UpdatedAt:     now,
	}

	if err := u.varRepo.Create(ctx, v); err != nil {
		return nil, fmt.Errorf("%s: %w", funcName, err)
	}

	return v, nil
}
