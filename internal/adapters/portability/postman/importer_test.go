package postman_test

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tetiva-app/client/internal/adapters/portability/postman"
	"github.com/tetiva-app/client/internal/domain"
	"github.com/tetiva-app/client/internal/domain/entities"
	"github.com/tetiva-app/client/internal/domain/usecase/collection"
	"github.com/tetiva-app/client/internal/domain/usecase/request"
	"github.com/tetiva-app/client/internal/domain/usecase/websocket"
)

type stubCollectionUC struct {
	created []collection.Create
}

func (s *stubCollectionUC) Create(_ context.Context, input collection.Create, _ collection.CreateOpt) (*entities.Collection, error) {
	// Mirrors collection.Create.Validate — a nameless folder is what aborted the import.
	if input.Name == "" {
		return nil, errors.New("validation: name is required")
	}
	s.created = append(s.created, input)
	id := uuid.New()
	return &entities.Collection{ID: id, Name: input.Name, ParentID: input.ParentID}, nil
}

func (s *stubCollectionUC) GetByID(context.Context, uuid.UUID) (*entities.Collection, error) {
	return nil, nil
}
func (s *stubCollectionUC) List(context.Context, collection.ListOpt) ([]*entities.Collection, error) {
	return nil, nil
}
func (s *stubCollectionUC) Edit(context.Context, collection.Edit, collection.EditOpt) (*entities.Collection, error) {
	return nil, nil
}
func (s *stubCollectionUC) Delete(context.Context, collection.DeleteOpt) error { return nil }
func (s *stubCollectionUC) Reorder(context.Context, uuid.UUID, int) error      { return nil }
func (s *stubCollectionUC) Move(context.Context, collection.MoveOpt) (*entities.Collection, error) {
	return nil, nil
}

type stubRequestUC struct {
	created []request.Create
}

func (s *stubRequestUC) Create(_ context.Context, input request.Create, _ request.CreateOpt) (*entities.Request, error) {
	s.created = append(s.created, input)
	return &entities.Request{ID: uuid.New(), Name: input.Name}, nil
}

func (s *stubRequestUC) GetByID(context.Context, uuid.UUID) (*entities.Request, error) {
	return nil, nil
}
func (s *stubRequestUC) List(context.Context, request.ListOpt) ([]*entities.Request, error) {
	return nil, nil
}
func (s *stubRequestUC) Edit(context.Context, request.Edit, request.EditOpt) (*entities.Request, error) {
	return nil, nil
}
func (s *stubRequestUC) Delete(context.Context, request.DeleteOpt) error { return nil }
func (s *stubRequestUC) Reorder(context.Context, uuid.UUID, int) error   { return nil }
func (s *stubRequestUC) Execute(context.Context, uuid.UUID, request.ExecuteOpt) (*entities.Response, error) {
	return nil, nil
}
func (s *stubRequestUC) BuildCurl(context.Context, uuid.UUID, request.BuildCurlOpt) (request.CurlResult, error) {
	return request.CurlResult{}, nil
}
func (s *stubRequestUC) Move(context.Context, request.MoveOpt) (*entities.Request, error) {
	return nil, nil
}
func (s *stubRequestUC) GRPCListServices(context.Context, request.GRPCConnectRequest) (*request.GRPCSchema, error) {
	return nil, nil
}
func (s *stubRequestUC) GRPCGenerateExample(context.Context, request.GRPCConnectRequest, string, string) (string, error) {
	return "", nil
}
func (s *stubRequestUC) GRPCGetProtoDefinition(context.Context, request.GRPCConnectRequest, string, string) (string, error) {
	return "", nil
}
func (s *stubRequestUC) GraphQLIntrospect(context.Context, request.GraphQLIntrospectRequest) (*request.GraphQLSchema, error) {
	return nil, nil
}
func (s *stubRequestUC) GraphQLGenerateExample(context.Context, request.GraphQLIntrospectRequest, string) (*request.GraphQLExampleResponse, error) {
	return nil, nil
}
func (s *stubRequestUC) GraphQLGetTypeDefinition(context.Context, request.GraphQLIntrospectRequest, string) (string, error) {
	return "", nil
}
func (s *stubRequestUC) CreateDraftFromHistory(context.Context, request.CreateDraftFromHistoryOpt) (*entities.Request, error) {
	return nil, nil
}

func (s *stubRequestUC) DeleteDraft(context.Context, uuid.UUID) error { return nil }

func (s *stubRequestUC) PromoteDraft(context.Context, request.PromoteDraftOpt) (*entities.Request, error) {
	return nil, nil
}

func (s *stubRequestUC) CleanupDrafts(context.Context) (int, error) { return 0, nil }

func (s *stubRequestUC) ResolveWebSocket(_ context.Context, _, _ uuid.UUID, _ string) (websocket.ResolvedDial, error) {
	return websocket.ResolvedDial{}, nil
}

func (s *stubRequestUC) SubstituteMessage(_ context.Context, _ uuid.UUID, text string) (string, error) {
	return text, nil
}

// items builds the pointer-to-slice a folder carries, keeping [] and absent distinguishable.
func items(v ...postman.PostmanItem) *[]postman.PostmanItem { return &v }

func TestImportCollection_SimpleStructure(t *testing.T) {
	data := postman.PostmanCollection{
		Info: postman.PostmanInfo{Name: "My API", Schema: postman.SchemaV21},
		Item: []postman.PostmanItem{
			{
				Name: "Auth",
				Item: items(
					postman.PostmanItem{
						Name: "Login",
						Request: &postman.PostmanRequest{
							Method: "POST",
							URL:    postman.PostmanURL{Raw: "{{host}}/api/login"},
							Body: &postman.PostmanBody{
								Mode: "raw",
								Raw:  `{"user":"test"}`,
								Options: &postman.PostmanBodyOpt{
									Raw: &postman.PostmanRawOpt{Language: "json"},
								},
							},
						},
					},
				),
			},
			{
				Name: "Ping",
				Request: &postman.PostmanRequest{
					Method: "GET",
					URL:    postman.PostmanURL{Raw: "{{host}}/api/ping"},
				},
			},
		},
	}

	raw, err := json.Marshal(data)
	require.NoError(t, err)

	collUC := &stubCollectionUC{}
	reqUC := &stubRequestUC{}
	workspaceID := uuid.MustParse("00000000-0000-4000-a000-000000000001")

	result, err := postman.ImportCollection(context.Background(), raw, postman.ImportOpts{
		WorkspaceID: workspaceID,
		UserID:      "local_user",
	}, collUC, reqUC)

	require.NoError(t, err)
	assert.Equal(t, 2, result.FoldersCreated)
	assert.Equal(t, 2, result.RequestsCreated)

	assert.Equal(t, 2, len(collUC.created))
	assert.Equal(t, "My API", collUC.created[0].Name)
	assert.Nil(t, collUC.created[0].ParentID)
	assert.Equal(t, "Auth", collUC.created[1].Name)
	assert.NotNil(t, collUC.created[1].ParentID)

	assert.Equal(t, 2, len(reqUC.created))
	assert.Equal(t, "Login", reqUC.created[0].Name)
	assert.Equal(t, entities.MethodPOST, reqUC.created[0].Method)
	assert.Equal(t, "{{host}}/api/login", reqUC.created[0].URL)
	assert.Equal(t, entities.BodyTypeJSON, reqUC.created[0].BodyType)
	assert.Equal(t, `{"user":"test"}`, reqUC.created[0].Body)

	assert.Equal(t, "Ping", reqUC.created[1].Name)
	assert.Equal(t, entities.MethodGET, reqUC.created[1].Method)
}

func TestImportCollection_WithParentID(t *testing.T) {
	data := postman.PostmanCollection{
		Info: postman.PostmanInfo{Name: "Imported", Schema: postman.SchemaV21},
		Item: []postman.PostmanItem{
			{Name: "Health", Request: &postman.PostmanRequest{Method: "GET", URL: postman.PostmanURL{Raw: "/health"}}},
		},
	}

	raw, err := json.Marshal(data)
	require.NoError(t, err)

	collUC := &stubCollectionUC{}
	reqUC := &stubRequestUC{}
	parentID := uuid.New()

	result, err := postman.ImportCollection(context.Background(), raw, postman.ImportOpts{
		WorkspaceID: uuid.New(),
		UserID:      "local_user",
		ParentID:    &parentID,
	}, collUC, reqUC)

	require.NoError(t, err)
	assert.Equal(t, 1, result.FoldersCreated)
	assert.Equal(t, 1, result.RequestsCreated)
	assert.Equal(t, &parentID, collUC.created[0].ParentID)
}

func TestImportCollection_InvalidSchema(t *testing.T) {
	data := postman.PostmanCollection{
		Info: postman.PostmanInfo{Name: "Old", Schema: "https://schema.getpostman.com/json/collection/v2.0.0/collection.json"},
		Item: []postman.PostmanItem{},
	}

	raw, err := json.Marshal(data)
	require.NoError(t, err)

	_, err = postman.ImportCollection(context.Background(), raw, postman.ImportOpts{
		WorkspaceID: uuid.New(),
		UserID:      "local_user",
	}, &stubCollectionUC{}, &stubRequestUC{})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "v2.1")
}

func TestImportCollection_Headers(t *testing.T) {
	data := postman.PostmanCollection{
		Info: postman.PostmanInfo{Name: "Headers Test", Schema: postman.SchemaV21},
		Item: []postman.PostmanItem{
			{
				Name: "With Headers",
				Request: &postman.PostmanRequest{
					Method: "GET",
					URL:    postman.PostmanURL{Raw: "/test"},
					Header: []postman.PostmanKV{
						{Key: "X-Custom", Value: "val1"},
						{Key: "X-Custom", Value: "val2"},
						{Key: "Content-Type", Value: "application/json"},
						{Key: "Disabled-Header", Value: "skip", Disabled: true},
					},
				},
			},
		},
	}

	raw, err := json.Marshal(data)
	require.NoError(t, err)

	reqUC := &stubRequestUC{}
	_, err = postman.ImportCollection(context.Background(), raw, postman.ImportOpts{
		WorkspaceID: uuid.New(),
		UserID:      "local_user",
	}, &stubCollectionUC{}, reqUC)

	require.NoError(t, err)
	require.Equal(t, 1, len(reqUC.created))

	headers := reqUC.created[0].Headers
	require.Equal(t, 4, len(headers))
	assert.Equal(t, "X-Custom", headers[0].Key)
	assert.Equal(t, "val1", headers[0].Value)
	assert.True(t, headers[0].Enabled)
	assert.Equal(t, "X-Custom", headers[1].Key)
	assert.Equal(t, "val2", headers[1].Value)
	assert.True(t, headers[1].Enabled)
	assert.Equal(t, "Content-Type", headers[2].Key)
	assert.True(t, headers[2].Enabled)
	assert.Equal(t, "Disabled-Header", headers[3].Key)
	assert.False(t, headers[3].Enabled)
}

func TestImportCollection_FormData(t *testing.T) {
	data := postman.PostmanCollection{
		Info: postman.PostmanInfo{Name: "Form Test", Schema: postman.SchemaV21},
		Item: []postman.PostmanItem{
			{
				Name: "Upload",
				Request: &postman.PostmanRequest{
					Method: "POST",
					URL:    postman.PostmanURL{Raw: "/upload"},
					Body: &postman.PostmanBody{
						Mode: "formdata",
						FormData: []postman.PostmanKV{
							{Key: "name", Value: "test", Type: "text"},
							{Key: "file", Value: "", Type: "file"},
						},
					},
				},
			},
		},
	}

	raw, err := json.Marshal(data)
	require.NoError(t, err)

	reqUC := &stubRequestUC{}
	_, err = postman.ImportCollection(context.Background(), raw, postman.ImportOpts{
		WorkspaceID: uuid.New(),
		UserID:      "local_user",
	}, &stubCollectionUC{}, reqUC)

	require.NoError(t, err)
	require.Equal(t, 1, len(reqUC.created))

	assert.Equal(t, entities.BodyTypeForm, reqUC.created[0].BodyType)
	assert.Contains(t, reqUC.created[0].Body, `"name"`)
	assert.Contains(t, reqUC.created[0].Body, `"test"`)
}

func TestImportCollection_InvalidJSON(t *testing.T) {
	_, err := postman.ImportCollection(context.Background(), []byte("not json"), postman.ImportOpts{
		WorkspaceID: uuid.New(),
		UserID:      "local_user",
	}, &stubCollectionUC{}, &stubRequestUC{})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid JSON")
}

func TestImportCollection_WithDescriptionAndAuth(t *testing.T) {
	data := postman.PostmanCollection{
		Info: postman.PostmanInfo{
			Name:        "Authed API",
			Description: &postman.PostmanDescription{Content: "# My API\nWith auth."},
			Schema:      postman.SchemaV21,
		},
		Auth: &postman.PostmanAuth{
			Type:   "bearer",
			Bearer: authKVs(map[string]string{"token": "root-token"}),
		},
		Item: []postman.PostmanItem{
			{
				Name:        "Admin",
				Description: &postman.PostmanDescription{Content: "Admin folder"},
				Auth: &postman.PostmanAuth{
					Type:  "basic",
					Basic: authKVs(map[string]string{"username": "admin", "password": "pass"}),
				},
				Item: items(
					postman.PostmanItem{Name: "Users", Request: &postman.PostmanRequest{Method: "GET", URL: postman.PostmanURL{Raw: "/users"}}},
				),
			},
		},
	}

	raw, err := json.Marshal(data)
	require.NoError(t, err)

	collUC := &stubCollectionUC{}
	reqUC := &stubRequestUC{}

	_, err = postman.ImportCollection(context.Background(), raw, postman.ImportOpts{
		WorkspaceID: uuid.New(), UserID: "local_user",
	}, collUC, reqUC)
	require.NoError(t, err)

	require.True(t, len(collUC.created) >= 2)
	assert.Equal(t, "# My API\nWith auth.", collUC.created[0].Description)
	assert.Equal(t, entities.AuthTypeBearer, collUC.created[0].AuthType)
	assert.Contains(t, collUC.created[0].AuthData, "root-token")

	assert.Equal(t, "Admin folder", collUC.created[1].Description)
	assert.Equal(t, entities.AuthTypeBasic, collUC.created[1].AuthType)
	assert.Contains(t, collUC.created[1].AuthData, "admin")
}

func TestImportCollection_RequestLevelDescription(t *testing.T) {
	data, err := os.ReadFile("testdata/request-description.postman_collection.json")
	require.NoError(t, err)

	collUC := &stubCollectionUC{}
	reqUC := &stubRequestUC{}

	res, err := postman.ImportCollection(context.Background(), data, postman.ImportOpts{
		WorkspaceID: uuid.New(), UserID: "local_user",
	}, collUC, reqUC)
	require.NoError(t, err)

	byName := make(map[string]request.Create, len(reqUC.created))
	for _, r := range reqUC.created {
		byName[r.Name] = r
	}
	require.Len(t, byName, 5)

	assert.Equal(t, "Returns pong.", byName["Ping"].Description)
	assert.Equal(t, "Item level only.", byName["Health"].Description,
		"an item-level description is still the fallback")
	assert.Equal(t, "Request level wins.", byName["Status"].Description)
	assert.Equal(t, "Rebuilds the search index.", byName["Reindex"].Description)
	assert.Empty(t, byName["Cleared"].Description,
		"an explicit empty request description is a choice, not an absent one")

	require.Len(t, collUC.created, 2)
	assert.Equal(t, "# Docs API\nCollection level.", collUC.created[0].Description)
	assert.Equal(t, "Folder documentation.", collUC.created[1].Description)

	warnings := strings.Join(res.Warnings, "\n")
	assert.Contains(t, warnings, `"Status": item and request descriptions differ, request level kept`)
	assert.Contains(t, warnings, `"Cleared": item and request descriptions differ, request level kept`)
}

func TestImportCollection_NestedUnderRequestAndUnnamedItem(t *testing.T) {
	data, err := os.ReadFile("testdata/nested-under-request.postman_collection.json")
	require.NoError(t, err)

	collUC, reqUC := &stubCollectionUC{}, &stubRequestUC{}
	res, err := postman.ImportCollection(context.Background(), data, postman.ImportOpts{
		WorkspaceID: uuid.New(), UserID: "local_user",
	}, collUC, reqUC)
	require.NoError(t, err)

	var names []string
	for _, r := range reqUC.created {
		names = append(names, r.Name)
	}
	assert.Equal(t, []string{"Reports", "Ping"}, names, "the request itself is still imported")
	assert.Len(t, collUC.created, 1, "an unnamed empty item must not become a nameless folder")

	warnings := strings.Join(res.Warnings, "\n")
	assert.Contains(t, warnings, `request "Reports": items nested under a request were skipped`)
	assert.Contains(t, warnings, "an unnamed item with neither a request nor children was skipped")
}

func TestImportCollection_NamelessFolders(t *testing.T) {
	data, err := os.ReadFile("testdata/nameless-folder.postman_collection.json")
	require.NoError(t, err)

	collUC, reqUC := &stubCollectionUC{}, &stubRequestUC{}
	res, err := postman.ImportCollection(context.Background(), data, postman.ImportOpts{
		WorkspaceID: uuid.New(), UserID: "local_user",
	}, collUC, reqUC)
	require.NoError(t, err, `an explicitly empty "item": [] under a nameless folder must not abort the import`)

	var folders []string
	for _, c := range collUC.created {
		folders = append(folders, c.Name)
	}
	assert.Equal(t, []string{"Odd Names", "Untitled folder"}, folders)

	require.Len(t, reqUC.created, 1, "the children of a nameless folder are still imported")
	assert.Equal(t, "Ping", reqUC.created[0].Name)

	warnings := strings.Join(res.Warnings, "\n")
	assert.Contains(t, warnings, "an unnamed item with neither a request nor children was skipped")
	assert.Contains(t, warnings, `an unnamed folder with 1 item(s) was imported as "Untitled folder"`)
}

func TestImportCollection_EmptyFolderAndDescriptionTypes(t *testing.T) {
	data, err := os.ReadFile("testdata/empty-folder.postman_collection.json")
	require.NoError(t, err)

	collUC, reqUC := &stubCollectionUC{}, &stubRequestUC{}
	res, err := postman.ImportCollection(context.Background(), data, postman.ImportOpts{
		WorkspaceID: uuid.New(), UserID: "local_user",
	}, collUC, reqUC)
	require.NoError(t, err)

	assert.Equal(t, 3, res.FoldersCreated, "an item with neither request nor item is an empty folder")
	assert.Equal(t, "Legacy", collUC.created[1].Name)
	assert.Equal(t, "Chapter", collUC.created[2].Name)
	assert.Equal(t, "<b>Chapter docs.</b>", collUC.created[2].Description)

	require.Len(t, reqUC.created, 1)
	assert.Equal(t, "<b>x</b>", reqUC.created[0].Description, "content is kept verbatim")

	warnings := strings.Join(res.Warnings, "\n")
	assert.Contains(t, warnings, `collection "Legacy API": description is text/html, imported as plain text`)
	assert.Contains(t, warnings, `folder "Chapter": description is text/html, imported as plain text`)
	assert.Contains(t, warnings, `request "Html": description is text/html, imported as plain text`)
	assert.Contains(t, warnings, `request "Html": item and request descriptions differ, request level kept`)
}

func TestImportCollection_RealPostmanFile(t *testing.T) {
	data, err := os.ReadFile("testdata/sample.postman_collection.json")
	if os.IsNotExist(err) {
		t.Skip("testdata/sample.postman_collection.json not found")
	}
	require.NoError(t, err)

	collUC := &stubCollectionUC{}
	reqUC := &stubRequestUC{}

	result, err := postman.ImportCollection(context.Background(), data, postman.ImportOpts{
		WorkspaceID: uuid.MustParse("00000000-0000-4000-a000-000000000001"),
		UserID:      "local_user",
	}, collUC, reqUC)

	require.NoError(t, err)

	t.Logf("Folders created: %d", result.FoldersCreated)
	t.Logf("Requests created: %d", result.RequestsCreated)

	assert.True(t, result.FoldersCreated >= 18, "expected at least 18 folders (root + 17 sub)")
	assert.True(t, result.RequestsCreated >= 80, "expected at least 80 requests")

	require.True(t, len(collUC.created) > 0)
	assert.NotEmpty(t, collUC.created[0].Name)

	hasVariable := false
	for _, r := range reqUC.created {
		if strings.Contains(r.URL, "{{") {
			hasVariable = true
			break
		}
	}
	assert.True(t, hasVariable, "should preserve {{variable}} syntax in URLs")

	bodyTypes := make(map[entities.BodyType]int)
	for _, r := range reqUC.created {
		bodyTypes[r.BodyType]++
	}
	t.Logf("Body types: %v", bodyTypes)
	assert.True(t, bodyTypes[entities.BodyTypeJSON] > 0, "should have JSON body requests")
}

func TestImportCollection_GraphQL(t *testing.T) {
	data := postman.PostmanCollection{
		Info: postman.PostmanInfo{Name: "GraphQL API", Schema: postman.SchemaV21},
		Item: []postman.PostmanItem{
			{
				Name: "Get Users",
				Request: &postman.PostmanRequest{
					Method: "POST",
					URL:    postman.PostmanURL{Raw: "https://api.example.com/graphql"},
					Body: &postman.PostmanBody{
						Mode: "graphql",
						Graphql: &postman.PostmanGraphQLBody{
							Query:     "query GetUsers { users { id name } }",
							Variables: `{"limit":10}`,
						},
					},
				},
			},
			{
				Name: "Create User",
				Request: &postman.PostmanRequest{
					Method: "POST",
					URL:    postman.PostmanURL{Raw: "https://api.example.com/graphql"},
					Body: &postman.PostmanBody{
						Mode: "graphql",
						Graphql: &postman.PostmanGraphQLBody{
							Query: "mutation CreateUser($name: String!) { createUser(name: $name) { id } }",
						},
					},
				},
			},
		},
	}

	raw, err := json.Marshal(data)
	require.NoError(t, err)

	reqUC := &stubRequestUC{}
	_, err = postman.ImportCollection(context.Background(), raw, postman.ImportOpts{
		WorkspaceID: uuid.New(),
		UserID:      "local_user",
	}, &stubCollectionUC{}, reqUC)

	require.NoError(t, err)
	require.Equal(t, 2, len(reqUC.created))

	r0 := reqUC.created[0]
	assert.Equal(t, "Get Users", r0.Name)
	assert.Equal(t, entities.ProtocolGraphQL, r0.Protocol)
	assert.Equal(t, entities.MethodPOST, r0.Method)
	assert.Equal(t, "https://api.example.com/graphql", r0.URL)
	assert.Equal(t, entities.BodyTypeNone, r0.BodyType)
	assert.Equal(t, "", r0.Body)
	assert.Equal(t, "query GetUsers { users { id name } }", r0.GraphQLQuery)
	assert.Equal(t, `{"limit":10}`, r0.GraphQLVariables)

	r1 := reqUC.created[1]
	assert.Equal(t, "Create User", r1.Name)
	assert.Equal(t, entities.ProtocolGraphQL, r1.Protocol)
	assert.Equal(t, "mutation CreateUser($name: String!) { createUser(name: $name) { id } }", r1.GraphQLQuery)
	assert.Equal(t, "", r1.GraphQLVariables)
}

func TestImportCollection_RequestDescriptionAndAPIKeyLocation(t *testing.T) {
	data := postman.PostmanCollection{
		Info: postman.PostmanInfo{Name: "Docs", Schema: postman.SchemaV21},
		Item: []postman.PostmanItem{
			{
				Name:        "Ping",
				Description: &postman.PostmanDescription{Content: "# Ping\nReturns pong."},
				Request: &postman.PostmanRequest{
					Method: "GET",
					URL:    postman.PostmanURL{Raw: "https://api.example.com/ping"},
					Auth: &postman.PostmanAuth{
						Type:   "apikey",
						APIKey: authKVs(map[string]string{"key": "api_key", "value": "sk_live", "in": "query"}),
					},
				},
			},
		},
	}

	raw, err := json.Marshal(data)
	require.NoError(t, err)

	reqUC := &stubRequestUC{}
	_, err = postman.ImportCollection(context.Background(), raw, postman.ImportOpts{
		WorkspaceID: uuid.New(), UserID: "local_user",
	}, &stubCollectionUC{}, reqUC)
	require.NoError(t, err)

	require.Len(t, reqUC.created, 1)
	assert.Equal(t, "# Ping\nReturns pong.", reqUC.created[0].Description)
	assert.Equal(t, entities.AuthTypeAPIKey, reqUC.created[0].AuthType)
	assert.Contains(t, reqUC.created[0].AuthData, `"addTo":"query"`,
		"the executor reads addTo, not Postman's in")
}

func TestImportCollection_TruncatesOversizedDescription(t *testing.T) {
	long := strings.Repeat("я", domain.MaxDescriptionLen/2+5)
	require.Greater(t, len(long), domain.MaxDescriptionLen)

	data := postman.PostmanCollection{
		Info: postman.PostmanInfo{Name: "Big Docs", Schema: postman.SchemaV21},
		Item: []postman.PostmanItem{
			{
				Name:        "Ping",
				Description: &postman.PostmanDescription{Content: long},
				Request: &postman.PostmanRequest{
					Method: "GET",
					URL:    postman.PostmanURL{Raw: "https://api.example.com/ping"},
				},
			},
		},
	}
	raw, err := json.Marshal(data)
	require.NoError(t, err)

	collUC, reqUC := &stubCollectionUC{}, &stubRequestUC{}
	res, err := postman.ImportCollection(context.Background(), raw, postman.ImportOpts{
		WorkspaceID: uuid.New(), UserID: "local_user",
	}, collUC, reqUC)
	require.NoError(t, err)

	require.Len(t, reqUC.created, 1)
	created := reqUC.created[0]
	assert.LessOrEqual(t, len(created.Description), domain.MaxDescriptionLen)
	assert.True(t, utf8.ValidString(created.Description), "truncation must not split a rune")
	input := created
	assert.NoError(t, input.Validate())

	warnings := strings.Join(res.Warnings, "\n")
	assert.Contains(t, warnings, `request "Ping": description longer than 16384 bytes was truncated`)
}
