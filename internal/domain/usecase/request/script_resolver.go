package request

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/tetiva-app/client/internal/domain/entities"
)

const maxCollectionDepth = 50

type scriptResolver struct {
	collectionReader CollectionReader
}

// NewScriptResolver creates a ScriptResolver that walks the collection hierarchy.
func NewScriptResolver(cr CollectionReader) ScriptResolver {
	return &scriptResolver{collectionReader: cr}
}

// ResolvePreScript returns the effective pre-script for a request.
func (r *scriptResolver) ResolvePreScript(ctx context.Context, req *entities.Request) (string, error) {
	if req.PreScript != "" {
		return req.PreScript, nil
	}
	return r.walkChain(ctx, req.CollectionID, func(c *entities.Collection) string {
		return c.PreScript
	})
}

// ResolvePostScript returns the effective post-script for a request.
func (r *scriptResolver) ResolvePostScript(ctx context.Context, req *entities.Request) (string, error) {
	if req.PostScript != "" {
		return req.PostScript, nil
	}
	return r.walkChain(ctx, req.CollectionID, func(c *entities.Collection) string {
		return c.PostScript
	})
}

// walkChain walks the collection hierarchy upwards,
// returning the first non-empty script found by the getter.
func (r *scriptResolver) walkChain(ctx context.Context, collectionID uuid.UUID, getter func(*entities.Collection) string) (string, error) {
	const funcName = "scriptResolver.walkChain"

	currentID := collectionID
	for depth := 0; depth < maxCollectionDepth; depth++ {
		coll, err := r.collectionReader.GetByID(ctx, currentID)
		if err != nil {
			return "", fmt.Errorf("%s: %w", funcName, err)
		}
		if coll == nil {
			return "", nil
		}

		if script := getter(coll); script != "" {
			return script, nil
		}

		if coll.ParentID == nil {
			return "", nil
		}
		currentID = *coll.ParentID
	}

	return "", nil
}
