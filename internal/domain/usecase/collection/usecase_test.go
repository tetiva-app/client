package collection_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/tetiva-app/client/internal/domain"
	"github.com/tetiva-app/client/internal/domain/entities"
	"github.com/tetiva-app/client/internal/domain/usecase/collection"
)

var testWorkspaceID = uuid.MustParse("00000000-0000-4000-a000-000000000001")

// mockRepo is an in-memory implementation of collection.Repository for testing.
type mockRepo struct {
	collections map[uuid.UUID]*entities.Collection
}

func newMockRepo() *mockRepo {
	return &mockRepo{
		collections: make(map[uuid.UUID]*entities.Collection),
	}
}

func (m *mockRepo) Create(_ context.Context, c *entities.Collection) error {
	m.collections[c.ID] = c
	return nil
}

func (m *mockRepo) GetByID(_ context.Context, id uuid.UUID) (*entities.Collection, error) {
	c, ok := m.collections[id]
	if !ok {
		return nil, nil
	}
	// Return a copy to avoid shared pointer mutations.
	cp := *c
	return &cp, nil
}

func (m *mockRepo) List(_ context.Context, filter collection.Filter) ([]*entities.Collection, error) {
	var result []*entities.Collection
	for _, c := range m.collections {
		if c.WorkspaceID != filter.WorkspaceID {
			continue
		}
		if c.IsDelete {
			continue
		}
		if filter.ParentID != nil {
			if c.ParentID == nil || *c.ParentID != *filter.ParentID {
				continue
			}
		}
		cp := *c
		result = append(result, &cp)
	}
	return result, nil
}

func (m *mockRepo) Update(_ context.Context, c *entities.Collection) error {
	m.collections[c.ID] = c
	return nil
}

func (m *mockRepo) UpdateSortOrder(_ context.Context, id uuid.UUID, sortOrder int) error {
	c, ok := m.collections[id]
	if !ok {
		return nil
	}
	c.SortOrder = sortOrder
	return nil
}

func (m *mockRepo) SoftDeleteDescendants(_ context.Context, parentID uuid.UUID, updatedBy string, updatedAt time.Time) error {
	var findDescendants func(pid uuid.UUID) []uuid.UUID
	findDescendants = func(pid uuid.UUID) []uuid.UUID {
		var ids []uuid.UUID
		for _, c := range m.collections {
			if c.ParentID != nil && *c.ParentID == pid && !c.IsDelete {
				ids = append(ids, c.ID)
				ids = append(ids, findDescendants(c.ID)...)
			}
		}
		return ids
	}

	for _, id := range findDescendants(parentID) {
		c := m.collections[id]
		c.IsDelete = true
		c.Version++
		c.UpdatedBy = updatedBy
		c.UpdatedAt = updatedAt
	}
	return nil
}

func newTestUsecase() (collection.Usecase, *mockRepo) {
	repo := newMockRepo()
	uc := collection.NewUsecase(repo)
	return uc, repo
}

func TestCreate(t *testing.T) {
	uc, _ := newTestUsecase()
	ctx := context.Background()

	input := collection.Create{Name: "My Collection"}
	opt := collection.CreateOpt{
		UserID:      "user-1",
		WorkspaceID: testWorkspaceID,
	}

	result, err := uc.Create(ctx, input, opt)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil collection")
	}
	if result.Name != "My Collection" {
		t.Errorf("expected name %q, got %q", "My Collection", result.Name)
	}
	if result.WorkspaceID != testWorkspaceID {
		t.Errorf("expected workspace ID %s, got %s", testWorkspaceID, result.WorkspaceID)
	}
	if result.Version != 1 {
		t.Errorf("expected version 1, got %d", result.Version)
	}
	if result.IsDelete {
		t.Error("expected IsDelete to be false")
	}
	if result.CreatedBy != "user-1" {
		t.Errorf("expected CreatedBy %q, got %q", "user-1", result.CreatedBy)
	}
	if result.UpdatedBy != "user-1" {
		t.Errorf("expected UpdatedBy %q, got %q", "user-1", result.UpdatedBy)
	}
	if result.ID == uuid.Nil {
		t.Error("expected non-nil UUID for ID")
	}
	if result.CreatedAt.IsZero() {
		t.Error("expected non-zero CreatedAt")
	}
	if result.UpdatedAt.IsZero() {
		t.Error("expected non-zero UpdatedAt")
	}
}

func TestCreate_EmptyName(t *testing.T) {
	uc, _ := newTestUsecase()
	ctx := context.Background()

	input := collection.Create{Name: ""}
	opt := collection.CreateOpt{
		UserID:      "user-1",
		WorkspaceID: testWorkspaceID,
	}

	result, err := uc.Create(ctx, input, opt)
	if result != nil {
		t.Errorf("expected nil result, got %+v", result)
	}
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	var valErr *domain.ValidationError
	if !errors.As(err, &valErr) {
		t.Fatalf("expected *domain.ValidationError, got %T: %v", err, err)
	}

	msg, ok := valErr.Fields["name"]
	if !ok {
		t.Fatal("expected validation error for field 'name'")
	}
	if msg != "required" {
		t.Errorf("expected field message %q, got %q", "required", msg)
	}
}

func TestEdit(t *testing.T) {
	uc, _ := newTestUsecase()
	ctx := context.Background()

	created, err := uc.Create(ctx, collection.Create{Name: "Original"}, collection.CreateOpt{
		UserID:      "user-1",
		WorkspaceID: testWorkspaceID,
	})
	if err != nil {
		t.Fatalf("setup: unexpected error: %v", err)
	}

	input := collection.Edit{Name: "Renamed"}
	opt := collection.EditOpt{
		CollectionID: created.ID,
		UserID:       "user-2",
		Version:      created.Version,
	}

	edited, err := uc.Edit(ctx, input, opt)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if edited.Name != "Renamed" {
		t.Errorf("expected name %q, got %q", "Renamed", edited.Name)
	}
	if edited.Version != created.Version+1 {
		t.Errorf("expected version %d, got %d", created.Version+1, edited.Version)
	}
	if edited.UpdatedBy != "user-2" {
		t.Errorf("expected UpdatedBy %q, got %q", "user-2", edited.UpdatedBy)
	}
}

func TestEdit_VersionConflict(t *testing.T) {
	uc, _ := newTestUsecase()
	ctx := context.Background()

	created, err := uc.Create(ctx, collection.Create{Name: "Original"}, collection.CreateOpt{
		UserID:      "user-1",
		WorkspaceID: testWorkspaceID,
	})
	if err != nil {
		t.Fatalf("setup: unexpected error: %v", err)
	}

	wrongVersion := created.Version + 99
	input := collection.Edit{Name: "Should Fail"}
	opt := collection.EditOpt{
		CollectionID: created.ID,
		UserID:       "user-2",
		Version:      wrongVersion,
	}

	result, err := uc.Edit(ctx, input, opt)
	if result != nil {
		t.Errorf("expected nil result, got %+v", result)
	}
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	var conflictErr *domain.ConflictError
	if !errors.As(err, &conflictErr) {
		t.Fatalf("expected *domain.ConflictError, got %T: %v", err, err)
	}
	if conflictErr.Entity != "collection" {
		t.Errorf("expected entity %q, got %q", "collection", conflictErr.Entity)
	}
}

func TestDelete(t *testing.T) {
	uc, _ := newTestUsecase()
	ctx := context.Background()

	created, err := uc.Create(ctx, collection.Create{Name: "To Delete"}, collection.CreateOpt{
		UserID:      "user-1",
		WorkspaceID: testWorkspaceID,
	})
	if err != nil {
		t.Fatalf("setup: unexpected error: %v", err)
	}

	err = uc.Delete(ctx, collection.DeleteOpt{
		CollectionID: created.ID,
		UserID:       "user-1",
		Version:      created.Version,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	list, err := uc.List(ctx, collection.ListOpt{WorkspaceID: testWorkspaceID})
	if err != nil {
		t.Fatalf("unexpected error listing: %v", err)
	}
	for _, c := range list {
		if c.ID == created.ID {
			t.Errorf("deleted collection %s should not appear in list", created.ID)
		}
	}
}

func TestDelete_WithChildren(t *testing.T) {
	uc, _ := newTestUsecase()
	ctx := context.Background()

	parent, err := uc.Create(ctx, collection.Create{Name: "Parent"}, collection.CreateOpt{
		UserID:      "user-1",
		WorkspaceID: testWorkspaceID,
	})
	if err != nil {
		t.Fatalf("setup parent: %v", err)
	}

	child, err := uc.Create(ctx, collection.Create{Name: "Child", ParentID: &parent.ID}, collection.CreateOpt{
		UserID:      "user-1",
		WorkspaceID: testWorkspaceID,
	})
	if err != nil {
		t.Fatalf("setup child: %v", err)
	}

	grandchild, err := uc.Create(ctx, collection.Create{Name: "Grandchild", ParentID: &child.ID}, collection.CreateOpt{
		UserID:      "user-1",
		WorkspaceID: testWorkspaceID,
	})
	if err != nil {
		t.Fatalf("setup grandchild: %v", err)
	}

	err = uc.Delete(ctx, collection.DeleteOpt{
		CollectionID: parent.ID,
		UserID:       "user-1",
		Version:      parent.Version,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	list, err := uc.List(ctx, collection.ListOpt{WorkspaceID: testWorkspaceID})
	if err != nil {
		t.Fatalf("unexpected error listing: %v", err)
	}
	if len(list) != 0 {
		names := make([]string, len(list))
		for i, c := range list {
			names[i] = c.Name
		}
		t.Errorf("expected 0 collections after cascading delete, got %d: %v", len(list), names)
	}

	_ = grandchild
}

func TestList(t *testing.T) {
	uc, _ := newTestUsecase()
	ctx := context.Background()

	_, err := uc.Create(ctx, collection.Create{Name: "Collection A"}, collection.CreateOpt{
		UserID:      "user-1",
		WorkspaceID: testWorkspaceID,
	})
	if err != nil {
		t.Fatalf("setup: unexpected error: %v", err)
	}

	_, err = uc.Create(ctx, collection.Create{Name: "Collection B"}, collection.CreateOpt{
		UserID:      "user-1",
		WorkspaceID: testWorkspaceID,
	})
	if err != nil {
		t.Fatalf("setup: unexpected error: %v", err)
	}

	list, err := uc.List(ctx, collection.ListOpt{WorkspaceID: testWorkspaceID})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("expected 2 collections, got %d", len(list))
	}

	names := map[string]bool{}
	for _, c := range list {
		names[c.Name] = true
	}
	if !names["Collection A"] {
		t.Error("expected 'Collection A' in list")
	}
	if !names["Collection B"] {
		t.Error("expected 'Collection B' in list")
	}
}

func TestMove(t *testing.T) {
	uc, _ := newTestUsecase()
	ctx := context.Background()

	source, err := uc.Create(ctx, collection.Create{Name: "Source"}, collection.CreateOpt{
		UserID: "user-1", WorkspaceID: testWorkspaceID,
	})
	if err != nil {
		t.Fatalf("setup source: %v", err)
	}

	target, err := uc.Create(ctx, collection.Create{Name: "Target"}, collection.CreateOpt{
		UserID: "user-1", WorkspaceID: testWorkspaceID,
	})
	if err != nil {
		t.Fatalf("setup target: %v", err)
	}

	moved, err := uc.Move(ctx, collection.MoveOpt{
		CollectionID:   source.ID,
		TargetParentID: &target.ID,
		UserID:         "user-1",
		Version:        source.Version,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if moved.ParentID == nil || *moved.ParentID != target.ID {
		t.Errorf("expected parentID %s, got %v", target.ID, moved.ParentID)
	}
	if moved.Version != source.Version+1 {
		t.Errorf("expected version %d, got %d", source.Version+1, moved.Version)
	}
}

func TestMove_ToRoot(t *testing.T) {
	uc, _ := newTestUsecase()
	ctx := context.Background()

	parent, err := uc.Create(ctx, collection.Create{Name: "Parent"}, collection.CreateOpt{
		UserID: "user-1", WorkspaceID: testWorkspaceID,
	})
	if err != nil {
		t.Fatalf("setup: %v", err)
	}

	child, err := uc.Create(ctx, collection.Create{Name: "Child", ParentID: &parent.ID}, collection.CreateOpt{
		UserID: "user-1", WorkspaceID: testWorkspaceID,
	})
	if err != nil {
		t.Fatalf("setup: %v", err)
	}

	moved, err := uc.Move(ctx, collection.MoveOpt{
		CollectionID:   child.ID,
		TargetParentID: nil,
		UserID:         "user-1",
		Version:        child.Version,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if moved.ParentID != nil {
		t.Errorf("expected nil parentID, got %v", moved.ParentID)
	}
}

func TestMove_CircularDependency(t *testing.T) {
	uc, _ := newTestUsecase()
	ctx := context.Background()

	parent, err := uc.Create(ctx, collection.Create{Name: "Parent"}, collection.CreateOpt{
		UserID: "user-1", WorkspaceID: testWorkspaceID,
	})
	if err != nil {
		t.Fatalf("setup: %v", err)
	}

	child, err := uc.Create(ctx, collection.Create{Name: "Child", ParentID: &parent.ID}, collection.CreateOpt{
		UserID: "user-1", WorkspaceID: testWorkspaceID,
	})
	if err != nil {
		t.Fatalf("setup: %v", err)
	}

	_, err = uc.Move(ctx, collection.MoveOpt{
		CollectionID:   parent.ID,
		TargetParentID: &child.ID,
		UserID:         "user-1",
		Version:        parent.Version,
	})
	if err == nil {
		t.Fatal("expected error for circular move, got nil")
	}

	var valErr *domain.ValidationError
	if !errors.As(err, &valErr) {
		t.Fatalf("expected *domain.ValidationError, got %T: %v", err, err)
	}
}

func TestMove_IntoSelf(t *testing.T) {
	uc, _ := newTestUsecase()
	ctx := context.Background()

	c, err := uc.Create(ctx, collection.Create{Name: "Self"}, collection.CreateOpt{
		UserID: "user-1", WorkspaceID: testWorkspaceID,
	})
	if err != nil {
		t.Fatalf("setup: %v", err)
	}

	_, err = uc.Move(ctx, collection.MoveOpt{
		CollectionID:   c.ID,
		TargetParentID: &c.ID,
		UserID:         "user-1",
		Version:        c.Version,
	})
	if err == nil {
		t.Fatal("expected error for self-move, got nil")
	}
}

func TestMove_VersionConflict(t *testing.T) {
	uc, _ := newTestUsecase()
	ctx := context.Background()

	source, err := uc.Create(ctx, collection.Create{Name: "Source"}, collection.CreateOpt{
		UserID: "user-1", WorkspaceID: testWorkspaceID,
	})
	if err != nil {
		t.Fatalf("setup: %v", err)
	}

	target, err := uc.Create(ctx, collection.Create{Name: "Target"}, collection.CreateOpt{
		UserID: "user-1", WorkspaceID: testWorkspaceID,
	})
	if err != nil {
		t.Fatalf("setup: %v", err)
	}

	_, err = uc.Move(ctx, collection.MoveOpt{
		CollectionID:   source.ID,
		TargetParentID: &target.ID,
		UserID:         "user-1",
		Version:        source.Version + 99,
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	var conflictErr *domain.ConflictError
	if !errors.As(err, &conflictErr) {
		t.Fatalf("expected *domain.ConflictError, got %T: %v", err, err)
	}
}

func TestCreate_WithDescriptionAndAuth(t *testing.T) {
	uc, _ := newTestUsecase()
	ctx := context.Background()

	input := collection.Create{
		Name:        "API Collection",
		Description: "# My API\nEndpoints for user management.",
		AuthType:    entities.AuthTypeBasic,
		AuthData:    `{"username":"admin","password":"secret"}`,
	}
	opt := collection.CreateOpt{UserID: "user-1", WorkspaceID: testWorkspaceID}

	result, err := uc.Create(ctx, input, opt)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Description != "# My API\nEndpoints for user management." {
		t.Errorf("expected description preserved, got %q", result.Description)
	}
	if result.AuthType != entities.AuthTypeBasic {
		t.Errorf("expected AuthType %q, got %q", entities.AuthTypeBasic, result.AuthType)
	}
	if result.AuthData != `{"username":"admin","password":"secret"}` {
		t.Errorf("expected AuthData preserved, got %q", result.AuthData)
	}
}

func TestCreate_DefaultsAuthFields(t *testing.T) {
	uc, _ := newTestUsecase()
	ctx := context.Background()

	input := collection.Create{Name: "No Auth"}
	opt := collection.CreateOpt{UserID: "user-1", WorkspaceID: testWorkspaceID}

	result, err := uc.Create(ctx, input, opt)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.AuthType != entities.AuthTypeNone {
		t.Errorf("expected default AuthType %q, got %q", entities.AuthTypeNone, result.AuthType)
	}
	if result.AuthData != "{}" {
		t.Errorf("expected default AuthData %q, got %q", "{}", result.AuthData)
	}
}

func TestCreate_RejectsInheritAuthType(t *testing.T) {
	uc, _ := newTestUsecase()
	ctx := context.Background()

	input := collection.Create{Name: "Bad Auth", AuthType: entities.AuthTypeInherit}
	opt := collection.CreateOpt{UserID: "user-1", WorkspaceID: testWorkspaceID}

	_, err := uc.Create(ctx, input, opt)
	if err == nil {
		t.Fatal("expected error for inherit auth type on collection")
	}
	var valErr *domain.ValidationError
	if !errors.As(err, &valErr) {
		t.Fatalf("expected *domain.ValidationError, got %T", err)
	}
	if _, ok := valErr.Fields["authType"]; !ok {
		t.Error("expected validation error on field 'authType'")
	}
}

func TestCreate_RejectsInvalidAuthData(t *testing.T) {
	uc, _ := newTestUsecase()
	ctx := context.Background()

	input := collection.Create{Name: "Bad JSON", AuthType: entities.AuthTypeBasic, AuthData: "not-json"}
	opt := collection.CreateOpt{UserID: "user-1", WorkspaceID: testWorkspaceID}

	_, err := uc.Create(ctx, input, opt)
	if err == nil {
		t.Fatal("expected error for invalid auth data JSON")
	}
	var valErr *domain.ValidationError
	if !errors.As(err, &valErr) {
		t.Fatalf("expected *domain.ValidationError, got %T", err)
	}
	if _, ok := valErr.Fields["authData"]; !ok {
		t.Error("expected validation error on field 'authData'")
	}
}

func TestEdit_WithDescriptionAndAuth(t *testing.T) {
	uc, _ := newTestUsecase()
	ctx := context.Background()

	created, err := uc.Create(ctx, collection.Create{Name: "Original"}, collection.CreateOpt{
		UserID: "user-1", WorkspaceID: testWorkspaceID,
	})
	if err != nil {
		t.Fatalf("setup: %v", err)
	}

	input := collection.Edit{
		Name:        "Updated",
		Description: "Updated description",
		AuthType:    entities.AuthTypeBearer,
		AuthData:    `{"token":"abc123"}`,
	}
	opt := collection.EditOpt{CollectionID: created.ID, UserID: "user-2", Version: created.Version}

	edited, err := uc.Edit(ctx, input, opt)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if edited.Description != "Updated description" {
		t.Errorf("expected description %q, got %q", "Updated description", edited.Description)
	}
	if edited.AuthType != entities.AuthTypeBearer {
		t.Errorf("expected AuthType %q, got %q", entities.AuthTypeBearer, edited.AuthType)
	}
	if edited.AuthData != `{"token":"abc123"}` {
		t.Errorf("expected AuthData preserved, got %q", edited.AuthData)
	}
}

func TestEdit_RejectsInheritAuthType(t *testing.T) {
	uc, _ := newTestUsecase()
	ctx := context.Background()

	created, err := uc.Create(ctx, collection.Create{Name: "Original"}, collection.CreateOpt{
		UserID: "user-1", WorkspaceID: testWorkspaceID,
	})
	if err != nil {
		t.Fatalf("setup: %v", err)
	}

	input := collection.Edit{Name: "Bad", AuthType: entities.AuthTypeInherit}
	opt := collection.EditOpt{CollectionID: created.ID, UserID: "user-2", Version: created.Version}

	_, err = uc.Edit(ctx, input, opt)
	if err == nil {
		t.Fatal("expected error for inherit auth type on collection")
	}
	var valErr *domain.ValidationError
	if !errors.As(err, &valErr) {
		t.Fatalf("expected *domain.ValidationError, got %T", err)
	}
}

func TestGetByID_NotFound(t *testing.T) {
	uc, _ := newTestUsecase()
	ctx := context.Background()

	nonExistentID := uuid.New()

	result, err := uc.GetByID(ctx, nonExistentID)
	if result != nil {
		t.Errorf("expected nil result, got %+v", result)
	}
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	var notFoundErr *domain.NotFoundError
	if !errors.As(err, &notFoundErr) {
		t.Fatalf("expected *domain.NotFoundError, got %T: %v", err, err)
	}
	if notFoundErr.Entity != "collection" {
		t.Errorf("expected entity %q, got %q", "collection", notFoundErr.Entity)
	}
	if notFoundErr.ID != nonExistentID.String() {
		t.Errorf("expected ID %q, got %q", nonExistentID.String(), notFoundErr.ID)
	}
}
