package cookie

import (
	"context"
	"net/url"
	"testing"

	"github.com/google/uuid"

	"github.com/tetiva-app/client/internal/domain/entities"
)

type memRepo struct {
	store map[uuid.UUID]*entities.Cookie
}

func newMemRepo() *memRepo { return &memRepo{store: map[uuid.UUID]*entities.Cookie{}} }

func (m *memRepo) Upsert(_ context.Context, c *entities.Cookie) error {
	if c.ID == uuid.Nil {
		c.ID = uuid.New()
	}
	m.store[c.ID] = c
	return nil
}
func (m *memRepo) GetByID(_ context.Context, id uuid.UUID) (*entities.Cookie, error) {
	return m.store[id], nil
}
func (m *memRepo) List(_ context.Context, f Filter) ([]*entities.Cookie, error) {
	out := []*entities.Cookie{}
	for _, c := range m.store {
		if f.WorkspaceID != uuid.Nil && c.WorkspaceID != f.WorkspaceID {
			continue
		}
		if f.Domain != "" && c.Domain != f.Domain {
			continue
		}
		out = append(out, c)
	}
	return out, nil
}
func (m *memRepo) Delete(_ context.Context, id uuid.UUID) error { delete(m.store, id); return nil }
func (m *memRepo) DeleteByDomain(_ context.Context, ws uuid.UUID, d string) (int, error) {
	n := 0
	for id, c := range m.store {
		if c.WorkspaceID == ws && c.Domain == d {
			delete(m.store, id)
			n++
		}
	}
	return n, nil
}
func (m *memRepo) Clear(_ context.Context, ws uuid.UUID) (int, error) {
	n := 0
	for id, c := range m.store {
		if c.WorkspaceID == ws {
			delete(m.store, id)
			n++
		}
	}
	return n, nil
}
func (m *memRepo) MatchForRequest(_ context.Context, _ uuid.UUID, _ *url.URL) ([]*entities.Cookie, error) {
	return nil, nil
}

func TestAdd_RejectsEmptyName(t *testing.T) {
	uc := NewUsecase(newMemRepo())
	_, err := uc.Add(context.Background(), Add{
		WorkspaceID: uuid.New(),
		Name:        "",
		Value:       "v",
		Domain:      "example.com",
	}, Opt{})
	if err == nil {
		t.Fatal("expected validation error for empty name")
	}
}

func TestListAndDelete(t *testing.T) {
	repo := newMemRepo()
	uc := NewUsecase(repo)
	ws := uuid.New()

	c, _ := uc.Add(context.Background(), Add{
		WorkspaceID: ws, Name: "a", Value: "1", Domain: "example.com",
	}, Opt{})
	_, _ = uc.Add(context.Background(), Add{
		WorkspaceID: ws, Name: "b", Value: "2", Domain: "other.com",
	}, Opt{})

	all, _ := uc.List(context.Background(), ListOpt{WorkspaceID: ws})
	if len(all) != 2 {
		t.Fatalf("expected 2, got %d", len(all))
	}

	filtered, _ := uc.List(context.Background(), ListOpt{WorkspaceID: ws, Domain: "example.com"})
	if len(filtered) != 1 {
		t.Fatalf("expected 1, got %d", len(filtered))
	}

	if err := uc.Delete(context.Background(), DeleteOpt{ID: c.ID}); err != nil {
		t.Fatal(err)
	}
	all, _ = uc.List(context.Background(), ListOpt{WorkspaceID: ws})
	if len(all) != 1 {
		t.Fatalf("expected 1 after delete, got %d", len(all))
	}
}

func TestClearAndDeleteByDomain(t *testing.T) {
	repo := newMemRepo()
	uc := NewUsecase(repo)
	ws := uuid.New()

	for _, d := range []string{"a.com", "a.com", "b.com"} {
		_, _ = uc.Add(context.Background(), Add{
			WorkspaceID: ws, Name: uuid.NewString(), Value: "v", Domain: d,
		}, Opt{})
	}

	n, _ := uc.DeleteByDomain(context.Background(), ws, "a.com")
	if n != 2 {
		t.Fatalf("expected 2 deleted, got %d", n)
	}

	n, _ = uc.Clear(context.Background(), ws)
	if n != 1 {
		t.Fatalf("expected 1 cleared, got %d", n)
	}
}

func TestAdd_PersistsCookie(t *testing.T) {
	repo := newMemRepo()
	uc := NewUsecase(repo)
	ws := uuid.New()
	got, err := uc.Add(context.Background(), Add{
		WorkspaceID: ws,
		Name:        "session", Value: "abc",
		Domain: "example.com", Path: "/",
	}, Opt{})
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if got.Path != "/" || got.Domain != "example.com" {
		t.Fatalf("unexpected: %+v", got)
	}
	if got.ID == uuid.Nil {
		t.Fatal("expected ID")
	}
	if got.CreatedAt.IsZero() || got.UpdatedAt.IsZero() {
		t.Fatal("expected timestamps")
	}
}
