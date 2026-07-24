package wails

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tetiva-app/client/internal/domain/entities"
	"github.com/tetiva-app/client/internal/domain/usecase/search"
)

// stubSearchUsecase is an injectable search.Usecase double.
type stubSearchUsecase struct {
	inWorkspaceFn func(context.Context, search.Input) (entities.SearchResult, error)
}

func (s *stubSearchUsecase) InWorkspace(ctx context.Context, in search.Input) (entities.SearchResult, error) {
	if s.inWorkspaceFn == nil {
		panic("inWorkspaceFn not set")
	}
	return s.inWorkspaceFn(ctx, in)
}

func TestSearchService_InWorkspace_Happy(t *testing.T) {
	wsID := uuid.New()
	parentID := uuid.New()
	hitID := uuid.New()
	method := "GET"
	proto := entities.Protocol("http")

	uc := &stubSearchUsecase{
		inWorkspaceFn: func(_ context.Context, in search.Input) (entities.SearchResult, error) {
			assert.Equal(t, wsID, in.WorkspaceID)
			assert.Equal(t, "users", in.Query)
			assert.Equal(t, 50, in.Limit)
			return entities.SearchResult{
				Hits: []entities.SearchHit{
					{
						ID:       hitID,
						Kind:     entities.HitRequest,
						Name:     "list users",
						ParentID: &parentID,
						Protocol: &proto,
						Method:   &method,
					},
				},
				LimitReached: true,
			}, nil
		},
	}
	svc := NewSearchService(uc)

	res := svc.InWorkspace(wsID.String(), "users", 50)

	require.Nil(t, res.Error)
	require.Len(t, res.Data.Hits, 1)
	assert.True(t, res.Data.LimitReached)
	hit := res.Data.Hits[0]
	assert.Equal(t, hitID.String(), hit.ID)
	assert.Equal(t, "request", hit.Kind)
	assert.Equal(t, "list users", hit.Name)
	require.NotNil(t, hit.ParentID)
	assert.Equal(t, parentID.String(), *hit.ParentID)
	require.NotNil(t, hit.Method)
	assert.Equal(t, "GET", *hit.Method)
	require.NotNil(t, hit.Protocol)
	assert.Equal(t, "http", *hit.Protocol)
}

func TestSearchService_InWorkspace_Error_BadUUID(t *testing.T) {
	// usecase must NOT be invoked when UUID parsing fails.
	uc := &stubSearchUsecase{}
	svc := NewSearchService(uc)

	res := svc.InWorkspace("not-a-uuid", "anything", 10)

	require.NotNil(t, res.Error)
	assert.Equal(t, ErrCodeValidation, res.Error.Code)
	assert.Contains(t, res.Error.Fields, "workspaceId")
}
