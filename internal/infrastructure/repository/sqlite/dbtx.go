package sqlite

import (
	"context"
	"database/sql"
	"fmt"
)

// DBTX abstracts *sql.DB and *sql.Tx for query execution.
type DBTX interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

type txKey struct{}

// ContextWithTx stores a *sql.Tx in context.
func ContextWithTx(ctx context.Context, tx *sql.Tx) context.Context {
	return context.WithValue(ctx, txKey{}, tx)
}

// DBTXFromContext returns the *sql.Tx from context if present, otherwise returns db.
func DBTXFromContext(ctx context.Context, db *sql.DB) DBTX {
	if tx, ok := ctx.Value(txKey{}).(*sql.Tx); ok {
		return tx
	}
	return db
}

// WithTx executes fn within a transaction. If ctx already has a TX, reuses it (nested).
func WithTx(ctx context.Context, db *sql.DB, fn func(ctx context.Context) error) error {
	// If already in a TX, just call fn (no nested TX in SQLite).
	if _, ok := ctx.Value(txKey{}).(*sql.Tx); ok {
		return fn(ctx)
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}

	txCtx := ContextWithTx(ctx, tx)

	if err := fn(txCtx); err != nil {
		_ = tx.Rollback()
		return err
	}

	return tx.Commit()
}
