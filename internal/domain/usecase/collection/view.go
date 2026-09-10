package collection

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"

	"github.com/tetiva-app/client/internal/domain"
	"github.com/tetiva-app/client/internal/domain/entities"
)

type Filter struct {
	WorkspaceID uuid.UUID
	ParentID    *uuid.UUID
}

type ListOpt struct {
	WorkspaceID uuid.UUID
	ParentID    *uuid.UUID
}

type DeleteOpt struct {
	CollectionID uuid.UUID
	UserID       string
	Version      int
}

type MoveOpt struct {
	CollectionID   uuid.UUID
	TargetParentID *uuid.UUID
	UserID         string
	Version        int
}

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

func (u *usecase) List(ctx context.Context, opt ListOpt) ([]*entities.Collection, error) {
	const funcName = "collection.List"

	filter := Filter(opt)

	collections, err := u.repo.List(ctx, filter)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", funcName, err)
	}

	return collections, nil
}

// Delete is a soft delete: the row stays with is_delete = true.
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

	owner := entities.AuthOwner{
		WorkspaceID: existing.WorkspaceID,
		Kind:        entities.AuthOwnerKindCollection,
		ID:          existing.ID,
	}
	if err := u.tokens().Clear(ctx, owner); err != nil {
		return fmt.Errorf("%s: clear tokens: %w", funcName, err)
	}
	// Requests under the subtree keep is_delete = 0, so only the ancestor-walking sweep
	// reaches their tokens; the delete is committed, so a failed sweep waits for the next one.
	if _, err := u.tokens().DeleteOrphans(ctx); err != nil {
		slog.Warn("collection: token sweep failed", "op", funcName, "err", err)
	}

	return nil
}

func (u *usecase) Reorder(ctx context.Context, id uuid.UUID, sortOrder int) error {
	const funcName = "collection.Reorder"

	if err := u.repo.UpdateSortOrder(ctx, id, sortOrder); err != nil {
		return fmt.Errorf("%s: %w", funcName, err)
	}

	return nil
}
