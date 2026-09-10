package sqlite

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/google/uuid"
)

type SyncConfig struct {
	ClientID       string
	ServerURL      string
	UserEmail      string
	Enabled        bool
	RefreshToken   string
	AuthGeneration int
	// ReauthRequired survives restarts: the cleanup repeats until a sign-in clears it.
	ReauthRequired bool
}

// Manages the sync configuration singleton.
type SyncConfigRepository interface {
	// Creates the row with a new UUID client_id when absent.
	GetOrCreate(ctx context.Context) (*SyncConfig, error)
	// A partial write would zero the columns the caller did not read.
	Update(ctx context.Context, cfg *SyncConfig) error
	// Returns nil when no config exists yet.
	Get(ctx context.Context) (*SyncConfig, error)
}

type SyncConfigRepo struct {
	db *sql.DB
}

func NewSyncConfigRepo(db *sql.DB) SyncConfigRepository {
	return &SyncConfigRepo{db: db}
}

func (r *SyncConfigRepo) GetOrCreate(ctx context.Context) (*SyncConfig, error) {
	const funcName = "SyncConfigRepo.GetOrCreate"

	cfg, err := r.Get(ctx)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", funcName, err)
	}
	if cfg != nil {
		return cfg, nil
	}

	clientID := uuid.New().String()
	query := `INSERT INTO sync_config (id, client_id, server_url, user_email, enabled, refresh_token, auth_generation, reauth_required)
		VALUES (1, ?, '', '', 0, '', 0, 0)`
	_, err = r.db.ExecContext(ctx, query, clientID)
	if err != nil {
		return nil, fmt.Errorf("%s: insert: %w", funcName, err)
	}

	return &SyncConfig{ClientID: clientID}, nil
}

// client_id is immutable.
func (r *SyncConfigRepo) Update(ctx context.Context, cfg *SyncConfig) error {
	const funcName = "SyncConfigRepo.Update"

	query := `UPDATE sync_config SET server_url = ?, user_email = ?, enabled = ?, refresh_token = ?,
		auth_generation = ?, reauth_required = ? WHERE id = 1`

	_, err := r.db.ExecContext(ctx, query, cfg.ServerURL, cfg.UserEmail, boolToInt(cfg.Enabled), cfg.RefreshToken,
		cfg.AuthGeneration, boolToInt(cfg.ReauthRequired))
	if err != nil {
		return fmt.Errorf("%s: %w", funcName, err)
	}

	return nil
}

func (r *SyncConfigRepo) Get(ctx context.Context) (*SyncConfig, error) {
	const funcName = "SyncConfigRepo.Get"

	query := `SELECT client_id, server_url, user_email, enabled, COALESCE(refresh_token, ''), auth_generation, reauth_required
		FROM sync_config WHERE id = 1`

	var (
		cfg     SyncConfig
		enabled int
		reauth  int
	)

	err := r.db.QueryRowContext(ctx, query).Scan(
		&cfg.ClientID,
		&cfg.ServerURL,
		&cfg.UserEmail,
		&enabled,
		&cfg.RefreshToken,
		&cfg.AuthGeneration,
		&reauth,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("%s: %w", funcName, err)
	}

	cfg.Enabled = enabled != 0
	cfg.ReauthRequired = reauth != 0
	return &cfg, nil
}
