package request_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/tetiva-app/client/internal/domain"
	"github.com/tetiva-app/client/internal/domain/entities"
	"github.com/tetiva-app/client/internal/domain/usecase/request"
)

var (
	testCollectionID = uuid.MustParse("00000000-0000-4000-a000-000000000010")
	testWorkspaceID  = uuid.MustParse("00000000-0000-4000-a000-000000000001")
)

// fixtureCollections holds the collection every test request lives in; auth
// resolution loads it on every execution path to learn the workspace.
func fixtureCollections() *mockCollectionReader {
	return &mockCollectionReader{
		collections: map[uuid.UUID]*entities.Collection{
			testCollectionID: {
				ID:          testCollectionID,
				WorkspaceID: testWorkspaceID,
				AuthType:    entities.AuthTypeNone,
				AuthData:    "{}",
			},
		},
	}
}

type mockRepo struct {
	requests       map[uuid.UUID]*entities.Request
	hardDeletedIDs map[uuid.UUID]bool
	cleanupCalls   int
}

func newMockRepo() *mockRepo {
	return &mockRepo{
		requests:       make(map[uuid.UUID]*entities.Request),
		hardDeletedIDs: make(map[uuid.UUID]bool),
	}
}

func (m *mockRepo) hardDeleted(id uuid.UUID) bool {
	return m.hardDeletedIDs[id]
}

func copyStringSliceMap(src map[string][]string) map[string][]string {
	if src == nil {
		return nil
	}
	dst := make(map[string][]string, len(src))
	for k, v := range src {
		cp := make([]string, len(v))
		copy(cp, v)
		dst[k] = cp
	}
	return dst
}

func copyHeaderItems(src []entities.HeaderItem) []entities.HeaderItem {
	if src == nil {
		return nil
	}
	dst := make([]entities.HeaderItem, len(src))
	copy(dst, src)
	return dst
}

func (m *mockRepo) Create(_ context.Context, r *entities.Request) error {
	m.requests[r.ID] = r
	return nil
}

func (m *mockRepo) GetByID(_ context.Context, id uuid.UUID) (*entities.Request, error) {
	r, ok := m.requests[id]
	if !ok {
		return nil, nil
	}
	// Return a deep copy to avoid shared pointer mutations.
	cp := *r
	cp.Headers = copyHeaderItems(r.Headers)
	cp.GRPCMetadata = copyStringSliceMap(r.GRPCMetadata)
	return &cp, nil
}

func (m *mockRepo) GetDescriptionByID(_ context.Context, id uuid.UUID) (string, error) {
	r, ok := m.requests[id]
	if !ok {
		return "", nil
	}
	return r.Description, nil
}

func (m *mockRepo) List(_ context.Context, filter request.Filter) ([]*entities.Request, error) {
	var result []*entities.Request
	for _, r := range m.requests {
		if r.CollectionID != filter.CollectionID {
			continue
		}
		if r.IsDelete {
			continue
		}
		cp := *r
		cp.Headers = copyHeaderItems(r.Headers)
		cp.GRPCMetadata = copyStringSliceMap(r.GRPCMetadata)
		result = append(result, &cp)
	}
	return result, nil
}

func (m *mockRepo) Update(_ context.Context, r *entities.Request) error {
	m.requests[r.ID] = r
	return nil
}

func (m *mockRepo) UpdateSortOrder(_ context.Context, id uuid.UUID, sortOrder int) error {
	r, ok := m.requests[id]
	if !ok {
		return nil
	}
	r.SortOrder = sortOrder
	return nil
}

func (m *mockRepo) DeleteHard(_ context.Context, id uuid.UUID) error {
	delete(m.requests, id)
	if m.hardDeletedIDs == nil {
		m.hardDeletedIDs = make(map[uuid.UUID]bool)
	}
	m.hardDeletedIDs[id] = true
	return nil
}

func (m *mockRepo) CleanupDrafts(_ context.Context) (int, error) {
	m.cleanupCalls++
	n := 0
	for id, r := range m.requests {
		if r.IsDraft {
			delete(m.requests, id)
			n++
		}
	}
	return n, nil
}

type mockHistoryRepo struct {
	entries []*entities.History
	byID    map[uuid.UUID]*entities.History
}

func (m *mockHistoryRepo) Create(_ context.Context, h *entities.History) error {
	m.entries = append(m.entries, h)
	if m.byID == nil {
		m.byID = make(map[uuid.UUID]*entities.History)
	}
	m.byID[h.ID] = h
	return nil
}

func (m *mockHistoryRepo) GetByID(_ context.Context, id uuid.UUID) (*entities.History, error) {
	if m.byID == nil {
		return nil, nil
	}
	return m.byID[id], nil
}

type mockRequester struct {
	response    *entities.Response
	err         error
	lastRequest request.HTTPExecuteRequest
}

func (m *mockRequester) Execute(_ context.Context, req request.HTTPExecuteRequest) (*entities.Response, error) {
	m.lastRequest = req
	return m.response, m.err
}

type mockEnvResolver struct {
	vars  map[string]string
	calls int
}

func (m *mockEnvResolver) ResolveVariables(_ context.Context, _ uuid.UUID) (map[string]string, error) {
	m.calls++
	if m.vars != nil {
		return m.vars, nil
	}
	return map[string]string{}, nil
}

type noopScriptEngine struct{}

func (n *noopScriptEngine) RunPreScript(_ context.Context, _ string, _ request.ScriptContext) (*request.PreScriptResult, error) {
	return &request.PreScriptResult{}, nil
}

func (n *noopScriptEngine) RunPostScript(_ context.Context, _ string, _ request.ScriptContext) (*request.PostScriptResult, error) {
	return &request.PostScriptResult{}, nil
}

type noopVarPersister struct{}

func (n *noopVarPersister) PersistVariableChanges(_ context.Context, _ uuid.UUID, _ string, _ map[string]string) error {
	return nil
}

type noopScriptResolver struct{}

func (n *noopScriptResolver) ResolvePreScript(_ context.Context, _ *entities.Request) (string, error) {
	return "", nil
}

func (n *noopScriptResolver) ResolvePostScript(_ context.Context, _ *entities.Request) (string, error) {
	return "", nil
}

func newTestUsecase() (request.Usecase, *mockRepo, *mockHistoryRepo, *mockRequester) {
	repo := newMockRepo()
	historyRepo := &mockHistoryRepo{}
	requester := &mockRequester{}
	uc := request.NewUsecase(repo, historyRepo, requester, nil, nil, &mockEnvResolver{}, &noopScriptEngine{}, &noopScriptResolver{}, &noopVarPersister{}, request.NewAuthResolver(fixtureCollections()), nil, nil, nil, nil)
	return uc, repo, historyRepo, requester
}

func TestCreate(t *testing.T) {
	uc, _, _, _ := newTestUsecase()
	ctx := context.Background()

	input := request.Create{
		CollectionID: testCollectionID,
		Name:         "Get Users",
		Protocol:     entities.ProtocolHTTP,
		Method:       entities.MethodGET,
		URL:          "https://api.example.com/users",
		BodyType:     entities.BodyTypeNone,
		AuthType:     entities.AuthTypeNone,
	}
	opt := request.CreateOpt{UserID: "user-1"}

	result, err := uc.Create(ctx, input, opt)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil request")
	}
	if result.Name != "Get Users" {
		t.Errorf("expected name %q, got %q", "Get Users", result.Name)
	}
	if result.Protocol != entities.ProtocolHTTP {
		t.Errorf("expected protocol %q, got %q", entities.ProtocolHTTP, result.Protocol)
	}
	if result.Method != entities.MethodGET {
		t.Errorf("expected method %q, got %q", entities.MethodGET, result.Method)
	}
	if result.Version != 1 {
		t.Errorf("expected version 1, got %d", result.Version)
	}
	if result.CreatedBy != "user-1" {
		t.Errorf("expected CreatedBy %q, got %q", "user-1", result.CreatedBy)
	}
	if result.ID == uuid.Nil {
		t.Error("expected non-nil UUID for ID")
	}
	if result.CreatedAt.IsZero() {
		t.Error("expected non-zero CreatedAt")
	}
	if result.UpdatedAt.IsZero() {
		t.Error("expected non-zero UpdatedAt")
	}
}

func TestCreate_ValidationError_EmptyName(t *testing.T) {
	uc, _, _, _ := newTestUsecase()
	ctx := context.Background()

	input := request.Create{
		CollectionID: testCollectionID,
		Name:         "",
		Protocol:     entities.ProtocolHTTP,
		Method:       entities.MethodGET,
		BodyType:     entities.BodyTypeNone,
		AuthType:     entities.AuthTypeNone,
	}
	opt := request.CreateOpt{UserID: "user-1"}

	result, err := uc.Create(ctx, input, opt)
	if result != nil {
		t.Errorf("expected nil result, got %+v", result)
	}
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	var valErr *domain.ValidationError
	if !errors.As(err, &valErr) {
		t.Fatalf("expected *domain.ValidationError, got %T: %v", err, err)
	}

	msg, ok := valErr.Fields["name"]
	if !ok {
		t.Fatal("expected validation error for field 'name'")
	}
	if msg != "required" {
		t.Errorf("expected field message %q, got %q", "required", msg)
	}
}

func TestCreate_ValidationError_InvalidMethod(t *testing.T) {
	uc, _, _, _ := newTestUsecase()
	ctx := context.Background()

	input := request.Create{
		CollectionID: testCollectionID,
		Name:         "Bad Method",
		Protocol:     entities.ProtocolHTTP,
		Method:       entities.HTTPMethod("INVALID"),
		BodyType:     entities.BodyTypeNone,
		AuthType:     entities.AuthTypeNone,
	}
	opt := request.CreateOpt{UserID: "user-1"}

	result, err := uc.Create(ctx, input, opt)
	if result != nil {
		t.Errorf("expected nil result, got %+v", result)
	}
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	var valErr *domain.ValidationError
	if !errors.As(err, &valErr) {
		t.Fatalf("expected *domain.ValidationError, got %T: %v", err, err)
	}

	msg, ok := valErr.Fields["method"]
	if !ok {
		t.Fatal("expected validation error for field 'method'")
	}
	if msg != "invalid" {
		t.Errorf("expected field message %q, got %q", "invalid", msg)
	}
}

func TestCreate_ValidationError_InvalidBodyType(t *testing.T) {
	uc, _, _, _ := newTestUsecase()
	ctx := context.Background()

	input := request.Create{
		CollectionID: testCollectionID,
		Name:         "Bad Body",
		Protocol:     entities.ProtocolHTTP,
		Method:       entities.MethodPOST,
		BodyType:     entities.BodyType("html"),
		AuthType:     entities.AuthTypeNone,
	}
	opt := request.CreateOpt{UserID: "user-1"}

	result, err := uc.Create(ctx, input, opt)
	if result != nil {
		t.Errorf("expected nil result, got %+v", result)
	}
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	var valErr *domain.ValidationError
	if !errors.As(err, &valErr) {
		t.Fatalf("expected *domain.ValidationError, got %T: %v", err, err)
	}

	msg, ok := valErr.Fields["bodyType"]
	if !ok {
		t.Fatal("expected validation error for field 'bodyType'")
	}
	if msg != "invalid" {
		t.Errorf("expected field message %q, got %q", "invalid", msg)
	}
}

func TestEdit(t *testing.T) {
	uc, _, _, _ := newTestUsecase()
	ctx := context.Background()

	created, err := uc.Create(ctx, request.Create{
		CollectionID: testCollectionID,
		Name:         "Original",
		Protocol:     entities.ProtocolHTTP,
		Method:       entities.MethodGET,
		URL:          "https://api.example.com/old",
		BodyType:     entities.BodyTypeNone,
		AuthType:     entities.AuthTypeNone,
	}, request.CreateOpt{UserID: "user-1"})
	if err != nil {
		t.Fatalf("setup: unexpected error: %v", err)
	}

	input := request.Edit{
		Name:     "Renamed",
		Method:   entities.MethodPOST,
		URL:      "https://api.example.com/new",
		BodyType: entities.BodyTypeJSON,
		AuthType: entities.AuthTypeNone,
	}
	opt := request.EditOpt{
		RequestID: created.ID,
		UserID:    "user-2",
		Version:   created.Version,
	}

	edited, err := uc.Edit(ctx, input, opt)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if edited.Name != "Renamed" {
		t.Errorf("expected name %q, got %q", "Renamed", edited.Name)
	}
	if edited.Method != entities.MethodPOST {
		t.Errorf("expected method %q, got %q", entities.MethodPOST, edited.Method)
	}
	if edited.URL != "https://api.example.com/new" {
		t.Errorf("expected URL %q, got %q", "https://api.example.com/new", edited.URL)
	}
	if edited.Version != created.Version+1 {
		t.Errorf("expected version %d, got %d", created.Version+1, edited.Version)
	}
	if edited.UpdatedBy != "user-2" {
		t.Errorf("expected UpdatedBy %q, got %q", "user-2", edited.UpdatedBy)
	}
}

func TestEdit_NotFound(t *testing.T) {
	uc, _, _, _ := newTestUsecase()
	ctx := context.Background()

	input := request.Edit{
		Name:     "Ghost",
		Method:   entities.MethodGET,
		BodyType: entities.BodyTypeNone,
		AuthType: entities.AuthTypeNone,
	}
	opt := request.EditOpt{
		RequestID: uuid.New(),
		UserID:    "user-1",
		Version:   1,
	}

	result, err := uc.Edit(ctx, input, opt)
	if result != nil {
		t.Errorf("expected nil result, got %+v", result)
	}
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	var notFoundErr *domain.NotFoundError
	if !errors.As(err, &notFoundErr) {
		t.Fatalf("expected *domain.NotFoundError, got %T: %v", err, err)
	}
	if notFoundErr.Entity != "request" {
		t.Errorf("expected entity %q, got %q", "request", notFoundErr.Entity)
	}
}

func TestEdit_VersionConflict(t *testing.T) {
	uc, _, _, _ := newTestUsecase()
	ctx := context.Background()

	created, err := uc.Create(ctx, request.Create{
		CollectionID: testCollectionID,
		Name:         "Original",
		Protocol:     entities.ProtocolHTTP,
		Method:       entities.MethodGET,
		BodyType:     entities.BodyTypeNone,
		AuthType:     entities.AuthTypeNone,
	}, request.CreateOpt{UserID: "user-1"})
	if err != nil {
		t.Fatalf("setup: unexpected error: %v", err)
	}

	wrongVersion := created.Version + 99
	input := request.Edit{
		Name:     "Should Fail",
		Method:   entities.MethodGET,
		BodyType: entities.BodyTypeNone,
		AuthType: entities.AuthTypeNone,
	}
	opt := request.EditOpt{
		RequestID: created.ID,
		UserID:    "user-2",
		Version:   wrongVersion,
	}

	result, err := uc.Edit(ctx, input, opt)
	if result != nil {
		t.Errorf("expected nil result, got %+v", result)
	}
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	var conflictErr *domain.ConflictError
	if !errors.As(err, &conflictErr) {
		t.Fatalf("expected *domain.ConflictError, got %T: %v", err, err)
	}
	if conflictErr.Entity != "request" {
		t.Errorf("expected entity %q, got %q", "request", conflictErr.Entity)
	}
}

func TestDelete(t *testing.T) {
	uc, _, _, _ := newTestUsecase()
	ctx := context.Background()

	created, err := uc.Create(ctx, request.Create{
		CollectionID: testCollectionID,
		Name:         "To Delete",
		Protocol:     entities.ProtocolHTTP,
		Method:       entities.MethodGET,
		BodyType:     entities.BodyTypeNone,
		AuthType:     entities.AuthTypeNone,
	}, request.CreateOpt{UserID: "user-1"})
	if err != nil {
		t.Fatalf("setup: unexpected error: %v", err)
	}

	err = uc.Delete(ctx, request.DeleteOpt{
		RequestID: created.ID,
		UserID:    "user-1",
		Version:   created.Version,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	list, err := uc.List(ctx, request.ListOpt{CollectionID: testCollectionID})
	if err != nil {
		t.Fatalf("unexpected error listing: %v", err)
	}
	for _, r := range list {
		if r.ID == created.ID {
			t.Errorf("deleted request %s should not appear in list", created.ID)
		}
	}
}

func TestDelete_NotFound(t *testing.T) {
	uc, _, _, _ := newTestUsecase()
	ctx := context.Background()

	err := uc.Delete(ctx, request.DeleteOpt{
		RequestID: uuid.New(),
		UserID:    "user-1",
		Version:   1,
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	var notFoundErr *domain.NotFoundError
	if !errors.As(err, &notFoundErr) {
		t.Fatalf("expected *domain.NotFoundError, got %T: %v", err, err)
	}
	if notFoundErr.Entity != "request" {
		t.Errorf("expected entity %q, got %q", "request", notFoundErr.Entity)
	}
}

func TestDelete_VersionConflict(t *testing.T) {
	uc, _, _, _ := newTestUsecase()
	ctx := context.Background()

	created, err := uc.Create(ctx, request.Create{
		CollectionID: testCollectionID,
		Name:         "Conflict Delete",
		Protocol:     entities.ProtocolHTTP,
		Method:       entities.MethodGET,
		BodyType:     entities.BodyTypeNone,
		AuthType:     entities.AuthTypeNone,
	}, request.CreateOpt{UserID: "user-1"})
	if err != nil {
		t.Fatalf("setup: unexpected error: %v", err)
	}

	wrongVersion := created.Version + 99
	err = uc.Delete(ctx, request.DeleteOpt{
		RequestID: created.ID,
		UserID:    "user-1",
		Version:   wrongVersion,
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	var conflictErr *domain.ConflictError
	if !errors.As(err, &conflictErr) {
		t.Fatalf("expected *domain.ConflictError, got %T: %v", err, err)
	}
	if conflictErr.Entity != "request" {
		t.Errorf("expected entity %q, got %q", "request", conflictErr.Entity)
	}
}

func TestList(t *testing.T) {
	uc, _, _, _ := newTestUsecase()
	ctx := context.Background()

	_, err := uc.Create(ctx, request.Create{
		CollectionID: testCollectionID,
		Name:         "Request A",
		Protocol:     entities.ProtocolHTTP,
		Method:       entities.MethodGET,
		BodyType:     entities.BodyTypeNone,
		AuthType:     entities.AuthTypeNone,
	}, request.CreateOpt{UserID: "user-1"})
	if err != nil {
		t.Fatalf("setup: unexpected error: %v", err)
	}

	_, err = uc.Create(ctx, request.Create{
		CollectionID: testCollectionID,
		Name:         "Request B",
		Protocol:     entities.ProtocolHTTP,
		Method:       entities.MethodPOST,
		BodyType:     entities.BodyTypeJSON,
		AuthType:     entities.AuthTypeNone,
	}, request.CreateOpt{UserID: "user-1"})
	if err != nil {
		t.Fatalf("setup: unexpected error: %v", err)
	}

	list, err := uc.List(ctx, request.ListOpt{CollectionID: testCollectionID})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("expected 2 requests, got %d", len(list))
	}

	names := map[string]bool{}
	for _, r := range list {
		names[r.Name] = true
	}
	if !names["Request A"] {
		t.Error("expected 'Request A' in list")
	}
	if !names["Request B"] {
		t.Error("expected 'Request B' in list")
	}
}

func TestGetByID_NotFound(t *testing.T) {
	uc, _, _, _ := newTestUsecase()
	ctx := context.Background()

	nonExistentID := uuid.New()

	result, err := uc.GetByID(ctx, nonExistentID)
	if result != nil {
		t.Errorf("expected nil result, got %+v", result)
	}
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	var notFoundErr *domain.NotFoundError
	if !errors.As(err, &notFoundErr) {
		t.Fatalf("expected *domain.NotFoundError, got %T: %v", err, err)
	}
	if notFoundErr.Entity != "request" {
		t.Errorf("expected entity %q, got %q", "request", notFoundErr.Entity)
	}
	if notFoundErr.ID != nonExistentID.String() {
		t.Errorf("expected ID %q, got %q", nonExistentID.String(), notFoundErr.ID)
	}
}

func TestExecute_Success(t *testing.T) {
	uc, _, historyRepo, requester := newTestUsecase()
	ctx := context.Background()

	created, err := uc.Create(ctx, request.Create{
		CollectionID: testCollectionID,
		Name:         "Execute Test",
		Protocol:     entities.ProtocolHTTP,
		Method:       entities.MethodGET,
		URL:          "https://api.example.com/users",
		Headers:      []entities.HeaderItem{{Key: "Accept", Value: "application/json", Enabled: true}},
		BodyType:     entities.BodyTypeNone,
		AuthType:     entities.AuthTypeNone,
	}, request.CreateOpt{UserID: "user-1"})
	if err != nil {
		t.Fatalf("setup: %v", err)
	}

	requester.response = &entities.Response{
		StatusCode: 200,
		StatusText: "OK",
		Headers:    map[string][]string{"Content-Type": {"application/json"}},
		Body:       `{"users":[]}`,
		Size:       12,
		Duration:   150 * time.Millisecond,
		Protocol:   entities.ProtocolHTTP,
	}
	requester.err = nil

	opt := request.ExecuteOpt{
		UserID:      "user-1",
		WorkspaceID: testWorkspaceID,
	}

	resp, err := uc.Execute(ctx, created.ID, opt)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != 200 {
		t.Errorf("expected status 200, got %d", resp.StatusCode)
	}
	if resp.Body != `{"users":[]}` {
		t.Errorf("unexpected body: %s", resp.Body)
	}

	if len(historyRepo.entries) != 1 {
		t.Fatalf("expected 1 history entry, got %d", len(historyRepo.entries))
	}
	h := historyRepo.entries[0]
	if h.RequestID != created.ID {
		t.Errorf("expected request ID %s, got %s", created.ID, h.RequestID)
	}
	if h.ResponseStatus != 200 {
		t.Errorf("expected response status 200, got %d", h.ResponseStatus)
	}
	if h.ErrorMessage != "" {
		t.Errorf("expected empty error message, got %q", h.ErrorMessage)
	}
	if h.URL != "https://api.example.com/users" {
		t.Errorf("expected URL %q, got %q", "https://api.example.com/users", h.URL)
	}
}

func TestExecute_NetworkError(t *testing.T) {
	uc, _, historyRepo, requester := newTestUsecase()
	ctx := context.Background()

	created, err := uc.Create(ctx, request.Create{
		CollectionID: testCollectionID,
		Name:         "Error Test",
		Protocol:     entities.ProtocolHTTP,
		Method:       entities.MethodGET,
		URL:          "https://unreachable.example.com",
		BodyType:     entities.BodyTypeNone,
		AuthType:     entities.AuthTypeNone,
	}, request.CreateOpt{UserID: "user-1"})
	if err != nil {
		t.Fatalf("setup: %v", err)
	}

	requester.response = nil
	requester.err = errors.New("connection refused")

	opt := request.ExecuteOpt{
		UserID:      "user-1",
		WorkspaceID: testWorkspaceID,
	}

	resp, err := uc.Execute(ctx, created.ID, opt)
	if resp != nil {
		t.Errorf("expected nil response, got %+v", resp)
	}
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	if len(historyRepo.entries) != 1 {
		t.Fatalf("expected 1 history entry, got %d", len(historyRepo.entries))
	}
	h := historyRepo.entries[0]
	if h.ErrorMessage == "" {
		t.Error("expected non-empty error message in history")
	}
	if h.ResponseStatus != 0 {
		t.Errorf("expected response status 0, got %d", h.ResponseStatus)
	}
}

func TestExecute_NotFound(t *testing.T) {
	uc, _, _, _ := newTestUsecase()
	ctx := context.Background()

	opt := request.ExecuteOpt{
		UserID:      "user-1",
		WorkspaceID: testWorkspaceID,
	}

	resp, err := uc.Execute(ctx, uuid.New(), opt)
	if resp != nil {
		t.Errorf("expected nil response, got %+v", resp)
	}
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	var notFoundErr *domain.NotFoundError
	if !errors.As(err, &notFoundErr) {
		t.Fatalf("expected *domain.NotFoundError, got %T: %v", err, err)
	}
}

func TestCreate_WithAuth(t *testing.T) {
	uc, _, _, _ := newTestUsecase()
	ctx := context.Background()

	input := request.Create{
		CollectionID: testCollectionID,
		Name:         "Auth Request",
		Protocol:     entities.ProtocolHTTP,
		Method:       entities.MethodGET,
		URL:          "https://api.example.com/users",
		BodyType:     entities.BodyTypeNone,
		AuthType:     entities.AuthTypeBasic,
		AuthData:     `{"username":"admin","password":"secret"}`,
	}
	opt := request.CreateOpt{UserID: "user-1"}

	result, err := uc.Create(ctx, input, opt)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.AuthType != entities.AuthTypeBasic {
		t.Errorf("expected auth_type %q, got %q", entities.AuthTypeBasic, result.AuthType)
	}
	if result.AuthData != `{"username":"admin","password":"secret"}` {
		t.Errorf("unexpected auth_data: %s", result.AuthData)
	}
}

func TestCreate_ValidationError_InvalidAuthType(t *testing.T) {
	uc, _, _, _ := newTestUsecase()
	ctx := context.Background()

	input := request.Create{
		CollectionID: testCollectionID,
		Name:         "Bad Auth",
		Protocol:     entities.ProtocolHTTP,
		Method:       entities.MethodGET,
		BodyType:     entities.BodyTypeNone,
		AuthType:     entities.AuthType("hawk"),
	}
	opt := request.CreateOpt{UserID: "user-1"}

	_, err := uc.Create(ctx, input, opt)
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	var valErr *domain.ValidationError
	if !errors.As(err, &valErr) {
		t.Fatalf("expected *domain.ValidationError, got %T: %v", err, err)
	}
	if _, ok := valErr.Fields["authType"]; !ok {
		t.Fatal("expected validation error for field 'authType'")
	}
}

func TestEdit_WithAuth(t *testing.T) {
	uc, _, _, _ := newTestUsecase()
	ctx := context.Background()

	created, err := uc.Create(ctx, request.Create{
		CollectionID: testCollectionID,
		Name:         "No Auth",
		Protocol:     entities.ProtocolHTTP,
		Method:       entities.MethodGET,
		BodyType:     entities.BodyTypeNone,
		AuthType:     entities.AuthTypeNone,
	}, request.CreateOpt{UserID: "user-1"})
	if err != nil {
		t.Fatalf("setup: %v", err)
	}

	edited, err := uc.Edit(ctx, request.Edit{
		Name:     "With Auth",
		Method:   entities.MethodGET,
		BodyType: entities.BodyTypeNone,
		AuthType: entities.AuthTypeBearer,
		AuthData: `{"token":"eyJhbG...","prefix":"Bearer"}`,
	}, request.EditOpt{
		RequestID: created.ID,
		UserID:    "user-1",
		Version:   created.Version,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if edited.AuthType != entities.AuthTypeBearer {
		t.Errorf("expected auth_type %q, got %q", entities.AuthTypeBearer, edited.AuthType)
	}
}

func TestExecute_BasicAuth_HeaderApplied(t *testing.T) {
	uc, _, _, requester := newTestUsecase()
	ctx := context.Background()

	created, err := uc.Create(ctx, request.Create{
		CollectionID: testCollectionID,
		Name:         "Basic Auth Verify",
		Protocol:     entities.ProtocolHTTP,
		Method:       entities.MethodGET,
		URL:          "https://api.example.com",
		BodyType:     entities.BodyTypeNone,
		AuthType:     entities.AuthTypeBasic,
		AuthData:     `{"username":"admin","password":"secret"}`,
	}, request.CreateOpt{UserID: "user-1"})
	if err != nil {
		t.Fatalf("setup: %v", err)
	}

	requester.response = &entities.Response{StatusCode: 200, StatusText: "OK", Headers: map[string][]string{}, Duration: 10 * time.Millisecond}
	opt := request.ExecuteOpt{UserID: "user-1", WorkspaceID: testWorkspaceID}

	_, err = uc.Execute(ctx, created.ID, opt)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	authHeader := requester.lastRequest.Headers["Authorization"]
	if len(authHeader) == 0 {
		t.Fatal("expected Authorization header to be set")
	}
	// base64("admin:secret") = "YWRtaW46c2VjcmV0"
	expected := "Basic YWRtaW46c2VjcmV0"
	if authHeader[0] != expected {
		t.Errorf("expected Authorization %q, got %q", expected, authHeader[0])
	}
}

func TestExecute_APIKey_Query(t *testing.T) {
	uc, _, _, requester := newTestUsecase()
	ctx := context.Background()

	created, err := uc.Create(ctx, request.Create{
		CollectionID: testCollectionID,
		Name:         "API Key Query",
		Protocol:     entities.ProtocolHTTP,
		Method:       entities.MethodGET,
		URL:          "https://api.example.com/data",
		BodyType:     entities.BodyTypeNone,
		AuthType:     entities.AuthTypeAPIKey,
		AuthData:     `{"key":"api_key","value":"sk_live_123","addTo":"query"}`,
	}, request.CreateOpt{UserID: "user-1"})
	if err != nil {
		t.Fatalf("setup: %v", err)
	}

	requester.response = &entities.Response{StatusCode: 200, StatusText: "OK", Headers: map[string][]string{}, Duration: 10 * time.Millisecond}
	opt := request.ExecuteOpt{UserID: "user-1", WorkspaceID: testWorkspaceID}

	_, err = uc.Execute(ctx, created.ID, opt)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(requester.lastRequest.URL, "api_key=sk_live_123") {
		t.Errorf("expected URL to contain api_key param, got %q", requester.lastRequest.URL)
	}
}

func TestExecute_AutoContentType_JSON(t *testing.T) {
	uc, _, _, requester := newTestUsecase()
	ctx := context.Background()

	created, err := uc.Create(ctx, request.Create{
		CollectionID: testCollectionID,
		Name:         "JSON Body",
		Protocol:     entities.ProtocolHTTP,
		Method:       entities.MethodPOST,
		URL:          "https://api.example.com/data",
		Body:         `{"name":"test"}`,
		BodyType:     entities.BodyTypeJSON,
		AuthType:     entities.AuthTypeNone,
	}, request.CreateOpt{UserID: "user-1"})
	if err != nil {
		t.Fatalf("setup: %v", err)
	}

	requester.response = &entities.Response{StatusCode: 200, StatusText: "OK", Headers: map[string][]string{}, Duration: 10 * time.Millisecond}
	opt := request.ExecuteOpt{UserID: "user-1", WorkspaceID: testWorkspaceID}

	_, err = uc.Execute(ctx, created.ID, opt)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	ct := requester.lastRequest.Headers["Content-Type"]
	if len(ct) == 0 || ct[0] != "application/json" {
		t.Errorf("expected Content-Type application/json, got %v", ct)
	}
}

func TestExecute_BinaryBody_RejectsPathTraversal(t *testing.T) {
	uc, _, _, requester := newTestUsecase()
	ctx := context.Background()

	created, err := uc.Create(ctx, request.Create{
		CollectionID: testCollectionID,
		Name:         "Traversal Test",
		Protocol:     entities.ProtocolHTTP,
		Method:       entities.MethodPOST,
		URL:          "https://api.example.com/upload",
		Body:         "/Users/test/../../../etc/passwd",
		BodyType:     entities.BodyTypeBinary,
		AuthType:     entities.AuthTypeNone,
	}, request.CreateOpt{UserID: "user-1"})
	if err != nil {
		t.Fatalf("setup: %v", err)
	}

	requester.response = &entities.Response{StatusCode: 200, StatusText: "OK", Headers: map[string][]string{}, Duration: 10 * time.Millisecond}
	opt := request.ExecuteOpt{UserID: "user-1", WorkspaceID: testWorkspaceID}

	_, err = uc.Execute(ctx, created.ID, opt)
	if err == nil {
		t.Fatal("expected error for path traversal, got nil")
	}
	if !strings.Contains(err.Error(), "path traversal") {
		t.Errorf("expected 'path traversal' in error, got: %v", err)
	}
}

func TestExecute_FormBody(t *testing.T) {
	uc, _, _, requester := newTestUsecase()
	ctx := context.Background()

	created, err := uc.Create(ctx, request.Create{
		CollectionID: testCollectionID,
		Name:         "Form Request",
		Protocol:     entities.ProtocolHTTP,
		Method:       entities.MethodPOST,
		URL:          "https://api.example.com/login",
		Body:         `[{"key":"username","value":"admin","enabled":true},{"key":"password","value":"secret","enabled":true}]`,
		BodyType:     entities.BodyTypeForm,
		AuthType:     entities.AuthTypeNone,
	}, request.CreateOpt{UserID: "user-1"})
	if err != nil {
		t.Fatalf("setup: %v", err)
	}

	requester.response = &entities.Response{StatusCode: 200, StatusText: "OK", Headers: map[string][]string{}, Duration: 10 * time.Millisecond}
	opt := request.ExecuteOpt{UserID: "user-1", WorkspaceID: testWorkspaceID}

	_, err = uc.Execute(ctx, created.ID, opt)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	body := requester.lastRequest.Body
	if !strings.Contains(body, "username=admin") || !strings.Contains(body, "password=secret") {
		t.Errorf("expected form-encoded body, got %q", body)
	}

	ct := requester.lastRequest.Headers["Content-Type"]
	if len(ct) == 0 || ct[0] != "application/x-www-form-urlencoded" {
		t.Errorf("expected Content-Type application/x-www-form-urlencoded, got %v", ct)
	}
}

func TestExecute_BearerAuth_EmptyPrefix(t *testing.T) {
	uc, _, _, requester := newTestUsecase()
	ctx := context.Background()

	created, err := uc.Create(ctx, request.Create{
		CollectionID: testCollectionID,
		Name:         "Bearer No Prefix",
		Protocol:     entities.ProtocolHTTP,
		Method:       entities.MethodGET,
		URL:          "https://api.example.com",
		BodyType:     entities.BodyTypeNone,
		AuthType:     entities.AuthTypeBearer,
		AuthData:     `{"token":"my-token","prefix":""}`,
	}, request.CreateOpt{UserID: "user-1"})
	if err != nil {
		t.Fatalf("setup: %v", err)
	}

	requester.response = &entities.Response{StatusCode: 200, StatusText: "OK", Headers: map[string][]string{}, Duration: 10 * time.Millisecond}
	opt := request.ExecuteOpt{UserID: "user-1", WorkspaceID: testWorkspaceID}

	_, err = uc.Execute(ctx, created.ID, opt)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	authHeader := requester.lastRequest.Headers["Authorization"]
	if len(authHeader) == 0 {
		t.Fatal("expected Authorization header")
	}
	// Empty prefix means token only, no leading space
	if authHeader[0] != "my-token" {
		t.Errorf("expected %q, got %q", "my-token", authHeader[0])
	}
}

func TestExecute_BearerAuth_DefaultPrefix(t *testing.T) {
	uc, _, _, requester := newTestUsecase()
	ctx := context.Background()

	// No "prefix" key at all in auth data — should default to "Bearer"
	created, err := uc.Create(ctx, request.Create{
		CollectionID: testCollectionID,
		Name:         "Bearer Default",
		Protocol:     entities.ProtocolHTTP,
		Method:       entities.MethodGET,
		URL:          "https://api.example.com",
		BodyType:     entities.BodyTypeNone,
		AuthType:     entities.AuthTypeBearer,
		AuthData:     `{"token":"my-token"}`,
	}, request.CreateOpt{UserID: "user-1"})
	if err != nil {
		t.Fatalf("setup: %v", err)
	}

	requester.response = &entities.Response{StatusCode: 200, StatusText: "OK", Headers: map[string][]string{}, Duration: 10 * time.Millisecond}
	opt := request.ExecuteOpt{UserID: "user-1", WorkspaceID: testWorkspaceID}

	_, err = uc.Execute(ctx, created.ID, opt)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	authHeader := requester.lastRequest.Headers["Authorization"]
	if len(authHeader) == 0 {
		t.Fatal("expected Authorization header")
	}
	if authHeader[0] != "Bearer my-token" {
		t.Errorf("expected %q, got %q", "Bearer my-token", authHeader[0])
	}
}

func TestExecute_APIKey_RejectsReservedHeaders(t *testing.T) {
	reservedHeaders := []string{"Host", "Content-Length", "Authorization", "host", "CONTENT-LENGTH"}

	for _, header := range reservedHeaders {
		t.Run(header, func(t *testing.T) {
			uc, _, _, requester := newTestUsecase()
			ctx := context.Background()

			authData := fmt.Sprintf(`{"key":"%s","value":"test","addTo":"header"}`, header)
			created, err := uc.Create(ctx, request.Create{
				CollectionID: testCollectionID,
				Name:         "Reserved Header " + header,
				Protocol:     entities.ProtocolHTTP,
				Method:       entities.MethodGET,
				URL:          "https://api.example.com",
				BodyType:     entities.BodyTypeNone,
				AuthType:     entities.AuthTypeAPIKey,
				AuthData:     authData,
			}, request.CreateOpt{UserID: "user-1"})
			if err != nil {
				t.Fatalf("setup: %v", err)
			}

			requester.response = &entities.Response{StatusCode: 200, StatusText: "OK", Headers: map[string][]string{}, Duration: 10 * time.Millisecond}
			opt := request.ExecuteOpt{UserID: "user-1", WorkspaceID: testWorkspaceID}

			_, err = uc.Execute(ctx, created.ID, opt)
			if err == nil {
				t.Fatalf("expected error for reserved header %q, got nil", header)
			}
			if !strings.Contains(err.Error(), "reserved") {
				t.Errorf("expected 'reserved' in error, got: %v", err)
			}
		})
	}
}

func newTestUsecaseWithVars(vars map[string]string) (request.Usecase, *mockRepo, *mockHistoryRepo, *mockRequester) {
	repo := newMockRepo()
	historyRepo := &mockHistoryRepo{}
	requester := &mockRequester{}
	uc := request.NewUsecase(repo, historyRepo, requester, nil, nil, &mockEnvResolver{vars: vars}, &noopScriptEngine{}, &noopScriptResolver{}, &noopVarPersister{}, request.NewAuthResolver(fixtureCollections()), nil, nil, nil, nil)
	return uc, repo, historyRepo, requester
}

func TestExecute_SubstitutesURL(t *testing.T) {
	vars := map[string]string{"base_url": "https://api.staging.example.com"}
	uc, _, _, requester := newTestUsecaseWithVars(vars)
	ctx := context.Background()

	created, _ := uc.Create(ctx, request.Create{
		CollectionID: testCollectionID,
		Name:         "Var URL",
		Protocol:     entities.ProtocolHTTP,
		Method:       entities.MethodGET,
		URL:          "{{base_url}}/users",
		BodyType:     entities.BodyTypeNone,
		AuthType:     entities.AuthTypeNone,
	}, request.CreateOpt{UserID: "user-1"})

	requester.response = &entities.Response{StatusCode: 200, StatusText: "OK", Headers: map[string][]string{}, Duration: 10 * time.Millisecond}
	opt := request.ExecuteOpt{UserID: "user-1", WorkspaceID: testWorkspaceID}

	_, err := uc.Execute(ctx, created.ID, opt)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if requester.lastRequest.URL != "https://api.staging.example.com/users" {
		t.Errorf("expected substituted URL, got %q", requester.lastRequest.URL)
	}
}

func TestExecute_SubstitutesHeaders(t *testing.T) {
	vars := map[string]string{"api_key": "sk-123"}
	uc, _, _, requester := newTestUsecaseWithVars(vars)
	ctx := context.Background()

	created, _ := uc.Create(ctx, request.Create{
		CollectionID: testCollectionID,
		Name:         "Var Headers",
		Protocol:     entities.ProtocolHTTP,
		Method:       entities.MethodGET,
		URL:          "https://api.example.com",
		Headers:      []entities.HeaderItem{{Key: "X-API-Key", Value: "{{api_key}}", Enabled: true}},
		BodyType:     entities.BodyTypeNone,
		AuthType:     entities.AuthTypeNone,
	}, request.CreateOpt{UserID: "user-1"})

	requester.response = &entities.Response{StatusCode: 200, StatusText: "OK", Headers: map[string][]string{}, Duration: 10 * time.Millisecond}
	opt := request.ExecuteOpt{UserID: "user-1", WorkspaceID: testWorkspaceID}

	_, err := uc.Execute(ctx, created.ID, opt)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	apiKey := requester.lastRequest.Headers["X-API-Key"]
	if len(apiKey) == 0 || apiKey[0] != "sk-123" {
		t.Errorf("expected header X-API-Key=sk-123, got %v", apiKey)
	}
}

func TestExecute_SubstitutesBody(t *testing.T) {
	vars := map[string]string{"user_name": "Alice"}
	uc, _, _, requester := newTestUsecaseWithVars(vars)
	ctx := context.Background()

	created, _ := uc.Create(ctx, request.Create{
		CollectionID: testCollectionID,
		Name:         "Var Body",
		Protocol:     entities.ProtocolHTTP,
		Method:       entities.MethodPOST,
		URL:          "https://api.example.com/users",
		Body:         `{"name":"{{user_name}}"}`,
		BodyType:     entities.BodyTypeJSON,
		AuthType:     entities.AuthTypeNone,
	}, request.CreateOpt{UserID: "user-1"})

	requester.response = &entities.Response{StatusCode: 200, StatusText: "OK", Headers: map[string][]string{}, Duration: 10 * time.Millisecond}
	opt := request.ExecuteOpt{UserID: "user-1", WorkspaceID: testWorkspaceID}

	_, err := uc.Execute(ctx, created.ID, opt)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if requester.lastRequest.Body != `{"name":"Alice"}` {
		t.Errorf("expected substituted body, got %q", requester.lastRequest.Body)
	}
}

func TestExecute_UnresolvedVarsLeftAsIs(t *testing.T) {
	uc, _, _, requester := newTestUsecaseWithVars(map[string]string{})
	ctx := context.Background()

	created, _ := uc.Create(ctx, request.Create{
		CollectionID: testCollectionID,
		Name:         "Unresolved",
		Protocol:     entities.ProtocolHTTP,
		Method:       entities.MethodGET,
		URL:          "{{unknown_var}}/users",
		BodyType:     entities.BodyTypeNone,
		AuthType:     entities.AuthTypeNone,
	}, request.CreateOpt{UserID: "user-1"})

	requester.response = &entities.Response{StatusCode: 200, StatusText: "OK", Headers: map[string][]string{}, Duration: 10 * time.Millisecond}
	opt := request.ExecuteOpt{UserID: "user-1", WorkspaceID: testWorkspaceID}

	_, err := uc.Execute(ctx, created.ID, opt)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if requester.lastRequest.URL != "{{unknown_var}}/users" {
		t.Errorf("expected unresolved var to stay, got %q", requester.lastRequest.URL)
	}
}

func TestExecute_SubstitutesAuthData(t *testing.T) {
	vars := map[string]string{"my_token": "sk-secret-token"}
	uc, _, _, requester := newTestUsecaseWithVars(vars)
	ctx := context.Background()

	created, _ := uc.Create(ctx, request.Create{
		CollectionID: testCollectionID,
		Name:         "Var Auth",
		Protocol:     entities.ProtocolHTTP,
		Method:       entities.MethodGET,
		URL:          "https://api.example.com",
		BodyType:     entities.BodyTypeNone,
		AuthType:     entities.AuthTypeBearer,
		AuthData:     `{"token":"{{my_token}}"}`,
	}, request.CreateOpt{UserID: "user-1"})

	requester.response = &entities.Response{StatusCode: 200, StatusText: "OK", Headers: map[string][]string{}, Duration: 10 * time.Millisecond}
	opt := request.ExecuteOpt{UserID: "user-1", WorkspaceID: testWorkspaceID}

	_, err := uc.Execute(ctx, created.ID, opt)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	authHeader := requester.lastRequest.Headers["Authorization"]
	if len(authHeader) == 0 {
		t.Fatal("expected Authorization header")
	}
	if authHeader[0] != "Bearer sk-secret-token" {
		t.Errorf("expected 'Bearer sk-secret-token', got %q", authHeader[0])
	}
}

func TestMove(t *testing.T) {
	uc, repo, _, _ := newTestUsecase()
	ctx := context.Background()

	collA := uuid.New()
	collB := uuid.New()

	created, err := uc.Create(ctx, request.Create{
		CollectionID: collA,
		Name:         "Move Me",
		Protocol:     entities.ProtocolHTTP,
		Method:       entities.MethodGET,
		BodyType:     entities.BodyTypeNone,
		AuthType:     entities.AuthTypeNone,
	}, request.CreateOpt{UserID: "user-1"})
	if err != nil {
		t.Fatalf("setup: %v", err)
	}
	_ = repo

	moved, err := uc.Move(ctx, request.MoveOpt{
		RequestID:          created.ID,
		TargetCollectionID: collB,
		UserID:             "user-1",
		Version:            created.Version,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if moved.CollectionID != collB {
		t.Errorf("expected collectionID %s, got %s", collB, moved.CollectionID)
	}
	if moved.Version != created.Version+1 {
		t.Errorf("expected version %d, got %d", created.Version+1, moved.Version)
	}
}

func TestMove_NotFound(t *testing.T) {
	uc, _, _, _ := newTestUsecase()
	ctx := context.Background()

	_, err := uc.Move(ctx, request.MoveOpt{
		RequestID:          uuid.New(),
		TargetCollectionID: uuid.New(),
		UserID:             "user-1",
		Version:            1,
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	var notFoundErr *domain.NotFoundError
	if !errors.As(err, &notFoundErr) {
		t.Fatalf("expected *domain.NotFoundError, got %T: %v", err, err)
	}
}

func TestMove_VersionConflict(t *testing.T) {
	uc, _, _, _ := newTestUsecase()
	ctx := context.Background()

	created, err := uc.Create(ctx, request.Create{
		CollectionID: testCollectionID,
		Name:         "Conflict Move",
		Protocol:     entities.ProtocolHTTP,
		Method:       entities.MethodGET,
		BodyType:     entities.BodyTypeNone,
		AuthType:     entities.AuthTypeNone,
	}, request.CreateOpt{UserID: "user-1"})
	if err != nil {
		t.Fatalf("setup: %v", err)
	}

	_, err = uc.Move(ctx, request.MoveOpt{
		RequestID:          created.ID,
		TargetCollectionID: uuid.New(),
		UserID:             "user-1",
		Version:            created.Version + 99,
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	var conflictErr *domain.ConflictError
	if !errors.As(err, &conflictErr) {
		t.Fatalf("expected *domain.ConflictError, got %T: %v", err, err)
	}
}

func TestExecute_DisabledHeadersNotSent(t *testing.T) {
	uc, _, _, requester := newTestUsecase()
	ctx := context.Background()

	created, err := uc.Create(ctx, request.Create{
		CollectionID: testCollectionID,
		Name:         "Disabled Headers",
		Protocol:     entities.ProtocolHTTP,
		Method:       entities.MethodGET,
		URL:          "https://api.example.com",
		Headers: []entities.HeaderItem{
			{Key: "X-Active", Value: "yes", Enabled: true},
			{Key: "X-Disabled", Value: "no", Enabled: false},
			{Key: "Accept", Value: "application/json", Enabled: true},
		},
		BodyType: entities.BodyTypeNone,
		AuthType: entities.AuthTypeNone,
	}, request.CreateOpt{UserID: "user-1"})
	if err != nil {
		t.Fatalf("setup: %v", err)
	}

	requester.response = &entities.Response{StatusCode: 200, StatusText: "OK", Headers: map[string][]string{}, Duration: 10 * time.Millisecond}
	opt := request.ExecuteOpt{UserID: "user-1", WorkspaceID: testWorkspaceID}

	_, err = uc.Execute(ctx, created.ID, opt)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	sent := requester.lastRequest.Headers
	if _, ok := sent["X-Disabled"]; ok {
		t.Error("disabled header X-Disabled should not be sent")
	}
	if _, ok := sent["X-Active"]; !ok {
		t.Error("enabled header X-Active should be sent")
	}
	if _, ok := sent["Accept"]; !ok {
		t.Error("enabled header Accept should be sent")
	}
}

func TestFormHasFiles(t *testing.T) {
	tests := []struct {
		name     string
		body     string
		expected bool
	}{
		{"empty", "", false},
		{"empty array", "[]", false},
		{"text only", `[{"key":"name","value":"test","type":"text","enabled":true}]`, false},
		{"file field", `[{"key":"avatar","value":"/tmp/photo.jpg","type":"file","enabled":true}]`, true},
		{"disabled file", `[{"key":"avatar","value":"/tmp/photo.jpg","type":"file","enabled":false}]`, false},
		{"mixed", `[{"key":"name","value":"test","type":"text","enabled":true},{"key":"doc","value":"/tmp/doc.pdf","type":"file","enabled":true}]`, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := request.FormHasFiles(tt.body)
			if got != tt.expected {
				t.Errorf("formHasFiles(%q) = %v, want %v", tt.body, got, tt.expected)
			}
		})
	}
}

// History must record the raw JSON form body (post-substitution, pre-encoding),
// not the url-encoded wire payload.
func TestExecute_HistoryRecordsRawFormBody(t *testing.T) {
	uc, repo, historyRepo, requester := newTestUsecase()
	ctx := context.Background()

	id := uuid.New()
	rawBody := `[{"key":"user","value":"alice","type":"text","enabled":true}]`
	repo.requests[id] = &entities.Request{
		ID:           id,
		CollectionID: testCollectionID,
		Protocol:     entities.ProtocolHTTP,
		Method:       entities.MethodPOST,
		URL:          "https://api.example.com/login",
		Body:         rawBody,
		BodyType:     entities.BodyTypeForm,
		AuthType:     entities.AuthTypeNone,
	}
	requester.response = &entities.Response{StatusCode: 200, Headers: map[string][]string{}}

	if _, err := uc.Execute(ctx, id, request.ExecuteOpt{WorkspaceID: testWorkspaceID}); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if len(historyRepo.entries) != 1 {
		t.Fatalf("expected 1 history entry, got %d", len(historyRepo.entries))
	}
	if got := historyRepo.entries[0].RequestBody; got != rawBody {
		t.Errorf("history body: got %q, want raw JSON %q", got, rawBody)
	}
}

// JSONC comments are stripped from the wire body, while history keeps the
// original body with comments intact (Postman parity).
func TestExecute_StripsJSONCCommentsFromWireBody(t *testing.T) {
	uc, repo, historyRepo, requester := newTestUsecase()
	ctx := context.Background()

	id := uuid.New()
	rawBody := `{
  // user payload
  "name": "Alice", /* primary */
  "age": 30
}`
	repo.requests[id] = &entities.Request{
		ID:           id,
		CollectionID: testCollectionID,
		Protocol:     entities.ProtocolHTTP,
		Method:       entities.MethodPOST,
		URL:          "https://api.example.com/users",
		Body:         rawBody,
		BodyType:     entities.BodyTypeJSON,
		AuthType:     entities.AuthTypeNone,
	}
	requester.response = &entities.Response{StatusCode: 200, Headers: map[string][]string{}}

	if _, err := uc.Execute(ctx, id, request.ExecuteOpt{WorkspaceID: testWorkspaceID}); err != nil {
		t.Fatalf("execute: %v", err)
	}

	if strings.Contains(requester.lastRequest.Body, "//") {
		t.Errorf("wire body still contains // comment: %q", requester.lastRequest.Body)
	}
	if strings.Contains(requester.lastRequest.Body, "/*") {
		t.Errorf("wire body still contains /* block comment: %q", requester.lastRequest.Body)
	}
	var parsed map[string]any
	if err := json.Unmarshal([]byte(requester.lastRequest.Body), &parsed); err != nil {
		t.Errorf("wire body is not valid JSON: %v\nbody: %q", err, requester.lastRequest.Body)
	}

	if len(historyRepo.entries) != 1 {
		t.Fatalf("expected 1 history entry, got %d", len(historyRepo.entries))
	}
	if got := historyRepo.entries[0].RequestBody; got != rawBody {
		t.Errorf("history body should keep comments\n  got:  %q\n  want: %q", got, rawBody)
	}
}

// History must record the substituted absolute path for binary bodies, not the
// "{{root}}/file.bin" template. Execute opens the file, so a real temp file is required.
func TestExecute_HistoryRecordsBinaryWithSubstitutedPath(t *testing.T) {
	tempDir := t.TempDir()
	binPath := filepath.Join(tempDir, "payload.bin")
	if err := os.WriteFile(binPath, []byte("binary-payload"), 0o600); err != nil {
		t.Fatalf("setup: write temp file: %v", err)
	}

	repo := newMockRepo()
	historyRepo := &mockHistoryRepo{}
	requester := &mockRequester{response: &entities.Response{StatusCode: 200, Headers: map[string][]string{}}}
	envResolver := &mockEnvResolver{vars: map[string]string{"root": tempDir}}
	uc := request.NewUsecase(repo, historyRepo, requester, nil, nil, envResolver, &noopScriptEngine{}, &noopScriptResolver{}, &noopVarPersister{}, request.NewAuthResolver(fixtureCollections()), nil, nil, nil, nil)

	id := uuid.New()
	repo.requests[id] = &entities.Request{
		ID:           id,
		CollectionID: testCollectionID,
		Protocol:     entities.ProtocolHTTP,
		Method:       entities.MethodPOST,
		URL:          "https://api.example.com/upload",
		Body:         "{{root}}/payload.bin",
		BodyType:     entities.BodyTypeBinary,
		AuthType:     entities.AuthTypeNone,
	}

	ctx := context.Background()
	if _, err := uc.Execute(ctx, id, request.ExecuteOpt{WorkspaceID: testWorkspaceID}); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if len(historyRepo.entries) != 1 {
		t.Fatalf("expected 1 history entry, got %d", len(historyRepo.entries))
	}
	want := "[binary: " + binPath + "]"
	if got := historyRepo.entries[0].RequestBody; got != want {
		t.Errorf("history binary body: got %q, want %q", got, want)
	}
}

func TestCreate_PersistsDescription(t *testing.T) {
	uc, _, _, _ := newTestUsecase()
	ctx := context.Background()

	created, err := uc.Create(ctx, request.Create{
		CollectionID: testCollectionID,
		Name:         "Documented",
		Description:  "# Ping\n\nReturns `pong`.",
		Protocol:     entities.ProtocolHTTP,
		Method:       entities.MethodGET,
		BodyType:     entities.BodyTypeNone,
		AuthType:     entities.AuthTypeNone,
	}, request.CreateOpt{UserID: "user-1"})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if created.Description != "# Ping\n\nReturns `pong`." {
		t.Errorf("description: got %q", created.Description)
	}

	got, err := uc.GetByID(ctx, created.ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got.Description != created.Description {
		t.Errorf("description after reload: got %q, want %q", got.Description, created.Description)
	}
}

func TestEdit_UpdatesDescription(t *testing.T) {
	uc, _, _, _ := newTestUsecase()
	ctx := context.Background()

	created, err := uc.Create(ctx, request.Create{
		CollectionID: testCollectionID,
		Name:         "Documented",
		Description:  "old docs",
		Protocol:     entities.ProtocolHTTP,
		Method:       entities.MethodGET,
		BodyType:     entities.BodyTypeNone,
		AuthType:     entities.AuthTypeNone,
	}, request.CreateOpt{UserID: "user-1"})
	if err != nil {
		t.Fatalf("setup: %v", err)
	}

	edited, err := uc.Edit(ctx, request.Edit{
		Name:        "Documented",
		Description: "new docs",
		Method:      entities.MethodGET,
		BodyType:    entities.BodyTypeNone,
		AuthType:    entities.AuthTypeNone,
	}, request.EditOpt{RequestID: created.ID, UserID: "user-1", Version: created.Version})
	if err != nil {
		t.Fatalf("Edit: %v", err)
	}
	if edited.Description != "new docs" {
		t.Errorf("description: got %q, want %q", edited.Description, "new docs")
	}
}

const wsSettingsDoc = `{"version":1,"pingIntervalSec":20,"subprotocols":["json"],"messages":[{"id":"m1","name":"Login","format":"json","data":"{}"}]}`

func TestCreate_WebSocket_KeepsSettingsAndForcesRawBody(t *testing.T) {
	uc, _, _, _ := newTestUsecase()
	ctx := context.Background()

	created, err := uc.Create(ctx, request.Create{
		CollectionID: testCollectionID,
		Name:         "Feed",
		Protocol:     entities.ProtocolWebSocket,
		URL:          "wss://example.com/ws",
		Body:         wsSettingsDoc,
		BodyType:     entities.BodyTypeNone,
		AuthType:     entities.AuthTypeNone,
	}, request.CreateOpt{UserID: "user-1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if created.Body != wsSettingsDoc {
		t.Errorf("body: got %q, want %q", created.Body, wsSettingsDoc)
	}
	if created.BodyType != entities.BodyTypeRaw {
		t.Errorf("bodyType: got %q, want %q", created.BodyType, entities.BodyTypeRaw)
	}
}

func TestCreate_WebSocket_RejectsMalformedSettings(t *testing.T) {
	uc, repo, _, _ := newTestUsecase()
	ctx := context.Background()

	created, err := uc.Create(ctx, request.Create{
		CollectionID: testCollectionID,
		Name:         "Feed",
		Protocol:     entities.ProtocolWebSocket,
		URL:          "wss://example.com/ws",
		Body:         "not a document",
		BodyType:     entities.BodyTypeRaw,
		AuthType:     entities.AuthTypeNone,
	}, request.CreateOpt{UserID: "user-1"})
	if created != nil {
		t.Fatal("expected no request on a validation error")
	}
	var verr *domain.ValidationError
	if !errors.As(err, &verr) {
		t.Fatalf("expected *domain.ValidationError, got %T (%v)", err, err)
	}
	if verr.Fields["body"] == "" {
		t.Errorf("expected a reason under the body field, got %v", verr.Fields)
	}
	if len(repo.requests) != 0 {
		t.Errorf("expected nothing persisted, got %d requests", len(repo.requests))
	}
}

func TestEdit_WebSocket_KeepsSettingsAndForcesRawBody(t *testing.T) {
	uc, _, _, _ := newTestUsecase()
	ctx := context.Background()

	created, err := uc.Create(ctx, request.Create{
		CollectionID: testCollectionID,
		Name:         "Feed",
		Protocol:     entities.ProtocolWebSocket,
		URL:          "wss://example.com/ws",
		BodyType:     entities.BodyTypeRaw,
		AuthType:     entities.AuthTypeNone,
	}, request.CreateOpt{UserID: "user-1"})
	if err != nil {
		t.Fatalf("setup: %v", err)
	}

	edited, err := uc.Edit(ctx, request.Edit{
		Name:     "Feed",
		URL:      "wss://example.com/ws",
		Body:     wsSettingsDoc,
		BodyType: entities.BodyTypeJSON,
		AuthType: entities.AuthTypeNone,
	}, request.EditOpt{RequestID: created.ID, UserID: "user-1", Version: created.Version})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if edited.Body != wsSettingsDoc {
		t.Errorf("body: got %q, want %q", edited.Body, wsSettingsDoc)
	}
	if edited.BodyType != entities.BodyTypeRaw {
		t.Errorf("bodyType: got %q, want %q", edited.BodyType, entities.BodyTypeRaw)
	}
}

func TestEdit_WebSocket_RejectsMalformedSettings(t *testing.T) {
	uc, _, _, _ := newTestUsecase()
	ctx := context.Background()

	created, err := uc.Create(ctx, request.Create{
		CollectionID: testCollectionID,
		Name:         "Feed",
		Protocol:     entities.ProtocolWebSocket,
		URL:          "wss://example.com/ws",
		Body:         wsSettingsDoc,
		BodyType:     entities.BodyTypeRaw,
		AuthType:     entities.AuthTypeNone,
	}, request.CreateOpt{UserID: "user-1"})
	if err != nil {
		t.Fatalf("setup: %v", err)
	}

	edited, err := uc.Edit(ctx, request.Edit{
		Name:     "Feed",
		URL:      "wss://example.com/ws",
		Body:     `{"version":9}`,
		BodyType: entities.BodyTypeRaw,
		AuthType: entities.AuthTypeNone,
	}, request.EditOpt{RequestID: created.ID, UserID: "user-1", Version: created.Version})
	if edited != nil {
		t.Fatal("expected no request on a validation error")
	}
	var verr *domain.ValidationError
	if !errors.As(err, &verr) {
		t.Fatalf("expected *domain.ValidationError, got %T (%v)", err, err)
	}

	stored, err := uc.GetByID(ctx, created.ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if stored.Body != wsSettingsDoc {
		t.Errorf("stored body changed: got %q", stored.Body)
	}
}

func TestCreateEdit_HTTPBodyNotValidatedAsSettings(t *testing.T) {
	uc, _, _, _ := newTestUsecase()
	ctx := context.Background()

	created, err := uc.Create(ctx, request.Create{
		CollectionID: testCollectionID,
		Name:         "Plain",
		Protocol:     entities.ProtocolHTTP,
		Method:       entities.MethodPOST,
		URL:          "https://api.example.com/users",
		Body:         "not a document",
		BodyType:     entities.BodyTypeRaw,
		AuthType:     entities.AuthTypeNone,
	}, request.CreateOpt{UserID: "user-1"})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if created.Body != "not a document" || created.BodyType != entities.BodyTypeRaw {
		t.Errorf("http body altered: %q / %q", created.Body, created.BodyType)
	}

	edited, err := uc.Edit(ctx, request.Edit{
		Name:     "Plain",
		Method:   entities.MethodPOST,
		URL:      "https://api.example.com/users",
		Body:     `{"still":"free-form"}`,
		BodyType: entities.BodyTypeJSON,
		AuthType: entities.AuthTypeNone,
	}, request.EditOpt{RequestID: created.ID, UserID: "user-1", Version: created.Version})
	if err != nil {
		t.Fatalf("Edit: %v", err)
	}
	if edited.BodyType != entities.BodyTypeJSON {
		t.Errorf("http bodyType altered: %q", edited.BodyType)
	}
}

type mockGraphQLRequester struct {
	response *entities.Response
	last     request.GraphQLExecuteRequest
}

func (m *mockGraphQLRequester) Execute(_ context.Context, req request.GraphQLExecuteRequest) (*entities.Response, error) {
	m.last = req
	return m.response, nil
}

func (m *mockGraphQLRequester) Introspect(_ context.Context, _ request.GraphQLIntrospectRequest) (*request.GraphQLSchema, error) {
	return nil, nil
}

func (m *mockGraphQLRequester) GenerateExampleQuery(_ *request.GraphQLSchema, _ string) (*request.GraphQLExampleResponse, error) {
	return nil, nil
}

func ucWithPreScript(repo *mockRepo, vars map[string]string, engine request.ScriptEngine, httpReq *mockRequester, gqlReq *mockGraphQLRequester) request.Usecase {
	return request.NewUsecase(repo, &mockHistoryRepo{}, httpReq, nil, gqlReq,
		&mockEnvResolver{vars: vars}, engine, &scriptResolverWithPre{pre: "// script"},
		&noopVarPersister{}, request.NewAuthResolver(fixtureCollections()), nil, nil, nil, nil)
}

func TestExecute_HTTP_ScriptHeaderPlaceholderIsResolved(t *testing.T) {
	repo := newMockRepo()
	requester := &mockRequester{response: &entities.Response{StatusCode: 200}}
	engine := &captureScriptEngine{preHeaders: map[string][]string{"X-Token": {"{{token}}"}}}
	uc := ucWithPreScript(repo, map[string]string{"token": "secret"}, engine, requester, nil)

	id := uuid.New()
	repo.requests[id] = &entities.Request{
		ID: id, CollectionID: testCollectionID, Protocol: entities.ProtocolHTTP, Method: entities.MethodGET,
		URL: "https://api.example.com/x", BodyType: entities.BodyTypeNone, AuthType: entities.AuthTypeNone,
	}

	if _, err := uc.Execute(context.Background(), id, request.ExecuteOpt{WorkspaceID: testWorkspaceID}); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if got := requester.lastRequest.Headers["X-Token"]; len(got) != 1 || got[0] != "secret" {
		t.Fatalf("X-Token: got %v, want [secret]", got)
	}
}

func TestExecute_HTTP_ScriptSeesRawHeadersAndNewVariableApplies(t *testing.T) {
	repo := newMockRepo()
	requester := &mockRequester{response: &entities.Response{StatusCode: 200}}
	engine := &captureScriptEngine{preVars: map[string]string{"v": "new"}}
	uc := ucWithPreScript(repo, map[string]string{"v": "old"}, engine, requester, nil)

	id := uuid.New()
	repo.requests[id] = &entities.Request{
		ID: id, CollectionID: testCollectionID, Protocol: entities.ProtocolHTTP, Method: entities.MethodGET,
		URL:      "https://api.example.com/x",
		Headers:  []entities.HeaderItem{{Key: "X-Test", Value: "{{v}}", Enabled: true}},
		BodyType: entities.BodyTypeNone, AuthType: entities.AuthTypeNone,
	}

	if _, err := uc.Execute(context.Background(), id, request.ExecuteOpt{WorkspaceID: testWorkspaceID}); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if got := engine.seenHeaders["X-Test"]; len(got) != 1 || got[0] != "{{v}}" {
		t.Fatalf("script saw X-Test = %v, want the raw placeholder", got)
	}
	if got := requester.lastRequest.Headers["X-Test"]; len(got) != 1 || got[0] != "new" {
		t.Fatalf("X-Test: got %v, want [new]", got)
	}
}

func TestExecute_GraphQL_ScriptHeaderPlaceholderIsResolved(t *testing.T) {
	repo := newMockRepo()
	gql := &mockGraphQLRequester{response: &entities.Response{StatusCode: 200}}
	engine := &captureScriptEngine{preHeaders: map[string][]string{"X-Token": {"{{token}}"}}}
	uc := ucWithPreScript(repo, map[string]string{"token": "secret"}, engine, &mockRequester{}, gql)

	id := uuid.New()
	repo.requests[id] = &entities.Request{
		ID: id, CollectionID: testCollectionID, Protocol: entities.ProtocolGraphQL, URL: "https://api.example.com/graphql",
		GraphQLQuery: "{ me }", AuthType: entities.AuthTypeNone,
	}

	if _, err := uc.Execute(context.Background(), id, request.ExecuteOpt{WorkspaceID: testWorkspaceID}); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if got := gql.last.Headers["X-Token"]; len(got) != 1 || got[0] != "secret" {
		t.Fatalf("X-Token: got %v, want [secret]", got)
	}
}

func TestExecute_GraphQL_ScriptSeesRawHeadersAndNewVariableApplies(t *testing.T) {
	repo := newMockRepo()
	gql := &mockGraphQLRequester{response: &entities.Response{StatusCode: 200}}
	engine := &captureScriptEngine{preVars: map[string]string{"v": "new"}}
	uc := ucWithPreScript(repo, map[string]string{"v": "old"}, engine, &mockRequester{}, gql)

	id := uuid.New()
	repo.requests[id] = &entities.Request{
		ID: id, CollectionID: testCollectionID, Protocol: entities.ProtocolGraphQL, URL: "https://api.example.com/graphql",
		Headers:      []entities.HeaderItem{{Key: "X-Test", Value: "{{v}}", Enabled: true}},
		GraphQLQuery: "{ me }", AuthType: entities.AuthTypeNone,
	}

	if _, err := uc.Execute(context.Background(), id, request.ExecuteOpt{WorkspaceID: testWorkspaceID}); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if got := engine.seenHeaders["X-Test"]; len(got) != 1 || got[0] != "{{v}}" {
		t.Fatalf("script saw X-Test = %v, want the raw placeholder", got)
	}
	if got := gql.last.Headers["X-Test"]; len(got) != 1 || got[0] != "new" {
		t.Fatalf("X-Test: got %v, want [new]", got)
	}
}

func TestExecute_GraphQL_PassesWorkspaceIDForCookieJar(t *testing.T) {
	repo := newMockRepo()
	gql := &mockGraphQLRequester{response: &entities.Response{StatusCode: 200}}
	uc := ucWithPreScript(repo, nil, &captureScriptEngine{}, &mockRequester{}, gql)

	id := uuid.New()
	repo.requests[id] = &entities.Request{
		ID: id, CollectionID: testCollectionID, Protocol: entities.ProtocolGraphQL, URL: "https://api.example.com/graphql",
		GraphQLQuery: "{ me }", AuthType: entities.AuthTypeNone,
	}

	wsID := testWorkspaceID
	if _, err := uc.Execute(context.Background(), id, request.ExecuteOpt{WorkspaceID: wsID}); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if gql.last.WorkspaceID != wsID {
		t.Fatalf("WorkspaceID: got %s, want %s", gql.last.WorkspaceID, wsID)
	}
}

// digest and aws_sigv4 reach the requester as HTTPExecuteRequest.Auth: only the
// final request can carry a challenge response or a signature.
func TestExecute_WireAuth_HandedToTheRequester(t *testing.T) {
	uc, _, _, requester := newTestUsecase()
	ctx := context.Background()

	created, err := uc.Create(ctx, request.Create{
		CollectionID: testCollectionID,
		Name:         "Digest",
		Protocol:     entities.ProtocolHTTP,
		Method:       entities.MethodGET,
		URL:          "https://api.example.com",
		BodyType:     entities.BodyTypeNone,
		AuthType:     entities.AuthTypeDigest,
		AuthData:     `{"username":"neo","password":"trinity"}`,
	}, request.CreateOpt{UserID: "user-1"})
	if err != nil {
		t.Fatalf("setup: %v", err)
	}

	requester.response = &entities.Response{StatusCode: 200, StatusText: "OK", Headers: map[string][]string{}, Duration: 10 * time.Millisecond}
	if _, err = uc.Execute(ctx, created.ID, request.ExecuteOpt{UserID: "user-1", WorkspaceID: testWorkspaceID}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	auth := requester.lastRequest.Auth
	if auth == nil {
		t.Fatal("expected the requester to receive the auth scheme")
	}
	if auth.Type != entities.AuthTypeDigest {
		t.Errorf("auth type: got %q, want digest", auth.Type)
	}
	if auth.Fields["username"] != "neo" || auth.Fields["password"] != "trinity" {
		t.Errorf("unexpected auth fields: %v", auth.Fields)
	}
	if _, ok := requester.lastRequest.Headers["Authorization"]; ok {
		t.Errorf("digest must not set a header up front: %v", requester.lastRequest.Headers)
	}
}

func TestCreate_RejectsOversizedDescription(t *testing.T) {
	uc, _, _, _ := newTestUsecase()
	ctx := context.Background()

	input := request.Create{
		CollectionID: testCollectionID,
		Name:         "Docs",
		Protocol:     entities.ProtocolHTTP,
		Method:       entities.MethodGET,
		BodyType:     entities.BodyTypeNone,
		AuthType:     entities.AuthTypeNone,
		Description:  strings.Repeat("a", domain.MaxDescriptionLen+1),
	}
	_, err := uc.Create(ctx, input, request.CreateOpt{UserID: "user-1"})
	var verr *domain.ValidationError
	if !errors.As(err, &verr) || verr.Fields["description"] == "" {
		t.Fatalf("expected a description validation error, got %v", err)
	}

	input.Description = strings.Repeat("a", domain.MaxDescriptionLen)
	if _, err := uc.Create(ctx, input, request.CreateOpt{UserID: "user-1"}); err != nil {
		t.Fatalf("exactly the limit must pass: %v", err)
	}
}

func TestEdit_RejectsOversizedDescription(t *testing.T) {
	uc, repo, _, _ := newTestUsecase()
	ctx := context.Background()

	created, err := uc.Create(ctx, request.Create{
		CollectionID: testCollectionID,
		Name:         "Docs",
		Protocol:     entities.ProtocolHTTP,
		Method:       entities.MethodGET,
		BodyType:     entities.BodyTypeNone,
		AuthType:     entities.AuthTypeNone,
	}, request.CreateOpt{UserID: "user-1"})
	if err != nil {
		t.Fatalf("setup: unexpected error: %v", err)
	}

	edit := func(name, description string, version int) (*entities.Request, error) {
		return uc.Edit(ctx, request.Edit{
			Name:        name,
			Description: description,
			Method:      entities.MethodGET,
			BodyType:    entities.BodyTypeNone,
			AuthType:    entities.AuthTypeNone,
		}, request.EditOpt{RequestID: created.ID, UserID: "user-1", Version: version})
	}

	var verr *domain.ValidationError
	_, err = edit("Docs", strings.Repeat("a", domain.MaxDescriptionLen+1), created.Version)
	if !errors.As(err, &verr) || verr.Fields["description"] == "" {
		t.Fatalf("expected a description validation error, got %v", err)
	}

	legacy := strings.Repeat("b", 20*1024)
	repo.requests[created.ID].Description = legacy

	renamed, err := edit("Renamed", legacy, created.Version)
	if err != nil {
		t.Fatalf("renaming an entity with a legacy description must pass: %v", err)
	}

	shortened, err := edit("Renamed", strings.Repeat("b", 18*1024), renamed.Version)
	if err != nil {
		t.Fatalf("shortening an oversized description must pass: %v", err)
	}

	if _, err := edit("Renamed", strings.Repeat("b", 21*1024), shortened.Version); !errors.As(err, &verr) || verr.Fields["description"] == "" {
		t.Fatalf("growing an oversized description must be rejected, got %v", err)
	}
}
