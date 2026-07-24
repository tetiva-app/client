package wails

import (
	"context"
	"fmt"
	"os"

	"github.com/google/uuid"
	"github.com/wailsapp/wails/v3/pkg/application"

	"github.com/tetiva-app/client/internal/adapters/portability/postman"
	"github.com/tetiva-app/client/internal/adapters/wails/dto"
	"github.com/tetiva-app/client/internal/domain"
	"github.com/tetiva-app/client/internal/domain/entities"
	"github.com/tetiva-app/client/internal/domain/usecase/collection"
	"github.com/tetiva-app/client/internal/domain/usecase/environment"
	"github.com/tetiva-app/client/internal/domain/usecase/request"
)

// PortabilityService exposes import/export operations to the Wails frontend.
type PortabilityService struct {
	app           *application.App
	collectionUC  collection.Usecase
	requestUC     request.Usecase
	environmentUC environment.Usecase
}

// NewPortabilityService creates a new PortabilityService instance.
func NewPortabilityService(
	collectionUC collection.Usecase,
	requestUC request.Usecase,
	environmentUC environment.Usecase,
) *PortabilityService {
	return &PortabilityService{
		collectionUC:  collectionUC,
		requestUC:     requestUC,
		environmentUC: environmentUC,
	}
}

// SetApp sets the Wails application instance (called after app creation in main.go).
func (s *PortabilityService) SetApp(app *application.App) {
	s.app = app
}

// ImportCollection imports a Postman Collection v2.1 JSON string.
func (s *PortabilityService) ImportCollection(req dto.ImportCollectionRequest) Result[dto.ImportCollectionResponse] {
	ctx := context.Background()

	workspaceID, err := uuid.Parse(req.WorkspaceID)
	if err != nil {
		return Err[dto.ImportCollectionResponse](&domain.ValidationError{
			Fields: map[string]string{"workspaceId": "invalid UUID"},
		})
	}

	var parentID *uuid.UUID
	if req.ParentID != nil && *req.ParentID != "" {
		parsed, err := uuid.Parse(*req.ParentID)
		if err != nil {
			return Err[dto.ImportCollectionResponse](&domain.ValidationError{
				Fields: map[string]string{"parentId": "invalid UUID"},
			})
		}
		parentID = &parsed
	}

	result, err := postman.ImportCollection(ctx, []byte(req.Content), postman.ImportOpts{
		WorkspaceID: workspaceID,
		UserID:      defaultUserID,
		ParentID:    parentID,
	}, s.collectionUC, s.requestUC)
	if err != nil {
		return Err[dto.ImportCollectionResponse](fmt.Errorf("import failed: %w", err))
	}

	return OK(dto.ImportCollectionResponse{
		FoldersCreated:  result.FoldersCreated,
		RequestsCreated: result.RequestsCreated,
	})
}

// ExportCollection builds a Postman Collection v2.1 JSON for the given collection tree,
// prompts the user for a save location via a native dialog, and writes the file.
func (s *PortabilityService) ExportCollection(req dto.ExportCollectionRequest) Result[dto.ExportResponse] {
	const funcName = "PortabilityService.ExportCollection"

	data, suggestedName, err := s.buildCollectionExport(req)
	if err != nil {
		return Err[dto.ExportResponse](err)
	}

	return s.promptAndWrite(funcName, data, suggestedName)
}

// ImportEnvironment imports a Postman Environment JSON string.
func (s *PortabilityService) ImportEnvironment(req dto.ImportEnvironmentRequest) Result[dto.ImportEnvironmentResponse] {
	ctx := context.Background()

	workspaceID, err := uuid.Parse(req.WorkspaceID)
	if err != nil {
		return Err[dto.ImportEnvironmentResponse](&domain.ValidationError{
			Fields: map[string]string{"workspaceId": "invalid UUID"},
		})
	}

	result, err := postman.ImportEnvironment(ctx, []byte(req.Content), postman.ImportEnvOpts{
		WorkspaceID: workspaceID,
		UserID:      defaultUserID,
	}, s.environmentUC)
	if err != nil {
		return Err[dto.ImportEnvironmentResponse](fmt.Errorf("import failed: %w", err))
	}

	return OK(dto.ImportEnvironmentResponse{
		EnvironmentName:  result.EnvironmentName,
		VariablesCreated: result.VariablesCreated,
	})
}

// ExportEnvironment builds a Postman Environment JSON, prompts for a save location,
// and writes the file.
func (s *PortabilityService) ExportEnvironment(req dto.ExportEnvironmentRequest) Result[dto.ExportResponse] {
	const funcName = "PortabilityService.ExportEnvironment"

	data, suggestedName, err := s.buildEnvironmentExport(req)
	if err != nil {
		return Err[dto.ExportResponse](err)
	}

	return s.promptAndWrite(funcName, data, suggestedName)
}

func (s *PortabilityService) buildCollectionExport(req dto.ExportCollectionRequest) ([]byte, string, error) {
	ctx := context.Background()

	workspaceID, err := uuid.Parse(req.WorkspaceID)
	if err != nil {
		return nil, "", &domain.ValidationError{Fields: map[string]string{"workspaceId": "invalid UUID"}}
	}

	rootID, err := uuid.Parse(req.ID)
	if err != nil {
		return nil, "", &domain.ValidationError{Fields: map[string]string{"id": "invalid UUID"}}
	}

	allCollections, err := s.collectionUC.List(ctx, collection.ListOpt{WorkspaceID: workspaceID})
	if err != nil {
		return nil, "", err
	}

	subtreeCollections := filterSubtree(rootID, allCollections)

	var allRequests []*entities.Request
	for _, c := range subtreeCollections {
		reqs, err := s.requestUC.List(ctx, request.ListOpt{CollectionID: c.ID})
		if err != nil {
			return nil, "", err
		}
		allRequests = append(allRequests, reqs...)
	}

	data, err := postman.ExportCollection(rootID, subtreeCollections, allRequests)
	if err != nil {
		return nil, "", err
	}

	suggestedName := "collection.postman_collection.json"
	if root, _ := s.collectionUC.GetByID(ctx, rootID); root != nil {
		suggestedName = root.Name + ".postman_collection.json"
	}

	return data, suggestedName, nil
}

func (s *PortabilityService) buildEnvironmentExport(req dto.ExportEnvironmentRequest) ([]byte, string, error) {
	ctx := context.Background()

	envID, err := uuid.Parse(req.ID)
	if err != nil {
		return nil, "", &domain.ValidationError{Fields: map[string]string{"id": "invalid UUID"}}
	}

	env, err := s.environmentUC.GetByID(ctx, envID)
	if err != nil {
		return nil, "", err
	}

	vars, err := s.environmentUC.ListVariables(ctx, envID)
	if err != nil {
		return nil, "", err
	}

	data, err := postman.ExportEnvironment(env, vars)
	if err != nil {
		return nil, "", err
	}

	return data, env.Name + ".postman_environment.json", nil
}

// promptAndWrite shows a native Save File dialog and writes the data to the chosen path.
// Returns Canceled=true if the user dismisses the dialog.
func (s *PortabilityService) promptAndWrite(funcName string, data []byte, suggestedName string) Result[dto.ExportResponse] {
	if s.app == nil {
		return Err[dto.ExportResponse](fmt.Errorf("%s: app not initialized", funcName))
	}

	path, err := s.app.Dialog.SaveFile().
		SetFilename(suggestedName).
		SetButtonText("Export").
		PromptForSingleSelection()
	if err != nil {
		return Err[dto.ExportResponse](fmt.Errorf("%s: dialog: %w", funcName, err))
	}
	if path == "" {
		return OK(dto.ExportResponse{Canceled: true})
	}

	if err := os.WriteFile(path, data, 0o644); err != nil {
		return Err[dto.ExportResponse](fmt.Errorf("%s: write file: %w", funcName, err))
	}

	return OK(dto.ExportResponse{Path: path})
}

// filterSubtree returns only collections that are descendants of rootID (including root).
func filterSubtree(rootID uuid.UUID, all []*entities.Collection) []*entities.Collection {
	inSubtree := make(map[uuid.UUID]bool)
	inSubtree[rootID] = true

	changed := true
	for changed {
		changed = false
		for _, c := range all {
			if c.ParentID != nil && inSubtree[*c.ParentID] && !inSubtree[c.ID] {
				inSubtree[c.ID] = true
				changed = true
			}
		}
	}

	var result []*entities.Collection
	for _, c := range all {
		if inSubtree[c.ID] {
			result = append(result, c)
		}
	}
	return result
}
