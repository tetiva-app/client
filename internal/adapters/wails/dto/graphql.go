package dto

import "github.com/tetiva-app/client/internal/domain/usecase/request"

type GraphQLIntrospectRequest struct {
	Endpoint    string            `json:"endpoint"`
	SchemaPath  string            `json:"schemaPath"`
	Headers     map[string]string `json:"headers"`
	WorkspaceID string            `json:"workspaceId"`
}

type GraphQLGenerateExampleRequest struct {
	Endpoint      string            `json:"endpoint"`
	SchemaPath    string            `json:"schemaPath"`
	Headers       map[string]string `json:"headers"`
	WorkspaceID   string            `json:"workspaceId"`
	OperationName string            `json:"operationName"`
}

type GraphQLGetTypeDefinitionRequest struct {
	Endpoint    string            `json:"endpoint"`
	SchemaPath  string            `json:"schemaPath"`
	Headers     map[string]string `json:"headers"`
	WorkspaceID string            `json:"workspaceId"`
	TypeName    string            `json:"typeName"`
}

type GraphQLSchemaResponse struct {
	Queries   []GraphQLOperationResponse `json:"queries"`
	Mutations []GraphQLOperationResponse `json:"mutations"`
	Types     []GraphQLTypeResponse      `json:"types"`
	Source    string                     `json:"source"`
}

type GraphQLOperationResponse struct {
	Name       string               `json:"name"`
	Args       []GraphQLArgResponse `json:"args"`
	ReturnType string               `json:"returnType"`
	Definition string               `json:"definition"`
}

type GraphQLArgResponse struct {
	Name         string `json:"name"`
	Type         string `json:"type"`
	DefaultValue string `json:"defaultValue"`
}

type GraphQLTypeResponse struct {
	Name          string                 `json:"name"`
	Kind          string                 `json:"kind"`
	Fields        []GraphQLFieldResponse `json:"fields"`
	EnumValues    []string               `json:"enumValues"`
	PossibleTypes []string               `json:"possibleTypes"`
	Definition    string                 `json:"definition"`
}

type GraphQLFieldResponse struct {
	Name string               `json:"name"`
	Type string               `json:"type"`
	Args []GraphQLArgResponse `json:"args"`
}

type GraphQLExampleResponseDTO struct {
	Query     string `json:"query"`
	Variables string `json:"variables"`
}

func GraphQLSchemaToResponse(s *request.GraphQLSchema) GraphQLSchemaResponse {
	queries := make([]GraphQLOperationResponse, len(s.Queries))
	for i, q := range s.Queries {
		queries[i] = graphqlOperationToResponse(q)
	}

	mutations := make([]GraphQLOperationResponse, len(s.Mutations))
	for i, m := range s.Mutations {
		mutations[i] = graphqlOperationToResponse(m)
	}

	types := make([]GraphQLTypeResponse, len(s.Types))
	for i, t := range s.Types {
		types[i] = graphqlTypeToResponse(t)
	}

	return GraphQLSchemaResponse{
		Queries:   queries,
		Mutations: mutations,
		Types:     types,
		Source:    s.Source,
	}
}

func graphqlOperationToResponse(op request.GraphQLOperation) GraphQLOperationResponse {
	args := make([]GraphQLArgResponse, len(op.Args))
	for i, a := range op.Args {
		args[i] = GraphQLArgResponse{Name: a.Name, Type: a.Type, DefaultValue: a.DefaultValue}
	}
	return GraphQLOperationResponse{
		Name:       op.Name,
		Args:       args,
		ReturnType: op.ReturnType,
		Definition: op.Definition,
	}
}

func graphqlTypeToResponse(t request.GraphQLType) GraphQLTypeResponse {
	fields := make([]GraphQLFieldResponse, len(t.Fields))
	for i, f := range t.Fields {
		args := make([]GraphQLArgResponse, len(f.Args))
		for j, a := range f.Args {
			args[j] = GraphQLArgResponse{Name: a.Name, Type: a.Type, DefaultValue: a.DefaultValue}
		}
		fields[i] = GraphQLFieldResponse{Name: f.Name, Type: f.Type, Args: args}
	}

	enumValues := t.EnumValues
	if enumValues == nil {
		enumValues = []string{}
	}

	possibleTypes := t.PossibleTypes
	if possibleTypes == nil {
		possibleTypes = []string{}
	}

	return GraphQLTypeResponse{
		Name:          t.Name,
		Kind:          t.Kind,
		Fields:        fields,
		EnumValues:    enumValues,
		PossibleTypes: possibleTypes,
		Definition:    t.Definition,
	}
}
