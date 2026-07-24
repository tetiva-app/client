package requester_test

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/tetiva-app/client/internal/adapters/requester"
	"github.com/tetiva-app/client/internal/domain/entities"
	req "github.com/tetiva-app/client/internal/domain/usecase/request"
)

func TestGraphQLRequester_Execute_Success(t *testing.T) {
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
		if body["query"] != "{ users { id name } }" {
			t.Errorf("unexpected query: %v", body["query"])
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"data":{"users":[{"id":"1","name":"Alice"}]}}`))
	}))
	defer server.Close()

	r := requester.NewGraphQLRequester()
	resp, err := r.Execute(t.Context(), req.GraphQLExecuteRequest{
		Endpoint: server.URL,
		Query:    "{ users { id name } }",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
	if resp.Protocol != entities.ProtocolGraphQL {
		t.Errorf("expected protocol graphql, got %s", resp.Protocol)
	}
	if resp.Body != `{"data":{"users":[{"id":"1","name":"Alice"}]}}` {
		t.Errorf("unexpected body: %s", resp.Body)
	}
	if resp.Duration <= 0 {
		t.Error("expected positive duration")
	}
	if resp.Size <= 0 {
		t.Error("expected positive size")
	}
}

func TestGraphQLRequester_Execute_WithVariables(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		bodyBytes, _ := io.ReadAll(r.Body)

		var body map[string]json.RawMessage
		if err := json.Unmarshal(bodyBytes, &body); err != nil {
			t.Errorf("failed to decode request body: %v", err)
		}

		var query string
		if err := json.Unmarshal(body["query"], &query); err != nil {
			t.Errorf("failed to decode query: %v", err)
		}
		if query != "query GetUser($id: ID!) { user(id: $id) { name } }" {
			t.Errorf("unexpected query: %s", query)
		}

		var operationName string
		if err := json.Unmarshal(body["operationName"], &operationName); err != nil {
			t.Errorf("failed to decode operationName: %v", err)
		}
		if operationName != "GetUser" {
			t.Errorf("unexpected operationName: %s", operationName)
		}

		var vars map[string]string
		if err := json.Unmarshal(body["variables"], &vars); err != nil {
			t.Errorf("failed to decode variables: %v", err)
		}
		if vars["id"] != "42" {
			t.Errorf("unexpected variables: %v", vars)
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"data":{"user":{"name":"Bob"}}}`))
	}))
	defer server.Close()

	r := requester.NewGraphQLRequester()
	resp, err := r.Execute(t.Context(), req.GraphQLExecuteRequest{
		Endpoint:      server.URL,
		Query:         "query GetUser($id: ID!) { user(id: $id) { name } }",
		Variables:     `{"id":"42"}`,
		OperationName: "GetUser",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
	if resp.Body != `{"data":{"user":{"name":"Bob"}}}` {
		t.Errorf("unexpected body: %s", resp.Body)
	}
}

func TestGraphQLRequester_Execute_GraphQLErrors(t *testing.T) {
	// GraphQL errors in body with HTTP 200 — should NOT return a Go error.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"errors":[{"message":"field 'foo' not found","locations":[{"line":1,"column":3}]}]}`))
	}))
	defer server.Close()

	r := requester.NewGraphQLRequester()
	resp, err := r.Execute(t.Context(), req.GraphQLExecuteRequest{
		Endpoint: server.URL,
		Query:    "{ foo }",
	})
	if err != nil {
		t.Fatalf("unexpected Go error (GraphQL errors should be in body): %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
	if resp.Protocol != entities.ProtocolGraphQL {
		t.Errorf("expected protocol graphql, got %s", resp.Protocol)
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(resp.Body), &parsed); err != nil {
		t.Fatalf("failed to parse response body: %v", err)
	}
	if _, ok := parsed["errors"]; !ok {
		t.Error("expected errors key in response body")
	}
}

func TestGraphQLRequester_Execute_CustomHeaders(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer token123" {
			t.Errorf("expected Authorization header, got %q", r.Header.Get("Authorization"))
		}
		if r.Header.Get("X-Custom-Header") != "custom-value" {
			t.Errorf("expected X-Custom-Header, got %q", r.Header.Get("X-Custom-Header"))
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"data":{}}`))
	}))
	defer server.Close()

	r := requester.NewGraphQLRequester()
	resp, err := r.Execute(t.Context(), req.GraphQLExecuteRequest{
		Endpoint: server.URL,
		Query:    "{ __typename }",
		Headers: map[string][]string{
			"Authorization":   {"Bearer token123"},
			"X-Custom-Header": {"custom-value"},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
}

func TestGraphQLRequester_Execute_HTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"errors":[{"message":"internal server error"}]}`))
	}))
	defer server.Close()

	r := requester.NewGraphQLRequester()
	resp, err := r.Execute(t.Context(), req.GraphQLExecuteRequest{
		Endpoint: server.URL,
		Query:    "{ __typename }",
	})
	// HTTP 500 should NOT return a Go error — status is reflected in resp.StatusCode.
	if err != nil {
		t.Fatalf("unexpected Go error for HTTP 500: %v", err)
	}
	if resp.StatusCode != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", resp.StatusCode)
	}
	if resp.Protocol != entities.ProtocolGraphQL {
		t.Errorf("expected protocol graphql, got %s", resp.Protocol)
	}
}
