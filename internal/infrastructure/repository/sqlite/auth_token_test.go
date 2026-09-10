package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/tetiva-app/client/internal/domain/entities"
	"github.com/tetiva-app/client/internal/domain/usecase/auth"
	"github.com/tetiva-app/client/migrations"
	"github.com/tetiva-app/client/pkg/migrate"
)

// Collection and request are both oauth2 so the orphan predicate keeps their token rows.
func seedOAuthOwners(t *testing.T, db *sql.DB) (*entities.Collection, *entities.Request) {
	t.Helper()
	ctx := context.Background()

	coll := newTestCollection("Owners", nil)
	coll.AuthType = entities.AuthTypeOAuth2
	coll.AuthData = `{"grant":"client_credentials"}`
	if err := NewCollectionRepo(db).Create(ctx, coll); err != nil {
		t.Fatalf("create collection: %v", err)
	}

	req := newTestRequest("Owner request", coll.ID)
	req.AuthType = entities.AuthTypeOAuth2
	req.AuthData = `{"grant":"client_credentials"}`
	if err := NewRequestRepo(db).Create(ctx, req); err != nil {
		t.Fatalf("create request: %v", err)
	}

	return coll, req
}

func requestOwner(r *entities.Request) entities.AuthOwner {
	return entities.AuthOwner{WorkspaceID: testWorkspaceID, Kind: entities.AuthOwnerKindRequest, ID: r.ID}
}

func collectionOwner(c *entities.Collection) entities.AuthOwner {
	return entities.AuthOwner{WorkspaceID: c.WorkspaceID, Kind: entities.AuthOwnerKindCollection, ID: c.ID}
}

func sampleToken() *auth.Token {
	now := time.Now().Truncate(time.Second)
	return &auth.Token{
		AccessToken:  "at-1",
		RefreshToken: "rt-1",
		TokenType:    "Bearer",
		Scope:        "read",
		IDToken:      "id-1",
		ExpiresAt:    now.Add(time.Hour),
		ObtainedAt:   now,
	}
}

func TestAuthTokenRepo_ReserveThenPutThenGet(t *testing.T) {
	db := setupTestDB(t)
	repo := NewAuthTokenRepo(db)
	ctx := context.Background()
	_, req := seedOAuthOwners(t, db)
	owner := requestOwner(req)

	if got, err := repo.Get(ctx, owner); err != nil || got != nil {
		t.Fatalf("Get before reserve = %v, %v; want nil, nil", got, err)
	}

	gen, err := repo.Reserve(ctx, owner)
	if err != nil {
		t.Fatalf("Reserve: %v", err)
	}
	if gen == 0 {
		t.Fatal("first generation is the zero value a stale writer would quote")
	}
	// A reserved row holds no token yet.
	if got, err := repo.Get(ctx, owner); err != nil || got != nil {
		t.Fatalf("Get after reserve = %v, %v; want nil, nil", got, err)
	}

	tok := sampleToken()
	if err := repo.Put(ctx, owner, gen, "hash-1", tok); err != nil {
		t.Fatalf("Put: %v", err)
	}

	got, err := repo.Get(ctx, owner)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got == nil {
		t.Fatal("Get returned nil after Put")
	}
	if got.AccessToken != tok.AccessToken || got.RefreshToken != tok.RefreshToken ||
		got.TokenType != tok.TokenType || got.Scope != tok.Scope || got.IDToken != tok.IDToken {
		t.Errorf("stored token mismatch: %+v", got.Token)
	}
	if !got.ExpiresAt.Equal(tok.ExpiresAt) || !got.ObtainedAt.Equal(tok.ObtainedAt) {
		t.Errorf("stored times mismatch: %v / %v", got.ExpiresAt, got.ObtainedAt)
	}
	// Put is a compare-and-swap: the write advances the generation it consumed.
	if got.ConfigHash != "hash-1" || got.Generation != gen+1 {
		t.Errorf("hash/generation = %q/%d, want hash-1/%d", got.ConfigHash, got.Generation, gen+1)
	}

	// Reserving again keeps the stored token and reports the generation Put left.
	gen2, err := repo.Reserve(ctx, owner)
	if err != nil {
		t.Fatalf("second Reserve: %v", err)
	}
	if gen2 != gen+1 {
		t.Errorf("second generation = %d, want %d", gen2, gen+1)
	}
	if got, err := repo.Get(ctx, owner); err != nil || got == nil || got.AccessToken != "at-1" {
		t.Errorf("token lost by second Reserve: %v, %v", got, err)
	}
}

func TestAuthTokenRepo_ReserveCollectionOwner(t *testing.T) {
	db := setupTestDB(t)
	repo := NewAuthTokenRepo(db)
	ctx := context.Background()
	coll, _ := seedOAuthOwners(t, db)

	gen, err := repo.Reserve(ctx, collectionOwner(coll))
	if err != nil {
		t.Fatalf("Reserve: %v", err)
	}
	if err := repo.Put(ctx, collectionOwner(coll), gen, "h", sampleToken()); err != nil {
		t.Fatalf("Put: %v", err)
	}
	if got, err := repo.Get(ctx, collectionOwner(coll)); err != nil || got == nil {
		t.Fatalf("Get = %v, %v", got, err)
	}
}

func TestAuthTokenRepo_Reserve_DeadOwner(t *testing.T) {
	db := setupTestDB(t)
	repo := NewAuthTokenRepo(db)
	ctx := context.Background()
	coll, req := seedOAuthOwners(t, db)

	cases := []struct {
		name  string
		owner entities.AuthOwner
		setup func()
	}{
		{
			name:  "unknown request",
			owner: entities.AuthOwner{WorkspaceID: testWorkspaceID, Kind: entities.AuthOwnerKindRequest, ID: uuid.New()},
		},
		{
			name:  "unknown collection",
			owner: entities.AuthOwner{WorkspaceID: testWorkspaceID, Kind: entities.AuthOwnerKindCollection, ID: uuid.New()},
		},
		{
			name:  "other workspace",
			owner: entities.AuthOwner{WorkspaceID: uuid.New(), Kind: entities.AuthOwnerKindRequest, ID: req.ID},
		},
		{
			name:  "soft-deleted request",
			owner: requestOwner(req),
			setup: func() {
				if _, err := db.Exec(`UPDATE requests SET is_delete = 1 WHERE id = ?`, req.ID.String()); err != nil {
					t.Fatalf("soft delete: %v", err)
				}
			},
		},
		{
			name:  "soft-deleted collection",
			owner: collectionOwner(coll),
			setup: func() {
				if _, err := db.Exec(`UPDATE collections SET is_delete = 1 WHERE id = ?`, coll.ID.String()); err != nil {
					t.Fatalf("soft delete: %v", err)
				}
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.setup != nil {
				tc.setup()
			}
			if _, err := repo.Reserve(ctx, tc.owner); !errors.Is(err, auth.ErrOwnerGone) {
				t.Fatalf("Reserve error = %v, want ErrOwnerGone", err)
			}
		})
	}
}

func TestAuthTokenRepo_Put_AdvancesGeneration(t *testing.T) {
	db := setupTestDB(t)
	repo := NewAuthTokenRepo(db)
	ctx := context.Background()
	_, req := seedOAuthOwners(t, db)
	owner := requestOwner(req)

	gen, err := repo.Reserve(ctx, owner)
	if err != nil {
		t.Fatalf("Reserve: %v", err)
	}
	if err := repo.Put(ctx, owner, gen, "h", sampleToken()); err != nil {
		t.Fatalf("Put: %v", err)
	}

	after, err := repo.Generation(ctx, owner)
	if err != nil {
		t.Fatalf("Generation: %v", err)
	}
	if after != gen+1 {
		t.Fatalf("generation after Put = %d, want %d", after, gen+1)
	}
	// A second writer holding the same reservation must now be refused.
	if err := repo.Put(ctx, owner, gen, "h2", sampleToken()); !errors.Is(err, auth.ErrStaleToken) {
		t.Fatalf("replayed Put = %v, want ErrStaleToken", err)
	}
	if got, err := repo.Get(ctx, owner); err != nil || got == nil || got.ConfigHash != "h" {
		t.Fatalf("the refused write changed the row: %+v, %v", got, err)
	}
}

// Two writers that reserved the same generation race on an uncapped pool — the shape NewDB
// hands the app. Exactly one commits; busy_timeout from the DSN keeps the other off SQLITE_BUSY.
func TestAuthTokenRepo_TwoWritersOneReserve_UncappedPool(t *testing.T) {
	db, err := sql.Open("sqlite", DSN(filepath.Join(t.TempDir(), "test.db")))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := migrate.Run(db, migrations.FS, "."); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	repo := NewAuthTokenRepo(db)
	ctx := context.Background()
	_, req := seedOAuthOwners(t, db)
	owner := requestOwner(req)

	for round := range 20 {
		gen, err := repo.Reserve(ctx, owner)
		if err != nil {
			t.Fatalf("round %d: Reserve: %v", round, err)
		}

		errs := make([]error, 2)
		start := make(chan struct{})
		var wg sync.WaitGroup
		for i := range errs {
			wg.Add(1)
			go func() {
				defer wg.Done()
				<-start
				errs[i] = repo.Put(ctx, owner, gen, "h", sampleToken())
			}()
		}
		close(start)
		wg.Wait()

		var won, stale int
		for _, err := range errs {
			switch {
			case err == nil:
				won++
			case errors.Is(err, auth.ErrStaleToken):
				stale++
			case errors.Is(err, auth.ErrStoreBusy):
				t.Fatalf("round %d: busy_timeout did not reach the pool: %v", round, err)
			default:
				t.Fatalf("round %d: Put: %v", round, err)
			}
		}
		if won != 1 || stale != 1 {
			t.Fatalf("round %d: %d winners, %d stale; want 1 and 1", round, won, stale)
		}
	}
}

func TestAuthTokenRepo_Put_StaleGeneration(t *testing.T) {
	db := setupTestDB(t)
	repo := NewAuthTokenRepo(db)
	ctx := context.Background()
	_, req := seedOAuthOwners(t, db)
	owner := requestOwner(req)

	gen, err := repo.Reserve(ctx, owner)
	if err != nil {
		t.Fatalf("Reserve: %v", err)
	}
	if err := repo.Clear(ctx, owner); err != nil {
		t.Fatalf("Clear: %v", err)
	}

	if err := repo.Put(ctx, owner, gen, "h", sampleToken()); !errors.Is(err, auth.ErrStaleToken) {
		t.Fatalf("Put after Clear = %v, want ErrStaleToken", err)
	}
	if got, err := repo.Get(ctx, owner); err != nil || got != nil {
		t.Fatalf("Get after stale Put = %v, %v; want nil, nil", got, err)
	}

	// The generation the clear left behind is the one a new acquisition quotes.
	newGen, err := repo.Reserve(ctx, owner)
	if err != nil {
		t.Fatalf("Reserve after Clear: %v", err)
	}
	if newGen != gen+1 {
		t.Fatalf("generation after Clear = %d, want %d", newGen, gen+1)
	}
	if err := repo.Put(ctx, owner, newGen, "h", sampleToken()); err != nil {
		t.Fatalf("Put with fresh generation: %v", err)
	}
}

func TestAuthTokenRepo_Put_AfterTheOwnerRequalifies(t *testing.T) {
	db := setupTestDB(t)
	repo := NewAuthTokenRepo(db)
	ctx := context.Background()
	_, req := seedOAuthOwners(t, db)
	owner := requestOwner(req)

	gen, err := repo.Reserve(ctx, owner)
	if err != nil {
		t.Fatalf("Reserve: %v", err)
	}

	// The owner leaves oauth2 and comes back while the acquisition is still out:
	// the sweep drops the row in between, so the second Reserve builds a new one.
	if _, err := db.Exec(`UPDATE requests SET auth_type = 'none' WHERE id = ?`, req.ID.String()); err != nil {
		t.Fatalf("change auth: %v", err)
	}
	if err := repo.ClearOwners(ctx, entities.AuthOwnerKindRequest, []uuid.UUID{req.ID}); err != nil {
		t.Fatalf("ClearOwners: %v", err)
	}
	if _, err := repo.DeleteOrphans(ctx); err != nil {
		t.Fatalf("DeleteOrphans: %v", err)
	}
	if left, err := repo.Generation(ctx, owner); err != nil || left != 0 {
		t.Fatalf("row survived the sweep: generation %d, %v", left, err)
	}
	if _, err := db.Exec(`UPDATE requests SET auth_type = 'oauth2' WHERE id = ?`, req.ID.String()); err != nil {
		t.Fatalf("restore auth: %v", err)
	}

	newGen, err := repo.Reserve(ctx, owner)
	if err != nil {
		t.Fatalf("Reserve after requalifying: %v", err)
	}
	if newGen == gen {
		t.Fatalf("recreated row reused generation %d", gen)
	}
	if err := repo.Put(ctx, owner, gen, "h", sampleToken()); !errors.Is(err, auth.ErrStaleToken) {
		t.Fatalf("Put from the old acquisition = %v, want ErrStaleToken", err)
	}
	if got, err := repo.Get(ctx, owner); err != nil || got != nil {
		t.Fatalf("Get after stale Put = %v, %v; want nil, nil", got, err)
	}
	if err := repo.Put(ctx, owner, newGen, "h", sampleToken()); err != nil {
		t.Fatalf("Put from the current acquisition: %v", err)
	}
}

func TestAuthTokenRepo_Clear_WithoutRow_CreatesTombstone(t *testing.T) {
	db := setupTestDB(t)
	repo := NewAuthTokenRepo(db)
	ctx := context.Background()
	_, req := seedOAuthOwners(t, db)
	owner := requestOwner(req)

	if gen, err := repo.Generation(ctx, owner); err != nil || gen != 0 {
		t.Fatalf("Generation without row = %d, %v; want 0, nil", gen, err)
	}
	if err := repo.Clear(ctx, owner); err != nil {
		t.Fatalf("Clear: %v", err)
	}

	gen, err := repo.Generation(ctx, owner)
	if err != nil {
		t.Fatalf("Generation: %v", err)
	}
	if gen == 0 {
		t.Fatal("Clear left no tombstone")
	}
	// A fetch that started before the clear quoted generation 0 and must fail.
	if err := repo.Put(ctx, owner, 0, "h", sampleToken()); !errors.Is(err, auth.ErrStaleToken) {
		t.Fatalf("Put = %v, want ErrStaleToken", err)
	}
}

func TestAuthTokenRepo_Clear_BlanksStoredToken(t *testing.T) {
	db := setupTestDB(t)
	repo := NewAuthTokenRepo(db)
	ctx := context.Background()
	_, req := seedOAuthOwners(t, db)
	owner := requestOwner(req)

	gen, err := repo.Reserve(ctx, owner)
	if err != nil {
		t.Fatalf("Reserve: %v", err)
	}
	if err := repo.Put(ctx, owner, gen, "h", sampleToken()); err != nil {
		t.Fatalf("Put: %v", err)
	}
	if err := repo.Clear(ctx, owner); err != nil {
		t.Fatalf("Clear: %v", err)
	}

	var access, refresh, idToken, hash string
	err = db.QueryRow(`SELECT access_token, refresh_token, id_token, config_hash FROM auth_tokens
		WHERE workspace_id = ? AND owner_kind = ? AND owner_id = ?`,
		owner.WorkspaceID.String(), owner.Kind, owner.ID.String()).Scan(&access, &refresh, &idToken, &hash)
	if err != nil {
		t.Fatalf("read row: %v", err)
	}
	if access != "" || refresh != "" || idToken != "" || hash != "" {
		t.Fatalf("token survived Clear: %q %q %q %q", access, refresh, idToken, hash)
	}
}

func TestAuthTokenRepo_ClearOwners(t *testing.T) {
	db := setupTestDB(t)
	repo := NewAuthTokenRepo(db)
	ctx := context.Background()
	coll, req := seedOAuthOwners(t, db)

	other := newTestRequest("Other", coll.ID)
	other.AuthType = entities.AuthTypeOAuth2
	if err := NewRequestRepo(db).Create(ctx, other); err != nil {
		t.Fatalf("create request: %v", err)
	}

	gens := map[uuid.UUID]int64{}
	for _, owner := range []entities.AuthOwner{requestOwner(req), requestOwner(other), collectionOwner(coll)} {
		gen, err := repo.Reserve(ctx, owner)
		if err != nil {
			t.Fatalf("Reserve: %v", err)
		}
		gens[owner.ID] = gen
		if err := repo.Put(ctx, owner, gen, "h", sampleToken()); err != nil {
			t.Fatalf("Put: %v", err)
		}
	}

	if err := repo.ClearOwners(ctx, entities.AuthOwnerKindRequest, []uuid.UUID{req.ID, other.ID}); err != nil {
		t.Fatalf("ClearOwners: %v", err)
	}

	for _, owner := range []entities.AuthOwner{requestOwner(req), requestOwner(other)} {
		if got, err := repo.Get(ctx, owner); err != nil || got != nil {
			t.Errorf("request token survived: %v, %v", got, err)
		}
		// One step for the Put that stored the token, one for the sweep.
		if gen, err := repo.Generation(ctx, owner); err != nil || gen != gens[owner.ID]+2 {
			t.Errorf("generation = %d, %v; want %d", gen, err, gens[owner.ID]+2)
		}
	}
	if got, err := repo.Get(ctx, collectionOwner(coll)); err != nil || got == nil {
		t.Errorf("collection token cleared by request sweep: %v, %v", got, err)
	}

	if err := repo.ClearOwners(ctx, entities.AuthOwnerKindRequest, nil); err != nil {
		t.Fatalf("ClearOwners with no ids: %v", err)
	}
}

func TestAuthTokenRepo_ClearOwnersUnlessHash(t *testing.T) {
	db := setupTestDB(t)
	repo := NewAuthTokenRepo(db)
	ctx := context.Background()
	coll, req := seedOAuthOwners(t, db)
	owner := requestOwner(req)

	gen, err := repo.Reserve(ctx, owner)
	if err != nil {
		t.Fatalf("Reserve: %v", err)
	}
	if err := repo.Put(ctx, owner, gen, "keep-me", sampleToken()); err != nil {
		t.Fatalf("Put: %v", err)
	}

	// The edit saved the very configuration the token was acquired with.
	if err := repo.ClearOwnersUnlessHash(ctx, entities.AuthOwnerKindRequest, []uuid.UUID{req.ID}, "keep-me"); err != nil {
		t.Fatalf("ClearOwnersUnlessHash: %v", err)
	}
	got, err := repo.Get(ctx, owner)
	if err != nil || got == nil || got.AccessToken != "at-1" {
		t.Fatalf("token dropped for its own config: %v, %v", got, err)
	}
	if got.Generation != gen+1 {
		t.Errorf("generation = %d, want %d", got.Generation, gen+1)
	}

	if err := repo.ClearOwnersUnlessHash(ctx, entities.AuthOwnerKindRequest, []uuid.UUID{req.ID}, "another-config"); err != nil {
		t.Fatalf("ClearOwnersUnlessHash: %v", err)
	}
	if got, err := repo.Get(ctx, owner); err != nil || got != nil {
		t.Fatalf("token survived a different config: %v, %v", got, err)
	}

	// A reserved but empty row is a token in flight; the hash cannot spare it.
	collOwner := collectionOwner(coll)
	inFlight, err := repo.Reserve(ctx, collOwner)
	if err != nil {
		t.Fatalf("Reserve: %v", err)
	}
	if err := repo.ClearOwnersUnlessHash(ctx, entities.AuthOwnerKindCollection, []uuid.UUID{coll.ID}, "keep-me"); err != nil {
		t.Fatalf("ClearOwnersUnlessHash: %v", err)
	}
	if err := repo.Put(ctx, collOwner, inFlight, "keep-me", sampleToken()); !errors.Is(err, auth.ErrStaleToken) {
		t.Fatalf("Put from the acquisition in flight = %v, want ErrStaleToken", err)
	}
}

func TestAuthTokenRepo_DeleteOrphans(t *testing.T) {
	ctx := context.Background()

	// The owner needs a real token so the sweep has something to blank before dropping the row.
	storeToken := func(t *testing.T, repo *AuthTokenRepo, owner entities.AuthOwner) {
		t.Helper()
		gen, err := repo.Reserve(ctx, owner)
		if err != nil {
			t.Fatalf("Reserve: %v", err)
		}
		if err := repo.Put(ctx, owner, gen, "h", sampleToken()); err != nil {
			t.Fatalf("Put: %v", err)
		}
	}

	cases := []struct {
		name string
		// mutate breaks the owner after the token is stored.
		mutate func(t *testing.T, db *sql.DB, coll *entities.Collection, req *entities.Request)
		// owner picks the token row under test.
		owner     func(coll *entities.Collection, req *entities.Request) entities.AuthOwner
		wantSwept bool
	}{
		{
			name:      "healthy owner keeps its token",
			owner:     func(_ *entities.Collection, r *entities.Request) entities.AuthOwner { return requestOwner(r) },
			wantSwept: false,
		},
		{
			name: "hard-deleted request drops the row",
			mutate: func(t *testing.T, db *sql.DB, _ *entities.Collection, r *entities.Request) {
				if _, err := db.Exec(`DELETE FROM requests WHERE id = ?`, r.ID.String()); err != nil {
					t.Fatalf("delete: %v", err)
				}
			},
			owner:     func(_ *entities.Collection, r *entities.Request) entities.AuthOwner { return requestOwner(r) },
			wantSwept: true,
		},
		{
			name: "soft-deleted request is swept",
			mutate: func(t *testing.T, db *sql.DB, _ *entities.Collection, r *entities.Request) {
				if _, err := db.Exec(`UPDATE requests SET is_delete = 1 WHERE id = ?`, r.ID.String()); err != nil {
					t.Fatalf("soft delete: %v", err)
				}
			},
			owner:     func(_ *entities.Collection, r *entities.Request) entities.AuthOwner { return requestOwner(r) },
			wantSwept: true,
		},
		{
			name: "request whose auth left oauth2 is swept",
			mutate: func(t *testing.T, db *sql.DB, _ *entities.Collection, r *entities.Request) {
				if _, err := db.Exec(`UPDATE requests SET auth_type = 'inherit' WHERE id = ?`, r.ID.String()); err != nil {
					t.Fatalf("change auth: %v", err)
				}
			},
			owner:     func(_ *entities.Collection, r *entities.Request) entities.AuthOwner { return requestOwner(r) },
			wantSwept: true,
		},
		{
			name: "request moved to another workspace is swept",
			mutate: func(t *testing.T, db *sql.DB, _ *entities.Collection, r *entities.Request) {
				moveToOtherWorkspace(t, db, r)
			},
			owner:     func(_ *entities.Collection, r *entities.Request) entities.AuthOwner { return requestOwner(r) },
			wantSwept: true,
		},
		{
			name: "request under a soft-deleted ancestor is swept",
			mutate: func(t *testing.T, db *sql.DB, c *entities.Collection, _ *entities.Request) {
				if _, err := db.Exec(`UPDATE collections SET is_delete = 1 WHERE id = ?`, c.ID.String()); err != nil {
					t.Fatalf("soft delete: %v", err)
				}
			},
			owner:     func(_ *entities.Collection, r *entities.Request) entities.AuthOwner { return requestOwner(r) },
			wantSwept: true,
		},
		{
			name: "request under a soft-deleted grandparent is swept",
			mutate: func(t *testing.T, db *sql.DB, c *entities.Collection, r *entities.Request) {
				grandparent := newTestCollection("Grandparent", nil)
				if err := NewCollectionRepo(db).Create(ctx, grandparent); err != nil {
					t.Fatalf("create grandparent: %v", err)
				}
				if _, err := db.Exec(`UPDATE collections SET parent_id = ? WHERE id = ?`, grandparent.ID.String(), c.ID.String()); err != nil {
					t.Fatalf("reparent: %v", err)
				}
				if _, err := db.Exec(`UPDATE collections SET is_delete = 1 WHERE id = ?`, grandparent.ID.String()); err != nil {
					t.Fatalf("soft delete: %v", err)
				}
			},
			owner:     func(_ *entities.Collection, r *entities.Request) entities.AuthOwner { return requestOwner(r) },
			wantSwept: true,
		},
		{
			name: "soft-deleted collection is swept",
			mutate: func(t *testing.T, db *sql.DB, c *entities.Collection, _ *entities.Request) {
				if _, err := db.Exec(`UPDATE collections SET is_delete = 1 WHERE id = ?`, c.ID.String()); err != nil {
					t.Fatalf("soft delete: %v", err)
				}
			},
			owner:     func(c *entities.Collection, _ *entities.Request) entities.AuthOwner { return collectionOwner(c) },
			wantSwept: true,
		},
		{
			name: "collection whose auth left oauth2 is swept",
			mutate: func(t *testing.T, db *sql.DB, c *entities.Collection, _ *entities.Request) {
				if _, err := db.Exec(`UPDATE collections SET auth_type = 'basic' WHERE id = ?`, c.ID.String()); err != nil {
					t.Fatalf("change auth: %v", err)
				}
			},
			owner:     func(c *entities.Collection, _ *entities.Request) entities.AuthOwner { return collectionOwner(c) },
			wantSwept: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			db := setupTestDB(t)
			repo := NewAuthTokenRepo(db)
			coll, req := seedOAuthOwners(t, db)
			owner := tc.owner(coll, req)
			storeToken(t, repo, owner)

			if tc.mutate != nil {
				tc.mutate(t, db, coll, req)
			}

			n, err := repo.DeleteOrphans(ctx)
			if err != nil {
				t.Fatalf("DeleteOrphans: %v", err)
			}
			if tc.wantSwept != (n > 0) {
				t.Fatalf("swept %d rows, wantSwept=%v", n, tc.wantSwept)
			}

			got, err := repo.Get(ctx, owner)
			if err != nil {
				t.Fatalf("Get: %v", err)
			}
			if tc.wantSwept && got != nil {
				t.Fatalf("token survived the sweep: %+v", got)
			}
			if !tc.wantSwept && got == nil {
				t.Fatal("healthy token was swept")
			}

			var rows int
			if err := db.QueryRow(`SELECT COUNT(1) FROM auth_tokens WHERE owner_id = ?`, owner.ID.String()).Scan(&rows); err != nil {
				t.Fatalf("count rows: %v", err)
			}
			wantRows := 1
			if tc.wantSwept {
				wantRows = 0
			}
			if rows != wantRows {
				t.Fatalf("row count = %d, want %d", rows, wantRows)
			}

			// The sweep leaves nothing for the next one to touch.
			again, err := repo.DeleteOrphans(ctx)
			if err != nil {
				t.Fatalf("second DeleteOrphans: %v", err)
			}
			if again != 0 {
				t.Fatalf("second sweep removed %d rows, want 0", again)
			}
		})
	}
}

// Under a collection of a second workspace, its token row still keyed to the one it came from.
func moveToOtherWorkspace(t *testing.T, db *sql.DB, r *entities.Request) {
	t.Helper()
	otherWS := uuid.New()
	if _, err := db.Exec(`INSERT INTO workspaces (id, name) VALUES (?, 'Other')`, otherWS.String()); err != nil {
		t.Fatalf("workspace: %v", err)
	}
	target := newTestCollection("Elsewhere", nil)
	target.WorkspaceID = otherWS
	if err := NewCollectionRepo(db).Create(context.Background(), target); err != nil {
		t.Fatalf("collection: %v", err)
	}
	if _, err := db.Exec(`UPDATE requests SET collection_id = ? WHERE id = ?`, target.ID.String(), r.ID.String()); err != nil {
		t.Fatalf("move: %v", err)
	}
}

func TestAuthTokenRepo_DeleteOrphans_ReservedRowGoesStale(t *testing.T) {
	ctx := context.Background()

	cases := []struct {
		name string
		// mutate breaks the owner while the acquisition is still in flight.
		mutate func(t *testing.T, db *sql.DB, coll *entities.Collection, req *entities.Request)
	}{
		{
			name: "parent collection soft-deleted",
			mutate: func(t *testing.T, db *sql.DB, c *entities.Collection, _ *entities.Request) {
				if _, err := db.Exec(`UPDATE collections SET is_delete = 1 WHERE id = ?`, c.ID.String()); err != nil {
					t.Fatalf("soft delete: %v", err)
				}
			},
		},
		{
			name: "request moved to another workspace",
			mutate: func(t *testing.T, db *sql.DB, _ *entities.Collection, r *entities.Request) {
				moveToOtherWorkspace(t, db, r)
			},
		},
		{
			name: "request auth left oauth2",
			mutate: func(t *testing.T, db *sql.DB, _ *entities.Collection, r *entities.Request) {
				if _, err := db.Exec(`UPDATE requests SET auth_type = 'basic' WHERE id = ?`, r.ID.String()); err != nil {
					t.Fatalf("change auth: %v", err)
				}
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			db := setupTestDB(t)
			repo := NewAuthTokenRepo(db)
			coll, req := seedOAuthOwners(t, db)
			owner := requestOwner(req)

			gen, err := repo.Reserve(ctx, owner)
			if err != nil {
				t.Fatalf("Reserve: %v", err)
			}

			tc.mutate(t, db, coll, req)

			n, err := repo.DeleteOrphans(ctx)
			if err != nil {
				t.Fatalf("DeleteOrphans: %v", err)
			}
			if n != 1 {
				t.Fatalf("swept %d rows, want 1", n)
			}

			if err := repo.Put(ctx, owner, gen, "h", sampleToken()); !errors.Is(err, auth.ErrStaleToken) {
				t.Fatalf("Put after the sweep = %v, want ErrStaleToken", err)
			}
			if got, err := repo.Get(ctx, owner); err != nil || got != nil {
				t.Fatalf("Get after the sweep = %v, %v; want nil, nil", got, err)
			}
		})
	}
}

func TestAuthTokenRepo_DeleteOrphans_CollectionCycleTerminates(t *testing.T) {
	db := setupTestDB(t)
	repo := NewAuthTokenRepo(db)
	ctx := context.Background()
	coll, req := seedOAuthOwners(t, db)

	child := newTestCollection("Child", &coll.ID)
	child.AuthType = entities.AuthTypeOAuth2
	if err := NewCollectionRepo(db).Create(ctx, child); err != nil {
		t.Fatalf("create child: %v", err)
	}
	// A parent cycle can only arrive from a broken sync payload; the walk must
	// still terminate.
	if _, err := db.Exec(`UPDATE collections SET parent_id = ?, is_delete = 1 WHERE id = ?`, child.ID.String(), coll.ID.String()); err != nil {
		t.Fatalf("cycle: %v", err)
	}

	owner := requestOwner(req)
	if _, err := repo.Reserve(ctx, owner); err == nil {
		t.Fatal("Reserve under a deleted collection should fail")
	}

	done := make(chan struct{})
	go func() {
		defer close(done)
		if _, err := repo.DeleteOrphans(ctx); err != nil {
			t.Errorf("DeleteOrphans: %v", err)
		}
	}()
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("DeleteOrphans did not terminate on a parent cycle")
	}
}
