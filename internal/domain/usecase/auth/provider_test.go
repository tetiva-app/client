package auth

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/tetiva-app/client/internal/domain/entities"
)

// fakeRepo mirrors the SQLite store's contract: rows survive a Clear as
// tombstones, Put is conditional on the generation, Get hides tombstones.
type fakeRepo struct {
	mu        sync.Mutex
	rows      map[string]*StoredToken
	ownerGone bool

	reserves int
	puts     int
	clears   int
}

func newFakeRepo() *fakeRepo { return &fakeRepo{rows: map[string]*StoredToken{}} }

func (r *fakeRepo) Get(_ context.Context, owner entities.AuthOwner) (*StoredToken, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	row, ok := r.rows[ownerKey(owner)]
	if !ok || row.AccessToken == "" {
		return nil, nil
	}
	copied := *row
	return &copied, nil
}

func (r *fakeRepo) Reserve(_ context.Context, owner entities.AuthOwner) (int64, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.reserves++
	if r.ownerGone {
		return 0, ErrOwnerGone
	}
	key := ownerKey(owner)
	if _, ok := r.rows[key]; !ok {
		r.rows[key] = &StoredToken{}
	}
	return r.rows[key].Generation, nil
}

func (r *fakeRepo) Put(_ context.Context, owner entities.AuthOwner, expectedGeneration int64, hash string, t *Token) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.puts++
	row, ok := r.rows[ownerKey(owner)]
	if !ok || row.Generation != expectedGeneration {
		return ErrStaleToken
	}
	row.Token = *t
	row.ConfigHash = hash
	// The store's Put is a compare-and-swap; a double that skips this lets two
	// writers holding one reservation both commit.
	row.Generation++
	return nil
}

func (r *fakeRepo) Clear(_ context.Context, owner entities.AuthOwner) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.clears++
	key := ownerKey(owner)
	row, ok := r.rows[key]
	if !ok {
		r.rows[key] = &StoredToken{Generation: 1}
		return nil
	}
	row.Token = Token{}
	row.ConfigHash = ""
	row.Generation++
	return nil
}

func (r *fakeRepo) ClearOwners(context.Context, string, []uuid.UUID) error { return nil }

func (r *fakeRepo) ClearOwnersUnlessHash(context.Context, string, []uuid.UUID, string) error {
	return nil
}

func (r *fakeRepo) DeleteOrphans(context.Context) (int, error) { return 0, nil }

func (r *fakeRepo) Generation(_ context.Context, owner entities.AuthOwner) (int64, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	row, ok := r.rows[ownerKey(owner)]
	if !ok {
		return 0, nil
	}
	return row.Generation, nil
}

func (r *fakeRepo) seed(owner entities.AuthOwner, hash string, t Token) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.rows[ownerKey(owner)] = &StoredToken{Token: t, ConfigHash: hash}
}

func (r *fakeRepo) row(owner entities.AuthOwner) *StoredToken {
	r.mu.Lock()
	defer r.mu.Unlock()

	row, ok := r.rows[ownerKey(owner)]
	if !ok {
		return nil
	}
	copied := *row
	return &copied
}

func (r *fakeRepo) counts() (reserves, puts, clears int) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.reserves, r.puts, r.clears
}

func testOwner() entities.AuthOwner {
	return entities.AuthOwner{WorkspaceID: uuid.New(), Kind: entities.AuthOwnerKindRequest, ID: uuid.New()}
}

func newTestProvider(repo TokenRepository) Provider {
	return NewProvider(repo, NewTokenHTTPClient(), func() time.Time { return testNow })
}

func ccConfig(tokenURL string) OAuth2Config {
	return OAuth2Config{
		Grant: GrantClientCredentials, TokenURL: tokenURL, ClientID: "app",
		ClientSecret: "s3cret", ClientAuth: ClientAuthBasic, Scope: "read",
	}
}

func TestProviderAccessTokenFetchesThenServesFromCache(t *testing.T) {
	server := newIDP(t, `{"access_token":"at-1","expires_in":3600}`)
	repo := newFakeRepo()
	p := newTestProvider(repo)
	owner := testOwner()
	cfg := ccConfig(server.url())

	got, err := p.AccessToken(context.Background(), owner, cfg)
	if err != nil {
		t.Fatalf("AccessToken: %v", err)
	}
	if got != "at-1" {
		t.Fatalf("token: got %q", got)
	}
	row := repo.row(owner)
	if row == nil || row.ConfigHash != ConfigHash(cfg) {
		t.Fatalf("token not stored under the config hash: %+v", row)
	}

	if got, err = p.AccessToken(context.Background(), owner, cfg); err != nil || got != "at-1" {
		t.Fatalf("second call: %q, %v", got, err)
	}
	if server.hitCount() != 1 {
		t.Errorf("cached token must not hit the IdP again: %d hits", server.hitCount())
	}
}

func TestProviderRefreshesBeforeExpiryAndKeepsRefreshToken(t *testing.T) {
	server := newIDP(t, `{"access_token":"at-2","expires_in":3600}`)
	repo := newFakeRepo()
	p := newTestProvider(repo)
	owner := testOwner()
	cfg := ccConfig(server.url())
	// Still valid by the clock, but inside the 30 s skew.
	repo.seed(owner, ConfigHash(cfg), Token{
		AccessToken: "old", RefreshToken: "rt-old", ExpiresAt: testNow.Add(10 * time.Second),
	})

	got, err := p.AccessToken(context.Background(), owner, cfg)
	if err != nil {
		t.Fatalf("AccessToken: %v", err)
	}
	if got != "at-2" {
		t.Fatalf("token: got %q", got)
	}

	assertForm(t, server.lastForm(), url.Values{
		"grant_type":    {"refresh_token"},
		"refresh_token": {"rt-old"},
		"scope":         {"read"},
	})
	row := repo.row(owner)
	if row.RefreshToken != "rt-old" {
		t.Errorf("a response without refresh_token must keep the old one, got %q", row.RefreshToken)
	}
	if !row.ExpiresAt.Equal(testNow.Add(time.Hour)) {
		t.Errorf("ExpiresAt: got %v", row.ExpiresAt)
	}
}

func TestProviderExpiredWithoutRefreshTokenFetchesAgain(t *testing.T) {
	server := newIDP(t, `{"access_token":"at-3","expires_in":3600}`)
	repo := newFakeRepo()
	p := newTestProvider(repo)
	owner := testOwner()
	cfg := ccConfig(server.url())
	repo.seed(owner, ConfigHash(cfg), Token{AccessToken: "old", ExpiresAt: testNow.Add(-time.Minute)})

	got, err := p.AccessToken(context.Background(), owner, cfg)
	if err != nil {
		t.Fatalf("AccessToken: %v", err)
	}
	if got != "at-3" {
		t.Fatalf("token: got %q", got)
	}
	if form := server.lastForm(); form.Get("grant_type") != GrantClientCredentials {
		t.Errorf("grant_type: got %q", form.Get("grant_type"))
	}
}

func TestProviderHashMismatchIsACacheMiss(t *testing.T) {
	server := newIDP(t, `{"access_token":"at-4","expires_in":3600}`)
	repo := newFakeRepo()
	p := newTestProvider(repo)
	owner := testOwner()
	cfg := ccConfig(server.url())
	repo.seed(owner, "hash-of-another-environment", Token{
		AccessToken: "other-env", ExpiresAt: testNow.Add(time.Hour),
	})

	tok, ok, err := p.Peek(context.Background(), owner, cfg)
	if err != nil {
		t.Fatalf("Peek: %v", err)
	}
	if ok || tok != "" {
		t.Fatalf("a token from another configuration must not be served: %q", tok)
	}
	if server.hitCount() != 0 {
		t.Error("Peek must never touch the network")
	}
	if row := repo.row(owner); row == nil || row.AccessToken != "other-env" {
		t.Error("a hash mismatch must not delete the stored row")
	}
	if status, err := p.Status(context.Background(), owner, cfg); err != nil || status.State != TokenStateNone {
		t.Errorf("Status: %+v, %v", status, err)
	}

	got, err := p.AccessToken(context.Background(), owner, cfg)
	if err != nil {
		t.Fatalf("AccessToken: %v", err)
	}
	if got != "at-4" {
		t.Fatalf("token: got %q", got)
	}
	if row := repo.row(owner); row.ConfigHash != ConfigHash(cfg) {
		t.Error("the fresh acquisition must replace the stored hash")
	}
}

func TestProviderPeekAndStatusStates(t *testing.T) {
	repo := newFakeRepo()
	p := newTestProvider(repo)
	owner := testOwner()
	cfg := ccConfig("https://idp.example/token")
	hash := ConfigHash(cfg)

	if status, err := p.Status(context.Background(), owner, cfg); err != nil || status.State != TokenStateNone {
		t.Fatalf("no row: %+v, %v", status, err)
	}

	repo.seed(owner, hash, Token{AccessToken: "at", ExpiresAt: testNow.Add(time.Hour)})
	status, err := p.Status(context.Background(), owner, cfg)
	if err != nil || status.State != TokenStateValid || !status.ExpiresAt.Equal(testNow.Add(time.Hour)) {
		t.Fatalf("valid: %+v, %v", status, err)
	}
	if tok, ok, err := p.Peek(context.Background(), owner, cfg); err != nil || !ok || tok != "at" {
		t.Fatalf("Peek on a valid token: %q, %v, %v", tok, ok, err)
	}

	repo.seed(owner, hash, Token{AccessToken: "at", RefreshToken: "rt", ExpiresAt: testNow.Add(-time.Second)})
	if status, err = p.Status(context.Background(), owner, cfg); err != nil || status.State != TokenStateExpiredRefreshable {
		t.Fatalf("expired refreshable: %+v, %v", status, err)
	}

	repo.seed(owner, hash, Token{AccessToken: "at", ExpiresAt: testNow.Add(-time.Second)})
	if status, err = p.Status(context.Background(), owner, cfg); err != nil || status.State != TokenStateExpired {
		t.Fatalf("expired: %+v, %v", status, err)
	}
	if _, ok, err := p.Peek(context.Background(), owner, cfg); err != nil || ok {
		t.Fatalf("Peek on an expired token: %v, %v", ok, err)
	}

	// A token without a stated expiry stays usable.
	repo.seed(owner, hash, Token{AccessToken: "at"})
	if status, err = p.Status(context.Background(), owner, cfg); err != nil || status.State != TokenStateValid {
		t.Fatalf("no expiry: %+v, %v", status, err)
	}
}

func TestProviderInteractiveGrantNeedsAToken(t *testing.T) {
	server := newIDP(t, `{"access_token":"never"}`)
	repo := newFakeRepo()
	p := newTestProvider(repo)
	owner := testOwner()
	cfg := OAuth2Config{
		Grant: GrantAuthorizationCode, TokenURL: server.url(), AuthURL: "https://idp.example/authorize",
		ClientID: "app", ClientAuth: ClientAuthBasic,
	}

	_, err := p.AccessToken(context.Background(), owner, cfg)
	if !errors.Is(err, ErrTokenRequired) {
		t.Fatalf("AccessToken: got %v, want ErrTokenRequired", err)
	}
	if _, err = p.Fetch(context.Background(), owner, cfg); !errors.Is(err, ErrTokenRequired) {
		t.Fatalf("Fetch: got %v, want ErrTokenRequired", err)
	}
	if server.hitCount() != 0 {
		t.Error("an interactive grant must not call the token endpoint without a token")
	}

	// A stored refresh token is the one way stage A can renew an interactive grant.
	repo.seed(owner, ConfigHash(cfg), Token{
		AccessToken: "old", RefreshToken: "rt", ExpiresAt: testNow.Add(-time.Minute),
	})
	got, err := p.AccessToken(context.Background(), owner, cfg)
	if err != nil {
		t.Fatalf("AccessToken with a refresh token: %v", err)
	}
	if got != "never" {
		t.Fatalf("token: got %q", got)
	}
}

func TestProviderFetchAlwaysAcquires(t *testing.T) {
	server := newIDP(t, `{"access_token":"fresh","expires_in":3600}`)
	repo := newFakeRepo()
	p := newTestProvider(repo)
	owner := testOwner()
	cfg := ccConfig(server.url())
	repo.seed(owner, ConfigHash(cfg), Token{AccessToken: "cached", ExpiresAt: testNow.Add(time.Hour)})

	tok, err := p.Fetch(context.Background(), owner, cfg)
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if tok.AccessToken != "fresh" {
		t.Fatalf("Fetch must ignore the cache, got %q", tok.AccessToken)
	}
	if server.hitCount() != 1 {
		t.Errorf("hits: got %d", server.hitCount())
	}
	if row := repo.row(owner); row.AccessToken != "fresh" {
		t.Errorf("stored token: got %q", row.AccessToken)
	}
}

func TestProviderClearedDuringAcquisitionIsStale(t *testing.T) {
	repo := newFakeRepo()
	owner := testOwner()
	server := newIDP(t, `{"access_token":"late","expires_in":3600}`)
	p := newTestProvider(repo)
	cfg := ccConfig(server.url())

	// The owner is cleared while the token endpoint call is in flight.
	hold := server.block()
	done := make(chan error, 1)
	go func() {
		_, err := p.AccessToken(context.Background(), owner, cfg)
		done <- err
	}()
	<-server.arrived
	if err := repo.Clear(context.Background(), owner); err != nil {
		t.Fatalf("Clear: %v", err)
	}
	close(hold)

	if err := <-done; !errors.Is(err, ErrStaleToken) {
		t.Fatalf("AccessToken: got %v, want ErrStaleToken", err)
	}
	if row := repo.row(owner); row.AccessToken != "" {
		t.Errorf("a stale write must not resurrect the token, got %q", row.AccessToken)
	}
}

func TestProviderOwnerGoneAbortsBeforeNetwork(t *testing.T) {
	server := newIDP(t, `{"access_token":"never"}`)
	repo := newFakeRepo()
	repo.ownerGone = true
	p := newTestProvider(repo)

	_, err := p.AccessToken(context.Background(), testOwner(), ccConfig(server.url()))
	if !errors.Is(err, ErrOwnerGone) {
		t.Fatalf("AccessToken: got %v, want ErrOwnerGone", err)
	}
	if server.hitCount() != 0 {
		t.Error("a gone owner must not reach the token endpoint")
	}
}

func TestProviderRejectsInsecureEndpointBeforeReserve(t *testing.T) {
	repo := newFakeRepo()
	p := newTestProvider(repo)
	cfg := ccConfig("http://idp.example/token")

	if _, err := p.AccessToken(context.Background(), testOwner(), cfg); err == nil {
		t.Fatal("expected plain http to a public host to be rejected")
	}
	if reserves, _, _ := repo.counts(); reserves != 0 {
		t.Errorf("a rejected configuration must not reserve a row: %d", reserves)
	}
}

func TestProviderSingleflight(t *testing.T) {
	server := newIDP(t, `{"access_token":"shared","expires_in":3600}`)
	repo := newFakeRepo()
	p := newTestProvider(repo)
	owner := testOwner()
	cfg := ccConfig(server.url())

	hold := server.block()
	var wg sync.WaitGroup
	results := make([]string, 10)
	errs := make([]error, 10)
	for i := range results {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			results[i], errs[i] = p.AccessToken(context.Background(), owner, cfg)
		}(i)
	}
	<-server.arrived
	// The leader is inside the handler; give the others time to join the flight.
	time.Sleep(150 * time.Millisecond)
	close(hold)
	wg.Wait()

	for i := range results {
		if errs[i] != nil {
			t.Fatalf("call %d: %v", i, errs[i])
		}
		if results[i] != "shared" {
			t.Fatalf("call %d: got %q", i, results[i])
		}
	}
	if server.hitCount() != 1 {
		t.Errorf("a fan-out must acquire one token, got %d requests", server.hitCount())
	}
}

func TestProviderClearDelegatesToTheStore(t *testing.T) {
	repo := newFakeRepo()
	p := newTestProvider(repo)
	owner := testOwner()
	repo.seed(owner, "hash", Token{AccessToken: "at"})

	if err := p.Clear(context.Background(), owner); err != nil {
		t.Fatalf("Clear: %v", err)
	}
	row := repo.row(owner)
	if row.AccessToken != "" || row.Generation != 1 {
		t.Fatalf("Clear must tombstone the row: %+v", row)
	}
}

// grantSwitchIDP answers refresh_token exchanges with an RFC 6749 rejection and
// every other grant with a fresh token.
func grantSwitchIDP(t *testing.T) (url string, refreshes, grants func() int) {
	t.Helper()

	var mu sync.Mutex
	var refreshCalls, grantCalls int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			t.Errorf("parse form: %v", err)
		}
		mu.Lock()
		refresh := r.PostForm.Get("grant_type") == "refresh_token"
		if refresh {
			refreshCalls++
		} else {
			grantCalls++
		}
		mu.Unlock()

		w.Header().Set("Content-Type", "application/json")
		if refresh {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"error":"invalid_grant","error_description":"refresh token revoked"}`))
			return
		}
		_, _ = w.Write([]byte(`{"access_token":"fresh","expires_in":3600}`))
	}))
	t.Cleanup(server.Close)

	count := func(n *int) func() int {
		return func() int {
			mu.Lock()
			defer mu.Unlock()
			return *n
		}
	}

	return server.URL + "/token", count(&refreshCalls), count(&grantCalls)
}

func TestProviderRejectedRefreshFallsBackToTheGrant(t *testing.T) {
	tokenURL, refreshes, grants := grantSwitchIDP(t)
	repo := newFakeRepo()
	p := newTestProvider(repo)
	owner := testOwner()
	cfg := ccConfig(tokenURL)
	repo.seed(owner, ConfigHash(cfg), Token{
		AccessToken: "stale", RefreshToken: "rt-revoked", ExpiresAt: testNow.Add(-time.Minute),
	})

	got, err := p.AccessToken(context.Background(), owner, cfg)
	if err != nil {
		t.Fatalf("AccessToken: %v", err)
	}
	if got != "fresh" {
		t.Fatalf("token: got %q, want the re-acquired one", got)
	}
	if refreshes() != 1 || grants() != 1 {
		t.Errorf("expected one rejected refresh and one grant, got %d/%d", refreshes(), grants())
	}
	if row := repo.row(owner); row.RefreshToken != "" || row.AccessToken != "fresh" {
		t.Errorf("the row must hold the new token: %+v", row)
	}

	// The owner is not wedged: the next send is served from the new token.
	if got, err = p.AccessToken(context.Background(), owner, cfg); err != nil || got != "fresh" {
		t.Fatalf("second call: %q, %v", got, err)
	}
	if grants() != 1 {
		t.Errorf("the stored token must be reused, got %d grants", grants())
	}
}

func TestProviderRejectedRefreshOnAnInteractiveGrantSaysWhatToDo(t *testing.T) {
	tokenURL, refreshes, grants := grantSwitchIDP(t)
	repo := newFakeRepo()
	p := newTestProvider(repo)
	owner := testOwner()
	cfg := OAuth2Config{
		Grant: GrantAuthorizationCode, TokenURL: tokenURL, ClientID: "app", ClientAuth: ClientAuthNone,
	}
	repo.seed(owner, ConfigHash(cfg), Token{
		AccessToken: "stale", RefreshToken: "rt-revoked", ExpiresAt: testNow.Add(-time.Minute),
	})

	_, err := p.AccessToken(context.Background(), owner, cfg)
	if err == nil {
		t.Fatal("expected the rejected refresh to surface")
	}
	for _, want := range []string{"Clear", "invalid_grant", "refresh token revoked"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q must mention %q", err, want)
		}
	}
	if refreshes() != 1 || grants() != 0 {
		t.Errorf("an interactive grant cannot re-acquire on its own: %d/%d", refreshes(), grants())
	}
	// The row is left for the user to decide about, not silently dropped.
	if row := repo.row(owner); row == nil || row.RefreshToken != "rt-revoked" {
		t.Errorf("the stored token must survive: %+v", row)
	}
}

func TestProviderRefreshIsSharedBetweenSendAndGetToken(t *testing.T) {
	server := newIDP(t, `{"access_token":"refreshed","expires_in":3600}`)
	repo := newFakeRepo()
	p := newTestProvider(repo)
	owner := testOwner()
	cfg := OAuth2Config{
		Grant: GrantAuthorizationCode, TokenURL: server.url(), ClientID: "app", ClientAuth: ClientAuthNone,
	}
	repo.seed(owner, ConfigHash(cfg), Token{
		AccessToken: "stale", RefreshToken: "rt-1", ExpiresAt: testNow.Add(-time.Minute),
	})

	hold := server.block()
	var wg sync.WaitGroup
	wg.Add(2)
	var sendErr, fetchErr error
	var sent string
	go func() {
		defer wg.Done()
		sent, sendErr = p.AccessToken(context.Background(), owner, cfg)
	}()
	<-server.arrived
	go func() {
		defer wg.Done()
		_, fetchErr = p.Fetch(context.Background(), owner, cfg)
	}()
	// The first exchange is inside the handler; let the second caller join it.
	time.Sleep(150 * time.Millisecond)
	close(hold)
	wg.Wait()

	if sendErr != nil || fetchErr != nil {
		t.Fatalf("send: %v, fetch: %v", sendErr, fetchErr)
	}
	if sent != "refreshed" {
		t.Errorf("token: got %q", sent)
	}
	// A rotated refresh token would be invalidated by presenting it twice.
	if server.hitCount() != 1 {
		t.Errorf("expected one refresh exchange, got %d", server.hitCount())
	}
}

func TestProviderSharedFetchSurvivesACancelledCaller(t *testing.T) {
	server := newIDP(t, `{"access_token":"shared","expires_in":3600}`)
	repo := newFakeRepo()
	p := newTestProvider(repo)
	owner := testOwner()
	cfg := ccConfig(server.url())

	hold := server.block()
	leaderCtx, cancelLeader := context.WithCancel(context.Background())
	var wg sync.WaitGroup
	wg.Add(2)
	var leaderErr, followerErr error
	var follower string
	go func() {
		defer wg.Done()
		_, leaderErr = p.AccessToken(leaderCtx, owner, cfg)
	}()
	<-server.arrived
	go func() {
		defer wg.Done()
		follower, followerErr = p.AccessToken(context.Background(), owner, cfg)
	}()
	time.Sleep(150 * time.Millisecond)
	cancelLeader()
	time.Sleep(50 * time.Millisecond)
	close(hold)
	wg.Wait()

	if !errors.Is(leaderErr, context.Canceled) {
		t.Errorf("the cancelled caller must see its own cancellation, got %v", leaderErr)
	}
	if followerErr != nil || follower != "shared" {
		t.Fatalf("the shared acquisition must finish: %q, %v", follower, followerErr)
	}
	if server.hitCount() != 1 {
		t.Errorf("expected one exchange, got %d", server.hitCount())
	}
}
