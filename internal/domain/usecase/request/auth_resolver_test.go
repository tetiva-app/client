package request_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tetiva-app/client/internal/domain"
	"github.com/tetiva-app/client/internal/domain/entities"
	"github.com/tetiva-app/client/internal/domain/usecase/request"
)

func TestAuthResolver_RequestHasOwnAuth(t *testing.T) {
	reqID := uuid.New()
	resolver := request.NewAuthResolver(fixtureCollections())
	req := &entities.Request{
		ID:           reqID,
		CollectionID: testCollectionID,
		AuthType:     entities.AuthTypeBasic,
		AuthData:     `{"username":"u","password":"p"}`,
	}

	ra, err := resolver.ResolveAuth(context.Background(), req)
	require.NoError(t, err)
	assert.Equal(t, entities.AuthTypeBasic, ra.Type)
	assert.Equal(t, `{"username":"u","password":"p"}`, ra.Data)
	assert.Equal(t, entities.AuthOwner{
		WorkspaceID: testWorkspaceID,
		Kind:        entities.AuthOwnerKindRequest,
		ID:          reqID,
	}, ra.Owner)
}

func TestAuthResolver_InheritFromCollection(t *testing.T) {
	collID := uuid.New()
	workspaceID := uuid.New()
	reader := &mockCollectionReader{
		collections: map[uuid.UUID]*entities.Collection{
			collID: {
				ID:          collID,
				WorkspaceID: workspaceID,
				AuthType:    entities.AuthTypeBearer,
				AuthData:    `{"token":"abc"}`,
			},
		},
	}
	resolver := request.NewAuthResolver(reader)
	req := &entities.Request{
		ID:           uuid.New(),
		CollectionID: collID,
		AuthType:     entities.AuthTypeInherit,
	}

	ra, err := resolver.ResolveAuth(context.Background(), req)
	require.NoError(t, err)
	assert.Equal(t, entities.AuthTypeBearer, ra.Type)
	assert.Equal(t, `{"token":"abc"}`, ra.Data)
	assert.Equal(t, entities.AuthOwner{
		WorkspaceID: workspaceID,
		Kind:        entities.AuthOwnerKindCollection,
		ID:          collID,
	}, ra.Owner)
}

func TestAuthResolver_InheritWalksUpToParent(t *testing.T) {
	rootID := uuid.New()
	childID := uuid.New()
	workspaceID := uuid.New()
	reader := &mockCollectionReader{
		collections: map[uuid.UUID]*entities.Collection{
			childID: {ID: childID, WorkspaceID: workspaceID, ParentID: &rootID, AuthType: entities.AuthTypeNone, AuthData: "{}"},
			rootID:  {ID: rootID, WorkspaceID: workspaceID, AuthType: entities.AuthTypeAPIKey, AuthData: `{"key":"X-Key","value":"secret","addTo":"header"}`},
		},
	}
	resolver := request.NewAuthResolver(reader)
	req := &entities.Request{
		ID:           uuid.New(),
		CollectionID: childID,
		AuthType:     entities.AuthTypeInherit,
	}

	ra, err := resolver.ResolveAuth(context.Background(), req)
	require.NoError(t, err)
	assert.Equal(t, entities.AuthTypeAPIKey, ra.Type)
	assert.Contains(t, ra.Data, "X-Key")
	assert.Equal(t, entities.AuthOwner{
		WorkspaceID: workspaceID,
		Kind:        entities.AuthOwnerKindCollection,
		ID:          rootID,
	}, ra.Owner)
}

// Nothing stops a collection tree from spanning workspaces (create, move and
// inbound sync all accept a foreign parent), so the owner keeps its own workspace.
func TestAuthResolver_InheritOwnerKeepsItsOwnWorkspace(t *testing.T) {
	rootID := uuid.New()
	childID := uuid.New()
	rootWorkspace := uuid.New()
	reader := &mockCollectionReader{
		collections: map[uuid.UUID]*entities.Collection{
			childID: {ID: childID, WorkspaceID: testWorkspaceID, ParentID: &rootID, AuthType: entities.AuthTypeNone, AuthData: "{}"},
			rootID:  {ID: rootID, WorkspaceID: rootWorkspace, AuthType: entities.AuthTypeBearer, AuthData: `{"token":"abc"}`},
		},
	}
	resolver := request.NewAuthResolver(reader)
	req := &entities.Request{ID: uuid.New(), CollectionID: childID, AuthType: entities.AuthTypeInherit}

	ra, err := resolver.ResolveAuth(context.Background(), req)
	require.NoError(t, err)
	assert.Equal(t, entities.AuthOwner{
		WorkspaceID: rootWorkspace,
		Kind:        entities.AuthOwnerKindCollection,
		ID:          rootID,
	}, ra.Owner)
}

func TestAuthResolver_InheritNoAuthInChain(t *testing.T) {
	collID := uuid.New()
	workspaceID := uuid.New()
	reader := &mockCollectionReader{
		collections: map[uuid.UUID]*entities.Collection{
			collID: {ID: collID, WorkspaceID: workspaceID, AuthType: entities.AuthTypeNone, AuthData: "{}"},
		},
	}
	resolver := request.NewAuthResolver(reader)
	req := &entities.Request{
		ID:           uuid.New(),
		CollectionID: collID,
		AuthType:     entities.AuthTypeInherit,
	}

	ra, err := resolver.ResolveAuth(context.Background(), req)
	require.NoError(t, err)
	assert.Equal(t, entities.AuthTypeNone, ra.Type)
	assert.Equal(t, "{}", ra.Data)
	assert.Equal(t, workspaceID, ra.Owner.WorkspaceID, "the workspace guard needs it even without an owner")
	assert.Empty(t, ra.Owner.Kind)
	assert.Equal(t, uuid.Nil, ra.Owner.ID)
}

// A soft-deleted or missing collection reads as nil; running under it would use
// another workspace's variables, cookies and tokens.
func TestAuthResolver_MissingCollectionIsNotFound(t *testing.T) {
	resolver := request.NewAuthResolver(&mockCollectionReader{})
	for _, at := range []entities.AuthType{entities.AuthTypeInherit, entities.AuthTypeBasic} {
		req := &entities.Request{ID: uuid.New(), CollectionID: uuid.New(), AuthType: at}

		_, err := resolver.ResolveAuth(context.Background(), req)

		var notFound *domain.NotFoundError
		require.ErrorAs(t, err, &notFound, "auth type %q", at)
		assert.Equal(t, "collection", notFound.Entity)
	}
}

func TestAuthResolver_BrokenParentLinkEndsChain(t *testing.T) {
	collID := uuid.New()
	goneID := uuid.New()
	reader := &mockCollectionReader{
		collections: map[uuid.UUID]*entities.Collection{
			collID: {ID: collID, WorkspaceID: testWorkspaceID, ParentID: &goneID, AuthType: entities.AuthTypeNone, AuthData: "{}"},
		},
	}
	resolver := request.NewAuthResolver(reader)
	req := &entities.Request{ID: uuid.New(), CollectionID: collID, AuthType: entities.AuthTypeInherit}

	ra, err := resolver.ResolveAuth(context.Background(), req)
	require.NoError(t, err)
	assert.Equal(t, entities.AuthTypeNone, ra.Type)
	assert.Equal(t, testWorkspaceID, ra.Owner.WorkspaceID)
}

func TestAuthResolver_MaxDepthGuard(t *testing.T) {
	collections := make(map[uuid.UUID]*entities.Collection)
	var prevID *uuid.UUID
	var bottomID uuid.UUID
	for i := 0; i < 60; i++ {
		id := uuid.New()
		c := &entities.Collection{ID: id, WorkspaceID: testWorkspaceID, ParentID: prevID, AuthType: entities.AuthTypeNone, AuthData: "{}"}
		collections[id] = c
		prevID = &id
		if i == 0 {
			bottomID = id
		}
	}

	resolver := request.NewAuthResolver(&mockCollectionReader{collections: collections})
	req := &entities.Request{
		ID:           uuid.New(),
		CollectionID: bottomID,
		AuthType:     entities.AuthTypeInherit,
	}

	ra, err := resolver.ResolveAuth(context.Background(), req)
	require.NoError(t, err)
	assert.Equal(t, entities.AuthTypeNone, ra.Type)
}

// newGuardUsecase wires the collaborators the resolve-first stage touches; the
// requesters stay nil so a rejected request cannot reach the wire.
func newGuardUsecase(repo *mockRepo, env *mockEnvResolver) request.Usecase {
	return newGuardUsecaseWith(repo, env, fixtureCollections())
}

func newGuardUsecaseWith(repo *mockRepo, env *mockEnvResolver, reader request.CollectionReader) request.Usecase {
	return request.NewUsecase(repo, &mockHistoryRepo{}, nil, nil, nil,
		env, &noopScriptEngine{}, &noopScriptResolver{}, &noopVarPersister{},
		request.NewAuthResolver(reader), nil, nil, nil, nil)
}

func guardRequest(id uuid.UUID, protocol entities.Protocol, authType entities.AuthType) *entities.Request {
	url := "https://api.example.com/x"
	if protocol == entities.ProtocolWebSocket {
		url = "wss://api.example.com/ws"
	}
	return &entities.Request{
		ID: id, CollectionID: testCollectionID, Protocol: protocol,
		Method: entities.MethodGET, URL: url, BodyType: entities.BodyTypeNone,
		AuthType: authType, AuthData: `{"username":"u","password":"p"}`,
	}
}

// Every path resolves auth right after loading the request, so a tab left open on
// another workspace never reaches its variables, scripts or cookies.
func TestResolveFirst_WorkspaceMismatchRejectedBeforeVariables(t *testing.T) {
	otherWorkspace := uuid.New()
	paths := []struct {
		name     string
		protocol entities.Protocol
		run      func(uc request.Usecase, id uuid.UUID) error
	}{
		{"execute http", entities.ProtocolHTTP, func(uc request.Usecase, id uuid.UUID) error {
			_, err := uc.Execute(context.Background(), id, request.ExecuteOpt{WorkspaceID: otherWorkspace})
			return err
		}},
		{"execute graphql", entities.ProtocolGraphQL, func(uc request.Usecase, id uuid.UUID) error {
			_, err := uc.Execute(context.Background(), id, request.ExecuteOpt{WorkspaceID: otherWorkspace})
			return err
		}},
		{"build curl", entities.ProtocolHTTP, func(uc request.Usecase, id uuid.UUID) error {
			_, err := uc.BuildCurl(context.Background(), id, request.BuildCurlOpt{WorkspaceID: otherWorkspace})
			return err
		}},
		{"resolve websocket", entities.ProtocolWebSocket, func(uc request.Usecase, id uuid.UUID) error {
			_, err := uc.ResolveWebSocket(context.Background(), id, otherWorkspace, "user-1")
			return err
		}},
	}

	for _, p := range paths {
		t.Run(p.name, func(t *testing.T) {
			id := uuid.New()
			repo := newMockRepo()
			repo.requests[id] = guardRequest(id, p.protocol, entities.AuthTypeNone)
			env := &mockEnvResolver{}

			err := p.run(newGuardUsecase(repo, env), id)

			var valErr *domain.ValidationError
			require.ErrorAs(t, err, &valErr)
			assert.Contains(t, valErr.Fields, "workspace")
			assert.Zero(t, env.calls, "variables must not be resolved for another workspace")
		})
	}
}

func TestResolveFirst_MissingCollectionIsNotFound(t *testing.T) {
	id := uuid.New()
	repo := newMockRepo()
	req := guardRequest(id, entities.ProtocolHTTP, entities.AuthTypeNone)
	req.CollectionID = uuid.New()
	repo.requests[id] = req
	env := &mockEnvResolver{}

	_, err := newGuardUsecase(repo, env).Execute(context.Background(), id, request.ExecuteOpt{WorkspaceID: testWorkspaceID})

	var notFound *domain.NotFoundError
	require.ErrorAs(t, err, &notFound)
	assert.Zero(t, env.calls)
}

// Digest and SigV4 sign the final HTTP request, so no other protocol can carry them.
func TestResolveFirst_RequesterOnlySchemesRejectedOffHTTP(t *testing.T) {
	for _, tc := range []struct {
		name     string
		protocol entities.Protocol
		authType entities.AuthType
	}{
		{"grpc digest", entities.ProtocolGRPC, entities.AuthTypeDigest},
		{"grpc sigv4", entities.ProtocolGRPC, entities.AuthTypeAWSSigV4},
		{"graphql digest", entities.ProtocolGraphQL, entities.AuthTypeDigest},
		{"websocket sigv4", entities.ProtocolWebSocket, entities.AuthTypeAWSSigV4},
	} {
		t.Run(tc.name, func(t *testing.T) {
			id := uuid.New()
			repo := newMockRepo()
			repo.requests[id] = guardRequest(id, tc.protocol, tc.authType)
			env := &mockEnvResolver{}
			uc := newGuardUsecase(repo, env)

			var err error
			if tc.protocol == entities.ProtocolWebSocket {
				_, err = uc.ResolveWebSocket(context.Background(), id, testWorkspaceID, "user-1")
			} else {
				_, err = uc.Execute(context.Background(), id, request.ExecuteOpt{WorkspaceID: testWorkspaceID})
			}

			var valErr *domain.ValidationError
			require.ErrorAs(t, err, &valErr)
			assert.Contains(t, valErr.Fields["authType"], "supported for HTTP only")
			assert.Zero(t, env.calls)
		})
	}
}

// A cross-workspace parent must not lend its credentials to a request in another
// workspace: the owner's workspace is what the caller is checked against.
func TestResolveFirst_InheritedOwnerInAnotherWorkspaceRejected(t *testing.T) {
	rootID := uuid.New()
	reader := fixtureCollections()
	reader.collections[testCollectionID].ParentID = &rootID
	reader.collections[rootID] = &entities.Collection{
		ID:          rootID,
		WorkspaceID: uuid.New(),
		AuthType:    entities.AuthTypeBearer,
		AuthData:    `{"token":"abc"}`,
	}

	id := uuid.New()
	repo := newMockRepo()
	repo.requests[id] = guardRequest(id, entities.ProtocolHTTP, entities.AuthTypeInherit)
	env := &mockEnvResolver{}

	_, err := newGuardUsecaseWith(repo, env, reader).Execute(context.Background(), id, request.ExecuteOpt{WorkspaceID: testWorkspaceID})

	var valErr *domain.ValidationError
	require.ErrorAs(t, err, &valErr)
	assert.Contains(t, valErr.Fields, "workspace")
	assert.Zero(t, env.calls)
}
