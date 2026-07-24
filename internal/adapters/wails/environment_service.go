package wails

import (
	"context"

	"github.com/google/uuid"

	"github.com/tetiva-app/client/internal/adapters/wails/dto"
	"github.com/tetiva-app/client/internal/domain"
	"github.com/tetiva-app/client/internal/domain/usecase/environment"
)

// EnvironmentService exposes Environment operations to the Wails frontend.
type EnvironmentService struct {
	uc environment.Usecase
}

// NewEnvironmentService creates a new EnvironmentService instance.
func NewEnvironmentService(uc environment.Usecase) *EnvironmentService {
	return &EnvironmentService{uc: uc}
}

// List returns all environments for the given workspace.
func (s *EnvironmentService) List(workspaceIDStr string) Result[[]dto.EnvironmentResponse] {
	ctx := context.Background()

	workspaceID, err := uuid.Parse(workspaceIDStr)
	if err != nil {
		return Err[[]dto.EnvironmentResponse](&domain.ValidationError{
			Fields: map[string]string{"workspaceId": "invalid UUID"},
		})
	}
	opt := environment.ListOpt{WorkspaceID: workspaceID}

	envs, err := s.uc.List(ctx, opt)
	if err != nil {
		return Err[[]dto.EnvironmentResponse](err)
	}

	return OK(dto.EnvironmentsToResponse(envs))
}

// Create creates a new environment.
func (s *EnvironmentService) Create(req dto.CreateEnvironmentRequest) Result[dto.EnvironmentResponse] {
	ctx := context.Background()

	workspaceID, err := uuid.Parse(req.WorkspaceID)
	if err != nil {
		return Err[dto.EnvironmentResponse](&domain.ValidationError{
			Fields: map[string]string{"workspaceId": "invalid UUID"},
		})
	}

	input := environment.Create{Name: req.Name}
	opt := environment.CreateOpt{
		UserID:      defaultUserID,
		WorkspaceID: workspaceID,
	}

	env, err := s.uc.Create(ctx, input, opt)
	if err != nil {
		return Err[dto.EnvironmentResponse](err)
	}

	return OK(dto.EnvironmentToResponse(env))
}

// Duplicate creates a copy of an environment with all its variables.
func (s *EnvironmentService) Duplicate(req dto.DuplicateEnvironmentRequest) Result[dto.EnvironmentResponse] {
	ctx := context.Background()

	sourceID, err := uuid.Parse(req.SourceID)
	if err != nil {
		return Err[dto.EnvironmentResponse](&domain.ValidationError{
			Fields: map[string]string{"sourceId": "invalid UUID"},
		})
	}

	workspaceID, err2 := uuid.Parse(req.WorkspaceID)
	if err2 != nil {
		return Err[dto.EnvironmentResponse](&domain.ValidationError{
			Fields: map[string]string{"workspaceId": "invalid UUID"},
		})
	}

	opt := environment.DuplicateOpt{
		SourceID:    sourceID,
		NewName:     req.NewName,
		UserID:      defaultUserID,
		WorkspaceID: workspaceID,
	}

	env, err := s.uc.Duplicate(ctx, opt)
	if err != nil {
		return Err[dto.EnvironmentResponse](err)
	}

	return OK(dto.EnvironmentToResponse(env))
}

// Edit updates an existing environment.
func (s *EnvironmentService) Edit(req dto.EditEnvironmentRequest) Result[dto.EnvironmentResponse] {
	ctx := context.Background()

	envID, err := uuid.Parse(req.ID)
	if err != nil {
		return Err[dto.EnvironmentResponse](&domain.ValidationError{
			Fields: map[string]string{"id": "invalid UUID"},
		})
	}

	input := environment.Edit{Name: req.Name}
	opt := environment.EditOpt{
		EnvironmentID: envID,
		UserID:        defaultUserID,
		Version:       req.Version,
	}

	env, err := s.uc.Edit(ctx, input, opt)
	if err != nil {
		return Err[dto.EnvironmentResponse](err)
	}

	return OK(dto.EnvironmentToResponse(env))
}

// Delete soft-deletes an environment.
func (s *EnvironmentService) Delete(req dto.DeleteEnvironmentRequest) Result[Empty] {
	ctx := context.Background()

	envID, err := uuid.Parse(req.ID)
	if err != nil {
		return Err[Empty](&domain.ValidationError{
			Fields: map[string]string{"id": "invalid UUID"},
		})
	}

	opt := environment.DeleteOpt{
		EnvironmentID: envID,
		UserID:        defaultUserID,
		Version:       req.Version,
	}

	if err := s.uc.Delete(ctx, opt); err != nil {
		return Err[Empty](err)
	}

	return OK(Empty{})
}

// SetActive sets the active environment for a workspace.
func (s *EnvironmentService) SetActive(workspaceIDStr string, req dto.SetActiveEnvironmentRequest) Result[Empty] {
	ctx := context.Background()

	workspaceID, err := uuid.Parse(workspaceIDStr)
	if err != nil {
		return Err[Empty](&domain.ValidationError{
			Fields: map[string]string{"workspaceId": "invalid UUID"},
		})
	}

	envID, err := uuid.Parse(req.EnvironmentID)
	if err != nil {
		return Err[Empty](&domain.ValidationError{
			Fields: map[string]string{"environmentId": "invalid UUID"},
		})
	}

	opt := environment.SetActiveOpt{
		WorkspaceID:   workspaceID,
		EnvironmentID: envID,
	}

	if err := s.uc.SetActive(ctx, opt); err != nil {
		return Err[Empty](err)
	}

	return OK(Empty{})
}

// ListVariables returns all variables for an environment.
func (s *EnvironmentService) ListVariables(environmentID string) Result[[]dto.VariableResponse] {
	ctx := context.Background()

	envID, err := uuid.Parse(environmentID)
	if err != nil {
		return Err[[]dto.VariableResponse](&domain.ValidationError{
			Fields: map[string]string{"environmentId": "invalid UUID"},
		})
	}

	vars, err := s.uc.ListVariables(ctx, envID)
	if err != nil {
		return Err[[]dto.VariableResponse](err)
	}

	return OK(dto.VariablesToResponse(vars))
}

// AddVariable adds a new variable to an environment.
func (s *EnvironmentService) AddVariable(req dto.AddVariableRequest) Result[dto.VariableResponse] {
	ctx := context.Background()

	envID, err := uuid.Parse(req.EnvironmentID)
	if err != nil {
		return Err[dto.VariableResponse](&domain.ValidationError{
			Fields: map[string]string{"environmentId": "invalid UUID"},
		})
	}

	input := environment.AddVariable{
		EnvironmentID: envID,
		Key:           req.Key,
		Value:         req.Value,
		IsSecret:      req.IsSecret,
	}
	opt := environment.AddVariableOpt{UserID: defaultUserID}

	v, err := s.uc.AddVariable(ctx, input, opt)
	if err != nil {
		return Err[dto.VariableResponse](err)
	}

	return OK(dto.VariableToResponse(v))
}

// EditVariable updates an existing variable.
func (s *EnvironmentService) EditVariable(req dto.EditVariableRequest) Result[dto.VariableResponse] {
	ctx := context.Background()

	varID, err := uuid.Parse(req.ID)
	if err != nil {
		return Err[dto.VariableResponse](&domain.ValidationError{
			Fields: map[string]string{"id": "invalid UUID"},
		})
	}

	input := environment.EditVariable{
		Key:      req.Key,
		Value:    req.Value,
		IsSecret: req.IsSecret,
		Enabled:  req.Enabled,
	}
	opt := environment.EditVariableOpt{
		VariableID: varID,
		UserID:     defaultUserID,
		Version:    req.Version,
	}

	v, err := s.uc.EditVariable(ctx, input, opt)
	if err != nil {
		return Err[dto.VariableResponse](err)
	}

	return OK(dto.VariableToResponse(v))
}

// DeleteVariable removes a variable.
func (s *EnvironmentService) DeleteVariable(req dto.DeleteVariableRequest) Result[Empty] {
	ctx := context.Background()

	varID, err := uuid.Parse(req.ID)
	if err != nil {
		return Err[Empty](&domain.ValidationError{
			Fields: map[string]string{"id": "invalid UUID"},
		})
	}

	opt := environment.DeleteVariableOpt{VariableID: varID}

	if err := s.uc.DeleteVariable(ctx, opt); err != nil {
		return Err[Empty](err)
	}

	return OK(Empty{})
}

// ResolveVariables returns the active environment's variables as a key-value map.
func (s *EnvironmentService) ResolveVariables(workspaceIDStr string) Result[map[string]string] {
	ctx := context.Background()

	workspaceID, err := uuid.Parse(workspaceIDStr)
	if err != nil {
		return Err[map[string]string](&domain.ValidationError{
			Fields: map[string]string{"workspaceId": "invalid UUID"},
		})
	}

	vars, err := s.uc.ResolveVariables(ctx, workspaceID)
	if err != nil {
		return Err[map[string]string](err)
	}

	return OK(vars)
}
