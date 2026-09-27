package example

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/tetiva-app/client/internal/domain"
	"github.com/tetiva-app/client/internal/domain/entities"
	"github.com/tetiva-app/client/internal/domain/secrets"
)

type DeleteOpt struct {
	ExampleID uuid.UUID
	UserID    string
	Version   int
}

func ChainWorkspace(ctx context.Context, requests RequestReader, collections CollectionReader, requestID uuid.UUID) (uuid.UUID, bool, error) {
	const funcName = "example.ChainWorkspace"

	req, err := requests.GetByID(ctx, requestID)
	if err != nil {
		return uuid.Nil, false, fmt.Errorf("%s: %w", funcName, err)
	}
	if req == nil || req.IsDelete {
		return uuid.Nil, false, nil
	}
	return chainWorkspaceFrom(ctx, collections, req)
}

func chainWorkspaceFrom(ctx context.Context, collections CollectionReader, req *entities.Request) (uuid.UUID, bool, error) {
	const funcName = "example.chainWorkspaceFrom"

	var workspaceID uuid.UUID
	seen := make(map[uuid.UUID]bool)
	for id := req.CollectionID; ; {
		if seen[id] {
			return uuid.Nil, false, nil
		}
		seen[id] = true

		c, err := collections.GetByID(ctx, id)
		if err != nil {
			return uuid.Nil, false, fmt.Errorf("%s: %w", funcName, err)
		}
		if c == nil || c.IsDelete {
			return uuid.Nil, false, nil
		}
		if workspaceID == uuid.Nil {
			workspaceID = c.WorkspaceID
		} else if c.WorkspaceID != workspaceID {
			return uuid.Nil, false, nil
		}
		if c.ParentID == nil {
			return workspaceID, true, nil
		}
		id = *c.ParentID
	}
}

func (u *usecase) isLive(ctx context.Context, e *entities.ResponseExample) (bool, error) {
	if e.IsDelete {
		return false, nil
	}
	workspaceID, ok, err := ChainWorkspace(ctx, u.requests, u.collections, e.RequestID)
	if err != nil {
		return false, err
	}
	return ok && workspaceID == e.WorkspaceID, nil
}

func (u *usecase) getLive(ctx context.Context, id uuid.UUID) (*entities.ResponseExample, error) {
	e, err := u.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if e == nil {
		return nil, &domain.NotFoundError{Entity: "example", ID: id.String()}
	}
	live, err := u.isLive(ctx, e)
	if err != nil {
		return nil, err
	}
	if !live {
		return nil, &domain.NotFoundError{Entity: "example", ID: id.String()}
	}
	return e, nil
}

func (u *usecase) ListByRequest(ctx context.Context, requestID uuid.UUID) ([]*entities.ResponseExample, error) {
	const funcName = "example.ListByRequest"

	workspaceID, ok, err := ChainWorkspace(ctx, u.requests, u.collections, requestID)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", funcName, err)
	}
	if !ok {
		return []*entities.ResponseExample{}, nil
	}

	all, err := u.repo.ListByRequest(ctx, requestID)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", funcName, err)
	}
	out := make([]*entities.ResponseExample, 0, len(all))
	for _, e := range all {
		if e.WorkspaceID == workspaceID {
			out = append(out, e)
		}
	}
	return out, nil
}

func (u *usecase) Delete(ctx context.Context, opt DeleteOpt) error {
	const funcName = "example.Delete"

	e, err := u.getLive(ctx, opt.ExampleID)
	if err != nil {
		return fmt.Errorf("%s: %w", funcName, err)
	}
	if e.Version != opt.Version {
		return &domain.ConflictError{Entity: "example", ID: opt.ExampleID.String()}
	}

	markDeleted(e, opt.UserID, time.Now())
	if err := u.repo.UpdateAtVersion(ctx, e, opt.Version); err != nil {
		return fmt.Errorf("%s: %w", funcName, err)
	}
	return nil
}

func (u *usecase) DeleteByRequest(ctx context.Context, requestID uuid.UUID, userID string) error {
	const funcName = "example.DeleteByRequest"

	examples, err := u.repo.ListByRequest(ctx, requestID)
	if err != nil {
		return fmt.Errorf("%s: %w", funcName, err)
	}
	now := time.Now()
	for _, e := range examples {
		markDeleted(e, userID, now)
		if err := u.repo.Update(ctx, e); err != nil {
			return fmt.Errorf("%s: %w", funcName, err)
		}
	}
	return nil
}

// MoveToWorkspace copies: sync routes by WorkspaceID, and the old one needs tombstones.
func (u *usecase) MoveToWorkspace(ctx context.Context, requestID, newWorkspaceID uuid.UUID, userID string) error {
	const funcName = "example.MoveToWorkspace"

	examples, err := u.repo.ListByRequest(ctx, requestID)
	if err != nil {
		return fmt.Errorf("%s: %w", funcName, err)
	}
	now := time.Now()
	for _, e := range examples {
		if e.WorkspaceID == newWorkspaceID {
			continue
		}
		moved := *e
		moved.ID = uuid.New()
		moved.WorkspaceID = newWorkspaceID
		moved.Headers = secrets.RedactHeaders(e.Headers)
		moved.Version = 1
		moved.CreatedBy = userID
		moved.CreatedAt = now
		moved.UpdatedBy = userID
		moved.UpdatedAt = now
		if err := u.repo.Create(ctx, &moved); err != nil {
			return fmt.Errorf("%s: copy %s: %w", funcName, e.ID, err)
		}

		markDeleted(e, userID, now)
		if err := u.repo.Update(ctx, e); err != nil {
			return fmt.Errorf("%s: delete %s: %w", funcName, e.ID, err)
		}
	}
	return nil
}

func markDeleted(e *entities.ResponseExample, userID string, now time.Time) {
	e.IsDelete = true
	e.Version++
	e.UpdatedBy = userID
	e.UpdatedAt = now
}
