package search_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/tetiva-app/client/internal/domain"
	"github.com/tetiva-app/client/internal/domain/entities"
	"github.com/tetiva-app/client/internal/domain/usecase/search"
)

var testWS = uuid.MustParse("00000000-0000-4000-a000-000000000001")

type stubRepo struct {
	lastFilter search.Filter
	result     entities.SearchResult
	callCount  int
}

func (s *stubRepo) SearchByName(_ context.Context, f search.Filter) (entities.SearchResult, error) {
	s.lastFilter = f
	s.callCount++
	return s.result, nil
}

func TestInWorkspace_EmptyQuery_NoRepoCall(t *testing.T) {
	repo := &stubRepo{}
	uc := search.NewUsecase(repo)

	got, err := uc.InWorkspace(context.Background(), search.Input{WorkspaceID: testWS, Query: ""})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got.Hits) != 0 || got.LimitReached {
		t.Errorf("expected empty result, got %+v", got)
	}
	if repo.callCount != 0 {
		t.Errorf("repo should not be called, got %d calls", repo.callCount)
	}
}

func TestInWorkspace_ShortQuery_NoRepoCall(t *testing.T) {
	repo := &stubRepo{}
	uc := search.NewUsecase(repo)

	got, err := uc.InWorkspace(context.Background(), search.Input{WorkspaceID: testWS, Query: "a"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got.Hits) != 0 {
		t.Errorf("expected empty result, got %+v", got)
	}
	if repo.callCount != 0 {
		t.Errorf("repo should not be called, got %d calls", repo.callCount)
	}
}

func TestInWorkspace_TrimAndLowercase(t *testing.T) {
	repo := &stubRepo{}
	uc := search.NewUsecase(repo)

	_, err := uc.InWorkspace(context.Background(), search.Input{WorkspaceID: testWS, Query: "  АВТОРИЗАЦИЯ  "})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if repo.lastFilter.Query != "авторизация" {
		t.Errorf("expected lowercased trimmed 'авторизация', got %q", repo.lastFilter.Query)
	}
}

func TestInWorkspace_DefaultLimit(t *testing.T) {
	repo := &stubRepo{}
	uc := search.NewUsecase(repo)

	_, err := uc.InWorkspace(context.Background(), search.Input{WorkspaceID: testWS, Query: "foo", Limit: 0})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if repo.lastFilter.Limit != 200 {
		t.Errorf("expected default limit 200, got %d", repo.lastFilter.Limit)
	}
}

func TestInWorkspace_CustomLimit(t *testing.T) {
	repo := &stubRepo{}
	uc := search.NewUsecase(repo)

	_, err := uc.InWorkspace(context.Background(), search.Input{WorkspaceID: testWS, Query: "foo", Limit: 50})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if repo.lastFilter.Limit != 50 {
		t.Errorf("expected limit 50, got %d", repo.lastFilter.Limit)
	}
}

func TestInWorkspace_MissingWorkspaceID(t *testing.T) {
	repo := &stubRepo{}
	uc := search.NewUsecase(repo)

	_, err := uc.InWorkspace(context.Background(), search.Input{Query: "foo"})
	if err == nil {
		t.Fatal("expected error for missing workspace_id")
	}
	var valErr *domain.ValidationError
	if !errors.As(err, &valErr) {
		t.Fatalf("expected ValidationError, got %T: %v", err, err)
	}
	if _, has := valErr.Fields["workspaceId"]; !has {
		t.Errorf("expected workspaceId in fields, got %+v", valErr.Fields)
	}
}
