package collection

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/tetiva-app/client/internal/domain"
	"github.com/tetiva-app/client/internal/domain/entities"
	"github.com/tetiva-app/client/internal/domain/usecase/auth"
)

type Edit struct {
	Name         string
	PreScript    string
	PostScript   string
	Description  string
	AuthType     entities.AuthType
	AuthData     string
	GRPCMetadata []entities.HeaderItem
}

type EditOpt struct {
	CollectionID uuid.UUID
	UserID       string
	Version      int
}

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

	// Only growth is rejected: a description stored before the cap must not lock the entity.
	if len(input.Description) > domain.MaxDescriptionLen && len(input.Description) > len(existing.Description) {
		return nil, &domain.ValidationError{Fields: map[string]string{
			"description": fmt.Sprintf("must be at most %d bytes", domain.MaxDescriptionLen),
		}}
	}

	prevAuthType, prevAuthData := existing.AuthType, existing.AuthData

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

	if auth.AcquisitionChanged(prevAuthType, existing.AuthType, prevAuthData, existing.AuthData) {
		// The saved configuration can no longer produce the stored token — unless
		// it is the one the token was acquired with, from the same editor buffer.
		keep := auth.AcquisitionHash(existing.AuthType, existing.AuthData)
		if err := u.tokens().ClearOwnersUnlessHash(ctx, entities.AuthOwnerKindCollection, []uuid.UUID{existing.ID}, keep); err != nil {
			return nil, fmt.Errorf("%s: clear tokens: %w", funcName, err)
		}
	}

	return existing, nil
}
