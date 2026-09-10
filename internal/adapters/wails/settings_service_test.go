package wails

import (
	"context"
	"errors"
	"net/http"
	"testing"

	mcpadapter "github.com/tetiva-app/client/internal/adapters/mcp"
	"github.com/tetiva-app/client/internal/adapters/wails/dto"
	"github.com/tetiva-app/client/internal/domain/usecase/settings"
)

type fakeRepo struct {
	data       map[string]string
	getErr     map[string]error
	setManyErr error
}

func (r *fakeRepo) Get(_ context.Context, k string) (string, bool, error) {
	if err, ok := r.getErr[k]; ok {
		return "", false, err
	}
	v, ok := r.data[k]
	return v, ok, nil
}
func (r *fakeRepo) Set(_ context.Context, k, v string) error { r.data[k] = v; return nil }

// SetMany is all-or-nothing, matching the SQLite transaction.
func (r *fakeRepo) SetMany(_ context.Context, values map[string]string) error {
	if r.setManyErr != nil {
		return r.setManyErr
	}
	for k, v := range values {
		r.data[k] = v
	}
	return nil
}

func boolPtr(v bool) *bool { return &v }

func newSvc(status *mcpadapter.RuntimeStatus) *SettingsService {
	return NewSettingsService(settings.NewUsecase(&fakeRepo{data: map[string]string{}}), status,
		mcpadapter.NewTokenAuth("", false))
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
		&mcpadapter.RuntimeStatus{Addr: "127.0.0.1:9300", Enabled: true, Running: true},
		mcpadapter.NewTokenAuth("", false))

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
	res := svc.SetMCPSettings(dto.SetMCPSettingsRequest{Enabled: true, Addr: ":9400", RequireToken: boolPtr(true)})
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

func TestGetMCPSettings_GeneratesTokenAndRequiresItByDefault(t *testing.T) {
	svc := newSvc(&mcpadapter.RuntimeStatus{Addr: ":9300"})

	res := svc.GetMCPSettings()
	if res.Error != nil {
		t.Fatal(res.Error.Message)
	}
	if res.Data.Token == "" {
		t.Fatal("expected a generated token")
	}
	if !res.Data.RequireToken {
		t.Error("expected the token to be required by default")
	}

	if again := svc.GetMCPSettings(); again.Data.Token != res.Data.Token {
		t.Error("reading settings must not rotate the token")
	}
}

// A UI save writes the whole MCP config; the token must survive it.
func TestSetMCPSettings_KeepsTokenAndAppliesAuthLive(t *testing.T) {
	auth := mcpadapter.NewTokenAuth("", false)
	svc := NewSettingsService(settings.NewUsecase(&fakeRepo{data: map[string]string{}}),
		&mcpadapter.RuntimeStatus{Addr: ":9300"}, auth)

	before := svc.GetMCPSettings().Data.Token
	res := svc.SetMCPSettings(dto.SetMCPSettingsRequest{Enabled: true, Addr: ":9400", RequireToken: boolPtr(true)})
	if res.Error != nil {
		t.Fatal(res.Error.Message)
	}
	if res.Data.Token != before {
		t.Errorf("token changed on save: %q -> %q", before, res.Data.Token)
	}
	if !auth.Enabled() {
		t.Error("running server did not pick up the token requirement")
	}
}

func TestSetMCPSettings_RequireTokenOffDisarmsGuard(t *testing.T) {
	auth := mcpadapter.NewTokenAuth("", false)
	svc := NewSettingsService(settings.NewUsecase(&fakeRepo{data: map[string]string{}}),
		&mcpadapter.RuntimeStatus{Addr: ":9300"}, auth)

	svc.GetMCPSettings()
	res := svc.SetMCPSettings(dto.SetMCPSettingsRequest{Enabled: true, Addr: ":9400", RequireToken: boolPtr(false)})
	if res.Error != nil {
		t.Fatal(res.Error.Message)
	}
	if res.Data.RequireToken || auth.Enabled() {
		t.Error("expected the guard to be off")
	}
}

func TestRegenerateMCPToken_RotatesAndAppliesLive(t *testing.T) {
	auth := mcpadapter.NewTokenAuth("", false)
	svc := NewSettingsService(settings.NewUsecase(&fakeRepo{data: map[string]string{}}),
		&mcpadapter.RuntimeStatus{Addr: ":9300"}, auth)

	before := svc.GetMCPSettings().Data.Token
	res := svc.RegenerateMCPToken()
	if res.Error != nil {
		t.Fatal(res.Error.Message)
	}
	if res.Data.Token == before || res.Data.Token == "" {
		t.Errorf("expected a fresh token, got %q", res.Data.Token)
	}
	if !auth.Enabled() {
		t.Error("regenerated token was not pushed to the running server")
	}
}

// resolveMCPAuth falls back to an unsaved ephemeral token when storage misfires;
// a later save used to push the empty stored token over it and open the server.
func TestSetMCPSettings_NeverBlanksTheRunningGuard(t *testing.T) {
	auth := mcpadapter.NewTokenAuth("ephemeral-token", true)
	svc := NewSettingsService(settings.NewUsecase(&fakeRepo{data: map[string]string{}}),
		&mcpadapter.RuntimeStatus{Addr: ":9300"}, auth)

	res := svc.SetMCPSettings(dto.SetMCPSettingsRequest{Enabled: true, Addr: ":9400", RequireToken: boolPtr(true)})
	if res.Error != nil {
		t.Fatal(res.Error.Message)
	}
	if res.Data.Token == "" {
		t.Fatal("expected a token to be generated on save")
	}

	anon, err := http.NewRequest(http.MethodGet, "http://127.0.0.1:9400/sse", nil)
	if err != nil {
		t.Fatal(err)
	}
	if auth.Allow(anon) {
		t.Error("guard let an anonymous request through after a save")
	}

	withToken, err := http.NewRequest(http.MethodGet, "http://127.0.0.1:9400/sse?token="+res.Data.Token, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !auth.Allow(withToken) {
		t.Error("guard rejected the token it just handed out")
	}
}

// A request that omits requireToken must not read as "turn the check off".
func TestSetMCPSettings_OmittedRequireTokenKeepsGuard(t *testing.T) {
	auth := mcpadapter.NewTokenAuth("", false)
	svc := NewSettingsService(settings.NewUsecase(&fakeRepo{data: map[string]string{}}),
		&mcpadapter.RuntimeStatus{Addr: ":9300"}, auth)

	svc.GetMCPSettings()
	res := svc.SetMCPSettings(dto.SetMCPSettingsRequest{Enabled: true, Addr: ":9400"})
	if res.Error != nil {
		t.Fatal(res.Error.Message)
	}
	if !res.Data.RequireToken || !auth.Enabled() {
		t.Error("omitting requireToken silently disabled the token check")
	}
}

// A failed config write must leave the running server exactly as it was: an
// armed guard the next launch will not find persisted is worse than no change.
func TestSetMCPSettings_FailedWriteAppliesNothing(t *testing.T) {
	repo := &fakeRepo{data: map[string]string{}, setManyErr: errors.New("disk full")}
	auth := mcpadapter.NewTokenAuth("", false)
	svc := NewSettingsService(settings.NewUsecase(repo), &mcpadapter.RuntimeStatus{Addr: ":9300"}, auth)

	res := svc.SetMCPSettings(dto.SetMCPSettingsRequest{Enabled: true, Addr: ":9400", RequireToken: boolPtr(true)})
	if res.Error == nil {
		t.Fatal("expected the save to fail")
	}
	if len(repo.data) != 0 {
		t.Errorf("partial config written: %v", repo.data)
	}
	if auth.Enabled() {
		t.Error("live guard was armed although nothing was saved")
	}
}

// The rotated token is dead in storage the moment it is written, so the running
// server must stop honouring the old one even when the follow-up read fails.
func TestRegenerateMCPToken_FlagReadFailureStillRevokesOldToken(t *testing.T) {
	repo := &fakeRepo{data: map[string]string{}}
	auth := mcpadapter.NewTokenAuth("old-token", true)
	svc := NewSettingsService(settings.NewUsecase(repo), &mcpadapter.RuntimeStatus{Addr: ":9300"}, auth)

	repo.getErr = map[string]error{"mcp.require_token": errors.New("read failed")}
	res := svc.RegenerateMCPToken()
	if res.Error == nil {
		t.Fatal("expected an error rather than a silent half-applied rotation")
	}

	old, err := http.NewRequest(http.MethodGet, "http://127.0.0.1:9300/sse?token=old-token", nil)
	if err != nil {
		t.Fatal(err)
	}
	if auth.Allow(old) {
		t.Error("the old token still works after a rotation")
	}

	fresh, err := http.NewRequest(http.MethodGet, "http://127.0.0.1:9300/sse?token="+repo.data["mcp.token"], nil)
	if err != nil {
		t.Fatal(err)
	}
	if !auth.Allow(fresh) {
		t.Error("the guard rejects the token that was just persisted")
	}
}

// The guard must be handed the token that was written, not a re-read of it.
func TestRegenerateMCPToken_GuardMatchesReturnedToken(t *testing.T) {
	repo := &fakeRepo{data: map[string]string{}}
	auth := mcpadapter.NewTokenAuth("", false)
	svc := NewSettingsService(settings.NewUsecase(repo), &mcpadapter.RuntimeStatus{Addr: ":9300"}, auth)

	res := svc.RegenerateMCPToken()
	if res.Error != nil {
		t.Fatal(res.Error.Message)
	}
	shown, err := http.NewRequest(http.MethodGet, "http://127.0.0.1:9300/sse?token="+res.Data.Token, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !auth.Allow(shown) {
		t.Error("the token the UI shows is not the one the guard accepts")
	}
}
