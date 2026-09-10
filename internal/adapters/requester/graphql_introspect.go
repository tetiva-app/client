package requester

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	gqlparser "github.com/vektah/gqlparser/v2"
	"github.com/vektah/gqlparser/v2/ast"

	"github.com/tetiva-app/client/internal/domain/usecase/request"
)

const introspectionQuery = `
query IntrospectionQuery {
  __schema {
    queryType { name }
    mutationType { name }
    types {
      name
      kind
      description
      fields(includeDeprecated: true) {
        name
        args {
          name
          type { ...TypeRef }
          defaultValue
        }
        type { ...TypeRef }
      }
      inputFields {
        name
        type { ...TypeRef }
        defaultValue
      }
      enumValues(includeDeprecated: true) {
        name
      }
      possibleTypes {
        ...TypeRef
      }
    }
  }
}

fragment TypeRef on __Type {
  kind
  name
  ofType {
    kind
    name
    ofType {
      kind
      name
      ofType {
        kind
        name
        ofType {
          kind
          name
        }
      }
    }
  }
}
`

type introspectionResponse struct {
	Data struct {
		Schema introspectionSchema `json:"__schema"`
	} `json:"data"`
	Errors []struct {
		Message string `json:"message"`
	} `json:"errors"`
}

type introspectionSchema struct {
	QueryType    *introspectionTypeRef `json:"queryType"`
	MutationType *introspectionTypeRef `json:"mutationType"`
	Types        []introspectionType   `json:"types"`
}

type introspectionTypeRef struct {
	Name string `json:"name"`
}

type introspectionType struct {
	Name          string                     `json:"name"`
	Kind          string                     `json:"kind"`
	Description   string                     `json:"description"`
	Fields        []introspectionField       `json:"fields"`
	InputFields   []introspectionArg         `json:"inputFields"`
	EnumValues    []introspectionEnumVal     `json:"enumValues"`
	PossibleTypes []introspectionTypeRefFull `json:"possibleTypes"`
}

type introspectionField struct {
	Name string                   `json:"name"`
	Type introspectionTypeRefFull `json:"type"`
	Args []introspectionArg       `json:"args"`
}

type introspectionArg struct {
	Name         string                   `json:"name"`
	Type         introspectionTypeRefFull `json:"type"`
	DefaultValue *string                  `json:"defaultValue"`
}

type introspectionEnumVal struct {
	Name string `json:"name"`
}

// introspectionTypeRefFull mirrors the nested ofType chain from the TypeRef fragment.
type introspectionTypeRefFull struct {
	Kind   string                    `json:"kind"`
	Name   string                    `json:"name"`
	OfType *introspectionTypeRefFull `json:"ofType"`
}

// Exactly one of req.Endpoint or req.SchemaPath must be non-empty.
func (r *GraphQLRequester) Introspect(ctx context.Context, req request.GraphQLIntrospectRequest) (*request.GraphQLSchema, error) {
	const funcName = "GraphQLRequester.Introspect"

	if req.Endpoint != "" && req.SchemaPath != "" {
		return nil, fmt.Errorf("%s: exactly one of Endpoint or SchemaPath must be set, got both", funcName)
	}
	if req.Endpoint == "" && req.SchemaPath == "" {
		return nil, fmt.Errorf("%s: exactly one of Endpoint or SchemaPath must be set, got neither", funcName)
	}

	if req.Endpoint != "" {
		schema, err := r.introspectFromEndpoint(ctx, req)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", funcName, err)
		}
		return schema, nil
	}

	schema, err := r.introspectFromFile(req.SchemaPath)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", funcName, err)
	}
	return schema, nil
}

func (r *GraphQLRequester) introspectFromEndpoint(ctx context.Context, req request.GraphQLIntrospectRequest) (*request.GraphQLSchema, error) {
	const funcName = "introspectFromEndpoint"

	payload := graphqlRequestBody{
		Query: introspectionQuery,
	}
	bodyBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("%s: failed to marshal introspection query: %w", funcName, err)
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

	httpResp, err := r.clientFor(ctx, req.WorkspaceID).Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("%s: request failed: %w", funcName, err)
	}
	defer func() { _ = httpResp.Body.Close() }()

	respBody, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return nil, fmt.Errorf("%s: failed to read response: %w", funcName, err)
	}

	var introspResp introspectionResponse
	if err := json.Unmarshal(respBody, &introspResp); err != nil {
		return nil, fmt.Errorf("%s: failed to unmarshal introspection response: %w", funcName, err)
	}

	if len(introspResp.Errors) > 0 {
		return nil, fmt.Errorf("%s: introspection returned errors: %s", funcName, introspResp.Errors[0].Message)
	}

	return convertIntrospectionSchema(&introspResp.Data.Schema), nil
}

func (r *GraphQLRequester) introspectFromFile(schemaPath string) (*request.GraphQLSchema, error) {
	const funcName = "introspectFromFile"

	data, err := os.ReadFile(schemaPath)
	if err != nil {
		return nil, fmt.Errorf("%s: failed to read schema file: %w", funcName, err)
	}

	src := &ast.Source{
		Name:  schemaPath,
		Input: string(data),
	}

	schema, err := gqlparser.LoadSchema(src)
	if err != nil {
		return nil, fmt.Errorf("%s: failed to parse schema: %w", funcName, err)
	}

	return convertASTSchema(schema), nil
}

func convertIntrospectionSchema(s *introspectionSchema) *request.GraphQLSchema {
	queryTypeName := ""
	if s.QueryType != nil {
		queryTypeName = s.QueryType.Name
	}
	mutationTypeName := ""
	if s.MutationType != nil {
		mutationTypeName = s.MutationType.Name
	}

	result := &request.GraphQLSchema{
		Source: "introspection",
	}

	for _, t := range s.Types {
		if strings.HasPrefix(t.Name, "__") {
			continue
		}

		switch t.Name {
		case queryTypeName:
			for _, f := range t.Fields {
				op := convertIntrospectionFieldToOperation(f)
				result.Queries = append(result.Queries, op)
			}
		case mutationTypeName:
			for _, f := range t.Fields {
				op := convertIntrospectionFieldToOperation(f)
				result.Mutations = append(result.Mutations, op)
			}
		default:
			gt := convertIntrospectionType(t)
			result.Types = append(result.Types, gt)
		}
	}

	return result
}

func convertIntrospectionFieldToOperation(f introspectionField) request.GraphQLOperation {
	args := make([]request.GraphQLArg, 0, len(f.Args))
	for _, a := range f.Args {
		arg := request.GraphQLArg{
			Name: a.Name,
			Type: flattenTypeRef(&a.Type),
		}
		if a.DefaultValue != nil {
			arg.DefaultValue = *a.DefaultValue
		}
		args = append(args, arg)
	}

	returnType := flattenTypeRef(&f.Type)

	def := buildOperationSDL(f.Name, args, returnType)

	return request.GraphQLOperation{
		Name:       f.Name,
		Args:       args,
		ReturnType: returnType,
		Definition: def,
	}
}

func convertIntrospectionType(t introspectionType) request.GraphQLType {
	gt := request.GraphQLType{
		Name: t.Name,
		Kind: t.Kind,
	}

	switch t.Kind {
	case "OBJECT", "INTERFACE":
		for _, f := range t.Fields {
			args := make([]request.GraphQLArg, 0, len(f.Args))
			for _, a := range f.Args {
				arg := request.GraphQLArg{
					Name: a.Name,
					Type: flattenTypeRef(&a.Type),
				}
				if a.DefaultValue != nil {
					arg.DefaultValue = *a.DefaultValue
				}
				args = append(args, arg)
			}
			gt.Fields = append(gt.Fields, request.GraphQLField{
				Name: f.Name,
				Type: flattenTypeRef(&f.Type),
				Args: args,
			})
		}
		for _, pt := range t.PossibleTypes {
			if pt.Name != "" {
				gt.PossibleTypes = append(gt.PossibleTypes, pt.Name)
			}
		}

	case "INPUT_OBJECT":
		for _, f := range t.InputFields {
			gt.Fields = append(gt.Fields, request.GraphQLField{
				Name: f.Name,
				Type: flattenTypeRef(&f.Type),
			})
		}

	case "ENUM":
		for _, ev := range t.EnumValues {
			gt.EnumValues = append(gt.EnumValues, ev.Name)
		}

	case "UNION":
		for _, pt := range t.PossibleTypes {
			if pt.Name != "" {
				gt.PossibleTypes = append(gt.PossibleTypes, pt.Name)
			}
		}
	}

	gt.Definition = buildTypeSDL(gt)
	return gt
}

// e.g. NON_NULL(LIST(NON_NULL(String))) → "[String!]!"
func flattenTypeRef(t *introspectionTypeRefFull) string {
	if t == nil {
		return ""
	}
	switch t.Kind {
	case "NON_NULL":
		inner := flattenTypeRef(t.OfType)
		return inner + "!"
	case "LIST":
		inner := flattenTypeRef(t.OfType)
		return "[" + inner + "]"
	default:
		return t.Name
	}
}

func convertASTSchema(schema *ast.Schema) *request.GraphQLSchema {
	result := &request.GraphQLSchema{
		Source: "schema_file",
	}

	queryTypeName := "Query"
	mutationTypeName := "Mutation"
	if schema.Query != nil {
		queryTypeName = schema.Query.Name
	}
	if schema.Mutation != nil {
		mutationTypeName = schema.Mutation.Name
	}

	for name, def := range schema.Types {
		if def.BuiltIn {
			continue
		}
		if strings.HasPrefix(name, "__") {
			continue
		}

		switch name {
		case queryTypeName:
			for _, f := range def.Fields {
				op := convertASTFieldToOperation(f)
				result.Queries = append(result.Queries, op)
			}
		case mutationTypeName:
			for _, f := range def.Fields {
				op := convertASTFieldToOperation(f)
				result.Mutations = append(result.Mutations, op)
			}
		default:
			gt := convertASTDefinition(def)
			result.Types = append(result.Types, gt)
		}
	}

	return result
}

func convertASTFieldToOperation(f *ast.FieldDefinition) request.GraphQLOperation {
	args := make([]request.GraphQLArg, 0, len(f.Arguments))
	for _, a := range f.Arguments {
		arg := request.GraphQLArg{
			Name: a.Name,
			Type: a.Type.String(),
		}
		if a.DefaultValue != nil {
			arg.DefaultValue = a.DefaultValue.String()
		}
		args = append(args, arg)
	}

	returnType := f.Type.String()
	def := buildOperationSDL(f.Name, args, returnType)

	return request.GraphQLOperation{
		Name:       f.Name,
		Args:       args,
		ReturnType: returnType,
		Definition: def,
	}
}

func convertASTDefinition(def *ast.Definition) request.GraphQLType {
	gt := request.GraphQLType{
		Name: def.Name,
		Kind: string(def.Kind),
	}

	switch def.Kind {
	case ast.Object, ast.Interface:
		for _, f := range def.Fields {
			args := make([]request.GraphQLArg, 0, len(f.Arguments))
			for _, a := range f.Arguments {
				arg := request.GraphQLArg{
					Name: a.Name,
					Type: a.Type.String(),
				}
				if a.DefaultValue != nil {
					arg.DefaultValue = a.DefaultValue.String()
				}
				args = append(args, arg)
			}
			gt.Fields = append(gt.Fields, request.GraphQLField{
				Name: f.Name,
				Type: f.Type.String(),
				Args: args,
			})
		}

	case ast.InputObject:
		for _, f := range def.Fields {
			gt.Fields = append(gt.Fields, request.GraphQLField{
				Name: f.Name,
				Type: f.Type.String(),
			})
		}

	case ast.Enum:
		for _, ev := range def.EnumValues {
			gt.EnumValues = append(gt.EnumValues, ev.Name)
		}

	case ast.Union:
		gt.PossibleTypes = append(gt.PossibleTypes, def.Types...)
	}

	gt.Definition = buildTypeSDL(gt)
	return gt
}

// e.g. "users(limit: Int, offset: Int): [User!]!"
func buildOperationSDL(name string, args []request.GraphQLArg, returnType string) string {
	var sb strings.Builder
	sb.WriteString(name)
	if len(args) > 0 {
		sb.WriteString("(")
		for i, arg := range args {
			if i > 0 {
				sb.WriteString(", ")
			}
			sb.WriteString(arg.Name)
			sb.WriteString(": ")
			sb.WriteString(arg.Type)
			if arg.DefaultValue != "" {
				sb.WriteString(" = ")
				sb.WriteString(arg.DefaultValue)
			}
		}
		sb.WriteString(")")
	}
	sb.WriteString(": ")
	sb.WriteString(returnType)
	return sb.String()
}

func buildTypeSDL(t request.GraphQLType) string {
	var sb strings.Builder

	switch t.Kind {
	case "OBJECT", "INTERFACE":
		keyword := "type"
		if t.Kind == "INTERFACE" {
			keyword = "interface"
		}
		sb.WriteString(keyword)
		sb.WriteString(" ")
		sb.WriteString(t.Name)
		sb.WriteString(" {\n")
		for _, f := range t.Fields {
			sb.WriteString("  ")
			sb.WriteString(f.Name)
			if len(f.Args) > 0 {
				sb.WriteString("(")
				for i, a := range f.Args {
					if i > 0 {
						sb.WriteString(", ")
					}
					sb.WriteString(a.Name)
					sb.WriteString(": ")
					sb.WriteString(a.Type)
				}
				sb.WriteString(")")
			}
			sb.WriteString(": ")
			sb.WriteString(f.Type)
			sb.WriteString("\n")
		}
		sb.WriteString("}")

	case "INPUT_OBJECT":
		sb.WriteString("input ")
		sb.WriteString(t.Name)
		sb.WriteString(" {\n")
		for _, f := range t.Fields {
			sb.WriteString("  ")
			sb.WriteString(f.Name)
			sb.WriteString(": ")
			sb.WriteString(f.Type)
			sb.WriteString("\n")
		}
		sb.WriteString("}")

	case "ENUM":
		sb.WriteString("enum ")
		sb.WriteString(t.Name)
		sb.WriteString(" {\n")
		for _, v := range t.EnumValues {
			sb.WriteString("  ")
			sb.WriteString(v)
			sb.WriteString("\n")
		}
		sb.WriteString("}")

	case "UNION":
		sb.WriteString("union ")
		sb.WriteString(t.Name)
		if len(t.PossibleTypes) > 0 {
			sb.WriteString(" = ")
			sb.WriteString(strings.Join(t.PossibleTypes, " | "))
		}

	case "SCALAR":
		sb.WriteString("scalar ")
		sb.WriteString(t.Name)
	}

	return sb.String()
}
