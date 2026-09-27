package sqlite

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"fmt"
)

type DBTX interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

type txKey struct{}

type afterCommitKey struct{}

func ContextWithTx(ctx context.Context, tx *sql.Tx) context.Context {
	return context.WithValue(ctx, txKey{}, tx)
}

// Falls back to db when ctx carries no transaction.
func DBTXFromContext(ctx context.Context, db *sql.DB) DBTX {
	if tx, ok := ctx.Value(txKey{}).(*sql.Tx); ok {
		return tx
	}
	return db
}

// If ctx already has a TX, reuses it — SQLite has no nested transactions.
func WithTx(ctx context.Context, db *sql.DB, fn func(ctx context.Context) error) error {
	if _, ok := ctx.Value(txKey{}).(*sql.Tx); ok {
		return fn(ctx)
	}
	return runTx(ctx, db, fn)
}

// WithInboundTx turns foreign keys off: a sync page may carry a child before its parent.
func WithInboundTx(ctx context.Context, db *sql.DB, fn func(ctx context.Context) error) error {
	if _, ok := ctx.Value(txKey{}).(*sql.Tx); ok {
		return fn(ctx)
	}

	conn, err := db.Conn(ctx)
	if err != nil {
		return fmt.Errorf("inbound conn: %w", err)
	}
	defer func() { _ = conn.Close() }()

	var fkWas int
	if err := conn.QueryRowContext(ctx, "PRAGMA foreign_keys").Scan(&fkWas); err != nil {
		return fmt.Errorf("read foreign_keys: %w", err)
	}
	if fkWas != 0 {
		if _, err := conn.ExecContext(ctx, "PRAGMA foreign_keys=OFF"); err != nil {
			return fmt.Errorf("disable foreign_keys: %w", err)
		}
		defer restoreForeignKeys(conn)
	}
	return runTx(ctx, conn, fn)
}

// restoreForeignKeys discards a connection it cannot restore rather than pool it.
func restoreForeignKeys(conn *sql.Conn) {
	if _, err := conn.ExecContext(context.Background(), "PRAGMA foreign_keys=ON"); err != nil {
		_ = conn.Raw(func(any) error { return driver.ErrBadConn })
	}
}

type txBeginner interface {
	BeginTx(ctx context.Context, opts *sql.TxOptions) (*sql.Tx, error)
}

func runTx(ctx context.Context, db txBeginner, fn func(ctx context.Context) error) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}

	hooks := new([]func())
	txCtx := context.WithValue(ContextWithTx(ctx, tx), afterCommitKey{}, hooks)

	if err := fn(txCtx); err != nil {
		_ = tx.Rollback()
		return err
	}

	if err := tx.Commit(); err != nil {
		return err
	}
	for _, hook := range *hooks {
		hook()
	}
	return nil
}

// Inside a TX fn waits for the outermost commit and is dropped on rollback.
func AfterCommit(ctx context.Context, fn func()) {
	if hooks, ok := ctx.Value(afterCommitKey{}).(*[]func()); ok {
		*hooks = append(*hooks, fn)
		return
	}
	fn()
}
