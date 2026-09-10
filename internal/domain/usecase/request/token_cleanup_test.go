package request_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/tetiva-app/client/internal/domain/entities"
	"github.com/tetiva-app/client/internal/domain/usecase/request"
)

type recordingCleaner struct {
	cleared     []entities.AuthOwner
	clearedKind string
	clearedIDs  []uuid.UUID
	keptHash    string
	sweeps      int
	sweepErr    error
}

func (c *recordingCleaner) Clear(_ context.Context, owner entities.AuthOwner) error {
	c.cleared = append(c.cleared, owner)
	return nil
}

func (c *recordingCleaner) ClearOwners(_ context.Context, kind string, ids []uuid.UUID) error {
	c.clearedKind = kind
	c.clearedIDs = append(c.clearedIDs, ids...)
	return nil
}

func (c *recordingCleaner) ClearOwnersUnlessHash(_ context.Context, kind string, ids []uuid.UUID, keepHash string) error {
	c.clearedKind = kind
	c.clearedIDs = append(c.clearedIDs, ids...)
	c.keptHash = keepHash
	return nil
}

func (c *recordingCleaner) DeleteOrphans(_ context.Context) (int, error) {
	c.sweeps++
	return 0, c.sweepErr
}

func newCleanupUsecase(repo *mockRepo, cleaner request.TokenCleaner) request.Usecase {
	return request.NewUsecase(repo, &mockHistoryRepo{}, &mockRequester{}, nil, nil,
		&mockEnvResolver{}, &noopScriptEngine{}, &noopScriptResolver{}, &noopVarPersister{},
		request.NewAuthResolver(fixtureCollections()), nil, fixtureCollections(), cleaner, nil)
}

func seedRequest(repo *mockRepo) *entities.Request {
	r := &entities.Request{
		ID:           uuid.New(),
		CollectionID: testCollectionID,
		Name:         "R",
		Protocol:     entities.ProtocolHTTP,
		Method:       entities.MethodGET,
		URL:          "https://example.com",
		AuthType:     entities.AuthTypeOAuth2,
		AuthData:     `{"grant":"client_credentials"}`,
		Version:      1,
	}
	repo.requests[r.ID] = r
	return r
}

func TestDelete_ClearsStoredToken(t *testing.T) {
	repo := newMockRepo()
	cleaner := &recordingCleaner{}
	uc := newCleanupUsecase(repo, cleaner)
	req := seedRequest(repo)

	if err := uc.Delete(context.Background(), request.DeleteOpt{RequestID: req.ID, Version: 1}); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	if cleaner.clearedKind != entities.AuthOwnerKindRequest {
		t.Errorf("cleared kind = %q, want %q", cleaner.clearedKind, entities.AuthOwnerKindRequest)
	}
	if len(cleaner.clearedIDs) != 1 || cleaner.clearedIDs[0] != req.ID {
		t.Errorf("cleared ids = %v, want [%s]", cleaner.clearedIDs, req.ID)
	}
}

func TestMove_SweepsTokens(t *testing.T) {
	repo := newMockRepo()
	cleaner := &recordingCleaner{}
	uc := newCleanupUsecase(repo, cleaner)
	req := seedRequest(repo)

	_, err := uc.Move(context.Background(), request.MoveOpt{
		RequestID:          req.ID,
		TargetCollectionID: uuid.New(),
		Version:            1,
	})
	if err != nil {
		t.Fatalf("Move: %v", err)
	}

	if cleaner.sweeps != 1 {
		t.Errorf("sweeps = %d, want 1", cleaner.sweeps)
	}
}

func TestDeleteDraft_ClearsBeforeHardDelete(t *testing.T) {
	repo := newMockRepo()
	cleaner := &recordingCleaner{}
	uc := newCleanupUsecase(repo, cleaner)
	draft := seedRequest(repo)
	draft.IsDraft = true

	if err := uc.DeleteDraft(context.Background(), draft.ID); err != nil {
		t.Fatalf("DeleteDraft: %v", err)
	}

	if len(cleaner.clearedIDs) != 1 || cleaner.clearedIDs[0] != draft.ID {
		t.Errorf("cleared ids = %v, want [%s]", cleaner.clearedIDs, draft.ID)
	}
	if cleaner.sweeps != 1 {
		t.Errorf("sweeps = %d, want 1", cleaner.sweeps)
	}
	if !repo.hardDeleted(draft.ID) {
		t.Error("draft was not hard-deleted")
	}
}

func TestPromoteDraft_SweepsTokens(t *testing.T) {
	repo := newMockRepo()
	cleaner := &recordingCleaner{}
	uc := newCleanupUsecase(repo, cleaner)
	draft := seedRequest(repo)
	draft.IsDraft = true

	_, err := uc.PromoteDraft(context.Background(), request.PromoteDraftOpt{
		DraftID:            draft.ID,
		Name:               "Saved",
		TargetCollectionID: testCollectionID,
		Version:            1,
	})
	if err != nil {
		t.Fatalf("PromoteDraft: %v", err)
	}

	if cleaner.sweeps != 1 {
		t.Errorf("sweeps = %d, want 1", cleaner.sweeps)
	}
}

func TestCleanupDrafts_SweepsTokens(t *testing.T) {
	repo := newMockRepo()
	cleaner := &recordingCleaner{}
	uc := newCleanupUsecase(repo, cleaner)

	if _, err := uc.CleanupDrafts(context.Background()); err != nil {
		t.Fatalf("CleanupDrafts: %v", err)
	}

	if cleaner.sweeps != 1 {
		t.Errorf("sweeps = %d, want 1", cleaner.sweeps)
	}
}

func TestCommittedWritesSurviveAFailingSweep(t *testing.T) {
	// The row is already gone or moved when the sweep runs; reporting its failure
	// would tell the user an operation that did happen did not.
	sweepErr := errors.New("database is locked")

	t.Run("DeleteDraft", func(t *testing.T) {
		repo := newMockRepo()
		cleaner := &recordingCleaner{sweepErr: sweepErr}
		uc := newCleanupUsecase(repo, cleaner)
		draft := seedRequest(repo)
		draft.IsDraft = true

		if err := uc.DeleteDraft(context.Background(), draft.ID); err != nil {
			t.Fatalf("DeleteDraft: %v", err)
		}
		if !repo.hardDeleted(draft.ID) {
			t.Error("draft was not hard-deleted")
		}
	})

	t.Run("PromoteDraft", func(t *testing.T) {
		repo := newMockRepo()
		cleaner := &recordingCleaner{sweepErr: sweepErr}
		uc := newCleanupUsecase(repo, cleaner)
		draft := seedRequest(repo)
		draft.IsDraft = true

		promoted, err := uc.PromoteDraft(context.Background(), request.PromoteDraftOpt{
			DraftID:            draft.ID,
			Name:               "Saved",
			TargetCollectionID: testCollectionID,
			Version:            1,
		})
		if err != nil {
			t.Fatalf("PromoteDraft: %v", err)
		}
		if promoted.IsDraft {
			t.Error("the draft was not promoted")
		}
	})

	t.Run("CleanupDrafts", func(t *testing.T) {
		repo := newMockRepo()
		uc := newCleanupUsecase(repo, &recordingCleaner{sweepErr: sweepErr})

		if _, err := uc.CleanupDrafts(context.Background()); err != nil {
			t.Fatalf("CleanupDrafts: %v", err)
		}
	})

	t.Run("Move", func(t *testing.T) {
		repo := newMockRepo()
		uc := newCleanupUsecase(repo, &recordingCleaner{sweepErr: sweepErr})
		moved := seedRequest(repo)

		if _, err := uc.Move(context.Background(), request.MoveOpt{
			RequestID:          moved.ID,
			TargetCollectionID: testCollectionID,
			Version:            1,
		}); err != nil {
			t.Fatalf("Move: %v", err)
		}
	})
}
