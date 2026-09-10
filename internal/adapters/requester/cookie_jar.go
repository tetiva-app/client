package requester

import (
	"context"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/tetiva-app/client/internal/domain/entities"
	"github.com/tetiva-app/client/internal/domain/usecase/cookie"
)

// Implemented by an adapter over cookie.Repository (see internal/app/usecases.go).
type CookieStore interface {
	GetCookiesFor(ctx context.Context, workspaceID uuid.UUID, u *url.URL) []*http.Cookie
	SetCookies(ctx context.Context, workspaceID uuid.UUID, u *url.URL, cookies []*http.Cookie) error
}

// Created per-call by HTTPRequester.Execute and discarded after the request.
type workspaceJar struct {
	readCtx     context.Context
	store       CookieStore
	workspaceID uuid.UUID
}

func newWorkspaceJar(readCtx context.Context, store CookieStore, ws uuid.UUID) *workspaceJar {
	return &workspaceJar{readCtx: readCtx, store: store, workspaceID: ws}
}

func (j *workspaceJar) Cookies(u *url.URL) []*http.Cookie {
	if j.store == nil {
		return nil
	}
	return j.store.GetCookiesFor(j.readCtx, j.workspaceID, u)
}

// http.CookieJar has no error channel, so persist on a detached 2s context and
// log failures — cancelling the request must not orphan cookies already sent.
func (j *workspaceJar) SetCookies(u *url.URL, cookies []*http.Cookie) {
	if j.store == nil || len(cookies) == 0 {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := j.store.SetCookies(ctx, j.workspaceID, u, cookies); err != nil {
		slog.Warn("workspaceJar: SetCookies failed",
			"workspace", j.workspaceID, "url", u.String(), "err", err)
	}
}

var _ http.CookieJar = (*workspaceJar)(nil)

type cookieStore struct{ repo cookie.Repository }

// NewCookieStore is the FX provider — implements CookieStore over the cookie repo.
func NewCookieStore(repo cookie.Repository) CookieStore {
	return &cookieStore{repo: repo}
}

func (s *cookieStore) GetCookiesFor(ctx context.Context, ws uuid.UUID, u *url.URL) []*http.Cookie {
	matched, err := s.repo.MatchForRequest(ctx, ws, u)
	if err != nil {
		return nil
	}
	out := make([]*http.Cookie, 0, len(matched))
	for _, c := range matched {
		hc := &http.Cookie{
			Name: c.Name, Value: c.Value,
			// Domain on the wire only when the cookie is not host-only —
			// matches browser behaviour and how curl treats the field.
			Path:     c.Path,
			HttpOnly: c.HTTPOnly, Secure: c.Secure,
		}
		if !c.HostOnly {
			hc.Domain = c.Domain
		}
		if c.ExpiresAt != nil {
			hc.Expires = *c.ExpiresAt
		}
		out = append(out, hc)
	}
	return out
}

// SetCookies persists Set-Cookie headers: empty Domain → host-only; MaxAge < 0 or
// past Expires deletes the row (RFC 6265 §5.3); omitted Path → "/" (not RFC default-path).
func (s *cookieStore) SetCookies(ctx context.Context, ws uuid.UUID, u *url.URL, cookies []*http.Cookie) error {
	now := time.Now().UTC()
	originHost := strings.ToLower(u.Hostname())
	for _, c := range cookies {
		hostOnly := c.Domain == ""
		domain := strings.ToLower(c.Domain)
		if hostOnly {
			domain = originHost
		}

		path := c.Path
		if path == "" {
			path = "/"
		}

		isDelete := c.MaxAge < 0 || (!c.Expires.IsZero() && c.Expires.Before(now))
		if isDelete {
			// Unique key is (workspace_id, domain, path, name); list by domain
			// and filter in-memory — the per-domain set is small.
			existing, err := s.repo.List(ctx, cookie.Filter{WorkspaceID: ws, Domain: domain})
			if err != nil {
				return err
			}
			for _, e := range existing {
				if e.Path == path && e.Name == c.Name {
					if err := s.repo.Delete(ctx, e.ID); err != nil {
						return err
					}
				}
			}
			continue
		}

		ent := &entities.Cookie{
			ID:          uuid.New(),
			WorkspaceID: ws, Domain: domain, HostOnly: hostOnly, Path: path,
			Name: c.Name, Value: c.Value,
			HTTPOnly: c.HttpOnly, Secure: c.Secure,
			SameSite:  sameSiteName(c.SameSite),
			CreatedAt: now, UpdatedAt: now,
		}
		if !c.Expires.IsZero() {
			t := c.Expires
			ent.ExpiresAt = &t
		} else if c.MaxAge > 0 {
			t := now.Add(time.Duration(c.MaxAge) * time.Second)
			ent.ExpiresAt = &t
		}
		if err := s.repo.Upsert(ctx, ent); err != nil {
			return err
		}
	}
	return nil
}

func sameSiteName(s http.SameSite) string {
	switch s {
	case http.SameSiteLaxMode:
		return "Lax"
	case http.SameSiteStrictMode:
		return "Strict"
	case http.SameSiteNoneMode:
		return "None"
	default:
		return ""
	}
}
