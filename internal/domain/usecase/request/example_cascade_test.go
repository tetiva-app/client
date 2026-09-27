package request_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/tetiva-app/client/internal/domain/entities"
	"github.com/tetiva-app/client/internal/domain/usecase/request"
)

type txMarker struct{}

type snapshotTx struct {
	repo *mockRepo
	runs int
}

func (t *snapshotTx) Run(ctx context.Context, fn func(ctx context.Context) error) error {
	t.runs++
	saved := make(map[uuid.UUID]*entities.Request, len(t.repo.requests))
	for id, r := range t.repo.requests {
		cp := *r
		saved[id] = &cp
	}
	if err := fn(context.WithValue(ctx, txMarker{}, true)); err != nil {
		t.repo.requests = saved
		return err
	}
	return nil
}

type cascadeCall struct {
	requestID   uuid.UUID
	workspaceID uuid.UUID
	userID      string
	inTx        bool
}

type recordingExampleCleaner struct {
	deletes []cascadeCall
	moves   []cascadeCall
	err     error
}

func (c *recordingExampleCleaner) DeleteByRequest(ctx context.Context, requestID uuid.UUID, userID string) error {
	c.deletes = append(c.deletes, cascadeCall{requestID: requestID, userID: userID, inTx: ctx.Value(txMarker{}) != nil})
	return c.err
}

func (c *recordingExampleCleaner) MoveToWorkspace(ctx context.Context, requestID, newWorkspaceID uuid.UUID, userID string) error {
	c.moves = append(c.moves, cascadeCall{requestID: requestID, workspaceID: newWorkspaceID, userID: userID, inTx: ctx.Value(txMarker{}) != nil})
	return c.err
}

var otherWorkspaceID = uuid.MustParse("00000000-0000-4000-a000-000000000002")

func cascadeCollections() (*mockCollectionReader, uuid.UUID) {
	cols := fixtureCollections()
	otherID := uuid.New()
	cols.collections[otherID] = &entities.Collection{ID: otherID, WorkspaceID: otherWorkspaceID}
	return cols, otherID
}

func newCascadeUsecase(repo *mockRepo, cols request.CollectionReader, tokens request.TokenCleaner,
	cleaner request.ExampleCleaner, tx request.TxRunner) request.Usecase {
	return request.NewUsecase(repo, &mockHistoryRepo{}, &mockRequester{}, nil, nil,
		&mockEnvResolver{}, &noopScriptEngine{}, &noopScriptResolver{}, &noopVarPersister{},
		request.NewAuthResolver(fixtureCollections()), nil, cols, tokens, nil, cleaner, tx)
}

func TestDelete_CascadesExamplesInsideTx(t *testing.T) {
	repo := newMockRepo()
	req := seedRequest(repo)
	cleaner := &recordingExampleCleaner{}
	tx := &snapshotTx{repo: repo}
	uc := newCascadeUsecase(repo, fixtureCollections(), nil, cleaner, tx)

	if err := uc.Delete(context.Background(), request.DeleteOpt{RequestID: req.ID, UserID: "u1", Version: 1}); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	if tx.runs != 1 {
		t.Errorf("tx runs = %d, want 1", tx.runs)
	}
	if len(cleaner.deletes) != 1 {
		t.Fatalf("cascade calls = %d, want 1", len(cleaner.deletes))
	}
	call := cleaner.deletes[0]
	if call.requestID != req.ID || call.userID != "u1" || !call.inTx {
		t.Errorf("cascade call = %+v", call)
	}
	if !repo.requests[req.ID].IsDelete {
		t.Error("request not deleted")
	}
}

func TestDelete_CascadeFailureKeepsRequestAlive(t *testing.T) {
	repo := newMockRepo()
	req := seedRequest(repo)
	tokens := &recordingCleaner{}
	cleaner := &recordingExampleCleaner{err: errors.New("disk full")}
	uc := newCascadeUsecase(repo, fixtureCollections(), tokens, cleaner, &snapshotTx{repo: repo})

	err := uc.Delete(context.Background(), request.DeleteOpt{RequestID: req.ID, UserID: "u1", Version: 1})
	if !errors.Is(err, cleaner.err) {
		t.Fatalf("err = %v, want the cascade error", err)
	}

	got := repo.requests[req.ID]
	if got.IsDelete || got.Version != 1 {
		t.Errorf("request = deleted %v, version %d; want it untouched", got.IsDelete, got.Version)
	}
	if len(tokens.clearedIDs) != 0 {
		t.Error("tokens cleared for a request that is still alive")
	}
}

func TestMove_ToAnotherWorkspaceMovesExamplesInsideTx(t *testing.T) {
	repo := newMockRepo()
	req := seedRequest(repo)
	cols, otherCollection := cascadeCollections()
	cleaner := &recordingExampleCleaner{}
	uc := newCascadeUsecase(repo, cols, nil, cleaner, &snapshotTx{repo: repo})

	moved, err := uc.Move(context.Background(), request.MoveOpt{
		RequestID: req.ID, TargetCollectionID: otherCollection, UserID: "u1", Version: 1,
	})
	if err != nil {
		t.Fatalf("Move: %v", err)
	}

	if moved.ID != req.ID || moved.CollectionID != otherCollection {
		t.Errorf("moved = %s in %s", moved.ID, moved.CollectionID)
	}
	if len(cleaner.moves) != 1 {
		t.Fatalf("example moves = %d, want 1", len(cleaner.moves))
	}
	call := cleaner.moves[0]
	if call.requestID != req.ID || call.workspaceID != otherWorkspaceID || call.userID != "u1" || !call.inTx {
		t.Errorf("example move = %+v", call)
	}
}

func TestMove_ExampleFailureKeepsRequestInPlace(t *testing.T) {
	repo := newMockRepo()
	req := seedRequest(repo)
	cols, otherCollection := cascadeCollections()
	cleaner := &recordingExampleCleaner{err: errors.New("disk full")}
	uc := newCascadeUsecase(repo, cols, nil, cleaner, &snapshotTx{repo: repo})

	_, err := uc.Move(context.Background(), request.MoveOpt{
		RequestID: req.ID, TargetCollectionID: otherCollection, UserID: "u1", Version: 1,
	})
	if !errors.Is(err, cleaner.err) {
		t.Fatalf("err = %v, want the example move error", err)
	}

	got := repo.requests[req.ID]
	if got.CollectionID != testCollectionID || got.Version != 1 {
		t.Errorf("request = in %s, version %d; want it untouched", got.CollectionID, got.Version)
	}
}

func TestMove_UnknownTargetSkipsExamples(t *testing.T) {
	repo := newMockRepo()
	req := seedRequest(repo)
	cleaner := &recordingExampleCleaner{}
	uc := newCascadeUsecase(repo, fixtureCollections(), nil, cleaner, &snapshotTx{repo: repo})

	if _, err := uc.Move(context.Background(), request.MoveOpt{
		RequestID: req.ID, TargetCollectionID: uuid.New(), UserID: "u1", Version: 1,
	}); err != nil {
		t.Fatalf("Move: %v", err)
	}
	if len(cleaner.moves) != 0 {
		t.Errorf("example moves = %d, want none", len(cleaner.moves))
	}
}

func TestDeleteAndMove_WithoutExampleCleanerOrTx(t *testing.T) {
	repo := newMockRepo()
	cols, otherCollection := cascadeCollections()
	uc := newCascadeUsecase(repo, cols, nil, nil, nil)

	moving := seedRequest(repo)
	if _, err := uc.Move(context.Background(), request.MoveOpt{
		RequestID: moving.ID, TargetCollectionID: otherCollection, UserID: "u1", Version: 1,
	}); err != nil {
		t.Fatalf("Move: %v", err)
	}

	deleting := seedRequest(repo)
	if err := uc.Delete(context.Background(), request.DeleteOpt{RequestID: deleting.ID, UserID: "u1", Version: 1}); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if !repo.requests[deleting.ID].IsDelete {
		t.Error("request not deleted")
	}
}
