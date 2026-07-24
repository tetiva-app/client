package search

import (
	"context"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"

	"github.com/tetiva-app/client/internal/domain"
	"github.com/tetiva-app/client/internal/domain/entities"
)

// InWorkspace searches by name within a single workspace.
// Short/empty queries return empty result without touching the repository.
func (u *usecase) InWorkspace(ctx context.Context, input Input) (entities.SearchResult, error) {
	const funcName = "search.InWorkspace"

	if input.WorkspaceID == uuid.Nil {
		return entities.SearchResult{}, fmt.Errorf("%s: %w", funcName, &domain.ValidationError{
			Fields: map[string]string{"workspaceId": "required"},
		})
	}

	query := strings.ToLower(strings.TrimSpace(input.Query))
	if utf8.RuneCountInString(query) < minQueryLen {
		return entities.SearchResult{}, nil
	}

	limit := input.Limit
	if limit <= 0 {
		limit = defaultLimit
	}

	result, err := u.repo.SearchByName(ctx, Filter{
		WorkspaceID: input.WorkspaceID,
		Query:       query,
		Limit:       limit,
	})
	if err != nil {
		return entities.SearchResult{}, fmt.Errorf("%s: repo search: %w", funcName, err)
	}
	return result, nil
}
