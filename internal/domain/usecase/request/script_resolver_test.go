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

// mockCollectionReader is a test double for CollectionReader.
type mockCollectionReader struct {
	collections map[uuid.UUID]*entities.Collection
	byWorkspace map[uuid.UUID][]*entities.Collection // nil → ListByWorkspace returns empty
}

func (m *mockCollectionReader) GetByID(_ context.Context, id uuid.UUID) (*entities.Collection, error) {
	return m.collections[id], nil
}

func (m *mockCollectionReader) ListByWorkspace(_ context.Context, workspaceID uuid.UUID) ([]*entities.Collection, error) {
	if m.byWorkspace == nil {
		return nil, nil
	}
	return m.byWorkspace[workspaceID], nil
}

func TestScriptResolver_RequestHasPreScript(t *testing.T) {
	resolver := request.NewScriptResolver(&mockCollectionReader{})
	req := &entities.Request{PreScript: "console.log('request')"}

	script, err := resolver.ResolvePreScript(context.Background(), req)
	require.NoError(t, err)
	assert.Equal(t, "console.log('request')", script)
}

func TestScriptResolver_RequestEmpty_CollectionHasScript(t *testing.T) {
	collID := uuid.New()
	reader := &mockCollectionReader{
		collections: map[uuid.UUID]*entities.Collection{
			collID: {ID: collID, PreScript: "console.log('collection')"},
		},
	}
	resolver := request.NewScriptResolver(reader)
	req := &entities.Request{CollectionID: collID, PreScript: ""}

	script, err := resolver.ResolvePreScript(context.Background(), req)
	require.NoError(t, err)
	assert.Equal(t, "console.log('collection')", script)
}

func TestScriptResolver_WalksUpToParent(t *testing.T) {
	rootID := uuid.New()
	childID := uuid.New()
	reader := &mockCollectionReader{
		collections: map[uuid.UUID]*entities.Collection{
			childID: {ID: childID, ParentID: &rootID, PreScript: ""},
			rootID:  {ID: rootID, PreScript: "console.log('root')"},
		},
	}
	resolver := request.NewScriptResolver(reader)
	req := &entities.Request{CollectionID: childID, PreScript: ""}

	script, err := resolver.ResolvePreScript(context.Background(), req)
	require.NoError(t, err)
	assert.Equal(t, "console.log('root')", script)
}

func TestScriptResolver_StopsAtFirstFound(t *testing.T) {
	rootID := uuid.New()
	childID := uuid.New()
	reader := &mockCollectionReader{
		collections: map[uuid.UUID]*entities.Collection{
			childID: {ID: childID, ParentID: &rootID, PreScript: "console.log('child')"},
			rootID:  {ID: rootID, PreScript: "console.log('root')"},
		},
	}
	resolver := request.NewScriptResolver(reader)
	req := &entities.Request{CollectionID: childID, PreScript: ""}

	script, err := resolver.ResolvePreScript(context.Background(), req)
	require.NoError(t, err)
	assert.Equal(t, "console.log('child')", script)
}

func TestScriptResolver_NoScriptAnywhere(t *testing.T) {
	collID := uuid.New()
	reader := &mockCollectionReader{
		collections: map[uuid.UUID]*entities.Collection{
			collID: {ID: collID, PreScript: ""},
		},
	}
	resolver := request.NewScriptResolver(reader)
	req := &entities.Request{CollectionID: collID, PreScript: ""}

	script, err := resolver.ResolvePreScript(context.Background(), req)
	require.NoError(t, err)
	assert.Empty(t, script)
}

func TestScriptResolver_PreAndPostResolvedIndependently(t *testing.T) {
	collID := uuid.New()
	reader := &mockCollectionReader{
		collections: map[uuid.UUID]*entities.Collection{
			collID: {ID: collID, PreScript: "console.log('coll-pre')", PostScript: "console.log('coll-post')"},
		},
	}
	resolver := request.NewScriptResolver(reader)
	req := &entities.Request{CollectionID: collID, PreScript: "console.log('req-pre')", PostScript: ""}

	pre, err := resolver.ResolvePreScript(context.Background(), req)
	require.NoError(t, err)
	assert.Equal(t, "console.log('req-pre')", pre) // request wins

	post, err := resolver.ResolvePostScript(context.Background(), req)
	require.NoError(t, err)
	assert.Equal(t, "console.log('coll-post')", post) // inherited from collection
}

func TestScriptResolver_MaxDepthGuard(t *testing.T) {
	// 60 levels — deeper than the resolver's max walk depth
	collections := make(map[uuid.UUID]*entities.Collection)
	var prevID *uuid.UUID
	var bottomID uuid.UUID
	for i := 0; i < 60; i++ {
		id := uuid.New()
		c := &entities.Collection{ID: id, ParentID: prevID}
		collections[id] = c
		prevID = &id
		if i == 0 {
			bottomID = id
		}
	}

	resolver := request.NewScriptResolver(&mockCollectionReader{collections: collections})
	req := &entities.Request{CollectionID: bottomID, PreScript: ""}

	script, err := resolver.ResolvePreScript(context.Background(), req)
	require.NoError(t, err)
	assert.Empty(t, script) // must not loop forever
}
