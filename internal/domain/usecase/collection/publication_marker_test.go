package collection_test

import (
	"context"
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/tetiva-app/client/internal/domain/entities"
	"github.com/tetiva-app/client/internal/domain/usecase/collection"
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

// txRepo logs which writes happened inside the transaction.
type txRepo struct {
	*mockRepo
	tx     *recordingTx
	writes []string
}

func (r *txRepo) Update(ctx context.Context, c *entities.Collection) error {
	r.writes = append(r.writes, writeTag("update", r.tx.inTx))
	return r.mockRepo.Update(ctx, c)
}

func (r *txRepo) SoftDeleteDescendants(ctx context.Context, parentID uuid.UUID, by string, at time.Time) error {
	r.writes = append(r.writes, writeTag("descendants", r.tx.inTx))
	return r.mockRepo.SoftDeleteDescendants(ctx, parentID, by, at)
}

type recordingMarker struct {
	repo   *txRepo
	marked []uuid.UUID
	err    error
}

func (m *recordingMarker) MarkPendingUnpublish(_ context.Context, ids []uuid.UUID) error {
	m.repo.writes = append(m.repo.writes, writeTag("mark", m.repo.tx.inTx))
	m.marked = append(m.marked, ids...)
	return m.err
}

func writeTag(name string, inTx bool) string {
	if inTx {
		return name + "@tx"
	}
	return name
}

func seedRoot(t *testing.T, repo *mockRepo) *entities.Collection {
	t.Helper()
	c := &entities.Collection{ID: uuid.New(), WorkspaceID: testWorkspaceID, Name: "Root", Version: 1}
	if err := repo.Create(context.Background(), c); err != nil {
		t.Fatal(err)
	}
	return c
}

func TestDelete_MarksPendingUnpublishInTheDeleteTransaction(t *testing.T) {
	tx := &recordingTx{}
	repo := &txRepo{mockRepo: newMockRepo(), tx: tx}
	marker := &recordingMarker{repo: repo}
	uc := collection.NewUsecase(repo, nil, tx, marker)
	root := seedRoot(t, repo.mockRepo)

	if err := uc.Delete(context.Background(), collection.DeleteOpt{CollectionID: root.ID, UserID: "u", Version: 1}); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	if want := []string{"update@tx", "descendants@tx", "mark@tx"}; !slices.Equal(repo.writes, want) {
		t.Errorf("writes = %v, want %v", repo.writes, want)
	}
	if tx.runs != 1 {
		t.Errorf("transactions = %d, want 1", tx.runs)
	}
	if !slices.Equal(marker.marked, []uuid.UUID{root.ID}) {
		t.Errorf("marked = %v, want [%s]", marker.marked, root.ID)
	}
}

func TestDelete_FailedMarkFailsTheDelete(t *testing.T) {
	tx := &recordingTx{}
	repo := &txRepo{mockRepo: newMockRepo(), tx: tx}
	boom := errors.New("disk full")
	uc := collection.NewUsecase(repo, nil, tx, &recordingMarker{repo: repo, err: boom})
	root := seedRoot(t, repo.mockRepo)

	err := uc.Delete(context.Background(), collection.DeleteOpt{CollectionID: root.ID, UserID: "u", Version: 1})
	if !errors.Is(err, boom) {
		t.Fatalf("Delete = %v, want the marker's error", err)
	}
}

func TestDelete_WithoutTxOrMarker(t *testing.T) {
	repo := newMockRepo()
	uc := collection.NewUsecase(repo, nil, nil, nil)
	root := seedRoot(t, repo)

	if err := uc.Delete(context.Background(), collection.DeleteOpt{CollectionID: root.ID, UserID: "u", Version: 1}); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if got, _ := repo.GetByID(context.Background(), root.ID); !got.IsDelete {
		t.Error("collection not deleted")
	}
}
