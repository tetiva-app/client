package requester

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/tetiva-app/client/internal/domain/usecase/request"
)

const maxQueryDepth = 3

// GenerateExampleQuery generates an example GraphQL query string and variables JSON
// for the given operation name found in the provided schema.
func (r *GraphQLRequester) GenerateExampleQuery(schema *request.GraphQLSchema, operationName string) (*request.GraphQLExampleResponse, error) {
	const funcName = "GraphQLRequester.GenerateExampleQuery"

	typeIndex := buildTypeIndex(schema)

	var op *request.GraphQLOperation
	isMutation := false

	for i := range schema.Queries {
		if schema.Queries[i].Name == operationName {
			op = &schema.Queries[i]
			break
		}
	}
	if op == nil {
		for i := range schema.Mutations {
			if schema.Mutations[i].Name == operationName {
				op = &schema.Mutations[i]
				isMutation = true
				break
			}
		}
	}
	if op == nil {
		return nil, fmt.Errorf("%s: operation %q not found in schema", funcName, operationName)
	}

	varDecls, callArgs := buildArgParts(op.Args)

	keyword := "query"
	if isMutation {
		keyword = "mutation"
	}

	var sb strings.Builder
	sb.WriteString(keyword)
	sb.WriteString(" ")
	sb.WriteString(operationName)

	if len(varDecls) > 0 {
		sb.WriteString("(")
		sb.WriteString(strings.Join(varDecls, ", "))
		sb.WriteString(")")
	}
	sb.WriteString(" {\n")

	sb.WriteString("  ")
	sb.WriteString(op.Name)
	if len(callArgs) > 0 {
		sb.WriteString("(")
		sb.WriteString(strings.Join(callArgs, ", "))
		sb.WriteString(")")
	}

	returnTypeName := stripModifiers(op.ReturnType)
	visited := map[string]bool{}
	selectionSet := buildSelectionSet(returnTypeName, typeIndex, 1, visited)
	if selectionSet != "" {
		sb.WriteString(" ")
		sb.WriteString(selectionSet)
	}
	sb.WriteString("\n}")

	varsMap := buildVariablesMap(op.Args, typeIndex)
	varsJSON, err := json.Marshal(varsMap)
	if err != nil {
		return nil, fmt.Errorf("%s: failed to marshal variables: %w", funcName, err)
	}

	return &request.GraphQLExampleResponse{
		Query:     sb.String(),
		Variables: string(varsJSON),
	}, nil
}

func buildTypeIndex(schema *request.GraphQLSchema) map[string]request.GraphQLType {
	idx := make(map[string]request.GraphQLType, len(schema.Types))
	for _, t := range schema.Types {
		idx[t.Name] = t
	}
	return idx
}

// buildArgParts returns parallel slices: varDecls "$name: Type" for the
// operation signature and callArgs "name: $name" for the call site.
func buildArgParts(args []request.GraphQLArg) (varDecls, callArgs []string) {
	for _, arg := range args {
		varDecls = append(varDecls, "$"+arg.Name+": "+arg.Type)
		callArgs = append(callArgs, arg.Name+": $"+arg.Name)
	}
	return varDecls, callArgs
}

// buildSelectionSet builds a selection set bounded by maxQueryDepth and
// cycle-protected via visited. Returns "" for scalars and unknown types.
func buildSelectionSet(typeName string, typeIndex map[string]request.GraphQLType, depth int, visited map[string]bool) string {
	t, ok := typeIndex[typeName]
	if !ok {
		// Built-in scalars are not in the index.
		return ""
	}

	switch t.Kind {
	case "SCALAR", "ENUM":
		return ""
	case "UNION", "INTERFACE":
		return buildUnionSelectionSet(t, typeIndex, depth, visited)
	case "OBJECT":
		return buildObjectSelectionSet(t, typeIndex, depth, visited)
	default:
		return ""
	}
}

func buildObjectSelectionSet(t request.GraphQLType, typeIndex map[string]request.GraphQLType, depth int, visited map[string]bool) string {
	if len(t.Fields) == 0 {
		return ""
	}

	indent := strings.Repeat("  ", depth)
	closingIndent := strings.Repeat("  ", depth-1)

	var sb strings.Builder
	sb.WriteString("{\n")

	atDepthLimit := depth >= maxQueryDepth

	for _, field := range t.Fields {
		fieldTypeName := stripModifiers(field.Type)
		fieldType, exists := typeIndex[fieldTypeName]

		isScalarOrEnum := !exists || fieldType.Kind == "SCALAR" || fieldType.Kind == "ENUM"

		if atDepthLimit {
			if isScalarOrEnum {
				sb.WriteString(indent)
				sb.WriteString(field.Name)
				sb.WriteString("\n")
			}
			continue
		}

		if isScalarOrEnum {
			sb.WriteString(indent)
			sb.WriteString(field.Name)
			sb.WriteString("\n")
			continue
		}

		if visited[fieldTypeName] {
			continue
		}
		visited[fieldTypeName] = true
		nested := buildSelectionSet(fieldTypeName, typeIndex, depth+1, visited)
		delete(visited, fieldTypeName)

		if nested != "" {
			sb.WriteString(indent)
			sb.WriteString(field.Name)
			sb.WriteString(" ")
			sb.WriteString(nested)
			sb.WriteString("\n")
		} else {
			// Nested type had no fields — treat like scalar.
			sb.WriteString(indent)
			sb.WriteString(field.Name)
			sb.WriteString("\n")
		}
	}

	sb.WriteString(closingIndent)
	sb.WriteString("}")
	return sb.String()
}

// buildUnionSelectionSet builds inline fragments for UNION and INTERFACE types.
func buildUnionSelectionSet(t request.GraphQLType, typeIndex map[string]request.GraphQLType, depth int, visited map[string]bool) string {
	if len(t.PossibleTypes) == 0 {
		return ""
	}

	indent := strings.Repeat("  ", depth)
	closingIndent := strings.Repeat("  ", depth-1)

	var sb strings.Builder
	sb.WriteString("{\n")

	for _, possibleTypeName := range t.PossibleTypes {
		possibleType, ok := typeIndex[possibleTypeName]
		if !ok {
			continue
		}

		sb.WriteString(indent)
		sb.WriteString("... on ")
		sb.WriteString(possibleTypeName)
		sb.WriteString(" {\n")

		innerIndent := strings.Repeat("  ", depth+1)
		for _, field := range possibleType.Fields {
			sb.WriteString(innerIndent)
			sb.WriteString(field.Name)
			sb.WriteString("\n")
		}

		sb.WriteString(indent)
		sb.WriteString("}\n")
	}

	sb.WriteString(closingIndent)
	sb.WriteString("}")
	return sb.String()
}

// stripModifiers removes GraphQL type modifiers (!, [, ]) to get the base type name.
// e.g. "[User!]!" → "User", "ID!" → "ID"
func stripModifiers(t string) string {
	t = strings.ReplaceAll(t, "!", "")
	t = strings.ReplaceAll(t, "[", "")
	t = strings.ReplaceAll(t, "]", "")
	return strings.TrimSpace(t)
}

func buildVariablesMap(args []request.GraphQLArg, typeIndex map[string]request.GraphQLType) map[string]interface{} {
	result := make(map[string]interface{}, len(args))
	for _, arg := range args {
		result[arg.Name] = placeholderValue(arg.Type, typeIndex)
	}
	return result
}

func placeholderValue(typStr string, typeIndex map[string]request.GraphQLType) interface{} {
	stripped := typStr
	stripped = strings.ReplaceAll(stripped, "!", "")
	stripped = strings.TrimSpace(stripped)
	if strings.HasPrefix(stripped, "[") {
		return []interface{}{}
	}

	baseName := stripModifiers(typStr)

	switch baseName {
	case "String":
		return ""
	case "Int":
		return 0
	case "Float":
		return 0.0
	case "Boolean":
		return false
	case "ID":
		return ""
	}

	t, ok := typeIndex[baseName]
	if !ok {
		return nil
	}

	switch t.Kind {
	case "ENUM":
		if len(t.EnumValues) > 0 {
			return t.EnumValues[0]
		}
		return ""
	case "INPUT_OBJECT":
		obj := make(map[string]interface{}, len(t.Fields))
		for _, field := range t.Fields {
			obj[field.Name] = placeholderValue(field.Type, typeIndex)
		}
		return obj
	case "SCALAR":
		// Custom scalar — empty string is a safe default.
		return ""
	default:
		return nil
	}
}
