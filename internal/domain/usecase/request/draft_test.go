package request_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/tetiva-app/client/internal/domain"
	"github.com/tetiva-app/client/internal/domain/entities"
	"github.com/tetiva-app/client/internal/domain/usecase/request"
)

// newDraftUsecase wires a draft usecase; nil is fine for collaborators the test does not exercise.
func newDraftUsecase(repo request.Repository, historyRepo request.HistoryRepository, collectionReader request.CollectionReader) request.Usecase {
	return request.NewUsecase(
		repo, historyRepo,
		nil, nil, nil,
		&mockEnvResolver{}, &noopScriptEngine{}, &noopScriptResolver{},
		&noopVarPersister{}, request.NewAuthResolver(fixtureCollections()),
		nil, collectionReader, nil, nil,
	)
}

func TestCreateDraftFromHistory_BuildsDraftWithSourceFields(t *testing.T) {
	ctx := context.Background()
	workspaceID := uuid.New()
	collectionID := uuid.New()
	historyID := uuid.New()
	sourceRequestID := uuid.New()

	historyRecord := &entities.History{
		ID:              historyID,
		RequestID:       sourceRequestID,
		WorkspaceID:     workspaceID,
		Protocol:        entities.ProtocolHTTP,
		Method:          "POST",
		URL:             "https://api.example.com/users",
		RequestHeaders:  map[string][]string{"Content-Type": {"application/json"}},
		RequestBody:     `{"name":"alice"}`,
		ResponseStatus:  201,
		ResponseHeaders: map[string][]string{},
		ResponseBody:    "",
		DurationMs:      150,
		CreatedAt:       time.Now(),
	}

	sourceRequest := &entities.Request{
		ID:           sourceRequestID,
		CollectionID: collectionID,
		Name:         "Create user",
		Protocol:     entities.ProtocolHTTP,
		Method:       entities.MethodPOST,
		URL:          "https://api.example.com/users",
		Version:      1,
	}

	repo := newMockRepo()
	repo.requests[sourceRequestID] = sourceRequest

	historyRepo := &mockHistoryRepo{}
	if err := historyRepo.Create(ctx, historyRecord); err != nil {
		t.Fatalf("setup history: %v", err)
	}

	uc := newDraftUsecase(repo, historyRepo, nil)

	draft, err := uc.CreateDraftFromHistory(ctx, request.CreateDraftFromHistoryOpt{
		HistoryID:   historyID,
		WorkspaceID: workspaceID,
		UserID:      "tester",
	})
	if err != nil {
		t.Fatalf("CreateDraftFromHistory: %v", err)
	}
	if draft == nil {
		t.Fatal("expected non-nil draft")
	}
	if !draft.IsDraft {
		t.Error("draft.IsDraft = false, want true")
	}
	if draft.CollectionID != collectionID {
		t.Errorf("CollectionID = %v, want %v", draft.CollectionID, collectionID)
	}
	if draft.Method != entities.MethodPOST {
		t.Errorf("Method = %v, want POST", draft.Method)
	}
	if draft.URL != "https://api.example.com/users" {
		t.Errorf("URL = %q, want https://api.example.com/users", draft.URL)
	}
	if draft.Body != `{"name":"alice"}` {
		t.Errorf("Body = %q, want %q", draft.Body, `{"name":"alice"}`)
	}
	if !hasHeader(draft.Headers, "Content-Type", "application/json") {
		t.Errorf("expected Content-Type header to be carried over, got %+v", draft.Headers)
	}
	if draft.Version != 1 {
		t.Errorf("Version = %d, want 1", draft.Version)
	}
	if draft.CreatedBy != "tester" || draft.UpdatedBy != "tester" {
		t.Errorf("CreatedBy/UpdatedBy = %q/%q, want tester", draft.CreatedBy, draft.UpdatedBy)
	}
	if _, ok := repo.requests[draft.ID]; !ok {
		t.Error("draft was not persisted via repo.Create")
	}
}

func TestCreateDraftFromHistory_HistoryNotFound(t *testing.T) {
	ctx := context.Background()
	repo := newMockRepo()
	historyRepo := &mockHistoryRepo{}
	uc := newDraftUsecase(repo, historyRepo, nil)

	_, err := uc.CreateDraftFromHistory(ctx, request.CreateDraftFromHistoryOpt{
		HistoryID:   uuid.New(),
		WorkspaceID: uuid.New(),
		UserID:      "tester",
	})
	if err == nil {
		t.Fatal("expected error for missing history, got nil")
	}
	var notFoundErr *domain.NotFoundError
	if !errors.As(err, &notFoundErr) {
		t.Fatalf("expected *domain.NotFoundError, got %T: %v", err, err)
	}
	if notFoundErr.Entity != "history" {
		t.Errorf("entity = %q, want history", notFoundErr.Entity)
	}
}

func TestCreateDraftFromHistory_WorkspaceMismatch(t *testing.T) {
	ctx := context.Background()
	historyID := uuid.New()
	historyRecord := &entities.History{
		ID:              historyID,
		WorkspaceID:     uuid.New(),
		Protocol:        entities.ProtocolHTTP,
		Method:          "GET",
		URL:             "https://api.example.com",
		RequestHeaders:  map[string][]string{},
		ResponseHeaders: map[string][]string{},
	}

	repo := newMockRepo()
	historyRepo := &mockHistoryRepo{}
	if err := historyRepo.Create(ctx, historyRecord); err != nil {
		t.Fatalf("setup history: %v", err)
	}

	uc := newDraftUsecase(repo, historyRepo, nil)

	_, err := uc.CreateDraftFromHistory(ctx, request.CreateDraftFromHistoryOpt{
		HistoryID:   historyID,
		WorkspaceID: uuid.New(), // different workspace
		UserID:      "tester",
	})
	if err == nil {
		t.Fatal("expected workspace mismatch error, got nil")
	}
	var valErr *domain.ValidationError
	if !errors.As(err, &valErr) {
		t.Fatalf("expected *domain.ValidationError, got %T: %v", err, err)
	}
	if _, ok := valErr.Fields["workspaceId"]; !ok {
		t.Errorf("expected validation error for 'workspaceId', got fields %+v", valErr.Fields)
	}
}

func TestCreateDraftFromHistory_FallsBackToFirstCollection(t *testing.T) {
	ctx := context.Background()
	workspaceID := uuid.New()
	historyID := uuid.New()
	// no original RequestID — source request was deleted
	historyRecord := &entities.History{
		ID:              historyID,
		RequestID:       uuid.Nil,
		WorkspaceID:     workspaceID,
		Protocol:        entities.ProtocolHTTP,
		Method:          "GET",
		URL:             "https://api.example.com/orphan",
		RequestHeaders:  map[string][]string{},
		ResponseHeaders: map[string][]string{},
	}

	repo := newMockRepo()
	historyRepo := &mockHistoryRepo{}
	if err := historyRepo.Create(ctx, historyRecord); err != nil {
		t.Fatalf("setup history: %v", err)
	}

	firstCollID := uuid.New()
	collectionReader := &mockCollectionReader{
		byWorkspace: map[uuid.UUID][]*entities.Collection{
			workspaceID: {
				{ID: firstCollID, WorkspaceID: workspaceID, Name: "First", SortOrder: 0},
				{ID: uuid.New(), WorkspaceID: workspaceID, Name: "Second", SortOrder: 1},
			},
		},
	}

	uc := newDraftUsecase(repo, historyRepo, collectionReader)

	draft, err := uc.CreateDraftFromHistory(ctx, request.CreateDraftFromHistoryOpt{
		HistoryID:   historyID,
		WorkspaceID: workspaceID,
		UserID:      "tester",
	})
	if err != nil {
		t.Fatalf("CreateDraftFromHistory: %v", err)
	}
	if draft.CollectionID != firstCollID {
		t.Errorf("CollectionID = %v, want first collection %v", draft.CollectionID, firstCollID)
	}
}

func TestCreateDraftFromHistory_NoCollectionAvailable(t *testing.T) {
	ctx := context.Background()
	workspaceID := uuid.New()
	historyID := uuid.New()
	historyRecord := &entities.History{
		ID:              historyID,
		RequestID:       uuid.Nil,
		WorkspaceID:     workspaceID,
		Protocol:        entities.ProtocolHTTP,
		Method:          "GET",
		URL:             "https://api.example.com",
		RequestHeaders:  map[string][]string{},
		ResponseHeaders: map[string][]string{},
	}

	repo := newMockRepo()
	historyRepo := &mockHistoryRepo{}
	if err := historyRepo.Create(ctx, historyRecord); err != nil {
		t.Fatalf("setup history: %v", err)
	}

	collectionReader := &mockCollectionReader{}

	uc := newDraftUsecase(repo, historyRepo, collectionReader)

	_, err := uc.CreateDraftFromHistory(ctx, request.CreateDraftFromHistoryOpt{
		HistoryID:   historyID,
		WorkspaceID: workspaceID,
		UserID:      "tester",
	})
	if err == nil {
		t.Fatal("expected error when no collection available, got nil")
	}
	if !errors.Is(err, request.ErrNoCollectionForDraft) {
		t.Errorf("expected ErrNoCollectionForDraft, got %v", err)
	}
}

// Regression guard: drafts must carry defaults that pass the json_valid(...)
// CHECK constraints on requests (auth_data, grpc_metadata) — "" is not valid JSON.
func TestCreateDraftFromHistory_PassesJSONCheckConstraints(t *testing.T) {
	ctx := context.Background()
	workspaceID := uuid.New()
	collectionID := uuid.New()
	historyID := uuid.New()
	sourceRequestID := uuid.New()

	historyRecord := &entities.History{
		ID:              historyID,
		RequestID:       sourceRequestID,
		WorkspaceID:     workspaceID,
		Protocol:        entities.ProtocolHTTP,
		Method:          "GET",
		URL:             "https://api.example.com/ping",
		RequestHeaders:  map[string][]string{},
		ResponseHeaders: map[string][]string{},
	}

	repo := newMockRepo()
	repo.requests[sourceRequestID] = &entities.Request{
		ID:           sourceRequestID,
		CollectionID: collectionID,
		Version:      1,
	}

	historyRepo := &mockHistoryRepo{}
	if err := historyRepo.Create(ctx, historyRecord); err != nil {
		t.Fatalf("setup history: %v", err)
	}

	uc := newDraftUsecase(repo, historyRepo, nil)

	draft, err := uc.CreateDraftFromHistory(ctx, request.CreateDraftFromHistoryOpt{
		HistoryID:   historyID,
		WorkspaceID: workspaceID,
		UserID:      "tester",
	})
	if err != nil {
		t.Fatalf("CreateDraftFromHistory: %v", err)
	}

	if draft.AuthData != "{}" {
		t.Errorf("AuthData = %q, want %q (must satisfy json_valid CHECK)", draft.AuthData, "{}")
	}

	// a nil map marshals to "null" (still json_valid), but regular requests serialize {}
	if draft.GRPCMetadata == nil {
		t.Error("GRPCMetadata must be non-nil so it serializes to a JSON object, not null")
	}
}

func hasHeader(items []entities.HeaderItem, key, value string) bool {
	for _, h := range items {
		if h.Key == key && h.Value == value && h.Enabled {
			return true
		}
	}
	return false
}

func TestDeleteDraft_HardDeletes(t *testing.T) {
	ctx := context.Background()
	draftID := uuid.New()
	repo := newMockRepo()
	repo.requests[draftID] = &entities.Request{ID: draftID, IsDraft: true, Version: 1}

	uc := newDraftUsecase(repo, &mockHistoryRepo{}, nil)

	if err := uc.DeleteDraft(ctx, draftID); err != nil {
		t.Fatalf("DeleteDraft: %v", err)
	}
	if !repo.hardDeleted(draftID) {
		t.Error("expected DeleteHard call")
	}
}

func TestDeleteDraft_RejectsNonDraft(t *testing.T) {
	ctx := context.Background()
	rid := uuid.New()
	repo := newMockRepo()
	repo.requests[rid] = &entities.Request{ID: rid, IsDraft: false, Version: 1}

	uc := newDraftUsecase(repo, &mockHistoryRepo{}, nil)

	err := uc.DeleteDraft(ctx, rid)
	if err == nil {
		t.Fatal("expected error for non-draft")
	}
	var valErr *domain.ValidationError
	if !errors.As(err, &valErr) {
		t.Fatalf("expected *domain.ValidationError, got %T: %v", err, err)
	}
}

func TestDeleteDraft_NotFound(t *testing.T) {
	ctx := context.Background()
	repo := newMockRepo()
	uc := newDraftUsecase(repo, &mockHistoryRepo{}, nil)

	err := uc.DeleteDraft(ctx, uuid.New())
	if err == nil {
		t.Fatal("expected NotFoundError, got nil")
	}
	var nfErr *domain.NotFoundError
	if !errors.As(err, &nfErr) {
		t.Fatalf("expected *domain.NotFoundError, got %T: %v", err, err)
	}
}

func TestPromoteDraft_FlipsFlagsAndPersists(t *testing.T) {
	ctx := context.Background()
	draftID := uuid.New()
	targetColl := uuid.New()
	repo := newMockRepo()
	repo.requests[draftID] = &entities.Request{
		ID:           draftID,
		IsDraft:      true,
		CollectionID: uuid.New(),
		Version:      1,
		Name:         "Replay: ...",
	}

	uc := newDraftUsecase(repo, &mockHistoryRepo{}, nil)

	promoted, err := uc.PromoteDraft(ctx, request.PromoteDraftOpt{
		DraftID:            draftID,
		Name:               "My new request",
		TargetCollectionID: targetColl,
		UserID:             "tester",
		Version:            1,
	})
	if err != nil {
		t.Fatalf("PromoteDraft: %v", err)
	}
	if promoted.IsDraft {
		t.Error("IsDraft must be false after promote")
	}
	if promoted.CollectionID != targetColl {
		t.Errorf("CollectionID = %v, want %v", promoted.CollectionID, targetColl)
	}
	if promoted.Name != "My new request" {
		t.Errorf("Name = %s, want My new request", promoted.Name)
	}
	if promoted.Version != 2 {
		t.Errorf("Version = %d, want 2", promoted.Version)
	}
	if promoted.UpdatedBy != "tester" {
		t.Errorf("UpdatedBy = %q, want tester", promoted.UpdatedBy)
	}
	stored, _ := repo.GetByID(ctx, draftID)
	if stored == nil || stored.IsDraft {
		t.Error("expected persisted, non-draft request after PromoteDraft")
	}
}

func TestPromoteDraft_NotFound(t *testing.T) {
	ctx := context.Background()
	repo := newMockRepo()
	uc := newDraftUsecase(repo, &mockHistoryRepo{}, nil)

	_, err := uc.PromoteDraft(ctx, request.PromoteDraftOpt{
		DraftID:            uuid.New(),
		Name:               "x",
		TargetCollectionID: uuid.New(),
		UserID:             "tester",
		Version:            1,
	})
	if err == nil {
		t.Fatal("expected NotFoundError, got nil")
	}
	var nfErr *domain.NotFoundError
	if !errors.As(err, &nfErr) {
		t.Fatalf("expected *domain.NotFoundError, got %T: %v", err, err)
	}
}

func TestPromoteDraft_RejectsNonDraft(t *testing.T) {
	ctx := context.Background()
	rid := uuid.New()
	repo := newMockRepo()
	repo.requests[rid] = &entities.Request{ID: rid, IsDraft: false, Version: 1}

	uc := newDraftUsecase(repo, &mockHistoryRepo{}, nil)

	_, err := uc.PromoteDraft(ctx, request.PromoteDraftOpt{
		DraftID:            rid,
		Name:               "x",
		TargetCollectionID: uuid.New(),
		UserID:             "tester",
		Version:            1,
	})
	if err == nil {
		t.Fatal("expected validation error, got nil")
	}
	var valErr *domain.ValidationError
	if !errors.As(err, &valErr) {
		t.Fatalf("expected *domain.ValidationError, got %T: %v", err, err)
	}
}

func TestPromoteDraft_VersionConflict(t *testing.T) {
	ctx := context.Background()
	draftID := uuid.New()
	repo := newMockRepo()
	repo.requests[draftID] = &entities.Request{
		ID:      draftID,
		IsDraft: true,
		Version: 5,
	}

	uc := newDraftUsecase(repo, &mockHistoryRepo{}, nil)

	_, err := uc.PromoteDraft(ctx, request.PromoteDraftOpt{
		DraftID:            draftID,
		Name:               "x",
		TargetCollectionID: uuid.New(),
		UserID:             "tester",
		Version:            1, // wrong
	})
	if err == nil {
		t.Fatal("expected ConflictError, got nil")
	}
	var conflictErr *domain.ConflictError
	if !errors.As(err, &conflictErr) {
		t.Fatalf("expected *domain.ConflictError, got %T: %v", err, err)
	}
}

func TestCleanupDrafts_Delegates(t *testing.T) {
	ctx := context.Background()
	repo := newMockRepo()
	for i := 0; i < 5; i++ {
		id := uuid.New()
		repo.requests[id] = &entities.Request{ID: id, IsDraft: true, Version: 1}
	}
	keepID := uuid.New()
	repo.requests[keepID] = &entities.Request{ID: keepID, IsDraft: false, Version: 1}

	uc := newDraftUsecase(repo, &mockHistoryRepo{}, nil)

	n, err := uc.CleanupDrafts(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if n != 5 {
		t.Errorf("got %d, want 5", n)
	}
	if repo.cleanupCalls != 1 {
		t.Errorf("expected 1 CleanupDrafts call, got %d", repo.cleanupCalls)
	}
	if _, ok := repo.requests[keepID]; !ok {
		t.Error("non-draft request must remain")
	}
}

func TestCreateDraftFromHistory_StripsCredentials(t *testing.T) {
	ctx := context.Background()
	workspaceID := uuid.New()
	historyID := uuid.New()
	sourceRequestID := uuid.New()
	collectionID := uuid.New()

	historyRepo := &mockHistoryRepo{}
	if err := historyRepo.Create(ctx, &entities.History{
		ID:          historyID,
		RequestID:   sourceRequestID,
		WorkspaceID: workspaceID,
		Protocol:    entities.ProtocolHTTP,
		Method:      "GET",
		URL:         "https://api.example.com/users?page=2&access_token=secret&sess=abc",
		RequestHeaders: map[string][]string{
			"authorization": {"Bearer secret"},
			"Cookie":        {"sid=1"},
			"X-Trace":       {"keep"},
		},
		AuthQueryKeys: []string{"sess"},
		CreatedAt:     time.Now(),
	}); err != nil {
		t.Fatalf("setup history: %v", err)
	}

	repo := newMockRepo()
	repo.requests[sourceRequestID] = &entities.Request{
		ID: sourceRequestID, CollectionID: collectionID, Version: 1,
	}
	uc := newDraftUsecase(repo, historyRepo, nil)

	draft, err := uc.CreateDraftFromHistory(ctx, request.CreateDraftFromHistoryOpt{
		HistoryID:   historyID,
		WorkspaceID: workspaceID,
		UserID:      "tester",
	})
	if err != nil {
		t.Fatalf("CreateDraftFromHistory: %v", err)
	}

	if draft.URL != "https://api.example.com/users?page=2" {
		t.Errorf("URL = %q, want the credential parameters gone", draft.URL)
	}
	for _, h := range draft.Headers {
		if strings.EqualFold(h.Key, "authorization") || strings.EqualFold(h.Key, "cookie") {
			t.Errorf("header %q must not reach the draft", h.Key)
		}
	}
	if !hasHeader(draft.Headers, "X-Trace", "keep") {
		t.Errorf("ordinary headers must survive, got %+v", draft.Headers)
	}
}
