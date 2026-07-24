package sqlite

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/google/uuid"
)

// SyncConfig holds sync server configuration.
type SyncConfig struct {
	ClientID     string
	ServerURL    string
	UserEmail    string
	Enabled      bool
	RefreshToken string
}

// SyncConfigRepository manages the sync configuration singleton.
type SyncConfigRepository interface {
	// GetOrCreate returns the existing config or creates one with a new UUID client_id.
	GetOrCreate(ctx context.Context) (*SyncConfig, error)
	// Update persists server_url, user_email and enabled fields.
	Update(ctx context.Context, cfg *SyncConfig) error
	// Get returns the config or nil if not yet created.
	Get(ctx context.Context) (*SyncConfig, error)
}

// SyncConfigRepo implements SyncConfigRepository using SQLite.
type SyncConfigRepo struct {
	db *sql.DB
}

// NewSyncConfigRepo creates a new SyncConfigRepo instance.
func NewSyncConfigRepo(db *sql.DB) SyncConfigRepository {
	return &SyncConfigRepo{db: db}
}

// GetOrCreate returns the existing sync config or creates one with a new UUID client_id.
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
	query := `INSERT INTO sync_config (id, client_id, server_url, user_email, enabled) VALUES (1, ?, '', '', 0)`
	_, err = r.db.ExecContext(ctx, query, clientID)
	if err != nil {
		return nil, fmt.Errorf("%s: insert: %w", funcName, err)
	}

	return &SyncConfig{
		ClientID:  clientID,
		ServerURL: "",
		UserEmail: "",
		Enabled:   false,
	}, nil
}

// Update persists server_url, user_email and enabled fields (client_id is immutable).
func (r *SyncConfigRepo) Update(ctx context.Context, cfg *SyncConfig) error {
	const funcName = "SyncConfigRepo.Update"

	query := `UPDATE sync_config SET server_url = ?, user_email = ?, enabled = ?, refresh_token = ? WHERE id = 1`

	_, err := r.db.ExecContext(ctx, query, cfg.ServerURL, cfg.UserEmail, boolToInt(cfg.Enabled), cfg.RefreshToken)
	if err != nil {
		return fmt.Errorf("%s: %w", funcName, err)
	}

	return nil
}

// Get returns the sync config, or nil if no config has been created yet.
func (r *SyncConfigRepo) Get(ctx context.Context) (*SyncConfig, error) {
	const funcName = "SyncConfigRepo.Get"

	query := `SELECT client_id, server_url, user_email, enabled, COALESCE(refresh_token, '') FROM sync_config WHERE id = 1`

	var (
		cfg     SyncConfig
		enabled int
	)

	err := r.db.QueryRowContext(ctx, query).Scan(
		&cfg.ClientID,
		&cfg.ServerURL,
		&cfg.UserEmail,
		&enabled,
		&cfg.RefreshToken,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("%s: %w", funcName, err)
	}

	cfg.Enabled = enabled != 0
	return &cfg, nil
}
