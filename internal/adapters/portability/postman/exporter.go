package postman

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"

	"github.com/google/uuid"

	"github.com/tetiva-app/client/internal/domain/entities"
	"github.com/tetiva-app/client/internal/domain/usecase/auth"
)

func stringVal(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

// collections and requests must include the whole subtree under rootID.
func ExportCollection(
	rootID uuid.UUID,
	collections []*entities.Collection,
	requests []*entities.Request,
) ([]byte, error) {
	const funcName = "postman.ExportCollection"

	var root *entities.Collection
	for _, c := range collections {
		if c.ID == rootID {
			root = c
			break
		}
	}
	if root == nil {
		return nil, fmt.Errorf("%s: root collection not found", funcName)
	}

	childrenMap := make(map[uuid.UUID][]*entities.Collection)
	for _, c := range collections {
		if c.ID == rootID {
			continue
		}
		if c.ParentID != nil {
			childrenMap[*c.ParentID] = append(childrenMap[*c.ParentID], c)
		}
	}

	for k := range childrenMap {
		sort.Slice(childrenMap[k], func(i, j int) bool {
			return childrenMap[k][i].SortOrder < childrenMap[k][j].SortOrder
		})
	}

	requestsMap := make(map[uuid.UUID][]*entities.Request)
	for _, r := range requests {
		requestsMap[r.CollectionID] = append(requestsMap[r.CollectionID], r)
	}

	for k := range requestsMap {
		sort.Slice(requestsMap[k], func(i, j int) bool {
			return requestsMap[k][i].SortOrder < requestsMap[k][j].SortOrder
		})
	}

	items := buildItems(rootID, childrenMap, requestsMap)
	if items == nil {
		items = []PostmanItem{}
	}

	pc := PostmanCollection{
		Info: PostmanInfo{
			PostmanID:   uuid.New().String(),
			Name:        root.Name,
			Description: descriptionOf(root.Description),
			Schema:      SchemaV21,
		},
		Auth: buildAuth(root.AuthType, root.AuthData),
		Item: items,
	}

	data, err := json.MarshalIndent(pc, "", "\t")
	if err != nil {
		return nil, fmt.Errorf("%s: failed to marshal: %w", funcName, err)
	}

	return data, nil
}

func buildItems(
	parentID uuid.UUID,
	childrenMap map[uuid.UUID][]*entities.Collection,
	requestsMap map[uuid.UUID][]*entities.Request,
) []PostmanItem {
	var items []PostmanItem

	for _, child := range childrenMap[parentID] {
		subItems := buildItems(child.ID, childrenMap, requestsMap)
		// A nil slice would be omitted and read back as a request, losing the folder.
		if subItems == nil {
			subItems = []PostmanItem{}
		}
		items = append(items, PostmanItem{
			Name:        child.Name,
			Description: descriptionOf(child.Description),
			Auth:        buildAuth(child.AuthType, child.AuthData),
			Item:        &subItems,
		})
	}

	for _, req := range requestsMap[parentID] {
		items = append(items, buildRequestItem(req))
	}

	return items
}

func buildRequestItem(req *entities.Request) PostmanItem {
	method := string(req.Method)

	// GraphQL requests are always exported as POST in Postman.
	if req.Protocol == entities.ProtocolGraphQL {
		method = "POST"
	}

	pmReq := PostmanRequest{
		Method:      method,
		Header:      buildHeaders(req.Headers),
		URL:         PostmanURL{Raw: req.URL},
		Description: descriptionOf(req.Description),
	}

	// A WebSocket body holds the settings document, not a payload Postman could send.
	hasSendableBody := req.Protocol != entities.ProtocolWebSocket &&
		req.BodyType != entities.BodyTypeNone && req.Body != ""

	if req.Protocol == entities.ProtocolGraphQL {
		pmReq.Body = buildGraphQLBody(req.GraphQLQuery, req.GraphQLVariables)
	} else if hasSendableBody {
		pmReq.Body = buildBody(req.BodyType, req.Body)
	}

	if req.AuthType != entities.AuthTypeNone {
		pmReq.Auth = buildAuth(req.AuthType, req.AuthData)
	}

	return PostmanItem{
		Name:    req.Name,
		Request: &pmReq,
	}
}

func buildHeaders(headers []entities.HeaderItem) []PostmanKV {
	kvs := make([]PostmanKV, 0, len(headers))
	for _, h := range headers {
		kv := PostmanKV{Key: h.Key, Value: h.Value}
		if !h.Enabled {
			kv.Disabled = true
		}
		kvs = append(kvs, kv)
	}
	return kvs
}

func buildGraphQLBody(query, variables string) *PostmanBody {
	return &PostmanBody{
		Mode: "graphql",
		Graphql: &PostmanGraphQLBody{
			Query:     query,
			Variables: variables,
		},
	}
}

func buildBody(bodyType entities.BodyType, body string) *PostmanBody {
	switch bodyType {
	case entities.BodyTypeJSON:
		return &PostmanBody{
			Mode: "raw",
			Raw:  body,
			Options: &PostmanBodyOpt{
				Raw: &PostmanRawOpt{Language: "json"},
			},
		}
	case entities.BodyTypeXML:
		return &PostmanBody{
			Mode: "raw",
			Raw:  body,
			Options: &PostmanBodyOpt{
				Raw: &PostmanRawOpt{Language: "xml"},
			},
		}
	case entities.BodyTypeRaw:
		return &PostmanBody{
			Mode: "raw",
			Raw:  body,
		}
	case entities.BodyTypeForm:
		var formData []PostmanKV
		var items []map[string]any
		if err := json.Unmarshal([]byte(body), &items); err == nil {
			for _, item := range items {
				kv := PostmanKV{
					Key:   stringVal(item["key"]),
					Value: stringVal(item["value"]),
					Type:  stringVal(item["type"]),
				}
				if kv.Type == "" {
					kv.Type = "text"
				}
				if enabled, ok := item["enabled"]; ok {
					if b, ok := enabled.(bool); ok && !b {
						kv.Disabled = true
					}
				}
				formData = append(formData, kv)
			}
		}
		return &PostmanBody{
			Mode:     "formdata",
			FormData: formData,
		}
	default:
		return nil
	}
}

// buildAuth renders our auth_data as the matching Postman auth block.
func buildAuth(authType entities.AuthType, authData string) *PostmanAuth {
	f, err := auth.ParseFields(authData)
	if err != nil {
		f = auth.Fields{}
	}

	switch authType {
	case entities.AuthTypeBearer:
		return &PostmanAuth{
			Type:   "bearer",
			Bearer: []PostmanAuthKV{authKV("token", f.Str("token"))},
		}
	case entities.AuthTypeBasic:
		return &PostmanAuth{
			Type: "basic",
			Basic: []PostmanAuthKV{
				authKV("username", f.Str("username")),
				authKV("password", f.Str("password")),
			},
		}
	case entities.AuthTypeAPIKey:
		return &PostmanAuth{
			Type: "apikey",
			APIKey: []PostmanAuthKV{
				authKV("key", f.Str("key")),
				authKV("value", f.Str("value")),
				authKV("in", apiKeyAddTo(f)),
			},
		}
	case entities.AuthTypeOAuth2:
		return &PostmanAuth{Type: "oauth2", OAuth2: buildOAuth2KVs(f)}
	case entities.AuthTypeJWT:
		return &PostmanAuth{Type: "jwt", JWT: buildJWTKVs(f)}
	case entities.AuthTypeDigest:
		return &PostmanAuth{
			Type: "digest",
			Digest: []PostmanAuthKV{
				authKV("username", f.Str("username")),
				authKV("password", f.Str("password")),
			},
		}
	case entities.AuthTypeAWSSigV4:
		kvs := []PostmanAuthKV{
			authKV("accessKey", f.Str("accessKeyId")),
			authKV("secretKey", f.Str("secretAccessKey")),
			authKV("region", f.Str("region")),
			authKV("service", f.Str("service")),
		}
		if token := f.Str("sessionToken"); token != "" {
			kvs = append(kvs, authKV("sessionToken", token))
		}
		return &PostmanAuth{Type: "awsv4", AWSV4: kvs}
	default:
		return nil
	}
}

// ourOAuth2Grants is the reverse of postmanOAuth2Grants; unknown values go out
// verbatim so a Postman import of our export can still show them.
var ourOAuth2Grants = map[string]string{
	auth.GrantClientCredentials: "client_credentials",
	auth.GrantPassword:          "password_credentials",
	auth.GrantAuthorizationCode: "authorization_code",
	auth.GrantDeviceCode:        "device_code",
}

func buildOAuth2KVs(f auth.Fields) []PostmanAuthKV {
	grant := f.Str("grant")
	if mapped, ok := ourOAuth2Grants[grant]; ok {
		grant = mapped
	}

	var kvs []PostmanAuthKV
	appendKV := func(key, value string) {
		if value != "" {
			kvs = append(kvs, authKV(key, value))
		}
	}
	appendKV("grant_type", grant)
	appendKV("accessTokenUrl", f.Str("tokenUrl"))
	appendKV("authUrl", f.Str("authUrl"))
	appendKV("clientId", f.Str("clientId"))
	appendKV("clientSecret", f.Str("clientSecret"))
	appendKV("scope", f.Str("scope"))
	appendKV("audience", f.Str("audience"))
	appendKV("username", f.Str("username"))
	appendKV("password", f.Str("password"))
	appendKV("headerPrefix", f.Str("headerPrefix"))

	switch f.Str("clientAuth") {
	case auth.ClientAuthBasic:
		appendKV("client_authentication", "header")
	case auth.ClientAuthBody:
		appendKV("client_authentication", "body")
	case auth.ClientAuthNone:
		appendKV(clientAuthExtKey, auth.ClientAuthNone)
	}
	appendKV("addTokenTo", ourAddTokenTo(f.Str("addTo")))
	appendKV(queryParamExtKey, authFieldText(f, "queryParam"))
	appendKV(redirectPortExtKey, authFieldText(f, "redirectPort"))
	appendKV(deviceAuthURLExtKey, authFieldText(f, "deviceAuthUrl"))

	return kvs
}

func buildJWTKVs(f auth.Fields) []PostmanAuthKV {
	var kvs []PostmanAuthKV
	appendKV := func(key, value string) {
		if value != "" {
			kvs = append(kvs, authKV(key, value))
		}
	}
	appendKV("algorithm", f.Str("alg"))
	appendKV("secret", f.Str("secret"))
	appendKV("privateKey", f.Str("privateKey"))
	if _, ok := f["secretBase64"]; ok {
		kvs = append(kvs, boolAuthKV("isSecretBase64Encoded", f.Str("secretBase64") == "true"))
	}
	appendKV("payload", jsonObjectText(f.Obj("claims")))
	appendKV("header", jsonObjectText(f.Obj("header")))
	appendKV("headerPrefix", f.Str("headerPrefix"))
	appendKV("queryParamKey", f.Str("queryParam"))
	appendKV("addTokenTo", ourAddTokenTo(f.Str("addTo")))
	appendKV(expiresInExtKey, authFieldText(f, "expiresIn"))

	return kvs
}

// authFieldText renders a leaf the forms write as text but an import or a synced
// document may carry as a JSON number.
func authFieldText(f auth.Fields, key string) string {
	switch v := f[key].(type) {
	case string:
		return v
	case float64:
		return strconv.FormatFloat(v, 'f', -1, 64)
	default:
		return ""
	}
}

func ourAddTokenTo(v string) string {
	switch v {
	case "header":
		return "header"
	case "query":
		return "queryParams"
	default:
		return ""
	}
}

// jsonObjectText renders a nested auth_data object as the JSON text Postman keeps.
func jsonObjectText(obj map[string]any) string {
	if len(obj) == 0 {
		return ""
	}
	raw, err := json.Marshal(obj)
	if err != nil {
		return ""
	}
	return string(raw)
}

// apiKeyAddTo falls back to the legacy "in" key written by imports made before
// the executor's "addTo" spelling won.
func apiKeyAddTo(f auth.Fields) string {
	if v := f.Str("addTo"); v != "" {
		return v
	}
	return f.Str("in")
}
