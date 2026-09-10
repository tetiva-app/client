package mcp

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/google/uuid"
	mcplib "github.com/mark3labs/mcp-go/mcp"

	"github.com/tetiva-app/client/internal/domain/entities"
)

// Tool results reach the MCP client verbatim, so stored credentials are masked in them.
const redactedValue = "[redacted]"

// maskAuthData keeps the "auth is configured" signal without the credentials.
func maskAuthData(data string) string {
	trimmed := strings.TrimSpace(data)
	if trimmed == "" || trimmed == "{}" {
		return ""
	}
	return redactedValue
}

// sensitiveHeaders carry credentials by convention; their values never reach
// the MCP client. Keys are lowercase, lookups fold case.
var sensitiveHeaders = map[string]struct{}{
	"authorization":        {},
	"proxy-authorization":  {},
	"cookie":               {},
	"set-cookie":           {},
	"x-api-key":            {},
	"api-key":              {},
	"x-auth-token":         {},
	"x-access-token":       {},
	"x-csrf-token":         {},
	"x-session-token":      {},
	"x-amz-security-token": {},
	"x-amz-content-sha256": {},
	// A Digest challenge carries the nonce a replay would need.
	"www-authenticate": {},
}

// sensitiveQueryParams are the query keys whose values are treated as credentials.
var sensitiveQueryParams = map[string]struct{}{
	"token":         {},
	"access_token":  {},
	"refresh_token": {},
	"api_key":       {},
	"apikey":        {},
	"key":           {},
	"secret":        {},
	"password":      {},
	"sig":           {},
	"signature":     {},
	"code":          {},
	"code_verifier": {},
	"client_secret": {},
	"assertion":     {},
	"id_token":      {},
	// The presigned-URL twins of the x-amz-* headers above.
	"x-amz-signature":      {},
	"x-amz-credential":     {},
	"x-amz-security-token": {},
}

// authTypeDoc renders an auth-type registry for a tool description, so a new
// scheme reaches MCP clients without a second edit here.
func authTypeDoc(types []entities.AuthType) string {
	names := make([]string, len(types))
	for i, t := range types {
		names[i] = string(t)
	}
	return strings.Join(names, ", ")
}

// isVariableRef reports whether a value is nothing but a template reference:
// "session=abc; theme={{t}}" still carries a literal secret next to the placeholder.
func isVariableRef(v string) bool {
	t := strings.TrimSpace(v)
	if len(t) <= 4 || !strings.HasPrefix(t, "{{") || !strings.HasSuffix(t, "}}") {
		return false
	}
	return !strings.Contains(t[2:len(t)-2], "}}")
}

func maskSensitiveValue(name, value string, sensitive map[string]struct{}) string {
	if value == "" || isVariableRef(value) {
		return value
	}
	if _, ok := sensitive[strings.ToLower(strings.TrimSpace(name))]; !ok {
		return value
	}
	return redactedValue
}

// maskHeaderItems returns a copy with credential header values redacted.
func maskHeaderItems(items []entities.HeaderItem) []entities.HeaderItem {
	if items == nil {
		return nil
	}
	out := make([]entities.HeaderItem, len(items))
	for i, h := range items {
		h.Value = maskSensitiveValue(h.Key, h.Value, sensitiveHeaders)
		out[i] = h
	}
	return out
}

// maskResponseHeaders is maskHeaderItems for the map form used by responses.
func maskResponseHeaders(headers map[string][]string) map[string][]string {
	if headers == nil {
		return nil
	}
	out := make(map[string][]string, len(headers))
	for k, values := range headers {
		masked := make([]string, len(values))
		for i, v := range values {
			masked[i] = maskSensitiveValue(k, v, sensitiveHeaders)
		}
		out[k] = masked
	}
	return out
}

// paramKey normalizes a query parameter name for lookups: the receiver decodes
// "access%5Ftoken" as "access_token", so classification must see the same name.
func paramKey(name string) string {
	if decoded, err := url.QueryUnescape(name); err == nil {
		name = decoded
	}
	return strings.ToLower(strings.TrimSpace(name))
}

// splitQuery cuts raw into prefix, query and fragment. Written by hand rather
// than via net/url so that "{{var}}" placeholders survive untouched.
func splitQuery(raw string) (prefix, query, suffix string, ok bool) {
	q := strings.IndexByte(raw, '?')
	if q < 0 {
		return "", "", "", false
	}
	prefix, query = raw[:q+1], raw[q+1:]
	if frag := strings.IndexByte(query, '#'); frag >= 0 {
		suffix, query = query[frag:], query[:frag]
	}
	if query == "" {
		return "", "", "", false
	}
	return prefix, query, suffix, true
}

// maskURLSecrets redacts credential query parameters.
func maskURLSecrets(raw string) string {
	prefix, query, suffix, ok := splitQuery(raw)
	if !ok {
		return raw
	}

	parts := strings.Split(query, "&")
	changed := false
	for i, part := range parts {
		eq := strings.IndexByte(part, '=')
		if eq < 0 {
			continue
		}
		name, value := part[:eq], part[eq+1:]
		masked := maskSensitiveValue(paramKey(name), value, sensitiveQueryParams)
		if masked != value {
			parts[i] = name + "=" + masked
			changed = true
		}
	}
	if !changed {
		return raw
	}
	return prefix + strings.Join(parts, "&") + suffix
}

// maskVariableValue redacts variables the user flagged as secret.
func maskVariableValue(value string, isSecret bool) string {
	if !isSecret || value == "" {
		return value
	}
	return redactedValue
}

// redactedEchoErr answers a write carrying the mask itself, which an agent
// working from a redacted read cannot always avoid sending.
func redactedEchoErr(field string) error {
	return fmt.Errorf("%s is %q, which is a mask and not a value: pass the real value or omit the field",
		field, redactedValue)
}

// Masking replaces the whole value, so a mask buried inside a larger one has no stored
// part to splice back and writing it through would persist the marker as the credential.
func redactedEmbeddedErr(field string) error {
	return fmt.Errorf("%s embeds %q, which is a mask and not a value: pass the real value or omit the field",
		field, redactedValue)
}

// rejectMask refuses any value carrying the marker, whole or embedded.
func rejectMask(field, value string) error {
	if !strings.Contains(value, redactedValue) {
		return nil
	}
	if value == redactedValue {
		return redactedEchoErr(field)
	}
	return redactedEmbeddedErr(field)
}

// restoreAuthData answers an auth_data echoed back from a masked read: only the
// whole field is ever masked, so a marker inside JSON is a corrupted write.
func restoreAuthData(input, current string) (string, error) {
	if input == redactedValue {
		return current, nil
	}
	if strings.Contains(input, redactedValue) {
		return "", redactedEmbeddedErr("auth_data")
	}
	return input, nil
}

// queryValues indexes a URL's query by lowercased parameter name.
func queryValues(raw string) map[string][]string {
	_, query, _, ok := splitQuery(raw)
	if !ok {
		return nil
	}
	out := make(map[string][]string)
	for _, part := range strings.Split(query, "&") {
		if eq := strings.IndexByte(part, '='); eq >= 0 {
			key := paramKey(part[:eq])
			out[key] = append(out[key], part[eq+1:])
		}
	}
	return out
}

// restoreURLSecrets puts the stored value back under every query parameter the
// agent echoed as a mask, so a read-then-write keeps the secret.
func restoreURLSecrets(input, current string) (string, error) {
	if !strings.Contains(input, redactedValue) {
		return input, nil
	}
	prefix, query, suffix, ok := splitQuery(input)
	if !ok {
		return "", redactedEchoErr("url")
	}

	stored := queryValues(current)
	seen := make(map[string]int)
	parts := strings.Split(query, "&")
	for i, part := range parts {
		eq := strings.IndexByte(part, '=')
		if eq < 0 {
			continue
		}
		name, value := part[:eq], part[eq+1:]
		key := paramKey(name)
		idx := seen[key]
		seen[key]++
		if value != redactedValue {
			continue
		}
		if idx >= len(stored[key]) {
			return "", redactedEchoErr("url parameter " + name)
		}
		parts[i] = name + "=" + stored[key][idx]
	}
	joined := prefix + strings.Join(parts, "&") + suffix
	if strings.Contains(joined, redactedValue) {
		return "", redactedEchoErr("url")
	}
	return joined, nil
}

// restoreHeaderSecrets is restoreURLSecrets for header items, matched by key.
func restoreHeaderSecrets(input, current []entities.HeaderItem) ([]entities.HeaderItem, error) {
	if input == nil {
		return nil, nil
	}
	stored := make(map[string][]string, len(current))
	for _, h := range current {
		key := strings.ToLower(strings.TrimSpace(h.Key))
		stored[key] = append(stored[key], h.Value)
	}

	out := make([]entities.HeaderItem, len(input))
	copy(out, input)
	seen := make(map[string]int, len(out))
	for i := range out {
		key := strings.ToLower(strings.TrimSpace(out[i].Key))
		idx := seen[key]
		seen[key]++
		if out[i].Value != redactedValue {
			if strings.Contains(out[i].Value, redactedValue) {
				return nil, redactedEmbeddedErr("header " + out[i].Key)
			}
			continue
		}
		if idx >= len(stored[key]) {
			return nil, redactedEchoErr("header " + out[i].Key)
		}
		out[i].Value = stored[key][idx]
	}
	return out, nil
}

// restoreMetadataSecrets is restoreHeaderSecrets for the map form gRPC uses.
func restoreMetadataSecrets(input, current map[string][]string) (map[string][]string, error) {
	if input == nil {
		return nil, nil
	}
	stored := make(map[string][]string, len(current))
	for k, values := range current {
		key := strings.ToLower(strings.TrimSpace(k))
		stored[key] = append(stored[key], values...)
	}

	out := make(map[string][]string, len(input))
	for k, values := range input {
		key := strings.ToLower(strings.TrimSpace(k))
		restored := make([]string, len(values))
		copy(restored, values)
		for i, v := range restored {
			if v != redactedValue {
				if strings.Contains(v, redactedValue) {
					return nil, redactedEmbeddedErr("grpc_metadata " + k)
				}
				continue
			}
			if i >= len(stored[key]) {
				return nil, redactedEchoErr("grpc_metadata " + k)
			}
			restored[i] = stored[key][i]
		}
		out[k] = restored
	}
	return out, nil
}

// rejectRedactedValues guards the create path, where nothing is stored yet to
// restore a mask from.
func rejectRedactedValues(url, authData string, headers []entities.HeaderItem, metadata map[string][]string) error {
	if err := rejectMask("url", url); err != nil {
		return err
	}
	if err := rejectMask("auth_data", authData); err != nil {
		return err
	}
	for _, h := range headers {
		if err := rejectMask("header "+h.Key, h.Value); err != nil {
			return err
		}
	}
	for k, values := range metadata {
		for _, v := range values {
			if err := rejectMask("grpc_metadata "+k, v); err != nil {
				return err
			}
		}
	}
	return nil
}

// optionalString reports whether the argument was actually sent, so update tools can
// leave a field alone instead of blanking it — including deliberate clears to "".
func optionalString(req mcplib.CallToolRequest, name string) (string, bool) {
	v, ok := req.GetArguments()[name]
	if !ok {
		return "", false
	}
	s, ok := v.(string)
	return s, ok
}

// applyStringArgs copies every string argument that was sent into its target field.
func applyStringArgs(req mcplib.CallToolRequest, targets map[string]*string) {
	for name, dst := range targets {
		if v, ok := optionalString(req, name); ok {
			*dst = v
		}
	}
}

// Returns nil when the argument is absent or empty.
func uuidPtrArg(req mcplib.CallToolRequest, name string) (*uuid.UUID, error) {
	raw := stringArg(req, name, "")
	if raw == "" {
		return nil, nil
	}
	id, err := uuid.Parse(raw)
	if err != nil {
		return nil, err
	}
	return &id, nil
}

// Expected format: [{"key": "X-Foo", "value": "bar", "enabled": true}, ...]
func headersFromArg(req mcplib.CallToolRequest, name string) []entities.HeaderItem {
	v, ok := req.GetArguments()[name]
	if !ok {
		return nil
	}
	arr, ok := v.([]any)
	if !ok {
		return nil
	}
	out := make([]entities.HeaderItem, 0, len(arr))
	for _, item := range arr {
		obj, ok := item.(map[string]any)
		if !ok {
			continue
		}
		h := entities.HeaderItem{Enabled: true}
		if k, ok := obj["key"].(string); ok {
			h.Key = k
		}
		if val, ok := obj["value"].(string); ok {
			h.Value = val
		}
		if en, ok := obj["enabled"].(bool); ok {
			h.Enabled = en
		}
		if h.Key == "" {
			continue
		}
		out = append(out, h)
	}
	return out
}

// Accepts either [{"key","value"}, ...] or {"key": ["v1","v2"]}.
func grpcMetadataFromArg(req mcplib.CallToolRequest, name string) map[string][]string {
	v, ok := req.GetArguments()[name]
	if !ok {
		return nil
	}
	out := make(map[string][]string)
	switch val := v.(type) {
	case map[string]any:
		for k, raw := range val {
			if arr, ok := raw.([]any); ok {
				for _, s := range arr {
					if sv, ok := s.(string); ok {
						out[k] = append(out[k], sv)
					}
				}
			}
		}
	case []any:
		for _, item := range val {
			obj, ok := item.(map[string]any)
			if !ok {
				continue
			}
			k, _ := obj["key"].(string)
			sv, _ := obj["value"].(string)
			if k == "" {
				continue
			}
			out[k] = append(out[k], sv)
		}
	}
	return out
}

func serializeCollection(c *entities.Collection) map[string]any {
	return map[string]any{
		"id":           c.ID.String(),
		"workspace_id": c.WorkspaceID.String(),
		"parent_id":    uuidPtrStr(c.ParentID),
		"name":         c.Name,
		"description":  c.Description,
		"auth_type":    string(c.AuthType),
		"auth_data":    maskAuthData(c.AuthData),
		"pre_script":   c.PreScript,
		"post_script":  c.PostScript,
		"sort_order":   c.SortOrder,
		"version":      c.Version,
		"is_delete":    c.IsDelete,
		"created_at":   c.CreatedAt.Format("2006-01-02T15:04:05Z"),
		"updated_at":   c.UpdatedAt.Format("2006-01-02T15:04:05Z"),
	}
}

func serializeRequest(r *entities.Request) map[string]any {
	return map[string]any{
		"id":                  r.ID.String(),
		"collection_id":       r.CollectionID.String(),
		"name":                r.Name,
		"description":         r.Description,
		"protocol":            string(r.Protocol),
		"method":              string(r.Method),
		"url":                 maskURLSecrets(r.URL),
		"headers":             maskHeaderItems(r.Headers),
		"body":                r.Body,
		"body_type":           string(r.BodyType),
		"auth_type":           string(r.AuthType),
		"auth_data":           maskAuthData(r.AuthData),
		"pre_script":          r.PreScript,
		"post_script":         r.PostScript,
		"grpc_service":        r.GRPCService,
		"grpc_method":         r.GRPCMethod,
		"grpc_proto_path":     r.GRPCProtoPath,
		"grpc_metadata":       maskResponseHeaders(r.GRPCMetadata),
		"graphql_query":       r.GraphQLQuery,
		"graphql_variables":   r.GraphQLVariables,
		"graphql_schema_path": r.GraphQLSchemaPath,
		"graphql_operation":   r.GraphQLOperation,
		"sort_order":          r.SortOrder,
		"version":             r.Version,
		"is_delete":           r.IsDelete,
		"created_at":          r.CreatedAt.Format("2006-01-02T15:04:05Z"),
		"updated_at":          r.UpdatedAt.Format("2006-01-02T15:04:05Z"),
	}
}
