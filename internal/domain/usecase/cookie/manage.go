package cookie

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/tetiva-app/client/internal/domain"
	"github.com/tetiva-app/client/internal/domain/entities"
)

type Add struct {
	WorkspaceID uuid.UUID
	Domain      string
	HostOnly    bool
	Path        string
	Name        string
	Value       string
	ExpiresAt   *time.Time
	HTTPOnly    bool
	Secure      bool
	SameSite    string
}

// Edit has replace semantics: every field overwrites the stored value, WorkspaceID stays.
type Edit struct {
	ID        uuid.UUID
	Domain    string
	HostOnly  bool
	Path      string
	Name      string
	Value     string
	ExpiresAt *time.Time
	HTTPOnly  bool
	Secure    bool
	SameSite  string
}

func (u *usecase) Add(ctx context.Context, input Add, _ Opt) (*entities.Cookie, error) {
	const funcName = "cookie.Add"

	if err := validateAdd(&input); err != nil {
		return nil, err
	}

	now := time.Now().UTC()
	c := &entities.Cookie{
		ID:          uuid.New(),
		WorkspaceID: input.WorkspaceID,
		Domain:      strings.ToLower(strings.TrimSpace(input.Domain)),
		HostOnly:    input.HostOnly,
		Path:        defaultPath(input.Path),
		Name:        input.Name,
		Value:       input.Value,
		ExpiresAt:   input.ExpiresAt,
		HTTPOnly:    input.HTTPOnly,
		Secure:      input.Secure,
		SameSite:    input.SameSite,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	if err := u.repo.Upsert(ctx, c); err != nil {
		return nil, fmt.Errorf("%s: %w", funcName, err)
	}
	return c, nil
}

func (u *usecase) Edit(ctx context.Context, input Edit, _ Opt) (*entities.Cookie, error) {
	const funcName = "cookie.Edit"

	if input.ID == uuid.Nil {
		return nil, &domain.ValidationError{Fields: map[string]string{"id": "required"}}
	}
	existing, err := u.repo.GetByID(ctx, input.ID)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", funcName, err)
	}
	if existing == nil {
		return nil, &domain.NotFoundError{Entity: "cookie", ID: input.ID.String()}
	}

	addInput := Add{
		WorkspaceID: existing.WorkspaceID,
		Domain:      input.Domain, HostOnly: input.HostOnly, Path: input.Path,
		Name: input.Name, Value: input.Value,
		ExpiresAt: input.ExpiresAt, HTTPOnly: input.HTTPOnly,
		Secure: input.Secure, SameSite: input.SameSite,
	}
	if err := validateAdd(&addInput); err != nil {
		return nil, err
	}

	existing.Domain = strings.ToLower(strings.TrimSpace(input.Domain))
	existing.HostOnly = input.HostOnly
	existing.Path = defaultPath(input.Path)
	existing.Name = input.Name
	existing.Value = input.Value
	existing.ExpiresAt = input.ExpiresAt
	existing.HTTPOnly = input.HTTPOnly
	existing.Secure = input.Secure
	existing.SameSite = input.SameSite
	existing.UpdatedAt = time.Now().UTC()

	if err := u.repo.Upsert(ctx, existing); err != nil {
		return nil, fmt.Errorf("%s: %w", funcName, err)
	}
	return existing, nil
}

func validateAdd(in *Add) error {
	fields := map[string]string{}
	if in.WorkspaceID == uuid.Nil {
		fields["workspaceId"] = "required"
	}
	if strings.TrimSpace(in.Name) == "" {
		fields["name"] = "required"
	}
	if strings.TrimSpace(in.Domain) == "" {
		fields["domain"] = "required"
	}
	if in.SameSite != "" && in.SameSite != "Lax" && in.SameSite != "Strict" && in.SameSite != "None" {
		fields["sameSite"] = "must be one of: Lax, Strict, None"
	}
	if len(fields) > 0 {
		return &domain.ValidationError{Fields: fields}
	}
	return nil
}

func defaultPath(p string) string {
	p = strings.TrimSpace(p)
	if p == "" {
		return "/"
	}
	return p
}
