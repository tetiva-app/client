package sqlite

import (
	"testing"

	"github.com/google/uuid"
)

func TestGetByIDIncludingDeleted_ReturnsSoftDeletedRows(t *testing.T) {
	db := setupTestDB(t)
	ctx := testCtx(t)

	collections := NewCollectionRepo(db).(*CollectionRepo)
	requests := NewRequestRepo(db).(*RequestRepo)
	environments := NewEnvironmentRepo(db).(*EnvironmentRepo)
	variables := NewVariableRepo(db).(*VariableRepo)
	examples := NewResponseExampleRepo(db)

	coll := newTestCollection("Gone", nil)
	req := newTestRequest("Gone", coll.ID)
	env := newTestEnvironment("Gone")
	variable := newTestVariable(env.ID, "token", "abc")
	ex := newTestExample(req.ID, "Gone", 0)
	for _, err := range []error{
		collections.Create(ctx, coll), requests.Create(ctx, req), environments.Create(ctx, env),
		variables.Create(ctx, variable), examples.Create(ctx, ex),
	} {
		if err != nil {
			t.Fatalf("seed: %v", err)
		}
	}
	coll.IsDelete, req.IsDelete, env.IsDelete, variable.IsDelete, ex.IsDelete = true, true, true, true, true
	for _, err := range []error{
		collections.Update(ctx, coll), requests.Update(ctx, req), environments.Update(ctx, env),
		variables.Update(ctx, variable), examples.Update(ctx, ex),
	} {
		if err != nil {
			t.Fatalf("soft delete: %v", err)
		}
	}

	if got, err := collections.GetByIDIncludingDeleted(ctx, coll.ID); err != nil || got == nil || !got.IsDelete || got.Name != "Gone" {
		t.Errorf("collection: %+v, %v", got, err)
	}
	if got, err := requests.GetByIDIncludingDeleted(ctx, req.ID); err != nil || got == nil || !got.IsDelete || got.CollectionID != coll.ID {
		t.Errorf("request: %+v, %v", got, err)
	}
	if got, err := environments.GetByIDIncludingDeleted(ctx, env.ID); err != nil || got == nil || !got.IsDelete {
		t.Errorf("environment: %+v, %v", got, err)
	}
	if got, err := variables.GetByIDIncludingDeleted(ctx, variable.ID); err != nil || got == nil || !got.IsDelete || got.EnvironmentID != env.ID {
		t.Errorf("variable: %+v, %v", got, err)
	}
	if got, err := examples.GetByIDIncludingDeleted(ctx, ex.ID); err != nil || got == nil || !got.IsDelete || got.RequestID != req.ID {
		t.Errorf("example: %+v, %v", got, err)
	}

	missing := uuid.New()
	if got, err := collections.GetByIDIncludingDeleted(ctx, missing); err != nil || got != nil {
		t.Errorf("missing collection: %+v, %v", got, err)
	}
	if got, err := examples.GetByIDIncludingDeleted(ctx, missing); err != nil || got != nil {
		t.Errorf("missing example: %+v, %v", got, err)
	}
}
