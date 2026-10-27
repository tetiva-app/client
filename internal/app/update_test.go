package app

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	wailsadapter "github.com/tetiva-app/client/internal/adapters/wails"
	"github.com/tetiva-app/client/internal/constants/flavortest"
	"github.com/tetiva-app/client/internal/domain/entities"
	"github.com/tetiva-app/client/internal/domain/usecase/appupdate"
	"github.com/tetiva-app/client/internal/infrastructure/repository/sqlite"
)

func TestUpdateUsecase_PreviewNeverUpdates(t *testing.T) {
	flavortest.Preview(t)
	dir := t.TempDir()

	uc, err := newUpdateUsecase(sqlite.DataDir(dir), wailsadapter.NewUpdateEventSink())
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	uc.Start(ctx)

	for _, st := range []entities.UpdateState{uc.Check(ctx), uc.Download(ctx), uc.Status()} {
		if st.Phase != entities.UpdateIdle || st.Reason != appupdate.ReasonPreviewBuild {
			t.Fatalf("got %+v", st)
		}
	}
	if err := uc.Apply(ctx, entities.RestoreTabs{}); err == nil {
		t.Fatal("Apply must refuse")
	}
	if _, err := os.Stat(filepath.Join(dir, "updates")); !os.IsNotExist(err) {
		t.Fatalf("the preview must not stage updates: %v", err)
	}
}

func TestUpdateUsecase_StableStagesInTheDataDir(t *testing.T) {
	dir := t.TempDir()

	if _, err := newUpdateUsecase(sqlite.DataDir(dir), wailsadapter.NewUpdateEventSink()); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "updates")); err != nil {
		t.Fatalf("updates dir: %v", err)
	}
}
