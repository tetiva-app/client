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

type Create struct {
	Name        string
	ParentID    *uuid.UUID
	PreScript   string
	PostScript  string
	Description string
	AuthType    entities.AuthType
	AuthData    string
}

type CreateOpt struct {
	UserID      string
	WorkspaceID uuid.UUID
}

func (c *Create) Validate() error {
	errs := make(map[string]string)
	if c.Name == "" {
		errs["name"] = "required"
	}
	if len(c.Description) > domain.MaxDescriptionLen {
		errs["description"] = fmt.Sprintf("must be at most %d bytes", domain.MaxDescriptionLen)
	}
	if c.AuthType == entities.AuthTypeInherit {
		errs["authType"] = "inherit is not valid for collections"
	} else if c.AuthType != "" && !c.AuthType.IsValid() {
		errs["authType"] = "invalid auth type"
	}
	if c.AuthData != "" && c.AuthData != "{}" {
		if !json.Valid([]byte(c.AuthData)) {
			errs["authData"] = "invalid JSON"
		}
	}
	if len(errs) > 0 {
		return &domain.ValidationError{Fields: errs}
	}
	return nil
}

func (u *usecase) Create(ctx context.Context, input Create, opt CreateOpt) (*entities.Collection, error) {
	const funcName = "collection.Create"

	if err := input.Validate(); err != nil {
		return nil, err
	}

	authType := input.AuthType
	if authType == "" {
		authType = entities.AuthTypeNone
	}
	authData := input.AuthData
	if authData == "" {
		authData = "{}"
	}

	now := time.Now()
	c := &entities.Collection{
		ID:           uuid.New(),
		WorkspaceID:  opt.WorkspaceID,
		ParentID:     input.ParentID,
		Name:         input.Name,
		PreScript:    input.PreScript,
		PostScript:   input.PostScript,
		Description:  input.Description,
		AuthType:     authType,
		AuthData:     authData,
		GRPCMetadata: []entities.HeaderItem{},
		SortOrder:    0,
		Version:      1,
		IsDelete:     false,
		CreatedBy:    opt.UserID,
		CreatedAt:    now,
		UpdatedBy:    opt.UserID,
		UpdatedAt:    now,
	}

	if err := u.repo.Create(ctx, c); err != nil {
		return nil, fmt.Errorf("%s: %w", funcName, err)
	}

	return c, nil
}
