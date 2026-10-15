package postman

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"

	"github.com/tetiva-app/client/internal/adapters/portability"
	"github.com/tetiva-app/client/internal/domain"
	"github.com/tetiva-app/client/internal/domain/entities"
	"github.com/tetiva-app/client/internal/domain/usecase/auth"
	"github.com/tetiva-app/client/internal/domain/usecase/collection"
	"github.com/tetiva-app/client/internal/domain/usecase/environment"
	"github.com/tetiva-app/client/internal/domain/usecase/example"
	"github.com/tetiva-app/client/internal/domain/usecase/request"
)

const maxExampleNameLen = 200

type ImportOpts struct {
	WorkspaceID    uuid.UUID
	UserID         string
	ParentID       *uuid.UUID
	IncludeScripts bool
}

// Warnings name what could not be imported as-is; the import still succeeded.
type ImportResult struct {
	RootID          uuid.UUID
	FoldersCreated  int
	RequestsCreated int
	ExamplesCreated int
	EnvironmentID   uuid.UUID
	EnvironmentName string
	Warnings        []portability.Warning
	scriptsImported bool
	suspectExamples []string
}

type (
	CollectionCreator interface {
		Create(ctx context.Context, input collection.Create, opt collection.CreateOpt) (*entities.Collection, error)
	}
	RequestCreator interface {
		Create(ctx context.Context, input request.Create, opt request.CreateOpt) (*entities.Request, error)
	}
	ExampleCreator interface {
		Create(ctx context.Context, in example.Create, opt example.CreateOpt) (*entities.ResponseExample, error)
	}
)

func ImportCollection(
	ctx context.Context,
	data []byte,
	opts ImportOpts,
	collUC CollectionCreator,
	reqUC RequestCreator,
	exUC ExampleCreator,
	envUC EnvironmentCreator,
) (*ImportResult, error) {
	const funcName = "postman.ImportCollection"
	const collectionVariablesName = "Collection variables"

	var pc PostmanCollection
	if err := json.Unmarshal(data, &pc); err != nil {
		return nil, fmt.Errorf("%s: invalid JSON: %w", funcName, err)
	}

	if !strings.Contains(pc.Info.Schema, "v2.1") {
		return nil, fmt.Errorf("%s: unsupported schema, only Postman Collection v2.1 is supported (export your collection as v2.1 from Postman)", funcName)
	}

	result := &ImportResult{}

	rootSubject := portability.Subject{Kind: portability.KindCollection, Name: pc.Info.Name}
	rootAuthType, rootAuthData, warnings := mapAuth(pc.Auth, rootSubject)
	result.Warnings = append(result.Warnings, warnings...)
	result.Warnings = append(result.Warnings, warnDescriptionType(rootSubject, pc.Info.Description)...)
	rootDescription, rootDescWarnings := portability.ClampDescription(rootSubject, pc.Info.Description.Text())
	result.Warnings = append(result.Warnings, rootDescWarnings...)
	rootScripts := collectScripts(pc.Event, rootSubject, opts.IncludeScripts, result)

	rootColl, err := collUC.Create(ctx, collection.Create{
		Name:        pc.Info.Name,
		Description: rootDescription,
		ParentID:    opts.ParentID,
		AuthType:    rootAuthType,
		AuthData:    rootAuthData,
		PreScript:   rootScripts.pre,
		PostScript:  rootScripts.post,
	}, collection.CreateOpt{
		UserID:      opts.UserID,
		WorkspaceID: opts.WorkspaceID,
	})
	if err != nil {
		return nil, fmt.Errorf("%s: failed to create root collection: %w", funcName, err)
	}
	result.RootID = rootColl.ID
	result.FoldersCreated++

	if err := importItems(ctx, pc.Item, rootColl.ID, opts, collUC, reqUC, exUC, result); err != nil {
		return nil, err
	}
	vars, varWarnings := collectionVariables(pc.Variable)
	if len(vars) > 0 {
		name := strings.TrimSpace(pc.Info.Name)
		if name == "" {
			name = collectionVariablesName
		}
		env, err := createEnvironment(ctx, envUC, name, vars, opts.WorkspaceID, opts.UserID)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", funcName, err)
		}
		result.EnvironmentID = env.ID
		result.EnvironmentName = env.Name
	}
	result.Warnings = append(result.Warnings, varWarnings...)
	if result.scriptsImported {
		result.warn(portability.WarnPostmanScripts, portability.Subject{})
	}
	if len(result.suspectExamples) > 0 {
		result.warn(portability.WarnExampleSecrets, portability.Subject{},
			"examples", portability.List{Items: result.suspectExamples, Sep: "; "})
	}

	return result, nil
}

func collectionVariables(raw json.RawMessage) ([]environment.AddVariable, []portability.Warning) {
	if len(raw) == 0 {
		return nil, nil
	}
	var items []json.RawMessage
	if err := json.Unmarshal(raw, &items); err != nil {
		return nil, []portability.Warning{portability.Warn(portability.WarnPostmanVariablesUnreadable, portability.Subject{})}
	}

	var (
		vars       []environment.AddVariable
		unreadable bool
		unnamed    int
	)
	for _, item := range items {
		var fields map[string]json.RawMessage
		if !bytes.HasPrefix(item, []byte("{")) || json.Unmarshal(item, &fields) != nil {
			unreadable = true
			continue
		}
		key, keyOK := variableField[string](fields, "key")
		if strings.TrimSpace(key) == "" {
			id, idOK := variableField[string](fields, "id")
			key = id
			keyOK = keyOK && idOK
		}
		if strings.TrimSpace(key) == "" {
			if keyOK {
				unnamed++
			} else {
				unreadable = true
			}
			continue
		}
		value, _ := variableField[postmanValue](fields, "value")
		typ, _ := variableField[string](fields, "type")
		disabled, _ := variableField[bool](fields, "disabled")
		vars = append(vars, environment.AddVariable{
			Key:      key,
			Value:    string(value),
			IsSecret: typ == "secret",
			Disabled: disabled,
		})
	}

	var warnings []portability.Warning
	if unreadable {
		warnings = append(warnings, portability.Warn(portability.WarnPostmanVariablesUnreadable, portability.Subject{}))
	}
	if unnamed > 0 {
		warnings = append(warnings, portability.Warn(portability.WarnVariablesUnnamed, portability.Subject{}, "n", unnamed))
	}
	return vars, warnings
}

func variableField[T any](fields map[string]json.RawMessage, name string) (T, bool) {
	var v T
	raw, found := fields[name]
	if !found {
		return v, true
	}
	if err := json.Unmarshal(raw, &v); err != nil {
		var zero T
		return zero, false
	}
	return v, true
}

func importItems(
	ctx context.Context,
	items []PostmanItem,
	parentCollectionID uuid.UUID,
	opts ImportOpts,
	collUC CollectionCreator,
	reqUC RequestCreator,
	exUC ExampleCreator,
	result *ImportResult,
) error {
	const funcName = "postman.importItems"
	const untitledFolder = "Untitled folder"

	for _, item := range items {
		children := derefItems(item.Item)
		nameless := strings.TrimSpace(item.Name) == ""

		if item.Request == nil && nameless && len(children) == 0 {
			result.warn(portability.WarnPostmanItemSkipped, portability.Subject{})
			continue
		}
		if item.IsFolder() {
			folderName := item.Name
			if nameless {
				folderName = untitledFolder
				result.warn(portability.WarnPostmanUnnamedFolder, portability.Subject{},
					"n", len(children), "name", untitledFolder)
			}
			folderSubject := portability.Subject{Kind: portability.KindFolder, Name: folderName}
			folderAuthType, folderAuthData, warnings := mapAuth(item.Auth, folderSubject)
			result.Warnings = append(result.Warnings, warnings...)
			result.Warnings = append(result.Warnings, warnDescriptionType(folderSubject, item.Description)...)
			folderDescription, folderDescWarnings := portability.ClampDescription(folderSubject, item.Description.Text())
			result.Warnings = append(result.Warnings, folderDescWarnings...)
			folderScripts := collectScripts(item.Event, folderSubject, opts.IncludeScripts, result)
			subColl, err := collUC.Create(ctx, collection.Create{
				Name:        folderName,
				Description: folderDescription,
				ParentID:    &parentCollectionID,
				AuthType:    folderAuthType,
				AuthData:    folderAuthData,
				PreScript:   folderScripts.pre,
				PostScript:  folderScripts.post,
			}, collection.CreateOpt{
				UserID:      opts.UserID,
				WorkspaceID: opts.WorkspaceID,
			})
			if err != nil {
				return fmt.Errorf("%s: failed to create folder %q: %w", funcName, folderName, err)
			}
			result.FoldersCreated++

			if err := importItems(ctx, children, subColl.ID, opts, collUC, reqUC, exUC, result); err != nil {
				return err
			}
		} else {
			requestSubject := portability.Subject{Kind: portability.KindRequest, Name: item.Name}
			if len(children) > 0 {
				result.warn(portability.WarnPostmanNestedItemsSkipped, requestSubject)
			}
			input, warnings := mapPostmanRequest(item, parentCollectionID, opts.IncludeScripts, result)
			result.Warnings = append(result.Warnings, warnings...)

			created, err := reqUC.Create(ctx, input, request.CreateOpt{
				UserID: opts.UserID,
			})
			if err != nil {
				return fmt.Errorf("%s: failed to create request %q: %w", funcName, item.Name, err)
			}
			result.RequestsCreated++

			importExamples(ctx, item.Response, created.ID, input.Protocol, requestSubject, opts.UserID, exUC, result)
		}
	}

	return nil
}

func (r *ImportResult) warn(code portability.WarningCode, subject portability.Subject, kv ...any) {
	r.Warnings = append(r.Warnings, portability.Warn(code, subject, kv...))
}

func derefItems(p *[]PostmanItem) []PostmanItem {
	if p == nil {
		return nil
	}
	return *p
}

func warnDescriptionType(subject portability.Subject, d *PostmanDescription) []portability.Warning {
	if d == nil || d.Type == "" || d.Type == "text/markdown" || d.Type == "text/plain" {
		return nil
	}
	return []portability.Warning{portability.Warn(portability.WarnPostmanDescriptionType, subject, "type", d.Type)}
}

// Picks by presence: an empty request-level description is a clear, not a fallback.
func requestDescription(item PostmanItem, subject portability.Subject) (string, []portability.Warning) {
	var (
		warnings []portability.Warning
		reqDesc  *PostmanDescription
	)
	if item.Request != nil {
		reqDesc = item.Request.Description
	}
	for _, d := range []*PostmanDescription{item.Description, reqDesc} {
		for _, w := range warnDescriptionType(subject, d) {
			if !slices.ContainsFunc(warnings, func(seen portability.Warning) bool { return seen.Text == w.Text }) {
				warnings = append(warnings, w)
			}
		}
	}

	itemText, reqText := item.Description.Text(), reqDesc.Text()
	if item.Description != nil && reqDesc != nil && itemText != reqText {
		warnings = append(warnings, portability.Warn(portability.WarnPostmanDescriptionsDiffer, subject))
	}
	text := itemText
	if reqDesc != nil {
		text = reqText
	}
	text, clampWarnings := portability.ClampDescription(subject, text)
	return text, append(warnings, clampWarnings...)
}

func mapPostmanRequest(
	item PostmanItem, collectionID uuid.UUID, includeScripts bool, result *ImportResult,
) (request.Create, []portability.Warning) {
	req := item.Request

	method := entities.HTTPMethod(strings.ToUpper(req.Method))
	if !method.IsValid() {
		method = entities.MethodGET
	}

	headers := mapHeaders(req.Header)
	subject := portability.Subject{Kind: portability.KindRequest, Name: item.Name}
	authType, authData, warnings := mapRequestAuth(req.Auth, subject)
	description, descWarnings := requestDescription(item, subject)
	warnings = append(warnings, descWarnings...)
	scripts := collectScripts(item.Event, subject, includeScripts, result)

	// Detect GraphQL body mode — store data in GraphQL fields, not Body.
	if req.Body != nil && req.Body.Mode == "graphql" && req.Body.Graphql != nil {
		return request.Create{
			CollectionID:     collectionID,
			Name:             item.Name,
			Description:      description,
			Protocol:         entities.ProtocolGraphQL,
			Method:           entities.MethodPOST,
			URL:              req.URL.Raw,
			Headers:          headers,
			BodyType:         entities.BodyTypeNone,
			AuthType:         authType,
			AuthData:         authData,
			PreScript:        scripts.pre,
			PostScript:       scripts.post,
			GraphQLQuery:     req.Body.Graphql.Query,
			GraphQLVariables: req.Body.Graphql.Variables,
		}, warnings
	}

	bodyType, body, bodyWarnings := mapBody(req.Body, subject)
	warnings = append(warnings, bodyWarnings...)

	return request.Create{
		CollectionID: collectionID,
		Name:         item.Name,
		Description:  description,
		Protocol:     entities.ProtocolHTTP,
		Method:       method,
		URL:          req.URL.Raw,
		Headers:      headers,
		Body:         body,
		BodyType:     bodyType,
		AuthType:     authType,
		AuthData:     authData,
		PreScript:    scripts.pre,
		PostScript:   scripts.post,
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

// File paths are never imported: a foreign path must not pick what we upload.
func mapBody(body *PostmanBody, subject portability.Subject) (entities.BodyType, string, []portability.Warning) {
	if body == nil || body.Mode == "" {
		return entities.BodyTypeNone, "", nil
	}

	switch body.Mode {
	case "raw":
		lang := ""
		if body.Options != nil && body.Options.Raw != nil {
			lang = body.Options.Raw.Language
		}

		switch lang {
		case "json":
			return entities.BodyTypeJSON, body.Raw, nil
		case "xml", "html":
			return entities.BodyTypeXML, body.Raw, nil
		default:
			return entities.BodyTypeRaw, body.Raw, nil
		}
	case "formdata":
		fields, warnings := mapFormData(body.FormData, subject)
		return entities.BodyTypeForm, fields, warnings
	case "urlencoded":
		return entities.BodyTypeForm, mapURLEncoded(body.URLEncoded), nil
	case "file":
		return entities.BodyTypeBinary, "", []portability.Warning{portability.Warn(portability.WarnFileBodyDropped, subject)}
	case "graphql":
		// GraphQL data is handled separately in mapPostmanRequest; body fields unused.
		return entities.BodyTypeNone, "", nil
	default:
		return entities.BodyTypeRaw, body.Raw, nil
	}
}

func mapFormData(formData []PostmanKV, subject portability.Subject) (string, []portability.Warning) {
	var warnings []portability.Warning
	fields := make([]map[string]any, 0, len(formData))
	for _, fd := range formData {
		fieldType := fd.Type
		if fieldType == "" {
			fieldType = "text"
		}
		value := fd.Value
		if fieldType == "file" {
			value = ""
			warnings = append(warnings, portability.Warn(portability.WarnFileFieldDropped, subject, "field", fd.Key))
		}
		fields = append(fields, formField(fd.Key, value, fieldType, !fd.Disabled))
	}
	return formFieldsJSON(fields), warnings
}

func mapURLEncoded(kvs []PostmanKV) string {
	fields := make([]map[string]any, 0, len(kvs))
	for _, kv := range kvs {
		fields = append(fields, formField(kv.Key, kv.Value, "text", !kv.Disabled))
	}
	return formFieldsJSON(fields)
}

func formField(key, value, fieldType string, enabled bool) map[string]any {
	return map[string]any{"key": key, "value": value, "type": fieldType, "enabled": enabled}
}

func formFieldsJSON(fields []map[string]any) string {
	if len(fields) == 0 {
		return "[]"
	}
	data, _ := json.Marshal(fields)
	return string(data)
}

type itemScripts struct{ pre, post string }

func collectScripts(events []PostmanEvent, subject portability.Subject, include bool, result *ImportResult) itemScripts {
	if !include {
		return itemScripts{}
	}
	var pre, post []string
	for _, ev := range events {
		code := strings.Join(ev.Script.Exec, "\n")
		if strings.TrimSpace(code) == "" {
			continue
		}
		if ev.Listen != "prerequest" && ev.Listen != "test" {
			continue
		}
		if ev.Disabled {
			result.warn(portability.WarnPostmanDisabledScript, subject, "event", ev.Listen)
			continue
		}
		if ev.Listen == "prerequest" {
			pre = append(pre, code)
		} else {
			post = append(post, code)
		}
		result.scriptsImported = true
	}
	return itemScripts{pre: strings.Join(pre, "\n"), post: strings.Join(post, "\n")}
}

// In Postman a missing auth block inherits; noauth is an explicit none.
func mapRequestAuth(a *PostmanAuth, subject portability.Subject) (entities.AuthType, string, []portability.Warning) {
	if a == nil {
		return entities.AuthTypeInherit, "{}", nil
	}
	return mapAuth(a, subject)
}

// mapAuth translates a Postman auth block; unsupported schemes degrade to none
// and are reported so the import can tell the user what it dropped.
func mapAuth(a *PostmanAuth, subject portability.Subject) (entities.AuthType, string, []portability.Warning) {
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
		return mapOAuth2Auth(a.OAuth2, subject)
	case "jwt":
		return mapJWTAuth(a.JWT, subject)
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
		return entities.AuthTypeNone, "{}", []portability.Warning{portability.Warn(portability.WarnAuthUnsupported, subject, "type", a.Type)}
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

func mapOAuth2Auth(kvs []PostmanAuthKV, subject portability.Subject) (entities.AuthType, string, []portability.Warning) {
	tokenURL := findAuthKV(kvs, "accessTokenUrl")
	// A block with only a pasted token cannot acquire anything; it is a bearer token.
	if tokenURL == "" {
		if token := findAuthKV(kvs, "accessToken"); token != "" {
			return entities.AuthTypeBearer, authDataJSON(map[string]any{"token": token}), []portability.Warning{
				portability.Warn(portability.WarnPostmanOAuth2TokenOnly, subject),
			}
		}
	}

	var warnings []portability.Warning
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
			warnings = append(warnings, portability.Warn(portability.WarnOAuth2GrantUnsupported, subject, "grant", raw))
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

func mapJWTAuth(kvs []PostmanAuthKV, subject portability.Subject) (entities.AuthType, string, []portability.Warning) {
	var warnings []portability.Warning
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
			warnings = append(warnings, portability.Warn(portability.WarnJWTFieldDropped, subject, "field", field.postman))
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

func importExamples(
	ctx context.Context,
	responses []json.RawMessage,
	requestID uuid.UUID,
	protocol entities.Protocol,
	subject portability.Subject,
	userID string,
	exUC ExampleCreator,
	result *ImportResult,
) {
	for i, raw := range responses {
		warn := func(code portability.WarningCode, kv ...any) {
			result.warn(code, subject, append([]any{"index", i + 1}, kv...)...)
		}

		resp, err := decodeResponse(raw)
		if err != nil {
			note := decodeNote(err)
			warn(note.code, note.kv...)
			continue
		}
		in, notes := mapPostmanResponse(resp, requestID, protocol)
		for _, note := range notes {
			warn(note.code, note.kv...)
		}
		if len(in.Body) > domain.MaxExampleBodyLen {
			warn(portability.WarnPostmanExampleTooLarge, "kb", domain.MaxExampleBodyLen/1024)
			continue
		}
		if _, err := exUC.Create(ctx, in, example.CreateOpt{UserID: userID}); err != nil {
			note := rejectionNote(err)
			warn(note.code, note.kv...)
			continue
		}
		result.ExamplesCreated++
		if found := example.SuspectedSecrets(in.Headers, in.Body); len(found) > 0 {
			result.suspectExamples = append(result.suspectExamples, portability.SecretsLabel(subject.Name, in.Name, found))
		}
	}
}

// decodeResponse shadows originalRequest: an unreadable one must not cost the example.
func decodeResponse(raw json.RawMessage) (PostmanResponse, error) {
	var in struct {
		PostmanResponse
		OriginalRequest json.RawMessage `json:"originalRequest"`
	}
	if err := json.Unmarshal(raw, &in); err != nil {
		return PostmanResponse{}, err
	}
	return in.PostmanResponse, nil
}

func decodeNote(err error) exampleNote {
	var te *json.UnmarshalTypeError
	if !errors.As(err, &te) {
		return exampleNote{portability.WarnPostmanExampleSkipped, []any{"reason", err.Error()}}
	}
	field := strings.TrimPrefix(te.Field, "PostmanResponse.")
	if field == "" {
		return exampleNote{portability.WarnPostmanExampleNotObject, []any{"got", te.Value}}
	}
	var want string
	switch te.Type.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		want = "integer"
	case reflect.String:
		want = "string"
	case reflect.Slice:
		want = "array"
	default:
		want = "object"
	}
	return exampleNote{portability.WarnPostmanExampleWrongType, []any{"field", field, "want", want, "got", te.Value}}
}

func rejectionNote(err error) exampleNote {
	var ve *domain.ValidationError
	if !errors.As(err, &ve) || len(ve.Fields) == 0 {
		return exampleNote{portability.WarnPostmanExampleSkipped, []any{"reason", err.Error()}}
	}
	if _, ok := ve.Fields[example.PayloadField]; ok {
		return exampleNote{portability.WarnPostmanExamplePayloadTooLarge, []any{"kb", domain.MaxExamplePayloadLen / 1024}}
	}
	return exampleNote{portability.WarnPostmanExampleSkipped, []any{"reason", portability.Describe(ve)}}
}

type exampleNote struct {
	code portability.WarningCode
	kv   []any
}

func mapPostmanResponse(resp PostmanResponse, requestID uuid.UUID, protocol entities.Protocol) (example.Create, []exampleNote) {
	var notes []exampleNote

	code := resp.Code
	if code < 0 || code > 999 {
		notes = append(notes, exampleNote{portability.WarnPostmanExampleStatus, []any{"status", code}})
		code = 0
	}

	name := strings.TrimSpace(resp.Name)
	if name == "" {
		name = defaultExampleName(code, resp.Status)
	}
	if utf8.RuneCountInString(name) > maxExampleNameLen {
		name = string([]rune(name)[:maxExampleNameLen])
		notes = append(notes, exampleNote{portability.WarnPostmanExampleNameTruncated, []any{"max", maxExampleNameLen}})
	}

	headers := mapHeaders(resp.Header)
	if headers == nil {
		headers = []entities.HeaderItem{}
	}
	contentType := contentTypeHeader(headers)
	if contentType == "" {
		contentType = previewLanguageTypes[strings.ToLower(resp.PreviewLanguage)]
	}

	return example.Create{
		RequestID:   requestID,
		Name:        name,
		StatusCode:  code,
		StatusText:  resp.Status,
		Headers:     headers,
		Body:        resp.Body,
		ContentType: contentType,
		Protocol:    protocol,
	}, notes
}

var previewLanguageTypes = map[string]string{
	"json":       "application/json",
	"xml":        "application/xml",
	"html":       "text/html",
	"javascript": "application/javascript",
	"text":       "text/plain",
}

func contentTypeHeader(headers []entities.HeaderItem) string {
	for _, h := range headers {
		if strings.EqualFold(strings.TrimSpace(h.Key), "Content-Type") {
			return h.Value
		}
	}
	return ""
}

func defaultExampleName(code int, status string) string {
	var parts []string
	if code != 0 {
		parts = append(parts, strconv.Itoa(code))
	}
	if s := strings.TrimSpace(status); s != "" {
		parts = append(parts, s)
	}
	if len(parts) == 0 {
		return "Example"
	}
	return strings.Join(parts, " ")
}
