package history_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/tetiva-app/client/internal/domain"
	"github.com/tetiva-app/client/internal/domain/entities"
	"github.com/tetiva-app/client/internal/domain/usecase/history"
)

type fakeRepo struct {
	listFn      func(ctx context.Context, f history.Filter) ([]*entities.History, error)
	countFn     func(ctx context.Context, f history.Filter) (int, error)
	getByIDFn   func(ctx context.Context, id uuid.UUID) (*entities.History, error)
	deleteFn    func(ctx context.Context, id uuid.UUID) error
	deleteAllFn func(ctx context.Context, workspaceID uuid.UUID) error
}

func (f *fakeRepo) List(ctx context.Context, fi history.Filter) ([]*entities.History, error) {
	return f.listFn(ctx, fi)
}

func (f *fakeRepo) Count(ctx context.Context, fi history.Filter) (int, error) {
	return f.countFn(ctx, fi)
}

func (f *fakeRepo) GetByID(ctx context.Context, id uuid.UUID) (*entities.History, error) {
	return f.getByIDFn(ctx, id)
}

func (f *fakeRepo) Delete(ctx context.Context, id uuid.UUID) error {
	return f.deleteFn(ctx, id)
}

func (f *fakeRepo) DeleteAll(ctx context.Context, ws uuid.UUID) error {
	return f.deleteAllFn(ctx, ws)
}

func TestList_RejectsZeroWorkspace(t *testing.T) {
	uc := history.NewUsecase(&fakeRepo{})
	_, _, err := uc.List(context.Background(), history.ListOpt{})
	var v *domain.ValidationError
	if !errors.As(err, &v) {
		t.Fatalf("want ValidationError, got %v", err)
	}
}

func TestList_AppliesDefaultLimit(t *testing.T) {
	var seen history.Filter
	repo := &fakeRepo{
		listFn:  func(ctx context.Context, f history.Filter) ([]*entities.History, error) { seen = f; return nil, nil },
		countFn: func(ctx context.Context, f history.Filter) (int, error) { return 0, nil },
	}
	uc := history.NewUsecase(repo)
	ws := uuid.New()
	_, _, err := uc.List(context.Background(), history.ListOpt{WorkspaceID: ws})
	if err != nil {
		t.Fatal(err)
	}
	if seen.Limit != history.DefaultPageSize {
		t.Errorf("limit = %d, want %d", seen.Limit, history.DefaultPageSize)
	}
	if seen.WorkspaceID != ws {
		t.Errorf("workspaceID propagated incorrectly")
	}
}

func TestGetByID_WorkspaceMismatchReturnsNotFound(t *testing.T) {
	id := uuid.New()
	other := uuid.New()
	repo := &fakeRepo{
		getByIDFn: func(ctx context.Context, _ uuid.UUID) (*entities.History, error) {
			return &entities.History{ID: id, WorkspaceID: other, CreatedAt: time.Now()}, nil
		},
	}
	uc := history.NewUsecase(repo)
	_, err := uc.GetByID(context.Background(), id, uuid.New())
	var nf *domain.NotFoundError
	if !errors.As(err, &nf) {
		t.Fatalf("want NotFoundError, got %v", err)
	}
}

func TestDelete_GuardsCrossWorkspace(t *testing.T) {
	id := uuid.New()
	other := uuid.New()
	deleted := false
	repo := &fakeRepo{
		getByIDFn: func(ctx context.Context, _ uuid.UUID) (*entities.History, error) {
			return &entities.History{ID: id, WorkspaceID: other}, nil
		},
		deleteFn: func(ctx context.Context, _ uuid.UUID) error { deleted = true; return nil },
	}
	uc := history.NewUsecase(repo)
	err := uc.Delete(context.Background(), history.DeleteOpt{HistoryID: id, WorkspaceID: uuid.New()})
	var nf *domain.NotFoundError
	if !errors.As(err, &nf) {
		t.Fatalf("want NotFoundError, got %v", err)
	}
	if deleted {
		t.Error("delete must not be invoked on workspace mismatch")
	}
}

func TestClear_RejectsZeroWorkspace(t *testing.T) {
	uc := history.NewUsecase(&fakeRepo{})
	err := uc.Clear(context.Background(), history.ClearOpt{})
	var v *domain.ValidationError
	if !errors.As(err, &v) {
		t.Fatalf("want ValidationError, got %v", err)
	}
}

func TestClear_DelegatesToRepo(t *testing.T) {
	called := uuid.Nil
	repo := &fakeRepo{
		deleteAllFn: func(ctx context.Context, ws uuid.UUID) error { called = ws; return nil },
	}
	uc := history.NewUsecase(repo)
	ws := uuid.New()
	if err := uc.Clear(context.Background(), history.ClearOpt{WorkspaceID: ws}); err != nil {
		t.Fatal(err)
	}
	if called != ws {
		t.Errorf("DeleteAll called with %v, want %v", called, ws)
	}
}
