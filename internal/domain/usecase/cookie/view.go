package cookie

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/tetiva-app/client/internal/domain/entities"
)

// Filter — criteria for listing cookies.
type Filter struct {
	WorkspaceID uuid.UUID
	Domain      string // optional; empty = all
}

// ListOpt — options for List.
type ListOpt struct {
	WorkspaceID uuid.UUID
	Domain      string
}

// DeleteOpt — options for Delete.
type DeleteOpt struct {
	ID uuid.UUID
}

func (u *usecase) List(ctx context.Context, opt ListOpt) ([]*entities.Cookie, error) {
	const funcName = "cookie.List"
	cookies, err := u.repo.List(ctx, Filter(opt))
	if err != nil {
		return nil, fmt.Errorf("%s: %w", funcName, err)
	}
	return cookies, nil
}

func (u *usecase) Delete(ctx context.Context, opt DeleteOpt) error {
	const funcName = "cookie.Delete"
	if opt.ID == uuid.Nil {
		return fmt.Errorf("%s: id required", funcName)
	}
	if err := u.repo.Delete(ctx, opt.ID); err != nil {
		return fmt.Errorf("%s: %w", funcName, err)
	}
	return nil
}

func (u *usecase) DeleteByDomain(ctx context.Context, workspaceID uuid.UUID, domain string) (int, error) {
	const funcName = "cookie.DeleteByDomain"
	n, err := u.repo.DeleteByDomain(ctx, workspaceID, domain)
	if err != nil {
		return 0, fmt.Errorf("%s: %w", funcName, err)
	}
	return n, nil
}

func (u *usecase) Clear(ctx context.Context, workspaceID uuid.UUID) (int, error) {
	const funcName = "cookie.Clear"
	n, err := u.repo.Clear(ctx, workspaceID)
	if err != nil {
		return 0, fmt.Errorf("%s: %w", funcName, err)
	}
	return n, nil
}
