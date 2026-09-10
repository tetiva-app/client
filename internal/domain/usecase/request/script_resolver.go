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

func NewScriptResolver(cr CollectionReader) ScriptResolver {
	return &scriptResolver{collectionReader: cr}
}

func (r *scriptResolver) ResolvePreScript(ctx context.Context, req *entities.Request) (string, error) {
	if req.PreScript != "" {
		return req.PreScript, nil
	}
	return r.walkChain(ctx, req.CollectionID, func(c *entities.Collection) string {
		return c.PreScript
	})
}

func (r *scriptResolver) ResolvePostScript(ctx context.Context, req *entities.Request) (string, error) {
	if req.PostScript != "" {
		return req.PostScript, nil
	}
	return r.walkChain(ctx, req.CollectionID, func(c *entities.Collection) string {
		return c.PostScript
	})
}

// walkChain returns the first non-empty script the getter finds walking up the hierarchy.
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
