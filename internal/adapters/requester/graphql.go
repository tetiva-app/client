package requester

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/tetiva-app/client/internal/domain/entities"
	"github.com/tetiva-app/client/internal/domain/usecase/request"
)

// GraphQLRequester implements request.GraphQLRequester.
type GraphQLRequester struct {
	client *http.Client
}

// NewGraphQLRequester creates a new GraphQL requester.
func NewGraphQLRequester() *GraphQLRequester {
	return &GraphQLRequester{
		client: &http.Client{Timeout: 30 * time.Second},
	}
}

type graphqlRequestBody struct {
	Query         string          `json:"query"`
	Variables     json.RawMessage `json:"variables,omitempty"`
	OperationName string          `json:"operationName,omitempty"`
}

// Execute sends a GraphQL query/mutation via HTTP POST.
func (r *GraphQLRequester) Execute(ctx context.Context, req request.GraphQLExecuteRequest) (*entities.Response, error) {
	const funcName = "GraphQLRequester.Execute"

	var vars json.RawMessage
	if req.Variables != "" && req.Variables != "{}" {
		vars = json.RawMessage(req.Variables)
	}

	payload := graphqlRequestBody{
		Query:         req.Query,
		Variables:     vars,
		OperationName: req.OperationName,
	}

	bodyBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("%s: failed to marshal request: %w", funcName, err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, req.Endpoint, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("%s: failed to create HTTP request: %w", funcName, err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	for key, values := range req.Headers {
		for _, v := range values {
			httpReq.Header.Add(key, v)
		}
	}

	start := time.Now()
	httpResp, err := r.client.Do(httpReq)
	elapsed := time.Since(start)
	if err != nil {
		return nil, fmt.Errorf("%s: request failed: %w", funcName, err)
	}
	defer func() { _ = httpResp.Body.Close() }()

	respBody, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return nil, fmt.Errorf("%s: failed to read response: %w", funcName, err)
	}

	respHeaders := make(map[string][]string, len(httpResp.Header))
	for k, v := range httpResp.Header {
		respHeaders[k] = v
	}

	return &entities.Response{
		Protocol:   entities.ProtocolGraphQL,
		StatusCode: httpResp.StatusCode,
		StatusText: httpResp.Status,
		Headers:    respHeaders,
		Body:       string(respBody),
		Size:       int64(len(respBody)),
		Duration:   elapsed,
	}, nil
}
