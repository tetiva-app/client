package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/tetiva-app/client/internal/domain/entities"
	"github.com/tetiva-app/client/internal/domain/usecase/cookie"
)

type CookieRepo struct {
	db *sql.DB
}

func NewCookieRepo(db *sql.DB) cookie.Repository {
	return &CookieRepo{db: db}
}

func (r *CookieRepo) Upsert(ctx context.Context, c *entities.Cookie) error {
	const funcName = "CookieRepo.Upsert"
	const q = `
INSERT INTO cookies (id, workspace_id, domain, host_only, path, name, value, expires_at,
                     http_only, secure, same_site, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(workspace_id, domain, path, name) DO UPDATE SET
    value      = excluded.value,
    host_only  = excluded.host_only,
    expires_at = excluded.expires_at,
    http_only  = excluded.http_only,
    secure     = excluded.secure,
    same_site  = excluded.same_site,
    updated_at = excluded.updated_at
`
	var expires sql.NullInt64
	if c.ExpiresAt != nil {
		expires = sql.NullInt64{Int64: c.ExpiresAt.Unix(), Valid: true}
	}
	_, err := DBTXFromContext(ctx, r.db).ExecContext(ctx, q,
		c.ID.String(), c.WorkspaceID.String(),
		strings.ToLower(c.Domain), boolToInt(c.HostOnly),
		c.Path, c.Name, c.Value, expires,
		boolToInt(c.HTTPOnly), boolToInt(c.Secure), c.SameSite,
		c.CreatedAt.Unix(), c.UpdatedAt.Unix(),
	)
	if err != nil {
		return fmt.Errorf("%s: %w", funcName, err)
	}
	return nil
}

func (r *CookieRepo) GetByID(ctx context.Context, id uuid.UUID) (*entities.Cookie, error) {
	const funcName = "CookieRepo.GetByID"
	const q = `SELECT id, workspace_id, domain, host_only, path, name, value, expires_at,
                       http_only, secure, same_site, created_at, updated_at
               FROM cookies WHERE id = ?`
	row := DBTXFromContext(ctx, r.db).QueryRowContext(ctx, q, id.String())
	c, err := scanCookie(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("%s: %w", funcName, err)
	}
	return c, nil
}

func (r *CookieRepo) List(ctx context.Context, f cookie.Filter) ([]*entities.Cookie, error) {
	const funcName = "CookieRepo.List"
	q := `SELECT id, workspace_id, domain, host_only, path, name, value, expires_at,
                  http_only, secure, same_site, created_at, updated_at
          FROM cookies WHERE workspace_id = ?`
	args := []any{f.WorkspaceID.String()}
	if f.Domain != "" {
		q += ` AND domain = ?`
		args = append(args, strings.ToLower(f.Domain))
	}
	q += ` ORDER BY domain, path, name`

	rows, err := DBTXFromContext(ctx, r.db).QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", funcName, err)
	}
	defer func() { _ = rows.Close() }()

	var out []*entities.Cookie
	for rows.Next() {
		c, err := scanCookie(rows)
		if err != nil {
			return nil, fmt.Errorf("%s scan: %w", funcName, err)
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func (r *CookieRepo) Delete(ctx context.Context, id uuid.UUID) error {
	const funcName = "CookieRepo.Delete"
	if _, err := DBTXFromContext(ctx, r.db).ExecContext(ctx, `DELETE FROM cookies WHERE id = ?`, id.String()); err != nil {
		return fmt.Errorf("%s: %w", funcName, err)
	}
	return nil
}

func (r *CookieRepo) DeleteByDomain(ctx context.Context, workspaceID uuid.UUID, domain string) (int, error) {
	const funcName = "CookieRepo.DeleteByDomain"
	res, err := DBTXFromContext(ctx, r.db).ExecContext(ctx,
		`DELETE FROM cookies WHERE workspace_id = ? AND domain = ?`,
		workspaceID.String(), strings.ToLower(domain),
	)
	if err != nil {
		return 0, fmt.Errorf("%s: %w", funcName, err)
	}
	n, _ := res.RowsAffected()
	return int(n), nil
}

func (r *CookieRepo) Clear(ctx context.Context, workspaceID uuid.UUID) (int, error) {
	const funcName = "CookieRepo.Clear"
	res, err := DBTXFromContext(ctx, r.db).ExecContext(ctx, `DELETE FROM cookies WHERE workspace_id = ?`, workspaceID.String())
	if err != nil {
		return 0, fmt.Errorf("%s: %w", funcName, err)
	}
	n, _ := res.RowsAffected()
	return int(n), nil
}

// Non-expired cookies only, matched per RFC 6265 domain, path and secure rules.
func (r *CookieRepo) MatchForRequest(ctx context.Context, workspaceID uuid.UUID, u *url.URL) ([]*entities.Cookie, error) {
	const funcName = "CookieRepo.MatchForRequest"
	const q = `SELECT id, workspace_id, domain, host_only, path, name, value, expires_at,
                       http_only, secure, same_site, created_at, updated_at
               FROM cookies
               WHERE workspace_id = ?
                 AND (expires_at IS NULL OR expires_at > ?)`
	rows, err := DBTXFromContext(ctx, r.db).QueryContext(ctx, q, workspaceID.String(), time.Now().Unix())
	if err != nil {
		return nil, fmt.Errorf("%s: %w", funcName, err)
	}
	defer func() { _ = rows.Close() }()

	host := strings.ToLower(u.Hostname())
	urlPath := u.Path
	if urlPath == "" {
		urlPath = "/"
	}
	isHTTPS := u.Scheme == "https"

	var out []*entities.Cookie
	for rows.Next() {
		c, err := scanCookie(rows)
		if err != nil {
			return nil, fmt.Errorf("%s scan: %w", funcName, err)
		}
		if !domainMatches(c.Domain, c.HostOnly, host) {
			continue
		}
		if !pathMatches(c.Path, urlPath) {
			continue
		}
		if c.Secure && !isHTTPS {
			continue
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// RFC 6265 §5.1.3: host-only cookies match their origin host, others also match subdomains.
func domainMatches(stored string, hostOnly bool, host string) bool {
	stored = strings.ToLower(stored)
	if hostOnly {
		return host == stored
	}
	return host == stored || strings.HasSuffix(host, "."+stored)
}

// RFC 6265 §5.1.4 path-match.
func pathMatches(stored, urlPath string) bool {
	if stored == "" || stored == "/" {
		return true
	}
	if urlPath == stored {
		return true
	}
	if strings.HasPrefix(urlPath, stored) {
		// /api matches /api/x but not /apixyz
		next := urlPath[len(stored):]
		return next[0] == '/' || strings.HasSuffix(stored, "/")
	}
	return false
}

func scanCookie(s interface{ Scan(...any) error }) (*entities.Cookie, error) {
	var (
		idStr, wsStr, domain, path, name, value, sameSite string
		expires                                           sql.NullInt64
		hostOnly, httpOnly, secure                        int
		createdAt, updatedAt                              int64
	)
	if err := s.Scan(&idStr, &wsStr, &domain, &hostOnly, &path, &name, &value, &expires,
		&httpOnly, &secure, &sameSite, &createdAt, &updatedAt); err != nil {
		return nil, err
	}
	id, _ := uuid.Parse(idStr)
	ws, _ := uuid.Parse(wsStr)
	c := &entities.Cookie{
		ID: id, WorkspaceID: ws,
		Domain: domain, HostOnly: hostOnly == 1,
		Path: path, Name: name, Value: value,
		HTTPOnly: httpOnly == 1, Secure: secure == 1, SameSite: sameSite,
		CreatedAt: time.Unix(createdAt, 0).UTC(),
		UpdatedAt: time.Unix(updatedAt, 0).UTC(),
	}
	if expires.Valid {
		t := time.Unix(expires.Int64, 0).UTC()
		c.ExpiresAt = &t
	}
	return c, nil
}
