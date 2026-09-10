package search

import (
	"context"

	"github.com/google/uuid"

	"github.com/tetiva-app/client/internal/domain/entities"
)

// Filter is the low-level repository filter (query already trimmed+lowercased by usecase).
type Filter struct {
	WorkspaceID uuid.UUID
	Query       string // non-empty, >= 2 runes, already strings.ToLower'ed
	Limit       int    // > 0
}

type Input struct {
	WorkspaceID uuid.UUID
	Query       string
	Limit       int // 0 → default 200
}

const (
	defaultLimit = 200
	minQueryLen  = 2
)

type Repository interface {
	SearchByName(ctx context.Context, filter Filter) (entities.SearchResult, error)
}

type Usecase interface {
	InWorkspace(ctx context.Context, input Input) (entities.SearchResult, error)
}

type usecase struct {
	repo Repository
}

func NewUsecase(repo Repository) Usecase {
	return &usecase{repo: repo}
}
