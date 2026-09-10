package wails

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tetiva-app/client/internal/adapters/wails/dto"
	"github.com/tetiva-app/client/internal/domain/entities"
	"github.com/tetiva-app/client/internal/domain/usecase/auth"
	"github.com/tetiva-app/client/internal/domain/usecase/request"
)

type authProviderStub struct {
	status    auth.TokenStatus
	statusErr error
	fetchErr  error

	statusCfg []auth.OAuth2Config
	peekCfg   []auth.OAuth2Config
	owners    []entities.AuthOwner
	fetches   int
	cleared   []entities.AuthOwner
}

func (p *authProviderStub) AccessToken(context.Context, entities.AuthOwner, auth.OAuth2Config) (string, error) {
	panic("AccessToken not expected")
}

func (p *authProviderStub) Peek(_ context.Context, owner entities.AuthOwner, cfg auth.OAuth2Config) (string, bool, error) {
	p.owners = append(p.owners, owner)
	p.peekCfg = append(p.peekCfg, cfg)
	return "", false, nil
}

func (p *authProviderStub) Status(_ context.Context, owner entities.AuthOwner, cfg auth.OAuth2Config) (auth.TokenStatus, error) {
	p.owners = append(p.owners, owner)
	p.statusCfg = append(p.statusCfg, cfg)
	return p.status, p.statusErr
}

func (p *authProviderStub) Fetch(_ context.Context, owner entities.AuthOwner, _ auth.OAuth2Config) (*auth.Token, error) {
	p.owners = append(p.owners, owner)
	p.fetches++
	if p.fetchErr != nil {
		return nil, p.fetchErr
	}
	return &auth.Token{AccessToken: "at"}, nil
}

func (p *authProviderStub) Clear(_ context.Context, owner entities.AuthOwner) error {
	p.cleared = append(p.cleared, owner)
	return nil
}

// authRequestRepoStub serves GetByID; the rest of request.Repository is not part
// of the auth service's job and panics if something reaches for it.
type authRequestRepoStub struct {
	byID map[uuid.UUID]*entities.Request
}

func (s *authRequestRepoStub) GetByID(_ context.Context, id uuid.UUID) (*entities.Request, error) {
	return s.byID[id], nil
}

func (s *authRequestRepoStub) Create(context.Context, *entities.Request) error { panic("not used") }

func (s *authRequestRepoStub) List(context.Context, request.Filter) ([]*entities.Request, error) {
	panic("not used")
}

func (s *authRequestRepoStub) Update(context.Context, *entities.Request) error { panic("not used") }

func (s *authRequestRepoStub) GetDescriptionByID(context.Context, uuid.UUID) (string, error) {
	panic("not used")
}

func (s *authRequestRepoStub) UpdateSortOrder(context.Context, uuid.UUID, int) error {
	panic("not used")
}

func (s *authRequestRepoStub) DeleteHard(context.Context, uuid.UUID) error { panic("not used") }

func (s *authRequestRepoStub) CleanupDrafts(context.Context) (int, error) { panic("not used") }

type authCollectionReaderStub struct {
	byID map[uuid.UUID]*entities.Collection
}

func (s *authCollectionReaderStub) GetByID(_ context.Context, id uuid.UUID) (*entities.Collection, error) {
	return s.byID[id], nil
}

func (s *authCollectionReaderStub) ListByWorkspace(context.Context, uuid.UUID) ([]*entities.Collection, error) {
	panic("not used")
}

type authEnvStub struct{ vars map[string]string }

func (s *authEnvStub) ResolveVariables(context.Context, uuid.UUID) (map[string]string, error) {
	return s.vars, nil
}

type authScriptResolverStub struct{}

func (authScriptResolverStub) ResolvePreScript(context.Context, *entities.Request) (string, error) {
	return "", nil
}

func (authScriptResolverStub) ResolvePostScript(context.Context, *entities.Request) (string, error) {
	return "", nil
}

const authFixtureData = `{"grant":"client_credentials","tokenUrl":"{{idp}}/token","clientId":"cid","clientSecret":"sec","scope":"read"}`

type authFixture struct {
	svc      *AuthService
	provider *authProviderStub
	flows    *flowManagerStub
	sink     *AuthFlowEventSink
	requests *authRequestRepoStub
	colls    *authCollectionReaderStub
	env      *authEnvStub
	req      *entities.Request
	coll     *entities.Collection
}

// newAuthFixture builds a request with its own oauth2 config under one collection.
func newAuthFixture(t *testing.T) *authFixture {
	t.Helper()

	coll := fixtureCollection("api", 1)
	req := fixtureRequest("get-users", 1)
	req.CollectionID = coll.ID
	req.AuthType = entities.AuthTypeOAuth2
	req.AuthData = authFixtureData

	f := &authFixture{
		provider: &authProviderStub{status: auth.TokenStatus{State: auth.TokenStateNone}},
		requests: &authRequestRepoStub{byID: map[uuid.UUID]*entities.Request{req.ID: req}},
		colls:    &authCollectionReaderStub{byID: map[uuid.UUID]*entities.Collection{coll.ID: coll}},
		env:      &authEnvStub{vars: map[string]string{"idp": "https://idp.example"}},
		req:      req,
		coll:     coll,
	}
	f.flows = &flowManagerStub{}
	f.sink = NewAuthFlowEventSink()
	f.svc = NewAuthService(f.provider, f.flows, f.sink, f.requests, f.colls, request.NewAuthResolver(f.colls), f.env)

	return f
}

func (f *authFixture) startRequest(flowID string) dto.StartFlowRequest {
	return dto.StartFlowRequest{
		OwnerKind: entities.AuthOwnerKindRequest,
		OwnerID:   f.req.ID.String(),
		AuthType:  string(entities.AuthTypeOAuth2),
		AuthData:  f.req.AuthData,
		FlowID:    flowID,
	}
}

func (f *authFixture) requestConfig() dto.AuthConfigRequest {
	return dto.AuthConfigRequest{
		OwnerKind: entities.AuthOwnerKindRequest,
		OwnerID:   f.req.ID.String(),
		AuthType:  string(entities.AuthTypeOAuth2),
		AuthData:  f.req.AuthData,
	}
}

func TestAuthService_TokenStatus_None(t *testing.T) {
	f := newAuthFixture(t)

	res := f.svc.TokenStatus(f.requestConfig())

	require.Nil(t, res.Error)
	assert.Equal(t, auth.TokenStateNone, res.Data.State)
	assert.Empty(t, res.Data.ExpiresAt)
}

func TestAuthService_TokenStatus_Valid(t *testing.T) {
	f := newAuthFixture(t)
	expires := time.Date(2026, 9, 4, 14, 5, 0, 0, time.UTC)
	f.provider.status = auth.TokenStatus{State: auth.TokenStateValid, ExpiresAt: expires}

	res := f.svc.TokenStatus(f.requestConfig())

	require.Nil(t, res.Error)
	assert.Equal(t, auth.TokenStateValid, res.Data.State)
	assert.Equal(t, "2026-09-04T14:05:00Z", res.Data.ExpiresAt)
}

func TestAuthService_TokenStatus_Expired(t *testing.T) {
	f := newAuthFixture(t)
	f.provider.status = auth.TokenStatus{State: auth.TokenStateExpired}

	res := f.svc.TokenStatus(f.requestConfig())

	require.Nil(t, res.Error)
	assert.Equal(t, auth.TokenStateExpired, res.Data.State)
}

func TestAuthService_TokenStatus_OwnerCarriesCollectionWorkspace(t *testing.T) {
	f := newAuthFixture(t)

	res := f.svc.TokenStatus(f.requestConfig())

	require.Nil(t, res.Error)
	require.Len(t, f.provider.owners, 1)
	assert.Equal(t, entities.AuthOwner{
		WorkspaceID: f.coll.WorkspaceID,
		Kind:        entities.AuthOwnerKindRequest,
		ID:          f.req.ID,
	}, f.provider.owners[0])
}

func TestAuthService_TokenStatus_SubstitutesVariables(t *testing.T) {
	f := newAuthFixture(t)

	res := f.svc.TokenStatus(f.requestConfig())

	require.Nil(t, res.Error)
	require.Len(t, f.provider.statusCfg, 1)
	assert.Equal(t, "https://idp.example/token", f.provider.statusCfg[0].TokenURL)
}

func TestAuthService_TokenStatus_NonOAuth2IsNone(t *testing.T) {
	f := newAuthFixture(t)
	cfg := f.requestConfig()
	cfg.AuthType = string(entities.AuthTypeBasic)

	res := f.svc.TokenStatus(cfg)

	require.Nil(t, res.Error)
	assert.Equal(t, auth.TokenStateNone, res.Data.State)
	assert.Empty(t, f.provider.owners)
}

func TestAuthService_TokenStatus_UnknownOwner(t *testing.T) {
	f := newAuthFixture(t)
	cfg := f.requestConfig()
	cfg.OwnerID = uuid.New().String()

	res := f.svc.TokenStatus(cfg)

	require.NotNil(t, res.Error)
	assert.Equal(t, ErrCodeNotFound, res.Error.Code)
}

func TestAuthService_TokenStatus_InvalidOwnerKind(t *testing.T) {
	f := newAuthFixture(t)
	cfg := f.requestConfig()
	cfg.OwnerKind = "workspace"

	res := f.svc.TokenStatus(cfg)

	require.NotNil(t, res.Error)
	assert.Equal(t, ErrCodeValidation, res.Error.Code)
	assert.Contains(t, res.Error.Fields, "ownerKind")
}

func TestAuthService_TokenStatus_InvalidAuthData(t *testing.T) {
	f := newAuthFixture(t)
	cfg := f.requestConfig()
	cfg.AuthData = "{not json"

	res := f.svc.TokenStatus(cfg)

	require.NotNil(t, res.Error)
	assert.Equal(t, ErrCodeValidation, res.Error.Code)
	assert.Contains(t, res.Error.Fields, "authData")
}

func TestAuthService_FetchToken_Happy(t *testing.T) {
	f := newAuthFixture(t)
	expires := time.Date(2026, 9, 4, 15, 0, 0, 0, time.UTC)
	f.provider.status = auth.TokenStatus{State: auth.TokenStateValid, ExpiresAt: expires}

	res := f.svc.FetchToken(f.requestConfig())

	require.Nil(t, res.Error)
	assert.Equal(t, 1, f.provider.fetches)
	assert.Equal(t, auth.TokenStateValid, res.Data.State)
	assert.Equal(t, "2026-09-04T15:00:00Z", res.Data.ExpiresAt)
}

func TestAuthService_FetchToken_CollectionOwner(t *testing.T) {
	f := newAuthFixture(t)
	f.coll.AuthType = entities.AuthTypeOAuth2
	f.coll.AuthData = authFixtureData

	res := f.svc.FetchToken(dto.AuthConfigRequest{
		OwnerKind: entities.AuthOwnerKindCollection,
		OwnerID:   f.coll.ID.String(),
		AuthType:  string(entities.AuthTypeOAuth2),
		AuthData:  f.coll.AuthData,
	})

	require.Nil(t, res.Error)
	require.NotEmpty(t, f.provider.owners)
	assert.Equal(t, entities.AuthOwner{
		WorkspaceID: f.coll.WorkspaceID,
		Kind:        entities.AuthOwnerKindCollection,
		ID:          f.coll.ID,
	}, f.provider.owners[0])
}

func TestAuthService_FetchToken_InteractiveGrant(t *testing.T) {
	f := newAuthFixture(t)
	f.provider.fetchErr = auth.ErrTokenRequired

	res := f.svc.FetchToken(f.requestConfig())

	require.NotNil(t, res.Error)
	assert.Equal(t, ErrCodeValidation, res.Error.Code)
	assert.Contains(t, res.Error.Fields, "auth")
}

func TestAuthService_FetchToken_OwnerGone(t *testing.T) {
	f := newAuthFixture(t)
	f.provider.fetchErr = auth.ErrOwnerGone

	res := f.svc.FetchToken(f.requestConfig())

	require.NotNil(t, res.Error)
	assert.Equal(t, ErrCodeNotFound, res.Error.Code)
}

func TestAuthService_FetchToken_StaleToken(t *testing.T) {
	f := newAuthFixture(t)
	f.provider.fetchErr = auth.ErrStaleToken

	res := f.svc.FetchToken(f.requestConfig())

	require.NotNil(t, res.Error)
	assert.Equal(t, ErrCodeValidation, res.Error.Code)
}

func TestAuthService_FetchToken_RejectsNonOAuth2(t *testing.T) {
	f := newAuthFixture(t)
	cfg := f.requestConfig()
	cfg.AuthType = string(entities.AuthTypeJWT)

	res := f.svc.FetchToken(cfg)

	require.NotNil(t, res.Error)
	assert.Equal(t, ErrCodeValidation, res.Error.Code)
	assert.Zero(t, f.provider.fetches)
}

func TestAuthService_ClearToken_Happy(t *testing.T) {
	f := newAuthFixture(t)

	res := f.svc.ClearToken(f.requestConfig())

	require.Nil(t, res.Error)
	require.Len(t, f.provider.cleared, 1)
	assert.Equal(t, f.req.ID, f.provider.cleared[0].ID)
	assert.Equal(t, f.coll.WorkspaceID, f.provider.cleared[0].WorkspaceID)
}

func TestAuthService_ClearToken_AfterTypeSwitch(t *testing.T) {
	f := newAuthFixture(t)
	cfg := f.requestConfig()
	cfg.AuthType = string(entities.AuthTypeNone)

	res := f.svc.ClearToken(cfg)

	require.Nil(t, res.Error)
	assert.Len(t, f.provider.cleared, 1)
}

func TestAuthService_ClearToken_InvalidOwnerID(t *testing.T) {
	f := newAuthFixture(t)
	cfg := f.requestConfig()
	cfg.OwnerID = "nope"

	res := f.svc.ClearToken(cfg)

	require.NotNil(t, res.Error)
	assert.Equal(t, ErrCodeValidation, res.Error.Code)
	assert.Contains(t, res.Error.Fields, "ownerId")
	assert.Empty(t, f.provider.cleared)
}

func TestAuthService_ResolveOwner_OwnAuth(t *testing.T) {
	f := newAuthFixture(t)

	res := f.svc.ResolveOwner(dto.ResolveOwnerRequest{RequestID: f.req.ID.String()})

	require.Nil(t, res.Error)
	assert.Equal(t, entities.AuthOwnerKindRequest, res.Data.OwnerKind)
	assert.Equal(t, f.req.ID.String(), res.Data.OwnerID)
	assert.Equal(t, string(entities.AuthTypeOAuth2), res.Data.AuthType)
	assert.Equal(t, authFixtureData, res.Data.AuthData)
}

func TestAuthService_ResolveOwner_InheritFindsAncestor(t *testing.T) {
	f := newAuthFixture(t)
	root := fixtureCollection("root", 1)
	root.WorkspaceID = f.coll.WorkspaceID
	root.AuthType = entities.AuthTypeOAuth2
	root.AuthData = authFixtureData
	f.coll.ParentID = &root.ID
	f.colls.byID[root.ID] = root
	f.req.AuthType = entities.AuthTypeInherit
	f.req.AuthData = "{}"

	res := f.svc.ResolveOwner(dto.ResolveOwnerRequest{RequestID: f.req.ID.String()})

	require.Nil(t, res.Error)
	assert.Equal(t, entities.AuthOwnerKindCollection, res.Data.OwnerKind)
	assert.Equal(t, root.ID.String(), res.Data.OwnerID)
	assert.Equal(t, string(entities.AuthTypeOAuth2), res.Data.AuthType)
	assert.Equal(t, authFixtureData, res.Data.AuthData)
}

func TestAuthService_ResolveOwner_InheritWithoutConfiguredAncestor(t *testing.T) {
	f := newAuthFixture(t)
	f.req.AuthType = entities.AuthTypeInherit
	f.req.AuthData = "{}"

	res := f.svc.ResolveOwner(dto.ResolveOwnerRequest{RequestID: f.req.ID.String()})

	require.Nil(t, res.Error)
	assert.Equal(t, string(entities.AuthTypeNone), res.Data.AuthType)
	assert.Empty(t, res.Data.OwnerKind)
	assert.Empty(t, res.Data.OwnerID)
}

func TestAuthService_ResolveOwner_UnknownRequest(t *testing.T) {
	f := newAuthFixture(t)

	res := f.svc.ResolveOwner(dto.ResolveOwnerRequest{RequestID: uuid.New().String()})

	require.NotNil(t, res.Error)
	assert.Equal(t, ErrCodeNotFound, res.Error.Code)
}

func TestAuthService_ResolveOwner_MissingCollection(t *testing.T) {
	f := newAuthFixture(t)
	delete(f.colls.byID, f.coll.ID)

	res := f.svc.ResolveOwner(dto.ResolveOwnerRequest{RequestID: f.req.ID.String()})

	require.NotNil(t, res.Error)
	assert.Equal(t, ErrCodeNotFound, res.Error.Code)
}

// The Auth tab must address the same cached token the send path does: drive both
// pipelines over one request and compare the configuration hash they produce.
func TestAuthService_ConfigHashMatchesSendPath(t *testing.T) {
	f := newAuthFixture(t)

	uc := request.NewUsecase(
		f.requests, nil, nil, nil, nil,
		f.env, nil, authScriptResolverStub{}, nil,
		request.NewAuthResolver(f.colls), nil, f.colls, nil, f.provider,
	)

	curl, err := uc.BuildCurl(context.Background(), f.req.ID, request.BuildCurlOpt{WorkspaceID: f.coll.WorkspaceID})
	require.NoError(t, err)
	require.NotEmpty(t, curl.Warnings, "no cached token: the send path must report it")
	require.Len(t, f.provider.peekCfg, 1)

	res := f.svc.TokenStatus(f.requestConfig())
	require.Nil(t, res.Error)
	require.Len(t, f.provider.statusCfg, 1)

	assert.Equal(t, f.provider.peekCfg[0], f.provider.statusCfg[0])
	assert.Equal(t, auth.ConfigHash(f.provider.peekCfg[0]), auth.ConfigHash(f.provider.statusCfg[0]))
	require.Len(t, f.provider.owners, 2)
	assert.Equal(t, f.provider.owners[0], f.provider.owners[1])
}

func TestAuthService_FetchToken_UnexpectedErrorIsInternal(t *testing.T) {
	f := newAuthFixture(t)
	f.provider.fetchErr = errors.New("idp unreachable")

	res := f.svc.FetchToken(f.requestConfig())

	require.NotNil(t, res.Error)
	assert.Equal(t, ErrCodeInternal, res.Error.Code)
}

// flowManagerStub records what the service handed the manager and replays a
// canned answer; the manager's own behaviour is covered in usecase/auth.
type flowManagerStub struct {
	info      auth.FlowInfo
	startErr  error
	status    auth.FlowStatus
	statusOK  bool
	cancelErr error

	codeStarts   []flowStart
	deviceStarts []flowStart
	statusIDs    []string
	cancelled    []string
}

type flowStart struct {
	flowID string
	owner  entities.AuthOwner
	cfg    auth.OAuth2Config
	port   string
}

func (m *flowManagerStub) StartAuthCode(_ context.Context, flowID string, owner entities.AuthOwner,
	cfg auth.OAuth2Config, redirectPort string,
) (auth.FlowInfo, error) {
	m.codeStarts = append(m.codeStarts, flowStart{flowID: flowID, owner: owner, cfg: cfg, port: redirectPort})
	if m.startErr != nil {
		return auth.FlowInfo{}, m.startErr
	}

	return m.info, nil
}

func (m *flowManagerStub) StartDevice(_ context.Context, flowID string, owner entities.AuthOwner,
	cfg auth.OAuth2Config,
) (auth.FlowInfo, error) {
	m.deviceStarts = append(m.deviceStarts, flowStart{flowID: flowID, owner: owner, cfg: cfg})
	if m.startErr != nil {
		return auth.FlowInfo{}, m.startErr
	}

	return m.info, nil
}

func (m *flowManagerStub) Status(flowID string) (auth.FlowStatus, bool) {
	m.statusIDs = append(m.statusIDs, flowID)
	return m.status, m.statusOK
}

func (m *flowManagerStub) Cancel(flowID string) error {
	m.cancelled = append(m.cancelled, flowID)
	return m.cancelErr
}

func (m *flowManagerStub) Shutdown(context.Context) error { return nil }

const authCodeFixtureData = `{"grant":"authorization_code","tokenUrl":"{{idp}}/token","authUrl":"{{idp}}/authorize",` +
	`"clientId":"cid","clientAuth":"none","redirectPort":"{{port}}"}`

const deviceFixtureData = `{"grant":"device_code","tokenUrl":"{{idp}}/token","deviceAuthUrl":"{{idp}}/device",` +
	`"clientId":"cid","clientAuth":"none"}`

func TestAuthService_StartAuthCodeFlow_ReturnsInfo(t *testing.T) {
	f := newAuthFixture(t)
	f.req.AuthData = authCodeFixtureData
	f.env.vars["port"] = "31000"
	f.flows.info = auth.FlowInfo{
		ID:           "flow-1",
		AuthorizeURL: "https://idp.example/authorize?state=s",
		ExpiresAt:    time.Date(2026, 9, 5, 12, 0, 0, 0, time.UTC),
	}
	id := uuid.NewString()

	res := f.svc.StartAuthCodeFlow(f.startRequest(id))

	require.Nil(t, res.Error)
	assert.Equal(t, "flow-1", res.Data.FlowID)
	assert.Equal(t, "https://idp.example/authorize?state=s", res.Data.AuthorizeURL)
	assert.Equal(t, "2026-09-05T12:00:00Z", res.Data.ExpiresAt)
	require.Len(t, f.flows.codeStarts, 1)
	assert.Equal(t, id, f.flows.codeStarts[0].flowID)
	assert.Equal(t, "31000", f.flows.codeStarts[0].port, "redirectPort comes from the substituted document")
	assert.Equal(t, "https://idp.example/authorize", f.flows.codeStarts[0].cfg.AuthURL)
	assert.Equal(t, entities.AuthOwner{
		WorkspaceID: f.coll.WorkspaceID,
		Kind:        entities.AuthOwnerKindRequest,
		ID:          f.req.ID,
	}, f.flows.codeStarts[0].owner)
}

func TestAuthService_StartDeviceFlow_ReturnsInfo(t *testing.T) {
	f := newAuthFixture(t)
	f.req.AuthData = deviceFixtureData
	f.flows.info = auth.FlowInfo{
		ID:                      "flow-2",
		UserCode:                "WDJB-MJHT",
		VerificationURI:         "https://idp.example/device",
		VerificationURIComplete: "https://idp.example/device?user_code=WDJB-MJHT",
		Interval:                5 * time.Second,
	}
	id := uuid.NewString()

	res := f.svc.StartDeviceFlow(f.startRequest(id))

	require.Nil(t, res.Error)
	assert.Equal(t, "WDJB-MJHT", res.Data.UserCode)
	assert.Equal(t, "https://idp.example/device", res.Data.VerificationURI)
	assert.Equal(t, "https://idp.example/device?user_code=WDJB-MJHT", res.Data.VerificationURIComplete)
	assert.Equal(t, 5, res.Data.IntervalSec)
	assert.Empty(t, res.Data.ExpiresAt)
	require.Len(t, f.flows.deviceStarts, 1)
	assert.Equal(t, id, f.flows.deviceStarts[0].flowID)
	assert.Equal(t, "https://idp.example/device", f.flows.deviceStarts[0].cfg.DeviceAuthURL)
	assert.Empty(t, f.flows.codeStarts)
}

func TestAuthService_StartFlow_RejectsNonUUIDFlowID(t *testing.T) {
	f := newAuthFixture(t)
	f.req.AuthData = authCodeFixtureData

	res := f.svc.StartAuthCodeFlow(f.startRequest("not-a-uuid"))

	require.NotNil(t, res.Error)
	assert.Equal(t, ErrCodeValidation, res.Error.Code)
	assert.Contains(t, res.Error.Fields, "flowId")
	assert.Empty(t, f.flows.codeStarts)
}

func TestAuthService_StartFlow_RejectsNonOAuth2(t *testing.T) {
	f := newAuthFixture(t)
	req := f.startRequest(uuid.NewString())
	req.AuthType = string(entities.AuthTypeJWT)

	res := f.svc.StartDeviceFlow(req)

	require.NotNil(t, res.Error)
	assert.Equal(t, ErrCodeValidation, res.Error.Code)
	assert.Contains(t, res.Error.Fields, "authType")
	assert.Empty(t, f.flows.deviceStarts)
}

// The manager's field-keyed ValidationErrors must reach the form untouched.
func TestAuthService_StartFlow_ValidationErrorCarriesFields(t *testing.T) {
	f := newAuthFixture(t)
	f.req.AuthData = authCodeFixtureData
	f.flows.startErr = auth.ValidateFlowConfig(auth.OAuth2Config{
		Grant:      auth.GrantAuthorizationCode,
		TokenURL:   "https://idp.example/token",
		ClientID:   "cid",
		ClientAuth: auth.ClientAuthBasic,
	}, auth.GrantAuthorizationCode)
	require.Error(t, f.flows.startErr)

	res := f.svc.StartAuthCodeFlow(f.startRequest(uuid.NewString()))

	require.NotNil(t, res.Error)
	assert.Equal(t, ErrCodeValidation, res.Error.Code)
	assert.Contains(t, res.Error.Fields, "authUrl")
}

func TestAuthService_StartFlow_BadClientAuthKeepsFieldKey(t *testing.T) {
	f := newAuthFixture(t)
	f.req.AuthData = authCodeFixtureData
	f.flows.startErr = auth.ValidateFlowConfig(auth.OAuth2Config{
		Grant:      auth.GrantAuthorizationCode,
		TokenURL:   "https://idp.example/token",
		AuthURL:    "https://idp.example/authorize",
		ClientID:   "cid",
		ClientAuth: "mtls",
	}, auth.GrantAuthorizationCode)

	res := f.svc.StartAuthCodeFlow(f.startRequest(uuid.NewString()))

	require.NotNil(t, res.Error)
	assert.Equal(t, ErrCodeValidation, res.Error.Code)
	assert.Contains(t, res.Error.Fields, "clientAuth")
}

func TestAuthService_StartFlow_OwnerGone(t *testing.T) {
	f := newAuthFixture(t)
	f.req.AuthData = deviceFixtureData
	f.flows.startErr = auth.ErrOwnerGone

	res := f.svc.StartDeviceFlow(f.startRequest(uuid.NewString()))

	require.NotNil(t, res.Error)
	assert.Equal(t, ErrCodeNotFound, res.Error.Code)
}

func TestAuthService_StartFlow_InvalidAuthData(t *testing.T) {
	f := newAuthFixture(t)
	req := f.startRequest(uuid.NewString())
	req.AuthData = "{nope"

	res := f.svc.StartAuthCodeFlow(req)

	require.NotNil(t, res.Error)
	assert.Equal(t, ErrCodeValidation, res.Error.Code)
	assert.Contains(t, res.Error.Fields, "authData")
	assert.Empty(t, f.flows.codeStarts)
}

func TestAuthService_FlowStatus_UnknownIDIsEmptyState(t *testing.T) {
	f := newAuthFixture(t)
	id := uuid.NewString()

	res := f.svc.FlowStatus(dto.FlowRequest{FlowID: id})

	require.Nil(t, res.Error)
	assert.Empty(t, res.Data.State)
	assert.Empty(t, res.Data.Error)
	assert.Equal(t, []string{id}, f.flows.statusIDs)
}

func TestAuthService_FlowStatus_TerminalCarriesInfo(t *testing.T) {
	f := newAuthFixture(t)
	f.flows.statusOK = true
	f.flows.status = auth.FlowStatus{
		State: auth.FlowError,
		Info: auth.FlowInfo{
			ID:              "flow-3",
			UserCode:        "WDJB-MJHT",
			VerificationURI: "https://idp.example/device",
			Interval:        7 * time.Second,
			ExpiresAt:       time.Date(2026, 9, 5, 13, 30, 0, 0, time.UTC),
		},
		Err: errors.New("the token was cleared while the flow was running"),
	}

	res := f.svc.FlowStatus(dto.FlowRequest{FlowID: uuid.NewString()})

	require.Nil(t, res.Error)
	assert.Equal(t, string(auth.FlowError), res.Data.State)
	assert.Equal(t, "the token was cleared while the flow was running", res.Data.Error)
	assert.Equal(t, "flow-3", res.Data.Info.FlowID)
	assert.Equal(t, "WDJB-MJHT", res.Data.Info.UserCode)
	assert.Equal(t, 7, res.Data.Info.IntervalSec)
	assert.Equal(t, "2026-09-05T13:30:00Z", res.Data.Info.ExpiresAt)
}

func TestAuthService_FlowStatus_RejectsNonUUID(t *testing.T) {
	f := newAuthFixture(t)

	res := f.svc.FlowStatus(dto.FlowRequest{FlowID: "nope"})

	require.NotNil(t, res.Error)
	assert.Equal(t, ErrCodeValidation, res.Error.Code)
	assert.Contains(t, res.Error.Fields, "flowId")
	assert.Empty(t, f.flows.statusIDs)
}

func TestAuthService_CancelFlow_UnknownIDIsOK(t *testing.T) {
	f := newAuthFixture(t)
	id := uuid.NewString()

	res := f.svc.CancelFlow(dto.FlowRequest{FlowID: id})

	require.Nil(t, res.Error)
	assert.Equal(t, []string{id}, f.flows.cancelled)
}

func TestAuthService_CancelFlow_RejectsNonUUID(t *testing.T) {
	f := newAuthFixture(t)

	res := f.svc.CancelFlow(dto.FlowRequest{FlowID: "nope"})

	require.NotNil(t, res.Error)
	assert.Equal(t, ErrCodeValidation, res.Error.Code)
	assert.Contains(t, res.Error.Fields, "flowId")
	assert.Empty(t, f.flows.cancelled)
}

func TestAuthService_CancelFlow_SurvivorSurfaces(t *testing.T) {
	f := newAuthFixture(t)
	f.flows.cancelErr = errors.New("auth: the token request did not stop within 5s")

	res := f.svc.CancelFlow(dto.FlowRequest{FlowID: uuid.NewString()})

	require.NotNil(t, res.Error)
	assert.Equal(t, ErrCodeInternal, res.Error.Code)
}

func TestAuthService_SetEventEmitterFeedsTheSink(t *testing.T) {
	f := newAuthFixture(t)
	var name string
	f.svc.SetEventEmitter(func(n string, _ any) { name = n })

	f.sink.OnFlowState("flow-9", auth.FlowPending, nil)

	assert.Equal(t, "auth:flow:flow-9", name)
}
