package wails

import (
	"context"

	"github.com/google/uuid"

	"github.com/tetiva-app/client/internal/adapters/wails/dto"
	"github.com/tetiva-app/client/internal/domain"
	"github.com/tetiva-app/client/internal/domain/usecase/workspace"
)

// WorkspaceService exposes Workspace operations to the Wails frontend.
type WorkspaceService struct {
	uc workspace.Usecase
}

func NewWorkspaceService(uc workspace.Usecase) *WorkspaceService {
	return &WorkspaceService{uc: uc}
}

func (s *WorkspaceService) List() Result[[]dto.WorkspaceResponse] {
	ctx := context.Background()

	workspaces, err := s.uc.List(ctx)
	if err != nil {
		return Err[[]dto.WorkspaceResponse](err)
	}

	return OK(dto.WorkspacesToResponse(workspaces))
}

func (s *WorkspaceService) Create(req dto.CreateWorkspaceRequest) Result[dto.WorkspaceResponse] {
	ctx := context.Background()

	input := workspace.Create{Name: req.Name}
	opt := workspace.CreateOpt{UserID: defaultUserID}

	w, err := s.uc.Create(ctx, input, opt)
	if err != nil {
		return Err[dto.WorkspaceResponse](err)
	}

	return OK(dto.WorkspaceToResponse(w))
}

func (s *WorkspaceService) Edit(req dto.EditWorkspaceRequest) Result[dto.WorkspaceResponse] {
	ctx := context.Background()

	workspaceID, err := uuid.Parse(req.ID)
	if err != nil {
		return Err[dto.WorkspaceResponse](&domain.ValidationError{
			Fields: map[string]string{"id": "invalid UUID"},
		})
	}

	input := workspace.Edit{Name: req.Name}
	opt := workspace.EditOpt{
		WorkspaceID: workspaceID,
		UserID:      defaultUserID,
		Version:     req.Version,
	}

	w, err := s.uc.Edit(ctx, input, opt)
	if err != nil {
		return Err[dto.WorkspaceResponse](err)
	}

	return OK(dto.WorkspaceToResponse(w))
}

func (s *WorkspaceService) Delete(req dto.DeleteWorkspaceRequest) Result[Empty] {
	ctx := context.Background()

	workspaceID, err := uuid.Parse(req.ID)
	if err != nil {
		return Err[Empty](&domain.ValidationError{
			Fields: map[string]string{"id": "invalid UUID"},
		})
	}

	opt := workspace.DeleteOpt{
		WorkspaceID: workspaceID,
		UserID:      defaultUserID,
		Version:     req.Version,
	}

	if err := s.uc.Delete(ctx, opt); err != nil {
		return Err[Empty](err)
	}

	return OK(Empty{})
}

func (s *WorkspaceService) GetActive() Result[dto.WorkspaceResponse] {
	ctx := context.Background()

	w, err := s.uc.GetActive(ctx)
	if err != nil {
		return Err[dto.WorkspaceResponse](err)
	}

	return OK(dto.WorkspaceToResponse(w))
}

func (s *WorkspaceService) SetActive(req dto.SetActiveWorkspaceRequest) Result[Empty] {
	ctx := context.Background()

	workspaceID, err := uuid.Parse(req.WorkspaceID)
	if err != nil {
		return Err[Empty](&domain.ValidationError{
			Fields: map[string]string{"workspaceId": "invalid UUID"},
		})
	}

	opt := workspace.SetActiveOpt{WorkspaceID: workspaceID}

	if err := s.uc.SetActive(ctx, opt); err != nil {
		return Err[Empty](err)
	}

	return OK(Empty{})
}
