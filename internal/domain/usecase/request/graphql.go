package request

import (
	"context"
	"fmt"
)

const graphqlFuncPrefix = "request.usecase"

// GraphQLIntrospect fetches the GraphQL schema via introspection or from a file.
func (u *usecase) GraphQLIntrospect(ctx context.Context, req GraphQLIntrospectRequest) (*GraphQLSchema, error) {
	return u.graphqlRequester.Introspect(ctx, req)
}

// GraphQLGenerateExample generates an example query and variables for a named operation.
func (u *usecase) GraphQLGenerateExample(ctx context.Context, req GraphQLIntrospectRequest, operationName string) (*GraphQLExampleResponse, error) {
	schema, err := u.graphqlRequester.Introspect(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("%s.GraphQLGenerateExample: %w", graphqlFuncPrefix, err)
	}
	return u.graphqlRequester.GenerateExampleQuery(schema, operationName)
}

// GraphQLGetTypeDefinition returns the SDL definition for a named GraphQL type.
func (u *usecase) GraphQLGetTypeDefinition(ctx context.Context, req GraphQLIntrospectRequest, typeName string) (string, error) {
	schema, err := u.graphqlRequester.Introspect(ctx, req)
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
