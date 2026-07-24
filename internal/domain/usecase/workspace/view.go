package workspace

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/tetiva-app/client/internal/domain"
	"github.com/tetiva-app/client/internal/domain/entities"
)

// DeleteOpt holds contextual options for workspace deletion.
type DeleteOpt struct {
	WorkspaceID uuid.UUID
	UserID      string
	Version     int
}

// SetActiveOpt holds contextual options for setting active workspace.
type SetActiveOpt struct {
	WorkspaceID uuid.UUID
}

func (u *usecase) Delete(ctx context.Context, opt DeleteOpt) error {
	const funcName = "workspace.Delete"

	count, err := u.repo.CountNonDeleted(ctx)
	if err != nil {
		return fmt.Errorf("%s: %w", funcName, err)
	}
	if count <= 1 {
		return &domain.ValidationError{
			Fields: map[string]string{"workspace": "cannot delete the last workspace"},
		}
	}

	existing, err := u.repo.GetByID(ctx, opt.WorkspaceID)
	if err != nil {
		return fmt.Errorf("%s: %w", funcName, err)
	}
	if existing == nil {
		return &domain.NotFoundError{Entity: "workspace", ID: opt.WorkspaceID.String()}
	}
	if existing.Version != opt.Version {
		return &domain.ConflictError{Entity: "workspace", ID: opt.WorkspaceID.String()}
	}

	wasActive := existing.IsActive

	existing.IsDelete = true
	existing.IsActive = false
	existing.Version++
	existing.UpdatedBy = opt.UserID
	existing.UpdatedAt = time.Now()

	if err := u.repo.Update(ctx, existing); err != nil {
		return fmt.Errorf("%s: %w", funcName, err)
	}

	if wasActive {
		remaining, err := u.repo.List(ctx)
		if err != nil {
			return fmt.Errorf("%s: list remaining: %w", funcName, err)
		}
		if len(remaining) > 0 {
			if err := u.repo.SetActive(ctx, remaining[0].ID); err != nil {
				return fmt.Errorf("%s: auto-activate: %w", funcName, err)
			}
		}
	}

	return nil
}

func (u *usecase) List(ctx context.Context) ([]*entities.Workspace, error) {
	const funcName = "workspace.List"

	workspaces, err := u.repo.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", funcName, err)
	}

	return workspaces, nil
}

func (u *usecase) GetByID(ctx context.Context, id uuid.UUID) (*entities.Workspace, error) {
	const funcName = "workspace.GetByID"

	w, err := u.repo.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", funcName, err)
	}
	if w == nil {
		return nil, &domain.NotFoundError{Entity: "workspace", ID: id.String()}
	}

	return w, nil
}

func (u *usecase) GetActive(ctx context.Context) (*entities.Workspace, error) {
	const funcName = "workspace.GetActive"

	w, err := u.repo.GetActive(ctx)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", funcName, err)
	}
	if w == nil {
		return nil, &domain.NotFoundError{Entity: "workspace", ID: "active"}
	}

	return w, nil
}

func (u *usecase) SetActive(ctx context.Context, opt SetActiveOpt) error {
	const funcName = "workspace.SetActive"

	w, err := u.repo.GetByID(ctx, opt.WorkspaceID)
	if err != nil {
		return fmt.Errorf("%s: %w", funcName, err)
	}
	if w == nil {
		return &domain.NotFoundError{Entity: "workspace", ID: opt.WorkspaceID.String()}
	}

	if err := u.repo.SetActive(ctx, opt.WorkspaceID); err != nil {
		return fmt.Errorf("%s: %w", funcName, err)
	}

	return nil
}
