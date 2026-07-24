package collection

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/tetiva-app/client/internal/domain/entities"
)

// Repository defines the persistence contract for Collection usecase (ISP).
type Repository interface {
	Create(ctx context.Context, c *entities.Collection) error
	GetByID(ctx context.Context, id uuid.UUID) (*entities.Collection, error)
	List(ctx context.Context, filter Filter) ([]*entities.Collection, error)
	Update(ctx context.Context, c *entities.Collection) error
	UpdateSortOrder(ctx context.Context, id uuid.UUID, sortOrder int) error
	SoftDeleteDescendants(ctx context.Context, parentID uuid.UUID, updatedBy string, updatedAt time.Time) error
}

// Usecase defines the public API for Collection operations.
type Usecase interface {
	Create(ctx context.Context, input Create, opt CreateOpt) (*entities.Collection, error)
	GetByID(ctx context.Context, id uuid.UUID) (*entities.Collection, error)
	List(ctx context.Context, opt ListOpt) ([]*entities.Collection, error)
	Edit(ctx context.Context, input Edit, opt EditOpt) (*entities.Collection, error)
	Delete(ctx context.Context, opt DeleteOpt) error
	Reorder(ctx context.Context, id uuid.UUID, sortOrder int) error
	Move(ctx context.Context, opt MoveOpt) (*entities.Collection, error)
}

type usecase struct {
	repo Repository
}

// NewUsecase creates a new Collection usecase instance.
func NewUsecase(repo Repository) Usecase {
	return &usecase{repo: repo}
}
