package workspace

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"github.com/tetiva-app/client/internal/domain"
	"github.com/tetiva-app/client/internal/domain/entities"
)

// mockRepo implements workspace.Repository in-memory for testing.
type mockRepo struct {
	workspaces map[uuid.UUID]*entities.Workspace
}

func newMockRepo() *mockRepo {
	return &mockRepo{workspaces: make(map[uuid.UUID]*entities.Workspace)}
}

func (m *mockRepo) Create(_ context.Context, w *entities.Workspace) error {
	m.workspaces[w.ID] = w
	return nil
}

func (m *mockRepo) GetByID(_ context.Context, id uuid.UUID) (*entities.Workspace, error) {
	w, ok := m.workspaces[id]
	if !ok {
		return nil, nil
	}
	cp := *w
	return &cp, nil
}

func (m *mockRepo) List(_ context.Context) ([]*entities.Workspace, error) {
	var result []*entities.Workspace
	for _, w := range m.workspaces {
		if !w.IsDelete {
			cp := *w
			result = append(result, &cp)
		}
	}
	return result, nil
}

func (m *mockRepo) Update(_ context.Context, w *entities.Workspace) error {
	m.workspaces[w.ID] = w
	return nil
}

func (m *mockRepo) GetActive(_ context.Context) (*entities.Workspace, error) {
	for _, w := range m.workspaces {
		if w.IsActive && !w.IsDelete {
			cp := *w
			return &cp, nil
		}
	}
	return nil, nil
}

func (m *mockRepo) SetActive(_ context.Context, id uuid.UUID) error {
	for k, w := range m.workspaces {
		w.IsActive = k == id
	}
	return nil
}

func (m *mockRepo) CountNonDeleted(_ context.Context) (int, error) {
	count := 0
	for _, w := range m.workspaces {
		if !w.IsDelete {
			count++
		}
	}
	return count, nil
}

func (m *mockRepo) GetByRemoteID(_ context.Context, remoteID string) (*entities.Workspace, error) {
	for _, w := range m.workspaces {
		if w.RemoteWorkspaceID != nil && *w.RemoteWorkspaceID == remoteID && !w.IsDelete {
			cp := *w
			return &cp, nil
		}
	}
	return nil, nil
}

func newTestUsecase() (Usecase, *mockRepo) {
	repo := newMockRepo()
	uc := NewUsecase(repo)
	return uc, repo
}

func TestCreate_Success(t *testing.T) {
	uc, _ := newTestUsecase()
	ctx := context.Background()

	result, err := uc.Create(ctx, Create{Name: "My Workspace"}, CreateOpt{UserID: "user-1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil workspace")
	}
	if result.Name != "My Workspace" {
		t.Errorf("Name = %q, want %q", result.Name, "My Workspace")
	}
	if result.Version != 1 {
		t.Errorf("Version = %d, want 1", result.Version)
	}
	if result.CreatedBy != "user-1" {
		t.Errorf("CreatedBy = %q, want %q", result.CreatedBy, "user-1")
	}
}

func TestCreate_EmptyName(t *testing.T) {
	uc, _ := newTestUsecase()
	ctx := context.Background()

	_, err := uc.Create(ctx, Create{Name: ""}, CreateOpt{UserID: "user-1"})
	if err == nil {
		t.Fatal("expected error for empty name")
	}
	var valErr *domain.ValidationError
	if !errorAs(err, &valErr) {
		t.Fatalf("expected ValidationError, got %T", err)
	}
	if valErr.Fields["name"] != "required" {
		t.Errorf("Fields[name] = %q, want %q", valErr.Fields["name"], "required")
	}
}

func TestCreate_NameTooLong(t *testing.T) {
	uc, _ := newTestUsecase()
	ctx := context.Background()

	longName := make([]byte, 101)
	for i := range longName {
		longName[i] = 'a'
	}

	_, err := uc.Create(ctx, Create{Name: string(longName)}, CreateOpt{UserID: "user-1"})
	if err == nil {
		t.Fatal("expected error for long name")
	}
	var valErr *domain.ValidationError
	if !errorAs(err, &valErr) {
		t.Fatalf("expected ValidationError, got %T", err)
	}
}

func TestCreate_NotActive(t *testing.T) {
	uc, _ := newTestUsecase()
	ctx := context.Background()

	result, err := uc.Create(ctx, Create{Name: "New WS"}, CreateOpt{UserID: "user-1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsActive {
		t.Error("new workspace should not be active")
	}
}

func TestEdit_Success(t *testing.T) {
	uc, _ := newTestUsecase()
	ctx := context.Background()

	created, _ := uc.Create(ctx, Create{Name: "Original"}, CreateOpt{UserID: "user-1"})

	edited, err := uc.Edit(ctx, Edit{Name: "Renamed"}, EditOpt{
		WorkspaceID: created.ID,
		UserID:      "user-1",
		Version:     1,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if edited.Name != "Renamed" {
		t.Errorf("Name = %q, want %q", edited.Name, "Renamed")
	}
	if edited.Version != 2 {
		t.Errorf("Version = %d, want 2", edited.Version)
	}
}

func TestEdit_VersionMismatch(t *testing.T) {
	uc, _ := newTestUsecase()
	ctx := context.Background()

	created, _ := uc.Create(ctx, Create{Name: "Original"}, CreateOpt{UserID: "user-1"})

	_, err := uc.Edit(ctx, Edit{Name: "Renamed"}, EditOpt{
		WorkspaceID: created.ID,
		UserID:      "user-1",
		Version:     999,
	})
	if err == nil {
		t.Fatal("expected error for version mismatch")
	}
	var conflictErr *domain.ConflictError
	if !errorAs(err, &conflictErr) {
		t.Fatalf("expected ConflictError, got %T", err)
	}
}

func TestEdit_NotFound(t *testing.T) {
	uc, _ := newTestUsecase()
	ctx := context.Background()

	_, err := uc.Edit(ctx, Edit{Name: "Renamed"}, EditOpt{
		WorkspaceID: uuid.New(),
		UserID:      "user-1",
		Version:     1,
	})
	if err == nil {
		t.Fatal("expected error for not found")
	}
	var notFoundErr *domain.NotFoundError
	if !errorAs(err, &notFoundErr) {
		t.Fatalf("expected NotFoundError, got %T", err)
	}
}

func TestDelete_Success(t *testing.T) {
	uc, _ := newTestUsecase()
	ctx := context.Background()

	w1, err := uc.Create(ctx, Create{Name: "WS 1"}, CreateOpt{UserID: "user-1"})
	if err != nil {
		t.Fatalf("Create WS 1: %v", err)
	}
	if _, err := uc.Create(ctx, Create{Name: "WS 2"}, CreateOpt{UserID: "user-1"}); err != nil {
		t.Fatalf("Create WS 2: %v", err)
	}

	err = uc.Delete(ctx, DeleteOpt{
		WorkspaceID: w1.ID,
		UserID:      "user-1",
		Version:     1,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	list, _ := uc.List(ctx)
	if len(list) != 1 {
		t.Errorf("List len = %d, want 1", len(list))
	}
}

func TestDelete_LastWorkspace(t *testing.T) {
	uc, _ := newTestUsecase()
	ctx := context.Background()

	w, _ := uc.Create(ctx, Create{Name: "Only WS"}, CreateOpt{UserID: "user-1"})

	err := uc.Delete(ctx, DeleteOpt{
		WorkspaceID: w.ID,
		UserID:      "user-1",
		Version:     1,
	})
	if err == nil {
		t.Fatal("expected error when deleting last workspace")
	}
	var valErr *domain.ValidationError
	if !errorAs(err, &valErr) {
		t.Fatalf("expected ValidationError, got %T", err)
	}
	if valErr.Fields["workspace"] != "cannot delete the last workspace" {
		t.Errorf("unexpected validation message: %q", valErr.Fields["workspace"])
	}
}

func TestDelete_ActiveWorkspace_AutoActivatesAnother(t *testing.T) {
	uc, repo := newTestUsecase()
	ctx := context.Background()

	w1, _ := uc.Create(ctx, Create{Name: "Active WS"}, CreateOpt{UserID: "user-1"})
	w2, _ := uc.Create(ctx, Create{Name: "Other WS"}, CreateOpt{UserID: "user-1"})

	_ = repo.SetActive(ctx, w1.ID)

	err := uc.Delete(ctx, DeleteOpt{
		WorkspaceID: w1.ID,
		UserID:      "user-1",
		Version:     1,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	active, err := uc.GetActive(ctx)
	if err != nil {
		t.Fatalf("GetActive: %v", err)
	}
	if active.ID != w2.ID {
		t.Errorf("Active ID = %s, want %s", active.ID, w2.ID)
	}
}

func TestList_ReturnsNonDeleted(t *testing.T) {
	uc, _ := newTestUsecase()
	ctx := context.Background()

	if _, err := uc.Create(ctx, Create{Name: "WS 1"}, CreateOpt{UserID: "user-1"}); err != nil {
		t.Fatalf("Create WS 1: %v", err)
	}
	if _, err := uc.Create(ctx, Create{Name: "WS 2"}, CreateOpt{UserID: "user-1"}); err != nil {
		t.Fatalf("Create WS 2: %v", err)
	}
	w3, err := uc.Create(ctx, Create{Name: "WS 3"}, CreateOpt{UserID: "user-1"})
	if err != nil {
		t.Fatalf("Create WS 3: %v", err)
	}

	if err := uc.Delete(ctx, DeleteOpt{WorkspaceID: w3.ID, UserID: "user-1", Version: 1}); err != nil {
		t.Fatalf("Delete WS 3: %v", err)
	}

	list, err := uc.List(ctx)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(list) != 2 {
		t.Errorf("List len = %d, want 2", len(list))
	}
}

func TestSetActive_Success(t *testing.T) {
	uc, _ := newTestUsecase()
	ctx := context.Background()

	w, _ := uc.Create(ctx, Create{Name: "WS"}, CreateOpt{UserID: "user-1"})

	err := uc.SetActive(ctx, SetActiveOpt{WorkspaceID: w.ID})
	if err != nil {
		t.Fatalf("SetActive: %v", err)
	}

	active, err := uc.GetActive(ctx)
	if err != nil {
		t.Fatalf("GetActive: %v", err)
	}
	if active.ID != w.ID {
		t.Errorf("Active ID = %s, want %s", active.ID, w.ID)
	}
}

func TestSetActive_NotFound(t *testing.T) {
	uc, _ := newTestUsecase()
	ctx := context.Background()

	err := uc.SetActive(ctx, SetActiveOpt{WorkspaceID: uuid.New()})
	if err == nil {
		t.Fatal("expected error for not found")
	}
	var notFoundErr *domain.NotFoundError
	if !errorAs(err, &notFoundErr) {
		t.Fatalf("expected NotFoundError, got %T", err)
	}
}

func TestGetActive_ReturnsActive(t *testing.T) {
	uc, _ := newTestUsecase()
	ctx := context.Background()

	w1, err := uc.Create(ctx, Create{Name: "WS 1"}, CreateOpt{UserID: "user-1"})
	if err != nil {
		t.Fatalf("Create WS 1: %v", err)
	}
	if _, err := uc.Create(ctx, Create{Name: "WS 2"}, CreateOpt{UserID: "user-1"}); err != nil {
		t.Fatalf("Create WS 2: %v", err)
	}

	if err := uc.SetActive(ctx, SetActiveOpt{WorkspaceID: w1.ID}); err != nil {
		t.Fatalf("SetActive: %v", err)
	}

	active, err := uc.GetActive(ctx)
	if err != nil {
		t.Fatalf("GetActive: %v", err)
	}
	if active.ID != w1.ID {
		t.Errorf("Active ID = %s, want %s", active.ID, w1.ID)
	}
	if !active.IsActive {
		t.Error("expected IsActive = true")
	}
}

func errorAs[T error](err error, target *T) bool {
	return err != nil && func() bool {
		for {
			if e, ok := err.(T); ok {
				*target = e
				return true
			}
			u, ok := err.(interface{ Unwrap() error })
			if !ok {
				return false
			}
			err = u.Unwrap()
		}
	}()
}
