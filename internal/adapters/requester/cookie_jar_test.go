package requester

import (
	"context"
	"net/http"
	"net/url"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/tetiva-app/client/internal/domain/entities"
	"github.com/tetiva-app/client/internal/domain/usecase/cookie"
)

type fakeStore struct {
	got map[uuid.UUID][]*http.Cookie
}

func (f *fakeStore) GetCookiesFor(_ context.Context, ws uuid.UUID, _ *url.URL) []*http.Cookie {
	return f.got[ws]
}
func (f *fakeStore) SetCookies(_ context.Context, ws uuid.UUID, _ *url.URL, cs []*http.Cookie) error {
	if f.got == nil {
		f.got = map[uuid.UUID][]*http.Cookie{}
	}
	f.got[ws] = append(f.got[ws], cs...)
	return nil
}

func TestWorkspaceJar_RoutesByWorkspace(t *testing.T) {
	store := &fakeStore{}
	wsA := uuid.New()
	wsB := uuid.New()
	jarA := newWorkspaceJar(context.Background(), store, wsA)
	jarB := newWorkspaceJar(context.Background(), store, wsB)

	u, _ := url.Parse("https://example.com/")
	jarA.SetCookies(u, []*http.Cookie{{Name: "a", Value: "1"}})
	jarB.SetCookies(u, []*http.Cookie{{Name: "b", Value: "2"}})

	if len(store.got[wsA]) != 1 || store.got[wsA][0].Name != "a" {
		t.Fatalf("workspace A jar mis-routed: %+v", store.got[wsA])
	}
	if len(store.got[wsB]) != 1 || store.got[wsB][0].Name != "b" {
		t.Fatalf("workspace B jar mis-routed: %+v", store.got[wsB])
	}
}

// fakeRepo — minimal cookie.Repository for adapter tests.
type fakeRepo struct {
	upserted []*entities.Cookie
	listOut  []*entities.Cookie
	deleted  []uuid.UUID
	matchOut []*entities.Cookie
}

func (f *fakeRepo) Upsert(_ context.Context, c *entities.Cookie) error {
	f.upserted = append(f.upserted, c)
	return nil
}
func (f *fakeRepo) GetByID(context.Context, uuid.UUID) (*entities.Cookie, error) { return nil, nil }
func (f *fakeRepo) List(_ context.Context, _ cookie.Filter) ([]*entities.Cookie, error) {
	return f.listOut, nil
}
func (f *fakeRepo) Delete(_ context.Context, id uuid.UUID) error {
	f.deleted = append(f.deleted, id)
	return nil
}
func (f *fakeRepo) DeleteByDomain(context.Context, uuid.UUID, string) (int, error) { return 0, nil }
func (f *fakeRepo) Clear(context.Context, uuid.UUID) (int, error)                  { return 0, nil }
func (f *fakeRepo) MatchForRequest(_ context.Context, _ uuid.UUID, _ *url.URL) ([]*entities.Cookie, error) {
	return f.matchOut, nil
}

func TestCookieStoreAdapter_GetCookiesFor(t *testing.T) {
	repo := &fakeRepo{matchOut: []*entities.Cookie{
		{Name: "a", Value: "1", Domain: "example.com", Path: "/"},
	}}
	store := NewCookieStore(repo)
	u, _ := url.Parse("https://example.com")
	got := store.GetCookiesFor(context.Background(), uuid.New(), u)
	if len(got) != 1 || got[0].Name != "a" {
		t.Fatalf("unexpected: %+v", got)
	}
}

func TestCookieStoreAdapter_SetCookies_HostOnlyDefault(t *testing.T) {
	repo := &fakeRepo{}
	store := NewCookieStore(repo)
	u, _ := url.Parse("https://api.example.com/x")
	ws := uuid.New()
	expires := time.Now().Add(time.Hour)
	err := store.SetCookies(context.Background(), ws, u, []*http.Cookie{
		{Name: "session", Value: "abc", Path: "/", Expires: expires, HttpOnly: true},
		{Name: "csrf", Value: "xyz"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(repo.upserted) != 2 {
		t.Fatalf("expected 2 upserts, got %d", len(repo.upserted))
	}
	if repo.upserted[0].Domain != "api.example.com" {
		t.Fatalf("expected domain defaulted to host, got %q", repo.upserted[0].Domain)
	}
	if !repo.upserted[0].HostOnly {
		t.Fatal("cookie without Domain= must be host-only")
	}
	if repo.upserted[0].WorkspaceID != ws {
		t.Fatal("workspace not propagated")
	}
}

func TestCookieStoreAdapter_SetCookies_ExplicitDomainNotHostOnly(t *testing.T) {
	repo := &fakeRepo{}
	store := NewCookieStore(repo)
	u, _ := url.Parse("https://api.example.com/x")
	err := store.SetCookies(context.Background(), uuid.New(), u, []*http.Cookie{
		{Name: "session", Value: "abc", Domain: "example.com"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if repo.upserted[0].HostOnly {
		t.Fatal("cookie with explicit Domain= must NOT be host-only")
	}
	if repo.upserted[0].Domain != "example.com" {
		t.Fatalf("expected domain example.com, got %q", repo.upserted[0].Domain)
	}
}

func TestCookieStoreAdapter_SetCookies_MaxAgeNegativeDeletes(t *testing.T) {
	existingID := uuid.New()
	repo := &fakeRepo{
		listOut: []*entities.Cookie{
			{ID: existingID, Domain: "example.com", Path: "/", Name: "session"},
		},
	}
	store := NewCookieStore(repo)
	u, _ := url.Parse("https://example.com/")
	err := store.SetCookies(context.Background(), uuid.New(), u, []*http.Cookie{
		{Name: "session", Value: "", MaxAge: -1},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(repo.upserted) != 0 {
		t.Fatalf("delete cookie must not upsert, got %d upserts", len(repo.upserted))
	}
	if len(repo.deleted) != 1 || repo.deleted[0] != existingID {
		t.Fatalf("expected delete of existing session row, got deletions: %+v", repo.deleted)
	}
}

func TestCookieStoreAdapter_SetCookies_PastExpiresDeletes(t *testing.T) {
	existingID := uuid.New()
	repo := &fakeRepo{
		listOut: []*entities.Cookie{
			{ID: existingID, Domain: "example.com", Path: "/", Name: "old"},
		},
	}
	store := NewCookieStore(repo)
	u, _ := url.Parse("https://example.com/")
	past := time.Now().Add(-time.Hour)
	err := store.SetCookies(context.Background(), uuid.New(), u, []*http.Cookie{
		{Name: "old", Value: "", Expires: past},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(repo.deleted) != 1 || repo.deleted[0] != existingID {
		t.Fatalf("expected delete of expired-cookie row, got: %+v", repo.deleted)
	}
}
