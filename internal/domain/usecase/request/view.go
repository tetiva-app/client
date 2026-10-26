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

// DuplicateOpt leaves the original's method and URL on the copy when Method or URL is empty.
type DuplicateOpt struct {
	RequestID uuid.UUID
	Name      string
	Method    entities.HTTPMethod
	URL       string
	UserID    string
}

func (o *DuplicateOpt) Validate() error {
	if o.Name == "" {
		return &domain.ValidationError{Fields: map[string]string{"name": "required"}}
	}
	if o.Method != "" && !o.Method.IsValid() {
		return &domain.ValidationError{Fields: map[string]string{"method": "invalid"}}
	}
	return nil
}

func (u *usecase) Duplicate(ctx context.Context, opt DuplicateOpt) (*entities.Request, error) {
	const funcName = "request.Duplicate"

	if err := opt.Validate(); err != nil {
		return nil, err
	}

	var dup entities.Request
	err := u.tx().Run(ctx, func(txCtx context.Context) error {
		source, err := u.repo.GetByID(txCtx, opt.RequestID)
		if err != nil {
			return err
		}
		if source == nil {
			return &domain.NotFoundError{Entity: "request", ID: opt.RequestID.String()}
		}
		if source.IsDraft {
			return &domain.ValidationError{Fields: map[string]string{"request": "save the request before duplicating it"}}
		}

		now := time.Now()
		dup = *source
		dup.ID = uuid.New()
		dup.Name = opt.Name
		if opt.Method != "" {
			dup.Method = opt.Method
		}
		if opt.URL != "" {
			dup.URL = opt.URL
		}
		dup.SortOrder = newRequestSortOrder
		dup.Version = 1
		dup.CreatedBy = opt.UserID
		dup.CreatedAt = now
		dup.UpdatedBy = opt.UserID
		dup.UpdatedAt = now
		if err := u.repo.Create(txCtx, &dup); err != nil {
			return err
		}
		if u.exampleCleaner == nil {
			return nil
		}
		return u.exampleCleaner.CopyToRequest(txCtx, source.ID, dup.ID, opt.UserID)
	})
	if err != nil {
		return nil, fmt.Errorf("%s: %w", funcName, err)
	}

	return &dup, nil
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

	err = u.tx().Run(ctx, func(txCtx context.Context) error {
		if err := u.repo.Update(txCtx, existing); err != nil {
			return err
		}
		return u.moveExamples(txCtx, existing.ID, opt.TargetCollectionID, opt.UserID)
	})
	if err != nil {
		return nil, fmt.Errorf("%s: %w", funcName, err)
	}

	// A move can land the request in another workspace, and the sweep drops a token row whose
	// workspace no longer matches its owner; the move is committed, so a failed sweep waits.
	if _, err := u.tokens().DeleteOrphans(ctx); err != nil {
		slog.Warn("request: token sweep failed", "op", funcName, "err", err)
	}

	return existing, nil
}

func (u *usecase) moveExamples(ctx context.Context, requestID, collectionID uuid.UUID, userID string) error {
	if u.exampleCleaner == nil || u.collectionReader == nil {
		return nil
	}
	target, err := u.collectionReader.GetByID(ctx, collectionID)
	if err != nil {
		return err
	}
	if target == nil {
		return nil
	}
	return u.exampleCleaner.MoveToWorkspace(ctx, requestID, target.WorkspaceID, userID)
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

	err = u.tx().Run(ctx, func(txCtx context.Context) error {
		if err := u.repo.Update(txCtx, existing); err != nil {
			return err
		}
		if u.exampleCleaner == nil {
			return nil
		}
		return u.exampleCleaner.DeleteByRequest(txCtx, existing.ID, opt.UserID)
	})
	if err != nil {
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
