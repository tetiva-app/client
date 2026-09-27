package sqlite

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/google/uuid"

	"github.com/tetiva-app/client/internal/domain"
	"github.com/tetiva-app/client/internal/domain/entities"
	"github.com/tetiva-app/client/internal/domain/usecase/example"
)

// barrierRepo holds every reader until all of them have read, so both saves start from the same version.
type barrierRepo struct {
	*ResponseExampleRepo
	reads *sync.WaitGroup
}

func (r barrierRepo) GetByID(ctx context.Context, id uuid.UUID) (*entities.ResponseExample, error) {
	e, err := r.ResponseExampleRepo.GetByID(ctx, id)
	r.reads.Done()
	r.reads.Wait()
	return e, err
}

type lockingEnv struct {
	*cascadeEnv
	repo *ResponseExampleRepo
	ex   *entities.ResponseExample
}

func newLockingEnv(t *testing.T) *lockingEnv {
	t.Helper()
	env := newCascadeEnv(t)
	list, err := env.examples.ListByRequest(testCtx(t), env.req.ID)
	if err != nil || len(list) == 0 {
		t.Fatalf("list examples: %v (%d)", err, len(list))
	}
	return &lockingEnv{cascadeEnv: env, repo: NewResponseExampleRepo(env.db), ex: list[0]}
}

// racing runs both calls after each has read the example.
func (env *lockingEnv) racing(t *testing.T, a, b func(uc example.Usecase) error) (errA, errB error) {
	t.Helper()
	reads := &sync.WaitGroup{}
	reads.Add(2)
	uc := example.NewUsecase(barrierRepo{ResponseExampleRepo: env.repo, reads: reads}, NewRequestRepo(env.db), env.cols)

	var wg sync.WaitGroup
	wg.Add(2)
	go func() { defer wg.Done(); errA = a(uc) }()
	go func() { defer wg.Done(); errB = b(uc) }()
	wg.Wait()
	return errA, errB
}

func (env *lockingEnv) edit(name string) func(uc example.Usecase) error {
	return func(uc example.Usecase) error {
		_, err := uc.Edit(context.Background(), example.Edit{Name: name, StatusCode: 200, Body: "{}"},
			example.EditOpt{ExampleID: env.ex.ID, UserID: "u", Version: env.ex.Version})
		return err
	}
}

func (env *lockingEnv) remove() func(uc example.Usecase) error {
	return func(uc example.Usecase) error {
		return uc.Delete(context.Background(), example.DeleteOpt{ExampleID: env.ex.ID, UserID: "u", Version: env.ex.Version})
	}
}

func isConflict(err error) bool {
	var ce *domain.ConflictError
	return errors.As(err, &ce)
}

func TestExampleEdit_ConcurrentSavesOfOneVersionConflict(t *testing.T) {
	env := newLockingEnv(t)

	errA, errB := env.racing(t, env.edit("A"), env.edit("B"))

	if (errA == nil) == (errB == nil) {
		t.Fatalf("want exactly one save to win: errA=%v errB=%v", errA, errB)
	}
	loser, winner := errB, "A"
	if errA != nil {
		loser, winner = errA, "B"
	}
	if !isConflict(loser) {
		t.Fatalf("losing save: got %v, want ConflictError", loser)
	}
	got, err := env.repo.GetByID(testCtx(t), env.ex.ID)
	if err != nil || got == nil {
		t.Fatalf("GetByID: %v %v", got, err)
	}
	if got.Name != winner || got.Version != env.ex.Version+1 {
		t.Errorf("stored %q v%d, want %q v%d", got.Name, got.Version, winner, env.ex.Version+1)
	}
}

func TestExampleEdit_RacingDeleteConflicts(t *testing.T) {
	env := newLockingEnv(t)

	editErr, deleteErr := env.racing(t, env.edit("Edited"), env.remove())

	if (editErr == nil) == (deleteErr == nil) {
		t.Fatalf("want exactly one to win: edit=%v delete=%v", editErr, deleteErr)
	}
	var deleted int
	if err := env.db.QueryRow(`SELECT is_delete FROM response_examples WHERE id = ?`, env.ex.ID.String()).Scan(&deleted); err != nil {
		t.Fatal(err)
	}
	if editErr != nil {
		if !isConflict(editErr) || deleted != 1 {
			t.Errorf("delete won: edit err %v, is_delete %d", editErr, deleted)
		}
		return
	}
	if !isConflict(deleteErr) || deleted != 0 {
		t.Errorf("edit won: delete err %v, is_delete %d", deleteErr, deleted)
	}
}

func TestResponseExampleRepo_UpdateAtVersion(t *testing.T) {
	env := newLockingEnv(t)
	ctx := testCtx(t)
	base := env.ex.Version

	stale := *env.ex
	stale.Name, stale.Version = "stale", base+1
	if err := env.repo.UpdateAtVersion(ctx, &stale, base-1); !isConflict(err) {
		t.Fatalf("wrong base version: got %v, want ConflictError", err)
	}

	next := *env.ex
	next.Name, next.Version = "next", base+1
	if err := env.repo.UpdateAtVersion(ctx, &next, base); err != nil {
		t.Fatalf("UpdateAtVersion: %v", err)
	}
	got, _ := env.repo.GetByID(ctx, env.ex.ID)
	if got == nil || got.Name != "next" || got.Version != base+1 {
		t.Fatalf("stored %+v", got)
	}

	gone := next
	gone.IsDelete, gone.Version = true, base+2
	if err := env.repo.UpdateAtVersion(ctx, &gone, base+1); err != nil {
		t.Fatalf("delete at version: %v", err)
	}
	revived := next
	revived.Version = base + 3
	if err := env.repo.UpdateAtVersion(ctx, &revived, base+2); !isConflict(err) {
		t.Errorf("write over a deleted row: got %v, want ConflictError", err)
	}
}
