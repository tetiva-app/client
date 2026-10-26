package postman_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tetiva-app/client/internal/adapters/portability/postman"
	"github.com/tetiva-app/client/internal/domain/entities"
)

func importOne(t *testing.T, item string) (vars []entities.PathVariable, url string, env *stubEnvironmentUC, res *postman.ImportResult) {
	t.Helper()
	data := []byte(`{"info":{"name":"Petstore","schema":"` + postman.SchemaV21 + `"},"item":[` + item + `]}`)
	reqUC := &stubRequestUC{}
	env = &stubEnvironmentUC{}
	res, err := postman.ImportCollection(context.Background(), data, postman.ImportOpts{
		WorkspaceID: uuid.New(), UserID: "local_user",
	}, &stubCollectionUC{}, reqUC, &stubExampleUC{}, env)
	require.NoError(t, err)
	require.Len(t, reqUC.created, 1)
	return reqUC.created[0].PathVariables, reqUC.created[0].URL, env, res
}

func TestImportCollection_PathVariablesFillTheTable(t *testing.T) {
	vars, url, env, res := importOne(t, `{"name":"Get post","request":{"method":"GET","url":{`+
		`"raw":"{{baseUrl}}/users/:id/posts/:postId?next=/:notpath",`+
		`"variable":[{"key":"id","value":"{{userId}}","description":"User id"},{"key":"postId","value":7}]}}}`)

	assert.Equal(t, "{{baseUrl}}/users/:id/posts/:postId?next=/:notpath", url)
	assert.Equal(t, []entities.PathVariable{{Key: "id", Value: "{{userId}}"}, {Key: "postId", Value: "7"}}, vars)
	assert.Nil(t, env.createdEnv, "path values stay with the request")
	assert.Empty(t, res.Warnings)
}

func TestImportCollection_GraphQLPathValuesGoIntoTheURL(t *testing.T) {
	vars, url, _, _ := importOne(t, `{"name":"Q","request":{"method":"POST",`+
		`"url":{"raw":"{{host}}/tenants/:tenant/graphql/:shard","variable":[{"key":"tenant","value":"acme"}]},`+
		`"body":{"mode":"graphql","graphql":{"query":"{ me { id } }"}}}}`)

	assert.Equal(t, "{{host}}/tenants/acme/graphql/:shard", url)
	assert.Empty(t, vars)
}

func TestExportCollection_PathVariables(t *testing.T) {
	rootID := uuid.New()
	collections := []*entities.Collection{{ID: rootID, Name: "Users"}}
	pathVars := []entities.PathVariable{{Key: "id", Value: "{{userId}}"}, {Key: "postId", Value: ""}}
	requests := []*entities.Request{{
		ID: uuid.New(), CollectionID: rootID, Name: "Get post",
		Protocol: entities.ProtocolHTTP, Method: entities.MethodGET,
		URL: "{{baseUrl}}/users/:id/posts/:postId", PathVariables: pathVars,
		BodyType: entities.BodyTypeNone, AuthType: entities.AuthTypeNone, AuthData: "{}",
	}, {
		ID: uuid.New(), CollectionID: rootID, Name: "List users",
		Protocol: entities.ProtocolHTTP, Method: entities.MethodGET,
		URL: "{{baseUrl}}/users", PathVariables: []entities.PathVariable{},
		BodyType: entities.BodyTypeNone, AuthType: entities.AuthTypeNone, AuthData: "{}",
	}}

	data, _, err := postman.ExportCollection(rootID, collections, requests, nil)
	require.NoError(t, err)

	var doc struct {
		Item []struct {
			Request struct {
				URL map[string]json.RawMessage `json:"url"`
			} `json:"request"`
		} `json:"item"`
	}
	require.NoError(t, json.Unmarshal(data, &doc))
	require.Len(t, doc.Item, 2)
	assert.JSONEq(t, `[{"key":"id","value":"{{userId}}"},{"key":"postId","value":""}]`, string(doc.Item[0].Request.URL["variable"]))
	assert.NotContains(t, doc.Item[1].Request.URL, "variable")

	reqUC := &stubRequestUC{}
	_, err = postman.ImportCollection(context.Background(), data, postman.ImportOpts{
		WorkspaceID: uuid.New(), UserID: "local_user",
	}, &stubCollectionUC{}, reqUC, &stubExampleUC{}, &stubEnvironmentUC{})
	require.NoError(t, err)
	assert.Equal(t, pathVars, reqUC.created[0].PathVariables)
}
