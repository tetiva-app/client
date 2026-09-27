package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"strconv"
	"testing"

	"github.com/google/uuid"

	"github.com/tetiva-app/client/internal/domain/entities"
	"github.com/tetiva-app/client/internal/domain/usecase/collection"
	"github.com/tetiva-app/client/internal/domain/usecase/example"
	"github.com/tetiva-app/client/internal/domain/usecase/request"
)

type cascadeCollectionReader struct{ repo collection.Repository }

func (r cascadeCollectionReader) GetByID(ctx context.Context, id uuid.UUID) (*entities.Collection, error) {
	return r.repo.GetByID(ctx, id)
}

func (r cascadeCollectionReader) ListByWorkspace(ctx context.Context, workspaceID uuid.UUID) ([]*entities.Collection, error) {
	return r.repo.List(ctx, collection.Filter{WorkspaceID: workspaceID})
}

// failingCleaner lets the real cascade write, then fails, so the rollback has something to undo.
type failingCleaner struct{ example.Usecase }

var errCascade = errors.New("cascade failed")

func (c failingCleaner) DeleteByRequest(ctx context.Context, requestID uuid.UUID, userID string) error {
	if err := c.Usecase.DeleteByRequest(ctx, requestID, userID); err != nil {
		return err
	}
	return errCascade
}

type cascadeEnv struct {
	db       *sql.DB
	cols     collection.Repository
	examples example.Usecase
	req      *entities.Request
}

func newCascadeEnv(t *testing.T) *cascadeEnv {
	t.Helper()
	db := setupTestDB(t)
	ctx := testCtx(t)
	cols := NewCollectionRepo(db)
	reqs := NewRequestRepo(db)

	root := newTestCollection("Root", nil)
	folder := newTestCollection("Folder", &root.ID)
	for _, c := range []*entities.Collection{root, folder} {
		if err := cols.Create(ctx, c); err != nil {
			t.Fatalf("create collection: %v", err)
		}
	}
	req := newTestRequest("Get user", folder.ID)
	if err := reqs.Create(ctx, req); err != nil {
		t.Fatalf("create request: %v", err)
	}

	env := &cascadeEnv{
		db:       db,
		cols:     cols,
		examples: example.NewUsecase(NewResponseExampleRepo(db), reqs, cols),
		req:      req,
	}
	for _, code := range []int{200, 404, 500} {
		_, err := env.examples.Create(ctx, example.Create{
			RequestID: req.ID, Name: strconv.Itoa(code), StatusCode: code, Body: "{}", Protocol: entities.ProtocolHTTP,
		}, example.CreateOpt{UserID: "test_user"})
		if err != nil {
			t.Fatalf("create example: %v", err)
		}
	}
	return env
}

func (env *cascadeEnv) requestUsecase(cleaner request.ExampleCleaner) request.Usecase {
	return request.NewUsecase(NewRequestRepo(env.db), nil, nil, nil, nil, nil, nil, nil, nil, nil, nil,
		cascadeCollectionReader{repo: env.cols}, nil, nil, cleaner, NewTxRunner(env.db))
}

type exampleRow struct {
	workspaceID string
	isDelete    int
	version     int
}

func (env *cascadeEnv) exampleRows(t *testing.T) map[string]exampleRow {
	t.Helper()
	rows, err := env.db.Query(`SELECT id, workspace_id, is_delete, version FROM response_examples WHERE request_id = ?`,
		env.req.ID.String())
	if err != nil {
		t.Fatalf("query examples: %v", err)
	}
	defer func() { _ = rows.Close() }()
	out := make(map[string]exampleRow)
	for rows.Next() {
		var id string
		var r exampleRow
		if err := rows.Scan(&id, &r.workspaceID, &r.isDelete, &r.version); err != nil {
			t.Fatalf("scan: %v", err)
		}
		out[id] = r
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("rows: %v", err)
	}
	return out
}

func TestExampleCascade_RequestDeleteSoftDeletesExamples(t *testing.T) {
	env := newCascadeEnv(t)
	uc := env.requestUsecase(env.examples)

	if err := uc.Delete(testCtx(t), request.DeleteOpt{RequestID: env.req.ID, UserID: "test_user", Version: 1}); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	rows := env.exampleRows(t)
	if len(rows) != 3 {
		t.Fatalf("rows = %d, want 3", len(rows))
	}
	for id, r := range rows {
		if r.isDelete != 1 || r.version != 2 {
			t.Errorf("example %s = %+v, want soft-deleted at version 2", id, r)
		}
	}
}

func TestExampleCascade_FailedCascadeRollsBackRequestAndExamples(t *testing.T) {
	env := newCascadeEnv(t)
	uc := env.requestUsecase(failingCleaner{env.examples})
	ctx := testCtx(t)

	err := uc.Delete(ctx, request.DeleteOpt{RequestID: env.req.ID, UserID: "test_user", Version: 1})
	if !errors.Is(err, errCascade) {
		t.Fatalf("err = %v, want the cascade error", err)
	}

	got, err := NewRequestRepo(env.db).GetByID(ctx, env.req.ID)
	if err != nil || got == nil || got.Version != 1 {
		t.Fatalf("request after rollback = %+v, err %v; want it live at version 1", got, err)
	}
	for id, r := range env.exampleRows(t) {
		if r.isDelete != 0 || r.version != 1 {
			t.Errorf("example %s = %+v, want untouched", id, r)
		}
	}
}

func TestExampleCascade_MoveToAnotherWorkspaceCopiesExamples(t *testing.T) {
	env := newCascadeEnv(t)
	ctx := testCtx(t)
	otherWS := uuid.New()
	if _, err := env.db.Exec(`INSERT INTO workspaces (id, name) VALUES (?, 'Other')`, otherWS.String()); err != nil {
		t.Fatalf("insert workspace: %v", err)
	}
	target := newTestCollection("Target", nil)
	target.WorkspaceID = otherWS
	if err := env.cols.Create(ctx, target); err != nil {
		t.Fatalf("create target: %v", err)
	}
	before := env.exampleRows(t)

	if _, err := env.requestUsecase(env.examples).Move(ctx, request.MoveOpt{
		RequestID: env.req.ID, TargetCollectionID: target.ID, UserID: "test_user", Version: 1,
	}); err != nil {
		t.Fatalf("Move: %v", err)
	}

	after := env.exampleRows(t)
	if len(after) != 6 {
		t.Fatalf("rows = %d, want 3 tombstoned originals and 3 copies", len(after))
	}
	for id, r := range after {
		if _, original := before[id]; original {
			if r.isDelete != 1 || r.workspaceID != testWorkspaceID.String() {
				t.Errorf("original %s = %+v, want soft-deleted in the old workspace", id, r)
			}
			continue
		}
		if r.isDelete != 0 || r.workspaceID != otherWS.String() || r.version != 1 {
			t.Errorf("copy %s = %+v, want live in the new workspace", id, r)
		}
	}

	list, err := env.examples.ListByRequest(ctx, env.req.ID)
	if err != nil || len(list) != 3 {
		t.Fatalf("visible examples = %d, err %v; want the 3 copies", len(list), err)
	}
}
