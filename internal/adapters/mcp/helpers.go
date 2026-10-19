package mcp

import (
	"cmp"
	"fmt"
	"net/url"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"
	mcplib "github.com/mark3labs/mcp-go/mcp"

	"github.com/tetiva-app/client/internal/domain/entities"
	"github.com/tetiva-app/client/internal/domain/secrets"
)

// Tool results reach the MCP client verbatim, so stored credentials are masked in them.
const redactedValue = "[redacted]"

const redactedEchoDoc = "Values returned as [redacted] may be sent back unchanged: the stored secret is kept. To change a secret, pass the new value."

const authDataDoc = "Auth payload as JSON string. Shapes: bearer {\"token\":\"...\",\"prefix\":\"Bearer\"}; basic and digest {\"username\":\"...\",\"password\":\"...\"}; api_key {\"key\":\"X-Api-Key\",\"value\":\"...\",\"addTo\":\"header\"|\"query\"}; oauth2 {\"grant\":\"client_credentials\",\"tokenUrl\":\"...\",\"clientId\":\"...\",\"clientSecret\":\"...\",\"scope\":\"...\"}; jwt {\"alg\":\"HS256\",\"secret\":\"...\",\"claims\":{}}; aws_sigv4 {\"accessKeyId\":\"...\",\"secretAccessKey\":\"...\",\"region\":\"...\",\"service\":\"...\",\"sessionToken\":\"...\" (optional)}. Values may use {{name}}."

// maskAuthData keeps the "auth is configured" signal without the credentials.
func maskAuthData(data string) string {
	trimmed := strings.TrimSpace(data)
	if trimmed == "" || trimmed == "{}" {
		return ""
	}
	return redactedValue
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

func maskSensitiveValue(value string, sensitive bool) string {
	if !sensitive || !secrets.HasLiteral(value) {
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
		h.Value = maskSensitiveValue(h.Value, secrets.IsSensitiveHeader(h.Key))
		out[i] = h
	}
	return out
}

type headerItemDTO struct {
	Key     string `json:"key"`
	Value   string `json:"value"`
	Enabled bool   `json:"enabled"`
}

func headerItemDTOs(items []entities.HeaderItem) []headerItemDTO {
	if items == nil {
		return nil
	}
	out := make([]headerItemDTO, len(items))
	for i, h := range items {
		out[i] = headerItemDTO{Key: h.Key, Value: h.Value, Enabled: h.Enabled}
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
			masked[i] = maskSensitiveValue(v, secrets.IsSensitiveHeader(k))
		}
		out[k] = masked
	}
	return out
}

// maskURLSecrets redacts credential query parameters.
func maskURLSecrets(raw string) string {
	return secrets.MaskQuery(raw, redactedValue)
}

const minScriptSecretLen = 4

func sentSecrets(sent *entities.SentRequest) []string {
	var out []string
	add := func(v string) { out = append(out, v, url.QueryEscape(v)) }
	for k, vs := range sent.Headers {
		if !secrets.IsSensitiveHeader(k) && !slices.ContainsFunc(sent.AuthHeaderKeys, func(a string) bool { return strings.EqualFold(a, k) }) {
			continue
		}
		for _, v := range vs {
			if strings.EqualFold(k, "Cookie") {
				for pair := range strings.SplitSeq(v, ";") {
					if _, value, ok := strings.Cut(pair, "="); ok {
						add(strings.TrimSpace(value))
					}
				}
			}
			if scheme, credential, ok := strings.Cut(v, " "); ok && !strings.ContainsAny(scheme, "=;") {
				v = credential
			}
			add(v)
		}
	}
	if u, err := url.Parse(sent.URL); err == nil {
		for k, vs := range u.Query() {
			if slices.Contains(sent.AuthQueryKeys, k) || secrets.IsSensitiveQueryParam(k) {
				for _, v := range vs {
					add(v)
				}
			}
		}
	}
	return out
}

const chaiTruncateThreshold = 40

// redactScriptResult also masks the prefix + "…" that chai prints for a long string.
func redactScriptResult(r *entities.ScriptResult, values []string) *entities.ScriptResult {
	values = slices.DeleteFunc(slices.Clone(values), func(v string) bool { return len(v) < minScriptSecretLen })
	if len(values) == 0 {
		return r
	}
	slices.SortFunc(values, func(a, b string) int { return cmp.Or(len(b)-len(a), strings.Compare(a, b)) })
	values = slices.Compact(values)
	pairs := make([]string, 0, 2*len(values))
	for _, v := range values {
		pairs = append(pairs, v, redactedValue)
	}
	rep := strings.NewReplacer(pairs...)
	redact := func(s string) string { return maskTruncated(rep.Replace(s), values) }

	out := *r
	out.PreConsole = replaceLines(redact, r.PreConsole)
	out.PostConsole = replaceLines(redact, r.PostConsole)
	out.Tests = slices.Clone(r.Tests)
	for i := range out.Tests {
		out.Tests[i].Name = redact(out.Tests[i].Name)
		out.Tests[i].Error = redact(out.Tests[i].Error)
	}
	out.Errors = slices.Clone(r.Errors)
	for i := range out.Errors {
		out.Errors[i].Message = redact(out.Errors[i].Message)
	}
	return &out
}

func maskTruncated(s string, values []string) string {
	const ellipsis = "…"
	if !strings.Contains(s, ellipsis) {
		return s
	}
	var b strings.Builder
	for {
		head, rest, found := strings.Cut(s, ellipsis)
		if !found {
			b.WriteString(s)
			return b.String()
		}
		if n := truncatedSecret(head, values); n > 0 {
			b.WriteString(head[:len(head)-n])
			b.WriteString(redactedValue)
		} else {
			b.WriteString(head)
			b.WriteString(ellipsis)
		}
		s = rest
	}
}

func truncatedSecret(head string, values []string) int {
	for n := min(len(head), chaiTruncateThreshold); n >= minScriptSecretLen; n-- {
		for _, v := range values {
			if n < len(v) && strings.HasPrefix(v, head[len(head)-n:]) {
				return n
			}
		}
	}
	return 0
}

func replaceLines(redact func(string) string, lines []string) []string {
	out := slices.Clone(lines)
	for i := range out {
		out[i] = redact(out[i])
	}
	return out
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
func restoreAuthData(input, current string) (string, bool, error) {
	if input == redactedValue {
		return current, true, nil
	}
	if strings.Contains(input, redactedValue) {
		return "", false, redactedEmbeddedErr("auth_data")
	}
	return input, false, nil
}

// queryValues indexes a URL's query by lowercased parameter name.
func queryValues(raw string) map[string][]string {
	_, query, _, ok := secrets.SplitQuery(raw)
	if !ok {
		return nil
	}
	out := make(map[string][]string)
	for _, part := range strings.Split(query, "&") {
		if eq := strings.IndexByte(part, '='); eq >= 0 {
			key := secrets.ParamKey(part[:eq])
			out[key] = append(out[key], part[eq+1:])
		}
	}
	return out
}

// restoreURLSecrets puts the stored value back under every query parameter the
// agent echoed as a mask, so a read-then-write keeps the secret.
func restoreURLSecrets(input, current string) (string, []string, error) {
	if !strings.Contains(input, redactedValue) {
		return input, nil, nil
	}
	prefix, query, suffix, ok := secrets.SplitQuery(input)
	if !ok {
		return "", nil, redactedEchoErr("url")
	}

	stored := queryValues(current)
	seen := make(map[string]int)
	var kept []string
	parts := strings.Split(query, "&")
	for i, part := range parts {
		eq := strings.IndexByte(part, '=')
		if eq < 0 {
			continue
		}
		name, value := part[:eq], part[eq+1:]
		key := secrets.ParamKey(name)
		idx := seen[key]
		seen[key]++
		if value != redactedValue {
			continue
		}
		if idx >= len(stored[key]) {
			return "", nil, redactedEchoErr("url parameter " + name)
		}
		parts[i] = name + "=" + stored[key][idx]
		kept = append(kept, name)
	}
	joined := prefix + strings.Join(parts, "&") + suffix
	if strings.Contains(joined, redactedValue) {
		return "", nil, redactedEchoErr("url")
	}
	return joined, kept, nil
}

// restoreHeaderSecrets is restoreURLSecrets for header items, matched by key.
func restoreHeaderSecrets(input, current []entities.HeaderItem) ([]entities.HeaderItem, []string, error) {
	if input == nil {
		return nil, nil, nil
	}
	stored := make(map[string][]string, len(current))
	for _, h := range current {
		key := strings.ToLower(strings.TrimSpace(h.Key))
		stored[key] = append(stored[key], h.Value)
	}

	out := make([]entities.HeaderItem, len(input))
	copy(out, input)
	seen := make(map[string]int, len(out))
	var kept []string
	for i := range out {
		key := strings.ToLower(strings.TrimSpace(out[i].Key))
		idx := seen[key]
		seen[key]++
		if out[i].Value != redactedValue {
			if strings.Contains(out[i].Value, redactedValue) {
				return nil, nil, redactedEmbeddedErr("header " + out[i].Key)
			}
			continue
		}
		if idx >= len(stored[key]) {
			return nil, nil, redactedEchoErr("header " + out[i].Key)
		}
		out[i].Value = stored[key][idx]
		kept = append(kept, out[i].Key)
	}
	return out, kept, nil
}

// restoreMetadataSecrets is restoreHeaderSecrets for the map form gRPC uses.
func restoreMetadataSecrets(input, current map[string][]string) (map[string][]string, []string, error) {
	if input == nil {
		return nil, nil, nil
	}
	stored := make(map[string][]string, len(current))
	for k, values := range current {
		key := strings.ToLower(strings.TrimSpace(k))
		stored[key] = append(stored[key], values...)
	}

	out := make(map[string][]string, len(input))
	var kept []string
	for k, values := range input {
		key := strings.ToLower(strings.TrimSpace(k))
		restored := make([]string, len(values))
		copy(restored, values)
		keptKey := false
		for i, v := range restored {
			if v != redactedValue {
				if strings.Contains(v, redactedValue) {
					return nil, nil, redactedEmbeddedErr("grpc_metadata " + k)
				}
				continue
			}
			if i >= len(stored[key]) {
				return nil, nil, redactedEchoErr("grpc_metadata " + k)
			}
			restored[i] = stored[key][i]
			keptKey = true
		}
		out[k] = restored
		if keptKey {
			kept = append(kept, k)
		}
	}
	slices.Sort(kept)
	return out, kept, nil
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
		return nil, notUUIDErr(name)
	}
	return &id, nil
}

func notUUIDErr(name string) error {
	return fmt.Errorf("%s must be a UUID; get it from the list_* tools", name)
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
		if k, ok := headerField(obj, "key").(string); ok {
			h.Key = k
		}
		if val, ok := headerField(obj, "value").(string); ok {
			h.Value = val
		}
		if en, ok := headerField(obj, "enabled").(bool); ok {
			h.Enabled = en
		}
		if h.Key == "" {
			continue
		}
		out = append(out, h)
	}
	return out
}

func headerField(obj map[string]any, name string) any {
	if v, ok := obj[name]; ok {
		return v
	}
	return obj[strings.ToUpper(name[:1])+name[1:]]
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

func formatTimestamp(t time.Time) string {
	return t.UTC().Format(time.RFC3339)
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
		"created_at":   formatTimestamp(c.CreatedAt),
		"updated_at":   formatTimestamp(c.UpdatedAt),
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
		"headers":             headerItemDTOs(maskHeaderItems(r.Headers)),
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
		"created_at":          formatTimestamp(r.CreatedAt),
		"updated_at":          formatTimestamp(r.UpdatedAt),
	}
}
