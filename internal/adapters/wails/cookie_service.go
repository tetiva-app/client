package wails

import (
	"context"
	"net/http"
	"time"

	"github.com/google/uuid"

	"github.com/tetiva-app/client/internal/adapters/wails/dto"
	"github.com/tetiva-app/client/internal/domain"
	"github.com/tetiva-app/client/internal/domain/usecase/cookie"
	"github.com/tetiva-app/client/internal/domain/usecase/request"
)

// CookieService exposes the per-workspace cookie jar to the Wails frontend.
type CookieService struct {
	uc     cookie.Usecase
	reader request.CookieReader
}

func NewCookieService(uc cookie.Usecase, reader request.CookieReader) *CookieService {
	return &CookieService{uc: uc, reader: reader}
}

// Ordered by domain/path/name.
func (s *CookieService) List(workspaceIDStr string) Result[[]dto.CookieResponse] {
	ctx := context.Background()
	ws, err := uuid.Parse(workspaceIDStr)
	if err != nil {
		return Err[[]dto.CookieResponse](&domain.ValidationError{
			Fields: map[string]string{"workspaceId": "invalid UUID"},
		})
	}
	cookies, err := s.uc.List(ctx, cookie.ListOpt{WorkspaceID: ws})
	if err != nil {
		return Err[[]dto.CookieResponse](err)
	}
	return OK(dto.CookiesToResponse(cookies))
}

// Used by the response-viewer Cookies tab to render the "Sent" section.
func (s *CookieService) GetForURL(workspaceIDStr, rawURL string) Result[[]dto.CookieResponse] {
	ctx := context.Background()
	ws, err := uuid.Parse(workspaceIDStr)
	if err != nil {
		return Err[[]dto.CookieResponse](&domain.ValidationError{
			Fields: map[string]string{"workspaceId": "invalid UUID"},
		})
	}

	cookies := s.reader.CookiesFor(ctx, ws, rawURL)
	out := make([]dto.CookieResponse, 0, len(cookies))
	for _, c := range cookies {
		out = append(out, dto.CookieResponse{
			Domain:   c.Domain,
			Path:     c.Path,
			Name:     c.Name,
			Value:    c.Value,
			HTTPOnly: c.HttpOnly,
			Secure:   c.Secure,
			SameSite: sameSiteString(c.SameSite),
		})
	}
	return OK(out)
}

func (s *CookieService) Add(req dto.AddCookieRequest) Result[dto.CookieResponse] {
	ctx := context.Background()
	ws, err := uuid.Parse(req.WorkspaceID)
	if err != nil {
		return Err[dto.CookieResponse](&domain.ValidationError{
			Fields: map[string]string{"workspaceId": "invalid UUID"},
		})
	}
	input := cookie.Add{
		WorkspaceID: ws,
		Domain:      req.Domain, HostOnly: req.HostOnly, Path: req.Path,
		Name: req.Name, Value: req.Value,
		HTTPOnly: req.HTTPOnly, Secure: req.Secure, SameSite: req.SameSite,
	}
	if req.ExpiresAt != nil {
		t := time.Unix(*req.ExpiresAt, 0).UTC()
		input.ExpiresAt = &t
	}
	c, err := s.uc.Add(ctx, input, cookie.Opt{})
	if err != nil {
		return Err[dto.CookieResponse](err)
	}
	return OK(dto.CookieToResponse(c))
}

// PUT semantics: the UI must send all fields.
func (s *CookieService) Edit(req dto.EditCookieRequest) Result[dto.CookieResponse] {
	ctx := context.Background()
	id, err := uuid.Parse(req.ID)
	if err != nil {
		return Err[dto.CookieResponse](&domain.ValidationError{
			Fields: map[string]string{"id": "invalid UUID"},
		})
	}
	input := cookie.Edit{
		ID:     id,
		Domain: req.Domain, HostOnly: req.HostOnly, Path: req.Path,
		Name: req.Name, Value: req.Value,
		HTTPOnly: req.HTTPOnly, Secure: req.Secure, SameSite: req.SameSite,
	}
	if req.ExpiresAt != nil {
		t := time.Unix(*req.ExpiresAt, 0).UTC()
		input.ExpiresAt = &t
	}
	c, err := s.uc.Edit(ctx, input, cookie.Opt{})
	if err != nil {
		return Err[dto.CookieResponse](err)
	}
	return OK(dto.CookieToResponse(c))
}

func (s *CookieService) Delete(idStr string) Result[struct{}] {
	ctx := context.Background()
	id, err := uuid.Parse(idStr)
	if err != nil {
		return Err[struct{}](&domain.ValidationError{
			Fields: map[string]string{"id": "invalid UUID"},
		})
	}
	if err := s.uc.Delete(ctx, cookie.DeleteOpt{ID: id}); err != nil {
		return Err[struct{}](err)
	}
	return OK(struct{}{})
}

func (s *CookieService) DeleteByDomain(workspaceIDStr, domainStr string) Result[int] {
	ctx := context.Background()
	ws, err := uuid.Parse(workspaceIDStr)
	if err != nil {
		return Err[int](&domain.ValidationError{
			Fields: map[string]string{"workspaceId": "invalid UUID"},
		})
	}
	n, err := s.uc.DeleteByDomain(ctx, ws, domainStr)
	if err != nil {
		return Err[int](err)
	}
	return OK(n)
}

func (s *CookieService) Clear(workspaceIDStr string) Result[int] {
	ctx := context.Background()
	ws, err := uuid.Parse(workspaceIDStr)
	if err != nil {
		return Err[int](&domain.ValidationError{
			Fields: map[string]string{"workspaceId": "invalid UUID"},
		})
	}
	n, err := s.uc.Clear(ctx, ws)
	if err != nil {
		return Err[int](err)
	}
	return OK(n)
}

func sameSiteString(s http.SameSite) string {
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
