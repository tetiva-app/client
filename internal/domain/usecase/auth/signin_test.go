package auth

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/tetiva-app/client/internal/domain"
)

const signInDiscoveryURL = "https://app.example/desktop-signin"

// The two halves of the worst commit the plan budgets for: a config write bound
// by its own context, then a keychain write that does not watch one.
const (
	syncConfigWorstCase = 1500 * time.Millisecond
	keychainWorstCase   = 2 * time.Second
)

type pollStep struct {
	res SignInPoll
	err error
}

func stepPending() pollStep { return pollStep{res: SignInPoll{Status: SignInPending}} }

func stepApproved(tokens SignInTokens) pollStep {
	return pollStep{res: SignInPoll{Status: SignInApproved, Tokens: &tokens}}
}

// fakeSignInClient scripts the six calls the manager makes and counts the ones
// the ownership rules are about: Close, Logout and CancelDesktopSignIn.
type fakeSignInClient struct {
	// Set before Start and never written again.
	info      ServerInfo
	infoErr   error
	infoGate  chan struct{}
	start     SignInStart
	startErr  error
	startGate chan struct{}
	// pollBlock ignores its context on purpose: it is the uncooperative client.
	pollBlock chan struct{}
	afterPoll func(n int)

	mu    sync.Mutex
	steps []pollStep

	startCalls  int
	startParams SignInParams
	redirectURI string
	challenge   string
	claim       string

	pollCalls     int
	pollTimes     []time.Time
	verifier      string
	pollClientID  string
	pollRequestID string

	cancelCalls int
	cancelErr   error
	logoutCalls int
	loggedOut   []string
	logoutErr   error
	closeCalls  int
}

func newFakeSignInClient(steps ...pollStep) *fakeSignInClient {
	return &fakeSignInClient{
		info: ServerInfo{DesktopSignIn: true, DesktopSignInURL: signInDiscoveryURL},
		start: SignInStart{
			RequestID:    "dsr_1",
			LoginURL:     signInDiscoveryURL + "?request=dsr_1",
			ExpiresAt:    testNow.Add(time.Minute),
			PollInterval: 20 * time.Millisecond,
		},
		steps: steps,
	}
}

func signInGate(ctx context.Context, gate chan struct{}) error {
	if gate == nil {
		return nil
	}

	select {
	case <-gate:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (c *fakeSignInClient) GetServerInfo(ctx context.Context) (ServerInfo, error) {
	if err := signInGate(ctx, c.infoGate); err != nil {
		return ServerInfo{}, err
	}

	return c.info, c.infoErr
}

func (c *fakeSignInClient) StartDesktopSignIn(ctx context.Context, p SignInParams,
	redirectURI, codeChallenge, claimChallenge string,
) (SignInStart, error) {
	c.mu.Lock()
	c.startCalls++
	c.startParams, c.redirectURI, c.challenge, c.claim = p, redirectURI, codeChallenge, claimChallenge
	c.mu.Unlock()

	if err := signInGate(ctx, c.startGate); err != nil {
		return SignInStart{}, err
	}

	return c.start, c.startErr
}

func (c *fakeSignInClient) PollDesktopSignIn(_ context.Context, requestID, clientID, codeVerifier string) (SignInPoll, error) {
	if c.pollBlock != nil {
		<-c.pollBlock
	}

	c.mu.Lock()
	c.pollCalls++
	n := c.pollCalls
	c.pollTimes = append(c.pollTimes, time.Now())
	c.verifier, c.pollClientID, c.pollRequestID = codeVerifier, clientID, requestID
	step := stepPending()
	if len(c.steps) > 0 {
		step = c.steps[min(n, len(c.steps))-1]
	}
	c.mu.Unlock()

	if c.afterPoll != nil {
		c.afterPoll(n)
	}

	return step.res, step.err
}

func (c *fakeSignInClient) CancelDesktopSignIn(_ context.Context, _, _, _ string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.cancelCalls++

	return c.cancelErr
}

func (c *fakeSignInClient) Logout(_ context.Context, accessToken string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.logoutCalls++
	c.loggedOut = append(c.loggedOut, accessToken)

	return c.logoutErr
}

func (c *fakeSignInClient) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.closeCalls++

	return nil
}

func (c *fakeSignInClient) counts() (polls, closes, cancels, logouts int) {
	c.mu.Lock()
	defer c.mu.Unlock()

	return c.pollCalls, c.closeCalls, c.cancelCalls, c.logoutCalls
}

func (c *fakeSignInClient) polls() int   { p, _, _, _ := c.counts(); return p }
func (c *fakeSignInClient) closes() int  { _, cl, _, _ := c.counts(); return cl }
func (c *fakeSignInClient) cancels() int { _, _, ca, _ := c.counts(); return ca }
func (c *fakeSignInClient) logouts() int { _, _, _, l := c.counts(); return l }

func (c *fakeSignInClient) times() []time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()

	return append([]time.Time(nil), c.pollTimes...)
}

func (c *fakeSignInClient) startArgs() (redirectURI, challenge, claim string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	return c.redirectURI, c.challenge, c.claim
}

func (c *fakeSignInClient) seenVerifier() string {
	c.mu.Lock()
	defer c.mu.Unlock()

	return c.verifier
}

// hookProbe records what the four hooks saw, so a test can tell a commit from
// what runs after it.
type hookProbe struct {
	mu sync.Mutex

	approvals   int
	tokens      SignInTokens
	deadline    time.Time
	hasDeadline bool
	inCommit    bool
	commitFor   time.Duration
	commitErr   error
	commitGate  chan struct{}

	dones        int
	outcome      SignInOutcome
	doneInCommit bool
	extraDone    func(SignInOutcome)

	callbacks int
	terminals []FlowState
}

func (p *hookProbe) hooks() SignInHooks {
	return SignInHooks{
		OnApproved: func(ctx context.Context, _ SignInClient, _ SignInParams, tokens SignInTokens) error {
			p.mu.Lock()
			p.approvals++
			p.tokens = tokens
			p.deadline, p.hasDeadline = ctx.Deadline()
			p.inCommit = true
			gate, hold, err := p.commitGate, p.commitFor, p.commitErr
			p.mu.Unlock()

			if gate != nil {
				<-gate
			}
			if hold > 0 {
				time.Sleep(hold)
			}

			p.mu.Lock()
			p.inCommit = false
			p.mu.Unlock()

			return err
		},
		OnDone: func(outcome SignInOutcome) {
			p.mu.Lock()
			p.dones++
			p.outcome = outcome
			p.doneInCommit = p.inCommit
			extra := p.extraDone
			p.mu.Unlock()

			if extra != nil {
				extra(outcome)
			}
		},
		OnCallback: func() {
			p.mu.Lock()
			defer p.mu.Unlock()

			p.callbacks++
		},
		OnTerminal: func(state FlowState) {
			p.mu.Lock()
			defer p.mu.Unlock()

			p.terminals = append(p.terminals, state)
		},
	}
}

func (p *hookProbe) snapshot() hookProbe {
	p.mu.Lock()
	defer p.mu.Unlock()

	return hookProbe{
		approvals:    p.approvals,
		tokens:       p.tokens,
		deadline:     p.deadline,
		hasDeadline:  p.hasDeadline,
		dones:        p.dones,
		outcome:      p.outcome,
		doneInCommit: p.doneInCommit,
		callbacks:    p.callbacks,
		terminals:    append([]FlowState(nil), p.terminals...),
	}
}

type signInEvent struct {
	id string
	st SignInStatus
}

type signInRecorder struct {
	mu     sync.Mutex
	events []signInEvent
}

func (r *signInRecorder) OnSignInState(flowID string, st SignInStatus) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.events = append(r.events, signInEvent{id: flowID, st: st})
}

func (r *signInRecorder) of(flowID string) []signInEvent {
	r.mu.Lock()
	defer r.mu.Unlock()

	var out []signInEvent
	for _, e := range r.events {
		if e.id == flowID {
			out = append(out, e)
		}
	}

	return out
}

func signInOptions(clock *fakeClock) FlowOptions {
	return FlowOptions{
		Now:         clock.now,
		Rand:        &seqReader{},
		MinInterval: time.Millisecond,
		MaxInterval: 200 * time.Millisecond,
		JoinCap:     2 * time.Second,
	}
}

func waitForSignIn(t *testing.T, what string, cond func() bool) {
	t.Helper()

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatalf("timed out waiting for %s", what)
}

func waitSignInState(t *testing.T, m SignInManager, flowID string, want FlowState) SignInStatus {
	t.Helper()

	var got SignInStatus
	waitForSignIn(t, fmt.Sprintf("flow to reach %s", want), func() bool {
		st, ok := m.Status(flowID)
		if !ok || st.State != want {
			return false
		}
		got = st

		return true
	})

	return got
}

// signInFixture is the manager as the tests build it: an injected clock for the
// deadlines, real timers for the poll interval.
type signInFixture struct {
	m      SignInManager
	sink   *signInRecorder
	clock  *fakeClock
	client *fakeSignInClient
	probe  *hookProbe
	id     string
}

func newSignInFixture(t *testing.T, client *fakeSignInClient, opts FlowOptions) *signInFixture {
	t.Helper()

	sink := &signInRecorder{}
	f := &signInFixture{
		m:      NewSignInManager(sink, opts),
		sink:   sink,
		client: client,
		probe:  &hookProbe{},
		id:     uuid.NewString(),
	}
	t.Cleanup(func() { _ = f.m.Shutdown(context.Background()) })

	return f
}

func newSignInClockFixture(t *testing.T, client *fakeSignInClient) *signInFixture {
	t.Helper()

	clock := newFakeClock()
	f := newSignInFixture(t, client, signInOptions(clock))
	f.clock = clock

	return f
}

func (f *signInFixture) start(t *testing.T) (SignInInfo, error) {
	t.Helper()

	return f.m.Start(context.Background(), f.id, f.client,
		SignInParams{ServerURL: "sync.example:443", ClientID: "cid", Intent: "signin"}, f.probe.hooks())
}

func (f *signInFixture) mustStart(t *testing.T) SignInInfo {
	t.Helper()

	info, err := f.start(t)
	if err != nil {
		t.Fatalf("Start: %v", err)
	}

	return info
}

func TestSignInPollsUntilTheServerApproves(t *testing.T) {
	tokens := SignInTokens{AccessToken: "at", RefreshToken: "rt", ActiveOrgID: "org", Email: "a@example.com"}
	client := newFakeSignInClient(stepPending(), stepPending(), stepApproved(tokens))
	f := newSignInClockFixture(t, client)

	info := f.mustStart(t)
	if info.Host != "app.example" {
		t.Errorf("Info.Host = %q, want app.example", info.Host)
	}

	st := waitSignInState(t, f.m, f.id, FlowDone)
	if st.Outcome == nil || st.Outcome.Email != "a@example.com" {
		t.Fatalf("Status().Outcome = %+v, want the email from the poll", st.Outcome)
	}

	probe := f.probe.snapshot()
	if probe.approvals != 1 || probe.tokens.RefreshToken != "rt" {
		t.Errorf("OnApproved ran %d time(s) with %+v", probe.approvals, probe.tokens)
	}
	if probe.dones != 1 {
		t.Errorf("OnDone ran %d time(s), want 1", probe.dones)
	}
	if len(probe.terminals) != 0 {
		t.Errorf("OnTerminal ran on done: %v", probe.terminals)
	}
	if closes := client.closes(); closes != 0 {
		t.Errorf("client closed %d time(s) on done, want 0 — it belongs to the hook now", closes)
	}
}

func TestSignInLoginURLCarriesTheClaimAndItsHashTravels(t *testing.T) {
	client := newFakeSignInClient(stepPending())
	f := newSignInClockFixture(t, client)

	info := f.mustStart(t)
	base, claim, ok := strings.Cut(info.LoginURL, "#claim=")
	if !ok || base != client.start.LoginURL {
		t.Fatalf("LoginURL = %q, want the server's URL plus a claim fragment", info.LoginURL)
	}

	waitForSignIn(t, "the first poll", func() bool { return client.polls() >= 1 })

	_, challenge, claimChallenge := client.startArgs()
	if sum := sha256.Sum256([]byte(claim)); base64.RawURLEncoding.EncodeToString(sum[:]) != claimChallenge {
		t.Errorf("claim_challenge %q is not the SHA-256 of the fragment", claimChallenge)
	}
	if len(claimChallenge) != 43 {
		t.Errorf("claim_challenge is %d characters, want 43", len(claimChallenge))
	}
	if sum := sha256.Sum256([]byte(client.seenVerifier())); base64.RawURLEncoding.EncodeToString(sum[:]) != challenge {
		t.Errorf("code_challenge %q is not the SHA-256 of the verifier", challenge)
	}
}

func TestSignInRedirectCutsTheIntervalShortExactlyOnce(t *testing.T) {
	client := newFakeSignInClient(stepPending(), stepPending(), stepApproved(SignInTokens{AccessToken: "at"}))
	client.start.PollInterval = 300 * time.Millisecond
	clock := newFakeClock()
	opts := signInOptions(clock)
	// Above the server's interval: the cap bounds it too, and this test is about
	// the redirect.
	opts.MaxInterval = 400 * time.Millisecond
	f := newSignInFixture(t, client, opts)
	f.clock = clock

	f.mustStart(t)
	waitForSignIn(t, "the first poll", func() bool { return client.polls() >= 1 })

	redirectURI, _, _ := client.startArgs()
	resp := callbackGet(t, redirectURI)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("redirect: status = %d", resp.StatusCode)
	}

	waitForSignIn(t, "the poll the redirect woke", func() bool { return client.polls() >= 2 })
	times := client.times()
	if gap := times[1].Sub(times[0]); gap > 200*time.Millisecond {
		t.Errorf("the poll after the redirect waited %s, the redirect should have cut the interval short", gap)
	}

	waitSignInState(t, f.m, f.id, FlowDone)
	times = client.times()
	if gap := times[2].Sub(times[1]); gap < 200*time.Millisecond {
		t.Errorf("the next poll came after %s — a consumed redirect keeps waking the loop", gap)
	}
	if probe := f.probe.snapshot(); probe.callbacks != 1 {
		t.Errorf("OnCallback ran %d time(s), want 1", probe.callbacks)
	}
}

func TestSignInThrottledGrowsTheIntervalUpToTheCap(t *testing.T) {
	throttled := pollStep{err: fmt.Errorf("poll: %w", ErrSignInThrottled)}
	client := newFakeSignInClient(throttled, throttled, throttled, throttled, throttled,
		stepApproved(SignInTokens{AccessToken: "at"}))
	client.start.PollInterval = 20 * time.Millisecond
	clock := newFakeClock()
	opts := signInOptions(clock)
	opts.MaxInterval = 60 * time.Millisecond
	f := newSignInFixture(t, client, opts)
	f.clock = clock

	f.mustStart(t)
	waitSignInState(t, f.m, f.id, FlowDone)

	times := client.times()
	if len(times) < 6 {
		t.Fatalf("polls = %d, want 6", len(times))
	}
	if grown := times[2].Sub(times[1]); grown < 30*time.Millisecond {
		t.Errorf("the interval after two throttles was %s, it should have doubled past 20ms", grown)
	}
	// Uncapped, the fifth wait would be 320 ms; the cap keeps it at 60 ms.
	if capped := times[5].Sub(times[4]); capped > 200*time.Millisecond {
		t.Errorf("the interval after five throttles was %s, MaxInterval should have capped it", capped)
	}
}

func TestSignInClampsAnOutsizedServerInterval(t *testing.T) {
	client := newFakeSignInClient(stepPending(), stepApproved(SignInTokens{AccessToken: "at"}))
	client.start.PollInterval = 24 * time.Hour
	f := newSignInClockFixture(t, client)

	f.mustStart(t)
	waitSignInState(t, f.m, f.id, FlowDone)

	times := client.times()
	if len(times) < 2 {
		t.Fatalf("polls = %d, want 2", len(times))
	}
	if gap := times[1].Sub(times[0]); gap > time.Second {
		t.Errorf("the second poll waited %s, MaxInterval should have capped the server's interval", gap)
	}
}

func TestSignInSurvivesTransientPollFailures(t *testing.T) {
	transient := pollStep{err: fmt.Errorf("poll: %w", ErrSignInTransient)}
	client := newFakeSignInClient(transient, transient, transient, stepApproved(SignInTokens{AccessToken: "at"}))
	f := newSignInClockFixture(t, client)

	f.mustStart(t)
	waitSignInState(t, f.m, f.id, FlowDone)

	if polls := client.polls(); polls != 4 {
		t.Errorf("polls = %d, want 4", polls)
	}
}

func TestSignInReportsEmailVerificationPendingOnce(t *testing.T) {
	verifying := pollStep{res: SignInPoll{Status: SignInPending, EmailVerificationPending: true}}
	client := newFakeSignInClient(verifying, verifying, stepApproved(SignInTokens{
		AccessToken: "at", Email: "a@example.com", RequiresEmailVerification: true,
	}))
	f := newSignInClockFixture(t, client)

	f.mustStart(t)
	waitForSignIn(t, "the verification hint", func() bool {
		st, ok := f.m.Status(f.id)

		return ok && st.EmailVerificationPending
	})

	st := waitSignInState(t, f.m, f.id, FlowDone)
	if st.Outcome == nil || !st.Outcome.RequiresEmailVerification {
		t.Fatalf("Status().Outcome = %+v, want the verification flag", st.Outcome)
	}

	events := f.sink.of(f.id)
	if len(events) != 2 {
		t.Fatalf("sink got %d event(s), want one for the flag and one for done: %+v", len(events), events)
	}
	if events[0].st.State != FlowPending || !events[0].st.EmailVerificationPending {
		t.Errorf("first event = %+v, want pending with the flag", events[0].st)
	}
	if events[1].st.State != FlowDone {
		t.Errorf("second event = %+v, want done", events[1].st)
	}
}

func TestSignInFollowsAnExtendedServerDeadline(t *testing.T) {
	extended := testNow.Add(31 * time.Minute)
	client := newFakeSignInClient(
		pollStep{res: SignInPoll{Status: SignInPending, ExpiresAt: extended}},
		stepPending(),
		pollStep{res: SignInPoll{Status: SignInPending, ExpiresAt: testNow.Add(-time.Minute)}},
		stepApproved(SignInTokens{AccessToken: "at"}),
	)
	f := newSignInClockFixture(t, client)
	// Past the request's original five-minute life, so only the extension keeps it.
	client.afterPoll = func(n int) {
		if n == 1 {
			f.clock.advance(2 * time.Minute)
		}
	}

	f.mustStart(t)
	st := waitSignInState(t, f.m, f.id, FlowDone)
	if !st.Info.ExpiresAt.Equal(extended) {
		t.Errorf("Info.ExpiresAt = %s, want the extended deadline %s", st.Info.ExpiresAt, extended)
	}

	events := f.sink.of(f.id)
	if len(events) == 0 || !events[0].st.Info.ExpiresAt.Equal(extended) {
		t.Fatalf("sink never saw the new deadline: %+v", events)
	}
}

func TestSignInEndsWhenItsDeadlinePasses(t *testing.T) {
	client := newFakeSignInClient(stepPending())
	f := newSignInClockFixture(t, client)
	client.afterPoll = func(n int) {
		if n == 1 {
			f.clock.advance(2 * time.Minute)
		}
	}

	f.mustStart(t)
	st := waitSignInState(t, f.m, f.id, FlowError)
	if !errors.Is(st.Err, ErrSignInExpired) {
		t.Errorf("Status().Err = %v, want ErrSignInExpired", st.Err)
	}
	waitForSignIn(t, "the client to be closed", func() bool { return client.closes() == 1 })
}

func TestSignInTerminalServerAnswers(t *testing.T) {
	tests := []struct {
		name string
		step pollStep
		want error
	}{
		{"denied", pollStep{res: SignInPoll{Status: SignInDenied}}, ErrSignInDenied},
		{"expired", pollStep{res: SignInPoll{Status: SignInExpired}}, ErrSignInExpired},
		{"approved without tokens", pollStep{res: SignInPoll{Status: SignInApproved}}, ErrSignInMalformed},
		{"unknown status", pollStep{res: SignInPoll{Status: "quantum"}}, ErrSignInMalformed},
		{"refused", pollStep{err: errors.New("unauthenticated")}, nil},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			client := newFakeSignInClient(tc.step)
			f := newSignInClockFixture(t, client)

			f.mustStart(t)
			st := waitSignInState(t, f.m, f.id, FlowError)
			if tc.want != nil && !errors.Is(st.Err, tc.want) {
				t.Errorf("Status().Err = %v, want %v", st.Err, tc.want)
			}
			if st.Err == nil {
				t.Error("Status().Err is nil, the panel would have nothing to show")
			}
			waitForSignIn(t, "the client to be closed", func() bool { return client.closes() == 1 })
			if probe := f.probe.snapshot(); probe.approvals != 0 {
				t.Errorf("OnApproved ran %d time(s) on a failed flow", probe.approvals)
			}
			if probe := f.probe.snapshot(); len(probe.terminals) != 1 || probe.terminals[0] != FlowError {
				t.Errorf("OnTerminal saw %v, want one error", probe.terminals)
			}
		})
	}
}

func TestSignInRevokesTheSessionWhenTheCommitFails(t *testing.T) {
	client := newFakeSignInClient(stepApproved(SignInTokens{AccessToken: "at"}))
	f := newSignInClockFixture(t, client)
	f.probe.commitErr = errors.New("sqlite is busy")

	f.mustStart(t)
	st := waitSignInState(t, f.m, f.id, FlowError)
	if st.Err == nil || !strings.Contains(st.Err.Error(), "sqlite is busy") {
		t.Errorf("Status().Err = %v, want the commit error", st.Err)
	}
	if logouts := client.logouts(); logouts != 1 {
		t.Errorf("Logout ran %d time(s), want 1", logouts)
	}
	waitForSignIn(t, "the client to be closed", func() bool { return client.closes() == 1 })
}

func TestSignInCommitContextIgnoresThePollDeadline(t *testing.T) {
	client := newFakeSignInClient(stepApproved(SignInTokens{AccessToken: "at"}))
	client.start.ExpiresAt = testNow.Add(time.Hour)
	f := newSignInClockFixture(t, client)

	f.mustStart(t)
	waitSignInState(t, f.m, f.id, FlowDone)

	probe := f.probe.snapshot()
	if !probe.hasDeadline {
		t.Fatal("OnApproved got a context without a deadline")
	}
	if budget := time.Until(probe.deadline); budget <= 0 || budget > signInCommitTimeout {
		t.Errorf("commit budget = %s, want (0, %s]", budget, signInCommitTimeout)
	}
}

func TestSignInOnDoneRunsOutsideTheCommitAndTheLock(t *testing.T) {
	client := newFakeSignInClient(stepApproved(SignInTokens{AccessToken: "at"}))
	f := newSignInClockFixture(t, client)

	type reentry struct {
		state   FlowState
		elapsed time.Duration
	}
	observed := make(chan reentry, 1)
	f.probe.extraDone = func(SignInOutcome) {
		started := time.Now()
		st, _ := f.m.Status(f.id)
		_ = f.m.Cancel(uuid.NewString())
		observed <- reentry{state: st.State, elapsed: time.Since(started)}
	}

	f.mustStart(t)
	waitSignInState(t, f.m, f.id, FlowDone)

	var got reentry
	select {
	case got = <-observed:
	case <-time.After(5 * time.Second):
		t.Fatal("OnDone never returned — the manager held its lock")
	}

	if probe := f.probe.snapshot(); probe.doneInCommit {
		t.Error("OnDone ran while the commit was still in flight")
	}
	if got.state != FlowDone {
		t.Errorf("Status() inside OnDone = %s, want done", got.state)
	}
	if got.elapsed > time.Second {
		t.Errorf("Status and Cancel inside OnDone took %s — the manager's lock was held", got.elapsed)
	}
}

func TestSignInCancelBeforeTheCommitRevokesTheSession(t *testing.T) {
	client := newFakeSignInClient(stepApproved(SignInTokens{AccessToken: "at"}))
	f := newSignInClockFixture(t, client)
	client.afterPoll = func(int) {
		go func() { _ = f.m.Cancel(f.id) }()
		waitForSignIn(t, "the cancel to land", func() bool {
			st, ok := f.m.Status(f.id)

			return ok && st.State == FlowCancelled
		})
	}

	f.mustStart(t)
	waitForSignIn(t, "the revoked session", func() bool { return client.logouts() == 1 })

	st, _ := f.m.Status(f.id)
	if st.State != FlowCancelled {
		t.Errorf("state = %s, want cancelled", st.State)
	}
	if probe := f.probe.snapshot(); probe.approvals != 0 {
		t.Errorf("OnApproved ran %d time(s) after the cancel won", probe.approvals)
	}
}

func TestSignInCancelDuringTheCommitWaitsForIt(t *testing.T) {
	client := newFakeSignInClient(stepApproved(SignInTokens{AccessToken: "at"}))
	f := newSignInClockFixture(t, client)
	gate := make(chan struct{})
	f.probe.commitGate = gate

	f.mustStart(t)
	waitForSignIn(t, "the commit to start", func() bool { return f.probe.snapshot().approvals == 1 })

	go func() {
		time.Sleep(50 * time.Millisecond)
		close(gate)
	}()
	if err := f.m.Cancel(f.id); err != nil {
		t.Fatalf("Cancel during commit: %v", err)
	}
	if st, _ := f.m.Status(f.id); st.State != FlowDone {
		t.Errorf("state = %s, want done — the commit had already won", st.State)
	}
}

func TestSignInCancelAndStatusWorkWhileStarting(t *testing.T) {
	client := newFakeSignInClient(stepPending())
	client.infoGate = make(chan struct{})
	f := newSignInClockFixture(t, client)

	errCh := make(chan error, 1)
	go func() {
		_, err := f.start(t)
		errCh <- err
	}()

	waitForSignIn(t, "the reserved flow", func() bool {
		st, ok := f.m.Status(f.id)

		return ok && st.State == FlowStarting
	})
	if err := f.m.Cancel(f.id); err != nil {
		t.Fatalf("Cancel while starting: %v", err)
	}
	if err := <-errCh; err == nil {
		t.Fatal("Start returned nil after its flow was cancelled")
	}

	st, ok := f.m.Status(f.id)
	if !ok || st.State != FlowCancelled {
		t.Errorf("state = %+v, want the cancel to survive the setup failure", st)
	}
	if closes := client.closes(); closes != 1 {
		t.Errorf("client closed %d time(s), want 1", closes)
	}
}

func TestSignInProductionJoinCapOutlastsTheWorstCommit(t *testing.T) {
	client := newFakeSignInClient(stepApproved(SignInTokens{AccessToken: "at"}))
	client.start.ExpiresAt = time.Now().Add(time.Minute)
	sink := &signInRecorder{}
	probe := &hookProbe{}
	// The worst commit the plan allows: a 1.5 s config write, then a 2 s keychain
	// write that does not watch the context.
	probe.commitFor = syncConfigWorstCase + keychainWorstCase
	m := NewSignInManager(sink, FlowOptions{JoinCap: 10 * time.Second})
	t.Cleanup(func() { _ = m.Shutdown(context.Background()) })

	flowID := uuid.NewString()
	if _, err := m.Start(context.Background(), flowID, client, SignInParams{ClientID: "cid"}, probe.hooks()); err != nil {
		t.Fatalf("Start: %v", err)
	}
	waitForSignIn(t, "the commit to start", func() bool { return probe.snapshot().approvals == 1 })

	started := time.Now()
	if err := m.Cancel(flowID); err != nil {
		t.Fatalf("Cancel during a %s commit: %v", probe.commitFor, err)
	}
	waited := time.Since(started)
	if waited > 5*time.Second {
		t.Errorf("Cancel waited %s, the commit must fit in 5 s", waited)
	}
	if st, _ := m.Status(flowID); st.State != FlowDone {
		t.Errorf("state = %s, want done", st.State)
	}
}

func TestSignInStartsWithoutALoopbackWhenTheBindFails(t *testing.T) {
	client := newFakeSignInClient(stepPending())
	clock := newFakeClock()
	opts := signInOptions(clock)
	opts.Listen = func(string, string) (net.Listener, error) { return nil, errors.New("boom") }
	f := newSignInFixture(t, client, opts)
	f.clock = clock

	f.mustStart(t)
	waitForSignIn(t, "the first poll", func() bool { return client.polls() >= 1 })

	if redirectURI, _, _ := client.startArgs(); redirectURI != "" {
		t.Errorf("redirect_uri = %q, want empty when the listener could not bind", redirectURI)
	}
	if st, _ := f.m.Status(f.id); st.State != FlowPending {
		t.Errorf("state = %s, want pending — a failed bind is not fatal", st.State)
	}
}

func TestSignInRejectsALoginURLWithAFragment(t *testing.T) {
	client := newFakeSignInClient(stepPending())
	client.start.LoginURL = signInDiscoveryURL + "?request=dsr_1#claim=theirs"
	f := newSignInClockFixture(t, client)

	if _, err := f.start(t); err == nil {
		t.Fatal("Start accepted a login_url carrying a fragment")
	}
	st, ok := f.m.Status(f.id)
	if !ok || st.State != FlowError || st.Err == nil {
		t.Errorf("Status() = %+v, %v, want a retained error", st, ok)
	}
	if cancels := client.cancels(); cancels != 1 {
		t.Errorf("CancelDesktopSignIn ran %d time(s), the server request must not be left pending", cancels)
	}
	if closes := client.closes(); closes != 1 {
		t.Errorf("client closed %d time(s), want 1", closes)
	}
}

func TestSignInFailedInstallReleasesEverythingSetupCreated(t *testing.T) {
	loser := newFakeSignInClient(stepPending())
	loser.startGate = make(chan struct{})
	f := newSignInClockFixture(t, loser)

	errCh := make(chan error, 1)
	go func() {
		_, err := f.start(t)
		errCh <- err
	}()
	waitForSignIn(t, "the losing start to reach the server", func() bool { return loser.polls() == 0 && loserStarted(loser) })

	winner := newFakeSignInClient(stepPending())
	winnerProbe := &hookProbe{}
	if _, err := f.m.Start(context.Background(), uuid.NewString(), winner,
		SignInParams{ClientID: "cid"}, winnerProbe.hooks()); err != nil {
		t.Fatalf("the second Start: %v", err)
	}

	close(loser.startGate)
	if err := <-errCh; err == nil {
		t.Fatal("the losing Start returned nil")
	}

	redirectURI, _, _ := loser.startArgs()
	if !portIsFree(t, signInPort(t, redirectURI)) {
		t.Error("the losing flow left its loopback listening")
	}
	if cancels := loser.cancels(); cancels != 1 {
		t.Errorf("CancelDesktopSignIn ran %d time(s), want 1", cancels)
	}
	if closes := loser.closes(); closes != 1 {
		t.Errorf("client closed %d time(s), want 1", closes)
	}
	if st, ok := f.m.Status(f.id); !ok || st.State != FlowError {
		t.Errorf("Status() = %+v, %v, want a retained error", st, ok)
	}
}

func loserStarted(c *fakeSignInClient) bool {
	c.mu.Lock()
	defer c.mu.Unlock()

	return c.startCalls == 1
}

func signInPort(t *testing.T, redirectURI string) int {
	t.Helper()

	u, err := url.Parse(redirectURI)
	if err != nil {
		t.Fatalf("parse %q: %v", redirectURI, err)
	}

	return portNumber(t, u.Port())
}

func TestSignInStartClosesTheClientItRefuses(t *testing.T) {
	t.Run("invalid uuid", func(t *testing.T) {
		client := newFakeSignInClient()
		f := newSignInClockFixture(t, client)
		if _, err := f.m.Start(context.Background(), "nope", client, SignInParams{}, SignInHooks{}); err == nil {
			t.Fatal("Start accepted a flow id that is not a UUID")
		}
		if closes := client.closes(); closes != 1 {
			t.Errorf("client closed %d time(s), want 1", closes)
		}
	})

	t.Run("id already used", func(t *testing.T) {
		f := newSignInClockFixture(t, newFakeSignInClient(stepPending()))
		f.mustStart(t)

		second := newFakeSignInClient(stepPending())
		if _, err := f.m.Start(context.Background(), f.id, second, SignInParams{}, SignInHooks{}); err == nil {
			t.Fatal("Start accepted a flow id that is already in use")
		}
		if closes := second.closes(); closes != 1 {
			t.Errorf("the refused client closed %d time(s), want 1", closes)
		}
	})

	t.Run("after shutdown", func(t *testing.T) {
		f := newSignInClockFixture(t, newFakeSignInClient())
		if err := f.m.Shutdown(context.Background()); err != nil {
			t.Fatalf("Shutdown: %v", err)
		}

		client := newFakeSignInClient()
		_, err := f.m.Start(context.Background(), uuid.NewString(), client, SignInParams{}, SignInHooks{})
		if !errors.Is(err, ErrManagerClosed) {
			t.Fatalf("Start after Shutdown = %v, want ErrManagerClosed", err)
		}
		if closes := client.closes(); closes != 1 {
			t.Errorf("client closed %d time(s), want 1", closes)
		}
	})
}

func TestSignInSecondStartReplacesTheFirst(t *testing.T) {
	first := newFakeSignInClient(stepPending())
	f := newSignInClockFixture(t, first)
	f.mustStart(t)

	second := newFakeSignInClient(stepPending())
	secondID := uuid.NewString()
	secondProbe := &hookProbe{}
	if _, err := f.m.Start(context.Background(), secondID, second, SignInParams{ClientID: "cid"}, secondProbe.hooks()); err != nil {
		t.Fatalf("the second Start: %v", err)
	}

	st, _ := f.m.Status(f.id)
	if st.State != FlowCancelled {
		t.Errorf("the first flow is %s, want cancelled", st.State)
	}
	waitForSignIn(t, "the first request to be cancelled on the server", func() bool { return first.cancels() == 1 })
	waitForSignIn(t, "the replaced client to be closed", func() bool { return first.closes() == 1 })
	if probe := f.probe.snapshot(); len(probe.terminals) != 1 || probe.terminals[0] != FlowCancelled {
		t.Errorf("OnTerminal saw %v, want one cancelled", probe.terminals)
	}
	if st, ok := f.m.Status(secondID); !ok || st.State != FlowPending {
		t.Errorf("the second flow is %+v, want pending", st)
	}
}

func TestSignInRefusesToReplaceAFlowStuckInItsCommit(t *testing.T) {
	stuck := newFakeSignInClient(stepApproved(SignInTokens{AccessToken: "at"}))
	clock := newFakeClock()
	opts := signInOptions(clock)
	opts.JoinCap = 200 * time.Millisecond
	f := newSignInFixture(t, stuck, opts)
	f.clock = clock

	gate := make(chan struct{})
	f.probe.commitGate = gate
	t.Cleanup(func() { close(gate) })

	f.mustStart(t)
	waitForSignIn(t, "the commit to start", func() bool { return f.probe.snapshot().approvals == 1 })

	second := newFakeSignInClient(stepPending())
	_, err := f.m.Start(context.Background(), uuid.NewString(), second, SignInParams{ClientID: "cid"}, SignInHooks{})
	var invalid *domain.ValidationError
	if !errors.As(err, &invalid) || invalid.Fields["auth"] == "" {
		t.Fatalf("the second Start = %v, want a validation error on auth", err)
	}
	if closes := second.closes(); closes != 1 {
		t.Errorf("the refused client closed %d time(s), want 1", closes)
	}
}

func TestSignInRefusesAServerThatCannotRunTheFlow(t *testing.T) {
	tests := []struct {
		name  string
		setup func(*fakeSignInClient)
	}{
		{"no discovery endpoint", func(c *fakeSignInClient) { c.infoErr = ErrServerInfoUnsupported }},
		{"capability missing", func(c *fakeSignInClient) { c.info.DesktopSignIn = false }},
		{"unusable sign-in url", func(c *fakeSignInClient) { c.info.DesktopSignInURL = "ftp://app.example/x" }},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			client := newFakeSignInClient()
			tc.setup(client)
			f := newSignInClockFixture(t, client)

			_, err := f.start(t)
			if !errors.Is(err, ErrSignInUnsupported) {
				t.Fatalf("Start = %v, want ErrSignInUnsupported", err)
			}
			if closes := client.closes(); closes != 1 {
				t.Errorf("client closed %d time(s), want 1", closes)
			}
			if st, ok := f.m.Status(f.id); !ok || st.State != FlowError {
				t.Fatalf("Status() = %+v, %v, want a retained error", st, ok)
			}

			f.clock.advance(defaultStatusRetention + time.Minute)
			if _, ok := f.m.Status(f.id); ok {
				t.Error("the failed flow outlived StatusRetention")
			}
		})
	}
}

func TestSignInShutdownJoinsOrReportsSurvivors(t *testing.T) {
	t.Run("cooperative", func(t *testing.T) {
		client := newFakeSignInClient(stepPending())
		f := newSignInClockFixture(t, client)
		f.mustStart(t)

		if err := f.m.Shutdown(context.Background()); err != nil {
			t.Fatalf("Shutdown: %v", err)
		}
		if closes := client.closes(); closes != 1 {
			t.Errorf("client closed %d time(s), want 1", closes)
		}
		if st, _ := f.m.Status(f.id); st.State != FlowCancelled {
			t.Errorf("state = %s, want cancelled", st.State)
		}
	})

	t.Run("uncooperative", func(t *testing.T) {
		client := newFakeSignInClient(stepPending())
		client.pollBlock = make(chan struct{})
		clock := newFakeClock()
		opts := signInOptions(clock)
		opts.JoinCap = 150 * time.Millisecond
		f := newSignInFixture(t, client, opts)
		t.Cleanup(func() { close(client.pollBlock) })

		f.mustStart(t)
		waitForSignIn(t, "the wedged poll", func() bool {
			st, ok := f.m.Status(f.id)

			return ok && st.State == FlowPending
		})
		time.Sleep(60 * time.Millisecond)

		first := f.m.Shutdown(context.Background())
		if first == nil {
			t.Fatal("Shutdown returned nil with a wedged flow")
		}
		if second := f.m.Shutdown(context.Background()); second == nil {
			t.Fatal("the second Shutdown returned nil while the survivor was still there")
		}
	})
}

func TestSignInKeepsSecretsOutOfTheLog(t *testing.T) {
	var buf bytes.Buffer
	previous := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})))
	t.Cleanup(func() { slog.SetDefault(previous) })

	client := newFakeSignInClient(stepPending())
	client.cancelErr = errors.New("cancel refused")
	clock := newFakeClock()
	opts := signInOptions(clock)
	opts.Listen = func(string, string) (net.Listener, error) { return nil, errors.New("boom") }
	f := newSignInFixture(t, client, opts)

	info := f.mustStart(t)
	waitForSignIn(t, "the first poll", func() bool { return client.polls() >= 1 })
	if err := f.m.Cancel(f.id); err != nil {
		t.Fatalf("Cancel: %v", err)
	}
	waitForSignIn(t, "the server request to be cancelled", func() bool { return client.cancels() == 1 })

	logged := buf.String()
	if logged == "" {
		t.Fatal("nothing was logged, the test proves nothing")
	}
	_, claim, _ := strings.Cut(info.LoginURL, "#claim=")
	for name, secret := range map[string]string{
		"the claim secret": claim,
		"the verifier":     client.seenVerifier(),
		"the login URL":    info.LoginURL,
	} {
		if secret != "" && strings.Contains(logged, secret) {
			t.Errorf("%s reached the log", name)
		}
	}
}
