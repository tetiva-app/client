package request_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/tetiva-app/client/internal/domain/entities"
	"github.com/tetiva-app/client/internal/domain/usecase/auth"
	"github.com/tetiva-app/client/internal/domain/usecase/request"
)

// memTokenStore is the token store and the usecase's TokenCleaner at once, so a
// test can watch an edit invalidate what an acquisition stored.
type memTokenStore struct {
	mu   sync.Mutex
	rows map[string]*auth.StoredToken
}

func newMemTokenStore() *memTokenStore {
	return &memTokenStore{rows: map[string]*auth.StoredToken{}}
}

func tokenKey(o entities.AuthOwner) string {
	return o.WorkspaceID.String() + "|" + o.Kind + "|" + o.ID.String()
}

func (s *memTokenStore) Get(_ context.Context, owner entities.AuthOwner) (*auth.StoredToken, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	row, ok := s.rows[tokenKey(owner)]
	if !ok || row.AccessToken == "" {
		return nil, nil
	}
	copied := *row
	return &copied, nil
}

func (s *memTokenStore) Reserve(_ context.Context, owner entities.AuthOwner) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	key := tokenKey(owner)
	if _, ok := s.rows[key]; !ok {
		s.rows[key] = &auth.StoredToken{}
	}
	return s.rows[key].Generation, nil
}

func (s *memTokenStore) Put(_ context.Context, owner entities.AuthOwner, expectedGeneration int64, hash string, t *auth.Token) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	row, ok := s.rows[tokenKey(owner)]
	if !ok || row.Generation != expectedGeneration {
		return auth.ErrStaleToken
	}
	row.Token = *t
	row.ConfigHash = hash
	// The store's Put is a compare-and-swap; the generation advances on success.
	row.Generation++
	return nil
}

func (s *memTokenStore) Clear(_ context.Context, owner entities.AuthOwner) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.clearLocked(tokenKey(owner))
	return nil
}

func (s *memTokenStore) ClearOwners(_ context.Context, kind string, ids []uuid.UUID) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, id := range ids {
		for key := range s.rows {
			if strings.HasSuffix(key, "|"+kind+"|"+id.String()) {
				s.clearLocked(key)
			}
		}
	}
	return nil
}

// ClearOwnersUnlessHash mirrors the SQLite predicate: a row holding the token
// keepHash names survives, a reserved but empty one never does.
func (s *memTokenStore) ClearOwnersUnlessHash(_ context.Context, kind string, ids []uuid.UUID, keepHash string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, id := range ids {
		for key, row := range s.rows {
			if !strings.HasSuffix(key, "|"+kind+"|"+id.String()) {
				continue
			}
			if row.AccessToken != "" && row.ConfigHash == keepHash {
				continue
			}
			s.clearLocked(key)
		}
	}
	return nil
}

func (s *memTokenStore) clearLocked(key string) {
	row, ok := s.rows[key]
	if !ok {
		s.rows[key] = &auth.StoredToken{Generation: 1}
		return
	}
	row.Token = auth.Token{}
	row.ConfigHash = ""
	row.Generation++
}

func (s *memTokenStore) DeleteOrphans(context.Context) (int, error) { return 0, nil }

func (s *memTokenStore) Generation(_ context.Context, owner entities.AuthOwner) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	row, ok := s.rows[tokenKey(owner)]
	if !ok {
		return 0, nil
	}
	return row.Generation, nil
}

func (s *memTokenStore) seed(owner entities.AuthOwner, hash, accessToken string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.rows[tokenKey(owner)] = &auth.StoredToken{
		Token:      auth.Token{AccessToken: accessToken, ExpiresAt: time.Now().Add(time.Hour)},
		ConfigHash: hash,
	}
}

// idpServer is a token endpoint that counts calls and can be held open.
type idpServer struct {
	*httptest.Server
	mu      sync.Mutex
	hits    int
	hold    chan struct{}
	arrived chan struct{}
}

func newIDPServer(t *testing.T) *idpServer {
	t.Helper()
	s := &idpServer{arrived: make(chan struct{}, 1)}
	s.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		s.mu.Lock()
		s.hits++
		hold := s.hold
		s.mu.Unlock()
		if hold != nil {
			s.arrived <- struct{}{}
			<-hold
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"access_token":"at-1","token_type":"Bearer","expires_in":3600}`))
	}))
	t.Cleanup(s.Close)
	return s
}

func (s *idpServer) hitCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.hits
}

func (s *idpServer) block() chan struct{} {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.hold = make(chan struct{})
	return s.hold
}

func ccAuthData(tokenURL, scope string) string {
	return `{"grant":"client_credentials","tokenUrl":"` + tokenURL +
		`","clientId":"cid","clientSecret":"sec","scope":"` + scope + `"}`
}

func requestOwner(id uuid.UUID) entities.AuthOwner {
	return entities.AuthOwner{WorkspaceID: testWorkspaceID, Kind: entities.AuthOwnerKindRequest, ID: id}
}

func configHashOf(t *testing.T, authData string) string {
	t.Helper()
	fields, err := auth.ParseFields(authData)
	if err != nil {
		t.Fatalf("ParseFields: %v", err)
	}
	return auth.ConfigHash(auth.OAuth2ConfigFromFields(fields))
}

type oauth2Deps struct {
	repo      *mockRepo
	store     *memTokenStore
	provider  auth.Provider
	requester *mockRequester
	graphql   *mockGraphQLRequester
	history   *mockHistoryRepo
	scripts   request.ScriptResolver
	engine    request.ScriptEngine
}

func newOAuth2Usecase(d *oauth2Deps) request.Usecase {
	if d.scripts == nil {
		d.scripts = &noopScriptResolver{}
	}
	if d.engine == nil {
		d.engine = &noopScriptEngine{}
	}
	return request.NewUsecase(d.repo, d.history, d.requester, nil, d.graphql,
		&mockEnvResolver{}, d.engine, d.scripts, &noopVarPersister{},
		request.NewAuthResolver(fixtureCollections()), nil, fixtureCollections(), d.store, d.provider)
}

func newOAuth2Deps(t *testing.T) *oauth2Deps {
	t.Helper()
	store := newMemTokenStore()
	return &oauth2Deps{
		repo:      newMockRepo(),
		store:     store,
		provider:  auth.NewProvider(store, nil, nil),
		requester: &mockRequester{response: &entities.Response{StatusCode: 200}},
		history:   &mockHistoryRepo{},
	}
}

func TestExecuteOAuth2AttachesBearerAndReusesTheToken(t *testing.T) {
	idp := newIDPServer(t)
	d := newOAuth2Deps(t)
	uc := newOAuth2Usecase(d)

	id := uuid.New()
	d.repo.requests[id] = &entities.Request{
		ID: id, CollectionID: testCollectionID, Protocol: entities.ProtocolHTTP, Method: entities.MethodGET,
		URL: "https://api.example.com/x", BodyType: entities.BodyTypeNone,
		AuthType: entities.AuthTypeOAuth2, AuthData: ccAuthData(idp.URL, "read"),
	}

	for i := 0; i < 2; i++ {
		if _, err := uc.Execute(context.Background(), id, request.ExecuteOpt{WorkspaceID: testWorkspaceID}); err != nil {
			t.Fatalf("Execute %d: %v", i, err)
		}
	}

	if got := d.requester.lastRequest.Headers["Authorization"]; len(got) != 1 || got[0] != "Bearer at-1" {
		t.Errorf("Authorization = %v, want [Bearer at-1]", got)
	}
	if idp.hitCount() != 1 {
		t.Errorf("token endpoint hits = %d, want 1 (second send must use the cache)", idp.hitCount())
	}
}

func TestExecuteGraphQLOAuth2QueryKeyLandsInHistory(t *testing.T) {
	idp := newIDPServer(t)
	d := newOAuth2Deps(t)
	d.graphql = &mockGraphQLRequester{response: &entities.Response{StatusCode: 200}}
	uc := newOAuth2Usecase(d)

	id := uuid.New()
	d.repo.requests[id] = &entities.Request{
		ID: id, CollectionID: testCollectionID, Protocol: entities.ProtocolGraphQL,
		URL: "https://api.example.com/graphql", GraphQLQuery: "{ me { id } }",
		AuthType: entities.AuthTypeOAuth2,
		AuthData: `{"grant":"client_credentials","tokenUrl":"` + idp.URL +
			`","clientId":"cid","addTo":"query","queryParam":"tok"}`,
	}

	if _, err := uc.Execute(context.Background(), id, request.ExecuteOpt{WorkspaceID: testWorkspaceID}); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	if !strings.Contains(d.graphql.last.Endpoint, "tok=at-1") {
		t.Errorf("endpoint = %q, want the token in the query", d.graphql.last.Endpoint)
	}
	if len(d.history.entries) != 1 {
		t.Fatalf("history entries = %d, want 1", len(d.history.entries))
	}
	if got := d.history.entries[0].AuthQueryKeys; len(got) != 1 || got[0] != "tok" {
		t.Errorf("history AuthQueryKeys = %v, want [tok]", got)
	}
}

func TestBuildCurlOAuth2WithoutCachedTokenWarnsAndSkipsTheIdP(t *testing.T) {
	idp := newIDPServer(t)
	d := newOAuth2Deps(t)
	d.scripts = &scriptResolverWithPre{pre: "// inject"}
	d.engine = &captureScriptEngine{preHeaders: map[string][]string{"X-Trace": {"abc"}}, preVars: map[string]string{}}
	uc := newOAuth2Usecase(d)

	id := uuid.New()
	d.repo.requests[id] = &entities.Request{
		ID: id, CollectionID: testCollectionID, Protocol: entities.ProtocolHTTP, Method: entities.MethodGET,
		URL: "https://api.example.com/x", BodyType: entities.BodyTypeNone,
		AuthType: entities.AuthTypeOAuth2, AuthData: ccAuthData(idp.URL, "read"),
	}

	res, err := uc.BuildCurl(context.Background(), id, request.BuildCurlOpt{WorkspaceID: testWorkspaceID})
	if err != nil {
		t.Fatalf("BuildCurl: %v", err)
	}
	if idp.hitCount() != 0 {
		t.Errorf("token endpoint hits = %d, want 0: Copy as cURL must not acquire a token", idp.hitCount())
	}
	if strings.Contains(res.Command, "Authorization") {
		t.Errorf("command carries an Authorization header without a cached token:\n%s", res.Command)
	}
	if len(res.Warnings) != 1 || !strings.Contains(res.Warnings[0], "OAuth 2.0 token") {
		t.Errorf("warnings = %v, want one about the missing token", res.Warnings)
	}
	if !strings.Contains(res.Command, "-H 'X-Trace: abc'") {
		t.Errorf("pre-script header missing:\n%s", res.Command)
	}
	if res.ScriptResult == nil {
		t.Error("script result must survive the warning")
	}
}

func TestBuildCurlOAuth2UsesTheCachedToken(t *testing.T) {
	idp := newIDPServer(t)
	d := newOAuth2Deps(t)
	uc := newOAuth2Usecase(d)

	id := uuid.New()
	authData := ccAuthData(idp.URL, "read")
	d.repo.requests[id] = &entities.Request{
		ID: id, CollectionID: testCollectionID, Protocol: entities.ProtocolHTTP, Method: entities.MethodGET,
		URL: "https://api.example.com/x", BodyType: entities.BodyTypeNone,
		AuthType: entities.AuthTypeOAuth2, AuthData: authData,
	}
	d.store.seed(requestOwner(id), configHashOf(t, authData), "cached-1")

	res, err := uc.BuildCurl(context.Background(), id, request.BuildCurlOpt{WorkspaceID: testWorkspaceID})
	if err != nil {
		t.Fatalf("BuildCurl: %v", err)
	}
	if !strings.Contains(res.Command, "-H 'Authorization: Bearer cached-1'") {
		t.Errorf("cached token missing from the command:\n%s", res.Command)
	}
	if len(res.Warnings) != 0 {
		t.Errorf("warnings = %v, want none", res.Warnings)
	}
	if idp.hitCount() != 0 {
		t.Errorf("token endpoint hits = %d, want 0", idp.hitCount())
	}
}

func TestEditClearsTheTokenWhenTheAcquisitionConfigChanges(t *testing.T) {
	idp := newIDPServer(t)
	d := newOAuth2Deps(t)
	uc := newOAuth2Usecase(d)

	id := uuid.New()
	authData := ccAuthData(idp.URL, "read")
	d.repo.requests[id] = &entities.Request{
		ID: id, CollectionID: testCollectionID, Name: "R", Protocol: entities.ProtocolHTTP, Method: entities.MethodGET,
		URL: "https://api.example.com/x", BodyType: entities.BodyTypeNone,
		AuthType: entities.AuthTypeOAuth2, AuthData: authData, Version: 1,
	}
	owner := requestOwner(id)
	d.store.seed(owner, configHashOf(t, authData), "cached-1")

	edit := func(t *testing.T, data string, version int) {
		t.Helper()
		_, err := uc.Edit(context.Background(), request.Edit{
			Name: "R", Method: entities.MethodGET, URL: "https://api.example.com/x",
			BodyType: entities.BodyTypeNone, AuthType: entities.AuthTypeOAuth2, AuthData: data,
		}, request.EditOpt{RequestID: id, Version: version})
		if err != nil {
			t.Fatalf("Edit: %v", err)
		}
	}

	edit(t, authData, 1)
	stored, err := d.store.Get(context.Background(), owner)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if stored == nil || stored.AccessToken != "cached-1" {
		t.Fatalf("an unchanged configuration must keep the token, got %+v", stored)
	}

	edit(t, ccAuthData(idp.URL, "write"), 2)
	stored, err = d.store.Get(context.Background(), owner)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if stored != nil {
		t.Errorf("a changed scope must clear the token, got %+v", stored)
	}
}

func TestEditKeepsTheTokenTheSavedConfigProduced(t *testing.T) {
	idp := newIDPServer(t)
	d := newOAuth2Deps(t)
	uc := newOAuth2Usecase(d)

	id := uuid.New()
	saved := ccAuthData(idp.URL, "read")
	buffer := ccAuthData(idp.URL, "write")
	d.repo.requests[id] = &entities.Request{
		ID: id, CollectionID: testCollectionID, Name: "R", Protocol: entities.ProtocolHTTP, Method: entities.MethodGET,
		URL: "https://api.example.com/x", BodyType: entities.BodyTypeNone,
		AuthType: entities.AuthTypeOAuth2, AuthData: saved, Version: 1,
	}
	owner := requestOwner(id)

	// "Get token" runs on the editor buffer, which Cmd+S only commits afterwards.
	fields, err := auth.ParseFields(buffer)
	if err != nil {
		t.Fatalf("ParseFields: %v", err)
	}
	if _, err := d.provider.Fetch(context.Background(), owner, auth.OAuth2ConfigFromFields(fields)); err != nil {
		t.Fatalf("Fetch: %v", err)
	}

	if _, err := uc.Edit(context.Background(), request.Edit{
		Name: "R", Method: entities.MethodGET, URL: "https://api.example.com/x",
		BodyType: entities.BodyTypeNone, AuthType: entities.AuthTypeOAuth2, AuthData: buffer,
	}, request.EditOpt{RequestID: id, Version: 1}); err != nil {
		t.Fatalf("Edit: %v", err)
	}

	stored, err := d.store.Get(context.Background(), owner)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if stored == nil || stored.ConfigHash != configHashOf(t, buffer) {
		t.Fatalf("saving the configuration that produced the token dropped it, got %+v", stored)
	}
	if _, err := uc.Execute(context.Background(), id, request.ExecuteOpt{WorkspaceID: testWorkspaceID}); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if idp.hitCount() != 1 {
		t.Errorf("token endpoint hits = %d, want 1", idp.hitCount())
	}
}

func TestEditDuringAcquisitionMakesTheWriteStale(t *testing.T) {
	idp := newIDPServer(t)
	d := newOAuth2Deps(t)
	uc := newOAuth2Usecase(d)

	id := uuid.New()
	authData := ccAuthData(idp.URL, "read")
	d.repo.requests[id] = &entities.Request{
		ID: id, CollectionID: testCollectionID, Name: "R", Protocol: entities.ProtocolHTTP, Method: entities.MethodGET,
		URL: "https://api.example.com/x", BodyType: entities.BodyTypeNone,
		AuthType: entities.AuthTypeOAuth2, AuthData: authData, Version: 1,
	}
	owner := requestOwner(id)
	fields, err := auth.ParseFields(authData)
	if err != nil {
		t.Fatalf("ParseFields: %v", err)
	}
	cfg := auth.OAuth2ConfigFromFields(fields)

	hold := idp.block()
	done := make(chan error, 1)
	go func() {
		_, tokenErr := d.provider.AccessToken(context.Background(), owner, cfg)
		done <- tokenErr
	}()

	<-idp.arrived
	if _, editErr := uc.Edit(context.Background(), request.Edit{
		Name: "R", Method: entities.MethodGET, URL: "https://api.example.com/x",
		BodyType: entities.BodyTypeNone, AuthType: entities.AuthTypeOAuth2,
		AuthData: ccAuthData(idp.URL, "write"),
	}, request.EditOpt{RequestID: id, Version: 1}); editErr != nil {
		t.Fatalf("Edit: %v", editErr)
	}
	close(hold)

	if tokenErr := <-done; !errors.Is(tokenErr, auth.ErrStaleToken) {
		t.Fatalf("AccessToken: got %v, want ErrStaleToken", tokenErr)
	}
	stored, err := d.store.Get(context.Background(), owner)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if stored != nil {
		t.Errorf("a token acquired for the old configuration must not survive, got %+v", stored)
	}
}

func TestResolveWebSocketOAuth2DirectAndInherited(t *testing.T) {
	idp := newIDPServer(t)

	collectionAuth := `{"grant":"client_credentials","tokenUrl":"` + idp.URL + `","clientId":"cid"}`
	collections := fixtureCollections()
	collections.collections[testCollectionID].AuthType = entities.AuthTypeOAuth2
	collections.collections[testCollectionID].AuthData = collectionAuth

	store := newMemTokenStore()
	repo := newMockRepo()
	uc := request.NewUsecase(repo, &mockHistoryRepo{}, &mockRequester{}, nil, nil,
		&mockEnvResolver{}, &noopScriptEngine{}, &noopScriptResolver{}, &noopVarPersister{},
		request.NewAuthResolver(collections), nil, collections, store, auth.NewProvider(store, nil, nil))

	inheritID := uuid.New()
	inherit := wsRequest(inheritID, "wss://example.com/ws")
	inherit.AuthType = entities.AuthTypeInherit
	repo.requests[inheritID] = inherit

	directID := uuid.New()
	direct := wsRequest(directID, "wss://example.com/ws")
	direct.AuthType = entities.AuthTypeOAuth2
	direct.AuthData = `{"grant":"client_credentials","tokenUrl":"` + idp.URL +
		`","clientId":"cid","addTo":"query","queryParam":"tok"}`
	repo.requests[directID] = direct

	dial, err := uc.ResolveWebSocket(context.Background(), inheritID, testWorkspaceID, "user-1")
	if err != nil {
		t.Fatalf("ResolveWebSocket inherit: %v", err)
	}
	if dial.Failed != "" {
		t.Fatalf("inherit failed: %s", dial.Failed)
	}
	if got := dial.Headers["Authorization"]; len(got) != 1 || got[0] != "Bearer at-1" {
		t.Errorf("inherited Authorization = %v, want [Bearer at-1]", got)
	}

	dial, err = uc.ResolveWebSocket(context.Background(), directID, testWorkspaceID, "user-1")
	if err != nil {
		t.Fatalf("ResolveWebSocket direct: %v", err)
	}
	if dial.Failed != "" {
		t.Fatalf("direct failed: %s", dial.Failed)
	}
	if !strings.Contains(dial.URL, "tok=at-1") {
		t.Errorf("direct URL = %q, want the token in the query", dial.URL)
	}
	if len(dial.AuthQueryKeys) != 1 || dial.AuthQueryKeys[0] != "tok" {
		t.Errorf("AuthQueryKeys = %v, want [tok]", dial.AuthQueryKeys)
	}
	// The collection and the request are separate owners, so each fetched once.
	if idp.hitCount() != 2 {
		t.Errorf("token endpoint hits = %d, want 2", idp.hitCount())
	}
}
