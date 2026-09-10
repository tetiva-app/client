package app

import (
	"context"

	"github.com/google/uuid"

	"github.com/tetiva-app/client/internal/domain/entities"
	"github.com/tetiva-app/client/internal/domain/usecase/collection"
	"github.com/tetiva-app/client/internal/domain/usecase/request"
)

// collectionReaderAdapter narrows collection.Repository: the request layer lists by workspace, not by Filter.
type collectionReaderAdapter struct {
	cr collection.Repository
}

func NewCollectionReaderAdapter(cr collection.Repository) request.CollectionReader {
	return &collectionReaderAdapter{cr: cr}
}

func (a *collectionReaderAdapter) GetByID(ctx context.Context, id uuid.UUID) (*entities.Collection, error) {
	return a.cr.GetByID(ctx, id)
}

func (a *collectionReaderAdapter) ListByWorkspace(ctx context.Context, workspaceID uuid.UUID) ([]*entities.Collection, error) {
	return a.cr.List(ctx, collection.Filter{WorkspaceID: workspaceID})
}
