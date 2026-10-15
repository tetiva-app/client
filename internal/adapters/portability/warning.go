package portability

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

type WarningCode string

const (
	WarnDescriptionTruncated   WarningCode = "description_truncated"
	WarnExampleSkipped         WarningCode = "example_skipped"
	WarnExampleTooLarge        WarningCode = "example_too_large"
	WarnExamplePayloadTooLarge WarningCode = "example_payload_too_large"
	WarnExampleSecrets         WarningCode = "example_secrets"
	WarnFileFieldDropped       WarningCode = "file_field_dropped"
	WarnFileBodyDropped        WarningCode = "file_body_dropped"
	WarnAuthUnsupported        WarningCode = "auth_unsupported"
	WarnOAuth2GrantUnsupported WarningCode = "oauth2_grant_unsupported"
	WarnJWTFieldDropped        WarningCode = "jwt_field_dropped"
	WarnFormsSentURLEncoded    WarningCode = "forms_sent_urlencoded"
	WarnForeignScripts         WarningCode = "foreign_scripts"
	WarnVariablesUnnamed       WarningCode = "variables_unnamed"
	WarnUnnamedRenamed         WarningCode = "unnamed_renamed"
	WarnUnnamedRenamedIn       WarningCode = "unnamed_renamed_in"
	WarnNameTruncated          WarningCode = "name_truncated"

	WarnSnapshotPartMissing         WarningCode = "snapshot_part_missing"
	WarnSnapshotProtocolUnsupported WarningCode = "snapshot_protocol_unsupported"
	WarnSnapshotMethodUnsupported   WarningCode = "snapshot_method_unsupported"
	WarnSnapshotBodyUnsupported     WarningCode = "snapshot_body_unsupported"
	WarnSnapshotWSMessageFormat     WarningCode = "snapshot_ws_message_format"
	WarnSnapshotWSExamplesSkipped   WarningCode = "snapshot_ws_examples_skipped"
	WarnSnapshotExampleStatus       WarningCode = "snapshot_example_status"

	WarnPostmanScripts                WarningCode = "postman_scripts"
	WarnPostmanVariablesUnreadable    WarningCode = "postman_variables_unreadable"
	WarnPostmanItemSkipped            WarningCode = "postman_item_skipped"
	WarnPostmanUnnamedFolder          WarningCode = "postman_unnamed_folder"
	WarnPostmanNestedItemsSkipped     WarningCode = "postman_nested_items_skipped"
	WarnPostmanDescriptionType        WarningCode = "postman_description_type"
	WarnPostmanDescriptionsDiffer     WarningCode = "postman_descriptions_differ"
	WarnPostmanDisabledScript         WarningCode = "postman_disabled_script"
	WarnPostmanOAuth2TokenOnly        WarningCode = "postman_oauth2_token_only"
	WarnPostmanExampleSkipped         WarningCode = "postman_example_skipped"
	WarnPostmanExampleTooLarge        WarningCode = "postman_example_too_large"
	WarnPostmanExamplePayloadTooLarge WarningCode = "postman_example_payload_too_large"
	WarnPostmanExampleWrongType       WarningCode = "postman_example_wrong_type"
	WarnPostmanExampleNotObject       WarningCode = "postman_example_not_object"
	WarnPostmanExampleStatus          WarningCode = "postman_example_status"
	WarnPostmanExampleNameTruncated   WarningCode = "postman_example_name_truncated"
	WarnPostmanGlobals                WarningCode = "postman_globals"

	WarnInsomniaPathParamsDiffer  WarningCode = "insomnia_path_params_differ"
	WarnInsomniaWorkspaces        WarningCode = "insomnia_workspaces"
	WarnInsomniaDisabledParams    WarningCode = "insomnia_disabled_params"
	WarnInsomniaDisabledMetadata  WarningCode = "insomnia_disabled_metadata"
	WarnInsomniaGRPCNoProto       WarningCode = "insomnia_grpc_no_proto"
	WarnInsomniaVaultSecrets      WarningCode = "insomnia_vault_secrets"
	WarnInsomniaTemplateTags      WarningCode = "insomnia_template_tags"
	WarnInsomniaSocketIOSkipped   WarningCode = "insomnia_socketio_skipped"
	WarnInsomniaUnitTestsSkipped  WarningCode = "insomnia_unit_tests_skipped"
	WarnInsomniaCookiesSkipped    WarningCode = "insomnia_cookies_skipped"
	WarnInsomniaSpecsSkipped      WarningCode = "insomnia_specs_skipped"
	WarnInsomniaFolderHeaders     WarningCode = "insomnia_folder_headers"
	WarnInsomniaFolderEnvironment WarningCode = "insomnia_folder_environment"
	WarnInsomniaMethodUnsupported WarningCode = "insomnia_method_unsupported"
	WarnInsomniaGraphQLNotJSON    WarningCode = "insomnia_graphql_not_json"
	WarnInsomniaAuthDisabled      WarningCode = "insomnia_auth_disabled"
	WarnInsomniaAPIKeyLocation    WarningCode = "insomnia_apikey_location"

	WarnHoppscotchVariableConflicts WarningCode = "hoppscotch_variable_conflicts"
	WarnHoppscotchSecretsEmpty      WarningCode = "hoppscotch_secrets_empty"
	WarnHoppscotchHeadersCopied     WarningCode = "hoppscotch_headers_copied"
	WarnHoppscotchAuthOff           WarningCode = "hoppscotch_auth_off"

	WarnOpenAPIExternalRef               WarningCode = "openapi_external_ref"
	WarnOpenAPIBrokenRef                 WarningCode = "openapi_broken_ref"
	WarnOpenAPIAuthHeaderParam           WarningCode = "openapi_auth_header_param"
	WarnOpenAPIWebhooksSkipped           WarningCode = "openapi_webhooks_skipped"
	WarnOpenAPINoServer                  WarningCode = "openapi_no_server"
	WarnOpenAPIRelativeServer            WarningCode = "openapi_relative_server"
	WarnOpenAPIOwnServer                 WarningCode = "openapi_own_server"
	WarnOpenAPITraceSkipped              WarningCode = "openapi_trace_skipped"
	WarnOpenAPICallbacksSkipped          WarningCode = "openapi_callbacks_skipped"
	WarnOpenAPIQueryStyle                WarningCode = "openapi_query_style"
	WarnOpenAPILinksSkipped              WarningCode = "openapi_links_skipped"
	WarnOpenAPISchemesTogether           WarningCode = "openapi_schemes_together"
	WarnOpenAPISchemeUndefined           WarningCode = "openapi_scheme_undefined"
	WarnOpenAPISchemeUnsupported         WarningCode = "openapi_scheme_unsupported"
	WarnOpenAPISchemeLocationUnsupported WarningCode = "openapi_scheme_location_unsupported"
	WarnOpenAPIImplicitFlow              WarningCode = "openapi_implicit_flow"

	WarnHARAssetsSkipped     WarningCode = "har_assets_skipped"
	WarnHAREntriesSkipped    WarningCode = "har_entries_skipped"
	WarnHARUnreadableSkipped WarningCode = "har_unreadable_skipped"
	WarnHARCookiesDropped    WarningCode = "har_cookies_dropped"
	WarnHARCredentialsPlain  WarningCode = "har_credentials_plain"
	WarnHARFilesDropped      WarningCode = "har_files_dropped"
	WarnHARBodiesLeftOut     WarningCode = "har_bodies_left_out"
)

const (
	KindCollection  = "collection"
	KindFolder      = "folder"
	KindRequest     = "request"
	KindEnvironment = "environment"
	KindExample     = "example"
	KindOperation   = "operation"
)

type Subject struct {
	Kind string
	Name string
}

func (s Subject) String() string {
	switch {
	case s.Kind == KindOperation:
		return s.Name
	case s.Name == "":
		return s.Kind
	default:
		return s.Kind + ` "` + s.Name + `"`
	}
}

// Text is the English rendering for logs, MCP and a UI that does not know the code.
type Warning struct {
	Code    WarningCode
	Subject Subject
	Params  map[string]string
	Text    string
}

type List struct {
	Items []string
	Sep   string
}

type warningText struct{ one, other string }

// other, when set, is picked unless params["n"] is "1".
var warningTexts = map[WarningCode]warningText{
	WarnDescriptionTruncated:   {one: "description longer than {max} bytes was truncated"},
	WarnExampleSkipped:         {one: `example "{example}" skipped: {reason}`},
	WarnExampleTooLarge:        {one: `example "{example}" skipped: body is larger than {kb} KB`},
	WarnExamplePayloadTooLarge: {one: `example "{example}" skipped: with its headers it is larger than {kb} KB`},
	WarnExampleSecrets:         {one: "examples may contain secrets, check them before sharing: {examples}"},
	WarnFileFieldDropped:       {one: `file field "{field}" was imported without its file; pick it again`},
	WarnFileBodyDropped:        {one: "the file body was imported without its file; pick it again"},
	WarnAuthUnsupported:        {one: `auth type "{type}" is not supported and was imported as no auth`},
	WarnOAuth2GrantUnsupported: {one: `OAuth 2.0 grant "{grant}" is not supported; pick one in the Auth tab`},
	WarnJWTFieldDropped:        {one: "the JWT {field} is not a JSON object and was dropped"},
	WarnFormsSentURLEncoded: {
		one:   "{n} multipart request has no file field and will be sent as a URL-encoded form, not multipart: {requests}",
		other: "{n} multipart requests have no file field and will be sent as a URL-encoded form, not multipart: {requests}",
	},
	WarnForeignScripts: {one: "{app} scripts use the {api} API, which Tetiva does not run: rewrite them to pm.* after importing"},
	WarnVariablesUnnamed: {
		one:   "a variable without a name was skipped",
		other: "{n} variables without a name were skipped",
	},
	WarnUnnamedRenamed:   {one: `an unnamed {kind} was imported as "{name}"`},
	WarnUnnamedRenamedIn: {one: `an unnamed {kind} in "{parent}" was imported as "{name}"`},
	WarnNameTruncated:    {one: "name longer than {max} characters was truncated"},

	WarnSnapshotPartMissing:         {one: "the {part} part is missing, the request was skipped"},
	WarnSnapshotProtocolUnsupported: {one: `protocol "{protocol}" is not supported, the request was skipped`},
	WarnSnapshotMethodUnsupported:   {one: `method "{method}" is not supported, imported as GET`},
	WarnSnapshotBodyUnsupported:     {one: `body type "{type}" is not supported, imported without a body`},
	WarnSnapshotWSMessageFormat:     {one: `message "{message}" has an unknown format "{format}" and was skipped`},
	WarnSnapshotWSExamplesSkipped: {
		one:   "{n} example(s) skipped: WebSocket requests have no examples",
		other: "{n} example(s) skipped: WebSocket requests have no examples",
	},
	WarnSnapshotExampleStatus: {one: `example "{example}": status {status} is outside 0–999, imported as 0`},

	WarnPostmanScripts: {one: "scripts were imported: only part of the pm.* API is available " +
		"(pm.environment, pm.collectionVariables, pm.request, pm.response, pm.test, pm.expect), so some may need changes"},
	WarnPostmanVariablesUnreadable: {one: "collection variables could not be read and were skipped"},
	WarnPostmanItemSkipped:         {one: "an unnamed item with neither a request nor children was skipped"},
	WarnPostmanUnnamedFolder: {
		one:   `an unnamed folder with {n} item(s) was imported as "{name}" rather than failing the import`,
		other: `an unnamed folder with {n} item(s) was imported as "{name}" rather than failing the import`,
	},
	WarnPostmanNestedItemsSkipped:     {one: "items nested under a request were skipped"},
	WarnPostmanDescriptionType:        {one: "description is {type}, imported as plain text"},
	WarnPostmanDescriptionsDiffer:     {one: "item and request descriptions differ, request level kept"},
	WarnPostmanDisabledScript:         {one: "disabled {event} script was not imported"},
	WarnPostmanOAuth2TokenOnly:        {one: "the OAuth 2.0 block has an access token but no token URL; imported as a bearer token"},
	WarnPostmanExampleSkipped:         {one: "example {index} skipped: {reason}"},
	WarnPostmanExampleTooLarge:        {one: "example {index} skipped: body is larger than {kb} KB"},
	WarnPostmanExamplePayloadTooLarge: {one: "example {index} skipped: with its headers it is larger than {kb} KB"},
	WarnPostmanExampleWrongType:       {one: `example {index} skipped: field "{field}" must be of type {want}, got {got}`},
	WarnPostmanExampleNotObject:       {one: "example {index} skipped: it is {got}, not an object"},
	WarnPostmanExampleStatus:          {one: "example {index}: status code {status} is out of range, imported as 0"},
	WarnPostmanExampleNameTruncated:   {one: "example {index}: name longer than {max} characters was truncated"},
	WarnPostmanGlobals:                {one: `Tetiva has no global variables: they were imported as the environment "{name}"`},

	WarnInsomniaPathParamsDiffer: {
		one:   "path parameter {names} differs between requests: the environment keeps the first value",
		other: "path parameters {names} differ between requests: the environment keeps the first value",
	},
	WarnInsomniaWorkspaces: {
		one:   "the file has {n} workspaces, each was imported as its own collection: {names}",
		other: "the file has {n} workspaces, each was imported as its own collection: {names}",
	},
	WarnInsomniaDisabledParams: {
		one:   "{n} disabled query parameter was dropped: Tetiva keeps query parameters in the URL",
		other: "{n} disabled query parameters were dropped: Tetiva keeps query parameters in the URL",
	},
	WarnInsomniaDisabledMetadata: {
		one:   "{n} disabled gRPC metadata item was dropped",
		other: "{n} disabled gRPC metadata items were dropped",
	},
	WarnInsomniaGRPCNoProto: {one: "gRPC requests were imported without their proto files: pick a .proto file or use server reflection"},
	WarnInsomniaVaultSecrets: {
		one:   "{n} vault secret was imported as an empty secret variable: fill in its value",
		other: "{n} vault secrets were imported as empty secret variables: fill in their values",
	},
	WarnInsomniaTemplateTags: {
		one:   "{n} Insomnia template tag was left as-is ({names}): Tetiva does not run it, use a variable or a script instead",
		other: "{n} Insomnia template tags were left as-is ({names}): Tetiva does not run them, use variables or scripts instead",
	},
	WarnInsomniaSocketIOSkipped: {
		one:   "{n} Socket.IO request was skipped: Tetiva does not support Socket.IO",
		other: "{n} Socket.IO requests were skipped: Tetiva does not support Socket.IO",
	},
	WarnInsomniaUnitTestsSkipped: {one: "{n} unit test was not imported", other: "{n} unit tests were not imported"},
	WarnInsomniaCookiesSkipped: {
		one:   "{n} cookie from the cookie jar was not imported",
		other: "{n} cookies from the cookie jar were not imported",
	},
	WarnInsomniaSpecsSkipped:      {one: "{n} API spec document was not imported", other: "{n} API spec documents were not imported"},
	WarnInsomniaFolderHeaders:     {one: "folder headers were not imported"},
	WarnInsomniaFolderEnvironment: {one: "folder environment variables were not imported"},
	WarnInsomniaMethodUnsupported: {one: "method {method} is not supported and was imported as GET"},
	WarnInsomniaGraphQLNotJSON:    {one: "the GraphQL body is not JSON and was imported as the query"},
	WarnInsomniaAuthDisabled:      {one: "{type} auth is disabled in Insomnia and was imported as no auth"},
	WarnInsomniaAPIKeyLocation:    {one: "an API key sent as a {location} is not supported and was imported as no auth"},

	WarnHoppscotchVariableConflicts: {one: "variables defined more than once kept their first value: {keys}"},
	WarnHoppscotchSecretsEmpty:      {one: "Hoppscotch exports secrets without their values: fill in {keys}"},
	WarnHoppscotchHeadersCopied:     {one: "headers set on Hoppscotch collections and folders were copied into their requests"},
	WarnHoppscotchAuthOff: {
		one: "requests in folder {folders} will inherit the parent's auth: " +
			"Tetiva cannot turn auth off at a folder, set them to No Auth if they must go without",
		other: "requests in folders {folders} will inherit the parent's auth: " +
			"Tetiva cannot turn auth off at a folder, set them to No Auth if they must go without",
	},

	WarnOpenAPIExternalRef: {one: `references to "{target}" are not supported and were left empty`},
	WarnOpenAPIBrokenRef:   {one: `reference "{ref}" points to nothing and was left empty`},
	WarnOpenAPIAuthHeaderParam: {one: "the Authorization header parameter of {operations} was not imported: " +
		"set the auth on the collection or the request instead"},
	WarnOpenAPIWebhooksSkipped:  {one: "webhooks were skipped: {names}"},
	WarnOpenAPINoServer:         {one: "the document names no server: set baseUrl in the environment"},
	WarnOpenAPIRelativeServer:   {one: `the server URL "{url}" is relative: set baseUrl in the environment to the full address`},
	WarnOpenAPIOwnServer:        {one: `its own server "{url}" was ignored, the request uses {{baseUrl}}`},
	WarnOpenAPITraceSkipped:     {one: "TRACE {path} was skipped: TRACE requests are not supported"},
	WarnOpenAPICallbacksSkipped: {one: "callbacks were skipped"},
	WarnOpenAPIQueryStyle:       {one: `query parameter "{param}" uses style {style}, which was not reproduced`},
	WarnOpenAPILinksSkipped:     {one: "response links were skipped"},
	WarnOpenAPISchemesTogether:  {one: `security schemes {schemes} are required together: only "{scheme}" was imported`},
	WarnOpenAPISchemeUndefined:  {one: `security scheme "{scheme}" is not defined: requests that use it were imported without auth`},
	WarnOpenAPISchemeUnsupported: {one: `security scheme "{scheme}" ({type}) is not supported: ` +
		"requests that use it were imported without auth"},
	WarnOpenAPISchemeLocationUnsupported: {one: `security scheme "{scheme}" ({type} in {in}) is not supported: ` +
		"requests that use it were imported without auth"},
	WarnOpenAPIImplicitFlow: {one: `security scheme "{scheme}": the OAuth 2.0 implicit flow is not supported, ` +
		"imported as authorization code without a token URL"},

	WarnHARAssetsSkipped: {
		one:   "{n} page asset entry skipped: images, scripts, stylesheets, fonts and media are not imported",
		other: "{n} page asset entries skipped: images, scripts, stylesheets, fonts and media are not imported",
	},
	WarnHAREntriesSkipped: {
		one:   "{n} {scheme} entry skipped: only HTTP requests are imported",
		other: "{n} {scheme} entries skipped: only HTTP requests are imported",
	},
	WarnHARUnreadableSkipped: {
		one:   "{n} unreadable URL entry skipped: only HTTP requests are imported",
		other: "{n} unreadable URL entries skipped: only HTTP requests are imported",
	},
	WarnHARCookiesDropped: {one: "cookie headers were not imported: add the cookies in the cookie manager if the requests need them"},
	WarnHARCredentialsPlain: {
		one: "{n} request keeps credentials from the capture in its headers or query parameters as plain values: " +
			"move them to secret variables before sharing",
		other: "{n} requests keep credentials from the capture in their headers or query parameters as plain values: " +
			"move them to secret variables before sharing",
	},
	WarnHARFilesDropped: {
		one:   "{n} file field was imported without the file; pick it again in the request body",
		other: "{n} file fields were imported without the file; pick it again in the request body",
	},
	WarnHARBodiesLeftOut: {
		one:   "{n} response body was left out of the examples: only JSON, XML and plain-text bodies up to {kb} KB are kept",
		other: "{n} response bodies were left out of the examples: only JSON, XML and plain-text bodies up to {kb} KB are kept",
	},
}

const maxListed = 5

// The UI formats these with Intl, so English Text groups them the same way.
var groupedParams = map[string]bool{"n": true, "max": true, "kb": true}

var placeholder = regexp.MustCompile(`\{(\w+)\}`)

// kv alternates keys and values; a value is a string, an int or a List.
func Warn(code WarningCode, subject Subject, kv ...any) Warning {
	var params map[string]string
	if len(kv) > 0 {
		params = make(map[string]string, len(kv)/2)
	}
	for i := 0; i+1 < len(kv); i += 2 {
		key := fmt.Sprint(kv[i])
		switch v := kv[i+1].(type) {
		case string:
			params[key] = v
		case int:
			params[key] = strconv.Itoa(v)
		case List:
			items, sep := v.Items, v.Sep
			if sep == "" {
				sep = ", "
			}
			if len(items) > maxListed {
				params[key+"More"] = strconv.Itoa(len(items) - maxListed)
				items = items[:maxListed]
			}
			params[key] = strings.Join(items, sep)
		default:
			params[key] = fmt.Sprint(v)
		}
	}
	return Warning{Code: code, Subject: subject, Params: params, Text: render(code, subject, params)}
}

func render(code WarningCode, subject Subject, params map[string]string) string {
	forms, ok := warningTexts[code]
	if !ok {
		return string(code)
	}
	tmpl := forms.one
	if forms.other != "" && params["n"] != "1" {
		tmpl = forms.other
	}
	text := placeholder.ReplaceAllStringFunc(tmpl, func(m string) string {
		key := m[1 : len(m)-1]
		v, ok := params[key]
		if !ok {
			return m
		}
		if groupedParams[key] {
			v = groupThousands(v)
		}
		if more := params[key+"More"]; more != "" {
			return v + " and " + more + " more"
		}
		return v
	})
	if subject.Kind != "" {
		return subject.String() + ": " + text
	}
	return text
}

func groupThousands(s string) string {
	if len(s) <= 3 || strings.Trim(s, "0123456789") != "" {
		return s
	}
	var b strings.Builder
	for i := range len(s) {
		if i > 0 && (len(s)-i)%3 == 0 {
			b.WriteByte(',')
		}
		b.WriteByte(s[i])
	}
	return b.String()
}

func Texts(ws []Warning) []string {
	out := make([]string, 0, len(ws))
	for _, w := range ws {
		out = append(out, w.Text)
	}
	return out
}
