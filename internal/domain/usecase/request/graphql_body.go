package request

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
)

const graphQLVariablesWarning = "GraphQL variables are not valid JSON; shown as written"

func GraphQLBody(query, variables, operationName string) (string, error) {
	const funcName = "request.GraphQLBody"

	var vars []byte
	if !blankVariables(variables) {
		var buf bytes.Buffer
		if err := json.Compact(&buf, []byte(variables)); err != nil {
			return "", fmt.Errorf("%s: invalid variables: %w", funcName, err)
		}
		vars = buf.Bytes()
	}
	return graphQLBody(query, vars, operationName), nil
}

func GraphQLBodyLenient(query, variables, operationName string) (string, []string) {
	if body, err := GraphQLBody(query, variables, operationName); err == nil {
		return body, nil
	}
	return graphQLBody(query, []byte(strings.TrimSpace(variables)), operationName), []string{graphQLVariablesWarning}
}

func blankVariables(v string) bool {
	v = strings.TrimSpace(v)
	return v == "" || v == "{}"
}

// Written by hand: a map would sort away the query, variables, operationName order.
func graphQLBody(query string, variables []byte, operationName string) string {
	var b strings.Builder
	b.WriteString(`{"query":`)
	b.WriteString(graphQLString(query))
	if len(variables) > 0 {
		b.WriteString(`,"variables":`)
		b.Write(variables)
	}
	if operationName != "" {
		b.WriteString(`,"operationName":`)
		b.WriteString(graphQLString(operationName))
	}
	b.WriteByte('}')
	return b.String()
}

// graphQLString leaves <, > and & as they are, as the variables that go in verbatim do.
func graphQLString(s string) string {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	// A string always encodes.
	_ = enc.Encode(s)
	return strings.TrimSuffix(buf.String(), "\n")
}
