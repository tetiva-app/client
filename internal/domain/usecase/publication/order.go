package publication

import (
	"bytes"
	"cmp"
	"slices"
	"time"

	"github.com/google/uuid"

	"github.com/tetiva-app/client/internal/domain/entities"
)

// tree is the live part of the input in publication order. Repositories order by sort_order and
// created_at only, so the id breaks ties here: the same data must give the same bytes.
type tree struct {
	children  map[uuid.UUID][]*entities.Collection
	requests  map[uuid.UUID][]*entities.Request
	examples  map[uuid.UUID][]*entities.ResponseExample
	variables []*entities.Variable
}

func newTree(in BuildInput) *tree {
	t := &tree{
		children: map[uuid.UUID][]*entities.Collection{},
		requests: map[uuid.UUID][]*entities.Request{},
		examples: map[uuid.UUID][]*entities.ResponseExample{},
	}
	for _, c := range in.Collections {
		if c == nil || c.IsDelete || c.ParentID == nil || c.ID == in.Root.ID {
			continue
		}
		t.children[*c.ParentID] = append(t.children[*c.ParentID], c)
	}
	for _, r := range in.Requests {
		if r == nil || r.IsDelete || r.IsDraft {
			continue
		}
		t.requests[r.CollectionID] = append(t.requests[r.CollectionID], r)
	}
	for requestID, list := range in.Examples {
		for _, e := range list {
			if e != nil && !e.IsDelete {
				t.examples[requestID] = append(t.examples[requestID], e)
			}
		}
	}
	if in.Environment != nil {
		for _, v := range in.Variables {
			if v != nil && !v.IsDelete && v.Enabled && v.EnvironmentID == in.Environment.ID {
				t.variables = append(t.variables, v)
			}
		}
	}

	for _, list := range t.children {
		slices.SortFunc(list, func(a, b *entities.Collection) int {
			return compareEntity(a.SortOrder, a.CreatedAt, a.ID, b.SortOrder, b.CreatedAt, b.ID)
		})
	}
	for _, list := range t.requests {
		slices.SortFunc(list, func(a, b *entities.Request) int {
			return compareEntity(a.SortOrder, a.CreatedAt, a.ID, b.SortOrder, b.CreatedAt, b.ID)
		})
	}
	for _, list := range t.examples {
		slices.SortFunc(list, func(a, b *entities.ResponseExample) int {
			return compareEntity(a.SortOrder, a.CreatedAt, a.ID, b.SortOrder, b.CreatedAt, b.ID)
		})
	}
	slices.SortFunc(t.variables, func(a, b *entities.Variable) int {
		return compareEntity(a.SortOrder, a.CreatedAt, a.ID, b.SortOrder, b.CreatedAt, b.ID)
	})
	return t
}

func compareEntity(aSort int, aAt time.Time, aID uuid.UUID, bSort int, bAt time.Time, bID uuid.UUID) int {
	if c := cmp.Compare(aSort, bSort); c != 0 {
		return c
	}
	if c := aAt.Compare(bAt); c != 0 {
		return c
	}
	return bytes.Compare(aID[:], bID[:])
}
