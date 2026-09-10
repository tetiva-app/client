// Package cookie defines the per-workspace cookie jar usecase.
package cookie

import (
	"context"
	"net/url"

	"github.com/google/uuid"

	"github.com/tetiva-app/client/internal/domain/entities"
)

type Repository interface {
	Upsert(ctx context.Context, c *entities.Cookie) error
	GetByID(ctx context.Context, id uuid.UUID) (*entities.Cookie, error)
	List(ctx context.Context, filter Filter) ([]*entities.Cookie, error)
	Delete(ctx context.Context, id uuid.UUID) error
	DeleteByDomain(ctx context.Context, workspaceID uuid.UUID, domain string) (int, error)
	Clear(ctx context.Context, workspaceID uuid.UUID) (int, error)
	// MatchForRequest filters by domain, path, secure flag and expiration.
	MatchForRequest(ctx context.Context, workspaceID uuid.UUID, u *url.URL) ([]*entities.Cookie, error)
}

type Usecase interface {
	Add(ctx context.Context, input Add, opt Opt) (*entities.Cookie, error)
	Edit(ctx context.Context, input Edit, opt Opt) (*entities.Cookie, error)
	List(ctx context.Context, opt ListOpt) ([]*entities.Cookie, error)
	Delete(ctx context.Context, opt DeleteOpt) error
	DeleteByDomain(ctx context.Context, workspaceID uuid.UUID, domain string) (int, error)
	Clear(ctx context.Context, workspaceID uuid.UUID) (int, error)
}

// Opt — generic context for Add/Edit (placeholder for future permission checks).
type Opt struct {
	UserID string
}

type usecase struct {
	repo Repository
}

func NewUsecase(repo Repository) Usecase {
	return &usecase{repo: repo}
}
