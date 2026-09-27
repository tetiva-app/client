package sqlite

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/google/uuid"
)

func TestAfterCommit_OutsideTxRunsImmediately(t *testing.T) {
	calls := 0
	AfterCommit(context.Background(), func() { calls++ })
	if calls != 1 {
		t.Fatalf("calls = %d, want 1", calls)
	}
}

func TestAfterCommit_NestedTxRunsOnceAfterOuterCommit(t *testing.T) {
	db := setupTestDB(t)
	repo := NewResponseExampleRepo(db)
	ctx := testCtx(t)
	ex := newTestExample(uuid.New(), "Committed", 0)

	var order []string
	seen := func(name string) func() {
		return func() {
			// The pool has one connection, so this read only gets through once the tx has released it.
			got, err := repo.GetByID(ctx, ex.ID)
			if err != nil || got == nil {
				t.Errorf("hook %s: committed row not visible: %v, %v", name, got, err)
			}
			order = append(order, name)
		}
	}

	err := WithTx(ctx, db, func(outer context.Context) error {
		AfterCommit(outer, seen("outer"))
		err := WithTx(outer, db, func(inner context.Context) error {
			if err := repo.Create(inner, ex); err != nil {
				return err
			}
			AfterCommit(inner, seen("inner"))
			return nil
		})
		if err != nil {
			return err
		}
		if len(order) != 0 {
			t.Errorf("hooks ran before the outer commit: %v", order)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("WithTx: %v", err)
	}

	if want := []string{"outer", "inner"}; !reflect.DeepEqual(order, want) {
		t.Errorf("hooks = %v, want %v", order, want)
	}
}

func TestAfterCommit_RollbackDropsHooks(t *testing.T) {
	db := setupTestDB(t)
	ctx := testCtx(t)
	errBoom := errors.New("boom")

	calls := 0
	err := WithTx(ctx, db, func(outer context.Context) error {
		AfterCommit(outer, func() { calls++ })
		return WithTx(outer, db, func(inner context.Context) error {
			AfterCommit(inner, func() { calls++ })
			return errBoom
		})
	})
	if !errors.Is(err, errBoom) {
		t.Fatalf("WithTx err = %v, want boom", err)
	}
	if calls != 0 {
		t.Errorf("hooks ran %d times after rollback", calls)
	}
}

func foreignKeys(t *testing.T, db DBTX) int {
	t.Helper()
	var on int
	if err := db.QueryRowContext(context.Background(), "PRAGMA foreign_keys").Scan(&on); err != nil {
		t.Fatalf("read foreign_keys: %v", err)
	}
	return on
}

func TestWithInboundTx_StoresAChildBeforeItsParent(t *testing.T) {
	db := setupTestDB(t)
	ctx := testCtx(t)
	parent := newTestCollection("Parent", nil)
	child := newTestCollection("Child", &parent.ID)
	repo := NewCollectionRepo(db)

	err := WithInboundTx(ctx, db, func(txCtx context.Context) error {
		if got := foreignKeys(t, DBTXFromContext(txCtx, db)); got != 0 {
			t.Errorf("foreign_keys inside = %d, want 0", got)
		}
		if err := repo.Create(txCtx, child); err != nil {
			return err
		}
		return repo.Create(txCtx, parent)
	})
	if err != nil {
		t.Fatalf("WithInboundTx: %v", err)
	}

	if got, _ := repo.GetByID(ctx, child.ID); got == nil {
		t.Error("child collection was not stored")
	}
	if got := foreignKeys(t, db); got != 1 {
		t.Errorf("foreign_keys after = %d, want the pool connection back at 1", got)
	}
}

func TestWithInboundTx_RollsBackAndRunsHooksOnlyAfterCommit(t *testing.T) {
	db := setupTestDB(t)
	ctx := testCtx(t)
	errBoom := errors.New("boom")
	repo := NewCollectionRepo(db)
	dropped := newTestCollection("Dropped", nil)

	calls := 0
	err := WithInboundTx(ctx, db, func(txCtx context.Context) error {
		AfterCommit(txCtx, func() { calls++ })
		if err := repo.Create(txCtx, dropped); err != nil {
			return err
		}
		return errBoom
	})
	if !errors.Is(err, errBoom) {
		t.Fatalf("err = %v, want boom", err)
	}
	if got, _ := repo.GetByID(ctx, dropped.ID); got != nil {
		t.Error("a rolled-back write is visible")
	}
	if calls != 0 {
		t.Errorf("hook ran %d times after a rollback", calls)
	}
	if got := foreignKeys(t, db); got != 1 {
		t.Errorf("foreign_keys after a rollback = %d, want 1", got)
	}

	err = WithInboundTx(ctx, db, func(txCtx context.Context) error {
		AfterCommit(txCtx, func() { calls++ })
		return nil
	})
	if err != nil || calls != 1 {
		t.Fatalf("commit: err = %v, hook calls = %d", err, calls)
	}
}

func TestWithInboundTx_JoinsTheCallersTransaction(t *testing.T) {
	db := setupTestDB(t)
	ctx := testCtx(t)
	repo := NewCollectionRepo(db)
	coll := newTestCollection("Joined", nil)

	err := WithTx(ctx, db, func(outer context.Context) error {
		return WithInboundTx(outer, db, func(inner context.Context) error {
			if DBTXFromContext(inner, db) != DBTXFromContext(outer, db) {
				t.Error("the inbound write left the caller's transaction")
			}
			return repo.Create(inner, coll)
		})
	})
	if err != nil {
		t.Fatalf("WithTx: %v", err)
	}
	if got, _ := repo.GetByID(ctx, coll.ID); got == nil {
		t.Error("the joined write was not committed")
	}
}
