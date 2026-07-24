package sqlite

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/tetiva-app/client/internal/domain/entities"
	"github.com/tetiva-app/client/internal/domain/usecase/history"
)

func newTestHistory(requestID uuid.UUID) *entities.History {
	now := time.Now().Truncate(time.Second)
	return &entities.History{
		ID:          uuid.New(),
		RequestID:   requestID,
		WorkspaceID: testWorkspaceID,
		Protocol:    entities.ProtocolHTTP,
		Method:      string(entities.MethodGET),
		URL:         "https://example.com/api/users",
		RequestHeaders: map[string][]string{
			"Content-Type":  {"application/json"},
			"Authorization": {"Bearer token123"},
		},
		RequestBody:    "",
		ResponseStatus: 200,
		ResponseHeaders: map[string][]string{
			"Content-Type": {"application/json"},
		},
		ResponseBody: `{"users":[]}`,
		ResponseSize: 12,
		DurationMs:   150,
		ErrorMessage: "",
		CreatedAt:    now,
	}
}

// createTestRequest inserts a collection and request, returning the request ID.
func createTestRequest(t *testing.T, db *sql.DB) uuid.UUID {
	t.Helper()
	collRepo := NewCollectionRepo(db)
	reqRepo := NewRequestRepo(db)
	ctx := context.Background()

	coll := newTestCollection("History Test Collection", nil)
	if err := collRepo.Create(ctx, coll); err != nil {
		t.Fatalf("Create collection failed: %v", err)
	}

	req := newTestRequest("History Test Request", coll.ID)
	if err := reqRepo.Create(ctx, req); err != nil {
		t.Fatalf("Create request failed: %v", err)
	}

	return req.ID
}

func TestHistoryRepo_Create(t *testing.T) {
	db := setupTestDB(t)
	requestID := createTestRequest(t, db)
	repo := NewHistoryRepo(db)
	ctx := context.Background()

	h := newTestHistory(requestID)

	if err := repo.Create(ctx, h); err != nil {
		t.Fatalf("Create history failed: %v", err)
	}

	var (
		id           string
		reqID        string
		workspaceID  string
		protocol     string
		method       string
		url          string
		reqHeaders   string
		reqBody      string
		respStatus   int
		respHeaders  string
		respBody     string
		respSize     int64
		durationMs   int64
		errorMessage string
		createdAt    string
	)

	err := db.QueryRowContext(ctx,
		`SELECT id, request_id, workspace_id, protocol, method, url,
			request_headers, request_body, response_status, response_headers,
			response_body, response_size, duration_ms, error_message, created_at
		FROM history WHERE id = ?`, h.ID.String(),
	).Scan(&id, &reqID, &workspaceID, &protocol, &method, &url,
		&reqHeaders, &reqBody, &respStatus, &respHeaders,
		&respBody, &respSize, &durationMs, &errorMessage, &createdAt,
	)
	if err != nil {
		t.Fatalf("Query history row failed: %v", err)
	}

	if id != h.ID.String() {
		t.Errorf("ID mismatch: got %q, want %q", id, h.ID.String())
	}
	if reqID != requestID.String() {
		t.Errorf("RequestID mismatch: got %q, want %q", reqID, requestID.String())
	}
	if workspaceID != testWorkspaceID.String() {
		t.Errorf("WorkspaceID mismatch: got %q, want %q", workspaceID, testWorkspaceID.String())
	}
	if protocol != "http" {
		t.Errorf("Protocol mismatch: got %q, want %q", protocol, "http")
	}
	if method != "GET" {
		t.Errorf("Method mismatch: got %q, want %q", method, "GET")
	}
	if url != h.URL {
		t.Errorf("URL mismatch: got %q, want %q", url, h.URL)
	}
	if respStatus != 200 {
		t.Errorf("ResponseStatus mismatch: got %d, want 200", respStatus)
	}
	if respBody != `{"users":[]}` {
		t.Errorf("ResponseBody mismatch: got %q, want %q", respBody, `{"users":[]}`)
	}
	if respSize != 12 {
		t.Errorf("ResponseSize mismatch: got %d, want 12", respSize)
	}
	if durationMs != 150 {
		t.Errorf("DurationMs mismatch: got %d, want 150", durationMs)
	}
	if errorMessage != "" {
		t.Errorf("ErrorMessage should be empty, got %q", errorMessage)
	}
}

func TestHistoryRepo_Create_WithError(t *testing.T) {
	db := setupTestDB(t)
	requestID := createTestRequest(t, db)
	repo := NewHistoryRepo(db)
	ctx := context.Background()

	h := newTestHistory(requestID)
	h.ResponseStatus = 0
	h.ResponseHeaders = map[string][]string{}
	h.ResponseBody = ""
	h.ResponseSize = 0
	h.DurationMs = 50
	h.ErrorMessage = "connection refused: 127.0.0.1:8080"

	if err := repo.Create(ctx, h); err != nil {
		t.Fatalf("Create history with error failed: %v", err)
	}

	var (
		respStatus   int
		errorMessage string
		durationMs   int64
	)

	err := db.QueryRowContext(ctx,
		`SELECT response_status, error_message, duration_ms FROM history WHERE id = ?`,
		h.ID.String(),
	).Scan(&respStatus, &errorMessage, &durationMs)
	if err != nil {
		t.Fatalf("Query history row failed: %v", err)
	}

	if respStatus != 0 {
		t.Errorf("ResponseStatus should be 0, got %d", respStatus)
	}
	if errorMessage != "connection refused: 127.0.0.1:8080" {
		t.Errorf("ErrorMessage mismatch: got %q, want %q", errorMessage, "connection refused: 127.0.0.1:8080")
	}
	if durationMs != 50 {
		t.Errorf("DurationMs mismatch: got %d, want 50", durationMs)
	}
}

// insertWorkspace creates a bare workspaces row to satisfy the workspace_id FK.
// The name derives from the ID so multiple workspaces can coexist in one test.
func insertWorkspace(t *testing.T, db *sql.DB, id uuid.UUID) {
	t.Helper()
	if _, err := db.Exec(
		`INSERT INTO workspaces (id, name) VALUES (?, ?)`,
		id.String(), "test-"+id.String(),
	); err != nil {
		t.Fatalf("insert workspace: %v", err)
	}
}

func newHistoryRow(workspaceID uuid.UUID, method, url string, status int, errMsg string, proto entities.Protocol, ts time.Time) *entities.History {
	return &entities.History{
		ID:              uuid.New(),
		WorkspaceID:     workspaceID,
		Protocol:        proto,
		Method:          method,
		URL:             url,
		ResponseStatus:  status,
		ErrorMessage:    errMsg,
		RequestHeaders:  map[string][]string{},
		ResponseHeaders: map[string][]string{},
		CreatedAt:       ts,
	}
}

func TestHistoryRepo_List_WithFilters(t *testing.T) {
	db := setupTestDB(t)
	defer func() { _ = db.Close() }()
	repo := NewHistoryRepo(db)
	var historyRepo history.Repository = repo
	ctx := context.Background()
	ws := uuid.New()
	insertWorkspace(t, db, ws)

	insert := func(method, url string, status int, errMsg string, proto entities.Protocol, ts time.Time) uuid.UUID {
		h := newHistoryRow(ws, method, url, status, errMsg, proto, ts)
		if err := repo.Create(ctx, h); err != nil {
			t.Fatal(err)
		}
		return h.ID
	}

	now := time.Now().Truncate(time.Second)
	id1 := insert("GET", "https://api.example.com/users", 200, "", entities.ProtocolHTTP, now.Add(-3*time.Minute))
	id2 := insert("POST", "https://api.example.com/users", 201, "", entities.ProtocolHTTP, now.Add(-2*time.Minute))
	id3 := insert("GET", "https://api.other.com/x", 500, "", entities.ProtocolHTTP, now.Add(-1*time.Minute))
	id4 := insert("GET", "https://api.example.com/timeout", 0, "context deadline exceeded", entities.ProtocolHTTP, now)

	items, err := historyRepo.List(ctx, history.Filter{WorkspaceID: ws, Limit: 100})
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 4 {
		t.Fatalf("got %d, want 4", len(items))
	}
	if items[0].ID != id4 {
		t.Errorf("first item should be newest (id4); got %v", items[0].ID)
	}

	items, err = historyRepo.List(ctx, history.Filter{WorkspaceID: ws, URLContains: "example.com", Limit: 100})
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 3 {
		t.Errorf("URL filter: got %d, want 3", len(items))
	}
	_ = id1
	_ = id2
	_ = id3

	items, err = historyRepo.List(ctx, history.Filter{WorkspaceID: ws, StatusKinds: []history.StatusKind{history.StatusKind5xx}, Limit: 100})
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 {
		t.Errorf("5xx filter: got %d, want 1", len(items))
	}

	items, err = historyRepo.List(ctx, history.Filter{WorkspaceID: ws, StatusKinds: []history.StatusKind{history.StatusKindError}, Limit: 100})
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 {
		t.Errorf("error filter: got %d, want 1", len(items))
	}

	n, err := historyRepo.Count(ctx, history.Filter{WorkspaceID: ws})
	if err != nil {
		t.Fatal(err)
	}
	if n != 4 {
		t.Errorf("count = %d, want 4", n)
	}
}

func TestHistoryRepo_List_StatusFilter_PartialFailureClassifiedAsError(t *testing.T) {
	db := setupTestDB(t)
	defer func() { _ = db.Close() }()
	repo := NewHistoryRepo(db)
	var historyRepo history.Repository = repo
	ctx := context.Background()
	ws := uuid.New()
	insertWorkspace(t, db, ws)

	// status=200 plus a non-empty error_message (client failed mid-read):
	// per StatusKind contract the row classifies only as Error, never 2xx.
	h := newHistoryRow(ws, "GET", "https://api.example.com/x", 200, "partial read", entities.ProtocolHTTP, time.Now())
	if err := repo.Create(ctx, h); err != nil {
		t.Fatalf("Create: %v", err)
	}

	items, err := historyRepo.List(ctx, history.Filter{
		WorkspaceID: ws,
		StatusKinds: []history.StatusKind{history.StatusKind2xx},
		Limit:       100,
	})
	if err != nil {
		t.Fatalf("List 2xx: %v", err)
	}
	if len(items) != 0 {
		t.Errorf("2xx filter on partial-failure row: got %d, want 0 (transport failure wins)", len(items))
	}

	items, err = historyRepo.List(ctx, history.Filter{
		WorkspaceID: ws,
		StatusKinds: []history.StatusKind{history.StatusKindError},
		Limit:       100,
	})
	if err != nil {
		t.Fatalf("List Error: %v", err)
	}
	if len(items) != 1 {
		t.Errorf("Error filter on partial-failure row: got %d, want 1", len(items))
	}
}

func TestHistoryRepo_Create_WithNilRequestID_WritesNULL(t *testing.T) {
	db := setupTestDB(t)
	defer func() { _ = db.Close() }()
	repo := NewHistoryRepo(db)
	ctx := context.Background()
	ws := uuid.New()
	insertWorkspace(t, db, ws)

	// A zero-UUID RequestID must persist as SQL NULL, not a literal zero string —
	// otherwise the FK constraint rejects orphan rows (ad-hoc or deleted requests).
	h := newHistoryRow(ws, "GET", "https://api.example.com/orphan", 200, "", entities.ProtocolHTTP, time.Now())
	h.RequestID = uuid.Nil

	if err := repo.Create(ctx, h); err != nil {
		t.Fatalf("Create with nil RequestID: %v", err)
	}

	var isNull int
	if err := db.QueryRowContext(ctx,
		`SELECT request_id IS NULL FROM history WHERE id = ?`, h.ID.String(),
	).Scan(&isNull); err != nil {
		t.Fatalf("query request_id IS NULL: %v", err)
	}
	if isNull != 1 {
		t.Errorf("request_id IS NULL = %d, want 1 (NULL)", isNull)
	}

	got, err := repo.GetByID(ctx, h.ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got == nil {
		t.Fatal("GetByID returned nil, want entity")
	}
	if got.RequestID != uuid.Nil {
		t.Errorf("got.RequestID = %v, want uuid.Nil", got.RequestID)
	}
}

func TestHistoryRepo_Delete_RemovesSingleRow(t *testing.T) {
	db := setupTestDB(t)
	defer func() { _ = db.Close() }()
	repo := NewHistoryRepo(db)
	var historyRepo history.Repository = repo
	ctx := context.Background()
	ws := uuid.New()
	insertWorkspace(t, db, ws)

	h := newHistoryRow(ws, "GET", "https://api.example.com/x", 200, "", entities.ProtocolHTTP, time.Now())
	if err := repo.Create(ctx, h); err != nil {
		t.Fatalf("Create: %v", err)
	}

	if err := historyRepo.Delete(ctx, h.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	n, err := historyRepo.Count(ctx, history.Filter{WorkspaceID: ws})
	if err != nil {
		t.Fatalf("Count: %v", err)
	}
	if n != 0 {
		t.Errorf("count after delete = %d, want 0", n)
	}
}

func TestHistoryRepo_DeleteAll_OnlyAffectsWorkspace(t *testing.T) {
	db := setupTestDB(t)
	defer func() { _ = db.Close() }()
	repo := NewHistoryRepo(db)
	var historyRepo history.Repository = repo
	ctx := context.Background()
	ws1 := uuid.New()
	ws2 := uuid.New()
	insertWorkspace(t, db, ws1)
	insertWorkspace(t, db, ws2)

	now := time.Now().Truncate(time.Second)
	rows := []*entities.History{
		newHistoryRow(ws1, "GET", "https://api.example.com/a", 200, "", entities.ProtocolHTTP, now),
		newHistoryRow(ws1, "GET", "https://api.example.com/b", 200, "", entities.ProtocolHTTP, now),
		newHistoryRow(ws2, "GET", "https://api.example.com/c", 200, "", entities.ProtocolHTTP, now),
	}
	for _, h := range rows {
		if err := repo.Create(ctx, h); err != nil {
			t.Fatalf("Create: %v", err)
		}
	}

	if err := historyRepo.DeleteAll(ctx, ws1); err != nil {
		t.Fatalf("DeleteAll: %v", err)
	}

	n1, err := historyRepo.Count(ctx, history.Filter{WorkspaceID: ws1})
	if err != nil {
		t.Fatalf("Count ws1: %v", err)
	}
	if n1 != 0 {
		t.Errorf("ws1 count after DeleteAll = %d, want 0", n1)
	}

	n2, err := historyRepo.Count(ctx, history.Filter{WorkspaceID: ws2})
	if err != nil {
		t.Fatalf("Count ws2: %v", err)
	}
	if n2 != 1 {
		t.Errorf("ws2 count after DeleteAll(ws1) = %d, want 1", n2)
	}
}
