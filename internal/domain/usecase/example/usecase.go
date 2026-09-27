package example

import (
	"context"

	"github.com/google/uuid"

	"github.com/tetiva-app/client/internal/domain/entities"
)

type Repository interface {
	Create(ctx context.Context, e *entities.ResponseExample) error
	// GetByID returns nil for a missing or soft-deleted row.
	GetByID(ctx context.Context, id uuid.UUID) (*entities.ResponseExample, error)
	// ListByRequest skips soft-deleted rows but does not check the request or its collections.
	ListByRequest(ctx context.Context, requestID uuid.UUID) ([]*entities.ResponseExample, error)
	// Update matches the row by id, soft-deleted or not.
	Update(ctx context.Context, e *entities.ResponseExample) error
	// UpdateAtVersion writes e only over the live row still at baseVersion and returns
	// *domain.ConflictError otherwise: a read-then-write from two windows must not both land.
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
	// DeleteByRequest and MoveToWorkspace run inside the request's own transaction, after the
	// request row changed, so they skip the liveness check every other method applies.
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
