package sqlite

import (
	"context"
	"net/url"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/tetiva-app/client/internal/domain/entities"
	"github.com/tetiva-app/client/internal/domain/usecase/cookie"
)

func TestCookieRepo_UpsertAndList(t *testing.T) {
	db := setupTestDB(t)
	repo := NewCookieRepo(db)
	ctx := context.Background()
	ws := uuid.New()

	c := &entities.Cookie{
		ID: uuid.New(), WorkspaceID: ws, Domain: "example.com", Path: "/",
		Name: "session", Value: "v1", CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}
	if err := repo.Upsert(ctx, c); err != nil {
		t.Fatal(err)
	}

	// Upsert same key — value updates, no new row
	c2 := *c
	c2.Value = "v2"
	c2.UpdatedAt = c.UpdatedAt.Add(time.Second)
	if err := repo.Upsert(ctx, &c2); err != nil {
		t.Fatal(err)
	}

	got, _ := repo.List(ctx, cookie.Filter{WorkspaceID: ws})
	if len(got) != 1 {
		t.Fatalf("expected 1 row, got %d", len(got))
	}
	if got[0].Value != "v2" {
		t.Fatalf("expected updated value, got %q", got[0].Value)
	}
}

func TestCookieRepo_MatchForRequest_DomainAndPath(t *testing.T) {
	db := setupTestDB(t)
	repo := NewCookieRepo(db)
	ctx := context.Background()
	ws := uuid.New()

	mk := func(domain string, hostOnly bool, path, name string) *entities.Cookie {
		return &entities.Cookie{
			ID: uuid.New(), WorkspaceID: ws,
			Domain: domain, HostOnly: hostOnly, Path: path,
			Name: name, Value: "v", CreatedAt: time.Now(), UpdatedAt: time.Now(),
		}
	}
	for _, c := range []*entities.Cookie{
		mk("example.com", true, "/", "host_only_example"),
		mk("example.com", false, "/", "domain_example"),
		mk("api.example.com", false, "/v1", "scoped"),
		mk("other.com", false, "/", "other"),
	} {
		if err := repo.Upsert(ctx, c); err != nil {
			t.Fatal(err)
		}
	}

	u, _ := url.Parse("https://api.example.com/v1/users")
	got, err := repo.MatchForRequest(ctx, ws, u)
	if err != nil {
		t.Fatal(err)
	}

	names := map[string]bool{}
	for _, c := range got {
		names[c.Name] = true
	}
	if !names["domain_example"] || !names["scoped"] {
		t.Fatalf("expected domain_example + scoped, got %+v", names)
	}
	if names["host_only_example"] {
		t.Fatalf("host_only example.com cookie must not match api.example.com: %+v", names)
	}
	if names["other"] {
		t.Fatalf("did not expect other.com cookie: %+v", names)
	}
}

func TestCookieRepo_MatchForRequest_ExpiredFiltered(t *testing.T) {
	db := setupTestDB(t)
	repo := NewCookieRepo(db)
	ctx := context.Background()
	ws := uuid.New()
	past := time.Now().Add(-1 * time.Hour)

	if err := repo.Upsert(ctx, &entities.Cookie{
		ID: uuid.New(), WorkspaceID: ws, Domain: "example.com", Path: "/",
		Name: "expired", Value: "v", ExpiresAt: &past,
		CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}); err != nil {
		t.Fatal(err)
	}

	u, _ := url.Parse("https://example.com/")
	got, _ := repo.MatchForRequest(ctx, ws, u)
	if len(got) != 0 {
		t.Fatalf("expected expired cookie filtered, got %d", len(got))
	}
}
