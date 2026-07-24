package request_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tetiva-app/client/internal/domain/entities"
	"github.com/tetiva-app/client/internal/domain/usecase/request"
)

func TestAuthResolver_RequestHasOwnAuth(t *testing.T) {
	resolver := request.NewAuthResolver(&mockCollectionReader{})
	req := &entities.Request{
		AuthType: entities.AuthTypeBasic,
		AuthData: `{"username":"u","password":"p"}`,
	}

	authType, authData, err := resolver.ResolveAuth(context.Background(), req)
	require.NoError(t, err)
	assert.Equal(t, entities.AuthTypeBasic, authType)
	assert.Equal(t, `{"username":"u","password":"p"}`, authData)
}

func TestAuthResolver_InheritFromCollection(t *testing.T) {
	collID := uuid.New()
	reader := &mockCollectionReader{
		collections: map[uuid.UUID]*entities.Collection{
			collID: {
				ID:       collID,
				AuthType: entities.AuthTypeBearer,
				AuthData: `{"token":"abc"}`,
			},
		},
	}
	resolver := request.NewAuthResolver(reader)
	req := &entities.Request{
		CollectionID: collID,
		AuthType:     entities.AuthTypeInherit,
	}

	authType, authData, err := resolver.ResolveAuth(context.Background(), req)
	require.NoError(t, err)
	assert.Equal(t, entities.AuthTypeBearer, authType)
	assert.Equal(t, `{"token":"abc"}`, authData)
}

func TestAuthResolver_InheritWalksUpToParent(t *testing.T) {
	rootID := uuid.New()
	childID := uuid.New()
	reader := &mockCollectionReader{
		collections: map[uuid.UUID]*entities.Collection{
			childID: {ID: childID, ParentID: &rootID, AuthType: entities.AuthTypeNone, AuthData: "{}"},
			rootID:  {ID: rootID, AuthType: entities.AuthTypeAPIKey, AuthData: `{"key":"X-Key","value":"secret","addTo":"header"}`},
		},
	}
	resolver := request.NewAuthResolver(reader)
	req := &entities.Request{
		CollectionID: childID,
		AuthType:     entities.AuthTypeInherit,
	}

	authType, authData, err := resolver.ResolveAuth(context.Background(), req)
	require.NoError(t, err)
	assert.Equal(t, entities.AuthTypeAPIKey, authType)
	assert.Contains(t, authData, "X-Key")
}

func TestAuthResolver_InheritNoAuthInChain(t *testing.T) {
	collID := uuid.New()
	reader := &mockCollectionReader{
		collections: map[uuid.UUID]*entities.Collection{
			collID: {ID: collID, AuthType: entities.AuthTypeNone, AuthData: "{}"},
		},
	}
	resolver := request.NewAuthResolver(reader)
	req := &entities.Request{
		CollectionID: collID,
		AuthType:     entities.AuthTypeInherit,
	}

	authType, authData, err := resolver.ResolveAuth(context.Background(), req)
	require.NoError(t, err)
	assert.Equal(t, entities.AuthTypeNone, authType)
	assert.Equal(t, "{}", authData)
}

func TestAuthResolver_InheritSoftDeletedCollection(t *testing.T) {
	collID := uuid.New()
	reader := &mockCollectionReader{
		collections: map[uuid.UUID]*entities.Collection{}, // collection not found
	}
	resolver := request.NewAuthResolver(reader)
	req := &entities.Request{
		CollectionID: collID,
		AuthType:     entities.AuthTypeInherit,
	}

	authType, _, err := resolver.ResolveAuth(context.Background(), req)
	require.NoError(t, err)
	assert.Equal(t, entities.AuthTypeNone, authType)
}

func TestAuthResolver_MaxDepthGuard(t *testing.T) {
	collections := make(map[uuid.UUID]*entities.Collection)
	var prevID *uuid.UUID
	var bottomID uuid.UUID
	for i := 0; i < 60; i++ {
		id := uuid.New()
		c := &entities.Collection{ID: id, ParentID: prevID, AuthType: entities.AuthTypeNone, AuthData: "{}"}
		collections[id] = c
		prevID = &id
		if i == 0 {
			bottomID = id
		}
	}

	resolver := request.NewAuthResolver(&mockCollectionReader{collections: collections})
	req := &entities.Request{
		CollectionID: bottomID,
		AuthType:     entities.AuthTypeInherit,
	}

	authType, _, err := resolver.ResolveAuth(context.Background(), req)
	require.NoError(t, err)
	assert.Equal(t, entities.AuthTypeNone, authType)
}
