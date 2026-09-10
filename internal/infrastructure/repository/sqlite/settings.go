package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/tetiva-app/client/internal/domain/usecase/settings"
)

// Generic KV store.
type SettingsRepo struct {
	db *sql.DB
}

func NewSettingsRepo(db *sql.DB) settings.Repository {
	return &SettingsRepo{db: db}
}

func (r *SettingsRepo) Get(ctx context.Context, key string) (string, bool, error) {
	const funcName = "SettingsRepo.Get"
	const q = `SELECT value FROM app_settings WHERE key = ?`
	var v string
	err := DBTXFromContext(ctx, r.db).QueryRowContext(ctx, q, key).Scan(&v)
	if err == sql.ErrNoRows {
		return "", false, nil
	}
	if err != nil {
		return "", false, fmt.Errorf("%s: %w", funcName, err)
	}
	return v, true, nil
}

func (r *SettingsRepo) Set(ctx context.Context, key, value string) error {
	const funcName = "SettingsRepo.Set"
	const q = `
	INSERT INTO app_settings (key, value, updated_at) VALUES (?, ?, ?)
	ON CONFLICT(key) DO UPDATE SET value = excluded.value, updated_at = excluded.updated_at`
	_, err := DBTXFromContext(ctx, r.db).ExecContext(ctx, q, key, value, time.Now().UTC().Format(time.RFC3339))
	if err != nil {
		return fmt.Errorf("%s: %w", funcName, err)
	}
	return nil
}

func (r *SettingsRepo) SetMany(ctx context.Context, values map[string]string) error {
	const funcName = "SettingsRepo.SetMany"
	if len(values) == 0 {
		return nil
	}
	err := WithTx(ctx, r.db, func(ctx context.Context) error {
		for key, value := range values {
			if err := r.Set(ctx, key, value); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("%s: %w", funcName, err)
	}
	return nil
}
