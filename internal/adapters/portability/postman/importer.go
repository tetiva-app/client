package postman

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/google/uuid"

	"github.com/tetiva-app/client/internal/domain/entities"
	"github.com/tetiva-app/client/internal/domain/usecase/auth"
	"github.com/tetiva-app/client/internal/domain/usecase/collection"
	"github.com/tetiva-app/client/internal/domain/usecase/request"
)

type ImportOpts struct {
	WorkspaceID uuid.UUID
	UserID      string
	ParentID    *uuid.UUID
}

// Warnings name the items whose auth could not be imported as-is; the import still succeeded.
type ImportResult struct {
	FoldersCreated  int
	RequestsCreated int
	Warnings        []string
}

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

	rootAuthType, rootAuthData, warnings := mapAuth(pc.Auth, itemLabel("collection", pc.Info.Name))
	result.Warnings = append(result.Warnings, warnings...)

	rootColl, err := collUC.Create(ctx, collection.Create{
		Name:        pc.Info.Name,
		Description: string(pc.Info.Description),
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
			folderAuthType, folderAuthData, warnings := mapAuth(item.Auth, itemLabel("folder", item.Name))
			result.Warnings = append(result.Warnings, warnings...)
			subColl, err := collUC.Create(ctx, collection.Create{
				Name:        item.Name,
				Description: string(item.Description),
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
			input, warnings := mapPostmanRequest(item, parentCollectionID)
			result.Warnings = append(result.Warnings, warnings...)

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

// requestDescription prefers the request-level description: v2.1 allows it in
// both places, and Postman writes documentation to whichever the author used.
func requestDescription(item PostmanItem) string {
	if item.Request != nil && item.Request.Description != "" {
		return string(item.Request.Description)
	}
	return string(item.Description)
}

func mapPostmanRequest(item PostmanItem, collectionID uuid.UUID) (request.Create, []string) {
	req := item.Request

	method := entities.HTTPMethod(strings.ToUpper(req.Method))
	if !method.IsValid() {
		method = entities.MethodGET
	}

	headers := mapHeaders(req.Header)
	authType, authData, warnings := mapAuth(req.Auth, itemLabel("request", item.Name))

	// Detect GraphQL body mode — store data in GraphQL fields, not Body.
	if req.Body != nil && req.Body.Mode == "graphql" && req.Body.Graphql != nil {
		return request.Create{
			CollectionID:     collectionID,
			Name:             item.Name,
			Description:      requestDescription(item),
			Protocol:         entities.ProtocolGraphQL,
			Method:           entities.MethodPOST,
			URL:              req.URL.Raw,
			Headers:          headers,
			BodyType:         entities.BodyTypeNone,
			AuthType:         authType,
			AuthData:         authData,
			GraphQLQuery:     req.Body.Graphql.Query,
			GraphQLVariables: req.Body.Graphql.Variables,
		}, warnings
	}

	bodyType, body := mapBody(req.Body)

	return request.Create{
		CollectionID: collectionID,
		Name:         item.Name,
		Description:  requestDescription(item),
		Protocol:     entities.ProtocolHTTP,
		Method:       method,
		URL:          req.URL.Raw,
		Headers:      headers,
		Body:         body,
		BodyType:     bodyType,
		AuthType:     authType,
		AuthData:     authData,
	}, warnings
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

// itemLabel names an item in an import warning.
func itemLabel(kind, name string) string {
	if name == "" {
		return kind
	}
	return fmt.Sprintf("%s %q", kind, name)
}

// mapAuth translates a Postman auth block; unsupported schemes degrade to none
// and are reported so the import can tell the user what it dropped.
func mapAuth(a *PostmanAuth, label string) (entities.AuthType, string, []string) {
	if a == nil || a.Type == "" || a.Type == "noauth" {
		return entities.AuthTypeNone, "{}", nil
	}

	switch a.Type {
	case "bearer":
		return entities.AuthTypeBearer, authDataJSON(map[string]any{
			"token": findAuthKV(a.Bearer, "token"),
		}), nil
	case "basic":
		return entities.AuthTypeBasic, authDataJSON(map[string]any{
			"username": findAuthKV(a.Basic, "username"),
			"password": findAuthKV(a.Basic, "password"),
		}), nil
	case "apikey":
		addTo := findAuthKV(a.APIKey, "in")
		if addTo == "" {
			addTo = "header"
		}
		// Postman calls it "in", the request executor reads "addTo".
		return entities.AuthTypeAPIKey, authDataJSON(map[string]any{
			"key":   findAuthKV(a.APIKey, "key"),
			"value": findAuthKV(a.APIKey, "value"),
			"addTo": addTo,
		}), nil
	case "oauth2":
		return mapOAuth2Auth(a.OAuth2, label)
	case "jwt":
		return mapJWTAuth(a.JWT, label)
	case "digest":
		return entities.AuthTypeDigest, authDataJSON(map[string]any{
			"username": findAuthKV(a.Digest, "username"),
			"password": findAuthKV(a.Digest, "password"),
		}), nil
	case "awsv4":
		data := map[string]any{
			"accessKeyId":     findAuthKV(a.AWSV4, "accessKey"),
			"secretAccessKey": findAuthKV(a.AWSV4, "secretKey"),
			"region":          findAuthKV(a.AWSV4, "region"),
			"service":         findAuthKV(a.AWSV4, "service"),
		}
		putIfSet(data, "sessionToken", findAuthKV(a.AWSV4, "sessionToken"))
		return entities.AuthTypeAWSSigV4, authDataJSON(data), nil
	default:
		return entities.AuthTypeNone, "{}", []string{
			fmt.Sprintf("%s: auth type %q is not supported and was imported as no auth", label, a.Type),
		}
	}
}

// postmanOAuth2Grants maps Postman grant names onto ours; PKCE collapses into
// the authorization code grant because that is the only way we run it.
var postmanOAuth2Grants = map[string]string{
	"client_credentials":           auth.GrantClientCredentials,
	"password_credentials":         auth.GrantPassword,
	"password":                     auth.GrantPassword,
	"authorization_code":           auth.GrantAuthorizationCode,
	"authorization_code_with_pkce": auth.GrantAuthorizationCode,
	"device_code":                  auth.GrantDeviceCode,
}

func mapOAuth2Auth(kvs []PostmanAuthKV, label string) (entities.AuthType, string, []string) {
	tokenURL := findAuthKV(kvs, "accessTokenUrl")
	// A block with only a pasted token cannot acquire anything; it is a bearer token.
	if tokenURL == "" {
		if token := findAuthKV(kvs, "accessToken"); token != "" {
			return entities.AuthTypeBearer, authDataJSON(map[string]any{"token": token}), []string{
				fmt.Sprintf("%s: the OAuth 2.0 block has an access token but no token URL; imported as a bearer token", label),
			}
		}
	}

	var warnings []string
	data := map[string]any{}
	putIfSet(data, "tokenUrl", tokenURL)
	putIfSet(data, "authUrl", findAuthKV(kvs, "authUrl"))
	putIfSet(data, "clientId", findAuthKV(kvs, "clientId"))
	putIfSet(data, "clientSecret", findAuthKV(kvs, "clientSecret"))
	putIfSet(data, "scope", findAuthKV(kvs, "scope"))
	putIfSet(data, "audience", findAuthKV(kvs, "audience"))
	putIfSet(data, "username", findAuthKV(kvs, "username"))
	putIfSet(data, "password", findAuthKV(kvs, "password"))
	putIfSet(data, "headerPrefix", findAuthKV(kvs, "headerPrefix"))

	if raw := findAuthKV(kvs, "grant_type"); raw != "" {
		grant, ok := postmanOAuth2Grants[raw]
		if !ok {
			grant = raw
			warnings = append(warnings, fmt.Sprintf("%s: OAuth 2.0 grant %q is not supported; pick one in the Auth tab", label, raw))
		}
		data["grant"] = grant
	}
	switch findAuthKV(kvs, "client_authentication") {
	case "header":
		data["clientAuth"] = auth.ClientAuthBasic
	case "body":
		data["clientAuth"] = auth.ClientAuthBody
	}
	if findAuthKV(kvs, clientAuthExtKey) == auth.ClientAuthNone {
		data["clientAuth"] = auth.ClientAuthNone
	}
	putIfSet(data, "addTo", postmanAddTokenTo(findAuthKV(kvs, "addTokenTo")))
	putIfSet(data, "queryParam", findAuthKV(kvs, queryParamExtKey))
	putIfSet(data, "redirectPort", findAuthKV(kvs, redirectPortExtKey))
	putIfSet(data, "deviceAuthUrl", findAuthKV(kvs, deviceAuthURLExtKey))

	return entities.AuthTypeOAuth2, authDataJSON(data), warnings
}

func mapJWTAuth(kvs []PostmanAuthKV, label string) (entities.AuthType, string, []string) {
	var warnings []string
	data := map[string]any{}
	putIfSet(data, "alg", findAuthKV(kvs, "algorithm"))
	putIfSet(data, "secret", findAuthKV(kvs, "secret"))
	putIfSet(data, "privateKey", findAuthKV(kvs, "privateKey"))
	putIfSet(data, "headerPrefix", findAuthKV(kvs, "headerPrefix"))
	putIfSet(data, "queryParam", findAuthKV(kvs, "queryParamKey"))
	putIfSet(data, "addTo", postmanAddTokenTo(findAuthKV(kvs, "addTokenTo")))
	putIfSet(data, "expiresIn", findAuthKV(kvs, expiresInExtKey))

	if raw := findAuthKV(kvs, "isSecretBase64Encoded"); raw != "" {
		data["secretBase64"] = strconv.FormatBool(raw == "true")
	}

	for _, field := range []struct{ postman, ours string }{{"payload", "claims"}, {"header", "header"}} {
		obj, ok := jsonObjectField(findAuthKV(kvs, field.postman))
		switch {
		case !ok:
			warnings = append(warnings, fmt.Sprintf("%s: the JWT %s is not a JSON object and was dropped", label, field.postman))
		case len(obj) > 0:
			data[field.ours] = obj
		}
	}

	return entities.AuthTypeJWT, authDataJSON(data), warnings
}

func postmanAddTokenTo(v string) string {
	switch v {
	case "header":
		return "header"
	case "queryParams", "query":
		return "query"
	default:
		return ""
	}
}

// jsonObjectField reads a Postman field carrying a JSON object as text; ok is
// false only when a non-empty value is not an object.
func jsonObjectField(raw string) (map[string]any, bool) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil, true
	}
	var obj map[string]any
	if err := json.Unmarshal([]byte(trimmed), &obj); err != nil || obj == nil {
		return nil, false
	}
	return obj, true
}

func putIfSet(data map[string]any, key, value string) {
	if value != "" {
		data[key] = value
	}
}

func authDataJSON(data map[string]any) string {
	raw, err := json.Marshal(data)
	if err != nil {
		return "{}"
	}
	return string(raw)
}

func findAuthKV(kvs []PostmanAuthKV, key string) string {
	for _, kv := range kvs {
		if kv.Key == key {
			return kv.String()
		}
	}
	return ""
}
