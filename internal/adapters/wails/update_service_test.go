package wails

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tetiva-app/client/internal/adapters/wails/dto"
	"github.com/tetiva-app/client/internal/domain"
	"github.com/tetiva-app/client/internal/domain/entities"
	"github.com/tetiva-app/client/internal/infrastructure/repository/sqlite"
)

type fakeUpdateUsecase struct {
	state    entities.UpdateState
	applyErr error
	applied  []entities.RestoreTabs
	restore  *entities.RestoreTabs
}

func (f *fakeUpdateUsecase) Start(context.Context)                      {}
func (f *fakeUpdateUsecase) Status() entities.UpdateState               { return f.state }
func (f *fakeUpdateUsecase) Check(context.Context) entities.UpdateState { return f.state }
func (f *fakeUpdateUsecase) Download(context.Context) entities.UpdateState {
	return f.state
}
func (f *fakeUpdateUsecase) Cancel() {}

func (f *fakeUpdateUsecase) Apply(_ context.Context, r entities.RestoreTabs) error {
	f.applied = append(f.applied, r)
	return f.applyErr
}

func (f *fakeUpdateUsecase) TakeRestore() (entities.RestoreTabs, bool) {
	if f.restore == nil {
		return entities.RestoreTabs{}, false
	}
	return *f.restore, true
}

func newUpdateServiceForTest(t *testing.T, uc *fakeUpdateUsecase, wsUC *fakeWSUsecase) (*UpdateService, chan struct{}) {
	t.Helper()
	svc := NewUpdateService(uc, wsUC, NewUpdateEventSink(), sqlite.DataDir(t.TempDir()))
	svc.exit = func(int) {}
	quit := make(chan struct{}, 2)
	svc.SetQuit(func() { quit <- struct{}{} })
	return svc, quit
}

func TestUpdateService_Status_MapsEveryField(t *testing.T) {
	uc := &fakeUpdateUsecase{state: entities.UpdateState{
		Phase:    entities.UpdateDownloading,
		Version:  "1.2.2",
		Current:  "1.2.1",
		Received: 10,
		Total:    20,
		Install:  entities.InstallInApp,
		Reason:   "install_failed",
	}}
	svc, _ := newUpdateServiceForTest(t, uc, &fakeWSUsecase{})

	res := svc.Status()

	require.Nil(t, res.Error)
	assert.Equal(t, dto.UpdateStateDTO{
		Phase:    "downloading",
		Version:  "1.2.2",
		Current:  "1.2.1",
		Received: 10,
		Total:    20,
		Install:  "in_app",
		Reason:   "install_failed",
	}, res.Data)
}

func TestUpdateService_Apply_QuitsOnce(t *testing.T) {
	uc := &fakeUpdateUsecase{}
	svc, quit := newUpdateServiceForTest(t, uc, &fakeWSUsecase{})

	res := svc.Apply(dto.RestoreTabsDTO{
		WorkspaceID: "ws-1",
		Tabs:        []dto.RestoreTabDTO{{Type: "request", ID: "r-1"}},
		ActiveTabID: "r-1",
	})

	require.Nil(t, res.Error)
	select {
	case <-quit:
	case <-time.After(time.Second):
		t.Fatal("quit not called")
	}
	select {
	case <-quit:
		t.Fatal("quit called twice")
	case <-time.After(50 * time.Millisecond):
	}
	require.Len(t, uc.applied, 1)
	got := uc.applied[0]
	assert.Equal(t, "ws-1", got.WorkspaceID)
	assert.Equal(t, []entities.RestoreTab{{Type: "request", ID: "r-1"}}, got.Tabs)
	assert.Equal(t, "r-1", got.ActiveTabID)
	assert.False(t, got.SavedAt.IsZero())
}

func TestUpdateService_Apply_ErrorKeepsRunning(t *testing.T) {
	uc := &fakeUpdateUsecase{applyErr: fmt.Errorf("appupdate.Apply: %w",
		&domain.ReasonError{Reason: "install_failed", Err: errors.New("swap failed")})}
	svc, quit := newUpdateServiceForTest(t, uc, &fakeWSUsecase{})

	res := svc.Apply(dto.RestoreTabsDTO{})

	require.NotNil(t, res.Error)
	assert.Equal(t, "install_failed", res.Error.Reason)
	select {
	case <-quit:
		t.Fatal("quit called after a failed install")
	case <-time.After(50 * time.Millisecond):
	}
}

func TestUpdateService_TakeRestore_Empty(t *testing.T) {
	svc, _ := newUpdateServiceForTest(t, &fakeUpdateUsecase{}, &fakeWSUsecase{})

	res := svc.TakeRestore()

	require.Nil(t, res.Error)
	assert.Nil(t, res.Data)
}

func TestUpdateService_TakeRestore_Saved(t *testing.T) {
	uc := &fakeUpdateUsecase{restore: &entities.RestoreTabs{
		WorkspaceID: "ws-1",
		Tabs:        []entities.RestoreTab{{Type: "request", ID: "r-1"}, {Type: "collection", ID: "c-1"}},
		ActiveTabID: "c-1",
	}}
	svc, _ := newUpdateServiceForTest(t, uc, &fakeWSUsecase{})

	res := svc.TakeRestore()

	require.Nil(t, res.Error)
	require.NotNil(t, res.Data)
	assert.Equal(t, dto.RestoreTabsDTO{
		WorkspaceID: "ws-1",
		Tabs:        []dto.RestoreTabDTO{{Type: "request", ID: "r-1"}, {Type: "collection", ID: "c-1"}},
		ActiveTabID: "c-1",
	}, *res.Data)
}

func TestUpdateService_OpenConnections(t *testing.T) {
	svc, _ := newUpdateServiceForTest(t, &fakeUpdateUsecase{}, &fakeWSUsecase{count: 2})

	res := svc.OpenConnections()

	require.Nil(t, res.Error)
	assert.Equal(t, 2, res.Data)
}

func TestUpdateEventSink_Publish(t *testing.T) {
	sink := NewUpdateEventSink()
	var name string
	var data any
	sink.SetEmit(func(n string, d any) { name, data = n, d })

	sink.Publish(entities.UpdateState{Phase: entities.UpdateReady, Version: "1.2.2"})

	assert.Equal(t, "update:state", name)
	assert.Equal(t, dto.UpdateStateDTO{Phase: "ready", Version: "1.2.2"}, data)
}

func TestUpdateEventSink_NoEmitter(t *testing.T) {
	assert.NotPanics(t, func() {
		NewUpdateEventSink().Publish(entities.UpdateState{Phase: entities.UpdateReady})
	})
}
