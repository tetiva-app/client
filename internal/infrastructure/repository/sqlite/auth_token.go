package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"math/rand/v2"
	"time"

	"github.com/google/uuid"
	sqlitedrv "modernc.org/sqlite"
	sqlite3lib "modernc.org/sqlite/lib"

	"github.com/tetiva-app/client/internal/domain/entities"
	"github.com/tetiva-app/client/internal/domain/usecase/auth"
)

// Caps the ancestor walk of the orphan sweep, mirroring the script and auth resolvers.
const maxCollectionDepth = 50

// Every collection that is soft-deleted or sits under one. UNION (not UNION ALL)
// plus the depth cap stops a cyclic parent_id chain — only a broken sync payload makes one.
const deadCollectionsCTE = `WITH RECURSIVE dead(id, depth) AS (
		SELECT id, 0 FROM collections WHERE is_delete = 1
		UNION
		SELECT c.id, d.depth + 1 FROM collections c JOIN dead d ON c.parent_id = d.id WHERE d.depth < ?
	)`

// Resets every secret-bearing column; used by the clear and sweep paths.
const tokenColumnsBlank = `config_hash = '', access_token = '', refresh_token = '', token_type = '',
		scope = '', id_token = '', expires_at = ''`

// Matches a row carrying nothing: a fresh Reserve row, or one a tombstone already blanked.
const tokenRowBlank = `(access_token = '' AND refresh_token = '' AND id_token = '' AND config_hash = '')`

// Owners that still exist but no longer qualify: moved workspace, soft-deleted, under a dead
// collection, or not oauth2. Needs the dead CTE; placeholders: collection kind, oauth2, request kind, oauth2.
const disqualifiedOwnerSQL = `((owner_kind = ? AND EXISTS (
			SELECT 1 FROM collections c WHERE c.id = auth_tokens.owner_id
			  AND (c.workspace_id <> auth_tokens.workspace_id
				OR c.auth_type <> ?
				OR c.id IN (SELECT id FROM dead))))
		OR (owner_kind = ? AND EXISTS (
			SELECT 1 FROM requests r LEFT JOIN collections c ON c.id = r.collection_id
			WHERE r.id = auth_tokens.owner_id
			  AND (r.is_delete = 1
				OR c.id IS NULL
				OR c.workspace_id <> auth_tokens.workspace_id
				OR r.auth_type <> ?
				OR r.collection_id IN (SELECT id FROM dead)))))`

// OAuth 2.0 tokens live only here: the table never syncs or exports, and rows are
// physically deleted — the documented exception to the soft-delete rule.
type AuthTokenRepo struct {
	db *sql.DB
}

var _ auth.TokenRepository = (*AuthTokenRepo)(nil)

func NewAuthTokenRepo(db *sql.DB) *AuthTokenRepo {
	return &AuthTokenRepo{db: db}
}

// Returns nil when there is no row or only a tombstone.
func (r *AuthTokenRepo) Get(ctx context.Context, owner entities.AuthOwner) (*auth.StoredToken, error) {
	const funcName = "AuthTokenRepo.Get"

	var (
		st                  auth.StoredToken
		expiresAt, obtained string
	)
	err := DBTXFromContext(ctx, r.db).QueryRowContext(ctx,
		`SELECT config_hash, access_token, refresh_token, token_type, scope, id_token,
			expires_at, obtained_at, generation
		FROM auth_tokens WHERE workspace_id = ? AND owner_kind = ? AND owner_id = ?`,
		owner.WorkspaceID.String(), owner.Kind, owner.ID.String(),
	).Scan(&st.ConfigHash, &st.AccessToken, &st.RefreshToken, &st.TokenType, &st.Scope,
		&st.IDToken, &expiresAt, &obtained, &st.Generation)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("%s: %w", funcName, err)
	}
	if st.AccessToken == "" {
		return nil, nil
	}

	if st.ExpiresAt, err = parseAuthTime(expiresAt); err != nil {
		return nil, fmt.Errorf("%s: parse expires_at: %w", funcName, err)
	}
	if st.ObtainedAt, err = parseAuthTime(obtained); err != nil {
		return nil, fmt.Errorf("%s: parse obtained_at: %w", funcName, err)
	}

	return &st, nil
}

// Returns the generation the following Put must quote; called before any network I/O.
func (r *AuthTokenRepo) Reserve(ctx context.Context, owner entities.AuthOwner) (int64, error) {
	const funcName = "AuthTokenRepo.Reserve"

	live, err := ownerLiveSQL(owner.Kind)
	if err != nil {
		return 0, fmt.Errorf("%s: %w", funcName, err)
	}
	db := DBTXFromContext(ctx, r.db)
	now := formatAuthTime(time.Now())

	// The insert leaves an existing row (and its token) untouched.
	_, err = db.ExecContext(ctx,
		`INSERT INTO auth_tokens (workspace_id, owner_kind, owner_id, config_hash,
			access_token, obtained_at, updated_at, generation)
		SELECT ?, ?, ?, '', '', ?, ?, ? WHERE EXISTS (`+live+`)
		ON CONFLICT (workspace_id, owner_kind, owner_id) DO NOTHING`,
		owner.WorkspaceID.String(), owner.Kind, owner.ID.String(), now, now, newGeneration(),
		owner.ID.String(), owner.WorkspaceID.String())
	if err != nil {
		return 0, fmt.Errorf("%s: %w", funcName, err)
	}

	var generation int64
	err = db.QueryRowContext(ctx,
		`SELECT generation FROM auth_tokens
		WHERE workspace_id = ? AND owner_kind = ? AND owner_id = ? AND EXISTS (`+live+`)`,
		owner.WorkspaceID.String(), owner.Kind, owner.ID.String(),
		owner.ID.String(), owner.WorkspaceID.String()).Scan(&generation)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, auth.ErrOwnerGone
	}
	if err != nil {
		return 0, fmt.Errorf("%s: %w", funcName, err)
	}

	return generation, nil
}

// Refuses a write whose generation moved, advances it on success: Reserve is no
// exclusive lease, so only this compare-and-swap keeps the losing writer out.
func (r *AuthTokenRepo) Put(ctx context.Context, owner entities.AuthOwner, expectedGeneration int64, hash string, t *auth.Token) error {
	const funcName = "AuthTokenRepo.Put"

	if t == nil {
		return fmt.Errorf("%s: nil token", funcName)
	}

	res, err := DBTXFromContext(ctx, r.db).ExecContext(ctx,
		`UPDATE auth_tokens SET config_hash = ?, access_token = ?, refresh_token = ?,
			token_type = ?, scope = ?, id_token = ?, expires_at = ?, obtained_at = ?, updated_at = ?,
			generation = generation + 1
		WHERE workspace_id = ? AND owner_kind = ? AND owner_id = ? AND generation = ?`,
		hash, t.AccessToken, t.RefreshToken, t.TokenType, t.Scope, t.IDToken,
		formatAuthTime(t.ExpiresAt), formatAuthTime(t.ObtainedAt), formatAuthTime(time.Now()),
		owner.WorkspaceID.String(), owner.Kind, owner.ID.String(), expectedGeneration)
	if err != nil {
		if isStoreBusy(err) {
			return fmt.Errorf("%s: %w", funcName, auth.ErrStoreBusy)
		}

		return fmt.Errorf("%s: %w", funcName, err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("%s: rows affected: %w", funcName, err)
	}
	if n == 0 {
		return auth.ErrStaleToken
	}

	return nil
}

// Creates the row when none existed, so a first acquisition in flight cannot write back.
func (r *AuthTokenRepo) Clear(ctx context.Context, owner entities.AuthOwner) error {
	const funcName = "AuthTokenRepo.Clear"

	now := formatAuthTime(time.Now())
	_, err := DBTXFromContext(ctx, r.db).ExecContext(ctx,
		`INSERT INTO auth_tokens (workspace_id, owner_kind, owner_id, config_hash,
			access_token, obtained_at, updated_at, generation)
		VALUES (?, ?, ?, '', '', ?, ?, ?)
		ON CONFLICT (workspace_id, owner_kind, owner_id) DO UPDATE SET
			`+tokenColumnsBlank+`, updated_at = excluded.updated_at,
			generation = auth_tokens.generation + 1`,
		owner.WorkspaceID.String(), owner.Kind, owner.ID.String(), now, now, newGeneration())
	if err != nil {
		return fmt.Errorf("%s: %w", funcName, err)
	}

	return nil
}

// Hits every workspace: the owner's own is not at hand for a deleted or hard-dropped request.
func (r *AuthTokenRepo) ClearOwners(ctx context.Context, kind string, ids []uuid.UUID) error {
	const funcName = "AuthTokenRepo.ClearOwners"

	if len(ids) == 0 {
		return nil
	}

	args := make([]any, 0, len(ids)+2)
	args = append(args, formatAuthTime(time.Now()), kind)
	for _, id := range ids {
		args = append(args, id.String())
	}

	query := `UPDATE auth_tokens SET ` + tokenColumnsBlank + `, updated_at = ?,
			generation = generation + 1
		WHERE owner_kind = ? AND owner_id IN (` + placeholders(len(ids)) + `)`
	if _, err := DBTXFromContext(ctx, r.db).ExecContext(ctx, query, args...); err != nil {
		return fmt.Errorf("%s: %w", funcName, err)
	}

	return nil
}

// Spares rows that still hold the token keepHash names. A reserved but empty row is
// never spared: an acquisition in flight for the old configuration must still be refused.
func (r *AuthTokenRepo) ClearOwnersUnlessHash(ctx context.Context, kind string, ids []uuid.UUID, keepHash string) error {
	const funcName = "AuthTokenRepo.ClearOwnersUnlessHash"

	if len(ids) == 0 {
		return nil
	}

	args := make([]any, 0, len(ids)+3)
	args = append(args, formatAuthTime(time.Now()), kind)
	for _, id := range ids {
		args = append(args, id.String())
	}
	args = append(args, keepHash)

	query := `UPDATE auth_tokens SET ` + tokenColumnsBlank + `, updated_at = ?,
			generation = generation + 1
		WHERE owner_kind = ? AND owner_id IN (` + placeholders(len(ids)) + `)
		  AND NOT (access_token <> '' AND config_hash = ?)`
	if _, err := DBTXFromContext(ctx, r.db).ExecContext(ctx, query, args...); err != nil {
		return fmt.Errorf("%s: %w", funcName, err)
	}

	return nil
}

// Rows whose owner is gone or no longer qualifies. Secrets are blanked first, then the
// row goes, so a Put from an earlier acquisition updates nothing and reports ErrStaleToken.
func (r *AuthTokenRepo) DeleteOrphans(ctx context.Context) (int, error) {
	const funcName = "AuthTokenRepo.DeleteOrphans"

	db := DBTXFromContext(ctx, r.db)

	// Rows that carry nothing are skipped here — they hold no secret and the
	// delete below takes them, so repeated sweeps do not churn writes.
	_, err := db.ExecContext(ctx,
		deadCollectionsCTE+`
		UPDATE auth_tokens SET `+tokenColumnsBlank+`, updated_at = ?,
			generation = generation + 1
		WHERE NOT `+tokenRowBlank+` AND `+disqualifiedOwnerSQL,
		maxCollectionDepth, formatAuthTime(time.Now()),
		entities.AuthOwnerKindCollection, entities.AuthTypeOAuth2,
		entities.AuthOwnerKindRequest, entities.AuthTypeOAuth2)
	if err != nil {
		return 0, fmt.Errorf("%s: blank: %w", funcName, err)
	}

	deleted, err := db.ExecContext(ctx,
		deadCollectionsCTE+`
		DELETE FROM auth_tokens
		WHERE (owner_kind = ? AND NOT EXISTS (SELECT 1 FROM requests r WHERE r.id = auth_tokens.owner_id))
		   OR (owner_kind = ? AND NOT EXISTS (SELECT 1 FROM collections c WHERE c.id = auth_tokens.owner_id))
		   OR (`+tokenRowBlank+` AND `+disqualifiedOwnerSQL+`)`,
		maxCollectionDepth,
		entities.AuthOwnerKindRequest, entities.AuthOwnerKindCollection,
		entities.AuthOwnerKindCollection, entities.AuthTypeOAuth2,
		entities.AuthOwnerKindRequest, entities.AuthTypeOAuth2)
	if err != nil {
		return 0, fmt.Errorf("%s: delete: %w", funcName, err)
	}

	n, err := deleted.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("%s: rows affected: %w", funcName, err)
	}

	return int(n), nil
}

// 0 when the owner has no row.
func (r *AuthTokenRepo) Generation(ctx context.Context, owner entities.AuthOwner) (int64, error) {
	const funcName = "AuthTokenRepo.Generation"

	var generation int64
	err := DBTXFromContext(ctx, r.db).QueryRowContext(ctx,
		`SELECT generation FROM auth_tokens WHERE workspace_id = ? AND owner_kind = ? AND owner_id = ?`,
		owner.WorkspaceID.String(), owner.Kind, owner.ID.String()).Scan(&generation)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("%s: %w", funcName, err)
	}

	return generation, nil
}

// Probes "owner exists in this workspace and is not deleted"; args are owner id, then workspace id.
func ownerLiveSQL(kind string) (string, error) {
	switch kind {
	case entities.AuthOwnerKindRequest:
		return `SELECT 1 FROM requests r JOIN collections c ON c.id = r.collection_id
			WHERE r.id = ? AND r.is_delete = 0 AND c.is_delete = 0 AND c.workspace_id = ?`, nil
	case entities.AuthOwnerKindCollection:
		return `SELECT 1 FROM collections c
			WHERE c.id = ? AND c.is_delete = 0 AND c.workspace_id = ?`, nil
	default:
		return "", fmt.Errorf("unknown owner kind %q", kind)
	}
}

// The sweep can delete an owner's row, so the same owner may get a second one; a fixed
// seed would reissue a generation an older acquisition still holds, and its Put would resurrect the token.
func newGeneration() int64 {
	return rand.Int64()
}

// The primary code is masked out: WAL mode reports extended ones such as SQLITE_BUSY_SNAPSHOT.
func isStoreBusy(err error) bool {
	var driverErr *sqlitedrv.Error
	if !errors.As(err, &driverErr) {
		return false
	}
	primary := driverErr.Code() & 0xff

	return primary == sqlite3lib.SQLITE_BUSY || primary == sqlite3lib.SQLITE_LOCKED
}

func formatAuthTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Format(time.RFC3339)
}

func parseAuthTime(v string) (time.Time, error) {
	if v == "" {
		return time.Time{}, nil
	}
	return time.Parse(time.RFC3339, v)
}
