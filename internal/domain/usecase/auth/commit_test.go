package auth

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/tetiva-app/client/internal/domain/entities"
)

// scriptedRepo replays a canned outcome per Put attempt on top of fakeRepo, so a commit can meet a
// busy store or a lost race in turn; a blocking attempt honours its context.
type scriptedRepo struct {
	*fakeRepo

	mu        sync.Mutex
	script    []error
	blocks    int
	getBlocks bool
	calls     int
}

func newScriptedRepo(script ...error) *scriptedRepo {
	return &scriptedRepo{fakeRepo: newFakeRepo(), script: script}
}

func (r *scriptedRepo) Put(ctx context.Context, owner entities.AuthOwner, generation int64, hash string, t *Token) error {
	r.mu.Lock()
	attempt := r.calls
	r.calls++
	var scripted error
	if attempt < len(r.script) {
		scripted = r.script[attempt]
	}
	blocks := attempt < r.blocks
	r.mu.Unlock()

	if blocks {
		<-ctx.Done()

		return ctx.Err()
	}
	if scripted != nil {
		return scripted
	}

	return r.fakeRepo.Put(ctx, owner, generation, hash, t)
}

func (r *scriptedRepo) Get(ctx context.Context, owner entities.AuthOwner) (*StoredToken, error) {
	r.mu.Lock()
	blocks := r.getBlocks
	r.mu.Unlock()

	if blocks {
		<-ctx.Done()

		return nil, ctx.Err()
	}

	return r.fakeRepo.Get(ctx, owner)
}

func (r *scriptedRepo) attempts() int {
	r.mu.Lock()
	defer r.mu.Unlock()

	return r.calls
}

func freshToken() *Token {
	return &Token{AccessToken: "fresh", RefreshToken: "rt-fresh", ObtainedAt: testNow}
}

func TestCommitTokenWritesAndReturnsTheToken(t *testing.T) {
	repo := newScriptedRepo()
	owner := testOwner()
	gen, err := repo.Reserve(context.Background(), owner)
	if err != nil {
		t.Fatalf("Reserve: %v", err)
	}
	tok := freshToken()

	got, err := commitToken(context.Background(), repo, owner, gen, "hash-1", tok, testNow)
	if err != nil {
		t.Fatalf("commitToken: %v", err)
	}
	if got != tok {
		t.Errorf("got %+v, want the token that was written", got)
	}
	if row := repo.row(owner); row == nil || row.AccessToken != "fresh" || row.ConfigHash != "hash-1" {
		t.Errorf("stored row: %+v", row)
	}
}

func TestCommitTokenAdoptsTheWinnersToken(t *testing.T) {
	repo := newScriptedRepo(ErrStaleToken)
	owner := testOwner()
	repo.seed(owner, "hash-1", Token{AccessToken: "winner", ExpiresAt: testNow.Add(time.Hour)})

	got, err := commitToken(context.Background(), repo, owner, 0, "hash-1", freshToken(), testNow)
	if err != nil {
		t.Fatalf("commitToken: %v", err)
	}
	// The losing flow discards its own token, refresh token and all: the adopted
	// one was minted for the same configuration and is usable now.
	if got.AccessToken != "winner" {
		t.Errorf("adopted token = %q, want the stored one", got.AccessToken)
	}
}

func TestCommitTokenStaleWithoutAdoptableToken(t *testing.T) {
	owner := testOwner()

	tests := []struct {
		name string
		seed func(repo *scriptedRepo)
	}{
		{name: "row cleared", seed: func(repo *scriptedRepo) {
			if err := repo.Clear(context.Background(), owner); err != nil {
				t.Fatalf("Clear: %v", err)
			}
		}},
		{name: "another configuration", seed: func(repo *scriptedRepo) {
			repo.seed(owner, "hash-of-another-env", Token{AccessToken: "other", ExpiresAt: testNow.Add(time.Hour)})
		}},
		{name: "stored token expired", seed: func(repo *scriptedRepo) {
			repo.seed(owner, "hash-1", Token{AccessToken: "old", ExpiresAt: testNow.Add(-time.Minute)})
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := newScriptedRepo(ErrStaleToken)
			tt.seed(repo)

			got, err := commitToken(context.Background(), repo, owner, 0, "hash-1", freshToken(), testNow)
			if !errors.Is(err, ErrStaleToken) {
				t.Fatalf("commitToken = %v, %v; want ErrStaleToken", got, err)
			}
		})
	}
}

func TestCommitTokenRetriesABusyStore(t *testing.T) {
	repo := newScriptedRepo(ErrStoreBusy)
	owner := testOwner()
	gen, err := repo.Reserve(context.Background(), owner)
	if err != nil {
		t.Fatalf("Reserve: %v", err)
	}

	start := time.Now()
	got, err := commitToken(context.Background(), repo, owner, gen, "hash-1", freshToken(), testNow)
	if err != nil {
		t.Fatalf("commitToken: %v", err)
	}
	if got.AccessToken != "fresh" {
		t.Errorf("token = %q", got.AccessToken)
	}
	if repo.attempts() != 2 {
		t.Errorf("attempts = %d, want 2", repo.attempts())
	}
	if elapsed := time.Since(start); elapsed < commitBackoff {
		t.Errorf("retried after %v, want at least %v", elapsed, commitBackoff)
	}
}

func TestCommitTokenSurfacesAPersistentlyBusyStore(t *testing.T) {
	repo := newScriptedRepo(ErrStoreBusy, ErrStoreBusy)
	owner := testOwner()

	_, err := commitToken(context.Background(), repo, owner, 0, "hash-1", freshToken(), testNow)
	if !errors.Is(err, ErrStoreBusy) {
		t.Fatalf("commitToken = %v, want ErrStoreBusy", err)
	}
	if repo.attempts() != 2 {
		t.Errorf("attempts = %d, want 2", repo.attempts())
	}
}

// A jammed connection must not outlive the commit budget: a flow manager derives
// its join deadline from it.
func TestCommitTokenBoundsAJammedPut(t *testing.T) {
	repo := newScriptedRepo()
	repo.blocks = 1
	owner := testOwner()
	gen, err := repo.Reserve(context.Background(), owner)
	if err != nil {
		t.Fatalf("Reserve: %v", err)
	}

	start := time.Now()
	got, err := commitToken(context.Background(), repo, owner, gen, "hash-1", freshToken(), testNow)
	elapsed := time.Since(start)
	if err != nil {
		t.Fatalf("commitToken: %v", err)
	}
	if got.AccessToken != "fresh" {
		t.Errorf("token = %q", got.AccessToken)
	}
	if elapsed < commitAttempt {
		t.Errorf("the first attempt gave up after %v, want at least %v", elapsed, commitAttempt)
	}
	if budget := 2*commitAttempt + commitBackoff; elapsed > budget {
		t.Errorf("commit took %v, over its %v budget", elapsed, budget)
	}
}

// A commit that starts on a dead context must not spend the backoff on a second
// doomed Put: Cancel and Shutdown join on this.
func TestCommitTokenDoesNotRetryOnACancelledParent(t *testing.T) {
	repo := newScriptedRepo()
	repo.blocks = 2
	owner := testOwner()
	gen, err := repo.Reserve(context.Background(), owner)
	if err != nil {
		t.Fatalf("Reserve: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	start := time.Now()
	_, err = commitToken(ctx, repo, owner, gen, "hash-1", freshToken(), testNow)
	elapsed := time.Since(start)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("commitToken = %v, want context.Canceled", err)
	}
	if repo.attempts() != 1 {
		t.Errorf("attempts = %d, want 1", repo.attempts())
	}
	if elapsed >= commitBackoff {
		t.Errorf("returned after %v, want less than the %v backoff", elapsed, commitBackoff)
	}
}

// The adoption re-read is part of the commit, so a contended store cannot stretch
// it past the budget the join deadline is derived from.
func TestCommitTokenBoundsTheAdoptionRead(t *testing.T) {
	repo := newScriptedRepo(ErrStaleToken)
	repo.getBlocks = true
	owner := testOwner()
	repo.seed(owner, "hash-1", Token{AccessToken: "winner", ExpiresAt: testNow.Add(time.Hour)})

	start := time.Now()
	_, err := commitToken(context.Background(), repo, owner, 0, "hash-1", freshToken(), testNow)
	elapsed := time.Since(start)
	if !errors.Is(err, ErrStaleToken) {
		t.Fatalf("commitToken = %v, want ErrStaleToken", err)
	}
	if elapsed < commitAttempt {
		t.Errorf("the re-read gave up after %v, want at least %v", elapsed, commitAttempt)
	}
	if elapsed > 2*commitAttempt {
		t.Errorf("the re-read ran for %v, over its %v budget", elapsed, commitAttempt)
	}
}
