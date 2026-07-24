package sqlite

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"github.com/tetiva-app/client/internal/domain/entities"
	"github.com/tetiva-app/client/internal/domain/usecase/search"
)

func TestSearchRepo_FindsCollectionByName(t *testing.T) {
	db := setupTestDB(t)
	repo := NewSearchRepo(db)

	colID := uuid.New()
	_, err := db.Exec(
		`INSERT INTO collections (id, workspace_id, name) VALUES (?, ?, ?)`,
		colID.String(), testWorkspaceID.String(), "Auth",
	)
	if err != nil {
		t.Fatal(err)
	}

	got, err := repo.SearchByName(context.Background(), search.Filter{
		WorkspaceID: testWorkspaceID,
		Query:       "auth",
		Limit:       10,
	})
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if len(got.Hits) != 1 {
		t.Fatalf("expected 1 hit, got %d", len(got.Hits))
	}
	if got.Hits[0].Kind != entities.HitCollection || got.Hits[0].Name != "Auth" {
		t.Errorf("bad hit: %+v", got.Hits[0])
	}
	if got.LimitReached {
		t.Error("LimitReached should be false")
	}
}

func TestSearchRepo_FindsRequestByName(t *testing.T) {
	db := setupTestDB(t)
	repo := NewSearchRepo(db)

	colID := uuid.New()
	reqID := uuid.New()
	if _, err := db.Exec(
		`INSERT INTO collections (id, workspace_id, name) VALUES (?, ?, ?)`,
		colID.String(), testWorkspaceID.String(), "Folder",
	); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(
		`INSERT INTO requests (id, collection_id, name, method, protocol) VALUES (?, ?, ?, ?, ?)`,
		reqID.String(), colID.String(), "Login via Telegram", "POST", "http",
	); err != nil {
		t.Fatal(err)
	}

	got, err := repo.SearchByName(context.Background(), search.Filter{
		WorkspaceID: testWorkspaceID,
		Query:       "telegram",
		Limit:       10,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Hits) != 1 {
		t.Fatalf("expected 1 hit, got %d: %+v", len(got.Hits), got.Hits)
	}
	h := got.Hits[0]
	if h.Kind != entities.HitRequest || h.Name != "Login via Telegram" {
		t.Errorf("bad hit: %+v", h)
	}
	if h.Protocol == nil || *h.Protocol != entities.ProtocolHTTP {
		t.Errorf("expected protocol http, got %v", h.Protocol)
	}
	if h.Method == nil || *h.Method != "POST" {
		t.Errorf("expected method POST, got %v", h.Method)
	}
	if h.ParentID == nil || *h.ParentID != colID {
		t.Errorf("expected parentID=%s, got %v", colID, h.ParentID)
	}
}

func TestSearchRepo_FiltersByWorkspace(t *testing.T) {
	db := setupTestDB(t)
	repo := NewSearchRepo(db)

	otherWS := uuid.MustParse("00000000-0000-4000-a000-000000000099")
	if _, err := db.Exec(
		`INSERT INTO workspaces (id, name) VALUES (?, ?)`,
		otherWS.String(), "Other",
	); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(
		`INSERT INTO collections (id, workspace_id, name) VALUES (?, ?, ?)`,
		uuid.New().String(), otherWS.String(), "Auth",
	); err != nil {
		t.Fatal(err)
	}

	got, err := repo.SearchByName(context.Background(), search.Filter{
		WorkspaceID: testWorkspaceID,
		Query:       "auth",
		Limit:       10,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Hits) != 0 {
		t.Errorf("expected 0 hits (other workspace), got %d", len(got.Hits))
	}
}

func TestSearchRepo_SkipsDeleted(t *testing.T) {
	db := setupTestDB(t)
	repo := NewSearchRepo(db)

	if _, err := db.Exec(
		`INSERT INTO collections (id, workspace_id, name, is_delete) VALUES (?, ?, ?, 1)`,
		uuid.New().String(), testWorkspaceID.String(), "Auth",
	); err != nil {
		t.Fatal(err)
	}

	got, err := repo.SearchByName(context.Background(), search.Filter{
		WorkspaceID: testWorkspaceID, Query: "auth", Limit: 10,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Hits) != 0 {
		t.Errorf("deleted collection should be excluded, got %d hits", len(got.Hits))
	}
}

func TestSearchRepo_CyrillicCaseInsensitive(t *testing.T) {
	db := setupTestDB(t)
	repo := NewSearchRepo(db)

	if _, err := db.Exec(
		`INSERT INTO collections (id, workspace_id, name) VALUES (?, ?, ?)`,
		uuid.New().String(), testWorkspaceID.String(), "Авторизация",
	); err != nil {
		t.Fatal(err)
	}

	got, err := repo.SearchByName(context.Background(), search.Filter{
		WorkspaceID: testWorkspaceID, Query: "авто", Limit: 10,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Hits) != 1 {
		t.Errorf("expected 1 hit for cyrillic substring, got %d", len(got.Hits))
	}
}

func TestSearchRepo_LimitReachedTrueOnlyWhenExceeded(t *testing.T) {
	db := setupTestDB(t)
	repo := NewSearchRepo(db)

	for i := 0; i < 3; i++ {
		if _, err := db.Exec(
			`INSERT INTO collections (id, workspace_id, name) VALUES (?, ?, ?)`,
			uuid.New().String(), testWorkspaceID.String(), "Match "+string(rune('A'+i)),
		); err != nil {
			t.Fatal(err)
		}
	}

	// Exactly 3 matches, limit 3 → LimitReached must be false.
	got, err := repo.SearchByName(context.Background(), search.Filter{
		WorkspaceID: testWorkspaceID, Query: "match", Limit: 3,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Hits) != 3 || got.LimitReached {
		t.Errorf("3 hits / limit 3: expected LimitReached=false, got hits=%d LR=%v", len(got.Hits), got.LimitReached)
	}

	got, err = repo.SearchByName(context.Background(), search.Filter{
		WorkspaceID: testWorkspaceID, Query: "match", Limit: 2,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Hits) != 2 || !got.LimitReached {
		t.Errorf("3 hits / limit 2: expected LimitReached=true, got hits=%d LR=%v", len(got.Hits), got.LimitReached)
	}
}
