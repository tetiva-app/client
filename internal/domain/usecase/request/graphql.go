package request

import (
	"context"
	"fmt"
)

const graphqlFuncPrefix = "request.usecase"

func (u *usecase) GraphQLIntrospect(ctx context.Context, req GraphQLIntrospectRequest) (*GraphQLSchema, error) {
	vars, err := u.schemaVariables(ctx, req.WorkspaceID)
	if err != nil {
		return nil, fmt.Errorf("%s.GraphQLIntrospect: %w", graphqlFuncPrefix, err)
	}
	req.Endpoint = substituteVariables(req.Endpoint, vars)
	req.Headers = substituteHeaders(req.Headers, vars)
	return u.graphqlRequester.Introspect(ctx, req)
}

func (u *usecase) GraphQLGenerateExample(ctx context.Context, req GraphQLIntrospectRequest, operationName string) (*GraphQLExampleResponse, error) {
	schema, err := u.GraphQLIntrospect(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("%s.GraphQLGenerateExample: %w", graphqlFuncPrefix, err)
	}
	return u.graphqlRequester.GenerateExampleQuery(schema, operationName)
}

func (u *usecase) GraphQLGetTypeDefinition(ctx context.Context, req GraphQLIntrospectRequest, typeName string) (string, error) {
	schema, err := u.GraphQLIntrospect(ctx, req)
	if err != nil {
		return "", fmt.Errorf("%s.GraphQLGetTypeDefinition: %w", graphqlFuncPrefix, err)
	}

	for _, t := range schema.Types {
		if t.Name == typeName {
			return t.Definition, nil
		}
	}

	return "", fmt.Errorf("%s.GraphQLGetTypeDefinition: type %q not found", graphqlFuncPrefix, typeName)
}
