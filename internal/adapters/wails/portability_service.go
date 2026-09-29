package wails

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/google/uuid"
	"github.com/wailsapp/wails/v3/pkg/application"

	"github.com/tetiva-app/client/internal/adapters/portability"
	"github.com/tetiva-app/client/internal/adapters/portability/postman"
	"github.com/tetiva-app/client/internal/adapters/portability/snapshot"
	"github.com/tetiva-app/client/internal/adapters/publication/snapshotjson"
	"github.com/tetiva-app/client/internal/adapters/wails/dto"
	"github.com/tetiva-app/client/internal/domain"
	"github.com/tetiva-app/client/internal/domain/entities"
	"github.com/tetiva-app/client/internal/domain/usecase/collection"
	"github.com/tetiva-app/client/internal/domain/usecase/environment"
	"github.com/tetiva-app/client/internal/domain/usecase/example"
	"github.com/tetiva-app/client/internal/domain/usecase/request"
	"github.com/tetiva-app/client/internal/infrastructure/publicapi"
)

// PortabilityService exposes import/export operations to the Wails frontend.
type PortabilityService struct {
	app            *application.App
	collectionUC   collection.Usecase
	requestUC      request.Usecase
	environmentUC  environment.Usecase
	exampleUC      example.Usecase
	links          *publicapi.Client
	tx             portability.TxRunner
	previews       *previewCache
	snapshots      *snapshot.Importer
	postman        *postman.Importer
	chooseSavePath func(suggestedName string) (string, error)
}

// A nil tx runs imports without a transaction.
func NewPortabilityService(
	collectionUC collection.Usecase,
	requestUC request.Usecase,
	environmentUC environment.Usecase,
	exampleUC example.Usecase,
	links *publicapi.Client,
	tx portability.TxRunner,
) *PortabilityService {
	if tx == nil {
		tx = directTx{}
	}
	return &PortabilityService{
		collectionUC:  collectionUC,
		requestUC:     requestUC,
		environmentUC: environmentUC,
		exampleUC:     exampleUC,
		links:         links,
		tx:            tx,
		previews:      newPreviewCache(),
		snapshots:     snapshot.NewImporter(collectionUC, requestUC, exampleUC, environmentUC),
		postman:       postman.NewImporter(collectionUC, requestUC, exampleUC, environmentUC),
	}
}

type directTx struct{}

func (directTx) Run(ctx context.Context, fn func(ctx context.Context) error) error { return fn(ctx) }

// Called after app creation in main.go.
func (s *PortabilityService) SetApp(app *application.App) {
	s.app = app
}

// ImportCollection never imports scripts: only ImportConfirm shows them for review first.
func (s *PortabilityService) ImportCollection(req dto.ImportCollectionRequest) Result[dto.ImportCollectionResponse] {
	opt, err := importOpt(req.WorkspaceID, req.ParentID, false)
	if err != nil {
		return Err[dto.ImportCollectionResponse](err)
	}
	result, err := s.importData(context.Background(), []byte(req.Content), opt)
	if err != nil {
		return Err[dto.ImportCollectionResponse](fmt.Errorf("import failed: %w", err))
	}

	// This response has always counted the imported collection itself as a folder.
	return OK(dto.ImportCollectionResponse{
		FoldersCreated:  result.Folders + 1,
		RequestsCreated: result.Requests,
		Warnings:        result.Warnings,
	})
}

func (s *PortabilityService) LinkMeta(req dto.LinkMetaRequest) Result[dto.LinkMeta] {
	m, err := s.links.Meta(context.Background(), req.Slug)
	if err != nil {
		return Err[dto.LinkMeta](linkError(req.Slug, err))
	}
	updated := ""
	if !m.UpdatedAt.IsZero() {
		updated = m.UpdatedAt.UTC().Format(time.RFC3339)
	}
	return OK(dto.LinkMeta{
		Slug: m.Slug, Title: m.Title, PasswordRequired: m.PasswordRequired, Revision: m.Revision, UpdatedAt: updated,
	})
}

func (s *PortabilityService) LinkUnlock(req dto.LinkUnlockRequest) Result[dto.LinkUnlockResult] {
	if req.Password == "" {
		return Err[dto.LinkUnlockResult](&domain.ValidationError{Fields: map[string]string{"password": "required"}})
	}
	token, err := s.links.Unlock(context.Background(), req.Slug, req.Password)
	if err != nil {
		return Err[dto.LinkUnlockResult](linkError(req.Slug, err))
	}
	return OK(dto.LinkUnlockResult{Token: token})
}

func (s *PortabilityService) LinkFetch(req dto.LinkFetchRequest) Result[dto.ImportPreviewResult] {
	const funcName = "PortabilityService.LinkFetch"

	data, err := s.links.Snapshot(context.Background(), req.Slug, req.Token)
	if err != nil {
		return Err[dto.ImportPreviewResult](linkError(req.Slug, err))
	}
	preview, err := s.snapshots.Preview(data)
	if err != nil {
		return Err[dto.ImportPreviewResult](fmt.Errorf("%s: %w", funcName, err))
	}
	id, err := s.previews.put(data)
	if err != nil {
		return Err[dto.ImportPreviewResult](fmt.Errorf("%s: %w", funcName, err))
	}
	return OK(dto.ImportPreviewResult{PreviewID: id, Preview: dto.ImportPreviewFrom(preview)})
}

func (s *PortabilityService) ImportPreview(req dto.ImportPreviewRequest) Result[dto.ImportPreview] {
	const funcName = "PortabilityService.ImportPreview"

	data := []byte(req.Content)
	if postman.IsEnvironment(data) {
		return Err[dto.ImportPreview](&domain.ReasonError{
			Reason: portability.ReasonEnvironmentFile,
			Err:    &domain.ValidationError{Fields: map[string]string{"content": "a Postman environment, not a collection"}},
		})
	}
	imp, err := portability.Select(data, s.snapshots, s.postman)
	if err != nil {
		return Err[dto.ImportPreview](err)
	}
	preview, err := imp.Preview(data)
	if err != nil {
		return Err[dto.ImportPreview](fmt.Errorf("%s: %w", funcName, err))
	}
	return OK(dto.ImportPreviewFrom(preview))
}

// ImportConfirm imports a previewed link or a file in one transaction.
func (s *PortabilityService) ImportConfirm(req dto.ImportConfirmRequest) Result[dto.ImportConfirmResult] {
	const funcName = "PortabilityService.ImportConfirm"

	if (req.PreviewID == "") == (req.Content == "") {
		return Err[dto.ImportConfirmResult](&domain.ValidationError{Fields: map[string]string{
			"previewId": "exactly one of previewId and content is required",
		}})
	}
	opt, err := importOpt(req.WorkspaceID, req.ParentID, req.IncludeScripts)
	if err != nil {
		return Err[dto.ImportConfirmResult](err)
	}
	data := []byte(req.Content)
	if req.PreviewID != "" {
		cached, ok := s.previews.get(req.PreviewID)
		if !ok {
			return Err[dto.ImportConfirmResult](&domain.ReasonError{
				Reason: portability.ReasonPreviewExpired,
				Err:    &domain.NotFoundError{Entity: "import preview", ID: req.PreviewID},
			})
		}
		data = cached
	}

	result, err := s.importData(context.Background(), data, opt)
	if err != nil {
		return Err[dto.ImportConfirmResult](fmt.Errorf("%s: %w", funcName, err))
	}
	if req.PreviewID != "" {
		s.previews.remove(req.PreviewID)
	}
	return OK(dto.ImportConfirmResultFrom(result))
}

func importOpt(workspaceID string, parentID *string, includeScripts bool) (portability.ImportOpt, error) {
	ws, err := parseUUIDField("workspaceId", workspaceID)
	if err != nil {
		return portability.ImportOpt{}, err
	}
	opt := portability.ImportOpt{WorkspaceID: ws, UserID: defaultUserID, IncludeScripts: includeScripts}
	if parentID != nil && *parentID != "" {
		id, err := parseUUIDField("parentId", *parentID)
		if err != nil {
			return portability.ImportOpt{}, err
		}
		opt.ParentID = &id
	}
	return opt, nil
}

func (s *PortabilityService) importData(ctx context.Context, data []byte, opt portability.ImportOpt) (*portability.ImportResult, error) {
	imp, err := portability.Select(data, s.snapshots, s.postman)
	if err != nil {
		return nil, err
	}
	var result *portability.ImportResult
	err = s.tx.Run(ctx, func(ctx context.Context) error {
		var err error
		result, err = imp.Import(ctx, data, opt)
		return err
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

func linkError(slug string, err error) error {
	reason := func(r string, inner error) error { return &domain.ReasonError{Reason: r, Err: inner} }
	switch {
	case errors.Is(err, publicapi.ErrNotFound):
		return reason(portability.ReasonLinkNotFound, &domain.NotFoundError{Entity: "published collection", ID: slug})
	case errors.Is(err, publicapi.ErrWrongPassword):
		return reason(portability.ReasonPasswordInvalid, &domain.ValidationError{Fields: map[string]string{"password": "wrong password"}})
	case errors.Is(err, publicapi.ErrPasswordRequired):
		return reason(portability.ReasonPasswordRequired, &domain.ValidationError{Fields: map[string]string{"password": "required"}})
	case errors.Is(err, publicapi.ErrRateLimited):
		return reason(portability.ReasonRateLimited, err)
	case errors.Is(err, publicapi.ErrTooLarge):
		return reason(snapshotjson.ReasonTooLarge, &domain.ValidationError{Fields: map[string]string{"snapshot": "larger than 8 MiB"}})
	case errors.Is(err, publicapi.ErrUnreachable):
		return reason(ReasonServerUnreachable, err)
	default:
		return err
	}
}

// ExportCollection builds a Postman Collection v2.1 JSON for the given collection tree,
// prompts the user for a save location via a native dialog, and writes the file.
func (s *PortabilityService) ExportCollection(req dto.ExportCollectionRequest) Result[dto.ExportResponse] {
	const funcName = "PortabilityService.ExportCollection"

	data, suggestedName, warnings, err := s.buildCollectionExport(req)
	if err != nil {
		return Err[dto.ExportResponse](err)
	}

	return s.promptAndWrite(funcName, data, suggestedName, warnings)
}

// ImportEnvironment imports a Postman Environment JSON string.
func (s *PortabilityService) ImportEnvironment(req dto.ImportEnvironmentRequest) Result[dto.ImportEnvironmentResponse] {
	workspaceID, err := uuid.Parse(req.WorkspaceID)
	if err != nil {
		return Err[dto.ImportEnvironmentResponse](&domain.ValidationError{
			Fields: map[string]string{"workspaceId": "invalid UUID"},
		})
	}

	var result *postman.ImportEnvResult
	err = s.tx.Run(context.Background(), func(ctx context.Context) error {
		var err error
		result, err = postman.ImportEnvironment(ctx, []byte(req.Content), postman.ImportEnvOpts{
			WorkspaceID: workspaceID,
			UserID:      defaultUserID,
		}, s.environmentUC)
		return err
	})
	if err != nil {
		return Err[dto.ImportEnvironmentResponse](fmt.Errorf("import failed: %w", err))
	}

	return OK(dto.ImportEnvironmentResponseFrom(result))
}

// ExportEnvironment builds a Postman Environment JSON, prompts for a save location,
// and writes the file.
func (s *PortabilityService) ExportEnvironment(req dto.ExportEnvironmentRequest) Result[dto.ExportResponse] {
	const funcName = "PortabilityService.ExportEnvironment"

	data, suggestedName, err := s.buildEnvironmentExport(req)
	if err != nil {
		return Err[dto.ExportResponse](err)
	}

	return s.promptAndWrite(funcName, data, suggestedName, nil)
}

func (s *PortabilityService) buildCollectionExport(req dto.ExportCollectionRequest) ([]byte, string, []string, error) {
	const funcName = "PortabilityService.buildCollectionExport"
	ctx := context.Background()

	workspaceID, err := uuid.Parse(req.WorkspaceID)
	if err != nil {
		return nil, "", nil, &domain.ValidationError{Fields: map[string]string{"workspaceId": "invalid UUID"}}
	}

	rootID, err := uuid.Parse(req.ID)
	if err != nil {
		return nil, "", nil, &domain.ValidationError{Fields: map[string]string{"id": "invalid UUID"}}
	}

	allCollections, err := s.collectionUC.List(ctx, collection.ListOpt{WorkspaceID: workspaceID})
	if err != nil {
		return nil, "", nil, err
	}

	subtreeCollections := filterSubtree(rootID, allCollections)

	var allRequests []*entities.Request
	for _, c := range subtreeCollections {
		reqs, err := s.requestUC.List(ctx, request.ListOpt{CollectionID: c.ID})
		if err != nil {
			return nil, "", nil, err
		}
		allRequests = append(allRequests, reqs...)
	}

	examples := make(map[uuid.UUID][]*entities.ResponseExample)
	for _, r := range allRequests {
		if r.Protocol == entities.ProtocolGRPC || r.Protocol == entities.ProtocolWebSocket {
			continue
		}
		list, err := s.exampleUC.ListByRequest(ctx, r.ID)
		if err != nil {
			return nil, "", nil, fmt.Errorf("%s: %w", funcName, err)
		}
		examples[r.ID] = list
	}

	data, warnings, err := postman.ExportCollection(rootID, subtreeCollections, allRequests, examples)
	if err != nil {
		return nil, "", nil, err
	}

	suggestedName := "collection.postman_collection.json"
	if root, _ := s.collectionUC.GetByID(ctx, rootID); root != nil {
		suggestedName = root.Name + ".postman_collection.json"
	}

	return data, suggestedName, warnings, nil
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

// Returns Canceled=true if the user dismisses the dialog.
func (s *PortabilityService) promptAndWrite(funcName string, data []byte, suggestedName string, warnings []string) Result[dto.ExportResponse] {
	choose := s.chooseSavePath
	if choose == nil {
		if s.app == nil {
			return Err[dto.ExportResponse](fmt.Errorf("%s: app not initialized", funcName))
		}
		choose = s.dialogSavePath
	}

	path, err := choose(suggestedName)
	if err != nil {
		return Err[dto.ExportResponse](fmt.Errorf("%s: dialog: %w", funcName, err))
	}
	if path == "" {
		return OK(dto.ExportResponse{Canceled: true})
	}

	if err := os.WriteFile(path, data, 0o644); err != nil {
		return Err[dto.ExportResponse](fmt.Errorf("%s: write file: %w", funcName, err))
	}

	return OK(dto.ExportResponse{Path: path, Warnings: warnings})
}

func (s *PortabilityService) dialogSavePath(suggestedName string) (string, error) {
	return s.app.Dialog.SaveFile().
		SetFilename(suggestedName).
		SetButtonText("Export").
		PromptForSingleSelection()
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
