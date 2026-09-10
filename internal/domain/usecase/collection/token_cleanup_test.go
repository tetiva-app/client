package collection_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/tetiva-app/client/internal/domain/entities"
	"github.com/tetiva-app/client/internal/domain/usecase/auth"
	"github.com/tetiva-app/client/internal/domain/usecase/collection"
)

type recordingCleaner struct {
	cleared    []entities.AuthOwner
	clearedIDs []uuid.UUID
	keptHash   string
	sweeps     int
	sweepErr   error
}

func (c *recordingCleaner) Clear(_ context.Context, owner entities.AuthOwner) error {
	c.cleared = append(c.cleared, owner)
	return nil
}

func (c *recordingCleaner) ClearOwnersUnlessHash(_ context.Context, _ string, ids []uuid.UUID, keepHash string) error {
	c.clearedIDs = append(c.clearedIDs, ids...)
	c.keptHash = keepHash
	return nil
}

func (c *recordingCleaner) DeleteOrphans(_ context.Context) (int, error) {
	c.sweeps++
	return 0, c.sweepErr
}

func TestDelete_SurvivesAFailingSweep(t *testing.T) {
	// The collection is already soft-deleted when the sweep runs; reporting its
	// failure would tell the user a delete that did happen did not.
	repo := newMockRepo()
	uc := collection.NewUsecase(repo, &recordingCleaner{sweepErr: errors.New("database is locked")})
	ctx := context.Background()

	col := &entities.Collection{ID: uuid.New(), WorkspaceID: testWorkspaceID, Name: "Doomed", Version: 1}
	if err := repo.Create(ctx, col); err != nil {
		t.Fatalf("create: %v", err)
	}

	if err := uc.Delete(ctx, collection.DeleteOpt{CollectionID: col.ID, Version: 1}); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	stored, err := repo.GetByID(ctx, col.ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if stored != nil && !stored.IsDelete {
		t.Error("the collection must stay deleted")
	}
}

func TestDelete_ClearsTokenAndSweeps(t *testing.T) {
	repo := newMockRepo()
	cleaner := &recordingCleaner{}
	uc := collection.NewUsecase(repo, cleaner)
	ctx := context.Background()

	parent := &entities.Collection{
		ID:          uuid.New(),
		WorkspaceID: testWorkspaceID,
		Name:        "Parent",
		AuthType:    entities.AuthTypeOAuth2,
		AuthData:    `{"grant":"client_credentials"}`,
		Version:     1,
	}
	if err := repo.Create(ctx, parent); err != nil {
		t.Fatalf("create: %v", err)
	}
	child := &entities.Collection{
		ID:          uuid.New(),
		WorkspaceID: testWorkspaceID,
		ParentID:    &parent.ID,
		Name:        "Child",
		Version:     1,
	}
	if err := repo.Create(ctx, child); err != nil {
		t.Fatalf("create child: %v", err)
	}

	if err := uc.Delete(ctx, collection.DeleteOpt{CollectionID: parent.ID, Version: 1}); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	want := entities.AuthOwner{
		WorkspaceID: testWorkspaceID,
		Kind:        entities.AuthOwnerKindCollection,
		ID:          parent.ID,
	}
	if len(cleaner.cleared) != 1 || cleaner.cleared[0] != want {
		t.Errorf("cleared = %v, want [%v]", cleaner.cleared, want)
	}
	// Descendants and their requests are only reachable through the sweep.
	if cleaner.sweeps != 1 {
		t.Errorf("sweeps = %d, want 1", cleaner.sweeps)
	}
}

func TestEdit_ClearsTokenOnlyWhenTheAcquisitionConfigChanges(t *testing.T) {
	repo := newMockRepo()
	cleaner := &recordingCleaner{}
	uc := collection.NewUsecase(repo, cleaner)
	ctx := context.Background()

	const authData = `{"grant":"client_credentials","tokenUrl":"https://idp.example/token","clientId":"cid","scope":"read"}`
	coll := &entities.Collection{
		ID:          uuid.New(),
		WorkspaceID: testWorkspaceID,
		Name:        "C",
		AuthType:    entities.AuthTypeOAuth2,
		AuthData:    authData,
		Version:     1,
	}
	if err := repo.Create(ctx, coll); err != nil {
		t.Fatalf("create: %v", err)
	}

	if _, err := uc.Edit(ctx, collection.Edit{
		Name: "Renamed", AuthType: entities.AuthTypeOAuth2, AuthData: authData,
	}, collection.EditOpt{CollectionID: coll.ID, Version: 1}); err != nil {
		t.Fatalf("Edit: %v", err)
	}
	if len(cleaner.clearedIDs) != 0 {
		t.Fatalf("cleared = %v, want none for an unchanged configuration", cleaner.clearedIDs)
	}

	changed := `{"grant":"client_credentials","tokenUrl":"https://idp.example/token","clientId":"cid","scope":"write"}`
	if _, err := uc.Edit(ctx, collection.Edit{
		Name: "Renamed", AuthType: entities.AuthTypeOAuth2, AuthData: changed,
	}, collection.EditOpt{CollectionID: coll.ID, Version: 2}); err != nil {
		t.Fatalf("Edit: %v", err)
	}

	if len(cleaner.clearedIDs) != 1 || cleaner.clearedIDs[0] != coll.ID {
		t.Errorf("cleared = %v, want [%v]", cleaner.clearedIDs, coll.ID)
	}
	// A token the just-saved configuration produced is kept, not re-acquired.
	if want := auth.AcquisitionHash(entities.AuthTypeOAuth2, changed); cleaner.keptHash != want {
		t.Errorf("kept hash = %q, want %q", cleaner.keptHash, want)
	}
}
