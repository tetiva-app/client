package wails

import (
	"context"

	"github.com/google/uuid"

	"github.com/tetiva-app/client/internal/adapters/wails/dto"
	"github.com/tetiva-app/client/internal/domain"
	"github.com/tetiva-app/client/internal/domain/usecase/search"
)

// SearchService exposes the search usecase to the Wails frontend.
type SearchService struct {
	uc search.Usecase
}

// NewSearchService wires the usecase.
func NewSearchService(uc search.Usecase) *SearchService {
	return &SearchService{uc: uc}
}

// InWorkspace runs a name-only substring search within a workspace.
func (s *SearchService) InWorkspace(workspaceID string, query string, limit int) Result[dto.SearchResponse] {
	ctx := context.Background()

	wsID, err := uuid.Parse(workspaceID)
	if err != nil {
		return Err[dto.SearchResponse](&domain.ValidationError{
			Fields: map[string]string{"workspaceId": "invalid UUID"},
		})
	}

	result, err := s.uc.InWorkspace(ctx, search.Input{
		WorkspaceID: wsID,
		Query:       query,
		Limit:       limit,
	})
	if err != nil {
		return Err[dto.SearchResponse](err)
	}
	return OK(dto.ToSearchResponse(result))
}
