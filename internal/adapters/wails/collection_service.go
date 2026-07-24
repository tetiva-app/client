package wails

import (
	"context"

	"github.com/google/uuid"

	"github.com/tetiva-app/client/internal/adapters/wails/dto"
	"github.com/tetiva-app/client/internal/domain"
	"github.com/tetiva-app/client/internal/domain/entities"
	"github.com/tetiva-app/client/internal/domain/usecase/collection"
)

const defaultUserID = "local_user"

// CollectionService exposes Collection operations to the Wails frontend.
type CollectionService struct {
	uc collection.Usecase
}

// NewCollectionService creates a new CollectionService instance.
func NewCollectionService(uc collection.Usecase) *CollectionService {
	return &CollectionService{uc: uc}
}

// Create creates a new collection and returns the result.
func (s *CollectionService) Create(req dto.CreateCollectionRequest) Result[dto.CollectionResponse] {
	ctx := context.Background()

	workspaceID, err := uuid.Parse(req.WorkspaceID)
	if err != nil {
		return Err[dto.CollectionResponse](&domain.ValidationError{
			Fields: map[string]string{"workspaceId": "invalid UUID"},
		})
	}

	var parentID *uuid.UUID
	if req.ParentID != nil {
		parsed, err := uuid.Parse(*req.ParentID)
		if err != nil {
			return Err[dto.CollectionResponse](&domain.ValidationError{
				Fields: map[string]string{"parentId": "invalid UUID"},
			})
		}
		parentID = &parsed
	}

	input := collection.Create{
		Name:        req.Name,
		ParentID:    parentID,
		Description: req.Description,
		AuthType:    entities.AuthType(req.AuthType),
		AuthData:    req.AuthData,
	}
	opt := collection.CreateOpt{
		UserID:      defaultUserID,
		WorkspaceID: workspaceID,
	}

	c, err := s.uc.Create(ctx, input, opt)
	if err != nil {
		return Err[dto.CollectionResponse](err)
	}

	return OK(dto.CollectionToResponse(c))
}

// GetByID retrieves a collection by its ID.
func (s *CollectionService) GetByID(id string) Result[dto.CollectionResponse] {
	ctx := context.Background()

	parsed, err := uuid.Parse(id)
	if err != nil {
		return Err[dto.CollectionResponse](&domain.ValidationError{
			Fields: map[string]string{"id": "invalid UUID"},
		})
	}

	c, err := s.uc.GetByID(ctx, parsed)
	if err != nil {
		return Err[dto.CollectionResponse](err)
	}

	return OK(dto.CollectionToResponse(c))
}

// List returns all collections for the given workspace.
func (s *CollectionService) List(workspaceIDStr string) Result[[]dto.CollectionResponse] {
	ctx := context.Background()

	workspaceID, err := uuid.Parse(workspaceIDStr)
	if err != nil {
		return Err[[]dto.CollectionResponse](&domain.ValidationError{
			Fields: map[string]string{"workspaceId": "invalid UUID"},
		})
	}

	opt := collection.ListOpt{
		WorkspaceID: workspaceID,
	}

	collections, err := s.uc.List(ctx, opt)
	if err != nil {
		return Err[[]dto.CollectionResponse](err)
	}

	return OK(dto.CollectionsToResponse(collections))
}

// Edit updates an existing collection.
func (s *CollectionService) Edit(req dto.EditCollectionRequest) Result[dto.CollectionResponse] {
	ctx := context.Background()

	collectionID, err := uuid.Parse(req.ID)
	if err != nil {
		return Err[dto.CollectionResponse](&domain.ValidationError{
			Fields: map[string]string{"id": "invalid UUID"},
		})
	}

	input := collection.Edit{
		Name:         req.Name,
		PreScript:    req.PreScript,
		PostScript:   req.PostScript,
		Description:  req.Description,
		AuthType:     entities.AuthType(req.AuthType),
		AuthData:     req.AuthData,
		GRPCMetadata: dto.HeaderItemsToEntity(req.GRPCMetadata),
	}
	opt := collection.EditOpt{
		CollectionID: collectionID,
		UserID:       defaultUserID,
		Version:      req.Version,
	}

	c, err := s.uc.Edit(ctx, input, opt)
	if err != nil {
		return Err[dto.CollectionResponse](err)
	}

	return OK(dto.CollectionToResponse(c))
}

// Delete soft-deletes a collection by ID.
func (s *CollectionService) Delete(req dto.DeleteCollectionRequest) Result[Empty] {
	ctx := context.Background()

	collectionID, err := uuid.Parse(req.ID)
	if err != nil {
		return Err[Empty](&domain.ValidationError{
			Fields: map[string]string{"id": "invalid UUID"},
		})
	}

	opt := collection.DeleteOpt{
		CollectionID: collectionID,
		UserID:       defaultUserID,
		Version:      req.Version,
	}

	if err := s.uc.Delete(ctx, opt); err != nil {
		return Err[Empty](err)
	}

	return OK(Empty{})
}

// Move changes the parent of a collection.
func (s *CollectionService) Move(req dto.MoveCollectionRequest) Result[dto.CollectionResponse] {
	ctx := context.Background()

	collectionID, err := uuid.Parse(req.ID)
	if err != nil {
		return Err[dto.CollectionResponse](&domain.ValidationError{
			Fields: map[string]string{"id": "invalid UUID"},
		})
	}

	var targetParentID *uuid.UUID
	if req.TargetParentID != nil {
		parsed, err := uuid.Parse(*req.TargetParentID)
		if err != nil {
			return Err[dto.CollectionResponse](&domain.ValidationError{
				Fields: map[string]string{"targetParentId": "invalid UUID"},
			})
		}
		targetParentID = &parsed
	}

	opt := collection.MoveOpt{
		CollectionID:   collectionID,
		TargetParentID: targetParentID,
		UserID:         defaultUserID,
		Version:        req.Version,
	}

	c, err := s.uc.Move(ctx, opt)
	if err != nil {
		return Err[dto.CollectionResponse](err)
	}

	return OK(dto.CollectionToResponse(c))
}

// Reorder updates the sort order of a collection.
func (s *CollectionService) Reorder(req dto.ReorderCollectionRequest) Result[Empty] {
	ctx := context.Background()

	collectionID, err := uuid.Parse(req.ID)
	if err != nil {
		return Err[Empty](&domain.ValidationError{
			Fields: map[string]string{"id": "invalid UUID"},
		})
	}

	if err := s.uc.Reorder(ctx, collectionID, req.SortOrder); err != nil {
		return Err[Empty](err)
	}

	return OK(Empty{})
}
