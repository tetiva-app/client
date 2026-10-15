package request

import (
	"context"
	"fmt"
	"strings"

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

func (r *scriptResolver) ResolvePreScript(ctx context.Context, req *entities.Request) (ResolvedScript, error) {
	if strings.TrimSpace(req.PreScript) != "" {
		return ResolvedScript{Body: req.PreScript}, nil
	}
	return r.walkChain(ctx, req.CollectionID, func(c *entities.Collection) string {
		return c.PreScript
	})
}

func (r *scriptResolver) ResolvePostScript(ctx context.Context, req *entities.Request) (ResolvedScript, error) {
	if strings.TrimSpace(req.PostScript) != "" {
		return ResolvedScript{Body: req.PostScript}, nil
	}
	return r.walkChain(ctx, req.CollectionID, func(c *entities.Collection) string {
		return c.PostScript
	})
}

// walkChain returns the first non-blank script the getter finds walking up the hierarchy.
func (r *scriptResolver) walkChain(ctx context.Context, collectionID uuid.UUID, getter func(*entities.Collection) string) (ResolvedScript, error) {
	const funcName = "scriptResolver.walkChain"

	currentID := collectionID
	for depth := 0; depth < maxCollectionDepth; depth++ {
		coll, err := r.collectionReader.GetByID(ctx, currentID)
		if err != nil {
			return ResolvedScript{}, fmt.Errorf("%s: %w", funcName, err)
		}
		if coll == nil {
			return ResolvedScript{}, nil
		}

		if script := getter(coll); strings.TrimSpace(script) != "" {
			return ResolvedScript{Body: script, Collection: coll.Name}, nil
		}

		if coll.ParentID == nil {
			return ResolvedScript{}, nil
		}
		currentID = *coll.ParentID
	}

	return ResolvedScript{}, nil
}
