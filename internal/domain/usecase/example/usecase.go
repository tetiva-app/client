package example

import (
	"context"

	"github.com/google/uuid"

	"github.com/tetiva-app/client/internal/domain/entities"
)

type Repository interface {
	Create(ctx context.Context, e *entities.ResponseExample) error
	GetByID(ctx context.Context, id uuid.UUID) (*entities.ResponseExample, error)
	ListByRequest(ctx context.Context, requestID uuid.UUID) ([]*entities.ResponseExample, error)
	Update(ctx context.Context, e *entities.ResponseExample) error
	// UpdateAtVersion returns *domain.ConflictError unless the live row is at baseVersion.
	UpdateAtVersion(ctx context.Context, e *entities.ResponseExample, baseVersion int) error
}

type RequestReader interface {
	GetByID(ctx context.Context, id uuid.UUID) (*entities.Request, error)
}

type CollectionReader interface {
	GetByID(ctx context.Context, id uuid.UUID) (*entities.Collection, error)
}

type Usecase interface {
	Create(ctx context.Context, in Create, opt CreateOpt) (*entities.ResponseExample, error)
	Edit(ctx context.Context, in Edit, opt EditOpt) (*entities.ResponseExample, error)
	Delete(ctx context.Context, opt DeleteOpt) error
	ListByRequest(ctx context.Context, requestID uuid.UUID) ([]*entities.ResponseExample, error)
	// DeleteByRequest, MoveToWorkspace: inside the request's transaction, no liveness check.
	DeleteByRequest(ctx context.Context, requestID uuid.UUID, userID string) error
	MoveToWorkspace(ctx context.Context, requestID, newWorkspaceID uuid.UUID, userID string) error
}

type usecase struct {
	repo        Repository
	requests    RequestReader
	collections CollectionReader
}

func NewUsecase(repo Repository, requests RequestReader, collections CollectionReader) Usecase {
	return &usecase{repo: repo, requests: requests, collections: collections}
}
