package postman

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/google/uuid"

	"github.com/tetiva-app/client/internal/domain/entities"
	"github.com/tetiva-app/client/internal/domain/usecase/collection"
	"github.com/tetiva-app/client/internal/domain/usecase/request"
)

// ImportOpts provides options for the import operation.
type ImportOpts struct {
	WorkspaceID uuid.UUID
	UserID      string
	ParentID    *uuid.UUID
}

// ImportResult reports what was created during import.
type ImportResult struct {
	FoldersCreated  int
	RequestsCreated int
}

// ImportCollection parses a Postman Collection v2.1 JSON and creates
// collections and requests using the provided usecases.
func ImportCollection(
	ctx context.Context,
	data []byte,
	opts ImportOpts,
	collUC collection.Usecase,
	reqUC request.Usecase,
) (*ImportResult, error) {
	const funcName = "postman.ImportCollection"

	var pc PostmanCollection
	if err := json.Unmarshal(data, &pc); err != nil {
		return nil, fmt.Errorf("%s: invalid JSON: %w", funcName, err)
	}

	if !strings.Contains(pc.Info.Schema, "v2.1") {
		return nil, fmt.Errorf("%s: unsupported schema, only Postman Collection v2.1 is supported (export your collection as v2.1 from Postman)", funcName)
	}

	result := &ImportResult{}

	rootAuthType, rootAuthData := mapAuth(pc.Auth)

	rootColl, err := collUC.Create(ctx, collection.Create{
		Name:        pc.Info.Name,
		Description: pc.Info.Description,
		ParentID:    opts.ParentID,
		AuthType:    rootAuthType,
		AuthData:    rootAuthData,
	}, collection.CreateOpt{
		UserID:      opts.UserID,
		WorkspaceID: opts.WorkspaceID,
	})
	if err != nil {
		return nil, fmt.Errorf("%s: failed to create root collection: %w", funcName, err)
	}
	result.FoldersCreated++

	if err := importItems(ctx, pc.Item, rootColl.ID, opts, collUC, reqUC, result); err != nil {
		return nil, err
	}

	return result, nil
}

func importItems(
	ctx context.Context,
	items []PostmanItem,
	parentCollectionID uuid.UUID,
	opts ImportOpts,
	collUC collection.Usecase,
	reqUC request.Usecase,
	result *ImportResult,
) error {
	const funcName = "postman.importItems"

	for _, item := range items {
		if item.IsFolder() {
			folderAuthType, folderAuthData := mapAuth(item.Auth)
			subColl, err := collUC.Create(ctx, collection.Create{
				Name:        item.Name,
				Description: item.Description,
				ParentID:    &parentCollectionID,
				AuthType:    folderAuthType,
				AuthData:    folderAuthData,
			}, collection.CreateOpt{
				UserID:      opts.UserID,
				WorkspaceID: opts.WorkspaceID,
			})
			if err != nil {
				return fmt.Errorf("%s: failed to create folder %q: %w", funcName, item.Name, err)
			}
			result.FoldersCreated++

			if err := importItems(ctx, item.Item, subColl.ID, opts, collUC, reqUC, result); err != nil {
				return err
			}
		} else if item.Request != nil {
			input := mapPostmanRequest(item, parentCollectionID)

			_, err := reqUC.Create(ctx, input, request.CreateOpt{
				UserID: opts.UserID,
			})
			if err != nil {
				return fmt.Errorf("%s: failed to create request %q: %w", funcName, item.Name, err)
			}
			result.RequestsCreated++
		}
	}

	return nil
}

func mapPostmanRequest(item PostmanItem, collectionID uuid.UUID) request.Create {
	req := item.Request

	method := entities.HTTPMethod(strings.ToUpper(req.Method))
	if !method.IsValid() {
		method = entities.MethodGET
	}

	headers := mapHeaders(req.Header)
	authType, authData := mapAuth(req.Auth)

	// Detect GraphQL body mode — store data in GraphQL fields, not Body.
	if req.Body != nil && req.Body.Mode == "graphql" && req.Body.Graphql != nil {
		return request.Create{
			CollectionID:     collectionID,
			Name:             item.Name,
			Protocol:         entities.ProtocolGraphQL,
			Method:           entities.MethodPOST,
			URL:              req.URL.Raw,
			Headers:          headers,
			BodyType:         entities.BodyTypeNone,
			AuthType:         authType,
			AuthData:         authData,
			GraphQLQuery:     req.Body.Graphql.Query,
			GraphQLVariables: req.Body.Graphql.Variables,
		}
	}

	bodyType, body := mapBody(req.Body)

	return request.Create{
		CollectionID: collectionID,
		Name:         item.Name,
		Protocol:     entities.ProtocolHTTP,
		Method:       method,
		URL:          req.URL.Raw,
		Headers:      headers,
		Body:         body,
		BodyType:     bodyType,
		AuthType:     authType,
		AuthData:     authData,
	}
}

func mapHeaders(headers []PostmanKV) []entities.HeaderItem {
	if len(headers) == 0 {
		return nil
	}

	result := make([]entities.HeaderItem, 0, len(headers))
	for _, h := range headers {
		result = append(result, entities.HeaderItem{
			Key:     h.Key,
			Value:   h.Value,
			Enabled: !h.Disabled,
		})
	}

	return result
}

func mapBody(body *PostmanBody) (entities.BodyType, string) {
	if body == nil || body.Mode == "" {
		return entities.BodyTypeNone, ""
	}

	switch body.Mode {
	case "raw":
		lang := ""
		if body.Options != nil && body.Options.Raw != nil {
			lang = body.Options.Raw.Language
		}

		switch lang {
		case "json":
			return entities.BodyTypeJSON, body.Raw
		case "xml", "html":
			return entities.BodyTypeXML, body.Raw
		default:
			return entities.BodyTypeRaw, body.Raw
		}
	case "formdata":
		return entities.BodyTypeForm, mapFormData(body.FormData)
	case "graphql":
		// GraphQL data is handled separately in mapPostmanRequest; body fields unused.
		return entities.BodyTypeNone, ""
	default:
		return entities.BodyTypeRaw, body.Raw
	}
}

func mapFormData(formData []PostmanKV) string {
	var fields []map[string]any
	for _, fd := range formData {
		fieldType := fd.Type
		if fieldType == "" {
			fieldType = "text"
		}
		fields = append(fields, map[string]interface{}{
			"key":     fd.Key,
			"value":   fd.Value,
			"type":    fieldType,
			"enabled": !fd.Disabled,
		})
	}

	if len(fields) == 0 {
		return "[]"
	}

	data, _ := json.Marshal(fields)
	return string(data)
}

func mapAuth(auth *PostmanAuth) (entities.AuthType, string) {
	if auth == nil || auth.Type == "" || auth.Type == "noauth" {
		return entities.AuthTypeNone, "{}"
	}

	switch auth.Type {
	case "bearer":
		token := findKV(auth.Bearer, "token")
		data, _ := json.Marshal(map[string]string{"token": token})
		return entities.AuthTypeBearer, string(data)
	case "basic":
		username := findKV(auth.Basic, "username")
		password := findKV(auth.Basic, "password")
		data, _ := json.Marshal(map[string]string{"username": username, "password": password})
		return entities.AuthTypeBasic, string(data)
	case "apikey":
		key := findKV(auth.APIKey, "key")
		value := findKV(auth.APIKey, "value")
		in := findKV(auth.APIKey, "in")
		if in == "" {
			in = "header"
		}
		data, _ := json.Marshal(map[string]string{"key": key, "value": value, "in": in})
		return entities.AuthTypeAPIKey, string(data)
	default:
		return entities.AuthTypeNone, "{}"
	}
}

func findKV(kvs []PostmanKV, key string) string {
	for _, kv := range kvs {
		if kv.Key == key {
			return kv.Value
		}
	}
	return ""
}
