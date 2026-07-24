package workspace

import (
	"context"

	"github.com/google/uuid"

	"github.com/tetiva-app/client/internal/domain/entities"
)

// Repository defines the persistence contract for Workspace usecase (ISP).
type Repository interface {
	Create(ctx context.Context, w *entities.Workspace) error
	GetByID(ctx context.Context, id uuid.UUID) (*entities.Workspace, error)
	List(ctx context.Context) ([]*entities.Workspace, error)
	Update(ctx context.Context, w *entities.Workspace) error
	GetActive(ctx context.Context) (*entities.Workspace, error)
	SetActive(ctx context.Context, id uuid.UUID) error
	CountNonDeleted(ctx context.Context) (int, error)
	GetByRemoteID(ctx context.Context, remoteID string) (*entities.Workspace, error)
}

// Usecase defines the public API for Workspace operations.
type Usecase interface {
	Create(ctx context.Context, input Create, opt CreateOpt) (*entities.Workspace, error)
	Edit(ctx context.Context, input Edit, opt EditOpt) (*entities.Workspace, error)
	Delete(ctx context.Context, opt DeleteOpt) error
	List(ctx context.Context) ([]*entities.Workspace, error)
	GetByID(ctx context.Context, id uuid.UUID) (*entities.Workspace, error)
	GetActive(ctx context.Context) (*entities.Workspace, error)
	SetActive(ctx context.Context, opt SetActiveOpt) error
}

type usecase struct {
	repo Repository
}

// NewUsecase creates a new Workspace usecase instance.
func NewUsecase(repo Repository) Usecase {
	return &usecase{repo: repo}
}
