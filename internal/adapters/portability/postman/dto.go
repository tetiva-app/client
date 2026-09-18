package postman

import (
	"encoding/json"
	"fmt"
	"strings"
)

// PostmanDescription is a v2.1 description: a plain string or a {content, type} object.
type PostmanDescription struct {
	Content string
	Type    string
}

func (d *PostmanDescription) UnmarshalJSON(b []byte) error {
	const funcName = "postman.PostmanDescription.UnmarshalJSON"

	if string(b) == "null" {
		*d = PostmanDescription{}
		return nil
	}
	var s string
	if err := json.Unmarshal(b, &s); err == nil {
		*d = PostmanDescription{Content: s}
		return nil
	}
	var obj struct {
		Content string `json:"content"`
		Type    string `json:"type"`
	}
	if err := json.Unmarshal(b, &obj); err != nil {
		return fmt.Errorf("%s: %w", funcName, err)
	}
	*d = PostmanDescription{Content: obj.Content, Type: obj.Type}
	return nil
}

func (d PostmanDescription) MarshalJSON() ([]byte, error) {
	const funcName = "postman.PostmanDescription.MarshalJSON"

	raw, err := json.Marshal(d.Content)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", funcName, err)
	}
	return raw, nil
}

func (d *PostmanDescription) Text() string {
	if d == nil {
		return ""
	}
	return d.Content
}

func descriptionOf(s string) *PostmanDescription {
	if s == "" {
		return nil
	}
	return &PostmanDescription{Content: s, Type: "text/markdown"}
}

type PostmanCollection struct {
	Info  PostmanInfo    `json:"info"`
	Item  []PostmanItem  `json:"item"`
	Auth  *PostmanAuth   `json:"auth,omitempty"`
	Event []PostmanEvent `json:"event,omitempty"`
}

type PostmanInfo struct {
	PostmanID   string              `json:"_postman_id,omitempty"`
	Name        string              `json:"name"`
	Description *PostmanDescription `json:"description,omitempty"`
	Schema      string              `json:"schema"`
	ExporterID  string              `json:"_exporter_id,omitempty"`
}

type PostmanItem struct {
	Name        string              `json:"name"`
	Description *PostmanDescription `json:"description,omitempty"`
	Item        *[]PostmanItem      `json:"item,omitempty"`
	Request     *PostmanRequest     `json:"request,omitempty"`
	Auth        *PostmanAuth        `json:"auth,omitempty"`
	Event       []PostmanEvent      `json:"event,omitempty"`
}

// A childless folder has no item array, so only the request tells them apart.
func (i *PostmanItem) IsFolder() bool {
	return i.Request == nil
}

type PostmanRequest struct {
	Method      string              `json:"method"`
	Header      []PostmanKV         `json:"header"`
	Body        *PostmanBody        `json:"body,omitempty"`
	URL         PostmanURL          `json:"url"`
	Auth        *PostmanAuth        `json:"auth,omitempty"`
	Description *PostmanDescription `json:"description,omitempty"`
}

type PostmanURL struct {
	Raw   string      `json:"raw"`
	Host  []string    `json:"host,omitempty"`
	Path  []string    `json:"path,omitempty"`
	Query []PostmanKV `json:"query,omitempty"`
}

// PostmanKV is a key-value pair used for headers, query params, and form data.
type PostmanKV struct {
	Key      string `json:"key"`
	Value    string `json:"value"`
	Disabled bool   `json:"disabled,omitempty"`
	Type     string `json:"type,omitempty"`
}

type PostmanBody struct {
	Mode     string              `json:"mode"`
	Raw      string              `json:"raw,omitempty"`
	Options  *PostmanBodyOpt     `json:"options,omitempty"`
	FormData []PostmanKV         `json:"formdata,omitempty"`
	Graphql  *PostmanGraphQLBody `json:"graphql,omitempty"`
}

// PostmanGraphQLBody holds GraphQL query and variables for the graphql body mode.
type PostmanGraphQLBody struct {
	Query     string `json:"query"`
	Variables string `json:"variables,omitempty"`
}

type PostmanBodyOpt struct {
	Raw *PostmanRawOpt `json:"raw,omitempty"`
}

type PostmanRawOpt struct {
	Language string `json:"language,omitempty"`
}

type PostmanAuth struct {
	Type   string          `json:"type"`
	Bearer []PostmanAuthKV `json:"bearer,omitempty"`
	Basic  []PostmanAuthKV `json:"basic,omitempty"`
	APIKey []PostmanAuthKV `json:"apikey,omitempty"`
	OAuth2 []PostmanAuthKV `json:"oauth2,omitempty"`
	JWT    []PostmanAuthKV `json:"jwt,omitempty"`
	Digest []PostmanAuthKV `json:"digest,omitempty"`
	AWSV4  []PostmanAuthKV `json:"awsv4,omitempty"`
}

// Real v2.1 exports put booleans and arrays in an auth value (isSecretBase64Encoded,
// authRequestParams), so it stays raw and is coerced on read instead of failing the import.
type PostmanAuthKV struct {
	Key   string          `json:"key"`
	Value json.RawMessage `json:"value,omitempty"`
	Type  string          `json:"type,omitempty"`
}

// String coerces a scalar value to text; arrays and objects have no text form here.
func (kv PostmanAuthKV) String() string {
	raw := strings.TrimSpace(string(kv.Value))
	if raw == "" || raw == "null" {
		return ""
	}
	var s string
	if err := json.Unmarshal(kv.Value, &s); err == nil {
		return s
	}
	if raw[0] == '[' || raw[0] == '{' {
		return ""
	}
	return raw
}

// Extension keys carry auth fields Postman's format cannot express, so an
// export of our own collection reads back unchanged. Postman ignores them.
const (
	// Postman's client_authentication has no value for a public client
	// (clientAuth "none"), and a missing one reads back as the basic default.
	clientAuthExtKey = "tetivaClientAuth"
	// Postman names the JWT query parameter but not the OAuth 2.0 one.
	queryParamExtKey = "tetivaQueryParam"
	// Loopback port and device endpoint: Postman runs neither flow our way.
	redirectPortExtKey  = "tetivaRedirectPort"
	deviceAuthURLExtKey = "tetivaDeviceAuthUrl"
	// Postman wants a literal exp claim instead of a lifetime.
	expiresInExtKey = "tetivaExpiresIn"
)

// authKV builds the string-valued pair the exporter writes.
func authKV(key, value string) PostmanAuthKV {
	raw, _ := json.Marshal(value)
	return PostmanAuthKV{Key: key, Value: raw}
}

// boolAuthKV writes a real JSON boolean; Postman reads the string "false" as true.
func boolAuthKV(key string, value bool) PostmanAuthKV {
	raw, _ := json.Marshal(value)
	return PostmanAuthKV{Key: key, Value: raw}
}

// PostmanEvent represents a pre-request or test script.
type PostmanEvent struct {
	Listen string        `json:"listen"`
	Script PostmanScript `json:"script"`
}

type PostmanScript struct {
	Type string   `json:"type"`
	Exec []string `json:"exec"`
}

type PostmanEnvironment struct {
	ID     string            `json:"id"`
	Name   string            `json:"name"`
	Values []PostmanEnvValue `json:"values"`
	Scope  string            `json:"_postman_variable_scope,omitempty"`
}

type PostmanEnvValue struct {
	Key     string `json:"key"`
	Value   string `json:"value"`
	Type    string `json:"type,omitempty"`
	Enabled bool   `json:"enabled"`
}

const SchemaV21 = "https://schema.getpostman.com/json/collection/v2.1.0/collection.json"
