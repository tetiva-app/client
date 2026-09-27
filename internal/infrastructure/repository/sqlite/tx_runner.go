package sqlite

import (
	"context"
	"database/sql"
)

type TxRunner struct {
	db *sql.DB
}

func NewTxRunner(db *sql.DB) *TxRunner {
	return &TxRunner{db: db}
}

func (t *TxRunner) Run(ctx context.Context, fn func(ctx context.Context) error) error {
	return WithTx(ctx, t.db, fn)
}
