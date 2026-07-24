package collection

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/tetiva-app/client/internal/domain"
	"github.com/tetiva-app/client/internal/domain/entities"
)

// Edit holds the data required to edit an existing collection.
type Edit struct {
	Name         string
	PreScript    string
	PostScript   string
	Description  string
	AuthType     entities.AuthType
	AuthData     string
	GRPCMetadata []entities.HeaderItem
}

// EditOpt holds contextual options for the Edit operation.
type EditOpt struct {
	CollectionID uuid.UUID
	UserID       string
	Version      int
}

// Validate checks that all required fields are present.
func (e *Edit) Validate() error {
	errs := make(map[string]string)
	if e.Name == "" {
		errs["name"] = "required"
	}
	if e.AuthType == entities.AuthTypeInherit {
		errs["authType"] = "inherit is not valid for collections"
	} else if e.AuthType != "" && !e.AuthType.IsValid() {
		errs["authType"] = "invalid auth type"
	}
	if e.AuthData != "" && e.AuthData != "{}" {
		if !json.Valid([]byte(e.AuthData)) {
			errs["authData"] = "invalid JSON"
		}
	}
	if len(errs) > 0 {
		return &domain.ValidationError{Fields: errs}
	}
	return nil
}

// Edit validates input, applies optimistic locking, updates and persists the collection.
func (u *usecase) Edit(ctx context.Context, input Edit, opt EditOpt) (*entities.Collection, error) {
	const funcName = "collection.Edit"

	if err := input.Validate(); err != nil {
		return nil, err
	}

	existing, err := u.repo.GetByID(ctx, opt.CollectionID)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", funcName, err)
	}
	if existing == nil {
		return nil, &domain.NotFoundError{Entity: "collection", ID: opt.CollectionID.String()}
	}
	// Optimistic locking lives here, not in the repo: repo.Update is last-write-wins
	// (shared with the sync apply path), so stale edits must be rejected at this layer.
	if existing.Version != opt.Version {
		return nil, &domain.ConflictError{Entity: "collection", ID: opt.CollectionID.String()}
	}

	authType := input.AuthType
	if authType == "" {
		authType = entities.AuthTypeNone
	}
	authData := input.AuthData
	if authData == "" {
		authData = "{}"
	}

	existing.Name = input.Name
	existing.PreScript = input.PreScript
	existing.PostScript = input.PostScript
	existing.Description = input.Description
	existing.AuthType = authType
	existing.AuthData = authData
	if input.GRPCMetadata != nil {
		existing.GRPCMetadata = input.GRPCMetadata
	}
	existing.Version++
	existing.UpdatedBy = opt.UserID
	existing.UpdatedAt = time.Now()

	if err := u.repo.Update(ctx, existing); err != nil {
		return nil, fmt.Errorf("%s: %w", funcName, err)
	}

	return existing, nil
}
