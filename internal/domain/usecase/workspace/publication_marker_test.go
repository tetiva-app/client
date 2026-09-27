package workspace

import (
	"context"
	"errors"
	"slices"
	"testing"

	"github.com/google/uuid"

	"github.com/tetiva-app/client/internal/domain/entities"
)

type recordingTx struct {
	inTx bool
	runs int
}

func (r *recordingTx) Run(ctx context.Context, fn func(ctx context.Context) error) error {
	r.runs++
	r.inTx = true
	defer func() { r.inTx = false }()
	return fn(ctx)
}

type txRepo struct {
	*mockRepo
	tx     *recordingTx
	writes []string
}

func (r *txRepo) Update(ctx context.Context, w *entities.Workspace) error {
	r.writes = append(r.writes, writeTag("update", r.tx.inTx))
	return r.mockRepo.Update(ctx, w)
}

type recordingMarker struct {
	repo   *txRepo
	marked []uuid.UUID
	err    error
}

func (m *recordingMarker) MarkPendingUnpublishWorkspace(_ context.Context, id uuid.UUID) error {
	m.repo.writes = append(m.repo.writes, writeTag("mark", m.repo.tx.inTx))
	m.marked = append(m.marked, id)
	return m.err
}

func writeTag(name string, inTx bool) string {
	if inTx {
		return name + "@tx"
	}
	return name
}

func seedTwoWorkspaces(t *testing.T, repo *mockRepo) *entities.Workspace {
	t.Helper()
	doomed := &entities.Workspace{ID: uuid.New(), Name: "Doomed", Version: 1}
	for _, w := range []*entities.Workspace{doomed, {ID: uuid.New(), Name: "Other", Version: 1}} {
		if err := repo.Create(context.Background(), w); err != nil {
			t.Fatal(err)
		}
	}
	return doomed
}

func TestDelete_MarksPendingUnpublishInTheDeleteTransaction(t *testing.T) {
	tx := &recordingTx{}
	repo := &txRepo{mockRepo: newMockRepo(), tx: tx}
	marker := &recordingMarker{repo: repo}
	uc := NewUsecase(repo, tx, marker)
	doomed := seedTwoWorkspaces(t, repo.mockRepo)

	if err := uc.Delete(context.Background(), DeleteOpt{WorkspaceID: doomed.ID, UserID: "u", Version: 1}); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if want := []string{"update@tx", "mark@tx"}; !slices.Equal(repo.writes, want) {
		t.Errorf("writes = %v, want %v", repo.writes, want)
	}
	if tx.runs != 1 {
		t.Errorf("transactions = %d, want 1", tx.runs)
	}
	if !slices.Equal(marker.marked, []uuid.UUID{doomed.ID}) {
		t.Errorf("marked = %v, want [%s]", marker.marked, doomed.ID)
	}
}

func TestDelete_FailedMarkFailsTheDelete(t *testing.T) {
	tx := &recordingTx{}
	repo := &txRepo{mockRepo: newMockRepo(), tx: tx}
	boom := errors.New("disk full")
	uc := NewUsecase(repo, tx, &recordingMarker{repo: repo, err: boom})
	doomed := seedTwoWorkspaces(t, repo.mockRepo)

	if err := uc.Delete(context.Background(), DeleteOpt{WorkspaceID: doomed.ID, UserID: "u", Version: 1}); !errors.Is(err, boom) {
		t.Fatalf("Delete = %v, want the marker's error", err)
	}
}
