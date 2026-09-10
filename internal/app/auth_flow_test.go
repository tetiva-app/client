package app

import (
	"context"
	"errors"
	"testing"

	"go.uber.org/fx"

	"github.com/tetiva-app/client/internal/domain/entities"
	"github.com/tetiva-app/client/internal/domain/usecase/auth"
)

type flowManagerSpy struct {
	shutdowns int
	err       error
}

func (m *flowManagerSpy) StartAuthCode(context.Context, string, entities.AuthOwner,
	auth.OAuth2Config, string,
) (auth.FlowInfo, error) {
	panic("not used")
}

func (m *flowManagerSpy) StartDevice(context.Context, string, entities.AuthOwner,
	auth.OAuth2Config,
) (auth.FlowInfo, error) {
	panic("not used")
}

func (m *flowManagerSpy) Status(string) (auth.FlowStatus, bool) { panic("not used") }

func (m *flowManagerSpy) Cancel(string) error { panic("not used") }

func (m *flowManagerSpy) Shutdown(context.Context) error {
	m.shutdowns++
	return m.err
}

type signInManagerSpy struct {
	shutdowns int
	err       error
}

func (m *signInManagerSpy) Start(context.Context, string, auth.SignInClient, auth.SignInParams,
	auth.SignInHooks,
) (auth.SignInInfo, error) {
	panic("not used")
}

func (m *signInManagerSpy) Status(string) (auth.SignInStatus, bool) { panic("not used") }

func (m *signInManagerSpy) Cancel(string) error { panic("not used") }

func (m *signInManagerSpy) Shutdown(context.Context) error {
	m.shutdowns++
	return m.err
}

func shutdownApp(flows *flowManagerSpy, signIn *signInManagerSpy) *fx.App {
	return fx.New(
		fx.Provide(func() auth.FlowManager { return flows }),
		fx.Provide(func() auth.SignInManager { return signIn }),
		fx.Invoke(RegisterFlowShutdownHook),
		fx.NopLogger,
	)
}

func TestFlowShutdownHookStopsBothManagers(t *testing.T) {
	flows, signIn := &flowManagerSpy{}, &signInManagerSpy{}
	app := shutdownApp(flows, signIn)

	if err := app.Start(context.Background()); err != nil {
		t.Fatalf("start: %v", err)
	}
	if err := app.Stop(context.Background()); err != nil {
		t.Fatalf("stop: %v", err)
	}
	if flows.shutdowns != 1 || signIn.shutdowns != 1 {
		t.Fatalf("Shutdown calls = %d/%d, want 1/1", flows.shutdowns, signIn.shutdowns)
	}
}

func TestFlowShutdownHookReportsSurvivors(t *testing.T) {
	flowErr := errors.New("auth: 1 flow(s) did not stop")
	signInErr := errors.New("auth: 1 sign-in(s) did not stop")

	tests := map[string]struct {
		flows  error
		signIn error
	}{
		"the oauth manager":   {flows: flowErr},
		"the sign-in manager": {signIn: signInErr},
		"both":                {flows: flowErr, signIn: signInErr},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			flows := &flowManagerSpy{err: tc.flows}
			signIn := &signInManagerSpy{err: tc.signIn}
			app := shutdownApp(flows, signIn)

			if err := app.Start(context.Background()); err != nil {
				t.Fatalf("start: %v", err)
			}
			err := app.Stop(context.Background())
			if err == nil {
				t.Fatal("stop: want the survivor error, got nil")
			}
			if tc.flows != nil && !errors.Is(err, tc.flows) {
				t.Fatalf("stop: %v does not carry the oauth survivor", err)
			}
			if tc.signIn != nil && !errors.Is(err, tc.signIn) {
				t.Fatalf("stop: %v does not carry the sign-in survivor", err)
			}
			// A failing manager must not stop the other one from being asked.
			if flows.shutdowns != 1 || signIn.shutdowns != 1 {
				t.Fatalf("Shutdown calls = %d/%d, want 1/1", flows.shutdowns, signIn.shutdowns)
			}
		})
	}
}
