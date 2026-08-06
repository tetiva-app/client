package wails

import (
	"context"
	"testing"

	mcpadapter "github.com/tetiva-app/client/internal/adapters/mcp"
	"github.com/tetiva-app/client/internal/adapters/wails/dto"
	"github.com/tetiva-app/client/internal/domain/usecase/settings"
)

type fakeRepo struct{ data map[string]string }

func (r *fakeRepo) Get(_ context.Context, k string) (string, bool, error) {
	v, ok := r.data[k]
	return v, ok, nil
}
func (r *fakeRepo) Set(_ context.Context, k, v string) error { r.data[k] = v; return nil }

func newSvc(status *mcpadapter.RuntimeStatus) *SettingsService {
	return NewSettingsService(settings.NewUsecase(&fakeRepo{data: map[string]string{}}), status)
}

func TestGetMCPSettings_DefaultsAndSSEURL(t *testing.T) {
	svc := newSvc(&mcpadapter.RuntimeStatus{Addr: ":9300", Running: false})
	res := svc.GetMCPSettings()
	if res.Error != nil {
		t.Fatal(res.Error.Message)
	}
	if res.Data.SSEURL != "http://127.0.0.1:9300/sse" {
		t.Errorf("got sseUrl %q", res.Data.SSEURL)
	}
	if res.Data.Enabled || res.Data.EnvManaged {
		t.Errorf("expected disabled, not env-managed")
	}
}

// Settings must surface the address that will actually bind, not the legacy
// host-less one, otherwise RestartRequired never clears.
func TestGetMCPSettings_ReportsNormalizedAddr(t *testing.T) {
	repo := &fakeRepo{data: map[string]string{"mcp.enabled": "true", "mcp.addr": ":9300"}}
	svc := NewSettingsService(settings.NewUsecase(repo),
		&mcpadapter.RuntimeStatus{Addr: "127.0.0.1:9300", Enabled: true, Running: true})

	res := svc.GetMCPSettings()
	if res.Data.Addr != "127.0.0.1:9300" {
		t.Errorf("got addr %q", res.Data.Addr)
	}
	if res.Data.RestartRequired {
		t.Errorf("normalized addr matches the running server, no restart expected")
	}
}

func TestSetMCPSettings_PersistsAndReturns(t *testing.T) {
	svc := newSvc(&mcpadapter.RuntimeStatus{Addr: ":9300"})
	res := svc.SetMCPSettings(dto.SetMCPSettingsRequest{Enabled: true, Addr: ":9400"})
	if res.Error != nil {
		t.Fatal(res.Error.Message)
	}
	if !res.Data.Enabled || res.Data.Addr != "127.0.0.1:9400" {
		t.Errorf("got %+v", res.Data)
	}
	if !res.Data.RestartRequired {
		t.Errorf("expected restart required after enabling a stopped server")
	}
	res2 := svc.GetMCPSettings()
	if !res2.Data.Enabled || res2.Data.Addr != "127.0.0.1:9400" {
		t.Errorf("persisted state wrong: %+v", res2.Data)
	}
}

func TestSetMCPSettings_InvalidAddr(t *testing.T) {
	svc := newSvc(&mcpadapter.RuntimeStatus{Addr: ":9300"})
	res := svc.SetMCPSettings(dto.SetMCPSettingsRequest{Enabled: true, Addr: "not-an-addr"})
	if res.Error == nil || res.Error.Code != ErrCodeValidation {
		t.Errorf("expected validation error, got %+v", res.Error)
	}
}

func TestSetMCPSettings_RejectedWhenEnvManaged(t *testing.T) {
	svc := newSvc(&mcpadapter.RuntimeStatus{Addr: ":9300", EnvManaged: true, Enabled: true})
	res := svc.SetMCPSettings(dto.SetMCPSettingsRequest{Enabled: false, Addr: ":9300"})
	if res.Error == nil {
		t.Errorf("expected rejection when env-managed")
	}
}

func TestGetMCPSettings_EnvManagedUsesStatus(t *testing.T) {
	svc := newSvc(&mcpadapter.RuntimeStatus{Addr: ":9500", Enabled: true, Running: true, EnvManaged: true})
	res := svc.GetMCPSettings()
	if !res.Data.Enabled || res.Data.Addr != ":9500" || !res.Data.EnvManaged || !res.Data.Running {
		t.Errorf("got %+v", res.Data)
	}
}
