// Package history defines the request execution history usecase.
package history

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/tetiva-app/client/internal/domain"
	"github.com/tetiva-app/client/internal/domain/entities"
)

type Usecase interface {
	List(ctx context.Context, opt ListOpt) ([]*entities.History, int, error) // items, total count
	GetByID(ctx context.Context, id uuid.UUID, workspaceID uuid.UUID) (*entities.History, error)
	Delete(ctx context.Context, opt DeleteOpt) error
	Clear(ctx context.Context, opt ClearOpt) error
}

type Repository interface {
	GetByID(ctx context.Context, id uuid.UUID) (*entities.History, error)
	List(ctx context.Context, filter Filter) ([]*entities.History, error)
	Count(ctx context.Context, filter Filter) (int, error)
	Delete(ctx context.Context, id uuid.UUID) error
	DeleteAll(ctx context.Context, workspaceID uuid.UUID) error
}

type usecase struct {
	repo Repository
}

func NewUsecase(repo Repository) Usecase {
	return &usecase{repo: repo}
}

func (u *usecase) List(ctx context.Context, opt ListOpt) ([]*entities.History, int, error) {
	const funcName = "history.List"
	if opt.WorkspaceID == uuid.Nil {
		return nil, 0, &domain.ValidationError{Fields: map[string]string{"workspaceId": "required"}}
	}
	f := opt.Filter
	f.WorkspaceID = opt.WorkspaceID
	if f.Limit <= 0 {
		f.Limit = DefaultPageSize
	}

	items, err := u.repo.List(ctx, f)
	if err != nil {
		return nil, 0, fmt.Errorf("%s: %w", funcName, err)
	}
	total, err := u.repo.Count(ctx, f)
	if err != nil {
		return nil, 0, fmt.Errorf("%s: count: %w", funcName, err)
	}
	return items, total, nil
}

func (u *usecase) GetByID(ctx context.Context, id, workspaceID uuid.UUID) (*entities.History, error) {
	const funcName = "history.GetByID"
	h, err := u.repo.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", funcName, err)
	}
	if h == nil {
		return nil, &domain.NotFoundError{Entity: "history", ID: id.String()}
	}
	if h.WorkspaceID != workspaceID {
		return nil, &domain.NotFoundError{Entity: "history", ID: id.String()}
	}
	return h, nil
}

func (u *usecase) Delete(ctx context.Context, opt DeleteOpt) error {
	const funcName = "history.Delete"
	h, err := u.repo.GetByID(ctx, opt.HistoryID)
	if err != nil {
		return fmt.Errorf("%s: %w", funcName, err)
	}
	if h == nil {
		return &domain.NotFoundError{Entity: "history", ID: opt.HistoryID.String()}
	}
	if h.WorkspaceID != opt.WorkspaceID {
		return &domain.NotFoundError{Entity: "history", ID: opt.HistoryID.String()}
	}
	if err := u.repo.Delete(ctx, opt.HistoryID); err != nil {
		return fmt.Errorf("%s: %w", funcName, err)
	}
	return nil
}

func (u *usecase) Clear(ctx context.Context, opt ClearOpt) error {
	const funcName = "history.Clear"
	if opt.WorkspaceID == uuid.Nil {
		return &domain.ValidationError{Fields: map[string]string{"workspaceId": "required"}}
	}
	if err := u.repo.DeleteAll(ctx, opt.WorkspaceID); err != nil {
		return fmt.Errorf("%s: %w", funcName, err)
	}
	return nil
}
