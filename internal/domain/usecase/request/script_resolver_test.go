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
	assert.Equal(t, request.ResolvedScript{Body: "console.log('request')"}, script)
}

func TestScriptResolver_RequestEmpty_CollectionHasScript(t *testing.T) {
	collID := uuid.New()
	reader := &mockCollectionReader{
		collections: map[uuid.UUID]*entities.Collection{
			collID: {ID: collID, Name: "Users", PreScript: "console.log('collection')"},
		},
	}
	resolver := request.NewScriptResolver(reader)
	req := &entities.Request{CollectionID: collID, PreScript: ""}

	script, err := resolver.ResolvePreScript(context.Background(), req)
	require.NoError(t, err)
	assert.Equal(t, request.ResolvedScript{Body: "console.log('collection')", Collection: "Users"}, script)
}

func TestScriptResolver_WalksUpToParent(t *testing.T) {
	rootID := uuid.New()
	childID := uuid.New()
	reader := &mockCollectionReader{
		collections: map[uuid.UUID]*entities.Collection{
			childID: {ID: childID, ParentID: &rootID, PreScript: ""},
			rootID:  {ID: rootID, Name: "Acme API", PreScript: "console.log('root')"},
		},
	}
	resolver := request.NewScriptResolver(reader)
	req := &entities.Request{CollectionID: childID, PreScript: ""}

	script, err := resolver.ResolvePreScript(context.Background(), req)
	require.NoError(t, err)
	assert.Equal(t, request.ResolvedScript{Body: "console.log('root')", Collection: "Acme API"}, script)
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
	assert.Equal(t, "console.log('child')", script.Body)
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
	assert.Equal(t, "console.log('req-pre')", pre.Body)

	post, err := resolver.ResolvePostScript(context.Background(), req)
	require.NoError(t, err)
	assert.Equal(t, "console.log('coll-post')", post.Body)
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

func TestScriptResolver_BlankRequestScriptFallsThroughToTheCollection(t *testing.T) {
	collID := uuid.New()
	reader := &mockCollectionReader{
		collections: map[uuid.UUID]*entities.Collection{
			collID: {ID: collID, Name: "F1", PreScript: "folder()", PostScript: "folderPost()"},
		},
	}
	resolver := request.NewScriptResolver(reader)
	req := &entities.Request{CollectionID: collID, PreScript: "\n  \n", PostScript: " \t"}

	pre, err := resolver.ResolvePreScript(context.Background(), req)
	require.NoError(t, err)
	post, err := resolver.ResolvePostScript(context.Background(), req)
	require.NoError(t, err)

	assert.Equal(t, request.ResolvedScript{Body: "folder()", Collection: "F1"}, pre)
	assert.Equal(t, request.ResolvedScript{Body: "folderPost()", Collection: "F1"}, post)
}

func TestScriptResolver_BlankCollectionScriptIsSkipped(t *testing.T) {
	rootID, childID := uuid.New(), uuid.New()
	reader := &mockCollectionReader{
		collections: map[uuid.UUID]*entities.Collection{
			childID: {ID: childID, Name: "Child", ParentID: &rootID, PreScript: "\n"},
			rootID:  {ID: rootID, Name: "Root", PreScript: "root()"},
		},
	}

	script, err := request.NewScriptResolver(reader).ResolvePreScript(context.Background(), &entities.Request{CollectionID: childID})
	require.NoError(t, err)

	assert.Equal(t, request.ResolvedScript{Body: "root()", Collection: "Root"}, script)
}
