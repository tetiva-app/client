package sqlite

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/tetiva-app/client/internal/domain/entities"
	"github.com/tetiva-app/client/internal/domain/usecase/request"
)

func newTestRequest(name string, collectionID uuid.UUID) *entities.Request {
	now := time.Now().Truncate(time.Second)
	return &entities.Request{
		ID:            uuid.New(),
		CollectionID:  collectionID,
		Name:          name,
		Protocol:      entities.ProtocolHTTP,
		Method:        entities.MethodGET,
		URL:           "https://example.com/api",
		Headers:       []entities.HeaderItem{{Key: "Content-Type", Value: "application/json", Enabled: true}},
		Body:          "",
		BodyType:      entities.BodyTypeNone,
		AuthType:      entities.AuthTypeNone,
		AuthData:      "{}",
		GRPCService:   "",
		GRPCMethod:    "",
		GRPCProtoPath: "",
		GRPCMetadata:  map[string][]string{},
		PreScript:     "",
		PostScript:    "",
		SortOrder:     0,
		Version:       1,
		IsDelete:      false,
		CreatedBy:     "test_user",
		CreatedAt:     now,
		UpdatedBy:     "test_user",
		UpdatedAt:     now,
	}
}

func TestRequestRepo_CreateAndGetByID(t *testing.T) {
	db := setupTestDB(t)
	collRepo := NewCollectionRepo(db)
	reqRepo := NewRequestRepo(db)
	ctx := context.Background()

	// Create parent collection (FK requirement)
	coll := newTestCollection("Test Collection", nil)
	if err := collRepo.Create(ctx, coll); err != nil {
		t.Fatalf("Create collection failed: %v", err)
	}

	req := newTestRequest("Get Users", coll.ID)

	if err := reqRepo.Create(ctx, req); err != nil {
		t.Fatalf("Create request failed: %v", err)
	}

	got, err := reqRepo.GetByID(ctx, req.ID)
	if err != nil {
		t.Fatalf("GetByID failed: %v", err)
	}
	if got == nil {
		t.Fatal("GetByID returned nil")
	}

	if got.ID != req.ID {
		t.Errorf("ID mismatch: got %s, want %s", got.ID, req.ID)
	}
	if got.CollectionID != req.CollectionID {
		t.Errorf("CollectionID mismatch: got %s, want %s", got.CollectionID, req.CollectionID)
	}
	if got.Name != req.Name {
		t.Errorf("Name mismatch: got %q, want %q", got.Name, req.Name)
	}
	if got.Protocol != req.Protocol {
		t.Errorf("Protocol mismatch: got %q, want %q", got.Protocol, req.Protocol)
	}
	if got.Method != req.Method {
		t.Errorf("Method mismatch: got %q, want %q", got.Method, req.Method)
	}
	if got.URL != req.URL {
		t.Errorf("URL mismatch: got %q, want %q", got.URL, req.URL)
	}
	if len(got.Headers) != len(req.Headers) {
		t.Errorf("Headers length mismatch: got %d, want %d", len(got.Headers), len(req.Headers))
	}
	if got.Body != req.Body {
		t.Errorf("Body mismatch: got %q, want %q", got.Body, req.Body)
	}
	if got.BodyType != req.BodyType {
		t.Errorf("BodyType mismatch: got %q, want %q", got.BodyType, req.BodyType)
	}
	if got.GRPCService != req.GRPCService {
		t.Errorf("GRPCService mismatch: got %q, want %q", got.GRPCService, req.GRPCService)
	}
	if got.GRPCMethod != req.GRPCMethod {
		t.Errorf("GRPCMethod mismatch: got %q, want %q", got.GRPCMethod, req.GRPCMethod)
	}
	if got.GRPCProtoPath != req.GRPCProtoPath {
		t.Errorf("GRPCProtoPath mismatch: got %q, want %q", got.GRPCProtoPath, req.GRPCProtoPath)
	}
	if got.PreScript != req.PreScript {
		t.Errorf("PreScript mismatch: got %q, want %q", got.PreScript, req.PreScript)
	}
	if got.PostScript != req.PostScript {
		t.Errorf("PostScript mismatch: got %q, want %q", got.PostScript, req.PostScript)
	}
	if got.SortOrder != req.SortOrder {
		t.Errorf("SortOrder mismatch: got %d, want %d", got.SortOrder, req.SortOrder)
	}
	if got.Version != req.Version {
		t.Errorf("Version mismatch: got %d, want %d", got.Version, req.Version)
	}
	if got.IsDelete != false {
		t.Errorf("IsDelete should be false")
	}
	if got.CreatedBy != req.CreatedBy {
		t.Errorf("CreatedBy mismatch: got %q, want %q", got.CreatedBy, req.CreatedBy)
	}
	if !got.CreatedAt.Equal(req.CreatedAt) {
		t.Errorf("CreatedAt mismatch: got %v, want %v", got.CreatedAt, req.CreatedAt)
	}
	if got.UpdatedBy != req.UpdatedBy {
		t.Errorf("UpdatedBy mismatch: got %q, want %q", got.UpdatedBy, req.UpdatedBy)
	}
	if !got.UpdatedAt.Equal(req.UpdatedAt) {
		t.Errorf("UpdatedAt mismatch: got %v, want %v", got.UpdatedAt, req.UpdatedAt)
	}

	notFound, err := reqRepo.GetByID(ctx, uuid.New())
	if err != nil {
		t.Fatalf("GetByID for missing ID returned error: %v", err)
	}
	if notFound != nil {
		t.Fatal("GetByID for missing ID should return nil")
	}
}

func TestRequestRepo_List(t *testing.T) {
	db := setupTestDB(t)
	collRepo := NewCollectionRepo(db)
	reqRepo := NewRequestRepo(db)
	ctx := context.Background()

	coll := newTestCollection("API Collection", nil)
	if err := collRepo.Create(ctx, coll); err != nil {
		t.Fatalf("Create collection failed: %v", err)
	}

	r1 := newTestRequest("Gamma Request", coll.ID)
	r1.SortOrder = 2
	r2 := newTestRequest("Alpha Request", coll.ID)
	r2.SortOrder = 0
	r3 := newTestRequest("Beta Request", coll.ID)
	r3.SortOrder = 1

	for _, r := range []*entities.Request{r1, r2, r3} {
		if err := reqRepo.Create(ctx, r); err != nil {
			t.Fatalf("Create request failed: %v", err)
		}
	}

	filter := request.Filter{CollectionID: coll.ID}
	list, err := reqRepo.List(ctx, filter)
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}

	if len(list) != 3 {
		t.Fatalf("expected 3 requests, got %d", len(list))
	}

	if list[0].Name != "Alpha Request" {
		t.Errorf("first request should be Alpha Request (sort_order=0), got %q", list[0].Name)
	}
	if list[1].Name != "Beta Request" {
		t.Errorf("second request should be Beta Request (sort_order=1), got %q", list[1].Name)
	}
	if list[2].Name != "Gamma Request" {
		t.Errorf("third request should be Gamma Request (sort_order=2), got %q", list[2].Name)
	}

	otherColl := newTestCollection("Other Collection", nil)
	if err := collRepo.Create(ctx, otherColl); err != nil {
		t.Fatalf("Create other collection failed: %v", err)
	}
	emptyFilter := request.Filter{CollectionID: otherColl.ID}
	empty, err := reqRepo.List(ctx, emptyFilter)
	if err != nil {
		t.Fatalf("List with other collection failed: %v", err)
	}
	if len(empty) != 0 {
		t.Errorf("expected 0 requests for empty collection, got %d", len(empty))
	}
}

func TestRequestRepo_Update(t *testing.T) {
	db := setupTestDB(t)
	collRepo := NewCollectionRepo(db)
	reqRepo := NewRequestRepo(db)
	ctx := context.Background()

	coll := newTestCollection("Update Collection", nil)
	if err := collRepo.Create(ctx, coll); err != nil {
		t.Fatalf("Create collection failed: %v", err)
	}

	req := newTestRequest("Original", coll.ID)
	if err := reqRepo.Create(ctx, req); err != nil {
		t.Fatalf("Create request failed: %v", err)
	}

	req.Name = "Updated Request"
	req.Method = entities.MethodPOST
	req.URL = "https://example.com/api/v2/users"
	req.Headers = []entities.HeaderItem{
		{Key: "Authorization", Value: "Bearer token123", Enabled: true},
		{Key: "Content-Type", Value: "application/json", Enabled: true},
	}
	req.Body = `{"name":"test"}`
	req.BodyType = entities.BodyTypeJSON
	req.Version = 2
	req.UpdatedBy = "editor"
	req.UpdatedAt = time.Now().Truncate(time.Second)

	if err := reqRepo.Update(ctx, req); err != nil {
		t.Fatalf("Update failed: %v", err)
	}

	got, err := reqRepo.GetByID(ctx, req.ID)
	if err != nil {
		t.Fatalf("GetByID after update failed: %v", err)
	}
	if got == nil {
		t.Fatal("GetByID returned nil after update")
	}

	if got.Name != "Updated Request" {
		t.Errorf("Name not updated: got %q, want %q", got.Name, "Updated Request")
	}
	if got.Method != entities.MethodPOST {
		t.Errorf("Method not updated: got %q, want %q", got.Method, entities.MethodPOST)
	}
	if got.URL != "https://example.com/api/v2/users" {
		t.Errorf("URL not updated: got %q", got.URL)
	}
	if len(got.Headers) != 2 {
		t.Errorf("Headers length mismatch: got %d, want 2", len(got.Headers))
	}
	if got.Body != `{"name":"test"}` {
		t.Errorf("Body not updated: got %q", got.Body)
	}
	if got.BodyType != entities.BodyTypeJSON {
		t.Errorf("BodyType not updated: got %q, want %q", got.BodyType, entities.BodyTypeJSON)
	}
	if got.Version != 2 {
		t.Errorf("Version not updated: got %d, want 2", got.Version)
	}
	if got.UpdatedBy != "editor" {
		t.Errorf("UpdatedBy not updated: got %q, want %q", got.UpdatedBy, "editor")
	}
}

func TestRequestRepo_SoftDelete(t *testing.T) {
	db := setupTestDB(t)
	collRepo := NewCollectionRepo(db)
	reqRepo := NewRequestRepo(db)
	ctx := context.Background()

	coll := newTestCollection("Delete Collection", nil)
	if err := collRepo.Create(ctx, coll); err != nil {
		t.Fatalf("Create collection failed: %v", err)
	}

	req := newTestRequest("ToDelete", coll.ID)
	if err := reqRepo.Create(ctx, req); err != nil {
		t.Fatalf("Create request failed: %v", err)
	}

	req.IsDelete = true
	req.Version = 2
	if err := reqRepo.Update(ctx, req); err != nil {
		t.Fatalf("Update for soft delete failed: %v", err)
	}

	got, err := reqRepo.GetByID(ctx, req.ID)
	if err != nil {
		t.Fatalf("GetByID after soft delete returned error: %v", err)
	}
	if got != nil {
		t.Error("GetByID should return nil for soft-deleted request")
	}

	filter := request.Filter{CollectionID: coll.ID}
	list, err := reqRepo.List(ctx, filter)
	if err != nil {
		t.Fatalf("List after soft delete failed: %v", err)
	}
	for _, item := range list {
		if item.ID == req.ID {
			t.Error("List should not include soft-deleted request")
		}
	}
}

func TestRequestRepo_HeadersJSONRoundtrip(t *testing.T) {
	db := setupTestDB(t)
	collRepo := NewCollectionRepo(db)
	reqRepo := NewRequestRepo(db)
	ctx := context.Background()

	coll := newTestCollection("Headers Collection", nil)
	if err := collRepo.Create(ctx, coll); err != nil {
		t.Fatalf("Create collection failed: %v", err)
	}

	req := newTestRequest("Multi Headers", coll.ID)
	req.Headers = []entities.HeaderItem{
		{Key: "Accept", Value: "application/json", Enabled: true},
		{Key: "Accept", Value: "text/html", Enabled: true},
		{Key: "X-Custom", Value: "value1", Enabled: true},
		{Key: "X-Custom", Value: "value2", Enabled: false},
		{Key: "Content-Type", Value: "application/json", Enabled: true},
	}
	req.GRPCMetadata = map[string][]string{
		"authorization": {"Bearer token"},
		"x-request-id":  {"abc-123", "def-456"},
	}

	if err := reqRepo.Create(ctx, req); err != nil {
		t.Fatalf("Create request failed: %v", err)
	}

	got, err := reqRepo.GetByID(ctx, req.ID)
	if err != nil {
		t.Fatalf("GetByID failed: %v", err)
	}
	if got == nil {
		t.Fatal("GetByID returned nil")
	}

	if len(got.Headers) != 5 {
		t.Fatalf("Headers length mismatch: got %d, want 5", len(got.Headers))
	}

	if got.Headers[3].Key != "X-Custom" || got.Headers[3].Value != "value2" || got.Headers[3].Enabled != false {
		t.Errorf("Disabled header not preserved: got %+v", got.Headers[3])
	}

	resolved := entities.EnabledHeadersToMap(got.Headers)
	if len(resolved["Accept"]) != 2 {
		t.Errorf("Accept should have 2 enabled values, got %d", len(resolved["Accept"]))
	}
	if len(resolved["X-Custom"]) != 1 {
		t.Errorf("X-Custom should have 1 enabled value (one is disabled), got %d", len(resolved["X-Custom"]))
	}

	if len(got.GRPCMetadata) != 2 {
		t.Fatalf("GRPCMetadata length mismatch: got %d, want 2", len(got.GRPCMetadata))
	}

	authValues := got.GRPCMetadata["authorization"]
	if len(authValues) != 1 || authValues[0] != "Bearer token" {
		t.Errorf("authorization metadata mismatch: got %v", authValues)
	}

	reqIDValues := got.GRPCMetadata["x-request-id"]
	if len(reqIDValues) != 2 || reqIDValues[0] != "abc-123" || reqIDValues[1] != "def-456" {
		t.Errorf("x-request-id metadata mismatch: got %v", reqIDValues)
	}
}

func TestRequestRepo_List_ExcludesDrafts(t *testing.T) {
	db := setupTestDB(t)
	collRepo := NewCollectionRepo(db)
	reqRepo := NewRequestRepo(db)
	ctx := context.Background()

	coll := newTestCollection("Drafts Collection", nil)
	if err := collRepo.Create(ctx, coll); err != nil {
		t.Fatalf("Create collection failed: %v", err)
	}

	normal := newTestRequest("normal", coll.ID)
	normal.URL = "https://example.com/normal"
	if err := reqRepo.Create(ctx, normal); err != nil {
		t.Fatalf("create normal: %v", err)
	}

	draft := newTestRequest("draft", coll.ID)
	draft.URL = "https://example.com/draft"
	draft.IsDraft = true
	if err := reqRepo.Create(ctx, draft); err != nil {
		t.Fatalf("create draft: %v", err)
	}

	list, err := reqRepo.List(ctx, request.Filter{CollectionID: coll.ID})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("len=%d, want 1 (draft must be excluded)", len(list))
	}
	if list[0].Name != "normal" {
		t.Errorf("got %q, want normal", list[0].Name)
	}

	// GetByID should still return the draft (tabs need it for opening the draft tab).
	gotDraft, err := reqRepo.GetByID(ctx, draft.ID)
	if err != nil {
		t.Fatalf("GetByID(draft) failed: %v", err)
	}
	if gotDraft == nil {
		t.Fatal("GetByID(draft) returned nil; drafts must remain accessible by ID")
	}
	if !gotDraft.IsDraft {
		t.Errorf("IsDraft not persisted: got %+v", gotDraft.IsDraft)
	}
}

func TestRequestRepo_DeleteHard_RemovesRow(t *testing.T) {
	db := setupTestDB(t)
	defer func() { _ = db.Close() }()
	collRepo := NewCollectionRepo(db)
	repo := NewRequestRepo(db)
	ctx := context.Background()

	coll := newTestCollection("Hard Delete Collection", nil)
	if err := collRepo.Create(ctx, coll); err != nil {
		t.Fatalf("Create collection failed: %v", err)
	}

	r := &entities.Request{
		ID:           uuid.New(),
		CollectionID: coll.ID,
		Name:         "x",
		Protocol:     entities.ProtocolHTTP,
		Method:       "GET",
		URL:          "x",
		AuthData:     "{}",
		Version:      1,
		IsDraft:      true,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}
	if err := repo.Create(ctx, r); err != nil {
		t.Fatal(err)
	}

	if err := repo.DeleteHard(ctx, r.ID); err != nil {
		t.Fatalf("DeleteHard: %v", err)
	}

	got, _ := repo.GetByID(ctx, r.ID)
	if got != nil {
		t.Errorf("expected nil after hard delete, got %+v", got)
	}
}

func TestRequestRepo_CleanupDrafts_RemovesAllDrafts(t *testing.T) {
	db := setupTestDB(t)
	defer func() { _ = db.Close() }()
	collRepo := NewCollectionRepo(db)
	repo := NewRequestRepo(db)
	ctx := context.Background()

	coll := newTestCollection("Cleanup Drafts Collection", nil)
	if err := collRepo.Create(ctx, coll); err != nil {
		t.Fatalf("Create collection failed: %v", err)
	}

	for i := 0; i < 3; i++ {
		d := &entities.Request{
			ID:           uuid.New(),
			CollectionID: coll.ID,
			Name:         "draft",
			Protocol:     entities.ProtocolHTTP,
			Method:       "GET",
			URL:          "x",
			AuthData:     "{}",
			Version:      1,
			IsDraft:      true,
			CreatedAt:    time.Now(),
			UpdatedAt:    time.Now(),
		}
		if err := repo.Create(ctx, d); err != nil {
			t.Fatalf("create draft: %v", err)
		}
	}
	normal := &entities.Request{
		ID:           uuid.New(),
		CollectionID: coll.ID,
		Name:         "keep",
		Protocol:     entities.ProtocolHTTP,
		Method:       "GET",
		URL:          "x",
		AuthData:     "{}",
		Version:      1,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}
	if err := repo.Create(ctx, normal); err != nil {
		t.Fatalf("create normal: %v", err)
	}

	n, err := repo.CleanupDrafts(ctx)
	if err != nil {
		t.Fatalf("CleanupDrafts: %v", err)
	}
	if n != 3 {
		t.Errorf("cleaned %d, want 3", n)
	}

	list, _ := repo.List(ctx, request.Filter{CollectionID: coll.ID})
	if len(list) != 1 || list[0].Name != "keep" {
		t.Errorf("after cleanup got %v, want only [keep]", list)
	}
}

func TestRequestSchema_HasIsDraftColumn(t *testing.T) {
	db := setupTestDB(t)
	defer func() { _ = db.Close() }()

	rows, err := db.Query("PRAGMA table_info(requests)")
	if err != nil {
		t.Fatalf("PRAGMA: %v", err)
	}
	defer func() { _ = rows.Close() }()

	found := false
	for rows.Next() {
		var cid int
		var name, ctype string
		var notnull, pk int
		var dflt sql.NullString
		if err := rows.Scan(&cid, &name, &ctype, &notnull, &dflt, &pk); err != nil {
			t.Fatalf("scan: %v", err)
		}
		if name == "is_draft" {
			found = true
			if ctype != "INTEGER" {
				t.Errorf("is_draft type = %s, want INTEGER", ctype)
			}
			if notnull != 1 {
				t.Errorf("is_draft notnull = %d, want 1", notnull)
			}
		}
	}
	if !found {
		t.Fatal("is_draft column not found in requests table")
	}
}

// Regression: the json_valid(auth_data) CHECK rejects the Go zero-value "" —
// this once crashed Replay until the draft usecase set AuthData = "{}".
func TestRequestRepo_Create_RejectsEmptyAuthData(t *testing.T) {
	db := setupTestDB(t)
	collRepo := NewCollectionRepo(db)
	reqRepo := NewRequestRepo(db)
	ctx := context.Background()

	coll := newTestCollection("c", nil)
	if err := collRepo.Create(ctx, coll); err != nil {
		t.Fatalf("create collection: %v", err)
	}

	req := newTestRequest("draft-shape", coll.ID)
	req.AuthData = "" // simulates the pre-fix draft construction

	if err := reqRepo.Create(ctx, req); err == nil {
		t.Fatal("expected json_valid(auth_data) CHECK to fail on empty AuthData, got nil error")
	}
}

// Exercises the exact entity shape request.CreateDraftFromHistory builds —
// json_valid-safe defaults (AuthData "{}", empty GRPCMetadata) — against the real schema.
func TestRequestRepo_Create_AcceptsDraftDefaults(t *testing.T) {
	db := setupTestDB(t)
	collRepo := NewCollectionRepo(db)
	reqRepo := NewRequestRepo(db)
	ctx := context.Background()

	coll := newTestCollection("c", nil)
	if err := collRepo.Create(ctx, coll); err != nil {
		t.Fatalf("create collection: %v", err)
	}

	now := time.Now().Truncate(time.Second)
	draft := &entities.Request{
		ID:           uuid.New(),
		CollectionID: coll.ID,
		Name:         "Replay: https://api.example.com/ping",
		Protocol:     entities.ProtocolHTTP,
		Method:       entities.MethodGET,
		URL:          "https://api.example.com/ping",
		Headers:      []entities.HeaderItem{},
		Body:         "",
		BodyType:     entities.BodyTypeRaw,
		AuthType:     entities.AuthTypeNone,
		AuthData:     "{}",
		GRPCMetadata: map[string][]string{},
		IsDraft:      true,
		Version:      1,
		CreatedBy:    "tester",
		CreatedAt:    now,
		UpdatedBy:    "tester",
		UpdatedAt:    now,
	}

	if err := reqRepo.Create(ctx, draft); err != nil {
		t.Fatalf("Create with draft defaults must satisfy json_valid CHECKs: %v", err)
	}

	got, err := reqRepo.GetByID(ctx, draft.ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got == nil {
		t.Fatal("draft was not persisted")
	}
	if !got.IsDraft {
		t.Error("persisted record should be flagged IsDraft")
	}
}
