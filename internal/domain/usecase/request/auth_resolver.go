package request

import (
	"context"
	"fmt"

	"github.com/tetiva-app/client/internal/domain/entities"
)

// AuthResolver resolves the effective auth for a request,
// walking up the collection hierarchy if the request uses inherit.
type AuthResolver interface {
	ResolveAuth(ctx context.Context, req *entities.Request) (entities.AuthType, string, error)
}

type authResolver struct {
	collectionReader CollectionReader
}

// NewAuthResolver creates an AuthResolver that walks the collection hierarchy.
func NewAuthResolver(cr CollectionReader) AuthResolver {
	return &authResolver{collectionReader: cr}
}

// ResolveAuth returns the effective auth type and data for a request.
func (r *authResolver) ResolveAuth(ctx context.Context, req *entities.Request) (entities.AuthType, string, error) {
	if req.AuthType != entities.AuthTypeInherit {
		return req.AuthType, req.AuthData, nil
	}

	const funcName = "authResolver.ResolveAuth"
	currentID := req.CollectionID

	for depth := 0; depth < maxCollectionDepth; depth++ {
		coll, err := r.collectionReader.GetByID(ctx, currentID)
		if err != nil {
			return entities.AuthTypeNone, "{}", fmt.Errorf("%s: %w", funcName, err)
		}
		if coll == nil {
			return entities.AuthTypeNone, "{}", nil
		}

		if coll.AuthType != entities.AuthTypeNone {
			return coll.AuthType, coll.AuthData, nil
		}

		if coll.ParentID == nil {
			return entities.AuthTypeNone, "{}", nil
		}
		currentID = *coll.ParentID
	}

	return entities.AuthTypeNone, "{}", nil
}
