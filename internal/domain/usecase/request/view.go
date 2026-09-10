package request

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
	CollectionID uuid.UUID
}

type ListOpt struct {
	CollectionID uuid.UUID
}

type DeleteOpt struct {
	RequestID uuid.UUID
	UserID    string
	Version   int
}

type MoveOpt struct {
	RequestID          uuid.UUID
	TargetCollectionID uuid.UUID
	UserID             string
	Version            int
}

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

	// A move can land the request in another workspace, and the sweep drops a token row whose
	// workspace no longer matches its owner; the move is committed, so a failed sweep waits.
	if _, err := u.tokens().DeleteOrphans(ctx); err != nil {
		slog.Warn("request: token sweep failed", "op", funcName, "err", err)
	}

	return existing, nil
}

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

func (u *usecase) List(ctx context.Context, opt ListOpt) ([]*entities.Request, error) {
	const funcName = "request.List"

	filter := Filter(opt)

	requests, err := u.repo.List(ctx, filter)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", funcName, err)
	}

	return requests, nil
}

// Delete is a soft delete: the row stays with is_delete = true.
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

	// A deleted request must not leave a usable token behind.
	if err := u.tokens().ClearOwners(ctx, entities.AuthOwnerKindRequest, []uuid.UUID{opt.RequestID}); err != nil {
		return fmt.Errorf("%s: clear tokens: %w", funcName, err)
	}

	return nil
}

func (u *usecase) Reorder(ctx context.Context, id uuid.UUID, sortOrder int) error {
	const funcName = "request.Reorder"

	if err := u.repo.UpdateSortOrder(ctx, id, sortOrder); err != nil {
		return fmt.Errorf("%s: %w", funcName, err)
	}

	return nil
}
