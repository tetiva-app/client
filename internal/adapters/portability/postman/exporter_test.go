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

func TestExportCollection(t *testing.T) {
	rootID := uuid.New()
	subID := uuid.New()

	collections := []*entities.Collection{
		{ID: rootID, Name: "My API", ParentID: nil},
		{ID: subID, Name: "Auth", ParentID: &rootID},
	}

	requests := []*entities.Request{
		{
			ID:           uuid.New(),
			CollectionID: subID,
			Name:         "Login",
			Protocol:     entities.ProtocolHTTP,
			Method:       entities.MethodPOST,
			URL:          "{{host}}/api/login",
			Headers:      []entities.HeaderItem{{Key: "Content-Type", Value: "application/json", Enabled: true}},
			Body:         `{"user":"test"}`,
			BodyType:     entities.BodyTypeJSON,
			AuthType:     entities.AuthTypeNone,
			AuthData:     "{}",
		},
		{
			ID:           uuid.New(),
			CollectionID: rootID,
			Name:         "Ping",
			Protocol:     entities.ProtocolHTTP,
			Method:       entities.MethodGET,
			URL:          "{{host}}/api/ping",
			Headers:      []entities.HeaderItem{},
			BodyType:     entities.BodyTypeNone,
			AuthType:     entities.AuthTypeNone,
			AuthData:     "{}",
		},
	}

	data, err := postman.ExportCollection(rootID, collections, requests)
	require.NoError(t, err)

	var pc postman.PostmanCollection
	err = json.Unmarshal(data, &pc)
	require.NoError(t, err)

	assert.Equal(t, "My API", pc.Info.Name)
	assert.Equal(t, postman.SchemaV21, pc.Info.Schema)

	require.Equal(t, 2, len(pc.Item))

	var authFolder, pingReq *postman.PostmanItem
	for i := range pc.Item {
		if pc.Item[i].IsFolder() {
			authFolder = &pc.Item[i]
		} else {
			pingReq = &pc.Item[i]
		}
	}

	require.NotNil(t, authFolder)
	assert.Equal(t, "Auth", authFolder.Name)
	require.Equal(t, 1, len(authFolder.Item))
	assert.Equal(t, "Login", authFolder.Item[0].Name)
	assert.Equal(t, "POST", authFolder.Item[0].Request.Method)
	assert.Equal(t, "{{host}}/api/login", authFolder.Item[0].Request.URL.Raw)

	require.NotNil(t, pingReq)
	assert.Equal(t, "Ping", pingReq.Name)
	assert.Equal(t, "GET", pingReq.Request.Method)
}

func TestExportCollection_Headers(t *testing.T) {
	rootID := uuid.New()
	collections := []*entities.Collection{
		{ID: rootID, Name: "Test"},
	}
	requests := []*entities.Request{
		{
			ID: uuid.New(), CollectionID: rootID, Name: "Multi Headers",
			Protocol: entities.ProtocolHTTP, Method: entities.MethodGET, URL: "/test",
			Headers: []entities.HeaderItem{
				{Key: "X-Custom", Value: "val1", Enabled: true},
				{Key: "X-Custom", Value: "val2", Enabled: true},
			},
			BodyType: entities.BodyTypeNone, AuthType: entities.AuthTypeNone, AuthData: "{}",
		},
	}

	data, err := postman.ExportCollection(rootID, collections, requests)
	require.NoError(t, err)

	var pc postman.PostmanCollection
	require.NoError(t, json.Unmarshal(data, &pc))

	require.Equal(t, 1, len(pc.Item))
	headers := pc.Item[0].Request.Header
	assert.Equal(t, 2, len(headers))
	assert.Equal(t, "X-Custom", headers[0].Key)
	assert.Equal(t, "val1", headers[0].Value)
	assert.Equal(t, "X-Custom", headers[1].Key)
	assert.Equal(t, "val2", headers[1].Value)
}

func TestExportCollection_DisabledHeaders(t *testing.T) {
	rootID := uuid.New()
	collections := []*entities.Collection{
		{ID: rootID, Name: "Disabled Test"},
	}
	requests := []*entities.Request{
		{
			ID: uuid.New(), CollectionID: rootID, Name: "With Disabled",
			Protocol: entities.ProtocolHTTP, Method: entities.MethodGET, URL: "/test",
			Headers: []entities.HeaderItem{
				{Key: "X-Active", Value: "yes", Enabled: true},
				{Key: "X-Disabled", Value: "no", Enabled: false},
			},
			BodyType: entities.BodyTypeNone, AuthType: entities.AuthTypeNone, AuthData: "{}",
		},
	}

	data, err := postman.ExportCollection(rootID, collections, requests)
	require.NoError(t, err)

	var pc postman.PostmanCollection
	require.NoError(t, json.Unmarshal(data, &pc))

	require.Equal(t, 1, len(pc.Item))
	headers := pc.Item[0].Request.Header
	require.Equal(t, 2, len(headers))

	assert.Equal(t, "X-Active", headers[0].Key)
	assert.False(t, headers[0].Disabled)

	assert.Equal(t, "X-Disabled", headers[1].Key)
	assert.True(t, headers[1].Disabled)
}

func TestExportCollection_FormBody(t *testing.T) {
	rootID := uuid.New()
	collections := []*entities.Collection{
		{ID: rootID, Name: "Form Test"},
	}
	requests := []*entities.Request{
		{
			ID: uuid.New(), CollectionID: rootID, Name: "Upload",
			Protocol: entities.ProtocolHTTP, Method: entities.MethodPOST, URL: "/upload",
			Body:     `[{"key":"name","value":"test"}]`,
			BodyType: entities.BodyTypeForm,
			AuthType: entities.AuthTypeNone, AuthData: "{}",
		},
	}

	data, err := postman.ExportCollection(rootID, collections, requests)
	require.NoError(t, err)

	var pc postman.PostmanCollection
	require.NoError(t, json.Unmarshal(data, &pc))

	require.Equal(t, 1, len(pc.Item))
	body := pc.Item[0].Request.Body
	require.NotNil(t, body)
	assert.Equal(t, "formdata", body.Mode)
	require.Equal(t, 1, len(body.FormData))
	assert.Equal(t, "name", body.FormData[0].Key)
}

func TestExportCollection_WithDescriptionAndAuth(t *testing.T) {
	rootID := uuid.New()
	subID := uuid.New()

	collections := []*entities.Collection{
		{
			ID: rootID, Name: "API", ParentID: nil,
			Description: "# Root API",
			AuthType:    entities.AuthTypeBearer,
			AuthData:    `{"token":"root-token"}`,
		},
		{
			ID: subID, Name: "Admin", ParentID: &rootID,
			Description: "Admin endpoints",
			AuthType:    entities.AuthTypeBasic,
			AuthData:    `{"username":"admin","password":"pass"}`,
		},
	}

	requests := []*entities.Request{
		{
			ID: uuid.New(), CollectionID: subID, Name: "Users",
			Protocol: entities.ProtocolHTTP, Method: entities.MethodGET, URL: "/users",
			BodyType: entities.BodyTypeNone, AuthType: entities.AuthTypeNone, AuthData: "{}",
		},
	}

	data, err := postman.ExportCollection(rootID, collections, requests)
	require.NoError(t, err)

	var pc postman.PostmanCollection
	require.NoError(t, json.Unmarshal(data, &pc))

	assert.Equal(t, "# Root API", string(pc.Info.Description))

	require.NotNil(t, pc.Auth)
	assert.Equal(t, "bearer", pc.Auth.Type)
	assert.Equal(t, "root-token", pc.Auth.Bearer[0].String())

	require.Equal(t, 1, len(pc.Item))
	assert.Equal(t, "Admin", pc.Item[0].Name)
	assert.Equal(t, "Admin endpoints", string(pc.Item[0].Description))
	require.NotNil(t, pc.Item[0].Auth)
	assert.Equal(t, "basic", pc.Item[0].Auth.Type)
}

func TestExportCollection_NoAuthOmitted(t *testing.T) {
	rootID := uuid.New()
	collections := []*entities.Collection{
		{ID: rootID, Name: "No Auth", AuthType: entities.AuthTypeNone, AuthData: "{}"},
	}

	data, err := postman.ExportCollection(rootID, collections, nil)
	require.NoError(t, err)

	var pc postman.PostmanCollection
	require.NoError(t, json.Unmarshal(data, &pc))

	assert.Nil(t, pc.Auth) // omitempty — no auth in JSON
}

func TestExportCollection_RootNotFound(t *testing.T) {
	_, err := postman.ExportCollection(uuid.New(), nil, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "root collection not found")
}

func TestExportCollection_GraphQL(t *testing.T) {
	rootID := uuid.New()
	collections := []*entities.Collection{
		{ID: rootID, Name: "GraphQL API"},
	}
	requests := []*entities.Request{
		{
			ID:               uuid.New(),
			CollectionID:     rootID,
			Name:             "Get Users",
			Protocol:         entities.ProtocolGraphQL,
			Method:           entities.MethodPOST,
			URL:              "https://api.example.com/graphql",
			BodyType:         entities.BodyTypeNone,
			GraphQLQuery:     "query GetUsers { users { id name } }",
			GraphQLVariables: `{"limit":10}`,
			AuthType:         entities.AuthTypeNone,
			AuthData:         "{}",
		},
		{
			ID:           uuid.New(),
			CollectionID: rootID,
			Name:         "Create User",
			Protocol:     entities.ProtocolGraphQL,
			Method:       entities.MethodPOST,
			URL:          "https://api.example.com/graphql",
			BodyType:     entities.BodyTypeNone,
			GraphQLQuery: "mutation CreateUser($name: String!) { createUser(name: $name) { id } }",
			AuthType:     entities.AuthTypeNone,
			AuthData:     "{}",
		},
	}

	data, err := postman.ExportCollection(rootID, collections, requests)
	require.NoError(t, err)

	var pc postman.PostmanCollection
	require.NoError(t, json.Unmarshal(data, &pc))

	require.Equal(t, 2, len(pc.Item))

	item0 := pc.Item[0]
	assert.Equal(t, "Get Users", item0.Name)
	require.NotNil(t, item0.Request)
	assert.Equal(t, "POST", item0.Request.Method)
	assert.Equal(t, "https://api.example.com/graphql", item0.Request.URL.Raw)

	body0 := item0.Request.Body
	require.NotNil(t, body0)
	assert.Equal(t, "graphql", body0.Mode)
	require.NotNil(t, body0.Graphql)
	assert.Equal(t, "query GetUsers { users { id name } }", body0.Graphql.Query)
	assert.Equal(t, `{"limit":10}`, body0.Graphql.Variables)

	item1 := pc.Item[1]
	assert.Equal(t, "Create User", item1.Name)
	require.NotNil(t, item1.Request)
	assert.Equal(t, "POST", item1.Request.Method)

	body1 := item1.Request.Body
	require.NotNil(t, body1)
	assert.Equal(t, "graphql", body1.Mode)
	require.NotNil(t, body1.Graphql)
	assert.Equal(t, "mutation CreateUser($name: String!) { createUser(name: $name) { id } }", body1.Graphql.Query)
	assert.Equal(t, "", body1.Graphql.Variables)
}

func TestExportCollection_RequestDescriptionAndAPIKeyLocation(t *testing.T) {
	rootID := uuid.New()
	collections := []*entities.Collection{{ID: rootID, Name: "API", ParentID: nil}}
	requests := []*entities.Request{
		{
			ID: uuid.New(), CollectionID: rootID, Name: "Ping",
			Description: "# Ping\nReturns pong.",
			Protocol:    entities.ProtocolHTTP, Method: entities.MethodGET,
			URL: "https://api.example.com/ping", BodyType: entities.BodyTypeNone,
			AuthType: entities.AuthTypeAPIKey,
			AuthData: `{"key":"api_key","value":"sk_live","addTo":"query"}`,
		},
		{
			ID: uuid.New(), CollectionID: rootID, Name: "Legacy",
			Protocol: entities.ProtocolHTTP, Method: entities.MethodGET,
			URL: "https://api.example.com/legacy", BodyType: entities.BodyTypeNone,
			AuthType: entities.AuthTypeAPIKey,
			AuthData: `{"key":"api_key","value":"sk_live","in":"query"}`,
		},
	}

	data, err := postman.ExportCollection(rootID, collections, requests)
	require.NoError(t, err)

	var pc postman.PostmanCollection
	require.NoError(t, json.Unmarshal(data, &pc))

	require.Len(t, pc.Item, 2)
	assert.Equal(t, "# Ping\nReturns pong.", string(pc.Item[0].Description))
	assert.Equal(t, "query", findKV(t, pc.Item[0].Request.Auth.APIKey, "in"))
	assert.Equal(t, "query", findKV(t, pc.Item[1].Request.Auth.APIKey, "in"),
		"collections imported before the addTo rename must still export their location")
}

// The exporter writes descriptions at item level and the importer prefers the
// request level, so a round trip is the only proof the two still meet.
func TestExportImportRoundTrip_KeepsDescriptions(t *testing.T) {
	rootID := uuid.New()
	folderID := uuid.New()
	collections := []*entities.Collection{
		{ID: rootID, Name: "API", Description: "# API\nRoot docs."},
		{ID: folderID, Name: "Admin", Description: "Folder docs.", ParentID: &rootID},
	}
	requests := []*entities.Request{
		{
			ID: uuid.New(), CollectionID: folderID, Name: "Reindex",
			Description: "Rebuilds the search index.",
			Protocol:    entities.ProtocolHTTP, Method: entities.MethodPOST,
			URL: "https://api.example.com/admin/reindex", BodyType: entities.BodyTypeNone,
			AuthType: entities.AuthTypeNone, AuthData: "{}",
		},
	}

	data, err := postman.ExportCollection(rootID, collections, requests)
	require.NoError(t, err)

	collUC := &stubCollectionUC{}
	reqUC := &stubRequestUC{}
	_, err = postman.ImportCollection(context.Background(), data, postman.ImportOpts{
		WorkspaceID: uuid.New(), UserID: "local_user",
	}, collUC, reqUC)
	require.NoError(t, err)

	require.Len(t, collUC.created, 2)
	assert.Equal(t, "# API\nRoot docs.", collUC.created[0].Description)
	assert.Equal(t, "Folder docs.", collUC.created[1].Description)

	require.Len(t, reqUC.created, 1)
	assert.Equal(t, "Rebuilds the search index.", reqUC.created[0].Description)
}

func findKV(t *testing.T, kvs []postman.PostmanAuthKV, key string) string {
	t.Helper()
	for _, kv := range kvs {
		if kv.Key == key {
			return kv.String()
		}
	}
	t.Fatalf("key %q not found in %+v", key, kvs)
	return ""
}

func TestExportCollection_WebSocketOmitsSettingsBody(t *testing.T) {
	rootID := uuid.New()
	collections := []*entities.Collection{{ID: rootID, Name: "Realtime"}}
	requests := []*entities.Request{
		{
			ID:           uuid.New(),
			CollectionID: rootID,
			Name:         "Ticker",
			Description:  "# Ticker\nStreams prices.",
			Protocol:     entities.ProtocolWebSocket,
			Method:       entities.MethodGET,
			URL:          "wss://api.example.com/ws",
			Headers:      []entities.HeaderItem{{Key: "X-Test", Value: "1", Enabled: true}},
			Body:         `{"version":1,"pingIntervalSec":30,"subprotocols":["graphql-ws"],"messages":[{"id":"m1","name":"Login","format":"json","data":"{}"}]}`,
			BodyType:     entities.BodyTypeRaw,
			AuthType:     entities.AuthTypeBearer,
			AuthData:     `{"token":"tok"}`,
		},
	}

	data, err := postman.ExportCollection(rootID, collections, requests)
	require.NoError(t, err)

	var pc postman.PostmanCollection
	require.NoError(t, json.Unmarshal(data, &pc))
	require.Equal(t, 1, len(pc.Item))

	item := pc.Item[0]
	require.NotNil(t, item.Request)
	assert.Nil(t, item.Request.Body, "websocket settings must not be exported as a request body")
	assert.NotContains(t, string(data), "pingIntervalSec")

	assert.Equal(t, "Ticker", item.Name)
	assert.Equal(t, "# Ticker\nStreams prices.", string(item.Description))
	assert.Equal(t, "wss://api.example.com/ws", item.Request.URL.Raw)
	require.Equal(t, 1, len(item.Request.Header))
	assert.Equal(t, "X-Test", item.Request.Header[0].Key)
	require.NotNil(t, item.Request.Auth)
	assert.Equal(t, "bearer", item.Request.Auth.Type)
}
