package collection

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/tetiva-app/client/internal/domain"
	"github.com/tetiva-app/client/internal/domain/entities"
)

// Filter defines criteria for listing collections.
type Filter struct {
	WorkspaceID uuid.UUID
	ParentID    *uuid.UUID
}

// ListOpt holds contextual options for the List operation.
type ListOpt struct {
	WorkspaceID uuid.UUID
	ParentID    *uuid.UUID
}

// DeleteOpt holds contextual options for the Delete operation.
type DeleteOpt struct {
	CollectionID uuid.UUID
	UserID       string
	Version      int
}

// MoveOpt holds contextual options for the Move operation.
type MoveOpt struct {
	CollectionID   uuid.UUID
	TargetParentID *uuid.UUID
	UserID         string
	Version        int
}

// Move changes the parent of a collection, with circular dependency validation.
func (u *usecase) Move(ctx context.Context, opt MoveOpt) (*entities.Collection, error) {
	const funcName = "collection.Move"

	existing, err := u.repo.GetByID(ctx, opt.CollectionID)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", funcName, err)
	}
	if existing == nil {
		return nil, &domain.NotFoundError{Entity: "collection", ID: opt.CollectionID.String()}
	}
	if existing.Version != opt.Version {
		return nil, &domain.ConflictError{Entity: "collection", ID: opt.CollectionID.String()}
	}

	if opt.TargetParentID != nil && *opt.TargetParentID == opt.CollectionID {
		return nil, &domain.ValidationError{Fields: map[string]string{"targetParentId": "cannot move collection into itself"}}
	}

	// Moving into a descendant would create a cycle.
	if opt.TargetParentID != nil {
		isDesc, err := u.isDescendant(ctx, opt.CollectionID, *opt.TargetParentID)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", funcName, err)
		}
		if isDesc {
			return nil, &domain.ValidationError{Fields: map[string]string{"targetParentId": "cannot move collection into its own descendant"}}
		}
	}

	existing.ParentID = opt.TargetParentID
	existing.Version++
	existing.UpdatedBy = opt.UserID
	existing.UpdatedAt = time.Now()

	if err := u.repo.Update(ctx, existing); err != nil {
		return nil, fmt.Errorf("%s: %w", funcName, err)
	}

	return existing, nil
}

// isDescendant checks if candidateID is a descendant of ancestorID by walking up the tree.
func (u *usecase) isDescendant(ctx context.Context, ancestorID, candidateID uuid.UUID) (bool, error) {
	current := candidateID
	for {
		c, err := u.repo.GetByID(ctx, current)
		if err != nil {
			return false, err
		}
		if c == nil || c.ParentID == nil {
			return false, nil
		}
		if *c.ParentID == ancestorID {
			return true, nil
		}
		current = *c.ParentID
	}
}

// GetByID retrieves a single collection by its ID.
func (u *usecase) GetByID(ctx context.Context, id uuid.UUID) (*entities.Collection, error) {
	const funcName = "collection.GetByID"

	c, err := u.repo.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", funcName, err)
	}
	if c == nil {
		return nil, &domain.NotFoundError{Entity: "collection", ID: id.String()}
	}

	return c, nil
}

// List returns collections matching the given options.
func (u *usecase) List(ctx context.Context, opt ListOpt) ([]*entities.Collection, error) {
	const funcName = "collection.List"

	filter := Filter(opt)

	collections, err := u.repo.List(ctx, filter)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", funcName, err)
	}

	return collections, nil
}

// Delete performs a soft delete on the collection (sets is_delete = true).
func (u *usecase) Delete(ctx context.Context, opt DeleteOpt) error {
	const funcName = "collection.Delete"

	existing, err := u.repo.GetByID(ctx, opt.CollectionID)
	if err != nil {
		return fmt.Errorf("%s: %w", funcName, err)
	}
	if existing == nil {
		return &domain.NotFoundError{Entity: "collection", ID: opt.CollectionID.String()}
	}
	if existing.Version != opt.Version {
		return &domain.ConflictError{Entity: "collection", ID: opt.CollectionID.String()}
	}

	now := time.Now()

	existing.IsDelete = true
	existing.Version++
	existing.UpdatedBy = opt.UserID
	existing.UpdatedAt = now

	if err := u.repo.Update(ctx, existing); err != nil {
		return fmt.Errorf("%s: %w", funcName, err)
	}

	if err := u.repo.SoftDeleteDescendants(ctx, opt.CollectionID, opt.UserID, now); err != nil {
		return fmt.Errorf("%s: soft delete descendants: %w", funcName, err)
	}

	return nil
}

// Reorder updates the sort order of a collection.
func (u *usecase) Reorder(ctx context.Context, id uuid.UUID, sortOrder int) error {
	const funcName = "collection.Reorder"

	if err := u.repo.UpdateSortOrder(ctx, id, sortOrder); err != nil {
		return fmt.Errorf("%s: %w", funcName, err)
	}

	return nil
}
