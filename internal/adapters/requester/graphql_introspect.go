package requester

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
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
        description
        args {
          name
          type { ...TypeRef }
          defaultValue
        }
        type { ...TypeRef }
      }
      inputFields {
        name
        description
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
	Name        string                   `json:"name"`
	Description string                   `json:"description"`
	Type        introspectionTypeRefFull `json:"type"`
	Args        []introspectionArg       `json:"args"`
}

type introspectionArg struct {
	Name         string                   `json:"name"`
	Description  string                   `json:"description"`
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

func (r *GraphQLRequester) Introspect(ctx context.Context, req request.GraphQLIntrospectRequest) (*request.GraphQLSchema, error) {
	const funcName = "GraphQLRequester.Introspect"

	if req.Endpoint == "" && req.SchemaPath == "" {
		return nil, fmt.Errorf("%s: neither Endpoint nor SchemaPath is set", funcName)
	}

	if req.SchemaPath != "" {
		schema, fileErr := r.introspectFromFile(req.SchemaPath)
		if fileErr == nil {
			return schema, nil
		}
		if req.Endpoint == "" || !schemaFileMissing(fileErr) {
			return nil, fmt.Errorf("%s: %w", funcName, fileErr)
		}
		schema, err := r.introspectFromEndpoint(ctx, req)
		if err != nil {
			return nil, fmt.Errorf("%s: %w; introspecting the URL instead: %w", funcName, fileErr, err)
		}
		return schema, nil
	}

	schema, err := r.introspectFromEndpoint(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", funcName, err)
	}
	return schema, nil
}

func schemaFileMissing(err error) bool {
	return errors.Is(err, fs.ErrNotExist) || errors.Is(err, errNetworkSchemaPath)
}

func (r *GraphQLRequester) introspectFromEndpoint(ctx context.Context, req request.GraphQLIntrospectRequest) (*request.GraphQLSchema, error) {
	const funcName = "introspectFromEndpoint"

	body, err := request.GraphQLBody(introspectionQuery, "", "")
	if err != nil {
		return nil, fmt.Errorf("%s: %w", funcName, err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, req.Endpoint, strings.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("%s: failed to create HTTP request: %w", funcName, redactURLError(err, nil))
	}

	httpReq.Header.Set("Content-Type", "application/json")
	for key, values := range req.Headers {
		for _, v := range values {
			httpReq.Header.Add(key, v)
		}
	}

	httpResp, err := r.clientFor(ctx, req.WorkspaceID).Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("%s: request failed: %w", funcName, redactURLError(err, nil))
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

	data, err := readSchemaFile(schemaPath)
	if err != nil {
		return nil, fmt.Errorf("%s: failed to read schema file: %w", funcName, err)
	}
	data = bytes.TrimPrefix(data, []byte("\ufeff"))

	if trimmed := bytes.TrimSpace(data); len(trimmed) > 0 && trimmed[0] == '{' {
		schema, err := schemaFromIntrospectionJSON(trimmed)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", funcName, err)
		}
		return schema, nil
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

// Accepts both a saved introspection response and the bare {"__schema": …} that graphql-js and Apollo write.
func schemaFromIntrospectionJSON(data []byte) (*request.GraphQLSchema, error) {
	const funcName = "schemaFromIntrospectionJSON"

	var file struct {
		Data struct {
			Schema *introspectionSchema `json:"__schema"`
		} `json:"data"`
		Schema *introspectionSchema `json:"__schema"`
	}
	if err := json.Unmarshal(data, &file); err != nil {
		return nil, fmt.Errorf("%s: failed to parse introspection JSON: %w", funcName, err)
	}

	parsed := file.Schema
	if parsed == nil {
		parsed = file.Data.Schema
	}
	if parsed == nil {
		return nil, fmt.Errorf("%s: no __schema in introspection JSON", funcName)
	}

	schema := convertIntrospectionSchema(parsed)
	schema.Source = "schema_file"
	return schema, nil
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

	op := request.GraphQLOperation{
		Name:        f.Name,
		Description: f.Description,
		Args:        args,
		ReturnType:  flattenTypeRef(&f.Type),
	}
	op.Definition = buildOperationSDL(op)
	return op
}

func convertIntrospectionType(t introspectionType) request.GraphQLType {
	gt := request.GraphQLType{
		Name:        t.Name,
		Description: t.Description,
		Kind:        t.Kind,
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
				Name:        f.Name,
				Description: f.Description,
				Type:        flattenTypeRef(&f.Type),
				Args:        args,
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
				Name:        f.Name,
				Description: f.Description,
				Type:        flattenTypeRef(&f.Type),
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
				// gqlparser adds __schema and __type to the query root.
				if strings.HasPrefix(f.Name, "__") {
					continue
				}
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
			arg.DefaultValue = sdlValue(a.DefaultValue)
		}
		args = append(args, arg)
	}

	op := request.GraphQLOperation{
		Name:        f.Name,
		Description: f.Description,
		Args:        args,
		ReturnType:  f.Type.String(),
	}
	op.Definition = buildOperationSDL(op)
	return op
}

func convertASTDefinition(def *ast.Definition) request.GraphQLType {
	gt := request.GraphQLType{
		Name:        def.Name,
		Description: def.Description,
		Kind:        string(def.Kind),
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
					arg.DefaultValue = sdlValue(a.DefaultValue)
				}
				args = append(args, arg)
			}
			gt.Fields = append(gt.Fields, request.GraphQLField{
				Name:        f.Name,
				Description: f.Description,
				Type:        f.Type.String(),
				Args:        args,
			})
		}

	case ast.InputObject:
		for _, f := range def.Fields {
			gt.Fields = append(gt.Fields, request.GraphQLField{
				Name:        f.Name,
				Description: f.Description,
				Type:        f.Type.String(),
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
func buildOperationSDL(op request.GraphQLOperation) string {
	var sb strings.Builder
	writeDescription(&sb, op.Description, "")
	sb.WriteString(op.Name)
	writeArgs(&sb, op.Args)
	sb.WriteString(": ")
	sb.WriteString(op.ReturnType)
	return sb.String()
}

func writeArgs(sb *strings.Builder, args []request.GraphQLArg) {
	if len(args) == 0 {
		return
	}
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

// A trailing " or \ would run into the closing """, so such text gets its own lines.
func writeDescription(sb *strings.Builder, desc, indent string) {
	if desc == "" {
		return
	}
	if !blockStringKeeps(desc) {
		sb.WriteString(indent + quoteGraphQL(desc) + "\n")
		return
	}
	desc = strings.ReplaceAll(desc, `"""`, `\"""`)
	if !strings.Contains(desc, "\n") && !strings.HasSuffix(desc, `"`) && !strings.HasSuffix(desc, `\`) {
		sb.WriteString(indent + `"""` + desc + `"""` + "\n")
		return
	}
	sb.WriteString(indent + `"""` + "\n")
	for _, line := range strings.Split(desc, "\n") {
		if line != "" {
			sb.WriteString(indent + line)
		}
		sb.WriteString("\n")
	}
	sb.WriteString(indent + `"""` + "\n")
}

// Block strings drop edge blank lines and common indentation and fold \r into \n.
func blockStringKeeps(desc string) bool {
	if strings.Contains(desc, "\r") {
		return false
	}
	lines := strings.Split(desc, "\n")
	if strings.TrimSpace(lines[0]) == "" || strings.TrimSpace(lines[len(lines)-1]) == "" {
		return false
	}
	for _, line := range lines {
		if line != "" && strings.TrimLeft(line, " \t") == line {
			return true
		}
	}
	return false
}

// JSON string escapes are a subset of GraphQL's; Go's %q (\x1b) is not.
func quoteGraphQL(s string) string {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	_ = enc.Encode(s)
	return strings.TrimSuffix(buf.String(), "\n")
}

func sdlValue(v *ast.Value) string {
	switch v.Kind {
	case ast.StringValue, ast.BlockValue:
		return quoteGraphQL(v.Raw)
	case ast.ListValue:
		items := make([]string, len(v.Children))
		for i, c := range v.Children {
			items[i] = sdlValue(c.Value)
		}
		return "[" + strings.Join(items, ", ") + "]"
	case ast.ObjectValue:
		fields := make([]string, len(v.Children))
		for i, c := range v.Children {
			fields[i] = c.Name + ": " + sdlValue(c.Value)
		}
		return "{" + strings.Join(fields, ", ") + "}"
	default:
		return v.String()
	}
}

func buildTypeSDL(t request.GraphQLType) string {
	var sb strings.Builder
	writeDescription(&sb, t.Description, "")

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
			writeDescription(&sb, f.Description, "  ")
			sb.WriteString("  ")
			sb.WriteString(f.Name)
			writeArgs(&sb, f.Args)
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
			writeDescription(&sb, f.Description, "  ")
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
