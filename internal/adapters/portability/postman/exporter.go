package postman

import (
	"encoding/json"
	"fmt"
	"sort"

	"github.com/google/uuid"

	"github.com/tetiva-app/client/internal/domain/entities"
)

func stringVal(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

// ExportCollection builds a Postman Collection v2.1 JSON; collections and
// requests must include the whole subtree under rootID.
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

	pc := PostmanCollection{
		Info: PostmanInfo{
			PostmanID:   uuid.New().String(),
			Name:        root.Name,
			Description: root.Description,
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
		items = append(items, PostmanItem{
			Name:        child.Name,
			Description: child.Description,
			Auth:        buildAuth(child.AuthType, child.AuthData),
			Item:        subItems,
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
		Method: method,
		Header: buildHeaders(req.Headers),
		URL:    PostmanURL{Raw: req.URL},
	}

	if req.Protocol == entities.ProtocolGraphQL {
		pmReq.Body = buildGraphQLBody(req.GraphQLQuery, req.GraphQLVariables)
	} else if req.BodyType != entities.BodyTypeNone && req.Body != "" {
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

func buildAuth(authType entities.AuthType, authData string) *PostmanAuth {
	var data map[string]string
	_ = json.Unmarshal([]byte(authData), &data)

	switch authType {
	case entities.AuthTypeBearer:
		return &PostmanAuth{
			Type: "bearer",
			Bearer: []PostmanKV{
				{Key: "token", Value: data["token"]},
			},
		}
	case entities.AuthTypeBasic:
		return &PostmanAuth{
			Type: "basic",
			Basic: []PostmanKV{
				{Key: "username", Value: data["username"]},
				{Key: "password", Value: data["password"]},
			},
		}
	case entities.AuthTypeAPIKey:
		return &PostmanAuth{
			Type: "apikey",
			APIKey: []PostmanKV{
				{Key: "key", Value: data["key"]},
				{Key: "value", Value: data["value"]},
				{Key: "in", Value: data["in"]},
			},
		}
	default:
		return nil
	}
}
