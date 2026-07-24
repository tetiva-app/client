package request

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/tetiva-app/client/internal/domain"
	"github.com/tetiva-app/client/internal/domain/entities"
)

// Filter defines criteria for listing requests.
type Filter struct {
	CollectionID uuid.UUID
}

// ListOpt holds contextual options for the List operation.
type ListOpt struct {
	CollectionID uuid.UUID
}

// DeleteOpt holds contextual options for the Delete operation.
type DeleteOpt struct {
	RequestID uuid.UUID
	UserID    string
	Version   int
}

// MoveOpt holds contextual options for the Move operation.
type MoveOpt struct {
	RequestID          uuid.UUID
	TargetCollectionID uuid.UUID
	UserID             string
	Version            int
}

// Move changes the collection of a request.
func (u *usecase) Move(ctx context.Context, opt MoveOpt) (*entities.Request, error) {
	const funcName = "request.Move"

	existing, err := u.repo.GetByID(ctx, opt.RequestID)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", funcName, err)
	}
	if existing == nil {
		return nil, &domain.NotFoundError{Entity: "request", ID: opt.RequestID.String()}
	}
	if existing.Version != opt.Version {
		return nil, &domain.ConflictError{Entity: "request", ID: opt.RequestID.String()}
	}

	existing.CollectionID = opt.TargetCollectionID
	existing.Version++
	existing.UpdatedBy = opt.UserID
	existing.UpdatedAt = time.Now()

	if err := u.repo.Update(ctx, existing); err != nil {
		return nil, fmt.Errorf("%s: %w", funcName, err)
	}

	return existing, nil
}

// GetByID retrieves a single request by its ID.
func (u *usecase) GetByID(ctx context.Context, id uuid.UUID) (*entities.Request, error) {
	const funcName = "request.GetByID"

	r, err := u.repo.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", funcName, err)
	}
	if r == nil {
		return nil, &domain.NotFoundError{Entity: "request", ID: id.String()}
	}

	return r, nil
}

// List returns requests matching the given options.
func (u *usecase) List(ctx context.Context, opt ListOpt) ([]*entities.Request, error) {
	const funcName = "request.List"

	filter := Filter(opt)

	requests, err := u.repo.List(ctx, filter)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", funcName, err)
	}

	return requests, nil
}

// Delete performs a soft delete on the request (sets is_delete = true).
func (u *usecase) Delete(ctx context.Context, opt DeleteOpt) error {
	const funcName = "request.Delete"

	existing, err := u.repo.GetByID(ctx, opt.RequestID)
	if err != nil {
		return fmt.Errorf("%s: %w", funcName, err)
	}
	if existing == nil {
		return &domain.NotFoundError{Entity: "request", ID: opt.RequestID.String()}
	}
	if existing.Version != opt.Version {
		return &domain.ConflictError{Entity: "request", ID: opt.RequestID.String()}
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

// Reorder updates the sort order of a request.
func (u *usecase) Reorder(ctx context.Context, id uuid.UUID, sortOrder int) error {
	const funcName = "request.Reorder"

	if err := u.repo.UpdateSortOrder(ctx, id, sortOrder); err != nil {
		return fmt.Errorf("%s: %w", funcName, err)
	}

	return nil
}
