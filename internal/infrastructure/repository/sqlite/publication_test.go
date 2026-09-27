package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/tetiva-app/client/internal/domain/entities"
)

func seedCloudWorkspace(t *testing.T, db *sql.DB) uuid.UUID {
	t.Helper()
	now := time.Now().Truncate(time.Second)
	remote := "remote-ws"
	ws := &entities.Workspace{
		ID: uuid.New(), Name: "Cloud", RemoteWorkspaceID: &remote, Version: 1,
		CreatedBy: "test_user", CreatedAt: now, UpdatedBy: "test_user", UpdatedAt: now,
	}
	if err := NewWorkspaceRepo(db).Create(context.Background(), ws); err != nil {
		t.Fatalf("create cloud workspace: %v", err)
	}
	return ws.ID
}

func newTestPublicationRow(workspaceID uuid.UUID) *PublicationRow {
	return &PublicationRow{
		CollectionID:  uuid.New(),
		WorkspaceID:   workspaceID,
		OwnerKey:      "sync.test:443\na@b.c",
		PublicationID: "pub-" + uuid.NewString()[:8],
		Slug:          "petstore-k3f9x2qa",
		PublicURL:     "https://share.tetiva.app/petstore-k3f9x2qa",
		Visibility:    "unlisted",
		Status:        "active",
		ContentHash:   "abc123",
		Revision:      3,
		Badge:         true,
		CanManage:     true,
		Settings: &PublicationSettings{
			EnvironmentID: uuid.NewString(), EnvironmentName: "Prod", IncludeScripts: true,
			PublishAsIs: []string{"d93148ea5eb1/var/0"},
		},
		Counters:        PublicationCounters{Views: 10, Imports: 2, Downloads: 1},
		ServerUpdatedAt: time.Date(2026, 9, 25, 10, 0, 0, 0, time.UTC),
		RefreshedAt:     time.Date(2026, 9, 25, 11, 0, 0, 0, time.UTC),
	}
}

func pendingOf(t *testing.T, repo *PublicationRepo, r *PublicationRow) bool {
	t.Helper()
	row, err := repo.Get(context.Background(), r.OwnerKey, r.CollectionID)
	if err != nil {
		t.Fatal(err)
	}
	if row == nil {
		t.Fatalf("row %s is gone", r.CollectionID)
	}
	return row.PendingUnpublish
}

func TestPublicationRepo_UpsertGetList(t *testing.T) {
	db := setupTestDB(t)
	repo := NewPublicationRepo(db)
	ctx := context.Background()

	missing, err := repo.Get(ctx, "sync.test:443\na@b.c", uuid.New())
	if err != nil || missing != nil {
		t.Fatalf("Get(missing) = %v, %v; want nil, nil", missing, err)
	}

	row := newTestPublicationRow(testWorkspaceID)
	row.Cloud, row.UnpublishAttempts, row.UnpublishError = true, 2, "unknown service"
	if err := repo.Upsert(ctx, row); err != nil {
		t.Fatalf("Upsert: %v", err)
	}
	got, err := repo.Get(ctx, row.OwnerKey, row.CollectionID)
	if err != nil {
		t.Fatal(err)
	}
	if got.CreatedAt.IsZero() || got.UpdatedAt.IsZero() {
		t.Errorf("timestamps not set: created %v, updated %v", got.CreatedAt, got.UpdatedAt)
	}
	want := *row
	want.CreatedAt, want.UpdatedAt = got.CreatedAt, got.UpdatedAt
	if !reflect.DeepEqual(*got, want) {
		t.Errorf("round trip:\n got %+v\nwant %+v", *got, want)
	}

	created := got.CreatedAt
	if _, err := db.Exec(`UPDATE publications SET created_at = '2026-01-01T00:00:00Z', updated_at = '2026-01-01T00:00:00Z'`); err != nil {
		t.Fatal(err)
	}
	row.Settings = nil
	row.Status = "revoked"
	if err := repo.Upsert(ctx, row); err != nil {
		t.Fatalf("second Upsert: %v", err)
	}
	got, err = repo.Get(ctx, row.OwnerKey, row.CollectionID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Settings != nil || got.Status != "revoked" {
		t.Errorf("update lost: settings %+v, status %q", got.Settings, got.Status)
	}
	if !got.CreatedAt.Equal(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)) {
		t.Errorf("created_at rewritten on update: %v (first insert %v)", got.CreatedAt, created)
	}
	if !got.UpdatedAt.After(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)) {
		t.Errorf("updated_at not refreshed: %v", got.UpdatedAt)
	}

	other := newTestPublicationRow(testWorkspaceID)
	other.Settings = &PublicationSettings{}
	if err := repo.Upsert(ctx, other); err != nil {
		t.Fatal(err)
	}
	list, err := repo.List(ctx, row.OwnerKey)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 2 {
		t.Fatalf("List = %d rows, want 2", len(list))
	}
	for _, r := range list {
		if r.CollectionID == other.CollectionID && (r.Settings == nil || r.Settings.PublishAsIs == nil) {
			t.Errorf("empty settings read back as %+v, want non-nil with an empty list", r.Settings)
		}
	}

	if err := repo.Delete(ctx, row.OwnerKey, row.CollectionID); err != nil {
		t.Fatal(err)
	}
	if got, _ := repo.Get(ctx, row.OwnerKey, row.CollectionID); got != nil {
		t.Error("Delete left the row")
	}
}

func TestPublicationRepo_RowsBelongToTheirOwner(t *testing.T) {
	db := setupTestDB(t)
	repo := NewPublicationRepo(db)
	ctx := context.Background()

	mine := newTestPublicationRow(testWorkspaceID)
	theirs := newTestPublicationRow(testWorkspaceID)
	theirs.CollectionID, theirs.OwnerKey, theirs.PublicationID = mine.CollectionID, "other.server:443\na@b.c", "pub-theirs"
	for _, r := range []*PublicationRow{mine, theirs} {
		if err := repo.Upsert(ctx, r); err != nil {
			t.Fatal(err)
		}
	}

	got, err := repo.Get(ctx, theirs.OwnerKey, mine.CollectionID)
	if err != nil || got == nil || got.PublicationID != "pub-theirs" {
		t.Fatalf("Get(theirs) = %+v, %v", got, err)
	}
	if got, _ := repo.Get(ctx, "", mine.CollectionID); got != nil {
		t.Error("a signed-out read found a row")
	}
	list, err := repo.List(ctx, mine.OwnerKey)
	if err != nil || len(list) != 1 || list[0].PublicationID != mine.PublicationID {
		t.Fatalf("List(mine) = %+v, %v", list, err)
	}

	if err := repo.Delete(ctx, mine.OwnerKey, mine.CollectionID); err != nil {
		t.Fatal(err)
	}
	if got, _ := repo.Get(ctx, theirs.OwnerKey, mine.CollectionID); got == nil {
		t.Error("deleting my row took another account's row with it")
	}
}

func TestPublicationRepo_WorkspaceLinked(t *testing.T) {
	db := setupTestDB(t)
	repo := NewPublicationRepo(db)
	ctx := context.Background()
	deletedID := seedCloudWorkspace(t, db)
	if _, err := db.Exec(`UPDATE workspaces SET remote_workspace_id = 'remote-ws-gone', is_delete = 1 WHERE id = ?`, deletedID.String()); err != nil {
		t.Fatal(err)
	}
	linkedID := seedCloudWorkspace(t, db)

	for _, c := range []struct {
		name string
		id   uuid.UUID
		want bool
	}{
		{"linked", linkedID, true},
		{"linked, soft-deleted", deletedID, true},
		{"local", testWorkspaceID, false},
		{"missing", uuid.New(), false},
	} {
		got, err := repo.WorkspaceLinked(ctx, c.id)
		if err != nil || got != c.want {
			t.Errorf("%s: WorkspaceLinked = %v, %v; want %v", c.name, got, err, c.want)
		}
	}
}

func TestPublicationRepo_MarkPendingUnpublish(t *testing.T) {
	db := setupTestDB(t)
	repo := NewPublicationRepo(db)
	ctx := context.Background()
	linkedID := seedCloudWorkspace(t, db)

	local := newTestPublicationRow(testWorkspaceID)
	cloud := newTestPublicationRow(testWorkspaceID)
	cloud.Cloud = true
	linked := newTestPublicationRow(linkedID)
	for _, r := range []*PublicationRow{local, cloud, linked} {
		if err := repo.Upsert(ctx, r); err != nil {
			t.Fatal(err)
		}
	}

	if err := repo.MarkPendingUnpublish(ctx, []uuid.UUID{uuid.New()}); err != nil {
		t.Fatalf("no row: %v", err)
	}
	if list, _ := repo.List(ctx, local.OwnerKey); len(list) != 3 {
		t.Fatalf("a mark without a row created one: %d rows", len(list))
	}
	if err := repo.MarkPendingUnpublish(ctx, nil); err != nil {
		t.Fatalf("empty list: %v", err)
	}

	if err := repo.MarkPendingUnpublish(ctx, []uuid.UUID{local.CollectionID, cloud.CollectionID, linked.CollectionID}); err != nil {
		t.Fatal(err)
	}
	if !pendingOf(t, repo, local) {
		t.Error("local publication not marked")
	}
	if pendingOf(t, repo, cloud) {
		t.Error("cloud publication marked: its live page belongs to the team")
	}
	if pendingOf(t, repo, linked) {
		t.Error("publication of a linked workspace marked: the server moves it into the cloud")
	}

	signedOut := newTestPublicationRow(testWorkspaceID)
	signedOut.CollectionID, signedOut.OwnerKey = local.CollectionID, "other.server:443\na@b.c"
	if err := repo.Upsert(ctx, signedOut); err != nil {
		t.Fatal(err)
	}
	if err := repo.MarkPendingUnpublish(ctx, []uuid.UUID{local.CollectionID}); err != nil {
		t.Fatal(err)
	}
	if !pendingOf(t, repo, signedOut) {
		t.Error("a delete marks the row of every account, signed in or not")
	}
}

func TestPublicationRepo_MarkPendingUnpublishWorkspace(t *testing.T) {
	db := setupTestDB(t)
	repo := NewPublicationRepo(db)
	ctx := context.Background()
	otherID := seedCloudWorkspace(t, db)

	first := newTestPublicationRow(testWorkspaceID)
	second := newTestPublicationRow(testWorkspaceID)
	cloud := newTestPublicationRow(testWorkspaceID)
	cloud.Cloud = true
	elsewhere := newTestPublicationRow(otherID)
	for _, r := range []*PublicationRow{first, second, cloud, elsewhere} {
		if err := repo.Upsert(ctx, r); err != nil {
			t.Fatal(err)
		}
	}

	if err := repo.MarkPendingUnpublishWorkspace(ctx, testWorkspaceID); err != nil {
		t.Fatal(err)
	}
	if !pendingOf(t, repo, first) || !pendingOf(t, repo, second) {
		t.Error("not every local publication of the workspace was marked")
	}
	if pendingOf(t, repo, cloud) {
		t.Error("cloud publication marked")
	}
	if pendingOf(t, repo, elsewhere) {
		t.Error("another workspace's row was marked")
	}

	if _, err := db.Exec(`UPDATE workspaces SET is_delete = 1 WHERE id = ?`, otherID.String()); err != nil {
		t.Fatal(err)
	}
	if err := repo.MarkPendingUnpublishWorkspace(ctx, otherID); err != nil {
		t.Fatal(err)
	}
	if pendingOf(t, repo, elsewhere) {
		t.Error("the local copy of a linked workspace went and took its page with it")
	}
}

func TestPublicationRepo_MarkRollsBackWithTheTransaction(t *testing.T) {
	db := setupTestDB(t)
	repo := NewPublicationRepo(db)
	ctx := context.Background()

	row := newTestPublicationRow(testWorkspaceID)
	if err := repo.Upsert(ctx, row); err != nil {
		t.Fatal(err)
	}

	boom := errors.New("delete failed")
	err := WithTx(ctx, db, func(ctx context.Context) error {
		if err := repo.MarkPendingUnpublish(ctx, []uuid.UUID{row.CollectionID}); err != nil {
			return err
		}
		if err := repo.MarkPendingUnpublishWorkspace(ctx, testWorkspaceID); err != nil {
			return err
		}
		return boom
	})
	if !errors.Is(err, boom) {
		t.Fatalf("WithTx = %v", err)
	}
	if pendingOf(t, repo, row) {
		t.Error("the mark outlived its rolled-back transaction")
	}
}

func TestPublicationRepo_MarkSignalsOnceTheDeleteCommits(t *testing.T) {
	db := setupTestDB(t)
	repo := NewPublicationRepo(db)
	ctx := context.Background()

	row := newTestPublicationRow(testWorkspaceID)
	cloud := newTestPublicationRow(testWorkspaceID)
	cloud.Cloud = true
	for _, r := range []*PublicationRow{row, cloud} {
		if err := repo.Upsert(ctx, r); err != nil {
			t.Fatal(err)
		}
	}
	signalled := func() bool {
		select {
		case <-repo.Marked():
			return true
		default:
			return false
		}
	}

	if err := repo.MarkPendingUnpublish(ctx, []uuid.UUID{uuid.New(), cloud.CollectionID}); err != nil {
		t.Fatal(err)
	}
	if signalled() {
		t.Error("a delete that marked nothing signalled")
	}

	boom := errors.New("delete failed")
	_ = WithTx(ctx, db, func(ctx context.Context) error {
		if err := repo.MarkPendingUnpublish(ctx, []uuid.UUID{row.CollectionID}); err != nil {
			return err
		}
		return boom
	})
	if signalled() {
		t.Error("a rolled-back delete signalled")
	}

	if err := WithTx(ctx, db, func(ctx context.Context) error {
		if err := repo.MarkPendingUnpublish(ctx, []uuid.UUID{row.CollectionID}); err != nil {
			return err
		}
		if signalled() {
			t.Error("signalled before the delete committed")
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if !signalled() {
		t.Error("a committed mark did not signal")
	}

	if err := repo.MarkPendingUnpublishWorkspace(ctx, testWorkspaceID); err != nil {
		t.Fatal(err)
	}
	if !signalled() {
		t.Error("a workspace delete that marked a row did not signal")
	}
}
