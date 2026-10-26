package request_test

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/tetiva-app/client/internal/domain"
	"github.com/tetiva-app/client/internal/domain/entities"
	"github.com/tetiva-app/client/internal/domain/usecase/request"
)

func seedSiblings(repo *mockRepo, names ...string) []*entities.Request {
	base := time.Date(2026, 10, 1, 9, 0, 0, 0, time.UTC)
	out := make([]*entities.Request, 0, len(names))
	for i, name := range names {
		r := &entities.Request{
			ID:           uuid.New(),
			CollectionID: testCollectionID,
			Name:         name,
			Protocol:     entities.ProtocolHTTP,
			Method:       entities.MethodGET,
			BodyType:     entities.BodyTypeNone,
			AuthType:     entities.AuthTypeNone,
			AuthData:     "{}",
			Version:      1,
			CreatedAt:    base.Add(time.Duration(i) * time.Minute),
			UpdatedAt:    base.Add(time.Duration(i) * time.Minute),
		}
		repo.requests[r.ID] = r
		out = append(out, r)
	}
	return out
}

func TestDuplicate_CopiesTheSavedRequest(t *testing.T) {
	repo := newMockRepo()
	original := seedSiblings(repo, "List pets", "Create pet", "Delete pet")[1]
	original.Description = "Adds a pet to the store"
	original.Method = entities.MethodPOST
	original.URL = "{{base}}/pets"
	original.Headers = []entities.HeaderItem{{Key: "X-Trace", Value: "1", Enabled: true}, {Key: "X-Off", Value: "2"}}
	original.Body = `{"name":"Rex"}`
	original.BodyType = entities.BodyTypeJSON
	original.AuthType = entities.AuthTypeBearer
	original.AuthData = `{"token":"{{token}}"}`
	original.PreScript = "tetiva.variables.set('id', 1)"
	original.PostScript = "tetiva.test('created', () => {})"
	original.SortOrder = 3
	original.Version = 7
	original.CreatedBy = "someone"
	snapshot, _ := repo.GetByID(context.Background(), original.ID)
	cleaner := &recordingExampleCleaner{}
	uc := newCascadeUsecase(repo, fixtureCollections(), nil, cleaner, &snapshotTx{repo: repo})
	before := time.Now()

	dup, err := uc.Duplicate(context.Background(), request.DuplicateOpt{
		RequestID: original.ID, Name: "Create pet (copy)", UserID: "u1",
	})
	if err != nil {
		t.Fatalf("Duplicate: %v", err)
	}

	if dup.ID == uuid.Nil || dup.ID == original.ID {
		t.Fatalf("copy id = %s, want a new one", dup.ID)
	}
	if dup.CreatedAt.Before(before) || !dup.UpdatedAt.Equal(dup.CreatedAt) {
		t.Errorf("timestamps = %s / %s, want both fresh", dup.CreatedAt, dup.UpdatedAt)
	}
	want := *snapshot
	want.ID = dup.ID
	want.Name = "Create pet (copy)"
	want.SortOrder = 0
	want.Version = 1
	want.CreatedBy, want.UpdatedBy = "u1", "u1"
	want.CreatedAt, want.UpdatedAt = dup.CreatedAt, dup.UpdatedAt
	if !reflect.DeepEqual(*dup, want) {
		t.Errorf("copy =\n%+v\nwant\n%+v", *dup, want)
	}
	stored, err := repo.GetByID(context.Background(), dup.ID)
	if err != nil || stored == nil || !reflect.DeepEqual(*stored, want) {
		t.Errorf("stored copy = %+v, err %v", stored, err)
	}
	if got, _ := repo.GetByID(context.Background(), original.ID); !reflect.DeepEqual(got, snapshot) {
		t.Errorf("original = %+v, want it untouched", got)
	}

	if len(cleaner.copies) != 1 {
		t.Fatalf("example copies = %d, want 1", len(cleaner.copies))
	}
	if c := cleaner.copies[0]; c.requestID != original.ID || c.targetID != dup.ID || c.userID != "u1" || !c.inTx {
		t.Errorf("example copy call = %+v", c)
	}
}

func TestDuplicate_TakesMethodAndURLForTheCopy(t *testing.T) {
	repo := newMockRepo()
	original := seedSiblings(repo, "Create order")[0]
	original.Method = entities.MethodPOST
	original.URL = "{{base}}/api/order"
	original.Body = `{"qty":1}`
	uc := newCascadeUsecase(repo, fixtureCollections(), nil, &recordingExampleCleaner{}, &snapshotTx{repo: repo})

	dup, err := uc.Duplicate(context.Background(), request.DuplicateOpt{
		RequestID: original.ID, Name: "Update order", Method: entities.MethodPATCH, URL: "{{base}}/api/order/1", UserID: "u1",
	})
	if err != nil {
		t.Fatalf("Duplicate: %v", err)
	}

	if dup.Name != "Update order" || dup.Method != entities.MethodPATCH || dup.URL != "{{base}}/api/order/1" || dup.Body != `{"qty":1}` {
		t.Errorf("copy = %s %s %s body %s", dup.Name, dup.Method, dup.URL, dup.Body)
	}
	if original.Method != entities.MethodPOST || original.URL != "{{base}}/api/order" {
		t.Errorf("original = %s %s, want it untouched", original.Method, original.URL)
	}
}

type concurrentMoveTx struct {
	snapshotTx
	requestID, targetID uuid.UUID
}

func (t *concurrentMoveTx) Run(ctx context.Context, fn func(ctx context.Context) error) error {
	t.repo.requests[t.requestID].CollectionID = t.targetID
	return t.snapshotTx.Run(ctx, fn)
}

func TestDuplicate_ReadsTheOriginalInsideTheTx(t *testing.T) {
	repo := newMockRepo()
	original := seedSiblings(repo, "Create pet")[0]
	cols, otherID := cascadeCollections()
	tx := &concurrentMoveTx{snapshotTx: snapshotTx{repo: repo}, requestID: original.ID, targetID: otherID}
	uc := newCascadeUsecase(repo, cols, nil, &recordingExampleCleaner{}, tx)

	dup, err := uc.Duplicate(context.Background(), request.DuplicateOpt{RequestID: original.ID, Name: "Create pet (copy)", UserID: "u1"})
	if err != nil {
		t.Fatalf("Duplicate: %v", err)
	}

	if dup.CollectionID != otherID {
		t.Errorf("copy collection = %s, want %s where the original was moved", dup.CollectionID, otherID)
	}
}

func TestDuplicate_RejectsBadInput(t *testing.T) {
	repo := newMockRepo()
	reqs := seedSiblings(repo, "Live", "Replay")
	reqs[1].IsDraft = true
	cleaner := &recordingExampleCleaner{}
	uc := newCascadeUsecase(repo, fixtureCollections(), nil, cleaner, &snapshotTx{repo: repo})

	var notFound *domain.NotFoundError
	var invalid *domain.ValidationError
	cases := []struct {
		name   string
		opt    request.DuplicateOpt
		target any
	}{
		{"missing original", request.DuplicateOpt{RequestID: uuid.New(), Name: "x (copy)"}, &notFound},
		{"replay draft", request.DuplicateOpt{RequestID: reqs[1].ID, Name: "Replay (copy)"}, &invalid},
		{"empty name", request.DuplicateOpt{RequestID: reqs[0].ID}, &invalid},
		{"unknown method", request.DuplicateOpt{RequestID: reqs[0].ID, Name: "Live (copy)", Method: "FETCH"}, &invalid},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := uc.Duplicate(context.Background(), tc.opt)
			if !errors.As(err, tc.target) {
				t.Fatalf("err = %v, want %T", err, tc.target)
			}
			if len(repo.requests) != 2 || len(cleaner.copies) != 0 {
				t.Errorf("requests = %d, example copies = %d; want no copy", len(repo.requests), len(cleaner.copies))
			}
		})
	}
}

func TestDuplicate_ExampleCopyFailureLeavesNoCopy(t *testing.T) {
	repo := newMockRepo()
	reqs := seedSiblings(repo, "A", "B", "C")
	cleaner := &recordingExampleCleaner{err: errors.New("disk full")}
	uc := newCascadeUsecase(repo, fixtureCollections(), nil, cleaner, &snapshotTx{repo: repo})

	_, err := uc.Duplicate(context.Background(), request.DuplicateOpt{RequestID: reqs[0].ID, Name: "A (copy)", UserID: "u1"})
	if !errors.Is(err, cleaner.err) {
		t.Fatalf("err = %v, want the example copy error", err)
	}

	if len(repo.requests) != 3 {
		t.Errorf("requests = %d, want the copy rolled back", len(repo.requests))
	}
}
