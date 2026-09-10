package requester_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/google/uuid"

	"github.com/tetiva-app/client/internal/adapters/requester"
	req "github.com/tetiva-app/client/internal/domain/usecase/request"
)

func introspectionFixture() []byte {
	resp := map[string]interface{}{
		"data": map[string]interface{}{
			"__schema": map[string]interface{}{
				"queryType":    map[string]interface{}{"name": "Query"},
				"mutationType": map[string]interface{}{"name": "Mutation"},
				"types": []interface{}{
					map[string]interface{}{
						"name": "Query",
						"kind": "OBJECT",
						"fields": []interface{}{
							map[string]interface{}{
								"name": "users",
								"type": map[string]interface{}{
									"kind": "NON_NULL",
									"name": nil,
									"ofType": map[string]interface{}{
										"kind": "LIST",
										"name": nil,
										"ofType": map[string]interface{}{
											"kind": "NON_NULL",
											"name": nil,
											"ofType": map[string]interface{}{
												"kind":   "OBJECT",
												"name":   "User",
												"ofType": nil,
											},
										},
									},
								},
								"args": []interface{}{
									map[string]interface{}{
										"name": "limit",
										"type": map[string]interface{}{
											"kind":   "SCALAR",
											"name":   "Int",
											"ofType": nil,
										},
										"defaultValue": nil,
									},
								},
							},
							map[string]interface{}{
								"name": "user",
								"type": map[string]interface{}{
									"kind":   "OBJECT",
									"name":   "User",
									"ofType": nil,
								},
								"args": []interface{}{
									map[string]interface{}{
										"name": "id",
										"type": map[string]interface{}{
											"kind": "NON_NULL",
											"name": nil,
											"ofType": map[string]interface{}{
												"kind":   "SCALAR",
												"name":   "ID",
												"ofType": nil,
											},
										},
										"defaultValue": nil,
									},
								},
							},
						},
					},
					map[string]interface{}{
						"name": "Mutation",
						"kind": "OBJECT",
						"fields": []interface{}{
							map[string]interface{}{
								"name": "createUser",
								"type": map[string]interface{}{
									"kind": "NON_NULL",
									"name": nil,
									"ofType": map[string]interface{}{
										"kind":   "OBJECT",
										"name":   "User",
										"ofType": nil,
									},
								},
								"args": []interface{}{
									map[string]interface{}{
										"name": "input",
										"type": map[string]interface{}{
											"kind": "NON_NULL",
											"name": nil,
											"ofType": map[string]interface{}{
												"kind":   "INPUT_OBJECT",
												"name":   "CreateUserInput",
												"ofType": nil,
											},
										},
										"defaultValue": nil,
									},
								},
							},
						},
					},
					map[string]interface{}{
						"name": "User",
						"kind": "OBJECT",
						"fields": []interface{}{
							map[string]interface{}{
								"name": "id",
								"type": map[string]interface{}{
									"kind": "NON_NULL",
									"name": nil,
									"ofType": map[string]interface{}{
										"kind":   "SCALAR",
										"name":   "ID",
										"ofType": nil,
									},
								},
								"args": []interface{}{},
							},
							map[string]interface{}{
								"name": "name",
								"type": map[string]interface{}{
									"kind": "NON_NULL",
									"name": nil,
									"ofType": map[string]interface{}{
										"kind":   "SCALAR",
										"name":   "String",
										"ofType": nil,
									},
								},
								"args": []interface{}{},
							},
						},
					},
					map[string]interface{}{
						"name": "CreateUserInput",
						"kind": "INPUT_OBJECT",
						"inputFields": []interface{}{
							map[string]interface{}{
								"name": "name",
								"type": map[string]interface{}{
									"kind": "NON_NULL",
									"name": nil,
									"ofType": map[string]interface{}{
										"kind":   "SCALAR",
										"name":   "String",
										"ofType": nil,
									},
								},
								"defaultValue": nil,
							},
						},
					},
					// Built-in type that should be filtered out
					map[string]interface{}{
						"name":   "__Type",
						"kind":   "OBJECT",
						"fields": []interface{}{},
					},
					map[string]interface{}{
						"name":   "__Schema",
						"kind":   "OBJECT",
						"fields": []interface{}{},
					},
				},
			},
		},
	}

	b, _ := json.Marshal(resp)
	return b
}

func TestGraphQL_Introspect_FromEndpoint(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("expected Content-Type application/json, got %q", r.Header.Get("Content-Type"))
		}

		var body map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("failed to decode request body: %v", err)
		}
		if _, ok := body["query"]; !ok {
			t.Error("expected query field in request body")
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(introspectionFixture())
	}))
	defer server.Close()

	r := requester.NewGraphQLRequester(nil)
	schema, err := r.Introspect(t.Context(), newIntrospectReq(server.URL, ""))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if schema.Source != "introspection" {
		t.Errorf("expected source introspection, got %q", schema.Source)
	}

	if len(schema.Queries) != 2 {
		t.Fatalf("expected 2 queries, got %d", len(schema.Queries))
	}
	queryNames := map[string]bool{}
	for _, q := range schema.Queries {
		queryNames[q.Name] = true
	}
	if !queryNames["users"] {
		t.Error("expected 'users' query")
	}
	if !queryNames["user"] {
		t.Error("expected 'user' query")
	}

	if len(schema.Mutations) != 1 {
		t.Fatalf("expected 1 mutation, got %d", len(schema.Mutations))
	}
	if schema.Mutations[0].Name != "createUser" {
		t.Errorf("expected mutation 'createUser', got %q", schema.Mutations[0].Name)
	}

	typeNames := map[string]bool{}
	for _, ty := range schema.Types {
		typeNames[ty.Name] = true
	}
	if !typeNames["User"] {
		t.Error("expected 'User' type")
	}
	if !typeNames["CreateUserInput"] {
		t.Error("expected 'CreateUserInput' type")
	}

	var usersQuery *struct {
		name       string
		returnType string
		args       int
	}
	for _, q := range schema.Queries {
		if q.Name == "users" {
			usersQuery = &struct {
				name       string
				returnType string
				args       int
			}{q.Name, q.ReturnType, len(q.Args)}
			break
		}
	}
	if usersQuery == nil {
		t.Fatal("users query not found")
	}
	// NON_NULL(LIST(NON_NULL(User))) → [User!]!
	if usersQuery.returnType != "[User!]!" {
		t.Errorf("expected returnType '[User!]!', got %q", usersQuery.returnType)
	}
	if usersQuery.args != 1 {
		t.Errorf("expected 1 arg for users, got %d", usersQuery.args)
	}

	var userQuery *struct{ argType string }
	for _, q := range schema.Queries {
		if q.Name == "user" && len(q.Args) > 0 {
			userQuery = &struct{ argType string }{q.Args[0].Type}
			break
		}
	}
	if userQuery == nil {
		t.Fatal("user query or its args not found")
	}
	if userQuery.argType != "ID!" {
		t.Errorf("expected arg type 'ID!', got %q", userQuery.argType)
	}

	createMutation := schema.Mutations[0]
	if len(createMutation.Args) != 1 {
		t.Fatalf("expected 1 arg for createUser, got %d", len(createMutation.Args))
	}
	if createMutation.Args[0].Type != "CreateUserInput!" {
		t.Errorf("expected arg type 'CreateUserInput!', got %q", createMutation.Args[0].Type)
	}

	if createMutation.Definition == "" {
		t.Error("expected non-empty Definition for createUser mutation")
	}
	if !strings.Contains(createMutation.Definition, "createUser") {
		t.Errorf("expected Definition to contain 'createUser', got %q", createMutation.Definition)
	}
}

func TestGraphQL_Introspect_FromFile(t *testing.T) {
	sdl := `
type Query {
  products(category: String): [Product!]!
  product(id: ID!): Product
}

type Mutation {
  addProduct(input: AddProductInput!): Product!
}

type Product {
  id: ID!
  name: String!
  price: Float
}

input AddProductInput {
  name: String!
  price: Float!
}

enum ProductStatus {
  ACTIVE
  INACTIVE
}
`
	tmpDir := t.TempDir()
	schemaPath := filepath.Join(tmpDir, "schema.graphql")
	if err := os.WriteFile(schemaPath, []byte(sdl), 0o644); err != nil {
		t.Fatalf("failed to write schema file: %v", err)
	}

	r := requester.NewGraphQLRequester(nil)
	schema, err := r.Introspect(t.Context(), newIntrospectReq("", schemaPath))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if schema.Source != "schema_file" {
		t.Errorf("expected source schema_file, got %q", schema.Source)
	}

	queryNames := map[string]bool{}
	for _, q := range schema.Queries {
		queryNames[q.Name] = true
	}
	if !queryNames["products"] {
		t.Errorf("expected 'products' query, got queries: %v", schema.Queries)
	}
	if !queryNames["product"] {
		t.Errorf("expected 'product' query, got queries: %v", schema.Queries)
	}

	mutNames := map[string]bool{}
	for _, m := range schema.Mutations {
		mutNames[m.Name] = true
	}
	if !mutNames["addProduct"] {
		t.Errorf("expected 'addProduct' mutation, got mutations: %v", schema.Mutations)
	}

	typeNames := map[string]string{}
	for _, ty := range schema.Types {
		typeNames[ty.Name] = ty.Kind
	}
	if typeNames["Product"] == "" {
		t.Errorf("expected 'Product' type, got types: %v", schema.Types)
	}
	if typeNames["AddProductInput"] == "" {
		t.Errorf("expected 'AddProductInput' type, got types: %v", schema.Types)
	}
	if typeNames["ProductStatus"] == "" {
		t.Errorf("expected 'ProductStatus' type, got types: %v", schema.Types)
	}

	var productType *struct {
		fields []string
	}
	for _, ty := range schema.Types {
		if ty.Name == "Product" {
			fields := make([]string, 0, len(ty.Fields))
			for _, f := range ty.Fields {
				fields = append(fields, f.Name)
			}
			productType = &struct{ fields []string }{fields}
			break
		}
	}
	if productType == nil {
		t.Fatal("Product type not found")
	}
	fieldSet := map[string]bool{}
	for _, f := range productType.fields {
		fieldSet[f] = true
	}
	if !fieldSet["id"] || !fieldSet["name"] || !fieldSet["price"] {
		t.Errorf("Product fields incomplete: %v", productType.fields)
	}

	for _, ty := range schema.Types {
		if ty.Name == "ProductStatus" {
			if len(ty.EnumValues) != 2 {
				t.Errorf("expected 2 enum values for ProductStatus, got %d", len(ty.EnumValues))
			}
			break
		}
	}

	for _, ty := range schema.Types {
		if ty.Name == "Product" {
			if !strings.Contains(ty.Definition, "type Product") {
				t.Errorf("expected Definition to contain 'type Product', got: %s", ty.Definition)
			}
			if !strings.Contains(ty.Definition, "id: ID!") {
				t.Errorf("expected Definition to contain 'id: ID!', got: %s", ty.Definition)
			}
			break
		}
	}
}

func TestGraphQL_Introspect_BothSet(t *testing.T) {
	r := requester.NewGraphQLRequester(nil)
	_, err := r.Introspect(t.Context(), newIntrospectReq("http://example.com/graphql", "/some/path.graphql"))
	if err == nil {
		t.Fatal("expected error when both Endpoint and SchemaPath are set")
	}
	if !strings.Contains(err.Error(), "both") {
		t.Errorf("expected error message to mention 'both', got: %v", err)
	}
}

func TestGraphQL_Introspect_NeitherSet(t *testing.T) {
	r := requester.NewGraphQLRequester(nil)
	_, err := r.Introspect(t.Context(), newIntrospectReq("", ""))
	if err == nil {
		t.Fatal("expected error when neither Endpoint nor SchemaPath is set")
	}
	if !strings.Contains(err.Error(), "neither") {
		t.Errorf("expected error message to mention 'neither', got: %v", err)
	}
}

func TestGraphQL_Introspect_FiltersBuiltinTypes(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(introspectionFixture())
	}))
	defer server.Close()

	r := requester.NewGraphQLRequester(nil)
	schema, err := r.Introspect(t.Context(), newIntrospectReq(server.URL, ""))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	for _, ty := range schema.Types {
		if strings.HasPrefix(ty.Name, "__") {
			t.Errorf("built-in type %q should be filtered out", ty.Name)
		}
	}
	for _, ty := range schema.Types {
		if ty.Name == "Query" || ty.Name == "Mutation" {
			t.Errorf("root operation type %q should not appear in Types list", ty.Name)
		}
	}
}

func newIntrospectReq(endpoint, schemaPath string) req.GraphQLIntrospectRequest {
	return req.GraphQLIntrospectRequest{
		Endpoint:   endpoint,
		SchemaPath: schemaPath,
	}
}

func TestGraphQL_Introspect_SendsWorkspaceCookies(t *testing.T) {
	var mu sync.Mutex
	var sent string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		sent = r.Header.Get("Cookie")
		mu.Unlock()
		w.Header().Set("Set-Cookie", "sid=xyz; Path=/")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(introspectionFixture())
	}))
	defer server.Close()

	store := &fakeCookieStore{send: []*http.Cookie{{Name: "session", Value: "abc"}}}
	wsID := uuid.New()
	r := requester.NewGraphQLRequester(store)

	introspectReq := newIntrospectReq(server.URL, "")
	introspectReq.WorkspaceID = wsID
	if _, err := r.Introspect(t.Context(), introspectReq); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	mu.Lock()
	got := sent
	mu.Unlock()
	if got != "session=abc" {
		t.Fatalf("Cookie header = %q, want session=abc", got)
	}

	gotWS, gotURL, cookies := store.snapshot()
	if len(cookies) != 1 || cookies[0].Name != "sid" || cookies[0].Value != "xyz" {
		t.Fatalf("persisted cookies = %+v, want sid=xyz", cookies)
	}
	if gotWS != wsID {
		t.Fatalf("persisted for workspace %s, want %s", gotWS, wsID)
	}
	if gotURL == nil || gotURL.Scheme != "http" {
		t.Fatalf("jar saw URL %v, want http scheme", gotURL)
	}
}
