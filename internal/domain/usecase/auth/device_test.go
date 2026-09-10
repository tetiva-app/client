package auth

import (
	"context"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

const deviceOK = `{"device_code":"dc-1","user_code":"WDJB-MJHT",` +
	`"verification_uri":"https://idp.example/device",` +
	`"verification_uri_complete":"https://idp.example/device?user_code=WDJB-MJHT",` +
	`"interval":7,"expires_in":600}`

func TestRequestDeviceAuthorizationReadsTheResponse(t *testing.T) {
	f := newFlowFixture(t, nil)
	f.idp.setDevice(http.StatusOK, deviceOK)
	cfg := f.deviceConfig()
	cfg.Scope = "read"
	cfg.Audience = "api://x"

	da, err := requestDeviceAuthorization(context.Background(), NewTokenHTTPClient(), cfg, testNow, f.opts)
	if err != nil {
		t.Fatalf("requestDeviceAuthorization: %v", err)
	}

	if da.DeviceCode != "dc-1" || da.UserCode != "WDJB-MJHT" {
		t.Errorf("codes: %+v", da)
	}
	if da.VerificationURI != "https://idp.example/device" ||
		da.VerificationURIComplete != "https://idp.example/device?user_code=WDJB-MJHT" {
		t.Errorf("verification URLs: %+v", da)
	}
	if da.Interval != 7*time.Second {
		t.Errorf("Interval = %s", da.Interval)
	}
	if !da.ExpiresAt.Equal(testNow.Add(10 * time.Minute)) {
		t.Errorf("ExpiresAt = %v, want the local deadline", da.ExpiresAt)
	}

	forms := f.idp.deviceForms()
	if len(forms) != 1 {
		t.Fatalf("device endpoint hits: %d", len(forms))
	}
	assertForm(t, forms[0], url.Values{
		"client_id": {"app"}, "scope": {"read"}, "audience": {"api://x"},
	})
}

func TestRequestDeviceAuthorizationDefaultsWhenFieldsAreAbsent(t *testing.T) {
	f := newFlowFixture(t, nil)
	f.idp.setDevice(http.StatusOK, `{"device_code":"dc","user_code":"UC","verification_uri":"http://127.0.0.1:9877/device"}`)

	da, err := requestDeviceAuthorization(context.Background(), NewTokenHTTPClient(), f.deviceConfig(), testNow, f.opts)
	if err != nil {
		t.Fatalf("requestDeviceAuthorization: %v", err)
	}
	if da.Interval != f.opts.DefaultInterval {
		t.Errorf("Interval = %s, want the default %s", da.Interval, f.opts.DefaultInterval)
	}
	if !da.ExpiresAt.Equal(testNow.Add(f.opts.DeviceTimeout)) {
		t.Errorf("ExpiresAt = %v", da.ExpiresAt)
	}
}

func TestRequestDeviceAuthorizationRejectsMalformedResponses(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{"not json", `<html>nope</html>`},
		{"no device_code", `{"user_code":"UC","verification_uri":"https://idp.example/device"}`},
		{"no user_code", `{"device_code":"dc","verification_uri":"https://idp.example/device"}`},
		{"no verification_uri", `{"device_code":"dc","user_code":"UC"}`},
		{"javascript verification_uri", `{"device_code":"dc","user_code":"UC","verification_uri":"javascript:alert(1)"}`},
		{
			"javascript verification_uri_complete",
			`{"device_code":"dc","user_code":"UC","verification_uri":"https://idp.example/device","verification_uri_complete":"javascript:alert(1)"}`,
		},
		{"unusable interval", `{"device_code":"dc","user_code":"UC","verification_uri":"https://idp.example/device","interval":"soon"}`},
		{"interval out of range", `{"device_code":"dc","user_code":"UC","verification_uri":"https://idp.example/device","interval":3600}`},
		{"negative expires_in", `{"device_code":"dc","user_code":"UC","verification_uri":"https://idp.example/device","expires_in":-1}`},
		{"expires_in out of range", `{"device_code":"dc","user_code":"UC","verification_uri":"https://idp.example/device","expires_in":86400}`},
		{"oversized body", `{"device_code":"` + strings.Repeat("d", 1<<20) + `"}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newFlowFixture(t, nil)
			f.idp.setDevice(http.StatusOK, tt.body)
			owner := testOwner()

			if _, err := requestDeviceAuthorization(context.Background(), NewTokenHTTPClient(),
				f.deviceConfig(), testNow, f.opts); err == nil {
				t.Fatal("a malformed device authorization response was accepted")
			}

			id := uuid.NewString()
			if _, err := f.mgr.StartDevice(context.Background(), id, owner, f.deviceConfig()); err == nil {
				t.Fatal("StartDevice accepted a malformed response")
			}
			if _, ok := f.mgr.Status(id); ok {
				t.Error("a failed start installed a flow")
			}
			if _, puts, _ := f.repo.counts(); puts != 0 {
				t.Errorf("puts = %d, want none", puts)
			}
		})
	}
}

func TestDeviceFlowPollsThenStoresTheToken(t *testing.T) {
	f := newFlowFixture(t, nil)
	f.idp.setDevice(http.StatusOK, deviceOK)
	f.idp.script(
		oauthErrorReply(OAuthErrAuthorizationPending),
		oauthErrorReply(OAuthErrAuthorizationPending),
		idpReply{status: http.StatusOK, body: `{"access_token":"at-device","expires_in":3600}`},
	)
	owner := testOwner()

	id, info := f.startDevice(owner)
	if info.UserCode != "WDJB-MJHT" || info.Interval != 7*time.Second {
		t.Fatalf("info = %+v", info)
	}
	if !info.ExpiresAt.Equal(testNow.Add(10 * time.Minute)) {
		t.Errorf("ExpiresAt = %v", info.ExpiresAt)
	}

	f.waitState(id, FlowDone)

	row := f.repo.row(owner)
	if row == nil || row.AccessToken != "at-device" {
		t.Fatalf("stored row: %+v", row)
	}
	forms := f.idp.tokenForms()
	if len(forms) != 3 {
		t.Fatalf("poll count = %d", len(forms))
	}
	assertForm(t, forms[0], url.Values{
		"grant_type":  {deviceCodeGrant},
		"device_code": {"dc-1"},
		"client_id":   {"app"},
	})
	for _, slept := range f.clock.sleeps() {
		if slept != 7*time.Second {
			t.Errorf("slept %s between polls, want the response's interval", slept)
		}
	}
}

func TestDeviceFlowGrowsTheIntervalOnSlowDown(t *testing.T) {
	f := newFlowFixture(t, func(o *FlowOptions) { o.MaxInterval = 12 * time.Second })
	f.idp.setDevice(http.StatusOK, `{"device_code":"dc","user_code":"UC","verification_uri":"https://idp.example/device","expires_in":600}`)
	f.idp.script(
		oauthErrorReply(OAuthErrSlowDown),
		oauthErrorReply(OAuthErrSlowDown),
		idpReply{status: http.StatusOK, body: `{"access_token":"at-slow"}`},
	)

	id, _ := f.startDevice(testOwner())
	f.waitState(id, FlowDone)

	want := []time.Duration{5 * time.Second, 10 * time.Second, 12 * time.Second}
	got := f.clock.sleeps()
	if len(got) != len(want) {
		t.Fatalf("sleeps = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("sleep %d = %s, want %s", i, got[i], want[i])
		}
	}
}

func TestSlowDownNeverShortensAnIntervalAboveTheCap(t *testing.T) {
	f := newFlowFixture(t, nil)
	f.idp.setDevice(http.StatusOK,
		`{"device_code":"dc","user_code":"UC","verification_uri":"https://idp.example/device","interval":120,"expires_in":600}`)
	f.idp.script(
		oauthErrorReply(OAuthErrSlowDown),
		idpReply{status: http.StatusOK, body: `{"access_token":"at-slow"}`},
	)

	id, _ := f.startDevice(testOwner())
	f.waitState(id, FlowDone)

	for i, slept := range f.clock.sleeps() {
		if slept != 120*time.Second {
			t.Errorf("sleep %d = %s, want the IdP's own interval", i, slept)
		}
	}
}

func TestDeviceFlowTerminalErrors(t *testing.T) {
	for _, code := range []string{OAuthErrAccessDenied, OAuthErrExpiredToken, OAuthErrInvalidGrant} {
		t.Run(code, func(t *testing.T) {
			f := newFlowFixture(t, nil)
			f.idp.setDevice(http.StatusOK, deviceOK)
			f.idp.script(oauthErrorReply(code))

			id, _ := f.startDevice(testOwner())
			status := f.waitState(id, FlowError)
			if !strings.Contains(status.Err.Error(), code) {
				t.Errorf("error = %v", status.Err)
			}
			if _, puts, _ := f.repo.counts(); puts != 0 {
				t.Errorf("puts = %d, want none", puts)
			}
		})
	}
}

func TestDeviceFlowEndsAtItsLocalDeadline(t *testing.T) {
	f := newFlowFixture(t, nil)
	f.idp.setDevice(http.StatusOK,
		`{"device_code":"dc","user_code":"UC","verification_uri":"https://idp.example/device","interval":5,"expires_in":12}`)
	f.idp.script(oauthErrorReply(OAuthErrAuthorizationPending))

	id, _ := f.startDevice(testOwner())

	status := f.waitState(id, FlowError)
	if !strings.Contains(status.Err.Error(), "expired") {
		t.Errorf("error = %v", status.Err)
	}
	if hits := len(f.idp.tokenForms()); hits != 2 {
		t.Errorf("polls before the deadline: %d, want 2", hits)
	}
	want := []time.Duration{5 * time.Second, 5 * time.Second, 2 * time.Second}
	if got := f.clock.sleeps(); !slices.Equal(got, want) {
		t.Errorf("sleeps = %v, want %v", got, want)
	}
}

func TestDeviceFlowNeverSleepsPastItsDeadline(t *testing.T) {
	f := newFlowFixture(t, nil)
	f.idp.setDevice(http.StatusOK,
		`{"device_code":"dc","user_code":"UC","verification_uri":"https://idp.example/device","interval":300,"expires_in":1}`)
	f.idp.script(oauthErrorReply(OAuthErrAuthorizationPending))

	id, _ := f.startDevice(testOwner())

	status := f.waitState(id, FlowError)
	if !strings.Contains(status.Err.Error(), "expired") {
		t.Errorf("error = %v", status.Err)
	}
	want := []time.Duration{time.Second}
	if got := f.clock.sleeps(); !slices.Equal(got, want) {
		t.Errorf("sleeps = %v, want %v", got, want)
	}
	if hits := len(f.idp.tokenForms()); hits != 0 {
		t.Errorf("polls after the deadline: %d, want none", hits)
	}
}

func TestDeviceFlowIsCancelledPromptly(t *testing.T) {
	f := newFlowFixture(t, nil)
	f.idp.setDevice(http.StatusOK, deviceOK)
	f.idp.script(oauthErrorReply(OAuthErrAuthorizationPending))
	owner := testOwner()

	id, _ := f.startDevice(owner)
	if err := f.mgr.Cancel(id); err != nil {
		t.Fatalf("Cancel: %v", err)
	}
	if status, _ := f.mgr.Status(id); status.State != FlowCancelled {
		t.Errorf("state = %s", status.State)
	}
	if _, puts, _ := f.repo.counts(); puts != 0 {
		t.Errorf("puts = %d, want none", puts)
	}
}

func TestStartDeviceRejectsBadPreconditions(t *testing.T) {
	tests := []struct {
		name  string
		id    string
		cfg   func(f *flowFixture) OAuth2Config
		field string
	}{
		{
			name: "not a uuid", id: "flow-2",
			cfg: func(f *flowFixture) OAuth2Config { return f.deviceConfig() }, field: "flowId",
		},
		{
			name: "grant mismatch", id: uuid.NewString(),
			cfg: func(f *flowFixture) OAuth2Config { return f.codeConfig() }, field: "grant",
		},
		{
			name: "missing deviceAuthUrl", id: uuid.NewString(),
			cfg: func(f *flowFixture) OAuth2Config {
				cfg := f.deviceConfig()
				cfg.DeviceAuthURL = ""

				return cfg
			},
			field: "deviceAuthUrl",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newFlowFixture(t, nil)

			_, err := f.mgr.StartDevice(context.Background(), tt.id, testOwner(), tt.cfg(f))
			if err == nil {
				t.Fatal("StartDevice accepted an invalid start")
			}
			fieldError(t, err, tt.field)

			if reserves, _, _ := f.repo.counts(); reserves != 0 {
				t.Errorf("the store was reserved before validation: %d", reserves)
			}
			if hits := len(f.idp.deviceForms()); hits != 0 {
				t.Errorf("the device endpoint was called %d times", hits)
			}
		})
	}
}
