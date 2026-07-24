package environment

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/tetiva-app/client/internal/domain/entities"
)

// PersistVariableChanges syncs the given variable map with the active environment.
func (u *usecase) PersistVariableChanges(ctx context.Context, workspaceID uuid.UUID, userID string, newVars map[string]string) error {
	const funcName = "environment.PersistVariableChanges"

	active, err := u.repo.GetActive(ctx, workspaceID)
	if err != nil {
		return fmt.Errorf("%s: %w", funcName, err)
	}
	if active == nil {
		return nil
	}

	existing, err := u.varRepo.List(ctx, active.ID)
	if err != nil {
		return fmt.Errorf("%s: %w", funcName, err)
	}

	existingByKey := make(map[string]*entities.Variable)
	for _, v := range existing {
		existingByKey[v.Key] = v
	}

	now := time.Now()

	for key, value := range newVars {
		if ev, ok := existingByKey[key]; ok {
			if ev.Value != value {
				ev.Value = value
				ev.Version++
				ev.UpdatedBy = userID
				ev.UpdatedAt = now
				if err := u.varRepo.Update(ctx, ev); err != nil {
					return fmt.Errorf("%s: update %q: %w", funcName, key, err)
				}
			}
			delete(existingByKey, key)
		} else {
			newVar := &entities.Variable{
				ID:            uuid.New(),
				EnvironmentID: active.ID,
				Key:           key,
				Value:         value,
				IsSecret:      false,
				Enabled:       true,
				SortOrder:     0,
				Version:       1,
				IsDelete:      false,
				CreatedBy:     userID,
				CreatedAt:     now,
				UpdatedBy:     userID,
				UpdatedAt:     now,
			}
			if err := u.varRepo.Create(ctx, newVar); err != nil {
				return fmt.Errorf("%s: create %q: %w", funcName, key, err)
			}
		}
	}

	return nil
}
