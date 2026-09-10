package requester_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/tetiva-app/client/internal/adapters/requester"
	req "github.com/tetiva-app/client/internal/domain/usecase/request"
)

func TestGraphQL_GenerateExample_SimpleQuery(t *testing.T) {
	schema := &req.GraphQLSchema{
		Queries: []req.GraphQLOperation{
			{
				Name:       "getUser",
				ReturnType: "User!",
				Args: []req.GraphQLArg{
					{Name: "id", Type: "ID!"},
				},
			},
		},
		Types: []req.GraphQLType{
			{
				Name: "User",
				Kind: "OBJECT",
				Fields: []req.GraphQLField{
					{Name: "id", Type: "ID!"},
					{Name: "name", Type: "String!"},
					{Name: "email", Type: "String"},
				},
			},
		},
	}

	r := requester.NewGraphQLRequester(nil)
	result, err := r.GenerateExampleQuery(schema, "getUser")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.HasPrefix(result.Query, "query getUser") {
		t.Errorf("expected query to start with 'query getUser', got:\n%s", result.Query)
	}
	if !strings.Contains(result.Query, "$id: ID!") {
		t.Errorf("expected variable declaration '$id: ID!', got:\n%s", result.Query)
	}
	if !strings.Contains(result.Query, "id: $id") {
		t.Errorf("expected call argument 'id: $id', got:\n%s", result.Query)
	}
	for _, field := range []string{"id", "name", "email"} {
		if !strings.Contains(result.Query, field) {
			t.Errorf("expected field %q in query, got:\n%s", field, result.Query)
		}
	}

	var vars map[string]interface{}
	if err := json.Unmarshal([]byte(result.Variables), &vars); err != nil {
		t.Fatalf("failed to parse variables JSON: %v", err)
	}
	if vars["id"] != "" {
		t.Errorf("expected id placeholder \"\", got %v", vars["id"])
	}
}

func TestGraphQL_GenerateExample_NestedFields(t *testing.T) {
	schema := &req.GraphQLSchema{
		Queries: []req.GraphQLOperation{
			{
				Name:       "getUser",
				ReturnType: "User!",
				Args:       []req.GraphQLArg{},
			},
		},
		Types: []req.GraphQLType{
			{
				Name: "User",
				Kind: "OBJECT",
				Fields: []req.GraphQLField{
					{Name: "id", Type: "ID!"},
					{Name: "name", Type: "String!"},
					{Name: "orders", Type: "[Order!]!"},
				},
			},
			{
				Name: "Order",
				Kind: "OBJECT",
				Fields: []req.GraphQLField{
					{Name: "id", Type: "ID!"},
					{Name: "total", Type: "Float!"},
					{Name: "items", Type: "[Item!]!"},
				},
			},
			{
				Name: "Item",
				Kind: "OBJECT",
				Fields: []req.GraphQLField{
					{Name: "id", Type: "ID!"},
					{Name: "name", Type: "String!"},
					{Name: "subItems", Type: "[SubItem!]!"},
				},
			},
			{
				Name: "SubItem",
				Kind: "OBJECT",
				Fields: []req.GraphQLField{
					{Name: "id", Type: "ID!"},
				},
			},
		},
	}

	r := requester.NewGraphQLRequester(nil)
	result, err := r.GenerateExampleQuery(schema, "getUser")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(result.Query, "orders") {
		t.Errorf("expected 'orders' field in query, got:\n%s", result.Query)
	}
	if !strings.Contains(result.Query, "items") {
		t.Errorf("expected 'items' field in query, got:\n%s", result.Query)
	}
	if !strings.Contains(result.Query, "id") {
		t.Errorf("expected 'id' field in query, got:\n%s", result.Query)
	}
	if !strings.Contains(result.Query, "name") {
		t.Errorf("expected 'name' field in query, got:\n%s", result.Query)
	}
	if strings.Contains(result.Query, "subItems") {
		t.Errorf("expected 'subItems' to be omitted at depth limit, got:\n%s", result.Query)
	}
}

func TestGraphQL_GenerateExample_CycleProtection(t *testing.T) {
	schema := &req.GraphQLSchema{
		Queries: []req.GraphQLOperation{
			{
				Name:       "getA",
				ReturnType: "TypeA",
				Args:       []req.GraphQLArg{},
			},
		},
		Types: []req.GraphQLType{
			{
				Name: "TypeA",
				Kind: "OBJECT",
				Fields: []req.GraphQLField{
					{Name: "id", Type: "ID!"},
					{Name: "b", Type: "TypeB"},
				},
			},
			{
				Name: "TypeB",
				Kind: "OBJECT",
				Fields: []req.GraphQLField{
					{Name: "value", Type: "String!"},
					{Name: "a", Type: "TypeA"}, // cycle back to TypeA
				},
			},
		},
	}

	r := requester.NewGraphQLRequester(nil)
	result, err := r.GenerateExampleQuery(schema, "getA")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Query == "" {
		t.Error("expected non-empty query")
	}
	if !strings.Contains(result.Query, "id") {
		t.Errorf("expected 'id' field in query, got:\n%s", result.Query)
	}
}

func TestGraphQL_GenerateExample_EnumArgs(t *testing.T) {
	schema := &req.GraphQLSchema{
		Queries: []req.GraphQLOperation{
			{
				Name:       "listItems",
				ReturnType: "[Item!]!",
				Args: []req.GraphQLArg{
					{Name: "status", Type: "ItemStatus!"},
				},
			},
		},
		Types: []req.GraphQLType{
			{
				Name:       "ItemStatus",
				Kind:       "ENUM",
				EnumValues: []string{"ACTIVE", "INACTIVE", "ARCHIVED"},
			},
			{
				Name: "Item",
				Kind: "OBJECT",
				Fields: []req.GraphQLField{
					{Name: "id", Type: "ID!"},
				},
			},
		},
	}

	r := requester.NewGraphQLRequester(nil)
	result, err := r.GenerateExampleQuery(schema, "listItems")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var vars map[string]interface{}
	if err := json.Unmarshal([]byte(result.Variables), &vars); err != nil {
		t.Fatalf("failed to parse variables JSON: %v", err)
	}

	if vars["status"] != "ACTIVE" {
		t.Errorf("expected status placeholder 'ACTIVE', got %v", vars["status"])
	}
}

func TestGraphQL_GenerateExample_InputTypeArgs(t *testing.T) {
	schema := &req.GraphQLSchema{
		Mutations: []req.GraphQLOperation{
			{
				Name:       "createUser",
				ReturnType: "User!",
				Args: []req.GraphQLArg{
					{Name: "input", Type: "CreateUserInput!"},
				},
			},
		},
		Types: []req.GraphQLType{
			{
				Name: "CreateUserInput",
				Kind: "INPUT_OBJECT",
				Fields: []req.GraphQLField{
					{Name: "name", Type: "String!"},
					{Name: "age", Type: "Int!"},
				},
			},
			{
				Name: "User",
				Kind: "OBJECT",
				Fields: []req.GraphQLField{
					{Name: "id", Type: "ID!"},
					{Name: "name", Type: "String!"},
				},
			},
		},
	}

	r := requester.NewGraphQLRequester(nil)
	result, err := r.GenerateExampleQuery(schema, "createUser")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var vars map[string]interface{}
	if err := json.Unmarshal([]byte(result.Variables), &vars); err != nil {
		t.Fatalf("failed to parse variables JSON: %v", err)
	}

	inputObj, ok := vars["input"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected 'input' to be an object, got %T: %v", vars["input"], vars["input"])
	}
	if inputObj["name"] != "" {
		t.Errorf("expected input.name placeholder \"\", got %v", inputObj["name"])
	}
	if inputObj["age"] != float64(0) {
		t.Errorf("expected input.age placeholder 0, got %v", inputObj["age"])
	}
}

func TestGraphQL_GenerateExample_UnionType(t *testing.T) {
	schema := &req.GraphQLSchema{
		Queries: []req.GraphQLOperation{
			{
				Name:       "search",
				ReturnType: "SearchResult",
				Args:       []req.GraphQLArg{},
			},
		},
		Types: []req.GraphQLType{
			{
				Name:          "SearchResult",
				Kind:          "UNION",
				PossibleTypes: []string{"UserResult", "PostResult"},
			},
			{
				Name: "UserResult",
				Kind: "OBJECT",
				Fields: []req.GraphQLField{
					{Name: "userId", Type: "ID!"},
					{Name: "username", Type: "String!"},
				},
			},
			{
				Name: "PostResult",
				Kind: "OBJECT",
				Fields: []req.GraphQLField{
					{Name: "postId", Type: "ID!"},
					{Name: "title", Type: "String!"},
				},
			},
		},
	}

	r := requester.NewGraphQLRequester(nil)
	result, err := r.GenerateExampleQuery(schema, "search")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(result.Query, "... on UserResult") {
		t.Errorf("expected '... on UserResult' inline fragment, got:\n%s", result.Query)
	}
	if !strings.Contains(result.Query, "... on PostResult") {
		t.Errorf("expected '... on PostResult' inline fragment, got:\n%s", result.Query)
	}
	if !strings.Contains(result.Query, "userId") {
		t.Errorf("expected 'userId' field in union fragment, got:\n%s", result.Query)
	}
	if !strings.Contains(result.Query, "postId") {
		t.Errorf("expected 'postId' field in union fragment, got:\n%s", result.Query)
	}
}

func TestGraphQL_GenerateExample_MutationQuery(t *testing.T) {
	schema := &req.GraphQLSchema{
		Mutations: []req.GraphQLOperation{
			{
				Name:       "deletePost",
				ReturnType: "Boolean!",
				Args: []req.GraphQLArg{
					{Name: "id", Type: "ID!"},
				},
			},
		},
	}

	r := requester.NewGraphQLRequester(nil)
	result, err := r.GenerateExampleQuery(schema, "deletePost")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.HasPrefix(result.Query, "mutation deletePost") {
		t.Errorf("expected query to start with 'mutation deletePost', got:\n%s", result.Query)
	}
	if strings.HasPrefix(result.Query, "query") {
		t.Errorf("mutation must not use 'query' keyword, got:\n%s", result.Query)
	}

	var vars map[string]interface{}
	if err := json.Unmarshal([]byte(result.Variables), &vars); err != nil {
		t.Fatalf("failed to parse variables JSON: %v", err)
	}
	if vars["id"] != "" {
		t.Errorf("expected id placeholder \"\", got %v", vars["id"])
	}
}

func TestGraphQL_GenerateExample_OperationNotFound(t *testing.T) {
	schema := &req.GraphQLSchema{
		Queries: []req.GraphQLOperation{
			{Name: "getUser", ReturnType: "User", Args: []req.GraphQLArg{}},
		},
	}

	r := requester.NewGraphQLRequester(nil)
	result, err := r.GenerateExampleQuery(schema, "nonExistentOperation")
	if err == nil {
		t.Fatalf("expected error for unknown operation, got result: %+v", result)
	}
	if !strings.Contains(err.Error(), "nonExistentOperation") {
		t.Errorf("expected error to mention operation name, got: %v", err)
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("expected error to mention 'not found', got: %v", err)
	}
}
